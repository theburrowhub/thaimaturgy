package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
	"github.com/theburrowhub/thaimaturgy/internal/types"
)

// gameTools is the stable LLM-facing gateway. Session, principal, rules lock,
// revision, request ID, continuations, and entropy are supplied by the host and
// deliberately do not appear in these schemas.
var gameTools = []types.Tool{
	{
		Name:        "game_observe",
		Description: "Observe the authorized mechanical view exposed by the loaded game rules package.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
	},
	{
		Name:        "game_list_actions",
		Description: "List the mechanical actions currently provided by the loaded game rules package, including each action's input schema.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
	},
	{
		Name:        "game_get_action_schema",
		Description: "Get the input schema for one action advertised by game_list_actions.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"action_id":{"type":"string"}},
			"required":["action_id"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        "game_submit_intent",
		Description: "Submit a typed mechanical intent to the loaded rules package. The host performs any required random draw and records its result.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"action_id":{"type":"string"},
				"actor_id":{"type":"string"},
				"arguments":{"type":"object"}
			},
			"required":["action_id","arguments"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        "game_respond",
		Description: "Respond to a pending rules decision or adjudication using a resolution ID previously returned by the host.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"resolution_id":{"type":"string"},
				"response":{}
			},
			"required":["resolution_id","response"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        "game_preview",
		Description: "Check an intent without drawing randomness or committing effects. Returns only the first rules step, not every possible future outcome.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"action_id":{"type":"string"},
				"actor_id":{"type":"string"},
				"arguments":{"type":"object"}
			},
			"required":["action_id","arguments"],
			"additionalProperties":false
		}`),
	},
	{
		Name:        "game_explain",
		Description: "Explain a visible rule reference from the loaded rules package.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"reference":{"type":"string"},"locale":{"type":"string"}},
			"required":["reference"],
			"additionalProperties":false
		}`),
	},
}

const maxAutomaticRuleSteps = 8

// Receipts are deliberately bounded until they move into the persisted rules
// transaction log. This protects a long-running router from unbounded request-ID
// growth while still covering normal provider/MCP retry windows.
const maxInMemoryRuleReceipts = 1024

type cachedRuleResult struct {
	fingerprint [sha256.Size]byte
	result      types.ToolResult
}

// rulesGateway adapts the context-aware rules protocol to the repository's
// synchronous ToolProvider interface. The mutex serializes draws and makes
// request-ID receipts idempotent inside a bounded retry window. Persisted
// receipts and an event log belong to the later transactional-host phase.
type rulesGateway struct {
	session *domain.Session
	lock    rules.Lock
	ruleset rules.Ruleset

	resolveDice func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error)

	mu            sync.Mutex
	receipts      map[string]cachedRuleResult
	receiptOrder  []string
	receiptCursor int
}

func newRulesGateway(session *domain.Session) (*rulesGateway, error) {
	if session == nil || session.State == nil {
		return nil, nil
	}

	snapshot, pinned := session.State.RulesSnapshot()
	if !pinned && !usesLegacyDND5E(session.Adventure) {
		return nil, nil
	}

	implementation := dnd5e.New()
	artifact, err := dnd5e.NewArtifact()
	if err != nil {
		return nil, fmt.Errorf("initialize built-in dnd5e artifact: %w", err)
	}
	registry := rules.NewRegistry()
	if err := registry.Register(context.Background(), artifact, implementation); err != nil {
		return nil, fmt.Errorf("register built-in dnd5e ruleset: %w", err)
	}

	if !pinned {
		initialState := dnd5e.InitialState()
		created, err := session.State.BindRules(artifact.Lock(), initialState)
		if err != nil {
			return nil, fmt.Errorf("bind built-in dnd5e ruleset: %w", err)
		}
		if created {
			session.MarkModified()
		}
		snapshot, pinned = session.State.RulesSnapshot()
	}
	if !pinned {
		return nil, fmt.Errorf("load rules snapshot: session has no valid rules binding")
	}
	loaded, err := registry.Lookup(snapshot.Ruleset)
	if err != nil {
		return nil, fmt.Errorf("load pinned ruleset: %w", err)
	}
	if err := loaded.ValidateState(context.Background(), rules.ValidateStateRequest{Snapshot: snapshot}); err != nil {
		return nil, fmt.Errorf("validate pinned rules state: %w", err)
	}

	gateway := &rulesGateway{
		session:  session,
		lock:     snapshot.Ruleset,
		ruleset:  loaded,
		receipts: make(map[string]cachedRuleResult),
	}
	gateway.resolveDice = func(request dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		if request.Count < 1 || request.Count > 100 || request.Sides < 1 || request.Sides > 1000 {
			return dnd5e.DiceRandomResponse{}, fmt.Errorf("invalid dice random request: count=%d sides=%d", request.Count, request.Sides)
		}
		roll := Roll(request.Count, request.Sides, 0)
		return dnd5e.DiceRandomResponse{Rolls: append([]int(nil), roll.Rolls...)}, nil
	}
	return gateway, nil
}

