package dnd5e

import (
	"fmt"

	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

var emptyState = func() core.Payload {
	state, err := core.NewPayload([]byte(`{}`))
	if err != nil {
		panic(err)
	}
	return state
}()

// InitialState returns the empty state owned by this stateless compatibility
// ruleset. The returned payload is immutable and always encodes exactly {}.
func InitialState() core.Payload { return emptyState }

func decodeState(payload core.Payload) error {
	var state struct{}
	if err := decodePayload(payload, &state); err != nil {
		return fmt.Errorf("dnd5e: decode state: %w", err)
	}
	return nil
}

func validateSnapshot(snapshot core.Snapshot) error {
	if err := snapshot.Validate(); err != nil {
		return err
	}
	artifact, err := NewArtifact()
	if err != nil {
		return fmt.Errorf("dnd5e: build artifact identity: %w", err)
	}
	if snapshot.Ruleset != artifact.Lock() {
		return fmt.Errorf("dnd5e: snapshot belongs to a different rules artifact")
	}
	return decodeState(snapshot.State)
}
