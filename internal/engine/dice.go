package engine

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/diceexpr"
)

var diceRng = rand.New(rand.NewSource(time.Now().UnixNano()))
var diceMu sync.Mutex

type DiceRoll struct {
	Notation  string `json:"notation"`
	NumDice   int    `json:"num_dice"`
	DiceSides int    `json:"dice_sides"`
	Modifier  int    `json:"modifier"`
	Rolls     []int  `json:"rolls"`
	Total     int    `json:"total"`
}

func ParseDice(notation string) (*DiceRoll, error) {
	expression, err := diceexpr.Parse(notation)
	if err != nil {
		return nil, err
	}
	return &DiceRoll{
		Notation:  expression.Notation,
		NumDice:   expression.NumDice,
		DiceSides: expression.DiceSides,
		Modifier:  expression.Modifier,
	}, nil
}

func (dr *DiceRoll) Roll() int {
	diceMu.Lock()
	defer diceMu.Unlock()
	dr.Rolls = make([]int, dr.NumDice)
	sum := 0

	for i := 0; i < dr.NumDice; i++ {
		roll := diceRng.Intn(dr.DiceSides) + 1
		dr.Rolls[i] = roll
		sum += roll
	}

	dr.Total = sum + dr.Modifier
	return dr.Total
}

func (dr *DiceRoll) String() string {
	if dr.Modifier > 0 {
		return fmt.Sprintf("%dd%d+%d", dr.NumDice, dr.DiceSides, dr.Modifier)
	} else if dr.Modifier < 0 {
		return fmt.Sprintf("%dd%d%d", dr.NumDice, dr.DiceSides, dr.Modifier)
	}
	return fmt.Sprintf("%dd%d", dr.NumDice, dr.DiceSides)
}

func (dr *DiceRoll) ResultString() string {
	rollsStr := make([]string, len(dr.Rolls))
	for i, r := range dr.Rolls {
		rollsStr[i] = strconv.Itoa(r)
	}

	if dr.Modifier != 0 {
		modStr := fmt.Sprintf("%+d", dr.Modifier)
		return fmt.Sprintf("[%s]%s = %d", strings.Join(rollsStr, "+"), modStr, dr.Total)
	}
	return fmt.Sprintf("[%s] = %d", strings.Join(rollsStr, "+"), dr.Total)
}

func (dr *DiceRoll) IsCriticalHit() bool {
	return dr.NumDice == 1 && dr.DiceSides == 20 && len(dr.Rolls) > 0 && dr.Rolls[0] == 20
}

func (dr *DiceRoll) IsCriticalFail() bool {
	return dr.NumDice == 1 && dr.DiceSides == 20 && len(dr.Rolls) > 0 && dr.Rolls[0] == 1
}

func RollDice(notation string) (*DiceRoll, error) {
	roll, err := ParseDice(notation)
	if err != nil {
		return nil, err
	}
	roll.Roll()
	return roll, nil
}

func Roll(numDice, diceSides, modifier int) *DiceRoll {
	dr := &DiceRoll{
		NumDice:   numDice,
		DiceSides: diceSides,
		Modifier:  modifier,
	}
	dr.Notation = dr.String()
	dr.Roll()
	return dr
}

func RollD20() *DiceRoll {
	return Roll(1, 20, 0)
}

func RollD20WithMod(modifier int) *DiceRoll {
	return Roll(1, 20, modifier)
}

func RollAbilityScore() *DiceRoll {
	diceMu.Lock()
	defer diceMu.Unlock()
	rolls := make([]int, 4)
	for i := 0; i < 4; i++ {
		rolls[i] = diceRng.Intn(6) + 1
	}

	minIdx := 0
	for i, r := range rolls {
		if r < rolls[minIdx] {
			minIdx = i
		}
	}

	sum := 0
	kept := make([]int, 0, 3)
	for i, r := range rolls {
		if i != minIdx {
			sum += r
			kept = append(kept, r)
		}
	}

	return &DiceRoll{
		Notation:  "4d6 drop lowest",
		NumDice:   4,
		DiceSides: 6,
		Modifier:  0,
		Rolls:     kept,
		Total:     sum,
	}
}

func RollFullAbilityScores() [6]int {
	var scores [6]int
	for i := 0; i < 6; i++ {
		scores[i] = RollAbilityScore().Total
	}
	return scores
}

type Roller struct {
	rng *rand.Rand
}

func NewRoller(seed int64) *Roller {
	return &Roller{
		rng: rand.New(rand.NewSource(seed)),
	}
}

func (r *Roller) Roll(notation string) (*DiceRoll, error) {
	roll, err := ParseDice(notation)
	if err != nil {
		return nil, err
	}

	roll.Rolls = make([]int, roll.NumDice)
	sum := 0
	for i := 0; i < roll.NumDice; i++ {
		result := r.rng.Intn(roll.DiceSides) + 1
		roll.Rolls[i] = result
		sum += result
	}
	roll.Total = sum + roll.Modifier

	return roll, nil
}
