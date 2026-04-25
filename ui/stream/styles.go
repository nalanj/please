package stream

import "github.com/nalanj/please/util/ansi"

// ThoughtStyleANSI contains the raw ANSI codes for thinking style.
// Using ANSI directly avoids adding trailing whitespace on multi-line content.
const (
	ThoughtStyleANSI     = "\x1b[3;2m" // italic + faint
	ThoughtStyleANSIRest = "\x1b[0m"   // reset
)

// InfoStyleANSI contains the raw ANSI codes for info/faint styling.
const InfoStyleANSI = "\x1b[2m" // faint (dim)

// InfoStyle renders the end-of-turn info line in faint.
// Deprecated: Use ansi.Faint() instead.
var InfoStyle = struct {
	Render func(string) string
}{
	Render: func(s string) string {
		return ansi.Faint(s)
	},
}
