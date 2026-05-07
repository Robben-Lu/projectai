# ProjectAI

AI-native task and knowledge aggregation across GitHub, Google Tasks, Apple Reminders, and Drafts.

## What is this?

ProjectAI fills the CLI gaps for macOS-native apps, then provides a Skill that teaches AI to aggregate across all sources:

1. **Reminders CLI** — Go CLI for Apple Reminders (list, add, done, delete)
2. **Drafts CLI** — Go CLI for Drafts app (list, search, get, create, archive, tag)
3. **ProjectAI CLI** — `projectai today` for daily cross-source aggregation
4. **ProjectAI Skill** — Claude Code skill for unified task aggregation

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
# CLIs
go install github.com/Robben-Lu/projectai/cmd/projectai@latest
go install github.com/Robben-Lu/projectai/cmd/reminders@latest
go install github.com/Robben-Lu/projectai/cmd/drafts@latest

# Or build locally
make build    # outputs to bin/

# Skill: copy skill/projectai.md to your Claude Code skill directory
```

## ProjectAI Today

`projectai today` pulls the daily working set from GitHub Project, Google Tasks, Apple Reminders, and flagged Drafts. Missing tools or unavailable apps are reported as warnings on stderr and do not stop the other sources.

```bash
projectai today
projectai today --format json | jq
projectai today --format ndjson
projectai today --source github,gtasks
projectai today --window 14d
projectai overdue
projectai gh --status 进行中
projectai gh --priority P1 --system WorkForce
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--window <duration>` | `7d` | Future due window. Overdue items are always included; `0d` shows overdue only. |
| `--source <list>` | all | Comma-separated sources: `github`, `gtasks`, `reminders`, `drafts`. Aliases include `gh`, `gws`, and `rmd`. |
| `--format <format>` | `table` | `table`, `json`, or `ndjson`. |
| `--owner <org>` | `PROJECTAI_GH_OWNER` or `Ecomulch` | GitHub Project owner. |
| `--project <num>` | `1` | GitHub Project number. |

Table output:

```text
Source  Status       Due        Title                              Link
GH      进行中        -          #488 [P0] CashOps V13-V17 ...      https://github.com/...
GTasks  needsAction  2026-05-10 设置 Shared Drive 成员权限          https://tasks.google.com/...
RMD     open         OVERDUE    缴话费                              -
```

Screenshot placeholder: add `docs/images/projectai-today.png` after capturing local output.

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

`reminders` and `drafts` output JSON by default. `projectai today` outputs a table by default; use `--format json` or `--format ndjson` for AI-readable output.

## Four-Source Model

| Source | Role | CLI | Collaboration |
|--------|------|-----|--------------|
| GitHub | Code projects | `gh` | Team |
| Google Tasks | Business tasks | `gws tasks` | Team (via assignment) |
| Apple Reminders | Personal tasks | `reminders` | Individual |
| Drafts | Quick notes & ideas | `drafts` | Individual |

## License

MIT
