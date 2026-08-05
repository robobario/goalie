package display

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"goalie/internal/journal"
)

var mentionRe = regexp.MustCompile(`@[a-zA-Z0-9][a-zA-Z0-9-]{0,38}`)

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

// BoldGreen wraps s in ANSI bold+green escape when tty is true.
func BoldGreen(s string, tty bool) string {
	if !tty {
		return s
	}
	return "\033[1;32m" + s + "\033[0m"
}

// HighlightMentions replaces @handle tokens in note with styled versions.
// Mentions matching selfUsername get extra emphasis (bold+bright-green); others get bold+green.
func HighlightMentions(note, selfUsername string, tty bool) string {
	if !tty {
		return note
	}
	return mentionRe.ReplaceAllStringFunc(note, func(m string) string {
		if selfUsername != "" && m == selfUsername {
			return "\033[1;92m" + m + "\033[0m"
		}
		return BoldGreen(m, tty)
	})
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
func FormatSummaryEntry(e journal.Entry, selfUsername string, prevBlocked bool, now time.Time, tty bool) string {
	age := ageString(e.TS, now)
	note := HighlightMentions(e.Note, selfUsername, tty)
	if e.Blocked != prevBlocked {
		if e.Blocked {
			return "- " + Red("[Blocked]", tty) + " " + note + " — " + age
		}
		return "- " + Green("[Unblocked]", tty) + " " + note + " — " + age
	}
	return "- " + note + " — " + age
}

func FormatEntry(e journal.Entry, selfUsername string, now time.Time, tty bool) string {
	age := ageString(e.TS, now)
	taskPart := ""
	if e.Task != nil {
		taskPart = *e.Task + " "
	}
	note := HighlightMentions(e.Note, selfUsername, tty)
	if e.Blocked {
		return Red("[BLOCKED]", tty) + " " + Username(e.Username, tty) + " " + taskPart + note + " - " + age
	}
	return Username(e.Username, tty) + " " + taskPart + note + " - " + age
}

func FormatStatusEntry(e journal.Entry, selfUsername string, now time.Time, tty bool) string {
	age := ageString(e.TS, now)
	goalPart := ""
	if e.Goal != nil {
		goalPart = "(" + *e.Goal + ")"
	}
	taskPart := ""
	if e.Task != nil {
		taskPart = *e.Task + " "
	}
	note := HighlightMentions(e.Note, selfUsername, tty)
	if e.Blocked {
		return Red("[BLOCKED]", tty) + goalPart + " " + taskPart + note + " - " + age
	}
	if goalPart != "" {
		return goalPart + " " + taskPart + note + " - " + age
	}
	return taskPart + note + " - " + age
}

// WrapStatusEntry formats a status entry, wrapping the note at availableWidth
// characters. availableWidth is the column budget after any caller-applied
// indent. When availableWidth <= 0 it falls back to the unwrapped format.
func WrapStatusEntry(e journal.Entry, selfUsername string, now time.Time, tty bool, availableWidth int) string {
	age := ageString(e.TS, now)
	suffix := " - " + age

	goalPart := ""
	if e.Goal != nil {
		goalPart = "(" + *e.Goal + ")"
	}
	taskPart := ""
	if e.Task != nil {
		taskPart = *e.Task + " "
	}

	// prefix mirrors FormatStatusEntry's structure and always ends with a space
	// when non-empty, so prefix+note produces the correct rendered form.
	var prefix, prefixPlain string
	if e.Blocked {
		prefix = Red("[BLOCKED]", tty) + goalPart + " " + taskPart
		prefixPlain = "[BLOCKED]" + goalPart + " " + taskPart
	} else if goalPart != "" {
		prefix = goalPart + " " + taskPart
		prefixPlain = goalPart + " " + taskPart
	} else if taskPart != "" {
		prefix = taskPart
		prefixPlain = taskPart
	}

	maxFirstLine := availableWidth - len(prefixPlain)

	if availableWidth <= 0 || maxFirstLine <= 0 {
		return FormatStatusEntry(e, selfUsername, now, tty)
	}

	notePlain := e.Note
	if len(notePlain)+len(suffix) <= maxFirstLine {
		return prefix + HighlightMentions(notePlain, selfUsername, tty) + suffix
	}

	const contIndent = "  "
	firstChunk, remaining := takeFirstLine(notePlain, maxFirstLine)

	var sb strings.Builder
	sb.WriteString(prefix + HighlightMentions(firstChunk, selfUsername, tty))

	contNoteWidth := availableWidth - len(contIndent)
	contLines := wrapWords(remaining, contNoteWidth)
	for i, cl := range contLines {
		sb.WriteString("\n")
		rendered := HighlightMentions(cl, selfUsername, tty)
		if i == len(contLines)-1 {
			sb.WriteString(contIndent + rendered + suffix)
		} else {
			sb.WriteString(contIndent + rendered)
		}
	}
	return sb.String()
}

func wrapWords(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	var current strings.Builder
	for _, w := range words {
		if current.Len() == 0 {
			current.WriteString(w)
		} else if current.Len()+1+len(w) <= maxWidth {
			current.WriteByte(' ')
			current.WriteString(w)
		} else {
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(w)
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

func takeFirstLine(text string, maxWidth int) (string, string) {
	if maxWidth <= 0 {
		return "", text
	}
	words := strings.Fields(text)
	var current strings.Builder
	taken := 0
	for i, w := range words {
		if current.Len() == 0 {
			if len(w) > maxWidth {
				break
			}
			current.WriteString(w)
			taken = i + 1
		} else if current.Len()+1+len(w) <= maxWidth {
			current.WriteByte(' ')
			current.WriteString(w)
			taken = i + 1
		} else {
			break
		}
	}
	return current.String(), strings.Join(words[taken:], " ")
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
