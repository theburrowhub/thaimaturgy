package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
)

type CommandType int

const (
	CmdUnknown CommandType = iota
	CmdHelp
	CmdNew
	CmdSave
	CmdLoad
	CmdQuit
	CmdCharSet
	CmdInvAdd
	CmdInvRemove
	CmdCondAdd
	CmdCondRemove
	CmdProvider
	CmdModel
	CmdTemp
	CmdSystem
	CmdRoll
	CmdStatus
	CmdInventory
	CmdQuests
	CmdLook
	CmdNarration
)

type Command struct {
	Type    CommandType
	Raw     string
	Args    []string
	Params  map[string]string
}

func ParseCommand(input string) *Command {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	cmd := &Command{
		Raw:    input,
		Args:   []string{},
		Params: make(map[string]string),
	}

	if !strings.HasPrefix(input, "/") && !strings.HasPrefix(input, ":") {
		cmd.Type = CmdNarration
		cmd.Args = []string{input}
		return cmd
	}

	input = strings.TrimPrefix(input, "/")
	input = strings.TrimPrefix(input, ":")

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmdName := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmdName {
	case "help", "h", "?":
		cmd.Type = CmdHelp
	case "new", "n":
		cmd.Type = CmdNew
	case "save", "s":
		cmd.Type = CmdSave
		cmd.Args = args
	case "load", "l":
		cmd.Type = CmdLoad
		cmd.Args = args
	case "quit", "q", "exit":
		cmd.Type = CmdQuit
	case "char", "character", "c":
		if len(args) > 0 && args[0] == "set" {
			cmd.Type = CmdCharSet
			cmd.Params = parseKeyValues(args[1:])
		} else {
			cmd.Type = CmdStatus
		}
	case "inv", "inventory", "i":
		if len(args) > 0 {
			switch args[0] {
			case "add", "a":
				cmd.Type = CmdInvAdd
				cmd.Args = args[1:]
			case "rm", "remove", "r":
				cmd.Type = CmdInvRemove
				cmd.Args = args[1:]
			default:
				cmd.Type = CmdInventory
			}
		} else {
			cmd.Type = CmdInventory
		}
	case "cond", "condition":
		if len(args) > 0 {
			switch args[0] {
			case "add", "a":
				cmd.Type = CmdCondAdd
				cmd.Args = args[1:]
			case "rm", "remove", "r":
				cmd.Type = CmdCondRemove
				cmd.Args = args[1:]
			}
		}
	case "provider", "p":
		cmd.Type = CmdProvider
		cmd.Args = args
	case "model", "m":
		cmd.Type = CmdModel
		cmd.Args = args
	case "temp", "temperature":
		cmd.Type = CmdTemp
		cmd.Args = args
	case "system":
		cmd.Type = CmdSystem
	case "roll", "r":
		cmd.Type = CmdRoll
		cmd.Args = args
	case "status", "st":
		cmd.Type = CmdStatus
	case "quests", "quest":
		cmd.Type = CmdQuests
	case "look":
		cmd.Type = CmdLook
	default:
		cmd.Type = CmdUnknown
		cmd.Args = parts
	}

	return cmd
}

func parseKeyValues(args []string) map[string]string {
	result := make(map[string]string)
	for _, arg := range args {
		if idx := strings.Index(arg, "="); idx > 0 {
			key := strings.ToLower(arg[:idx])
			value := arg[idx+1:]
			result[key] = value
		}
	}
	return result
}

type CommandResult struct {
	Success  bool
	Message  string
	Events   []domain.Event
	Response string
	ShouldQuit bool
	NeedsUI    bool
	UIAction   string
}

type CommandHandler struct {
	session *domain.GameSession
}

func NewCommandHandler(session *domain.GameSession) *CommandHandler {
	return &CommandHandler{session: session}
}

