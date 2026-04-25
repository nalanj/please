package stream

import (
	"fmt"
	"strings"

	"github.com/nalanj/please/ui/markdown"
)

// FlushThreshold is the number of queued items at which we switch to instant flushing.
const FlushThreshold = 10

// OutputHandler manages streaming output with buffers for different content kinds.
type OutputHandler struct {
	md        *md.Renderer
	textBuf   strings.Builder
	thinkBuf  strings.Builder
	prevKind  string
}

// New creates a new OutputHandler with the given md renderer.
func New(md *md.Renderer) *OutputHandler {
	return &OutputHandler{
		md: md,
	}
}

// Handle processes a chunk of output of the given kind.
// kind is one of: "text", "thinking", "tool_start", "tool_result", "flush".
func (h *OutputHandler) Handle(kind, content string) string {
	return h.handleChunk(kind, content, false)
}

// handleChunk processes a chunk, returning styled output.
func (h *OutputHandler) handleChunk(kind, content string, instant bool) string {
	// Check for section switch
	if h.prevKind != "" && h.prevKind != kind {
		h.switchSection()
	}
	h.prevKind = kind

	switch kind {
	case "text":
		return h.processText(content, instant)
	case "thinking":
		return h.processThinking(content, instant)
	case "tool_start", "tool_result":
		h.switchSection()
		return ""
	case "flush":
		return h.finalFlush(true)
	default:
		return ""
	}
}

// SwitchSection signals a section boundary, flushing buffers and resetting state.
func (h *OutputHandler) SwitchSection() {
	h.switchSection()
}

// switchSection performs the actual section switch logic.
func (h *OutputHandler) switchSection() {
	// Flush text buffer
	if h.textBuf.Len() > 0 {
		fmt.Print(h.md.Write(h.textBuf.String()))
		h.textBuf.Reset()
	}
	// Flush thinking buffer
	if h.thinkBuf.Len() > 0 {
		mdRendered := h.md.Write(h.thinkBuf.String())
		fmt.Print(ThoughtStyle.Render(mdRendered))
		h.thinkBuf.Reset()
	}
	// Reset md renderer state for new context
	h.md.Reset()
}

// FinalFlush does a final flush of all buffers and returns the output.
func (h *OutputHandler) FinalFlush() string {
	return h.finalFlush(true)
}

// finalFlush performs the actual final flush.
func (h *OutputHandler) finalFlush(instant bool) string {
	var sb strings.Builder

	// Flush text buffer
	if h.textBuf.Len() > 0 {
		sb.WriteString(h.md.Write(h.textBuf.String()))
		h.textBuf.Reset()
	}

	// Flush thinking buffer
	if h.thinkBuf.Len() > 0 {
		mdRendered := h.md.Write(h.thinkBuf.String())
		sb.WriteString(ThoughtStyle.Render(mdRendered))
		h.thinkBuf.Reset()
	}

	// Flush md renderer for any remaining content
	if remaining := h.md.Flush(); remaining != "" {
		sb.WriteString(remaining)
	}

	return sb.String()
}

// isWordBoundary returns true if r is a word boundary character.
func isWordBoundary(r rune) bool {
	return r == ' ' || r == '\n' || r == '\t' || r == '.' || r == ',' || r == '!' || r == '?' || r == ';' || r == ':'
}

// processText handles text content with word-boundary flushing.
func (h *OutputHandler) processText(content string, instant bool) string {
	var result strings.Builder

	for _, r := range content {
		h.textBuf.WriteRune(r)
		if isWordBoundary(r) {
			result.WriteString(h.md.Write(h.textBuf.String()))
			h.textBuf.Reset()
		}
	}

	return result.String()
}

// processThinking handles thinking content with word-boundary flushing.
// Content is rendered through markdown first, then styled with ThoughtStyle.
func (h *OutputHandler) processThinking(content string, instant bool) string {
	var result strings.Builder

	for _, r := range content {
		h.thinkBuf.WriteRune(r)
		if isWordBoundary(r) {
			mdRendered := h.md.Write(h.thinkBuf.String())
			result.WriteString(ThoughtStyle.Render(mdRendered))
			h.thinkBuf.Reset()
		}
	}

	return result.String()
}
