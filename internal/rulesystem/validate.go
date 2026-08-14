package rulesystem

import (
	"fmt"
	"strings"
)

// Validate checks a pack for structural completeness. It does not verify rules
// accuracy against a publisher PDF.
func Validate(p *Pack) error {
	if p == nil {
		return fmt.Errorf("pack is nil")
	}
	if p.APIVersion != APIVersion {
		return fmt.Errorf("api_version: want %q, got %q", APIVersion, p.APIVersion)
	}
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(p.Dice.Primary) == "" {
		return fmt.Errorf("dice.primary is required")
	}
	if len(p.Attributes) == 0 {
		return fmt.Errorf("at least one attribute is required")
	}
	if len(p.Resources) == 0 {
		return fmt.Errorf("at least one resource is required")
	}
	if strings.TrimSpace(p.Prompts.OracleContext) == "" {
		return fmt.Errorf("prompts.oracle_context is required")
	}

	seen := map[string]bool{}
	for _, a := range p.Attributes {
		if a.ID == "" {
			return fmt.Errorf("attribute id is required")
		}
		if seen[a.ID] {
			return fmt.Errorf("duplicate attribute id %q", a.ID)
		}
		seen[a.ID] = true
	}

	enabled := 0
	for _, t := range p.Tools {
		if !t.Enabled {
			continue
		}
		enabled++
		if t.CanonicalID == "" || t.Name == "" {
			return fmt.Errorf("enabled tool must have canonical_id and name")
		}
		if _, ok := canonicalByID(t.CanonicalID); !ok {
			return fmt.Errorf("unknown canonical tool %q", t.CanonicalID)
		}
		if len(t.Parameters) == 0 {
			return fmt.Errorf("tool %q parameters are required", t.Name)
		}
	}
	if enabled == 0 {
		return fmt.Errorf("at least one tool must be enabled")
	}
	return nil
}
