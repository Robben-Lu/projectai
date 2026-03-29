package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Robben-Lu/projectai/internal/eventkit"
)

const usage = `reminders — CLI for Apple Reminders

Usage:
  reminders lists                              List all reminder lists
  reminders list [--list <name>] [--due today|tomorrow|week] [--all]
  reminders add <title> [--list <name>] [--due <datetime>] [--notes <text>] [--priority high|medium|low]
  reminders done <id>                          Mark reminder as completed
  reminders delete <id>                        Delete a reminder

Flags:
  --format json|table    Output format (default: json)
  --all                  Include completed reminders
  --help                 Show this help
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "lists":
		cmdLists()
	case "list":
		cmdList()
	case "add":
		cmdAdd()
	case "done":
		cmdDone()
	case "delete":
		cmdDelete()
	case "--help", "-h", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

func cmdLists() {
	fs := flag.NewFlagSet("lists", flag.ExitOnError)
	format := fs.String("format", "json", "Output format: json or table")
	fs.Parse(os.Args[2:])

	lists, err := eventkit.GetLists()
	exitOnErr(err)

	if *format == "table" {
		for _, l := range lists {
			fmt.Println(l.Name)
		}
		return
	}
	printJSON(lists)
}

func cmdList() {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	listName := fs.String("list", "", "Filter by list name")
	due := fs.String("due", "", "Filter by due: today, tomorrow, week")
	all := fs.Bool("all", false, "Include completed reminders")
	format := fs.String("format", "json", "Output format: json or table")
	fs.Parse(os.Args[2:])

	ekReminders, err := eventkit.GetReminders(*listName, !*all)
	exitOnErr(err)

	// Convert to local Reminder type for filtering/display
	reminders := make([]reminder, len(ekReminders))
	for i, r := range ekReminders {
		reminders[i] = reminder{
			ID:        r.ID,
			Title:     r.Title,
			List:      r.List,
			Priority:  r.Priority,
			Completed: r.Completed,
			Notes:     r.Notes,
		}
		if r.DueDate != "" {
			if t, err := time.Parse(time.RFC3339, r.DueDate); err == nil {
				reminders[i].DueDate = &t
			} else if t, err := time.Parse("2006-01-02T15:04:05Z", r.DueDate); err == nil {
				reminders[i].DueDate = &t
			}
		}
	}

	// Filter by due date if specified
	if *due != "" {
		reminders = filterByDue(reminders, *due)
	}

	if *format == "table" {
		printTable(reminders)
		return
	}
	printJSON(reminders)
}

func cmdAdd() {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	listName := fs.String("list", "", "Reminder list (default: Reminders)")
	dueStr := fs.String("due", "", "Due date/time (e.g., 'today 5pm', '2026-03-28', 'tomorrow')")
	notes := fs.String("notes", "", "Notes/description")
	priorityStr := fs.String("priority", "", "Priority: high, medium, low")
	format := fs.String("format", "json", "Output format: json or table")
	fs.Parse(os.Args[2:])

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "error: title is required")
		fmt.Fprintf(os.Stderr, "usage: reminders add <title> [flags]\n")
		os.Exit(1)
	}
	title := strings.Join(fs.Args(), " ")

	var dueISO string
	if *dueStr != "" {
		t, err := parseDue(*dueStr)
		exitOnErr(err)
		dueISO = t.Format(time.RFC3339)
	}

	priority := parsePriority(*priorityStr)
	ln := *listName
	if ln == "" {
		ln = "Reminders"
	}

	id, err := eventkit.AddReminder(ln, title, *notes, dueISO, priority)
	exitOnErr(err)

	result := map[string]string{"id": id, "title": title, "status": "created"}
	if *format == "table" {
		fmt.Printf("Created: %s (id: %s)\n", title, id)
		return
	}
	printJSON(result)
}

func cmdDone() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "error: reminder id is required")
		fmt.Fprintf(os.Stderr, "usage: reminders done <id>\n")
		os.Exit(1)
	}
	id := os.Args[2]
	err := eventkit.CompleteReminder(id)
	exitOnErr(err)
	printJSON(map[string]string{"id": id, "status": "completed"})
}

func cmdDelete() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "error: reminder id is required")
		fmt.Fprintf(os.Stderr, "usage: reminders delete <id>\n")
		os.Exit(1)
	}
	id := os.Args[2]
	err := eventkit.DeleteReminder(id)
	exitOnErr(err)
	printJSON(map[string]string{"id": id, "status": "deleted"})
}

// reminder is a local type for filtering and display.
type reminder struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	List      string     `json:"list"`
	DueDate   *time.Time `json:"due,omitempty"`
	Priority  int        `json:"priority"`
	Completed bool       `json:"completed"`
	Notes     string     `json:"notes,omitempty"`
}

func filterByDue(reminders []reminder, due string) []reminder {
	now := time.Now()
	var start, end time.Time

	switch due {
	case "today":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 1)
	case "tomorrow":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
		end = start.AddDate(0, 0, 1)
	case "week":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 7)
	default:
		return reminders // unknown filter, return all
	}

	var filtered []reminder
	for _, r := range reminders {
		if r.DueDate != nil && !r.DueDate.Before(start) && r.DueDate.Before(end) {
			filtered = append(filtered, r)
		}
		// Also include overdue items for "today"
		if due == "today" && r.DueDate != nil && r.DueDate.Before(start) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func parseDue(s string) (time.Time, error) {
	now := time.Now()
	loc := now.Location()

	switch strings.ToLower(s) {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, loc), nil
	case "tomorrow":
		t := now.AddDate(0, 0, 1)
		return time.Date(t.Year(), t.Month(), t.Day(), 9, 0, 0, 0, loc), nil
	}

	// Try "today 5pm" format
	if strings.HasPrefix(strings.ToLower(s), "today ") {
		timeStr := strings.TrimPrefix(strings.ToLower(s), "today ")
		t, err := parseTimeOfDay(timeStr)
		if err == nil {
			return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
		}
	}

	// Try "tomorrow 5pm" format
	if strings.HasPrefix(strings.ToLower(s), "tomorrow ") {
		timeStr := strings.TrimPrefix(strings.ToLower(s), "tomorrow ")
		t, err := parseTimeOfDay(timeStr)
		if err == nil {
			tm := now.AddDate(0, 0, 1)
			return time.Date(tm.Year(), tm.Month(), tm.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
		}
	}

	// Try ISO date
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		return t.Add(9 * time.Hour), nil // default to 9am
	}

	// Try ISO datetime
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, loc); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("cannot parse due date: %q (try: today, tomorrow, today 5pm, 2026-03-28)", s)
}

func parseTimeOfDay(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	// "5pm", "5:30pm", "17:00"
	for _, layout := range []string{"3pm", "3:04pm", "15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %q", s)
}

func parsePriority(s string) int {
	switch strings.ToLower(s) {
	case "high":
		return 1
	case "medium":
		return 5
	case "low":
		return 9
	default:
		return 0 // none
	}
}

func printTable(reminders []reminder) {
	for _, r := range reminders {
		due := ""
		if r.DueDate != nil {
			due = r.DueDate.Format("2006-01-02 15:04")
		}
		pri := ""
		switch r.Priority {
		case 1:
			pri = "HIGH"
		case 5:
			pri = "MED "
		case 9:
			pri = "LOW "
		}
		status := " "
		if r.Completed {
			status = "x"
		}
		fmt.Printf("[%s] %s%-10s %-16s %s\n", status, pri, r.List, due, r.Title)
	}
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
