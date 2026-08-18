package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
	"github.com/theburrowhub/thaimaturgy/internal/rules/ruleskit"
	"github.com/theburrowhub/thaimaturgy/internal/ruleshost"
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

const (
	// A continuation may legitimately need many bounded automatic exchanges
	// (for example both exploding dice in Savage Worlds). Keep the host budget
	// aligned with the protocol's maximum collection size rather than an
	// arbitrary small number that rejects a valid package result.
	maxAutomaticRuleSteps = rules.MaxCollectionItems
	maxGenericDiceSides   = 1_000_000

	// DefaultRulesRequestTimeoutSeconds preserves the historical rules-call
	// deadline when a host configuration omits or disables its request timeout.
	// MaxRulesRequestTimeoutSeconds prevents a malformed configuration from
	// turning one package call into an effectively unbounded operation. These
	// values are also the contract used by the MCP child process.
	DefaultRulesRequestTimeoutSeconds = 90
	MaxRulesRequestTimeoutSeconds     = 3600
)

// Read-only query receipts are deliberately bounded in memory. Effectful game
// requests use SessionState's durable, non-evicting receipt log instead.
const maxInMemoryRuleReceipts = 1024

type cachedRuleResult struct {
	fingerprint [sha256.Size]byte
	result      types.ToolResult
}

// rulesGateway adapts the context-aware rules protocol to the repository's
// synchronous ToolProvider interface. Mutating calls use SessionState's durable
// transaction API; the small memory cache remains only for read-only queries.
type rulesGateway struct {
	session *domain.Session
	lock    rules.Lock
	ruleset rules.Ruleset
	// legacyDND5E is true only for the exact built-in artifact. A foreign package
	// cannot opt into legacy result parsing merely by returning a `legacy` field.
	legacyDND5E bool

	// dnd5e's request/response structs are the historical Go names for the
	// system-neutral dice.roll wire contract shared by every package.
	resolveDice func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error)
	random      *ruleshost.RandomDispatcher

	// Test-only crash boundary. Returning an error simulates process loss after
	// an automatic response has been durably committed and before Resume.
	afterCheckpoint func() error

	mu            sync.Mutex
	receipts      map[string]cachedRuleResult
	receiptOrder  []string
	receiptCursor int
}

func newRulesGateway(session *domain.Session) (*rulesGateway, error) {
	if session == nil || session.State == nil {
		return nil, nil
	}
	runtime, exists, err := session.State.RulesRuntimeSnapshotStrict()
	if err != nil {
		return nil, fmt.Errorf("validate pinned rules state: %w", err)
	}
	if !exists {
		return nil, nil
	}
	if session.RulesResolver == nil {
		return nil, errors.New("load pinned ruleset: session has no rules resolver")
	}
	loaded, err := session.RulesResolver.Lookup(runtime.Lock)
	if err != nil {
		return nil, fmt.Errorf("load pinned ruleset: %w", err)
	}
	replayBatches := make([]ruleshost.ReplayBatch, len(runtime.EventBatches))
	for i, batch := range runtime.EventBatches {
		replayBatches[i] = ruleshost.ReplayBatch{
			Sequence: batch.Sequence, BaseRevision: batch.BaseRevision,
			Revision: batch.Revision, Events: batch.Events,
		}
	}
	ctx, cancel := newRulesContext(session)
	defer cancel()
	replayed, err := ruleshost.Replay(ctx, loaded, runtime.Lock, runtime.InitialState, replayBatches)
	if err != nil {
		return nil, fmt.Errorf("validate pinned rules replay: %w", err)
	}
	if replayed.Revision != runtime.Revision || replayed.State.String() != runtime.State.String() {
		return nil, errors.New("validate pinned rules replay: materialized state does not match event history")
	}

	gateway := newRulesGatewayWithRuleset(session, runtime.Lock, loaded)
	if err := gateway.recoverAutomaticCheckpoints(ctx); err != nil {
		return nil, fmt.Errorf("recover committed rules checkpoint: %w", err)
	}
	return gateway, nil
}

