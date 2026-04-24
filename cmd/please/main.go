// Package main provides the please CLI.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nalanj/please/ui/markdown"
	"github.com/nalanj/please/ui/render"
	"github.com/google/uuid"

	"github.com/nalanj/please/ops/agent/takeTurn"
	"github.com/nalanj/please/util/llm"
	"github.com/nalanj/please/util/llm/anthropic"
	"github.com/nalanj/please/util/tools"
)

const (
	defaultModel         = "minimax-m2.7"
	systemFile           = "SYSTEM.md"
	dotPleaseDir         = ".please"
	sessionsDir          = "sessions"
	currentSessionSym    = "current-session"
	defaultContextLimit  = 200000
)

// ThoughtStyle for rendering thought/thinking output
var ThoughtStyle = lipgloss.NewStyle().Italic(true)

// Markdown renderer for streaming text
var mdRenderer = md.New()

// InfoStyle for rendering the end-of-turn info line
var InfoStyle = lipgloss.NewStyle().Faint(true)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Ensure .please directory and sessions exist
	if err := os.MkdirAll(filepath.Join(dotPleaseDir, sessionsDir), 0o755); err != nil {
		return fmt.Errorf("creating .please directory: %w", err)
	}

	// Check for --new flag first
	newSession := false
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "--new" {
			newSession = true
			args = append(args[:i], args[i+1:]...)
			break
		}
	}

	// Handle --help flag
	if len(args) >= 1 && args[0] == "--help" {
		fmt.Println("Usage: please [options] <message>")
		fmt.Println("       please --help")
		fmt.Println("       please --new <message>")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --new    Start a new session instead of continuing the current one.")
		fmt.Println()
		fmt.Println("A turn-based agent CLI. Provide a message to continue the current")
		fmt.Println("session or start a new one.")
		return nil
	}

	// Message from args
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: please <message>\n")
		return fmt.Errorf("no message provided")
	}
	message := strings.Join(args, " ")
	if message == "" {
		return fmt.Errorf("no message provided")
	}

	// Load existing session or create new one
	var sessionPath string
	var err error
	if newSession {
		sessionPath, err = createNewSession()
	} else {
		sessionPath, err = loadOrCreateSession()
	}
	if err != nil {
		return fmt.Errorf("session: %w", err)
	}

	// Build provider
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("MINIMAX_API_KEY environment variable is not set")
	}
	provider := anthropic.NewMiniMaxProvider(apiKey)

	// Read optional system prompt
	var system string
	if data, err := os.ReadFile(systemFile); err == nil {
		system = string(data)
	}

	// Load existing session messages
	existing, err := loadSession(sessionPath)
	if err != nil {
		return fmt.Errorf("loading session: %w", err)
	}

	// Build agent
	opts := []takeTurn.Option{
		takeTurn.WithTools(tools.All()...),
	}
	if system != "" {
		opts = append(opts, takeTurn.WithSystem(system))
	}
	if len(existing) > 0 {
		opts = append(opts, takeTurn.WithMessages(existing...))
	}
	agent := takeTurn.New(provider, defaultModel, opts...)
	prevCount := len(existing)

	startTime := time.Now()

	// Run the agent
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := agent.Run(ctx, message)
	defer stream.Close()

	// Buffer for building styled tool blocks
	var (
		toolCallName   string
		toolCallInput string
		buffering      bool
		bufferedOutput strings.Builder
	)

	flushBuffer := func() {
		if buffering {
			fmt.Fprintf(os.Stderr, "%s", bufferedOutput.String())
			bufferedOutput.Reset()
			buffering = false
		}
	}

	for stream.Next() {
		evt := stream.Event()
		switch evt.Type {
		case takeTurn.EventTypeText:
			if buffering {
				bufferedOutput.WriteString(evt.Text)
			} else {
				fmt.Print(mdRenderer.Write(evt.Text))
			}

		case takeTurn.EventTypeThinking:
			// Collapse mid-line excessive whitespace (3+ spaces) that causes visual gaps
			// but preserve meaningful spacing
			lines := strings.Split(evt.Thinking, "\n")
			for i, line := range lines {
				// Trim trailing first
				line = strings.TrimRight(line, " \t")
				// Collapse 3+ spaces to single space (formatting artifacts)
				for strings.Contains(line, "   ") {
					line = strings.Replace(line, "   ", " ", -1)
				}
				lines[i] = line
			}
			thinking := strings.TrimRight(strings.Join(lines, "\n"), "\r\n")
			if thinking != "" {
				fmt.Print(ThoughtStyle.Render(thinking))
			}

		case takeTurn.EventTypeToolCall:
			toolCallName = evt.ToolCall.ToolUseName
			toolCallInput = string(evt.ToolCall.ToolUseInput)
			buffering = true
			bufferedOutput.Reset()

		case takeTurn.EventTypeToolResult:
			// Render the complete styled block
			fmt.Fprintf(os.Stderr, "\n")
			if evt.ToolResult.ToolResultError {
				render.RenderToolError(toolCallName, toolCallInput, evt.ToolResult.ToolResultContent, evt.ToolResult.ToolResultContent)
			} else {
				render.RenderToolCall(toolCallName, toolCallInput, evt.ToolResult.ToolResultContent, formatResultSummary(toolCallName, evt.ToolResult.ToolResultContent))
			}
			bufferedOutput.Reset()
			buffering = false

		case takeTurn.EventTypeDone:
			flushBuffer()
			// Flush any remaining markdown buffer
			if remaining := mdRenderer.Flush(); remaining != "" {
				fmt.Print(remaining)
			}
			fmt.Println()
		}
	}
	flushBuffer()
	if err := stream.Err(); err != nil {
		return fmt.Errorf("agent: %w", err)
	}

	duration := time.Since(startTime)
	messages := agent.Messages()
	totalTokens := estimateTokens(messages)
	contextPct := float64(totalTokens) / float64(defaultContextLimit) * 100

	fmt.Fprintf(os.Stderr, "%s\n",
		InfoStyle.Render(fmt.Sprintf("%s via %s • %s • %.1f%% • %.1fs",
			defaultModel, providerName(), sessionID(sessionPath), contextPct, duration.Seconds())))

	// Persist new messages
	newMessages := agent.Messages()[prevCount:]
	if err := appendSession(sessionPath, newMessages); err != nil {
		return fmt.Errorf("writing session: %w", err)
	}

	return nil
}