func usesLegacyDND5E(adventure *domain.Adventure) bool {
	if adventure == nil {
		return false
	}
	system := strings.ToLower(strings.TrimSpace(adventure.System))
	switch system {
	case "", "d&d 5e", "dnd 5e", "dnd5e", "dungeons & dragons 5e":
		return true
	default:
		return false
	}
}

func (g *rulesGateway) toolDefinitions() []types.Tool {
	return cloneToolDefinitions(gameTools)
}

func cloneToolDefinitions(source []types.Tool) []types.Tool {
	cloned := make([]types.Tool, len(source))
	for i, definition := range source {
		cloned[i] = definition
		cloned[i].Parameters = append(json.RawMessage(nil), definition.Parameters...)
	}
	return cloned
}

func (g *rulesGateway) principal() rules.Principal {
	role := "assistant"
	if g.session.State.EffectiveMode() == domain.ModeVirtualDM {
		role = "game-master"
	}
	return rules.Principal{ID: "host:oracle", Kind: "llm", Roles: []string{role}}
}

func (g *rulesGateway) snapshot() (rules.Snapshot, error) {
	snapshot, ok := g.session.State.RulesSnapshot()
	if !ok {
		return rules.Snapshot{}, fmt.Errorf("session has no valid rules snapshot")
	}
	if snapshot.Ruleset != g.lock {
		return rules.Snapshot{}, fmt.Errorf("session rules lock changed without migration")
	}
	return snapshot, nil
}

func (g *rulesGateway) execute(call types.ToolCall) types.ToolResult {
	return g.executeCached(call, func() types.ToolResult {
		switch call.Name {
		case "game_observe":
			return g.observe(call)
		case "game_list_actions":
			return g.listActions(call)
		case "game_get_action_schema":
			return g.getActionSchema(call)
		case "game_submit_intent":
			return g.submitIntent(call, true)
		case "game_preview":
			return g.preview(call)
		case "game_explain":
			return g.explain(call)
		case "game_respond":
			return g.respond(call)
		default:
			return errResult(call.ID, "unknown game tool: "+call.Name)
		}
	})
}

func (g *rulesGateway) executeCached(call types.ToolCall, execute func() types.ToolResult) types.ToolResult {
	fingerprint := sha256.Sum256(append(append([]byte(call.Name), 0), call.Arguments...))
	g.mu.Lock()
	defer g.mu.Unlock()
	if call.ID != "" {
		if cached, ok := g.receipts[call.ID]; ok {
			if cached.fingerprint != fingerprint {
				return errResult(call.ID, "rules request ID was already used with different tool arguments")
			}
			return cached.result
		}
	}
	result := execute()
	if call.ID != "" {
		if len(g.receiptOrder) < maxInMemoryRuleReceipts {
			g.receiptOrder = append(g.receiptOrder, call.ID)
		} else {
			delete(g.receipts, g.receiptOrder[g.receiptCursor])
			g.receiptOrder[g.receiptCursor] = call.ID
			g.receiptCursor = (g.receiptCursor + 1) % maxInMemoryRuleReceipts
		}
		g.receipts[call.ID] = cachedRuleResult{fingerprint: fingerprint, result: result}
	}
	return result
}

