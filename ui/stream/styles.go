// Package stream provides streaming output handling for the please CLI.
package stream

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// init sets the color profile to ANSI to ensure styling works in non-TTY
// environments (tests, pipes). This is safe because:
// - ThoughtStyle uses only basic attributes (italic, faint) supported by ANSI
// - Other packages use RawANSI renderer which bypasses color profile
func init() {
	// Only set if no color profile has been detected yet
	if termenv.ColorProfile() == termenv.Ascii {
		lipgloss.SetColorProfile(termenv.ANSI)
	}
}

// ThoughtStyle renders thinking output in italic and lighter/fainter.
var ThoughtStyle = lipgloss.NewStyle().
	Italic(true).
	Faint(true)

// InfoStyle renders the end-of-turn info line in faint.
var InfoStyle = lipgloss.NewStyle().Faint(true)
