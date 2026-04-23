package takeTurn

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/nalanj/please/util/llm"
)

const defaultMaxTokens = 8192

// Agent runs an agentic loop backed by an [llm.Provider].
// It maintains conversation history internally and handles the
// model → tool → model cycle automatically.
type Agent struct {
	provider  llm.Provider
	model     string
	system    string
	tools     []Tool
	toolIndex map[string]*Tool
	maxTokens int

	// contextLimit is the maximum context window size (in tokens).
	// Default is 200000 (Claude 3.5/3.7 default).
	contextLimit int

	mu       sync.Mutex
	messages []llm.Message
}

// ContextLimitOption configures the context window limit for compaction.
func ContextLimitOption(limit int) Option {
	return func(a *Agent) { a.contextLimit = limit }
}

// Option configures an [Agent].
type Option func(*Agent)

// WithSystem sets the system prompt sent on every request.
func WithSystem(system string) Option {
	return func(a *Agent) { a.system = system }
}

// WithTools registers tools that the model may call.
func WithTools(tools ...Tool) Option {
	return func(a *Agent) { a.tools = append(a.tools, tools...) }
}

// WithMaxTokens overrides the maximum number of tokens generated per LLM call
// (default: 8192).
func WithMaxTokens(n int) Option {
	return func(a *Agent) { a.maxTokens = n }
}

// WithMessages seeds the conversation history with the provided messages before
// any call to [Agent.Run]. This is useful for injecting few-shot examples or
// replaying a prior conversation.
//
// Messages are appended in order and are preserved across Reset calls only if
// WithMessages is passed again to a new Agent.
func WithMessages(messages ...llm.Message) Option {
	return func(a *Agent) { a.messages = append(a.messages, messages...) }
}

// New creates an Agent backed by provider using the specified model.
func New(provider llm.Provider, model string, opts ...Option) *Agent {
	a := &Agent{
		provider:     provider,
		model:        model,
		maxTokens:    defaultMaxTokens,
		toolIndex:    map[string]*Tool{},
		contextLimit: 200000, // Default context window (Claude 3.5/3.7)
	}
	for _, o := range opts {
		o(a)
	}
	for i := range a.tools {
		a.toolIndex[a.tools[i].Name] = &a.tools[i]
	}
	return a
}

// Messages returns a snapshot of the current conversation history.
// Safe to call concurrently with a running [Stream].
func (a *Agent) Messages() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]llm.Message, len(a.messages))
	copy(out, a.messages)
	return out
}

// Reset clears the conversation history. Do not call while a [Stream] is open.
func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = nil
}

// Continue starts the agent loop using the existing conversation history
// without appending a new user message. This is useful for resuming after an
// error or an interrupted turn.
//
// Returns an error stream if there are no messages in the history.
// Continue must not be called concurrently on the same Agent.
func (a *Agent) Continue(ctx context.Context) *Stream {
	a.mu.Lock()
	empty := len(a.messages) == 0
	a.mu.Unlock()
	if empty {
		_, cancel := context.WithCancel(ctx)
		cancel()
		ch := make(chan chanItem, 1)
		ch <- chanItem{err: fmt.Errorf("no conversation to continue: session is empty")}
		close(ch)
		return newStream(ch, cancel)
	}

	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan chanItem, 32)
	go func() {
		defer close(ch)
		a.runLoop(ctx, ch)
	}()
	return newStream(ch, cancel)
}

// Run appends userMessage to the conversation and starts the agent loop.
// It returns a [*Stream] that emits events as the model generates text,
// calls tools, and receives results. The loop runs until the model reaches
// end_turn or the context is cancelled.
//
// Run must not be called concurrently on the same Agent.
func (a *Agent) Run(ctx context.Context, userMessage string) *Stream {
	ctx, cancel := context.WithCancel(ctx)

	a.mu.Lock()
	a.messages = append(a.messages, llm.TextMessage(llm.RoleUser, userMessage))
	a.mu.Unlock()

	ch := make(chan chanItem, 32)
	go func() {
		defer close(ch)
		a.runLoop(ctx, ch)
	}()

	return newStream(ch, cancel)
}

