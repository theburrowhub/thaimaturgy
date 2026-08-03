package engine

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/types"
)

// AvailableTools is the oracle tool set. Tools fall into three groups:
//   - retrieval: read authored content from the adventure module on demand
//   - mutation:  record the running session state the DM feeds in
//   - dice:      quick mechanical rolls for the DM
var AvailableTools = []types.Tool{
	// --- Retrieval ------------------------------------------------------
	{
		Name:        "get_room",
		Description: "Read the full authored details of a room by its ID (read-aloud text, DM notes, NPCs, features, exits).",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"room_id":{"type":"string","description":"The room ID"}},
			"required":["room_id"]
		}`),
	},
	{
		Name:        "get_zone",
		Description: "Read a zone's overview and its list of rooms by zone ID.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"zone_id":{"type":"string"}},
			"required":["zone_id"]
		}`),
	},
	{
		Name:        "get_npc",
		Description: "Read an NPC's full dossier by ID: appearance, personality, motivations, secrets, voice, knowledge, sample dialogue and stat block.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"npc_id":{"type":"string"}},
			"required":["npc_id"]
		}`),
	},
	{
		Name:        "get_event",
		Description: "Read a scripted event by ID: trigger, description, read-aloud, DM notes, consequences and branching outcomes.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"event_id":{"type":"string"}},
			"required":["event_id"]
		}`),
	},
	{
		Name:        "get_item",
		Description: "Read an item/treasure entry by ID.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"item_id":{"type":"string"}},
			"required":["item_id"]
		}`),
	},
	{
		Name:        "get_table",
		Description: "Read a random/reference table (encounters, treasure, name lists, roll tables) by ID, including all its rows.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"table_id":{"type":"string"}},
			"required":["table_id"]
		}`),
	},
	{
		Name:        "roll_table",
		Description: "Roll on a random table by ID: rolls its dice and returns the resulting row. Use for wandering monsters, random treasure, etc.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"table_id":{"type":"string"}},
			"required":["table_id"]
		}`),
	},
	{
		Name:        "search_module",
		Description: "Search the whole adventure module (zones, rooms, NPCs, events, items, lore) for a keyword and return matching IDs with names.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"query":{"type":"string"}},
			"required":["query"]
		}`),
	},
	{
		Name:        "list_present_npcs",
		Description: "List the NPCs currently in the party's room.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	},
	// --- Session mutation ----------------------------------------------
	{
		Name:        "set_location",
		Description: "Record that the party has moved to a room (and optionally its zone). Marks the room visited.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"room_id":{"type":"string"},
				"zone_id":{"type":"string","description":"Optional; inferred from the room if omitted"}
			},
			"required":["room_id"]
		}`),
	},
	{
		Name:        "mark_npc_met",
		Description: "Record that the party has met an NPC.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"npc_id":{"type":"string"}},
			"required":["npc_id"]
		}`),
	},
	{
		Name:        "set_npc_disposition",
		Description: "Record an NPC's current disposition toward the party (e.g. friendly, hostile, suspicious).",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"npc_id":{"type":"string"},"disposition":{"type":"string"}},
			"required":["npc_id","disposition"]
		}`),
	},
	{
		Name:        "set_npc_alive",
		Description: "Record whether an NPC is alive or dead.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"npc_id":{"type":"string"},"alive":{"type":"boolean"}},
			"required":["npc_id","alive"]
		}`),
	},
	{
		Name:        "trigger_event",
		Description: "Record that a scripted event has occurred at the table.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"event_id":{"type":"string"}},
			"required":["event_id"]
		}`),
	},
	{
		Name:        "set_flag",
		Description: "Set a boolean story flag tracking a decision or world change (e.g. 'gate_opened').",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"key":{"type":"string"},"value":{"type":"boolean"}},
			"required":["key","value"]
		}`),
	},
	{
		Name:        "set_variable",
		Description: "Set a named string variable tracking session state.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"key":{"type":"string"},"value":{"type":"string"}},
			"required":["key","value"]
		}`),
	},
	{
		Name:        "log_note",
		Description: "Append a free-form note to the session timeline recording what happened.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"text":{"type":"string"}},
			"required":["text"]
		}`),
	},
	{
		Name:        "advance_quest",
		Description: "Create or update a quest/objective's status (active, completed, failed).",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"id":{"type":"string"},"name":{"type":"string"},"status":{"type":"string"}},
			"required":["id","status"]
		}`),
	},
	{
		Name:        "update_party_member",
		Description: "Record or update a player character the DM is tracking (HP, AC, notes).",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"name":{"type":"string"},
				"current_hp":{"type":"integer"},
				"max_hp":{"type":"integer"},
				"ac":{"type":"integer"},
				"notes":{"type":"string"}
			},
			"required":["name"]
		}`),
	},
	// --- Dice -----------------------------------------------------------
	{
		Name:        "roll_dice",
		Description: "Roll dice in standard notation (e.g. '1d20', '2d6+3', '8d6'). Use for the DM's quick rolls.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"notation":{"type":"string"},"reason":{"type":"string"}},
			"required":["notation"]
		}`),
	},
	{
		Name:        "ability_check",
		Description: "Roll a d20 + modifier against a DC and report success/failure.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"modifier":{"type":"integer"},
				"dc":{"type":"integer"},
				"label":{"type":"string","description":"What the check is for"}
			},
			"required":["modifier","dc"]
		}`),
	},
}

