package rules

// Resolver is the host-owned package catalog boundary used by a live session.
// Requirements are resolved only when a session is first bound; persisted
// sessions always use Lookup with their complete immutable lock.
//
// Implementations must be safe for concurrent reads. Resolution must never
// silently substitute a different package ID or ignore a malformed constraint.
type Resolver interface {
	Lookup(lock Lock) (Ruleset, error)
	Resolve(requirement Requirement) (Lock, Ruleset, error)
	InitialState(lock Lock) (Payload, error)
}