// runLoop is the core agent loop, run in a background goroutine.
func (a *Agent) runLoop(ctx context.Context, ch chan<- chanItem) {
	send := func(item chanItem) bool {
		select {
		case ch <- item:
			return true
		case <-ctx.Done():
			return false
		}
	}

	for {
		a.mu.Lock()
		msgs := make([]llm.Message, len(a.messages))
		copy(msgs, a.messages)
		a.mu.Unlock()

		req := llm.Request{
			Model:     a.model,
			System:    a.system,
			Messages:  msgs,
			MaxTokens: a.maxTokens,
			Tools:     a.llmTools(),
		}

		llmStream, err := a.provider.Stream(ctx, req)
		if err != nil {
			send(chanItem{err: err})
			return
		}

		var response *llm.Response
		for llmStream.Next() {
			evt := llmStream.Event()
			switch evt.Type {
			case llm.StreamEventTypeText:
				if !send(chanItem{event: Event{Type: EventTypeText, Text: evt.Text}}) {
					_ = llmStream.Close()
					return
				}
			case llm.StreamEventTypeThinking:
				if !send(chanItem{event: Event{Type: EventTypeThinking, Thinking: evt.Thinking}}) {
					_ = llmStream.Close()
					return
				}
			case llm.StreamEventTypeToolUse:
				if !send(chanItem{event: Event{Type: EventTypeToolCall, ToolCall: evt.ToolUse}}) {
					_ = llmStream.Close()
					return
				}
			case llm.StreamEventTypeDone:
				response = evt.Response
			}
		}
		_ = llmStream.Close()

		if err := llmStream.Err(); err != nil {
			send(chanItem{err: err})
			return
		}
		if response == nil {
			return
		}

		a.mu.Lock()
		a.messages = append(a.messages, response.Message)
		a.mu.Unlock()

		if response.StopReason != llm.StopReasonToolUse {
			send(chanItem{event: Event{Type: EventTypeDone, Response: response}})
			return
		}

		// Execute each tool call in parallel.
		// Collect tool use blocks first, preserving their original order.
		var toolUseBlocks []*llm.ContentBlock
		for i := range response.Message.Content {
			block := &response.Message.Content[i]
			if block.Type == llm.ContentTypeToolUse {
				toolUseBlocks = append(toolUseBlocks, block)
			}
		}

		if len(toolUseBlocks) == 0 {
			return
		}

		// Run tools in parallel, preserving order for the model.
		type toolResult struct {
			index int
			block llm.ContentBlock
		}
		results := make(chan toolResult, len(toolUseBlocks))
		var wg sync.WaitGroup
		for idx, tb := range toolUseBlocks {
			wg.Add(1)
			go func(i int, block *llm.ContentBlock) {
				defer wg.Done()
				content, isError := a.executeTool(ctx, block)
				results <- toolResult{
					index: i,
					block: llm.ContentBlock{
						Type:              llm.ContentTypeToolResult,
						ToolResultID:      block.ToolUseID,
						ToolResultContent: content,
						ToolResultError:   isError,
					},
				}
			}(idx, tb)
		}

		// Wait for all tools to complete, then close the results channel.
		wg.Wait()
		close(results)

		// Collect results in order.
		orderedResults := make([]llm.ContentBlock, len(toolUseBlocks))
		for r := range results {
			orderedResults[r.index] = r.block
		}

		// Emit results and build the result blocks for the message.
		var resultBlocks []llm.ContentBlock
		for _, rb := range orderedResults {
			if rb.Type != "" { // Skip empty results
				if !send(chanItem{event: Event{Type: EventTypeToolResult, ToolResult: &rb}}) {
					return
				}
				resultBlocks = append(resultBlocks, rb)
			}
		}

		if len(resultBlocks) == 0 {
			return
		}

		a.mu.Lock()
		a.messages = append(a.messages, llm.ToolResultMessage(resultBlocks...))
		a.mu.Unlock()

		if ctx.Err() != nil {
			return
		}
	}
}

