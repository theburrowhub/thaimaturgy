package rulesystem

// DefaultEnrichmentSpec returns an LLM-ready enrichment specification for a pack.
func DefaultEnrichmentSpec(p *Pack) EnrichmentSpec {
	if p == nil {
		return EnrichmentSpec{Enabled: false, Objective: "no pack provided"}
	}
	objective := "Enrich the RPG rules pack with concise oracle-facing summaries, missing edge cases, and tool preconditions while preserving canonical tool IDs."
	inputFields := []string{
		"id", "name", "family", "attributes", "skills", "resources", "conditions",
		"workflows", "mechanics", "tables", "chapters", "tools", "oracle_guide",
	}
	outputFields := []string{
		"rules_summary", "chapters.sections.body", "mechanics.summary",
		"tools.preconditions", "tools.examples", "oracle_guide.scenarios",
		"prompts.oracle_context", "enrichment.notes",
	}
	hints := []string{
		"Do not invent new canonical tool IDs; only use those defined in the pack compatibility map.",
		"Keep chapter bodies under 400 words per section; prefer bullet lists for procedures.",
		"Align workflow steps with bound tool names.",
		"Flag ambiguous rules with [NEEDS_RULING] rather than guessing.",
	}
	if p.Family != "" {
		hints = append(hints, "Respect family conventions for "+p.Family+".")
	}
	return EnrichmentSpec{
		Enabled:      true,
		Objective:    objective,
		InputFields:  inputFields,
		OutputFields: outputFields,
		PromptHints:  hints,
	}
}
