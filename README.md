# Turn-based agent CLI

## Usage

Continue the current session, or start a new one, and execute a single turn
with the agent.

```
please "Update the readme to describe all of the features"
```

### Options

- `--new` - Start a new session instead of continuing the current one.
- `--one-off` - Take a turn without updating the current session symlink.
- `-c, --completion <shell>` - Generate shell completion script.

### Shell Completions

Generate a completion script for your shell:

```bash
# Bash
please --completion bash >> ~/.bashrc

# Zsh
please --completion zsh >> ~/.zshrc

# Fish
please --completion fish > ~/.config/fish/completions/please.fish
```

Supported shells: `bash`, `zsh`, `fish`, `powershell`.

After sourcing the completion script, you'll get tab completion for flags
(`-n`, `--new`, `-1`, `--one-off`, `-h`, `--help`, `--completion`) when
typing `please`.

### System Prompt

System prompt files are looked up in the repo root (if in a git repository)
or the current working directory (if not in a git repo).

#### SYSTEM.md

Create a `SYSTEM.md` file in the repo root to customize the agent's behavior.
The contents of this file are sent as the system prompt on every request.

If no `SYSTEM.md` exists, a built-in default prompt is used.

#### AGENTS.md

You can also create an `AGENTS.md` file for foundational agent instructions.
When starting a new session (or continuing an empty session), the contents of
`AGENTS.md` are sent as the first user message, followed by your actual message:

```
User: <AGENTS.md content>

User: <your message>
```

This allows AGENTS.md to function as persistent foundational instructions
that are injected at the start of each new conversation context.

## Providers

`please` uses the MiniMax API via the Anthropic Messages API format. Set your
API key via the `MINIMAX_API_KEY` environment variable.

The default model is `minimax-m2.7`.

## Tools

The agent has access to the following tools:

### bash

Execute bash commands. The combined stdout and stderr are returned.
Non-zero exit codes are reported at the end of the output.

### find

Find files using glob patterns. Supports `**` for recursive matching.
The `.git` directory is excluded by default.

### read

Read a file with line numbers and content hashes for edit verification.
Use `start_line` and `end_line` parameters to read specific ranges.

### write_file

Apply line-level edits to a file. Operations are validated before any write
occurs. Supported operations:
- `replace`: Replace a line with new content
- `insert_before`: Insert content before a line
- `insert_after`: Insert content after a line

## The `.please` directory

`please` maintains the state for the project in a folder called `.please`. Be sure
to add it to your `.gitignore` file.

Each new session is managed in `.please/sessions/` with a uuid-based filename
(e.g. `abc12345.jsonl`). A symlink is placed in `.please/current-session` to
reference the currently active session. If no session is active when run, a new
session is created.

## Development

Please uses charmbracelet/lipgloss for sane defaults to make it look nice.