func (g *rulesGateway) respond(call types.ToolCall) types.ToolResult {
	if call.ID == "" {
		return errResult(call.ID, "game_respond requires a host request ID")
	}
	var arguments struct {
		ResolutionID string          `json:"resolution_id"`
		Response     json.RawMessage `json:"response"`
	}
	if err := decodeToolArguments(call.Arguments, &arguments); err != nil {
		return errResult(call.ID, err.Error())
	}
	if arguments.ResolutionID == "" {
		return errResult(call.ID, "missing 'resolution_id'")
	}
	if len(bytes.TrimSpace(arguments.Response)) == 0 {
		return errResult(call.ID, "missing 'response'")
	}
	return errResult(call.ID, "no externally pending resolution exists in the built-in dnd5e compatibility ruleset")
}

func (g *rulesGateway) observe(call types.ToolCall) types.ToolResult {
	if err := decodeToolArguments(call.Arguments, &struct{}{}); err != nil {
		return errResult(call.ID, err.Error())
	}
	snapshot, err := g.snapshot()
	if err != nil {
		return errResult(call.ID, err.Error())
	}
	projection, err := g.ruleset.Project(context.Background(), rules.ProjectRequest{
		Snapshot: snapshot, Principal: g.principal(),
	})
	if err != nil {
		return errResult(call.ID, "project rules state: "+err.Error())
	}
	if err := projection.Validate(); err != nil {
		return errResult(call.ID, "invalid rules projection: "+err.Error())
	}
	return gameJSONResult(call.ID, gameEnvelope{
		Status:   "resolved",
		Ruleset:  snapshot.Ruleset,
		Revision: snapshot.Revision,
		Data:     json.RawMessage(projection.View.Bytes()),
	})
}

func (g *rulesGateway) actions() (rules.Snapshot, []rules.ActionDescriptor, error) {
	snapshot, err := g.snapshot()
	if err != nil {
		return rules.Snapshot{}, nil, err
	}
	actions, err := g.ruleset.ListActions(context.Background(), rules.CatalogRequest{
		Snapshot: snapshot, Principal: g.principal(),
	})
	if err != nil {
		return rules.Snapshot{}, nil, err
	}
	if err := rules.ValidateActions(actions); err != nil {
		return rules.Snapshot{}, nil, fmt.Errorf("invalid rules action catalog: %w", err)
	}
	return snapshot, actions, nil
}

func (g *rulesGateway) listActions(call types.ToolCall) types.ToolResult {
	if err := decodeToolArguments(call.Arguments, &struct{}{}); err != nil {
		return errResult(call.ID, err.Error())
	}
	snapshot, actions, err := g.actions()
	if err != nil {
		return errResult(call.ID, "list rules actions: "+err.Error())
	}
	return gameJSONResult(call.ID, gameEnvelope{
		Status:   "resolved",
		Ruleset:  snapshot.Ruleset,
		Revision: snapshot.Revision,
		Data:     map[string]any{"actions": actions},
	})
}

func (g *rulesGateway) getActionSchema(call types.ToolCall) types.ToolResult {
	var arguments struct {
		ActionID string `json:"action_id"`
	}
	if err := decodeToolArguments(call.Arguments, &arguments); err != nil {
		return errResult(call.ID, err.Error())
	}
	if arguments.ActionID == "" {
		return errResult(call.ID, "missing 'action_id'")
	}
	snapshot, actions, err := g.actions()
	if err != nil {
		return errResult(call.ID, "list rules actions: "+err.Error())
	}
	for _, action := range actions {
		if action.ID == arguments.ActionID {
			return gameJSONResult(call.ID, gameEnvelope{
				Status:   "resolved",
				Ruleset:  snapshot.Ruleset,
				Revision: snapshot.Revision,
				Data:     action,
			})
		}
	}
	return errResult(call.ID, "rules action is not available: "+arguments.ActionID)
}

type intentToolArguments struct {
	ActionID  string          `json:"action_id"`
	ActorID   string          `json:"actor_id,omitempty"`
	Arguments json.RawMessage `json:"arguments"`
}

