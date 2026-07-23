package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"goalie/internal/cli"
	"goalie/internal/journal"
)

var blockedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

type entriesLoadedMsg struct {
	entries []journal.Entry
	err     error
}

type activityModel struct {
	entries    []journal.Entry
	filtered   []journal.Entry
	search     string
	searchMode bool
	err        error
	loaded     bool
}

func loadActivityCmd(ctx *cli.AppContext) tea.Cmd {
	return func() tea.Msg {
		entries, err := journal.CollectLatest(ctx.DataDir, ctx.Git, 30, ctx.EncryptionKey)
		return entriesLoadedMsg{entries: entries, err: err}
	}
}

// FilterEntries returns entries whose note+goal+task fuzzy-match query.
// Returns all entries when query is empty.
func FilterEntries(entries []journal.Entry, query string) []journal.Entry {
	if query == "" {
		return entries
	}
	searchable := make([]string, len(entries))
	for i, e := range entries {
		parts := []string{e.Note}
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

	for _, username := range usernames {
		entries := groups[username]
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Blocked != entries[j].Blocked {
				return entries[i].Blocked
			}
			return entries[i].TS > entries[j].TS
		})

		sb.WriteString(username + ":\n")
		for _, e := range entries {
			sb.WriteString("  " + formatActivityEntry(e, now) + "\n")
		}
	}

	return sb.String()
}

func formatActivityEntry(e journal.Entry, now time.Time) string {
	var parts []string
	if e.Blocked {
		parts = append(parts, blockedStyle.Render("[BLOCKED]"))
	}
	if e.Task != nil {
		parts = append(parts, *e.Task)
	}
	if e.Goal != nil {
		parts = append(parts, "("+*e.Goal+")")
	}
	parts = append(parts, e.Note)
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
