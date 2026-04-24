// Package md renders markdown with ANSI styling for streaming output.
package md

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Styles for markdown elements.
var (
	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#81A1C1")).
			Bold(true)

	BoldStyle = lipgloss.NewStyle().Bold(true)

	ItalicStyle = lipgloss.NewStyle().Italic(true)

	CodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A3BE8C")).
			Background(lipgloss.Color("#2E3440"))

	LinkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88C0D0")).
			Underline(true)

	ListStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D8DEE9"))
)

// Renderer handles streaming markdown rendering.
type Renderer struct {
	mu     sync.Mutex
	buffer strings.Builder
}

// New creates a new streaming markdown renderer.
func New() *Renderer {
	return &Renderer{}
}

// Write renders the incoming text chunk, applying markdown styling.
func (r *Renderer) Write(text string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.buffer.WriteString(text)

	buf := r.buffer.String()
	var result strings.Builder

	flushed := false
	for {
		before, styled, after := processMarkdown(buf)
		if styled == "" {
			if !hasPendingMarker(buf) {
				result.WriteString(buf)
				r.buffer.Reset()
				flushed = true
			}
			break
		}
		result.WriteString(before)
		result.WriteString(styled)
		buf = after
	}

	if !flushed {
		r.buffer.Reset()
		r.buffer.WriteString(buf)
	}

	return result.String()
}

// Flush returns any remaining buffered text without styling.
func (r *Renderer) Flush() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	flushed := r.buffer.String()
	r.buffer.Reset()
	return flushed
}

// hasPendingMarker checks if there's an incomplete markdown marker in text.
func hasPendingMarker(text string) bool {
	if strings.Count(text, "```")%2 != 0 {
		return true
	}
	if strings.Count(text, "**")%2 != 0 {
		return true
	}
	if !isInsideCodeBlock(text) {
		if strings.Count(text, "`")%2 != 0 {
			return true
		}
	}

	asteriskCount := 0
	inCodeBlock := false
	inBold := false
	i := 0
	for i < len(text) {
		if i+1 < len(text) && text[i] == '*' && text[i+1] == '*' {
			if !inCodeBlock {
				inBold = !inBold
			}
			i += 2
			continue
		}
		if text[i] == '*' && !inCodeBlock && !inBold {
			asteriskCount++
		}
		if i+2 < len(text) && text[i:i+3] == "```" {
			inCodeBlock = !inCodeBlock
		}
		i++
	}
	if asteriskCount%2 != 0 {
		return true
	}

	trimmed := strings.TrimLeft(text, " \t")
	if strings.HasPrefix(trimmed, "#") {
		if !strings.Contains(text, "\n") {
			return true
		}
	}

	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		if !strings.Contains(text, "\n") {
			return true
		}
	}

	// Ordered list items need the same check
	if isOrderedList(trimmed) {
		if !strings.Contains(text, "\n") {
			return true
		}
	}

	return false
}


// isInsideCodeBlock checks if we're inside an open code block.
func isInsideCodeBlock(text string) bool {
	return strings.Count(text, "```")%2 != 0
}

// isOrderedList checks if text starts with an ordered list item pattern (e.g., "1.", "2.", "10.").
func isOrderedList(text string) bool {
	trimmed := strings.TrimLeft(text, " \t")
	if trimmed == "" {
		return false
	}

	// Check for numbered list pattern: digits followed by ". "
	digits := 0
	for i := 0; i < len(trimmed) && i < 10; i++ {
		if trimmed[i] >= '0' && trimmed[i] <= '9' {
			digits++
		} else {
			break
		}
	}

	if digits == 0 {
		return false
	}

	afterDigits := trimmed[digits:]
	return strings.HasPrefix(afterDigits, ". ")
}


// isInsideCodeBlockAt checks if position in original text is inside a code block.
func isInsideCodeBlockAt(original string, pos int) bool {
	before := original[:pos]
	return strings.Count(before, "```")%2 != 0
}

