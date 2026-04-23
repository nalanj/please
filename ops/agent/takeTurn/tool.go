package takeTurn

import (
	"context"
	"encoding/json"
)

// Tool is an LLM-callable function. Name, Description, and InputSchema are
// sent to the model as part of every request; Handler is called locally
// whenever the model invokes the tool.
type Tool struct {
	// Name is the identifier the model uses to call this tool.
	Name string

	// Description explains what the tool does. The richer the description,
	// the better the model will use it.
	Description string

	// InputSchema is a JSON Schema object that describes the tool's parameters.
	// Example:
	//   {"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}
	InputSchema json.RawMessage

	// Handler is called when the model invokes this tool.
	// input is the raw JSON object the model supplied.
	// Returning an error sends the error message back to the model as a
	// tool_result with is_error=true, letting it respond gracefully.
	Handler func(ctx context.Context, input json.RawMessage) (string, error)
}