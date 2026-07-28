package domain

import (
	"encoding/json"
	"strconv"
	"strings"
)

// UnmarshalJSON makes a StatBlock tolerant of the type slips LLMs commonly make
// in generated JSON: a challenge rating given as a number (cr: 5) instead of a
// string, or AC / HP given as strings ("13") instead of numbers. Hand-authored
// modules using the documented types keep working unchanged.
func (s *StatBlock) UnmarshalJSON(data []byte) error {
	type alias StatBlock
	aux := struct {
		CR    json.RawMessage `json:"cr"`
		AC    json.RawMessage `json:"ac"`
		MaxHP json.RawMessage `json:"max_hp"`
		*alias
	}{alias: (*alias)(s)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	s.CR = rawToString(aux.CR)
	if v, ok := rawToInt(aux.AC); ok {
		s.AC = v
	}
	if v, ok := rawToInt(aux.MaxHP); ok {
		s.MaxHP = v
	}
	return nil
}

// rawToString accepts a JSON string or number and returns its string form.
func rawToString(raw json.RawMessage) string {
	t := strings.TrimSpace(string(raw))
	if t == "" || t == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.Trim(t, `"`)
}

// rawToInt accepts a JSON number or numeric string and returns its int value.
func rawToInt(raw json.RawMessage) (int, bool) {
	t := strings.TrimSpace(string(raw))
	if t == "" || t == "null" {
		return 0, false
	}
	var i int
	if json.Unmarshal(raw, &i) == nil {
		return i, true
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return int(f), true
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return n, true
		}
	}
	return 0, false
}
