package ruleskit

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

var emptyState = MustPayload(struct{}{})

// RandomMethodDiceRoll is the shared host method used by every built-in
// reference package. See DiceSpecification and DiceResponse for its wire form.
const RandomMethodDiceRoll = "dice.roll"

// InitialState returns the canonical immutable state of a stateless reference
// ruleset.
func InitialState() core.Payload { return emptyState }

// Decode applies the strict JSON protocol decoder to a payload.
func Decode(payload core.Payload, destination any) error {
	return jsonstrict.Decode(payload.Bytes(), destination)
}

// DecodeRequired strictly decodes an object and also rejects omitted fields.
// Go value fields alone cannot distinguish an omitted zero from an explicit
// zero, which matters for tamper-resistant persisted continuations.
func DecodeRequired(payload core.Payload, destination any, required ...string) error {
	if err := Decode(payload, destination); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload.Bytes(), &fields); err != nil {
		return err
	}
	if fields == nil {
		return fmt.Errorf("expected a JSON object")
	}
	for _, field := range required {
		if _, present := fields[field]; !present {
			return fmt.Errorf("missing required field %q", field)
		}
	}
	return nil
}

// Payload marshals a value into an immutable protocol payload.
func Payload(value any) (core.Payload, error) { return core.PayloadFrom(value) }

// MustPayload marshals a static built-in value or panics on programmer error.
func MustPayload(value any) core.Payload {
	payload, err := Payload(value)
	if err != nil {
		panic(fmt.Sprintf("ruleskit: build payload: %v", err))
	}
	return payload
}

// NewArtifact builds a stable built-in package artifact.
func NewArtifact(id, name, description, version, material string, capabilities []string) (core.Artifact, error) {
	manifest := core.Manifest{
		ID:              id,
		Name:            name,
		Description:     description,
		Version:         version,
		ProtocolVersion: core.ProtocolVersion,
		Runtime:         core.Runtime{Kind: core.RuntimeBuiltin},
		Capabilities:    append([]string(nil), capabilities...),
	}
	return core.NewArtifact(manifest, strings.NewReader(material))
}

// DiceSpecification is the shared auditable random contract. Each returned
// face is uniform in the inclusive range [1, Sides].
type DiceSpecification struct {
	Count int `json:"count"`
	Sides int `json:"sides"`
}

// DiceResponse is the host response for DiceSpecification.
type DiceResponse struct {
	Rolls []int `json:"rolls"`
}

// RandomStep creates a persistible request for uniformly distributed dice.
func RandomStep(id string, continuation any, count, sides int) (core.Step, error) {
	if count < 1 || count > core.MaxCollectionItems {
		return core.Step{}, fmt.Errorf("ruleskit: dice count %d is outside [1,%d]", count, core.MaxCollectionItems)
	}
	if sides < 2 || sides > 1_000_000 {
		return core.Step{}, fmt.Errorf("ruleskit: die sides %d is outside [2,1000000]", sides)
	}
	state, err := Payload(continuation)
	if err != nil {
		return core.Step{}, err
	}
	specification, err := Payload(DiceSpecification{Count: count, Sides: sides})
	if err != nil {
		return core.Step{}, err
	}
	return core.Step{
		ID:           id,
		Kind:         core.StepKindNeedRandom,
		Continuation: state,
		NeedRandom: &core.RandomRequest{
			Method:        RandomMethodDiceRoll,
			Specification: specification,
		},
	}, nil
}

// DecodeRolls strictly decodes and validates the cardinality and inclusive
// face range promised by a prior DiceSpecification.
func DecodeRolls(payload core.Payload, count, sides int) ([]int, error) {
	var response DiceResponse
	if err := Decode(payload, &response); err != nil {
		return nil, fmt.Errorf("decode random response: %w", err)
	}
	if len(response.Rolls) != count {
		return nil, fmt.Errorf("received %d rolls, want %d", len(response.Rolls), count)
	}
	for i, roll := range response.Rolls {
		if roll < 1 || roll > sides {
			return nil, fmt.Errorf("roll %d is %d, want a value between 1 and %d", i, roll, sides)
		}
	}
	return append([]int(nil), response.Rolls...), nil
}

// Complete creates a terminal structured result.
func Complete(id, outcome string, result any) (core.Step, error) {
	payload, err := Payload(result)
	if err != nil {
		return core.Step{}, err
	}
	return core.Step{
		ID:       id,
		Kind:     core.StepKindComplete,
		Complete: &core.Completion{Outcome: outcome, Result: payload},
	}, nil
}

// Reject creates a terminal expected refusal with a bounded safe message.
func Reject(id, code, message string) core.Step {
	return core.Step{
		ID:     id,
		Kind:   core.StepKindReject,
		Reject: &core.Rejection{Code: code, Message: sanitizeMessage(message)},
	}
}

// RejectArguments maps malformed system input to an expected protocol
// rejection rather than an execution failure.
func RejectArguments(id string, err error) (core.Step, error) {
	return Reject(id, "invalid.arguments", "failed to parse arguments: "+err.Error()), nil
}

// RequireRange returns a stable user-facing validation rejection when value is
// outside its inclusive range.
func RequireRange(id, name string, value, minimum, maximum int) (core.Step, bool) {
	if value >= minimum && value <= maximum {
		return core.Step{}, true
	}
	return Reject(id, "invalid.arguments", fmt.Sprintf("%s must be between %d and %d", name, minimum, maximum)), false
}

func sanitizeMessage(message string) string {
	var sanitized strings.Builder
	for _, r := range message {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			sanitized.WriteRune(unicode.ReplacementChar)
			continue
		}
		sanitized.WriteRune(r)
	}
	message = strings.TrimSpace(sanitized.String())
	if message == "" {
		message = "rules intent was rejected"
	}
	if len(message) <= core.MaxTextBytes {
		return message
	}
	const suffix = "…"
	limit := core.MaxTextBytes - len(suffix)
	for limit > 0 && !utf8.ValidString(message[:limit]) {
		limit--
	}
	return message[:limit] + suffix
}