// newRulesGatewayWithRuleset is the loader seam used by built-ins and dynamic
// package catalogs alike. Artifact selection and verification stay outside the
// transactional host.
func newRulesGatewayWithRuleset(session *domain.Session, lock rules.Lock, loaded rules.Ruleset) *rulesGateway {
	gateway := &rulesGateway{
		session: session, lock: lock, ruleset: loaded,
		receipts: make(map[string]cachedRuleResult), random: ruleshost.NewRandomDispatcher(),
	}
	gateway.legacyDND5E = IsBuiltinDND5ELock(lock)
	gateway.resolveDice = func(request dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		if request.Count < 1 || request.Count > rules.MaxCollectionItems || request.Sides < 1 || request.Sides > maxGenericDiceSides {
			return dnd5e.DiceRandomResponse{}, fmt.Errorf("invalid dice random request: count=%d sides=%d", request.Count, request.Sides)
		}
		roll := Roll(request.Count, request.Sides, 0)
		return dnd5e.DiceRandomResponse{Rolls: append([]int(nil), roll.Rolls...)}, nil
	}
	if err := gateway.random.Register(ruleskit.RandomMethodDiceRoll, gateway.resolveGenericDice); err != nil {
		panic("register generic dice resolver: " + err.Error())
	}
	return gateway
}

// IsBuiltinDND5ELock reports whether lock is the exact host-attested built-in
// D&D 5e artifact. Package IDs alone never enable compatibility utilities.
func IsBuiltinDND5ELock(lock rules.Lock) bool {
	artifact, err := dnd5e.NewArtifact()
	return err == nil && lock == artifact.Lock()
}

// SupportsDNDUtilities validates the live gateway before frontends expose any
// legacy D&D-only command or sheet utility.
func SupportsDNDUtilities(session *domain.Session) bool {
	gateway, err := newRulesGateway(session)
	return err == nil && gateway != nil && gateway.legacyDND5E
}

func newRulesContext(session *domain.Session) (context.Context, context.CancelFunc) {
	timeout := time.Duration(EffectiveRulesRequestTimeoutSeconds(session)) * time.Second
	return context.WithTimeout(context.Background(), timeout)
}

// EffectiveRulesRequestTimeoutSeconds returns the bounded deadline actually
// used by the rules host. Keeping this calculation in one place lets an MCP
// subprocess inherit exactly the same value instead of reapplying defaults.
func EffectiveRulesRequestTimeoutSeconds(session *domain.Session) int {
	seconds := DefaultRulesRequestTimeoutSeconds
	if session != nil && session.Config != nil && session.Config.RequestTimeoutSeconds > 0 {
		seconds = session.Config.RequestTimeoutSeconds
	}
	if seconds > MaxRulesRequestTimeoutSeconds {
		return MaxRulesRequestTimeoutSeconds
	}
	return seconds
}

func (g *rulesGateway) recoverAutomaticCheckpoints(ctx context.Context) error {
	if g == nil || g.session == nil || g.session.State == nil {
		return nil
	}
	release := g.session.State.LockRulesHost()
	defer release()
	for recovered := 0; recovered <= domain.MaxRulesPending; recovered++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		runtime, exists, err := g.session.State.RulesRuntimeSnapshotStrict()
		if err != nil {
			return err
		}
		if !exists || runtime.Lock != g.lock {
			return errors.New("rules binding changed during checkpoint recovery")
		}
		var checkpoint *domain.RulesPendingResolution
		var receipt *domain.RulesReceipt
		for i := range runtime.Pending {
			candidate := &runtime.Pending[i]
			if candidate.Response == nil {
				continue
			}
			for j := range runtime.Receipts {
				accepted := &runtime.Receipts[j]
				if accepted.RequestID == candidate.RequestID && accepted.ResolutionID == candidate.ResolutionID && accepted.Result == nil {
					checkpoint, receipt = candidate, accepted
					break
				}
			}
			if checkpoint != nil {
				break
			}
		}
		if checkpoint == nil {
			return nil
		}
		call := types.ToolCall{ID: receipt.RequestID, Name: receipt.Tool}
		legacy := g.legacyDND5E && (receipt.Tool == "roll_dice" || receipt.Tool == "ability_check")
		result := g.driveIntentWithBegin(
			ctx, call, nil, checkpoint.ResolutionID, legacy, true, g.beginRulesRecovery,
		)
		if result.Error != "" {
			return errors.New(result.Error)
		}
	}
	return fmt.Errorf("automatic checkpoint recovery exceeded %d pending resolutions", domain.MaxRulesPending)
}

