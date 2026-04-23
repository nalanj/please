// Package llm provides a provider-agnostic interface for interacting with
// large language model APIs.
package llm

import "encoding/json"

// Role represents the conversational role of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ContentType identifies the kind of content in a [ContentBlock].
type ContentType string

const (
	// ContentTypeText is plain text produced by the model or provided by the user.
	ContentTypeText ContentType = "text"

	// ContentTypeThinking is extended thinking output from the model (e.g., Claude's
	// internal reasoning process).
	ContentTypeThinking ContentType = "thinking"

	// ContentTypeToolUse is a tool invocation requested by the model.
	ContentTypeToolUse ContentType = "tool_use"

	// ContentTypeToolResult is the result returned to the model after executing a
	// tool call.
	ContentTypeToolResult ContentType = "tool_result"
)

// ContentBlock is a single piece of content within a [Message]. Its fields are
// populated based on [ContentType]:
//
//   - ContentTypeText: Text is set.
//   - ContentTypeThinking: Thinking is set.
//   - ContentTypeToolUse: ToolUseID, ToolUseName, ToolUseInput are set.
//   - ContentTypeToolResult: ToolResultID, ToolResultContent, ToolResultError are set.
type ContentBlock struct {
	Type ContentType `json:"type"`

	// Text (ContentTypeText)
	Text string `json:"text,omitempty"`

	// Thinking (ContentTypeThinking) — extended reasoning output from models
	// that support it (e.g., Claude with extended thinking enabled).
	Thinking string `json:"thinking,omitempty"`

	// Tool use — model requesting a tool call (ContentTypeToolUse).
	ToolUseID    string          `json:"tool_use_id,omitempty"`
	ToolUseName  string          `json:"tool_use_name,omitempty"`
	ToolUseInput json.RawMessage `json:"tool_use_input,omitempty"` // JSON-encoded tool arguments

	// Tool result — returning a result to the model (ContentTypeToolResult).
	ToolResultID      string `json:"tool_result_id,omitempty"`
	ToolResultContent string `json:"tool_result_content,omitempty"` // plain-text result
	ToolResultError   bool   `json:"tool_result_error,omitempty"`   // true if the tool call produced an error
}

// MarshalJSON implements json.Marshaler to handle nil or empty ToolUseInput correctly.
func (c ContentBlock) MarshalJSON() ([]byte, error) {
	// Use an alias type to avoid infinite recursion.
	type alias ContentBlock

	// Only add empty tool_use_input for tool_use blocks.
	// tool_result and thinking blocks should not have tool_use_input.
	if len(c.ToolUseInput) == 0 && c.Type == ContentTypeToolUse {
		// Replace nil or empty with empty object to avoid marshaling errors.
		aliasWithEmpty := alias(c)
		aliasWithEmpty.ToolUseInput = json.RawMessage(`{}`)
		return json.Marshal(aliasWithEmpty)
	}
	return json.Marshal(alias(c))
}

// Message is a single turn in a conversation.
type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

// TextMessage is a convenience constructor for a single-text-block message.
func TextMessage(role Role, text string) Message {
	return Message{
		Role:    role,
		Content: []ContentBlock{{Type: ContentTypeText, Text: text}},
	}
}

// ToolResultMessage is a convenience constructor for a message that returns one
// or more tool results.
func ToolResultMessage(results ...ContentBlock) Message {
	return Message{Role: RoleUser, Content: results}
}