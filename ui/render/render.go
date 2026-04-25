// Package render provides shared UI rendering for tool output.
package render

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nalanj/please/util/terminal"
)

// Styling
var (
	ToolBgStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#2E3440")).
			Foreground(lipgloss.Color("#D8DEE9"))

	ToolResultStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5C07B"))

	ToolErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E06C75"))
)

// TerminalWidth returns the terminal width, defaulting to 80.
func TerminalWidth() int {
	return terminal.Width()
}

// FormatToolInput creates a human-readable summary of tool input.
func FormatToolInput(name, input string) string {
	if input == "" {
		return ""
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(input), &m); err != nil {
		return input
	}

	switch name {
	case "bash":
		if cmd, ok := m["command"].(string); ok {
			if len(cmd) > 50 {
				return cmd[:50] + "..."
			}
			return cmd
		}
	case "find":
		if pattern, ok := m["pattern"].(string); ok {
			return pattern
		}
	case "read":
		if path, ok := m["path"].(string); ok {
			if start, ok := m["start_line"].(float64); ok {
				if end, ok := m["end_line"].(float64); ok {
					return path + ":" + strconv.Itoa(int(start)) + "-" + strconv.Itoa(int(end))
				}
				return path + ":" + strconv.Itoa(int(start)) + "+"
			}
			return path
		}
	case "write_file":
		if path, ok := m["path"].(string); ok {
			if ops, ok := m["operations"].([]any); ok {
				return path + " (" + strconv.Itoa(len(ops)) + " ops)"
			}
			return path
		}
	}

	return input
}
// RenderToolCall renders a tool call block:
//   - Title line: "[tool_name] [input]"
//   - Buffered output lines
//   - Result line
func RenderToolCall(name, input, output, result string) {
	width := TerminalWidth()

	// Blank line above (default background)
	os.Stderr.Write([]byte("\n"))

	// Styled block
	// Margin above
	os.Stderr.Write([]byte(ToolBgStyle.Width(width).Render("")))
	os.Stderr.Write([]byte("\n"))

	// Title with formatted input (write_file shows details below)
	title := " " + name
	if name != "write_file" && input != "" {
		formatted := FormatToolInput(name, input)
		if formatted != "" {
			title += " " + formatted
		}
	}
	title += " "
	RenderLine(title, width)

	// Blank line between title and output
	os.Stderr.Write([]byte(ToolBgStyle.Width(width).Render("")))
	os.Stderr.Write([]byte("\n"))

	// Output lines
	switch name {
	case "read":
		formatReadOutput(output, width)
	case "write_file":
		formatWriteOutput(input, width)
	default:
		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			if line != "" {
				RenderLine(" "+line+" ", width)
			}
		}
	}

	// Result
	RenderResultLine(" "+result+" ", width)

	// Margin below
	os.Stderr.Write([]byte(ToolBgStyle.Width(width).Render("")))
	os.Stderr.Write([]byte("\n"))

	// Blank line below (default background)
	os.Stderr.Write([]byte("\n"))
}

// RenderToolError renders a tool error block:
func RenderToolError(name, input, output, errMsg string) {
	width := TerminalWidth()

	// Blank line above (default background)
	os.Stderr.Write([]byte("\n"))

	// Styled block
	// Margin above
	os.Stderr.Write([]byte(ToolBgStyle.Width(width).Render("")))
	os.Stderr.Write([]byte("\n"))

	// Title with formatted input
	title := " " + name
	if input != "" {
		formatted := FormatToolInput(name, input)
		if formatted != "" {
			title += " " + formatted
		}
	}
	title += " "
	RenderLine(title, width)

	// Blank line between title and output
	os.Stderr.Write([]byte(ToolBgStyle.Width(width).Render("")))
	os.Stderr.Write([]byte("\n"))

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			RenderLine(" "+line+" ", width)
		}
	}

	RenderErrorLine(" "+errMsg+" ", width)

	// Margin below
	os.Stderr.Write([]byte(ToolBgStyle.Width(width).Render("")))
	os.Stderr.Write([]byte("\n"))

	// Blank line below (default background)
	os.Stderr.Write([]byte("\n"))
}

