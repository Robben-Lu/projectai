# ProjectAI

AI-native task and knowledge aggregation across GitHub, Google Tasks, Apple Reminders, and Drafts.

## What is this?

ProjectAI fills the CLI gaps for macOS-native apps, then provides a Skill that teaches AI to aggregate across all sources:

1. **Reminders CLI** — Go CLI for Apple Reminders (list, add, done, delete)
2. **Drafts CLI** — Go CLI for Drafts app (list, search, get, create, archive, tag)
3. **ProjectAI Skill** — Claude Code skill for unified task aggregation

## The idea

Instead of building a centralized task management platform, let AI be the aggregation layer:

```
AI (Claude Code / Gemini / Codex)
  ├── gh CLI          → GitHub Issues / Project  (existing)
  ├── gws CLI         → Google Tasks             (existing)
  ├── reminders CLI   → Apple Reminders          (this repo)
  └── drafts CLI      → Drafts app               (this repo)
```

The Skill tells AI which source to query and how to merge results. No sync, no database, no web UI.

## Install

```bash
# Both CLIs
go install github.com/Robben-Lu/projectai/cmd/reminders@latest
go install github.com/Robben-Lu/projectai/cmd/drafts@latest

# Or build locally
make build    # outputs to bin/

# Skill: copy skill/projectai.md to your Claude Code skill directory
```

## Reminders CLI

```bash
reminders lists                                    # List all reminder lists
reminders list [--list Shopping] [--due today]      # List reminders
reminders add "Buy coffee" --list Shopping --due "today 5pm"
reminders done <id>
reminders delete <id>
```

## Drafts CLI

```bash
drafts list [--folder inbox|archive|all] [--flagged]   # List drafts
drafts search "keyword" [--folder all]                  # Search by content
drafts get <id>                                         # Full content
drafts create "Quick note" [--tag meeting] [--flagged]  # Create
drafts append <id> "Additional text"                    # Append to draft
drafts flag <id>                                        # Flag/unflag
drafts archive <id>                                     # Move to archive
drafts trash <id>                                       # Move to trash
drafts tag <id> <tag>                                   # Add tag
```

All commands output JSON by default (AI-friendly). Use `--format table` for human-readable output.

## Four-Source Model

| Source | Role | CLI | Collaboration |
|--------|------|-----|--------------|
| GitHub | Code projects | `gh` | Team |
| Google Tasks | Business tasks | `gws tasks` | Team (via assignment) |
| Apple Reminders | Personal tasks | `reminders` | Individual |
| Drafts | Quick notes & ideas | `drafts` | Individual |

## License

MIT
