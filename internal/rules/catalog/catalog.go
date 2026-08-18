// Package catalog resolves content requirements to exact, host-attested rules
// artifacts. It owns range semantics; the rules kernel transports constraints
// without interpreting them.
package catalog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

var ErrNoCompatibleRuleset = errors.New("rules catalog: no compatible ruleset")

// Catalog combines exact registry lookup with deterministic requirement
// resolution. One ID/version release may have only one digest, as enforced by
// rules.Registry.
type Catalog struct {
	registry *rules.Registry
	mu       sync.RWMutex
	byID     map[string][]rules.Lock
	initial  map[rules.Lock]rules.Payload
}

var _ rules.Resolver = (*Catalog)(nil)

func New() *Catalog {
	return &Catalog{
		registry: rules.NewRegistry(),
		byID:     make(map[string][]rules.Lock),
		initial:  make(map[rules.Lock]rules.Payload),
	}
}

// Register adds one complete loadable package. initialState is validated both
// structurally and by the package before the exact artifact becomes visible.
func (c *Catalog) Register(
	ctx context.Context,
	artifact rules.Artifact,
	implementation rules.Ruleset,
	initialState rules.Payload,
) error {
	if c == nil {
		return fmt.Errorf("rules catalog: nil catalog")
	}
	if err := initialState.Validate(); err != nil {
		return fmt.Errorf("rules catalog: invalid initial state: %w", err)
	}
	// Probe the registry contract first so a manifest mismatch or incompatible
	// protocol cannot execute ValidateState under a forged artifact identity.
	probe := rules.NewRegistry()
	if err := probe.Register(ctx, artifact, implementation); err != nil {
		return err
	}
	lock := artifact.Lock()
	snapshot := rules.Snapshot{Ruleset: lock, State: initialState}
	if err := implementation.ValidateState(ctx, rules.ValidateStateRequest{Snapshot: snapshot}); err != nil {
		return fmt.Errorf("rules catalog: ruleset rejected initial state: %w", err)
	}
	// Exact lookup, requirement resolution, and initial state form one catalog
	// entry. Hold the catalog lock while publishing all three so concurrent
	// callers can never observe a partially registered package.
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.registry.Register(ctx, artifact, implementation); err != nil {
		return err
	}
	c.byID[lock.ID] = append(c.byID[lock.ID], lock)
	c.initial[lock] = initialState
	slices.SortFunc(c.byID[lock.ID], func(a, b rules.Lock) int {
		comparison := compareVersions(mustParseVersion(a.Version), mustParseVersion(b.Version))
		if comparison == 0 {
			// Build metadata has no SemVer precedence. Use the complete release
			// string as a stable tie-breaker so resolution is deterministic.
			comparison = strings.Compare(a.Version, b.Version)
		}
		return -comparison
	})
	return nil
}

// InitialState returns the immutable seed state registered for one exact lock.
func (c *Catalog) InitialState(lock rules.Lock) (rules.Payload, error) {
	if c == nil {
		return rules.Payload{}, fmt.Errorf("rules catalog: nil catalog")
	}
	if err := lock.Validate(); err != nil {
		return rules.Payload{}, err
	}
	c.mu.RLock()
	state, exists := c.initial[lock]
	c.mu.RUnlock()
	if !exists {
		return rules.Payload{}, fmt.Errorf("%w: initial state for %s@%s", rules.ErrRulesetNotFound, lock.ID, lock.Version)
	}
	return state, nil
}

// Lookup returns the implementation for one exact persisted lock.
func (c *Catalog) Lookup(lock rules.Lock) (rules.Ruleset, error) {
	if c == nil {
		return nil, fmt.Errorf("rules catalog: nil catalog")
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.registry.Lookup(lock)
}

// Resolve chooses the highest registered version satisfying requirement and
// returns both its exact lock and implementation. Resolution never substitutes
// another package ID and never ignores a malformed constraint.
func (c *Catalog) Resolve(requirement rules.Requirement) (rules.Lock, rules.Ruleset, error) {
	if c == nil {
		return rules.Lock{}, nil, fmt.Errorf("rules catalog: nil catalog")
	}
	if err := requirement.Validate(); err != nil {
		return rules.Lock{}, nil, err
	}
	constraint, err := parseConstraint(string(requirement.Version))
	if err != nil {
		return rules.Lock{}, nil, fmt.Errorf("rules catalog: invalid constraint %q: %w", requirement.Version, err)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	locks := c.byID[requirement.ID]
	for _, lock := range locks {
		if constraint.matches(mustParseVersion(lock.Version)) {
			implementation, err := c.registry.Lookup(lock)
			return lock, implementation, err
		}
	}
	return rules.Lock{}, nil, fmt.Errorf("%w: %s@%s", ErrNoCompatibleRuleset, requirement.ID, requirement.Version)
}

func (c *Catalog) Locks(id string) []rules.Lock {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]rules.Lock(nil), c.byID[id]...)
}

type version struct {
	raw                 string
	major, minor, patch string
	pre                 []string
}

func parseVersion(value string) (version, error) {
	core := value
	if plus := strings.IndexByte(core, '+'); plus >= 0 {
		if err := validateIdentifiers(core[plus+1:], true); err != nil {
			return version{}, fmt.Errorf("invalid build metadata: %w", err)
		}
		core = core[:plus]
	}
	var pre []string
	if dash := strings.IndexByte(core, '-'); dash >= 0 {
		prerelease := core[dash+1:]
		if err := validateIdentifiers(prerelease, false); err != nil {
			return version{}, fmt.Errorf("invalid prerelease: %w", err)
		}
		pre = strings.Split(prerelease, ".")
		core = core[:dash]
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version{}, fmt.Errorf("version must have major.minor.patch")
	}
	for _, part := range parts {
		if !isDecimal(part) || (len(part) > 1 && part[0] == '0') {
			return version{}, fmt.Errorf("invalid numeric component %q", part)
		}
	}
	return version{raw: value, major: parts[0], minor: parts[1], patch: parts[2], pre: pre}, nil
}

