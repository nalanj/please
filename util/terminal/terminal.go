// Package terminal provides terminal utilities.
package terminal

import (
	"os"
	"strconv"

	"golang.org/x/term"
)

// Width returns the terminal width, defaulting to 80 if unavailable.
func Width() int {
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
