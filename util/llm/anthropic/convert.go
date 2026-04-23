package anthropic

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"github.com/nalanj/please/util/llm"
)

// toParams converts an [llm.Request] into the SDK's MessageNewParams.
func toParams(req llm.Request) (sdk.MessageNewParams, error) {
	params := sdk.MessageNewParams{
		Model:     sdk.Model(req.Model),
		MaxTokens: int64(req.MaxTokens),
	}

	// System prompt.
	if req.System != "" {
		params.System = []sdk.TextBlockParam{{Text: req.System}}
	}

	// Stop sequences.
	params.StopSequences = req.StopSequences

	// Temperature.
	if req.Temperature != nil {
		params.Temperature = param.NewOpt(*req.Temperature)
	}

	// Thinking configuration.
	if req.Thinking.Mode != "" {
		switch req.Thinking.Mode {
		case llm.ThinkingModeEnabled:
			params.Thinking = sdk.ThinkingConfigParamUnion{
				OfEnabled: &sdk.ThinkingConfigEnabledParam{
					BudgetTokens: int64(req.Thinking.BudgetTokens),
				},
			}
		case llm.ThinkingModeDisabled:
			params.Thinking = sdk.ThinkingConfigParamUnion{
				OfDisabled: &sdk.ThinkingConfigDisabledParam{},
			}
		case llm.ThinkingModeAdaptive:
			params.Thinking = sdk.ThinkingConfigParamUnion{
				OfAdaptive: &sdk.ThinkingConfigAdaptiveParam{},
			}
		}
	}

	// Messages.
	msgs, err := toMessageParams(req.Messages)
	if err != nil {
		return sdk.MessageNewParams{}, err
	}
	params.Messages = msgs

	// Tools.
	if len(req.Tools) > 0 {
		tools, err := toToolParams(req.Tools)
		if err != nil {
			return sdk.MessageNewParams{}, err
		}
		params.Tools = tools
	}

	return params, nil
}

// toMessageParams converts a slice of [llm.Message] to SDK MessageParam slice.
func toMessageParams(msgs []llm.Message) ([]sdk.MessageParam, error) {
	out := make([]sdk.MessageParam, 0, len(msgs))
	for _, m := range msgs {
		blocks, err := toContentBlockParams(m.Content)
		if err != nil {
			return nil, err
		}
		switch m.Role {
		case llm.RoleUser:
			out = append(out, sdk.NewUserMessage(blocks...))
		case llm.RoleAssistant:
			out = append(out, sdk.NewAssistantMessage(blocks...))
		default:
			return nil, fmt.Errorf("unsupported role: %q", m.Role)
		}
	}
	return out, nil
}

// toContentBlockParams converts a slice of [llm.ContentBlock] to SDK
// ContentBlockParamUnion slice.
func toContentBlockParams(blocks []llm.ContentBlock) ([]sdk.ContentBlockParamUnion, error) {
	out := make([]sdk.ContentBlockParamUnion, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case llm.ContentTypeText:
			out = append(out, sdk.NewTextBlock(b.Text))

		case llm.ContentTypeToolUse:
			// Decode the tool input JSON so the SDK can re-encode it correctly.
			var input any
			if len(b.ToolUseInput) > 0 {
				if err := json.Unmarshal(b.ToolUseInput, &input); err != nil {
					// If decoding fails, use empty object (tool may handle this as no args).
					input = map[string]any{}
				}
			} else {
				// Empty input means no arguments provided.
				input = map[string]any{}
			}
			out = append(out, sdk.ContentBlockParamUnion{
				OfToolUse: &sdk.ToolUseBlockParam{
					ID:    b.ToolUseID,
					Name:  b.ToolUseName,
					Input: input,
				},
			})

		case llm.ContentTypeToolResult:
			out = append(out, sdk.NewToolResultBlock(
				b.ToolResultID,
				b.ToolResultContent,
				b.ToolResultError,
			))

		case llm.ContentTypeThinking:
			// Skip thinking blocks — they're internal reasoning that should not be
			// sent back to the API in subsequent requests.

		default:
			return nil, fmt.Errorf("unsupported content block type: %q", b.Type)
		}
	}
	return out, nil
}

