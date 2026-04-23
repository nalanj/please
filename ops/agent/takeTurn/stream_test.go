package takeTurn

import (
	"context"
	"encoding/json"
	"testing"
)

func TestEventTypes(t *testing.T) {
	tests := []struct {
		et    EventType
		value string
	}{
		{EventTypeText, "text"},
		{EventTypeToolCall, "tool_call"},
		{EventTypeToolResult, "tool_result"},
		{EventTypeDone, "done"},
	}
	for _, tt := range tests {
		if string(tt.et) != tt.value {
			t.Errorf("expected %q, got %q", tt.value, string(tt.et))
		}
	}
}

func TestStreamNew(t *testing.T) {
	ch := make(chan chanItem, 1)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newStream(ch, cancel)
	if s == nil {
		t.Fatal("expected non-nil stream")
	}
	if s.current.Type != "" {
		t.Errorf("expected empty current event, got %v", s.current.Type)
	}
	if s.err != nil {
		t.Errorf("expected nil error, got %v", s.err)
	}
	if s.done {
		t.Error("expected done to be false")
	}
}

func TestStreamNextEmpty(t *testing.T) {
	ch := make(chan chanItem)
	close(ch) // Close empty channel so receive returns immediately
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newStream(ch, cancel)
	// Channel is empty and closed, so Next should return false
	if s.Next() {
		t.Error("expected false for empty closed channel")
	}
}

func TestStreamNextWithEvent(t *testing.T) {
	ch := make(chan chanItem, 1)
	ch <- chanItem{event: Event{Type: EventTypeText, Text: "hello"}}
	close(ch)

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newStream(ch, cancel)
	if !s.Next() {
		t.Fatal("expected true from Next")
	}
	evt := s.Event()
	if evt.Type != EventTypeText {
		t.Errorf("expected text type, got %v", evt.Type)
	}
	if evt.Text != "hello" {
		t.Errorf("expected 'hello', got %q", evt.Text)
	}
}

func TestStreamNextWithError(t *testing.T) {
	ch := make(chan chanItem, 1)
	ch <- chanItem{err: errTest}
	close(ch)

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newStream(ch, cancel)
	if s.Next() {
		t.Error("expected false for error")
	}
	if s.Err() == nil {
		t.Error("expected error to be set")
	}
}

func TestStreamClose(t *testing.T) {
	ch := make(chan chanItem)
	_, cancel := context.WithCancel(context.Background())

	s := newStream(ch, cancel)
	s.Close()

	if !s.done {
		t.Error("expected done to be true after close")
	}
	// Channel should be drained
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed/drained")
		}
	default:
		// OK - channel is drained
	}
}

func TestStreamMultipleNext(t *testing.T) {
	ch := make(chan chanItem, 2)
	ch <- chanItem{event: Event{Type: EventTypeText, Text: "first"}}
	ch <- chanItem{event: Event{Type: EventTypeText, Text: "second"}}
	close(ch)

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newStream(ch, cancel)

	if !s.Next() {
		t.Fatal("expected first Next to return true")
	}
	if s.Event().Text != "first" {
		t.Errorf("expected 'first', got %q", s.Event().Text)
	}

	if !s.Next() {
		t.Fatal("expected second Next to return true")
	}
	if s.Event().Text != "second" {
		t.Errorf("expected 'second', got %q", s.Event().Text)
	}

	if s.Next() {
		t.Error("expected third Next to return false")
	}
}

var errTest = &testError{msg: "test error"}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }

func TestToolStruct(t *testing.T) {
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Handler: func(ctx context.Context, input json.RawMessage) (string, error) {
			return "result", nil
		},
	}
	if tool.Name != "test_tool" {
		t.Errorf("expected name 'test_tool', got %q", tool.Name)
	}
	if tool.Description != "A test tool" {
		t.Errorf("expected description 'A test tool', got %q", tool.Description)
	}
	if tool.Handler == nil {
		t.Error("expected non-nil handler")
	}
}

func TestNewStreamNilChannel(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// nil channel should cause Next to block forever if not cancelled
	// We can't really test this without a timeout, but we can verify
	// the stream is created without panic
	var ch <-chan chanItem
	s := newStream(ch, cancel)
	if s == nil {
		t.Error("expected non-nil stream")
	}
}

func TestStreamErrAfterDone(t *testing.T) {
	ch := make(chan chanItem, 1)
	ch <- chanItem{event: Event{Type: EventTypeDone}}
	close(ch)

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newStream(ch, cancel)
	// First Next should succeed
	if !s.Next() {
		t.Fatal("expected Next to return true")
	}
	// Second Next should return false (done)
	if s.Next() {
		t.Error("expected Next to return false when done")
	}
	// Err should be nil
	if s.Err() != nil {
		t.Errorf("expected nil error, got %v", s.Err())
	}
}

func TestStreamErrAfterError(t *testing.T) {
	ch := make(chan chanItem, 1)
	ch <- chanItem{err: errTest}
	close(ch)

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newStream(ch, cancel)
	s.Next() // This will set the error
	s.Next() // Should still return false
	if s.Err() == nil {
		t.Error("expected error to be preserved")
	}
}

func TestStreamCloseMultiple(t *testing.T) {
	ch := make(chan chanItem)
	_, cancel := context.WithCancel(context.Background())

	s := newStream(ch, cancel)
	// Close multiple times should not panic
	s.Close()
	s.Close()
}

func TestStreamEvent(t *testing.T) {
	ch := make(chan chanItem, 1)
	ch <- chanItem{
		event: Event{
			Type:       EventTypeToolCall,
			ToolCall:   nil,
			ToolResult: nil,
		},
	}
	close(ch)

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := newStream(ch, cancel)
	s.Next()
	evt := s.Event()
	if evt.Type != EventTypeToolCall {
		t.Errorf("expected tool_call type, got %v", evt.Type)
	}
}