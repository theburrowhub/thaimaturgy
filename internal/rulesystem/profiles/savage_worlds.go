package profiles

import (
	"github.com/theburrowhub/thaimaturgy/internal/rulesystem"
)

// SavageWorlds returns a rich SWADE (Savage Worlds Adventure Edition) pack.
func SavageWorlds() *rulesystem.Pack {
	p := NewBasePack("savage_worlds", "Savage Worlds Adventure Edition", "swade")
	p.Version = "5.0"
	p.Source.Templates = []string{"SWADE Core Rules"}
	p.Source.Notes = "Fast! Furious! Fun! — trait rolls, wild die, bennies, wounds."

	p.Dice = rulesystem.DiceConfig{
		Primary:   "trait",
		Common:    []string{"d4", "d6", "d8", "d10", "d12"},
		Notation:  "trait_die + wild_die",
		Exploding: true,
		Keep:      "wild_die_for_wildcards",
		Modifiers: []rulesystem.DiceModifier{
			{ID: "wild_die", Label: "Wild Die", Roll: "d6", Notes: "Extra d6 for Wild Cards; take best."},
		},
		Notes: "Target Number 4; raises every +4 over TN.",
	}

	for _, a := range []rulesystem.AttributeDef{
		{ID: "agi", Label: "Agility", Abbrev: "Agi", Scale: "die_step"},
		{ID: "sma", Label: "Smarts", Abbrev: "Sma", Scale: "die_step"},
		{ID: "spi", Label: "Spirit", Abbrev: "Spi", Scale: "die_step"},
		{ID: "str", Label: "Strength", Abbrev: "Str", Scale: "die_step"},
		{ID: "vig", Label: "Vigor", Abbrev: "Vig", Scale: "die_step"},
	} {
		AddAttribute(p, a)
	}

	skills := []struct{ id, label, attr string }{
		{"athletics", "Athletics", "agi"}, {"fighting", "Fighting", "agi"}, {"shooting", "Shooting", "agi"},
		{"stealth", "Stealth", "agi"}, {"thievery", "Thievery", "agi"},
		{"notice", "Notice", "sma"}, {"research", "Research", "sma"}, {"repair", "Repair", "sma"},
		{"taunt", "Taunt", "sma"}, {"persuasion", "Persuasion", "spi"}, {"intimidation", "Intimidation", "spi"},
		{"spellcasting", "Spellcasting", "spi"}, {"faith", "Faith", "spi"},
		{"survival", "Survival", "spi"}, {"healing", "Healing", "spi"},
		{"common_knowledge", "Common Knowledge", "sma"},
	}
	for _, s := range skills {
		AddSkill(p, rulesystem.SkillDef{ID: s.id, Label: s.label, Attribute: s.attr, Training: true})
	}

	AddResource(p, rulesystem.ResourceDef{
		ID: "wounds", Label: "Wounds", Kind: "track", Primary: true, Min: 0, MaxFormula: "3",
		Overflow: "incapacitated", TrackSteps: []string{"0", "1", "2", "3"},
	})
	AddResource(p, rulesystem.ResourceDef{
		ID: "fatigue", Label: "Fatigue", Kind: "track", Min: 0, MaxFormula: "2",
	})
	AddResource(p, rulesystem.ResourceDef{
		ID: "bennies", Label: "Bennies", Kind: "pool", Min: 0, ResetOn: []string{"session_start"},
	})
	AddResource(p, rulesystem.ResourceDef{
		ID: "power_points", Label: "Power Points", Kind: "pool", MaxFormula: "power_points_by_rank",
		ResetOn: []string{"hour_rest"},
	})

	conditions := []rulesystem.ConditionDef{
		{ID: "shaken", Label: "Shaken", Severity: 2, Effects: []string{"cannot_actions_except_unshake", "free_move_away"}, EndsOn: []string{"unshake_roll"}},
		{ID: "stunned", Label: "Stunned", Severity: 2, Effects: []string{"shaken_plus_no_reactions"}, EndsOn: []string{"end_of_next_turn"}},
		{ID: "bound", Label: "Bound", Severity: 3, Effects: []string{"cannot_move", "attacks_against_have_bonus"}},
		{ID: "entangled", Label: "Entangled", Severity: 2, Effects: []string{"cannot_move", "escape_roll_required"}},
		{ID: "vulnerable", Label: "Vulnerable", Severity: 2, Effects: []string{"enemies_gain_plus2_to_hit"}},
		{ID: "distracted", Label: "Distracted", Severity: 1, Effects: []string{"minus2_to_all_rolls"}},
		{ID: "incapacitated", Label: "Incapacitated", Severity: 5, Effects: []string{"out_of_fight", "roll_on_injury_table"}},
	}
	for _, c := range conditions {
		AddCondition(p, c)
	}

	for _, dt := range []string{"physical", "fire", "cold", "electricity", "poison", "holy", "magic"} {
		AddDamageType(p, dt, dt, "")
	}

	p.Equipment.WeaponCategories = []rulesystem.NamedDesc{
		{ID: "melee", Label: "Melee"}, {ID: "ranged", Label: "Ranged"},
	}
	AddEquipmentTemplate(p, rulesystem.ItemTemplate{
		ID: "long_sword", Label: "Long Sword", Kind: "weapon",
		Stats: map[string]string{"damage": "Str+d8", "notes": "Parry +1"},
	})
	AddEquipmentTemplate(p, rulesystem.ItemTemplate{
		ID: "leather", Label: "Leather Armor", Kind: "armor",
		Stats: map[string]string{"armor": "+2"},
	})

	p.Resolution = rulesystem.ResolutionConfig{
		SkillCheck: rulesystem.CheckRule{
			Roll: "trait+wild", Compare: ">=", Target: "4",
			Success: "meet_tn", Failure: "below_tn",
			Critical: "raise_every_4", Fumble: "double_1_on_both_dice", WorkflowID: "trait_roll",
		},
		AbilityCheck: rulesystem.CheckRule{Roll: "attribute+wild", Compare: ">=", Target: "4", WorkflowID: "trait_roll"},
		SavingThrow:  rulesystem.CheckRule{Roll: "attribute+wild", Compare: ">=", Target: "4", WorkflowID: "trait_roll"},
		OpposedCheck: rulesystem.OpposedRule{
			AttackerRoll: "trait+wild", DefenderRoll: "trait+wild", Win: "higher_or_more_raises",
		},
		Attack: rulesystem.AttackRule{
			Roll: "fighting_or_shooting+wild", Target: "4", Compare: ">=",
			OnHit: []string{"damage_roll", "possible_shaken"}, OnMiss: []string{"no_effect"},
			OnCrit: []string{"extra_raise_effects"}, WorkflowID: "attack",
		},
		Defense: rulesystem.DefenseRule{Stat: "parry", Formula: "2+half_fighting+shield"},
		Spell: rulesystem.PowerRule{
			Cost: "power_points", Roll: "spellcasting", Components: []string{"verbal", "somatic"},
			WorkflowID: "power",
		},
		Power:   rulesystem.PowerRule{Cost: "power_points", Roll: "arcane_skill", WorkflowID: "power"},
		Damage:  rulesystem.DamageRule{Roll: "Str+weapon", OnApply: []string{"soak_damage", "apply_shaken"}},
		Initiative: rulesystem.CheckRule{Roll: "notice_or_deal", Compare: "order_desc"},
		Death: rulesystem.DeathRule{
			AtZero:     []string{"incapacitated_not_dead"},
			Recovery:   []string{"natural_healing", "magical_healing"},
			Permanent:  "three_wounds_incapacitated",
			WorkflowID: "incapacitation",
		},
	}

	p.Combat = rulesystem.CombatModel{
		Mode: "fast_turn_based",
		RoundSteps: []string{
			"1. Deal action cards / roll initiative",
			"2. Act in order; 3 squares movement typical",
			"3. Resolve hits -> damage -> shaken/wounds -> soak",
			"4. Discard unused cards at end of round",
		},
		ActionEconomy: rulesystem.ActionEconomy{
			Actions: []rulesystem.ActionType{
				{ID: "action", Label: "Action", PerTurn: 1, Examples: []string{"Attack", "Cast power", "Run", "Test"}},
				{ID: "movement", Label: "Movement", Examples: []string{"Move your Pace", "Run with penalty"}},
				{ID: "free", Label: "Free", Examples: []string{"Drop item", "Speak", "Unshake attempt"}},
			},
			Notes: "Multi-action penalty: -2 per extra action.",
		},
		Positioning: "grid_or_abstract",
	}

	p.Magic = rulesystem.MagicModel{
		Traditions: []rulesystem.NamedDesc{
			{ID: "arcane", Label: "Arcane"}, {ID: "divine", Label: "Divine"},
			{ID: "psionics", Label: "Psionics"},
		},
		Casting:  []string{"Spend PP", "Roll arcane skill", "Maintained powers cost ongoing"},
		Recovery: []string{"1 PP per hour rest", "Full restore on daily rest for many settings"},
	}

	p.Progression = rulesystem.ProgressionModel{
		Kind: "advances",
		Levels: []rulesystem.LevelRow{
			{Level: 1, XP: 0, Advance: "Novice rank", Notes: "4 advances"},
			{Level: 2, XP: 4, Advance: "Seasoned rank"},
			{Level: 3, XP: 8, Advance: "Veteran rank"},
			{Level: 4, XP: 12, Advance: "Heroic rank"},
			{Level: 5, XP: 16, Advance: "Legendary rank"},
		},
		Milestones: []string{"5 XP per advance", "Rank gates power points and edges"},
	}

	AddFormula(p, rulesystem.FormulaDef{ID: "parry", Label: "Parry", Expression: "2 + fighting/2 + shield"})
	AddFormula(p, rulesystem.FormulaDef{ID: "toughness", Label: "Toughness", Expression: "2 + vigor/2 + armor"})

	workflows := []rulesystem.WorkflowDef{
		{ID: "trait_roll", Label: "Trait Roll", Category: "resolution",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("roll", "Roll trait + wild die", "roll", "trait+wild", "compare"),
				WorkflowStep("compare", "Success vs TN 4", "branch", "", "raises"),
				WorkflowStep("raises", "Count raises (+4 each)", "expression", "", ""),
			}, RelatedTools: []string{"skill_check", "roll_dice"}},
		{ID: "attack", Label: "Attack", Category: "combat",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("roll", "Fighting/Shooting roll", "roll", "trait+wild", "hit"),
				WorkflowStep("hit", "Raise on hit?", "branch", "", "damage"),
				WorkflowStep("damage", "Roll damage", "roll", "Str+weapon", "result"),
				WorkflowStep("result", "Shaken or wound", "branch", "", ""),
			}, RelatedTools: []string{"attack", "damage_roll"}},
		{ID: "soak", Label: "Soak Roll", Category: "combat", Trigger: "would_take_wound",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("offer_benny", "Offer benny reroll", "choice", "", "roll"),
				WorkflowStep("roll", "Roll Vigor", "roll", "vigor+wild", "result"),
				WorkflowStep("result", "Soak wound or take it", "branch", "", ""),
			}, RelatedTools: []string{"soak_damage", "spend_benny"}},
		{ID: "unshake", Label: "Unshake", Category: "combat", Trigger: "shaken",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("roll", "Spirit roll TN 4", "roll", "spi+wild", "clear"),
				WorkflowStep("clear", "Remove shaken on success", "state", "", ""),
			}, RelatedTools: []string{"remove_condition", "skill_check"}},
		{ID: "power", Label: "Activate Power", Category: "magic",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("pay", "Spend PP", "state", "", "roll"),
				WorkflowStep("roll", "Arcane skill roll", "roll", "spellcasting+wild", "effect"),
				WorkflowStep("effect", "Apply power", "effect", "", ""),
			}, RelatedTools: []string{"use_power", "cast_spell"}},
		{ID: "incapacitation", Label: "Incapacitation", Category: "combat", Trigger: "wounds_exceed_limit",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("check", "Wounds >= limit?", "branch", "", "table"),
				WorkflowStep("table", "Roll injury table", "roll", "1d20", ""),
			}, RelatedTools: []string{"roll_on_table", "apply_condition"}},
		{ID: "short_rest", Label: "Rest & Recovery", Category: "exploration",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("rest", "Rest period", "narrative", "", "heal"),
				WorkflowStep("heal", "Natural healing roll", "roll", "vigor", ""),
			}, RelatedTools: []string{"rest", "update_health"}},
		{ID: "benny", Label: "Spend Benny", Category: "meta",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("spend", "Spend benny", "state", "", "reroll"),
				WorkflowStep("reroll", "Reroll trait roll", "roll", "trait+wild", ""),
			}, RelatedTools: []string{"spend_benny", "draw_benny"}},
		{ID: "social", Label: "Social Test", Category: "social",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("roll", "Persuasion/Taunt/Intimidation", "roll", "trait+wild", ""),
			}, RelatedTools: []string{"social_conflict", "skill_check"}},
	}
	for _, wf := range workflows {
		AddWorkflow(p, wf)
	}

	mechanics := []rulesystem.MechanicDef{
		{ID: "wild_die", Label: "Wild Die", Category: "core", Summary: "Wild Cards roll skill/attribute die plus d6 wild; take best.", RelatedTools: []string{"roll_dice"}},
		{ID: "raises", Label: "Raises", Category: "core", Summary: "Every +4 over TN 4 is a raise for extra effects."},
		{ID: "bennies", Label: "Bennies", Category: "meta", Summary: "Spend bennies to reroll, soak, or resist.", RelatedTools: []string{"spend_benny", "draw_benny"}},
		{ID: "shaken", Label: "Shaken", Category: "combat", Summary: "Any hit without wound causes shaken; must unshake.", WorkflowID: "unshake", RelatedTools: []string{"apply_condition"}},
		{ID: "wounds", Label: "Wounds", Category: "combat", Summary: "Raises on damage cause wounds; soak with Vigor.", WorkflowID: "soak", RelatedTools: []string{"soak_damage"}},
		{ID: "multi_action", Label: "Multi-Action Penalty", Category: "combat", Summary: "-2 to all rolls when taking multiple actions."},
		{ID: "edges", Label: "Edges & Hindrances", Category: "character", Summary: "Edges grant bonuses; Hindrances impose limits or bennies."},
		{ID: "power_points", Label: "Power Points", Category: "magic", Summary: "Powers cost PP based on rank and modifiers.", WorkflowID: "power"},
		{ID: "tricks", Label: "Tricks & Tests of Will", Category: "combat", Summary: "Test opposed skills to distract or disarm."},
		{ID: "chase", Label: "Chase Rules", Category: "exploration", Summary: "Extended chase using opposed rolls."},
		{ID: "mass_battle", Label: "Mass Battle", Category: "combat", Summary: "Scale battles with tokens and battle die."},
	}
	for _, m := range mechanics {
		AddMechanic(p, m)
	}

	AddTable(p, rulesystem.TableDef{
		ID: "injury_incap", Label: "Incapacitation Injury", Roll: "1d20",
		Rows: []rulesystem.TableRow{
			{Key: "1-3", Result: "Minor scar"}, {Key: "4-6", Result: "Permanent limp"},
			{Key: "7-9", Result: "Broken limb"}, {Key: "10+", Result: "Serious injury"},
		},
	})
	AddTable(p, rulesystem.TableDef{
		ID: "random_encounter", Label: "Random Encounter Severity", Roll: "2d6",
		Rows: []rulesystem.TableRow{
			{Key: "2-4", Result: "Easy"}, {Key: "5-9", Result: "Medium"}, {Key: "10-12", Result: "Hard"},
		},
	})
	AddTable(p, rulesystem.TableDef{
		ID: "power_modifiers", Label: "Power Modifiers (sample)", Roll: "n/a",
		Columns: []string{"Modifier", "Cost"},
		Rows: []rulesystem.TableRow{
			{Key: "range", Result: "+1 PP"}, {Key: "duration", Result: "+2 PP"}, {Key: "area", Result: "+2 PP"},
		},
	})

	AddChapter(p, "combat", "Combat", "Fast cinematic combat with bennies.", []rulesystem.Section{
		Section("basics", "Basics", "Roll trait + wild vs TN 4.", "Raises every +4", "Shaken vs wounds"),
		Section("soak", "Soaking Wounds", "Vigor roll to soak; benny for reroll.", "Three wounds incapacitate Wild Cards"),
	}, "combat")
	AddChapter(p, "magic", "Powers", "Arcane and divine powers using PP.", []rulesystem.Section{
		Section("casting", "Activating Powers", "Spend PP, roll arcane skill.", "Maintained powers"),
	})
	AddChapter(p, "exploration", "Exploration", "Chases, travel, social challenges.", []rulesystem.Section{
		Section("tests", "Tests", "Standard trait rolls for challenges.", "Support rolls grant +1"),
	})
	AddChapter(p, "character", "Character", "Attributes, skills, edges, hindrances.", []rulesystem.Section{
		Section("creation", "Creation", "Five attributes as die steps.", "Skills start at d4 if untrained"),
		Section("advances", "Advances", "Spend XP for rank and advances.", "Every 4 advances increase rank"),
	})

	BindToolFromCanonical(p, "skill_check", "Trait Roll", "Roll attribute/skill + wild die vs TN.", "resolution", "trait_roll",
		nil, nil, []rulesystem.ToolExample{{Title: "Notice roll", Input: map[string]any{"skill": "notice", "actor_id": "wc_1"}, Output: "9 vs TN 4 — success with 1 raise"}})
	BindToolFromCanonical(p, "attack", "Attack", "Fighting or Shooting attack.", "combat", "attack", nil, nil, nil)
	BindToolFromCanonical(p, "damage_roll", "Damage", "Roll Str+weapon damage.", "combat", "attack", nil, nil, nil)
	BindToolFromCanonical(p, "soak_damage", "Soak Wound", "Vigor roll to avoid wound.", "combat", "soak",
		[]string{"would_take_wound"}, nil, nil)
	BindToolFromCanonical(p, "spend_benny", "Spend Benny", "Reroll or soak.", "meta", "benny", nil, nil, nil)
	BindToolFromCanonical(p, "draw_benny", "Draw Benny", "Refresh bennies at milestones.", "meta", "benny", nil, nil, nil)
	BindToolFromCanonical(p, "apply_condition", "Apply Shaken/Stunned", "Apply combat conditions.", "state", "unshake", nil, nil, nil)
	BindToolFromCanonical(p, "remove_condition", "Remove Condition", "Clear shaken etc.", "state", "unshake", nil, nil, nil)
	BindToolFromCanonical(p, "use_power", "Activate Power", "Spend PP and roll arcane skill.", "magic", "power", nil, nil, nil)
	BindToolFromCanonical(p, "cast_spell", "Cast Spell", "Alias for arcane/divine power.", "magic", "power", nil, nil, nil)
	BindToolFromCanonical(p, "update_health", "Adjust Wounds/Fatigue", "Modify wound/fatigue tracks.", "state", "", nil, nil, nil)
	BindToolFromCanonical(p, "initiative", "Initiative", "Deal cards or roll Notice.", "combat", "", nil, nil, nil)
	BindToolFromCanonical(p, "social_conflict", "Social Conflict", "Taunt, Persuasion, Intimidation.", "social", "social", nil, nil, nil)
	BindToolFromCanonical(p, "opposed_check", "Opposed Roll", "Opposed trait rolls.", "resolution", "trait_roll", nil, nil, nil)
	BindToolFromCanonical(p, "roll_on_table", "Roll on Table", "Injury/encounter tables.", "reference", "incapacitation", nil, nil, nil)
	BindToolFromCanonical(p, "rest", "Rest", "Natural healing and PP recovery.", "exploration", "short_rest", nil, nil, nil)
	BindToolFromCanonical(p, "award_experience", "Award XP", "Grant 1-3 XP for milestones.", "progression", "", nil, nil, nil)
	BindToolFromCanonical(p, "improve_skill", "Advance Skill", "Raise die step on advance.", "progression", "", nil, nil, nil)
	BindToolFromCanonical(p, "roll_dice", "Roll Dice", "Generic exploding dice.", "core", "", nil, nil, nil)
	BindToolFromCanonical(p, "lookup_creature", "Bestiary Lookup", "Fetch SWADE stat card.", "reference", "", nil, nil, nil)

	p.Character = rulesystem.CharacterSchema{
		Sections: []rulesystem.SchemaSection{
			{ID: "identity", Label: "Identity"}, {ID: "attributes", Label: "Attributes"}, {ID: "edges", Label: "Edges"},
		},
		Fields: []rulesystem.CharacterField{
			{ID: "name", Label: "Name", Kind: "string", Required: true},
			{ID: "rank", Label: "Rank", Kind: "string"},
			{ID: "wounds", Label: "Wounds", Kind: "resource"},
			{ID: "bennies", Label: "Bennies", Kind: "resource"},
			{ID: "parry", Label: "Parry", Kind: "number", Formula: "2 + fighting/2 + shield"},
			{ID: "toughness", Label: "Toughness", Kind: "number", Formula: "2 + vigor/2 + armor"},
		},
	}
	p.Creature = rulesystem.CreatureSchema{
		Fields: []rulesystem.CharacterField{
			{ID: "name", Label: "Name", Kind: "string"},
			{ID: "wild_card", Label: "Wild Card", Kind: "boolean"},
			{ID: "toughness", Label: "Toughness", Kind: "number"},
		},
		Templates: []rulesystem.CreatureTemplate{
			{ID: "orc", Label: "Orc", Stats: map[string]string{"toughness": "8", "fighting": "d8"}},
			{ID: "dragon", Label: "Young Dragon", Stats: map[string]string{"toughness": "15", "wild_card": "true"}},
		},
	}

	p.OracleGuide = rulesystem.OracleGuide{
		Principles: []string{
			"Always roll trait + wild die for Wild Cards.",
			"Apply shaken on any damaging hit without wound.",
			"Offer soak_damage and spend_benny when a wound is inflicted.",
		},
		ToolPriority: []string{"attack", "damage_roll", "soak_damage", "spend_benny", "skill_check"},
		AntiPatterns: []string{"Using d20 mechanics", "Ignoring wild die"},
		Scenarios: []rulesystem.GuideScenario{
			{Situation: "Wild Card shoots bandit", UseTools: []string{"attack", "damage_roll", "apply_condition"}, Avoid: []string{"auto_wound_without_raises"}},
			{Situation: "Hero would take wound", UseTools: []string{"soak_damage", "spend_benny"}, Avoid: []string{"skip_soak"}},
			{Situation: "Shaken warrior tries to act", UseTools: []string{"remove_condition"}, Avoid: []string{"full_actions_while_shaken"}},
			{Situation: "Milestone completed", UseTools: []string{"award_experience", "draw_benny"}, Avoid: []string{"forget_bennies"}},
		},
	}
	p.Compatibility = rulesystem.EngineCompat{
		CharacterType: "swade_character",
		StatBlockType: "swade_creature",
		ToolMap: map[string]string{
			"roll": "roll_dice", "trait": "skill_check", "attack": "attack",
			"benny": "spend_benny", "soak": "soak_damage", "power": "use_power",
		},
	}
	p.RulesSummary = []string{
		"Target Number 4; trait + wild die; raises every +4.",
		"Bennies for rerolls and soaking; shaken before wounds.",
		"Three wounds incapacitate Wild Cards.",
	}
	return p
}