// processMarkdown finds and processes complete markdown patterns.
func processMarkdown(text string) (before, styled, after string) {
	trimmed := strings.TrimLeft(text, " \t")
	if trimmed == "" {
		return "", "", text
	}
	prefixLen := len(text) - len(trimmed)

	// Code block ``` must be checked FIRST
	if strings.HasPrefix(trimmed, "```") {
		codeBlockEnd := findCodeBlockEnd(trimmed[3:])
		if codeBlockEnd >= 0 {
			content := trimmed[3 : 3+codeBlockEnd]
			styled = renderCodeBlock(content)
			remaining := trimmed[3+codeBlockEnd+3:]
			return "", styled, remaining
		}
	}

	// Headers (# to end of line)
	if strings.HasPrefix(trimmed, "#") && !isInsideCodeBlock(text) {
		level := 0
		for i := 0; i < len(trimmed) && i < 6; i++ {
			if trimmed[i] == '#' {
				level++
			} else if trimmed[i] == ' ' && level > 0 {
				break
			} else {
				return "", "", text
			}
		}
		if level > 0 && len(trimmed) > level {
			end := strings.Index(trimmed, "\n")
			if end < 0 {
				return "", "", text
			}
			headerText := trimmed[:end]
			if prefixLen > 0 {
				headerText = text[:prefixLen] + headerText
			}
			styled = ansiHeader + ansiBold + headerText + ansiReset
			return "", styled, text[end+prefixLen+1:]
		}
	}

	// Bold **text** - must not be inside code block
	if !isInsideCodeBlock(text) {
		idx := 0
		for {
			pos := strings.Index(text[idx:], "**")
			if pos < 0 {
				break
			}
			actualPos := idx + pos
			rest := text[actualPos+2:]
			endIdx := strings.Index(rest, "**")
			if endIdx >= 0 {
				boldText := rest[:endIdx]
				styled = ansiBold + boldText + ansiReset
				return text[:actualPos], styled, rest[endIdx+2:]
			}
			idx = actualPos + 2
		}
	}

	// Inline code `text` - must not be inside code block or adjacent to another backtick
	if !isInsideCodeBlock(text) {
		for i := 0; i < len(text); i++ {
			if text[i] == '`' {
				if i+2 < len(text) && text[i:i+3] == "```" {
					continue
				}
				if i > 0 && text[i-1] == '`' {
					continue
				}
				rest := text[i+1:]
				for j := 0; j < len(rest); j++ {
					if rest[j] == '`' {
						if j+1 < len(rest) && rest[j+1] == '`' {
							continue
						}
						codeText := rest[:j]
						styled = ansiCode + codeText + ansiReset
						return text[:i], styled, rest[j+1:]
					}
				}
				return "", "", text
			}
		}
	}

	// Italic *text* (but not **)
	if !isInsideCodeBlock(text) {
		for i := 0; i < len(text); i++ {
			if text[i] == '*' {
				if i+1 < len(text) && text[i+1] == '*' {
					continue
				}
				before := text[:i]
				if strings.Count(before, "**")%2 != 0 {
					continue
				}
				rest := text[i+1:]
				for j := 0; j < len(rest); j++ {
					if rest[j] == '*' {
						if j+1 < len(rest) && rest[j+1] == '*' {
							continue
						}
						between := rest[:j]
						if strings.Count(between, "**")%2 != 0 {
							continue
						}
						italicText := rest[:j]
						styled = ansiItalic + italicText + ansiReset
						return text[:i], styled, rest[j+1:]
					}
				}
				return "", "", text
			}
		}
	}

	// List items (- or * at start of line)
	if !isInsideCodeBlock(text) && (strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")) {
		end := strings.Index(trimmed, "\n")
		if end < 0 {
			return "", "", text
		}
		listText := trimmed[:end]
		if prefixLen > 0 {
			listText = text[:prefixLen] + listText
		}
		styled = ansiList + listText + "\n" + ansiReset
		afterStart := end + prefixLen + 1
		if afterStart < len(text) {
			return "", styled, text[afterStart:]
		}
		return "", styled, ""
	}
	// Ordered list items (1. 2. 10. etc.)
	if !isInsideCodeBlock(text) && isOrderedList(trimmed) {
		end := strings.Index(trimmed, "\n")
		if end < 0 {
			return "", "", text
		}
		listText := trimmed[:end]
		if prefixLen > 0 {
			listText = text[:prefixLen] + listText
		}
		styled = ansiList + listText + "\n" + ansiReset
		afterStart := end + prefixLen + 1
		if afterStart < len(text) {
			return "", styled, text[afterStart:]
		}
		return "", styled, ""
	}


	return "", "", text
}