func (g *rulesGateway) resolveGenericDice(ctx context.Context, specification rules.Payload) (rules.Payload, error) {
	if err := ctx.Err(); err != nil {
		return rules.Payload{}, err
	}
	var request dnd5e.DiceRandomRequest
	if err := jsonstrict.Decode(specification.Bytes(), &request); err != nil {
		return rules.Payload{}, fmt.Errorf("decode random specification: %w", err)
	}
	if request.Count < 1 || request.Count > rules.MaxCollectionItems || request.Sides < 1 || request.Sides > maxGenericDiceSides {
		return rules.Payload{}, fmt.Errorf("invalid dice random request: count=%d sides=%d", request.Count, request.Sides)
	}
	response, err := g.resolveDice(request)
	if err != nil {
		return rules.Payload{}, err
	}
	if err := ctx.Err(); err != nil {
		return rules.Payload{}, err
	}
	if len(response.Rolls) != request.Count {
		return rules.Payload{}, fmt.Errorf("invalid dice random response: received %d rolls, want %d", len(response.Rolls), request.Count)
	}
	for i, face := range response.Rolls {
		if face < 1 || face > request.Sides {
			return rules.Payload{}, fmt.Errorf("invalid dice random response: roll %d is %d, want 1..%d", i, face, request.Sides)
		}
	}
	payload, err := rules.PayloadFrom(response)
	if err != nil {
		return rules.Payload{}, fmt.Errorf("encode random response: %w", err)
	}
	return payload, nil
}

