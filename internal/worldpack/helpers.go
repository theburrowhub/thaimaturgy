package worldpack

import (
	"encoding/json"
	"regexp"
	"strings"
)

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// ToID converts a human label into a stable snake_case identifier.
func ToID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphaNum.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return s
}

// clonePack returns a deep copy of a pack via JSON round-trip.
func clonePack(p *Pack) (*Pack, error) {
	if p == nil {
		return nil, nil
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var out Pack
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// mergeMeta merges string metadata maps.
func mergeMeta(base map[string]string, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// containsFold reports whether s contains substr case-insensitively.
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// indexAppend appends id to a slice in a map, creating the slice if needed.
func indexAppend(m map[string][]string, key, id string) {
	if key == "" || id == "" {
		return
	}
	m[key] = append(m[key], id)
}

// uniqueStrings returns deduplicated strings preserving first-seen order.
func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// parseRollRange checks if roll value falls within a roll range string like "1-3" or "12".
func parseRollRange(rollSpec string, roll int) bool {
	rollSpec = strings.TrimSpace(rollSpec)
	if rollSpec == "" {
		return false
	}
	if strings.Contains(rollSpec, "-") {
		parts := strings.SplitN(rollSpec, "-", 2)
		if len(parts) != 2 {
			return false
		}
		lo, hi := atoi(parts[0]), atoi(parts[1])
		return roll >= lo && roll <= hi
	}
	return roll == atoi(rollSpec)
}

func atoi(s string) int {
	s = strings.TrimSpace(s)
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
