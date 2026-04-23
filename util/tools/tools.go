// Package tools provides LLM-callable tools for the agent.
package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/nalanj/please/ops/agent/takeTurn"
)

// BashTimeout is the maximum time a bash command may run before being killed.
// Exported so tests can adjust it.
var BashTimeout = 30 * time.Second

// Bash is the bash execution tool.
var Bash = takeTurn.Tool{
	Name:        "bash",
	Description: "Run a bash command and return its combined stdout and stderr. Non-zero exit codes are reported at the end of the output.",
	InputSchema: json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Bash command to execute."}},"required":["command"]}`),
	Handler:     bashHandler,
}

func bashHandler(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if params.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	ctx, cancel := context.WithTimeout(ctx, BashTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", params.Command)
	out, err := cmd.CombinedOutput()
	result := string(out)

	if ctx.Err() == context.DeadlineExceeded {
		if result != "" && !strings.HasSuffix(result, "\n") {
			result += "\n"
		}
		return result + "[killed: command exceeded timeout]", nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if result != "" && !strings.HasSuffix(result, "\n") {
				result += "\n"
			}
			return result + fmt.Sprintf("[exit code %d]", exitErr.ExitCode()), nil
		}
		return "", err
	}
	return result, nil
}

// Find is the file search tool.
var Find = takeTurn.Tool{
	Name: "find",
	Description: "Find files in the current directory tree using glob patterns. " +
		"Supports ** for recursive matching (e.g. **/*.go, cmd/**, *.md). " +
		"The .git directory is excluded by default unless the ignore list is overridden.",
	InputSchema: json.RawMessage(`{
		"type":"object",
		"properties":{
			"pattern":{"type":"string","description":"Glob pattern matched against file paths."},
			"ignore":{"type":"array","items":{"type":"string"},"description":"Glob patterns for paths to exclude."}
		},
		"required":["pattern"]
	}`),
	Handler: findHandler,
}

