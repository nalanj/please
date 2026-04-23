package llm

import (
	"context"
	"errors"
	"testing"
)

func TestStreamNextWhenDone(t *testing.T) {
	s := &Stream{done: true}
	if s.Next() {
		t.Error("expected false when stream is done")
	}
}

func TestStreamNextWhenError(t *testing.T) {
	s := &Stream{err: errors.New("test error")}
	if s.Next() {
		t.Error("expected false when stream has error")
	}
	if s.Err().Error() != "test error" {
		t.Errorf("expected 'test error', got %v", s.Err())
	}
}

func TestStreamEvent(t *testing.T) {
	s := &Stream{
		event: StreamEvent{
			Type: StreamEventTypeText,
			Text: "hello",
		},
	}
	evt := s.Event()
	if evt.Type != StreamEventTypeText {
		t.Errorf("expected text type, got %v", evt.Type)
	}
	if evt.Text != "hello" {
		t.Errorf("expected 'hello', got %q", evt.Text)
	}
}

func TestStreamErr(t *testing.T) {
	s := &Stream{err: errors.New("stream error")}
	if s.Err().Error() != "stream error" {
		t.Errorf("expected 'stream error', got %v", s.Err())
	}

	s = &Stream{}
	if s.Err() != nil {
		t.Errorf("expected nil error, got %v", s.Err())
	}
}

func TestStreamCloseNilFn(t *testing.T) {
	s := &Stream{closeFn: nil}
	if err := s.Close(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if s.done != true {
		t.Error("expected done to be true after close")
	}
}

func TestStreamCloseWithFn(t *testing.T) {
	called := false
	s := &Stream{
		closeFn: func() error {
			called = true
			return nil
		},
	}
	if err := s.Close(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !called {
		t.Error("expected closeFn to be called")
	}
}

func TestStreamCloseWithError(t *testing.T) {
	s := &Stream{
		closeFn: func() error {
			return errors.New("close error")
		},
	}
	if err := s.Close(); err == nil {
		t.Error("expected error from close")
	}
}

func TestStreamCloseMultiple(t *testing.T) {
	callCount := 0
	s := &Stream{
		closeFn: func() error {
			callCount++
			return nil
		},
	}
	if err := s.Close(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("unexpected error on second close: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected closeFn called once, got %d", callCount)
	}
}

func TestNewStream(t *testing.T) {
	calls := 0
	s := NewStream(
		func() (StreamEvent, bool, error) {
			calls++
			if calls == 1 {
				return StreamEvent{Type: StreamEventTypeText, Text: "hi"}, true, nil
			}
			return StreamEvent{}, false, nil
		},
		nil,
	)
	if !s.Next() {
		t.Fatal("expected first next to return true")
	}
	if s.Event().Text != "hi" {
		t.Errorf("expected 'hi', got %q", s.Event().Text)
	}
	if s.Next() {
		t.Error("expected second next to return false")
	}
}

func TestNewStreamWithError(t *testing.T) {
	s := NewStream(
		func() (StreamEvent, bool, error) {
			return StreamEvent{}, false, errors.New("test error")
		},
		nil,
	)
	if s.Next() {
		t.Error("expected next to return false")
	}
	if s.Err().Error() != "test error" {
		t.Errorf("expected 'test error', got %v", s.Err())
	}
}

func TestStreamEventTypes(t *testing.T) {
	tests := []struct {
		et    StreamEventType
		value string
	}{
		{StreamEventTypeText, "text"},
		{StreamEventTypeThinking, "thinking"},
		{StreamEventTypeToolUse, "tool_use"},
		{StreamEventTypeDone, "done"},
	}
	for _, tt := range tests {
		if string(tt.et) != tt.value {
			t.Errorf("expected %q, got %q", tt.value, string(tt.et))
		}
	}
}

func TestStreamWithContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := NewStream(
		func() (StreamEvent, bool, error) {
			return StreamEvent{}, false, ctx.Err()
		},
		nil,
	)
	if s.Next() {
		t.Error("expected next to return false for cancelled context")
	}
}

func TestStreamMultipleNextAfterDone(t *testing.T) {
	s := NewStream(
		func() (StreamEvent, bool, error) {
			return StreamEvent{}, false, nil
		},
		nil,
	)
	if s.Next() {
		t.Error("expected first next to return false")
	}
	// Calling Next multiple times after done should be safe
	if s.Next() {
		t.Error("expected second next to return false")
	}
	if s.Next() {
		t.Error("expected third next to return false")
	}
}

func TestStreamEventContent(t *testing.T) {
	evt := StreamEvent{
		Type:     StreamEventTypeThinking,
		Thinking: "thinking text",
	}
	if evt.Thinking != "thinking text" {
		t.Errorf("expected 'thinking text', got %q", evt.Thinking)
	}

	evt = StreamEvent{
		Type: StreamEventTypeToolUse,
		ToolUse: &ContentBlock{
			Type:        ContentTypeToolUse,
			ToolUseName: "test_tool",
		},
	}
	if evt.ToolUse.ToolUseName != "test_tool" {
		t.Errorf("expected 'test_tool', got %q", evt.ToolUse.ToolUseName)
	}
}

func TestStreamResponse(t *testing.T) {
	resp := &Response{
		ID:    "msg_1",
		Model: "test",
	}
	evt := StreamEvent{
		Type:     StreamEventTypeDone,
		Response: resp,
	}
	if evt.Response.ID != "msg_1" {
		t.Errorf("expected id 'msg_1', got %q", evt.Response.ID)
	}
}

type mockProvider struct {
	sendCalled  int
	streamCalls int
}

func (m *mockProvider) Send(ctx context.Context, req Request) (*Response, error) {
	m.sendCalled++
	return &Response{Model: req.Model}, nil
}

func (m *mockProvider) Stream(ctx context.Context, req Request) (*Stream, error) {
	m.streamCalls++
	return NewStream(func() (StreamEvent, bool, error) {
		return StreamEvent{Type: StreamEventTypeDone, Response: &Response{Model: req.Model}}, false, nil
	}, nil), nil
}

func TestProviderInterface(t *testing.T) {
	var p Provider = &mockProvider{}
	// Just verify the interface is satisfied
	_ = p
}