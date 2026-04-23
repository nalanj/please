package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nalanj/please/ui/render"
	"github.com/nalanj/please/util/tools"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ctx := context.Background()

	switch os.Args[1] {
	case "tool-bash":
		result, err := tools.Bash.Handler(ctx, json.RawMessage(`{"command":"ls"}`))
		if err != nil {
			render.RenderToolError("bash", `{"command":"ls"}`, "", err.Error())
		} else {
			render.RenderToolCall("bash", `{"command":"ls"}`, result, "→ done")
		}

	case "tool-find":
		result, err := tools.Find.Handler(ctx, json.RawMessage(`{"pattern":"*.yml"}`))
		if err != nil {
			render.RenderToolError("find", `{"pattern":"*.yml"}`, "", err.Error())
		} else {
			render.RenderToolCall("find", `{"pattern":"*.yml"}`, result, "→ done")
		}

	case "tool-read":
		result, err := tools.Read.Handler(ctx, json.RawMessage(`{"path":"ui/render/render.go"}`))
		if err != nil {
			render.RenderToolError("read", `{"path":"ui/render/render.go"}`, "", err.Error())
		} else {
			render.RenderToolCall("read", `{"path":"ui/render/render.go"}`, result, "→ done")
		}

	case "tool-write":
		// Read current file content to get the hash
		readResult, err := tools.Read.Handler(ctx, json.RawMessage(`{"path":"/tmp/please-demo.txt"}`))
		if err != nil {
			render.RenderToolError("write_file", `{"path":"/tmp/please-demo.txt"}`, "", err.Error())
			return
		}
		// Parse first line hash from read output (format: "line:hash|content")
		lines := strings.Split(strings.TrimSpace(readResult), "\n")
		var firstHash string
		for _, line := range lines {
			if idx := strings.Index(line, "|"); idx > 0 {
				beforeBar := line[:idx]
				if colonIdx := strings.Index(beforeBar, ":"); colonIdx > 0 {
					firstHash = beforeBar[colonIdx+1:]
					break
				}
			}
		}

		inputJSON := fmt.Sprintf(`{"path":"/tmp/please-demo.txt","operations":[{"op":"replace","line":1,"hash":"%s","content":"Updated by the UI demo tool write."}]}`, firstHash)

		result, err := tools.WriteFile.Handler(ctx, json.RawMessage(inputJSON))
		if err != nil {
			render.RenderToolError("write_file", inputJSON, "", err.Error())
		} else {
			render.RenderToolCall("write_file", inputJSON, result, "→ done")
		}

	case "tool-error":
		result, err := tools.Bash.Handler(ctx, json.RawMessage(`{"command":"cat missing.txt"}`))
		if err != nil {
			render.RenderToolError("bash", `{"command":"cat missing.txt"}`, result, err.Error())
		} else {
			render.RenderToolCall("bash", `{"command":"cat missing.txt"}`, result, "→ done")
		}

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: ui <sample>")
	fmt.Fprintln(os.Stderr, "Samples:")
	fmt.Fprintln(os.Stderr, "  tool-bash    - bash tool with file output")
	fmt.Fprintln(os.Stderr, "  tool-find    - find tool with file output")
	fmt.Fprintln(os.Stderr, "  tool-read    - read tool with file content")
	fmt.Fprintln(os.Stderr, "  tool-write   - write_file tool success")
	fmt.Fprintln(os.Stderr, "  tool-error   - error result")
}