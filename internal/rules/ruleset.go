package rules

import "context"

// Ruleset is a deterministic state transition function exposed as a small,
// context-aware protocol.
//
// Implementations must treat request values as immutable and must derive their
// result solely from explicit inputs. Context is for cancellation and deadlines,
// not for hidden rules input. Implementations must express entropy, I/O, human
// choices, adjudication, nested resolutions, and mutations as Step values.
type Ruleset interface {
	Manifest(ctx context.Context) (Manifest, error)
	ListActions(ctx context.Context, request CatalogRequest) ([]ActionDescriptor, error)
	Start(ctx context.Context, request StartRequest) (Step, error)
	Resume(ctx context.Context, request ResumeRequest) (Step, error)
	Project(ctx context.Context, request ProjectRequest) (Projection, error)
	Explain(ctx context.Context, request ExplainRequest) (Explanation, error)
	ValidateState(ctx context.Context, request ValidateStateRequest) error
	Reduce(ctx context.Context, request ReduceRequest) (ReduceResult, error)
	Migrate(ctx context.Context, request MigrateRequest) (MigrateResult, error)
}
