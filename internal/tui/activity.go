package tui

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"goalie/internal/cli"
	"goalie/internal/journal"
)

var tuiMentionRe = regexp.MustCompile(`@[a-zA-Z0-9][a-zA-Z0-9-]{0,38}`)

var blockedStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "124", Dark: "9"})
var doneStyle         = lipgloss.NewStyle().Faint(true)
var goalStyle         = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "27", Dark: "75"})
var goalDescStyle     = lipgloss.NewStyle().Faint(true).Italic(true)
var selectedItemStyle = lipgloss.NewStyle().Bold(true)
var taskTagStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "208"})
var usernameStyle     = lipgloss.NewStyle().Bold(true)
var mentionStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "76"})
var selfMentionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "22", Dark: "82"})

type entriesLoadedMsg struct {
	entries []journal.Entry
	err     error
}

type activityModel struct {
	entries      []journal.Entry
	filtered     []journal.Entry
	search       string
	searchMode   bool
	err          error
	loaded       bool
	selfUsername string
	width        int
	wrapWidth    int
}

func loadActivityCmd(ctx *cli.AppContext) tea.Cmd {
	return func() tea.Msg {
		entries, err := journal.CollectLatest(ctx.DataDir, ctx.Git, 30, ctx.EncryptionKey)
		return entriesLoadedMsg{entries: entries, err: err}
	}
}

// FilterEntries returns entries whose note+goal+task+username fuzzy-match query.
// Username is stored with an "@" prefix so both "alice" and "@alice" match.
// Returns all entries when query is empty.
func FilterEntries(entries []journal.Entry, query string) []journal.Entry {
	if query == "" {
		return entries
	}
	searchable := make([]string, len(entries))
	for i, e := range entries {
		parts := []string{e.Note, e.Username}
		if e.Goal != nil {
			parts = append(parts, *e.Goal)
		}
		if e.Task != nil {
			parts = append(parts, *e.Task)
		}
		searchable[i] = strings.Join(parts, " ")
	}
	matches := fuzzy.Find(query, searchable)
	result := make([]journal.Entry, 0, len(matches))
	for _, m := range matches {
		result = append(result, entries[m.Index])
	}
	return result
}

func (m activityModel) Update(msg tea.Msg) (activityModel, tea.Cmd) {
	switch msg := msg.(type) {
	case entriesLoadedMsg:
		m.loaded = true
		m.err = msg.err
		m.entries = msg.entries
		m.filtered = FilterEntries(m.entries, m.search)
	case tea.KeyMsg:
		if msg.Paste {
			m.searchMode = true
			m.search += string(msg.Runes)
			m.filtered = FilterEntries(m.entries, m.search)
			break
		}
		switch msg.String() {
		case "esc":
			m.search = ""
			m.searchMode = false
			m.filtered = m.entries
		case "enter":
			m.searchMode = false
		case "backspace":
			if len(m.search) > 0 {
				m.search = m.search[:len(m.search)-1]
				m.filtered = FilterEntries(m.entries, m.search)
			}
			if m.search == "" {
				m.searchMode = false
			}
		default:
			if len(msg.Runes) == 1 {
				m.searchMode = true
				m.search += string(msg.Runes)
				m.filtered = FilterEntries(m.entries, m.search)
			}
		}
	}
	return m, nil
}

func (m activityModel) View() string {
	if m.err != nil {
		return "Error: " + m.err.Error()
	}
	if !m.loaded {
		return "Loading..."
	}

	var sb strings.Builder

	if m.searchMode {
		searchLabel := "Search: " + m.search
		sb.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render(searchLabel))
		sb.WriteString("_")
	} else if m.search != "" {
		sb.WriteString("Search: " + m.search)
	} else {
		sb.WriteString(lipgloss.NewStyle().Faint(true).Render("start typing to filter"))
	}
	sb.WriteString("\n\n")

	groups := make(map[string][]journal.Entry)
	for _, e := range m.filtered {
		groups[e.Username] = append(groups[e.Username], e)
	}
	usernames := make([]string, 0, len(groups))
	for u := range groups {
		usernames = append(usernames, u)
	}
	sort.Strings(usernames)

	now := time.Now().UTC()
	doneHideCutoff := journal.PriorBusinessDayStart(now)

	const entryIndent = "  "

	for _, username := range usernames {
		entries := groups[username]
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Blocked != entries[j].Blocked {
				return entries[i].Blocked
			}
			return entries[i].TS > entries[j].TS
		})

		sb.WriteString(usernameStyle.Render(username) + ":\n")
		for _, e := range entries {
			if e.Done {
				ts, err := time.Parse(time.RFC3339, e.TS)
				if err == nil && ts.Before(doneHideCutoff) {
					continue
				}
			}
			// availableWidth is the column budget for content inside the entry indent,
			// capped at wrapWidth to avoid very long lines on wide terminals.
			effectiveWidth := m.width
			if m.wrapWidth > 0 && m.wrapWidth < effectiveWidth {
				effectiveWidth = m.wrapWidth
			}
			availableWidth := effectiveWidth - len(entryIndent)
			rendered := renderActivityEntry(e, now, m.selfUsername, availableWidth)
			for _, line := range strings.Split(rendered, "\n") {
				sb.WriteString(entryIndent + line + "\n")
			}
		}
	}

	return sb.String()
}

