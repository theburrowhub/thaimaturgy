package profiles

import (
	"github.com/theburrowhub/thaimaturgy/internal/rulesystem"
)

// DnD5e returns a production-quality Dungeons & Dragons 5th Edition pack.
func DnD5e() *rulesystem.Pack {
	p := NewBasePack("dnd5e", "Dungeons & Dragons 5th Edition", "d20_fantasy")
	p.Version = "2024.1"
	p.Source.Templates = []string{"SRD", "PHB"}
	p.Source.Notes = "Built-in SRD-aligned profile for thAImaturgy oracle wiring."

	p.Dice = rulesystem.DiceConfig{
		Primary:  "d20",
		Common:   []string{"d4", "d6", "d8", "d10", "d12", "d20", "d100"},
		Notation: "NdS+M",
		Keep:     "advantage/disadvantage",
		Notes:    "d20 tests with advantage/disadvantage; damage uses variable dice pools.",
	}

	// --- Attributes (6) ---
	abilities := []struct{ id, label, abbrev string }{
		{"str", "Strength", "STR"}, {"dex", "Dexterity", "DEX"}, {"con", "Constitution", "CON"},
		{"int", "Intelligence", "INT"}, {"wis", "Wisdom", "WIS"}, {"cha", "Charisma", "CHA"},
	}
	for _, a := range abilities {
		AddAttribute(p, rulesystem.AttributeDef{
			ID: a.id, Label: a.label, Abbrev: a.abbrev, Scale: "score",
			Min: 1, Max: 30, DefaultRoll: "1d20+" + a.abbrev + "_mod",
			ModifierFormula: "floor((score-10)/2)",
		})
	}

	// --- Skills (18) ---
	skills := []struct{ id, label, attr, cat string }{
		{"athletics", "Athletics", "str", "physical"},
		{"acrobatics", "Acrobatics", "dex", "physical"}, {"sleight_of_hand", "Sleight of Hand", "dex", "physical"},
		{"stealth", "Stealth", "dex", "physical"},
		{"arcana", "Arcana", "int", "knowledge"}, {"history", "History", "int", "knowledge"},
		{"investigation", "Investigation", "int", "knowledge"}, {"nature", "Nature", "int", "knowledge"},
		{"religion", "Religion", "int", "knowledge"},
		{"animal_handling", "Animal Handling", "wis", "social"}, {"insight", "Insight", "wis", "social"},
		{"medicine", "Medicine", "wis", "social"}, {"perception", "Perception", "wis", "social"},
		{"survival", "Survival", "wis", "social"},
		{"deception", "Deception", "cha", "social"}, {"intimidation", "Intimidation", "cha", "social"},
		{"performance", "Performance", "cha", "social"}, {"persuasion", "Persuasion", "cha", "social"},
	}
	for _, s := range skills {
		AddSkill(p, rulesystem.SkillDef{
			ID: s.id, Label: s.label, Attribute: s.attr, Category: s.cat, Training: true,
		})
	}

	// --- Resources ---
	AddResource(p, rulesystem.ResourceDef{
		ID: "hp", Label: "Hit Points", Kind: "pool", Primary: true, Min: 0,
		MaxFormula: "hit_die_max + con_mod + (level-1)*(hit_die_avg + con_mod)",
		ResetOn: []string{"long_rest_partial"}, Overflow: "death_saves",
	})
	AddResource(p, rulesystem.ResourceDef{
		ID: "spell_slots", Label: "Spell Slots", Kind: "slots",
		MaxFormula: "slots_by_level", ResetOn: []string{"long_rest"},
	})
	AddResource(p, rulesystem.ResourceDef{
		ID: "hit_dice", Label: "Hit Dice", Kind: "pool",
		MaxFormula: "level", ResetOn: []string{"long_rest"},
	})

	// --- Conditions (15) ---
	conditions := []struct {
		id, label string
		sev       int
		effects   []string
		endsOn    []string
	}{
		{"blinded", "Blinded", 2, []string{"auto_fail_sight_checks", "attacks_against_have_advantage", "your_attacks_have_disadvantage"}, nil},
		{"charmed", "Charmed", 1, []string{"cannot_attack_charmer", "charmer_has_advantage_on_social_vs_you"}, nil},
		{"deafened", "Deafened", 1, []string{"auto_fail_hearing_checks"}, nil},
		{"frightened", "Frightened", 2, []string{"disadvantage_while_source_visible", "cannot_move_closer_to_source"}, nil},
		{"grappled", "Grappled", 2, []string{"speed_zero", "escape_dc=strength_athletics_or_dex_acrobatics"}, []string{"escape", "grappler_incapacitated"}},
		{"incapacitated", "Incapacitated", 3, []string{"no_actions_or_reactions"}, nil},
		{"invisible", "Invisible", 2, []string{"heavily_obscured_for_hiding", "attacks_against_have_disadvantage", "your_attacks_have_advantage"}, nil},
		{"paralyzed", "Paralyzed", 4, []string{"incapacitated", "auto_fail_str_dex_saves", "attacks_within_5ft_are_crits"}, nil},
		{"petrified", "Petrified", 5, []string{"incapacitated", "resist_all_damage", "immune_poison_disease"}, nil},
		{"poisoned", "Poisoned", 2, []string{"disadvantage_on_attacks_and_ability_checks"}, nil},
		{"prone", "Prone", 1, []string{"melee_attacks_against_advantage", "ranged_attacks_against_disadvantage", "your_attacks_disadvantage"}, []string{"stand_up_costs_half_movement"}},
		{"restrained", "Restrained", 2, []string{"speed_zero", "disadvantage_on_attacks_and_dex_saves", "attacks_against_have_advantage"}, nil},
		{"stunned", "Stunned", 3, []string{"incapacitated", "auto_fail_str_dex_saves", "attacks_against_have_advantage"}, nil},
		{"unconscious", "Unconscious", 4, []string{"incapacitated", "prone", "auto_fail_str_dex_saves", "attacks_within_5ft_are_crits"}, nil},
		{"exhaustion", "Exhaustion", 3, []string{"level_1_disadvantage_ability_checks", "level_3_disadvantage_attacks_saves", "level_6_death"}, nil},
	}
	for _, c := range conditions {
		AddCondition(p, rulesystem.ConditionDef{
			ID: c.id, Label: c.label, Severity: c.sev, Effects: c.effects, EndsOn: c.endsOn,
		})
	}

	// --- Damage types ---
	for _, dt := range []rulesystem.NamedDesc{
		{ID: "bludgeoning", Label: "Bludgeoning"}, {ID: "piercing", Label: "Piercing"},
		{ID: "slashing", Label: "Slashing"}, {ID: "fire", Label: "Fire"},
		{ID: "cold", Label: "Cold"}, {ID: "lightning", Label: "Lightning"},
		{ID: "acid", Label: "Acid"}, {ID: "poison", Label: "Poison"},
		{ID: "necrotic", Label: "Necrotic"}, {ID: "radiant", Label: "Radiant"},
		{ID: "psychic", Label: "Psychic"}, {ID: "force", Label: "Force"},
		{ID: "thunder", Label: "Thunder"},
	} {
		AddDamageType(p, dt.ID, dt.Label, "")
	}

	// --- Equipment ---
	p.Equipment.WeaponCategories = []rulesystem.NamedDesc{
		{ID: "simple_melee", Label: "Simple Melee"}, {ID: "simple_ranged", Label: "Simple Ranged"},
		{ID: "martial_melee", Label: "Martial Melee"}, {ID: "martial_ranged", Label: "Martial Ranged"},
	}
	p.Equipment.ArmorCategories = []rulesystem.NamedDesc{
		{ID: "light", Label: "Light Armor"}, {ID: "medium", Label: "Medium Armor"}, {ID: "heavy", Label: "Heavy Armor"}, {ID: "shield", Label: "Shield"},
	}
	p.Equipment.Properties = []rulesystem.NamedDesc{
		{ID: "finesse", Label: "Finesse"}, {ID: "heavy", Label: "Heavy"}, {ID: "light", Label: "Light"},
		{ID: "loading", Label: "Loading"}, {ID: "reach", Label: "Reach"}, {ID: "two_handed", Label: "Two-Handed"},
		{ID: "versatile", Label: "Versatile"}, {ID: "thrown", Label: "Thrown"},
	}
	AddEquipmentTemplate(p, rulesystem.ItemTemplate{
		ID: "longsword", Label: "Longsword", Kind: "weapon",
		Stats: map[string]string{"damage": "1d8 slashing", "properties": "versatile"},
	})
	AddEquipmentTemplate(p, rulesystem.ItemTemplate{
		ID: "chain_mail", Label: "Chain Mail", Kind: "armor",
		Stats: map[string]string{"ac": "16", "category": "heavy", "stealth_disadvantage": "true"},
	})
	AddEquipmentTemplate(p, rulesystem.ItemTemplate{
		ID: "shortbow", Label: "Shortbow", Kind: "weapon",
		Stats: map[string]string{"damage": "1d6 piercing", "range": "80/320", "properties": "ammunition,two_handed"},
	})

	// --- Resolution ---
	p.Resolution = rulesystem.ResolutionConfig{
		SkillCheck: rulesystem.CheckRule{
			Roll: "1d20+mod", Compare: ">=", Target: "dc",
			Difficulty: []rulesystem.DifficultyTier{
				{ID: "very_easy", Label: "Very Easy", Target: 5},
				{ID: "easy", Label: "Easy", Target: 10},
				{ID: "medium", Label: "Medium", Target: 15},
				{ID: "hard", Label: "Hard", Target: 20},
				{ID: "very_hard", Label: "Very Hard", Target: 25},
				{ID: "nearly_impossible", Label: "Nearly Impossible", Target: 30},
			},
			Success: "meet_or_exceed_dc", Failure: "below_dc",
			Critical: "natural_20", Fumble: "natural_1", WorkflowID: "skill_check",
		},
		AbilityCheck: rulesystem.CheckRule{
			Roll: "1d20+ability_mod", Compare: ">=", Target: "dc", WorkflowID: "skill_check",
		},
		SavingThrow: rulesystem.CheckRule{
			Roll: "1d20+save_mod", Compare: ">=", Target: "effect_dc",
			Success: "half_or_none", Failure: "full_effect", WorkflowID: "skill_check",
		},
		OpposedCheck: rulesystem.OpposedRule{
			AttackerRoll: "1d20+mod", DefenderRoll: "1d20+mod", Win: "higher", Tie: "reroll_or_dm",
		},
		Attack: rulesystem.AttackRule{
			Roll: "1d20+attack_bonus", Target: "target_ac", Compare: ">=",
			OnHit: []string{"damage_roll"}, OnMiss: []string{"no_damage"},
			OnCrit: []string{"double_damage_dice"}, WorkflowID: "attack",
		},
		Defense: rulesystem.DefenseRule{Stat: "armor_class", Formula: "10+dex_mod+armor+shield"},
		Spell: rulesystem.PowerRule{
			Cost: "spell_slot", Roll: "spell_attack_or_save_dc",
			Components: []string{"verbal", "somatic", "material"}, WorkflowID: "spell_cast",
		},
		Power: rulesystem.PowerRule{Cost: "none", Roll: "varies"},
		Damage: rulesystem.DamageRule{
			Roll: "weapon_or_spell_dice", Resistance: "halve", OnApply: []string{"update_health", "concentration_check"},
		},
		Initiative: rulesystem.CheckRule{Roll: "1d20+dex_mod", Compare: "order_desc"},
		Death: rulesystem.DeathRule{
			AtZero: []string{"death_save", "stable_at_3_successes", "dead_at_3_failures"},
			Recovery: []string{"healing_above_zero", "stabilize_medicine_dc10"},
			WorkflowID: "death_save",
		},
	}

	// --- Combat ---
	p.Combat = rulesystem.CombatModel{
		Mode: "turn_based",
		RoundSteps: []string{
			"1. Determine surprise (if any)",
			"2. Roll initiative and establish order",
			"3. Take turns: move, action, bonus action, reaction, free object interaction",
			"4. End of round: duration ticks, concentration checks",
		},
		ActionEconomy: rulesystem.ActionEconomy{
			Actions: []rulesystem.ActionType{
				{ID: "action", Label: "Action", PerTurn: 1, Examples: []string{"Attack", "Cast a Spell", "Dash", "Dodge", "Help", "Hide", "Ready"}},
				{ID: "bonus", Label: "Bonus Action", PerTurn: 1, Examples: []string{"Off-hand attack", "Class feature", "Some spells"}},
				{ID: "reaction", Label: "Reaction", PerTurn: 1, Examples: []string{"Opportunity attack", "Shield", "Counterspell"}},
				{ID: "movement", Label: "Movement", Examples: []string{"Up to speed", "Split across action"}},
				{ID: "free", Label: "Free", Examples: []string{"Draw/stow one object", "Speak briefly"}},
			},
			Notes: "On your turn you get one action, one bonus action, movement up to speed, and one reaction per round.",
		},
		Positioning: "grid_or_theater_of_mind",
	}

	// --- Magic ---
	p.Magic = rulesystem.MagicModel{
		Traditions: []rulesystem.NamedDesc{
			{ID: "arcane", Label: "Arcane"}, {ID: "divine", Label: "Divine"},
			{ID: "primal", Label: "Primal"}, {ID: "psionic", Label: "Psionic"},
		},
		Casting: []string{
			"Expend spell slot of spell level or higher",
			"Verbal, somatic, material components unless replaced",
			"Concentration limits one concentration spell",
		},
		Recovery: []string{"Long rest restores spell slots per class table", "Some features restore on short rest"},
	}

	// --- Progression levels 1-5 ---
	p.Progression = rulesystem.ProgressionModel{
		Kind: "xp_level",
		Levels: []rulesystem.LevelRow{
			{Level: 1, XP: 0, Advance: "class_features_level_1", Notes: "Proficiency +2, starting equipment"},
			{Level: 2, XP: 300, Advance: "class_features_level_2"},
			{Level: 3, XP: 900, Advance: "subclass_choice"},
			{Level: 4, XP: 2700, Advance: "ability_score_improvement_or_feat"},
			{Level: 5, XP: 6500, Advance: "extra_attack_or_3rd_level_spells"},
		},
		Milestones: []string{"Session award 50-200 XP", "Quest completion lump sum"},
	}

	// --- Formulas ---
	AddFormula(p, rulesystem.FormulaDef{
		ID: "max_hp", Label: "Maximum Hit Points", Expression: "hit_die_max + con_mod + (level-1)*(hit_die_avg + con_mod)",
		Variables: map[string]string{"level": "character level", "con_mod": "CON modifier"},
	})
	AddFormula(p, rulesystem.FormulaDef{
		ID: "proficiency", Label: "Proficiency Bonus", Expression: "2 + floor((level-1)/4)",
		Variables: map[string]string{"level": "character level"},
	})
	AddFormula(p, rulesystem.FormulaDef{
		ID: "spell_save_dc", Label: "Spell Save DC", Expression: "8 + proficiency + spellcasting_mod",
	})
	AddFormula(p, rulesystem.FormulaDef{
		ID: "spell_attack", Label: "Spell Attack Bonus", Expression: "proficiency + spellcasting_mod",
	})

	// --- Workflows (8+) ---
	workflows := []rulesystem.WorkflowDef{
		{
			ID: "attack", Label: "Attack", Category: "combat", Trigger: "actor_declares_attack",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("declare", "Declare target and weapon", "input", "", "roll"),
				WorkflowStep("roll", "Roll attack", "roll", "1d20+attack_bonus", "compare"),
				WorkflowStep("compare", "Compare to AC", "branch", "", "damage"),
				WorkflowStep("damage", "Roll damage on hit", "roll", "damage_dice", "apply"),
				WorkflowStep("apply", "Apply damage and conditions", "effect", "", ""),
			},
			Outputs: []string{"hit", "miss", "critical", "damage_total"}, RelatedTools: []string{"attack", "damage_roll", "update_health"},
		},
		{
			ID: "spell_cast", Label: "Cast Spell", Category: "magic", Trigger: "caster_declares_spell",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("validate", "Validate slot and components", "check", "", "resolve"),
				WorkflowStep("resolve", "Resolve spell attack or save", "roll", "spell_resolution", "effect"),
				WorkflowStep("effect", "Apply spell effects", "effect", "", "concentration"),
				WorkflowStep("concentration", "Begin or maintain concentration", "state", "", ""),
			},
			RelatedTools: []string{"cast_spell", "saving_throw", "damage_roll", "apply_condition"},
		},
		{
			ID: "death_save", Label: "Death Saving Throw", Category: "combat", Trigger: "hp_at_zero",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("roll", "Roll d20 death save", "roll", "1d20", "track"),
				WorkflowStep("track", "Track successes/failures", "state", "", ""),
			},
			RelatedTools: []string{"death_save", "update_health"},
		},
		{
			ID: "concentration", Label: "Concentration Check", Category: "magic", Trigger: "damage_while_concentrating",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("compute_dc", "DC = max(10, damage/2)", "expression", "max(10,damage/2)", "roll"),
				WorkflowStep("roll", "CON save vs DC", "roll", "1d20+con_mod", "result"),
				WorkflowStep("result", "Maintain or drop concentration", "branch", "", ""),
			},
			RelatedTools: []string{"concentration_check", "remove_condition"},
		},
		{
			ID: "short_rest", Label: "Short Rest", Category: "exploration", Trigger: "party_short_rest",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("duration", "Rest 1 hour", "narrative", "", "spend_hd"),
				WorkflowStep("spend_hd", "Spend hit dice for healing", "roll", "hit_die+con_mod", "features"),
				WorkflowStep("features", "Recover short-rest features", "state", "", ""),
			},
			RelatedTools: []string{"rest", "update_health"},
		},
		{
			ID: "long_rest", Label: "Long Rest", Category: "exploration", Trigger: "party_long_rest",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("duration", "Rest 8 hours", "narrative", "", "recover"),
				WorkflowStep("recover", "Restore HP, slots, hit dice (half level)", "state", "", ""),
			},
			RelatedTools: []string{"rest", "update_health", "update_character"},
		},
		{
			ID: "skill_check", Label: "Skill / Ability Check", Category: "resolution", Trigger: "ability_or_skill_test",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("declare", "Choose skill and DC", "input", "", "roll"),
				WorkflowStep("roll", "Roll d20 + modifiers", "roll", "1d20+mod", "compare"),
				WorkflowStep("compare", "Success or failure", "branch", "", ""),
			},
			RelatedTools: []string{"skill_check", "ability_check", "roll_dice"},
		},
		{
			ID: "initiative", Label: "Initiative", Category: "combat", Trigger: "combat_start",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("roll_all", "Each combatant rolls initiative", "roll", "1d20+dex_mod", "order"),
				WorkflowStep("order", "Sort descending", "state", "", ""),
			},
			RelatedTools: []string{"initiative", "roll_dice"},
		},
		{
			ID: "social_persuade", Label: "Social Interaction", Category: "social", Trigger: "social_encounter",
			Steps: []rulesystem.WorkflowStep{
				WorkflowStep("approach", "Choose skill approach", "input", "", "roll"),
				WorkflowStep("roll", "Roll opposed or DC check", "roll", "1d20+cha_mod", "outcome"),
				WorkflowStep("outcome", "Apply attitude shift", "narrative", "", ""),
			},
			RelatedTools: []string{"social_conflict", "skill_check"},
		},
	}
	for _, wf := range workflows {
		AddWorkflow(p, wf)
	}

	// --- Mechanics (10+) ---
	mechanics := []rulesystem.MechanicDef{
		{ID: "advantage", Label: "Advantage / Disadvantage", Category: "core", Summary: "Roll 2d20, take higher (advantage) or lower (disadvantage).", Tags: []string{"d20"}},
		{ID: "proficiency", Label: "Proficiency Bonus", Category: "core", Summary: "Add proficiency bonus when proficient with skill, save, or attack.", WorkflowID: "skill_check"},
		{ID: "critical_hit", Label: "Critical Hit", Category: "combat", Summary: "Natural 20 on attack roll hits and doubles damage dice.", WorkflowID: "attack", RelatedTools: []string{"attack", "damage_roll"}},
		{ID: "cover", Label: "Cover", Category: "combat", Summary: "Half +2 AC, Three-quarters +5 AC, Total blocks direct attacks."},
		{ID: "opportunity_attack", Label: "Opportunity Attack", Category: "combat", Summary: "Reaction melee attack when hostile leaves reach.", WorkflowID: "attack"},
		{ID: "concentration_rules", Label: "Concentration", Category: "magic", Summary: "One concentration spell; CON save when damaged.", WorkflowID: "concentration", RelatedTools: []string{"concentration_check"}},
		{ID: "death_saves", Label: "Death Saves", Category: "combat", Summary: "At 0 HP roll d20: 10+ success, <10 failure, nat1 two failures, nat20 regain 1 HP.", WorkflowID: "death_save"},
		{ID: "exhaustion_track", Label: "Exhaustion", Category: "exploration", Summary: "Six levels of escalating penalties; long rest removes one level in some campaigns."},
		{ID: "ready_action", Label: "Ready Action", Category: "combat", Summary: "Use action to trigger later with reaction."},
		{ID: "help_action", Label: "Help Action", Category: "combat", Summary: "Grant ally advantage on next ability check or attack against adjacent foe."},
		{ID: "spell_components", Label: "Spell Components", Category: "magic", Summary: "V, S, M required unless feature replaces; somatic needs free hand."},
		{ID: "inspiration", Label: "Inspiration", Category: "social", Summary: "DM-granted reroll or advantage on one roll."},
	}
	for _, m := range mechanics {
		AddMechanic(p, m)
	}

	// --- Tables (3+) ---
	AddTable(p, rulesystem.TableDef{
		ID: "dc_guidance", Label: "Typical DCs", Roll: "n/a",
		Columns: []string{"Difficulty", "DC"},
		Rows: []rulesystem.TableRow{
			{Key: "very_easy", Result: "5"}, {Key: "easy", Result: "10"}, {Key: "medium", Result: "15"},
			{Key: "hard", Result: "20"}, {Key: "very_hard", Result: "25"}, {Key: "nearly_impossible", Result: "30"},
		},
	})
	AddTable(p, rulesystem.TableDef{
		ID: "death_save_outcomes", Label: "Death Save Outcomes", Roll: "1d20",
		Rows: []rulesystem.TableRow{
			{Key: "1", Result: "Two failures"}, {Key: "2-9", Result: "Failure"},
			{Key: "10-19", Result: "Success"}, {Key: "20", Result: "Regain 1 HP"},
		},
	})
	AddTable(p, rulesystem.TableDef{
		ID: "proficiency_by_level", Label: "Proficiency by Level", Roll: "n/a",
		Columns: []string{"Level", "Bonus"},
		Rows: []rulesystem.TableRow{
			{Key: "1-4", Result: "+2"}, {Key: "5-8", Result: "+3"}, {Key: "9-12", Result: "+4"},
			{Key: "13-16", Result: "+5"}, {Key: "17-20", Result: "+6"},
		},
	})
	AddTable(p, rulesystem.TableDef{
		ID: "wild_magic_surge", Label: "Wild Magic Surge (sample)", Roll: "1d20",
		Rows: []rulesystem.TableRow{
			{Key: "1", Result: "Roll on Wild Magic table"}, {Key: "2-19", Result: "No surge"}, {Key: "20", Result: "Regain 1 sorcery point"},
		},
		Notes: "Optional; for sorcerer tables.",
	})

	// --- Chapters (4+) ---
	AddChapter(p, "combat", "Combat", "Turn-based combat, attacks, damage, and death.", []rulesystem.Section{
		Section("overview", "Combat Overview", "Combat proceeds in rounds. Each participant acts on their initiative count.", "Surprise", "Initiative", "Actions in combat"),
		Section("attacks", "Making an Attack", "Roll d20 + modifiers vs target AC. On hit, roll damage.", "Critical hits on natural 20", "Unseen attackers have advantage"),
		Section("damage", "Damage and Healing", "Subtract damage from HP. At 0 HP, death saves begin.", "Damage types and resistance", "Healing restores HP up to maximum"),
	}, "combat")
	AddChapter(p, "magic", "Magic", "Spellcasting, slots, components, and concentration.", []rulesystem.Section{
		Section("casting", "Casting a Spell", "Choose spell, expend slot, resolve effect.", "Spell attack vs save", "Upcasting"),
		Section("concentration", "Concentration", "Maintain one spell; CON save when damaged.", "Breaking concentration ends spell effects"),
	})
	AddChapter(p, "exploration", "Exploration & Skills", "Ability checks, skills, rests, and travel.", []rulesystem.Section{
		Section("checks", "Ability Checks", "d20 + relevant modifier vs DC.", "Group checks", "Passive scores"),
		Section("rests", "Rests", "Short rest: 1 hour, spend hit dice. Long rest: 8 hours, restore most resources."),
	})
	AddChapter(p, "character", "Character Creation", "Abilities, skills, classes, and advancement.", []rulesystem.Section{
		Section("abilities", "Ability Scores", "Six abilities define modifiers used across the game.", "Standard array or point buy"),
		Section("advancement", "Leveling", "Gain XP, increase level, unlock class features.", "ASI at levels 4, 8, 12, 16, 19"),
	})

	// --- Tool bindings (15+) ---
	BindToolFromCanonical(p, "attack", "Melee/Ranged Attack", "Resolve an attack roll and damage against AC.", "combat", "attack",
		[]string{"attacker_has_action", "target_in_range"}, []string{"may_consume_action", "may_apply_damage"},
		[]rulesystem.ToolExample{{Title: "Fighter attacks goblin", Input: map[string]any{"attacker_id": "pc_1", "target_id": "goblin_1", "weapon_id": "longsword"}, Output: "Hit for 9 slashing"}})
	BindToolFromCanonical(p, "cast_spell", "Cast Spell", "Expend slot and resolve spell.", "magic", "spell_cast",
		[]string{"caster_has_slot", "components_available"}, []string{"consumes_slot", "may_require_concentration"},
		[]rulesystem.ToolExample{{Title: "Wizard casts Magic Missile", Input: map[string]any{"caster_id": "pc_wiz", "spell_id": "magic_missile", "slot_level": 1}, Output: "3 darts auto-hit for 3d4+3 force"}})
	BindToolFromCanonical(p, "skill_check", "Skill Check", "Roll d20 + skill modifier vs DC.", "resolution", "skill_check",
		[]string{"skill_declared"}, nil,
		[]rulesystem.ToolExample{{Title: "Rogue hides", Input: map[string]any{"skill": "stealth", "dc": 15, "actor_id": "pc_rogue"}, Output: "Stealth 18 vs DC 15 — success"}})
	BindToolFromCanonical(p, "saving_throw", "Saving Throw", "Roll save vs effect DC.", "resolution", "skill_check",
		nil, []string{"apply_effect_on_failure"}, nil)
	BindToolFromCanonical(p, "ability_check", "Ability Check", "Raw ability check without skill proficiency.", "resolution", "skill_check", nil, nil, nil)
	BindToolFromCanonical(p, "damage_roll", "Damage Roll", "Roll weapon or spell damage.", "combat", "attack", nil, []string{"feeds_update_health"}, nil)
	BindToolFromCanonical(p, "update_health", "Apply Damage/Healing", "Modify HP track.", "state", "",
		nil, []string{"updates_hp"}, nil)
	BindToolFromCanonical(p, "apply_condition", "Apply Condition", "Add blinded, prone, etc.", "state", "",
		nil, []string{"adds_condition"}, nil)
	BindToolFromCanonical(p, "remove_condition", "Remove Condition", "End a condition.", "state", "", nil, nil, nil)
	BindToolFromCanonical(p, "death_save", "Death Saving Throw", "Roll death save at 0 HP.", "combat", "death_save",
		[]string{"hp_is_zero"}, []string{"tracks_success_failure"}, nil)
	BindToolFromCanonical(p, "concentration_check", "Concentration Save", "CON save to maintain spell.", "magic", "concentration",
		[]string{"caster_is_concentrating", "damage_taken"}, nil, nil)
	BindToolFromCanonical(p, "initiative", "Roll Initiative", "Establish combat order.", "combat", "initiative", nil, nil, nil)
	BindToolFromCanonical(p, "rest", "Short/Long Rest", "Apply rest recovery.", "exploration", "long_rest", nil, nil, nil)
	BindToolFromCanonical(p, "award_experience", "Award XP", "Grant experience points.", "progression", "",
		nil, []string{"increments_xp"}, nil)
	BindToolFromCanonical(p, "inventory_add", "Add Inventory", "Add items to character.", "inventory", "", nil, nil, nil)
	BindToolFromCanonical(p, "inventory_remove", "Remove Inventory", "Remove items from character.", "inventory", "", nil, nil, nil)
	BindToolFromCanonical(p, "lookup_creature", "Lookup Monster", "Fetch SRD creature stat block.", "reference", "", nil, nil, nil)
	BindToolFromCanonical(p, "social_conflict", "Social Interaction", "Resolve persuasion/intimidation.", "social", "social_persuade", nil, nil, nil)
	BindToolFromCanonical(p, "roll_dice", "Roll Dice", "Generic dice roll.", "core", "", nil, nil, nil)
	BindToolFromCanonical(p, "update_character", "Update Character", "Patch character sheet fields.", "state", "", nil, nil, nil)
	BindToolFromCanonical(p, "roll_on_table", "Roll on Table", "Use pack random tables.", "reference", "", nil, nil, nil)

	// --- Character & creature schemas ---
	p.Character = rulesystem.CharacterSchema{
		Sections: []rulesystem.SchemaSection{
			{ID: "identity", Label: "Identity"}, {ID: "abilities", Label: "Abilities"},
			{ID: "combat", Label: "Combat"}, {ID: "spells", Label: "Spells"},
		},
		Fields: []rulesystem.CharacterField{
			{ID: "name", Label: "Name", Section: "identity", Kind: "string", Required: true},
			{ID: "level", Label: "Level", Section: "identity", Kind: "number", Required: true},
			{ID: "class", Label: "Class", Section: "identity", Kind: "string", Required: true},
			{ID: "race", Label: "Species/Race", Section: "identity", Kind: "string"},
			{ID: "hp", Label: "Hit Points", Section: "combat", Kind: "resource", Formula: "max_hp"},
			{ID: "ac", Label: "Armor Class", Section: "combat", Kind: "number", Formula: "10+dex_mod+armor"},
			{ID: "proficiency", Label: "Proficiency Bonus", Section: "combat", Kind: "number", Formula: "2 + floor((level-1)/4)"},
		},
	}
	p.Creature = rulesystem.CreatureSchema{
		Sections: []rulesystem.SchemaSection{{ID: "stat_block", Label: "Stat Block"}},
		Fields: []rulesystem.CharacterField{
			{ID: "name", Label: "Name", Kind: "string", Required: true},
			{ID: "ac", Label: "Armor Class", Kind: "number"},
			{ID: "hp", Label: "Hit Points", Kind: "resource"},
			{ID: "cr", Label: "Challenge Rating", Kind: "string"},
		},
		Templates: []rulesystem.CreatureTemplate{
			{ID: "goblin", Label: "Goblin", Stats: map[string]string{"ac": "15", "hp": "7", "cr": "1/4"}},
			{ID: "ogre", Label: "Ogre", Stats: map[string]string{"ac": "11", "hp": "59", "cr": "2"}},
		},
	}

	// --- Prompts & oracle ---
	p.Prompts = rulesystem.PromptBundle{
		OracleContext: "D&D 5e SRD: d20 + modifier vs DC/AC. Use attack, skill_check, cast_spell, and death_save tools for mechanics.",
		DMNotes:       "Apply advantage/disadvantage when fiction warrants. Track concentration and conditions explicitly.",
		Glossary: []rulesystem.NamedDesc{
			{ID: "ac", Label: "Armor Class"}, {ID: "dc", Label: "Difficulty Class"},
		},
	}
	p.OracleGuide = rulesystem.OracleGuide{
		Principles: []string{
			"Never narrate HP changes without update_health or damage_roll.",
			"Spell effects that last require apply_condition or concentration tracking.",
			"At 0 HP, switch to death_save workflow.",
		},
		ToolPriority: []string{"attack", "skill_check", "cast_spell", "death_save", "initiative"},
		AntiPatterns: []string{"Guessing DC without table reference", "Applying damage without type"},
		Scenarios: []rulesystem.GuideScenario{
			{Situation: "Player attacks goblin with sword", UseTools: []string{"attack", "damage_roll", "update_health"}, Avoid: []string{"freeform_damage"}},
			{Situation: "Wizard damaged while concentrating", UseTools: []string{"concentration_check"}, Avoid: []string{"auto_drop_concentration"}},
			{Situation: "Party completes quest", UseTools: []string{"award_experience", "advance_quest"}, Avoid: []string{"level_up_without_xp"}},
			{Situation: "Rogue attempts stealth", UseTools: []string{"skill_check"}, Avoid: []string{"ability_check_when_skill_applies"}},
		},
	}

	p.Compatibility = rulesystem.EngineCompat{
		CharacterType: "dnd5e_character",
		StatBlockType: "dnd5e_creature",
		ToolMap: map[string]string{
			"roll": "roll_dice", "check": "skill_check", "save": "saving_throw",
			"attack": "attack", "spell": "cast_spell", "heal": "update_health",
			"damage": "damage_roll", "condition": "apply_condition",
		},
		Notes: "Maps legacy engine hooks to canonical v2 tools.",
	}

	p.RulesSummary = []string{
		"d20 + modifier vs DC or AC; natural 20/1 matter on attacks.",
		"Six abilities, eighteen skills, fifteen standard conditions.",
		"Action + bonus + movement + one reaction per round.",
		"Spell slots, components, concentration; death saves at 0 HP.",
	}
	p.Enrichment = rulesystem.EnrichmentSpec{Enabled: false}
	return p
}
