package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Robben-Lu/projectai/internal/applescript"
	"github.com/Robben-Lu/projectai/internal/eventkit"
)

type fakeRunner struct {
	outputs map[string][]byte
	errors  map[string]error
}

func (f fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := commandKey(name, args)
	if err, ok := f.errors[key]; ok {
		return nil, err
	}
	if out, ok := f.outputs[key]; ok {
		return out, nil
	}
	return nil, errors.New("unexpected command: " + key)
}

func commandKey(name string, args []string) string {
	if name == "gh" {
		return "gh-project"
	}
	if name == "gws" && len(args) >= 3 && args[0] == "tasks" && args[1] == "tasklists" && args[2] == "list" {
		return "gws-tasklists"
	}
	if name == "gws" && len(args) >= 3 && args[0] == "tasks" && args[1] == "tasks" && args[2] == "list" {
		var params struct {
			Tasklist string `json:"tasklist"`
		}
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--params" {
				_ = json.Unmarshal([]byte(args[i+1]), &params)
				break
			}
		}
		return "gws-tasks:" + params.Tasklist
	}
	return name + " " + strings.Join(args, " ")
}

func TestTodayJSONAggregatesAndFilters(t *testing.T) {
	stdout, stderr, a := testApp(t, fakeRunner{
		outputs: map[string][]byte{
			"gh-project":         fixture(t, "gh_project.json"),
			"gws-tasklists":      fixture(t, "gws_tasklists.json"),
			"gws-tasks:work":     fixture(t, "gws_tasks_work.json"),
			"gws-tasks:personal": fixture(t, "gws_tasks_personal.json"),
		},
	})

	err := a.run(context.Background(), []string{"today", "--source", "github,gtasks", "--format", "json"})
	if err != nil {
		t.Fatalf("run today: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}

	items := decodeItems(t, stdout.Bytes())
	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d: %#v", len(items), items)
	}
	assertHasItem(t, items, "#488 [P0] CashOps V13-V17 detail repair", "none")
	assertHasItem(t, items, "#489 [P1] WorkForce overdue acceptance", "overdue")
	assertHasItem(t, items, "Overdue work task", "overdue")
	assertHasItem(t, items, "Shanghai midnight boundary", "today")
	assertHasItem(t, items, "No due work task", "none")
	assertMissingItem(t, items, "Backlog item")
	assertMissingItem(t, items, "Future out of window")
	assertMissingItem(t, items, "Completed task")
}

func TestOverdueShowsOnlyOverdueItems(t *testing.T) {
	stdout, _, a := testApp(t, fakeRunner{
		outputs: map[string][]byte{
			"gh-project":         fixture(t, "gh_project.json"),
			"gws-tasklists":      fixture(t, "gws_tasklists.json"),
			"gws-tasks:work":     fixture(t, "gws_tasks_work.json"),
			"gws-tasks:personal": fixture(t, "gws_tasks_personal.json"),
		},
	})

	err := a.run(context.Background(), []string{"overdue", "--source", "github,gtasks", "--format", "json"})
	if err != nil {
		t.Fatalf("run overdue: %v", err)
	}

	items := decodeItems(t, stdout.Bytes())
	if len(items) != 2 {
		t.Fatalf("expected 2 overdue items, got %d: %#v", len(items), items)
	}
	assertHasItem(t, items, "#489 [P1] WorkForce overdue acceptance", "overdue")
	assertHasItem(t, items, "Overdue work task", "overdue")
	assertMissingItem(t, items, "#488 [P0] CashOps V13-V17 detail repair")
	assertMissingItem(t, items, "Shanghai midnight boundary")
	assertMissingItem(t, items, "No due work task")
}

func TestTodayWarnsAndContinuesWhenSourceFails(t *testing.T) {
	stdout, stderr, a := testApp(t, fakeRunner{
		errors: map[string]error{
			"gws-tasklists": errors.New("gws not found in PATH"),
		},
	})
	a.getReminders = func() ([]eventkit.Reminder, error) {
		return []eventkit.Reminder{{
			ID:      "r1",
			Title:   "Pay phone bill",
			List:    "Personal",
			DueDate: "2026-05-06T09:00:00+08:00",
		}}, nil
	}

	err := a.run(context.Background(), []string{"today", "--source", "gtasks,reminders", "--format", "json"})
	if err != nil {
		t.Fatalf("run today: %v", err)
	}
	if !strings.Contains(stderr.String(), "warn: gtasks: gws not found in PATH") {
		t.Fatalf("expected gtasks warning, got: %s", stderr.String())
	}

	items := decodeItems(t, stdout.Bytes())
	if len(items) != 1 {
		t.Fatalf("expected reminder item only, got %d: %#v", len(items), items)
	}
	assertHasItem(t, items, "Pay phone bill", "overdue")
}

func TestTodayEmptyData(t *testing.T) {
	stdout, stderr, a := testApp(t, fakeRunner{
		outputs: map[string][]byte{
			"gh-project":    fixture(t, "empty_project.json"),
			"gws-tasklists": fixture(t, "empty_tasklists.json"),
		},
	})

	err := a.run(context.Background(), []string{"today", "--source", "github,gtasks,reminders,drafts", "--format", "json"})
	if err != nil {
		t.Fatalf("run today: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	items := decodeItems(t, stdout.Bytes())
	if len(items) != 0 {
		t.Fatalf("expected empty result, got %#v", items)
	}
}

func TestGHFilters(t *testing.T) {
	stdout, _, a := testApp(t, fakeRunner{
		outputs: map[string][]byte{
			"gh-project": fixture(t, "gh_project.json"),
		},
	})

	err := a.run(context.Background(), []string{"gh", "--status", "已完成", "--priority", "P1", "--system", "WorkForce", "--format", "json"})
	if err != nil {
		t.Fatalf("run gh: %v", err)
	}

	items := decodeItems(t, stdout.Bytes())
	if len(items) != 1 {
		t.Fatalf("expected one filtered GH item, got %d: %#v", len(items), items)
	}
	assertHasItem(t, items, "#489 [P1] WorkForce overdue acceptance", "overdue")
}

func testApp(t *testing.T, runner fakeRunner) (*bytes.Buffer, *bytes.Buffer, *app) {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	a := newApp(stdout, stderr)
	a.runner = runner
	a.location = loc
	a.now = func() time.Time {
		return time.Date(2026, 5, 7, 10, 0, 0, 0, loc)
	}
	a.getReminders = func() ([]eventkit.Reminder, error) {
		return []eventkit.Reminder{}, nil
	}
	a.listDrafts = func() ([]applescript.Draft, error) {
		return []applescript.Draft{}, nil
	}
	return stdout, stderr, a
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeItems(t *testing.T, data []byte) []taskItem {
	t.Helper()
	var items []taskItem
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("decode items: %v\n%s", err, string(data))
	}
	return items
}

func assertHasItem(t *testing.T, items []taskItem, title string, dueState string) {
	t.Helper()
	for _, item := range items {
		if item.Title == title {
			if item.DueState != dueState {
				t.Fatalf("item %q due_state: got %q want %q", title, item.DueState, dueState)
			}
			return
		}
	}
	t.Fatalf("missing item %q in %#v", title, items)
}

func assertMissingItem(t *testing.T, items []taskItem, title string) {
	t.Helper()
	for _, item := range items {
		if item.Title == title || strings.Contains(item.Title, title) {
			t.Fatalf("unexpected item %q in %#v", title, items)
		}
	}
}
