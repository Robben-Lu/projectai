package applescript

import (
	"fmt"
	"strings"
	"time"
)

// Draft represents a single Drafts app item.
type Draft struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	Folder           string    `json:"folder"` // inbox, archive, trash
	Flagged          bool      `json:"flagged"`
	Tags             []string  `json:"tags,omitempty"`
	Permalink        string    `json:"permalink"`
	CreationDate     time.Time `json:"created"`
	ModificationDate time.Time `json:"modified"`
}

// ListDrafts returns drafts filtered by folder and/or tag.
// folder: "inbox", "archive", "all", or "" (defaults to "inbox").
// tag: filter by tag name, or "" for all.
// limit: max number of drafts to return (0 = no limit, but capped at 100 for safety).
func ListDrafts(folder, tag string, flaggedOnly bool, limit int) ([]Draft, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if folder == "" {
		folder = "inbox"
	}

	// Build filter clause
	var filter string
	switch folder {
	case "all":
		filter = ""
	case "inbox":
		filter = `whose folder is inbox`
	case "archive":
		filter = `whose folder is archive`
	case "trash":
		filter = `whose folder is trash`
	default:
		filter = `whose folder is inbox`
	}

	if flaggedOnly {
		if filter == "" {
			filter = `whose flagged is true`
		} else {
			filter += ` and flagged is true`
		}
	}

	// AppleScript to fetch drafts with tab-separated fields
	script := fmt.Sprintf(`tell application "Drafts"
	set output to ""
	set counter to 0
	set maxCount to %d
	set allDrafts to (every draft %s)
	repeat with d in allDrafts
		if counter >= maxCount then exit repeat
		set dId to id of d
		set dTitle to title of d
		set dFolder to folder of d as string
		set dFlagged to flagged of d
		set dTags to tag list of d
		set dPerma to permalink of d
		try
			set dCreated to creation date of d as «class isot» as string
		on error
			set dCreated to ""
		end try
		try
			set dModified to modification date of d as «class isot» as string
		on error
			set dModified to ""
		end try
		set output to output & dId & "	" & dTitle & "	" & dFolder & "	" & dFlagged & "	" & dTags & "	" & dPerma & "	" & dCreated & "	" & dModified & linefeed
		set counter to counter + 1
	end repeat
	return output
end tell`, limit, filter)

	out, err := Run(script)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []Draft{}, nil
	}

	var drafts []Draft
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 8)
		if len(parts) < 6 {
			continue
		}

		d := Draft{
			ID:        parts[0],
			Title:     parts[1],
			Folder:    parts[2],
			Flagged:   parts[3] == "true",
			Permalink: parts[5],
		}

		// Parse tags
		if parts[4] != "" {
			d.Tags = strings.Split(parts[4], ", ")
		}

		// Parse dates
		if len(parts) > 6 && parts[6] != "" {
			if t, err := time.Parse("2006-01-02T15:04:05", parts[6]); err == nil {
				d.CreationDate = t
			}
		}
		if len(parts) > 7 && parts[7] != "" {
			if t, err := time.Parse("2006-01-02T15:04:05", parts[7]); err == nil {
				d.ModificationDate = t
			}
		}

		// Filter by tag if specified
		if tag != "" {
			found := false
			for _, t := range d.Tags {
				if strings.EqualFold(t, tag) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		drafts = append(drafts, d)
	}
	return drafts, nil
}

// GetDraft returns a single draft by ID, including full content.
func GetDraft(draftID string) (*Draft, error) {
	script := fmt.Sprintf(`tell application "Drafts"
	set d to first draft whose id is %q
	set dContent to content of d
	set dTitle to title of d
	set dFolder to folder of d as string
	set dFlagged to flagged of d
	set dTags to tag list of d
	set dPerma to permalink of d
	try
		set dCreated to creation date of d as «class isot» as string
	on error
		set dCreated to ""
	end try
	try
		set dModified to modification date of d as «class isot» as string
	on error
		set dModified to ""
	end try
	return dTitle & "	" & dFolder & "	" & dFlagged & "	" & dTags & "	" & dPerma & "	" & dCreated & "	" & dModified & "	CONTENT_SEP	" & dContent
end tell`, draftID)

	out, err := Run(script)
	if err != nil {
		return nil, err
	}

	// Split at CONTENT_SEP to separate metadata from content
	sepIdx := strings.Index(out, "\tCONTENT_SEP\t")
	if sepIdx == -1 {
		return nil, fmt.Errorf("unexpected output format")
	}
	meta := out[:sepIdx]
	content := out[sepIdx+len("\tCONTENT_SEP\t"):]

	parts := strings.SplitN(meta, "\t", 7)
	if len(parts) < 5 {
		return nil, fmt.Errorf("unexpected metadata format")
	}

	d := &Draft{
		ID:        draftID,
		Title:     parts[0],
		Folder:    parts[1],
		Flagged:   parts[2] == "true",
		Permalink: parts[4],
		Content:   content,
	}

	if parts[3] != "" {
		d.Tags = strings.Split(parts[3], ", ")
	}
	if len(parts) > 5 && parts[5] != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", parts[5]); err == nil {
			d.CreationDate = t
		}
	}
	if len(parts) > 6 && parts[6] != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", parts[6]); err == nil {
			d.ModificationDate = t
		}
	}

	return d, nil
}