// ToolRouter executes oracle tool calls against a running session.
type ToolRouter struct {
	session *domain.Session
}

// NewToolRouter binds a router to a session.
func NewToolRouter(session *domain.Session) *ToolRouter {
	return &ToolRouter{session: session}
}

// GetToolDefinitions returns the tool schema sent to the LLM.
func (tr *ToolRouter) GetToolDefinitions() []types.Tool { return AvailableTools }

func (tr *ToolRouter) adv() *domain.Adventure      { return tr.session.Adventure }
func (tr *ToolRouter) state() *domain.SessionState { return tr.session.State }

// Execute dispatches a tool call and returns its result.
func (tr *ToolRouter) Execute(call types.ToolCall) types.ToolResult {
	var args map[string]any
	if len(call.Arguments) > 0 {
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return errResult(call.ID, fmt.Sprintf("failed to parse arguments: %v", err))
		}
	}

	switch call.Name {
	case "get_room":
		return tr.getRoom(call.ID, args)
	case "get_zone":
		return tr.getZone(call.ID, args)
	case "get_npc":
		return tr.getNPC(call.ID, args)
	case "get_event":
		return tr.getEvent(call.ID, args)
	case "get_item":
		return tr.getItem(call.ID, args)
	case "get_table":
		return tr.getTable(call.ID, args)
	case "roll_table":
		return tr.rollTable(call.ID, args)
	case "search_module":
		return tr.searchModule(call.ID, args)
	case "list_present_npcs":
		return tr.listPresentNPCs(call.ID)
	case "set_location":
		return tr.setLocation(call.ID, args)
	case "mark_npc_met":
		return tr.markNPCMet(call.ID, args)
	case "set_npc_disposition":
		return tr.setNPCDisposition(call.ID, args)
	case "set_npc_alive":
		return tr.setNPCAlive(call.ID, args)
	case "trigger_event":
		return tr.triggerEvent(call.ID, args)
	case "set_flag":
		return tr.setFlag(call.ID, args)
	case "set_variable":
		return tr.setVariable(call.ID, args)
	case "log_note":
		return tr.logNote(call.ID, args)
	case "advance_quest":
		return tr.advanceQuest(call.ID, args)
	case "update_party_member":
		return tr.updatePartyMember(call.ID, args)
	case "roll_dice":
		return tr.rollDice(call.ID, args)
	case "ability_check":
		return tr.abilityCheck(call.ID, args)
	default:
		return errResult(call.ID, "unknown tool: "+call.Name)
	}
}

// --- Retrieval -----------------------------------------------------------

func (tr *ToolRouter) getRoom(id string, args map[string]any) types.ToolResult {
	rid, _ := args["room_id"].(string)
	r, _ := tr.adv().Room(rid)
	if r == nil {
		return errResult(id, "no room with id "+rid)
	}
	return okResult(id, FormatRoom(tr.adv(), r))
}

