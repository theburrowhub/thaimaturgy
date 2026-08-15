package profiles

import (
	"encoding/json"

	"github.com/theburrowhub/thaimaturgy/internal/rulesystem"
)

// NewBasePack creates a v2 pack skeleton with common defaults.
func NewBasePack(id, name, family string) *rulesystem.Pack {
	return &rulesystem.Pack{
		APIVersion: rulesystem.APIVersion,
		ID:         id,
		Name:       name,
		Family:     family,
		Version:    "1.0.0",
		Language:   "en",
		Source: rulesystem.SourceMeta{
			Type:       "builtin",
			Confidence: 1.0,
		},
		Dice: rulesystem.DiceConfig{
			Primary:  "d20",
			Common:   []string{"d4", "d6", "d8", "d10", "d12", "d20", "d100"},
			Notation: "NdS+M",
		},
		Combat: rulesystem.CombatModel{
			Mode: "turn_based",
			ActionEconomy: rulesystem.ActionEconomy{
				Actions: []rulesystem.ActionType{
					{ID: "action", Label: "Action", PerTurn: 1},
					{ID: "bonus", Label: "Bonus Action", PerTurn: 1},
					{ID: "reaction", Label: "Reaction", PerTurn: 1},
					{ID: "movement", Label: "Movement"},
					{ID: "free", Label: "Free"},
				},
			},
		},
		Progression: rulesystem.ProgressionModel{Kind: "level"},
		Prompts: rulesystem.PromptBundle{
			OracleContext: "Resolve actions using bound tools and pack workflows.",
		},
		OracleGuide: rulesystem.OracleGuide{
			Principles: []string{
				"Prefer structured tools over freeform narration for mechanical outcomes.",
				"Apply preconditions before invoking combat or magic tools.",
			},
		},
		Compatibility: rulesystem.EngineCompat{
			ToolMap: map[string]string{},
		},
		Metadata: map[string]string{"generator": "rulesystem/v2"},
	}
}

// AddChapter appends a rule chapter to the pack.
func AddChapter(p *rulesystem.Pack, id, title, summary string, sections []rulesystem.Section, tags ...string) {
	p.Chapters = append(p.Chapters, rulesystem.RuleChapter{
		ID: id, Title: title, Summary: summary, Sections: sections, Tags: tags,
	})
}

// AddWorkflow appends a workflow definition.
func AddWorkflow(p *rulesystem.Pack, wf rulesystem.WorkflowDef) {
	p.Workflows = append(p.Workflows, wf)
}

// AddMechanic appends a mechanic definition.
func AddMechanic(p *rulesystem.Pack, m rulesystem.MechanicDef) {
	p.Mechanics = append(p.Mechanics, m)
}

// AddTable appends a random table.
func AddTable(p *rulesystem.Pack, t rulesystem.TableDef) {
	p.Tables = append(p.Tables, t)
}

// AddFormula appends a formula definition.
func AddFormula(p *rulesystem.Pack, f rulesystem.FormulaDef) {
	p.Formulas = append(p.Formulas, f)
}

// BindToolFromCanonical creates and appends a tool binding from a canonical ID.
func BindToolFromCanonical(p *rulesystem.Pack, canonicalID, name, description, category, workflowID string, preconditions, effects []string, examples []rulesystem.ToolExample) {
	ct, ok := rulesystem.CanonicalByID(canonicalID)
	params := map[string]any{}
	if ok {
		params = ct.Parameters
	}
	raw, _ := json.Marshal(params)
	p.Tools = append(p.Tools, rulesystem.ToolBinding{
		CanonicalID:   canonicalID,
		Enabled:       true,
		Name:          name,
		Description:   description,
		Parameters:    raw,
		Category:      category,
		Preconditions: preconditions,
		Effects:       effects,
		Examples:      examples,
		WorkflowID:    workflowID,
	})
	if p.Compatibility.ToolMap == nil {
		p.Compatibility.ToolMap = map[string]string{}
	}
	p.Compatibility.ToolMap[name] = canonicalID
}

// SetResolution assigns resolution config.
func SetResolution(p *rulesystem.Pack, r rulesystem.ResolutionConfig) {
	p.Resolution = r
}

// AddAttribute appends an attribute definition.
func AddAttribute(p *rulesystem.Pack, a rulesystem.AttributeDef) {
	p.Attributes = append(p.Attributes, a)
}

// AddSkill appends a skill definition.
func AddSkill(p *rulesystem.Pack, s rulesystem.SkillDef) {
	p.Skills = append(p.Skills, s)
}

// AddResource appends a resource track.
func AddResource(p *rulesystem.Pack, r rulesystem.ResourceDef) {
	p.Resources = append(p.Resources, r)
}

// AddCondition appends a condition definition.
func AddCondition(p *rulesystem.Pack, c rulesystem.ConditionDef) {
	p.Conditions = append(p.Conditions, c)
}

// AddDamageType appends a damage type entry.
func AddDamageType(p *rulesystem.Pack, id, label, notes string) {
	p.DamageTypes = append(p.DamageTypes, rulesystem.NamedDesc{ID: id, Label: label, Notes: notes})
}

// AddEquipmentTemplate appends an item template.
func AddEquipmentTemplate(p *rulesystem.Pack, t rulesystem.ItemTemplate) {
	p.Equipment.Templates = append(p.Equipment.Templates, t)
}

// Section is a convenience constructor for rule sections.
func Section(id, title, body string, bullets ...string) rulesystem.Section {
	return rulesystem.Section{ID: id, Title: title, Body: body, Bullets: bullets}
}

// WorkflowStep is a convenience constructor for workflow steps.
func WorkflowStep(id, label, kind, roll, next string) rulesystem.WorkflowStep {
	return rulesystem.WorkflowStep{ID: id, Label: label, Kind: kind, Roll: roll, Next: next}
}
