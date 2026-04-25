// Package ansi provides minimal ANSI escape sequence utilities.
// Replaces lipgloss for terminal styling with direct control.
package ansi

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// ANSI escape prefix
const esc = "\x1b["

// Reset codes
const Reset = esc + "0m"

// Style codes
const (
	Bold      = esc + "1m"
	Dim       = esc + "2m"
	Italic    = esc + "3m"
	Underline = esc + "4m"
)

// Foreground colors (Nord palette)
const (
	FgDefault = esc + "39m"
	FgBlack   = esc + "30m"
	FgRed     = esc + "31m"
	FgGreen   = esc + "32m"
	FgYellow  = esc + "33m"
	FgBlue    = esc + "34m"
	FgMagenta = esc + "35m"
	FgCyan    = esc + "36m"
	FgWhite   = esc + "37m"
)

// Nord palette - 256 color foreground
const (
	FgNord0  = esc + "38;5;235m" // #2E3440 - dark background
	FgNord1  = esc + "38;5;235m"
	FgNord4  = esc + "38;5;188m" // #D8DEE9 - light gray
	FgNord5  = esc + "38;5;186m" // #E5E9F0
	FgNord6  = esc + "38;5;223m" // #ECEFF4 - white-ish
	FgNord7  = esc + "38;5;188m"
	FgNord8  = esc + "38;5;102m" // #8FBCBB - dark cyan
	FgNord9  = esc + "38;5;160m" // #BF616A - red
	FgNord10 = esc + "38;5;114m" // #A3BE8C - green
	FgNord11 = esc + "38;5;172m" // #EBCB8B - yellow
	FgNord12 = esc + "38;5;67m"  // #81A1C1 - blue
	FgNord13 = esc + "38;5;175m" // #B48EAD - magenta
	FgNord14 = esc + "38;5;109m" // #88C0D0 - cyan
)

// Background colors (Nord palette)
const (
	BgDefault = esc + "49m"
	BgBlack   = esc + "40m"
	BgRed     = esc + "41m"
	BgGreen   = esc + "42m"
	BgYellow  = esc + "43m"
	BgBlue    = esc + "44m"
	BgMagenta = esc + "45m"
	BgCyan    = esc + "46m"
	BgWhite   = esc + "47m"
)

// Nord palette - 256 color background
const (
	BgNord0 = esc + "48;5;235m" // #2E3440 - dark background
	BgNord1 = esc + "48;5;235m"
)

// colorEnabled tracks whether colors are enabled.
var colorEnabled = true

func init() {
	colorEnabled = supportsColor()
}

// supportsColor returns true if the terminal supports color output.
func supportsColor() bool {
	// Check NO_COLOR environment variable
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}

	// Check TERM environment variable
	termEnv := os.Getenv("TERM")
	if termEnv == "" || termEnv == "dumb" {
		return false
	}

	// Check if stderr is a terminal
	if !term.IsTerminal(2) {
		return false
	}

	return true
}

// IsColorEnabled returns whether color output is enabled.
func IsColorEnabled() bool {
	return colorEnabled
}

// Style applies ANSI styles to text. Pass zero or more style codes.
// Example: Style(text, Bold, FgNord12)
func Style(text string, codes ...string) string {
	if !colorEnabled || text == "" {
		return text
	}
	var b strings.Builder
	b.WriteString(strings.Join(codes, ""))
	b.WriteString(text)
	b.WriteString(Reset)
	return b.String()
}

// Faint applies faint/dim styling to text.
func Faint(text string) string {
	return Style(text, Dim)
}

// Wrap creates a padded string that fills width with spaces.
// Uses ANSI-compatible length calculation.
func Wrap(text string, width int) string {
	// Strip ANSI codes for length calculation
	ansiLen := 0
	inEscape := false
	for _, c := range text {
		if inEscape {
			if c == 'm' {
				inEscape = false
			}
			continue
		}
		if c == '\x1b' {
			inEscape = true
			continue
		}
		ansiLen++
	}

	padding := width - ansiLen
	if padding < 0 {
		padding = 0
	}

	if !colorEnabled {
		return text + strings.Repeat(" ", padding)
	}

	var b strings.Builder
	b.WriteString(text)
	for i := 0; i < padding; i++ {
		b.WriteRune(' ')
	}
	return b.String()
}