// RenderLine renders a line with the tool background and padding.
func RenderLine(text string, width int) {
	padding := width - len(text)
	if padding < 0 {
		padding = 0
	}
	os.Stderr.Write([]byte(ToolBgStyle.Width(width).Render(text)))
	os.Stderr.Write([]byte("\n"))
}

// RenderResultLine renders a result line with nested styling.
func RenderResultLine(text string, width int) {
	padding := width - len(text)
	if padding < 0 {
		padding = 0
	}
	os.Stderr.Write([]byte(ToolBgStyle.Width(width).Render(ToolResultStyle.Render(text))))
	os.Stderr.Write([]byte("\n"))
}

// RenderErrorLine renders an error line with nested styling.
func RenderErrorLine(text string, width int) {
	padding := width - len(text)
	if padding < 0 {
		padding = 0
	}
	os.Stderr.Write([]byte(ToolBgStyle.Width(width).Render(ToolErrorStyle.Render(text))))
	os.Stderr.Write([]byte("\n"))
}

// formatReadOutput formats read tool output as a table with line numbers.
// Input format: "{line}:{hash}|{content}"
func formatReadOutput(output string, width int) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return
	}

	// Parse all lines to find max line number width
	var parsedLines []struct {
		num     int
		content string
	}
	maxNumWidth := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Format is "{line}:{hash}|{content}"
		if idx := strings.Index(line, "|"); idx > 0 {
			beforeBar := line[:idx]
			content := line[idx+1:]
			if colonIdx := strings.Index(beforeBar, ":"); colonIdx > 0 {
				num, _ := strconv.Atoi(beforeBar[:colonIdx])
				parsedLines = append(parsedLines, struct {
					num     int
					content string
				}{num, content})
				if numWidth := len(beforeBar[:colonIdx]); numWidth > maxNumWidth {
					maxNumWidth = numWidth
				}
			} else {
				parsedLines = append(parsedLines, struct {
					num     int
					content string
				}{num: 0, content: line})
			}
		} else {
			parsedLines = append(parsedLines, struct {
				num     int
				content string
			}{num: 0, content: line})
		}
	}

	// Calculate max content width (leave 2 spaces for padding on each side)
	// Line format: " {line}  {content} "
	// So content max width = width - maxNumWidth - 4
	contentMaxWidth := width - maxNumWidth - 4
	if contentMaxWidth < 10 {
		contentMaxWidth = 10 // minimum content width
	}

	// Render as table, wrapping long content within the content column
	for _, pl := range parsedLines {
		// Wrap content to fit within contentMaxWidth
		wrappedLines := wrapText(pl.content, contentMaxWidth)
		for i, content := range wrappedLines {
			var lineStr string
			if i == 0 {
				// First line: include line number
				lineStr = fmt.Sprintf("%*d  %s", maxNumWidth, pl.num, content)
			} else {
				// Wrapped lines: indent to align with content
				indent := strings.Repeat(" ", maxNumWidth+2)
				lineStr = indent + content
			}
			RenderLine(" "+lineStr+" ", width)
		}
	}
}

// wrapText wraps text to fit within maxWidth, breaking on whitespace.
func wrapText(text string, maxWidth int) []string {
	if maxWidth < 1 {
		return []string{text}
	}
	if len(text) <= maxWidth {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	currentLine := ""

	for _, word := range words {
		if currentLine == "" {
			currentLine = word
		} else if len(currentLine)+1+len(word) <= maxWidth {
			currentLine += " " + word
		} else {
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// formatWriteOutput shows write_file operations in detail.
// Input format: JSON with path and operations
func formatWriteOutput(input string, width int) {
	var m map[string]any
	if err := json.Unmarshal([]byte(input), &m); err != nil {
		return
	}

	if ops, ok := m["operations"].([]any); ok {
		for i, opAny := range ops {
			if op, ok := opAny.(map[string]any); ok {
				opType, _ := op["op"].(string)
				line, _ := op["line"].(float64)
				content, _ := op["content"].(string)

				lineStr := fmt.Sprintf("%d. %s line %d", i+1, opType, int(line))
				RenderLine(" "+lineStr+" ", width)

				// Show content preview
				contentLines := strings.Split(content, "\n")
				for _, cl := range contentLines {
					if len(cl) > 40 {
						cl = cl[:40] + "..."
					}
					if cl != "" {
						RenderLine("   "+cl, width)
					}
				}
			}
		}
	}
}
