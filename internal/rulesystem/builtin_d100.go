package rulesystem

import "encoding/json"

// D100 returns a percentile / Basic Roleplaying-style profile (Call of Cthulhu,
// RuneQuest family). Skills are percentages; combat uses opposing rolls.
func D100() *Pack {
	params := func(id string) json.RawMessage {
		t, _ := canonicalByID(id)
		return t.Parameters
	}
	return &Pack{
		APIVersion: APIVersion,
		ID:         "d100",
		Name:       "Percentile (d100) System",
		Version:    "BRP-inspired starter",
		Language:   "en",
		Source: SourceMeta{
			Type:  "builtin",
			Notes: "Generic d100 profile for percentile skill systems (CoC, RuneQuest, BRP). Not a licensed reproduction of any single publisher text.",
		},
		Dice: DiceConfig{
			Primary:  "d100",
			Common:   []string{"d3", "d4", "d6", "d8", "d10", "d12", "d20", "d100"},
			Notation: "percentile",
			Notes:    "Roll d100 (or 1d100) aiming to roll under skill value unless noted. Hard/Extreme success tiers optional.",
		},
		Attributes: []AttributeDef{
			{ID: "str", Label: "Strength", Abbrev: "STR", Scale: "score"},
			{ID: "con", Label: "Constitution", Abbrev: "CON", Scale: "score"},
			{ID: "siz", Label: "Size", Abbrev: "SIZ", Scale: "score"},
			{ID: "dex", Label: "Dexterity", Abbrev: "DEX", Scale: "score"},
			{ID: "app", Label: "Appearance", Abbrev: "APP", Scale: "score"},
			{ID: "int", Label: "Intelligence", Abbrev: "INT", Scale: "score"},
			{ID: "pow", Label: "Power", Abbrev: "POW", Scale: "score"},
			{ID: "edu", Label: "Education", Abbrev: "EDU", Scale: "score"},
		},
		Skills: d100Skills(),
		Resources: []ResourceDef{
			{ID: "hp", Label: "Hit Points", Kind: "pool", Primary: true, Min: 0, DefaultMax: "(con + siz) / 2"},
			{ID: "mp", Label: "Magic Points", Kind: "pool", DefaultMax: "pow"},
			{ID: "san", Label: "Sanity", Kind: "pool", Notes: "Optional; common in horror variants."},
			{ID: "luck", Label: "Luck", Kind: "pool", Notes: "Optional spendable luck points."},
		},
		Conditions: []ConditionDef{
			{ID: "unconscious", Label: "Unconscious"},
			{ID: "dying", Label: "Dying"},
			{ID: "major_wound", Label: "Major wound"},
			{ID: "insane", Label: "Insane"},
		},
		Resolution: ResolutionConfig{
			SkillCheck: CheckRule{
				Roll: "1d100", Compare: "under", Target: "skill_value",
				Success: "roll <= skill", Critical: "roll <= skill/5", Fumble: "roll >= 96",
				Notes: "Opposed checks: both roll under skill; lower successful roll wins.",
			},
			AbilityCheck: CheckRule{
				Roll: "1d100", Compare: "under", Target: "characteristic_x5",
				Notes: "Untrained characteristic roll often uses score x5 as target.",
			},
			Attack: AttackRule{
				Roll: "1d100", Target: "fighting_skill", Compare: "under",
				Notes: "Opposed parry/dodge may apply; impale/critical on low rolls.",
			},
			Defense: DefenseRule{Stat: "armor_points", Formula: "location-based armor reduces damage"},
			Spell: PowerRule{
				Cost: "magic_points", Roll: "spell skill or POW vs POW", Notes: "Casting often costs MP and requires a skill roll.",
			},
			Damage: DamageRule{Roll: "weapon_damage_dice - armor", Notes: "Major wounds when single hit exceeds half max HP."},
			Initiative: CheckRule{Roll: "1d10 + dex_bonus", Compare: "descending_order"},
		},
		Tools: []ToolBinding{
			bindTool(ToolRollDice, "roll_dice", "Roll dice (d100 for skills, damage dice as needed).", "dice.roll", "", params(ToolRollDice)),
			bindTool(ToolSkillCheck, "skill_check", "Roll d100 under a skill value (with optional difficulty tiers).", "check.skill", "", params(ToolSkillCheck)),
			bindTool(ToolAbilityCheck, "characteristic_check", "Roll d100 under a characteristic x5 value.", "check.ability", "", params(ToolAbilityCheck)),
			bindTool(ToolAttack, "attack", "Resolve a fighting attack with opposed defense if applicable.", "combat.attack", "", params(ToolAttack)),
			bindTool(ToolCastSpell, "cast_spell", "Spend MP and roll spell casting skill.", "magic.cast", "", params(ToolCastSpell)),
			bindTool(ToolUpdateHealth, "update_health", "Apply damage, healing, or major wound tracking.", "character.health", "", params(ToolUpdateHealth)),
			bindTool(ToolApplyCondition, "apply_condition", "Apply shock, unconscious, insanity, etc.", "character.condition.add", "", params(ToolApplyCondition)),
			bindTool(ToolRemoveCondition, "remove_condition", "Remove a condition when recovered.", "character.condition.remove", "", params(ToolRemoveCondition)),
			bindTool(ToolUpdateCharacter, "update_character", "Track characteristics, skills, and notes.", "character.update", "", params(ToolUpdateCharacter)),
			bindTool(ToolRest, "rest", "Natural healing over days/weeks of rest.", "character.rest", "Recovery is slower than D&D.", params(ToolRest)),
			bindTool(ToolInitiative, "initiative", "Set combat order.", "combat.initiative", "", params(ToolInitiative)),
			bindTool(ToolInventoryAdd, "add_gear", "Add equipment or tomes.", "character.inventory.add", "", params(ToolInventoryAdd)),
			bindTool(ToolInventoryRemove, "remove_gear", "Lose or spend gear.", "character.inventory.remove", "", params(ToolInventoryRemove)),
		},
		Character: CharacterSchema{
			Fields: []CharacterField{
				{ID: "name", Label: "Name", Kind: "string", Required: true},
				{ID: "occupation", Label: "Occupation", Kind: "string"},
				{ID: "characteristics", Label: "Characteristics", Kind: "list", Required: true},
				{ID: "skills", Label: "Skills (percentile)", Kind: "list", Required: true},
				{ID: "hp", Label: "Hit points", Kind: "resource", Required: true},
				{ID: "mp", Label: "Magic points", Kind: "resource"},
				{ID: "san", Label: "Sanity", Kind: "resource"},
				{ID: "armor", Label: "Armor by location", Kind: "list"},
				{ID: "weapons", Label: "Weapons", Kind: "list"},
			},
		},
		Prompts: PromptBundle{
			OracleContext: "This adventure uses a percentile d100 ruleset. Skills are percentages: roll d100 equal to or under the skill to succeed. Characteristics are typically 3–18 (or 3d6). Combat uses fighting skills, opposed parry/dodge, armor points, and location-based hits. Magic costs magic points. Prefer BRP/CoC terminology unless the module specifies otherwise.",
			DMNotes:       "Not wired into thAImaturgy engine yet — pack generation only.",
		},
		RulesSummary: []string{
			"Roll d100 under skill value to succeed.",
			"Hard success at half skill; extreme at fifth (optional tiers).",
			"HP often (CON+SIZ)/2; major wound if single hit > half max HP.",
			"Sanity and luck are optional horror resources.",
		},
		Metadata: map[string]string{
			"family": "d100",
		},
	}
}

func d100Skills() []SkillDef {
	labels := []string{
		"Accounting", "Anthropology", "Archaeology", "Art/Craft", "Charm", "Climb",
		"Credit Rating", "Disguise", "Dodge", "Drive Auto", "Fast Talk", "Fighting",
		"Firearms", "First Aid", "History", "Insight", "Intimidate", "Jump", "Language",
		"Law", "Library Use", "Listen", "Locksmith", "Mechanical Repair", "Medicine",
		"Natural World", "Navigate", "Occult", "Operate Heavy Machinery", "Persuade",
		"Pilot", "Psychology", "Psychoanalysis", "Ride", "Science", "Sleight of Hand",
		"Spot Hidden", "Stealth", "Survival", "Swim", "Throw", "Track",
	}
	out := make([]SkillDef, 0, len(labels))
	for _, l := range labels {
		out = append(out, SkillDef{ID: toID(l), Label: l, Training: true})
	}
	return out
}
