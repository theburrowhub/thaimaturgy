package domain

import "strings"

// This file generates D&D-style player characters from a race, class and level,
// assigning ability scores, HP, AC, proficiency bonus, speed, skills and starting
// gold by simplified 5e rules. It is deterministic (no randomness) so a given
// (race, class, level, name) always yields the same sheet — handy for tests and
// for a reproducible default party.

// standardArray is the classic 5e ability spread, assigned to a class's ability
// priority order (highest score to the most important ability).
var standardArray = [6]int{15, 14, 13, 12, 10, 8}

type raceInfo struct {
	speed int
	bonus map[Ability]int // racial ability score increases
	names []string        // sample names used when none is supplied
}

// Races is the list of supported races (also used to populate UI choosers).
var Races = []string{"Human", "Elf", "Dwarf", "Halfling", "Half-Orc", "Half-Elf", "Tiefling", "Dragonborn", "Gnome"}

var raceTable = map[string]raceInfo{
	"human":      {speed: 30, bonus: map[Ability]int{STR: 1, DEX: 1, CON: 1, INT: 1, WIS: 1, CHA: 1}, names: []string{"Alden", "Mira", "Rowan", "Tessa"}},
	"elf":        {speed: 30, bonus: map[Ability]int{DEX: 2}, names: []string{"Aelar", "Naivara", "Thalion", "Sariel"}},
	"dwarf":      {speed: 25, bonus: map[Ability]int{CON: 2}, names: []string{"Thorin", "Bruenna", "Dain", "Helga"}},
	"halfling":   {speed: 25, bonus: map[Ability]int{DEX: 2}, names: []string{"Pip", "Merla", "Lyle", "Rosie"}},
	"half-orc":   {speed: 30, bonus: map[Ability]int{STR: 2, CON: 1}, names: []string{"Grosh", "Mazka", "Krull", "Emen"}},
	"half-elf":   {speed: 30, bonus: map[Ability]int{CHA: 2, DEX: 1}, names: []string{"Elyan", "Sella", "Caden", "Wrenna"}},
	"tiefling":   {speed: 30, bonus: map[Ability]int{CHA: 2, INT: 1}, names: []string{"Damaia", "Kairon", "Nisha", "Zerael"}},
	"dragonborn": {speed: 30, bonus: map[Ability]int{STR: 2, CHA: 1}, names: []string{"Rhogar", "Sora", "Balasar", "Kava"}},
	"gnome":      {speed: 25, bonus: map[Ability]int{INT: 2}, names: []string{"Fizwick", "Nissa", "Boddy", "Ella"}},
}

type classInfo struct {
	hitDie   int
	priority [6]Ability // ability assignment order, most important first
	acBase   int        // starting AC base (typical armor for the class)
	acDexCap int        // max DEX mod added to AC; -1 = no cap (light/no armor)
	skills   []string   // default proficient skills
	gold     int        // starting gold pieces
}

// Classes is the list of supported classes (also used to populate UI choosers).
var Classes = []string{"Fighter", "Wizard", "Rogue", "Cleric", "Ranger", "Barbarian", "Bard", "Druid", "Paladin", "Sorcerer", "Warlock", "Monk"}

var classTable = map[string]classInfo{
	"fighter":   {hitDie: 10, priority: [6]Ability{STR, CON, DEX, WIS, CHA, INT}, acBase: 16, acDexCap: 0, skills: []string{"Athletics", "Perception"}, gold: 75},
	"barbarian": {hitDie: 12, priority: [6]Ability{STR, CON, DEX, WIS, CHA, INT}, acBase: 14, acDexCap: 2, skills: []string{"Athletics", "Survival"}, gold: 50},
	"paladin":   {hitDie: 10, priority: [6]Ability{STR, CHA, CON, WIS, DEX, INT}, acBase: 16, acDexCap: 0, skills: []string{"Athletics", "Religion"}, gold: 75},
	"ranger":    {hitDie: 10, priority: [6]Ability{DEX, WIS, CON, STR, INT, CHA}, acBase: 14, acDexCap: 2, skills: []string{"Survival", "Stealth"}, gold: 50},
	"cleric":    {hitDie: 8, priority: [6]Ability{WIS, CON, STR, CHA, DEX, INT}, acBase: 14, acDexCap: 2, skills: []string{"Medicine", "Insight"}, gold: 60},
	"druid":     {hitDie: 8, priority: [6]Ability{WIS, CON, DEX, INT, CHA, STR}, acBase: 13, acDexCap: 2, skills: []string{"Nature", "Perception"}, gold: 40},
	"monk":      {hitDie: 8, priority: [6]Ability{DEX, WIS, CON, STR, INT, CHA}, acBase: 11, acDexCap: -1, skills: []string{"Acrobatics", "Stealth"}, gold: 20},
	"rogue":     {hitDie: 8, priority: [6]Ability{DEX, CHA, CON, INT, WIS, STR}, acBase: 11, acDexCap: -1, skills: []string{"Stealth", "Sleight of Hand"}, gold: 40},
	"bard":      {hitDie: 8, priority: [6]Ability{CHA, DEX, CON, WIS, INT, STR}, acBase: 11, acDexCap: -1, skills: []string{"Persuasion", "Performance"}, gold: 50},
	"warlock":   {hitDie: 8, priority: [6]Ability{CHA, CON, DEX, WIS, INT, STR}, acBase: 11, acDexCap: -1, skills: []string{"Arcana", "Deception"}, gold: 40},
	"sorcerer":  {hitDie: 6, priority: [6]Ability{CHA, CON, DEX, WIS, INT, STR}, acBase: 10, acDexCap: -1, skills: []string{"Arcana", "Persuasion"}, gold: 30},
	"wizard":    {hitDie: 6, priority: [6]Ability{INT, CON, DEX, WIS, CHA, STR}, acBase: 10, acDexCap: -1, skills: []string{"Arcana", "Investigation"}, gold: 30},
}

