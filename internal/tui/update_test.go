package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"goalie/internal/goals"
	"goalie/internal/journal"
)

func makeBlockedThread(tag string, goal *string, note string) blockedTask {
	return blockedTask{
		tag: tag,
		state: journal.TaskState{
			Goal:    goal,
			Note:    note,
			Blocked: true,
			TS:      time.Now().UTC().Format(time.RFC3339),
		},
	}
}

func makeRecentThread(tag string, goal *string, note string, hoursAgo int) recentTask {
	ts := time.Now().Add(-time.Duration(hoursAgo) * time.Hour).Format(time.RFC3339)
	return recentTask{
		tag: tag,
		state: journal.TaskState{
			Goal:    goal,
			Note:    note,
			Blocked: false,
			TS:      ts,
		},
	}
}

func TestUpdateViewMultiLineErrorPreserved(t *testing.T) {
	m := updateModel{err: errors.New("push rejected\nhint: fetch first")}
	got := m.View()
	want := "Error: push rejected\nhint: fetch first"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestUpdateInitialPhaseIsLoading(t *testing.T) {
	m := updateModel{}
	if m.phase != phaseLoading {
		t.Errorf("expected phaseLoading, got %v", m.phase)
	}
}

func TestUpdateThreadStatesLoadedSetsMenu(t *testing.T) {
	m := updateModel{}
	msg := taskStatesLoadedMsg{
		blocked: []blockedTask{
			makeBlockedThread("#thread1", nil, "waiting on infra"),
			makeBlockedThread("#thread2", nil, "pending review"),
		},
	}
	m, _ = m.Update(msg)
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after loading, got %v", m.phase)
	}
	if m.blockedIdx != 0 {
		t.Errorf("expected blockedIdx=0, got %d", m.blockedIdx)
	}
}

