package applescript

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Reminder represents a single Apple Reminder item.
type Reminder struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	List      string     `json:"list"`
	DueDate   *time.Time `json:"due,omitempty"`
	Priority  int        `json:"priority"` // 0=none, 1=high, 5=medium, 9=low (Apple's scheme)
	Completed bool       `json:"completed"`
	Notes     string     `json:"notes,omitempty"`
}

// ReminderList represents a Reminders list.
type ReminderList struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// GetLists returns all reminder lists.
func GetLists() ([]ReminderList, error) {
	script := `tell application "Reminders"
	set output to ""
	repeat with l in every list
		set output to output & id of l & "	" & name of l & linefeed
	end repeat
	return output
end tell`
	out, err := Run(script)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []ReminderList{}, nil
	}

	var lists []ReminderList
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		lists = append(lists, ReminderList{ID: parts[0], Name: parts[1]})
	}
	return lists, nil
}

// GetReminders returns reminders from a specific list (or all lists if listName is empty).
// If onlyIncomplete is true, only returns non-completed reminders.
func GetReminders(listName string, onlyIncomplete bool) ([]Reminder, error) {
	if listName != "" {
		return getRemindersFromList(listName, onlyIncomplete)
	}
	// No list specified: iterate each list to avoid global "every reminder" timeout
	lists, err := GetLists()
	if err != nil {
		return nil, err
	}
	var all []Reminder
	for _, l := range lists {
		rs, err := getRemindersFromList(l.Name, onlyIncomplete)
		if err != nil {
			return nil, fmt.Errorf("list %q: %w", l.Name, err)
		}
		all = append(all, rs...)
	}
	return all, nil
}

// getRemindersFromList fetches reminders from one list using a single query + per-item loop.
func getRemindersFromList(listName string, onlyIncomplete bool) ([]Reminder, error) {
	var filter string
	if onlyIncomplete {
		filter = "whose completed is false"
	}

	// Single query, then per-item property access in a loop.
	// This is faster than multiple bulk queries because each "whose" filter costs ~4s.
	script := fmt.Sprintf(`tell application "Reminders"
	set output to ""
	set targetReminders to (every reminder in list %q %s)
	repeat with r in targetReminders
		set rId to id of r
		set rName to name of r
		set rCompleted to completed of r
		set rPriority to priority of r
		try
			set rDue to due date of r as «class isot» as string
		on error
			set rDue to ""
		end try
		try
			set rBody to body of r
			if rBody is missing value then set rBody to ""
		on error
			set rBody to ""
		end try
		set output to output & rId & "	" & rName & "	" & %q & "	" & rCompleted & "	" & rPriority & "	" & rDue & "	" & rBody & linefeed
	end repeat
	return output
end tell`, listName, filter, listName)

	out, err := Run(script)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []Reminder{}, nil
	}

	var reminders []Reminder
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 7)
		if len(parts) < 5 {
			continue
		}

		r := Reminder{
			ID:        parts[0],
			Title:     parts[1],
			List:      parts[2],
			Completed: parts[3] == "true",
		}

		// Parse priority
		fmt.Sscanf(parts[4], "%d", &r.Priority)

		// Parse due date
		if len(parts) > 5 && parts[5] != "" {
			if t, err := time.Parse("2006-01-02T15:04:05", parts[5]); err == nil {
				r.DueDate = &t
			}
		}

		// Notes
		if len(parts) > 6 {
			r.Notes = parts[6]
		}

		reminders = append(reminders, r)
	}
	return reminders, nil
}

// AddReminder creates a new reminder in the specified list.
func AddReminder(listName, title, notes string, dueDate *time.Time, priority int) (string, error) {
	if listName == "" {
		listName = "Reminders" // default list
	}

	props := fmt.Sprintf(`{name:%q`, title)
	if notes != "" {
		props += fmt.Sprintf(`, body:%q`, notes)
	}
	if priority > 0 {
		props += fmt.Sprintf(`, priority:%d`, priority)
	}
	if dueDate != nil {
		props += fmt.Sprintf(`, due date:date "%s"`, dueDate.Format("January 2, 2006 3:04:05 PM"))
	}
	props += "}"

	script := fmt.Sprintf(`tell application "Reminders"
	set newReminder to make new reminder in list %q with properties %s
	return id of newReminder
end tell`, listName, props)

	return Run(script)
}

// CompleteReminder marks a reminder as completed.
func CompleteReminder(reminderID string) error {
	script := fmt.Sprintf(`tell application "Reminders"
	set targetReminder to first reminder whose id is %q
	set completed of targetReminder to true
end tell`, reminderID)
	_, err := Run(script)
	return err
}

// DeleteReminder removes a reminder.
func DeleteReminder(reminderID string) error {
	script := fmt.Sprintf(`tell application "Reminders"
	set targetReminder to first reminder whose id is %q
	delete targetReminder
end tell`, reminderID)
	_, err := Run(script)
	return err
}

// ToJSON serializes any value to indented JSON.
func ToJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
