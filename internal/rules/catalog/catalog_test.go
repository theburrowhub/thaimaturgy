package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

type manifestRuleset struct {
	manifest rules.Manifest
	stateErr error
}

func (r manifestRuleset) Manifest(context.Context) (rules.Manifest, error) { return r.manifest, nil }
func (manifestRuleset) ListActions(context.Context, rules.CatalogRequest) ([]rules.ActionDescriptor, error) {
	return nil, nil
}
func (manifestRuleset) Start(context.Context, rules.StartRequest) (rules.Step, error) {
	return rules.Step{}, nil
}
func (manifestRuleset) Resume(context.Context, rules.ResumeRequest) (rules.Step, error) {
	return rules.Step{}, nil
}
func (manifestRuleset) Project(context.Context, rules.ProjectRequest) (rules.Projection, error) {
	return rules.Projection{}, nil
}
func (manifestRuleset) Explain(context.Context, rules.ExplainRequest) (rules.Explanation, error) {
	return rules.Explanation{}, nil
}
func (r manifestRuleset) ValidateState(context.Context, rules.ValidateStateRequest) error {
	return r.stateErr
}
func (manifestRuleset) Reduce(context.Context, rules.ReduceRequest) (rules.ReduceResult, error) {
	return rules.ReduceResult{}, nil
}
func (manifestRuleset) Migrate(context.Context, rules.MigrateRequest) (rules.MigrateResult, error) {
	return rules.MigrateResult{}, nil
}

func registerVersion(t *testing.T, catalog *Catalog, id, version, material string) rules.Lock {
	t.Helper()
	manifest := rules.Manifest{
		ID: id, Name: id, Version: version, ProtocolVersion: rules.ProtocolVersion,
		Runtime: rules.Runtime{Kind: rules.RuntimeBuiltin},
	}
	artifact, err := rules.NewArtifact(manifest, strings.NewReader(material))
	if err != nil {
		t.Fatal(err)
	}
	initialState, err := rules.NewPayload([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.Register(context.Background(), artifact, manifestRuleset{manifest: manifest}, initialState); err != nil {
		t.Fatal(err)
	}
	return artifact.Lock()
}

func TestRegisterValidatesAndRetainsInitialStateAtomically(t *testing.T) {
	catalog := New()
	manifest := rules.Manifest{
		ID: "stateful", Name: "Stateful", Version: "1.0.0", ProtocolVersion: rules.ProtocolVersion,
		Runtime: rules.Runtime{Kind: rules.RuntimeBuiltin},
	}
	artifact, err := rules.NewArtifact(manifest, strings.NewReader("stateful"))
	if err != nil {
		t.Fatal(err)
	}
	initial, err := rules.NewPayload([]byte(`{"counter":0}`))
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("bad state")
	implementation := manifestRuleset{manifest: manifest, stateErr: sentinel}
	if err := catalog.Register(context.Background(), artifact, implementation, initial); !errors.Is(err, sentinel) {
		t.Fatalf("Register error = %v", err)
	}
	if _, err := catalog.Lookup(artifact.Lock()); !errors.Is(err, rules.ErrRulesetNotFound) {
		t.Fatalf("semantically invalid package became visible: %v", err)
	}

	implementation.stateErr = nil
	if err := catalog.Register(context.Background(), artifact, implementation, initial); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.InitialState(artifact.Lock())
	if err != nil || got.String() != initial.String() {
		t.Fatalf("InitialState = %s, %v", got.String(), err)
	}
}

func TestResolveSelectsHighestCompatibleExactArtifact(t *testing.T) {
	catalog := New()
	locks := []rules.Lock{
		registerVersion(t, catalog, "example", "1.0.0", "one"),
		registerVersion(t, catalog, "example", "1.5.0", "two"),
		registerVersion(t, catalog, "example", "2.0.0", "three"),
	}
	tests := []struct {
		constraint string
		want       rules.Lock
	}{
		{"1.0.0", locks[0]},
		{"^1.0.0", locks[1]},
		{"~1.0.0", locks[0]},
		{">=1.0.0 <2.0.0", locks[1]},
		{"*", locks[2]},
	}
	for _, test := range tests {
		t.Run(test.constraint, func(t *testing.T) {
			lock, implementation, err := catalog.Resolve(rules.Requirement{ID: "example", Version: rules.VersionConstraint(test.constraint)})
			if err != nil || lock != test.want || implementation == nil {
				t.Fatalf("Resolve = %#v, %v, %v; want %#v", lock, implementation, err, test.want)
			}
		})
	}
}

func TestResolveRejectsUnknownMalformedAndIncompatibleRequirements(t *testing.T) {
	catalog := New()
	registerVersion(t, catalog, "example", "1.2.3", "one")
	tests := []rules.Requirement{
		{ID: "missing", Version: "*"},
		{ID: "example", Version: "^2.0.0"},
		{ID: "example", Version: "banana"},
		{ID: "example", Version: ">=1.0.0 || <2.0.0"},
	}
	for _, requirement := range tests {
		if _, _, err := catalog.Resolve(requirement); err == nil {
			t.Fatalf("requirement %#v unexpectedly resolved", requirement)
		}
	}
	if _, _, err := catalog.Resolve(rules.Requirement{ID: "missing", Version: "*"}); !errors.Is(err, ErrNoCompatibleRuleset) {
		t.Fatalf("missing error = %v", err)
	}
}

func TestVersionOrderingSupportsKernelSizedNumericComponents(t *testing.T) {
	left, err := parseVersion("18446744073709551616.0.0")
	if err != nil {
		t.Fatal(err)
	}
	right, err := parseVersion("18446744073709551617.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if compareVersions(left, right) >= 0 {
		t.Fatal("unbounded SemVer numeric identifiers did not preserve order")
	}
	constraint, err := parseConstraint("^18446744073709551616.0.0")
	if err != nil || !constraint.matches(left) || constraint.matches(right) {
		t.Fatalf("large caret constraint = %#v, %v", constraint, err)
	}
	preLeft, _ := parseVersion("1.0.0-alpha.18446744073709551616")
	preRight, _ := parseVersion("1.0.0-alpha.18446744073709551617")
	if compareVersions(preLeft, preRight) >= 0 {
		t.Fatal("unbounded numeric prerelease identifiers did not preserve order")
	}
}

func TestExactConstraintDoesNotAliasBuildMetadata(t *testing.T) {
	plain, _ := parseVersion("1.0.0")
	built, _ := parseVersion("1.0.0+linux")
	constraint, err := parseConstraint("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !constraint.matches(plain) || constraint.matches(built) {
		t.Fatal("exact requirement did not preserve the complete release identity")
	}
}

func TestVersionOrderingIncludesPrereleases(t *testing.T) {
	versions := []string{"1.0.0-alpha.1", "1.0.0-alpha.2", "1.0.0", "1.1.0"}
	for index := 1; index < len(versions); index++ {
		left, err := parseVersion(versions[index-1])
		if err != nil {
			t.Fatal(err)
		}
		right, err := parseVersion(versions[index])
		if err != nil {
			t.Fatal(err)
		}
		if compareVersions(left, right) >= 0 {
			t.Fatalf("%s did not sort before %s", versions[index-1], versions[index])
		}
	}
}
