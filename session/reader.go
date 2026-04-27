package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// ContentBlock mirrors llm.ContentBlock for internal use in session files
type ContentBlock struct {
	Type string `json:"type"`

	// Text (ContentTypeText)
	Text string `json:"text,omitempty"`

	// Thinking (ContentTypeThinking)
	Thinking string `json:"thinking,omitempty"`

	// Tool use
	ToolUseID    string          `json:"tool_use_id,omitempty"`
	ToolUseName  string          `json:"tool_use_name,omitempty"`
	ToolUseInput json.RawMessage `json:"tool_use_input,omitempty"`

	// Tool result
	ToolResultID      string `json:"tool_result_id,omitempty"`
	ToolResultContent string `json:"tool_result_content,omitempty"`
	ToolResultError   bool   `json:"tool_result_error,omitempty"`
}

// Content type constants
const (
	ContentTypeText       = "text"
	ContentTypeThinking    = "thinking"
	ContentTypeToolUse    = "tool_use"
	ContentTypeToolResult = "tool_result"
)

// Reader reads session files for replay.
type Reader struct {
	path string
}

// NewReader creates a reader for the given session file.
func NewReader(path string) *Reader {
	return &Reader{path: path}
}

// Load reads all turns from the session file.
// It handles both the old message format (for backward compatibility)
// and the new turn-based format.
func (r *Reader) Load() ([]Turn, error) {
	f, err := os.Open(r.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var turns []Turn
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// First, detect whether this is the new Turn format or old Message format
		// by peeking at the JSON structure.
		var raw json.RawMessage
		if err := json.Unmarshal(line, &raw); err != nil {
			return nil, fmt.Errorf("malformed line: %w", err)
		}

		// Check for version field to identify new format
		var doc struct {
			Version int `json:"v"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("malformed line: %w", err)
		}

		if doc.Version > 0 {
			// New format - it's a Turn
			var turn Turn
			if err := json.Unmarshal(raw, &turn); err != nil {
				return nil, fmt.Errorf("malformed turn: %w", err)
			}
			turns = append(turns, turn)
		} else {
			// Old format - it's a Message, convert to Turn
			var msg struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			}
			if err := json.Unmarshal(raw, &msg); err != nil {
				return nil, fmt.Errorf("malformed message: %w", err)
			}

			// Convert message to turn
			turn := messageToTurn(msg.Role, msg.Content)
			turns = append(turns, turn)
		}
	}

	return turns, scanner.Err()
}

// messageToTurn converts an old-style message to a Turn.
func messageToTurn(role string, content json.RawMessage) Turn {
	var blocks []ContentBlock
	if err := json.Unmarshal(content, &blocks); err != nil {
		// If content isn't an array, it might be a simple text
		var text string
		if json.Unmarshal(content, &text) == nil {
			blocks = []ContentBlock{{Type: ContentTypeText, Text: text}}
		}
	}

	// Convert blocks to events
	var events []Event
	for _, block := range blocks {
		switch block.Type {
		case ContentTypeText:
			events = append(events, Event{Type: "text", Chunk: block.Text, Size: len(block.Text)})
		case ContentTypeThinking:
			events = append(events, Event{Type: "thinking", Chunk: block.Thinking, Size: len(block.Thinking)})
		case ContentTypeToolUse:
			events = append(events, Event{
				Type:  "tool_call",
				Name:  block.ToolUseName,
				ID:    block.ToolUseID,
				Input: string(block.ToolUseInput),
				Size:  len(block.ToolUseInput),
			})
		case ContentTypeToolResult:
			events = append(events, Event{
				Type:    "tool_result",
				ID:      block.ToolResultID,
				Content: block.ToolResultContent,
				Error:   block.ToolResultError,
				Size:    len(block.ToolResultContent),
			})
		}
	}

	// Extract input text from user messages
	var input string
	if role == "user" && len(blocks) > 0 && blocks[0].Type == ContentTypeText {
		input = blocks[0].Text
	}

	return Turn{
		Version:   FormatVersion,
		Turn:      0, // Will be set correctly when replaying old sessions
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Input:     input,
		Events:    events,
	}
}

// EventIterator iterates over events with timing information.
type EventIterator struct {
	turns     []Turn
	turnIdx   int
	eventIdx  int
	instant   bool
	scale     float64
	filter    string
	lastPrint time.Time
}

// ReplayOptions configures replay behavior.
type ReplayOptions struct {
	Instant  bool
	Scale    float64 // 1.0 = real time, 2.0 = 2x speed, 0.5 = half speed
	Filter   string  // "text", "thinking", "tool_call", "tool_result", or "" for all
	FromTurn int     // start from this turn number (1-indexed)
}

// Replay plays back events to the given writer.
func (r *Reader) Replay(opts ReplayOptions, w io.Writer) error {
	turns, err := r.Load()
	if err != nil {
		return err
	}

	iter := &EventIterator{
		turns:   turns,
		instant: opts.Instant,
		scale:   opts.Scale,
		filter:  opts.Filter,
	}

	if opts.Scale == 0 {
		iter.scale = 1.0
	}

	// Skip to FromTurn
	if opts.FromTurn > 0 {
		iter.turnIdx = opts.FromTurn - 1
		if iter.turnIdx >= len(turns) {
			return fmt.Errorf("turn %d not found (session has %d turns)", opts.FromTurn, len(turns))
		}
	}

	iter.lastPrint = time.Now()
	for iter.Next() {
		evt := iter.Event()
		if evt == nil {
			break
		}
		if !iter.instant && iter.scale > 0 {
			delay := time.Duration(float64(evt.Ms) / iter.scale * 1e6)
			elapsed := time.Since(iter.lastPrint)
			if elapsed < delay {
				time.Sleep(delay - elapsed)
			}
			iter.lastPrint = time.Now()
		}

		// Output the event
		if _, err := fmt.Fprintf(w, "%#v\n", evt); err != nil {
			return err
		}
	}

	return nil
}

// Next advances to the next event.
// Returns false when all events have been consumed.
func (i *EventIterator) Next() bool {
	for i.turnIdx < len(i.turns) {
		turn := &i.turns[i.turnIdx]
		if i.eventIdx < len(turn.Events) {
			event := &turn.Events[i.eventIdx]
			i.eventIdx++
			if i.filter != "" && event.Type != i.filter {
				continue
			}
			return true
		}
		i.turnIdx++
		i.eventIdx = 0
	}
	return false
}

// Event returns the current event.
func (i *EventIterator) Event() *Event {
	if i.turnIdx >= len(i.turns) {
		return nil
	}
	turn := &i.turns[i.turnIdx]
	if i.eventIdx >= len(turn.Events) {
		return nil
	}
	return &turn.Events[i.eventIdx]
}

// Turn returns metadata about the current turn.
func (i *EventIterator) Turn() *Turn {
	if i.turnIdx >= len(i.turns) {
		return nil
	}
	return &i.turns[i.turnIdx]
}
