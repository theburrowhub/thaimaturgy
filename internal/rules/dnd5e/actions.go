package dnd5e

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/theburrowhub/thaimaturgy/internal/diceexpr"
	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

const (
	ActionDiceRoll     = "dice.roll"
	ActionAbilityCheck = "ability.check"

	RandomMethodDiceRoll = "dice.roll"
)

// DiceRandomRequest is the random specification emitted by both supported
// actions. Each requested value is uniform in the inclusive range [1, Sides].
type DiceRandomRequest struct {
	Count int `json:"count"`
	Sides int `json:"sides"`
}

// DiceRandomResponse is the host response consumed by Resume.
type DiceRandomResponse struct {
	Rolls []int `json:"rolls"`
}

type continuation struct {
	SchemaVersion uint32 `json:"schema_version"`
	Action        string `json:"action"`
	Notation      string `json:"notation,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Modifier      *int   `json:"modifier,omitempty"`
	DC            *int   `json:"dc,omitempty"`
	Label         string `json:"label,omitempty"`
}

type diceRollArguments struct {
	Notation string `json:"notation"`
	Reason   string `json:"reason,omitempty"`
}

type abilityCheckArguments struct {
	Modifier *int   `json:"modifier"`
	DC       *int   `json:"dc"`
	Label    string `json:"label,omitempty"`
}

func decodeContinuation(payload core.Payload) (continuation, error) {
	var state continuation
	if err := decodePayload(payload, &state); err != nil {
		return continuation{}, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload.Bytes(), &fields); err != nil {
		return continuation{}, err
	}
	allowed := map[string]bool{"schema_version": true, "action": true}
	switch state.Action {
	case ActionDiceRoll:
		allowed["notation"] = true
		allowed["reason"] = true
	case ActionAbilityCheck:
		allowed["modifier"] = true
		allowed["dc"] = true
		allowed["label"] = true
		for _, required := range []string{"modifier", "dc"} {
			if _, present := fields[required]; !present {
				return continuation{}, fmt.Errorf("continuation for %q is missing field %q", state.Action, required)
			}
		}
	}
	for field := range fields {
		if !allowed[field] {
			return continuation{}, fmt.Errorf("continuation field %q is not valid for action %q", field, state.Action)
		}
	}
	if err := state.validate(); err != nil {
		return continuation{}, err
	}
	return state, nil
}

func (c continuation) validate() error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("continuation schema version %d is unsupported", c.SchemaVersion)
	}
	switch c.Action {
	case ActionDiceRoll:
		expression, err := diceexpr.Parse(c.Notation)
		if err != nil {
			return fmt.Errorf("invalid dice continuation: %w", err)
		}
		if expression.Notation != c.Notation {
			return fmt.Errorf("dice continuation notation is not normalized")
		}
	case ActionAbilityCheck:
		if c.Modifier == nil {
			return fmt.Errorf("ability-check continuation is missing modifier")
		}
		if c.DC == nil {
			return fmt.Errorf("ability-check continuation is missing dc")
		}
	default:
		return fmt.Errorf("continuation contains unknown action %q", c.Action)
	}
	return nil
}

func actionDescriptors() ([]core.ActionDescriptor, error) {
	diceSchema, err := core.NewPayload([]byte(fmt.Sprintf(`{
		"type":"object",
		"properties":{"notation":{"type":"string"},"reason":{"type":"string","maxLength":%d,"description":"UTF-8 text; the rules protocol accepts at most this many bytes"}},
		"required":["notation"],
		"additionalProperties":false
	}`, core.MaxTextBytes)))
	if err != nil {
		return nil, err
	}
	checkSchema, err := core.NewPayload([]byte(fmt.Sprintf(`{
		"type":"object",
		"properties":{
			"modifier":{"type":"integer"},
			"dc":{"type":"integer"},
			"label":{"type":"string","maxLength":%d,"description":"What the check is for; UTF-8 text with the same byte limit"}
		},
		"required":["modifier","dc"],
		"additionalProperties":false
	}`, core.MaxTextBytes)))
	if err != nil {
		return nil, err
	}
	return []core.ActionDescriptor{
		{
			ID:          ActionDiceRoll,
			Label:       "Roll dice",
			Description: "Roll dice in standard notation (e.g. '1d20', '2d6+3', '8d6'). Use for the DM's quick rolls.",
			InputSchema: diceSchema,
			Tags:        []string{"random"},
		},
		{
			ID:          ActionAbilityCheck,
			Label:       "Ability check",
			Description: "Roll a d20 + modifier against a DC and report success/failure.",
			InputSchema: checkSchema,
			Tags:        []string{"random"},
		},
	}, nil
}

func (r *Ruleset) startDiceRoll(request core.StartRequest) (core.Step, error) {
	var args diceRollArguments
	err := decodePayload(request.Intent.Arguments, &args)
	if err != nil {
		return rejection(request.Intent.ID, "invalid.arguments", "failed to parse arguments: "+err.Error()), nil
	}
	if args.Notation == "" {
		return rejection(request.Intent.ID, "invalid.arguments", "missing 'notation'"), nil
	}
	if len(args.Reason) > core.MaxTextBytes {
		return rejection(request.Intent.ID, "invalid.arguments", fmt.Sprintf("reason exceeds %d bytes", core.MaxTextBytes)), nil
	}
	expression, err := diceexpr.Parse(args.Notation)
	if err != nil {
		return rejection(request.Intent.ID, "invalid.notation", err.Error()), nil
	}
	return randomStep(request.Intent.ID, continuation{
		SchemaVersion: 1,
		Action:        ActionDiceRoll,
		Notation:      expression.Notation,
		Reason:        args.Reason,
	}, *expression)
}

func (r *Ruleset) startAbilityCheck(request core.StartRequest) (core.Step, error) {
	var args abilityCheckArguments
	err := decodePayload(request.Intent.Arguments, &args)
	if err != nil {
		return rejection(request.Intent.ID, "invalid.arguments", "failed to parse arguments: "+err.Error()), nil
	}
	if args.Modifier == nil {
		return rejection(request.Intent.ID, "invalid.arguments", "missing 'modifier'"), nil
	}
	if args.DC == nil {
		return rejection(request.Intent.ID, "invalid.arguments", "missing 'dc'"), nil
	}
	if len(args.Label) > core.MaxTextBytes {
		return rejection(request.Intent.ID, "invalid.arguments", fmt.Sprintf("label exceeds %d bytes", core.MaxTextBytes)), nil
	}
	expression, err := diceexpr.Parse("1d20")
	if err != nil {
		return core.Step{}, err
	}
	expression.Modifier = *args.Modifier
	return randomStep(request.Intent.ID, continuation{
		SchemaVersion: 1,
		Action:        ActionAbilityCheck,
		Modifier:      intPointer(*args.Modifier),
		DC:            intPointer(*args.DC),
		Label:         args.Label,
	}, *expression)
}

func intPointer(value int) *int { return &value }

func randomStep(id string, state continuation, expression diceexpr.Expression) (core.Step, error) {
	continuationPayload, err := payloadFrom(state)
	if err != nil {
		return core.Step{}, err
	}
	specification, err := payloadFrom(DiceRandomRequest{Count: expression.NumDice, Sides: expression.DiceSides})
	if err != nil {
		return core.Step{}, err
	}
	return core.Step{
		ID:           id,
		Kind:         core.StepKindNeedRandom,
		Continuation: continuationPayload,
		NeedRandom: &core.RandomRequest{
			Method:        RandomMethodDiceRoll,
			Specification: specification,
		},
	}, nil
}

func rejection(id, code, message string) core.Step {
	return core.Step{
		ID:     id,
		Kind:   core.StepKindReject,
		Reject: &core.Rejection{Code: code, Message: sanitizeRejectionMessage(message)},
	}
}

func sanitizeRejectionMessage(message string) string {
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
	message = strings.TrimSpace(message[:limit])
	if message == "" {
		return "rules intent was rejected"
	}
	return message + suffix
}
