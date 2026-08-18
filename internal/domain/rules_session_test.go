package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

const (
	rulesDigestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	rulesDigestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func rulesTestLock(digest string) rules.Lock {
	return rules.Lock{
		ID:              "example.rules",
		Version:         "1.2.3",
		Digest:          digest,
		ProtocolVersion: rules.ProtocolVersion,
	}
}

func rulesTestPayload(t *testing.T, raw string) rules.Payload {
	t.Helper()
	payload, err := rules.NewPayload([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestRulesSessionLegacyJSONAndRoundTrip(t *testing.T) {
	legacy := NewSessionState("legacy", nil)
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"rules"`) {
		t.Fatalf("legacy session unexpectedly contains rules block: %s", raw)
	}
	if _, ok := legacy.RulesSnapshot(); ok {
		t.Fatal("legacy session unexpectedly has a rules snapshot")
	}

	lock := rulesTestLock(rulesDigestA)
	state := rulesTestPayload(t, `{"phase":"opening"}`)
	created, err := legacy.BindRules(lock, state)
	if err != nil || !created {
		t.Fatalf("BindRules created=%v err=%v", created, err)
	}
	legacy.Rules.Revision = 7

	raw, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var restored SessionState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := restored.RulesSnapshot()
	if !ok {
		t.Fatal("restored session has no valid rules snapshot")
	}
	if snapshot.Ruleset != lock || snapshot.Revision != 7 || snapshot.State.String() != state.String() {
		t.Fatalf("restored snapshot = %+v state=%s", snapshot, snapshot.State.String())
	}
}

func TestBindRulesIsIdempotentAndPreservesState(t *testing.T) {
	session := NewSessionState("bound", nil)
	lock := rulesTestLock(rulesDigestA)
	initial := rulesTestPayload(t, `{"value":1}`)
	if created, err := session.BindRules(lock, initial); err != nil || !created {
		t.Fatalf("first bind created=%v err=%v", created, err)
	}
	session.Rules.Revision = 3

	replacement := rulesTestPayload(t, `{"value":2}`)
	if created, err := session.BindRules(lock, replacement); err != nil || created {
		t.Fatalf("idempotent bind created=%v err=%v", created, err)
	}
	snapshot, ok := session.RulesSnapshot()
	if !ok {
		t.Fatal("missing snapshot")
	}
	if snapshot.Revision != 3 || snapshot.State.String() != initial.String() {
		t.Fatalf("idempotent bind replaced state: revision=%d state=%s", snapshot.Revision, snapshot.State.String())
	}
}

func TestBindRulesRejectsAnotherLock(t *testing.T) {
	session := NewSessionState("bound", nil)
	initial := rulesTestPayload(t, `{"value":1}`)
	first := rulesTestLock(rulesDigestA)
	if _, err := session.BindRules(first, initial); err != nil {
		t.Fatal(err)
	}

	created, err := session.BindRules(rulesTestLock(rulesDigestB), rulesTestPayload(t, `{"value":2}`))
	if created || !errors.Is(err, ErrRulesLockConflict) {
		t.Fatalf("different lock created=%v err=%v", created, err)
	}
	snapshot, ok := session.RulesSnapshot()
	if !ok || snapshot.Ruleset != first || snapshot.State.String() != initial.String() {
		t.Fatalf("conflicting bind changed snapshot: ok=%v snapshot=%+v", ok, snapshot)
	}
}

func TestBindRulesValidatesLockAndState(t *testing.T) {
	session := NewSessionState("validation", nil)
	badLock := rulesTestLock(rulesDigestA)
	badLock.ProtocolVersion = ""
	if created, err := session.BindRules(badLock, rulesTestPayload(t, `{}`)); err == nil || created {
		t.Fatalf("invalid lock created=%v err=%v", created, err)
	}
	if created, err := session.BindRules(rulesTestLock(rulesDigestA), rules.Payload{}); err == nil || created {
		t.Fatalf("missing state created=%v err=%v", created, err)
	}
	if _, ok := session.RulesSnapshot(); ok {
		t.Fatal("invalid bind left a rules block behind")
	}
}

func TestImportStructuredCopiesOnlyCompatibleRulesBlock(t *testing.T) {
	lock := rulesTestLock(rulesDigestA)
	src := NewSessionState("src", nil)
	if _, err := src.BindRules(lock, rulesTestPayload(t, `{"source":true}`)); err != nil {
		t.Fatal(err)
	}
	src.Rules.Revision = 4

	dst := NewSessionState("dst", nil)
	dst.ImportStructured(src)
	snapshot, ok := dst.RulesSnapshot()
	if !ok || snapshot.Ruleset != lock || snapshot.Revision != 4 || snapshot.State.String() != `{"source":true}` {
		t.Fatalf("imported snapshot: ok=%v snapshot=%+v", ok, snapshot)
	}

	other := NewSessionState("other", nil)
	otherLock := rulesTestLock(rulesDigestB)
	if _, err := other.BindRules(otherLock, rulesTestPayload(t, `{"other":true}`)); err != nil {
		t.Fatal(err)
	}
	dst.ImportStructured(other)
	snapshot, ok = dst.RulesSnapshot()
	if !ok || snapshot.Ruleset != lock || snapshot.State.String() != `{"source":true}` {
		t.Fatalf("import silently changed lock: ok=%v snapshot=%+v", ok, snapshot)
	}
}

func TestImportStructuredDoesNotRollBackRulesRevisionOrRewriteEqualRevision(t *testing.T) {
	lock := rulesTestLock(rulesDigestA)
	dst := NewSessionState("dst", nil)
	if _, err := dst.BindRules(lock, rulesTestPayload(t, `{"value":"current"}`)); err != nil {
		t.Fatal(err)
	}
	dst.Rules.Revision = 4

	for _, test := range []struct {
		name     string
		revision uint64
		state    string
	}{
		{"older", 3, `{"value":"old"}`},
		{"same revision different state", 4, `{"value":"fork"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			src := NewSessionState("src", nil)
			if _, err := src.BindRules(lock, rulesTestPayload(t, test.state)); err != nil {
				t.Fatal(err)
			}
			src.Rules.Revision = test.revision
			dst.ImportStructured(src)
			snapshot, ok := dst.RulesSnapshot()
			if !ok || snapshot.Revision != 4 || snapshot.State.String() != `{"value":"current"}` {
				t.Fatalf("rules snapshot rolled back: ok=%v snapshot=%+v", ok, snapshot)
			}
		})
	}

	newer := NewSessionState("newer", nil)
	if _, err := newer.BindRules(lock, rulesTestPayload(t, `{"value":"new"}`)); err != nil {
		t.Fatal(err)
	}
	newer.Rules.Revision = 5
	dst.ImportStructured(newer)
	snapshot, ok := dst.RulesSnapshot()
	if !ok || snapshot.Revision != 5 || snapshot.State.String() != `{"value":"new"}` {
		t.Fatalf("newer rules snapshot was not imported: ok=%v snapshot=%+v", ok, snapshot)
	}
}
