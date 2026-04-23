package llm

import (
	"encoding/json"
	"testing"
)

func TestToolSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: schema,
	}
	if tool.Name != "test_tool" {
		t.Errorf("expected name 'test_tool', got %q", tool.Name)
	}
	if tool.Description != "A test tool" {
		t.Errorf("expected description 'A test tool', got %q", tool.Description)
	}
	var parsed map[string]any
	if err := json.Unmarshal(tool.InputSchema, &parsed); err != nil {
		t.Fatalf("failed to parse input schema: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("expected type 'object', got %v", parsed["type"])
	}
}

func TestRequestDefaults(t *testing.T) {
	req := Request{
		Model:     "test-model",
		Messages:  []Message{TextMessage(RoleUser, "hi")},
		MaxTokens: 100,
	}
	if req.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", req.Model)
	}
	if req.MaxTokens != 100 {
		t.Errorf("expected max tokens 100, got %d", req.MaxTokens)
	}
	if req.Temperature != nil {
		t.Errorf("expected nil temperature, got %v", *req.Temperature)
	}
	if req.Tools != nil {
		t.Errorf("expected nil tools, got %v", req.Tools)
	}
}

func TestRequestWithTemperature(t *testing.T) {
	temp := 0.7
	req := Request{
		Model:       "test",
		MaxTokens:   100,
		Temperature: &temp,
	}
	if *req.Temperature != 0.7 {
		t.Errorf("expected 0.7, got %v", *req.Temperature)
	}
}

func TestRequestWithTools(t *testing.T) {
	req := Request{
		Model:     "test",
		MaxTokens: 100,
		Tools: []Tool{
			{Name: "tool1", Description: "desc1"},
			{Name: "tool2", Description: "desc2"},
		},
	}
	if len(req.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(req.Tools))
	}
	if req.Tools[0].Name != "tool1" {
		t.Errorf("expected first tool 'tool1', got %q", req.Tools[0].Name)
	}
}

func TestRequestWithStopSequences(t *testing.T) {
	req := Request{
		Model:         "test",
		MaxTokens:     100,
		StopSequences: []string{"STOP", "END"},
	}
	if len(req.StopSequences) != 2 {
		t.Fatalf("expected 2 stop sequences, got %d", len(req.StopSequences))
	}
}

func TestRequestWithThinkingEnabled(t *testing.T) {
	req := Request{
		Model:     "test",
		MaxTokens: 100,
		Thinking: ThinkingConfig{
			Mode:          ThinkingModeEnabled,
			BudgetTokens:  1024,
		},
	}
	if req.Thinking.Mode != ThinkingModeEnabled {
		t.Errorf("expected thinking mode enabled, got %v", req.Thinking.Mode)
	}
	if req.Thinking.BudgetTokens != 1024 {
		t.Errorf("expected budget tokens 1024, got %d", req.Thinking.BudgetTokens)
	}
}

func TestRequestWithThinkingDisabled(t *testing.T) {
	req := Request{
		Model:     "test",
		MaxTokens: 100,
		Thinking: ThinkingConfig{
			Mode: ThinkingModeDisabled,
		},
	}
	if req.Thinking.Mode != ThinkingModeDisabled {
		t.Errorf("expected thinking mode disabled, got %v", req.Thinking.Mode)
	}
}

func TestRequestMarshal(t *testing.T) {
	temp := 0.5
	req := Request{
		Model:       "test-model",
		System:      "You are helpful",
		MaxTokens:   200,
		Temperature: &temp,
		Tools: []Tool{
			{Name: "test"},
		},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", decoded.Model)
	}
	if decoded.System != "You are helpful" {
		t.Errorf("expected system 'You are helpful', got %q", decoded.System)
	}
	if *decoded.Temperature != 0.5 {
		t.Errorf("expected temperature 0.5, got %v", *decoded.Temperature)
	}
}

func TestResponse(t *testing.T) {
	resp := Response{
		ID:    "msg_123",
		Model: "test-model",
		Message: Message{
			Role:    RoleAssistant,
			Content: []ContentBlock{{Type: ContentTypeText, Text: "hello"}},
		},
		StopReason: StopReasonEndTurn,
		Usage: Usage{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}
	if resp.ID != "msg_123" {
		t.Errorf("expected id 'msg_123', got %q", resp.ID)
	}
	if resp.StopReason != StopReasonEndTurn {
		t.Errorf("expected stop reason end_turn, got %v", resp.StopReason)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("expected input tokens 10, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 5 {
		t.Errorf("expected output tokens 5, got %d", resp.Usage.OutputTokens)
	}
}

func TestStopReasons(t *testing.T) {
	tests := []struct {
		sr    StopReason
		value string
	}{
		{StopReasonEndTurn, "end_turn"},
		{StopReasonMaxTokens, "max_tokens"},
		{StopReasonToolUse, "tool_use"},
		{StopReasonStopSequence, "stop_sequence"},
	}
	for _, tt := range tests {
		if string(tt.sr) != tt.value {
			t.Errorf("expected %q, got %q", tt.value, string(tt.sr))
		}
	}
}

func TestThinkingModes(t *testing.T) {
	tests := []struct {
		mode  ThinkingMode
		value string
	}{
		{ThinkingModeEnabled, "enabled"},
		{ThinkingModeDisabled, "disabled"},
		{ThinkingModeAdaptive, "adaptive"},
	}
	for _, tt := range tests {
		if string(tt.mode) != tt.value {
			t.Errorf("expected %q, got %q", tt.value, string(tt.mode))
		}
	}
}