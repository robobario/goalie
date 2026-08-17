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
	statusUnblockedStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "82"})
	statusDoneStyle        = lipgloss.NewStyle().Faint(true)
	statusMentionStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "76"})
	statusSelfMentionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "22", Dark: "82"})
	statusURLStyle         = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "136", Dark: "178"})
)

func applyGoalStyle(s string, ctx Context) string {
	if !ctx.IsTTY {
		return s
	}
	return statusGoalStyle.Render(s)
}

func applyTaskTagStyle(s string, ctx Context) string {
	if !ctx.IsTTY {
		return s
	}
	return statusTaskTagStyle.Render(s)
}

func applyStatusBlockedStyle(s string, ctx Context) string {
	if !ctx.IsTTY {
		return s
	}
	return statusBlockedStyle.Render(s)
}

func applyStatusDoneStyle(s string, ctx Context) string {
	if !ctx.IsTTY {
		return s
	}
	return statusDoneStyle.Render(s)
}

func applyStatusUnblockedStyle(s string, ctx Context) string {
	if !ctx.IsTTY {
		return s
	}
	return statusUnblockedStyle.Render(s)
}

// statusTag returns the styled and plain-text forms of an entry's status
// tag: "[done] " takes precedence over "[BLOCKED] "/"[UNBLOCKED] ", matching
// the TUI activity view. A blocked entry renders as "[UNBLOCKED] " instead
// of "[BLOCKED] " when unblockedTargets marks its (username, goal, task) as
// unblocked by someone else's entry (see journal.UnblockedTargets). Empty
// when none apply.
func statusTag(e journal.Entry, unblockedTargets map[journal.UnblockTarget]bool, ctx Context) (styled, plain string) {
	if e.Done {
		return applyStatusDoneStyle("[done]", ctx) + " ", "[done] "
	}
	if e.Blocked {
		if unblockedTargets[journal.TargetOf(e)] {
			return applyStatusUnblockedStyle("[UNBLOCKED]", ctx) + " ", "[UNBLOCKED] "
		}
		return applyStatusBlockedStyle("[BLOCKED]", ctx) + " ", "[BLOCKED] "
	}
	return "", ""
}

// highlightStatusNoteTokens applies colour to URLs and @mentions, matching
// the TUI's renderNoteWithMentions behaviour. When HyperLinks is true,
// GitHub PR/issue URLs are compressed and all URLs are wrapped in OSC 8.
func highlightStatusNoteTokens(note, selfUsername string, ctx Context) string {
	if !ctx.IsTTY {
		return note
	}
	return statusNoteTokenRe.ReplaceAllStringFunc(note, func(m string) string {
		if strings.HasPrefix(m, "http") {
			return RenderURL(m, statusURLStyle, ctx.HyperLinks)
		}
		if selfUsername != "" && m == selfUsername {
			return statusSelfMentionStyle.Render(m)
		}
		return statusMentionStyle.Render(m)
	})
}

func Teal(s string, ctx Context) string {
	if !ctx.IsTTY {
		return s
	}
	return "\033[36m" + s + "\033[0m"
}

// FormatMotd formats text as a #MOTD line, teal-coloured when IsTTY is true,
// wrapped at wrapWidth with continuation lines indented to align with the text.
func FormatMotd(text string, ctx Context, wrapWidth int) string {
	const prefix = "#MOTD - "
	const contIndent = "        " // 8 spaces, matching len(prefix)

	if wrapWidth <= 0 || len(prefix)+len(text) <= wrapWidth {
		return Teal(prefix+text, ctx)
	}

	maxFirst := wrapWidth - len(prefix)
	firstChunk, remaining := takeFirstLine(text, maxFirst)

	var sb strings.Builder
	sb.WriteString(Teal(prefix+firstChunk, ctx))
	contWidth := wrapWidth - len(contIndent)
	for _, cl := range wrapWords(remaining, contWidth) {
		sb.WriteString("\n")
		sb.WriteString(Teal(contIndent+cl, ctx))
	}
	return sb.String()
}

