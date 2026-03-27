# ProjectAI — Design Spec

**Date**: 2026-03-27
**Author**: Claude Code + Lubin
**Status**: Approved

---

## 1. Problem Statement

Current task management is fragmented across three systems with no unified AI-operable interface:

- **GitHub Issues/Project** — code project tasks, team collaboration
- **Google Tasks** — business/work tasks, team coordination
- **Apple Reminders** — personal tasks, individual only

AI tools (Claude Code, Gemini CLI, Codex) can already operate GitHub (`gh` CLI) and Google Tasks (`gws` CLI), but:
1. No CLI exists for Apple Reminders
2. No unified guidance teaches AI how to aggregate across all three sources

## 2. Solution

Two deliverables:

| Component | Purpose | Form |
|-----------|---------|------|
| **Reminders CLI** | Fill the missing Apple Reminders CLI gap | Go binary (`reminders`) |
| **ProjectAI Skill** | Teach AI the three-source model + aggregation rules | Claude Code Skill (`.md`) |

### What this replaces

This replaces the earlier **Project OS** concept (`apps/labs/project-os/PD.md` in SDPT repo) with a lighter, AI-native approach:

| | Project OS | ProjectAI |
|--|-----------|-----------|
| Form | Custom Web App (.NET + React + PG) | CLI tool + Skill |
| Storage | Self-hosted database | Existing platforms (GitHub, Google Tasks, Reminders) |
| Deployment | Container + nginx + CI/CD | `go install` / `brew install` |
| AI interaction | API calls | CLI exec + MCP (future) |
| Aggregation | Custom backend code | AI is the aggregation layer (via Skill) |

### Future: MCP Server (Phase 2)

A thin MCP wrapper can be added later to enable remote AI access. Not needed now since all AI tools (Claude Code, Gemini CLI, Codex) run locally.

## 3. Architecture

```
AI (Claude Code / Gemini / Codex)
  │
  ├── Skill (projectai.md) ← aggregation rules, routing logic
  │
  ├── gh CLI (existing)        → GitHub Issues / Project
  ├── gws CLI (existing)       → Google Tasks
  └── reminders CLI (new)      → Apple Reminders (via AppleScript)
```

The AI itself is the orchestration layer. The Skill provides the instructions; the CLIs provide the tools.

## 4. Reminders CLI

### 4.1 Commands

```bash
reminders list [--list <name>] [--due today|tomorrow|week] [--format json|table]
reminders add <title> [--list <name>] [--due <datetime>] [--notes <text>] [--priority low|medium|high]
reminders done <id>
reminders delete <id>
reminders lists                    # list all reminder lists
```

### 4.2 Technical approach

- **Language**: Go
- **macOS integration**: AppleScript via `osascript` (no cgo required)
- **Output**: JSON by default (AI-friendly), `--format table` for human use
- **ID scheme**: List-scoped, e.g., `Shopping/3` or internal AppleScript ID

### 4.3 Core AppleScript operations

| Operation | AppleScript |
|-----------|------------|
| List all lists | `tell application "Reminders" to get name of every list` |
| List reminders | `tell application "Reminders" to get properties of every reminder in list "X"` |
| Create | `tell application "Reminders" to make new reminder in list "X" with properties {...}` |
| Complete | `tell application "Reminders" to set completed of reminder id "X" to true` |
| Delete | `tell application "Reminders" to delete reminder id "X" in list "X"` |

### 4.4 JSON output format

```json
{
  "id": "x-apple-reminder://XXXXXXXX",
  "title": "Buy coffee",
  "list": "Shopping",
  "due": "2026-03-27T17:00:00+08:00",
  "priority": "medium",
  "completed": false,
  "notes": "",
  "created": "2026-03-27T09:00:00+08:00"
}
```

## 5. ProjectAI Skill

### 5.1 Location

`skill/projectai.md` in this repo. Users install by adding the plugin or copying the skill file.

### 5.2 Skill responsibilities

The Skill is a set of instructions (not code) that teaches AI:

1. **Three-source model**: GitHub = code projects, Google Tasks = business tasks, Reminders = personal
2. **Aggregated queries**: When asked "what are my tasks today", run three commands in parallel, merge results, present unified view sorted by priority/due
3. **Routing**: When asked to create a task, determine the right source based on task nature
4. **Source-specific conventions**:
   - GitHub: Use SDPT Project board field IDs, status lifecycle from `agents/CONVENTIONS.md`
   - Google Tasks: Use tasklist as category, `[WIP]` prefix for in-progress
   - Reminders: Use list name as category, priority mapping

### 5.3 Unified Task schema (conceptual, in Skill)

| Field | GitHub | Google Tasks | Reminders |
|-------|--------|-------------|-----------|
| Title | issue title | task title | reminder name |
| Status | open/closed + Project status | needsAction/completed | incomplete/complete |
| Priority | label or Project field | (none natively) | low/medium/high |
| Due | milestone/field | due date | due date |
| Assignee | assignee | notes annotation | N/A |
| URL | issue URL | N/A | N/A |
| Category | repo + labels | tasklist name | list name |

## 6. Project structure

```
projectai/
├── cmd/
│   └── reminders/              # CLI entry point
│       └── main.go
├── internal/
│   └── applescript/            # AppleScript bridge
│       ├── reminders.go        # Core Reminders operations
│       └── executor.go         # osascript execution helper
├── skill/
│   └── projectai.md            # Claude Code Skill
├── go.mod
├── Makefile                    # build, install, test
├── README.md
├── CLAUDE.md                   # AI development instructions
└── LICENSE
```

## 7. Installation

```bash
# Reminders CLI
go install github.com/Robben-Lu/projectai/cmd/reminders@latest
# or
brew install projectai  # (future, if added to homebrew)

# Skill: add to Claude Code plugin or copy to skill directory
```

## 8. Non-goals

- No data synchronization between sources
- No web UI
- No central database
- No user management (each person uses their own CLI auth)
- No replacing `gh` or `gws` — only filling the Reminders gap

## 9. Evolution path

| Phase | Scope |
|-------|-------|
| **Phase 1 (now)** | Reminders CLI + Skill |
| **Phase 2** | MCP Server wrapping all three CLIs for remote AI access |
| **Phase 3** | Direct API integration (replace `exec` with native Go API clients) if performance demands |
