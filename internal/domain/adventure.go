package domain

import (
	"fmt"
	"strings"
)

// SchemaVersion is the current adventure module schema version. Modules declare
// their own schema_version; the loader warns on mismatch but tries to parse.
const SchemaVersion = "1.0"

// Adventure is the complete, authored, immutable content of a D&D-style
// adventure module. It is loaded from the adventure.json inside a .tar.gz
// module and never mutated at runtime — the running game state lives in
// Session instead (see session.go).
type Adventure struct {
	SchemaVersion string `json:"schema_version"`
	ID            string `json:"id"`
	Title         string `json:"title"`
	Author        string `json:"author,omitempty"`
	System        string `json:"system,omitempty"` // e.g. "D&D 5e"
	Language      string `json:"language,omitempty"`
	Summary       string `json:"summary,omitempty"`

	// Background is the hidden lore/context for the DM: why things are the way
	// they are, the villain's plan, the true history behind the adventure.
	Background string `json:"background,omitempty"`

	// Introduction documents how the adventure begins (hook delivery, opening
	// scene). Conclusion documents the possible endings and how to resolve them.
	Introduction string   `json:"introduction,omitempty"`
	Conclusion   string   `json:"conclusion,omitempty"`
	Hooks        []string `json:"hooks,omitempty"`

	Zones    []Zone      `json:"zones,omitempty"`
	NPCs     []NPC       `json:"npcs,omitempty"`
	Events   []Event     `json:"events,omitempty"`
	Items    []Item      `json:"items,omitempty"`
	Factions []Faction   `json:"factions,omitempty"`
	Lore     []LoreEntry `json:"lore,omitempty"`
	Images   []ImageRef  `json:"images,omitempty"`

	Meta map[string]any `json:"meta,omitempty"`
}

// Zone is a coherent region of the adventure (a dungeon level, a town, a
// wilderness area) composed of rooms/locations.
type Zone struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Overview    string   `json:"overview,omitempty"` // DM-facing summary of the zone
	Description string   `json:"description,omitempty"`
	MapImage    string   `json:"map_image,omitempty"` // relative asset path
	Rooms       []Room   `json:"rooms,omitempty"`
	Connections []string `json:"connections,omitempty"` // other zone IDs reachable
}

// Room is a discrete location the party can occupy within a zone.
type Room struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// ReadAloud is boxed text meant to be read to the players verbatim.
	ReadAloud string `json:"read_aloud,omitempty"`
	// DMNotes is hidden information: what should happen here, secrets, tactics.
	DMNotes string `json:"dm_notes,omitempty"`

	Image      string      `json:"image,omitempty"` // relative asset path
	NPCIDs     []string    `json:"npc_ids,omitempty"`
	EventIDs   []string    `json:"event_ids,omitempty"`
	Exits      []Exit      `json:"exits,omitempty"`
	Encounters []Encounter `json:"encounters,omitempty"`
	Treasure   []string    `json:"treasure,omitempty"`
	Features   []Feature   `json:"features,omitempty"`
}

// Exit connects a room to another room or zone.
type Exit struct {
	To          string `json:"to"` // room ID or zone ID
	Direction   string `json:"direction,omitempty"`
	Description string `json:"description,omitempty"`
	Locked      bool   `json:"locked,omitempty"`
}

// Feature is an interactive element in a room: a trap, puzzle, or ability check.
type Feature struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Skill       string `json:"skill,omitempty"` // e.g. "Perception", "Thieves' Tools"
	DC          int    `json:"dc,omitempty"`
	Success     string `json:"success,omitempty"`
	Failure     string `json:"failure,omitempty"`
}

// Encounter is a combat or challenge staged in a room.
type Encounter struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Creatures   []string `json:"creatures,omitempty"`
	Difficulty  string   `json:"difficulty,omitempty"` // easy/medium/hard/deadly
	Tactics     string   `json:"tactics,omitempty"`
}