func (tr *ToolRouter) getZone(id string, args map[string]any) types.ToolResult {
	zid, _ := args["zone_id"].(string)
	z := tr.adv().Zone(zid)
	if z == nil {
		return errResult(id, "no zone with id "+zid)
	}
	return okResult(id, FormatZone(tr.adv(), z))
}

func (tr *ToolRouter) getNPC(id string, args map[string]any) types.ToolResult {
	nid, _ := args["npc_id"].(string)
	n := tr.adv().NPC(nid)
	if n == nil {
		return errResult(id, "no npc with id "+nid)
	}
	out := FormatNPC(tr.adv(), n)
	if st := tr.state().KnownNPCs[nid]; st != nil {
		out += fmt.Sprintf("\n[session: met=%v alive=%v disposition=%q]", st.Met, st.Alive, st.Disposition)
	}
	return okResult(id, out)
}

func (tr *ToolRouter) getEvent(id string, args map[string]any) types.ToolResult {
	eid, _ := args["event_id"].(string)
	e := tr.adv().Event(eid)
	if e == nil {
		return errResult(id, "no event with id "+eid)
	}
	out := FormatEvent(e)
	if tr.state().TriggeredEvents[eid] {
		out += "\n[session: already triggered]"
	}
	return okResult(id, out)
}

func (tr *ToolRouter) getItem(id string, args map[string]any) types.ToolResult {
	iid, _ := args["item_id"].(string)
	it := tr.adv().Item(iid)
	if it == nil {
		return errResult(id, "no item with id "+iid)
	}
	return okResult(id, FormatItem(tr.adv(), it))
}

func (tr *ToolRouter) getTable(id string, args map[string]any) types.ToolResult {
	tid, _ := args["table_id"].(string)
	t := tr.adv().Table(tid)
	if t == nil {
		return errResult(id, "no table with id "+tid)
	}
	return okResult(id, nameOrID(t.Name, t.ID)+"\n"+TableMarkdown(t))
}

func (tr *ToolRouter) rollTable(id string, args map[string]any) types.ToolResult {
	tid, _ := args["table_id"].(string)
	t := tr.adv().Table(tid)
	if t == nil {
		return errResult(id, "no table with id "+tid)
	}
	roll, row := RollTable(t)
	result := RowText(row)
	if result == "" {
		result = "(no matching row)"
	}
	name := nameOrID(t.Name, t.ID)
	tr.state().AddNote(fmt.Sprintf("Rolled %s (%d): %s", name, roll, result))
	return okResult(id, fmt.Sprintf("Rolled %s → %d: %s", name, roll, result))
}

func (tr *ToolRouter) searchModule(id string, args map[string]any) types.ToolResult {
	q, _ := args["query"].(string)
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return errResult(id, "empty query")
	}
	adv := tr.adv()
	var hits []string
	match := func(kind, id, name string, haystacks ...string) {
		for _, h := range haystacks {
			if strings.Contains(strings.ToLower(h), q) {
				hits = append(hits, fmt.Sprintf("%s [%s] %s", kind, id, name))
				return
			}
		}
	}
	for i := range adv.Zones {
		z := &adv.Zones[i]
		match("zone", z.ID, z.Name, z.Name, z.Overview, z.Description)
		for j := range z.Rooms {
			r := &z.Rooms[j]
			match("room", r.ID, r.Name, r.Name, r.ReadAloud, r.DMNotes)
		}
	}
	for i := range adv.NPCs {
		n := &adv.NPCs[i]
		match("npc", n.ID, n.Name, n.Name, n.Role, n.Personality, n.Motivations, n.Secrets)
	}
	for i := range adv.Events {
		e := &adv.Events[i]
		match("event", e.ID, e.Name, e.Name, e.Trigger, e.Description, e.DMNotes)
	}
	for i := range adv.Items {
		it := &adv.Items[i]
		match("item", it.ID, it.Name, it.Name, it.Description, it.Mechanics)
	}
	for i := range adv.Lore {
		l := &adv.Lore[i]
		match("lore", l.Title, l.Title, l.Title, l.Content)
	}
	if len(hits) == 0 {
		return okResult(id, "no matches for "+q)
	}
	return okResult(id, strings.Join(hits, "\n"))
}

