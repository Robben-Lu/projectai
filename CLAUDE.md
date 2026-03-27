# ProjectAI — Claude Code Instructions

## Project Overview

AI-native task aggregation tool. Two components:
1. **Reminders CLI** (`cmd/reminders/`) — Go CLI for Apple Reminders via AppleScript
2. **ProjectAI Skill** (`skill/projectai.md`) — Claude Code skill for three-source task management

## Tech Stack

- Go (latest stable)
- AppleScript via `osascript` for macOS Reminders integration
- No external dependencies beyond standard library

## Development

```bash
# Build
go build -o bin/reminders ./cmd/reminders

# Install locally
go install ./cmd/reminders

# Test
go test ./...
```

## Architecture

- `cmd/reminders/` — CLI entry point, flag parsing, output formatting
- `internal/applescript/` — AppleScript bridge, osascript execution
- `skill/` — Claude Code Skill (markdown, not code)

## Conventions

- JSON output by default (AI-friendly), `--format table` for humans
- Error messages to stderr, data to stdout
- All AppleScript execution goes through `internal/applescript/executor.go`
- No cgo — pure Go + osascript exec

## Related CLIs (not in this repo, but used by the Skill)

- `gh` — GitHub CLI (Issues, Projects)
- `gws` — Google Workspace CLI (Tasks)