// executeTool calls the named tool and returns (content, isError).
func (a *Agent) executeTool(ctx context.Context, block *llm.ContentBlock) (string, bool) {
	tool, ok := a.toolIndex[block.ToolUseName]
	if !ok {
		return fmt.Sprintf("unknown tool: %q", block.ToolUseName), true
	}
	// Ensure we pass valid JSON to the handler (empty input -> empty object).
	input := block.ToolUseInput
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	result, err := tool.Handler(ctx, input)
	if err != nil {
		return err.Error(), true
	}
	return result, false
}

// llmTools converts the agent's registered tools to the format expected by
// [llm.Request].
func (a *Agent) llmTools() []llm.Tool {
	if len(a.tools) == 0 {
		return nil
	}
	out := make([]llm.Tool, len(a.tools))
	for i, t := range a.tools {
		out[i] = llm.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	return out
}

// estimateTokens estimates the number of tokens in a message.
// Uses a rough heuristic: ~4 characters per token.
func estimateTokens(msg llm.Message) int {
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
	// Rough estimate: 4 characters per token
	return len(sb.String()) / 4
}

// totalTokens estimates the total token count for all messages.
func totalTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		total += estimateTokens(msg)
	}
	return total
}

// Compact attempts to reduce the conversation history to fit within the context limit.
// The strategy is:
//  1. Score messages from most recent to oldest in batches
//  2. Remove insignificant and low scored messages as we go
//  3. If still over 80% after scoring, ask the model to compress
func (a *Agent) Compact(ctx context.Context) error {
	a.mu.Lock()
	messages := make([]llm.Message, len(a.messages))
	copy(messages, a.messages)
	a.mu.Unlock()

	if len(messages) == 0 {
		return nil
	}

	currentTokens := totalTokens(messages)
	threshold := int(float64(a.contextLimit) * 0.8)

	fmt.Printf("Compaction: %d tokens (limit: %d, threshold: %d)\n", currentTokens, a.contextLimit, threshold)

	// If we're already under the threshold, nothing to do
	if currentTokens <= threshold {
		fmt.Println("Compaction: already under threshold, nothing to do")
		return nil
	}

	// Score messages from oldest to newest in batches.
	// Keep track of which messages to keep.
	keep := make([]bool, len(messages))
	for i := range keep {
		keep[i] = true // Default to keeping everything
	}
	// Never compact the system prompt (message 0)
	if len(messages) > 0 {
		keep[0] = true
	}

	// Process in batches from oldest to newest.
	// Use a reasonable message count that will typically fit within token limits.
	// With ~500 tokens/message average, 30 messages ≈ 15k tokens, well under typical limits.
	batchSize := 30
	fmt.Printf("Compaction: scoring %d messages in batches of %d\n", len(messages), batchSize)
	batchNum := 0
	for batchStart := 1; batchStart < len(messages); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(messages) {
			batchEnd = len(messages)
		}

		// Extract batch
		batch := make([]llm.Message, 0, batchSize)
		for i := batchStart; i < batchEnd; i++ {
			if keep[i] {
				batch = append(batch, messages[i])
			}
		}

		if len(batch) == 0 {
			continue
		}

		batchNum++
		fmt.Printf("Compaction: scoring batch %d (messages %d-%d)\n", batchNum, batchStart, batchEnd-1)

		// Score this batch
		scores, err := a.scoreMessages(ctx, batch)
		if err != nil {
			// If scoring fails, skip this batch
			fmt.Printf("Compaction: batch %d scoring failed: %v, skipping\n", batchNum, err)
			continue
		}

		// Mark messages to remove (insignificant and low)
		scoreIdx := 0
		removed := 0
		for i := batchStart; i < batchEnd; i++ {
			if keep[i] && scoreIdx < len(scores) {
				score := scores[scoreIdx]
				if score == "insignificant" || score == "low" {
					keep[i] = false
					removed++
				}
				scoreIdx++
			}
		}
		fmt.Printf("Compaction: batch %d complete, removed %d messages\n", batchNum, removed)

		// Check if we're under threshold now
		keptCount := 0
		for _, k := range keep {
			if k {
				keptCount++
			}
		}
		keptMessages := make([]llm.Message, 0, keptCount)
		for i, k := range keep {
			if k {
				keptMessages = append(keptMessages, messages[i])
			}
		}

		currentKeptTokens := totalTokens(keptMessages)
		fmt.Printf("Compaction: after batch %d: %d tokens (%d messages)\n", batchNum, currentKeptTokens, keptCount)

		if currentKeptTokens <= threshold {
			a.mu.Lock()
			a.messages = keptMessages
			a.mu.Unlock()
			fmt.Printf("Compaction: done after scoring, reduced to %d tokens\n", currentKeptTokens)
			return nil
		}
	}

	// Build filtered list
	filtered := make([]llm.Message, 0, len(messages))
	for i, k := range keep {
		if k {
			filtered = append(filtered, messages[i])
		}
	}

	// Ensure tool_use/tool_result pairs are handled correctly.
	// If a tool_use was removed, we need to also remove its tool_result.
	// Build a map of kept tool_use IDs to check against.
	keptToolUseIDs := make(map[string]bool)
	for _, msg := range filtered {
		for _, block := range msg.Content {
			if block.Type == llm.ContentTypeToolUse {
				keptToolUseIDs[block.ToolUseID] = true
			}
		}
	}

	// Remove any tool_result that references a removed tool_use
	cleanFiltered := make([]llm.Message, 0, len(filtered))
	for _, msg := range filtered {
		newContent := make([]llm.ContentBlock, 0, len(msg.Content))
		for _, block := range msg.Content {
			if block.Type == llm.ContentTypeToolResult {
				// Only keep if the corresponding tool_use is also kept
				if keptToolUseIDs[block.ToolResultID] {
					newContent = append(newContent, block)
				}
			} else {
				newContent = append(newContent, block)
			}
		}
		// Only add message if it has content (tool_result-only messages may become empty)
		if len(newContent) > 0 {
			msg.Content = newContent
			cleanFiltered = append(cleanFiltered, msg)
		}
	}

	// Use cleanFiltered instead of filtered from here on
	filtered = cleanFiltered

	// Check if we're under threshold now
	if totalTokens(filtered) <= threshold {
		a.mu.Lock()
		a.messages = filtered
		a.mu.Unlock()
		fmt.Printf("Compaction: done after scoring, reduced to %d tokens\n", totalTokens(filtered))
		return nil
	}

	// Step 3: Ask model to compress, keeping verbatim back to last user message
	fmt.Printf("Compaction: still over threshold, attempting compression\n")
	lastUserIdx := findLastUserMessage(filtered)
	if lastUserIdx > 0 {
		// Keep messages from last user message onwards verbatim
		verbatim := filtered[lastUserIdx:]
		compressible := filtered[:lastUserIdx]
		fmt.Printf("Compaction: compressing %d messages, keeping %d verbatim\n", len(compressible), len(verbatim))

		compressed, err := a.compressMessages(ctx, compressible)
		if err != nil {
			return fmt.Errorf("compressing messages: %w", err)
		}

		result := append(compressed, verbatim...)
		a.mu.Lock()
		a.messages = result
		a.mu.Unlock()
		fmt.Printf("Compaction: done after compression, %d messages\n", len(result))
		return nil
	}

	// No user message found, just use filtered
	a.mu.Lock()
	a.messages = filtered
	a.mu.Unlock()
	fmt.Printf("Compaction: done (no user message found), %d messages\n", len(filtered))
	return nil
}

