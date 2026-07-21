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

func Section(title string, w io.Writer, tty bool) {
	const width = 44
	dashes := strings.Repeat("─", max(0, width-len(title)-4))
	line := "── " + title + " " + dashes
	fmt.Fprintf(w, "\n%s\n", Bold(line, tty))
}

func FormatEntry(e journal.Entry, now time.Time, tty bool) string {
	age := ageString(e.TS, now)
	threadPart := ""
	if e.Thread != nil {
		threadPart = *e.Thread + " "
	}
	if e.Blocked {
		return Red("[BLOCKED]", tty) + " " + e.Username + " " + threadPart + e.Note + " - " + age
	}
	return e.Username + " " + threadPart + e.Note + " - " + age
}

func FormatStatusEntry(e journal.Entry, now time.Time, tty bool) string {
	age := ageString(e.TS, now)
	goalPart := ""
	if e.Goal != nil {
		goalPart = "(" + *e.Goal + ")"
	}
	threadPart := ""
	if e.Thread != nil {
		threadPart = *e.Thread + " "
	}
	if e.Blocked {
		return Red("[BLOCKED]", tty) + goalPart + " " + threadPart + e.Note + " - " + age
	}
	if goalPart != "" {
		return goalPart + " " + threadPart + e.Note + " - " + age
	}
	return threadPart + e.Note + " - " + age
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
