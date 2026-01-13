package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/theburrowhub/thaimaturgy/internal/domain"
	"github.com/theburrowhub/thaimaturgy/internal/providers"
)

const maxToolIterations = 5

type Orchestrator struct {
	session    *domain.GameSession
	provider   providers.Provider
	toolRouter *ToolRouter
}

func NewOrchestrator(session *domain.GameSession, provider providers.Provider) *Orchestrator {
	return &Orchestrator{
		session:    session,
		provider:   provider,
		toolRouter: NewToolRouter(session),
	}
}

func (o *Orchestrator) SetProvider(provider providers.Provider) {
	o.provider = provider
}

type OrchestratorResponse struct {
	Narrative   string
	Events      []domain.Event
	TokensUsed  int
	LatencyMs   int64
	Error       error
}

func (o *Orchestrator) ProcessInput(ctx context.Context, input string) *OrchestratorResponse {
	response := &OrchestratorResponse{
		Events: []domain.Event{},
	}

	if o.provider == nil {
		response.Error = fmt.Errorf("no AI provider configured")
		return response
	}

	o.session.State.Conversation.AddUserMessage(input)

	messages := o.buildMessages()
	tools := o.toolRouter.GetToolDefinitions()

	req := providers.ChatRequest{
		Messages:    messages,
		Tools:       tools,
		Model:       o.session.Config.Model,
		Temperature: o.session.Config.Temperature,
		MaxTokens:   o.session.Config.MaxTokens,
	}

	totalLatency := int64(0)
	totalTokens := 0

	for iteration := 0; iteration < maxToolIterations; iteration++ {
		resp, err := o.provider.Chat(ctx, req)
		if err != nil {
			response.Error = fmt.Errorf("AI request failed: %w", err)
			return response
		}

		totalLatency += resp.Latency
		totalTokens += resp.Usage.TotalTokens

		if len(resp.ToolCalls) == 0 {
			response.Narrative = resp.Content
			response.LatencyMs = totalLatency
			response.TokensUsed = totalTokens

			o.session.State.Conversation.AddAssistantMessage(resp.Content)
			o.session.MarkModified()

			return response
		}

		assistantMsg := providers.Message{
			Role:      providers.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		req.Messages = append(req.Messages, assistantMsg)

		for _, tc := range resp.ToolCalls {
			typesTC := providers.ConvertToolCallToTypesFormat(tc)
			result := o.toolRouter.Execute(typesTC)

			content := result.Content
			if result.Error != "" {
				content = "Error: " + result.Error
				response.Events = append(response.Events, domain.EventError(result.Error))
			}

			toolMsg := providers.Message{
				Role:       providers.RoleTool,
				Content:    content,
				ToolCallID: tc.ID,
			}
			req.Messages = append(req.Messages, toolMsg)
		}
	}

	response.Error = fmt.Errorf("maximum tool iterations reached")
	response.LatencyMs = totalLatency
	response.TokensUsed = totalTokens
	return response
}

func (o *Orchestrator) buildMessages() []providers.Message {
	var messages []providers.Message

	systemPrompt := o.buildSystemPrompt()
	messages = append(messages, providers.Message{
		Role:    providers.RoleSystem,
		Content: systemPrompt,
	})

	for _, msg := range o.session.State.Conversation.Messages {
		role := providers.RoleUser
		switch msg.Role {
		case domain.RoleAssistant:
			role = providers.RoleAssistant
		case domain.RoleSystem:
			role = providers.RoleSystem
		case domain.RoleTool:
			role = providers.RoleTool
		}

		messages = append(messages, providers.Message{
			Role:       role,
			Content:    msg.Content,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		})
	}

	return messages
}

func (o *Orchestrator) buildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString(o.session.Config.GetSystemPrompt())
	sb.WriteString("\n\n")

	sb.WriteString("=== CURRENT CHARACTER STATE ===\n")
	sb.WriteString(o.formatCharacterState())
	sb.WriteString("\n\n")

	sb.WriteString("=== CURRENT WORLD STATE ===\n")
	sb.WriteString(o.formatWorldState())
	sb.WriteString("\n\n")

	if o.session.State.World.MemorySummary != "" {
		sb.WriteString("=== STORY SO FAR ===\n")
		sb.WriteString(o.session.State.World.MemorySummary)
		sb.WriteString("\n\n")
	}

	sb.WriteString("=== INSTRUCTIONS ===\n")
	sb.WriteString("- Use tools to track game state changes (HP, inventory, conditions, etc.)\n")
	sb.WriteString("- Always use roll_dice for uncertain outcomes\n")
	sb.WriteString("- Keep responses atmospheric and engaging\n")
	sb.WriteString("- End with suggested actions for the player\n")

	return sb.String()
}