func (h *CommandHandler) Execute(cmd *Command) *CommandResult {
	result := &CommandResult{Success: true, Events: []domain.Event{}}

	switch cmd.Type {
	case CmdHelp:
		result.Response = h.helpText()
	case CmdQuit:
		result.ShouldQuit = true
		result.Message = "Farewell, adventurer..."
	case CmdSave:
		if len(cmd.Args) > 0 {
			h.session.State.SaveName = cmd.Args[0]
		}
		result.NeedsUI = true
		result.UIAction = "save"
		result.Message = fmt.Sprintf("Saving game as '%s'...", h.session.State.SaveName)
	case CmdLoad:
		result.NeedsUI = true
		result.UIAction = "load"
		if len(cmd.Args) > 0 {
			result.Message = cmd.Args[0]
		}
	case CmdNew:
		result.NeedsUI = true
		result.UIAction = "new"
	case CmdCharSet:
		events := h.handleCharSet(cmd.Params)
		result.Events = events
		result.Message = "Character updated"
	case CmdInvAdd:
		if len(cmd.Args) > 0 {
			item := strings.Join(cmd.Args, " ")
			h.session.State.Character.AddItem(domain.InventoryItem{Name: item, Quantity: 1})
			result.Events = append(result.Events, domain.EventItemAdd(item, 1))
			result.Message = fmt.Sprintf("Added %s to inventory", item)
			h.session.MarkModified()
		}
	case CmdInvRemove:
		if len(cmd.Args) > 0 {
			item := strings.Join(cmd.Args, " ")
			if h.session.State.Character.RemoveItem(item, 1) {
				result.Events = append(result.Events, domain.EventItemRemove(item, 1))
				result.Message = fmt.Sprintf("Removed %s from inventory", item)
				h.session.MarkModified()
			} else {
				result.Success = false
				result.Message = fmt.Sprintf("Item '%s' not found in inventory", item)
			}
		}
	case CmdCondAdd:
		if len(cmd.Args) > 0 {
			cond := domain.Condition(strings.Join(cmd.Args, " "))
			h.session.State.Character.AddCondition(cond)
			result.Events = append(result.Events, domain.EventConditionAdd(cond))
			result.Message = fmt.Sprintf("Condition added: %s", cond)
			h.session.MarkModified()
		}
	case CmdCondRemove:
		if len(cmd.Args) > 0 {
			cond := domain.Condition(strings.Join(cmd.Args, " "))
			h.session.State.Character.RemoveCondition(cond)
			result.Events = append(result.Events, domain.EventConditionRemove(cond))
			result.Message = fmt.Sprintf("Condition removed: %s", cond)
			h.session.MarkModified()
		}
	case CmdProvider:
		if len(cmd.Args) > 0 {
			provider := domain.ProviderType(strings.ToLower(cmd.Args[0]))
			if provider == domain.ProviderOpenAI || provider == domain.ProviderAnthropic {
				h.session.Config.Provider = provider
				result.Message = fmt.Sprintf("Provider set to: %s", provider)
				result.Events = append(result.Events, domain.EventSystemMessage(result.Message))
			} else {
				result.Success = false
				result.Message = "Invalid provider. Use 'openai' or 'anthropic'"
			}
		} else {
			result.Message = fmt.Sprintf("Current provider: %s", h.session.Config.Provider)
		}
	case CmdModel:
		if len(cmd.Args) > 0 {
			h.session.Config.Model = cmd.Args[0]
			result.Message = fmt.Sprintf("Model set to: %s", cmd.Args[0])
			result.Events = append(result.Events, domain.EventSystemMessage(result.Message))
		} else {
			result.Message = fmt.Sprintf("Current model: %s", h.session.Config.Model)
		}
	case CmdTemp:
		if len(cmd.Args) > 0 {
			temp, err := strconv.ParseFloat(cmd.Args[0], 64)
			if err == nil && temp >= 0 && temp <= 2 {
				h.session.Config.Temperature = temp
				result.Message = fmt.Sprintf("Temperature set to: %.1f", temp)
			} else {
				result.Success = false
				result.Message = "Invalid temperature. Use a value between 0 and 2"
			}
		} else {
			result.Message = fmt.Sprintf("Current temperature: %.1f", h.session.Config.Temperature)
		}
	case CmdSystem:
		result.NeedsUI = true
		result.UIAction = "system_prompt"
	case CmdRoll:
		if len(cmd.Args) > 0 {
			notation := cmd.Args[0]
			roll, err := RollDice(notation)
			if err != nil {
				result.Success = false
				result.Message = err.Error()
			} else {
				result.Message = fmt.Sprintf("Rolling %s: %s", roll.String(), roll.ResultString())
				result.Events = append(result.Events, domain.EventDiceRoll(roll.Notation, roll.Rolls, roll.Total, roll.Modifier))

				if roll.IsCriticalHit() {
					result.Message += " CRITICAL HIT!"
				} else if roll.IsCriticalFail() {
					result.Message += " CRITICAL FAIL!"
				}
			}
		} else {
			roll := RollD20()
			result.Message = fmt.Sprintf("Rolling d20: %s", roll.ResultString())
			result.Events = append(result.Events, domain.EventDiceRoll(roll.Notation, roll.Rolls, roll.Total, roll.Modifier))
		}
	case CmdStatus:
		result.Response = h.statusText()
	case CmdInventory:
		result.Response = h.inventoryText()
	case CmdQuests:
		result.Response = h.questsText()
	case CmdLook:
		result.Response = h.lookText()
	case CmdNarration:
		result.NeedsUI = true
		result.UIAction = "narration"
		result.Message = cmd.Args[0]
	case CmdUnknown:
		result.Success = false
		result.Message = fmt.Sprintf("Unknown command: %s. Type /help for available commands.", cmd.Raw)
	}

	return result
}

