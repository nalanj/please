package completion

import (
	"fmt"
	"io"
)

var supportedShells = []string{"bash", "zsh", "fish", "powershell"}

func Generate(w io.Writer, shell string) error {
	if shell == "" {
		return fmt.Errorf("shell required: bash, zsh, fish, or powershell")
	}

	switch shell {
	case "bash":
		return bashCompletion(w)
	case "zsh":
		return zshCompletion(w)
	case "fish":
		return fishCompletion(w)
	case "powershell":
		return powershellCompletion(w)
	default:
		return fmt.Errorf("unsupported shell: %s (supported: %v)", shell, supportedShells)
	}
}
