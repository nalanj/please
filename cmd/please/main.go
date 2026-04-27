package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nalanj/please/ui/markdown"
	"github.com/nalanj/please/ui/render"
	uistream "github.com/nalanj/please/ui/stream"

	"github.com/nalanj/please/session"

	"github.com/nalanj/please/ops/agent/takeTurn"
	"github.com/nalanj/please/util/llm"
	"github.com/nalanj/please/util/llm/anthropic"
	"github.com/nalanj/please/util/tools"


	"github.com/nalanj/please/cmd/completion"
)

const (
	defaultModel        = "minimax-m2.7"
	systemFile          = "SYSTEM.md"
	agentsFile          = "AGENTS.md"
	dotPleaseDir        = ".please"
	sessionsDir         = "sessions"
	currentSessionSym   = "current-session"
	defaultContextLimit = 200000
)

// Markdown renderer for streaming text
var mdRenderer = md.New()

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// repoRoot returns the root of the git repository if we're in one,
// otherwise the current working directory.
func repoRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	// Walk up the directory tree looking for .git
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return cwd
}

// buildSystemPrompt reads the system prompt from SYSTEM.md.
// Looks in repo root (if in git repo) or current directory.
// Returns built-in default if SYSTEM.md doesn't exist.
func buildSystemPrompt() string {
	root := repoRoot()

	// Read optional SYSTEM.md file
	systemPath := filepath.Join(root, systemFile)
	if data, err := os.ReadFile(systemPath); err == nil {
		return string(data)
	}

	return "You are a helpful assistant."
}

// loadAgentsPrompt returns the contents of AGENTS.md if it exists.
// Looks in repo root (if in git repo) or current directory.
func loadAgentsPrompt() string {
	root := repoRoot()
	agentsPath := filepath.Join(root, agentsFile)
	if data, err := os.ReadFile(agentsPath); err == nil {
		return string(data)
	}
	return ""
}