func (h *CommandHandler) handleCharSet(params map[string]string) []domain.Event {
	var events []domain.Event
	char := h.session.State.Character

	for key, value := range params {
		switch key {
		case "name":
			char.Name = value
		case "race":
			char.Race = value
		case "class":
			char.Class = value
		case "level":
			if lvl, err := strconv.Atoi(value); err == nil {
				char.Level = lvl
			}
		case "str":
			if v, err := strconv.Atoi(value); err == nil {
				char.Abilities.STR = v
			}
		case "dex":
			if v, err := strconv.Atoi(value); err == nil {
				char.Abilities.DEX = v
			}
		case "con":
			if v, err := strconv.Atoi(value); err == nil {
				char.Abilities.CON = v
			}
		case "int":
			if v, err := strconv.Atoi(value); err == nil {
				char.Abilities.INT = v
			}
		case "wis":
			if v, err := strconv.Atoi(value); err == nil {
				char.Abilities.WIS = v
			}
		case "cha":
			if v, err := strconv.Atoi(value); err == nil {
				char.Abilities.CHA = v
			}
		case "hp":
			if v, err := strconv.Atoi(value); err == nil {
				char.CurrentHP = v
			}
		case "maxhp":
			if v, err := strconv.Atoi(value); err == nil {
				char.MaxHP = v
			}
		case "ac":
			if v, err := strconv.Atoi(value); err == nil {
				char.AC = v
			}
		case "gold":
			if v, err := strconv.Atoi(value); err == nil {
				char.Gold = v
			}
		}
	}

	h.session.MarkModified()
	events = append(events, domain.EventSystemMessage("Character stats updated"))
	return events
}

func (h *CommandHandler) helpText() string {
	return `
COMMANDS:
  /help, /h, /?         Show this help
  /new, /n              Start new campaign
  /save [name], /s      Save current game
  /load [name], /l      Load saved game
  /quit, /q             Exit game

CHARACTER:
  /char set <key>=<val> Set character attributes
                        Keys: name, race, class, level,
                              str, dex, con, int, wis, cha,
                              hp, maxhp, ac, gold
  /status, /st          Show character status
  /inv, /i              Show inventory
  /inv add <item>       Add item to inventory
  /inv rm <item>        Remove item from inventory
  /cond add <cond>      Add condition
  /cond rm <cond>       Remove condition

GAMEPLAY:
  /roll <dice>          Roll dice (e.g., /roll 2d6+3)
  /look                 Describe current location
  /quests               Show active quests

SETTINGS:
  /provider <name>      Set LLM provider (openai/anthropic)
  /model <id>           Set model ID
  /temp <0-2>           Set temperature
  /system               Edit system prompt

Type any text without / to interact with the DM.
`
}

func (h *CommandHandler) statusText() string {
	c := h.session.State.Character
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s - Level %d %s %s\n", c.Name, c.Level, c.Race, c.Class))
	sb.WriteString(fmt.Sprintf("HP: %d/%d  AC: %d  Speed: %d\n\n", c.CurrentHP, c.MaxHP, c.AC, c.Speed))

	sb.WriteString("ABILITIES:\n")
	sb.WriteString(fmt.Sprintf("  STR: %2d (%s)  DEX: %2d (%s)  CON: %2d (%s)\n",
		c.Abilities.STR, domain.ModifierString(c.Abilities.STR),
		c.Abilities.DEX, domain.ModifierString(c.Abilities.DEX),
		c.Abilities.CON, domain.ModifierString(c.Abilities.CON)))
	sb.WriteString(fmt.Sprintf("  INT: %2d (%s)  WIS: %2d (%s)  CHA: %2d (%s)\n",
		c.Abilities.INT, domain.ModifierString(c.Abilities.INT),
		c.Abilities.WIS, domain.ModifierString(c.Abilities.WIS),
		c.Abilities.CHA, domain.ModifierString(c.Abilities.CHA)))

	if len(c.Conditions) > 0 {
		sb.WriteString(fmt.Sprintf("\nCONDITIONS: %v\n", c.Conditions))
	}

	sb.WriteString(fmt.Sprintf("\nGold: %d  XP: %d\n", c.Gold, c.XP))

	return sb.String()
}

func (h *CommandHandler) inventoryText() string {
	c := h.session.State.Character
	if len(c.Inventory) == 0 {
		return "Your inventory is empty."
	}

	var sb strings.Builder
	sb.WriteString("INVENTORY:\n")
	for _, item := range c.Inventory {
		if item.Quantity > 1 {
			sb.WriteString(fmt.Sprintf("  - %s (x%d)\n", item.Name, item.Quantity))
		} else {
			sb.WriteString(fmt.Sprintf("  - %s\n", item.Name))
		}
	}
	return sb.String()
}

func (h *CommandHandler) questsText() string {
	quests := h.session.State.World.GetActiveQuests()
	if len(quests) == 0 {
		return "No active quests."
	}

	var sb strings.Builder
	sb.WriteString("ACTIVE QUESTS:\n")
	for _, q := range quests {
		sb.WriteString(fmt.Sprintf("  [%s] %s\n", q.Status, q.Name))
		if q.Description != "" {
			sb.WriteString(fmt.Sprintf("        %s\n", q.Description))
		}
	}
	return sb.String()
}

func (h *CommandHandler) lookText() string {
	loc := h.session.State.World.CurrentLocation
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("LOCATION: %s\n\n", loc.Name))
	sb.WriteString(loc.Description)
	if len(loc.Exits) > 0 {
		sb.WriteString(fmt.Sprintf("\n\nExits: %s", strings.Join(loc.Exits, ", ")))
	}
	return sb.String()
}