// NormalizeRace maps free-form input to a supported race name (default Human).
func NormalizeRace(s string) string {
	key := strings.ToLower(strings.TrimSpace(s))
	if _, ok := raceTable[key]; ok {
		return titleRace(key)
	}
	return "Human"
}

// NormalizeClass maps free-form input to a supported class name (default Fighter).
func NormalizeClass(s string) string {
	key := strings.ToLower(strings.TrimSpace(s))
	if _, ok := classTable[key]; ok {
		return capitalize(key)
	}
	return "Fighter"
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func titleRace(key string) string {
	// Preserve the hyphenated capitalisation ("Half-Orc").
	parts := strings.Split(key, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "-")
}

// ProficiencyBonusForLevel returns the 5e proficiency bonus for a level.
func ProficiencyBonusForLevel(level int) int {
	if level < 1 {
		level = 1
	}
	return 2 + (level-1)/4
}

// GenerateCharacter builds a full character sheet from a race, class and level by
// simplified D&D 5e rules. Unknown race/class fall back to Human/Fighter; an empty
// name draws a sample name for the race. Level defaults to 1 when < 1.
func GenerateCharacter(name, race, class string, level int) *Character {
	if level < 1 {
		level = 1
	}
	raceName := NormalizeRace(race)
	className := NormalizeClass(class)
	ri := raceTable[strings.ToLower(raceName)]
	ci := classTable[strings.ToLower(className)]

	c := NewCharacter(name, raceName, className)
	c.Level = level

	// Ability scores: standard array assigned by the class's priority, then racial
	// increases applied.
	for i, ab := range ci.priority {
		c.Abilities.Set(ab, standardArray[i])
	}
	for ab, bonus := range ri.bonus {
		c.Abilities.Set(ab, c.Abilities.Get(ab)+bonus)
	}

	conMod := Modifier(c.Abilities.CON)
	dexMod := Modifier(c.Abilities.DEX)

	// HP: max hit die at level 1, then the fixed average per extra level, plus the
	// CON modifier each level (minimum 1 HP gained per level).
	hp := ci.hitDie + conMod
	perLevel := ci.hitDie/2 + 1
	for l := 2; l <= level; l++ {
		hp += max(perLevel+conMod, 1)
	}
	hp = max(hp, 1)
	c.MaxHP, c.CurrentHP = hp, hp

	// AC from the class's typical armor, capping the DEX contribution as needed.
	acDex := dexMod
	if ci.acDexCap >= 0 && acDex > ci.acDexCap {
		acDex = ci.acDexCap
	}
	c.AC = ci.acBase + acDex

	c.Speed = ri.speed
	c.Initiative = dexMod
	c.ProficiencyBonus = ProficiencyBonusForLevel(level)
	c.Gold = ci.gold

	// Mark the class's default skill proficiencies.
	for i := range c.Skills {
		for _, name := range ci.skills {
			if c.Skills[i].Name == name {
				c.Skills[i].Proficient = true
			}
		}
	}

	if c.Name == "" {
		c.Name = sampleName(ri, className)
	}
	return c
}

// sampleName picks a stable sample name for a race, falling back to the class.
func sampleName(ri raceInfo, class string) string {
	if len(ri.names) > 0 {
		return ri.names[0]
	}
	return "The " + class
}

// defaultRoster is the heterogeneous level-1 party created when none is set:
// distinct races and classes covering the classic martial/arcane/divine/skill mix.
var defaultRoster = []struct{ name, race, class string }{
	{"Alden", "Human", "Fighter"},
	{"Naivara", "Elf", "Wizard"},
	{"Thorin", "Dwarf", "Cleric"},
	{"Pip", "Halfling", "Rogue"},
}

// DefaultParty returns the default heterogeneous level-1 adventuring party, with
// stats generated by the D&D rules above.
func DefaultParty() []*Character {
	party := make([]*Character, 0, len(defaultRoster))
	for _, m := range defaultRoster {
		party = append(party, GenerateCharacter(m.name, m.race, m.class, 1))
	}
	return party
}
