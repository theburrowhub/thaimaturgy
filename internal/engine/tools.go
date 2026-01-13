package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/types"
)

var AvailableTools = []types.Tool{
	{
		Name:        "roll_dice",
		Description: "Roll dice using standard notation (e.g., '1d20', '2d6+3', '4d6'). Use this for attack rolls, skill checks, saving throws, and damage.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"notation": {
					"type": "string",
					"description": "Dice notation like '1d20', '2d6+3', '1d8-1'"
				},
				"reason": {
					"type": "string",
					"description": "Why the roll is being made (e.g., 'Attack roll', 'Perception check')"
				}
			},
			"required": ["notation"]
		}`),
	},
	{
		Name:        "update_hp",
		Description: "Modify the player's hit points. Use positive values for healing, negative for damage.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"delta": {
					"type": "integer",
					"description": "Amount to change HP (positive for healing, negative for damage)"
				},
				"reason": {
					"type": "string",
					"description": "What caused the HP change"
				}
			},
			"required": ["delta", "reason"]
		}`),
	},
	{
		Name:        "add_item",
		Description: "Add an item to the player's inventory.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"item": {
					"type": "string",
					"description": "Name of the item"
				},
				"quantity": {
					"type": "integer",
					"description": "How many to add (default: 1)"
				}
			},
			"required": ["item"]
		}`),
	},
	{
		Name:        "remove_item",
		Description: "Remove an item from the player's inventory.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"item": {
					"type": "string",
					"description": "Name of the item to remove"
				},
				"quantity": {
					"type": "integer",
					"description": "How many to remove (default: 1)"
				}
			},
			"required": ["item"]
		}`),
	},
	{
		Name:        "set_condition",
		Description: "Add or remove a condition on the player character.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"condition": {
					"type": "string",
					"description": "The condition name (e.g., 'Poisoned', 'Prone', 'Frightened')"
				},
				"add": {
					"type": "boolean",
					"description": "True to add the condition, false to remove it"
				}
			},
			"required": ["condition", "add"]
		}`),
	},
	{
		Name:        "update_gold",
		Description: "Change the player's gold amount.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"delta": {
					"type": "integer",
					"description": "Amount to change (positive to give, negative to take)"
				},
				"reason": {
					"type": "string",
					"description": "Why gold is changing"
				}
			},
			"required": ["delta", "reason"]
		}`),
	},
	{
		Name:        "award_xp",
		Description: "Award experience points to the player.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"amount": {
					"type": "integer",
					"description": "XP to award"
				},
				"reason": {
					"type": "string",
					"description": "What earned the XP"
				}
			},
			"required": ["amount"]
		}`),
	},
	{
		Name:        "set_location",
		Description: "Update the player's current location.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Name of the new location"
				},
				"description": {
					"type": "string",
					"description": "Description of the location"
				},
				"exits": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Available exits/directions"
				}
			},
			"required": ["name", "description"]
		}`),
	},
	{
		Name:        "add_quest",
		Description: "Add a new quest or update an existing quest.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {
					"type": "string",
					"description": "Unique quest identifier"
				},
				"name": {
					"type": "string",
					"description": "Quest name"
				},
				"description": {
					"type": "string",
					"description": "Quest description"
				},
				"status": {
					"type": "string",
					"description": "Quest status: 'active', 'completed', 'failed'"
				}
			},
			"required": ["id", "name", "status"]
		}`),
	},
	{
		Name:        "skill_check",
		Description: "Perform a skill check for the player against a DC.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"skill": {
					"type": "string",
					"description": "Skill name (e.g., 'Perception', 'Stealth')"
				},
				"dc": {
					"type": "integer",
					"description": "Difficulty class"
				}
			},
			"required": ["skill", "dc"]
		}`),
	},
	{
		Name:        "saving_throw",
		Description: "Perform a saving throw for the player against a DC.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"ability": {
					"type": "string",
					"description": "Ability for the save (STR, DEX, CON, INT, WIS, CHA)"
				},
				"dc": {
					"type": "integer",
					"description": "Difficulty class"
				}
			},
			"required": ["ability", "dc"]
		}`),
	},
}

type ToolRouter struct {
	session *domain.GameSession
}

func NewToolRouter(session *domain.GameSession) *ToolRouter {
	return &ToolRouter{session: session}
}

func (tr *ToolRouter) GetToolDefinitions() []types.Tool {
	return AvailableTools
}

func (tr *ToolRouter) Execute(call types.ToolCall) types.ToolResult {
	var args map[string]any
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return types.ToolResult{
			ToolCallID: call.ID,
			Error:      fmt.Sprintf("Failed to parse arguments: %v", err),
		}
	}

	result := types.ToolResult{ToolCallID: call.ID}

	switch call.Name {
	case "roll_dice":
		result = tr.rollDice(call.ID, args)
	case "update_hp":
		result = tr.updateHP(call.ID, args)
	case "add_item":
		result = tr.addItem(call.ID, args)
	case "remove_item":
		result = tr.removeItem(call.ID, args)
	case "set_condition":
		result = tr.setCondition(call.ID, args)
	case "update_gold":
		result = tr.updateGold(call.ID, args)
	case "award_xp":
		result = tr.awardXP(call.ID, args)
	case "set_location":
		result = tr.setLocation(call.ID, args)
	case "add_quest":
		result = tr.addQuest(call.ID, args)
	case "skill_check":
		result = tr.skillCheck(call.ID, args)
	case "saving_throw":
		result = tr.savingThrow(call.ID, args)
	default:
		result.Error = fmt.Sprintf("Unknown tool: %s", call.Name)
	}

	return result
}

func (tr *ToolRouter) rollDice(id string, args map[string]any) types.ToolResult {
	notation, ok := args["notation"].(string)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'notation' parameter"}
	}

	reason, _ := args["reason"].(string)

	roll, err := RollDice(notation)
	if err != nil {
		return types.ToolResult{ToolCallID: id, Error: err.Error()}
	}

	message := fmt.Sprintf("Rolled %s: %s", roll.String(), roll.ResultString())
	if roll.IsCriticalHit() {
		message += " [CRITICAL HIT!]"
	} else if roll.IsCriticalFail() {
		message += " [CRITICAL FAIL!]"
	}

	event := domain.EventDiceRoll(roll.Notation, roll.Rolls, roll.Total, roll.Modifier)
	if reason != "" {
		event.Message = fmt.Sprintf("%s - %s", reason, event.Message)
	}
	tr.session.State.EventLog.Add(event)

	return types.ToolResult{ToolCallID: id, Content: message}
}

func (tr *ToolRouter) updateHP(id string, args map[string]any) types.ToolResult {
	deltaFloat, ok := args["delta"].(float64)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'delta' parameter"}
	}
	delta := int(deltaFloat)

	reason, _ := args["reason"].(string)
	if reason == "" {
		reason = "unknown"
	}

	char := tr.session.State.Character
	if delta < 0 {
		char.TakeDamage(-delta)
	} else {
		char.Heal(delta)
	}
	tr.session.MarkModified()

	event := domain.EventHPChange(delta, reason, char.CurrentHP, char.MaxHP)
	tr.session.State.EventLog.Add(event)

	return types.ToolResult{
		ToolCallID: id,
		Content:    fmt.Sprintf("HP changed by %d (%s). Current: %d/%d", delta, reason, char.CurrentHP, char.MaxHP),
	}
}

func (tr *ToolRouter) addItem(id string, args map[string]any) types.ToolResult {
	item, ok := args["item"].(string)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'item' parameter"}
	}

	quantity := 1
	if q, ok := args["quantity"].(float64); ok {
		quantity = int(q)
	}

	tr.session.State.Character.AddItem(domain.InventoryItem{Name: item, Quantity: quantity})
	tr.session.MarkModified()

	event := domain.EventItemAdd(item, quantity)
	tr.session.State.EventLog.Add(event)

	return types.ToolResult{
		ToolCallID: id,
		Content:    fmt.Sprintf("Added %dx %s to inventory", quantity, item),
	}
}

func (tr *ToolRouter) removeItem(id string, args map[string]any) types.ToolResult {
	item, ok := args["item"].(string)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'item' parameter"}
	}

	quantity := 1
	if q, ok := args["quantity"].(float64); ok {
		quantity = int(q)
	}

	if !tr.session.State.Character.RemoveItem(item, quantity) {
		return types.ToolResult{
			ToolCallID: id,
			Error:      fmt.Sprintf("Item '%s' not found in inventory", item),
		}
	}
	tr.session.MarkModified()

	event := domain.EventItemRemove(item, quantity)
	tr.session.State.EventLog.Add(event)

	return types.ToolResult{
		ToolCallID: id,
		Content:    fmt.Sprintf("Removed %dx %s from inventory", quantity, item),
	}
}

func (tr *ToolRouter) setCondition(id string, args map[string]any) types.ToolResult {
	condName, ok := args["condition"].(string)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'condition' parameter"}
	}

	add, ok := args["add"].(bool)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'add' parameter"}
	}

	cond := domain.Condition(condName)
	char := tr.session.State.Character

	if add {
		char.AddCondition(cond)
		tr.session.State.EventLog.Add(domain.EventConditionAdd(cond))
	} else {
		char.RemoveCondition(cond)
		tr.session.State.EventLog.Add(domain.EventConditionRemove(cond))
	}
	tr.session.MarkModified()

	action := "added"
	if !add {
		action = "removed"
	}

	return types.ToolResult{
		ToolCallID: id,
		Content:    fmt.Sprintf("Condition '%s' %s", condName, action),
	}
}

func (tr *ToolRouter) updateGold(id string, args map[string]any) types.ToolResult {
	deltaFloat, ok := args["delta"].(float64)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'delta' parameter"}
	}
	delta := int(deltaFloat)

	reason, _ := args["reason"].(string)
	if reason == "" {
		reason = "transaction"
	}

	char := tr.session.State.Character
	char.Gold += delta
	if char.Gold < 0 {
		char.Gold = 0
	}
	tr.session.MarkModified()

	event := domain.EventGoldChange(delta, reason, char.Gold)
	tr.session.State.EventLog.Add(event)

	return types.ToolResult{
		ToolCallID: id,
		Content:    fmt.Sprintf("Gold changed by %d (%s). Total: %d", delta, reason, char.Gold),
	}
}

func (tr *ToolRouter) awardXP(id string, args map[string]any) types.ToolResult {
	amountFloat, ok := args["amount"].(float64)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'amount' parameter"}
	}
	amount := int(amountFloat)

	reason, _ := args["reason"].(string)

	char := tr.session.State.Character
	char.XP += amount
	tr.session.MarkModified()

	event := domain.EventXPGain(amount, char.XP)
	if reason != "" {
		event.Message = fmt.Sprintf("%s - %s", reason, event.Message)
	}
	tr.session.State.EventLog.Add(event)

	return types.ToolResult{
		ToolCallID: id,
		Content:    fmt.Sprintf("Awarded %d XP. Total: %d", amount, char.XP),
	}
}

func (tr *ToolRouter) setLocation(id string, args map[string]any) types.ToolResult {
	name, ok := args["name"].(string)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'name' parameter"}
	}

	description, _ := args["description"].(string)

	var exits []string
	if exitsRaw, ok := args["exits"].([]any); ok {
		for _, e := range exitsRaw {
			if s, ok := e.(string); ok {
				exits = append(exits, s)
			}
		}
	}

	loc := domain.Location{
		Name:        name,
		Description: description,
		Exits:       exits,
	}

	tr.session.State.World.SetLocation(loc)
	tr.session.MarkModified()

	event := domain.EventLocationChange(name)
	tr.session.State.EventLog.Add(event)

	return types.ToolResult{
		ToolCallID: id,
		Content:    fmt.Sprintf("Moved to: %s", name),
	}
}

func (tr *ToolRouter) addQuest(id string, args map[string]any) types.ToolResult {
	questID, ok := args["id"].(string)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'id' parameter"}
	}

	name, ok := args["name"].(string)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'name' parameter"}
	}

	status, _ := args["status"].(string)
	if status == "" {
		status = "active"
	}

	description, _ := args["description"].(string)

	world := tr.session.State.World

	if world.UpdateQuestStatus(questID, status) {
		tr.session.State.EventLog.Add(domain.EventQuestUpdate(name, status))
	} else {
		quest := domain.Quest{
			ID:          questID,
			Name:        name,
			Description: description,
			Status:      status,
		}
		world.AddQuest(quest)
		tr.session.State.EventLog.Add(domain.EventQuestAdd(name))
	}
	tr.session.MarkModified()

	return types.ToolResult{
		ToolCallID: id,
		Content:    fmt.Sprintf("Quest '%s' set to '%s'", name, status),
	}
}

func (tr *ToolRouter) skillCheck(id string, args map[string]any) types.ToolResult {
	skill, ok := args["skill"].(string)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'skill' parameter"}
	}

	dcFloat, ok := args["dc"].(float64)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'dc' parameter"}
	}
	dc := int(dcFloat)

	char := tr.session.State.Character
	bonus := char.SkillBonus(skill)

	roll := RollD20()
	total := roll.Total + bonus
	success := total >= dc

	event := domain.EventSkillCheck(skill, dc, roll.Rolls[0], bonus, success)
	tr.session.State.EventLog.Add(event)

	result := "FAILED"
	if success {
		result = "SUCCESS"
	}

	var critMsg string
	if roll.IsCriticalHit() {
		critMsg = " [NATURAL 20!]"
	} else if roll.IsCriticalFail() {
		critMsg = " [NATURAL 1!]"
	}

	return types.ToolResult{
		ToolCallID: id,
		Content:    fmt.Sprintf("%s check (DC %d): %d + %d = %d [%s]%s", skill, dc, roll.Rolls[0], bonus, total, result, critMsg),
	}
}

func (tr *ToolRouter) savingThrow(id string, args map[string]any) types.ToolResult {
	abilityStr, ok := args["ability"].(string)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'ability' parameter"}
	}

	dcFloat, ok := args["dc"].(float64)
	if !ok {
		return types.ToolResult{ToolCallID: id, Error: "Missing or invalid 'dc' parameter"}
	}
	dc := int(dcFloat)

	char := tr.session.State.Character
	abilityStr = strings.ToUpper(abilityStr)

	var ability domain.Ability
	switch abilityStr {
	case "STR":
		ability = domain.STR
	case "DEX":
		ability = domain.DEX
	case "CON":
		ability = domain.CON
	case "INT":
		ability = domain.INT
	case "WIS":
		ability = domain.WIS
	case "CHA":
		ability = domain.CHA
	default:
		return types.ToolResult{ToolCallID: id, Error: "Invalid ability: " + abilityStr}
	}

	bonus := domain.Modifier(char.Abilities.Get(ability))

	roll := RollD20()
	total := roll.Total + bonus
	success := total >= dc

	event := domain.EventSavingThrow(abilityStr, dc, roll.Rolls[0], bonus, success)
	tr.session.State.EventLog.Add(event)

	result := "FAILED"
	if success {
		result = "SUCCESS"
	}

	var critMsg string
	if roll.IsCriticalHit() {
		critMsg = " [NATURAL 20!]"
	} else if roll.IsCriticalFail() {
		critMsg = " [NATURAL 1!]"
	}

	return types.ToolResult{
		ToolCallID: id,
		Content:    fmt.Sprintf("%s save (DC %d): %d + %d = %d [%s]%s", abilityStr, dc, roll.Rolls[0], bonus, total, result, critMsg),
	}
}

func getInt(args map[string]any, key string) (int, bool) {
	if v, ok := args[key].(float64); ok {
		return int(v), true
	}
	if v, ok := args[key].(string); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}
	return 0, false
}