func Bold(s string, ctx Context) string {
	if !ctx.IsTTY {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

func Red(s string, ctx Context) string {
	if !ctx.IsTTY {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

func Green(s string, ctx Context) string {
	if !ctx.IsTTY {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func Username(name string, ctx Context) string {
	return Bold(name, ctx)
}

// BoldGreen wraps s in ANSI bold+green escape when IsTTY is true.
func BoldGreen(s string, ctx Context) string {
	if !ctx.IsTTY {
		return s
	}
	return "\033[1;32m" + s + "\033[0m"
}

// HighlightMentions replaces @handle tokens in note with styled versions.
// Mentions matching selfUsername get extra emphasis (bold+bright-green); others get bold+green.
func HighlightMentions(note, selfUsername string, ctx Context) string {
	if !ctx.IsTTY {
		return note
	}
	return mentionRe.ReplaceAllStringFunc(note, func(m string) string {
		if selfUsername != "" && m == selfUsername {
			return "\033[1;92m" + m + "\033[0m"
		}
		return BoldGreen(m, ctx)
	})
}

func Section(title string, w io.Writer, ctx Context) {
	const width = 44
	dashes := strings.Repeat("─", max(0, width-len(title)-4))
	line := "── " + title + " " + dashes
	fmt.Fprintf(w, "\n%s\n", Bold(line, ctx))
}

// FormatSummaryHeader returns the group header for a summary story block.
// goal is empty or "(no goal)"; task is the #hashtag; username is the slugified name.
func FormatSummaryHeader(goal, task, username string, ctx Context) string {
	return Bold("= "+goal+task+username, ctx)
}

// FormatSummaryEntry formats a single entry line within a summary story block.
// prevBlocked is the blocked state of the preceding entry (false for the first entry).
// A label is shown only when the blocked state differs from prevBlocked.
func FormatSummaryEntry(e journal.Entry, selfUsername string, prevBlocked bool, now time.Time, ctx Context) string {
	age := timeutil.AgeString(e.TS, now)
	note := highlightStatusNoteTokens(e.Note, selfUsername, ctx)
	if e.Blocked != prevBlocked {
		if e.Blocked {
			return "- " + Red("[Blocked]", ctx) + " " + note + " — " + age
		}
		return "- " + Green("[Unblocked]", ctx) + " " + note + " — " + age
	}
	return "- " + note + " — " + age
}

func FormatEntry(e journal.Entry, selfUsername string, now time.Time, ctx Context) string {
	age := timeutil.AgeString(e.TS, now)
	taskPart := ""
	if e.Task != nil {
		taskPart = *e.Task + " "
	}
	note := HighlightMentions(e.Note, selfUsername, ctx)
	if e.Blocked {
		return Red("[BLOCKED]", ctx) + " " + Username(e.Username, ctx) + " " + taskPart + note + " - " + age
	}
	return Username(e.Username, ctx) + " " + taskPart + note + " - " + age
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
func goalTaskComboStyled(e journal.Entry, ctx Context) (styled, plain string) {
	if e.Goal != nil && e.Task != nil {
		return applyGoalStyle(*e.Goal, ctx) + applyTaskTagStyle(*e.Task, ctx), *e.Goal + *e.Task
	}
	if e.Goal != nil {
		return applyGoalStyle(*e.Goal, ctx), *e.Goal
	}
	if e.Task != nil {
		return applyTaskTagStyle(*e.Task, ctx), *e.Task
	}
	return "", ""
}

func FormatStatusEntry(e journal.Entry, selfUsername string, now time.Time, ctx Context, unblockedTargets map[journal.UnblockTarget]bool) string {
	age := timeutil.AgeString(e.TS, now)
	comboStyled, comboPlain := goalTaskComboStyled(e, ctx)
	tagStyled, _ := statusTag(e, unblockedTargets, ctx)
	note := highlightStatusNoteTokens(e.Note, selfUsername, ctx)
	if comboPlain != "" {
		return tagStyled + comboStyled + " " + note + " - " + age
	}
	return tagStyled + note + " - " + age
}

// renderNoteWords renders a slice of NoteWord as a string with ANSI styling
// applied when IsTTY is true. Words with Token=false are passed through
// highlightStatusNoteTokens so that partial matches (e.g. "@alice's") receive
// correct inline styling without adjacent-character corruption.
func renderNoteWords(words []NoteWord, selfUsername string, ctx Context) string {
	if len(words) == 0 {
		return ""
	}
	if !ctx.IsTTY {
		parts := make([]string, len(words))
		for i, w := range words {
			parts[i] = w.Original
		}
		return strings.Join(parts, " ")
	}
	parts := make([]string, len(words))
	for i, w := range words {
		if !w.Token {
			parts[i] = highlightStatusNoteTokens(w.Original, selfUsername, ctx)
			continue
		}
		if strings.HasPrefix(w.Original, "http") {
			parts[i] = RenderURL(w.Original, statusURLStyle, ctx.HyperLinks)
		} else if selfUsername != "" && w.Original == selfUsername {
			parts[i] = statusSelfMentionStyle.Render(w.Original)
		} else {
			parts[i] = statusMentionStyle.Render(w.Original)
		}
	}
	return strings.Join(parts, " ")
}

// WrapStatusEntry formats a status entry, wrapping the note at availableWidth
// characters. availableWidth is the column budget after any caller-applied
// indent. When availableWidth <= 0 it falls back to the unwrapped format.
// Width calculations use the compressed form of URLs (when HyperLinks is true)
// so that compress_hyperlinks users see accurate line breaks.
func WrapStatusEntry(e journal.Entry, selfUsername string, now time.Time, ctx Context, availableWidth int, unblockedTargets map[journal.UnblockTarget]bool) string {
	age := timeutil.AgeString(e.TS, now)
	suffix := " - " + age

	comboStyled, comboPlain := goalTaskComboStyled(e, ctx)
	tagStyled, tagPlain := statusTag(e, unblockedTargets, ctx)

	// prefix always ends with a space when non-empty so prefix+note renders correctly.
	// prefixPlain is the unstyled equivalent used for column-width maths.
	var prefix, prefixPlain string
	if comboPlain != "" {
		prefix = tagStyled + comboStyled + " "
		prefixPlain = tagPlain + comboPlain + " "
	} else {
		prefix = tagStyled
		prefixPlain = tagPlain
	}

	maxFirstLine := availableWidth - len(prefixPlain)

	if availableWidth <= 0 || maxFirstLine <= 0 {
		return FormatStatusEntry(e, selfUsername, now, ctx, unblockedTargets)
	}

	words := TokenizeNoteWords(e.Note, ctx.IsTTY && ctx.HyperLinks)
	if noteWordsWidth(words)+len(suffix) <= maxFirstLine {
		return prefix + highlightStatusNoteTokens(e.Note, selfUsername, ctx) + suffix
	}

	const contIndent = "  "
	firstWords, remainingWords := TakeFirstLineWords(words, maxFirstLine)

	var sb strings.Builder
	sb.WriteString(prefix + renderNoteWords(firstWords, selfUsername, ctx))

	contNoteWidth := availableWidth - len(contIndent)
	contLines := WrapNoteWords(remainingWords, contNoteWidth)
	for i, lineWords := range contLines {
		sb.WriteString("\n")
		rendered := renderNoteWords(lineWords, selfUsername, ctx)
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