// NPC is a non-player character with both mechanical stats and roleplay guidance.
type NPC struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"` // e.g. "quest giver", "villain"

	Appearance string `json:"appearance,omitempty"`

	// Roleplay guidance for the DM.
	Personality    string   `json:"personality,omitempty"`
	Motivations    string   `json:"motivations,omitempty"`
	Secrets        string   `json:"secrets,omitempty"`
	Voice          string   `json:"voice,omitempty"` // how to portray them
	Knowledge      []string `json:"knowledge,omitempty"`
	SampleDialogue []string `json:"sample_dialogue,omitempty"`
	Disposition    string   `json:"disposition,omitempty"`

	// Mechanics.
	StatBlock *StatBlock `json:"stat_block,omitempty"`

	Image           string `json:"image,omitempty"`
	DefaultLocation string `json:"default_location,omitempty"` // room ID
}

// StatBlock holds the mechanical combat statistics of an NPC or creature.
type StatBlock struct {
	AC        int           `json:"ac,omitempty"`
	MaxHP     int           `json:"max_hp,omitempty"`
	Speed     string        `json:"speed,omitempty"`
	Abilities AbilityScores `json:"abilities,omitempty"`
	CR        string        `json:"cr,omitempty"` // challenge rating
	Skills    []string      `json:"skills,omitempty"`
	Traits    []string      `json:"traits,omitempty"`
	Actions   []Action      `json:"actions,omitempty"`
}

// Action is a single attack or special ability in a stat block.
type Action struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ToHit       string `json:"to_hit,omitempty"`
	Damage      string `json:"damage,omitempty"`
}

// Event is a scripted moment or branching decision documented for the DM.
type Event struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Trigger      string    `json:"trigger,omitempty"` // what player action triggers it
	Description  string    `json:"description,omitempty"`
	ReadAloud    string    `json:"read_aloud,omitempty"`
	DMNotes      string    `json:"dm_notes,omitempty"`
	Consequences string    `json:"consequences,omitempty"`
	Outcomes     []Outcome `json:"outcomes,omitempty"`
}

// Outcome is one branch of an Event.
type Outcome struct {
	Condition string `json:"condition"`
	Result    string `json:"result"`
}

// Item is a notable object, treasure, or magic item in the adventure.
type Item struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Rarity      string `json:"rarity,omitempty"`
	Mechanics   string `json:"mechanics,omitempty"`
	Image       string `json:"image,omitempty"`
}

// Faction is an organization or group with goals in the adventure.
type Faction struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Goals       string `json:"goals,omitempty"`
}

// LoreEntry is a piece of world background the DM can reference.
type LoreEntry struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// ImageRef catalogs an image asset shipped in the module.
type ImageRef struct {
	ID          string `json:"id"`
	Path        string `json:"path"`           // relative asset path
	Kind        string `json:"kind,omitempty"` // "map" | "art"
	Description string `json:"description,omitempty"`
}

// --- Lookups -------------------------------------------------------------

// Zone returns the zone with the given ID, or nil.
func (a *Adventure) Zone(id string) *Zone {
	for i := range a.Zones {
		if a.Zones[i].ID == id {
			return &a.Zones[i]
		}
	}
	return nil
}

// Room returns the room with the given ID (searching every zone) and its zone,
// or (nil, nil).
func (a *Adventure) Room(id string) (*Room, *Zone) {
	for zi := range a.Zones {
		for ri := range a.Zones[zi].Rooms {
			if a.Zones[zi].Rooms[ri].ID == id {
				return &a.Zones[zi].Rooms[ri], &a.Zones[zi]
			}
		}
	}
	return nil, nil
}

// NPC returns the NPC with the given ID, or nil.
func (a *Adventure) NPC(id string) *NPC {
	for i := range a.NPCs {
		if a.NPCs[i].ID == id {
			return &a.NPCs[i]
		}
	}
	return nil
}

// Event returns the event with the given ID, or nil.
func (a *Adventure) Event(id string) *Event {
	for i := range a.Events {
		if a.Events[i].ID == id {
			return &a.Events[i]
		}
	}
	return nil
}

