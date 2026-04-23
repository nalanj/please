package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLineHashHex(t *testing.T) {
	// Just verify that hash function works and produces consistent results
	h1 := lineHashHex("hello")
	h2 := lineHashHex("hello")
	if h1 != h2 {
		t.Errorf("inconsistent hash for same input: %q != %q", h1, h2)
	}

	// Verify different inputs produce different hashes (in most cases)
	if lineHashHex("foo") == lineHashHex("bar") {
		t.Error("different inputs produced same hash")
	}

	// Verify hash length is 2 hex chars
	if len(lineHashHex("test")) != 2 {
		t.Error("expected 2-character hex hash")
	}
}

func TestLineHashHexConsistency(t *testing.T) {
	// Same input should always produce same output
	expected := lineHashHex("test string")
	for i := 0; i < 100; i++ {
		result := lineHashHex("test string")
		if result != expected {
			t.Errorf("inconsistent hash: expected %q, got %q", expected, result)
		}
	}
}

func TestLineHashHexDifferentInputs(t *testing.T) {
	h1 := lineHashHex("foo")
	h2 := lineHashHex("bar")
	h3 := lineHashHex("baz")
	if h1 == h2 || h2 == h3 || h1 == h3 {
		t.Error("expected different hashes for different inputs")
	}
}

func TestBashTool(t *testing.T) {
	if Bash.Name != "bash" {
		t.Errorf("expected name 'bash', got %q", Bash.Name)
	}
	if Bash.Description == "" {
		t.Error("expected non-empty description")
	}
	if Bash.InputSchema == nil {
		t.Error("expected non-nil input schema")
	}
	if Bash.Handler == nil {
		t.Error("expected non-nil handler")
	}
}

func TestBashHandlerSimple(t *testing.T) {
	input := json.RawMessage(`{"command":"echo hello"}`)
	result, err := bashHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", result)
	}
}

func TestBashHandlerExitCode(t *testing.T) {
	input := json.RawMessage(`{"command":"exit 42"}`)
	result, err := bashHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "[exit code 42]" {
		t.Errorf("expected '[exit code 42]', got %q", result)
	}
}

func TestBashHandlerStderr(t *testing.T) {
	input := json.RawMessage(`{"command":"echo error >&2; exit 1"}`)
	result, err := bashHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "error\n[exit code 1]" {
		t.Errorf("expected 'error\\n[exit code 1]', got %q", result)
	}
}

func TestBashHandlerTimeout(t *testing.T) {
	originalTimeout := BashTimeout
	BashTimeout = 50 * 1000 // 50ms
	defer func() { BashTimeout = originalTimeout }()

	input := json.RawMessage(`{"command":"sleep 10"}`)
	result, err := bashHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !contains(result, "timeout") {
		t.Errorf("expected 'timeout' in result, got %q", result)
	}
}

