package rulesystem

import (
	"fmt"
	"strings"
)

// ValidationIssue describes a single validation problem.
type ValidationIssue struct {
	Path    string
	Message string
}

func (v ValidationIssue) Error() string {
	if v.Path == "" {
		return v.Message
	}
	return fmt.Sprintf("%s: %s", v.Path, v.Message)
}

// ValidatePack performs deep validation on a pack definition.
func ValidatePack(p *Pack) []ValidationIssue {
	if p == nil {
		return []ValidationIssue{{Message: "pack is nil"}}
	}
	var issues []ValidationIssue
	if p.ID == "" {
		issues = append(issues, ValidationIssue{Path: "id", Message: "required"})
	}
	if p.Name == "" {
		issues = append(issues, ValidationIssue{Path: "name", Message: "required"})
	}
	if p.APIVersion != "" && p.APIVersion != APIVersion {
		issues = append(issues, ValidationIssue{Path: "api_version", Message: fmt.Sprintf("expected %q", APIVersion)})
	}
	if len(p.Chapters) == 0 {
		issues = append(issues, ValidationIssue{Path: "chapters", Message: "at least one chapter required"})
	}
	for i, ch := range p.Chapters {
		if strings.TrimSpace(ch.Title) == "" {
			issues = append(issues, ValidationIssue{Path: fmt.Sprintf("chapters[%d].title", i), Message: "required"})
		}
		if len(ch.Sections) == 0 {
			issues = append(issues, ValidationIssue{Path: fmt.Sprintf("chapters[%d].sections", i), Message: "at least one section required"})
		}
	}
	attrIDs := map[string]struct{}{}
	for _, a := range p.Attributes {
		attrIDs[a.ID] = struct{}{}
	}
	skillIDs := map[string]struct{}{}
	for _, s := range p.Skills {
		skillIDs[s.ID] = struct{}{}
		if s.Attribute != "" {
			if _, ok := attrIDs[s.Attribute]; !ok {
				issues = append(issues, ValidationIssue{Path: "skills." + s.ID, Message: fmt.Sprintf("unknown attribute %q", s.Attribute)})
			}
		}
	}
	condIDs := map[string]struct{}{}
	for _, c := range p.Conditions {
		condIDs[c.ID] = struct{}{}
	}
	wfIDs := map[string]struct{}{}
	for _, w := range p.Workflows {
		wfIDs[w.ID] = struct{}{}
		stepIDs := map[string]struct{}{}
		for _, st := range w.Steps {
			stepIDs[st.ID] = struct{}{}
		}
		for j, st := range w.Steps {
			if st.Next != "" {
				if _, ok := stepIDs[st.Next]; !ok {
					issues = append(issues, ValidationIssue{
						Path:    fmt.Sprintf("workflows.%s.steps[%d].next", w.ID, j),
						Message: fmt.Sprintf("unknown step %q", st.Next),
					})
				}
			}
		}
		for _, rt := range w.RelatedTools {
			if _, ok := canonicalByID[rt]; !ok {
				issues = append(issues, ValidationIssue{
					Path:    "workflows." + w.ID + ".related_tools",
					Message: fmt.Sprintf("unknown canonical tool %q", rt),
				})
			}
		}
	}
	for _, m := range p.Mechanics {
		if m.WorkflowID != "" {
			if _, ok := wfIDs[m.WorkflowID]; !ok {
				issues = append(issues, ValidationIssue{
					Path:    "mechanics." + m.ID,
					Message: fmt.Sprintf("unknown workflow %q", m.WorkflowID),
				})
			}
		}
	}
	tableIDs := map[string]struct{}{}
	for _, t := range p.Tables {
		tableIDs[t.ID] = struct{}{}
	}
	for _, f := range p.Formulas {
		if _, err := EvalFormula(f.Expression, sampleVars(f.Variables)); err != nil {
			issues = append(issues, ValidationIssue{
				Path:    "formulas." + f.ID,
				Message: fmt.Sprintf("invalid expression: %v", err),
			})
		}
	}
	for _, r := range p.Resources {
		if r.MaxFormula != "" {
			if _, err := EvalFormula(r.MaxFormula, sampleVars(nil)); err != nil {
				issues = append(issues, ValidationIssue{
					Path:    "resources." + r.ID + ".max_formula",
					Message: fmt.Sprintf("invalid formula: %v", err),
				})
			}
		}
	}
	for i, tb := range p.Tools {
		if _, ok := canonicalByID[tb.CanonicalID]; !ok {
			issues = append(issues, ValidationIssue{
				Path:    fmt.Sprintf("tools[%d].canonical_id", i),
				Message: fmt.Sprintf("unknown canonical tool %q", tb.CanonicalID),
			})
		}
		if tb.WorkflowID != "" {
			if _, ok := wfIDs[tb.WorkflowID]; !ok {
				issues = append(issues, ValidationIssue{
					Path:    fmt.Sprintf("tools[%d].workflow_id", i),
					Message: fmt.Sprintf("unknown workflow %q", tb.WorkflowID),
				})
			}
		}
	}
	for _, field := range p.Character.Fields {
		if field.Formula != "" {
			if _, err := EvalFormula(field.Formula, sampleVars(nil)); err != nil {
				issues = append(issues, ValidationIssue{
					Path:    "character.fields." + field.ID,
					Message: fmt.Sprintf("invalid formula: %v", err),
				})
			}
		}
	}
	_ = condIDs
	_ = tableIDs
	_ = skillIDs
	return issues
}

func sampleVars(desc map[string]string) map[string]float64 {
	out := map[string]float64{
		"level": 5, "con_mod": 2, "dex_mod": 3, "wis_mod": 1, "str_mod": 2,
		"proficiency": 3, "rank": 2, "die_size": 8, "stat": 50,
		"con": 55, "siz": 65, "pow": 60, "dex": 45, "int": 50, "cha": 40,
		"hit_die_max": 10, "hit_die_avg": 6, "max_hp": 45,
		"armor": 5, "shield": 2, "fighting": 8, "vigor": 10,
		"spellcasting_mod": 3, "slots_by_level": 9, "power_points_by_rank": 15,
		"damage": 12,
	}
	for k := range desc {
		if _, ok := out[k]; !ok {
			out[k] = 1
		}
	}
	return out
}

// ValidatePackStrict returns an error if any issues are found.
func ValidatePackStrict(p *Pack) error {
	issues := ValidatePack(p)
	if len(issues) == 0 {
		return nil
	}
	msgs := make([]string, len(issues))
	for i, iss := range issues {
		msgs[i] = iss.Error()
	}
	return fmt.Errorf("pack validation failed:\n  - %s", strings.Join(msgs, "\n  - "))
}