func (g *rulesGateway) submitIntent(call types.ToolCall, commit bool) types.ToolResult {
	if commit && call.ID == "" {
		return errResult(call.ID, "game_submit_intent requires a host request ID")
	}
	var arguments intentToolArguments
	if err := decodeToolArguments(call.Arguments, &arguments); err != nil {
		return errResult(call.ID, err.Error())
	}
	if arguments.ActionID == "" {
		return errResult(call.ID, "missing 'action_id'")
	}
	payload, err := rules.NewPayload(arguments.Arguments)
	if err != nil {
		return errResult(call.ID, "invalid 'arguments': "+err.Error())
	}
	resolutionID := intentID(call, arguments.ActionID, payload)
	resolved, err := g.resolve(rules.Intent{
		ID:        resolutionID,
		ActionID:  arguments.ActionID,
		ActorID:   arguments.ActorID,
		Arguments: payload,
	})
	if err != nil {
		return errResult(call.ID, err.Error())
	}
	if resolved.rejection != nil {
		return gameJSONResult(call.ID, gameEnvelope{
			Status:       "rejected",
			Ruleset:      resolved.snapshot.Ruleset,
			Revision:     resolved.snapshot.Revision,
			ResolutionID: resolutionID,
			Data:         resolved.rejection,
		})
	}
	if commit {
		if err := g.commitLegacyResult(resolved.completion.Result); err != nil {
			return errResult(call.ID, "commit rules result: "+err.Error())
		}
	}
	return gameJSONResult(call.ID, gameEnvelope{
		Status:       "resolved",
		Ruleset:      resolved.snapshot.Ruleset,
		Revision:     resolved.snapshot.Revision,
		ResolutionID: resolutionID,
		Outcome:      resolved.completion.Outcome,
		Data:         json.RawMessage(resolved.completion.Result.Bytes()),
	})
}

func (g *rulesGateway) preview(call types.ToolCall) types.ToolResult {
	var arguments intentToolArguments
	if err := decodeToolArguments(call.Arguments, &arguments); err != nil {
		return errResult(call.ID, err.Error())
	}
	if arguments.ActionID == "" {
		return errResult(call.ID, "missing 'action_id'")
	}
	payload, err := rules.NewPayload(arguments.Arguments)
	if err != nil {
		return errResult(call.ID, "invalid 'arguments': "+err.Error())
	}
	snapshot, err := g.snapshot()
	if err != nil {
		return errResult(call.ID, err.Error())
	}
	resolutionID := intentID(call, arguments.ActionID, payload)
	step, err := g.ruleset.Start(context.Background(), rules.StartRequest{
		Snapshot:  snapshot,
		Principal: g.principal(),
		Intent: rules.Intent{
			ID: resolutionID, ActionID: arguments.ActionID,
			ActorID: arguments.ActorID, Arguments: payload,
		},
	})
	if err != nil {
		return errResult(call.ID, "preview rules intent: "+err.Error())
	}
	if err := step.Validate(); err != nil {
		return errResult(call.ID, "ruleset returned an invalid preview step: "+err.Error())
	}
	status := "resolved"
	var data any
	switch step.Kind {
	case rules.StepKindReject:
		status, data = "rejected", step.Reject
	case rules.StepKindComplete:
		status, data = "resolved", step.Complete
	case rules.StepKindNeedRandom:
		data = map[string]any{"next_step": step.Kind, "request": step.NeedRandom}
	case rules.StepKindNeedDecision:
		data = map[string]any{"next_step": step.Kind, "request": step.NeedDecision}
	case rules.StepKindNeedAdjudication:
		data = map[string]any{"next_step": step.Kind, "request": step.NeedAdjudication}
	case rules.StepKindStartChild:
		data = map[string]any{"next_step": step.Kind, "request": step.StartChild}
	case rules.StepKindEmit:
		data = map[string]any{"next_step": step.Kind, "request": step.Emit}
	}
	return gameJSONResult(call.ID, gameEnvelope{
		Status:       status,
		Ruleset:      snapshot.Ruleset,
		Revision:     snapshot.Revision,
		ResolutionID: resolutionID,
		Data:         data,
	})
}