func TestUpdateSkipBlockedThreadAdvancesIdx(t *testing.T) {
	m := updateModel{
		phase: phaseBlockedReview,
		blockedTasks: []blockedTask{
			makeBlockedThread("#thread1", nil, "waiting"),
			makeBlockedThread("#thread2", nil, "waiting"),
		},
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.blockedIdx != 1 {
		t.Errorf("expected blockedIdx=1 after skip, got %d", m.blockedIdx)
	}
}

func TestUpdateYesNotUnblockedWithNotesAdvancesIdx(t *testing.T) {
	m := updateModel{
		phase: phaseBlockedReview,
		blockedTasks: []blockedTask{
			makeBlockedThread("#thread1", nil, "waiting"),
			makeBlockedThread("#thread2", nil, "waiting"),
		},
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !m.awaitingUnblock {
		t.Fatal("expected awaitingUnblock=true after y")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if !m.inputMode {
		t.Fatal("expected inputMode=true after n on unblock question")
	}
	if m.nowUnblocked {
		t.Fatal("expected nowUnblocked=false")
	}

	for _, ch := range "still waiting" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.blockedIdx != 1 {
		t.Errorf("expected blockedIdx=1 after submitting, got %d", m.blockedIdx)
	}
}

func TestUpdateAllSkippedTransitionsToMenu(t *testing.T) {
	m := updateModel{
		phase: phaseBlockedReview,
		blockedTasks: []blockedTask{
			makeBlockedThread("#thread1", nil, "waiting"),
			makeBlockedThread("#thread2", nil, "waiting"),
		},
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after all skipped, got %v", m.phase)
	}
}

func TestUpdateLastThreadSubmittedTransitionsToMenu(t *testing.T) {
	m := updateModel{
		phase: phaseBlockedReview,
		blockedTasks: []blockedTask{
			makeBlockedThread("#thread1", nil, "waiting"),
		},
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	for _, ch := range "some notes" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after last thread submitted, got %v", m.phase)
	}
}

func TestUpdateLoadedAlwaysGoesToMenu(t *testing.T) {
	m := updateModel{}
	msg := taskStatesLoadedMsg{
		blocked: []blockedTask{},
		recent: []recentTask{
			makeRecentThread("#onboarding", nil, "Drafted outline", 24),
		},
		username: "alice",
	}
	m, _ = m.Update(msg)
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after loading, got %v", m.phase)
	}
	if len(m.recentTasks) != 1 {
		t.Errorf("expected 1 recent thread, got %d", len(m.recentTasks))
	}
}

func TestUpdateRecentListDownMovescursor(t *testing.T) {
	m := updateModel{
		phase: phaseRecentReview,
		recentSub: recentList,
		recentTasks: []recentTask{
			makeRecentThread("#alpha", nil, "in progress", 24),
			makeRecentThread("#beta", nil, "started", 48),
		},
		recentCursor: 0,
		updatedTags:  make(map[string]bool),
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.recentCursor != 1 {
		t.Errorf("expected recentCursor=1 after down, got %d", m.recentCursor)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.recentCursor != 1 {
		t.Errorf("expected recentCursor clamped at 1, got %d", m.recentCursor)
	}
}

func TestUpdateRecentListEnterSelectsThread(t *testing.T) {
	m := updateModel{
		phase: phaseRecentReview,
		recentSub: recentList,
		recentTasks: []recentTask{
			makeRecentThread("#onboarding", nil, "Drafted outline", 24),
		},
		recentCursor: 0,
		updatedTags:  make(map[string]bool),
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.recentSub != recentNotes {
		t.Errorf("expected recentSub=recentNotes after Enter, got %v", m.recentSub)
	}
}

func TestUpdateRecentNotesEnterMovesToBlocked(t *testing.T) {
	m := updateModel{
		phase: phaseRecentReview,
		recentSub: recentNotes,
		recentTasks: []recentTask{
			makeRecentThread("#onboarding", nil, "Drafted outline", 24),
		},
		recentCursor: 0,
		updatedTags:  make(map[string]bool),
	}
	for _, ch := range "some progress" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.recentSub != recentBlocked {
		t.Errorf("expected recentSub=recentBlocked after Enter in notes, got %v", m.recentSub)
	}
}

func TestMenuOptionsIncludesBlockedWhenPresent(t *testing.T) {
	m := updateModel{
		phase: phaseMenu,
		blockedTasks: []blockedTask{
			makeBlockedThread("#impl", nil, "waiting"),
		},
		recentTasks: []recentTask{
			makeRecentThread("#docs", nil, "in progress", 2),
		},
	}
	opts := m.menuOptions()
	if len(opts) != 3 {
		t.Fatalf("expected 3 options, got %d", len(opts))
	}
	if opts[0].phase != phaseBlockedReview {
		t.Errorf("expected first option to be blocked review, got %v", opts[0].phase)
	}
	if opts[1].phase != phaseRecentReview {
		t.Errorf("expected second option to be recent review, got %v", opts[1].phase)
	}
	if opts[2].phase != phaseNewTask {
		t.Errorf("expected third option to be new task, got %v", opts[2].phase)
	}
}

func TestMenuOptionsOmitsBlockedWhenNone(t *testing.T) {
	m := updateModel{phase: phaseMenu}
	opts := m.menuOptions()
	for _, o := range opts {
		if o.phase == phaseBlockedReview {
			t.Error("expected no blocked-review option when no blocked tasks")
		}
	}
}

func TestMenuDownMovesCursor(t *testing.T) {
	m := updateModel{
		phase: phaseMenu,
		recentTasks: []recentTask{
			makeRecentThread("#docs", nil, "work", 2),
		},
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.menuCursor != 1 {
		t.Errorf("expected menuCursor=1, got %d", m.menuCursor)
	}
}

func TestMenuEnterSelectsBlockedReview(t *testing.T) {
	m := updateModel{
		phase: phaseMenu,
		blockedTasks: []blockedTask{
			makeBlockedThread("#impl", nil, "waiting"),
		},
		menuCursor: 0,
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.phase != phaseBlockedReview {
		t.Errorf("expected phaseBlockedReview, got %v", m.phase)
	}
}

func TestMenuEnterSelectsRecentReview(t *testing.T) {
	m := updateModel{
		phase: phaseMenu,
		recentTasks: []recentTask{
			makeRecentThread("#docs", nil, "work", 2),
		},
		menuCursor: 0,
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.phase != phaseRecentReview {
		t.Errorf("expected phaseRecentReview, got %v", m.phase)
	}
}

func TestBlockedEscapeReturnsToMenu(t *testing.T) {
	m := updateModel{
		phase: phaseBlockedReview,
		blockedTasks: []blockedTask{
			makeBlockedThread("#impl", nil, "waiting"),
		},
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after Escape, got %v", m.phase)
	}
}

func TestRecentEscapeReturnsToMenu(t *testing.T) {
	m := updateModel{
		phase:       phaseRecentReview,
		recentSub:   recentList,
		recentTasks: []recentTask{makeRecentThread("#docs", nil, "work", 2)},
		updatedTags: make(map[string]bool),
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after Escape, got %v", m.phase)
	}
}

func TestNewTaskEscapeReturnsToMenu(t *testing.T) {
	m := updateModel{
		phase:  phaseNewTask,
		newSub: newNotes,
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after Escape, got %v", m.phase)
	}
}

func TestMenuViewContainsOptions(t *testing.T) {
	m := updateModel{
		phase: phaseMenu,
		blockedTasks: []blockedTask{
			makeBlockedThread("#impl", nil, "waiting"),
		},
		recentTasks: []recentTask{
			makeRecentThread("#docs", nil, "work", 2),
		},
	}
	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty menu view")
	}
	for _, want := range []string{"blocked", "recent", "new"} {
		if !containsFold(view, want) {
			t.Errorf("expected %q in menu view:\n%s", want, view)
		}
	}
}

func containsFold(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && containsFoldHelper(s, substr))
}

func containsFoldHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func TestPickerFuzzyFilterAndSelect(t *testing.T) {
	p := newPicker([]string{"PROJ-ALPHA", "PROJ-BETA"})
	for _, ch := range "bet" {
		p, _, _, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if len(p.matches) != 1 || p.matches[0] != "PROJ-BETA" {
		t.Errorf("expected matches=[PROJ-BETA], got %v", p.matches)
	}
	_, _, selected, wasSelected := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !wasSelected {
		t.Fatal("expected wasSelected=true")
	}
	if selected != "PROJ-BETA" {
		t.Errorf("expected selected=PROJ-BETA, got %q", selected)
	}
}

func TestNewAnotherYesResetsToGoalPick(t *testing.T) {
	m := updateModel{
		phase:    phaseNewTask,
		newSub:   newAnother,
		allGoals: []goals.Goal{{ID: "PROJ", State: "open"}},
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.newSub != newGoalPick {
		t.Errorf("expected newSub=newGoalPick, got %v", m.newSub)
	}
}

func TestNewAnotherNoTransitionsToMenu(t *testing.T) {
	m := updateModel{
		phase:  phaseNewTask,
		newSub: newAnother,
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu, got %v", m.phase)
	}
}

func TestInvalidThreadTagSetsTagError(t *testing.T) {
	m := updateModel{
		phase:  phaseNewTask,
		newSub: newTagPick,
		taskPicker: pickerModel{
			items:   []string{},
			matches: []string{},
			prefix:  "#",
			query:   "1starts-with-digit",
		},
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.tagError == "" {
		t.Error("expected tagError to be set for invalid tag")
	}
	if m.newSub != newTagPick {
		t.Errorf("expected newSub=newTagPick, got %v", m.newSub)
	}
}

func TestUpdateRecentBlockedAnswerRemovesThread(t *testing.T) {
	m := updateModel{
		phase: phaseRecentReview,
		recentSub: recentBlocked,
		recentTasks: []recentTask{
			makeRecentThread("#onboarding", nil, "Drafted outline", 24),
			makeRecentThread("#docs", nil, "Writing", 48),
		},
		recentCursor: 0,
		recentNotes:  "made progress",
		updatedTags:  make(map[string]bool),
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if len(m.recentTasks) != 1 {
		t.Errorf("expected 1 thread remaining, got %d", len(m.recentTasks))
	}
	if m.recentSub != recentList {
		t.Errorf("expected recentSub=recentList after answering blocked, got %v", m.recentSub)
	}
}
