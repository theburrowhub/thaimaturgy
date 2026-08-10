package domain

import (
	"fmt"
	"strings"
)

// SchemaVersion is the current adventure module schema version. Modules declare
// their own schema_version; the loader warns on mismatch but tries to parse.
//
// 1.1 adds the directional zone graph (Zone.Exits) and Adventure.StartRoom.
// Older 1.0 modules are migrated on load (see Adventure.Migrate).
const SchemaVersion = "1.1"

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

	// Context positions the adventure for the DM: its setting and tone, the
	// recommended character level and party, how to fit it into a larger campaign,
	// prerequisites, and general running advice. Distinct from Background (which is
	// the in-world history); Context is meta guidance for placing/running it.
	Context string `json:"context,omitempty"`

	// Background is the hidden lore/context for the DM: why things are the way
	// they are, the villain's plan, the true history behind the adventure.
	Background string `json:"background,omitempty"`

	// Introduction documents how the adventure begins (hook delivery, opening
	// scene). Conclusion documents the possible endings and how to resolve them.
	Introduction string   `json:"introduction,omitempty"`
	Conclusion   string   `json:"conclusion,omitempty"`
	Hooks        []string `json:"hooks,omitempty"`

	// StartRoom is the id of the room where the party begins. When empty the
	// loader/session falls back to the first authored room, but authoring an
	// explicit entry point avoids depending on the order zones/rooms are written.
	StartRoom string `json:"start_room,omitempty"`

	Zones    []Zone      `json:"zones,omitempty"`
	NPCs     []NPC       `json:"npcs,omitempty"`
	Events   []Event     `json:"events,omitempty"`
	Items    []Item      `json:"items,omitempty"`
	Tables   []Table     `json:"tables,omitempty"`
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
	MapImage    string   `json:"map_image,omitempty"` // relative asset path (legacy/direct)
	ImageIDs    []string `json:"image_ids,omitempty"` // references into Adventure.Images
	Rooms       []Room   `json:"rooms,omitempty"`
	Connections []string `json:"connections,omitempty"` // DEPRECATED: legacy undirected zone IDs; migrated into Exits on load

	// Exits is the directional zone-adjacency graph: which zone lies in each
	// direction from this one. This is what lets the DM keep the party's marching
	// order (a zone written earlier is not automatically "before" a later one).
	Exits []ZoneExit `json:"exits,omitempty"`
}

// Room is a discrete location the party can occupy within a zone.
type Room struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// ReadAloud is boxed text meant to be read to the players verbatim.
	ReadAloud string `json:"read_aloud,omitempty"`
	// DMNotes is hidden information: what should happen here, secrets, tactics.
	DMNotes string `json:"dm_notes,omitempty"`

	Image      string      `json:"image,omitempty"`     // relative asset path (legacy/direct)
	ImageIDs   []string    `json:"image_ids,omitempty"` // references into Adventure.Images
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

// Direction is a canonical compass/relative direction for a zone or room exit.
type Direction string

const (
	DirNorth     Direction = "north"
	DirSouth     Direction = "south"
	DirEast      Direction = "east"
	DirWest      Direction = "west"
	DirNortheast Direction = "northeast"
	DirNorthwest Direction = "northwest"
	DirSoutheast Direction = "southeast"
	DirSouthwest Direction = "southwest"
	DirUp        Direction = "up"
	DirDown      Direction = "down"
	DirIn        Direction = "in"
	DirOut       Direction = "out"
)

// dirAliases maps common English/Spanish spellings and abbreviations to the
// canonical Direction vocabulary.
var dirAliases = map[string]Direction{
	"n": DirNorth, "north": DirNorth, "norte": DirNorth,
	"s": DirSouth, "south": DirSouth, "sur": DirSouth,
	"e": DirEast, "east": DirEast, "este": DirEast,
	"w": DirWest, "west": DirWest, "oeste": DirWest, "o": DirWest,
	"ne": DirNortheast, "northeast": DirNortheast, "noreste": DirNortheast,
	"nw": DirNorthwest, "no": DirNorthwest, "northwest": DirNorthwest, "noroeste": DirNorthwest,
	"se": DirSoutheast, "southeast": DirSoutheast, "sureste": DirSoutheast, "sudeste": DirSoutheast,
	"sw": DirSouthwest, "so": DirSouthwest, "southwest": DirSouthwest, "suroeste": DirSouthwest, "sudoeste": DirSouthwest,
	"u": DirUp, "up": DirUp, "arriba": DirUp,
	"d": DirDown, "down": DirDown, "abajo": DirDown,
	"in": DirIn, "inside": DirIn, "dentro": DirIn, "adentro": DirIn,
	"out": DirOut, "outside": DirOut, "fuera": DirOut, "afuera": DirOut,
}

