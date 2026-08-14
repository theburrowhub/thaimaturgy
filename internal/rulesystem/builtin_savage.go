package rulesystem

import "encoding/json"

// SavageWorlds returns a Savage Worlds Adventure Edition-inspired profile.
func SavageWorlds() *Pack {
	params := func(id string) json.RawMessage {
		t, _ := canonicalByID(id)
		return t.Parameters
	}
	return &Pack{
		APIVersion: APIVersion,
		ID:         "savage_worlds",
		Name:       "Savage Worlds",
		Version:    "SWADE-inspired starter",
		Language:   "en",
		Source: SourceMeta{
			Type:  "builtin",
			Notes: "Generic SWADE-shaped profile (traits, wild die, bennies, raises). Not a licensed reproduction of Pinnacle text.",
		},
		Dice: DiceConfig{
			Primary:  "trait_d6",
			Common:   []string{"d4", "d6", "d8", "d10", "d12"},
			Notation: "trait_plus_wild_die",
			Notes:    "Roll trait die and wild die (d6); keep highest. Aces (max on die) roll again and add.",
		},
		Attributes: []AttributeDef{
			{ID: "agility", Label: "Agility", Abbrev: "Agi", Scale: "die_type"},
			{ID: "smarts", Label: "Smarts", Abbrev: "Sma", Scale: "die_type"},
			{ID: "spirit", Label: "Spirit", Abbrev: "Spi", Scale: "die_type"},
			{ID: "strength", Label: "Strength", Abbrev: "Str", Scale: "die_type"},
			{ID: "vigor", Label: "Vigor", Abbrev: "Vig", Scale: "die_type"},
		},
		Skills: swadeSkills(),
		Resources: []ResourceDef{
			{ID: "wounds", Label: "Wounds", Kind: "track", Primary: true, Min: 0, DefaultMax: "3 (+1 per size category for extras)"},
			{ID: "fatigue", Label: "Fatigue", Kind: "track", Min: 0, DefaultMax: "2 (incapacitated at 3)"},
			{ID: "bennies", Label: "Bennies", Kind: "bennies", Notes: "Meta-currency for rerolls and soak."},
			{ID: "power_points", Label: "Power Points", Kind: "pool", Notes: "For arcane/divine/psionic powers."},
		},
		Conditions: []ConditionDef{
			{ID: "shaken", Label: "Shaken"},
			{ID: "stunned", Label: "Stunned"},
			{ID: "bound", Label: "Bound"},
			{ID: "entangled", Label: "Entangled"},
			{ID: "incapacitated", Label: "Incapacitated"},
		},
		Resolution: ResolutionConfig{
			SkillCheck: CheckRule{
				Roll: "trait_die + wild_die", Compare: "gte", Target: "4",
				Success: "highest die >= 4", Critical: "raise: each +4 above TN",
				Notes: "Wild cards roll wild die; extras roll single die.",
			},
			AbilityCheck: CheckRule{
				Roll: "attribute_die + wild_die", Compare: "gte", Target: "4",
			},
			Attack: AttackRule{
				Roll: "fighting_or_shooting + wild_die", Target: "4", Compare: "gte",
				Notes: "Hit causes shaken; raise adds wound (after soak).",
			},
			Defense: DefenseRule{Stat: "parry", Formula: "2 + half fighting + shield"},
			Spell: PowerRule{
				Cost: "power_points", Roll: "arcane_skill + wild_die vs 4",
				Notes: "Powers list PP cost and range/duration.",
			},
			Damage: DamageRule{
				Roll: "weapon_or_power_damage + raises", Notes: "Soak roll vigor + wild; wound if fail.",
			},
			Initiative: CheckRule{
				Roll: "action_card", Compare: "suit_and_rank", Notes: "Deal action cards each round; jokers go first.",
			},
		},
		Tools: []ToolBinding{
			bindTool(ToolRollDice, "roll_trait", "Roll a trait die plus wild die (keep highest, aces explode).", "dice.roll", "", params(ToolRollDice)),
			bindTool(ToolSkillCheck, "skill_check", "Roll a skill (linked attribute) vs target number 4.", "check.skill", "", params(ToolSkillCheck)),
			bindTool(ToolAbilityCheck, "attribute_check", "Roll a raw attribute vs 4.", "check.ability", "", params(ToolAbilityCheck)),
			bindTool(ToolAttack, "attack", "Resolve a fighting/shooting attack; apply shaken/wounds.", "combat.attack", "", params(ToolAttack)),
			bindTool(ToolUsePower, "use_power", "Activate a power spending power points.", "power.use", "", params(ToolUsePower)),
			bindTool(ToolUpdateHealth, "update_wounds", "Apply wounds, fatigue, or healing/soak outcomes.", "character.health", "", params(ToolUpdateHealth)),
			bindTool(ToolApplyCondition, "apply_condition", "Apply shaken, stunned, bound…", "character.condition.add", "", params(ToolApplyCondition)),
			bindTool(ToolRemoveCondition, "remove_condition", "Recover from conditions.", "character.condition.remove", "", params(ToolRemoveCondition)),
			bindTool(ToolUpdateCharacter, "update_character", "Track edges, hindrances, gear, and notes.", "character.update", "", params(ToolUpdateCharacter)),
			bindTool(ToolRest, "rest", "Natural healing: one wound level per day of rest.", "character.rest", "", params(ToolRest)),
			bindTool(ToolInitiative, "initiative", "Deal action cards for the round.", "combat.initiative", "", params(ToolInitiative)),
			bindTool(ToolAwardExperience, "award_advance", "Grant advances or rank-ups.", "character.xp", "SW uses advances rather than XP in many settings.", params(ToolAwardExperience)),
			bindTool(ToolInventoryAdd, "add_gear", "Add gear with weight/value.", "character.inventory.add", "", params(ToolInventoryAdd)),
			bindTool(ToolInventoryRemove, "remove_gear", "Remove or consume gear.", "character.inventory.remove", "", params(ToolInventoryRemove)),
		},
		Character: CharacterSchema{
			Fields: []CharacterField{
				{ID: "name", Label: "Name", Kind: "string", Required: true},
				{ID: "rank", Label: "Rank", Kind: "string"},
				{ID: "attributes", Label: "Attributes (die steps)", Kind: "list", Required: true},
				{ID: "skills", Label: "Skills (die steps)", Kind: "list", Required: true},
				{ID: "edges", Label: "Edges", Kind: "list"},
				{ID: "hindrances", Label: "Hindrances", Kind: "list"},
				{ID: "wounds", Label: "Wounds", Kind: "resource", Required: true},
				{ID: "fatigue", Label: "Fatigue", Kind: "resource"},
				{ID: "bennies", Label: "Bennies", Kind: "resource"},
				{ID: "powers", Label: "Powers", Kind: "list"},
				{ID: "gear", Label: "Gear", Kind: "list"},
			},
		},
		Prompts: PromptBundle{
			OracleContext: "This adventure uses Savage Worlds (SWADE-style). Attributes and skills are dice steps (d4–d12). Wild cards roll a trait die plus a wild d6 and keep the highest; aces explode. Target number is usually 4. Success with a raise (+4) improves outcomes. Bennies allow rerolls and soak. Wounds and fatigue track harm; shaken is common.",
			DMNotes:       "Not wired into thAImaturgy engine yet — pack generation only.",
		},
		RulesSummary: []string{
			"Trait + wild die vs TN 4; each raise is +4 over TN.",
			"Shaken on hit; extra raise may cause wound after soak.",
			"Bennies reroll or soak one wound.",
			"Initiative via action cards each round.",
		},
		Metadata: map[string]string{
			"family": "savage_worlds",
		},
	}
}

func swadeSkills() []SkillDef {
	type skill struct {
		label, attr string
	}
	list := []skill{
		{"Athletics", "agility"}, {"Fighting", "agility"}, {"Shooting", "agility"},
		{"Stealth", "agility"}, {"Thievery", "agility"}, {"Notice", "smarts"},
		{"Research", "smarts"}, {"Repair", "smarts"}, {"Spellcasting", "smarts"},
		{"Taunt", "smarts"}, {"Intimidation", "spirit"}, {"Persuasion", "spirit"},
		{"Survival", "spirit"}, {"Healing", "spirit"}, {"Boating", "strength"},
		{"Driving", "agility"}, {"Piloting", "agility"}, {"Common Knowledge", "smarts"},
	}
	out := make([]SkillDef, 0, len(list))
	for _, s := range list {
		out = append(out, SkillDef{ID: toID(s.label), Label: s.label, Attribute: s.attr, Training: true})
	}
	return out
}
