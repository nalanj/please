package anthropic

import (
	"context"
	"encoding/json"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/nalanj/please/util/llm"
)

// Client implements [llm.Provider] using the Anthropic Messages API.
// Use [NewMiniMaxProvider] to create an instance.
type Client struct {
	sdk sdk.Client
}

// newClient is the shared constructor used by all provider constructors.
func newClient(apiKey, baseURL string, opts ...Option) *Client {
	cfg := &config{baseURL: baseURL}
	for _, o := range opts {
		o(cfg)
	}

	sdkOpts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(cfg.baseURL),
	}
	if cfg.httpClient != nil {
		sdkOpts = append(sdkOpts, option.WithHTTPClient(cfg.httpClient))
	}

	sdkClient := sdk.NewClient(sdkOpts...)
	return &Client{sdk: sdkClient}
}

// Send sends a request and returns the complete response.
func (c *Client) Send(ctx context.Context, req llm.Request) (*llm.Response, error) {
	params, err := toParams(req)
	if err != nil {
		return nil, err
	}

	msg, err := c.sdk.Messages.New(ctx, params)
	if err != nil {
		return nil, err
	}

	return fromMessage(msg)
}

// Stream sends a request and returns a [llm.Stream] of incremental events.
//
// Events are emitted as follows:
//   - [llm.StreamEventTypeText] for each text delta.
//   - [llm.StreamEventTypeToolUse] when a complete tool call is assembled.
//   - [llm.StreamEventTypeDone] once with the fully assembled [llm.Response].
func (c *Client) Stream(ctx context.Context, req llm.Request) (*llm.Stream, error) {
	params, err := toParams(req)
	if err != nil {
		return nil, err
	}

	sdkStream := c.sdk.Messages.NewStreaming(ctx, params)

	// State accumulated across events.
	var (
		msgID      string
		msgModel   string
		stopReason llm.StopReason
		usage      llm.Usage
		blocks     = map[int]*streamBlockRecord{}
	)

	nextFn := func() (llm.StreamEvent, bool, error) {
		for sdkStream.Next() {
			evt := sdkStream.Current()

			switch evt.Type {

			case "message_start":
				msgID = evt.Message.ID
				msgModel = string(evt.Message.Model)
				usage.InputTokens = int(evt.Message.Usage.InputTokens)

			case "content_block_start":
				idx := int(evt.Index)
				br := &streamBlockRecord{blockType: evt.ContentBlock.Type}
				if evt.ContentBlock.Type == "tool_use" {
					br.id = evt.ContentBlock.ID
					br.name = evt.ContentBlock.Name
				}
				blocks[idx] = br

			case "content_block_delta":
				idx := int(evt.Index)
				br, ok := blocks[idx]
				if !ok {
					continue
				}
				switch evt.Delta.Type {
				case "text_delta":
					text := evt.Delta.Text
					br.text.WriteString(text)
					return llm.StreamEvent{
						Type: llm.StreamEventTypeText,
						Text: text,
					}, true, nil
				case "thinking_delta":
					thinking := evt.Delta.Thinking
					br.thinking.WriteString(thinking)
					return llm.StreamEvent{
						Type:     llm.StreamEventTypeThinking,
						Thinking: thinking,
					}, true, nil
				case "input_json_delta":
					br.input.WriteString(evt.Delta.PartialJSON)
				}

			case "content_block_stop":
				idx := int(evt.Index)
				br, ok := blocks[idx]
				if !ok {
					continue
				}
				switch br.blockType {
				case "tool_use":
					block := &llm.ContentBlock{
						Type:         llm.ContentTypeToolUse,
						ToolUseID:    br.id,
						ToolUseName:  br.name,
						ToolUseInput: json.RawMessage(br.input.String()),
					}
					return llm.StreamEvent{
						Type:    llm.StreamEventTypeToolUse,
						ToolUse: block,
					}, true, nil
				case "thinking":
					// Don't re-emit - thinking was already streamed via thinking_delta
					// Return empty event with true to continue iteration
					return llm.StreamEvent{}, true, nil
				}

			case "message_delta":
				stopReason = llm.StopReason(evt.Delta.StopReason)
				usage.OutputTokens = int(evt.Usage.OutputTokens)

			case "message_stop":
				content := buildFinalContent(blocks)
				return llm.StreamEvent{
					Type: llm.StreamEventTypeDone,
					Response: &llm.Response{
						ID:    msgID,
						Model: msgModel,
						Message: llm.Message{
							Role:    llm.RoleAssistant,
							Content: content,
						},
						StopReason: stopReason,
						Usage:      usage,
					},
				}, true, nil
			}
		}

		if err := sdkStream.Err(); err != nil {
			return llm.StreamEvent{}, false, err
		}
		return llm.StreamEvent{}, false, nil
	}

	return llm.NewStream(nextFn, func() error { return sdkStream.Close() }), nil
}