// NormalizeDirection maps a free-text direction to the canonical vocabulary.
// Returns ("", false) when it cannot be recognized.
func NormalizeDirection(s string) (Direction, bool) {
	key := strings.ToLower(strings.TrimSpace(s))
	if key == "" {
		return "", false
	}
	if d, ok := dirAliases[key]; ok {
		return d, true
	}
	return "", false
}

// Valid reports whether d is a recognized canonical direction.
func (d Direction) Valid() bool {
	_, ok := dirAliases[strings.ToLower(string(d))]
	return ok
}

// Opposite returns the reverse direction (used to check/derive reciprocal
// zone exits). Returns "" when there is no defined opposite.
func (d Direction) Opposite() Direction {
	switch d {
	case DirNorth:
		return DirSouth
	case DirSouth:
		return DirNorth
	case DirEast:
		return DirWest
	case DirWest:
		return DirEast
	case DirNortheast:
		return DirSouthwest
	case DirSouthwest:
		return DirNortheast
	case DirNorthwest:
		return DirSoutheast
	case DirSoutheast:
		return DirNorthwest
	case DirUp:
		return DirDown
	case DirDown:
		return DirUp
	case DirIn:
		return DirOut
	case DirOut:
		return DirIn
	}
	return ""
}

// ZoneExit is a directional edge in the adventure's zone-adjacency graph: from
// the zone that holds it, going Direction, you reach zone To.
type ZoneExit struct {
	Direction   Direction `json:"direction,omitempty"`
	To          string    `json:"to"` // destination zone ID
	Locked      bool      `json:"locked,omitempty"`
	Condition   string    `json:"condition,omitempty"` // when/how the passage opens (DM-facing)
	Description string    `json:"description,omitempty"`
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

	Image           string   `json:"image,omitempty"`            // relative asset path (legacy/direct)
	ImageIDs        []string `json:"image_ids,omitempty"`        // references into Adventure.Images
	DefaultLocation string   `json:"default_location,omitempty"` // room ID
}

// StatBlock holds the mechanical combat statistics of an NPC or creature. It is a
// full 5e-style stat block; every field beyond the original core (AC/HP/speed/
// abilities/CR/skills/traits/actions) is optional and additive, so modules and
// sessions authored before #26 load unchanged.
type StatBlock struct {
	AC        int           `json:"ac,omitempty"`
	MaxHP     int           `json:"max_hp,omitempty"`
	HitDice   string        `json:"hit_dice,omitempty"` // e.g. "2d6" (average shown as MaxHP)
	Speed     string        `json:"speed,omitempty"`
	Abilities AbilityScores `json:"abilities,omitempty"`
	CR        string        `json:"cr,omitempty"` // challenge rating
	XP        int           `json:"xp,omitempty"`
	ProfBonus int           `json:"proficiency_bonus,omitempty"`

	// Descriptive classification.
	Size      string `json:"size,omitempty"`      // Tiny…Gargantuan
	Type      string `json:"type,omitempty"`      // e.g. Humanoid, Beast, Undead
	Alignment string `json:"alignment,omitempty"` // e.g. "Chaotic Evil"

	// Proficiencies and defenses, each rendered as free-form lines (e.g.
	// "DEX +4", "darkvision 60 ft.", "Poisoned").
	SavingThrows          []string `json:"saving_throws,omitempty"`
	Skills                []string `json:"skills,omitempty"`
	Senses                []string `json:"senses,omitempty"`
	Languages             []string `json:"languages,omitempty"`
	DamageResistances     []string `json:"damage_resistances,omitempty"`
	DamageImmunities      []string `json:"damage_immunities,omitempty"`
	DamageVulnerabilities []string `json:"damage_vulnerabilities,omitempty"`
	ConditionImmunities   []string `json:"condition_immunities,omitempty"`

	Traits           []string `json:"traits,omitempty"`
	Actions          []Action `json:"actions,omitempty"`
	Reactions        []Action `json:"reactions,omitempty"`
	LegendaryActions []Action `json:"legendary_actions,omitempty"`

	// Source notes where the block came from (e.g. "SRD 5.1") for attribution.
	Source string `json:"source,omitempty"`
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
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Rarity      string   `json:"rarity,omitempty"`
	Mechanics   string   `json:"mechanics,omitempty"`
	Image       string   `json:"image,omitempty"`     // relative asset path (legacy/direct)
	ImageIDs    []string `json:"image_ids,omitempty"` // references into Adventure.Images
}

