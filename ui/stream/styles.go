// Package stream provides streaming output handling for the please CLI.
package stream

import (
	"github.com/charmbracelet/lipgloss"
)

// ThoughtStyle renders thinking/thinking output in italic.
var ThoughtStyle = lipgloss.NewStyle().Italic(true)

// InfoStyle renders the end-of-turn info line in faint.
var InfoStyle = lipgloss.NewStyle().Faint(true)
