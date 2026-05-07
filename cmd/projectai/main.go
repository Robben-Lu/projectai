package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Robben-Lu/projectai/internal/applescript"
	"github.com/Robben-Lu/projectai/internal/eventkit"
	"github.com/fatih/color"
)

const usage = `projectai - cross-source task aggregation

Usage:
  projectai today [--window 7d] [--source github,gtasks,reminders,drafts] [--format table|json|ndjson]
  projectai overdue [--source github,gtasks,reminders,drafts] [--format table|json|ndjson]
  projectai gh [--status xxx] [--priority Pn] [--system xxx] [--format table|json|ndjson]

Flags:
  --window <duration>   Future due window, default 7d. 0d shows overdue only.
  --source <list>       Comma-separated sources: github, gtasks, reminders, drafts.
  --format <format>     table, json, or ndjson. Default: table.
  --owner <org>         GitHub Project owner. Default: PROJECTAI_GH_OWNER or Ecomulch.
  --project <num>       GitHub Project number. Default: 1.
`

const (
	sourceGitHub    = "github"
	sourceGTasks    = "gtasks"
	sourceReminders = "reminders"
	sourceDrafts    = "drafts"

	formatTable  = "table"
	formatJSON   = "json"
	formatNDJSON = "ndjson"

	overdueLookback = 30 * 24 * time.Hour
)

