#!/usr/bin/env swift
// reminders-helper: Fast EventKit bridge for reminders CLI.
// Compiled once, invoked as subprocess. ~10x faster than AppleScript.
//
// Usage:
//   reminders-helper lists
//   reminders-helper list [--list <name>] [--incomplete]
//   reminders-helper add --list <name> --title <title> [--due <ISO8601>] [--notes <text>] [--priority 0-9]
//   reminders-helper done <id>
//   reminders-helper delete <id>

import EventKit
import Foundation

let store = EKEventStore()
let args = CommandLine.arguments
let semaphore = DispatchSemaphore(value: 0)

func requestAccess(completion: @escaping () -> Void) {
    store.requestFullAccessToReminders { granted, error in
        guard granted else {
            fputs("error: Reminders access denied\n", stderr)
            exit(1)
        }
        completion()
    }
}

func printJSON(_ value: Any) {
    if let data = try? JSONSerialization.data(withJSONObject: value, options: [.prettyPrinted, .sortedKeys]),
       let str = String(data: data, encoding: .utf8) {
        print(str)
    }
}

func calendarByName(_ name: String) -> EKCalendar? {
    store.calendars(for: .reminder).first { $0.title == name }
}

func reminderToDict(_ r: EKReminder) -> [String: Any] {
    var d: [String: Any] = [
        "id": r.calendarItemIdentifier,
        "title": r.title ?? "",
        "list": r.calendar.title,
        "completed": r.isCompleted,
        "priority": r.priority
    ]
    if let comps = r.dueDateComponents, let date = Calendar.current.date(from: comps) {
        d["due"] = ISO8601DateFormatter().string(from: date)
    }
    if let notes = r.notes, !notes.isEmpty {
        d["notes"] = notes
    }
    return d
}

// --- Commands ---

func cmdLists() {
    requestAccess {
        let cals = store.calendars(for: .reminder).map { ["name": $0.title, "id": $0.calendarIdentifier] }
        printJSON(cals)
        semaphore.signal()
    }
    semaphore.wait()
}

func cmdList(listName: String?, incompleteOnly: Bool) {
    requestAccess {
        let calendars: [EKCalendar]?
        if let name = listName {
            guard let cal = calendarByName(name) else {
                fputs("error: list not found: \(name)\n", stderr)
                exit(1)
            }
            calendars = [cal]
        } else {
            calendars = store.calendars(for: .reminder)
        }

        let pred: NSPredicate
        if incompleteOnly {
            pred = store.predicateForIncompleteReminders(withDueDateStarting: nil, ending: nil, calendars: calendars)
        } else {
            pred = store.predicateForReminders(in: calendars)
        }

        store.fetchReminders(matching: pred) { reminders in
            let results = (reminders ?? []).map { reminderToDict($0) }
            printJSON(results)
            semaphore.signal()
        }
    }
    semaphore.wait()
}

func cmdAdd(listName: String, title: String, due: String?, notes: String?, priority: Int) {
    requestAccess {
        guard let cal = calendarByName(listName) ?? store.defaultCalendarForNewReminders() else {
            fputs("error: cannot find list: \(listName)\n", stderr)
            exit(1)
        }
        let reminder = EKReminder(eventStore: store)
        reminder.calendar = cal
        reminder.title = title
        reminder.priority = priority
        if let notes = notes {
            reminder.notes = notes
        }
        if let dueStr = due, let date = ISO8601DateFormatter().date(from: dueStr) {
            reminder.dueDateComponents = Calendar.current.dateComponents(
                [.year, .month, .day, .hour, .minute], from: date)
        }
        do {
            try store.save(reminder, commit: true)
            printJSON(["id": reminder.calendarItemIdentifier, "title": title, "status": "created"])
        } catch {
            fputs("error: \(error.localizedDescription)\n", stderr)
            exit(1)
        }
        semaphore.signal()
    }
    semaphore.wait()
}

func cmdDone(id: String) {
    requestAccess {
        let pred = store.predicateForReminders(in: nil)
        store.fetchReminders(matching: pred) { reminders in
            guard let r = reminders?.first(where: { $0.calendarItemIdentifier == id }) else {
                fputs("error: reminder not found: \(id)\n", stderr)
                exit(1)
            }
            r.isCompleted = true
            do {
                try store.save(r, commit: true)
                printJSON(["id": id, "status": "completed"])
            } catch {
                fputs("error: \(error.localizedDescription)\n", stderr)
                exit(1)
            }
            semaphore.signal()
        }
    }
    semaphore.wait()
}

func cmdDelete(id: String) {
    requestAccess {
        let pred = store.predicateForReminders(in: nil)
        store.fetchReminders(matching: pred) { reminders in
            guard let r = reminders?.first(where: { $0.calendarItemIdentifier == id }) else {
                fputs("error: reminder not found: \(id)\n", stderr)
                exit(1)
            }
            do {
                try store.remove(r, commit: true)
                printJSON(["id": id, "status": "deleted"])
            } catch {
                fputs("error: \(error.localizedDescription)\n", stderr)
                exit(1)
            }
            semaphore.signal()
        }
    }
    semaphore.wait()
}

// --- Arg parsing ---

func getArg(_ flag: String) -> String? {
    guard let idx = args.firstIndex(of: flag), idx + 1 < args.count else { return nil }
    return args[idx + 1]
}

func hasFlag(_ flag: String) -> Bool {
    args.contains(flag)
}

guard args.count >= 2 else {
    fputs("usage: reminders-helper <lists|list|add|done|delete> [options]\n", stderr)
    exit(1)
}

switch args[1] {
case "lists":
    cmdLists()
case "list":
    cmdList(listName: getArg("--list"), incompleteOnly: hasFlag("--incomplete"))
case "add":
    guard let title = getArg("--title") else {
        fputs("error: --title required\n", stderr)
        exit(1)
    }
    let listName = getArg("--list") ?? "Reminders"
    let priority = Int(getArg("--priority") ?? "0") ?? 0
    cmdAdd(listName: listName, title: title, due: getArg("--due"), notes: getArg("--notes"), priority: priority)
case "done":
    guard args.count >= 3 else { fputs("error: id required\n", stderr); exit(1) }
    cmdDone(id: args[2])
case "delete":
    guard args.count >= 3 else { fputs("error: id required\n", stderr); exit(1) }
    cmdDelete(id: args[2])
default:
    fputs("error: unknown command: \(args[1])\n", stderr)
    exit(1)
}
