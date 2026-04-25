package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nalanj/please/session"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, w io.Writer) error {
	var sessionPath string
	var opts session.ReplayOptions

	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--"):
			// flag
			parts := strings.SplitN(arg[2:], "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid flag: %s", arg)
			}
			switch parts[0] {
			case "instant":
				opts.Instant = true
			case "scale":
				fmt.Sscanf(parts[1], "%f", &opts.Scale)
			case "filter":
				opts.Filter = parts[1]
			case "from-turn":
				fmt.Sscanf(parts[1], "%d", &opts.FromTurn)
			default:
				return fmt.Errorf("unknown flag: --%s", parts[0])
			}
		default:
			sessionPath = arg
		}
	}

	if sessionPath == "" {
		return fmt.Errorf("usage: please replay <session> [--instant] [--scale=1.0] [--filter=type] [--from-turn=N]")
	}

	// Resolve session path
	if !strings.HasSuffix(sessionPath, ".jsonl") {
		sessionPath = sessionPath + ".jsonl"
	}
	if !strings.Contains(sessionPath, "/") {
		sessionPath = filepath.Join(".please", "sessions", sessionPath)
	}

	r := session.NewReader(sessionPath)
	turns, err := r.Load()
	if err != nil {
		return fmt.Errorf("loading session: %w", err)
	}

	// Manual iteration to avoid r.Replay issues
	for turnIdx := 0; turnIdx < len(turns); turnIdx++ {
		// Skip to fromTurn if specified
		if opts.FromTurn > 0 && turnIdx < opts.FromTurn-1 {
			continue
		}
		
		turn := &turns[turnIdx]
		for eventIdx := 0; eventIdx < len(turn.Events); eventIdx++ {
			evt := &turn.Events[eventIdx]
			
			// Apply filter
			if opts.Filter != "" && evt.Type != opts.Filter {
				continue
			}
			
			// Output the event
			fmt.Fprintf(w, "%#v\n", evt)
		}
	}

	return nil
}
