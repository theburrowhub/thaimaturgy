package domain

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/theburrowhub/thaimaturgy/internal/rules"
)

const (
	// MaxRulesReceipts bounds durable idempotency metadata without ever forgetting
	// an accepted request ID. At the limit, new mutations fail closed until an
	// explicit archival/checkpoint policy is introduced.
	MaxRulesReceipts = 4096
	// MaxRulesReceiptBytes prevents individually valid results from expanding a
	// loaded session into multi-gigabyte receipt metadata.
	MaxRulesReceiptBytes = 64 << 20
	// MaxRulesPending bounds unresolved continuations without silently evicting
	// one that still needs an authority response.
	MaxRulesPending     = 64
	maxRulesResultBytes = rules.MaxPayloadBytes
	// MaxRulesCommitBytes caps the aggregate host-controlled data added by one
	// transaction. Per-payload limits alone would still permit a multi-gigabyte
	// event batch because a collection may contain hundreds of payloads.
	MaxRulesCommitBytes = 8 << 20
)

var (
	ErrRulesReceiptConflict    = errors.New("domain: rules request ID conflict")
	ErrRulesRevisionConflict   = errors.New("domain: rules revision conflict")
	ErrRulesGenerationConflict = errors.New("domain: rules generation conflict")
	ErrRulesPendingNotFound    = errors.New("domain: pending rules resolution not found")
	ErrRulesRequestInactive    = errors.New("domain: rules request is not active")
	ErrRulesReceiptLimit       = errors.New("domain: durable rules receipt limit reached")
)

