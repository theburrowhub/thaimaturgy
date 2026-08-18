package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/rules"
	"github.com/theburrowhub/thaimaturgy/internal/rules/dnd5e"
	"github.com/theburrowhub/thaimaturgy/internal/ruleshost"
	"github.com/theburrowhub/thaimaturgy/internal/types"
)

const transactionTestDigest = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

type transactionTestRuleset struct {
	authority  string
	starts     atomic.Int32
	resumes    atomic.Int32
	reduces    atomic.Int32
	resumeRole atomic.Value
}

type panicTransactionReduceRuleset struct{ *transactionTestRuleset }

func (r panicTransactionReduceRuleset) Reduce(context.Context, rules.ReduceRequest) (rules.ReduceResult, error) {
	panic("broken transaction reducer")
}

func transactionPayload(t *testing.T, value any) rules.Payload {
	t.Helper()
	payload, err := rules.PayloadFrom(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func transactionLock() rules.Lock {
	return rules.Lock{
		ID: "test.transaction", Version: "1.0.0", Digest: transactionTestDigest,
		ProtocolVersion: rules.ProtocolVersion,
	}
}

func (r *transactionTestRuleset) Manifest(context.Context) (rules.Manifest, error) {
	return rules.Manifest{}, errors.New("not used")
}

func (r *transactionTestRuleset) ListActions(context.Context, rules.CatalogRequest) ([]rules.ActionDescriptor, error) {
	return nil, errors.New("not used")
}

func (r *transactionTestRuleset) Start(_ context.Context, request rules.StartRequest) (rules.Step, error) {
	r.starts.Add(1)
	continuation, _ := rules.PayloadFrom(map[string]any{"phase": "choice"})
	return rules.Step{
		ID: request.Intent.ID + ":choice", Kind: rules.StepKindNeedDecision, Continuation: continuation,
		NeedDecision: &rules.DecisionRequest{
			Authority: r.authority, Prompt: "Choose",
			Options: []rules.DecisionOption{{ID: "advance", Label: "Advance"}},
		},
	}, nil
}

func (r *transactionTestRuleset) Resume(_ context.Context, request rules.ResumeRequest) (rules.Step, error) {
	r.resumes.Add(1)
	if len(request.Principal.Roles) > 0 {
		r.resumeRole.Store(request.Principal.Roles[0])
	}
	switch request.Pending.Kind {
	case rules.StepKindNeedDecision:
		var response struct {
			Option string `json:"option"`
		}
		if err := json.Unmarshal(request.Response.Data.Bytes(), &response); err != nil {
			return rules.Step{}, err
		}
		if response.Option != "advance" {
			return rules.Step{}, errors.New("invalid option")
		}
		continuation, _ := rules.PayloadFrom(map[string]any{"phase": "emit"})
		data, _ := rules.PayloadFrom(map[string]any{"amount": 1})
		return rules.Step{
			ID: request.Pending.StepID + ":emit", Kind: rules.StepKindEmit, Continuation: continuation,
			Emit: &rules.Emission{Events: []rules.Event{{
				Type: "counter.incremented", SchemaVersion: 1, Data: data,
			}}},
		}, nil
	case rules.StepKindEmit:
		result, _ := rules.PayloadFrom(map[string]any{"counter": 1})
		return rules.Step{
			ID: request.Pending.StepID + ":complete", Kind: rules.StepKindComplete,
			Complete: &rules.Completion{Outcome: "test.advanced", Result: result},
		}, nil
	default:
		return rules.Step{}, errors.New("unexpected pending kind")
	}
}

func (r *transactionTestRuleset) Project(_ context.Context, request rules.ProjectRequest) (rules.Projection, error) {
	return rules.Projection{View: request.Snapshot.State}, nil
}

func (r *transactionTestRuleset) Explain(context.Context, rules.ExplainRequest) (rules.Explanation, error) {
	return rules.Explanation{}, errors.New("not used")
}

func (r *transactionTestRuleset) ValidateState(_ context.Context, request rules.ValidateStateRequest) error {
	var state struct {
		Counter int `json:"counter"`
	}
	if err := json.Unmarshal(request.Snapshot.State.Bytes(), &state); err != nil {
		return err
	}
	if state.Counter < 0 {
		return errors.New("negative counter")
	}
	return nil
}

func (r *transactionTestRuleset) Reduce(_ context.Context, request rules.ReduceRequest) (rules.ReduceResult, error) {
	r.reduces.Add(1)
	var state struct {
		Counter int `json:"counter"`
	}
	if err := json.Unmarshal(request.Snapshot.State.Bytes(), &state); err != nil {
		return rules.ReduceResult{}, err
	}
	for _, event := range request.Events {
		if event.Type != "counter.incremented" {
			return rules.ReduceResult{}, errors.New("unexpected event")
		}
		state.Counter++
	}
	payload, err := rules.PayloadFrom(state)
	return rules.ReduceResult{State: payload}, err
}

func (r *transactionTestRuleset) Migrate(context.Context, rules.MigrateRequest) (rules.MigrateResult, error) {
	return rules.MigrateResult{}, errors.New("not used")
}

func transactionTestSession(t *testing.T, implementation rules.Ruleset) (*domain.Session, *ToolRouter) {
	t.Helper()
	state := domain.NewSessionState("transaction", nil)
	if _, err := state.BindRules(transactionLock(), transactionPayload(t, map[string]any{"counter": 0})); err != nil {
		t.Fatal(err)
	}
	session := domain.NewSession(state, &domain.Adventure{System: "test.transaction"}, nil)
	gateway := newRulesGatewayWithRuleset(session, transactionLock(), implementation)
	return session, &ToolRouter{session: session, rules: gateway}
}

func restoreTransactionRouter(t *testing.T, source *domain.Session, implementation rules.Ruleset) (*domain.Session, *ToolRouter) {
	t.Helper()
	raw, err := json.Marshal(source.State)
	if err != nil {
		t.Fatal(err)
	}
	var restored domain.SessionState
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	session := domain.NewSession(&restored, source.Adventure, nil)
	gateway := newRulesGatewayWithRuleset(session, transactionLock(), implementation)
	return session, &ToolRouter{session: session, rules: gateway}
}

func TestPendingResolutionSurvivesRestartAndCommitsEventExactlyOnce(t *testing.T) {
	implementation := &transactionTestRuleset{authority: "host:oracle"}
	session, router := transactionTestSession(t, implementation)
	submit := types.ToolCall{
		ID: "submit-pending", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"test.choose","arguments":{}}`),
	}
	first := router.Execute(submit)
	if first.Error != "" || !strings.Contains(first.Content, `"status":"needs_input"`) {
		t.Fatalf("submit = %+v", first)
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if !ok || len(runtime.Pending) != 1 || len(runtime.Receipts) != 1 || runtime.Revision != 0 {
		t.Fatalf("pending runtime: ok=%v runtime=%+v", ok, runtime)
	}
	resolutionID := runtime.Pending[0].ResolutionID

	restarted, restartedRouter := restoreTransactionRouter(t, session, implementation)
	if retry := restartedRouter.Execute(submit); retry != first {
		t.Fatalf("submit retry changed: first=%+v retry=%+v", first, retry)
	}
	if implementation.starts.Load() != 1 {
		t.Fatalf("start calls = %d", implementation.starts.Load())
	}
	observed := restartedRouter.Execute(types.ToolCall{ID: "observe-pending", Name: "game_observe", Arguments: json.RawMessage(`{}`)})
	if observed.Error != "" || !strings.Contains(observed.Content, `"pending":[{`) ||
		!strings.Contains(observed.Content, `"resolution_id":"`+resolutionID+`"`) ||
		!strings.Contains(observed.Content, `"prompt":"Choose"`) {
		t.Fatalf("observe pending = %+v", observed)
	}
	if strings.Contains(observed.Content, "continuation") || strings.Contains(observed.Content, `"response"`) || strings.Contains(observed.Content, `"phase":"choice"`) {
		t.Fatalf("observe leaked host-only pending state: %s", observed.Content)
	}
	restarted.State.SetMode(domain.ModeVirtualDM)

	respond := types.ToolCall{
		ID: "respond-pending", Name: "game_respond",
		Arguments: json.RawMessage(`{"resolution_id":"` + resolutionID + `","response":{"option":"advance"}}`),
	}
	resolved := restartedRouter.Execute(respond)
	if resolved.Error != "" || !strings.Contains(resolved.Content, `"status":"resolved"`) || !strings.Contains(resolved.Content, `"revision":1`) {
		t.Fatalf("respond = %+v", resolved)
	}
	runtime, ok = restarted.State.RulesRuntimeSnapshot()
	if !ok || runtime.Revision != 1 || runtime.State.String() != `{"counter":1}` || len(runtime.Pending) != 0 || len(runtime.EventBatches) != 1 || len(runtime.Receipts) != 2 {
		t.Fatalf("resolved runtime: ok=%v runtime=%+v", ok, runtime)
	}
	batch := runtime.EventBatches[0]
	if batch.RequestID != respond.ID || batch.Ruleset != runtime.Lock || batch.Principal.ID != "host:oracle" || batch.ResolutionID != resolutionID {
		t.Fatalf("event audit attribution = %+v", batch)
	}
	if implementation.reduces.Load() != 1 {
		t.Fatalf("reduce calls = %d", implementation.reduces.Load())
	}
	if role, _ := implementation.resumeRole.Load().(string); role != "game-master" {
		t.Fatalf("Resume principal role = %q, want current game-master", role)
	}
	replayBatches := make([]ruleshost.ReplayBatch, len(runtime.EventBatches))
	for i, batch := range runtime.EventBatches {
		replayBatches[i] = ruleshost.ReplayBatch{
			Sequence: batch.Sequence, BaseRevision: batch.BaseRevision,
			Revision: batch.Revision, Events: batch.Events,
		}
	}
	replayed, err := ruleshost.Replay(context.Background(), implementation, runtime.Lock, runtime.InitialState, replayBatches)
	if err != nil || replayed.Revision != runtime.Revision || replayed.State.String() != runtime.State.String() {
		t.Fatalf("replay=%+v err=%v materialized=%+v", replayed, err, runtime)
	}
	reducesAfterReplay := implementation.reduces.Load()

	_, secondRestartRouter := restoreTransactionRouter(t, restarted, implementation)
	if retry := secondRestartRouter.Execute(respond); retry != resolved {
		t.Fatalf("respond retry changed: first=%+v retry=%+v", resolved, retry)
	}
	if implementation.resumes.Load() != 2 || implementation.reduces.Load() != reducesAfterReplay {
		t.Fatalf("retry repeated execution: resumes=%d reduces=%d", implementation.resumes.Load(), implementation.reduces.Load())
	}
}

func TestPendingResponseAuthorizationFailurePreservesMechanicalState(t *testing.T) {
	implementation := &transactionTestRuleset{authority: "player:one"}
	session, router := transactionTestSession(t, implementation)
	submit := router.Execute(types.ToolCall{
		ID: "submit-player-choice", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"test.choose","arguments":{}}`),
	})
	if submit.Error != "" {
		t.Fatal(submit.Error)
	}
	runtime, _ := session.State.RulesRuntimeSnapshot()
	resolutionID := runtime.Pending[0].ResolutionID
	response := router.Execute(types.ToolCall{
		ID: "unauthorized-response", Name: "game_respond",
		Arguments: json.RawMessage(`{"resolution_id":"` + resolutionID + `","response":{"option":"advance"}}`),
	})
	if !strings.Contains(response.Error, "not authorized") {
		t.Fatalf("response = %+v", response)
	}
	runtime, _ = session.State.RulesRuntimeSnapshot()
	if runtime.Revision != 0 || runtime.State.String() != `{"counter":0}` || len(runtime.Pending) != 1 || len(runtime.EventBatches) != 0 || len(runtime.RandomDraws) != 0 || implementation.resumes.Load() != 0 {
		t.Fatalf("unauthorized response mutated mechanics: %+v", runtime)
	}
}

func TestReducePanicFinalizesErrorWithoutMechanicalMutation(t *testing.T) {
	base := &transactionTestRuleset{authority: "host:oracle"}
	implementation := panicTransactionReduceRuleset{transactionTestRuleset: base}
	session, router := transactionTestSession(t, implementation)
	submit := router.Execute(types.ToolCall{
		ID: "panic-reduce-submit", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"test.choose","arguments":{}}`),
	})
	if submit.Error != "" {
		t.Fatal(submit.Error)
	}
	runtime, _ := session.State.RulesRuntimeSnapshot()
	resolutionID := runtime.Pending[0].ResolutionID
	respond := types.ToolCall{
		ID: "panic-reduce-response", Name: "game_respond",
		Arguments: json.RawMessage(`{"resolution_id":"` + resolutionID + `","response":{"option":"advance"}}`),
	}
	first := router.Execute(respond)
	if !strings.Contains(first.Error, "panicked") {
		t.Fatalf("response = %+v", first)
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if !ok || runtime.Revision != 0 || runtime.State.String() != `{"counter":0}` || len(runtime.EventBatches) != 0 || len(runtime.Pending) != 0 {
		t.Fatalf("reduce panic partially mutated mechanics: ok=%v runtime=%+v", ok, runtime)
	}
	if retry := router.Execute(respond); retry != first || base.resumes.Load() != 1 {
		t.Fatalf("panic retry=%+v first=%+v resumes=%d", retry, first, base.resumes.Load())
	}
}

func TestDurableReceiptSurvivesRestartWithoutRedrawOrDuplicateLog(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	draws := 0
	router.rules.resolveDice = func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		draws++
		return dnd5e.DiceRandomResponse{Rolls: []int{11}}, nil
	}
	call := types.ToolCall{
		ID: "durable-roll", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"1d20"}}`),
	}
	first := router.Execute(call)
	if first.Error != "" {
		t.Fatal(first.Error)
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if !ok || len(runtime.RandomDraws) != 1 || len(runtime.Receipts) != 1 || session.State.LogLen() != 1 {
		t.Fatalf("runtime after roll: ok=%v runtime=%+v log=%d", ok, runtime, session.State.LogLen())
	}
	draw := runtime.RandomDraws[0]
	if draw.RequestID != call.ID || draw.Ruleset != runtime.Lock || draw.Principal.ID != "host:oracle" || draw.Method != dnd5e.RandomMethodDiceRoll || draw.Source != "host" {
		t.Fatalf("random audit attribution = %+v", draw)
	}
	raw, err := json.Marshal(session.State)
	if err != nil {
		t.Fatal(err)
	}
	var restoredState domain.SessionState
	if err := json.Unmarshal(raw, &restoredState); err != nil {
		t.Fatal(err)
	}
	restored := domain.NewSession(&restoredState, session.Adventure, session.Config)
	restored.RulesResolver = session.RulesResolver
	restartedRouter := NewToolRouter(restored)
	restartedRouter.rules.resolveDice = func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		t.Fatal("retry redrew randomness")
		return dnd5e.DiceRandomResponse{}, nil
	}
	if retry := restartedRouter.Execute(call); retry != first {
		t.Fatalf("restart retry changed: first=%+v retry=%+v", first, retry)
	}
	if draws != 1 || restored.State.LogLen() != 1 {
		t.Fatalf("restart duplicated effects: draws=%d log=%d", draws, restored.State.LogLen())
	}
}

