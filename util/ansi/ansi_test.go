package ansi

import (
	"os"
	"testing"
)

func TestStyle(t *testing.T) {
	// Save original
	orig := colorEnabled
	defer func() { colorEnabled = orig }()
	colorEnabled = true

	// Test that Style applies codes correctly
	result := Style("hello", Bold, FgNord9)
	expected := Bold + FgNord9 + "hello" + Reset
	if result != expected {
		t.Errorf("Style() = %q, want %q", result, expected)
	}
}

func TestStyleNoColor(t *testing.T) {
	orig := colorEnabled
	defer func() { colorEnabled = orig }()
	colorEnabled = false

	result := Style("hello", Bold)
	if result != "hello" {
		t.Errorf("Style() with colors disabled = %q, want %q", result, "hello")
	}
}

func TestFaint(t *testing.T) {
	orig := colorEnabled
	defer func() { colorEnabled = orig }()
	colorEnabled = true

	result := Faint("test")
	expected := Dim + "test" + Reset
	if result != expected {
		t.Errorf("Faint() = %q, want %q", result, expected)
	}
}

func TestFaintNoColor(t *testing.T) {
	orig := colorEnabled
	defer func() { colorEnabled = orig }()
	colorEnabled = false

	result := Faint("test")
	if result != "test" {
		t.Errorf("Faint() with colors disabled = %q, want %q", result, "test")
	}
}

func TestWrap(t *testing.T) {
	orig := colorEnabled
	defer func() { colorEnabled = orig }()
	colorEnabled = true

	result := Wrap("hi", 10)
	if len(result) != 10 {
		t.Errorf("Wrap() len = %d, want 10", len(result))
	}
}

func TestWrapNoColor(t *testing.T) {
	orig := colorEnabled
	defer func() { colorEnabled = orig }()
	colorEnabled = false

	result := Wrap("hi", 10)
	if len(result) != 10 {
		t.Errorf("Wrap() with colors disabled len = %d, want 10", len(result))
	}
}

func TestSupportsColorNO_COLOR(t *testing.T) {
	// Save original env
	origTerm := os.Getenv("TERM")
	defer func() {
		os.Setenv("TERM", origTerm)
		os.Unsetenv("NO_COLOR")
	}()

	os.Setenv("TERM", "xterm-256color")
	os.Setenv("NO_COLOR", "1")
	colorEnabled = supportsColor()
	if colorEnabled {
		t.Error("supportsColor() should return false with NO_COLOR=1")
	}
}

func TestSupportsColorDumb(t *testing.T) {
	// Save original env
	origTerm := os.Getenv("TERM")
	defer func() {
		os.Setenv("TERM", origTerm)
	}()

	os.Setenv("TERM", "dumb")
	colorEnabled = supportsColor()
	if colorEnabled {
		t.Error("supportsColor() should return false with TERM=dumb")
	}
}

func TestIsColorEnabled(t *testing.T) {
	orig := colorEnabled
	defer func() { colorEnabled = orig }()

	colorEnabled = true
	if !IsColorEnabled() {
		t.Error("IsColorEnabled() should return true when colorEnabled is true")
	}

	colorEnabled = false
	if IsColorEnabled() {
		t.Error("IsColorEnabled() should return false when colorEnabled is false")
	}
}
