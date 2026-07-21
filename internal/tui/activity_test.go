package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"goalie/internal/journal"
)

func strPtr(s string) *string { return &s }

func TestFilterEntriesEmptyQueryReturnsAll(t *testing.T) {
	entries := []journal.Entry{
		{Note: "foo"},
		{Note: "bar"},
	}
	result := FilterEntries(entries, "")
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
}

func TestFilterEntriesMatchesNote(t *testing.T) {
	entries := []journal.Entry{
		{Note: "deploy the service"},
		{Note: "write documentation"},
		{Note: "fix the build"},
	}
	result := FilterEntries(entries, "deploy")
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Note != "deploy the service" {
		t.Errorf("unexpected entry note: %s", result[0].Note)
	}
}

func TestFilterEntriesMatchesGoalID(t *testing.T) {
	entries := []journal.Entry{
		{Note: "progress update", Goal: strPtr("PROJ-42")},
		{Note: "another update", Goal: strPtr("PROJ-99")},
	}
	result := FilterEntries(entries, "PROJ-42")
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Goal == nil || *result[0].Goal != "PROJ-42" {
		t.Errorf("unexpected goal: %v", result[0].Goal)
	}
}

func TestFilterEntriesMatchesThread(t *testing.T) {
	entries := []journal.Entry{
		{Note: "status update", Thread: strPtr("#backend")},
		{Note: "status update", Thread: strPtr("#frontend")},
	}
	result := FilterEntries(entries, "#backend")
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Thread == nil || *result[0].Thread != "#backend" {
		t.Errorf("unexpected thread: %v", result[0].Thread)
	}
}

func TestFilterEntriesNoMatchReturnsEmpty(t *testing.T) {
	entries := []journal.Entry{
		{Note: "working on auth"},
		{Note: "fixed pagination"},
	}
	result := FilterEntries(entries, "xyzzy99999")
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

func TestFilterEntriesFuzzyTolerance(t *testing.T) {
	entries := []journal.Entry{
		{Note: "unrelated task"},
		{Note: "bug fix for login", Thread: strPtr("#bug-fix")},
	}
	result := FilterEntries(entries, "bugfix")
	if len(result) == 0 {
		t.Error("expected fuzzy match on '#bug-fix' thread for query 'bugfix', got none")
	}
	matched := false
	for _, e := range result {
		if e.Thread != nil && *e.Thread == "#bug-fix" {
			matched = true
		}
	}
	if !matched {
		t.Error("expected entry with thread '#bug-fix' in results")
	}
}

func makeLoadedModel(entries []journal.Entry) activityModel {
	m := activityModel{}
	m, _ = m.Update(entriesLoadedMsg{entries: entries})
	return m
}

func TestUpdateSlashEntersSearchMode(t *testing.T) {
	m := activityModel{loaded: true}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.searchMode {
		t.Error("expected searchMode=true after '/' key")
	}
}

func TestUpdateEscapeClearsSearchAndExitsSearchMode(t *testing.T) {
	m := activityModel{loaded: true, searchMode: true, search: "hello"}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.searchMode {
		t.Error("expected searchMode=false after escape")
	}
	if m.search != "" {
		t.Errorf("expected empty search after escape, got %q", m.search)
	}
}

func TestUpdateAfterLoadSlashThenCharFiltersEntries(t *testing.T) {
	entries := []journal.Entry{
		{Note: "deploy service", TS: time.Now().Format(time.RFC3339)},
		{Note: "write docs", TS: time.Now().Format(time.RFC3339)},
	}
	m := makeLoadedModel(entries)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.searchMode {
		t.Fatal("expected searchMode=true after '/'")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	if m.search != "dep" {
		t.Errorf("expected search=%q, got %q", "dep", m.search)
	}
	if len(m.filtered) == 0 {
		t.Error("expected at least one filtered entry matching 'dep'")
	}
	for _, e := range m.filtered {
		if e.Note != "deploy service" {
			t.Errorf("unexpected entry in filtered: %s", e.Note)
		}
	}
}
