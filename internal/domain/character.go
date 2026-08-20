package domain

import (
	"fmt"
	"strings"
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

// Trait is a class/racial/background feature or trait on a character sheet
// (named CharacterTrait-style to avoid colliding with a room's Feature).
type Trait struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"` // e.g. Race, Class, Background, Feat
}

// Spell is an entry in a character's spellbook. Level 0 is a cantrip.
type Spell struct {
	Name        string `json:"name"`
	Level       int    `json:"level"`
	Prepared    bool   `json:"prepared,omitempty"`
	School      string `json:"school,omitempty"`
	Description string `json:"description,omitempty"`
}

// SpellSlots tracks max and used spell slots for spell levels 1..9. Index i holds
// the slots of spell level i+1 (index 0 = 1st-level slots); cantrips use no slots.
type SpellSlots struct {
	Max  [9]int `json:"max"`
	Used [9]int `json:"used"`
}

// slotIndex converts a 1..9 spell level to a slice index, reporting validity.
func slotIndex(level int) (int, bool) {
	if level < 1 || level > 9 {
		return 0, false
	}
	return level - 1, true
}

// MaxAt returns the maximum slots at a spell level (0 for out-of-range levels).
func (s *SpellSlots) MaxAt(level int) int {
	if i, ok := slotIndex(level); ok {
		return s.Max[i]
	}
	return 0
}

// RemainingAt returns the unspent slots at a spell level, clamped to [0, max].
func (s *SpellSlots) RemainingAt(level int) int {
	i, ok := slotIndex(level)
	if !ok {
		return 0
	}
	if r := s.Max[i] - s.Used[i]; r > 0 {
		return r
	}
	return 0
}

// Spellcasting is the spellcasting block of a caster's sheet. It is a pointer on
// Character (nil for non-casters), so martial characters and sessions saved
// before #23 keep an unchanged, compact sheet.
type Spellcasting struct {
	Ability     Ability    `json:"ability"` // the spellcasting ability (INT/WIS/CHA)
	SaveDC      int        `json:"save_dc,omitempty"`
	AttackBonus int        `json:"attack_bonus,omitempty"`
	Slots       SpellSlots `json:"slots"`
	Spells      []Spell    `json:"spells,omitempty"` // spellbook: known / prepared
}

// RestoreAllSlots recovers every spent spell slot (a long rest).
func (sc *Spellcasting) RestoreAllSlots() { sc.Slots.Used = [9]int{} }