func (tr *ToolRouter) listPresentNPCs(id string) types.ToolResult {
	r, _ := tr.adv().Room(tr.state().CurrentRoom)
	if r == nil {
		return okResult(id, "no current room set")
	}
	if len(r.NPCIDs) == 0 {
		return okResult(id, "no NPCs in the current room")
	}
	var lines []string
	for _, nid := range r.NPCIDs {
		if n := tr.adv().NPC(nid); n != nil {
			lines = append(lines, fmt.Sprintf("%s [%s] — %s", n.Name, n.ID, n.Role))
		}
	}
	return okResult(id, strings.Join(lines, "\n"))
}

// --- Session mutation ----------------------------------------------------

func (tr *ToolRouter) setLocation(id string, args map[string]any) types.ToolResult {
	rid, _ := args["room_id"].(string)
	r, z := tr.adv().Room(rid)
	if r == nil {
		return errResult(id, "no room with id "+rid)
	}
	zid, _ := args["zone_id"].(string)
	if zid == "" && z != nil {
		zid = z.ID
	}
	tr.state().SetLocation(zid, rid, r.Name)
	tr.session.MarkModified()
	return okResult(id, "party is now in "+r.Name)
}

func (tr *ToolRouter) markNPCMet(id string, args map[string]any) types.ToolResult {
	nid, _ := args["npc_id"].(string)
	n := tr.adv().NPC(nid)
	name := nid
	if n != nil {
		name = n.Name
	}
	tr.state().MeetNPC(nid, name)
	tr.session.MarkModified()
	return okResult(id, name+" marked as met")
}

func (tr *ToolRouter) setNPCDisposition(id string, args map[string]any) types.ToolResult {
	nid, _ := args["npc_id"].(string)
	disp, _ := args["disposition"].(string)
	st := tr.state().NPCState(nid)
	st.Disposition = disp
	tr.state().Log.Add(domain.LogEntry{Type: domain.LogNPC, Message: nid + " disposition → " + disp})
	tr.session.MarkModified()
	return okResult(id, "disposition updated")
}

func (tr *ToolRouter) setNPCAlive(id string, args map[string]any) types.ToolResult {
	nid, _ := args["npc_id"].(string)
	alive, _ := args["alive"].(bool)
	st := tr.state().NPCState(nid)
	st.Alive = alive
	status := "alive"
	if !alive {
		status = "dead"
	}
	tr.state().Log.Add(domain.LogEntry{Type: domain.LogNPC, Message: nid + " is now " + status})
	tr.session.MarkModified()
	return okResult(id, nid+" is now "+status)
}

func (tr *ToolRouter) triggerEvent(id string, args map[string]any) types.ToolResult {
	eid, _ := args["event_id"].(string)
	e := tr.adv().Event(eid)
	name := eid
	if e != nil {
		name = e.Name
	}
	tr.state().TriggerEvent(eid, name)
	tr.session.MarkModified()
	return okResult(id, "event triggered: "+name)
}

func (tr *ToolRouter) setFlag(id string, args map[string]any) types.ToolResult {
	key, _ := args["key"].(string)
	val, _ := args["value"].(bool)
	if key == "" {
		return errResult(id, "missing key")
	}
	tr.state().SetFlag(key, val)
	tr.session.MarkModified()
	return okResult(id, fmt.Sprintf("flag %s = %v", key, val))
}

func (tr *ToolRouter) setVariable(id string, args map[string]any) types.ToolResult {
	key, _ := args["key"].(string)
	val, _ := args["value"].(string)
	if key == "" {
		return errResult(id, "missing key")
	}
	tr.state().SetVariable(key, val)
	tr.session.MarkModified()
	return okResult(id, fmt.Sprintf("variable %s = %s", key, val))
}

