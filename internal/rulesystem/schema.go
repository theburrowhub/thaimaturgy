// Package rulesystem defines portable RPG system packs and a generator to
// produce them from built-in templates or source documents (PDF text). Packs
// are intentionally isolated from the running oracle/engine — they describe
// how thAImaturgy *would* wire generic tools once multi-system support lands.
package rulesystem

import "encoding/json"

const APIVersion = "rulesystem/v2"

// Pack is the on-disk definition of an RPG ruleset for thAImaturgy.
type Pack struct {
	APIVersion string `json:"api_version" yaml:"api_version"`
	ID         string `json:"id" yaml:"id"`
	Name       string `json:"name" yaml:"name"`
	Version    string `json:"version,omitempty" yaml:"version,omitempty"`
	Language   string `json:"language,omitempty" yaml:"language,omitempty"`
	Family     string `json:"family,omitempty" yaml:"family,omitempty"`

	Source      SourceMeta       `json:"source" yaml:"source"`
	Dice        DiceConfig       `json:"dice" yaml:"dice"`
	Attributes  []AttributeDef   `json:"attributes" yaml:"attributes"`
	Skills      []SkillDef       `json:"skills,omitempty" yaml:"skills,omitempty"`
	Resources   []ResourceDef    `json:"resources" yaml:"resources"`
	Conditions  []ConditionDef   `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	DamageTypes []NamedDesc      `json:"damage_types,omitempty" yaml:"damage_types,omitempty"`
	Equipment   EquipmentCatalog `json:"equipment,omitempty" yaml:"equipment,omitempty"`

	Resolution  ResolutionConfig `json:"resolution" yaml:"resolution"`
	Combat      CombatModel      `json:"combat" yaml:"combat"`
	Progression ProgressionModel   `json:"progression" yaml:"progression"`
	Magic       MagicModel       `json:"magic,omitempty" yaml:"magic,omitempty"`
	Social      SocialModel      `json:"social,omitempty" yaml:"social,omitempty"`

	Formulas  []FormulaDef  `json:"formulas,omitempty" yaml:"formulas,omitempty"`
	Workflows []WorkflowDef `json:"workflows" yaml:"workflows"`
	Mechanics []MechanicDef `json:"mechanics" yaml:"mechanics"`
	Tables    []TableDef    `json:"tables,omitempty" yaml:"tables,omitempty"`
	Chapters  []RuleChapter `json:"chapters" yaml:"chapters"`

	Tools         []ToolBinding  `json:"tools" yaml:"tools"`
	Character     CharacterSchema `json:"character" yaml:"character"`
	Creature      CreatureSchema  `json:"creature" yaml:"creature"`
	Prompts       PromptBundle    `json:"prompts" yaml:"prompts"`
	OracleGuide   OracleGuide     `json:"oracle_guide" yaml:"oracle_guide"`
	Compatibility EngineCompat    `json:"compatibility" yaml:"compatibility"`
	Enrichment    EnrichmentSpec  `json:"enrichment,omitempty" yaml:"enrichment,omitempty"`

	RulesSummary []string        `json:"rules_summary,omitempty" yaml:"rules_summary,omitempty"`
	RawExcerpts  []SourceExcerpt `json:"raw_excerpts,omitempty" yaml:"raw_excerpts,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type SourceMeta struct {
	Type       string   `json:"type" yaml:"type"`
	Document   string   `json:"document,omitempty" yaml:"document,omitempty"`
	Templates  []string `json:"templates,omitempty" yaml:"templates,omitempty"`
	Notes      string   `json:"notes,omitempty" yaml:"notes,omitempty"`
	Confidence float64  `json:"confidence,omitempty" yaml:"confidence,omitempty"`
}