// Item returns the item with the given ID, or nil.
func (a *Adventure) Item(id string) *Item {
	for i := range a.Items {
		if a.Items[i].ID == id {
			return &a.Items[i]
		}
	}
	return nil
}

// ImageRefs returns every distinct relative asset path referenced anywhere in
// the adventure (catalog, zone maps, room/NPC/item art).
func (a *Adventure) ImageRefs() []string {
	seen := make(map[string]bool)
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, img := range a.Images {
		add(img.Path)
	}
	for _, z := range a.Zones {
		add(z.MapImage)
		for _, r := range z.Rooms {
			add(r.Image)
		}
	}
	for _, n := range a.NPCs {
		add(n.Image)
	}
	for _, it := range a.Items {
		add(it.Image)
	}
	return out
}

// --- Validation ----------------------------------------------------------

// ValidateAdventure checks required fields and referential integrity. It
// returns a list of human-readable problems; an empty slice means the module
// is structurally valid. imageExists, if non-nil, is called for each referenced
// relative asset path to confirm the file is present on disk.
func ValidateAdventure(a *Adventure, imageExists func(relPath string) bool) []error {
	var errs []error
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	if a == nil {
		return []error{fmt.Errorf("adventure is nil")}
	}
	if strings.TrimSpace(a.ID) == "" {
		add("adventure: 'id' is required")
	}
	if strings.TrimSpace(a.Title) == "" {
		add("adventure: 'title' is required")
	}
	if len(a.Zones) == 0 {
		add("adventure: at least one zone is required")
	}

	// Collect known IDs and detect duplicates.
	zoneIDs := make(map[string]bool)
	roomIDs := make(map[string]bool)
	npcIDs := make(map[string]bool)
	eventIDs := make(map[string]bool)

	for _, n := range a.NPCs {
		if n.ID == "" {
			add("npc %q: 'id' is required", n.Name)
			continue
		}
		if npcIDs[n.ID] {
			add("npc: duplicate id %q", n.ID)
		}
		npcIDs[n.ID] = true
	}
	for _, e := range a.Events {
		if e.ID == "" {
			add("event %q: 'id' is required", e.Name)
			continue
		}
		if eventIDs[e.ID] {
			add("event: duplicate id %q", e.ID)
		}
		eventIDs[e.ID] = true
	}
	for _, z := range a.Zones {
		if z.ID == "" {
			add("zone %q: 'id' is required", z.Name)
			continue
		}
		if zoneIDs[z.ID] {
			add("zone: duplicate id %q", z.ID)
		}
		zoneIDs[z.ID] = true
		for _, r := range z.Rooms {
			if r.ID == "" {
				add("room %q in zone %q: 'id' is required", r.Name, z.ID)
				continue
			}
			if roomIDs[r.ID] {
				add("room: duplicate id %q", r.ID)
			}
			roomIDs[r.ID] = true
		}
	}

	// Referential integrity.
	for _, z := range a.Zones {
		for _, c := range z.Connections {
			if !zoneIDs[c] {
				add("zone %q: connection references unknown zone %q", z.ID, c)
			}
		}
		for _, r := range z.Rooms {
			for _, nid := range r.NPCIDs {
				if !npcIDs[nid] {
					add("room %q: references unknown npc %q", r.ID, nid)
				}
			}
			for _, eid := range r.EventIDs {
				if !eventIDs[eid] {
					add("room %q: references unknown event %q", r.ID, eid)
				}
			}
			for _, ex := range r.Exits {
				if ex.To != "" && !roomIDs[ex.To] && !zoneIDs[ex.To] {
					add("room %q: exit references unknown room/zone %q", r.ID, ex.To)
				}
			}
		}
	}
	for _, n := range a.NPCs {
		if n.DefaultLocation != "" && !roomIDs[n.DefaultLocation] {
			add("npc %q: default_location references unknown room %q", n.ID, n.DefaultLocation)
		}
	}

	// Image presence.
	if imageExists != nil {
		for _, p := range a.ImageRefs() {
			if !imageExists(p) {
				add("image asset not found: %q", p)
			}
		}
	}

	return errs
}