func (tr *ToolRouter) logNote(id string, args map[string]any) types.ToolResult {
	text, _ := args["text"].(string)
	if strings.TrimSpace(text) == "" {
		return errResult(id, "empty note")
	}
	tr.state().AddNote(text)
	tr.session.MarkModified()
	return okResult(id, "noted")
}

func (tr *ToolRouter) advanceQuest(id string, args map[string]any) types.ToolResult {
	qid, _ := args["id"].(string)
	name, _ := args["name"].(string)
	status, _ := args["status"].(string)
	if qid == "" || status == "" {
		return errResult(id, "missing id or status")
	}
	tr.state().AdvanceQuest(qid, name, status)
	tr.session.MarkModified()
	return okResult(id, fmt.Sprintf("quest %s → %s", qid, status))
}

func (tr *ToolRouter) updatePartyMember(id string, args map[string]any) types.ToolResult {
	name, _ := args["name"].(string)
	if name == "" {
		return errResult(id, "missing name")
	}
	st := tr.state()
	var pm *domain.PartyMember
	for _, m := range st.Party {
		if strings.EqualFold(m.Name, name) {
			pm = m
			break
		}
	}
	if pm == nil {
		pm = &domain.PartyMember{Name: name}
		st.Party = append(st.Party, pm)
	}
	if v, ok := intArg(args, "current_hp"); ok {
		pm.CurrentHP = v
	}
	if v, ok := intArg(args, "max_hp"); ok {
		pm.MaxHP = v
	}
	if v, ok := intArg(args, "ac"); ok {
		pm.AC = v
	}
	if notes, ok := args["notes"].(string); ok && notes != "" {
		pm.Notes = notes
	}
	st.Log.Add(domain.LogEntry{Type: domain.LogParty, Message: "Updated " + name})
	tr.session.MarkModified()
	return okResult(id, "party member updated: "+name)
}

// --- Dice ----------------------------------------------------------------

func (tr *ToolRouter) rollDice(id string, args map[string]any) types.ToolResult {
	notation, ok := args["notation"].(string)
	if !ok {
		return errResult(id, "missing 'notation'")
	}
	reason, _ := args["reason"].(string)
	roll, err := RollDice(notation)
	if err != nil {
		return errResult(id, err.Error())
	}
	msg := fmt.Sprintf("Rolled %s: %s", roll.String(), roll.ResultString())
	if roll.IsCriticalHit() {
		msg += " [CRIT!]"
	} else if roll.IsCriticalFail() {
		msg += " [FUMBLE!]"
	}
	logMsg := msg
	if reason != "" {
		logMsg = reason + " — " + msg
	}
	tr.state().Log.Add(domain.LogEntry{Type: domain.LogRoll, Message: logMsg})
	tr.session.MarkModified()
	return okResult(id, msg)
}

func (tr *ToolRouter) abilityCheck(id string, args map[string]any) types.ToolResult {
	mod, _ := intArg(args, "modifier")
	dc, ok := intArg(args, "dc")
	if !ok {
		return errResult(id, "missing 'dc'")
	}
	label, _ := args["label"].(string)
	roll := RollD20WithMod(mod)
	success := roll.Total >= dc
	res := "FAILURE"
	if success {
		res = "SUCCESS"
	}
	crit := ""
	if roll.IsCriticalHit() {
		crit = " [NAT 20]"
	} else if roll.IsCriticalFail() {
		crit = " [NAT 1]"
	}
	msg := fmt.Sprintf("Check (DC %d): d20(%d)%+d = %d [%s]%s", dc, roll.Rolls[0], mod, roll.Total, res, crit)
	if label != "" {
		msg = label + " — " + msg
	}
	tr.state().Log.Add(domain.LogEntry{Type: domain.LogRoll, Message: msg})
	tr.session.MarkModified()
	return okResult(id, msg)
}

// --- helpers -------------------------------------------------------------

func okResult(id, content string) types.ToolResult {
	return types.ToolResult{ToolCallID: id, Content: content}
}

func errResult(id, msg string) types.ToolResult {
	return types.ToolResult{ToolCallID: id, Error: msg}
}

func intArg(args map[string]any, key string) (int, bool) {
	switch v := args[key].(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	}
	return 0, false
}
