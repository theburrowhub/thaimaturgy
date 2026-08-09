package domain

import (
	"fmt"
)

type Ability int

const (
	STR Ability = iota
	DEX
	CON
	INT
	WIS
	CHA
)

func (a Ability) String() string {
	return [...]string{"STR", "DEX", "CON", "INT", "WIS", "CHA"}[a]
}

func (a Ability) FullName() string {
	return [...]string{"Strength", "Dexterity", "Constitution", "Intelligence", "Wisdom", "Charisma"}[a]
}

type AbilityScores struct {
	STR int `json:"str"`
	DEX int `json:"dex"`
	CON int `json:"con"`
	INT int `json:"int"`
	WIS int `json:"wis"`
	CHA int `json:"cha"`
}

func (a *AbilityScores) Get(ability Ability) int {
	switch ability {
	case STR:
		return a.STR
	case DEX:
		return a.DEX
	case CON:
		return a.CON
	case INT:
		return a.INT
	case WIS:
		return a.WIS
	case CHA:
		return a.CHA
	}
	return 10
}

func (a *AbilityScores) Set(ability Ability, value int) {
	switch ability {
	case STR:
		a.STR = value
	case DEX:
		a.DEX = value
	case CON:
		a.CON = value
	case INT:
		a.INT = value
	case WIS:
		a.WIS = value
	case CHA:
		a.CHA = value
	}
}

func Modifier(score int) int {
	return (score - 10) / 2
}

func ModifierString(score int) string {
	mod := Modifier(score)
	if mod >= 0 {
		return fmt.Sprintf("+%d", mod)
	}
	return fmt.Sprintf("%d", mod)
}

type Skill struct {
	Name       string  `json:"name"`
	Ability    Ability `json:"ability"`
	Proficient bool    `json:"proficient"`
	Expert     bool    `json:"expert"`
}

var DefaultSkills = []Skill{
	{Name: "Acrobatics", Ability: DEX},
	{Name: "Animal Handling", Ability: WIS},
	{Name: "Arcana", Ability: INT},
	{Name: "Athletics", Ability: STR},
	{Name: "Deception", Ability: CHA},
	{Name: "History", Ability: INT},
	{Name: "Insight", Ability: WIS},
	{Name: "Intimidation", Ability: CHA},
	{Name: "Investigation", Ability: INT},
	{Name: "Medicine", Ability: WIS},
	{Name: "Nature", Ability: INT},
	{Name: "Perception", Ability: WIS},
	{Name: "Performance", Ability: CHA},
	{Name: "Persuasion", Ability: CHA},
	{Name: "Religion", Ability: INT},
	{Name: "Sleight of Hand", Ability: DEX},
	{Name: "Stealth", Ability: DEX},
	{Name: "Survival", Ability: WIS},
}

type InventoryItem struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Weight   float64 `json:"weight,omitempty"`
	Equipped bool    `json:"equipped,omitempty"`
}

type Condition string

const (
	ConditionBlinded       Condition = "Blinded"
	ConditionCharmed       Condition = "Charmed"
	ConditionDeafened      Condition = "Deafened"
	ConditionExhausted     Condition = "Exhausted"
	ConditionFrightened    Condition = "Frightened"
	ConditionGrappled      Condition = "Grappled"
	ConditionIncapacitated Condition = "Incapacitated"
	ConditionInvisible     Condition = "Invisible"
	ConditionParalyzed     Condition = "Paralyzed"
	ConditionPetrified     Condition = "Petrified"
	ConditionPoisoned      Condition = "Poisoned"
	ConditionProne         Condition = "Prone"
	ConditionRestrained    Condition = "Restrained"
	ConditionStunned       Condition = "Stunned"
	ConditionUnconscious   Condition = "Unconscious"
)

type Character struct {
	Name       string `json:"name"`
	Race       string `json:"race"`
	Class      string `json:"class"`
	Level      int    `json:"level"`
	Background string `json:"background"`
	Alignment  string `json:"alignment,omitempty"`

	Abilities AbilityScores `json:"abilities"`

	MaxHP     int `json:"max_hp"`
	CurrentHP int `json:"current_hp"`
	TempHP    int `json:"temp_hp,omitempty"`

	// HitDiceUsed counts hit dice already spent (out of one per level). Recovered
	// on rests; used by ShortRest to bound healing.
	HitDiceUsed int `json:"hit_dice_used,omitempty"`

	AC         int `json:"ac"`
	Initiative int `json:"initiative"`
	Speed      int `json:"speed"`

	ProficiencyBonus int `json:"proficiency_bonus"`

	Skills     []Skill         `json:"skills"`
	Inventory  []InventoryItem `json:"inventory"`
	Conditions []Condition     `json:"conditions"`

	Gold  int    `json:"gold"`
	XP    int    `json:"xp"`
	Notes string `json:"notes,omitempty"`
}

func NewCharacter(name, race, class string) *Character {
	skills := make([]Skill, len(DefaultSkills))
	copy(skills, DefaultSkills)

	return &Character{
		Name:       name,
		Race:       race,
		Class:      class,
		Level:      1,
		Background: "Adventurer",
		Alignment:  "Neutral",
		Abilities: AbilityScores{
			STR: 10,
			DEX: 10,
			CON: 10,
			INT: 10,
			WIS: 10,
			CHA: 10,
		},
		MaxHP:            10,
		CurrentHP:        10,
		AC:               10,
		Initiative:       0,
		Speed:            30,
		ProficiencyBonus: 2,
		Skills:           skills,
		Inventory:        []InventoryItem{},
		Conditions:       []Condition{},
		Gold:             0,
		XP:               0,
	}
}