// renderCodeBlock renders a code block with full-width background.
func renderCodeBlock(content string) string {
	width := terminalWidth()
	if width <= 0 {
		width = 80
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return ""
	}

	// Check if first line is a language spec
	lang := ""
	if len(lines) > 0 && len(lines[0]) > 0 && len(lines[0]) < 20 && !strings.Contains(lines[0], " ") {
		lang = lines[0]
	}

	// ANSI escape sequences - darker background (color index 235)
	codeBg := "\x1b[48;5;235m"
	codeFg := "\x1b[38;5;188m"
	codeLangFg := "\x1b[38;5;6m"
	boldOn := "\x1b[1m"
	reset := "\x1b[0m"

	var result strings.Builder

	// Top border line
	borderLine := codeBg
	for i := 0; i < width; i++ {
		borderLine += " "
	}
	borderLine += reset
	result.WriteString(borderLine + "\n")

	// Language spec line (if present)
	if lang != "" && len(lines) > 1 {
		langLine := " " + lang + " "
		padding := width - len(langLine)
		if padding < 0 {
			padding = 0
		}
		langLineStyled := codeBg + codeLangFg + boldOn + langLine
		for i := 0; i < padding; i++ {
			langLineStyled += " "
		}
		langLineStyled += reset
		result.WriteString(langLineStyled + "\n")
	}

	// Content lines
	startIdx := 0
	if lang != "" {
		startIdx = 1
	}

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		// Skip trailing empty lines
		if line == "" && i == len(lines)-1 {
			continue
		}

		lineWithPadding := " " + line + " "
		padding := width - len(lineWithPadding)
		if padding < 0 {
			padding = 0
		}

		contentLine := codeBg + codeFg + lineWithPadding
		for j := 0; j < padding; j++ {
			contentLine += " "
		}
		contentLine += reset
		result.WriteString(contentLine + "\n")
	}

	// Bottom border line
	borderLine = codeBg
	for i := 0; i < width; i++ {
		borderLine += " "
	}
	borderLine += reset
	result.WriteString(borderLine + "\n")

	return result.String()
}

// terminalWidth returns the terminal width.
func terminalWidth() int {
	width, _, err := term.GetSize(0)
	if err != nil || width <= 0 {
		if cols := os.Getenv("COLUMNS"); cols != "" {
			if w, err := strconv.Atoi(cols); err == nil && w > 0 {
				return w
			}
		}
		return 80
	}
	return width
}

// findCodeBlockEnd finds the position of closing ``` in content after opening ```.
func findCodeBlockEnd(content string) int {
	firstNewline := strings.Index(content, "\n")
	if firstNewline < 0 {
		closingIdx := strings.Index(content, "```")
		if closingIdx > 0 {
			return closingIdx
		}
		return -1
	}

	searchFrom := content[firstNewline+1:]

	searchIdx := 0
	for {
		pos := strings.Index(searchFrom[searchIdx:], "```")
		if pos < 0 {
			return -1
		}
		actualPos := searchIdx + pos

		if actualPos+3+3 <= len(searchFrom) {
			if searchFrom[actualPos+3:actualPos+6] == "```" {
				searchIdx = actualPos + 3
				continue
			}
		}

		return firstNewline + 1 + actualPos
	}
}

// ANSI style constants for inline elements
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiItalic = "\x1b[3m"
	ansiHeader = "\x1b[38;5;226m"  // Yellow
	ansiCode   = "\x1b[38;5;114m" // Light green
	ansiList   = "\x1b[38;5;188m" // Light gray
)