func TestTwoRoutersShareOneInFlightRequest(t *testing.T) {
	session := createTestSession()
	firstRouter := NewToolRouter(session)
	secondRouter := NewToolRouter(session)
	var draws atomic.Int32
	resolver := func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		draws.Add(1)
		return dnd5e.DiceRandomResponse{Rolls: []int{9}}, nil
	}
	firstRouter.rules.resolveDice = resolver
	secondRouter.rules.resolveDice = resolver
	call := types.ToolCall{
		ID: "two-router-roll", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"1d20"}}`),
	}
	var results [2]types.ToolResult
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		results[0] = firstRouter.Execute(call)
	}()
	go func() {
		defer wait.Done()
		results[1] = secondRouter.Execute(call)
	}()
	wait.Wait()
	if results[0] != results[1] || results[0].Error != "" {
		t.Fatalf("results = %+v / %+v", results[0], results[1])
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if draws.Load() != 1 || !ok || len(runtime.RandomDraws) != 1 || len(runtime.Receipts) != 1 || session.State.LogLen() != 1 {
		t.Fatalf("draws=%d ok=%v runtime=%+v log=%d", draws.Load(), ok, runtime, session.State.LogLen())
	}
}

func TestTwoResponsesConsumePendingResolutionExactlyOnce(t *testing.T) {
	implementation := &transactionTestRuleset{authority: "host:oracle"}
	session, firstRouter := transactionTestSession(t, implementation)
	secondRouter := &ToolRouter{
		session: session,
		rules:   newRulesGatewayWithRuleset(session, transactionLock(), implementation),
	}
	submit := firstRouter.Execute(types.ToolCall{
		ID: "response-race-submit", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"test.choose","arguments":{}}`),
	})
	if submit.Error != "" {
		t.Fatal(submit.Error)
	}
	runtime, _ := session.State.RulesRuntimeSnapshot()
	resolutionID := runtime.Pending[0].ResolutionID
	calls := []types.ToolCall{
		{ID: "response-race-a", Name: "game_respond", Arguments: json.RawMessage(`{"resolution_id":"` + resolutionID + `","response":{"option":"advance"}}`)},
		{ID: "response-race-b", Name: "game_respond", Arguments: json.RawMessage(`{"resolution_id":"` + resolutionID + `","response":{"option":"advance"}}`)},
	}
	var results [2]types.ToolResult
	var wait sync.WaitGroup
	wait.Add(2)
	go func() { defer wait.Done(); results[0] = firstRouter.Execute(calls[0]) }()
	go func() { defer wait.Done(); results[1] = secondRouter.Execute(calls[1]) }()
	wait.Wait()
	resolved, conflicts := 0, 0
	for _, result := range results {
		if result.Error == "" && strings.Contains(result.Content, `"status":"resolved"`) {
			resolved++
		}
		if strings.Contains(result.Error, "conflict") {
			conflicts++
		}
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if resolved != 1 || conflicts != 1 || !ok || runtime.Revision != 1 || len(runtime.EventBatches) != 1 || len(runtime.Pending) != 0 || implementation.reduces.Load() != 1 {
		t.Fatalf("response race results=%+v resolved=%d conflicts=%d runtime=%+v reduces=%d", results, resolved, conflicts, runtime, implementation.reduces.Load())
	}
}

