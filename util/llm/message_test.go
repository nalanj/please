package llm

import (
	"encoding/json"
	"testing"
)

func TestTextMessage(t *testing.T) {
	msg := TextMessage(RoleUser, "hello")
	if msg.Role != RoleUser {
		t.Errorf("expected role user, got %v", msg.Role)
	}
	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(msg.Content))
	}
	if msg.Content[0].Type != ContentTypeText {
		t.Errorf("expected text content type, got %v", msg.Content[0].Type)
	}
	if msg.Content[0].Text != "hello" {
		t.Errorf("expected 'hello', got %q", msg.Content[0].Text)
	}
}

func TestTextMessageAssistant(t *testing.T) {
	msg := TextMessage(RoleAssistant, "world")
	if msg.Role != RoleAssistant {
		t.Errorf("expected role assistant, got %v", msg.Role)
	}
	if msg.Content[0].Text != "world" {
		t.Errorf("expected 'world', got %q", msg.Content[0].Text)
	}
}

func TestToolResultMessage(t *testing.T) {
	result := ContentBlock{
		Type:              ContentTypeToolResult,
		ToolResultID:      "tool_123",
		ToolResultContent: "output text",
		ToolResultError:   false,
	}
	msg := ToolResultMessage(result)
	if msg.Role != RoleUser {
		t.Errorf("expected role user for tool result, got %v", msg.Role)
	}
	if len(msg.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(msg.Content))
	}
	if msg.Content[0].ToolResultID != "tool_123" {
		t.Errorf("expected tool result id 'tool_123', got %q", msg.Content[0].ToolResultID)
	}
}

func TestToolResultMessageMultiple(t *testing.T) {
	result1 := ContentBlock{Type: ContentTypeToolResult, ToolResultID: "r1"}
	result2 := ContentBlock{Type: ContentTypeToolResult, ToolResultID: "r2"}
	msg := ToolResultMessage(result1, result2)
	if len(msg.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(msg.Content))
	}
	if msg.Content[0].ToolResultID != "r1" {
		t.Errorf("expected first result id 'r1', got %q", msg.Content[0].ToolResultID)
	}
	if msg.Content[1].ToolResultID != "r2" {
		t.Errorf("expected second result id 'r2', got %q", msg.Content[1].ToolResultID)
	}
}

func TestContentBlockMarshalText(t *testing.T) {
	block := ContentBlock{
		Type: ContentTypeText,
		Text: "hello world",
	}
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded ContentBlock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Text != "hello world" {
		t.Errorf("expected 'hello world', got %q", decoded.Text)
	}
}

func TestContentBlockMarshalToolUseEmptyInput(t *testing.T) {
	block := ContentBlock{
		Type:         ContentTypeToolUse,
		ToolUseID:    "abc123",
		ToolUseName:  "mytool",
		ToolUseInput: nil,
	}
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	// Should contain empty object for tool_use_input
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	input, ok := decoded["tool_use_input"]
	if !ok {
		t.Error("expected tool_use_input field")
	}
	inputMap, ok := input.(map[string]any)
	if !ok {
		t.Errorf("expected tool_use_input to be object, got %T", input)
	}
	if len(inputMap) != 0 {
		t.Errorf("expected empty object, got %v", inputMap)
	}
}

func TestContentBlockMarshalToolUseWithInput(t *testing.T) {
	block := ContentBlock{
		Type:         ContentTypeToolUse,
		ToolUseID:    "abc123",
		ToolUseName:  "mytool",
		ToolUseInput: json.RawMessage(`{"arg":"value"}`),
	}
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	input, ok := decoded["tool_use_input"]
	if !ok {
		t.Error("expected tool_use_input field")
	}
	inputMap, ok := input.(map[string]any)
	if !ok {
		t.Errorf("expected tool_use_input to be object, got %T", input)
	}
	if inputMap["arg"] != "value" {
		t.Errorf("expected arg=value, got %v", inputMap["arg"])
	}
}

func TestContentBlockMarshalToolResult(t *testing.T) {
	block := ContentBlock{
		Type:              ContentTypeToolResult,
		ToolResultID:      "res123",
		ToolResultContent: "result data",
		ToolResultError:   true,
	}
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded ContentBlock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.ToolResultError != true {
		t.Errorf("expected error=true, got %v", decoded.ToolResultError)
	}
	if decoded.ToolResultContent != "result data" {
		t.Errorf("expected 'result data', got %q", decoded.ToolResultContent)
	}
}

func TestContentBlockMarshalThinking(t *testing.T) {
	block := ContentBlock{
		Type:     ContentTypeThinking,
		Thinking: "reasoning text",
	}
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded ContentBlock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Thinking != "reasoning text" {
		t.Errorf("expected 'reasoning text', got %q", decoded.Thinking)
	}
}

func TestMessageMarshal(t *testing.T) {
	msg := Message{
		Role: RoleUser,
		Content: []ContentBlock{
			{Type: ContentTypeText, Text: "hello"},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Role != RoleUser {
		t.Errorf("expected role user, got %v", decoded.Role)
	}
	if decoded.Content[0].Text != "hello" {
		t.Errorf("expected 'hello', got %q", decoded.Content[0].Text)
	}
}

func TestContentTypes(t *testing.T) {
	tests := []struct {
		ct    ContentType
		value string
	}{
		{ContentTypeText, "text"},
		{ContentTypeThinking, "thinking"},
		{ContentTypeToolUse, "tool_use"},
		{ContentTypeToolResult, "tool_result"},
	}
	for _, tt := range tests {
		if string(tt.ct) != tt.value {
			t.Errorf("expected %q, got %q", tt.value, string(tt.ct))
		}
	}
}

func TestRoles(t *testing.T) {
	if RoleUser != "user" {
		t.Errorf("expected 'user', got %q", RoleUser)
	}
	if RoleAssistant != "assistant" {
		t.Errorf("expected 'assistant', got %q", RoleAssistant)
	}
}