func (g *rulesGateway) explain(call types.ToolCall) types.ToolResult {
	var arguments struct {
		Reference string `json:"reference"`
		Locale    string `json:"locale,omitempty"`
	}
	if err := decodeToolArguments(call.Arguments, &arguments); err != nil {
		return errResult(call.ID, err.Error())
	}
	if arguments.Reference == "" {
		return errResult(call.ID, "missing 'reference'")
	}
	if arguments.Locale == "" {
		if g.session.Config != nil {
			arguments.Locale = string(g.session.Config.Language)
		}
		if arguments.Locale == "" {
			arguments.Locale = "en"
		}
	}
	snapshot, err := g.snapshot()
	if err != nil {
		return errResult(call.ID, err.Error())
	}
	explanation, err := g.ruleset.Explain(context.Background(), rules.ExplainRequest{
		Snapshot: snapshot, Principal: g.principal(),
		Reference: arguments.Reference, Locale: arguments.Locale,
	})
	if err != nil {
		return errResult(call.ID, "explain rule: "+err.Error())
	}
	if err := explanation.Validate(); err != nil {
		return errResult(call.ID, "invalid rules explanation: "+err.Error())
	}
	return gameJSONResult(call.ID, gameEnvelope{
		Status:   "resolved",
		Ruleset:  snapshot.Ruleset,
		Revision: snapshot.Revision,
		Data:     explanation,
	})
}

type resolvedIntent struct {
	snapshot   rules.Snapshot
	completion *rules.Completion
	rejection  *rules.Rejection
}

func (g *rulesGateway) resolve(intent rules.Intent) (resolvedIntent, error) {
	snapshot, err := g.snapshot()
	if err != nil {
		return resolvedIntent{}, err
	}
	step, err := g.ruleset.Start(context.Background(), rules.StartRequest{
		Snapshot: snapshot, Principal: g.principal(), Intent: intent,
	})
	if err != nil {
		return resolvedIntent{}, fmt.Errorf("start rules intent: %w", err)
	}
	for iteration := 0; iteration < maxAutomaticRuleSteps; iteration++ {
		if err := step.Validate(); err != nil {
			return resolvedIntent{}, fmt.Errorf("ruleset returned an invalid step: %w", err)
		}
		switch step.Kind {
		case rules.StepKindReject:
			return resolvedIntent{snapshot: snapshot, rejection: step.Reject}, nil
		case rules.StepKindComplete:
			return resolvedIntent{snapshot: snapshot, completion: step.Complete}, nil
		case rules.StepKindNeedRandom:
			if step.NeedRandom.Method != dnd5e.RandomMethodDiceRoll {
				return resolvedIntent{}, fmt.Errorf("unsupported random method %q", step.NeedRandom.Method)
			}
			var specification dnd5e.DiceRandomRequest
			if err := jsonstrict.Decode(step.NeedRandom.Specification.Bytes(), &specification); err != nil {
				return resolvedIntent{}, fmt.Errorf("decode random specification: %w", err)
			}
			response, err := g.resolveDice(specification)
			if err != nil {
				return resolvedIntent{}, fmt.Errorf("perform random draw: %w", err)
			}
			responsePayload, err := rules.PayloadFrom(response)
			if err != nil {
				return resolvedIntent{}, fmt.Errorf("encode random response: %w", err)
			}
			pending, err := step.Pending()
			if err != nil {
				return resolvedIntent{}, fmt.Errorf("persist rules continuation: %w", err)
			}
			step, err = g.ruleset.Resume(context.Background(), rules.ResumeRequest{
				Snapshot: snapshot, Principal: g.principal(), Pending: pending,
				Response: rules.HostResponse{StepID: pending.StepID, Kind: pending.Kind, Data: responsePayload},
			})
			if err != nil {
				return resolvedIntent{}, fmt.Errorf("resume rules intent: %w", err)
			}
		default:
			return resolvedIntent{}, fmt.Errorf("automatic host does not yet support rules step %q", step.Kind)
		}
	}
	return resolvedIntent{}, fmt.Errorf("rules resolution exceeded %d automatic steps", maxAutomaticRuleSteps)
}

