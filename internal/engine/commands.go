package engine

import (
	"fmt"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

// CommandType enumerates the DM-facing slash commands.
type CommandType int

const (
	CmdUnknown CommandType = iota
	CmdHelp
	CmdQuit
	CmdSave
	CmdLoad
	CmdImport
	CmdGoto
	CmdRoom
	CmdZone
	CmdNPC
	CmdNPCs
	CmdEvent
	CmdItem
	CmdMap
	CmdArt
	CmdNote
	CmdFlag
	CmdQuests
	CmdParty
	CmdRoll
	CmdSearch
	CmdStatus
	CmdMode   // switch between oracle (assistant) and virtual-DM mode
	CmdOracle // free-form query to the oracle (no slash prefix)
)

// Command is a parsed DM instruction.
type Command struct {
	Type CommandType
	Raw  string
	Args []string
}

// ParseCommand turns raw input into a Command. Text without a leading '/' or
// ':' is an oracle query.
func ParseCommand(input string) *Command {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	cmd := &Command{Raw: input, Args: []string{}}

	if !strings.HasPrefix(input, "/") && !strings.HasPrefix(input, ":") {
		cmd.Type = CmdOracle
		cmd.Args = []string{input}
		return cmd
	}

	body := strings.TrimPrefix(strings.TrimPrefix(input, "/"), ":")
	parts := strings.Fields(body)
	if len(parts) == 0 {
		return nil
	}
	name := strings.ToLower(parts[0])
	cmd.Args = parts[1:]

	switch name {
	case "help", "h", "?":
		cmd.Type = CmdHelp
	case "quit", "q", "exit":
		cmd.Type = CmdQuit
	case "save", "s":
		cmd.Type = CmdSave
	case "load", "l":
		cmd.Type = CmdLoad
	case "import":
		cmd.Type = CmdImport
	case "goto", "enter", "go":
		cmd.Type = CmdGoto
	case "room", "look":
		cmd.Type = CmdRoom
	case "zone":
		cmd.Type = CmdZone
	case "npc":
		cmd.Type = CmdNPC
	case "npcs":
		cmd.Type = CmdNPCs
	case "event":
		cmd.Type = CmdEvent
	case "item":
		cmd.Type = CmdItem
	case "map":
		cmd.Type = CmdMap
	case "art":
		cmd.Type = CmdArt
	case "note":
		cmd.Type = CmdNote
	case "flag":
		cmd.Type = CmdFlag
	case "quests", "quest":
		cmd.Type = CmdQuests
	case "party":
		cmd.Type = CmdParty
	case "roll", "r":
		cmd.Type = CmdRoll
	case "search", "find":
		cmd.Type = CmdSearch
	case "status", "st":
		cmd.Type = CmdStatus
	case "mode", "dm", "dj", "gm":
		cmd.Type = CmdMode
	default:
		cmd.Type = CmdUnknown
	}
	return cmd
}

// CommandResult reports the outcome of a command. Local commands fill Response
// (a text block to show) and/or Message (a status line). Commands that require
// the TUI to act set NeedsUI + UIAction (+ UIArg).
type CommandResult struct {
	Success    bool
	Message    string
	Response   string
	ShouldQuit bool
	NeedsUI    bool
	UIAction   string // "oracle" | "save" | "load" | "import" | "image"
	UIArg      string
}

// CommandHandler executes DM commands against a running session.
type CommandHandler struct {
	session *domain.Session
}

// NewCommandHandler binds a handler to a session.
func NewCommandHandler(session *domain.Session) *CommandHandler {
	return &CommandHandler{session: session}
}

func (h *CommandHandler) adv() *domain.Adventure      { return h.session.Adventure }
func (h *CommandHandler) state() *domain.SessionState { return h.session.State }

// Execute runs a command and returns its result.
func (h *CommandHandler) Execute(cmd *Command) *CommandResult {
	r := &CommandResult{Success: true}

	switch cmd.Type {
	case CmdHelp:
		r.Response = helpText()
	case CmdStatus:
		r.Response = h.statusText()
	case CmdQuit:
		r.ShouldQuit = true
		r.Message = "Farewell, Dungeon Master."
	case CmdSave:
		if len(cmd.Args) > 0 {
			h.state().Name = cmd.Args[0]
		}
		r.NeedsUI, r.UIAction = true, "save"
		r.Message = "Saving session '" + h.state().Name + "'..."
	case CmdLoad:
		r.NeedsUI, r.UIAction = true, "load"
		if len(cmd.Args) > 0 {
			r.UIArg = cmd.Args[0]
		}
	case CmdImport:
		r.NeedsUI, r.UIAction = true, "import"
		if len(cmd.Args) > 0 {
			r.UIArg = strings.Join(cmd.Args, " ")
		}
	case CmdOracle:
		r.NeedsUI, r.UIAction = true, "oracle"
		r.UIArg = cmd.Args[0]
	case CmdGoto:
		h.handleGoto(cmd, r)
	case CmdRoom:
		h.handleRoom(cmd, r)
	case CmdZone:
		h.handleZone(cmd, r)
	case CmdNPC:
		h.handleNPC(cmd, r)
	case CmdNPCs:
		r.Response = h.presentNPCsText()
	case CmdEvent:
		h.handleEvent(cmd, r)
	case CmdItem:
		h.handleItem(cmd, r)
	case CmdMap:
		h.handleMap(cmd, r)
	case CmdArt:
		h.handleArt(cmd, r)
	case CmdNote:
		h.handleNote(cmd, r)
	case CmdFlag:
		h.handleFlag(cmd, r)
	case CmdQuests:
		r.Response = h.questsText()
	case CmdParty:
		r.Response = h.partyText()
	case CmdRoll:
		h.handleRoll(cmd, r)
	case CmdSearch:
		h.handleSearch(cmd, r)
	case CmdMode:
		h.handleMode(cmd, r)
	case CmdUnknown:
		r.Success = false
		r.Message = "Unknown command: " + cmd.Raw + ". Type /help."
	}
	return r
}

func (h *CommandHandler) handleGoto(cmd *Command, r *CommandResult) {
	if len(cmd.Args) == 0 {
		r.Success, r.Message = false, "Usage: /goto <room_id>"
		return
	}
	rid := cmd.Args[len(cmd.Args)-1] // allow "/goto <zone> <room>"
	room, zone := h.adv().Room(rid)
	if room == nil {
		r.Success, r.Message = false, "No room with id "+rid
		return
	}
	zid := ""
	if zone != nil {
		zid = zone.ID
	}
	h.state().SetLocation(zid, rid, room.Name)
	h.session.MarkModified()
	r.Message = "Party moved to " + room.Name
	r.Response = FormatRoom(h.adv(), room)
}

func (h *CommandHandler) handleRoom(_ *Command, r *CommandResult) {
	room, _ := h.adv().Room(h.state().CurrentRoom)
	if room == nil {
		r.Success, r.Message = false, "No current room. Use /goto <room_id>."
		return
	}
	r.Response = FormatRoom(h.adv(), room)
}

func (h *CommandHandler) handleZone(cmd *Command, r *CommandResult) {
	zid := h.state().CurrentZone
	if len(cmd.Args) > 0 {
		zid = cmd.Args[0]
	}
	z := h.adv().Zone(zid)
	if z == nil {
		r.Success, r.Message = false, "No zone with id "+zid
		return
	}
	r.Response = FormatZone(h.adv(), z)
}

func (h *CommandHandler) handleNPC(cmd *Command, r *CommandResult) {
	if len(cmd.Args) == 0 {
		r.Success, r.Message = false, "Usage: /npc <id>"
		return
	}
	n := h.adv().NPC(cmd.Args[0])
	if n == nil {
		r.Success, r.Message = false, "No NPC with id "+cmd.Args[0]
		return
	}
	r.Response = FormatNPC(h.adv(), n)
}

func (h *CommandHandler) handleEvent(cmd *Command, r *CommandResult) {
	if len(cmd.Args) == 0 {
		r.Success, r.Message = false, "Usage: /event <id>"
		return
	}
	e := h.adv().Event(cmd.Args[0])
	if e == nil {
		r.Success, r.Message = false, "No event with id "+cmd.Args[0]
		return
	}
	r.Response = FormatEvent(e)
}

func (h *CommandHandler) handleItem(cmd *Command, r *CommandResult) {
	if len(cmd.Args) == 0 {
		r.Success, r.Message = false, "Usage: /item <id>"
		return
	}
	it := h.adv().Item(cmd.Args[0])
	if it == nil {
		r.Success, r.Message = false, "No item with id "+cmd.Args[0]
		return
	}
	r.Response = FormatItem(h.adv(), it)
}

func (h *CommandHandler) handleMap(cmd *Command, r *CommandResult) {
	zid := h.state().CurrentZone
	if len(cmd.Args) > 0 {
		zid = cmd.Args[0]
	}
	z := h.adv().Zone(zid)
	mapPath := ""
	if z != nil {
		mapPath = h.adv().ZoneMap(z)
	}
	if mapPath == "" {
		r.Success, r.Message = false, "No map image for zone "+zid
		return
	}
	r.NeedsUI, r.UIAction, r.UIArg = true, "image", mapPath
	r.Message = "Opening map: " + mapPath
}

// handleArt resolves an art reference (npc/room/item id, image catalog id, or a
// literal relative path) to a relative asset path for the TUI to open.
func (h *CommandHandler) handleArt(cmd *Command, r *CommandResult) {
	if len(cmd.Args) == 0 {
		r.Success, r.Message = false, "Usage: /art <npc|room|item|image id | path>"
		return
	}
	ref := cmd.Args[0]
	path := h.resolveArt(ref)
	if path == "" {
		r.Success, r.Message = false, "No art found for "+ref
		return
	}
	r.NeedsUI, r.UIAction, r.UIArg = true, "image", path
	r.Message = "Opening art: " + path
}

func (h *CommandHandler) resolveArt(ref string) string {
	adv := h.adv()
	if n := adv.NPC(ref); n != nil {
		if imgs := adv.NPCImages(n); len(imgs) > 0 {
			return imgs[0]
		}
	}
	if room, _ := adv.Room(ref); room != nil {
		if imgs := adv.RoomImages(room); len(imgs) > 0 {
			return imgs[0]
		}
	}
	if it := adv.Item(ref); it != nil {
		if imgs := adv.ItemImages(it); len(imgs) > 0 {
			return imgs[0]
		}
	}
	if img := adv.ImageByID(ref); img != nil {
		return img.Path
	}
	if strings.Contains(ref, "/") || strings.Contains(ref, ".") {
		return ref // treat as literal relative path
	}
	return ""
}

func (h *CommandHandler) handleNote(cmd *Command, r *CommandResult) {
	if len(cmd.Args) == 0 {
		r.Success, r.Message = false, "Usage: /note <text>"
		return
	}
	text := strings.Join(cmd.Args, " ")
	h.state().AddNote(text)
	h.session.MarkModified()
	r.Message = "Noted."
}

func (h *CommandHandler) handleFlag(cmd *Command, r *CommandResult) {
	if len(cmd.Args) == 0 {
		r.Success, r.Message = false, "Usage: /flag key=true|false"
		return
	}
	arg := strings.Join(cmd.Args, "")
	idx := strings.Index(arg, "=")
	if idx <= 0 {
		r.Success, r.Message = false, "Usage: /flag key=true|false"
		return
	}
	key := arg[:idx]
	val := strings.EqualFold(arg[idx+1:], "true")
	h.state().SetFlag(key, val)
	h.session.MarkModified()
	r.Message = fmt.Sprintf("Flag %s = %v", key, val)
}

func (h *CommandHandler) handleRoll(cmd *Command, r *CommandResult) {
	notation := "1d20"
	if len(cmd.Args) > 0 {
		notation = cmd.Args[0]
	}
	roll, err := RollDice(notation)
	if err != nil {
		r.Success, r.Message = false, err.Error()
		return
	}
	msg := fmt.Sprintf("Rolled %s: %s", roll.String(), roll.ResultString())
	if roll.IsCriticalHit() {
		msg += " CRIT!"
	} else if roll.IsCriticalFail() {
		msg += " FUMBLE!"
	}
	h.state().Log.Add(domain.LogEntry{Type: domain.LogRoll, Message: msg})
	h.session.MarkModified()
	r.Message = msg
}

func (h *CommandHandler) handleSearch(cmd *Command, r *CommandResult) {
	if len(cmd.Args) == 0 {
		r.Success, r.Message = false, "Usage: /search <query>"
		return
	}
	router := NewToolRouter(h.session)
	res := router.searchModule("", map[string]any{"query": strings.Join(cmd.Args, " ")})
	if res.Error != "" {
		r.Success, r.Message = false, res.Error
		return
	}
	r.Response = res.Content
}

// handleMode switches between oracle (assistant) and virtual-DM mode. With no
// argument it toggles; "oracle"/"dm" set it explicitly. It signals the frontend
// (UIAction "mode") so it can re-render its panels for the new mode.
func (h *CommandHandler) handleMode(cmd *Command, r *CommandResult) {
	st := h.state()
	var target domain.SessionMode
	if len(cmd.Args) > 0 {
		switch strings.ToLower(cmd.Args[0]) {
		case "dm", "dj", "gm", "virtual", "player":
			target = domain.ModeVirtualDM
		case "oracle", "assistant", "help":
			target = domain.ModeAssistant
		default:
			r.Success, r.Message = false, "Usage: /mode [oracle|dm]"
			return
		}
		st.SetMode(target)
	} else {
		target = st.ToggleMode()
	}
	label := "Oracle (assistant to the human DM)"
	if target == domain.ModeVirtualDM {
		label = "Virtual DM (the AI runs the game; you play)"
	}
	r.Message = "Mode: " + label
	r.NeedsUI, r.UIAction, r.UIArg = true, "mode", string(target)
}

func (h *CommandHandler) presentNPCsText() string {
	room, _ := h.adv().Room(h.state().CurrentRoom)
	if room == nil || len(room.NPCIDs) == 0 {
		return "No NPCs in the current room."
	}
	var sb strings.Builder
	sb.WriteString("NPCs here:\n")
	for _, nid := range room.NPCIDs {
		if n := h.adv().NPC(nid); n != nil {
			fmt.Fprintf(&sb, "  - %s [%s] %s\n", n.Name, n.ID, n.Role)
		}
	}
	return sb.String()
}

func (h *CommandHandler) questsText() string {
	if len(h.state().Quests) == 0 {
		return "No tracked quests."
	}
	var sb strings.Builder
	sb.WriteString("QUESTS:\n")
	for _, q := range h.state().Quests {
		fmt.Fprintf(&sb, "  [%s] %s\n", q.Status, q.Name)
	}
	return sb.String()
}

func (h *CommandHandler) partyText() string {
	if len(h.state().Party) == 0 {
		return "No party members tracked. The oracle adds them via update_party_member."
	}
	var sb strings.Builder
	sb.WriteString("PARTY:\n")
	for _, p := range h.state().Party {
		fmt.Fprintf(&sb, "  %s — HP %d/%d AC %d %s\n", p.Name, p.CurrentHP, p.MaxHP, p.AC, p.Notes)
	}
	return sb.String()
}

func (h *CommandHandler) statusText() string {
	st := h.state()
	room, zone := h.adv().Room(st.CurrentRoom)
	var sb strings.Builder
	fmt.Fprintf(&sb, "Adventure: %s\n", h.adv().Title)
	if zone != nil {
		fmt.Fprintf(&sb, "Zone: %s\n", zone.Name)
	}
	if room != nil {
		fmt.Fprintf(&sb, "Room: %s\n", room.Name)
	}
	fmt.Fprintf(&sb, "Visited rooms: %d | Timeline entries: %d\n", len(st.VisitedRooms), st.LogLen())
	return sb.String()
}

func helpText() string {
	return `DM COMMANDS:
  /help, /?            Show this help
  /import <path>       Import an adventure module (.tar.gz)
  /save [name]         Save the session
  /load [name]         Load a session
  /quit                Exit

NAVIGATION & CONTENT:
  /goto <room_id>      Move the party to a room (marks it visited)
  /room, /look         Show the current room
  /zone [id]           Show a zone (current if omitted)
  /npc <id>            Show an NPC dossier
  /npcs                List NPCs in the current room
  /event <id>          Show a scripted event
  /item <id>           Show an item
  /search <query>      Search the whole module
  /map [zone_id]       Open a zone map image (external viewer)
  /art <id|path>       Open art for an NPC/room/item/image

SESSION STATE:
  /note <text>         Add a free-form note to the timeline
  /flag key=true|false Set a story flag
  /quests              Show tracked quests
  /party               Show tracked player characters
  /roll <dice>         Roll dice (e.g. /roll 2d6+3)
  /status              Session status
  /mode [oracle|dm]    Toggle Oracle ↔ Virtual DM (AI runs the game; you play)

Type any text without '/' to ask the oracle about the adventure.
In Virtual DM mode, type what your character does instead.`
}