func TestRespondSerializesPersistenceWithNewSubmit(t *testing.T) {
	implementation := &transactionTestRuleset{authority: "host:oracle"}
	session, respondRouter := transactionTestSession(t, implementation)
	submitRouter := &ToolRouter{
		session: session,
		rules:   newRulesGatewayWithRuleset(session, transactionLock(), implementation),
	}
	initial := respondRouter.Execute(types.ToolCall{
		ID: "persist-lock-initial", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"test.choose","arguments":{}}`),
	})
	if initial.Error != "" {
		t.Fatal(initial.Error)
	}
	runtime, _ := session.State.RulesRuntimeSnapshot()
	resolutionID := runtime.Pending[0].ResolutionID

	firstPersist := make(chan struct{})
	releasePersist := make(chan struct{})
	overlap := make(chan struct{}, 1)
	var blockFirst sync.Once
	var active atomic.Int32
	var generationsMu sync.Mutex
	var generations []uint64
	session.PersistRules = func(state *domain.SessionState) error {
		if active.Add(1) != 1 {
			select {
			case overlap <- struct{}{}:
			default:
			}
		}
		defer active.Add(-1)
		runtime, ok := state.RulesRuntimeSnapshot()
		if !ok {
			return errors.New("invalid runtime during persistence")
		}
		generationsMu.Lock()
		generations = append(generations, runtime.Generation)
		generationsMu.Unlock()
		blockFirst.Do(func() {
			close(firstPersist)
			<-releasePersist
		})
		return nil
	}

	respondDone := make(chan types.ToolResult, 1)
	go func() {
		respondDone <- respondRouter.Execute(types.ToolCall{
			ID: "persist-lock-response", Name: "game_respond",
			Arguments: json.RawMessage(`{"resolution_id":"` + resolutionID + `","response":{"option":"advance"}}`),
		})
	}()
	<-firstPersist
	submitStarted := make(chan struct{})
	submitDone := make(chan types.ToolResult, 1)
	go func() {
		close(submitStarted)
		submitDone <- submitRouter.Execute(types.ToolCall{
			ID: "persist-lock-next", Name: "game_submit_intent",
			Arguments: json.RawMessage(`{"action_id":"test.choose","arguments":{}}`),
		})
	}()
	<-submitStarted
	select {
	case <-overlap:
		t.Fatal("game_respond and game_submit_intent entered persistence concurrently")
	case <-time.After(100 * time.Millisecond):
	}
	close(releasePersist)
	if result := <-respondDone; result.Error != "" {
		t.Fatalf("response = %+v", result)
	}
	if result := <-submitDone; result.Error != "" || !strings.Contains(result.Content, `"status":"needs_input"`) {
		t.Fatalf("next submit = %+v", result)
	}
	select {
	case <-overlap:
		t.Fatal("persistence callbacks overlapped")
	default:
	}
	generationsMu.Lock()
	defer generationsMu.Unlock()
	for i := 1; i < len(generations); i++ {
		if generations[i] <= generations[i-1] {
			t.Fatalf("persistence generations are not ordered: %v", generations)
		}
	}
}

func TestRestartAfterRandomCheckpointReusesAuditWithoutRedraw(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	draws := 0
	router.rules.resolveDice = func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		draws++
		return dnd5e.DiceRandomResponse{Rolls: []int{13}}, nil
	}
	router.rules.afterCheckpoint = func() error { return errors.New("simulated crash") }
	call := types.ToolCall{
		ID: "crash-after-random", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"1d20"}}`),
	}
	interrupted := router.Execute(call)
	if !strings.Contains(interrupted.Error, "simulated crash") {
		t.Fatalf("interrupted result = %+v", interrupted)
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if !ok || len(runtime.RandomDraws) != 1 || len(runtime.Pending) != 1 || runtime.Pending[0].Response == nil || len(runtime.Receipts) != 1 || runtime.Receipts[0].Result != nil {
		t.Fatalf("random checkpoint was not durable: ok=%v runtime=%+v", ok, runtime)
	}

	restored, restartedRouter := restoreDNDTransactionRouter(t, session)
	runtime, ok = restored.State.RulesRuntimeSnapshot()
	if !ok || len(runtime.Pending) != 0 || runtime.Receipts[0].Result == nil || len(runtime.RandomDraws) != 1 {
		t.Fatalf("constructor did not recover random checkpoint: ok=%v runtime=%+v", ok, runtime)
	}
	restartedRouter.rules.resolveDice = func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		t.Fatal("restart redrew committed entropy")
		return dnd5e.DiceRandomResponse{}, nil
	}
	observed := restartedRouter.Execute(types.ToolCall{ID: "new-process:observe", Name: "game_observe", Arguments: json.RawMessage(`{}`)})
	if observed.Error != "" {
		t.Fatalf("new process call remained blocked: %+v", observed)
	}
	resolved := restartedRouter.Execute(call)
	if resolved.Error != "" || !strings.Contains(resolved.Content, `"status":"resolved"`) {
		t.Fatalf("resumed result = %+v", resolved)
	}
	runtime, ok = restored.State.RulesRuntimeSnapshot()
	if draws != 1 || !ok || len(runtime.RandomDraws) != 1 || len(runtime.Pending) != 0 || runtime.Receipts[0].Result == nil || restored.State.LogLen() != 1 {
		t.Fatalf("resumed random runtime: draws=%d ok=%v runtime=%+v log=%d", draws, ok, runtime, restored.State.LogLen())
	}
}