func (g *rulesGateway) toolDefinitions() []types.Tool {
	if g == nil {
		return nil
	}
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
	ctx, cancel := newRulesContext(g.session)
	defer cancel()
	switch call.Name {
	case "game_submit_intent":
		release := g.session.State.LockRulesHost()
		defer release()
		return g.submitIntent(ctx, call, true)
	case "game_respond":
		release := g.session.State.LockRulesHost()
		defer release()
		return g.respond(ctx, call)
	}
	return g.executeCached(call, func() types.ToolResult {
		switch call.Name {
		case "game_observe":
			return g.observe(ctx, call)
		case "game_list_actions":
			return g.listActions(ctx, call)
		case "game_get_action_schema":
			return g.getActionSchema(ctx, call)
		case "game_preview":
			return g.preview(ctx, call)
		case "game_explain":
			return g.explain(ctx, call)
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

func rulesRequestFingerprint(call types.ToolCall) string {
	digest := sha256.Sum256(append(append([]byte(call.Name), 0), call.Arguments...))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func toolResultFromReceipt(callID string, receipt *domain.RulesReceipt) types.ToolResult {
	if receipt == nil || receipt.Result == nil {
		return errResult(callID, "rules request has no terminal receipt")
	}
	return types.ToolResult{
		ToolCallID: callID,
		Content:    receipt.Result.Content,
		Error:      receipt.Result.Error,
	}
}

func storedToolResult(result types.ToolResult) *domain.RulesStoredResult {
	return &domain.RulesStoredResult{Content: result.Content, Error: result.Error}
}

func (g *rulesGateway) beginRulesCall(ctx context.Context, call types.ToolCall) (domain.RulesRequestHandle, *types.ToolResult, error) {
	handle, receipt, err := g.session.State.BeginRulesRequest(
		ctx, call.ID, call.Name, rulesRequestFingerprint(call),
	)
	if err != nil {
		return domain.RulesRequestHandle{}, nil, err
	}
	if receipt != nil {
		if err := g.persistRules(); err != nil {
			return domain.RulesRequestHandle{}, nil, err
		}
		result := toolResultFromReceipt(call.ID, receipt)
		return domain.RulesRequestHandle{}, &result, nil
	}
	return handle, nil, nil
}

func (g *rulesGateway) beginRulesRecovery(ctx context.Context, call types.ToolCall) (domain.RulesRequestHandle, *types.ToolResult, error) {
	handle, receipt, err := g.session.State.ResumeRulesRequest(ctx, call.ID)
	if err != nil {
		return domain.RulesRequestHandle{}, nil, err
	}
	if receipt != nil {
		if err := g.persistRules(); err != nil {
			return domain.RulesRequestHandle{}, nil, err
		}
		result := toolResultFromReceipt(call.ID, receipt)
		return domain.RulesRequestHandle{}, &result, nil
	}
	return handle, nil, nil
}

func (g *rulesGateway) persistRules() error {
	if g.session.PersistRules == nil {
		return nil
	}
	if err := g.session.PersistRules(g.session.State); err != nil {
		return fmt.Errorf("persist committed rules transaction: %w", err)
	}
	return nil
}

func (g *rulesGateway) executor() ruleshost.Executor {
	return ruleshost.Executor{Ruleset: g.ruleset, Random: g.random}
}

func (g *rulesGateway) commitRules(handle domain.RulesRequestHandle, commit domain.RulesCommit) error {
	if _, err := g.session.State.CommitRulesRequest(handle, commit); err != nil {
		return err
	}
	// SessionState performs the atomic mutation and timestamp update. This flag is
	// only set after the compare-and-swap succeeds.
	g.session.MarkModified()
	if err := g.persistRules(); err != nil {
		return err
	}
	return nil
}

func (g *rulesGateway) finishRulesCall(handle domain.RulesRequestHandle, result types.ToolResult, resolutionID, removePendingID string, logs []domain.LogEntry) types.ToolResult {
	err := g.commitRules(handle, domain.RulesCommit{
		State: handle.Snapshot.State, Principal: g.principal(), ResolutionID: resolutionID,
		RemovePendingID: removePendingID, Result: storedToolResult(result), LogEntries: logs,
	})
	if err != nil {
		return errResult(result.ToolCallID, "commit rules transaction: "+err.Error())
	}
	return result
}

func findPendingResolution(pending []domain.RulesPendingResolution, resolutionID string) *domain.RulesPendingResolution {
	for i := range pending {
		if pending[i].ResolutionID == resolutionID {
			copy := pending[i]
			return &copy
		}
	}
	return nil
}

func findRulesCheckpoint(pending []domain.RulesPendingResolution, resolutionID, requestID string) *domain.RulesPendingResolution {
	value := findPendingResolution(pending, resolutionID)
	if value == nil || value.RequestID != requestID || value.Response == nil {
		return nil
	}
	return value
}

type beginRulesRequestFunc func(context.Context, types.ToolCall) (domain.RulesRequestHandle, *types.ToolResult, error)

// driveIntent resumes only responses that were already committed as durable
// checkpoints. Randomness and emitted events are checkpointed before the next
// Ruleset.Resume, so a crash can never cause a redraw or a double reduction.
func (g *rulesGateway) driveIntent(ctx context.Context, call types.ToolCall, intent *rules.Intent, resolutionID string, legacy, checkpointDurable bool) types.ToolResult {
	return g.driveIntentWithBegin(ctx, call, intent, resolutionID, legacy, checkpointDurable, g.beginRulesCall)
}

func (g *rulesGateway) driveIntentWithBegin(ctx context.Context, call types.ToolCall, intent *rules.Intent, resolutionID string, legacy, checkpointDurable bool, begin beginRulesRequestFunc) types.ToolResult {
	for {
		handle, cached, err := begin(ctx, call)
		if err != nil {
			return errResult(call.ID, err.Error())
		}
		if cached != nil {
			return *cached
		}

		checkpoint := findRulesCheckpoint(handle.Pending, resolutionID, call.ID)
		var step rules.Step
		var stepCount uint32
		initiator := g.principal()
		removePendingID := ""
		if checkpoint != nil {
			if !checkpointDurable {
				if err := g.persistRules(); err != nil {
					g.session.State.AbortRulesRequest(handle)
					return errResult(call.ID, err.Error())
				}
			}
			initiator = checkpoint.Principal
			stepCount = checkpoint.StepCount + 1
			removePendingID = resolutionID
			step, err = g.executor().Resume(
				ctx, handle.Snapshot, g.principal(), checkpoint.Pending, checkpoint.Response.Data,
			)
		} else if intent != nil {
			if len(handle.Pending) != 0 {
				result := errResult(call.ID, "another rules resolution needs input before a new intent can start")
				return g.finishRulesCall(handle, result, resolutionID, "", nil)
			}
			stepCount = 1
			step, err = g.executor().Start(ctx, rules.StartRequest{
				Snapshot: handle.Snapshot, Principal: g.principal(), Intent: *intent,
			})
		} else {
			g.session.State.AbortRulesRequest(handle)
			return errResult(call.ID, "rules response checkpoint is no longer available")
		}
		if err != nil {
			result := errResult(call.ID, err.Error())
			return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil)
		}
		if stepCount > maxAutomaticRuleSteps {
			result := errResult(call.ID, fmt.Sprintf("rules resolution exceeded %d steps", maxAutomaticRuleSteps))
			return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil)
		}

		result, checkpointed := g.applyRulesStep(ctx, call, handle, resolutionID, step, stepCount, removePendingID, initiator, legacy)
		if !checkpointed {
			return result
		}
		if g.afterCheckpoint != nil {
			if err := g.afterCheckpoint(); err != nil {
				return errResult(call.ID, "rules checkpoint interruption: "+err.Error())
			}
		}
		// CommitRulesRequest closed the prior ownership claim. Reacquire a fresh
		// generation/revision before acknowledging the persisted response.
		intent = nil
		checkpointDurable = true
	}
}

func (g *rulesGateway) applyRulesStep(ctx context.Context, call types.ToolCall, handle domain.RulesRequestHandle, resolutionID string, step rules.Step, stepCount uint32, removePendingID string, initiator rules.Principal, legacy bool) (types.ToolResult, bool) {
	snapshot := handle.Snapshot
	switch step.Kind {
	case rules.StepKindReject:
		result := gameJSONResult(call.ID, gameEnvelope{
			Status: "rejected", Ruleset: snapshot.Ruleset, Revision: snapshot.Revision,
			ResolutionID: resolutionID, Data: step.Reject,
		})
		if legacy {
			result = errResult(call.ID, step.Reject.Message)
		}
		return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil), false

	case rules.StepKindComplete:
		var logs []domain.LogEntry
		if legacy {
			legacyResult, err := legacyProjection(step.Complete.Result)
			if err != nil {
				result := errResult(call.ID, "decode rules result: "+err.Error())
				return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil), false
			}
			result := okResult(call.ID, legacyResult.Content)
			logs = []domain.LogEntry{{Type: domain.LogRoll, Message: legacyResult.LogMessage}}
			return g.finishRulesCall(handle, result, resolutionID, removePendingID, logs), false
		}
		if g.legacyDND5E {
			legacyResult, present, err := optionalLegacyProjection(step.Complete.Result)
			if err != nil {
				result := errResult(call.ID, "project legacy rules result: "+err.Error())
				return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil), false
			}
			if present {
				logs = []domain.LogEntry{{Type: domain.LogRoll, Message: legacyResult.LogMessage}}
			}
		}
		result := gameJSONResult(call.ID, gameEnvelope{
			Status: "resolved", Ruleset: snapshot.Ruleset, Revision: snapshot.Revision,
			ResolutionID: resolutionID, Outcome: step.Complete.Outcome,
			Data: json.RawMessage(step.Complete.Result.Bytes()),
		})
		return g.finishRulesCall(handle, result, resolutionID, removePendingID, logs), false

	case rules.StepKindNeedDecision, rules.StepKindNeedAdjudication:
		pendingStep, err := step.Pending()
		if err != nil {
			result := errResult(call.ID, err.Error())
			return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil), false
		}
		request, err := ruleshost.PublicRequest(step)
		if err != nil {
			result := errResult(call.ID, err.Error())
			return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil), false
		}
		pending := &domain.RulesPendingResolution{
			ResolutionID: resolutionID, RequestID: call.ID, Principal: initiator,
			Pending: pendingStep, Request: request, StepCount: stepCount,
		}
		result := gameJSONResult(call.ID, gameEnvelope{
			Status: "needs_input", Ruleset: snapshot.Ruleset, Revision: snapshot.Revision,
			ResolutionID: resolutionID,
			Data:         map[string]any{"next_step": step.Kind, "request": json.RawMessage(request.Bytes())},
		})
		if err := g.commitRules(handle, domain.RulesCommit{
			State: snapshot.State, Principal: g.principal(), ResolutionID: resolutionID,
			Pending: pending, Result: storedToolResult(result),
		}); err != nil {
			return errResult(call.ID, "commit rules transaction: "+err.Error()), false
		}
		return result, false

	case rules.StepKindNeedRandom:
		pendingStep, _ := step.Pending()
		request, err := rules.PayloadFrom(step.NeedRandom)
		if err != nil {
			result := errResult(call.ID, "encode random request: "+err.Error())
			return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil), false
		}
		response, err := g.executor().ResolveRandom(ctx, *step.NeedRandom)
		if err != nil {
			result := errResult(call.ID, err.Error())
			return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil), false
		}
		pending := &domain.RulesPendingResolution{
			ResolutionID: resolutionID, RequestID: call.ID, Principal: initiator,
			Pending: pendingStep, Request: request, StepCount: stepCount,
			Response: &rules.HostResponse{StepID: pendingStep.StepID, Kind: pendingStep.Kind, Data: response},
		}
		if err := g.commitRules(handle, domain.RulesCommit{
			State: snapshot.State, Principal: g.principal(), ResolutionID: resolutionID, Pending: pending,
			RandomDraws: []domain.RulesRandomDraft{{
				ResolutionID: resolutionID, StepID: pendingStep.StepID, Method: step.NeedRandom.Method,
				Source: "host", Specification: step.NeedRandom.Specification, Result: response,
			}},
		}); err != nil {
			return errResult(call.ID, "commit rules checkpoint: "+err.Error()), false
		}
		return types.ToolResult{}, true

	case rules.StepKindEmit:
		pendingStep, _ := step.Pending()
		request, err := rules.PayloadFrom(step.Emit)
		if err != nil {
			result := errResult(call.ID, "encode emission request: "+err.Error())
			return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil), false
		}
		candidate, response, err := g.executor().Reduce(ctx, snapshot, *step.Emit)
		if err != nil {
			result := errResult(call.ID, err.Error())
			return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil), false
		}
		pending := &domain.RulesPendingResolution{
			ResolutionID: resolutionID, RequestID: call.ID, Principal: initiator,
			Pending: pendingStep, Request: request, StepCount: stepCount,
			Response: &rules.HostResponse{StepID: pendingStep.StepID, Kind: pendingStep.Kind, Data: response},
		}
		if err := g.commitRules(handle, domain.RulesCommit{
			State: candidate.State, Principal: g.principal(), ResolutionID: resolutionID, Pending: pending,
			EventBatches: []domain.RulesEventDraft{{ResolutionID: resolutionID, Events: step.Emit.Events}},
		}); err != nil {
			return errResult(call.ID, "commit rules checkpoint: "+err.Error()), false
		}
		return types.ToolResult{}, true

	case rules.StepKindStartChild:
		result := errResult(call.ID, "ruleset requested a child resolution, but child execution is not supported by this host")
		return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil), false
	default:
		result := errResult(call.ID, "ruleset returned unsupported step "+string(step.Kind))
		return g.finishRulesCall(handle, result, resolutionID, removePendingID, nil), false
	}
}