// toToolParams converts a slice of [llm.Tool] to SDK ToolUnionParam slice.
func toToolParams(tools []llm.Tool) ([]sdk.ToolUnionParam, error) {
	out := make([]sdk.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		schema, err := toToolInputSchema(t.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("building input schema for tool %q: %w", t.Name, err)
		}
		toolParam := sdk.ToolParam{
			Name:        t.Name,
			InputSchema: schema,
		}
		if t.Description != "" {
			toolParam.Description = param.NewOpt(t.Description)
		}
		out = append(out, sdk.ToolUnionParam{OfTool: &toolParam})
	}
	return out, nil
}

// toToolInputSchema converts a raw JSON Schema into the SDK's ToolInputSchemaParam.
// The schema should be a JSON object with at least a "properties" field.
func toToolInputSchema(schema json.RawMessage) (sdk.ToolInputSchemaParam, error) {
	if len(schema) == 0 {
		return sdk.ToolInputSchemaParam{}, nil
	}

	var m map[string]any
	if err := json.Unmarshal(schema, &m); err != nil {
		return sdk.ToolInputSchemaParam{}, fmt.Errorf("parsing JSON schema: %w", err)
	}

	result := sdk.ToolInputSchemaParam{}

	if props, ok := m["properties"]; ok {
		result.Properties = props
	}

	if raw, ok := m["required"]; ok {
		if items, ok := raw.([]any); ok {
			for _, item := range items {
				if s, ok := item.(string); ok {
					result.Required = append(result.Required, s)
				}
			}
		}
	}

	// Forward any extra JSON Schema keywords (description, $defs, etc.).
	skip := map[string]bool{"type": true, "properties": true, "required": true}
	extra := make(map[string]any)
	for k, v := range m {
		if !skip[k] {
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		result.ExtraFields = extra
	}

	return result, nil
}

// fromMessage converts an SDK Message to an [llm.Response].
func fromMessage(msg *sdk.Message) (*llm.Response, error) {
	content, err := fromContentBlocks(msg.Content)
	if err != nil {
		return nil, err
	}
	return &llm.Response{
		ID:    msg.ID,
		Model: string(msg.Model),
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: content,
		},
		StopReason: llm.StopReason(msg.StopReason),
		Usage: llm.Usage{
			InputTokens:  int(msg.Usage.InputTokens),
			OutputTokens: int(msg.Usage.OutputTokens),
		},
	}, nil
}

// fromContentBlocks converts SDK ContentBlockUnion slice to []llm.ContentBlock.
func fromContentBlocks(blocks []sdk.ContentBlockUnion) ([]llm.ContentBlock, error) {
	out := make([]llm.ContentBlock, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			tb := b.AsText()
			out = append(out, llm.ContentBlock{
				Type: llm.ContentTypeText,
				Text: tb.Text,
			})
		case "thinking":
			th := b.AsThinking()
			out = append(out, llm.ContentBlock{
				Type:     llm.ContentTypeThinking,
				Thinking: th.Thinking,
			})
		case "redacted_thinking":
			// Redacted thinking is thinking that was generated but then removed.
			// We skip it as it's not useful content.
		case "tool_use":
			tu := b.AsToolUse()
			out = append(out, llm.ContentBlock{
				Type:         llm.ContentTypeToolUse,
				ToolUseID:    tu.ID,
				ToolUseName:  tu.Name,
				ToolUseInput: tu.Input,
			})
		// Silently skip block types we don't model (images, etc.).
		}
	}
	return out, nil
}

// streamBlockRecord tracks the accumulated state of one streaming content block.
type streamBlockRecord struct {
	blockType string
	text      strings.Builder
	thinking  strings.Builder
	id        string
	name      string
	input     strings.Builder
}

// buildFinalContent sorts accumulated block records by index and converts them
// to []llm.ContentBlock.
func buildFinalContent(blocks map[int]*streamBlockRecord) []llm.ContentBlock {
	indices := make([]int, 0, len(blocks))
	for idx := range blocks {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	content := make([]llm.ContentBlock, 0, len(indices))
	for _, idx := range indices {
		br := blocks[idx]
		switch br.blockType {
		case "text":
			content = append(content, llm.ContentBlock{
				Type: llm.ContentTypeText,
				Text: br.text.String(),
			})
		case "thinking":
			content = append(content, llm.ContentBlock{
				Type:     llm.ContentTypeThinking,
				Thinking: br.thinking.String(),
			})
		case "tool_use":
			content = append(content, llm.ContentBlock{
				Type:         llm.ContentTypeToolUse,
				ToolUseID:    br.id,
				ToolUseName:  br.name,
				ToolUseInput: json.RawMessage(br.input.String()),
			})
		}
	}
	return content
}