func findHandler(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Pattern string   `json:"pattern"`
		Ignore  []string `json:"ignore"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if params.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	ignorePatterns := params.Ignore
	if len(ignorePatterns) == 0 {
		ignorePatterns = []string{".git"}
	}

	files, err := findFiles(ctx, params.Pattern, ignorePatterns)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "no files found", nil
	}
	return strings.Join(files, "\n"), nil
}

func findFiles(ctx context.Context, pattern string, ignorePatterns []string) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if isIgnoredPath(path, ignorePatterns) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		matched, err := doublestar.Match(pattern, filepath.ToSlash(path))
		if err != nil {
			return fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}

func isIgnoredPath(path string, patterns []string) bool {
	slashPath := filepath.ToSlash(path)
	parts := strings.Split(slashPath, "/")
	for _, pattern := range patterns {
		if ok, _ := doublestar.Match(pattern, slashPath); ok {
			return true
		}
		for _, part := range parts {
			if ok, _ := doublestar.Match(pattern, part); ok {
				return true
			}
		}
	}
	return false
}

// Read is the file reading tool.
var Read = takeTurn.Tool{
	Name: "read",
	Description: "Read a file and return its lines. Each line is prefixed with its " +
		"1-based line number, a 2-character hex hash, and a pipe: " +
		"\"{line}:{hash}|{text}\". Use start_line and end_line to read a specific range.",
	InputSchema: json.RawMessage(`{
		"type":"object",
		"properties":{
			"path":{"type":"string","description":"Path to the file to read."},
			"start_line":{"type":"integer","description":"First line to return, 1-based."},
			"end_line":{"type":"integer","description":"Last line to return, inclusive."}
		},
		"required":["path"]
	}`),
	Handler: readHandler,
}

func readHandler(_ context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		StartLine *int   `json:"start_line"`
		EndLine   *int   `json:"end_line"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	start := 1
	if params.StartLine != nil {
		start = *params.StartLine
	}
	if start < 1 {
		return "", fmt.Errorf("start_line must be >= 1")
	}

	f, err := os.Open(params.Path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	lineNum := 1
	for scanner.Scan() {
		if params.EndLine != nil && lineNum > *params.EndLine {
			break
		}
		if lineNum >= start {
			line := scanner.Text()
			fmt.Fprintf(&sb, "%d:%s|%s\n", lineNum, lineHashHex(line), line)
		}
		lineNum++
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading %s: %w", params.Path, err)
	}
	return sb.String(), nil
}

func lineHashHex(line string) string {
	h := fnv.New32a()
	h.Write([]byte(line))
	return fmt.Sprintf("%02x", h.Sum32()&0xff)
}

// WriteOp represents a single line-level edit operation.
type WriteOp struct {
	Op      string `json:"op"`
	Line    int    `json:"line"`
	Hash    string `json:"hash"`
	Content string `json:"content"`
}

// WriteFile is the file editing tool.
var WriteFile = takeTurn.Tool{
	Name: "write_file",
	Description: "Apply one or more line-level edits to a file. " +
		"op must be one of: \"replace\", \"insert_before\", \"insert_after\". " +
		"All operations are validated before any write occurs.",
	InputSchema: json.RawMessage(`{
		"type":"object",
		"properties":{
			"path":{"type":"string","description":"Path to the file to modify."},
			"operations":{"type":"array","minItems":1,"items":{"type":"object","properties":{"op":{"type":"string","enum":["replace","insert_before","insert_after"]},"line":{"type":"integer"},"hash":{"type":"string"},"content":{"type":"string"}},"required":["op","line","hash","content"]}}
		},
		"required":["path","operations"]
	}`),
	Handler: writeFileHandler,
}

func writeFileHandler(_ context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path       string    `json:"path"`
		Operations []WriteOp `json:"operations"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if len(params.Operations) == 0 {
		return "", fmt.Errorf("at least one operation is required")
	}

	for i, op := range params.Operations {
		switch op.Op {
		case "replace", "insert_before", "insert_after":
		default:
			return "", fmt.Errorf("operation %d: unknown op %q", i, op.Op)
		}
	}

	info, err := os.Stat(params.Path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(params.Path)
	if err != nil {
		return "", err
	}

	hasTrailingNewline := len(data) > 0 && data[len(data)-1] == '\n'
	original := strings.Split(string(data), "\n")
	if hasTrailingNewline && len(original) > 0 && original[len(original)-1] == "" {
		original = original[:len(original)-1]
	}

	// Validate every operation
	seen := make(map[int]bool, len(params.Operations))
	for i, op := range params.Operations {
		if op.Line < 1 || op.Line > len(original) {
			return "", fmt.Errorf("operation %d: line %d out of range (file has %d lines)",
				i, op.Line, len(original))
		}
		if seen[op.Line] {
			return "", fmt.Errorf("operation %d: line %d is targeted by multiple operations", i, op.Line)
		}
		seen[op.Line] = true
		if want := lineHashHex(original[op.Line-1]); op.Hash != want {
			return "", fmt.Errorf("operation %d: hash mismatch on line %d (want %s, got %s)",
				i, op.Line, want, op.Hash)
		}
	}

	opMap := make(map[int]WriteOp, len(params.Operations))
	for _, op := range params.Operations {
		opMap[op.Line] = op
	}

	result := make([]string, 0, len(original)+len(params.Operations))
	for i, line := range original {
		n := i + 1
		op, ok := opMap[n]
		if !ok {
			result = append(result, line)
			continue
		}
		content := strings.Split(op.Content, "\n")
		switch op.Op {
		case "replace":
			result = append(result, content...)
		case "insert_before":
			result = append(result, content...)
			result = append(result, line)
		case "insert_after":
			result = append(result, line)
			result = append(result, content...)
		}
	}

	out := strings.Join(result, "\n")
	if hasTrailingNewline {
		out += "\n"
	}
	if err := os.WriteFile(params.Path, []byte(out), info.Mode()); err != nil {
		return "", fmt.Errorf("writing %s: %w", params.Path, err)
	}
	return fmt.Sprintf("applied %d operation(s) to %s", len(params.Operations), params.Path), nil
}

// All returns the standard set of tools.
func All() []takeTurn.Tool {
	return []takeTurn.Tool{Bash, Find, Read, WriteFile}
}