package domain

import (
	"fmt"
	"time"
)

type EventType string

const (
	EventTypeNarration      EventType = "narration"
	EventTypeDiceRoll       EventType = "dice_roll"
	EventTypeHPChange       EventType = "hp_change"
	EventTypeItemAdd        EventType = "item_add"
	EventTypeItemRemove     EventType = "item_remove"
	EventTypeConditionAdd   EventType = "condition_add"
	EventTypeConditionRemove EventType = "condition_remove"
	EventTypeQuestAdd       EventType = "quest_add"
	EventTypeQuestUpdate    EventType = "quest_update"
	EventTypeLocationChange EventType = "location_change"
	EventTypeGoldChange     EventType = "gold_change"
	EventTypeXPGain         EventType = "xp_gain"
	EventTypeLevelUp        EventType = "level_up"
	EventTypeStatChange     EventType = "stat_change"
	EventTypeSkillCheck     EventType = "skill_check"
	EventTypeSavingThrow    EventType = "saving_throw"
	EventTypeAttack         EventType = "attack"
	EventTypeCombatStart    EventType = "combat_start"
	EventTypeCombatEnd      EventType = "combat_end"
	EventTypeRest           EventType = "rest"
	EventTypeTimePass       EventType = "time_pass"
	EventTypeNPCInteraction EventType = "npc_interaction"
	EventTypeSystemMessage  EventType = "system_message"
	EventTypeError          EventType = "error"
)

type Event struct {
	Type      EventType         `json:"type"`
	Message   string            `json:"message"`
	Data      map[string]any    `json:"data,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

func NewEvent(eventType EventType, message string) Event {
	return Event{
		Type:      eventType,
		Message:   message,
		Data:      make(map[string]any),
		Timestamp: time.Now(),
	}
}

func (e *Event) WithData(key string, value any) *Event {
	e.Data[key] = value
	return e
}

func EventDiceRoll(notation string, rolls []int, total int, modifier int) Event {
	e := NewEvent(EventTypeDiceRoll, fmt.Sprintf("Rolled %s: %v = %d", notation, rolls, total))
	e.Data["notation"] = notation
	e.Data["rolls"] = rolls
	e.Data["total"] = total
	e.Data["modifier"] = modifier
	return e
}

func EventHPChange(delta int, reason string, currentHP, maxHP int) Event {
	var msg string
	if delta > 0 {
		msg = fmt.Sprintf("Healed %d HP (%s) [%d/%d]", delta, reason, currentHP, maxHP)
	} else {
		msg = fmt.Sprintf("Took %d damage (%s) [%d/%d]", -delta, reason, currentHP, maxHP)
	}
	e := NewEvent(EventTypeHPChange, msg)
	e.Data["delta"] = delta
	e.Data["reason"] = reason
	e.Data["current_hp"] = currentHP
	e.Data["max_hp"] = maxHP
	return e
}

func EventItemAdd(item string, quantity int) Event {
	e := NewEvent(EventTypeItemAdd, fmt.Sprintf("Added %dx %s to inventory", quantity, item))
	e.Data["item"] = item
	e.Data["quantity"] = quantity
	return e
}

func EventItemRemove(item string, quantity int) Event {
	e := NewEvent(EventTypeItemRemove, fmt.Sprintf("Removed %dx %s from inventory", quantity, item))
	e.Data["item"] = item
	e.Data["quantity"] = quantity
	return e
}

func EventConditionAdd(condition Condition) Event {
	return NewEvent(EventTypeConditionAdd, fmt.Sprintf("Condition applied: %s", condition))
}

func EventConditionRemove(condition Condition) Event {
	return NewEvent(EventTypeConditionRemove, fmt.Sprintf("Condition removed: %s", condition))
}

func EventQuestAdd(questName string) Event {
	return NewEvent(EventTypeQuestAdd, fmt.Sprintf("New quest: %s", questName))
}

func EventQuestUpdate(questName, status string) Event {
	return NewEvent(EventTypeQuestUpdate, fmt.Sprintf("Quest '%s' updated: %s", questName, status))
}

func EventLocationChange(location string) Event {
	return NewEvent(EventTypeLocationChange, fmt.Sprintf("Traveled to: %s", location))
}

func EventGoldChange(delta int, reason string, total int) Event {
	var msg string
	if delta > 0 {
		msg = fmt.Sprintf("Gained %d gold (%s) [Total: %d]", delta, reason, total)
	} else {
		msg = fmt.Sprintf("Spent %d gold (%s) [Total: %d]", -delta, reason, total)
	}
	e := NewEvent(EventTypeGoldChange, msg)
	e.Data["delta"] = delta
	e.Data["reason"] = reason
	e.Data["total"] = total
	return e
}

func EventXPGain(amount int, total int) Event {
	return NewEvent(EventTypeXPGain, fmt.Sprintf("Gained %d XP [Total: %d]", amount, total))
}

func EventLevelUp(newLevel int, className string) Event {
	return NewEvent(EventTypeLevelUp, fmt.Sprintf("Level up! Now Level %d %s", newLevel, className))
}

func EventSkillCheck(skill string, dc int, roll int, bonus int, success bool) Event {
	result := "FAILED"
	if success {
		result = "SUCCESS"
	}
	msg := fmt.Sprintf("%s check (DC %d): %d + %d = %d [%s]", skill, dc, roll, bonus, roll+bonus, result)
	e := NewEvent(EventTypeSkillCheck, msg)
	e.Data["skill"] = skill
	e.Data["dc"] = dc
	e.Data["roll"] = roll
	e.Data["bonus"] = bonus
	e.Data["success"] = success
	return e
}

func EventSavingThrow(ability string, dc int, roll int, bonus int, success bool) Event {
	result := "FAILED"
	if success {
		result = "SUCCESS"
	}
	msg := fmt.Sprintf("%s save (DC %d): %d + %d = %d [%s]", ability, dc, roll, bonus, roll+bonus, result)
	e := NewEvent(EventTypeSavingThrow, msg)
	e.Data["ability"] = ability
	e.Data["dc"] = dc
	e.Data["roll"] = roll
	e.Data["bonus"] = bonus
	e.Data["success"] = success
	return e
}

func EventSystemMessage(message string) Event {
	return NewEvent(EventTypeSystemMessage, message)
}

func EventError(message string) Event {
	return NewEvent(EventTypeError, fmt.Sprintf("ERROR: %s", message))
}

type EventLog struct {
	Events  []Event `json:"events"`
	MaxSize int     `json:"max_size"`
}

func NewEventLog(maxSize int) *EventLog {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &EventLog{
		Events:  []Event{},
		MaxSize: maxSize,
	}
}

func (el *EventLog) Add(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	el.Events = append(el.Events, event)

	if len(el.Events) > el.MaxSize {
		el.Events = el.Events[len(el.Events)-el.MaxSize:]
	}
}

func (el *EventLog) GetLast(n int) []Event {
	if n <= 0 || n > len(el.Events) {
		return el.Events
	}
	return el.Events[len(el.Events)-n:]
}

func (el *EventLog) Clear() {
	el.Events = []Event{}
}

func (el *EventLog) Len() int {
	return len(el.Events)
}