// findLastUserMessage returns the index of the last user message.
func findLastUserMessage(messages []llm.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == llm.RoleUser {
			return i
		}
	}
	return -1
}

// scoreMessages asks the model to score each message in the conversation.
func (a *Agent) scoreMessages(ctx context.Context, messages []llm.Message) ([]string, error) {
	// Build a prompt that lists all messages and asks for scores
	var sb strings.Builder
	sb.WriteString("Score each message in the following conversation as one of: insignificant, low, medium, high.\n")
	sb.WriteString("Respond with a JSON array of scores, one per message in order.\n\n")
	sb.WriteString("Scoring guidelines:\n")
	sb.WriteString("- insignificant: Purely conversational, no task-relevant info\n")
	sb.WriteString("- low: Minor context, could be reconstructed from later messages\n")
	sb.WriteString("- medium: Important context, decisions or information\n")
	sb.WriteString("- high: Critical context, cannot be reconstructed\n\n")
	sb.WriteString("Messages:\n")

	for i, msg := range messages {
		fmt.Fprintf(&sb, "%d. [%s] ", i+1, msg.Role)
		for _, block := range msg.Content {
			switch block.Type {
			case llm.ContentTypeText:
				text := block.Text
				if len(text) > 200 {
					text = text[:200] + "..."
				}
				sb.WriteString(text)
			case llm.ContentTypeThinking:
				sb.WriteString("[thinking]")
			case llm.ContentTypeToolUse:
				fmt.Fprintf(&sb, "[tool: %s]", block.ToolUseName)
			case llm.ContentTypeToolResult:
				sb.WriteString("[tool result]")
			}
		}
		sb.WriteString("\n")
	}

	// Send to model
	req := llm.Request{
		Model:     a.model,
		System:    "You are a helpful assistant that analyzes conversation history.",
		Messages:  []llm.Message{llm.TextMessage(llm.RoleUser, sb.String())},
		MaxTokens: 8192,
		// Disable thinking for scoring to save tokens
		Thinking: llm.ThinkingConfig{Mode: llm.ThinkingModeDisabled},
	}

	resp, err := a.provider.Send(ctx, req)
	if err != nil {
		return nil, err
	}

	// Parse response - look for JSON array
	var scores []string
	for _, block := range resp.Message.Content {
		if block.Type == llm.ContentTypeText {
			// Try to extract JSON array from response
			text := strings.TrimSpace(block.Text)
			// Find JSON array in response
			start := strings.Index(text, "[")
			end := strings.LastIndex(text, "]")
			if start >= 0 && end > start {
				jsonStr := text[start : end+1]
				if err := json.Unmarshal([]byte(jsonStr), &scores); err == nil {
					return scores, nil
				}
			}
		}
	}

	// Fallback: if we can't parse, assume all medium
	scores = make([]string, len(messages))
	for i := range scores {
		scores[i] = "medium"
	}
	return scores, nil
}