// CreateDraft creates a new draft with the given content.
// Tags are applied post-creation (AppleScript tag names property is read-only).
func CreateDraft(content string, tags []string, flagged bool) (string, error) {
	flagStr := "false"
	if flagged {
		flagStr = "true"
	}

	script := fmt.Sprintf(`tell application "Drafts"
	set newDraft to make new draft with properties {content:%q, flagged:%s}
	return id of newDraft
end tell`, content, flagStr)

	id, err := Run(script)
	if err != nil {
		return "", err
	}

	// Apply tags via open URL (Drafts' AppleScript tag API is limited)
	for _, tag := range tags {
		tagScript := fmt.Sprintf(`open location "drafts://x-callback-url/tag?uuid=%s&tag=%s"`, id, tag)
		Run(tagScript)
	}

	return id, nil
}

// UpdateDraftContent replaces the content of an existing draft.
func UpdateDraftContent(draftID, content string) error {
	script := fmt.Sprintf(`tell application "Drafts"
	set d to first draft whose id is %q
	set content of d to %q
end tell`, draftID, content)
	_, err := Run(script)
	return err
}

// AppendToDraft appends text to an existing draft.
func AppendToDraft(draftID, text string) error {
	script := fmt.Sprintf(`tell application "Drafts"
	set d to first draft whose id is %q
	set content of d to (content of d) & linefeed & %q
end tell`, draftID, text)
	_, err := Run(script)
	return err
}

// FlagDraft sets the flagged status of a draft.
func FlagDraft(draftID string, flagged bool) error {
	script := fmt.Sprintf(`tell application "Drafts"
	set d to first draft whose id is %q
	set flagged of d to %t
end tell`, draftID, flagged)
	_, err := Run(script)
	return err
}

// ArchiveDraft moves a draft to the archive folder.
func ArchiveDraft(draftID string) error {
	script := fmt.Sprintf(`tell application "Drafts"
	set d to first draft whose id is %q
	set folder of d to archive
end tell`, draftID)
	_, err := Run(script)
	return err
}

// TrashDraft moves a draft to the trash folder.
func TrashDraft(draftID string) error {
	script := fmt.Sprintf(`tell application "Drafts"
	set d to first draft whose id is %q
	set folder of d to trash
end tell`, draftID)
	_, err := Run(script)
	return err
}

// TagDraft adds a tag to a draft.
func TagDraft(draftID, tag string) error {
	script := fmt.Sprintf(`tell application "Drafts"
	set d to first draft whose id is %q
	set currentTags to tag names of d
	if currentTags is "" then
		set tag names of d to %q
	else
		set tag names of d to currentTags & "," & %q
	end if
end tell`, draftID, tag, tag)
	_, err := Run(script)
	return err
}

// SearchDrafts searches drafts by content substring.
func SearchDrafts(query string, folder string, limit int) ([]Draft, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	if folder == "" {
		folder = "inbox"
	}

	var folderFilter string
	switch folder {
	case "all":
		folderFilter = ""
	case "inbox":
		folderFilter = `and folder of d is inbox`
	case "archive":
		folderFilter = `and folder of d is archive`
	default:
		folderFilter = `and folder of d is inbox`
	}

	script := fmt.Sprintf(`tell application "Drafts"
	set output to ""
	set counter to 0
	set maxCount to %d
	set queryStr to %q
	repeat with d in every draft
		if counter >= maxCount then exit repeat
		if content of d contains queryStr %s then
			set dId to id of d
			set dTitle to title of d
			set dFolder to folder of d as string
			set dFlagged to flagged of d
			set dTags to tag list of d
			set output to output & dId & "	" & dTitle & "	" & dFolder & "	" & dFlagged & "	" & dTags & linefeed
			set counter to counter + 1
		end if
	end repeat
	return output
end tell`, limit, query, folderFilter)

	out, err := Run(script)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []Draft{}, nil
	}

	var drafts []Draft
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 4 {
			continue
		}
		d := Draft{
			ID:      parts[0],
			Title:   parts[1],
			Folder:  parts[2],
			Flagged: parts[3] == "true",
		}
		if len(parts) > 4 && parts[4] != "" {
			d.Tags = strings.Split(parts[4], ", ")
		}
		drafts = append(drafts, d)
	}
	return drafts, nil
}
