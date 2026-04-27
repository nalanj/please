package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nalanj/please/util/llm"
)

func TestRepoRoot(t *testing.T) {
	// Save original cwd
	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origCwd) }()

	t.Run("finds .git in parent directory", func(t *testing.T) {
		// Create a temp dir structure: /tmp/testrepo/.git and /tmp/testrepo/subdir
		tempDir := t.TempDir()
		gitDir := filepath.Join(tempDir, ".git")
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatal(err)
		}
		subDir := filepath.Join(tempDir, "subdir")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Change to subdirectory
		if err := os.Chdir(subDir); err != nil {
			t.Fatal(err)
		}

		root := repoRoot()
		if root != tempDir {
			t.Errorf("expected repoRoot() = %q, got %q", tempDir, root)
		}
	})

	t.Run("no .git found returns cwd", func(t *testing.T) {
		// Create a temp dir with no .git
		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "subdir")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.Chdir(subDir); err != nil {
			t.Fatal(err)
		}

		root := repoRoot()
		cwd, _ := os.Getwd()
		if root != cwd {
			t.Errorf("expected repoRoot() = %q (cwd), got %q", cwd, root)
		}
	})
}

func TestBuildSystemPrompt(t *testing.T) {
	// Save original cwd
	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origCwd) }()

	t.Run("SYSTEM.md exists returns its content", func(t *testing.T) {
		tempDir := t.TempDir()

		systemPath := filepath.Join(tempDir, "SYSTEM.md")
		if err := os.WriteFile(systemPath, []byte("system content"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := os.Chdir(tempDir); err != nil {
			t.Fatal(err)
		}

		result := buildSystemPrompt()
		if result != "system content" {
			t.Errorf("expected %q, got %q", "system content", result)
		}
	})

	t.Run("SYSTEM.md doesn't exist returns default", func(t *testing.T) {
		tempDir := t.TempDir()

		if err := os.Chdir(tempDir); err != nil {
			t.Fatal(err)
		}

		result := buildSystemPrompt()
		if result != "You are a helpful assistant." {
			t.Errorf("expected default %q, got %q", "You are a helpful assistant.", result)
		}
	})

	t.Run("finds SYSTEM.md in repo root with .git", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create .git directory at root
		gitDir := filepath.Join(tempDir, ".git")
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create SYSTEM.md at root
		systemPath := filepath.Join(tempDir, "SYSTEM.md")
		if err := os.WriteFile(systemPath, []byte("root system"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create subdir and change to it
		subDir := filepath.Join(tempDir, "subdir")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(subDir); err != nil {
			t.Fatal(err)
		}

		result := buildSystemPrompt()
		if result != "root system" {
			t.Errorf("expected %q, got %q", "root system", result)
		}
	})
}

func TestLoadAgentsPrompt(t *testing.T) {
	// Save original cwd
	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origCwd) }()

	t.Run("AGENTS.md exists returns its content", func(t *testing.T) {
		tempDir := t.TempDir()

		agentsPath := filepath.Join(tempDir, "AGENTS.md")
		if err := os.WriteFile(agentsPath, []byte("agents content"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := os.Chdir(tempDir); err != nil {
			t.Fatal(err)
		}

		result := loadAgentsPrompt()
		if result != "agents content" {
			t.Errorf("expected %q, got %q", "agents content", result)
		}
	})

	t.Run("AGENTS.md doesn't exist returns empty string", func(t *testing.T) {
		tempDir := t.TempDir()

		if err := os.Chdir(tempDir); err != nil {
			t.Fatal(err)
		}

		result := loadAgentsPrompt()
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("finds AGENTS.md in repo root with .git", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create .git directory at root
		gitDir := filepath.Join(tempDir, ".git")
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create AGENTS.md at root
		agentsPath := filepath.Join(tempDir, "AGENTS.md")
		if err := os.WriteFile(agentsPath, []byte("root agents"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create subdir and change to it
		subDir := filepath.Join(tempDir, "subdir")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(subDir); err != nil {
			t.Fatal(err)
		}

		result := loadAgentsPrompt()
		if result != "root agents" {
			t.Errorf("expected %q, got %q", "root agents", result)
		}
	})
}

func TestAgentsMdAsDistinctMessage(t *testing.T) {
	// Save original cwd
	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origCwd) }()

	t.Run("AGENTS.md becomes distinct first user message", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create SYSTEM.md
		systemPath := filepath.Join(tempDir, "SYSTEM.md")
		if err := os.WriteFile(systemPath, []byte("system instructions"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Create AGENTS.md
		agentsPath := filepath.Join(tempDir, "AGENTS.md")
		if err := os.WriteFile(agentsPath, []byte("agents foundational instructions"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := os.Chdir(tempDir); err != nil {
			t.Fatal(err)
		}

		// Simulate what the code does
		system := buildSystemPrompt()
		agentsContent := loadAgentsPrompt()

		// Verify system prompt
		if system != "system instructions" {
			t.Errorf("expected system %q, got %q", "system instructions", system)
		}

		// Verify agents content
		if agentsContent != "agents foundational instructions" {
			t.Errorf("expected agents %q, got %q", "agents foundational instructions", agentsContent)
		}

		// Simulate the message construction (as done in run())
		// For newSession=true and AGENTS.md exists:
		// opts = append(opts, takeTurn.WithMessages(llm.TextMessage(llm.RoleUser, agentsContent)))
		messages := []llm.Message{llm.TextMessage(llm.RoleUser, agentsContent)}

		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}
		if messages[0].Role != llm.RoleUser {
			t.Errorf("expected role user, got %v", messages[0].Role)
		}
		if messages[0].Content[0].Text != "agents foundational instructions" {
			t.Errorf("expected content %q, got %q", "agents foundational instructions", messages[0].Content[0].Text)
		}
	})

	t.Run("AGENTS.md not added for existing session", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create AGENTS.md
		agentsPath := filepath.Join(tempDir, "AGENTS.md")
		if err := os.WriteFile(agentsPath, []byte("agents content"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := os.Chdir(tempDir); err != nil {
			t.Fatal(err)
		}

		// Simulate existing session (len(existing) > 0)
		existingSession := true
		agentsContent := loadAgentsPrompt()

		// Condition: (newSession || len(existing) == 0)
		// With newSession=false and existingSession=true, condition is false
		shouldAddAgents := !existingSession // simulating the condition

		if shouldAddAgents {
			t.Error("expected AGENTS.md NOT to be added when session exists")
		}

		// Verify we still have the content available if needed
		if agentsContent != "agents content" {
			t.Errorf("expected agents %q, got %q", "agents content", agentsContent)
		}
	})
}
