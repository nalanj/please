package llm

import "context"

// StreamEventType identifies the kind of event emitted during streaming.
type StreamEventType string

const (
	// StreamEventTypeText is emitted for each incremental text delta.
	StreamEventTypeText StreamEventType = "text"
	// StreamEventTypeThinking is emitted for each incremental thinking delta.
	StreamEventTypeThinking StreamEventType = "thinking"
	// StreamEventTypeToolUse is emitted when a complete tool call is assembled.
	StreamEventTypeToolUse StreamEventType = "tool_use"
	// StreamEventTypeDone is emitted once when the stream is complete.
	StreamEventTypeDone StreamEventType = "done"
)

// StreamEvent is a single event emitted during a streaming response.
type StreamEvent struct {
	Type StreamEventType

	// Text delta (StreamEventTypeText).
	Text string

	// Thinking delta (StreamEventTypeThinking).
	Thinking string

	// Tool call (StreamEventTypeToolUse).
	ToolUse *ContentBlock

	// Final response (StreamEventTypeDone).
	Response *Response
}

// Stream is an iterator over [StreamEvent]s from a streaming LLM call.
// It is returned by [Provider.Stream]; the caller must call [Stream.Close]
// when finished.
type Stream struct {
	nextFn  func() (StreamEvent, bool, error)
	closeFn func() error

	event StreamEvent
	err   error
	done  bool
}

// NewStream constructs a [Stream]. nextFn is called on each [Stream.Next]
// call and must return the next event (or false when done). closeFn is
// called by [Stream.Close] and may be nil.
func NewStream(nextFn func() (StreamEvent, bool, error), closeFn func() error) *Stream {
	return &Stream{
		nextFn:  nextFn,
		closeFn: closeFn,
	}
}

// Next advances the stream and returns true if an event is available.
// Returns false when the stream is exhausted or an error occurred.
func (s *Stream) Next() bool {
	if s.done || s.err != nil {
		return false
	}

	event, ok, err := s.nextFn()
	if err != nil {
		s.err = err
		s.done = true
		return false
	}
	if !ok {
		s.done = true
		return false
	}

	s.event = event
	return true
}

// Event returns the current event. It is only valid after a successful
// call to [Stream.Next].
func (s *Stream) Event() StreamEvent {
	return s.event
}

// Err returns the first error encountered during iteration, or nil.
func (s *Stream) Err() error {
	return s.err
}

// Close releases any resources held by the stream. It is safe to call
// Close multiple times. If closeFn returns an error, it is returned by Close.
func (s *Stream) Close() error {
	if s.done {
		return nil
	}
	s.done = true
	if s.closeFn == nil {
		return nil
	}
	return s.closeFn()
}

// Provider is the common interface for sending messages to an LLM API.
// All backend implementations (Anthropic, OpenCode, Kilo, …) satisfy this
// interface so that agent code can be written against it without depending on
// any specific vendor.
type Provider interface {
	// Send sends a request and blocks until the complete response is available.
	Send(ctx context.Context, req Request) (*Response, error)

	// Stream sends a request and returns a [Stream] that yields events as the
	// model generates. The caller must call [Stream.Close] when finished.
	Stream(ctx context.Context, req Request) (*Stream, error)
}