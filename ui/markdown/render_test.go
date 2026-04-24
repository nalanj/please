package md

import (
	"strings"
	"testing"
)

func TestHeader(t *testing.T) {
	r := New()
	
	tests := []struct {
		name     string
		input    string
		wantPart string // part that should appear (without ANSI codes)
	}{
		{"Complete header", "# Hello\n", "# Hello"},
		{"Incomplete header", "# Hello", ""},
		{"H2", "## Title\n", "## Title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Write(tt.input)
			if tt.wantPart != "" && !strings.Contains(got, tt.wantPart) {
				t.Errorf("got %q, want to contain %q", got, tt.wantPart)
			}
			if tt.wantPart == "" && got != "" {
				t.Errorf("got %q, want empty", got)
			}
		})
	}
}

func TestBold(t *testing.T) {
	r := New()
	
	tests := []struct {
		name     string
		input    string
		wantPart string
	}{
		{"Complete bold", "Hello **world**!", "world"},
		{"Incomplete bold", "Hello **world", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Write(tt.input)
			if tt.wantPart != "" && !strings.Contains(got, tt.wantPart) {
				t.Errorf("got %q, want to contain %q", got, tt.wantPart)
			}
			if tt.wantPart == "" && got != "" {
				t.Errorf("got %q, want empty", got)
			}
		})
	}
}

func TestInlineCode(t *testing.T) {
	r := New()
	
	tests := []struct {
		name     string
		input    string
		wantPart string
	}{
		{"Complete code", "Use `fmt.Println()`", "fmt.Println()"},
		{"Incomplete code", "Use `code", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Write(tt.input)
			if tt.wantPart != "" && !strings.Contains(got, tt.wantPart) {
				t.Errorf("got %q, want to contain %q", got, tt.wantPart)
			}
			if tt.wantPart == "" && got != "" {
				t.Errorf("got %q, want empty", got)
			}
		})
	}
}

func TestList(t *testing.T) {
	r := New()
	
	tests := []struct {
		name     string
		input    string
		wantPart string
	}{
		{"Complete list", "- item 1\n", "- item 1"},
		{"Incomplete list", "- item", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Write(tt.input)
			if tt.wantPart != "" && !strings.Contains(got, tt.wantPart) {
				t.Errorf("got %q, want to contain %q", got, tt.wantPart)
			}
			if tt.wantPart == "" && got != "" {
				t.Errorf("got %q, want empty", got)
			}
		})
	}
}
func TestOrderedList(t *testing.T) {
	r := New()

	tests := []struct {
		name     string
		input    string
		wantPart string
	}{
		{"Complete ordered list", "1. item 1\n", "1. item 1"},
		{"Incomplete ordered list", "1. item", ""},
		{"Multi-digit number", "10. item 10\n", "10. item 10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Write(tt.input)
			if tt.wantPart != "" && !strings.Contains(got, tt.wantPart) {
				t.Errorf("got %q, want to contain %q", got, tt.wantPart)
			}
			if tt.wantPart == "" && got != "" {
				t.Errorf("got %q, want empty", got)
			}
		})
	}
}


func TestFlush(t *testing.T) {
	r := New()
	
	// Write incomplete text - should be buffered
	r.Write("Hello **world")
	flushed := r.Flush()
	if flushed != "Hello **world" {
		t.Errorf("Flush = %q, want %q", flushed, "Hello **world")
	}
	
	// After flush, buffer should be empty
	flushed2 := r.Flush()
	if flushed2 != "" {
		t.Errorf("Second Flush = %q, want empty", flushed2)
	}
}

func TestCompleteBoldStreaming(t *testing.T) {
	r := New()
	
	// Stream partial bold, then complete it
	part1 := r.Write("Hello **wor")
	part2 := r.Write("ld**!")
	
	// part1 should be empty (incomplete pattern buffered)
	// part2 should have styled bold
	if part1 != "" {
		t.Errorf("part1 = %q, want empty", part1)
	}
	if !strings.Contains(part2, "world") {
		t.Errorf("part2 = %q, want to contain 'world'", part2)
	}
}
