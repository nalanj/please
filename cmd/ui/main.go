package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	md "github.com/nalanj/please/ui/markdown"
	"github.com/nalanj/please/ui/render"
	"github.com/nalanj/please/ui/stream"
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
		readResult, err := tools.Read.Handler(ctx, json.RawMessage(`{"path":"/tmp/please-demo.txt"}`))
		if err != nil {
			render.RenderToolError("write_file", `{"path":"/tmp/please-demo.txt"}`, "", err.Error())
			return
		}
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

	case "markdown", "markdown-stream":
		// Streaming demo with timing
		streamMarkdown(true)

	case "markdown-fast":
		// Fast streaming demo
		streamMarkdown(false)

	case "thinking", "thinking-fast":
		// Thinking streaming demo
		streamThinking()

	default:
		printUsage()
		os.Exit(1)
	}
}

// streamMarkdown simulates streaming markdown output like an LLM would send it.
func streamMarkdown(slow bool) {
	r := md.New()

	type chunk struct {
		text  string
		delay time.Duration
	}
	chunks := []chunk{
		{"# Hello World", 80 * time.Millisecond},
		{"\n\n", 50 * time.Millisecond},
		{"This is ", 30 * time.Millisecond},
		{"**bold**", 60 * time.Millisecond},
		{" and ", 40 * time.Millisecond},
		{"*italic*", 50 * time.Millisecond},
		{" text.\n\n", 70 * time.Millisecond},
		{"## Code Example\n\n", 90 * time.Millisecond},
		{"```go\nfunc main() {\n", 100 * time.Millisecond},
		{"    fmt.Println(`hello`)\n", 80 * time.Millisecond},
		{"}\n", 40 * time.Millisecond},
		{"```\n\n", 60 * time.Millisecond},
		{"- Item one\n", 50 * time.Millisecond},
		{"- Item two\n", 45 * time.Millisecond},
		{"- Item three\n", 55 * time.Millisecond},
	}

	for _, c := range chunks {
		output := r.Write(c.text)
		if output != "" {
			fmt.Print(output)
		}
		if slow {
			time.Sleep(c.delay)
		}
	}

	if remaining := r.Flush(); remaining != "" {
		fmt.Print(remaining)
	}
}

// streamThinking simulates streaming thinking output with markdown.
func streamThinking() {
	h := stream.New(md.New())

	// Thinking content with markdown elements
	thinkingContent := `Let me think about this...

## Analysis

I need to consider:
- **First point**: This is important
- *Second point*: Also matters

### Code Example

Here is some example code:

` + "```" + `python
def hello():
    print("world")
` + "```" + `

Let me wrap up my thinking.`

	// Stream the thinking content character by character with small delays
	// Using OutputHandler to properly handle thinking style
	for _, char := range thinkingContent {
		result := h.Handle("thinking", string(char))
		if result != "" {
			fmt.Print(result)
		}
		time.Sleep(15 * time.Millisecond)
	}

	// Final flush
	if final := h.FinalFlush(); final != "" {
		fmt.Print(final)
	}

	// Show a newline after thinking
	fmt.Println()
	fmt.Println()
	fmt.Print(lipgloss.NewStyle().Faint(true).Render("→ End of thinking"))
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: ui <sample>")
	fmt.Fprintln(os.Stderr, "Samples:")
	fmt.Fprintln(os.Stderr, "  tool-bash        - bash tool with file output")
	fmt.Fprintln(os.Stderr, "  tool-find        - find tool with file output")
	fmt.Fprintln(os.Stderr, "  tool-read        - read tool with file content")
	fmt.Fprintln(os.Stderr, "  tool-write       - write_file tool success")
	fmt.Fprintln(os.Stderr, "  tool-error       - error result")
	fmt.Fprintln(os.Stderr, "  markdown         - streaming markdown demo")
	fmt.Fprintln(os.Stderr, "  markdown-stream  - alias for markdown")
	fmt.Fprintln(os.Stderr, "  markdown-fast    - fast streaming without delays")
	fmt.Fprintln(os.Stderr, "  thinking         - streaming thinking with markdown (slow)")
	fmt.Fprintln(os.Stderr, "  thinking-fast    - fast thinking demo")
}