// RulesStoredResult is the exact tool result retained for an idempotent retry.
// It deliberately excludes the call ID because that is the receipt key.
type RulesStoredResult struct {
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (r RulesStoredResult) validate() error {
	if len(r.Content) > maxRulesResultBytes || len(r.Error) > maxRulesResultBytes {
		return fmt.Errorf("rules stored result exceeds %d bytes", maxRulesResultBytes)
	}
	if !utf8.ValidString(r.Content) || !utf8.ValidString(r.Error) {
		return errors.New("rules stored result is not valid UTF-8")
	}
	if r.Content == "" && r.Error == "" {
		return errors.New("rules stored result is empty")
	}
	return nil
}

// RulesReceipt makes a host request idempotent across routers and restarts.
type RulesReceipt struct {
	RequestID    string             `json:"request_id"`
	Tool         string             `json:"tool"`
	Fingerprint  string             `json:"fingerprint"`
	ResolutionID string             `json:"resolution_id"`
	Result       *RulesStoredResult `json:"result,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

func (r RulesReceipt) validate() error {
	if err := validateRulesRequestID(r.RequestID); err != nil {
		return err
	}
	if err := validateRulesOpaque("receipt.tool", r.Tool); err != nil {
		return err
	}
	if err := validateRulesFingerprint(r.Fingerprint); err != nil {
		return err
	}
	if err := validateRulesOpaque("receipt.resolution_id", r.ResolutionID); err != nil {
		return err
	}
	if r.Result != nil {
		if err := r.Result.validate(); err != nil {
			return err
		}
	}
	if r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) {
		return errors.New("rules receipt timestamp is missing")
	}
	return nil
}

// RulesPendingResolution is all state required to resume a rules step after a
// process restart. Request is the public request shown to the responding
// authority; Principal is the initiating principal retained for audit, while
// Pending.State is the opaque ruleset continuation.
type RulesPendingResolution struct {
	ResolutionID string              `json:"resolution_id"`
	RequestID    string              `json:"request_id"`
	Revision     uint64              `json:"revision"`
	Principal    rules.Principal     `json:"principal"`
	Pending      rules.PendingStep   `json:"pending"`
	Request      rules.Payload       `json:"request"`
	Response     *rules.HostResponse `json:"response,omitempty"`
	StepCount    uint32              `json:"step_count"`
	CreatedAt    time.Time           `json:"created_at"`
}

func (p RulesPendingResolution) validate() error {
	if err := validateRulesOpaque("pending.resolution_id", p.ResolutionID); err != nil {
		return err
	}
	if err := validateRulesRequestID(p.RequestID); err != nil {
		return err
	}
	if err := p.Principal.Validate(); err != nil {
		return fmt.Errorf("pending principal: %w", err)
	}
	if err := p.Pending.Validate(); err != nil {
		return fmt.Errorf("pending step: %w", err)
	}
	if err := p.Request.Validate(); err != nil {
		return fmt.Errorf("pending request: %w", err)
	}
	if p.Response != nil {
		if err := p.Response.Validate(); err != nil {
			return fmt.Errorf("pending response: %w", err)
		}
		if p.Response.StepID != p.Pending.StepID || p.Response.Kind != p.Pending.Kind {
			return errors.New("pending response does not match its step")
		}
	}
	if p.StepCount == 0 || p.StepCount > rules.MaxCollectionItems {
		return fmt.Errorf("pending rules step count must be between 1 and %d", rules.MaxCollectionItems)
	}
	if p.CreatedAt.IsZero() {
		return errors.New("pending rules resolution timestamp is missing")
	}
	return nil
}

// RulesEventBatch is one atomically reduced, ordered rules event batch.
type RulesEventBatch struct {
	ResolutionID string          `json:"resolution_id"`
	RequestID    string          `json:"request_id"`
	Ruleset      rules.Lock      `json:"ruleset"`
	Principal    rules.Principal `json:"principal"`
	Sequence     uint32          `json:"sequence"`
	BaseRevision uint64          `json:"base_revision"`
	Revision     uint64          `json:"revision"`
	Events       []rules.Event   `json:"events"`
	CreatedAt    time.Time       `json:"created_at"`
}

func (b RulesEventBatch) validate() error {
	if err := validateRulesOpaque("event_batch.resolution_id", b.ResolutionID); err != nil {
		return err
	}
	if err := validateRulesRequestID(b.RequestID); err != nil {
		return err
	}
	if err := b.Ruleset.Validate(); err != nil {
		return fmt.Errorf("rules event batch lock: %w", err)
	}
	if err := b.Principal.Validate(); err != nil {
		return fmt.Errorf("rules event batch principal: %w", err)
	}
	if b.Sequence == 0 {
		return errors.New("rules event batch sequence must be greater than zero")
	}
	if b.Revision != b.BaseRevision+1 {
		return errors.New("rules event batch must advance exactly one revision")
	}
	if len(b.Events) == 0 || len(b.Events) > rules.MaxCollectionItems {
		return fmt.Errorf("rules event batch must contain 1..%d events", rules.MaxCollectionItems)
	}
	for i, event := range b.Events {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("rules event batch event %d: %w", i, err)
		}
	}
	if b.CreatedAt.IsZero() {
		return errors.New("rules event batch timestamp is missing")
	}
	return nil
}

// RulesRandomDraw records both sides of an entropy request without assuming a
// dice model. Specification and Result remain package-owned JSON.
type RulesRandomDraw struct {
	ResolutionID  string          `json:"resolution_id"`
	RequestID     string          `json:"request_id"`
	Ruleset       rules.Lock      `json:"ruleset"`
	Principal     rules.Principal `json:"principal"`
	StepID        string          `json:"step_id"`
	Sequence      uint32          `json:"sequence"`
	Method        string          `json:"method"`
	Source        string          `json:"source"`
	Specification rules.Payload   `json:"specification"`
	Result        rules.Payload   `json:"result"`
	CreatedAt     time.Time       `json:"created_at"`
}

func (d RulesRandomDraw) validate() error {
	for field, value := range map[string]string{
		"random_draw.resolution_id": d.ResolutionID,
		"random_draw.step_id":       d.StepID,
		"random_draw.method":        d.Method,
		"random_draw.source":        d.Source,
	} {
		if err := validateRulesOpaque(field, value); err != nil {
			return err
		}
	}
	if err := validateRulesRequestID(d.RequestID); err != nil {
		return err
	}
	if err := d.Ruleset.Validate(); err != nil {
		return fmt.Errorf("rules random draw lock: %w", err)
	}
	if err := d.Principal.Validate(); err != nil {
		return fmt.Errorf("rules random draw principal: %w", err)
	}
	if d.Sequence == 0 {
		return errors.New("rules random draw sequence must be greater than zero")
	}
	if err := d.Specification.Validate(); err != nil {
		return fmt.Errorf("random draw specification: %w", err)
	}
	if err := d.Result.Validate(); err != nil {
		return fmt.Errorf("random draw result: %w", err)
	}
	if d.CreatedAt.IsZero() {
		return errors.New("rules random draw timestamp is missing")
	}
	return nil
}

func (r RulesSession) validateRuntime() error {
	if len(r.Receipts) > MaxRulesReceipts {
		return fmt.Errorf("rules receipts exceed %d", MaxRulesReceipts)
	}
	if len(r.Pending) > MaxRulesPending {
		return fmt.Errorf("pending rules resolutions exceed %d", MaxRulesPending)
	}
	receipts := make(map[string]struct{}, len(r.Receipts))
	receiptBytes := int64(0)
	for i, receipt := range r.Receipts {
		if err := receipt.validate(); err != nil {
			return fmt.Errorf("rules receipt %d: %w", i, err)
		}
		if _, exists := receipts[receipt.RequestID]; exists {
			return fmt.Errorf("duplicate rules receipt %q", receipt.RequestID)
		}
		receipts[receipt.RequestID] = struct{}{}
		receiptBytes += int64(len(receipt.RequestID) + len(receipt.Tool) + len(receipt.Fingerprint) + len(receipt.ResolutionID))
		if receipt.Result != nil {
			receiptBytes += int64(len(receipt.Result.Content) + len(receipt.Result.Error))
		}
		if receiptBytes > MaxRulesReceiptBytes {
			return fmt.Errorf("rules receipts exceed %d aggregate bytes", MaxRulesReceiptBytes)
		}
	}
	pendings := make(map[string]struct{}, len(r.Pending))
	for i, pending := range r.Pending {
		if err := pending.validate(); err != nil {
			return fmt.Errorf("pending rules resolution %d: %w", i, err)
		}
		if pending.Revision > r.Revision {
			return fmt.Errorf("pending rules resolution %q references future revision", pending.ResolutionID)
		}
		if _, exists := receipts[pending.RequestID]; !exists {
			return fmt.Errorf("pending rules resolution %q has no retained initiating receipt", pending.ResolutionID)
		}
		if _, exists := pendings[pending.ResolutionID]; exists {
			return fmt.Errorf("duplicate pending rules resolution %q", pending.ResolutionID)
		}
		pendings[pending.ResolutionID] = struct{}{}
	}
	for _, receipt := range r.Receipts {
		if receipt.Result == nil {
			found := false
			for _, pending := range r.Pending {
				if pending.RequestID == receipt.RequestID && pending.ResolutionID == receipt.ResolutionID && pending.Response != nil {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("incomplete rules receipt %q has no resumable checkpoint", receipt.RequestID)
			}
		}
	}
	for i, batch := range r.EventBatches {
		if err := batch.validate(); err != nil {
			return fmt.Errorf("rules event batch %d: %w", i, err)
		}
		if batch.Revision > r.Revision {
			return fmt.Errorf("rules event batch %d references future revision", i)
		}
		if batch.Ruleset != r.Lock {
			return fmt.Errorf("rules event batch %d has a different rules lock", i)
		}
		if i > 0 && batch.BaseRevision != r.EventBatches[i-1].Revision {
			return fmt.Errorf("rules event batch %d is not revision-contiguous", i)
		}
	}
	if len(r.EventBatches) > 0 {
		if r.EventBatches[0].BaseRevision != 0 {
			return errors.New("rules event history does not start at revision zero")
		}
		if r.EventBatches[len(r.EventBatches)-1].Revision != r.Revision {
			return errors.New("rules event history does not reach the materialized revision")
		}
	} else {
		if r.Revision != 0 {
			return errors.New("rules session without event history must be at revision zero")
		}
		if r.InitialState.String() != r.State.String() {
			return errors.New("rules session without event history differs from its initial state")
		}
	}
	for i, batch := range r.EventBatches {
		expected := uint32(i + 1)
		if batch.Sequence != expected {
			return fmt.Errorf("rules event batch %d has sequence %d, expected %d", i, batch.Sequence, expected)
		}
	}
	drawSequences := make(map[string]uint32)
	for i, draw := range r.RandomDraws {
		if err := draw.validate(); err != nil {
			return fmt.Errorf("rules random draw %d: %w", i, err)
		}
		if draw.Ruleset != r.Lock {
			return fmt.Errorf("rules random draw %d has a different rules lock", i)
		}
		expected := drawSequences[draw.ResolutionID] + 1
		if draw.Sequence != expected {
			return fmt.Errorf("rules random draw %d has sequence %d, expected %d", i, draw.Sequence, expected)
		}
		drawSequences[draw.ResolutionID] = draw.Sequence
	}
	return nil
}

type rulesRequestFlight struct {
	tool        string
	fingerprint string
	claim       uint64
	done        chan struct{}
}

// RulesRequestHandle is an optimistic snapshot plus a runtime-only ownership
// token. Callers must CommitRulesRequest or AbortRulesRequest exactly once.
type RulesRequestHandle struct {
	Snapshot rules.Snapshot
	Pending  []RulesPendingResolution

	requestID   string
	tool        string
	fingerprint string
	claim       uint64
	owner       *SessionState
	generation  uint64
}

// RulesEventDraft is a reduced event batch awaiting atomic persistence.
type RulesEventDraft struct {
	ResolutionID string
	Events       []rules.Event
}

// RulesRandomDraft is a completed entropy exchange awaiting persistence.
type RulesRandomDraft struct {
	ResolutionID  string
	StepID        string
	Method        string
	Source        string
	Specification rules.Payload
	Result        rules.Payload
}

// RulesCommit is the complete all-or-nothing mutation for one tool request.
// State must be the result of reducing EventBatches in order. Without an event
// batch it must exactly equal the handle snapshot, preventing unaudited state
// changes.
type RulesCommit struct {
	State           rules.Payload
	Principal       rules.Principal
	ResolutionID    string
	EventBatches    []RulesEventDraft
	RandomDraws     []RulesRandomDraft
	Pending         *RulesPendingResolution
	RemovePendingID string
	Result          *RulesStoredResult
	LogEntries      []LogEntry
}

// BeginRulesRequest returns a durable cached receipt or claims this request ID.
// Concurrent callers using the same ID and fingerprint wait for the owner, so
// entropy and effects execute once even when two routers share the session.
func (s *SessionState) BeginRulesRequest(ctx context.Context, requestID, tool, fingerprint string) (RulesRequestHandle, *RulesReceipt, error) {
	if ctx == nil {
		return RulesRequestHandle{}, nil, errors.New("nil rules request context")
	}
	if err := validateRulesRequestID(requestID); err != nil {
		return RulesRequestHandle{}, nil, err
	}
	if err := validateRulesOpaque("tool", tool); err != nil {
		return RulesRequestHandle{}, nil, err
	}
	if err := validateRulesFingerprint(fingerprint); err != nil {
		return RulesRequestHandle{}, nil, err
	}

	for {
		s.mu.Lock()
		s.ensureRulesRuntimeLocked()
		if s.Rules == nil {
			s.mu.Unlock()
			return RulesRequestHandle{}, nil, errors.New("session has no rules binding")
		}
		if err := s.Rules.Validate(); err != nil {
			s.mu.Unlock()
			return RulesRequestHandle{}, nil, err
		}
		receipt := findRulesReceipt(s.Rules.Receipts, requestID)
		if receipt != nil {
			if receipt.Tool != tool || receipt.Fingerprint != fingerprint {
				s.mu.Unlock()
				return RulesRequestHandle{}, nil, fmt.Errorf("%w: %q was already used with different tool arguments", ErrRulesReceiptConflict, requestID)
			}
			if receipt.Result != nil {
				copy := cloneRulesReceipt(*receipt)
				s.mu.Unlock()
				return RulesRequestHandle{}, &copy, nil
			}
		} else if len(s.Rules.Receipts) >= MaxRulesReceipts {
			s.mu.Unlock()
			return RulesRequestHandle{}, nil, fmt.Errorf("%w: retry a retained request or create an explicit checkpoint before accepting new mutations", ErrRulesReceiptLimit)
		}
		if flight := s.rulesInFlight[requestID]; flight != nil {
			if flight.tool != tool || flight.fingerprint != fingerprint {
				s.mu.Unlock()
				return RulesRequestHandle{}, nil, fmt.Errorf("%w: %q is active with different tool arguments", ErrRulesReceiptConflict, requestID)
			}
			done := flight.done
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				return RulesRequestHandle{}, nil, ctx.Err()
			case <-done:
				continue
			}
		}

		s.rulesClaimSeq++
		claim := s.rulesClaimSeq
		s.rulesInFlight[requestID] = &rulesRequestFlight{
			tool: tool, fingerprint: fingerprint, claim: claim, done: make(chan struct{}),
		}
		handle := RulesRequestHandle{
			Snapshot:  rules.Snapshot{Ruleset: s.Rules.Lock, Revision: s.Rules.Revision, State: s.Rules.State},
			Pending:   cloneRulesPending(s.Rules.Pending),
			requestID: requestID, tool: tool, fingerprint: fingerprint, claim: claim, owner: s,
			generation: s.Rules.Generation,
		}
		s.mu.Unlock()
		return handle, nil, nil
	}
}

// AbortRulesRequest releases an unfinished runtime claim without changing any
// persisted state. It is safe to call after CommitRulesRequest.
func (s *SessionState) AbortRulesRequest(handle RulesRequestHandle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finishRulesFlightLocked(handle)
}

// CommitRulesRequest compares the current revision with the claimed snapshot,
// validates a complete candidate first, then swaps all rules state/audit data,
// pending continuations, receipt, and legacy log entries under one mutex.
func (s *SessionState) CommitRulesRequest(handle RulesRequestHandle, commit RulesCommit) (RulesReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.finishRulesFlightLocked(handle)

	flight := s.rulesInFlight[handle.requestID]
	if handle.owner != s || flight == nil || flight.claim != handle.claim || flight.tool != handle.tool || flight.fingerprint != handle.fingerprint {
		return RulesReceipt{}, ErrRulesRequestInactive
	}
	if s.Rules == nil || s.Rules.Lock != handle.Snapshot.Ruleset {
		return RulesReceipt{}, ErrRulesLockConflict
	}
	if s.Rules.Revision != handle.Snapshot.Revision {
		return RulesReceipt{}, fmt.Errorf("%w: expected %d, found %d", ErrRulesRevisionConflict, handle.Snapshot.Revision, s.Rules.Revision)
	}
	if s.Rules.Generation != handle.generation {
		return RulesReceipt{}, fmt.Errorf("%w: expected generation %d, found %d", ErrRulesGenerationConflict, handle.generation, s.Rules.Generation)
	}
	if err := validateRulesOpaque("commit.resolution_id", commit.ResolutionID); err != nil {
		return RulesReceipt{}, err
	}
	if commit.Result != nil {
		if err := commit.Result.validate(); err != nil {
			return RulesReceipt{}, err
		}
	}
	if err := commit.State.Validate(); err != nil {
		return RulesReceipt{}, fmt.Errorf("rules commit state: %w", err)
	}
	if len(commit.EventBatches) == 0 && commit.State.String() != handle.Snapshot.State.String() {
		return RulesReceipt{}, errors.New("rules commit changed state without an event batch")
	}
	if len(commit.EventBatches) > rules.MaxCollectionItems || len(commit.RandomDraws) > rules.MaxCollectionItems || len(commit.LogEntries) > rules.MaxCollectionItems {
		return RulesReceipt{}, fmt.Errorf("rules commit contains more than %d items in one collection", rules.MaxCollectionItems)
	}
	if size := rulesCommitBytes(commit); size > MaxRulesCommitBytes {
		return RulesReceipt{}, fmt.Errorf("rules commit contains %d bytes, exceeds %d", size, MaxRulesCommitBytes)
	}

	now := time.Now().UTC()
	if len(commit.EventBatches) > 0 || len(commit.RandomDraws) > 0 {
		if err := commit.Principal.Validate(); err != nil {
			return RulesReceipt{}, fmt.Errorf("rules commit principal: %w", err)
		}
	}
	candidate := cloneRulesSession(*s.Rules)
	candidate.State = commit.State
	baseRevision := candidate.Revision
	for i, draft := range commit.EventBatches {
		batch := RulesEventBatch{
			ResolutionID: draft.ResolutionID, RequestID: handle.requestID,
			Ruleset: handle.Snapshot.Ruleset, Principal: cloneRulesPrincipal(commit.Principal),
			Sequence:     uint32(len(candidate.EventBatches) + 1),
			BaseRevision: baseRevision + uint64(i),
			Revision:     baseRevision + uint64(i) + 1,
			Events:       cloneRulesEvents(draft.Events),
			CreatedAt:    now,
		}
		candidate.EventBatches = append(candidate.EventBatches, batch)
	}
	candidate.Revision = baseRevision + uint64(len(commit.EventBatches))
	drawSequence := nextRulesDrawSequence(candidate.RandomDraws, commit.RandomDraws)
	for _, draft := range commit.RandomDraws {
		candidate.RandomDraws = append(candidate.RandomDraws, RulesRandomDraw{
			ResolutionID: draft.ResolutionID, RequestID: handle.requestID,
			Ruleset: handle.Snapshot.Ruleset, Principal: cloneRulesPrincipal(commit.Principal),
			StepID:   draft.StepID,
			Sequence: drawSequence[draft.ResolutionID], Method: draft.Method, Source: draft.Source,
			Specification: draft.Specification, Result: draft.Result, CreatedAt: now,
		})
		drawSequence[draft.ResolutionID]++
	}
	if commit.RemovePendingID != "" {
		index := findRulesPending(candidate.Pending, commit.RemovePendingID)
		if index < 0 {
			return RulesReceipt{}, fmt.Errorf("%w: %s", ErrRulesPendingNotFound, commit.RemovePendingID)
		}
		candidate.Pending = append(candidate.Pending[:index], candidate.Pending[index+1:]...)
	}
	if commit.Pending != nil {
		pending := cloneRulesPendingResolution(*commit.Pending)
		pending.Revision = candidate.Revision
		if pending.CreatedAt.IsZero() {
			pending.CreatedAt = now
		}
		if index := findRulesPending(candidate.Pending, pending.ResolutionID); index >= 0 {
			candidate.Pending[index] = pending
		} else {
			if len(candidate.Pending) >= MaxRulesPending {
				return RulesReceipt{}, fmt.Errorf("pending rules resolutions reached limit %d", MaxRulesPending)
			}
			candidate.Pending = append(candidate.Pending, pending)
		}
	}
	receiptIndex := findRulesReceiptIndex(candidate.Receipts, handle.requestID)
	var receipt RulesReceipt
	if receiptIndex >= 0 {
		receipt = candidate.Receipts[receiptIndex]
		if receipt.Tool != handle.tool || receipt.Fingerprint != handle.fingerprint || receipt.ResolutionID != commit.ResolutionID {
			return RulesReceipt{}, ErrRulesReceiptConflict
		}
		receipt.Result = cloneRulesStoredResult(commit.Result)
		receipt.UpdatedAt = now
		candidate.Receipts[receiptIndex] = receipt
	} else {
		receipt = RulesReceipt{
			RequestID: handle.requestID, Tool: handle.tool, Fingerprint: handle.fingerprint,
			ResolutionID: commit.ResolutionID, Result: cloneRulesStoredResult(commit.Result),
			CreatedAt: now, UpdatedAt: now,
		}
	}
	if receiptIndex < 0 {
		if len(candidate.Receipts) >= MaxRulesReceipts {
			return RulesReceipt{}, ErrRulesReceiptLimit
		}
		candidate.Receipts = append(candidate.Receipts, receipt)
	}
	candidate.Generation++
	if err := candidate.Validate(); err != nil {
		return RulesReceipt{}, fmt.Errorf("invalid rules commit: %w", err)
	}
	for _, entry := range commit.LogEntries {
		if !utf8.ValidString(entry.Message) {
			return RulesReceipt{}, errors.New("rules log message is not valid UTF-8")
		}
	}

	*s.Rules = candidate
	for _, entry := range commit.LogEntries {
		s.record(entry)
	}
	s.touch()
	return receipt, nil
}

// RulesRuntimeSnapshot returns a deep copy of the full persisted host state.
func (s *SessionState) RulesRuntimeSnapshot() (RulesSession, bool) {
	runtime, exists, err := s.RulesRuntimeSnapshotStrict()
	return runtime, exists && err == nil
}

// RulesRuntimeSnapshotStrict distinguishes an absent legacy binding from a
// present but invalid transactional block.
func (s *SessionState) RulesRuntimeSnapshotStrict() (RulesSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Rules == nil {
		return RulesSession{}, false, nil
	}
	if err := s.Rules.Validate(); err != nil {
		return RulesSession{}, true, err
	}
	return cloneRulesSession(*s.Rules), true, nil
}

func (s *SessionState) ensureRulesRuntimeLocked() {
	if s.rulesInFlight == nil {
		s.rulesInFlight = make(map[string]*rulesRequestFlight)
	}
}

func (s *SessionState) finishRulesFlightLocked(handle RulesRequestHandle) {
	if handle.owner != s {
		return
	}
	flight := s.rulesInFlight[handle.requestID]
	if flight == nil || flight.claim != handle.claim {
		return
	}
	delete(s.rulesInFlight, handle.requestID)
	close(flight.done)
}

func findRulesReceipt(receipts []RulesReceipt, requestID string) *RulesReceipt {
	for i := len(receipts) - 1; i >= 0; i-- {
		if receipts[i].RequestID == requestID {
			return &receipts[i]
		}
	}
	return nil
}

func findRulesReceiptIndex(receipts []RulesReceipt, requestID string) int {
	for i := len(receipts) - 1; i >= 0; i-- {
		if receipts[i].RequestID == requestID {
			return i
		}
	}
	return -1
}

func findRulesPending(pending []RulesPendingResolution, resolutionID string) int {
	for i := range pending {
		if pending[i].ResolutionID == resolutionID {
			return i
		}
	}
	return -1
}

func cloneRulesSession(source RulesSession) RulesSession {
	copy := source
	copy.Receipts = make([]RulesReceipt, len(source.Receipts))
	for i, receipt := range source.Receipts {
		copy.Receipts[i] = cloneRulesReceipt(receipt)
	}
	copy.Pending = cloneRulesPending(source.Pending)
	copy.EventBatches = make([]RulesEventBatch, len(source.EventBatches))
	for i, batch := range source.EventBatches {
		copy.EventBatches[i] = batch
		copy.EventBatches[i].Principal = cloneRulesPrincipal(batch.Principal)
		copy.EventBatches[i].Events = cloneRulesEvents(batch.Events)
	}
	copy.RandomDraws = make([]RulesRandomDraw, len(source.RandomDraws))
	for i, draw := range source.RandomDraws {
		copy.RandomDraws[i] = draw
		copy.RandomDraws[i].Principal = cloneRulesPrincipal(draw.Principal)
	}
	return copy
}

func cloneRulesPending(source []RulesPendingResolution) []RulesPendingResolution {
	copy := make([]RulesPendingResolution, len(source))
	for i, pending := range source {
		copy[i] = cloneRulesPendingResolution(pending)
	}
	return copy
}

func cloneRulesPendingResolution(source RulesPendingResolution) RulesPendingResolution {
	copy := source
	copy.Principal = cloneRulesPrincipal(source.Principal)
	if source.Response != nil {
		response := *source.Response
		copy.Response = &response
	}
	return copy
}

func cloneRulesReceipt(source RulesReceipt) RulesReceipt {
	copy := source
	copy.Result = cloneRulesStoredResult(source.Result)
	return copy
}

func cloneRulesStoredResult(source *RulesStoredResult) *RulesStoredResult {
	if source == nil {
		return nil
	}
	copy := *source
	return &copy
}

func cloneRulesPrincipal(source rules.Principal) rules.Principal {
	copy := source
	copy.Roles = append([]string(nil), source.Roles...)
	return copy
}

func cloneRulesEvents(source []rules.Event) []rules.Event {
	return append([]rules.Event(nil), source...)
}

func nextRulesDrawSequence(existing []RulesRandomDraw, drafts []RulesRandomDraft) map[string]uint32 {
	next := make(map[string]uint32)
	for _, draw := range existing {
		if draw.Sequence >= next[draw.ResolutionID] {
			next[draw.ResolutionID] = draw.Sequence + 1
		}
	}
	for _, draft := range drafts {
		if next[draft.ResolutionID] == 0 {
			next[draft.ResolutionID] = 1
		}
	}
	return next
}

func rulesCommitBytes(commit RulesCommit) int64 {
	size := int64(len(commit.State.String()) + len(commit.ResolutionID) + len(commit.RemovePendingID))
	if commit.Result != nil {
		size += int64(len(commit.Result.Content) + len(commit.Result.Error))
	}
	for _, batch := range commit.EventBatches {
		size += int64(len(batch.ResolutionID))
		for _, event := range batch.Events {
			size += int64(len(event.Type) + len(event.Data.String()))
		}
	}
	for _, draw := range commit.RandomDraws {
		size += int64(len(draw.ResolutionID) + len(draw.StepID) + len(draw.Method) + len(draw.Source))
		size += int64(len(draw.Specification.String()) + len(draw.Result.String()))
	}
	if commit.Pending != nil {
		pending := commit.Pending
		size += int64(len(pending.ResolutionID) + len(pending.RequestID) + len(pending.Pending.StepID))
		size += int64(len(pending.Pending.State.String()) + len(pending.Request.String()))
		if pending.Response != nil {
			size += int64(len(pending.Response.StepID) + len(pending.Response.Data.String()))
		}
	}
	for _, entry := range commit.LogEntries {
		size += int64(len(entry.Message))
	}
	return size
}

func validateRulesFingerprint(value string) error {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+64 {
		return errors.New("rules fingerprint must use sha256:<64 lowercase hex>")
	}
	hexPart := strings.TrimPrefix(value, prefix)
	if strings.ToLower(hexPart) != hexPart {
		return errors.New("rules fingerprint must use sha256:<64 lowercase hex>")
	}
	if _, err := hex.DecodeString(hexPart); err != nil {
		return errors.New("rules fingerprint must use sha256:<64 lowercase hex>")
	}
	return nil
}

func validateRulesOpaque(field, value string) error {
	if value == "" || len(value) > rules.MaxIdentifierBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("%s must be a non-empty opaque ID of at most %d bytes", field, rules.MaxIdentifierBytes)
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return fmt.Errorf("%s must not contain whitespace or control characters", field)
		}
	}
	return nil
}

func validateRulesRequestID(value string) error {
	if value == "" || len(value) > rules.MaxIdentifierBytes || !utf8.ValidString(value) {
		return fmt.Errorf("request_id must be non-empty and at most %d bytes", rules.MaxIdentifierBytes)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return errors.New("request_id must not contain control characters")
		}
	}
	return nil
}