func (g *rulesGateway) respond(ctx context.Context, call types.ToolCall) types.ToolResult {
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
	responsePayload, err := rules.NewPayload(arguments.Response)
	if err != nil {
		return errResult(call.ID, "invalid 'response': "+err.Error())
	}

	handle, cached, err := g.beginRulesCall(ctx, call)
	if err != nil {
		return errResult(call.ID, err.Error())
	}
	if cached != nil {
		return *cached
	}
	if checkpoint := findRulesCheckpoint(handle.Pending, arguments.ResolutionID, call.ID); checkpoint != nil {
		g.session.State.AbortRulesRequest(handle)
		return g.driveIntent(ctx, call, nil, arguments.ResolutionID, false, false)
	}
	persisted := findPendingResolution(handle.Pending, arguments.ResolutionID)
	if persisted == nil {
		result := errResult(call.ID, "rules response conflict: no externally pending resolution exists for "+arguments.ResolutionID)
		return g.finishRulesCall(handle, result, arguments.ResolutionID, "", nil)
	}
	if persisted.Response != nil {
		g.session.State.AbortRulesRequest(handle)
		return errResult(call.ID, "rules response conflict: resolution already has a committed response; retry after it completes")
	}
	if persisted.Revision != handle.Snapshot.Revision {
		result := errResult(call.ID, fmt.Sprintf("pending resolution is stale: expected revision %d, found %d", persisted.Revision, handle.Snapshot.Revision))
		return g.finishRulesCall(handle, result, arguments.ResolutionID, arguments.ResolutionID, nil)
	}
	if !g.principalMayRespond(*persisted) {
		result := errResult(call.ID, "current principal is not authorized to answer the pending rules request")
		return g.finishRulesCall(handle, result, arguments.ResolutionID, "", nil)
	}
	checkpoint := *persisted
	checkpoint.RequestID = call.ID
	checkpoint.Response = &rules.HostResponse{
		StepID: checkpoint.Pending.StepID, Kind: checkpoint.Pending.Kind, Data: responsePayload,
	}
	if err := g.commitRules(handle, domain.RulesCommit{
		State: handle.Snapshot.State, Principal: g.principal(), ResolutionID: arguments.ResolutionID,
		Pending: &checkpoint,
	}); err != nil {
		return errResult(call.ID, "commit rules response checkpoint: "+err.Error())
	}
	if g.afterCheckpoint != nil {
		if err := g.afterCheckpoint(); err != nil {
			return errResult(call.ID, "rules checkpoint interruption: "+err.Error())
		}
	}
	return g.driveIntent(ctx, call, nil, arguments.ResolutionID, false, true)
}