func TestRestartAfterEmitCheckpointResumesWithoutReducingTwice(t *testing.T) {
	implementation := &transactionTestRuleset{authority: "host:oracle"}
	session, router := transactionTestSession(t, implementation)
	submit := router.Execute(types.ToolCall{
		ID: "emit-crash-submit", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"test.choose","arguments":{}}`),
	})
	if submit.Error != "" {
		t.Fatal(submit.Error)
	}
	runtime, _ := session.State.RulesRuntimeSnapshot()
	resolutionID := runtime.Pending[0].ResolutionID
	checkpoints := 0
	router.rules.afterCheckpoint = func() error {
		checkpoints++
		if checkpoints == 2 {
			return errors.New("simulated crash after emit")
		}
		return nil
	}
	respond := types.ToolCall{
		ID: "emit-crash-response", Name: "game_respond",
		Arguments: json.RawMessage(`{"resolution_id":"` + resolutionID + `","response":{"option":"advance"}}`),
	}
	interrupted := router.Execute(respond)
	if !strings.Contains(interrupted.Error, "simulated crash after emit") {
		t.Fatalf("interrupted response = %+v", interrupted)
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if !ok || runtime.Revision != 1 || len(runtime.EventBatches) != 1 || len(runtime.Pending) != 1 || runtime.Pending[0].Pending.Kind != rules.StepKindEmit || runtime.Pending[0].Response == nil {
		t.Fatalf("emit checkpoint was not atomic: ok=%v runtime=%+v", ok, runtime)
	}
	if implementation.reduces.Load() != 1 {
		t.Fatalf("reduce calls before restart = %d", implementation.reduces.Load())
	}

	raw, err := json.Marshal(session.State)
	if err != nil {
		t.Fatal(err)
	}
	var restoredState domain.SessionState
	if err := json.Unmarshal(raw, &restoredState); err != nil {
		t.Fatal(err)
	}
	restored := domain.NewSession(&restoredState, session.Adventure, session.Config)
	restored.RulesResolver = exactTestResolver{lock: transactionLock(), implementation: implementation}
	restartedRouter := NewToolRouter(restored)
	if restartedRouter.rulesErr != nil || restartedRouter.rules == nil {
		t.Fatalf("restart gateway=%v error=%v", restartedRouter.rules, restartedRouter.rulesErr)
	}
	runtime, ok = restored.State.RulesRuntimeSnapshot()
	if !ok || len(runtime.Pending) != 0 || runtime.Receipts[len(runtime.Receipts)-1].Result == nil {
		t.Fatalf("constructor did not recover emit checkpoint: ok=%v runtime=%+v", ok, runtime)
	}
	observed := restartedRouter.Execute(types.ToolCall{ID: "new-process:emit-observe", Name: "game_observe", Arguments: json.RawMessage(`{}`)})
	if observed.Error != "" {
		t.Fatalf("new process call remained blocked: %+v", observed)
	}
	resolved := restartedRouter.Execute(respond)
	if resolved.Error != "" || !strings.Contains(resolved.Content, `"status":"resolved"`) {
		t.Fatalf("resumed response = %+v", resolved)
	}
	runtime, ok = restored.State.RulesRuntimeSnapshot()
	// The restart's defensive Replay invokes the pure reducer once to attest the
	// materialized state. Recovery itself must not reduce or append the already
	// committed emission again, so the total is exactly original+replay.
	if !ok || runtime.Revision != 1 || len(runtime.EventBatches) != 1 || len(runtime.Pending) != 0 || implementation.reduces.Load() != 2 {
		t.Fatalf("emit recovery duplicated an effect: ok=%v reduces=%d runtime=%+v", ok, implementation.reduces.Load(), runtime)
	}
}

func TestPersistenceBarrierRunsBeforeResumeAndBeforeReturn(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	router.rules.resolveDice = func(dnd5e.DiceRandomRequest) (dnd5e.DiceRandomResponse, error) {
		return dnd5e.DiceRandomResponse{Rolls: []int{7}}, nil
	}
	var checkpoints []string
	session.PersistRules = func(state *domain.SessionState) error {
		runtime, ok := state.RulesRuntimeSnapshot()
		if !ok {
			return errors.New("invalid runtime at persistence barrier")
		}
		phase := "terminal"
		if len(runtime.Pending) == 1 && runtime.Pending[0].Response != nil && runtime.Receipts[0].Result == nil {
			phase = "random-checkpoint"
		}
		checkpoints = append(checkpoints, phase)
		return nil
	}
	result := router.Execute(types.ToolCall{
		ID: "persistence-order", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"1d20"}}`),
	})
	if result.Error != "" {
		t.Fatal(result.Error)
	}
	if len(checkpoints) != 2 || checkpoints[0] != "random-checkpoint" || checkpoints[1] != "terminal" {
		t.Fatalf("persistence barriers = %v", checkpoints)
	}
}