// wrapWords splits plain text into lines of at most maxWidth characters.
// Words longer than maxWidth appear on their own line unsplit.
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

// renderActivityEntry formats an entry as "• PREFIX - AGE - note..." with the
// note wrapping to indented continuation lines.
func renderActivityEntry(e journal.Entry, now time.Time, selfUsername string, availableWidth int) string {
	const bullet = "•"
	const contIndent = "  "

	var prefixParts []string
	if e.Done {
		prefixParts = append(prefixParts, doneStyle.Render("[done]"))
	} else if e.Blocked {
		prefixParts = append(prefixParts, blockedStyle.Render("[BLOCKED]"))
	}
	if e.Goal != nil && e.Task != nil {
		prefixParts = append(prefixParts, goalStyle.Render(*e.Goal)+taskTagStyle.Render(*e.Task))
	} else if e.Goal != nil {
		prefixParts = append(prefixParts, goalStyle.Render(*e.Goal))
	} else if e.Task != nil {
		prefixParts = append(prefixParts, taskTagStyle.Render(*e.Task))
	}
	prefix := strings.Join(prefixParts, " ")
	age := ageString(e.TS, now)

	var fixedHeader string
	if lipgloss.Width(prefix) > 0 {
		fixedHeader = bullet + " " + prefix + " - " + age
	} else {
		fixedHeader = bullet + " - " + age
	}

	if strings.TrimSpace(e.Note) == "" {
		return fixedHeader
	}

	lineLeader := fixedHeader + " - "
	maxNoteOnFirstLine := availableWidth - lipgloss.Width(lineLeader)
	contWidth := max(1, availableWidth-len(contIndent))

	var sb strings.Builder
	if maxNoteOnFirstLine <= 0 {
		sb.WriteString(fixedHeader)
		for _, nl := range wrapWords(e.Note, contWidth) {
			sb.WriteString("\n" + contIndent + renderNoteWithMentions(nl, selfUsername))
		}
		return sb.String()
	}

	firstChunk, remaining := takeFirstLine(e.Note, maxNoteOnFirstLine)
	if firstChunk != "" {
		sb.WriteString(lineLeader + renderNoteWithMentions(firstChunk, selfUsername))
	} else {
		sb.WriteString(fixedHeader)
	}
	for _, cl := range wrapWords(remaining, contWidth) {
		sb.WriteString("\n" + contIndent + renderNoteWithMentions(cl, selfUsername))
	}
	return sb.String()
}

// takeFirstLine consumes as many complete words from text as fit within
// maxWidth and returns (firstLine, remaining).
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

func renderNoteWithMentions(note, selfUsername string) string {
	return tuiMentionRe.ReplaceAllStringFunc(note, func(m string) string {
		if selfUsername != "" && m == selfUsername {
			return selfMentionStyle.Render(m)
		}
		return mentionStyle.Render(m)
	})
}

func formatActivityEntry(e journal.Entry, now time.Time, selfUsername string) string {
	var parts []string
	if e.Done {
		parts = append(parts, doneStyle.Render("[done]"))
	} else if e.Blocked {
		parts = append(parts, blockedStyle.Render("[BLOCKED]"))
	}
	// Render as GOAL_ID#task-tag (no space between them) when both are present.
	if e.Goal != nil && e.Task != nil {
		parts = append(parts, goalStyle.Render(*e.Goal)+taskTagStyle.Render(*e.Task))
	} else if e.Goal != nil {
		parts = append(parts, goalStyle.Render(*e.Goal))
	} else if e.Task != nil {
		parts = append(parts, taskTagStyle.Render(*e.Task))
	}
	parts = append(parts, renderNoteWithMentions(e.Note, selfUsername))
	return strings.Join(parts, " ") + " — " + ageString(e.TS, now)
}

func ageString(ts string, now time.Time) string {
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "?d ago"
	}
	days := int(now.Sub(parsed).Hours() / 24)
	return fmt.Sprintf("%dd ago", days)
}