func (g *rulesGateway) principalMayRespond(pending domain.RulesPendingResolution) bool {
	var authority string
	switch pending.Pending.Kind {
	case rules.StepKindNeedDecision:
		var request rules.DecisionRequest
		if json.Unmarshal(pending.Request.Bytes(), &request) != nil {
			return false
		}
		authority = request.Authority
	case rules.StepKindNeedAdjudication:
		var request rules.AdjudicationRequest
		if json.Unmarshal(pending.Request.Bytes(), &request) != nil {
			return false
		}
		authority = request.Authority
	default:
		return false
	}
	principal := g.principal()
	if authority == principal.ID {
		return true
	}
	for _, role := range principal.Roles {
		if authority == role {
			return true
		}
	}
	return false
}

func (g *rulesGateway) observe(ctx context.Context, call types.ToolCall) types.ToolResult {
	if err := decodeToolArguments(call.Arguments, &struct{}{}); err != nil {
		return errResult(call.ID, err.Error())
	}
	runtime, ok := g.session.State.RulesRuntimeSnapshot()
	if !ok || runtime.Lock != g.lock {
		return errResult(call.ID, "session has no valid rules runtime for the loaded package")
	}
	snapshot := rules.Snapshot{Ruleset: runtime.Lock, Revision: runtime.Revision, State: runtime.State}
	projection, err := g.executor().Project(ctx, rules.ProjectRequest{
		Snapshot: snapshot, Principal: g.principal(),
	})
	if err != nil {
		return errResult(call.ID, err.Error())
	}
	var pendingViews []pendingResolutionView
	for _, pending := range runtime.Pending {
		if pending.Response == nil && g.principalMayRespond(pending) {
			pendingViews = append(pendingViews, pendingResolutionView{
				ResolutionID: pending.ResolutionID, Kind: pending.Pending.Kind,
				Revision: pending.Revision, Request: json.RawMessage(pending.Request.Bytes()),
			})
		}
	}
	return gameJSONResult(call.ID, gameEnvelope{
		Status:   "resolved",
		Ruleset:  snapshot.Ruleset,
		Revision: snapshot.Revision,
		Data:     json.RawMessage(projection.View.Bytes()),
		Pending:  pendingViews,
	})
}

