package domain

import (
	"time"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

type Message struct {
	ID        string    `json:"id"`
	Role      Role      `json:"role"`
	Content   string    `json:"content"`
	Name      string    `json:"name,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string   `json:"tool_call_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Conversation struct {
	Messages []Message `json:"messages"`
	MaxSize  int       `json:"max_size"`
}

func NewConversation(maxSize int) *Conversation {
	if maxSize <= 0 {
		maxSize = 50
	}
	return &Conversation{
		Messages: []Message{},
		MaxSize:  maxSize,
	}
}

func (c *Conversation) Add(msg Message) {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	c.Messages = append(c.Messages, msg)

	if len(c.Messages) > c.MaxSize {
		c.Messages = c.Messages[len(c.Messages)-c.MaxSize:]
	}
}

func (c *Conversation) AddUserMessage(content string) {
	c.Add(Message{
		Role:      RoleUser,
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (c *Conversation) AddAssistantMessage(content string) {
	c.Add(Message{
		Role:      RoleAssistant,
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (c *Conversation) AddSystemMessage(content string) {
	c.Add(Message{
		Role:      RoleSystem,
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (c *Conversation) GetLast(n int) []Message {
	if n <= 0 || n > len(c.Messages) {
		return c.Messages
	}
	return c.Messages[len(c.Messages)-n:]
}

func (c *Conversation) Clear() {
	c.Messages = []Message{}
}

func (c *Conversation) Len() int {
	return len(c.Messages)
}