func run() error {
	// Ensure .please directory and sessions exist
	if err := os.MkdirAll(filepath.Join(dotPleaseDir, sessionsDir), 0o755); err != nil {
		return fmt.Errorf("creating .please directory: %w", err)
	}

	// Check for --new and --one-off flags
	newSession := false
	oneOff := false
	args := os.Args[1:]

	// Handle --completion flag
	for i, arg := range args {
		if arg == "--completion" || arg == "-c" {
			shell := ""
			if i+1 < len(args) {
				shell = args[i+1]
			}
			if err := completion.Generate(os.Stdout, shell); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	for _, arg := range args {
		switch arg {
		case "--new", "-n":
			newSession = true
		case "--one-off", "-1":
			oneOff = true
		}
	}
	// Remove flags from args
	var filtered []string
	for _, arg := range args {
		if arg != "--new" && arg != "-n" && arg != "--one-off" && arg != "-1" {
			filtered = append(filtered, arg)
		}
	}
	args = filtered

	// Check for conflicting flags
	if newSession && oneOff {
		return fmt.Errorf("-n/--new and -1/--one-off cannot be used together")
	}

	// Handle --help flag
	if len(args) >= 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Println("Usage: please [options] <message>")
		fmt.Println("       please -h")
		fmt.Println("       please -n <message>")
		fmt.Println("       please --new <message>")
		fmt.Println("       please -1 <message>")
		fmt.Println("       please --one-off <message>")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -n, --new       Start a new session instead of continuing the current one.")
		fmt.Println("  -1, --one-off   Take a turn without updating the current session symlink.")
		fmt.Println()
		fmt.Println("  -h, --help      Show this help message.")
		fmt.Println("  -c, --completion Generate shell completion script for the specified shell.")
		fmt.Println("                   Supported shells: bash, zsh, fish, powershell")
		fmt.Println()
		fmt.Println("A turn-based agent CLI. Provide a message to continue the current")
		fmt.Println("session or start a new one.")
		return nil
	}

	// Message from args
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: please [options] <message>\n")
		fmt.Fprintf(os.Stderr, "       please -h\n")
		fmt.Fprintf(os.Stderr, "       please -n <message>\n")
		fmt.Fprintf(os.Stderr, "       please -1 <message>\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  -n, --new       Start a new session instead of continuing the current one.\n")
		fmt.Fprintf(os.Stderr, "  -1, --one-off   Take a turn without updating the current session symlink.\n")
		fmt.Fprintf(os.Stderr, "  -h, --help      Show this help message.\n")
		fmt.Fprintf(os.Stderr, "  -h, --help      Show this help message.\n")
		fmt.Fprintf(os.Stderr, "  -c, --completion Generate shell completion script for the specified shell.\n")
		fmt.Fprintf(os.Stderr, "                   Supported shells: bash, zsh, fish, powershell\n")
		fmt.Fprintf(os.Stderr, "\n")
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
	} else if oneOff {
		sessionPath, err = createOneOffSession()
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

	// Read system prompt from files
	system := buildSystemPrompt()

	// Load existing session messages
	existing, err := loadSession(sessionPath)
	if err != nil {
		return fmt.Errorf("loading session: %w", err)
	}

	// If one-off mode, create a new session file to avoid writing to the prior session.
	// We still keep the prior session's messages for context, but save new work elsewhere.
	if oneOff && len(existing) > 0 {
		newSessionID := uuid.New().String()[:8]
		newSessionFilename := fmt.Sprintf("%s.jsonl", newSessionID)
		newSessionPath := filepath.Join(dotPleaseDir, sessionsDir, newSessionFilename)

		// Create new session file and write prior messages to it
		f, err := os.Create(newSessionPath)
		if err != nil {
			return fmt.Errorf("creating one-off session: %w", err)
		}
		enc := json.NewEncoder(f)
		for _, msg := range existing {
			if err := enc.Encode(msg); err != nil {
			_ = f.Close()

				return fmt.Errorf("writing one-off session: %w", err)
			}
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("closing one-off session: %w", err)
		}

		sessionPath = newSessionPath

	}

	// Build agent
	opts := []takeTurn.Option{
		takeTurn.WithTools(tools.All()...),
	}
	if system != "" {
		opts = append(opts, takeTurn.WithSystem(system))
	}

	// Prepend AGENTS.md as a distinct first user message for new/empty sessions
	var prevCount int
	if agentsContent := loadAgentsPrompt(); agentsContent != "" && (newSession || len(existing) == 0) {
		opts = append(opts, takeTurn.WithMessages(llm.TextMessage(llm.RoleUser, agentsContent)))
		prevCount = 1 // AGENTS.md message will be in history before user message
	} else {
		prevCount = len(existing)
	}

	if len(existing) > 0 {
		opts = append(opts, takeTurn.WithMessages(existing...))
	}

	agent := takeTurn.New(provider, defaultModel, opts...)

	startTime := time.Now()

	// Run the agent
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for Ctrl-c (SIGINT = immediate stop) and Ctrl-\ (SIGQUIT = graceful stop)
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGQUIT)

	var shutdownReason string
	var shutdownOnce sync.Once
	handleSignal := func() {
		sig := <-signalCh
		shutdownOnce.Do(func() {
			if sig == syscall.SIGINT {
				shutdownReason = "Ctrl-c"
			} else {
				shutdownReason = "Ctrl-\\"
			}
			cancel()
		})
	}
	go handleSignal()

	stream := agent.Run(ctx, message)
	defer stream.Close()

	// Output handler for streaming markdown
	handler := uistream.New(mdRenderer)

	var toolCallName, toolCallInput string

	// Read events in a goroutine and send to channel
	eventCh := make(chan takeTurn.Event, 50)
	var streamErr error
	var streamWg sync.WaitGroup

	streamWg.Add(1)
	go func() {
		defer streamWg.Done()
		defer close(eventCh)
		for stream.Next() {
			eventCh <- stream.Event()
		}
		streamErr = stream.Err()
	}()

	// Process events with context cancellation check
	var done bool
	for !done {
		select {
		case <-ctx.Done():
			// Context cancelled - user hit Ctrl-c or Ctrl-\
			stream.Close()
			streamWg.Wait()
			_ = bufio.NewWriter(os.Stdout).Flush()
			fmt.Print(handler.FinalFlush())

			// Check if this was graceful shutdown (Ctrl-\)
			if shutdownReason == "Ctrl-\\" {
				// Save partial results before exiting
				newMessages := agent.Messages()[prevCount:]
				if err := appendSession(sessionPath, newMessages); err == nil {
					duration := time.Since(startTime)
					messages := agent.Messages()
					totalTokens := estimateTokens(messages)
					contextPct := float64(totalTokens) / float64(defaultContextLimit) * 100
					fmt.Fprintf(os.Stderr, "\n\n%s\n",
						uistream.InfoStyle.Render(fmt.Sprintf("%s via %s • %s • %.1f%% • %.1fs (saved)",
							defaultModel, providerName(), sessionID(sessionPath), contextPct, duration.Seconds())))
					return nil
				}
			}

			if shutdownReason == "" {
				shutdownReason = "interrupted"
			}
			fmt.Fprintf(os.Stderr, "\n\n%s\n", uistream.InfoStyle.Render(fmt.Sprintf("Session %s %s", sessionID(sessionPath), shutdownReason)))
			return nil

		case evt, ok := <-eventCh:
			if !ok {
				done = true
				break
			}
			switch evt.Type {
			case takeTurn.EventTypeText:
				fmt.Print(handler.Handle("text", evt.Text))

			case takeTurn.EventTypeThinking:
				fmt.Print(handler.Handle("thinking", evt.Thinking))

			case takeTurn.EventTypeToolCall:
				toolCallName = evt.ToolCall.ToolUseName
				toolCallInput = string(evt.ToolCall.ToolUseInput)
				handler.SwitchSection()

			case takeTurn.EventTypeToolResult:
				handler.SwitchSection()
				fmt.Fprintf(os.Stderr, "\n")
				if evt.ToolResult.ToolResultError {
					render.RenderToolError(toolCallName, toolCallInput, evt.ToolResult.ToolResultContent, evt.ToolResult.ToolResultContent)
				} else {
					render.RenderToolCall(toolCallName, toolCallInput, evt.ToolResult.ToolResultContent, formatResultSummary(toolCallName, evt.ToolResult.ToolResultContent))
				}

			case takeTurn.EventTypeDone:
				fmt.Print(handler.Handle("flush", ""))
			}
		}
	}

	streamWg.Wait()
	_ = bufio.NewWriter(os.Stdout).Flush()
	output := handler.FinalFlush()
	fmt.Print(output)

	// Check error after loop - handle signal-based cancellation
	if streamErr != nil {
		// Check if context was cancelled (Ctrl-c/Ctrl-\)
		if ctx.Err() != nil || shutdownReason != "" {
			_ = bufio.NewWriter(os.Stdout).Flush()
			msg := shutdownReason
			if msg == "" {
				msg = "interrupted"
			}
			fmt.Fprintf(os.Stderr, "\n\n%s\n", uistream.InfoStyle.Render(fmt.Sprintf("Session %s %s", sessionID(sessionPath), msg)))
			return nil
		}
		return fmt.Errorf("agent: %w", streamErr)
	}

	// Normal completion - print session summary
	duration := time.Since(startTime)
	messages := agent.Messages()
	totalTokens := estimateTokens(messages)
	contextPct := float64(totalTokens) / float64(defaultContextLimit) * 100

	// Ensure stdout is fully flushed before printing conclusion line
	_ = bufio.NewWriter(os.Stdout).Flush()
	fmt.Fprintf(os.Stderr, "\n\n%s\n",
		uistream.InfoStyle.Render(fmt.Sprintf("%s via %s • %s • %.1f%% • %.1fs",
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

// createOneOffSession creates a session for one-off mode without updating the symlink.
func createOneOffSession() (string, error) {
	symlinkPath := filepath.Join(dotPleaseDir, currentSessionSym)

	// Check if current-session symlink exists and is valid
	if info, err := os.Lstat(symlinkPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Readlink(symlinkPath); err == nil {
			if _, err := os.Stat(target); err == nil {
				return target, nil
			}
		}
	}

	// No active session, create a disposable one without updating symlink
	// Use a uuid-based filename
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

	return sessionPath, nil
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
	sessionPath, err := session.CreateNewSession(filepath.Join(dotPleaseDir, sessionsDir))
	if err != nil {
		return "", err
	}

	// Create symlink to current session
	if err := os.Symlink(sessionPath, symlinkPath); err != nil {
		return "", err
	}

	return sessionPath, nil
}

// loadSession reads messages from a session file using the session package.
func loadSession(path string) ([]llm.Message, error) {
	// Use the session.Reader to load turns
	reader := session.NewReader(path)
	turns, err := reader.Load()
	if err != nil {
		return nil, err
	}
	if turns == nil {
		return nil, nil
	}

	// Convert turns to messages
	messages := make([]llm.Message, 0, len(turns)*2)
	for _, turn := range turns {
		// Add user message
		if turn.Input != "" {
			messages = append(messages, llm.TextMessage(llm.RoleUser, turn.Input))
		}
		// Convert events to assistant message
		var content []llm.ContentBlock
		for _, evt := range turn.Events {
			switch evt.Type {
			case "text":
				content = append(content, llm.ContentBlock{Type: llm.ContentTypeText, Text: evt.Chunk})
			case "thinking":
				content = append(content, llm.ContentBlock{Type: llm.ContentTypeThinking, Thinking: evt.Chunk})
			case "tool_call":
				content = append(content, llm.ContentBlock{
					Type:        llm.ContentTypeToolUse,
					ToolUseID:   evt.ID,
					ToolUseName: evt.Name,
					ToolUseInput: json.RawMessage(evt.Input),
				})
			case "tool_result":
				content = append(content, llm.ContentBlock{
					Type:              llm.ContentTypeToolResult,
					ToolResultID:      evt.ID,
					ToolResultContent: evt.Content,
					ToolResultError:   evt.Error,
				})
			}
		}
		if len(content) > 0 {
			messages = append(messages, llm.Message{Role: llm.RoleAssistant, Content: content})
		}
	}

	return messages, nil
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
