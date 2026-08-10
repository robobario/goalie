package display

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"goalie/internal/journal"
	"goalie/internal/timeutil"
)

var mentionRe = regexp.MustCompile(`@[a-zA-Z0-9][a-zA-Z0-9-]{0,38}`)

// statusNoteTokenRe matches URLs or @mentions in a single pass, like the TUI.
var statusNoteTokenRe = regexp.MustCompile(`https?://\S+|@[a-zA-Z0-9][a-zA-Z0-9-]{0,38}`)

var (
	statusGoalStyle        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "27", Dark: "75"})
	statusTaskTagStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "208"})
	statusBlockedStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "124", Dark: "9"})
	statusMentionStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "76"})
	statusSelfMentionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "22", Dark: "82"})
	statusURLStyle         = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "136", Dark: "178"})
)

func applyGoalStyle(s string, tty bool) string {
	if !tty {
		return s
	}
	return statusGoalStyle.Render(s)
}

func applyTaskTagStyle(s string, tty bool) string {
	if !tty {
		return s
	}
	return statusTaskTagStyle.Render(s)
}

func applyStatusBlockedStyle(s string, tty bool) string {
	if !tty {
		return s
	}
	return statusBlockedStyle.Render(s)
}

// highlightStatusNoteTokens applies colour to URLs and @mentions, matching
// the TUI's renderNoteWithMentions behaviour. When hyperLinks is true,
// GitHub PR/issue URLs are compressed and all URLs are wrapped in OSC 8.
func highlightStatusNoteTokens(note, selfUsername string, tty, hyperLinks bool) string {
	if !tty {
		return note
	}
	return statusNoteTokenRe.ReplaceAllStringFunc(note, func(m string) string {
		if strings.HasPrefix(m, "http") {
			return RenderURL(m, statusURLStyle, hyperLinks)
		}
		if selfUsername != "" && m == selfUsername {
			return statusSelfMentionStyle.Render(m)
		}
		return statusMentionStyle.Render(m)
	})
}

func Teal(s string, tty bool) string {
	if !tty {
		return s
	}
	return "\033[36m" + s + "\033[0m"
}

// FormatMotd formats text as a #MOTD line, teal-coloured when tty is true,
// wrapped at wrapWidth with continuation lines indented to align with the text.
func FormatMotd(text string, tty bool, wrapWidth int) string {
	const prefix = "#MOTD - "
	const contIndent = "        " // 8 spaces, matching len(prefix)

	if wrapWidth <= 0 || len(prefix)+len(text) <= wrapWidth {
		return Teal(prefix+text, tty)
	}

	maxFirst := wrapWidth - len(prefix)
	firstChunk, remaining := takeFirstLine(text, maxFirst)

	var sb strings.Builder
	sb.WriteString(Teal(prefix+firstChunk, tty))
	contWidth := wrapWidth - len(contIndent)
	for _, cl := range wrapWords(remaining, contWidth) {
		sb.WriteString("\n")
		sb.WriteString(Teal(contIndent+cl, tty))
	}
	return sb.String()
}

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
func FormatSummaryEntry(e journal.Entry, selfUsername string, prevBlocked bool, now time.Time, tty, hyperLinks bool) string {
	age := timeutil.AgeString(e.TS, now)
	note := highlightStatusNoteTokens(e.Note, selfUsername, tty, hyperLinks)
	if e.Blocked != prevBlocked {
		if e.Blocked {
			return "- " + Red("[Blocked]", tty) + " " + note + " — " + age
		}
		return "- " + Green("[Unblocked]", tty) + " " + note + " — " + age
	}
	return "- " + note + " — " + age
}

func FormatEntry(e journal.Entry, selfUsername string, now time.Time, tty bool) string {
	age := timeutil.AgeString(e.TS, now)
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

// goalTaskCombo returns "GOAL#task", "GOAL", "#task", or "" in plain text.
func goalTaskCombo(e journal.Entry) string {
	if e.Goal != nil && e.Task != nil {
		return *e.Goal + *e.Task
	}
	if e.Goal != nil {
		return *e.Goal
	}
	if e.Task != nil {
		return *e.Task
	}
	return ""
}

// goalTaskComboStyled returns the styled rendering and its plain-text equivalent.
func goalTaskComboStyled(e journal.Entry, tty bool) (styled, plain string) {
	if e.Goal != nil && e.Task != nil {
		return applyGoalStyle(*e.Goal, tty) + applyTaskTagStyle(*e.Task, tty), *e.Goal + *e.Task
	}
	if e.Goal != nil {
		return applyGoalStyle(*e.Goal, tty), *e.Goal
	}
	if e.Task != nil {
		return applyTaskTagStyle(*e.Task, tty), *e.Task
	}
	return "", ""
}

func FormatStatusEntry(e journal.Entry, selfUsername string, now time.Time, tty, hyperLinks bool) string {
	age := timeutil.AgeString(e.TS, now)
	comboStyled, comboPlain := goalTaskComboStyled(e, tty)
	note := highlightStatusNoteTokens(e.Note, selfUsername, tty, hyperLinks)
	if e.Blocked && comboPlain != "" {
		return applyStatusBlockedStyle("[BLOCKED]", tty) + " " + comboStyled + " " + note + " - " + age
	}
	if e.Blocked {
		return applyStatusBlockedStyle("[BLOCKED]", tty) + " " + note + " - " + age
	}
	if comboPlain != "" {
		return comboStyled + " " + note + " - " + age
	}
	return note + " - " + age
}

// WrapStatusEntry formats a status entry, wrapping the note at availableWidth
// characters. availableWidth is the column budget after any caller-applied
// indent. When availableWidth <= 0 it falls back to the unwrapped format.
func WrapStatusEntry(e journal.Entry, selfUsername string, now time.Time, tty, hyperLinks bool, availableWidth int) string {
	age := timeutil.AgeString(e.TS, now)
	suffix := " - " + age

	comboStyled, comboPlain := goalTaskComboStyled(e, tty)

	// prefix always ends with a space when non-empty so prefix+note renders correctly.
	// prefixPlain is the unstyled equivalent used for column-width maths.
	var prefix, prefixPlain string
	if e.Blocked && comboPlain != "" {
		prefix = applyStatusBlockedStyle("[BLOCKED]", tty) + " " + comboStyled + " "
		prefixPlain = "[BLOCKED] " + comboPlain + " "
	} else if e.Blocked {
		prefix = applyStatusBlockedStyle("[BLOCKED]", tty) + " "
		prefixPlain = "[BLOCKED] "
	} else if comboPlain != "" {
		prefix = comboStyled + " "
		prefixPlain = comboPlain + " "
	}

	maxFirstLine := availableWidth - len(prefixPlain)

	if availableWidth <= 0 || maxFirstLine <= 0 {
		return FormatStatusEntry(e, selfUsername, now, tty, hyperLinks)
	}

	notePlain := e.Note
	if len(notePlain)+len(suffix) <= maxFirstLine {
		return prefix + highlightStatusNoteTokens(notePlain, selfUsername, tty, hyperLinks) + suffix
	}

	const contIndent = "  "
	firstChunk, remaining := takeFirstLine(notePlain, maxFirstLine)

	var sb strings.Builder
	sb.WriteString(prefix + highlightStatusNoteTokens(firstChunk, selfUsername, tty, hyperLinks))

	contNoteWidth := availableWidth - len(contIndent)
	contLines := wrapWords(remaining, contNoteWidth)
	for i, cl := range contLines {
		sb.WriteString("\n")
		rendered := highlightStatusNoteTokens(cl, selfUsername, tty, hyperLinks)
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