func TestTerminalReceiptRetryMustRecoverPersistenceBeforeReturningCachedSuccess(t *testing.T) {
	session := createTestSession()
	router := NewToolRouter(session)
	starts := 0
	router.rules.ruleset = startOverrideRuleset{
		Ruleset: router.rules.ruleset,
		start: func(_ context.Context, request rules.StartRequest) (rules.Step, error) {
			starts++
			return rules.Step{
				ID: request.Intent.ID + ":complete", Kind: rules.StepKindComplete,
				Complete: &rules.Completion{Outcome: "test.complete", Result: transactionPayload(t, map[string]any{"value": 1})},
			}, nil
		},
	}
	persistenceAvailable := false
	var persisted []byte
	session.PersistRules = func(state *domain.SessionState) error {
		if !persistenceAvailable {
			return errors.New("disk unavailable")
		}
		var err error
		persisted, err = json.Marshal(state)
		return err
	}
	call := types.ToolCall{
		ID: "terminal-persist-retry", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"dice.roll","arguments":{"notation":"1d20"}}`),
	}
	first := router.Execute(call)
	if !strings.Contains(first.Error, "disk unavailable") || starts != 1 {
		t.Fatalf("first=%+v starts=%d", first, starts)
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if !ok || len(runtime.Receipts) != 1 || runtime.Receipts[0].Result == nil || len(runtime.Pending) != 0 {
		t.Fatalf("terminal receipt not retained after persistence failure: ok=%v runtime=%+v", ok, runtime)
	}
	second := router.Execute(call)
	if !strings.Contains(second.Error, "disk unavailable") || starts != 1 {
		t.Fatalf("failed retry=%+v starts=%d", second, starts)
	}
	persistenceAvailable = true
	recovered := router.Execute(call)
	if recovered.Error != "" || !strings.Contains(recovered.Content, `"status":"resolved"`) || starts != 1 || len(persisted) == 0 {
		t.Fatalf("recovered=%+v starts=%d persisted=%d", recovered, starts, len(persisted))
	}
	var restoredState domain.SessionState
	if err := json.Unmarshal(persisted, &restoredState); err != nil {
		t.Fatal(err)
	}
	restored := domain.NewSession(&restoredState, session.Adventure, session.Config)
	restored.RulesResolver = session.RulesResolver
	restartedRouter := NewToolRouter(restored)
	if retry := restartedRouter.Execute(call); retry != recovered {
		t.Fatalf("restart retry=%+v recovered=%+v", retry, recovered)
	}
}

func TestPendingReceiptRetryPersistsBeforeReturningAndRemainsObservable(t *testing.T) {
	implementation := &transactionTestRuleset{authority: "host:oracle"}
	session, router := transactionTestSession(t, implementation)
	persistenceAvailable := false
	var persisted []byte
	session.PersistRules = func(state *domain.SessionState) error {
		if !persistenceAvailable {
			return errors.New("disk unavailable")
		}
		var err error
		persisted, err = json.Marshal(state)
		return err
	}
	call := types.ToolCall{
		ID: "pending-persist-retry", Name: "game_submit_intent",
		Arguments: json.RawMessage(`{"action_id":"test.choose","arguments":{}}`),
	}
	if first := router.Execute(call); !strings.Contains(first.Error, "disk unavailable") {
		t.Fatalf("first = %+v", first)
	}
	runtime, ok := session.State.RulesRuntimeSnapshot()
	if !ok || len(runtime.Pending) != 1 || runtime.Pending[0].Response != nil || runtime.Receipts[0].Result == nil {
		t.Fatalf("pending was not retained: ok=%v runtime=%+v", ok, runtime)
	}
	if retry := router.Execute(call); !strings.Contains(retry.Error, "disk unavailable") || implementation.starts.Load() != 1 {
		t.Fatalf("failed retry=%+v starts=%d", retry, implementation.starts.Load())
	}
	persistenceAvailable = true
	recovered := router.Execute(call)
	if recovered.Error != "" || !strings.Contains(recovered.Content, `"status":"needs_input"`) || len(persisted) == 0 {
		t.Fatalf("recovered pending = %+v persisted=%d", recovered, len(persisted))
	}
	var restoredState domain.SessionState
	if err := json.Unmarshal(persisted, &restoredState); err != nil {
		t.Fatal(err)
	}
	restored := domain.NewSession(&restoredState, session.Adventure, session.Config)
	restartedRouter := &ToolRouter{
		session: restored,
		rules:   newRulesGatewayWithRuleset(restored, transactionLock(), implementation),
	}
	observed := restartedRouter.Execute(types.ToolCall{ID: "observe-persisted-pending", Name: "game_observe", Arguments: json.RawMessage(`{}`)})
	if observed.Error != "" || !strings.Contains(observed.Content, `"resolution_id":"pending-persist-retry"`) {
		t.Fatalf("observe persisted pending = %+v", observed)
	}
}

func restoreDNDTransactionRouter(t *testing.T, source *domain.Session) (*domain.Session, *ToolRouter) {
	t.Helper()
	raw, err := json.Marshal(source.State)
	if err != nil {
		t.Fatal(err)
	}
	var restoredState domain.SessionState
	if err := json.Unmarshal(raw, &restoredState); err != nil {
		t.Fatal(err)
	}
	restored := domain.NewSession(&restoredState, source.Adventure, source.Config)
	restored.RulesResolver = source.RulesResolver
	return restored, NewToolRouter(restored)
}
