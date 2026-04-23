package llm

import "encoding/json"

// StopReason describes why the model stopped generating.
type StopReason string

const (
	// StopReasonEndTurn means the model reached a natural stopping point.
	StopReasonEndTurn StopReason = "end_turn"

	// StopReasonMaxTokens means the model hit the MaxTokens limit.
	StopReasonMaxTokens StopReason = "max_tokens"

	// StopReasonToolUse means the model invoked one or more tools.
	StopReasonToolUse StopReason = "tool_use"

	// StopReasonStopSequence means one of the custom stop sequences was matched.
	StopReasonStopSequence StopReason = "stop_sequence"
)

// ThinkingConfig configures extended thinking for models that support it
// (e.g., Claude with extended thinking).
type ThinkingConfig struct {
	// Mode specifies the thinking mode. Zero value means disabled.
	Mode ThinkingMode

	// BudgetTokens is the maximum number of tokens to use for thinking.
	// Required when Mode is ThinkingModeEnabled. Must be >= 1024 and < MaxTokens.
	BudgetTokens int
}

// ThinkingMode specifies how to use extended thinking.
type ThinkingMode string

const (
	// ThinkingModeEnabled turns on extended thinking with a token budget.
	ThinkingModeEnabled ThinkingMode = "enabled"

	// ThinkingModeDisabled turns off extended thinking.
	ThinkingModeDisabled ThinkingMode = "disabled"

	// ThinkingModeAdaptive lets the model decide when to use thinking.
	ThinkingModeAdaptive ThinkingMode = "adaptive"
)

// Tool describes a function the model is allowed to call.
type Tool struct {
	// Name is the identifier used in tool_use blocks.
	Name string

	// Description tells the model what the tool does. The more detail, the
	// better the model will use it.
	Description string

	// InputSchema is a JSON Schema object describing the tool's parameters.
	// Example:
	//   {"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}
	InputSchema json.RawMessage
}

// Request is the provider-agnostic input to a chat completion call.
type Request struct {
	// Model is the model identifier string (e.g. "claude-opus-4-5").
	Model string

	// System is the system prompt.
	System string

	// Messages is the conversation history.
	Messages []Message

	// MaxTokens is the maximum number of tokens to generate. Required.
	MaxTokens int

	// Temperature controls randomness (0.0–1.0). Nil means use the API default.
	Temperature *float64

	// Tools is the set of tools the model may call.
	Tools []Tool

	// StopSequences lists custom strings that will halt generation.
	StopSequences []string

	// Thinking configures extended thinking for models that support it.
	// Not all providers support this option.
	Thinking ThinkingConfig
}

// Response is the provider-agnostic output from a chat completion call.
type Response struct {
	// ID is the provider-assigned message identifier.
	ID string

	// Model is the model that produced the response.
	Model string

	// Message contains the assistant's generated content.
	Message Message

	// StopReason explains why generation ended.
	StopReason StopReason

	// Usage contains token counts for the request.
	Usage Usage
}

// Usage records token consumption for a request.
type Usage struct {
	InputTokens  int
	OutputTokens int
}