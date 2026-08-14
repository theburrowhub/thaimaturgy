package rulesystem

import "encoding/json"

// Canonical tool IDs shared across RPG systems. Each Pack maps them to
// system-specific names, descriptions, and parameters.
const (
	ToolRollDice         = "roll_dice"
	ToolAbilityCheck     = "ability_check"
	ToolSkillCheck       = "skill_check"
	ToolAttack           = "attack"
	ToolCastSpell        = "cast_spell"
	ToolUsePower         = "use_power"
	ToolUpdateHealth     = "update_health"
	ToolApplyCondition   = "apply_condition"
	ToolRemoveCondition  = "remove_condition"
	ToolUpdateCharacter  = "update_character"
	ToolRest             = "rest"
	ToolInitiative       = "initiative"
	ToolLookupCreature   = "lookup_creature"
	ToolAwardExperience  = "award_experience"
	ToolInventoryAdd     = "inventory_add"
	ToolInventoryRemove  = "inventory_remove"
)

// CanonicalTools lists the generic oracle tools every system pack should expose.
var CanonicalTools = []CanonicalTool{
	{
		ID:          ToolRollDice,
		Label:       "Roll dice",
		Description: "Roll dice using the system's standard notation.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"notation":{"type":"string"},"reason":{"type":"string"}},"required":["notation"]}`),
		EngineHook:  "dice.roll",
	},
	{
		ID:          ToolAbilityCheck,
		Label:       "Ability check",
		Description: "Resolve a raw attribute/ability test against a target number.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"attribute":{"type":"string"},"modifier":{"type":"integer"},"target":{"type":"integer"},"label":{"type":"string"}},"required":["attribute","target"]}`),
		EngineHook:  "check.ability",
	},
	{
		ID:          ToolSkillCheck,
		Label:       "Skill check",
		Description: "Resolve a trained or untrained skill test.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"skill":{"type":"string"},"modifier":{"type":"integer"},"target":{"type":"integer"},"label":{"type":"string"}},"required":["skill","target"]}`),
		EngineHook:  "check.skill",
	},
	{
		ID:          ToolAttack,
		Label:       "Attack",
		Description: "Resolve an attack roll against a defender's defense stat.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"attacker":{"type":"string"},"defender":{"type":"string"},"weapon_or_power":{"type":"string"},"modifier":{"type":"integer"},"damage":{"type":"string"}},"required":["attacker","defender"]}`),
		EngineHook:  "combat.attack",
	},
	{
		ID:          ToolCastSpell,
		Label:       "Cast spell",
		Description: "Spend a spell slot or power points and resolve the effect.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"character":{"type":"string"},"spell":{"type":"string"},"level_or_cost":{"type":"integer"},"target":{"type":"string"}},"required":["character","spell"]}`),
		EngineHook:  "magic.cast",
	},
	{
		ID:          ToolUsePower,
		Label:       "Use power",
		Description: "Activate a power, edge, stunt, or similar special ability.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"character":{"type":"string"},"power":{"type":"string"},"target":{"type":"string"},"modifier":{"type":"integer"}},"required":["character","power"]}`),
		EngineHook:  "power.use",
	},
	{
		ID:          ToolUpdateHealth,
		Label:       "Update health",
		Description: "Apply damage, healing, or set a health/wound resource.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"character":{"type":"string"},"resource":{"type":"string"},"delta":{"type":"integer"},"set":{"type":"integer"},"reason":{"type":"string"}},"required":["character"]}`),
		EngineHook:  "character.health",
	},
	{
		ID:          ToolApplyCondition,
		Label:       "Apply condition",
		Description: "Apply a status effect (poisoned, shaken, prone…).",
		Parameters: json.RawMessage(`{"type":"object","properties":{"character":{"type":"string"},"condition":{"type":"string"}},"required":["character","condition"]}`),
		EngineHook:  "character.condition.add",
	},
	{
		ID:          ToolRemoveCondition,
		Label:       "Remove condition",
		Description: "Remove a status effect from a character.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"character":{"type":"string"},"condition":{"type":"string"}},"required":["character","condition"]}`),
		EngineHook:  "character.condition.remove",
	},
	{
		ID:          ToolUpdateCharacter,
		Label:       "Update character",
		Description: "Record notes or tracked fields on a player character.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"character":{"type":"string"},"field":{"type":"string"},"value":{},"notes":{"type":"string"}},"required":["character"]}`),
		EngineHook:  "character.update",
	},
	{
		ID:          ToolRest,
		Label:       "Rest",
		Description: "Apply short or long rest recovery per the system rules.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"character":{"type":"string"},"kind":{"type":"string","enum":["short","long"]},"dice":{"type":"integer"}},"required":["character","kind"]}`),
		EngineHook:  "character.rest",
	},
	{
		ID:          ToolInitiative,
		Label:       "Roll initiative",
		Description: "Roll initiative or draw action cards for combat order.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"participants":{"type":"array","items":{"type":"string"}}},"required":["participants"]}`),
		EngineHook:  "combat.initiative",
	},
	{
		ID:          ToolLookupCreature,
		Label:       "Lookup creature",
		Description: "Fetch a standard creature/NPC stat block from an embedded bestiary if available.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`),
		EngineHook:  "bestiary.lookup",
	},
	{
		ID:          ToolAwardExperience,
		Label:       "Award experience",
		Description: "Grant experience, advances, or milestones.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"character":{"type":"string"},"amount":{"type":"integer"},"reason":{"type":"string"}},"required":["character","amount"]}`),
		EngineHook:  "character.xp",
	},
	{
		ID:          ToolInventoryAdd,
		Label:       "Add inventory item",
		Description: "Add gear, ammo, or treasure to a character sheet.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"character":{"type":"string"},"name":{"type":"string"},"quantity":{"type":"integer"}},"required":["character","name"]}`),
		EngineHook:  "character.inventory.add",
	},
	{
		ID:          ToolInventoryRemove,
		Label:       "Remove inventory item",
		Description: "Remove or spend items from a character sheet.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"character":{"type":"string"},"name":{"type":"string"},"quantity":{"type":"integer"}},"required":["character","name"]}`),
		EngineHook:  "character.inventory.remove",
	},
}

type CanonicalTool struct {
	ID          string
	Label       string
	Description string
	Parameters  json.RawMessage
	EngineHook  string
}

func canonicalByID(id string) (CanonicalTool, bool) {
	for _, t := range CanonicalTools {
		if t.ID == id {
			return t, true
		}
	}
	return CanonicalTool{}, false
}

func bindTool(canonicalID, name, description, engineHook, notes string, params json.RawMessage) ToolBinding {
	return ToolBinding{
		CanonicalID: canonicalID,
		Enabled:     true,
		Name:        name,
		Description: description,
		Parameters:  params,
		EngineHook:  engineHook,
		Notes:       notes,
	}
}
