package takeTurn

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/nalanj/please/util/llm"
)

func TestNewAgent(t *testing.T) {
	provider := &mockProvider{}
	agent := New(provider, "test-model")
	if agent == nil {
		t.Fatal("expected non-nil agent")
	}
	if agent.model != "test-model" {
		t.Errorf("expected model 'test-model', got %q", agent.model)
	}
	if agent.provider == nil {
		t.Error("expected provider to be set")
	}
	if agent.maxTokens != defaultMaxTokens {
		t.Errorf("expected default maxTokens %d, got %d", defaultMaxTokens, agent.maxTokens)
	}
	if agent.contextLimit != 200000 {
		t.Errorf("expected default contextLimit 200000, got %d", agent.contextLimit)
	}
}

func TestAgentWithOptions(t *testing.T) {
	provider := &mockProvider{}
	agent := New(provider, "test-model",
		WithSystem("You are helpful"),
		WithMaxTokens(1000),
		ContextLimitOption(100000),
	)
	if agent.system != "You are helpful" {
		t.Errorf("expected system 'You are helpful', got %q", agent.system)
	}
	if agent.maxTokens != 1000 {
		t.Errorf("expected maxTokens 1000, got %d", agent.maxTokens)
	}
	if agent.contextLimit != 100000 {
		t.Errorf("expected contextLimit 100000, got %d", agent.contextLimit)
	}
}

func TestAgentWithTools(t *testing.T) {
	provider := &mockProvider{}
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			return "result", nil
		},
	}
	agent := New(provider, "test-model", WithTools(tool))
	if len(agent.tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(agent.tools))
	}
	if agent.tools[0].Name != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got %q", agent.tools[0].Name)
	}
	if agent.toolIndex["test_tool"] == nil {
		t.Error("expected tool to be in index")
	}
}

func TestAgentWithMessages(t *testing.T) {
	provider := &mockProvider{}
	messages := []llm.Message{
		llm.TextMessage(llm.RoleUser, "hello"),
		llm.TextMessage(llm.RoleAssistant, "hi there"),
	}
	agent := New(provider, "test-model", WithMessages(messages...))
	if len(agent.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(agent.messages))
	}
	if agent.messages[0].Content[0].Text != "hello" {
		t.Errorf("expected first message 'hello', got %q", agent.messages[0].Content[0].Text)
	}
}

func TestAgentMessages(t *testing.T) {
	provider := &mockProvider{}
	messages := []llm.Message{llm.TextMessage(llm.RoleUser, "test")}
	agent := New(provider, "test", WithMessages(messages...))

	snapshot := agent.Messages()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 message, got %d", len(snapshot))
	}

	// Verify it's a copy of the slice (different underlying array)
	// by appending to snapshot and checking agent.messages is unchanged
	if len(append(snapshot, llm.TextMessage(llm.RoleUser, "added"))) != 2 {
		t.Error("append should work")
	}
	if len(agent.messages) != 1 {
		t.Error("Messages() should return a copy of the slice")
	}
}

func TestAgentReset(t *testing.T) {
	provider := &mockProvider{}
	agent := New(provider, "test", WithMessages(llm.TextMessage(llm.RoleUser, "test")))
	if len(agent.messages) != 1 {
		t.Fatal("expected 1 message before reset")
	}

	agent.Reset()
	if len(agent.messages) != 0 {
		t.Errorf("expected 0 messages after reset, got %d", len(agent.messages))
	}
}

func TestAgentLlmTools(t *testing.T) {
	provider := &mockProvider{}
	tool := Tool{
		Name:        "my_tool",
		Description: "Does something",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}
	agent := New(provider, "test", WithTools(tool))

	llmTools := agent.llmTools()
	if len(llmTools) != 1 {
		t.Fatalf("expected 1 llm tool, got %d", len(llmTools))
	}
	if llmTools[0].Name != "my_tool" {
		t.Errorf("expected name 'my_tool', got %q", llmTools[0].Name)
	}
	if llmTools[0].Description != "Does something" {
		t.Errorf("expected description 'Does something', got %q", llmTools[0].Description)
	}
}

func TestAgentLlmToolsEmpty(t *testing.T) {
	provider := &mockProvider{}
	agent := New(provider, "test")

	llmTools := agent.llmTools()
	if llmTools != nil {
		t.Errorf("expected nil tools, got %v", llmTools)
	}
}

func TestAgentExecuteTool(t *testing.T) {
	provider := &mockProvider{}
	toolCalled := false
	tool := Tool{
		Name:        "test_tool",
		Description: "Test",
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			toolCalled = true
			return "tool result", nil
		},
	}
	agent := New(provider, "test", WithTools(tool))

	block := &llm.ContentBlock{
		Type:         llm.ContentTypeToolUse,
		ToolUseID:    "id123",
		ToolUseName:  "test_tool",
		ToolUseInput: json.RawMessage(`{}`),
	}

	result, isError := agent.executeTool(context.Background(), block)
	if !toolCalled {
		t.Error("expected tool handler to be called")
	}
	if isError {
		t.Error("expected no error")
	}
	if result != "tool result" {
		t.Errorf("expected 'tool result', got %q", result)
	}
}

