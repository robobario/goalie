package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"goalie/internal/journal"
)

func strPtr(s string) *string { return &s }

func TestActivityViewMultiLineErrorPreserved(t *testing.T) {
	m := activityModel{err: errors.New("line one\nline two\nline three")}
	got := m.View()
	want := "Error: line one\nline two\nline three"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

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
		{Note: "status update", Task: strPtr("#backend")},
		{Note: "status update", Task: strPtr("#frontend")},
	}
	result := FilterEntries(entries, "#backend")
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Task == nil || *result[0].Task != "#backend" {
		t.Errorf("unexpected thread: %v", result[0].Task)
	}
}

func TestFilterEntriesMatchesUsername(t *testing.T) {
	entries := []journal.Entry{
		{Note: "some work", Username: "@alice"},
		{Note: "other work", Username: "@bob"},
	}
	result := FilterEntries(entries, "alice")
	if len(result) != 1 || result[0].Username != "@alice" {
		t.Errorf("expected alice's entry, got %v", result)
	}
}

func TestFilterEntriesMatchesAtPrefixUsername(t *testing.T) {
	entries := []journal.Entry{
		{Note: "some work", Username: "@alice"},
		{Note: "other work", Username: "@bob"},
	}
	result := FilterEntries(entries, "@alice")
	if len(result) != 1 || result[0].Username != "@alice" {
		t.Errorf("expected alice's entry when searching '@alice', got %v", result)
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
		{Note: "bug fix for login", Task: strPtr("#bug-fix")},
	}
	result := FilterEntries(entries, "bugfix")
	if len(result) == 0 {
		t.Error("expected fuzzy match on '#bug-fix' thread for query 'bugfix', got none")
	}
	matched := false
	for _, e := range result {
		if e.Task != nil && *e.Task == "#bug-fix" {
			matched = true
		}
	}
	if !matched {
		t.Error("expected entry with thread '#bug-fix' in results")
	}
}

func TestFormatActivityEntryGoalIncluded(t *testing.T) {
	goal := "ROUTING"
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "some work",
		Goal: strPtr(goal),
	}
	got := formatActivityEntry(e, time.Now())
	if !strings.Contains(got, goal) {
		t.Errorf("expected goal %q in entry output; got %q", goal, got)
	}
}

func TestFormatActivityEntryTaskTagIncluded(t *testing.T) {
	tag := "#impl"
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "some work",
		Task: strPtr(tag),
	}
	got := formatActivityEntry(e, time.Now())
	if !strings.Contains(got, tag) {
		t.Errorf("expected task tag %q in entry output; got %q", tag, got)
	}
}

func TestFormatActivityEntryDoneShowsLabel(t *testing.T) {
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "all finished",
		Done: true,
		Task: strPtr("#impl"),
	}
	got := formatActivityEntry(e, time.Now())
	if !strings.Contains(got, "[done]") {
		t.Errorf("expected '[done]' in done entry; got %q", got)
	}
	if strings.Contains(got, "[BLOCKED]") {
		t.Errorf("expected no '[BLOCKED]' in done entry; got %q", got)
	}
}

func TestFormatActivityEntryBlockedShowsLabel(t *testing.T) {
	e := journal.Entry{
		TS:      time.Now().Format(time.RFC3339),
		Note:    "waiting",
		Blocked: true,
	}
	got := formatActivityEntry(e, time.Now())
	if !strings.Contains(got, "[BLOCKED]") {
		t.Errorf("expected '[BLOCKED]' in blocked entry; got %q", got)
	}
	if strings.Contains(got, "[done]") {
		t.Errorf("expected no '[done]' in blocked entry; got %q", got)
	}
}

func makeLoadedModel(entries []journal.Entry) activityModel {
	m := activityModel{}
	m, _ = m.Update(entriesLoadedMsg{entries: entries})
	return m
}

func TestUpdateAnyRuneEntersSearchMode(t *testing.T) {
	m := activityModel{loaded: true}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !m.searchMode {
		t.Error("expected searchMode=true after typing a character")
	}
	if m.search != "a" {
		t.Errorf("expected search=%q, got %q", "a", m.search)
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

func TestUpdateBackspaceToEmptyExitsSearchMode(t *testing.T) {
	m := activityModel{loaded: true, searchMode: true, search: "a"}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.searchMode {
		t.Error("expected searchMode=false after backspacing to empty")
	}
	if m.search != "" {
		t.Errorf("expected empty search, got %q", m.search)
	}
}

func TestUpdateTypingFiltersEntries(t *testing.T) {
	entries := []journal.Entry{
		{Note: "deploy service", TS: time.Now().Format(time.RFC3339)},
		{Note: "write docs", TS: time.Now().Format(time.RFC3339)},
	}
	m := makeLoadedModel(entries)

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
