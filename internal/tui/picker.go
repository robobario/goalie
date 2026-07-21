package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

type pickerModel struct {
	items   []string
	query   string
	matches []string
	cursor  int
}

func newPicker(items []string) pickerModel {
	matches := make([]string, len(items))
	copy(matches, items)
	return pickerModel{
		items:   items,
		matches: matches,
	}
}

func (p pickerModel) filter() []string {
	if p.query == "" {
		result := make([]string, len(p.items))
		copy(result, p.items)
		return result
	}
	found := fuzzy.Find(p.query, p.items)
	result := make([]string, 0, len(found))
	for _, m := range found {
		result = append(result, m.Str)
	}
	return result
}

// Update handles printable chars, Backspace, Up, Down, and Enter.
// Returns (updated model, cmd, selected string, wasSelected bool).
func (p pickerModel) Update(msg tea.Msg) (pickerModel, tea.Cmd, string, bool) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil, "", false
	}
	switch keyMsg.String() {
	case "backspace":
		if len(p.query) > 0 {
			p.query = p.query[:len(p.query)-1]
			p.matches = p.filter()
			p.cursor = 0
		}
	case "up":
		if len(p.matches) > 0 {
			if p.cursor == 0 {
				p.cursor = len(p.matches) - 1
			} else {
				p.cursor--
			}
		}
	case "down":
		if len(p.matches) > 0 {
			if p.cursor >= len(p.matches)-1 {
				p.cursor = 0
			} else {
				p.cursor++
			}
		}
	case "enter":
		if len(p.matches) > 0 {
			return p, nil, p.matches[p.cursor], true
		}
		if p.query != "" {
			return p, nil, p.query, true
		}
		return p, nil, "", false
	default:
		if len(keyMsg.Runes) == 1 {
			p.query += string(keyMsg.Runes)
			p.matches = p.filter()
			p.cursor = 0
		}
	}
	return p, nil, "", false
}

func (p pickerModel) View() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Search: %s\n", p.query)
	for i, m := range p.matches {
		if i == p.cursor {
			fmt.Fprintf(&sb, "> %s\n", m)
		} else {
			fmt.Fprintf(&sb, "  %s\n", m)
		}
	}
	return sb.String()
}