func (c *Character) SkillBonus(skillName string) int {
	for _, skill := range c.Skills {
		if skill.Name == skillName {
			bonus := Modifier(c.Abilities.Get(skill.Ability))
			if skill.Expert {
				bonus += c.ProficiencyBonus * 2
			} else if skill.Proficient {
				bonus += c.ProficiencyBonus
			}
			return bonus
		}
	}
	return 0
}

func (c *Character) AddItem(item InventoryItem) {
	for i, existing := range c.Inventory {
		if existing.Name == item.Name {
			c.Inventory[i].Quantity += item.Quantity
			return
		}
	}
	c.Inventory = append(c.Inventory, item)
}

func (c *Character) RemoveItem(name string, quantity int) bool {
	for i, item := range c.Inventory {
		if item.Name == name {
			if item.Quantity <= quantity {
				c.Inventory = append(c.Inventory[:i], c.Inventory[i+1:]...)
			} else {
				c.Inventory[i].Quantity -= quantity
			}
			return true
		}
	}
	return false
}

func (c *Character) AddCondition(cond Condition) {
	for _, existing := range c.Conditions {
		if existing == cond {
			return
		}
	}
	c.Conditions = append(c.Conditions, cond)
}

func (c *Character) RemoveCondition(cond Condition) {
	for i, existing := range c.Conditions {
		if existing == cond {
			c.Conditions = append(c.Conditions[:i], c.Conditions[i+1:]...)
			return
		}
	}
}

func (c *Character) HasCondition(cond Condition) bool {
	for _, existing := range c.Conditions {
		if existing == cond {
			return true
		}
	}
	return false
}

func (c *Character) TakeDamage(damage int) {
	if c.TempHP > 0 {
		if damage <= c.TempHP {
			c.TempHP -= damage
			return
		}
		damage -= c.TempHP
		c.TempHP = 0
	}
	c.CurrentHP -= damage
	if c.CurrentHP < 0 {
		c.CurrentHP = 0
	}
}

func (c *Character) Heal(amount int) {
	c.CurrentHP += amount
	if c.CurrentHP > c.MaxHP {
		c.CurrentHP = c.MaxHP
	}
}

// HitDiceMax is the character's total hit dice (one per level, minimum 1).
func (c *Character) HitDiceMax() int {
	if c.Level < 1 {
		return 1
	}
	return c.Level
}

// HitDiceRemaining is how many hit dice the character can still spend.
func (c *Character) HitDiceRemaining() int {
	if r := c.HitDiceMax() - c.HitDiceUsed; r > 0 {
		return r
	}
	return 0
}

// LongRest restores the character after a long rest: HP to max, temp HP cleared,
// and up to half the total hit dice recovered (D&D 5e). Spell slots and other
// long-rest resources will be restored here once implemented (see #23).
func (c *Character) LongRest() {
	c.CurrentHP = c.MaxHP
	c.TempHP = 0
	recover := c.HitDiceMax() / 2
	if recover < 1 {
		recover = 1
	}
	if c.HitDiceUsed -= recover; c.HitDiceUsed < 0 {
		c.HitDiceUsed = 0
	}
}

// ShortRest spends up to `dice` hit dice (bounded by those remaining) to heal.
// Each spent die restores an average hit die (5, i.e. a d8's average) plus the
// CON modifier, at least 1. dice <= 0 spends all remaining. Returns the HP
// actually restored and the number of dice spent.
func (c *Character) ShortRest(dice int) (healed, spent int) {
	avail := c.HitDiceRemaining()
	if dice <= 0 || dice > avail {
		dice = avail
	}
	con := Modifier(c.Abilities.CON)
	for i := 0; i < dice && c.CurrentHP < c.MaxHP; i++ {
		per := 5 + con
		if per < 1 {
			per = 1
		}
		before := c.CurrentHP
		c.Heal(per)
		healed += c.CurrentHP - before
		c.HitDiceUsed++
		spent++
	}
	return healed, spent
}

// SetHP sets current HP directly, clamping to the valid [0, MaxHP] range so an
// explicit set can never persist invalid domain state.
func (c *Character) SetHP(hp int) {
	if hp < 0 {
		hp = 0
	}
	if hp > c.MaxHP {
		hp = c.MaxHP
	}
	c.CurrentHP = hp
}

// SetGold sets gold directly, clamping negatives to zero.
func (c *Character) SetGold(gold int) {
	if gold < 0 {
		gold = 0
	}
	c.Gold = gold
}

// AwardXP grants experience. Non-positive amounts are ignored so an errant call
// can't reduce or negate the character's total.
func (c *Character) AwardXP(amount int) {
	if amount <= 0 {
		return
	}
	c.XP += amount
}

func (c *Character) IsAlive() bool {
	return c.CurrentHP > 0
}

func (c *Character) Summary() string {
	return fmt.Sprintf("%s - Level %d %s %s | HP: %d/%d | AC: %d",
		c.Name, c.Level, c.Race, c.Class, c.CurrentHP, c.MaxHP, c.AC)
}
