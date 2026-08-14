// Package rulesystem defines portable RPG system packs and a generator to
// produce them from built-in templates or source documents (PDF text). Packs
// are intentionally isolated from the running oracle/engine — they describe
// how thAImaturgy *would* wire generic tools (attack, spell, HP, characters…)
// once multi-system support lands.
package rulesystem

import "encoding/json"

const APIVersion = "rulesystem/v1"

// Pack is the on-disk definition of an RPG ruleset for thAImaturgy.
type Pack struct {
	APIVersion string `json:"api_version" yaml:"api_version"`
	ID         string `json:"id" yaml:"id"`
	Name       string `json:"name" yaml:"name"`
	Version    string `json:"version,omitempty" yaml:"version,omitempty"`
	Language   string `json:"language,omitempty" yaml:"language,omitempty"`

	Source SourceMeta `json:"source" yaml:"source"`
	Dice   DiceConfig `json:"dice" yaml:"dice"`

	Attributes []AttributeDef `json:"attributes" yaml:"attributes"`
	Skills     []SkillDef     `json:"skills,omitempty" yaml:"skills,omitempty"`
	Resources  []ResourceDef  `json:"resources" yaml:"resources"`
	Conditions []ConditionDef `json:"conditions,omitempty" yaml:"conditions,omitempty"`

	Resolution ResolutionConfig `json:"resolution" yaml:"resolution"`
	Tools      []ToolBinding    `json:"tools" yaml:"tools"`

	Character CharacterSchema `json:"character" yaml:"character"`
	Prompts   PromptBundle    `json:"prompts" yaml:"prompts"`

	RulesSummary []string          `json:"rules_summary,omitempty" yaml:"rules_summary,omitempty"`
	RawExcerpts  []SourceExcerpt   `json:"raw_excerpts,omitempty" yaml:"raw_excerpts,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type SourceMeta struct {
	Type     string `json:"type" yaml:"type"` // builtin | pdf | manual
	Document string `json:"document,omitempty" yaml:"document,omitempty"`
	Notes    string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type SourceExcerpt struct {
	Page    int    `json:"page,omitempty" yaml:"page,omitempty"`
	Heading string `json:"heading,omitempty" yaml:"heading,omitempty"`
	Text    string `json:"text" yaml:"text"`
}

type DiceConfig struct {
	Primary string   `json:"primary" yaml:"primary"`
	Common  []string `json:"common" yaml:"common"`
	// Notation describes how dice are written (standard NdM+K, percentile, trait+d6…).
	Notation string `json:"notation" yaml:"notation"`
	Notes    string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type AttributeDef struct {
	ID    string `json:"id" yaml:"id"`
	Label string `json:"label" yaml:"label"`
	Abbrev string `json:"abbrev,omitempty" yaml:"abbrev,omitempty"`
	Scale string `json:"scale,omitempty" yaml:"scale,omitempty"` // score | die_type | percentile
}

type SkillDef struct {
	ID         string `json:"id" yaml:"id"`
	Label      string `json:"label" yaml:"label"`
	Attribute  string `json:"attribute,omitempty" yaml:"attribute,omitempty"`
	Training   bool   `json:"training,omitempty" yaml:"training,omitempty"`
	Notes      string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ResourceDef struct {
	ID          string `json:"id" yaml:"id"`
	Label       string `json:"label" yaml:"label"`
	Kind        string `json:"kind" yaml:"kind"` // pool | track | counter | bennies
	Primary     bool   `json:"primary,omitempty" yaml:"primary,omitempty"`
	Min         int    `json:"min,omitempty" yaml:"min,omitempty"`
	DefaultMax  string `json:"default_max,omitempty" yaml:"default_max,omitempty"`
	Notes       string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ConditionDef struct {
	ID    string `json:"id" yaml:"id"`
	Label string `json:"label" yaml:"label"`
	Notes string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// ResolutionConfig maps canonical mechanics to system-specific rolls.
type ResolutionConfig struct {
	SkillCheck   CheckRule   `json:"skill_check" yaml:"skill_check"`
	AbilityCheck CheckRule   `json:"ability_check" yaml:"ability_check"`
	Attack       AttackRule  `json:"attack" yaml:"attack"`
	Defense      DefenseRule `json:"defense" yaml:"defense"`
	Spell        PowerRule   `json:"spell" yaml:"spell"`
	Damage       DamageRule  `json:"damage" yaml:"damage"`
	Initiative   CheckRule   `json:"initiative" yaml:"initiative"`
}

type CheckRule struct {
	Roll       string `json:"roll" yaml:"roll"`
	Compare    string `json:"compare" yaml:"compare"` // gte | lte | under | open_ended
	Target     string `json:"target,omitempty" yaml:"target,omitempty"`
	Success    string `json:"success,omitempty" yaml:"success,omitempty"`
	Failure    string `json:"failure,omitempty" yaml:"failure,omitempty"`
	Critical   string `json:"critical,omitempty" yaml:"critical,omitempty"`
	Fumble     string `json:"fumble,omitempty" yaml:"fumble,omitempty"`
	Notes      string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type AttackRule struct {
	Roll    string `json:"roll" yaml:"roll"`
	Target  string `json:"target" yaml:"target"`
	Compare string `json:"compare" yaml:"compare"`
	Notes   string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type DefenseRule struct {
	Stat    string `json:"stat" yaml:"stat"`
	Formula string `json:"formula,omitempty" yaml:"formula,omitempty"`
	Notes   string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type PowerRule struct {
	Cost      string `json:"cost" yaml:"cost"`
	Roll      string `json:"roll" yaml:"roll"`
	Notes     string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type DamageRule struct {
	Roll  string `json:"roll" yaml:"roll"`
	Type  string `json:"type,omitempty" yaml:"type,omitempty"`
	Notes string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

// ToolBinding wires a canonical tool to LLM-facing schema + future engine hook.
type ToolBinding struct {
	CanonicalID string          `json:"canonical_id" yaml:"canonical_id"`
	Enabled     bool            `json:"enabled" yaml:"enabled"`
	Name        string          `json:"name" yaml:"name"`
	Description string          `json:"description" yaml:"description"`
	Parameters  json.RawMessage `json:"parameters" yaml:"parameters"`
	EngineHook  string          `json:"engine_hook,omitempty" yaml:"engine_hook,omitempty"`
	Notes       string          `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type CharacterSchema struct {
	Fields []CharacterField `json:"fields" yaml:"fields"`
	Notes  string           `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type CharacterField struct {
	ID       string `json:"id" yaml:"id"`
	Label    string `json:"label" yaml:"label"`
	Kind     string `json:"kind" yaml:"kind"` // string | int | list | resource | formula
	Required bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Notes    string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type PromptBundle struct {
	OracleContext string `json:"oracle_context" yaml:"oracle_context"`
	DMNotes       string `json:"dm_notes,omitempty" yaml:"dm_notes,omitempty"`
	PlayerNotes   string `json:"player_notes,omitempty" yaml:"player_notes,omitempty"`
}
