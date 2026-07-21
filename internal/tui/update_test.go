package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"goalie/internal/goals"
	"goalie/internal/journal"
)

func makeBlockedThread(tag string, goal *string, note string) blockedThread {
	return blockedThread{
		tag: tag,
		state: journal.ThreadState{
			Goal:    goal,
			Note:    note,
			Blocked: true,
			TS:      time.Now().UTC().Format(time.RFC3339),
		},
	}
}

func makeRecentThread(tag string, goal *string, note string, hoursAgo int) recentThread {
	ts := time.Now().Add(-time.Duration(hoursAgo) * time.Hour).Format(time.RFC3339)
	return recentThread{
		tag: tag,
		state: journal.ThreadState{
			Goal:    goal,
			Note:    note,
			Blocked: false,
			TS:      ts,
		},
	}
}

func TestUpdateInitialPhaseIsLoading(t *testing.T) {
	m := updateModel{}
	if m.phase != phaseLoading {
		t.Errorf("expected phaseLoading, got %v", m.phase)
	}
}

func TestUpdateThreadStatesLoadedSetsBlockedReview(t *testing.T) {
	m := updateModel{}
	msg := threadStatesLoadedMsg{
		blocked: []blockedThread{
			makeBlockedThread("#thread1", nil, "waiting on infra"),
			makeBlockedThread("#thread2", nil, "pending review"),
		},
	}
	m, _ = m.Update(msg)
	if m.phase != phaseBlockedReview {
		t.Errorf("expected phaseBlockedReview, got %v", m.phase)
	}
	if m.blockedIdx != 0 {
		t.Errorf("expected blockedIdx=0, got %d", m.blockedIdx)
	}
}

func TestUpdateSkipBlockedThreadAdvancesIdx(t *testing.T) {
	m := updateModel{
		phase: phaseBlockedReview,
		blockedThreads: []blockedThread{
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
		blockedThreads: []blockedThread{
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

func TestUpdateAllSkippedTransitionsToRecentReview(t *testing.T) {
	m := updateModel{
		phase: phaseBlockedReview,
		blockedThreads: []blockedThread{
			makeBlockedThread("#thread1", nil, "waiting"),
			makeBlockedThread("#thread2", nil, "waiting"),
		},
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.phase != phaseRecentReview {
		t.Errorf("expected phaseRecentReview after all skipped, got %v", m.phase)
	}
}

func TestUpdateLastThreadSubmittedTransitionsToRecentReview(t *testing.T) {
	m := updateModel{
		phase: phaseBlockedReview,
		blockedThreads: []blockedThread{
			makeBlockedThread("#thread1", nil, "waiting"),
		},
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	for _, ch := range "some notes" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.phase != phaseRecentReview {
		t.Errorf("expected phaseRecentReview after last thread submitted, got %v", m.phase)
	}
}

func TestUpdateNoBlockedThreadsTransitionsToRecentReview(t *testing.T) {
	m := updateModel{}
	msg := threadStatesLoadedMsg{
		blocked: []blockedThread{},
		recent: []recentThread{
			makeRecentThread("#onboarding", nil, "Drafted outline", 24),
		},
		username: "alice",
	}
	m, _ = m.Update(msg)
	if m.phase != phaseRecentReview {
		t.Errorf("expected phaseRecentReview when no blocked threads, got %v", m.phase)
	}
	if len(m.recentThreads) != 1 {
		t.Errorf("expected 1 recent thread, got %d", len(m.recentThreads))
	}
}

func TestUpdateRecentListDownMovescursor(t *testing.T) {
	m := updateModel{
		phase: phaseRecentReview,
		recentSub: recentList,
		recentThreads: []recentThread{
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
		recentThreads: []recentThread{
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
		recentThreads: []recentThread{
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
		phase:    phaseNewThread,
		newSub:   newAnother,
		allGoals: []goals.Goal{{ID: "PROJ", State: "open"}},
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.newSub != newGoalPick {
		t.Errorf("expected newSub=newGoalPick, got %v", m.newSub)
	}
}

func TestNewAnotherNoTransitionsToDone(t *testing.T) {
	m := updateModel{
		phase:  phaseNewThread,
		newSub: newAnother,
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.phase != phaseDone {
		t.Errorf("expected phaseDone, got %v", m.phase)
	}
}

func TestInvalidThreadTagSetsTagError(t *testing.T) {
	m := updateModel{
		phase:  phaseNewThread,
		newSub: newTagPick,
		threadPicker: pickerModel{
			items:   []string{},
			matches: []string{},
			query:   "no-hash-prefix",
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
		recentThreads: []recentThread{
			makeRecentThread("#onboarding", nil, "Drafted outline", 24),
			makeRecentThread("#docs", nil, "Writing", 48),
		},
		recentCursor: 0,
		recentNotes:  "made progress",
		updatedTags:  make(map[string]bool),
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if len(m.recentThreads) != 1 {
		t.Errorf("expected 1 thread remaining, got %d", len(m.recentThreads))
	}
	if m.recentSub != recentList {
		t.Errorf("expected recentSub=recentList after answering blocked, got %v", m.recentSub)
	}
}
