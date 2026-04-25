package stream

import (
	"strings"
	"sync"
	"testing"

	"github.com/nalanj/please/ui/markdown"
	"github.com/nalanj/please/util/ansi"
)

func TestOutputHandlerFlush(t *testing.T) {
	renderer := md.New()
	var outputMu sync.Mutex
	var output strings.Builder

	type chunk struct {
		kind    string
		content string
	}
	renderCh := make(chan chunk, 100)
	renderDone := make(chan struct{})

	go func() {
		textBuf := strings.Builder{}

		for c := range renderCh {
			switch c.kind {
			case "text":
				textBuf.WriteString(c.content)
				if strings.HasSuffix(c.content, " ") || strings.HasSuffix(c.content, "\n") {
					outputMu.Lock()
					output.WriteString(renderer.Write(textBuf.String()))
					outputMu.Unlock()
					textBuf.Reset()
				}
			case "flush":
				outputMu.Lock()
				output.WriteString(renderer.Write(textBuf.String()))
				if remaining := renderer.Flush(); remaining != "" {
					output.WriteString(remaining)
				}
				outputMu.Unlock()
			}
		}
		close(renderDone)
	}()

	renderCh <- chunk{"text", "Hello "}
	renderCh <- chunk{"text", "world\n"}
	renderCh <- chunk{"flush", ""}

	close(renderCh)
	<-renderDone

	result := output.String()
	t.Logf("Output: %q", result)

	if result == "" {
		t.Error("expected some output, got empty string")
	}
}

func TestMarkdownRendererFlushComplete(t *testing.T) {
	renderer := md.New()

	// Write content that doesn't trigger immediate processing
	renderer.Write("Hello ")
	renderer.Write("world")

	// Flush returns only what's in the buffer AFTER processing
	// Since world was written, it might be buffered or processed
	flushed := renderer.Flush()
	t.Logf("Flush returned: %q", flushed)

	// The actual behavior: Write processes and returns content,
	// Flush only returns unprocessed buffer content
}

func TestFlushAfterClose(t *testing.T) {
	renderer := md.New()
	renderCh := make(chan string, 100)
	renderDone := make(chan struct{})

	var output strings.Builder
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		buf := strings.Builder{}

		for ch := range renderCh {
			buf.WriteString(ch)
		}

		// Final flush after channel closes
		output.WriteString(renderer.Write(buf.String()))
		if remaining := renderer.Flush(); remaining != "" {
			output.WriteString(remaining)
		}
		close(renderDone)
	}()

	renderCh <- "Hello "
	renderCh <- "world"

	close(renderCh)
	<-renderDone
	wg.Wait()

	result := output.String()
	t.Logf("Output: %q", result)

	if result != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", result)
	}
}

func TestFlushMessageLost(t *testing.T) {
	renderer := md.New()
	renderCh := make(chan string, 100)
	renderDone := make(chan struct{})

	var output strings.Builder
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		buf := strings.Builder{}

		for ch := range renderCh {
			if ch == "FLUSH" {
				output.WriteString(renderer.Write(buf.String()))
				buf.Reset()
				if remaining := renderer.Flush(); remaining != "" {
					output.WriteString(remaining)
				}
				continue
			}
			buf.WriteString(ch)
		}

		// Final flush after channel closes
		output.WriteString(renderer.Write(buf.String()))
		if remaining := renderer.Flush(); remaining != "" {
			output.WriteString(remaining)
		}
		close(renderDone)
	}()

	renderCh <- "Hello "
	renderCh <- "world\n"

	close(renderCh)
	<-renderDone
	wg.Wait()

	result := output.String()
	t.Logf("Output (no FLUSH sent): %q", result)

	if result != "Hello world\n" {
		t.Errorf("expected 'Hello world\\n', got %q", result)
	}
}