type legacyCompletion struct {
	Legacy dnd5e.LegacyResult `json:"legacy"`
}

func (g *rulesGateway) commitLegacyResult(payload rules.Payload) error {
	var result legacyCompletion
	if err := json.Unmarshal(payload.Bytes(), &result); err != nil {
		return err
	}
	if result.Legacy.LogType != string(domain.LogRoll) {
		return fmt.Errorf("unsupported legacy log type %q", result.Legacy.LogType)
	}
	if result.Legacy.Content == "" || result.Legacy.LogMessage == "" {
		return fmt.Errorf("rules result omitted its legacy projection")
	}
	g.session.State.AppendLog(domain.LogEntry{Type: domain.LogRoll, Message: result.Legacy.LogMessage})
	g.session.MarkModified()
	return nil
}

func (g *rulesGateway) legacyRollDice(call types.ToolCall, arguments map[string]any) types.ToolResult {
	notation, ok := arguments["notation"].(string)
	if !ok {
		return errResult(call.ID, "missing 'notation'")
	}
	reason, _ := arguments["reason"].(string)
	payload, err := rules.PayloadFrom(map[string]any{"notation": notation, "reason": reason})
	if err != nil {
		return errResult(call.ID, err.Error())
	}
	return g.executeLegacyAction(call, dnd5e.ActionDiceRoll, payload)
}

func (g *rulesGateway) legacyAbilityCheck(call types.ToolCall, arguments map[string]any) types.ToolResult {
	modifier, _ := intArg(arguments, "modifier")
	dc, ok := intArg(arguments, "dc")
	if !ok {
		return errResult(call.ID, "missing 'dc'")
	}
	label, _ := arguments["label"].(string)
	payload, err := rules.PayloadFrom(map[string]any{"modifier": modifier, "dc": dc, "label": label})
	if err != nil {
		return errResult(call.ID, err.Error())
	}
	return g.executeLegacyAction(call, dnd5e.ActionAbilityCheck, payload)
}

func (g *rulesGateway) executeLegacyAction(call types.ToolCall, actionID string, payload rules.Payload) types.ToolResult {
	if call.ID == "" {
		return errResult(call.ID, call.Name+" requires a host request ID")
	}
	return g.executeCached(call, func() types.ToolResult {
		resolved, err := g.resolve(rules.Intent{
			ID: intentID(call, actionID, payload), ActionID: actionID, Arguments: payload,
		})
		if err != nil {
			return errResult(call.ID, err.Error())
		}
		if resolved.rejection != nil {
			return errResult(call.ID, resolved.rejection.Message)
		}
		if err := g.commitLegacyResult(resolved.completion.Result); err != nil {
			return errResult(call.ID, "commit rules result: "+err.Error())
		}
		var result legacyCompletion
		if err := json.Unmarshal(resolved.completion.Result.Bytes(), &result); err != nil {
			return errResult(call.ID, "decode rules result: "+err.Error())
		}
		return okResult(call.ID, result.Legacy.Content)
	})
}

func intentID(call types.ToolCall, actionID string, payload rules.Payload) string {
	if call.ID != "" {
		return call.ID
	}
	digest := sha256.Sum256(append(append([]byte(actionID), 0), payload.Bytes()...))
	return "intent:" + hex.EncodeToString(digest[:12])
}

type gameEnvelope struct {
	Status       string     `json:"status"`
	Ruleset      rules.Lock `json:"ruleset"`
	Revision     uint64     `json:"revision"`
	ResolutionID string     `json:"resolution_id,omitempty"`
	Outcome      string     `json:"outcome,omitempty"`
	Data         any        `json:"data,omitempty"`
}

func gameJSONResult(id string, envelope gameEnvelope) types.ToolResult {
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return errResult(id, "encode game result: "+err.Error())
	}
	return okResult(id, string(encoded))
}

func decodeToolArguments(raw json.RawMessage, target any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := jsonstrict.Decode(raw, target); err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}
	return nil
}