func (o *Orchestrator) formatCharacterState() string {
	c := o.session.State.Character
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Name: %s\n", c.Name))
	sb.WriteString(fmt.Sprintf("Race: %s, Class: %s, Level: %d\n", c.Race, c.Class, c.Level))
	sb.WriteString(fmt.Sprintf("HP: %d/%d, AC: %d, Speed: %d ft\n", c.CurrentHP, c.MaxHP, c.AC, c.Speed))

	sb.WriteString(fmt.Sprintf("Abilities: STR %d (%s), DEX %d (%s), CON %d (%s), INT %d (%s), WIS %d (%s), CHA %d (%s)\n",
		c.Abilities.STR, domain.ModifierString(c.Abilities.STR),
		c.Abilities.DEX, domain.ModifierString(c.Abilities.DEX),
		c.Abilities.CON, domain.ModifierString(c.Abilities.CON),
		c.Abilities.INT, domain.ModifierString(c.Abilities.INT),
		c.Abilities.WIS, domain.ModifierString(c.Abilities.WIS),
		c.Abilities.CHA, domain.ModifierString(c.Abilities.CHA)))

	sb.WriteString(fmt.Sprintf("Gold: %d, XP: %d\n", c.Gold, c.XP))

	if len(c.Conditions) > 0 {
		sb.WriteString(fmt.Sprintf("Conditions: %v\n", c.Conditions))
	}

	if len(c.Inventory) > 0 {
		sb.WriteString("Inventory: ")
		items := make([]string, len(c.Inventory))
		for i, item := range c.Inventory {
			if item.Quantity > 1 {
				items[i] = fmt.Sprintf("%s (x%d)", item.Name, item.Quantity)
			} else {
				items[i] = item.Name
			}
		}
		sb.WriteString(strings.Join(items, ", "))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (o *Orchestrator) formatWorldState() string {
	w := o.session.State.World
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Setting: %s\n", w.Setting))
	sb.WriteString(fmt.Sprintf("Location: %s\n", w.CurrentLocation.Name))
	if w.CurrentLocation.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", w.CurrentLocation.Description))
	}
	sb.WriteString(fmt.Sprintf("Time: Day %d, %s\n", w.DayNumber, w.TimeOfDay))

	if len(w.Quests) > 0 {
		sb.WriteString("Active Quests:\n")
		for _, q := range w.GetActiveQuests() {
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", q.Name, q.Description))
		}
	}

	return sb.String()
}

func (o *Orchestrator) StartNewGame(ctx context.Context) *OrchestratorResponse {
	intro := fmt.Sprintf("*The story begins for %s, a %s %s...*\n\nDescribe my surroundings and what I see before me.",
		o.session.State.Character.Name,
		o.session.State.Character.Race,
		o.session.State.Character.Class)

	return o.ProcessInput(ctx, intro)
}

func (o *Orchestrator) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"provider":     o.provider.Name(),
		"model":        o.session.Config.Model,
		"temperature":  o.session.Config.Temperature,
		"character":    o.session.State.Character.Summary(),
		"location":     o.session.State.World.CurrentLocation.Name,
		"conversation": o.session.State.Conversation.Len(),
		"events":       o.session.State.EventLog.Len(),
	}
}

type StreamCallback func(chunk string)

func (o *Orchestrator) ProcessInputStreaming(ctx context.Context, input string, callback StreamCallback) *OrchestratorResponse {
	return o.ProcessInput(ctx, input)
}

func (o *Orchestrator) UpdateMemorySummary(ctx context.Context) error {
	if o.session.State.Conversation.Len() < 10 {
		return nil
	}

	summaryPrompt := "Please provide a brief summary of the story so far, focusing on key events, decisions, and character developments. Keep it under 500 words."

	messages := []providers.Message{
		{
			Role:    providers.RoleSystem,
			Content: "You are a helpful assistant that summarizes RPG adventure stories.",
		},
	}

	for _, msg := range o.session.State.Conversation.Messages {
		role := providers.RoleUser
		if msg.Role == domain.RoleAssistant {
			role = providers.RoleAssistant
		}
		messages = append(messages, providers.Message{
			Role:    role,
			Content: msg.Content,
		})
	}

	messages = append(messages, providers.Message{
		Role:    providers.RoleUser,
		Content: summaryPrompt,
	})

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := o.provider.Chat(ctx, providers.ChatRequest{
		Messages:    messages,
		Model:       o.session.Config.Model,
		Temperature: 0.3,
		MaxTokens:   1000,
	})

	if err != nil {
		return err
	}

	o.session.State.World.MemorySummary = resp.Content
	o.session.MarkModified()

	return nil
}