// compressMessages asks the model to compress older messages.
func (a *Agent) compressMessages(ctx context.Context, messages []llm.Message) ([]llm.Message, error) {
	var sb strings.Builder
	sb.WriteString("Compress the following conversation history into a concise summary.\n")
	sb.WriteString("Preserve all important information, decisions, and context.\n")
	sb.WriteString("Respond with a single user message containing the summary.\n\n")
	sb.WriteString("Messages to compress:\n")

	for _, msg := range messages {
		fmt.Fprintf(&sb, "[%s] ", msg.Role)
		for _, block := range msg.Content {
			switch block.Type {
			case llm.ContentTypeText:
				sb.WriteString(block.Text)
			case llm.ContentTypeThinking:
				sb.WriteString("[thinking]")
			case llm.ContentTypeToolUse:
				fmt.Fprintf(&sb, "[tool: %s %s]", block.ToolUseName, block.ToolUseInput)
			case llm.ContentTypeToolResult:
				fmt.Fprintf(&sb, "[result: %s]", block.ToolResultContent)
			}
		}
		sb.WriteString("\n")
	}

	// Ask model to compress
	req := llm.Request{
		Model:     a.model,
		System:    "You are a helpful assistant that summarizes conversation history.",
		Messages:  []llm.Message{llm.TextMessage(llm.RoleUser, sb.String())},
		MaxTokens: 4096,
		Thinking:  llm.ThinkingConfig{Mode: llm.ThinkingModeDisabled},
	}

	resp, err := a.provider.Send(ctx, req)
	if err != nil {
		return nil, err
	}

	// Return the summary as a single assistant message
	if len(resp.Message.Content) > 0 {
		return []llm.Message{resp.Message}, nil
	}

	// If no content, return empty
	return nil, nil
}