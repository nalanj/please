# Agent Instructions for `please` Project

## Project Overview

`please` is a turn-based agent CLI that wraps LLM interactions with tool calling capabilities. It maintains conversation history in sessions and provides a set of file/system manipulation tools.

## Key Files and Their Purposes

- `cmd/please/main.go` - Entry point, session management, agent orchestration
- `ops/agent/takeTurn/agent.go` - Core agent loop with tool execution
- `util/llm/` - LLM provider abstraction and message types
- `util/tools/tools.go` - Tool definitions (bash, find, read, write_file)
- `session/` - Session persistence (JSONL format)

## Working with this Codebase

### Session Files
- Sessions are stored in `.please/sessions/` as `.jsonl` files
- Each line is a JSON-encoded message
- The `current-session` symlink points to the active session
- **Never modify session files directly** - always use the built-in tools

### File Editing Conventions
- Use the `write_file` tool for modifications
- Include the 2-character line hash for verification
- Use `replace` for straightforward changes
- Use `insert_before`/`insert_after` for adding new code

### Test-Driven Development (TDD)

This project follows the **red/green TDD** methodology:

1. **Red** - Write a failing test first. The test should describe the expected behavior without implementing the feature.
2. **Green** - Write the minimal code needed to make the test pass. Do not optimize or add extra features at this stage.
3. **Refactor** - Once tests pass, you may refactor to improve code quality while keeping tests green.

When adding features:
- Always write tests first
- Run `go test ./...` to verify all tests pass
- Run `golangci-lint run` to ensure code quality

### Adding New Features

1. **New tools** - Add to `util/tools/tools.go` following the existing patterns
2. **New LLM providers** - Implement the `llm.Provider` interface in `util/llm/`
3. **New prompt files** - Both `SYSTEM.md` (system prompt) and `AGENTS.md` (first user message) are loaded from repo root

## Agent Behavior Guidelines

- Be explicit about file changes in output
- Confirm destructive actions verbally before executing
- Preserve session state - don't interrupt active sessions without good reason
- When fixing lint errors, prefer the simplest fix that passes golangci-lint
- Follow red/green TDD: write failing test first, then implement