// createNewSession creates a new session, replacing any existing current session.
func createNewSession() (string, error) {
	symlinkPath := filepath.Join(dotPleaseDir, currentSessionSym)

	// Remove existing symlink
	if err := os.Remove(symlinkPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	return newSession(symlinkPath)
}

// loadOrCreateSession returns the path to the current session, creating a new one
// if no session is active.
func loadOrCreateSession() (string, error) {
	symlinkPath := filepath.Join(dotPleaseDir, currentSessionSym)

	// Check if current-session symlink exists and is valid
	if info, err := os.Lstat(symlinkPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Readlink(symlinkPath); err == nil {
			if _, err := os.Stat(target); err == nil {
				return target, nil
			}
		}
	}

	return newSession(symlinkPath)
}

// newSession creates a new session file and updates the symlink.
func newSession(symlinkPath string) (string, error) {
	sessionID := uuid.New().String()[:8]
	sessionFilename := fmt.Sprintf("%s.jsonl", sessionID)
	sessionPath := filepath.Join(dotPleaseDir, sessionsDir, sessionFilename)

	// Create empty session file
	f, err := os.Create(sessionPath)
	if err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	// Create symlink to current session
	if err := os.Symlink(sessionPath, symlinkPath); err != nil {
		return "", err
	}

	return sessionPath, nil
}

// loadSession reads messages from a session file.
func loadSession(path string) ([]llm.Message, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var messages []llm.Message
	scanner := bufio.NewScanner(f)
	// Allow large lines for tool results
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg llm.Message
		if err := json.Unmarshal(line, &msg); err != nil {
			return nil, fmt.Errorf("malformed session line: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, scanner.Err()
}

// formatResultSummary creates a human-readable summary of tool results.
func formatResultSummary(name, content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	switch name {
	case "read":
		// Count non-empty lines
		count := 0
		for _, line := range lines {
			if line != "" {
				count++
			}
		}
		return fmt.Sprintf("→ %d lines", count)
	default:
		return "→ done"
	}
}

// appendSession appends messages to the session file.
func appendSession(path string, messages []llm.Message) error {
	if len(messages) == 0 {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	enc := json.NewEncoder(f)
	for _, msg := range messages {
		if err := enc.Encode(msg); err != nil {
			return err
		}
	}
	return nil
}

// providerName returns a human-readable name for the current provider.
func providerName() string {
	return "MiniMax"
}

// sessionID extracts the session ID (uuid prefix) from a session file path.
func sessionID(path string) string {
	base := filepath.Base(path)
	if idx := strings.Index(base, ".jsonl"); idx != -1 {
		base = base[:idx]
	}
	return base
}

// estimateTokens estimates the total token count for all messages.
func estimateTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += estimateMessageTokens(msg)
	}
	return total
}

// estimateMessageTokens estimates the token count for a single message.
func estimateMessageTokens(msg llm.Message) int {
	var sb strings.Builder
	sb.WriteString(string(msg.Role))
	for _, block := range msg.Content {
		switch block.Type {
		case llm.ContentTypeText:
			sb.WriteString(block.Text)
		case llm.ContentTypeThinking:
			sb.WriteString(block.Thinking)
		case llm.ContentTypeToolUse:
			sb.WriteString(block.ToolUseName)
			sb.WriteString(string(block.ToolUseInput))
		case llm.ContentTypeToolResult:
			sb.WriteString(block.ToolResultContent)
		}
	}
	return len(sb.String()) / 4
}
