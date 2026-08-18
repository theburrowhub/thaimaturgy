// Package diceexpr parses and evaluates the legacy dice notation used by the
// engine without providing entropy or owning a random-number generator.
package diceexpr

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var diceRegex = regexp.MustCompile(`^(\d+)?d(\d+)([+-]\d+)?$`)

const (
	// MinModifier and MaxModifier bound user-controlled arithmetic while leaving
	// ample room for non-standard systems and compatibility commands.
	MinModifier = -1_000_000
	MaxModifier = 1_000_000
)

// Expression is a parsed legacy NdM+K dice expression. Notation retains the
// lower-cased, trimmed spelling supplied to Parse (so "d20" remains "d20").
type Expression struct {
	Notation  string `json:"notation"`
	NumDice   int    `json:"num_dice"`
	DiceSides int    `json:"dice_sides"`
	Modifier  int    `json:"modifier"`
}

// Parse accepts exactly the grammar, limits, normalization, and error messages
// of engine.ParseDice. It never rolls or otherwise obtains entropy.
func Parse(notation string) (*Expression, error) {
	notation = strings.ToLower(strings.TrimSpace(notation))

	matches := diceRegex.FindStringSubmatch(notation)
	if matches == nil {
		return nil, fmt.Errorf("invalid dice notation: %s (expected format: NdM or NdM+K)", notation)
	}

	numDice := 1
	if matches[1] != "" {
		var err error
		numDice, err = strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("invalid number of dice: %s", matches[1])
		}
	}

	diceSides, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, fmt.Errorf("invalid dice sides: %s", matches[2])
	}

	modifier := 0
	if matches[3] != "" {
		modifier, err = strconv.Atoi(matches[3])
		if err != nil {
			return nil, fmt.Errorf("invalid modifier: %s", matches[3])
		}
	}

	if numDice < 1 || numDice > 100 {
		return nil, fmt.Errorf("number of dice must be between 1 and 100")
	}
	if diceSides < 1 || diceSides > 1000 {
		return nil, fmt.Errorf("dice sides must be between 1 and 1000")
	}
	if modifier < MinModifier || modifier > MaxModifier {
		return nil, fmt.Errorf("modifier must be between %d and %d", MinModifier, MaxModifier)
	}

	return &Expression{
		Notation:  notation,
		NumDice:   numDice,
		DiceSides: diceSides,
		Modifier:  modifier,
	}, nil
}

// String returns the expanded canonical notation used by the legacy DiceRoll
// formatter (for example, "d20" becomes "1d20").
func (e Expression) String() string {
	if e.Modifier > 0 {
		return fmt.Sprintf("%dd%d+%d", e.NumDice, e.DiceSides, e.Modifier)
	}
	if e.Modifier < 0 {
		return fmt.Sprintf("%dd%d%d", e.NumDice, e.DiceSides, e.Modifier)
	}
	return fmt.Sprintf("%dd%d", e.NumDice, e.DiceSides)
}

// Total validates externally supplied rolls and applies the expression's
// modifier.
func (e Expression) Total(rolls []int) (int, error) {
	if len(rolls) != e.NumDice {
		return 0, fmt.Errorf("received %d rolls, want %d", len(rolls), e.NumDice)
	}
	total := e.Modifier
	maxInt := int(^uint(0) >> 1)
	for i, roll := range rolls {
		if roll < 1 || roll > e.DiceSides {
			return 0, fmt.Errorf("roll %d is %d, want a value between 1 and %d", i, roll, e.DiceSides)
		}
		if total > maxInt-roll {
			return 0, fmt.Errorf("dice total exceeds the supported integer range")
		}
		total += roll
	}
	return total, nil
}

// ResultString formats externally supplied rolls exactly like the legacy
// DiceRoll.ResultString method.
func (e Expression) ResultString(rolls []int) (string, error) {
	total, err := e.Total(rolls)
	if err != nil {
		return "", err
	}
	rollsText := make([]string, len(rolls))
	for i, roll := range rolls {
		rollsText[i] = strconv.Itoa(roll)
	}
	if e.Modifier != 0 {
		return fmt.Sprintf("[%s]%+d = %d", strings.Join(rollsText, "+"), e.Modifier, total), nil
	}
	return fmt.Sprintf("[%s] = %d", strings.Join(rollsText, "+"), total), nil
}

// IsCriticalHit reports the legacy natural-20 annotation condition.
func (e Expression) IsCriticalHit(rolls []int) bool {
	return e.NumDice == 1 && e.DiceSides == 20 && len(rolls) > 0 && rolls[0] == 20
}

// IsCriticalFail reports the legacy natural-1 annotation condition.
func (e Expression) IsCriticalFail(rolls []int) bool {
	return e.NumDice == 1 && e.DiceSides == 20 && len(rolls) > 0 && rolls[0] == 1
}
