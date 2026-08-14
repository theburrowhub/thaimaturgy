package rulesystem

import "encoding/json"

func DnD5e() *Pack {
	params := func(id string) json.RawMessage {
		t, _ := canonicalByID(id)
		return t.Parameters
	}
	return &Pack{
		APIVersion: APIVersion,
		ID:         "dnd5e",
		Name:       "Dungeons & Dragons 5th Edition",
		Version:    "5.24 (SRD subset)",
		Language:   "en",
		Source: SourceMeta{
			Type:  "builtin",
			Notes: "Starter profile aligned with thAImaturgy's current D&D 5e assumptions (d20, six abilities, HP, spell slots).",
		},
		Dice: DiceConfig{
			Primary:  "d20",
			Common:   []string{"d4", "d6", "d8", "d10", "d12", "d20", "d100"},
			Notation: "standard",
			Notes:    "Advantage/disadvantage roll 2d20 and keep highest/lowest.",
		},
		Attributes: []AttributeDef{
			{ID: "str", Label: "Strength", Abbrev: "STR", Scale: "score"},
			{ID: "dex", Label: "Dexterity", Abbrev: "DEX", Scale: "score"},
			{ID: "con", Label: "Constitution", Abbrev: "CON", Scale: "score"},
			{ID: "int", Label: "Intelligence", Abbrev: "INT", Scale: "score"},
			{ID: "wis", Label: "Wisdom", Abbrev: "WIS", Scale: "score"},
			{ID: "cha", Label: "Charisma", Abbrev: "CHA", Scale: "score"},
		},
		Skills: dnd5eSkills(),
		Resources: []ResourceDef{
			{ID: "hp", Label: "Hit Points", Kind: "pool", Primary: true, Min: 0, DefaultMax: "class_hit_die + con_mod * level"},
			{ID: "spell_slots", Label: "Spell Slots", Kind: "track", Notes: "Per spell level, restored on long rest for most casters."},
			{ID: "hit_dice", Label: "Hit Dice", Kind: "counter", Notes: "Spent on short rest to recover HP."},
			{ID: "gold", Label: "Gold", Kind: "counter"},
		},
		Conditions: dnd5eConditions(),
		Resolution: ResolutionConfig{
			SkillCheck: CheckRule{
				Roll: "1d20 + skill_modifier", Compare: "gte", Target: "dc",
				Success: "total >= dc", Critical: "natural 20", Fumble: "natural 1",
				Notes: "Proficiency adds PB when proficient; expertise doubles PB.",
			},
			AbilityCheck: CheckRule{
				Roll: "1d20 + ability_modifier", Compare: "gte", Target: "dc",
			},
			Attack: AttackRule{
				Roll: "1d20 + attack_bonus", Target: "ac", Compare: "gte",
				Notes: "Melee usually STR/DEX; ranged DEX; spells use spell attack bonus.",
			},
			Defense: DefenseRule{Stat: "ac", Formula: "10 + dex_mod + armor + shield + misc"},
			Spell: PowerRule{
				Cost: "spell_slot", Roll: "varies by spell", Notes: "Cantrips cost no slot.",
			},
			Damage: DamageRule{Roll: "weapon_or_spell_dice + mod", Type: "bludgeoning|piercing|slashing|elemental…"},
			Initiative: CheckRule{Roll: "1d20 + dex_mod", Compare: "descending_order"},
		},
		Tools: []ToolBinding{
			bindTool(ToolRollDice, "roll_dice", "Roll dice in standard notation (e.g. 1d20, 2d6+3).", "dice.roll", "", params(ToolRollDice)),
			bindTool(ToolAbilityCheck, "ability_check", "Roll a d20 ability check against a DC.", "check.ability", "", params(ToolAbilityCheck)),
			bindTool(ToolSkillCheck, "skill_check", "Roll a d20 skill check with proficiency if applicable.", "check.skill", "", params(ToolSkillCheck)),
			bindTool(ToolAttack, "attack", "Resolve an attack roll vs AC and apply damage on a hit.", "combat.attack", "", params(ToolAttack)),
			bindTool(ToolCastSpell, "cast_spell", "Expend a spell slot and resolve a spell.", "magic.cast", "", params(ToolCastSpell)),
			bindTool(ToolUpdateHealth, "update_hp", "Change current HP via damage (negative delta) or healing.", "character.health", "Maps to update_hp in virtual-DM mode today.", params(ToolUpdateHealth)),
			bindTool(ToolApplyCondition, "set_condition", "Apply a D&D 5e condition (Poisoned, Prone…).", "character.condition.add", "", params(ToolApplyCondition)),
			bindTool(ToolRemoveCondition, "remove_condition", "Remove a condition.", "character.condition.remove", "", params(ToolRemoveCondition)),
			bindTool(ToolRest, "rest", "Apply short or long rest recovery.", "character.rest", "", params(ToolRest)),
			bindTool(ToolInitiative, "initiative", "Roll initiative for combatants.", "combat.initiative", "", params(ToolInitiative)),
			bindTool(ToolLookupCreature, "lookup_creature", "Look up an SRD creature stat block by name.", "bestiary.lookup", "Uses internal/srd today.", params(ToolLookupCreature)),
			bindTool(ToolAwardExperience, "award_xp", "Grant XP to a character.", "character.xp", "", params(ToolAwardExperience)),
			bindTool(ToolInventoryAdd, "add_item", "Add an item to inventory.", "character.inventory.add", "", params(ToolInventoryAdd)),
			bindTool(ToolInventoryRemove, "remove_item", "Remove items from inventory.", "character.inventory.remove", "", params(ToolInventoryRemove)),
			bindTool(ToolUpdateCharacter, "update_party_member", "Track HP/AC/notes for a party member.", "character.update", "Maps to update_party_member today.", params(ToolUpdateCharacter)),
		},
		Character: CharacterSchema{
			Fields: []CharacterField{
				{ID: "name", Label: "Name", Kind: "string", Required: true},
				{ID: "race", Label: "Species/Race", Kind: "string"},
				{ID: "class", Label: "Class", Kind: "string", Required: true},
				{ID: "level", Label: "Level", Kind: "int", Required: true},
				{ID: "abilities", Label: "Ability scores", Kind: "list"},
				{ID: "hp", Label: "Hit points", Kind: "resource", Required: true},
				{ID: "ac", Label: "Armor class", Kind: "int", Required: true},
				{ID: "proficiency_bonus", Label: "Proficiency bonus", Kind: "int"},
				{ID: "spells", Label: "Spellbook", Kind: "list"},
				{ID: "inventory", Label: "Inventory", Kind: "list"},
				{ID: "conditions", Label: "Conditions", Kind: "list"},
			},
			Notes: "Matches domain.Character in internal/domain/character.go.",
		},
		Prompts: PromptBundle{
			OracleContext: "This adventure uses Dungeons & Dragons 5th Edition. Resolve checks with d20 + modifier vs DC. Attacks target AC. Use six abilities (STR, DEX, CON, INT, WIS, CHA), proficiency bonus, spell slots for casters, and standard 5e conditions. Prefer SRD terminology.",
			DMNotes:       "Current thAImaturgy engine tools (update_hp, ability_check, lookup_creature) are 5e-shaped.",
		},
		RulesSummary: []string{
			"d20 + modifier vs DC for checks and saves.",
			"Attack rolls vs Armor Class; crit on natural 20.",
			"HP reach 0 → death saves unless instant kill.",
			"Short rest spends hit dice; long rest restores HP, slots, and half hit dice.",
		},
		Metadata: map[string]string{
			"family": "d20",
			"srd":    "yes",
		},
	}
}