func (g *rulesGateway) actions(ctx context.Context) (rules.Snapshot, []rules.ActionDescriptor, error) {
	snapshot, err := g.snapshot()
	if err != nil {
		return rules.Snapshot{}, nil, err
	}
	actions, err := g.executor().ListActions(ctx, rules.CatalogRequest{
		Snapshot: snapshot, Principal: g.principal(),
	})
	if err != nil {
		return rules.Snapshot{}, nil, err
	}
	return snapshot, actions, nil
}

func (g *rulesGateway) listActions(ctx context.Context, call types.ToolCall) types.ToolResult {
	if err := decodeToolArguments(call.Arguments, &struct{}{}); err != nil {
		return errResult(call.ID, err.Error())
	}
	snapshot, actions, err := g.actions(ctx)
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

func (g *rulesGateway) getActionSchema(ctx context.Context, call types.ToolCall) types.ToolResult {
	var arguments struct {
		ActionID string `json:"action_id"`
	}
	if err := decodeToolArguments(call.Arguments, &arguments); err != nil {
		return errResult(call.ID, err.Error())
	}
	if arguments.ActionID == "" {
		return errResult(call.ID, "missing 'action_id'")
	}
	snapshot, actions, err := g.actions(ctx)
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

func (g *rulesGateway) submitIntent(ctx context.Context, call types.ToolCall, commit bool) types.ToolResult {
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
	if !commit {
		return errResult(call.ID, "internal error: non-committing intent must use game_preview")
	}
	intent := rules.Intent{
		ID: resolutionID, ActionID: arguments.ActionID,
		ActorID: arguments.ActorID, Arguments: payload,
	}
	return g.driveIntent(ctx, call, &intent, resolutionID, false, false)
}

func (g *rulesGateway) preview(ctx context.Context, call types.ToolCall) types.ToolResult {
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
	step, err := g.executor().Start(ctx, rules.StartRequest{
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
		return errResult(call.ID, "preview encountered unsupported child resolution")
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

func (g *rulesGateway) explain(ctx context.Context, call types.ToolCall) types.ToolResult {
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
	explanation, err := g.executor().Explain(ctx, rules.ExplainRequest{
		Snapshot: snapshot, Principal: g.principal(),
		Reference: arguments.Reference, Locale: arguments.Locale,
	})
	if err != nil {
		return errResult(call.ID, err.Error())
	}
	return gameJSONResult(call.ID, gameEnvelope{
		Status:   "resolved",
		Ruleset:  snapshot.Ruleset,
		Revision: snapshot.Revision,
		Data:     explanation,
	})
}

type legacyCompletion struct {
	Legacy dnd5e.LegacyResult `json:"legacy"`
}

func legacyProjection(payload rules.Payload) (dnd5e.LegacyResult, error) {
	var result legacyCompletion
	if err := json.Unmarshal(payload.Bytes(), &result); err != nil {
		return dnd5e.LegacyResult{}, err
	}
	if result.Legacy.LogType != string(domain.LogRoll) {
		return dnd5e.LegacyResult{}, fmt.Errorf("unsupported legacy log type %q", result.Legacy.LogType)
	}
	if result.Legacy.Content == "" || result.Legacy.LogMessage == "" {
		return dnd5e.LegacyResult{}, fmt.Errorf("rules result omitted its legacy projection")
	}
	return result.Legacy, nil
}

func optionalLegacyProjection(payload rules.Payload) (dnd5e.LegacyResult, bool, error) {
	trimmed := bytes.TrimSpace(payload.Bytes())
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return dnd5e.LegacyResult{}, false, nil
	}
	var raw struct {
		Legacy json.RawMessage `json:"legacy"`
	}
	if err := json.Unmarshal(payload.Bytes(), &raw); err != nil {
		return dnd5e.LegacyResult{}, false, err
	}
	if len(bytes.TrimSpace(raw.Legacy)) == 0 || bytes.Equal(bytes.TrimSpace(raw.Legacy), []byte("null")) {
		return dnd5e.LegacyResult{}, false, nil
	}
	legacy, err := legacyProjection(payload)
	return legacy, true, err
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
	if !g.legacyDND5E {
		return errResult(call.ID, "legacy D&D tool is unavailable for the loaded rules package")
	}
	release := g.session.State.LockRulesHost()
	defer release()
	ctx, cancel := newRulesContext(g.session)
	defer cancel()
	resolutionID := intentID(call, actionID, payload)
	intent := rules.Intent{ID: resolutionID, ActionID: actionID, Arguments: payload}
	return g.driveIntent(ctx, call, &intent, resolutionID, true, false)
}

func intentID(call types.ToolCall, actionID string, payload rules.Payload) string {
	if call.ID != "" {
		return call.ID
	}
	digest := sha256.Sum256(append(append([]byte(actionID), 0), payload.Bytes()...))
	return "intent:" + hex.EncodeToString(digest[:12])
}

type gameEnvelope struct {
	Status       string                  `json:"status"`
	Ruleset      rules.Lock              `json:"ruleset"`
	Revision     uint64                  `json:"revision"`
	ResolutionID string                  `json:"resolution_id,omitempty"`
	Outcome      string                  `json:"outcome,omitempty"`
	Data         any                     `json:"data,omitempty"`
	Pending      []pendingResolutionView `json:"pending,omitempty"`
}

type pendingResolutionView struct {
	ResolutionID string          `json:"resolution_id"`
	Kind         rules.StepKind  `json:"kind"`
	Revision     uint64          `json:"revision"`
	Request      json.RawMessage `json:"request"`
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
