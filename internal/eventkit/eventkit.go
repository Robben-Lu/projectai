// Package eventkit provides a fast bridge to Apple Reminders via a compiled Swift helper.
// This replaces the AppleScript approach (~500x faster) by using EventKit directly.
package eventkit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Reminder represents a single Apple Reminder item.
type Reminder struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	List      string `json:"list"`
	DueDate   string `json:"due,omitempty"`
	Priority  int    `json:"priority"`
	Completed bool   `json:"completed"`
	Notes     string `json:"notes,omitempty"`
}

// ReminderList represents a Reminders list.
type ReminderList struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// helperPath finds the reminders-helper binary.
// Looks in: same dir as executable, then PATH.
func helperPath() (string, error) {
	// 1. Same directory as the running binary
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "reminders-helper")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// 2. PATH lookup
	if p, err := exec.LookPath("reminders-helper"); err == nil {
		return p, nil
	}

	return "", fmt.Errorf("reminders-helper not found (install it next to the reminders binary or in PATH)")
}

func runHelper(args ...string) ([]byte, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("reminders-helper requires macOS")
	}
	hp, err := helperPath()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(hp, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("reminders-helper %v: %w", args, err)
	}
	return out, nil
}

// GetLists returns all reminder lists.
func GetLists() ([]ReminderList, error) {
	out, err := runHelper("lists")
	if err != nil {
		return nil, err
	}
	var lists []ReminderList
	if err := json.Unmarshal(out, &lists); err != nil {
		return nil, fmt.Errorf("parse lists: %w", err)
	}
	return lists, nil
}

// GetReminders returns reminders, optionally filtered by list and completion status.
func GetReminders(listName string, onlyIncomplete bool) ([]Reminder, error) {
	args := []string{"list"}
	if listName != "" {
		args = append(args, "--list", listName)
	}
	if onlyIncomplete {
		args = append(args, "--incomplete")
	}
	out, err := runHelper(args...)
	if err != nil {
		return nil, err
	}
	var reminders []Reminder
	if err := json.Unmarshal(out, &reminders); err != nil {
		return nil, fmt.Errorf("parse reminders: %w", err)
	}
	return reminders, nil
}

// AddReminder creates a new reminder.
func AddReminder(listName, title, notes, due string, priority int) (string, error) {
	args := []string{"add", "--list", listName, "--title", title}
	if notes != "" {
		args = append(args, "--notes", notes)
	}
	if due != "" {
		args = append(args, "--due", due)
	}
	if priority > 0 {
		args = append(args, "--priority", fmt.Sprintf("%d", priority))
	}
	out, err := runHelper(args...)
	if err != nil {
		return "", err
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", err
	}
	return result.ID, nil
}

// CompleteReminder marks a reminder as completed.
func CompleteReminder(id string) error {
	_, err := runHelper("done", id)
	return err
}

// DeleteReminder removes a reminder.
func DeleteReminder(id string) error {
	_, err := runHelper("delete", id)
	return err
}