func TestAgentExecuteToolUnknown(t *testing.T) {
	provider := &mockProvider{}
	agent := New(provider, "test")

	block := &llm.ContentBlock{
		Type:        llm.ContentTypeToolUse,
		ToolUseID:   "id123",
		ToolUseName: "unknown_tool",
	}

	result, isError := agent.executeTool(context.Background(), block)
	if !isError {
		t.Error("expected error for unknown tool")
	}
	if result == "" {
		t.Error("expected error message")
	}
}

func TestAgentExecuteToolError(t *testing.T) {
	provider := &mockProvider{}
	expectedErr := errors.New("tool failed")
	tool := Tool{
		Name:        "failing_tool",
		Description: "Fails",
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			return "", expectedErr
		},
	}
	agent := New(provider, "test", WithTools(tool))

	block := &llm.ContentBlock{
		Type:        llm.ContentTypeToolUse,
		ToolUseID:   "id123",
		ToolUseName: "failing_tool",
	}

	result, isError := agent.executeTool(context.Background(), block)
	if !isError {
		t.Error("expected error")
	}
	if result != "tool failed" {
		t.Errorf("expected 'tool failed', got %q", result)
	}
}

func TestAgentExecuteToolEmptyInput(t *testing.T) {
	provider := &mockProvider{}
	var receivedInput json.RawMessage
	tool := Tool{
		Name:        "test_tool",
		Description: "Test",
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			receivedInput = input
			return "ok", nil
		},
	}
	agent := New(provider, "test", WithTools(tool))

	block := &llm.ContentBlock{
		Type:         llm.ContentTypeToolUse,
		ToolUseID:    "id123",
		ToolUseName:  "test_tool",
		ToolUseInput: nil,
	}

	agent.executeTool(context.Background(), block)
	if receivedInput == nil {
		t.Error("expected non-nil input")
	}
	if string(receivedInput) != "{}" {
		t.Errorf("expected empty object '{}', got %q", string(receivedInput))
	}
}

func TestEstimateTokens(t *testing.T) {
	msg := llm.TextMessage(llm.RoleUser, "hello world")
	tokens := estimateTokens(msg)
	if tokens <= 0 {
		t.Error("expected positive token estimate")
	}
}

func TestTotalTokens(t *testing.T) {
	messages := []llm.Message{
		llm.TextMessage(llm.RoleUser, "hello"),
		llm.TextMessage(llm.RoleAssistant, "hi there"),
	}
	total := totalTokens(messages)
	if total <= 0 {
		t.Error("expected positive token count")
	}
}

func TestFindLastUserMessage(t *testing.T) {
	messages := []llm.Message{
		llm.TextMessage(llm.RoleUser, "first"),
		llm.TextMessage(llm.RoleAssistant, "response"),
		llm.TextMessage(llm.RoleUser, "last"),
	}
	idx := findLastUserMessage(messages)
	if idx != 2 {
		t.Errorf("expected index 2, got %d", idx)
	}
}

func TestFindLastUserMessageNone(t *testing.T) {
	messages := []llm.Message{
		llm.TextMessage(llm.RoleAssistant, "no user messages"),
	}
	idx := findLastUserMessage(messages)
	if idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

func TestFindLastUserMessageEmpty(t *testing.T) {
	messages := []llm.Message{}
	idx := findLastUserMessage(messages)
	if idx != -1 {
		t.Errorf("expected -1 for empty, got %d", idx)
	}
}

// Mock provider for testing

type mockProvider struct{}

func (m *mockProvider) Send(ctx context.Context, req llm.Request) (*llm.Response, error) {
	return &llm.Response{
		ID:    "msg_test",
		Model: req.Model,
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: []llm.ContentBlock{{Type: llm.ContentTypeText, Text: "test response"}},
		},
		StopReason: llm.StopReasonEndTurn,
	}, nil
}

func (m *mockProvider) Stream(ctx context.Context, req llm.Request) (*llm.Stream, error) {
	return llm.NewStream(func() (llm.StreamEvent, bool, error) {
		return llm.StreamEvent{
			Type: llm.StreamEventTypeDone,
			Response: &llm.Response{
				ID:    "msg_test",
				Model: req.Model,
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: []llm.ContentBlock{{Type: llm.ContentTypeText, Text: "test response"}},
				},
				StopReason: llm.StopReasonEndTurn,
			},
		}, false, nil
	}, nil), nil
}