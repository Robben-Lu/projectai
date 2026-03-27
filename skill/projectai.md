---
name: projectai
description: "Unified task management across GitHub, Google Tasks, Apple Reminders, and Drafts. Use when the user asks about tasks, todos, reminders, drafts, or wants a daily overview of their work."
---

# ProjectAI — AI-Native Task Aggregation

You have access to four task/content sources through CLI tools. Use them to give the user a unified view of their work.

## Sources

| Source | CLI | Role | Collaboration |
|--------|-----|------|--------------|
| **GitHub** | `gh` | Code project tasks (Issues, PRs, Project board) | Team |
| **Google Tasks** | `gws tasks` | Business/work tasks | Team (via assignment in notes) |
| **Apple Reminders** | `reminders` | Personal tasks & errands | Individual |
| **Drafts** | `drafts` | Quick notes, ideas, meeting notes | Individual |

## When to use each source

- **Code-related work** (bugs, features, refactors, infra) → GitHub Issue
- **Business/work tasks** (meetings prep, follow-ups, approvals, coordination) → Google Tasks
- **Personal errands** (shopping, appointments, personal reminders) → Apple Reminders
- **Quick capture / notes / ideas** → Drafts

## CLI Reference

### GitHub (existing `gh` CLI)

```bash
# List issues assigned to user
gh issue list --assignee @me --repo <owner/repo> --json number,title,labels,state

# List project board items
gh project item-list <project-number> --owner <org> --format json --limit 50

# Create issue
gh issue create --repo <owner/repo> --title "..." --body "..." --label "..."

# Close issue
gh issue close <number> --repo <owner/repo>
```

For SDPT project specifically:
- Project number: `1`, Owner: `Ecomulch`
- Statuses: 待办, 进行中, 待验收, 已完成
- See `agents/CONVENTIONS.md` §12 for field IDs and update commands

### Google Tasks (`gws tasks`)

```bash
# List all task lists
gws tasks tasklists list --format table

# List tasks in a specific list
gws tasks tasks list --params '{"tasklist":"<TASKLIST_ID>"}' --format table

# List tasks with due dates
gws tasks tasks list --params '{"tasklist":"<TASKLIST_ID>","dueMax":"<RFC3339>","showCompleted":false}' --format json

# Create a task
gws tasks tasks insert --params '{"tasklist":"<TASKLIST_ID>"}' --json '{"title":"...","notes":"...","due":"<RFC3339>"}'

# Complete a task
gws tasks tasks patch --params '{"tasklist":"<TASKLIST_ID>","task":"<TASK_ID>"}' --json '{"status":"completed"}'

# Delete a task
gws tasks tasks delete --params '{"tasklist":"<TASKLIST_ID>","task":"<TASK_ID>"}'
```

Note: If auth fails, ask the user to run `! gws auth login` to refresh.

### Apple Reminders (`reminders`)

```bash
# List all reminder lists
reminders lists [--format json|table]

# List reminders (incomplete by default)
reminders list [--list <name>] [--due today|tomorrow|week] [--all] [--format json|table]

# Add a reminder
reminders add "<title>" [--list <name>] [--due "today 5pm"|"tomorrow"|"2026-03-28"] [--notes "..."] [--priority high|medium|low]

# Mark as done
reminders done <id>

# Delete
reminders delete <id>
```

### Drafts (`drafts`)

```bash
# List drafts (inbox by default)
drafts list [--folder inbox|archive|all] [--tag <name>] [--flagged] [--limit N] [--format json|table]

# Search by content
drafts search "<query>" [--folder inbox|archive|all] [--limit N] [--format json|table]

# Read full content of a draft
drafts get <id> [--format json|table]

# Create a new draft
drafts create "<content>" [--tag <name>] [--flagged] [--format json|table]

# Append to existing draft
drafts append <id> "<text>"

# Flag/unflag
drafts flag <id>
drafts unflag <id>

# Archive or trash
drafts archive <id>
drafts trash <id>

# Add tag
drafts tag <id> <tag>
```

## Aggregated Queries

### "What are my tasks today?" / "今天有什么任务"

Run these three commands **in parallel** (use multiple Bash tool calls):

1. **GitHub**: `gh project item-list 1 --owner Ecomulch --format json --limit 50` → filter for 进行中/待办
2. **Google Tasks**: `gws tasks tasks list --params '{"tasklist":"<ID>","dueMax":"<today-end-RFC3339>","showCompleted":false}' --format json`
3. **Reminders**: `reminders list --due today --format json`

Then merge results and present as a unified table:

```
Source      | Priority | Task                          | Due
------------|----------|-------------------------------|--------
GitHub      | P1       | CashOps 应收逆向匹配            | -
Google      | -        | 准备周会材料                     | 14:00
Reminder    | HIGH     | 取消订阅                        | today
```

### "What notes/drafts do I have about X?"

```bash
drafts search "X" --folder all --limit 10 --format table
```

### "Show me all my open work"

Run in parallel:
1. `gh issue list --assignee @me --state open --json number,title,repository`
2. `gws tasks tasks list --params '{"tasklist":"<ID>","showCompleted":false}' --format json`
3. `reminders list --format json`

## Task Creation Routing

When the user says "add a task" or "remind me to...", determine the right source:

| Signal | Route to |
|--------|----------|
| Mentions a repo, bug, feature, PR, code | `gh issue create` |
| Work/business task, meeting, follow-up, coordination | `gws tasks tasks insert` |
| Personal errand, shopping, appointment | `reminders add` |
| Quick note, idea, thought to capture | `drafts create` |
| Ambiguous | Ask the user which source |

## Important Notes

- **No sync between sources** — each source is independent, the AI is the aggregation layer
- **Reminders output is JSON by default** — use `--format table` for human display
- **Drafts output is JSON by default** — use `--format table` for human display
- **Google Tasks auth may expire** — if you get a 401, tell the user to run `! gws auth login`
- **GitHub Project field IDs** — refer to `agents/CONVENTIONS.md` §12 for SDPT-specific field IDs
