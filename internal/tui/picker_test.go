package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPickerEmptyQueryReturnsAllItems(t *testing.T) {
	items := []string{"alpha", "beta", "gamma"}
	p := newPicker(items)
	if len(p.matches) != len(items) {
		t.Fatalf("expected %d matches, got %d", len(items), len(p.matches))
	}
	for i, item := range items {
		if p.matches[i] != item {
			t.Errorf("matches[%d] = %q, want %q", i, p.matches[i], item)
		}
	}
}

func TestPickerTypingNarrowsMatches(t *testing.T) {
	p := newPicker([]string{"alpha", "beta", "gamma"})
	p, _, _, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	p, _, _, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	p, _, _, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if len(p.matches) != 1 || p.matches[0] != "beta" {
		t.Errorf("expected [beta], got %v", p.matches)
	}
}

func TestPickerDownWrapsFromLastToFirst(t *testing.T) {
	p := newPicker([]string{"a", "b", "c"})
	p.cursor = 2
	p, _, _, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.cursor != 0 {
		t.Errorf("expected cursor 0 after wrap from last, got %d", p.cursor)
	}
}

func TestPickerUpWrapsFromFirstToLast(t *testing.T) {
	p := newPicker([]string{"a", "b", "c"})
	p.cursor = 0
	p, _, _, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.cursor != 2 {
		t.Errorf("expected cursor 2 after wrap from first, got %d", p.cursor)
	}
}

func TestPickerEnterReturnsItemAtCursor(t *testing.T) {
	p := newPicker([]string{"first", "second", "third"})
	p.cursor = 1
	_, _, selected, wasSelected := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !wasSelected {
		t.Fatal("expected wasSelected=true")
	}
	if selected != "second" {
		t.Errorf("expected selected=%q, got %q", "second", selected)
	}
}
