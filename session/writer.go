package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nalanj/please/ops/agent/takeTurn"
)

// Writer records events to a session file.
type Writer struct {
	mu        sync.Mutex
	path      string
	file      *os.File
	turn      *Turn
	turnNum   int
	startTime time.Time
	lastMs    int
}

// NewWriter creates a session writer that appends to the given file path.
func NewWriter(path string) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Writer{
		path:      path,
		file:      f,
		turnNum:   0,
		startTime: time.Now(),
	}, nil
}

// StartTurn begins a new turn with the given input and metadata.
func (w *Writer) StartTurn(input string, md Metadata) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.turnNum++
	w.lastMs = 0
	w.startTime = time.Now()
	w.turn = NewTurnPtr(w.turnNum, input, md)
}

// ms returns milliseconds since the turn started.
func (w *Writer) ms() int {
	return int(time.Since(w.startTime).Milliseconds())
}

// HandleEvent records an event from the agent stream.
func (w *Writer) HandleEvent(evt takeTurn.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.turn == nil {
		return
	}

	ms := w.ms()
	w.lastMs = ms

	switch evt.Type {
	case takeTurn.EventTypeText:
		w.turn.AddText(evt.Text, ms)
	case takeTurn.EventTypeThinking:
		w.turn.AddThinking(evt.Thinking, ms)
	case takeTurn.EventTypeToolCall:
		w.turn.AddToolCall(
			evt.ToolCall.ToolUseName,
			evt.ToolCall.ToolUseID,
			string(evt.ToolCall.ToolUseInput),
			ms,
		)
	case takeTurn.EventTypeToolResult:
		w.turn.AddToolResult(
			evt.ToolResult.ToolResultID,
			evt.ToolResult.ToolResultContent,
			len(evt.ToolResult.ToolResultContent),
			evt.ToolResult.ToolResultError,
			ms,
		)
	}
}

// EndTurn finalizes the current turn and writes it to the file.
func (w *Writer) EndTurn() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.turn == nil {
		return nil
	}

	data, err := json.Marshal(w.turn)
	if err != nil {
		return err
	}
	if _, err := w.file.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := w.file.Sync(); err != nil {
		return err
	}

	w.turn = nil
	return nil
}

// Close closes the session file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// CreateNewSession creates a new session file and returns the path.
func CreateNewSession(sessionsDir string) (string, error) {
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		return "", err
	}

	sessionID := newSessionID()
	filename := sessionID + ".jsonl"
	path := filepath.Join(sessionsDir, filename)

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	return path, nil
}

func newSessionID() string {
	// Simple ID generation - 8 hex chars
	t := time.Now().UnixNano()
	return formatUintHex(uint(t), 8)
}

func formatUintHex(n uint, width int) string {
	const hexChars = "0123456789abcdef"
	buf := make([]byte, width)
	for i := width - 1; i >= 0; i-- {
		buf[i] = hexChars[n&0xf]
		n >>= 4
	}
	return string(buf)
}
