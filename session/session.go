package session

import (
	"encoding/json"
	"time"
)

const FormatVersion = 1

// Session is a recorded conversation with replayable output.
type Session struct {
	Version int    `json:"v"`
	Turns   []Turn `json:"turns"`
}

// Turn is a single user turn and the events it produced.
type Turn struct {
	Version   int         `json:"v"`
	Turn      int         `json:"turn"`
	Timestamp string      `json:"timestamp"`
	Input     string      `json:"input"`
	Metadata  Metadata    `json:"metadata"`
	Events    []Event     `json:"events"`
}

// Metadata contains context for a turn that may change between turns.
type Metadata struct {
	Model    string    `json:"model"`
	Provider string    `json:"provider"`
	System   string    `json:"system"`
	Tools    []ToolDef `json:"tools"`
}

// ToolDef describes a tool available during a turn.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Event is a single output event from the LLM or tool execution.
type Event struct {
	Type    string `json:"type"`  // text, thinking, tool_call, tool_result
	Chunk   string `json:"chunk,omitempty"`
	Size    int    `json:"size"`
	Ms      int    `json:"ms"`
	Name    string `json:"name,omitempty"`
	ID      string `json:"id,omitempty"`
	Input   string `json:"input,omitempty"`
	Content string `json:"content,omitempty"`
	Error   bool   `json:"error,omitempty"`
}

// NewTurnPtr creates a new turn pointer with the given input and metadata.
func NewTurnPtr(turnNum int, input string, md Metadata) *Turn {
	return &Turn{
		Version:   FormatVersion,
		Turn:      turnNum,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Input:     input,
		Metadata:  md,
		Events:    []Event{},
	}
}

// AddText adds a text delta event.
func (t *Turn) AddText(chunk string, ms int) {
	t.Events = append(t.Events, Event{
		Type:  "text",
		Chunk: chunk,
		Size:  len(chunk),
		Ms:    ms,
	})
}

// AddThinking adds a thinking delta event.
func (t *Turn) AddThinking(chunk string, ms int) {
	t.Events = append(t.Events, Event{
		Type:  "thinking",
		Chunk: chunk,
		Size:  len(chunk),
		Ms:    ms,
	})
}

// AddToolCall adds a tool call event.
func (t *Turn) AddToolCall(name, id, input string, ms int) {
	t.Events = append(t.Events, Event{
		Type:  "tool_call",
		Name:  name,
		ID:    id,
		Input: input,
		Ms:    ms,
	})
}

// AddToolResult adds a tool result event.
func (t *Turn) AddToolResult(id, content string, size int, isError bool, ms int) {
	t.Events = append(t.Events, Event{
		Type:    "tool_result",
		ID:      id,
		Content: content,
		Size:    size,
		Error:   isError,
		Ms:      ms,
	})
}