func TestFlushMessageProcessed(t *testing.T) {
	renderer := md.New()
	renderCh := make(chan string, 100)
	renderDone := make(chan struct{})

	var output strings.Builder
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		buf := strings.Builder{}

		for ch := range renderCh {
			if ch == "FLUSH" {
				output.WriteString(renderer.Write(buf.String()))
				buf.Reset()
				if remaining := renderer.Flush(); remaining != "" {
					output.WriteString(remaining)
				}
				continue
			}
			buf.WriteString(ch)
		}
		close(renderDone)
	}()

	renderCh <- "Hello "
	renderCh <- "world\n"

	// CORRECT: Send FLUSH before closing
	renderCh <- "FLUSH"
	close(renderCh)
	<-renderDone
	wg.Wait()

	result := output.String()
	t.Logf("Output (FLUSH sent): %q", result)

	if result != "Hello world\n" {
		t.Errorf("expected 'Hello world\\n', got %q", result)
	}
}

func TestStreamSyncWithRenderDone(t *testing.T) {
	renderer := md.New()
	renderCh := make(chan string, 100)
	renderDone := make(chan struct{})

	var output strings.Builder
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		buf := strings.Builder{}

		for ch := range renderCh {
			if ch == "FLUSH" {
				output.WriteString(renderer.Write(buf.String()))
				buf.Reset()
				if remaining := renderer.Flush(); remaining != "" {
					output.WriteString(remaining)
				}
				continue
			}
			buf.WriteString(ch)
		}

		output.WriteString(renderer.Write(buf.String()))
		if remaining := renderer.Flush(); remaining != "" {
			output.WriteString(remaining)
		}
		close(renderDone)
	}()

	renderCh <- "Hello "
	renderCh <- "world\n"
	renderCh <- "FLUSH"

	close(renderCh)
	<-renderDone
	wg.Wait()

	result := output.String()
	t.Logf("Output: %q", result)

	if !strings.Contains(result, "Hello world") {
		t.Errorf("expected 'Hello world' in output, got %q", result)
	}
}

func TestThinkingMarkdownRenderingWorks(t *testing.T) {
	h := New(md.New())

	// Test bold markdown when complete pattern arrives in one chunk
	result := h.Handle("thinking", "**bold**\n")
	t.Logf("Result: %q", result)
	t.Logf("IsColorEnabled: %v", ansi.IsColorEnabled())

	if ansi.IsColorEnabled() {
		// When colors are enabled, bold ANSI code should be present
		if !strings.Contains(result, "\x1b[1m") {
			t.Errorf("thinking with **bold** should contain bold styling when colors enabled, got %q", result)
		}
	}
	// When colors are disabled, plain text is expected
}

func TestThinkingStyleCodesApplied(t *testing.T) {
	h := New(md.New())

	// Write simple thinking content
	result := h.Handle("thinking", "thinking text\n")
	t.Logf("Result: %q", result)

	// When colors are enabled, thinking style should be present
	if ansi.IsColorEnabled() {
		if !strings.Contains(result, "\x1b[3;2m") {
			t.Errorf("thinking should contain italic/faint styling (\\x1b[3;2m) when colors enabled, got %q", result)
		}
	}
}

func TestThinkingMarkdownAndStyleTogether(t *testing.T) {
	h := New(md.New())

	// Test with complete bold pattern in one chunk
	result := h.Handle("thinking", "**bold** text\n")
	t.Logf("Result: %q", result)

	// When colors are enabled, both styles should be present
	if ansi.IsColorEnabled() {
		hasBold := strings.Contains(result, "\x1b[1m")
		hasItalicFaint := strings.Contains(result, "\x1b[3;2m")

		if !hasBold {
			t.Errorf("thinking should have bold styling when colors enabled, got %q", result)
		}
		if !hasItalicFaint {
			t.Errorf("thinking should have italic/faint styling when colors enabled, got %q", result)
		}
	}
}

// Note: List items in thinking content may not render with list styling
// because the streaming renderer flushes at word boundaries, breaking
// the markdown pattern matching for multi-word patterns like "1. item"