type SourceExcerpt struct {
	Page       int      `json:"page,omitempty" yaml:"page,omitempty"`
	Heading    string   `json:"heading,omitempty" yaml:"heading,omitempty"`
	Category   string   `json:"category,omitempty" yaml:"category,omitempty"`
	Keywords   []string `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	Text       string   `json:"text" yaml:"text"`
	Confidence float64  `json:"confidence,omitempty" yaml:"confidence,omitempty"`
}

type NamedDesc struct {
	ID    string `json:"id" yaml:"id"`
	Label string `json:"label" yaml:"label"`
	Notes string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type DiceConfig struct {
	Primary   string         `json:"primary" yaml:"primary"`
	Common    []string       `json:"common" yaml:"common"`
	Notation  string         `json:"notation" yaml:"notation"`
	Exploding bool           `json:"exploding,omitempty" yaml:"exploding,omitempty"`
	Keep      string         `json:"keep,omitempty" yaml:"keep,omitempty"`
	Modifiers []DiceModifier `json:"modifiers,omitempty" yaml:"modifiers,omitempty"`
	Notes     string         `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type DiceModifier struct {
	ID    string `json:"id" yaml:"id"`
	Label string `json:"label" yaml:"label"`
	Roll  string `json:"roll" yaml:"roll"`
	Notes string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type AttributeDef struct {
	ID              string `json:"id" yaml:"id"`
	Label           string `json:"label" yaml:"label"`
	Abbrev          string `json:"abbrev,omitempty" yaml:"abbrev,omitempty"`
	Scale           string `json:"scale,omitempty" yaml:"scale,omitempty"`
	Min             int    `json:"min,omitempty" yaml:"min,omitempty"`
	Max             int    `json:"max,omitempty" yaml:"max,omitempty"`
	DefaultRoll     string `json:"default_roll,omitempty" yaml:"default_roll,omitempty"`
	DerivedFrom     string `json:"derived_from,omitempty" yaml:"derived_from,omitempty"`
	ModifierFormula string `json:"modifier_formula,omitempty" yaml:"modifier_formula,omitempty"`
	Notes           string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type SkillDef struct {
	ID        string   `json:"id" yaml:"id"`
	Label     string   `json:"label" yaml:"label"`
	Attribute string   `json:"attribute,omitempty" yaml:"attribute,omitempty"`
	Category  string   `json:"category,omitempty" yaml:"category,omitempty"`
	Training  bool     `json:"training,omitempty" yaml:"training,omitempty"`
	Specialty bool     `json:"specialty,omitempty" yaml:"specialty,omitempty"`
	Default   int      `json:"default,omitempty" yaml:"default,omitempty"`
	Tags      []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Notes     string   `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ResourceDef struct {
	ID         string   `json:"id" yaml:"id"`
	Label      string   `json:"label" yaml:"label"`
	Kind       string   `json:"kind" yaml:"kind"`
	Primary    bool     `json:"primary,omitempty" yaml:"primary,omitempty"`
	Min        int      `json:"min,omitempty" yaml:"min,omitempty"`
	MaxFormula string   `json:"max_formula,omitempty" yaml:"max_formula,omitempty"`
	ResetOn    []string `json:"reset_on,omitempty" yaml:"reset_on,omitempty"`
	Overflow   string   `json:"overflow,omitempty" yaml:"overflow,omitempty"`
	TrackSteps []string `json:"track_steps,omitempty" yaml:"track_steps,omitempty"`
	Notes      string   `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ConditionDef struct {
	ID       string   `json:"id" yaml:"id"`
	Label    string   `json:"label" yaml:"label"`
	Severity int      `json:"severity,omitempty" yaml:"severity,omitempty"`
	Effects  []string `json:"effects,omitempty" yaml:"effects,omitempty"`
	EndsOn   []string `json:"ends_on,omitempty" yaml:"ends_on,omitempty"`
	Notes    string   `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type EquipmentCatalog struct {
	WeaponCategories []NamedDesc    `json:"weapon_categories,omitempty" yaml:"weapon_categories,omitempty"`
	ArmorCategories  []NamedDesc    `json:"armor_categories,omitempty" yaml:"armor_categories,omitempty"`
	Properties       []NamedDesc    `json:"properties,omitempty" yaml:"properties,omitempty"`
	Templates        []ItemTemplate `json:"templates,omitempty" yaml:"templates,omitempty"`
}

type ItemTemplate struct {
	ID    string            `json:"id" yaml:"id"`
	Label string            `json:"label" yaml:"label"`
	Kind  string            `json:"kind" yaml:"kind"`
	Stats map[string]string `json:"stats,omitempty" yaml:"stats,omitempty"`
	Notes string            `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ResolutionConfig struct {
	SkillCheck   CheckRule   `json:"skill_check" yaml:"skill_check"`
	AbilityCheck CheckRule   `json:"ability_check" yaml:"ability_check"`
	SavingThrow  CheckRule   `json:"saving_throw" yaml:"saving_throw"`
	OpposedCheck OpposedRule `json:"opposed_check" yaml:"opposed_check"`
	Attack       AttackRule  `json:"attack" yaml:"attack"`
	Defense      DefenseRule `json:"defense" yaml:"defense"`
	Spell        PowerRule   `json:"spell" yaml:"spell"`
	Power        PowerRule   `json:"power" yaml:"power"`
	Damage       DamageRule  `json:"damage" yaml:"damage"`
	Initiative   CheckRule   `json:"initiative" yaml:"initiative"`
	Death        DeathRule   `json:"death" yaml:"death"`
}

type CheckRule struct {
	Roll       string           `json:"roll" yaml:"roll"`
	Compare    string           `json:"compare" yaml:"compare"`
	Target     string           `json:"target,omitempty" yaml:"target,omitempty"`
	Difficulty []DifficultyTier `json:"difficulty,omitempty" yaml:"difficulty,omitempty"`
	Success    string           `json:"success,omitempty" yaml:"success,omitempty"`
	Failure    string           `json:"failure,omitempty" yaml:"failure,omitempty"`
	Critical   string           `json:"critical,omitempty" yaml:"critical,omitempty"`
	Fumble     string           `json:"fumble,omitempty" yaml:"fumble,omitempty"`
	WorkflowID string           `json:"workflow_id,omitempty" yaml:"workflow_id,omitempty"`
	Notes      string           `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type DifficultyTier struct {
	ID     string `json:"id" yaml:"id"`
	Label  string `json:"label" yaml:"label"`
	Target int    `json:"target,omitempty" yaml:"target,omitempty"`
	Notes  string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type OpposedRule struct {
	AttackerRoll string `json:"attacker_roll" yaml:"attacker_roll"`
	DefenderRoll string `json:"defender_roll" yaml:"defender_roll"`
	Win          string `json:"win" yaml:"win"`
	Tie          string `json:"tie,omitempty" yaml:"tie,omitempty"`
	Notes        string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type AttackRule struct {
	Roll       string   `json:"roll" yaml:"roll"`
	Target     string   `json:"target" yaml:"target"`
	Compare    string   `json:"compare" yaml:"compare"`
	OnHit      []string `json:"on_hit,omitempty" yaml:"on_hit,omitempty"`
	OnMiss     []string `json:"on_miss,omitempty" yaml:"on_miss,omitempty"`
	OnCrit     []string `json:"on_crit,omitempty" yaml:"on_crit,omitempty"`
	WorkflowID string   `json:"workflow_id,omitempty" yaml:"workflow_id,omitempty"`
	Notes      string   `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type DefenseRule struct {
	Stat      string   `json:"stat" yaml:"stat"`
	Formula   string   `json:"formula,omitempty" yaml:"formula,omitempty"`
	Alternate []string `json:"alternate,omitempty" yaml:"alternate,omitempty"`
	Notes     string   `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type PowerRule struct {
	Cost       string   `json:"cost" yaml:"cost"`
	Roll       string   `json:"roll" yaml:"roll"`
	Components []string `json:"components,omitempty" yaml:"components,omitempty"`
	WorkflowID string   `json:"workflow_id,omitempty" yaml:"workflow_id,omitempty"`
	Notes      string   `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type DamageRule struct {
	Roll       string   `json:"roll" yaml:"roll"`
	Type       string   `json:"type,omitempty" yaml:"type,omitempty"`
	Resistance string   `json:"resistance,omitempty" yaml:"resistance,omitempty"`
	OnApply    []string `json:"on_apply,omitempty" yaml:"on_apply,omitempty"`
	Notes      string   `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type DeathRule struct {
	AtZero     []string `json:"at_zero" yaml:"at_zero"`
	Recovery   []string `json:"recovery,omitempty" yaml:"recovery,omitempty"`
	Permanent  string   `json:"permanent,omitempty" yaml:"permanent,omitempty"`
	WorkflowID string   `json:"workflow_id,omitempty" yaml:"workflow_id,omitempty"`
}

type CombatModel struct {
	Mode          string        `json:"mode" yaml:"mode"`
	RoundSteps    []string      `json:"round_steps,omitempty" yaml:"round_steps,omitempty"`
	ActionEconomy ActionEconomy `json:"action_economy" yaml:"action_economy"`
	Positioning   string        `json:"positioning,omitempty" yaml:"positioning,omitempty"`
	Notes         string        `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ActionEconomy struct {
	Actions []ActionType `json:"actions" yaml:"actions"`
	Notes   string       `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ActionType struct {
	ID       string   `json:"id" yaml:"id"`
	Label    string   `json:"label" yaml:"label"`
	PerTurn  int      `json:"per_turn,omitempty" yaml:"per_turn,omitempty"`
	Examples []string `json:"examples,omitempty" yaml:"examples,omitempty"`
}

type ProgressionModel struct {
	Kind       string     `json:"kind" yaml:"kind"`
	Levels     []LevelRow `json:"levels,omitempty" yaml:"levels,omitempty"`
	Milestones []string   `json:"milestones,omitempty" yaml:"milestones,omitempty"`
	Notes      string     `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type LevelRow struct {
	Level   int    `json:"level" yaml:"level"`
	XP      int    `json:"xp,omitempty" yaml:"xp,omitempty"`
	Advance string `json:"advance,omitempty" yaml:"advance,omitempty"`
	Notes   string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type MagicModel struct {
	Traditions []NamedDesc `json:"traditions,omitempty" yaml:"traditions,omitempty"`
	Casting    []string    `json:"casting,omitempty" yaml:"casting,omitempty"`
	Recovery   []string    `json:"recovery,omitempty" yaml:"recovery,omitempty"`
	Notes      string      `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type SocialModel struct {
	Conflicts []NamedDesc `json:"conflicts,omitempty" yaml:"conflicts,omitempty"`
	Notes     string      `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type FormulaDef struct {
	ID         string            `json:"id" yaml:"id"`
	Label      string            `json:"label" yaml:"label"`
	Expression string            `json:"expression" yaml:"expression"`
	Variables  map[string]string `json:"variables,omitempty" yaml:"variables,omitempty"`
	Notes      string            `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type WorkflowDef struct {
	ID           string         `json:"id" yaml:"id"`
	Label        string         `json:"label" yaml:"label"`
	Category     string         `json:"category" yaml:"category"`
	Trigger      string         `json:"trigger,omitempty" yaml:"trigger,omitempty"`
	Steps        []WorkflowStep `json:"steps" yaml:"steps"`
	Outputs      []string       `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	RelatedTools []string       `json:"related_tools,omitempty" yaml:"related_tools,omitempty"`
	Notes        string         `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type WorkflowStep struct {
	ID         string `json:"id" yaml:"id"`
	Label      string `json:"label" yaml:"label"`
	Kind       string `json:"kind" yaml:"kind"`
	Roll       string `json:"roll,omitempty" yaml:"roll,omitempty"`
	Expression string `json:"expression,omitempty" yaml:"expression,omitempty"`
	OnSuccess  string `json:"on_success,omitempty" yaml:"on_success,omitempty"`
	OnFailure  string `json:"on_failure,omitempty" yaml:"on_failure,omitempty"`
	Next       string `json:"next,omitempty" yaml:"next,omitempty"`
	Notes      string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type MechanicDef struct {
	ID           string   `json:"id" yaml:"id"`
	Label        string   `json:"label" yaml:"label"`
	Category     string   `json:"category" yaml:"category"`
	Summary      string   `json:"summary" yaml:"summary"`
	Steps        []string `json:"steps,omitempty" yaml:"steps,omitempty"`
	WorkflowID   string   `json:"workflow_id,omitempty" yaml:"workflow_id,omitempty"`
	RelatedTools []string `json:"related_tools,omitempty" yaml:"related_tools,omitempty"`
	Tags         []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type TableDef struct {
	ID      string     `json:"id" yaml:"id"`
	Label   string     `json:"label" yaml:"label"`
	Roll    string     `json:"roll" yaml:"roll"`
	Columns []string   `json:"columns,omitempty" yaml:"columns,omitempty"`
	Rows    []TableRow `json:"rows" yaml:"rows"`
	Notes   string     `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type TableRow struct {
	Key    string `json:"key" yaml:"key"`
	Result string `json:"result" yaml:"result"`
	Notes  string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type RuleChapter struct {
	ID       string    `json:"id" yaml:"id"`
	Title    string    `json:"title" yaml:"title"`
	Summary  string    `json:"summary" yaml:"summary"`
	Sections []Section `json:"sections" yaml:"sections"`
	Tags     []string  `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type Section struct {
	ID      string   `json:"id" yaml:"id"`
	Title   string   `json:"title" yaml:"title"`
	Body    string   `json:"body" yaml:"body"`
	Bullets []string `json:"bullets,omitempty" yaml:"bullets,omitempty"`
}

type ToolBinding struct {
	CanonicalID   string          `json:"canonical_id" yaml:"canonical_id"`
	Enabled       bool            `json:"enabled" yaml:"enabled"`
	Name          string          `json:"name" yaml:"name"`
	Description   string          `json:"description" yaml:"description"`
	Parameters    json.RawMessage `json:"parameters" yaml:"parameters"`
	EngineHook    string          `json:"engine_hook,omitempty" yaml:"engine_hook,omitempty"`
	Category      string          `json:"category,omitempty" yaml:"category,omitempty"`
	Preconditions []string        `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`
	Effects       []string        `json:"effects,omitempty" yaml:"effects,omitempty"`
	Examples      []ToolExample   `json:"examples,omitempty" yaml:"examples,omitempty"`
	WorkflowID    string          `json:"workflow_id,omitempty" yaml:"workflow_id,omitempty"`
	Notes         string          `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type ToolExample struct {
	Title  string         `json:"title" yaml:"title"`
	Input  map[string]any `json:"input,omitempty" yaml:"input,omitempty"`
	Output string         `json:"output" yaml:"output"`
}

type CharacterSchema struct {
	Sections []SchemaSection  `json:"sections" yaml:"sections"`
	Fields   []CharacterField `json:"fields" yaml:"fields"`
	Notes    string           `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type SchemaSection struct {
	ID    string `json:"id" yaml:"id"`
	Label string `json:"label" yaml:"label"`
	Notes string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type CharacterField struct {
	ID       string `json:"id" yaml:"id"`
	Label    string `json:"label" yaml:"label"`
	Section  string `json:"section,omitempty" yaml:"section,omitempty"`
	Kind     string `json:"kind" yaml:"kind"`
	Required bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Formula  string `json:"formula,omitempty" yaml:"formula,omitempty"`
	Notes    string `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type CreatureSchema struct {
	Fields    []CharacterField   `json:"fields" yaml:"fields"`
	Sections  []SchemaSection    `json:"sections,omitempty" yaml:"sections,omitempty"`
	Templates []CreatureTemplate `json:"templates,omitempty" yaml:"templates,omitempty"`
	Notes     string             `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type CreatureTemplate struct {
	ID    string            `json:"id" yaml:"id"`
	Label string            `json:"label" yaml:"label"`
	Stats map[string]string `json:"stats" yaml:"stats"`
	Notes string            `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type PromptBundle struct {
	OracleContext string      `json:"oracle_context" yaml:"oracle_context"`
	DMNotes       string      `json:"dm_notes,omitempty" yaml:"dm_notes,omitempty"`
	PlayerNotes   string      `json:"player_notes,omitempty" yaml:"player_notes,omitempty"`
	Glossary      []NamedDesc `json:"glossary,omitempty" yaml:"glossary,omitempty"`
}

type OracleGuide struct {
	Principles   []string        `json:"principles" yaml:"principles"`
	ToolPriority []string        `json:"tool_priority,omitempty" yaml:"tool_priority,omitempty"`
	AntiPatterns []string        `json:"anti_patterns,omitempty" yaml:"anti_patterns,omitempty"`
	Scenarios    []GuideScenario `json:"scenarios,omitempty" yaml:"scenarios,omitempty"`
}

type GuideScenario struct {
	Situation string   `json:"situation" yaml:"situation"`
	UseTools  []string `json:"use_tools" yaml:"use_tools"`
	Avoid     []string `json:"avoid,omitempty" yaml:"avoid,omitempty"`
}

type EngineCompat struct {
	CharacterType string            `json:"character_type,omitempty" yaml:"character_type,omitempty"`
	StatBlockType string            `json:"stat_block_type,omitempty" yaml:"stat_block_type,omitempty"`
	ToolMap       map[string]string `json:"tool_map,omitempty" yaml:"tool_map,omitempty"`
	Notes         string            `json:"notes,omitempty" yaml:"notes,omitempty"`
}

type EnrichmentSpec struct {
	Enabled      bool     `json:"enabled" yaml:"enabled"`
	Objective    string   `json:"objective" yaml:"objective"`
	InputFields  []string `json:"input_fields" yaml:"input_fields"`
	OutputFields []string `json:"output_fields" yaml:"output_fields"`
	PromptHints  []string `json:"prompt_hints,omitempty" yaml:"prompt_hints,omitempty"`
}