type Character struct {
	// ID links a character to a persistent roster entry (issue #33). Empty for an
	// ad-hoc party member; set when the character is saved to / loaded from the
	// campaign roster so progression can be written back to the right entry.
	ID string `json:"id,omitempty"`

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

	ProficiencyBonus int  `json:"proficiency_bonus"`
	Inspiration      bool `json:"inspiration,omitempty"`

	// SavingThrows lists the abilities the character is proficient in for saves.
	SavingThrows []Ability `json:"saving_throws,omitempty"`

	Skills        []Skill         `json:"skills"`
	Inventory     []InventoryItem `json:"inventory"`
	Conditions    []Condition     `json:"conditions"`
	Languages     []string        `json:"languages,omitempty"`
	Proficiencies []string        `json:"proficiencies,omitempty"` // armor / weapons / tools
	Features      []Trait         `json:"features,omitempty"`      // racial / class / background traits

	// Spellcasting is nil for non-casters (backward compatible).
	Spellcasting *Spellcasting `json:"spellcasting,omitempty"`

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

// SaveProficient reports whether the character is proficient in an ability's
// saving throw.
func (c *Character) SaveProficient(a Ability) bool {
	for _, s := range c.SavingThrows {
		if s == a {
			return true
		}
	}
	return false
}

// SaveBonus is the saving-throw bonus for an ability: the ability modifier plus
// the proficiency bonus when proficient.
func (c *Character) SaveBonus(a Ability) int {
	bonus := Modifier(c.Abilities.Get(a))
	if c.SaveProficient(a) {
		bonus += c.ProficiencyBonus
	}
	return bonus
}

// SetSaveProficient adds or removes a saving-throw proficiency.
func (c *Character) SetSaveProficient(a Ability, prof bool) {
	if prof {
		if !c.SaveProficient(a) {
			c.SavingThrows = append(c.SavingThrows, a)
		}
		return
	}
	for i, s := range c.SavingThrows {
		if s == a {
			c.SavingThrows = append(c.SavingThrows[:i], c.SavingThrows[i+1:]...)
			return
		}
	}
}

// SpellSlotsRemaining returns the unspent slots at a spell level (0 for a
// non-caster or an invalid level).
func (c *Character) SpellSlotsRemaining(level int) int {
	if c.Spellcasting == nil {
		return 0
	}
	return c.Spellcasting.Slots.RemainingAt(level)
}

// UseSpellSlot spends one slot at the given spell level, returning false when the
// character is not a caster, the level is invalid, or no slot remains.
func (c *Character) UseSpellSlot(level int) bool {
	if c.Spellcasting == nil {
		return false
	}
	i, ok := slotIndex(level)
	if !ok || c.Spellcasting.Slots.RemainingAt(level) <= 0 {
		return false
	}
	c.Spellcasting.Slots.Used[i]++
	return true
}

// RestoreSpellSlot recovers one spent slot at a spell level (a short-rest feature
// or an undo); it never drops Used below zero. It reports whether it actually
// changed state (false when there was no spent slot to recover), so callers don't
// report a no-op as a restore.
func (c *Character) RestoreSpellSlot(level int) bool {
	if c.Spellcasting == nil {
		return false
	}
	if i, ok := slotIndex(level); ok && c.Spellcasting.Slots.Used[i] > 0 {
		c.Spellcasting.Slots.Used[i]--
		return true
	}
	return false
}

// AddSpell adds (or, by name, updates) a spell in the spellbook, creating the
// spellcasting block if needed so a previously-martial character can learn magic.
func (c *Character) AddSpell(sp Spell) {
	if c.Spellcasting == nil {
		c.Spellcasting = &Spellcasting{Ability: INT}
	}
	for i := range c.Spellcasting.Spells {
		if strings.EqualFold(c.Spellcasting.Spells[i].Name, sp.Name) {
			c.Spellcasting.Spells[i] = sp
			return
		}
	}
	c.Spellcasting.Spells = append(c.Spellcasting.Spells, sp)
}

// RemoveSpell drops a spell from the spellbook by name (case-insensitive).
func (c *Character) RemoveSpell(name string) bool {
	if c.Spellcasting == nil {
		return false
	}
	for i, sp := range c.Spellcasting.Spells {
		if strings.EqualFold(sp.Name, name) {
			c.Spellcasting.Spells = append(c.Spellcasting.Spells[:i], c.Spellcasting.Spells[i+1:]...)
			return true
		}
	}
	return false
}

// SetSpellPrepared toggles a spell's prepared state by name (case-insensitive).
func (c *Character) SetSpellPrepared(name string, prepared bool) bool {
	if c.Spellcasting == nil {
		return false
	}
	for i := range c.Spellcasting.Spells {
		if strings.EqualFold(c.Spellcasting.Spells[i].Name, name) {
			c.Spellcasting.Spells[i].Prepared = prepared
			return true
		}
	}
	return false
}

// Normalize clamps the sheet to a self-consistent state after user editing, so an
// out-of-range value entered by hand can never persist: HP within [0, MaxHP] (a
// positive MaxHP), non-negative temp HP / gold / XP, hit dice used within
// [0, max], item quantities at least 1, and spell slots used within [0, max].
func (c *Character) Normalize() {
	if c.MaxHP < 1 {
		c.MaxHP = 1
	}
	c.SetHP(c.CurrentHP)
	if c.TempHP < 0 {
		c.TempHP = 0
	}
	if c.Gold < 0 {
		c.Gold = 0
	}
	if c.XP < 0 {
		c.XP = 0
	}
	if c.Level < 1 {
		c.Level = 1
	}
	if c.HitDiceUsed < 0 {
		c.HitDiceUsed = 0
	}
	if c.HitDiceUsed > c.HitDiceMax() {
		c.HitDiceUsed = c.HitDiceMax()
	}
	for i := range c.Inventory {
		if c.Inventory[i].Quantity < 1 {
			c.Inventory[i].Quantity = 1
		}
	}
	if c.Spellcasting != nil {
		for i := range c.Spellcasting.Slots.Used {
			if c.Spellcasting.Slots.Used[i] < 0 {
				c.Spellcasting.Slots.Used[i] = 0
			}
			if c.Spellcasting.Slots.Used[i] > c.Spellcasting.Slots.Max[i] {
				c.Spellcasting.Slots.Used[i] = c.Spellcasting.Slots.Max[i]
			}
		}
	}
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
// up to half the total hit dice recovered, and all spell slots restored (D&D 5e).
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
	if c.Spellcasting != nil {
		c.Spellcasting.RestoreAllSlots()
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