var (
	defaultGitHubStatuses = map[string]bool{
		"进行中":         true,
		"in progress": true,
		"待验收":         true,
	}
	defaultGitHubPriorities = map[string]bool{
		"P0": true,
		"P1": true,
	}
)

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath(name); err != nil {
		return nil, fmt.Errorf("%s not found in PATH", name)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.Bytes(), fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

type app struct {
	stdout       io.Writer
	stderr       io.Writer
	runner       commandRunner
	now          func() time.Time
	location     *time.Location
	getReminders func() ([]eventkit.Reminder, error)
	listDrafts   func() ([]applescript.Draft, error)
}

type todayOptions struct {
	window  string
	sources map[string]bool
	format  string
	owner   string
	project int
}

type ghOptions struct {
	format   string
	owner    string
	project  int
	status   string
	priority string
	system   string
}

type taskItem struct {
	Source   string     `json:"source"`
	Status   string     `json:"status"`
	Due      *time.Time `json:"due,omitempty"`
	DueState string     `json:"due_state,omitempty"`
	Title    string     `json:"title"`
	Link     string     `json:"link,omitempty"`
	Priority string     `json:"priority,omitempty"`
	System   string     `json:"system,omitempty"`
	List     string     `json:"list,omitempty"`
	ID       string     `json:"id,omitempty"`
}

type githubFilters struct {
	defaultToday bool
	status       string
	priority     string
	system       string
}

func main() {
	a := newApp(os.Stdout, os.Stderr)
	if err := a.run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newApp(stdout, stderr io.Writer) *app {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return &app{
		stdout:   stdout,
		stderr:   stderr,
		runner:   execRunner{},
		now:      time.Now,
		location: loc,
		getReminders: func() ([]eventkit.Reminder, error) {
			return eventkit.GetReminders("", true)
		},
		listDrafts: func() ([]applescript.Draft, error) {
			return applescript.ListDrafts("inbox", "", true, 100)
		},
	}
}

func (a *app) run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Fprint(a.stderr, usage)
		return errors.New("command is required")
	}

	switch args[0] {
	case "today":
		opts, err := a.parseTodayOptions("today", args[1:], "7d")
		if err != nil {
			return err
		}
		return a.runToday(ctx, opts)
	case "overdue":
		opts, err := a.parseTodayOptions("overdue", args[1:], "0d")
		if err != nil {
			return err
		}
		return a.runToday(ctx, opts)
	case "gh":
		opts, err := a.parseGHOptions(args[1:])
		if err != nil {
			return err
		}
		return a.runGH(ctx, opts)
	case "help", "--help", "-h":
		fmt.Fprint(a.stdout, usage)
		return nil
	default:
		fmt.Fprintf(a.stderr, "unknown command: %s\n\n%s", args[0], usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (a *app) parseTodayOptions(name string, args []string, defaultWindow string) (todayOptions, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	window := fs.String("window", defaultWindow, "future due window")
	sourceList := fs.String("source", "", "comma-separated sources")
	format := fs.String("format", formatTable, "table, json, or ndjson")
	owner := fs.String("owner", defaultOwner(), "GitHub Project owner")
	project := fs.Int("project", 1, "GitHub Project number")

	if err := fs.Parse(args); err != nil {
		return todayOptions{}, err
	}

	sources, err := parseSources(*sourceList)
	if err != nil {
		return todayOptions{}, err
	}
	if err := validateFormat(*format); err != nil {
		return todayOptions{}, err
	}
	if _, err := parseWindow(*window); err != nil {
		return todayOptions{}, err
	}

	return todayOptions{
		window:  *window,
		sources: sources,
		format:  *format,
		owner:   *owner,
		project: *project,
	}, nil
}

func (a *app) parseGHOptions(args []string) (ghOptions, error) {
	fs := flag.NewFlagSet("gh", flag.ContinueOnError)
	fs.SetOutput(a.stderr)

	status := fs.String("status", "", "GitHub Project status")
	priority := fs.String("priority", "", "GitHub Project priority")
	system := fs.String("system", "", "GitHub Project system")
	format := fs.String("format", formatTable, "table, json, or ndjson")
	owner := fs.String("owner", defaultOwner(), "GitHub Project owner")
	project := fs.Int("project", 1, "GitHub Project number")

	if err := fs.Parse(args); err != nil {
		return ghOptions{}, err
	}
	if err := validateFormat(*format); err != nil {
		return ghOptions{}, err
	}

	return ghOptions{
		format:   *format,
		owner:    *owner,
		project:  *project,
		status:   *status,
		priority: normalizePriority(*priority),
		system:   *system,
	}, nil
}

func defaultOwner() string {
	if owner := strings.TrimSpace(os.Getenv("PROJECTAI_GH_OWNER")); owner != "" {
		return owner
	}
	return "Ecomulch"
}

func validateFormat(format string) error {
	switch format {
	case formatTable, formatJSON, formatNDJSON:
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func parseSources(raw string) (map[string]bool, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]bool{
			sourceGitHub:    true,
			sourceGTasks:    true,
			sourceReminders: true,
			sourceDrafts:    true,
		}, nil
	}

	sources := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		alias := strings.ToLower(strings.TrimSpace(part))
		if alias == "" {
			continue
		}
		if alias == "all" {
			return parseSources("")
		}
		source, ok := normalizeSource(alias)
		if !ok {
			return nil, fmt.Errorf("unsupported source %q", part)
		}
		sources[source] = true
	}
	return sources, nil
}

func normalizeSource(alias string) (string, bool) {
	switch alias {
	case "github", "gh":
		return sourceGitHub, true
	case "gtasks", "gws", "google", "google-tasks", "tasks":
		return sourceGTasks, true
	case "reminders", "reminder", "rmd":
		return sourceReminders, true
	case "drafts", "draft":
		return sourceDrafts, true
	default:
		return "", false
	}
}

func parseWindow(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return 0, fmt.Errorf("window is required")
	}
	if strings.HasSuffix(raw, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(raw, "d"))
		if err != nil || days < 0 {
			return 0, fmt.Errorf("invalid day window %q", raw)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid window %q", raw)
	}
	return d, nil
}

func (a *app) runToday(ctx context.Context, opts todayOptions) error {
	window, err := parseWindow(opts.window)
	if err != nil {
		return err
	}

	var items []taskItem
	if opts.sources[sourceGitHub] {
		ghItems, err := a.fetchGitHub(ctx, opts.owner, opts.project, githubFilters{defaultToday: true})
		if err != nil {
			a.warnf("github: %v", err)
		} else {
			items = append(items, ghItems...)
		}
	}
	if opts.sources[sourceGTasks] {
		gtasks, err := a.fetchGTasks(ctx, window)
		if err != nil {
			a.warnf("gtasks: %v", err)
		} else {
			items = append(items, gtasks...)
		}
	}
	if opts.sources[sourceReminders] {
		reminders, err := a.fetchReminders(window)
		if err != nil {
			a.warnf("reminders: %v", err)
		} else {
			items = append(items, reminders...)
		}
	}
	if opts.sources[sourceDrafts] {
		drafts, err := a.fetchDrafts()
		if err != nil {
			a.warnf("drafts: %v", err)
		} else {
			items = append(items, drafts...)
		}
	}
	if window == 0 {
		items = filterOverdueOnly(items, a.todayStart())
	}

	a.sortItems(items)
	return a.printItems(items, opts.format)
}

func (a *app) runGH(ctx context.Context, opts ghOptions) error {
	filter := githubFilters{
		defaultToday: opts.status == "" && opts.priority == "" && opts.system == "",
		status:       opts.status,
		priority:     opts.priority,
		system:       opts.system,
	}
	items, err := a.fetchGitHub(ctx, opts.owner, opts.project, filter)
	if err != nil {
		return err
	}
	a.sortItems(items)
	return a.printItems(items, opts.format)
}

func (a *app) fetchGitHub(ctx context.Context, owner string, project int, filters githubFilters) ([]taskItem, error) {
	out, err := a.runner.Run(ctx, "gh", "project", "item-list", strconv.Itoa(project), "--owner", owner, "--format", "json", "--limit", "200")
	if err != nil {
		return nil, err
	}

	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse project items: %w", err)
	}

	items := make([]taskItem, 0, len(payload.Items))
	for _, raw := range payload.Items {
		item := githubItemFromMap(raw)
		if item.Title == "" {
			continue
		}
		if !matchesGitHubFilters(item, filters) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func githubItemFromMap(raw map[string]any) taskItem {
	content := mapFromAny(raw["content"])

	status := firstString(raw, "status", "Status")
	priority := firstString(raw, "priority", "Priority")
	if priority == "" {
		priority = priorityFromLabels(raw["labels"])
	}
	if priority == "" {
		priority = priorityFromLabels(content["labels"])
	}
	priority = normalizePriority(priority)

	system := firstString(raw, "system", "System")
	title := firstString(raw, "title", "Title")
	if title == "" {
		title = firstString(content, "title", "Title")
	}

	number := intFromAny(raw["number"])
	if number == 0 {
		number = intFromAny(content["number"])
	}
	displayTitle := title
	if priority != "" && !strings.Contains(displayTitle, "["+priority+"]") {
		displayTitle = "[" + priority + "] " + displayTitle
	}
	if number > 0 && !strings.HasPrefix(displayTitle, "#") {
		displayTitle = fmt.Sprintf("#%d %s", number, displayTitle)
	}

	link := firstString(raw, "url", "URL", "link")
	if link == "" {
		link = firstString(content, "url", "URL", "html_url")
	}

	due := parseDateFromMap(raw, "due", "Due", "dueDate", "Due Date", "deadline", "Deadline")

	return taskItem{
		Source:   sourceGitHub,
		Status:   status,
		Due:      due,
		DueState: dueState(due, defaultLocationNow()),
		Title:    displayTitle,
		Link:     link,
		Priority: priority,
		System:   system,
		ID:       firstString(raw, "id", "ID"),
	}
}

func matchesGitHubFilters(item taskItem, filters githubFilters) bool {
	if filters.status != "" && item.Status != filters.status {
		return false
	}
	if filters.priority != "" && strings.ToUpper(item.Priority) != strings.ToUpper(filters.priority) {
		return false
	}
	if filters.system != "" && !strings.EqualFold(item.System, filters.system) {
		return false
	}
	if filters.status != "" || filters.priority != "" || filters.system != "" {
		return true
	}
	if !filters.defaultToday {
		return true
	}
	return defaultGitHubStatuses[strings.ToLower(item.Status)] || defaultGitHubPriorities[strings.ToUpper(item.Priority)]
}

func (a *app) fetchGTasks(ctx context.Context, window time.Duration) ([]taskItem, error) {
	out, err := a.runner.Run(ctx, "gws", "tasks", "tasklists", "list", "--format", "json")
	if err != nil {
		return nil, err
	}

	var lists struct {
		Items []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &lists); err != nil {
		return nil, fmt.Errorf("parse task lists: %w", err)
	}

	var all []taskItem
	for _, list := range lists.Items {
		if list.ID == "" {
			continue
		}
		params, _ := json.Marshal(map[string]any{
			"tasklist":      list.ID,
			"showCompleted": false,
		})
		out, err := a.runner.Run(ctx, "gws", "tasks", "tasks", "list", "--params", string(params), "--format", "json")
		if err != nil {
			a.warnf("gtasks list %q: %v", list.Title, err)
			continue
		}

		items, err := a.parseGTasks(list.Title, out, window)
		if err != nil {
			a.warnf("gtasks list %q: %v", list.Title, err)
			continue
		}
		all = append(all, items...)
	}
	return all, nil
}

func (a *app) parseGTasks(listTitle string, out []byte, window time.Duration) ([]taskItem, error) {
	var payload struct {
		Items []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Status      string `json:"status"`
			Due         string `json:"due"`
			SelfLink    string `json:"selfLink"`
			WebViewLink string `json:"webViewLink"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse tasks: %w", err)
	}

	items := make([]taskItem, 0, len(payload.Items))
	for _, task := range payload.Items {
		if task.Title == "" || task.Status == "completed" {
			continue
		}
		var due *time.Time
		if task.Due != "" {
			parsed, err := parseFlexibleTime(task.Due, a.location)
			if err == nil {
				due = &parsed
			}
		}
		if !includeDueOrOpen(due, window, a.todayStart()) {
			continue
		}

		link := task.WebViewLink
		if link == "" {
			link = task.SelfLink
		}
		items = append(items, taskItem{
			Source:   sourceGTasks,
			Status:   valueOr(task.Status, "needsAction"),
			Due:      due,
			DueState: dueState(due, a.now().In(a.location)),
			Title:    task.Title,
			Link:     link,
			List:     listTitle,
			ID:       task.ID,
		})
	}
	return items, nil
}

func (a *app) fetchReminders(window time.Duration) ([]taskItem, error) {
	reminders, err := a.getReminders()
	if err != nil {
		return nil, err
	}

	items := make([]taskItem, 0, len(reminders))
	for _, reminder := range reminders {
		if reminder.Completed || reminder.Title == "" || reminder.DueDate == "" {
			continue
		}
		due, err := parseFlexibleTime(reminder.DueDate, a.location)
		if err != nil {
			continue
		}
		if !includeDueOnly(&due, window, a.todayStart()) {
			continue
		}
		items = append(items, taskItem{
			Source:   sourceReminders,
			Status:   "open",
			Due:      &due,
			DueState: dueState(&due, a.now().In(a.location)),
			Title:    reminder.Title,
			List:     reminder.List,
			ID:       reminder.ID,
		})
	}
	return items, nil
}

func (a *app) fetchDrafts() ([]taskItem, error) {
	drafts, err := a.listDrafts()
	if err != nil {
		return nil, err
	}

	items := make([]taskItem, 0, len(drafts))
	for _, draft := range drafts {
		if !draft.Flagged || draft.Title == "" {
			continue
		}
		items = append(items, taskItem{
			Source: sourceDrafts,
			Status: "flagged",
			Title:  draft.Title,
			Link:   draft.Permalink,
			ID:     draft.ID,
		})
	}
	return items, nil
}

func includeDueOnly(due *time.Time, window time.Duration, today time.Time) bool {
	if due == nil {
		return false
	}
	localDue := due.In(today.Location())
	if localDue.Before(today.Add(-overdueLookback)) {
		return false
	}
	if window == 0 {
		return localDue.Before(today)
	}
	if localDue.Before(today) {
		return true
	}
	return localDue.Before(today.Add(window).Add(24 * time.Hour))
}

func includeDueOrOpen(due *time.Time, window time.Duration, today time.Time) bool {
	if due == nil {
		return window > 0
	}
	return includeDueOnly(due, window, today)
}

func filterOverdueOnly(items []taskItem, today time.Time) []taskItem {
	filtered := items[:0]
	for _, item := range items {
		if item.Due != nil && item.Due.In(today.Location()).Before(today) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (a *app) todayStart() time.Time {
	now := a.now().In(a.location)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, a.location)
}

func (a *app) sortItems(items []taskItem) {
	now := a.now().In(a.location)
	for i := range items {
		items[i].DueState = dueState(items[i].Due, now)
	}
	sort.SliceStable(items, func(i, j int) bool {
		aBucket, bBucket := sortBucket(items[i]), sortBucket(items[j])
		if aBucket != bBucket {
			return aBucket < bBucket
		}
		if items[i].Due != nil && items[j].Due != nil && !items[i].Due.Equal(*items[j].Due) {
			return items[i].Due.Before(*items[j].Due)
		}
		if items[i].Source != items[j].Source {
			return sourceRank(items[i].Source) < sourceRank(items[j].Source)
		}
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})
}

func sortBucket(item taskItem) int {
	switch item.DueState {
	case "overdue":
		return 0
	case "today":
		return 1
	case "upcoming":
		return 2
	default:
		if item.Source == sourceGitHub {
			return 3
		}
		return 4
	}
}

func sourceRank(source string) int {
	switch source {
	case sourceGitHub:
		return 0
	case sourceGTasks:
		return 1
	case sourceReminders:
		return 2
	case sourceDrafts:
		return 3
	default:
		return 9
	}
}

func (a *app) printItems(items []taskItem, format string) error {
	if items == nil {
		items = []taskItem{}
	}
	switch format {
	case formatTable:
		return a.printTable(items)
	case formatJSON:
		enc := json.NewEncoder(a.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	case formatNDJSON:
		enc := json.NewEncoder(a.stdout)
		for _, item := range items {
			if err := enc.Encode(item); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func (a *app) printTable(items []taskItem) error {
	tw := tabwriter.NewWriter(a.stdout, 0, 0, 2, ' ', 0)
	header := color.New(color.FgCyan, color.Bold).SprintFunc()
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", header("Source"), header("Status"), header("Due"), header("Title"), header("Link"))
	for _, item := range items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			tableSource(item.Source),
			valueOr(item.Status, "-"),
			tableDue(item.Due, item.DueState),
			item.Title,
			valueOr(item.Link, "-"),
		)
	}
	return tw.Flush()
}

func tableSource(source string) string {
	switch source {
	case sourceGitHub:
		return "GH"
	case sourceGTasks:
		return "GTasks"
	case sourceReminders:
		return "RMD"
	case sourceDrafts:
		return "Drafts"
	default:
		return source
	}
}

func tableDue(due *time.Time, state string) string {
	switch state {
	case "overdue":
		return color.New(color.FgRed).Sprint("OVERDUE")
	case "today":
		return color.New(color.FgYellow).Sprint("TODAY")
	}
	if due == nil {
		return "-"
	}
	return due.Format("2006-01-02")
}

func dueState(due *time.Time, now time.Time) string {
	if due == nil {
		return "none"
	}
	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	localDue := due.In(loc)
	if localDue.Before(today) {
		return "overdue"
	}
	if localDue.Before(today.AddDate(0, 0, 1)) {
		return "today"
	}
	return "upcoming"
}

func (a *app) warnf(format string, args ...any) {
	fmt.Fprintf(a.stderr, "warn: "+format+"\n", args...)
}

func parseFlexibleTime(raw string, loc *time.Location) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	} {
		if layout == "2006-01-02T15:04:05" || layout == "2006-01-02 15:04:05" || layout == "2006-01-02 15:04" || layout == "2006-01-02" {
			if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
				return t, nil
			}
			continue
		}
		if t, err := time.Parse(layout, raw); err == nil {
			return t.In(loc), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", raw)
}

func parseDateFromMap(raw map[string]any, keys ...string) *time.Time {
	for _, key := range keys {
		value := strings.TrimSpace(stringFromAny(raw[key]))
		if value == "" {
			continue
		}
		t, err := parseFlexibleTime(value, defaultLocation())
		if err == nil {
			return &t
		}
	}
	return nil
}

func defaultLocationNow() time.Time {
	return time.Now().In(defaultLocation())
}

func defaultLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}

func mapFromAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := strings.TrimSpace(stringFromAny(m[key])); s != "" {
			return s
		}
	}
	return ""
}

func stringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, _ := strconv.Atoi(t)
		return n
	default:
		return 0
	}
}

func priorityFromLabels(v any) string {
	labels, ok := v.([]any)
	if !ok {
		return ""
	}
	for _, raw := range labels {
		label := ""
		switch t := raw.(type) {
		case string:
			label = t
		case map[string]any:
			label = firstString(t, "name", "Name")
		}
		label = strings.TrimSpace(label)
		if priority := normalizePriority(label); isKnownPriority(priority) {
			return priority
		}
		upper := strings.ToUpper(label)
		if strings.HasPrefix(upper, "PRIORITY:") {
			if priority := normalizePriority(strings.TrimSpace(strings.TrimPrefix(upper, "PRIORITY:"))); isKnownPriority(priority) {
				return priority
			}
		}
	}
	return ""
}

func normalizePriority(raw string) string {
	raw = strings.TrimSpace(strings.ToUpper(raw))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "PRIORITY:") {
		return normalizePriority(strings.TrimSpace(strings.TrimPrefix(raw, "PRIORITY:")))
	}
	for _, priority := range []string{"P0", "P1", "P2", "P3"} {
		if raw == priority || strings.HasPrefix(raw, priority+" ") || strings.HasPrefix(raw, priority+"-") || strings.HasPrefix(raw, priority+":") {
			return priority
		}
	}
	return raw
}

func isKnownPriority(priority string) bool {
	switch priority {
	case "P0", "P1", "P2", "P3":
		return true
	default:
		return false
	}
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
