package dnd5e

// LegacyResult preserves the exact text contract consumed by the current tool
// gateway and human session timeline.
type LegacyResult struct {
	Content    string `json:"content"`
	LogMessage string `json:"log_message"`
	LogType    string `json:"log_type"`
}

// CriticalTraits carries the legacy natural-20/natural-1 annotations without
// assigning automatic success or failure semantics.
type CriticalTraits struct {
	CriticalHit  bool `json:"critical_hit"`
	CriticalFail bool `json:"critical_fail"`
}

// DiceRollResult is the structured result of ActionDiceRoll.
type DiceRollResult struct {
	Notation string         `json:"notation"`
	Rolls    []int          `json:"rolls"`
	Modifier int            `json:"modifier"`
	Total    int            `json:"total"`
	Traits   CriticalTraits `json:"traits"`
	Legacy   LegacyResult   `json:"legacy"`
}

// AbilityCheckResult is the structured result of ActionAbilityCheck.
type AbilityCheckResult struct {
	Roll     int            `json:"roll"`
	Modifier int            `json:"modifier"`
	DC       int            `json:"dc"`
	Total    int            `json:"total"`
	Success  bool           `json:"success"`
	Traits   CriticalTraits `json:"traits"`
	Legacy   LegacyResult   `json:"legacy"`
}
