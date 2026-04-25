package stream

import (
	"github.com/charmbracelet/lipgloss"
)

// ThoughtStyleANSI contains the raw ANSI codes for thinking style.
// Using ANSI directly avoids lipgloss adding trailing whitespace on multi-line content.
const (
	ThoughtStyleANSI      = "\x1b[3;2m" // italic + faint
	ThoughtStyleANSIRest  = "\x1b[0m"  // reset
)

// ThoughtStyle is kept for compatibility but should not be used for rendering
// streaming content due to lipgloss adding trailing whitespace on multi-line content.
var ThoughtStyle = lipgloss.NewStyle().
	Italic(true).
	Faint(true)

// InfoStyle renders the end-of-turn info line in faint.
var InfoStyle = lipgloss.NewStyle().Faint(true)
