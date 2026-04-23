# please

Turn-based agent CLI

## Usage

Continue the current session, or start a new one, and execute a single turn
with the agent.

```
please "Update the readme to describe all of the features"
```

## Providers

Right now `please` is hard-coded to use MiniMax 2.7 with MiniMax. Providers will be further added in the future.

## Tools

_Needs content_

## The `.please` directory

`please` maintains the state for the project in a folder called `.please`. Be sure to add it to your `.gitignore` file.

Each new session is managed in `.please/sessions/` as a timestamp based file as to when it was created. A symlink is placed in `.please/current-session` to reference the currently active session. If no session is active when run, a new session is created.

## Development

Please uses charmbracelet/fang for sane defaults to make it look nice.
