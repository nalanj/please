// Package stream provides streaming output handling for the please CLI.
package stream

import (
	"github.com/charmbracelet/lipgloss"
)

// ThoughtStyle renders thinking output in italic and lighter/fainter.
var ThoughtStyle = lipgloss.NewStyle().
	Italic(true).
	Faint(true)

// InfoStyle renders the end-of-turn info line in faint.
var InfoStyle = lipgloss.NewStyle().Faint(true)