func mustParseVersion(value string) version {
	parsed, err := parseVersion(value)
	if err != nil {
		panic("rules catalog received kernel-validated non-semver: " + err.Error())
	}
	return parsed
}

func compareVersions(left, right version) int {
	for _, pair := range [][2]string{{left.major, right.major}, {left.minor, right.minor}, {left.patch, right.patch}} {
		if comparison := compareDecimal(pair[0], pair[1]); comparison != 0 {
			return comparison
		}
	}
	if len(left.pre) == 0 && len(right.pre) == 0 {
		return 0
	}
	if len(left.pre) == 0 {
		return 1
	}
	if len(right.pre) == 0 {
		return -1
	}
	for index := 0; index < len(left.pre) && index < len(right.pre); index++ {
		if comparison := comparePrereleaseIdentifier(left.pre[index], right.pre[index]); comparison != 0 {
			return comparison
		}
	}
	if len(left.pre) < len(right.pre) {
		return -1
	}
	if len(left.pre) > len(right.pre) {
		return 1
	}
	return 0
}

func comparePrereleaseIdentifier(left, right string) int {
	leftNumber := isDecimal(left)
	rightNumber := isDecimal(right)
	switch {
	case leftNumber && rightNumber:
		return compareDecimal(left, right)
	case leftNumber:
		return -1
	case rightNumber:
		return 1
	default:
		return strings.Compare(left, right)
	}
}

func validateIdentifiers(value string, build bool) error {
	if value == "" {
		return fmt.Errorf("identifier list is empty")
	}
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" {
			return fmt.Errorf("identifier is empty")
		}
		for index := 0; index < len(identifier); index++ {
			character := identifier[index]
			if !(character >= '0' && character <= '9') &&
				!(character >= 'A' && character <= 'Z') &&
				!(character >= 'a' && character <= 'z') && character != '-' {
				return fmt.Errorf("identifier %q contains an unsupported character", identifier)
			}
		}
		if !build && isDecimal(identifier) && len(identifier) > 1 && identifier[0] == '0' {
			return fmt.Errorf("numeric identifier %q has a leading zero", identifier)
		}
	}
	return nil
}

func isDecimal(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func compareDecimal(left, right string) int {
	if len(left) != len(right) {
		if len(left) < len(right) {
			return -1
		}
		return 1
	}
	return strings.Compare(left, right)
}

func incrementDecimal(value string) string {
	result := []byte(value)
	for index := len(result) - 1; index >= 0; index-- {
		if result[index] < '9' {
			result[index]++
			return string(result)
		}
		result[index] = '0'
	}
	return "1" + string(result)
}

type comparator struct {
	op       string
	version  version
	exactRaw string
}

type constraint []comparator

func parseConstraint(expression string) (constraint, error) {
	expression = strings.TrimSpace(expression)
	if expression == "*" {
		return constraint{}, nil
	}
	if strings.Contains(expression, "||") {
		return nil, fmt.Errorf("OR constraints are not supported")
	}
	if strings.HasPrefix(expression, "^") || strings.HasPrefix(expression, "~") {
		base, err := parseVersion(expression[1:])
		if err != nil {
			return nil, err
		}
		upper := base
		if expression[0] == '~' {
			upper.minor = incrementDecimal(upper.minor)
			upper.patch = "0"
			upper.pre = nil
			upper.raw = ""
		} else {
			switch {
			case base.major != "0":
				upper.major = incrementDecimal(upper.major)
				upper.minor, upper.patch = "0", "0"
			case base.minor != "0":
				upper.minor = incrementDecimal(upper.minor)
				upper.patch = "0"
			default:
				upper.patch = incrementDecimal(upper.patch)
			}
			upper.pre = nil
			upper.raw = ""
		}
		return constraint{{op: ">=", version: base}, {op: "<", version: upper}}, nil
	}

	tokens := strings.Fields(strings.ReplaceAll(expression, ",", " "))
	if len(tokens) == 0 {
		return nil, fmt.Errorf("constraint is empty")
	}
	result := make(constraint, 0, len(tokens))
	for _, token := range tokens {
		op := "="
		value := token
		for _, candidate := range []string{">=", "<=", ">", "<", "="} {
			if strings.HasPrefix(token, candidate) {
				op, value = candidate, strings.TrimPrefix(token, candidate)
				break
			}
		}
		parsed, err := parseVersion(value)
		if err != nil {
			return nil, err
		}
		comparison := comparator{op: op, version: parsed}
		if op == "=" {
			comparison.exactRaw = value
		}
		result = append(result, comparison)
	}
	return result, nil
}

func (c constraint) matches(candidate version) bool {
	for _, comparison := range c {
		value := compareVersions(candidate, comparison.version)
		switch comparison.op {
		case "=":
			if value != 0 || comparison.exactRaw != "" && candidate.raw != comparison.exactRaw {
				return false
			}
		case ">":
			if value <= 0 {
				return false
			}
		case ">=":
			if value < 0 {
				return false
			}
		case "<":
			if value >= 0 {
				return false
			}
		case "<=":
			if value > 0 {
				return false
			}
		}
	}
	return true
}
