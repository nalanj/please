package takeTurn

import (
	"context"

	"github.com/nalanj/please/util/llm"
)

// EventType identifies what kind of event a [Stream] has yielded.
type EventType string

const (
	// EventTypeText is emitted for each incremental text delta from the model.
	EventTypeText EventType = "text"

	// EventTypeToolCall is emitted when the model requests a tool invocation.
	// The agent executes the tool before continuing.
	EventTypeToolCall EventType = "tool_call"

	// EventTypeToolResult is emitted after a tool has been executed, with the
	// result that was sent back to the model.
	EventTypeToolResult EventType = "tool_result"

	// EventTypeDone is emitted once when the agent loop reaches end_turn.
	// The Response field contains the final assembled response.
	EventTypeDone EventType = "done"
)

// Event is a single event from a running [Agent] loop.
//
// The populated fields depend on [EventType]:
//
//   - EventTypeText: Text contains the incremental text delta.
//   - EventTypeToolCall: ToolCall is the tool invocation the model requested.
//   - EventTypeToolResult: ToolResult is the result returned to the model.
//   - EventTypeDone: Response is the final response from the last model turn.
type Event struct {
	Type EventType

	// Incremental text from the model (EventTypeText).
	Text string

	// Tool call requested by the model (EventTypeToolCall).
	// Type is always llm.ContentTypeToolUse.
	ToolCall *llm.ContentBlock

	// Result returned to the model (EventTypeToolResult).
	// Type is always llm.ContentTypeToolResult.
	ToolResult *llm.ContentBlock

	// Final model response (EventTypeDone).
	Response *llm.Response
}

// chanItem is the internal message type sent through the agent's event channel.
type chanItem struct {
	event Event
	err   error
}

// Stream is an iterator over events from a running [Agent] loop.
//
// Usage:
//
//	stream := agent.Run(ctx, "What's the weather in London?")
//	defer stream.Close()
//
//	for stream.Next() {
//	    switch e := stream.Event(); e.Type {
//	    case agent.EventTypeText:
//	        fmt.Print(e.Text)
//	    case agent.EventTypeToolCall:
//	        fmt.Printf("\n[calling %s]\n", e.ToolCall.ToolUseName)
//	    case agent.EventTypeDone:
//	        // e.Response has the complete final response
//	    }
//	}
//	if err := stream.Err(); err != nil { ... }
type Stream struct {
	ch      <-chan chanItem
	cancel  context.CancelFunc
	current Event
	err     error
	done    bool
}

func newStream(ch <-chan chanItem, cancel context.CancelFunc) *Stream {
	return &Stream{ch: ch, cancel: cancel}
}

// Next advances the stream and returns true if an event is available.
// Returns false when the loop is complete or an error occurred.
func (s *Stream) Next() bool {
	if s.done || s.err != nil {
		return false
	}
	item, ok := <-s.ch
	if !ok {
		s.done = true
		return false
	}
	if item.err != nil {
		s.err = item.err
		s.done = true
		return false
	}
	s.current = item.event
	return true
}

// Event returns the current event. Only valid after a successful call to Next.
func (s *Stream) Event() Event {
	return s.current
}

// Err returns the first error encountered during the agent loop, or nil.
func (s *Stream) Err() error {
	return s.err
}

// Close cancels the agent loop and signals it to exit.
// It is safe to call Close at any point; subsequent Next calls return false.
func (s *Stream) Close() {
	s.cancel()
	s.done = true
	// Drain any buffered events to unblock the goroutine.
	// Use non-blocking drain since goroutine may already be done.
	for {
		select {
		case _, ok := <-s.ch:
			if !ok {
				return
			}
		default:
			return
		}
	}
}