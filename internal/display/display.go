package display

import (
	"fmt"
	"io"
	"strings"
	"time"

	"goalie/internal/journal"
)

func Bold(s string, tty bool) string {
	if !tty {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

func Red(s string, tty bool) string {
	if !tty {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

func Green(s string, tty bool) string {
	if !tty {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func Username(name string, tty bool) string {
	return Bold(name, tty)
}

func Section(title string, w io.Writer, tty bool) {
	const width = 44
	dashes := strings.Repeat("─", max(0, width-len(title)-4))
	line := "── " + title + " " + dashes
	fmt.Fprintf(w, "\n%s\n", Bold(line, tty))
}

// FormatSummaryHeader returns the group header for a summary story block.
// goal is empty or "(no goal)"; task is the #hashtag; username is the slugified name.
func FormatSummaryHeader(goal, task, username string, tty bool) string {
	return Bold("= "+goal+task+username, tty)
}

// FormatSummaryEntry formats a single entry line within a summary story block.
// prevBlocked is the blocked state of the preceding entry (false for the first entry).
// A label is shown only when the blocked state differs from prevBlocked.
func FormatSummaryEntry(e journal.Entry, prevBlocked bool, now time.Time, tty bool) string {
	age := ageString(e.TS, now)
	if e.Blocked != prevBlocked {
		if e.Blocked {
			return "- " + Red("[Blocked]", tty) + " " + e.Note + " — " + age
		}
		return "- " + Green("[Unblocked]", tty) + " " + e.Note + " — " + age
	}
	return "- " + e.Note + " — " + age
}

func FormatEntry(e journal.Entry, now time.Time, tty bool) string {
	age := ageString(e.TS, now)
	taskPart := ""
	if e.Task != nil {
		taskPart = *e.Task + " "
	}
	if e.Blocked {
		return Red("[BLOCKED]", tty) + " " + Username(e.Username, tty) + " " + taskPart + e.Note + " - " + age
	}
	return Username(e.Username, tty) + " " + taskPart + e.Note + " - " + age
}

func FormatStatusEntry(e journal.Entry, now time.Time, tty bool) string {
	age := ageString(e.TS, now)
	goalPart := ""
	if e.Goal != nil {
		goalPart = "(" + *e.Goal + ")"
	}
	taskPart := ""
	if e.Task != nil {
		taskPart = *e.Task + " "
	}
	if e.Blocked {
		return Red("[BLOCKED]", tty) + goalPart + " " + taskPart + e.Note + " - " + age
	}
	if goalPart != "" {
		return goalPart + " " + taskPart + e.Note + " - " + age
	}
	return taskPart + e.Note + " - " + age
}

func ageString(ts string, now time.Time) string {
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "?d ago"
	}
	days := int(now.Sub(parsed).Hours() / 24)
	return fmt.Sprintf("%dd ago", days)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