// Table is a lookup or random table from the adventure — random encounters,
// treasure, name lists, "roll a d20" result tables, etc. When Dice is set the
// table is rollable: roll the dice and the row whose Roll range contains the
// result is the outcome. Reference tables (no Dice) are just structured data.
type Table struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"` // what the table is for / when to use it
	Dice        string     `json:"dice,omitempty"`        // e.g. "d20", "2d6", "d100"; set ⇒ rollable
	Headers     []string   `json:"headers,omitempty"`     // column headers (optional)
	Rows        []TableRow `json:"rows,omitempty"`
}

// TableRow is one row of a Table. Roll is the matching roll range for rollable
// tables (e.g. "1", "1-3", "18-20", "01-05"); Cells holds the row's values, one
// per Headers column (or a single result cell).
type TableRow struct {
	Roll  string   `json:"roll,omitempty"`
	Cells []string `json:"cells,omitempty"`
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

// Migrate upgrades a freshly-loaded adventure in place so older modules keep
// working: it normalizes exit directions to the canonical vocabulary and
// backfills the directional zone graph (Zone.Exits) from the legacy undirected
// Connections list. Idempotent — safe to call more than once.
func (a *Adventure) Migrate() {
	if a == nil {
		return
	}
	for zi := range a.Zones {
		z := &a.Zones[zi]
		// Normalize room-exit direction strings.
		for ri := range z.Rooms {
			for ei := range z.Rooms[ri].Exits {
				if d, ok := NormalizeDirection(z.Rooms[ri].Exits[ei].Direction); ok {
					z.Rooms[ri].Exits[ei].Direction = string(d)
				}
			}
		}
		// Backfill directional zone exits from legacy Connections when none are
		// authored. Direction is left empty (unknown) for migrated edges.
		if len(z.Exits) == 0 && len(z.Connections) > 0 {
			for _, c := range z.Connections {
				if c = strings.TrimSpace(c); c != "" {
					z.Exits = append(z.Exits, ZoneExit{To: c})
				}
			}
		}
		// Normalize explicit zone-exit directions.
		for ei := range z.Exits {
			if d, ok := NormalizeDirection(string(z.Exits[ei].Direction)); ok {
				z.Exits[ei].Direction = d
			}
		}
	}
}

// StartRoomID returns the party's entry room: the authored StartRoom when set
// and valid, otherwise the first authored room (fallback). Empty only when the
// adventure has no rooms at all.
func (a *Adventure) StartRoomID() string {
	if a == nil {
		return ""
	}
	if s := strings.TrimSpace(a.StartRoom); s != "" {
		if r, _ := a.Room(s); r != nil {
			return s
		}
	}
	if len(a.Zones) > 0 && len(a.Zones[0].Rooms) > 0 {
		return a.Zones[0].Rooms[0].ID
	}
	return ""
}

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

// Table returns the table with the given ID, or nil.
func (a *Adventure) Table(id string) *Table {
	for i := range a.Tables {
		if a.Tables[i].ID == id {
			return &a.Tables[i]
		}
	}
	return nil
}

// ImageByID returns the catalog image with the given ID, or nil.
func (a *Adventure) ImageByID(id string) *ImageRef {
	for i := range a.Images {
		if a.Images[i].ID == id {
			return &a.Images[i]
		}
	}
	return nil
}

// resolveImages turns a legacy direct path plus a list of catalog image IDs into
// the distinct relative asset paths they denote (in order, deduped). Unknown IDs
// are skipped.
func (a *Adventure) resolveImages(directPath string, ids []string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	add(directPath)
	for _, id := range ids {
		if img := a.ImageByID(id); img != nil {
			add(img.Path)
		}
	}
	return out
}

// ZoneImages returns a zone's image paths (map first when identifiable).
func (a *Adventure) ZoneImages(z *Zone) []string { return a.resolveImages(z.MapImage, z.ImageIDs) }

// ZoneMap returns the best map image path for a zone (its map_image, else the
// first catalog image of kind "map" among its image_ids, else its first image).
func (a *Adventure) ZoneMap(z *Zone) string {
	if strings.TrimSpace(z.MapImage) != "" {
		return z.MapImage
	}
	var first string
	for _, id := range z.ImageIDs {
		img := a.ImageByID(id)
		if img == nil {
			continue
		}
		if first == "" {
			first = img.Path
		}
		if img.Kind == "map" {
			return img.Path
		}
	}
	return first
}

// RoomImages returns a room's image paths.
func (a *Adventure) RoomImages(r *Room) []string { return a.resolveImages(r.Image, r.ImageIDs) }

// NPCImages returns an NPC's image paths.
func (a *Adventure) NPCImages(n *NPC) []string { return a.resolveImages(n.Image, n.ImageIDs) }

// ItemImages returns an item's image paths.
func (a *Adventure) ItemImages(it *Item) []string { return a.resolveImages(it.Image, it.ImageIDs) }

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
	tableIDs := make(map[string]bool)
	for _, t := range a.Tables {
		if t.ID == "" {
			add("table %q: 'id' is required", t.Name)
			continue
		}
		if tableIDs[t.ID] {
			add("table: duplicate id %q", t.ID)
		}
		tableIDs[t.ID] = true
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

	// Image catalog IDs (for image_ids references) and duplicate detection.
	imageIDs := make(map[string]bool)
	for _, img := range a.Images {
		if img.ID == "" {
			continue
		}
		if imageIDs[img.ID] {
			add("image: duplicate id %q", img.ID)
		}
		imageIDs[img.ID] = true
	}
	checkImageIDs := func(owner string, ids []string) {
		for _, id := range ids {
			if !imageIDs[id] {
				add("%s: references unknown image %q", owner, id)
			}
		}
	}

	// Referential integrity.
	for _, z := range a.Zones {
		for _, c := range z.Connections {
			if !zoneIDs[c] {
				add("zone %q: connection references unknown zone %q", z.ID, c)
			}
		}
		for _, ze := range z.Exits {
			if strings.TrimSpace(ze.To) == "" {
				add("zone %q: exit is missing a destination zone", z.ID)
			} else if !zoneIDs[ze.To] {
				add("zone %q: exit references unknown zone %q", z.ID, ze.To)
			}
			if ze.Direction != "" && !ze.Direction.Valid() {
				add("zone %q: exit to %q has invalid direction %q", z.ID, ze.To, ze.Direction)
			}
		}
		checkImageIDs("zone "+z.ID, z.ImageIDs)
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
			checkImageIDs("room "+r.ID, r.ImageIDs)
			for _, ex := range r.Exits {
				if ex.To != "" && !roomIDs[ex.To] && !zoneIDs[ex.To] {
					add("room %q: exit references unknown room/zone %q", r.ID, ex.To)
				}
			}
		}
	}
	if s := strings.TrimSpace(a.StartRoom); s != "" && !roomIDs[s] {
		add("adventure: start_room references unknown room %q", s)
	}
	for _, n := range a.NPCs {
		if n.DefaultLocation != "" && !roomIDs[n.DefaultLocation] {
			add("npc %q: default_location references unknown room %q", n.ID, n.DefaultLocation)
		}
		checkImageIDs("npc "+n.ID, n.ImageIDs)
	}
	for _, it := range a.Items {
		checkImageIDs("item "+it.ID, it.ImageIDs)
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
