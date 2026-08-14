package rulesystem

import "encoding/json"

// CanonicalTool describes a portable engine tool that packs may bind to.
type CanonicalTool struct {
	ID          string
	Label       string
	Category    string
	Description string
	Parameters  map[string]any
}

var canonicalTools = []CanonicalTool{
	{ID: "roll_dice", Label: "Roll Dice", Category: "core", Description: "Roll one or more dice using system notation.", Parameters: map[string]any{"notation": "string", "reason": "string"}},
	{ID: "ability_check", Label: "Ability Check", Category: "resolution", Description: "Resolve a raw attribute check against a DC.", Parameters: map[string]any{"ability": "string", "dc": "number", "actor_id": "string"}},
	{ID: "skill_check", Label: "Skill Check", Category: "resolution", Description: "Resolve a trained or untrained skill check.", Parameters: map[string]any{"skill": "string", "dc": "number", "actor_id": "string", "advantage": "boolean"}},
	{ID: "saving_throw", Label: "Saving Throw", Category: "resolution", Description: "Resolve a save against an effect DC.", Parameters: map[string]any{"ability": "string", "dc": "number", "actor_id": "string"}},
	{ID: "opposed_check", Label: "Opposed Check", Category: "resolution", Description: "Compare two rolls; higher wins.", Parameters: map[string]any{"attacker_id": "string", "defender_id": "string", "skill": "string"}},
	{ID: "attack", Label: "Attack", Category: "combat", Description: "Resolve a weapon or unarmed attack.", Parameters: map[string]any{"attacker_id": "string", "target_id": "string", "weapon_id": "string"}},
	{ID: "cast_spell", Label: "Cast Spell", Category: "magic", Description: "Expend a spell slot and resolve spell effects.", Parameters: map[string]any{"caster_id": "string", "spell_id": "string", "slot_level": "number", "targets": "array"}},
	{ID: "use_power", Label: "Use Power", Category: "magic", Description: "Activate a power, maneuver, or edge.", Parameters: map[string]any{"actor_id": "string", "power_id": "string", "targets": "array"}},
	{ID: "update_health", Label: "Update Health", Category: "state", Description: "Apply healing or damage to a resource track.", Parameters: map[string]any{"actor_id": "string", "resource": "string", "delta": "number", "source": "string"}},
	{ID: "apply_condition", Label: "Apply Condition", Category: "state", Description: "Add a condition to an actor.", Parameters: map[string]any{"actor_id": "string", "condition_id": "string", "duration": "string"}},
	{ID: "remove_condition", Label: "Remove Condition", Category: "state", Description: "Remove a condition from an actor.", Parameters: map[string]any{"actor_id": "string", "condition_id": "string"}},
	{ID: "update_character", Label: "Update Character", Category: "state", Description: "Patch character fields or derived stats.", Parameters: map[string]any{"actor_id": "string", "patch": "object"}},
	{ID: "rest", Label: "Rest", Category: "progression", Description: "Apply short or long rest recovery.", Parameters: map[string]any{"actor_id": "string", "rest_type": "string"}},
	{ID: "initiative", Label: "Initiative", Category: "combat", Description: "Roll and order combatants.", Parameters: map[string]any{"participants": "array"}},
	{ID: "lookup_creature", Label: "Lookup Creature", Category: "reference", Description: "Fetch a creature template or stat block.", Parameters: map[string]any{"template_id": "string", "scale": "string"}},
	{ID: "award_experience", Label: "Award Experience", Category: "progression", Description: "Grant XP or advancement points.", Parameters: map[string]any{"actor_ids": "array", "amount": "number", "reason": "string"}},
	{ID: "inventory_add", Label: "Inventory Add", Category: "inventory", Description: "Add items to an actor inventory.", Parameters: map[string]any{"actor_id": "string", "items": "array"}},
	{ID: "inventory_remove", Label: "Inventory Remove", Category: "inventory", Description: "Remove items from an actor inventory.", Parameters: map[string]any{"actor_id": "string", "items": "array"}},
	{ID: "damage_roll", Label: "Damage Roll", Category: "combat", Description: "Roll damage for a hit or effect.", Parameters: map[string]any{"notation": "string", "damage_type": "string", "critical": "boolean"}},
	{ID: "concentration_check", Label: "Concentration Check", Category: "magic", Description: "Check if concentration is maintained after damage.", Parameters: map[string]any{"caster_id": "string", "damage_taken": "number"}},
	{ID: "death_save", Label: "Death Save", Category: "combat", Description: "Resolve a death saving throw at 0 HP.", Parameters: map[string]any{"actor_id": "string"}},
	{ID: "soak_damage", Label: "Soak Damage", Category: "combat", Description: "Attempt to soak or reduce incoming damage.", Parameters: map[string]any{"actor_id": "string", "damage": "number", "damage_type": "string"}},
	{ID: "spend_benny", Label: "Spend Benny", Category: "meta", Description: "Spend a benny for reroll or soak.", Parameters: map[string]any{"actor_id": "string", "purpose": "string"}},
	{ID: "draw_benny", Label: "Draw Benny", Category: "meta", Description: "Draw bennies at session start or milestones.", Parameters: map[string]any{"actor_id": "string", "count": "number"}},
	{ID: "improve_skill", Label: "Improve Skill", Category: "progression", Description: "Advance a skill or ability during downtime.", Parameters: map[string]any{"actor_id": "string", "skill_id": "string", "cost": "number"}},
	{ID: "social_conflict", Label: "Social Conflict", Category: "social", Description: "Resolve persuasion, intimidation, or debate.", Parameters: map[string]any{"actor_id": "string", "target_id": "string", "approach": "string", "stakes": "string"}},
	{ID: "fear_sanity", Label: "Fear / Sanity", Category: "horror", Description: "Resolve fear, sanity, or dread checks.", Parameters: map[string]any{"actor_id": "string", "trigger": "string", "severity": "number"}},
	{ID: "apply_template", Label: "Apply Template", Category: "reference", Description: "Apply a creature or character template.", Parameters: map[string]any{"actor_id": "string", "template_id": "string"}},
	{ID: "roll_on_table", Label: "Roll On Table", Category: "reference", Description: "Roll on a random table defined in the pack.", Parameters: map[string]any{"table_id": "string", "modifier": "number"}},
	{ID: "advance_quest", Label: "Advance Quest", Category: "narrative", Description: "Generic quest or milestone progression hook.", Parameters: map[string]any{"quest_id": "string", "outcome": "string", "notes": "string"}},
}

var canonicalByID map[string]CanonicalTool

func init() {
	canonicalByID = make(map[string]CanonicalTool, len(canonicalTools))
	for _, t := range canonicalTools {
		canonicalByID[t.ID] = t
	}
}

// CanonicalByID returns a canonical tool definition by ID.
func CanonicalByID(id string) (CanonicalTool, bool) {
	t, ok := canonicalByID[id]
	return t, ok
}

// ListCanonicalTools returns all canonical tool definitions.
func ListCanonicalTools() []CanonicalTool {
	out := make([]CanonicalTool, len(canonicalTools))
	copy(out, canonicalTools)
	return out
}

// bindTool creates a ToolBinding from a canonical tool ID.
func bindTool(canonicalID, name, description, category, workflowID string, preconditions, effects []string, examples []ToolExample) ToolBinding {
	params := map[string]any{}
	if ct, ok := canonicalByID[canonicalID]; ok {
		params = ct.Parameters
	}
	raw, _ := json.Marshal(params)
	return ToolBinding{
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
	}
}
