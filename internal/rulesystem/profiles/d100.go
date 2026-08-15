package profiles

import (
	"github.com/theburrowhub/thaimaturgy/internal/rulesystem"
)

// D100 returns a rich d100 / BRP-style percentile system pack.
func D100() *rulesystem.Pack {
	p := NewBasePack("d100", "Basic Role-Playing (d100)", "percentile")
	p.Version = "7.5"
	p.Source.Templates = []string{"BRP", "CoC7"}
	p.Source.Notes = "Percentile skill system with opposed rolls, sanity, and major wounds."

	p.Dice = rulesystem.DiceConfig{
		Primary:  "d100",
		Common:   []string{"d10", "d100", "d6"},
		Notation: "d100 <= skill",
		Notes:    "Roll under skill for success; opposed rolls compare levels of success.",
	}

	// Attributes (8 classic BRP stats)
	for _, a := range []rulesystem.AttributeDef{
		{ID: "str", Label: "Strength", Abbrev: "STR", Scale: "3d6*5", Min: 1, Max: 100},
		{ID: "con", Label: "Constitution", Abbrev: "CON", Scale: "3d6*5", Min: 1, Max: 100},
		{ID: "siz", Label: "Size", Abbrev: "SIZ", Scale: "2d6+6*5", Min: 1, Max: 100},
		{ID: "dex", Label: "Dexterity", Abbrev: "DEX", Scale: "3d6*5", Min: 1, Max: 100},
		{ID: "app", Label: "Appearance", Abbrev: "APP", Scale: "3d6*5", Min: 1, Max: 100},
		{ID: "int", Label: "Intelligence", Abbrev: "INT", Scale: "2d6+6*5", Min: 1, Max: 100},
		{ID: "pow", Label: "Power", Abbrev: "POW", Scale: "3d6*5", Min: 1, Max: 100},
		{ID: "edu", Label: "Education", Abbrev: "EDU", Scale: "2d6+6*5", Min: 1, Max: 100},
	} {
		AddAttribute(p, a)
	}

	// Skills (representative set)
	skills := []struct{ id, label, attr string }{
		{"dodge", "Dodge", "dex"}, {"spot_hidden", "Spot Hidden", "int"}, {"listen", "Listen", "int"},
		{"persuade", "Persuade", "app"}, {"fast_talk", "Fast Talk", "app"}, {"intimidate", "Intimidate", "app"},
		{"psychology", "Psychology", "int"}, {"medicine", "Medicine", "edu"}, {"first_aid", "First Aid", "edu"},
		{"climb", "Climb", "str"}, {"swim", "Swim", "str"}, {"stealth", "Stealth", "dex"},
		{"firearms_handgun", "Firearms (Handgun)", "dex"}, {"fighting_brawl", "Fighting (Brawl)", "str"},
		{"occult", "Occult", "edu"}, {"library_use", "Library Use", "edu"}, {"credit_rating", "Credit Rating", "edu"},
		{"sanity", "Sanity", "pow"},
	}
	for _, s := range skills {
		AddSkill(p, rulesystem.SkillDef{ID: s.id, Label: s.label, Attribute: s.attr, Training: true, Default: 0})
	}

	AddResource(p, rulesystem.ResourceDef{
		ID: "hp", Label: "Hit Points", Kind: "pool", Primary: true, Min: 0,
		MaxFormula: "(con + siz) / 10",
	})
	AddResource(p, rulesystem.ResourceDef{
		ID: "sanity", Label: "Sanity Points", Kind: "pool",
		MaxFormula: "pow", ResetOn: []string{"therapy", "scenario_end"},
	})
	AddResource(p, rulesystem.ResourceDef{
		ID: "magic_points", Label: "Magic Points", Kind: "pool", MaxFormula: "pow / 5",
	})
	AddResource(p, rulesystem.ResourceDef{
		ID: "major_wound", Label: "Major Wound Track", Kind: "flag",
	})

	conditions := []rulesystem.ConditionDef{
		{ID: "unconscious", Label: "Unconscious", Severity: 4, Effects: []string{"cannot_act", "prone"}},
		{ID: "major_wound", Label: "Major Wound", Severity: 4, Effects: []string{"incapacitating_injury", "requires_first_aid_or_medicine"}},
		{ID: "temp_insanity", Label: "Temporary Insanity", Severity: 3, Effects: []string{"flee_or_faint", "duration_1d10_hours"}},
		{ID: "indef_insanity", Label: "Indefinite Insanity", Severity: 5, Effects: []string{"long_term_disorder"}},
		{ID: "prone", Label: "Prone", Severity: 1, Effects: []string{"melee_defense_harder"}},
		{ID: "bleeding", Label: "Bleeding", Severity: 2, Effects: []string{"lose_1_hp_per_round_until_treated"}},
	}
	for _, c := range conditions {
		AddCondition(p, c)
	}

	for _, dt := range []string{"bullet", "cutting", "blunt", "fire", "cold", "electric", "poison"} {
		AddDamageType(p, dt, dt, "")
	}

	p.Resolution = rulesystem.ResolutionConfig{
		SkillCheck: rulesystem.CheckRule{
			Roll: "1d100", Compare: "<=", Target: "skill_value",
			Success: "roll_under_skill", Failure: "roll_over_skill",
			Critical: "skill/20", Fumble: "96-100", WorkflowID: "skill_check",
		},
		AbilityCheck: rulesystem.CheckRule{
			Roll: "1d100", Compare: "<=", Target: "stat*5", WorkflowID: "skill_check",
		},
		SavingThrow: rulesystem.CheckRule{
			Roll: "1d100", Compare: "<=", Target: "skill_or_luck", WorkflowID: "skill_check",
		},
		OpposedCheck: rulesystem.OpposedRule{
			AttackerRoll: "1d100", DefenderRoll: "1d100",
			Win: "better_level_of_success", Tie: "higher_skill_wins",
			Notes: "Compare critical/success/fail/fumble levels.",
		},
		Defense: rulesystem.DefenseRule{Stat: "dodge_or_fighting", Alternate: []string{"dodge_skill", "fighting_parry"}},
		Spell:   rulesystem.PowerRule{Cost: "magic_points", Roll: "1d100<=casting", WorkflowID: "spell_cast"},
		Damage: rulesystem.DamageRule{Roll: "weapon_damage_dice", OnApply: []string{"update_health", "major_wound_check"}},
		Initiative: rulesystem.CheckRule{Roll: "1d10+dx_bonus", Compare: "order_desc"},
		Death: rulesystem.DeathRule{
			AtZero: []string{"unconscious", "dying_roll_each_round"},
			Recovery: []string{"first_aid", "medicine", "hospitalization"},
			WorkflowID: "major_wound",
		},
	}
	// Fix Attack - I used wrong helper, set directly
	p.Resolution.Attack = rulesystem.AttackRule{
		Roll: "1d100<=fighting_or_firearms", Target: "defense_value", Compare: "<=",
		OnHit: []string{"damage_roll", "major_wound_if_damage_ge_siz"}, WorkflowID: "attack",
	}

	p.Combat = rulesystem.CombatModel{
		Mode: "declared_actions",
		RoundSteps: []string{
			"1. Declare actions (lowest DEX first declaration in some variants)",
			"2. Resolve in DEX order",
			"3. Apply damage, major wounds, sanity",
		},
		ActionEconomy: rulesystem.ActionEconomy{
			Actions: []rulesystem.ActionType{
				{ID: "action", Label: "Action", PerTurn: 1, Examples: []string{"Attack", "Cast spell", "Skill use"}},
				{ID: "reaction", Label: "Reaction", PerTurn: 1, Examples: []string{"Dodge"}},
			},
		},
	}

	p.Magic = rulesystem.MagicModel{
		Traditions: []rulesystem.NamedDesc{{ID: "mythos", Label: "Mythos Magic"}, {ID: "folk", Label: "Folk Magic"}},
		Casting:    []string{"Spend MP", "Roll casting skill", "Sanity loss on mythos spells"},
	}

	p.Social = rulesystem.SocialModel{
		Conflicts: []rulesystem.NamedDesc{{ID: "debate", Label: "Debate"}, {ID: "bargain", Label: "Bargain"}},
	}

	p.Progression = rulesystem.ProgressionModel{
		Kind: "skill_improvement",
		Levels: []rulesystem.LevelRow{
			{Level: 1, Advance: "character_creation"},
			{Level: 2, Advance: "session_skill_checks"},
			{Level: 3, Advance: "improve_used_skills"},
			{Level: 4, Advance: "sanity_threshold_changes"},
			{Level: 5, Advance: "expertise_specialization"},
		},
		Milestones: []string{"End of scenario skill improvement", "Successful use during play"},
	}

	AddFormula(p, rulesystem.FormulaDef{ID: "max_hp", Label: "Max HP", Expression: "(con + siz) / 10", Variables: map[string]string{"con": "CON", "siz": "SIZ"}})
	AddFormula(p, rulesystem.FormulaDef{ID: "sanity_max", Label: "Sanity Max", Expression: "pow"})

	workflows := []rulesystem.WorkflowDef{
		{ID: "skill_check", Label: "Skill Check", Category: "resolution",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("roll", "Roll d100", "roll", "1d100", "compare"),
				WorkflowStep("compare", "Under skill?", "branch", "", ""),
			}, RelatedTools: []string{"skill_check", "roll_dice"}},
		{ID: "opposed", Label: "Opposed Roll", Category: "resolution",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("attacker", "Attacker rolls", "roll", "1d100", "defender"),
				WorkflowStep("defender", "Defender rolls", "roll", "1d100", "compare"),
				WorkflowStep("compare", "Level of success", "branch", "", ""),
			}, RelatedTools: []string{"opposed_check"}},
		{ID: "attack", Label: "Attack", Category: "combat",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("attack_roll", "Fighting/Firearms roll", "roll", "1d100", "damage"),
				WorkflowStep("damage", "Roll damage", "roll", "weapon_dice", "wound"),
				WorkflowStep("wound", "Check major wound", "branch", "", ""),
			}, RelatedTools: []string{"attack", "damage_roll"}},
		{ID: "major_wound", Label: "Major Wound", Category: "combat", Trigger: "damage_ge_siz",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("check", "Damage >= SIZ?", "branch", "", "effect"),
				WorkflowStep("effect", "Apply major wound", "effect", "", ""),
			}, RelatedTools: []string{"apply_condition", "update_health"}},
		{ID: "sanity", Label: "Sanity Test", Category: "horror", Trigger: "mythos_encounter",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("roll", "Roll SAN loss", "roll", "d100_sanity_loss", "threshold"),
				WorkflowStep("threshold", "Check temp/indef insanity", "branch", "", ""),
			}, RelatedTools: []string{"fear_sanity", "update_health"}},
		{ID: "spell_cast", Label: "Cast Spell", Category: "magic",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("cost", "Pay MP", "state", "", "roll"),
				WorkflowStep("roll", "Casting roll", "roll", "1d100", "effect"),
				WorkflowStep("effect", "Resolve spell", "effect", "", ""),
			}, RelatedTools: []string{"cast_spell", "use_power"}},
		{ID: "fear", Label: "Fear Response", Category: "horror",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("roll", "Roll fear/sanity", "roll", "1d100", "outcome"),
				WorkflowStep("outcome", "Apply fear effect", "branch", "", ""),
			}, RelatedTools: []string{"fear_sanity"}},
		{ID: "improve", Label: "Skill Improvement", Category: "progression",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("mark", "Mark used skills", "state", "", "roll"),
				WorkflowStep("roll", "Roll above current skill to improve", "roll", "1d100", ""),
			}, RelatedTools: []string{"improve_skill"}},
	}
	for _, wf := range workflows {
		AddWorkflow(p, wf)
	}

	mechanics := []rulesystem.MechanicDef{
		{ID: "levels_of_success", Label: "Levels of Success", Category: "core", Summary: "Critical (skill/20), Hard (half skill), Regular, Fail, Fumble (96+)."},
		{ID: "opposed_rolls", Label: "Opposed Rolls", Category: "core", Summary: "Both roll; better level wins.", WorkflowID: "opposed", RelatedTools: []string{"opposed_check"}},
		{ID: "major_wounds", Label: "Major Wounds", Category: "combat", Summary: "Single hit damage >= SIZ causes major wound.", WorkflowID: "major_wound"},
		{ID: "sanity_loss", Label: "Sanity Loss", Category: "horror", Summary: "Encounter mythos; lose SAN on failed roll.", WorkflowID: "sanity", RelatedTools: []string{"fear_sanity"}},
		{ID: "temp_insanity", Label: "Temporary Insanity", Category: "horror", Summary: "Lose 5+ SAN in one check triggers temp insanity."},
		{ID: "indef_insanity", Label: "Indefinite Insanity", Category: "horror", Summary: "SAN 0 triggers indefinite insanity."},
		{ID: "luck", Label: "Luck Points", Category: "meta", Summary: "Spend luck to adjust rolls."},
		{ID: "chase", Label: "Chase Rules", Category: "exploration", Summary: "Extended opposed rolls for pursuit."},
		{ID: "firearms", Label: "Firearms", Category: "combat", Summary: "Handgun/rifle skills; aim action improves chance."},
		{ID: "armor", Label: "Armor", Category: "combat", Summary: "Armor reduces damage by value."},
	}
	for _, m := range mechanics {
		AddMechanic(p, m)
	}

	AddTable(p, rulesystem.TableDef{
		ID: "sanity_loss_sample", Label: "Sample Sanity Loss", Roll: "1d100",
		Rows: []rulesystem.TableRow{
			{Key: "1-10", Result: "1d2 SAN"}, {Key: "11-50", Result: "1d6 SAN"}, {Key: "51-100", Result: "1d10 SAN"},
		},
	})
	AddTable(p, rulesystem.TableDef{
		ID: "major_wound_location", Label: "Major Wound Location", Roll: "1d10",
		Rows: []rulesystem.TableRow{
			{Key: "1", Result: "Leg"}, {Key: "2", Result: "Abdomen"}, {Key: "3", Result: "Chest"},
			{Key: "4", Result: "Arm"}, {Key: "5", Result: "Head"},
		},
	})
	AddTable(p, rulesystem.TableDef{
		ID: "insanity_episode", Label: "Insanity Episode", Roll: "1d10",
		Rows: []rulesystem.TableRow{
			{Key: "1-3", Result: "Flee"}, {Key: "4-6", Result: "Faint"}, {Key: "7-10", Result: "Phobia trigger"},
		},
	})

	AddChapter(p, "combat", "Combat", "d100 combat with major wounds.", []rulesystem.Section{
		Section("attacks", "Attacks", "Roll under fighting/firearms vs dodge or parry.", "Damage dice", "Major wound if damage >= SIZ"),
		Section("wounds", "Wounds & Healing", "First Aid stabilizes; Medicine for long-term.", "Bleeding", "Hospitalization"),
	}, "combat")
	AddChapter(p, "magic", "Magic", "MP-powered casting with mythos risks.", []rulesystem.Section{
		Section("casting", "Casting", "Pay MP, roll under casting skill.", "Mythos spells cost SAN"),
	})
	AddChapter(p, "skills", "Skills", "Percentile skills improve through use.", []rulesystem.Section{
		Section("checks", "Skill Checks", "Roll d100 under skill.", "Hard success at half", "Critical at 1/5 skill"),
	})
	AddChapter(p, "character", "Character", "Characteristics and occupation.", []rulesystem.Section{
		Section("chars", "Characteristics", "3d6*5 (or variant) for core stats.", "Derived HP and SAN"),
		Section("horror", "Horror", "Sanity tracks mental stability.", "Temp vs indefinite insanity"),
	})

	BindToolFromCanonical(p, "skill_check", "Percentile Skill Check", "Roll d100 under skill.", "resolution", "skill_check", nil, nil,
		[]rulesystem.ToolExample{{Title: "Spot Hidden", Input: map[string]any{"skill": "spot_hidden", "actor_id": "inv_1"}, Output: "42 vs 65 — success"}})
	BindToolFromCanonical(p, "opposed_check", "Opposed Roll", "Compare levels of success.", "resolution", "opposed", nil, nil, nil)
	BindToolFromCanonical(p, "attack", "Attack", "Fighting or firearms attack.", "combat", "attack", nil, nil, nil)
	BindToolFromCanonical(p, "damage_roll", "Damage", "Roll weapon damage.", "combat", "attack", nil, nil, nil)
	BindToolFromCanonical(p, "fear_sanity", "Sanity Test", "Resolve SAN loss.", "horror", "sanity", nil, nil, nil)
	BindToolFromCanonical(p, "apply_condition", "Apply Condition", "Major wound, insanity, etc.", "state", "", nil, nil, nil)
	BindToolFromCanonical(p, "update_health", "Adjust HP/SAN", "Modify pools.", "state", "", nil, nil, nil)
	BindToolFromCanonical(p, "improve_skill", "Improve Skill", "Post-scenario advancement.", "progression", "improve", nil, nil, nil)
	BindToolFromCanonical(p, "cast_spell", "Cast Spell", "Spend MP and roll casting.", "magic", "spell_cast", nil, nil, nil)
	BindToolFromCanonical(p, "social_conflict", "Social Conflict", "Persuade, fast talk, intimidate.", "social", "", nil, nil, nil)
	BindToolFromCanonical(p, "roll_on_table", "Roll on Table", "Sanity/insanity tables.", "reference", "", nil, nil, nil)
	BindToolFromCanonical(p, "lookup_creature", "Lookup Creature", "Fetch mythos entity.", "reference", "", nil, nil, nil)
	BindToolFromCanonical(p, "ability_check", "Characteristic Roll", "Roll under stat*5.", "resolution", "skill_check", nil, nil, nil)
	BindToolFromCanonical(p, "saving_throw", "Luck Save", "Use luck or POW for saves.", "resolution", "skill_check", nil, nil, nil)
	BindToolFromCanonical(p, "roll_dice", "Roll Dice", "Generic roll.", "core", "", nil, nil, nil)

	p.Character = rulesystem.CharacterSchema{
		Sections: []rulesystem.SchemaSection{{ID: "profile", Label: "Profile"}, {ID: "skills", Label: "Skills"}},
		Fields: []rulesystem.CharacterField{
			{ID: "name", Label: "Name", Kind: "string", Required: true},
			{ID: "occupation", Label: "Occupation", Kind: "string"},
			{ID: "hp", Label: "Hit Points", Kind: "resource", Formula: "(con + siz) / 10"},
			{ID: "sanity", Label: "Sanity", Kind: "resource", Formula: "pow"},
		},
	}
	p.Creature = rulesystem.CreatureSchema{
		Fields: []rulesystem.CharacterField{
			{ID: "name", Label: "Name", Kind: "string"},
			{ID: "hp", Label: "HP", Kind: "resource"},
			{ID: "san_loss", Label: "SAN Loss", Kind: "string"},
		},
		Templates: []rulesystem.CreatureTemplate{
			{ID: "deep_one", Label: "Deep One", Stats: map[string]string{"hp": "14", "san_loss": "0/1d6"}},
			{ID: "cultist", Label: "Cultist", Stats: map[string]string{"hp": "10", "fighting": "50"}},
		},
	}

	p.OracleGuide = rulesystem.OracleGuide{
		Principles: []string{
			"Use opposed_check for contests; skill_check for static DC.",
			"Always check major wound when damage >= SIZ.",
			"Sanity encounters use fear_sanity with table lookup.",
		},
		ToolPriority: []string{"skill_check", "opposed_check", "attack", "fear_sanity"},
		Scenarios: []rulesystem.GuideScenario{
			{Situation: "Investigator spots clue", UseTools: []string{"skill_check"}, Avoid: []string{"narrate_success_without_roll"}},
			{Situation: "Mythos horror revealed", UseTools: []string{"fear_sanity", "roll_on_table"}, Avoid: []string{"ignore_san_loss"}},
			{Situation: "Brawl in alley", UseTools: []string{"opposed_check", "attack", "damage_roll"}, Avoid: []string{"d20_mechanics"}},
		},
	}
	p.Compatibility = rulesystem.EngineCompat{
		CharacterType: "d100_character",
		StatBlockType: "d100_creature",
		ToolMap: map[string]string{"check": "skill_check", "oppose": "opposed_check", "sanity": "fear_sanity"},
	}
	p.RulesSummary = []string{
		"d100 roll under skill; levels of success for opposed rolls.",
		"Major wounds on big hits; SAN for horror.",
		"Skills improve through successful use over time.",
	}
	return p
}