func dnd5eSkills() []SkillDef {
	names := []struct {
		id, label, attr string
	}{
		{"acrobatics", "Acrobatics", "dex"},
		{"animal_handling", "Animal Handling", "wis"},
		{"arcana", "Arcana", "int"},
		{"athletics", "Athletics", "str"},
		{"deception", "Deception", "cha"},
		{"history", "History", "int"},
		{"insight", "Insight", "wis"},
		{"intimidation", "Intimidation", "cha"},
		{"investigation", "Investigation", "int"},
		{"medicine", "Medicine", "wis"},
		{"nature", "Nature", "int"},
		{"perception", "Perception", "wis"},
		{"performance", "Performance", "cha"},
		{"persuasion", "Persuasion", "cha"},
		{"religion", "Religion", "int"},
		{"sleight_of_hand", "Sleight of Hand", "dex"},
		{"stealth", "Stealth", "dex"},
		{"survival", "Survival", "wis"},
	}
	out := make([]SkillDef, 0, len(names))
	for _, s := range names {
		out = append(out, SkillDef{ID: s.id, Label: s.label, Attribute: s.attr, Training: true})
	}
	return out
}

func dnd5eConditions() []ConditionDef {
	labels := []string{
		"Blinded", "Charmed", "Deafened", "Exhaustion", "Frightened", "Grappled",
		"Incapacitated", "Invisible", "Paralyzed", "Petrified", "Poisoned", "Prone",
		"Restrained", "Stunned", "Unconscious",
	}
	out := make([]ConditionDef, 0, len(labels))
	for _, l := range labels {
		out = append(out, ConditionDef{ID: toID(l), Label: l})
	}
	return out
}
