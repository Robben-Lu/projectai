package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Robben-Lu/projectai/internal/applescript"
)

// splitFlagsAndArgs separates flag arguments (--key val) from positional args
// so that positional args can appear before flags. Go's flag package stops
// parsing at the first non-flag argument, which breaks "search <query> --flag".
func splitFlagsAndArgs(args []string) (flags, positional []string) {
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flags = append(flags, args[i])
			// If this flag takes a value (next arg isn't a flag), consume it too
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			positional = append(positional, args[i])
		}
	}
	return
}

const usage = `drafts — CLI for Drafts app

Usage:
  drafts list [--folder inbox|archive|all] [--tag <name>] [--flagged] [--limit N]
  drafts search <query> [--folder inbox|archive|all] [--limit N]
  drafts get <id>                              Get full draft content
  drafts create <content> [--tag <name>] [--flagged]
  drafts append <id> <text>                    Append text to a draft
  drafts flag <id>                             Flag a draft
  drafts unflag <id>                           Unflag a draft
  drafts archive <id>                          Move to archive
  drafts trash <id>                            Move to trash
  drafts tag <id> <tag>                        Add a tag

Flags:
  --format json|table    Output format (default: json)
  --help                 Show this help
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list":
		cmdList()
	case "search":
		cmdSearch()
	case "get":
		cmdGet()
	case "create":
		cmdCreate()
	case "append":
		cmdAppend()
	case "flag":
		cmdFlag(true)
	case "unflag":
		cmdFlag(false)
	case "archive":
		cmdArchive()
	case "trash":
		cmdTrash()
	case "tag":
		cmdTag()
	case "--help", "-h", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

func cmdList() {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	folder := fs.String("folder", "inbox", "Filter by folder: inbox, archive, all")
	tag := fs.String("tag", "", "Filter by tag")
	flagged := fs.Bool("flagged", false, "Show only flagged drafts")
	limit := fs.Int("limit", 50, "Max number of drafts")
	format := fs.String("format", "json", "Output format: json or table")
	fs.Parse(os.Args[2:])

	drafts, err := applescript.ListDrafts(*folder, *tag, *flagged, *limit)
	exitOnErr(err)

	if *format == "table" {
		printTable(drafts)
		return
	}
	printJSON(drafts)
}

func cmdSearch() {
	flagArgs, posArgs := splitFlagsAndArgs(os.Args[2:])
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	folder := fs.String("folder", "inbox", "Search in folder: inbox, archive, all")
	limit := fs.Int("limit", 20, "Max results")
	format := fs.String("format", "json", "Output format: json or table")
	fs.Parse(flagArgs)

	if len(posArgs) < 1 {
		fmt.Fprintln(os.Stderr, "error: search query is required")
		os.Exit(1)
	}
	query := strings.Join(posArgs, " ")

	drafts, err := applescript.SearchDrafts(query, *folder, *limit)
	exitOnErr(err)

	if *format == "table" {
		printTable(drafts)
		return
	}
	printJSON(drafts)
}

func cmdGet() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "error: draft id is required")
		os.Exit(1)
	}

	fs := flag.NewFlagSet("get", flag.ExitOnError)
	format := fs.String("format", "json", "Output format: json or table")
	fs.Parse(os.Args[3:])

	id := os.Args[2]
	draft, err := applescript.GetDraft(id)
	exitOnErr(err)

	if *format == "table" {
		fmt.Printf("Title:    %s\n", draft.Title)
		fmt.Printf("ID:       %s\n", draft.ID)
		fmt.Printf("Folder:   %s\n", draft.Folder)
		fmt.Printf("Flagged:  %t\n", draft.Flagged)
		if len(draft.Tags) > 0 {
			fmt.Printf("Tags:     %s\n", strings.Join(draft.Tags, ", "))
		}
		fmt.Printf("Created:  %s\n", draft.CreationDate.Format("2006-01-02 15:04"))
		fmt.Printf("Modified: %s\n", draft.ModificationDate.Format("2006-01-02 15:04"))
		fmt.Printf("Link:     %s\n", draft.Permalink)
		fmt.Println("---")
		fmt.Println(draft.Content)
		return
	}
	printJSON(draft)
}

func cmdCreate() {
	flagArgs, posArgs := splitFlagsAndArgs(os.Args[2:])
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	tag := fs.String("tag", "", "Tag for the new draft")
	flagged := fs.Bool("flagged", false, "Flag the draft")
	format := fs.String("format", "json", "Output format: json or table")
	fs.Parse(flagArgs)

	if len(posArgs) < 1 {
		fmt.Fprintln(os.Stderr, "error: content is required")
		os.Exit(1)
	}
	content := strings.Join(posArgs, " ")

	var tags []string
	if *tag != "" {
		tags = strings.Split(*tag, ",")
	}

	id, err := applescript.CreateDraft(content, tags, *flagged)
	exitOnErr(err)

	result := map[string]string{"id": id, "status": "created"}
	if *format == "table" {
		fmt.Printf("Created draft: %s\n", id)
		return
	}
	printJSON(result)
}

func cmdAppend() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "error: draft id and text are required")
		fmt.Fprintln(os.Stderr, "usage: drafts append <id> <text>")
		os.Exit(1)
	}
	id := os.Args[2]
	text := strings.Join(os.Args[3:], " ")

	err := applescript.AppendToDraft(id, text)
	exitOnErr(err)
	printJSON(map[string]string{"id": id, "status": "appended"})
}

func cmdFlag(flagged bool) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "error: draft id is required")
		os.Exit(1)
	}
	id := os.Args[2]
	err := applescript.FlagDraft(id, flagged)
	exitOnErr(err)

	status := "flagged"
	if !flagged {
		status = "unflagged"
	}
	printJSON(map[string]string{"id": id, "status": status})
}

func cmdArchive() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "error: draft id is required")
		os.Exit(1)
	}
	id := os.Args[2]
	err := applescript.ArchiveDraft(id)
	exitOnErr(err)
	printJSON(map[string]string{"id": id, "status": "archived"})
}

func cmdTrash() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "error: draft id is required")
		os.Exit(1)
	}
	id := os.Args[2]
	err := applescript.TrashDraft(id)
	exitOnErr(err)
	printJSON(map[string]string{"id": id, "status": "trashed"})
}

func cmdTag() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "error: draft id and tag are required")
		fmt.Fprintln(os.Stderr, "usage: drafts tag <id> <tag>")
		os.Exit(1)
	}
	id := os.Args[2]
	tag := os.Args[3]
	err := applescript.TagDraft(id, tag)
	exitOnErr(err)
	printJSON(map[string]string{"id": id, "tag": tag, "status": "tagged"})
}

func printTable(drafts []applescript.Draft) {
	for _, d := range drafts {
		flag := " "
		if d.Flagged {
			flag = "*"
		}
		tags := ""
		if len(d.Tags) > 0 {
			tags = " [" + strings.Join(d.Tags, ",") + "]"
		}
		fmt.Printf("%s %-8s %s%s\n", flag, d.Folder, d.Title, tags)
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