func TestBashHandlerMissingCommand(t *testing.T) {
	input := json.RawMessage(`{}`)
	_, err := bashHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestBashHandlerEmptyCommand(t *testing.T) {
	input := json.RawMessage(`{"command":""}`)
	_, err := bashHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestBashHandlerInvalidJSON(t *testing.T) {
	input := json.RawMessage(`{invalid}`)
	_, err := bashHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBashHandlerMultiLine(t *testing.T) {
	input := json.RawMessage(`{"command":"echo line1; echo line2; echo line3"}`)
	result, err := bashHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result, "line1") || !contains(result, "line2") || !contains(result, "line3") {
		t.Errorf("expected all lines in result, got %q", result)
	}
}

func TestBashHandlerWorkingDir(t *testing.T) {
	dir := t.TempDir()
	// Create a subprocess that changes directory
	cmd := "cd " + dir + " && pwd"
	input := json.RawMessage(`{"command":"` + cmd + `"}`)
	result, err := bashHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result, dir) {
		t.Errorf("expected %q in result, got %q", dir, result)
	}
}

func TestFindTool(t *testing.T) {
	if Find.Name != "find" {
		t.Errorf("expected name 'find', got %q", Find.Name)
	}
	if Find.Description == "" {
		t.Error("expected non-empty description")
	}
	if Find.InputSchema == nil {
		t.Error("expected non-nil input schema")
	}
	if Find.Handler == nil {
		t.Error("expected non-nil handler")
	}
}

func TestFindHandlerBasic(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	// Save current directory and restore after test
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"pattern":"*.txt"}`)
	result, err := findHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result, "test.txt") {
		t.Errorf("expected 'test.txt' in result, got %q", result)
	}
}

func TestFindHandlerNoMatch(t *testing.T) {
	input := json.RawMessage(`{"pattern":"*.nonexistent"}`)
	result, err := findHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "no files found" {
		t.Errorf("expected 'no files found', got %q", result)
	}
}

func TestFindHandlerRecurse(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	err := os.Mkdir(subDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(subDir, "deep.txt"), []byte("deep"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	// Save current directory and restore after test
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"pattern":"**/*.txt"}`)
	result, err := findHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result, "deep.txt") {
		t.Errorf("expected 'deep.txt' in result, got %q", result)
	}
}

func TestFindHandlerIgnore(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a .git directory
	gitDir := filepath.Join(tmpDir, ".git")
	err := os.Mkdir(gitDir, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(gitDir, "config"), []byte("git"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "normal.txt"), []byte("normal"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	// Save current directory and restore after test
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"pattern":"**/*.txt"}`)
	result, err := findHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result, "normal.txt") {
		t.Errorf("expected 'normal.txt' in result, got %q", result)
	}
	if contains(result, ".git") {
		t.Errorf("expected '.git' to be ignored, got %q", result)
	}
}

func TestFindHandlerMissingPattern(t *testing.T) {
	input := json.RawMessage(`{}`)
	_, err := findHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing pattern")
	}
}

func TestFindHandlerEmptyPattern(t *testing.T) {
	input := json.RawMessage(`{"pattern":""}`)
	_, err := findHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty pattern")
	}
}

func TestReadTool(t *testing.T) {
	if Read.Name != "read" {
		t.Errorf("expected name 'read', got %q", Read.Name)
	}
	if Read.Description == "" {
		t.Error("expected non-empty description")
	}
	if Read.InputSchema == nil {
		t.Error("expected non-nil input schema")
	}
	if Read.Handler == nil {
		t.Error("expected non-nil handler")
	}
}

func TestReadHandlerBasic(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"path":"` + testFile + `"}`)
	result, err := readHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result, "line1") || !contains(result, "line2") || !contains(result, "line3") {
		t.Errorf("expected all lines in result, got %q", result)
	}
}

func TestReadHandlerWithLineNumbers(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"path":"` + testFile + `"}`)
	result, err := readHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Each line should be prefixed with number:hash|
	if !contains(result, "1:") || !contains(result, "2:") || !contains(result, "3:") {
		t.Errorf("expected line numbers in result, got %q", result)
	}
}

func TestReadHandlerStartLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"path":"` + testFile + `","start_line":2}`)
	result, err := readHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result, "line2") || !contains(result, "line3") {
		t.Errorf("expected lines 2 and 3 in result, got %q", result)
	}
	if contains(result, "line1") {
		t.Errorf("expected 'line1' to be excluded, got %q", result)
	}
}

func TestReadHandlerEndLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"path":"` + testFile + `","end_line":2}`)
	result, err := readHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result, "line1") || !contains(result, "line2") {
		t.Errorf("expected lines 1 and 2 in result, got %q", result)
	}
	if contains(result, "line3") {
		t.Errorf("expected 'line3' to be excluded, got %q", result)
	}
}

func TestReadHandlerRange(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"path":"` + testFile + `","start_line":2,"end_line":3}`)
	result, err := readHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result, "line2") || !contains(result, "line3") {
		t.Errorf("expected lines 2 and 3 in result, got %q", result)
	}
	if contains(result, "line1") || contains(result, "line4") {
		t.Errorf("expected only lines 2-3, got %q", result)
	}
}

