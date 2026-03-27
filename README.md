# ProjectAI

AI-native task aggregation across GitHub, Google Tasks, and Apple Reminders.

## What is this?

ProjectAI provides two things:

1. **Reminders CLI** — A Go command-line tool for Apple Reminders (the missing piece — `gh` and `gws` already cover GitHub and Google Tasks)
2. **ProjectAI Skill** — A Claude Code skill that teaches AI to aggregate tasks across all three sources

## The idea

Instead of building a centralized task management platform, let AI be the aggregation layer:

```
AI (Claude Code / Gemini / Codex)
  ├── gh CLI          → GitHub Issues / Project
  ├── gws CLI         → Google Tasks
  └── reminders CLI   → Apple Reminders (this repo)
```

The Skill tells AI which source to query and how to merge results. No sync, no database, no web UI.

## Install

```bash
# Reminders CLI
go install github.com/Robben-Lu/projectai/cmd/reminders@latest

# Skill: copy skill/projectai.md to your Claude Code skill directory
```

## Reminders CLI Usage

```bash
reminders lists                          # List all reminder lists
reminders list [--list Shopping] [--due today]   # List reminders
reminders add "Buy coffee" --list Shopping --due "today 5pm"
reminders done <id>                      # Mark as complete
reminders delete <id>
```

Output is JSON by default (AI-friendly). Use `--format table` for human-readable output.

## Three-Source Model

| Source | Role | CLI | Collaboration |
|--------|------|-----|--------------|
| GitHub | Code projects | `gh` | Team |
| Google Tasks | Business tasks | `gws tasks` | Team (via assignment) |
| Apple Reminders | Personal tasks | `reminders` | Individual |

## License

MIT