func TestReadHandlerMissingPath(t *testing.T) {
	input := json.RawMessage(`{}`)
	_, err := readHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestReadHandlerEmptyPath(t *testing.T) {
	input := json.RawMessage(`{"path":""}`)
	_, err := readHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestReadHandlerInvalidStartLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"path":"` + testFile + `","start_line":0}`)
	_, err = readHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for start_line < 1")
	}
}

func TestReadHandlerNonexistentFile(t *testing.T) {
	input := json.RawMessage(`{"path":"/nonexistent/file.txt"}`)
	_, err := readHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadHandlerPreservesHash(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("hello world\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"path":"` + testFile + `"}`)
	result, err := readHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Hash of "hello world" should be consistent
	hash := lineHashHex("hello world")
	if !contains(result, hash) {
		t.Errorf("expected hash %q in result, got %q", hash, result)
	}
}

func TestWriteFileTool(t *testing.T) {
	if WriteFile.Name != "write_file" {
		t.Errorf("expected name 'write_file', got %q", WriteFile.Name)
	}
	if WriteFile.Description == "" {
		t.Error("expected non-empty description")
	}
	if WriteFile.InputSchema == nil {
		t.Error("expected non-nil input schema")
	}
	if WriteFile.Handler == nil {
		t.Error("expected non-nil handler")
	}
}

func TestWriteFileHandlerReplace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	hash := lineHashHex("line2")
	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"replace","line":2,"hash":"` + hash + `","content":"new line2"}]}`)
	result, err := writeFileHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(result, "applied 1 operation") {
		t.Errorf("expected success message, got %q", result)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	// Verify replacement worked - "new line2" should be in the file
	if !contains(string(data), "new line2") {
		t.Errorf("expected 'new line2' in file, got %q", string(data))
	}
	// Verify old content was replaced - check line by line
	lines := splitLines(string(data))
	found := false
	for _, line := range lines {
		if line == "line2" && !contains(string(data), "new line2\n") {
			// If "line2" exists AND "new line2\n" doesn't exist, then replacement failed
			found = true
		}
	}
	if found {
		t.Errorf("expected 'line2' to be replaced, got %q", string(data))
	}
}

func TestWriteFileHandlerInsertBefore(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	hash := lineHashHex("line2")
	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"insert_before","line":2,"hash":"` + hash + `","content":"before line2"}]}`)
	_, err = writeFileHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(string(data))
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
	if lines[1] != "before line2" {
		t.Errorf("expected 'before line2' at line 2, got %q", lines[1])
	}
}

func TestWriteFileHandlerInsertAfter(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	hash := lineHashHex("line1")
	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"insert_after","line":1,"hash":"` + hash + `","content":"after line1"}]}`)
	_, err = writeFileHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := splitLines(string(data))
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
	if lines[1] != "after line1" {
		t.Errorf("expected 'after line1' at line 2, got %q", lines[1])
	}
}

func TestWriteFileHandlerMultipleOperations(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	hash1 := lineHashHex("line1")
	hash3 := lineHashHex("line3")
	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"replace","line":1,"hash":"` + hash1 + `","content":"new line1"},{"op":"replace","line":3,"hash":"` + hash3 + `","content":"new line3"}]}`)
	_, err = writeFileHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), "new line1") {
		t.Errorf("expected 'new line1' in file, got %q", string(data))
	}
	if !contains(string(data), "new line3") {
		t.Errorf("expected 'new line3' in file, got %q", string(data))
	}
}

func TestWriteFileHandlerHashMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"replace","line":1,"hash":"wrong","content":"new content"}]}`)
	_, err = writeFileHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for hash mismatch")
	}
}

func TestWriteFileHandlerLineOutOfRange(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"replace","line":5,"hash":"xx","content":"new"}]}`)
	_, err = writeFileHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for line out of range")
	}
}

func TestWriteFileHandlerDuplicateLine(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	hash := lineHashHex("line1")
	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"replace","line":1,"hash":"` + hash + `","content":"new1"},{"op":"replace","line":1,"hash":"` + hash + `","content":"new2"}]}`)
	_, err = writeFileHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for duplicate line operations")
	}
}

func TestWriteFileHandlerMissingPath(t *testing.T) {
	input := json.RawMessage(`{"operations":[{"op":"replace","line":1,"hash":"xx","content":"new"}]}`)
	_, err := writeFileHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestWriteFileHandlerEmptyOperations(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"path":"` + testFile + `","operations":[]}`)
	_, err = writeFileHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty operations")
	}
}

func TestWriteFileHandlerInvalidOp(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"invalid","line":1,"hash":"xx","content":"new"}]}`)
	_, err = writeFileHandler(context.Background(), input)
	if err == nil {
		t.Error("expected error for invalid op")
	}
}

func TestWriteFileHandlerPreservesTrailingNewline(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	hash := lineHashHex("line1")
	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"replace","line":1,"hash":"` + hash + `","content":"line1"}]}`)
	_, err = writeFileHandler(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if !hasTrailingNewline(string(data)) {
		t.Error("expected trailing newline to be preserved")
	}
}

func TestWriteFileHandlerMultilineContent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\nline2\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	hash := lineHashHex("line1")
	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"replace","line":1,"hash":"` + hash + `","content":"multi\nline\ncontent"}]}`)
	_, err = writeFileHandler(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), "multi") || !contains(string(data), "line") || !contains(string(data), "content") {
		t.Errorf("expected multiline content in file, got %q", string(data))
	}
}

func TestWriteFileHandlerPreservesPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("line1\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(testFile, 0o755)
	if err != nil {
		t.Fatal(err)
	}

	hash := lineHashHex("line1")
	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"replace","line":1,"hash":"` + hash + `","content":"line1"}]}`)
	_, err = writeFileHandler(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode() != 0o755 {
		t.Errorf("expected permissions 0755, got %o", info.Mode())
	}
}

func TestWriteFileHandlerCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "new.txt")

	hash := lineHashHex("")
	input := json.RawMessage(`{"path":"` + testFile + `","operations":[{"op":"insert_before","line":1,"hash":"` + hash + `","content":"first line"}]}`)
	_, err := writeFileHandler(context.Background(), input)
	// writeFileHandler does not support creating new files - it fails on os.Stat
	if err == nil {
		t.Error("expected error when creating new file")
	}
}

func TestWriteOpStruct(t *testing.T) {
	op := WriteOp{
		Op:      "replace",
		Line:    5,
		Hash:    "ab",
		Content: "new content",
	}
	if op.Op != "replace" {
		t.Errorf("expected op 'replace', got %q", op.Op)
	}
	if op.Line != 5 {
		t.Errorf("expected line 5, got %d", op.Line)
	}
	if op.Hash != "ab" {
		t.Errorf("expected hash 'ab', got %q", op.Hash)
	}
	if op.Content != "new content" {
		t.Errorf("expected content 'new content', got %q", op.Content)
	}
}

func TestAll(t *testing.T) {
	tools := All()
	if len(tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(tools))
	}
	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}
	if !names["bash"] {
		t.Error("expected 'bash' in tools")
	}
	if !names["find"] {
		t.Error("expected 'find' in tools")
	}
	if !names["read"] {
		t.Error("expected 'read' in tools")
	}
	if !names["write_file"] {
		t.Error("expected 'write_file' in tools")
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func hasTrailingNewline(s string) bool {
	return len(s) > 0 && s[len(s)-1] == '\n'
}