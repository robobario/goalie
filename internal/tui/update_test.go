package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"goalie/internal/cli"
	"goalie/internal/config"
	"goalie/internal/crypto"
	"goalie/internal/git"
	"goalie/internal/goals"
	"goalie/internal/journal"
)

func makeActiveTask(tag string, goal *string, note string, blocked bool, hoursAgo int) activeTask {
	ts := time.Now().Add(-time.Duration(hoursAgo) * time.Hour).Format(time.RFC3339)
	return activeTask{
		tag: tag,
		state: journal.TaskState{
			Goal:    goal,
			Note:    note,
			Blocked: blocked,
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

func TestMenuViewSelectedItemHasCursor(t *testing.T) {
	m := updateModel{phase: phaseMenu, menuCursor: 1}
	view := m.View()
	// The second menu item should be on the "> " line.
	// (Bold is applied via lipgloss but stripped in non-TTY tests.)
	if !strings.Contains(view, "> Log progress on a new task") {
		t.Errorf("expected selected item on '> ' line; got:\n%s", view)
	}
}

func pasteKey(runes string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(runes), Paste: true}
}

func TestPasteIntoTaskUpdateNoteDoesNotAdvanceField(t *testing.T) {
	m := updateModel{
		phase:         phaseTaskUpdate,
		taskUpdateSub: taskUpdateNote,
	}
	// Pasting "enter" content should append, not advance the sub-phase.
	m, _ = m.Update(pasteKey("hello"))
	if m.taskUpdateNote.value != "hello" {
		t.Errorf("expected note='hello', got %q", m.taskUpdateNote.value)
	}
	if m.taskUpdateSub != taskUpdateNote {
		t.Errorf("expected to stay on taskUpdateNote, got %v", m.taskUpdateSub)
	}
}

func TestPasteIntoNewTaskNoteDoesNotAdvanceField(t *testing.T) {
	m := updateModel{phase: phaseNewTask, newSub: newFormNote}
	m, _ = m.Update(pasteKey("https://example.com/path"))
	if m.newNoteInput.value != "https://example.com/path" {
		t.Errorf("expected full URL in note; got %q", m.newNoteInput.value)
	}
	if m.newSub != newFormNote {
		t.Errorf("expected to stay on newFormNote, got %v", m.newSub)
	}
}

func TestPasteIntoEditNoteDoesNotAdvanceField(t *testing.T) {
	m := updateModel{phase: phaseEditEntry, editSub: editNote}
	m, _ = m.Update(pasteKey("pasted text"))
	if m.editNoteInput.value != "pasted text" {
		t.Errorf("expected 'pasted text'; got %q", m.editNoteInput.value)
	}
	if m.editSub != editNote {
		t.Errorf("expected to stay on editNote, got %v", m.editSub)
	}
}

func TestViewGoalPickerContainsItems(t *testing.T) {
	openGoals := []goals.Goal{{ID: "ROUTING", Description: "Implement routing layer", State: "open"}}
	m := updateModel{
		phase:    phaseNewTask,
		newSub:   newFormGoal,
		allGoals: openGoals,
		goalPicker: pickerModel{
			items:   goalPickerItems(openGoals),
			matches: goalPickerItems(openGoals),
		},
	}
	got := m.viewGoalPicker()
	if !strings.Contains(got, "ROUTING") {
		t.Errorf("expected ROUTING in goal picker; got %q", got)
	}
	if !strings.Contains(got, "Implement routing layer") {
		t.Errorf("expected description in goal picker; got %q", got)
	}
	if !strings.Contains(got, noGoalSentinel) {
		t.Errorf("expected sentinel in goal picker; got %q", got)
	}
}

func TestColorizeGoalInTaskDisplay_withGoal(t *testing.T) {
	got := colorizeGoalInTaskDisplay("ROUTING#impl some work — 2d ago")
	if !strings.Contains(got, "ROUTING") {
		t.Errorf("expected goal in output; got %q", got)
	}
	if !strings.Contains(got, "#impl") {
		t.Errorf("expected tag in output; got %q", got)
	}
	if !strings.Contains(got, "some work") {
		t.Errorf("expected note in output; got %q", got)
	}
}

func TestColorizeGoalInTaskDisplay_noGoal(t *testing.T) {
	got := colorizeGoalInTaskDisplay("#impl some work — 2d ago")
	// tag and note should still appear
	if !strings.Contains(got, "#impl") {
		t.Errorf("expected tag in output; got %q", got)
	}
	if !strings.Contains(got, "some work") {
		t.Errorf("expected note in output; got %q", got)
	}
}

func TestReloadDoesNotResetMenuCursor(t *testing.T) {
	// Regression: reloading task states (e.g. after a submit) was unconditionally
	// resetting menuCursor to 0, causing the selection to jump.
	m := updateModel{phase: phaseMenu, menuCursor: 2}
	msg := taskStatesLoadedMsg{activeTasks: []activeTask{}, username: "@alice"}
	m, _ = m.Update(msg)
	if m.menuCursor != 2 {
		t.Errorf("expected menuCursor=2 preserved on reload, got %d", m.menuCursor)
	}
}

func TestInitialLoadSetsMenuCursorToZero(t *testing.T) {
	m := updateModel{phase: phaseLoading}
	msg := taskStatesLoadedMsg{activeTasks: []activeTask{}, username: "@alice"}
	m, _ = m.Update(msg)
	if m.menuCursor != 0 {
		t.Errorf("expected menuCursor=0 on initial load, got %d", m.menuCursor)
	}
}

func TestUpdateInitialPhaseIsLoading(t *testing.T) {
	m := updateModel{}
	if m.phase != phaseLoading {
		t.Errorf("expected phaseLoading, got %v", m.phase)
	}
}

func TestAppendDoneNoErrorTriggersReload(t *testing.T) {
	m := updateModel{}
	_, cmd := m.Update(appendDoneMsg{err: nil})
	if cmd == nil {
		t.Fatal("expected reload command after successful appendDoneMsg, got nil")
	}
}

func TestAppendDoneWithErrorDoesNotReload(t *testing.T) {
	m := updateModel{}
	_, cmd := m.Update(appendDoneMsg{err: errors.New("push failed")})
	if cmd != nil {
		t.Error("expected no reload command when appendDoneMsg has an error")
	}
}

func TestUpdateEntryDoneNoErrorTriggersReload(t *testing.T) {
	m := updateModel{phase: phaseEditEntry}
	m2, cmd := m.Update(updateEntryDoneMsg{err: nil})
	if cmd == nil {
		t.Fatal("expected reload command after successful updateEntryDoneMsg, got nil")
	}
	if m2.phase != phaseMenu {
		t.Errorf("expected phaseMenu after successful edit, got %v", m2.phase)
	}
}

func TestUpdateLoadedSetsActiveTasks(t *testing.T) {
	m := updateModel{}
	active := makeActiveTask("#onboarding", nil, "Drafted outline", false, 24)
	msg := taskStatesLoadedMsg{
		activeTasks: []activeTask{active},
		username:    "alice",
	}
	m, _ = m.Update(msg)
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after loading, got %v", m.phase)
	}
	if len(m.activeTasks) != 1 {
		t.Errorf("expected 1 active task, got %d", len(m.activeTasks))
	}
}

func TestMenuOptionsAlwaysHasFourItems(t *testing.T) {
	// Menu always shows: Update a task, New task, Edit a recent entry, Unblock a teammate.
	m := updateModel{phase: phaseMenu}
	opts := m.menuOptions()
	if len(opts) != 4 {
		t.Fatalf("expected 4 options, got %d", len(opts))
	}
	if opts[0].phase != phaseTaskUpdate {
		t.Errorf("expected first option to be phaseTaskUpdate, got %v", opts[0].phase)
	}
	if opts[1].phase != phaseNewTask {
		t.Errorf("expected second option to be phaseNewTask, got %v", opts[1].phase)
	}
	if opts[2].phase != phaseEditEntry {
		t.Errorf("expected third option to be phaseEditEntry, got %v", opts[2].phase)
	}
	if opts[3].phase != phaseUnblockTeammate {
		t.Errorf("expected fourth option to be phaseUnblockTeammate, got %v", opts[3].phase)
	}
}

func TestMenuDownMovesCursor(t *testing.T) {
	m := updateModel{phase: phaseMenu}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.menuCursor != 1 {
		t.Errorf("expected menuCursor=1, got %d", m.menuCursor)
	}
}

func TestMenuEnterSelectsTaskUpdate(t *testing.T) {
	m := updateModel{
		phase:      phaseMenu,
		menuCursor: 0, // first item is "Update a task"
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.phase != phaseTaskUpdate {
		t.Errorf("expected phaseTaskUpdate, got %v", m.phase)
	}
}


func TestNewTaskEscapeReturnsToMenu(t *testing.T) {
	for _, sub := range []newTaskSub{newFormGoal, newFormTask, newFormNote, newFormBlocked} {
		m := updateModel{phase: phaseNewTask, newSub: sub}
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if m.phase != phaseMenu {
			t.Errorf("expected phaseMenu after Escape from newSub %v, got %v", sub, m.phase)
		}
	}
}

func TestMenuViewContainsOptions(t *testing.T) {
	m := updateModel{phase: phaseMenu}
	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty menu view")
	}
	for _, want := range []string{"Update a task", "new task", "Edit"} {
		if !containsFold(view, want) {
			t.Errorf("expected %q in menu view:\n%s", want, view)
		}
	}
}

func TestTaskUpdateEnterPhasePopulatesPicker(t *testing.T) {
	m := updateModel{
		phase: phaseMenu,
		activeTasks: []activeTask{
			makeActiveTask("#impl", nil, "waiting", true, 1),   // blocked first
			makeActiveTask("#docs", nil, "in progress", false, 2),
		},
	}
	m = m.enterPhaseTaskUpdate()
	if m.phase != phaseTaskUpdate {
		t.Errorf("expected phaseTaskUpdate, got %v", m.phase)
	}
	if len(m.taskUpdatePicker.items) != 2 {
		t.Errorf("expected 2 picker items, got %d", len(m.taskUpdatePicker.items))
	}
	if !strings.HasPrefix(m.taskUpdatePicker.items[0], "[BLOCKED]") {
		t.Errorf("expected blocked task first with [BLOCKED] prefix, got %q", m.taskUpdatePicker.items[0])
	}
}

func TestTaskUpdatePickerEnterAdvancesToNote(t *testing.T) {
	m := updateModel{
		phase: phaseMenu,
		activeTasks: []activeTask{
			makeActiveTask("#impl", nil, "waiting for review", true, 1),
		},
	}
	m = m.enterPhaseTaskUpdate()
	// Enter on the first picker item (blocked task)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.taskUpdateSub != taskUpdateNote {
		t.Errorf("expected taskUpdateNote after picker Enter, got %v", m.taskUpdateSub)
	}
	if m.taskUpdateState != entryBlocked {
		t.Errorf("expected state pre-filled as entryBlocked for blocked task, got %v", m.taskUpdateState)
	}
}

func TestTaskUpdateNoteEnterAdvancesToState(t *testing.T) {
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateNote,
		taskUpdateNote: newNoteInput("making progress"),
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.taskUpdateSub != taskUpdateState {
		t.Errorf("expected taskUpdateState after Enter, got %v", m.taskUpdateSub)
	}
}

func TestTaskUpdateStateUpGoesBackToNote(t *testing.T) {
	m := updateModel{
		phase:         phaseTaskUpdate,
		taskUpdateSub: taskUpdateState,
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.taskUpdateSub != taskUpdateNote {
		t.Errorf("expected taskUpdateNote after Up from state, got %v", m.taskUpdateSub)
	}
}

func TestTaskUpdateStateCyclesWithArrows(t *testing.T) {
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateState,
		taskUpdateState: entryBlocked,
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.taskUpdateState != entryUnblocked {
		t.Errorf("expected entryUnblocked after Right, got %v", m.taskUpdateState)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.taskUpdateState != entryDone {
		t.Errorf("expected entryDone after second Right, got %v", m.taskUpdateState)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.taskUpdateState != entryUnblocked {
		t.Errorf("expected entryUnblocked after Left, got %v", m.taskUpdateState)
	}
}

func TestTaskUpdateEscFromPickerGoesToMenu(t *testing.T) {
	m := updateModel{
		phase:         phaseTaskUpdate,
		taskUpdateSub: taskUpdatePicking,
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after Esc from picker, got %v", m.phase)
	}
}

func TestTaskUpdateEscFromFormGoesToPicker(t *testing.T) {
	m := updateModel{
		phase:         phaseTaskUpdate,
		taskUpdateSub: taskUpdateNote,
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.taskUpdateSub != taskUpdatePicking {
		t.Errorf("expected taskUpdatePicking after Esc from form, got %v", m.taskUpdateSub)
	}
}

func TestMenuIncludesEditOption(t *testing.T) {
	m := updateModel{phase: phaseMenu}
	opts := m.menuOptions()
	found := false
	for _, o := range opts {
		if o.phase == phaseEditEntry {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Edit a recent entry' option in menu")
	}
}

func TestEditEntriesLoadedSetsEntries(t *testing.T) {
	m := updateModel{phase: phaseEditEntry, editSub: editPicking}
	entries := []journal.Entry{
		{TS: time.Now().Add(-time.Hour).Format(time.RFC3339), Note: "latest", Task: strPtr("#impl")},
		{TS: time.Now().Add(-2 * time.Hour).Format(time.RFC3339), Note: "older", Task: strPtr("#impl")},
	}
	m, _ = m.Update(editEntriesLoadedMsg{entries: entries})
	if len(m.editEntries) != 2 {
		t.Fatalf("expected 2 editEntries, got %d", len(m.editEntries))
	}
}

func TestEditPickingEnterAdvancesToNote(t *testing.T) {
	m := updateModel{
		phase:   phaseEditEntry,
		editSub: editPicking,
		editEntries: []journal.Entry{
			{TS: time.Now().Add(-time.Hour).Format(time.RFC3339), Note: "fix this tpyo", Task: strPtr("#impl")},
		},
		editCursor: 0,
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.editSub != editNote {
		t.Errorf("expected editNote after Enter, got %v", m.editSub)
	}
	if m.editNoteInput.value != "fix this tpyo" {
		t.Errorf("expected note pre-filled, got %q", m.editNoteInput.value)
	}
}

func TestEditNoteEnterAdvancesToTask(t *testing.T) {
	m := updateModel{
		phase:         phaseEditEntry,
		editSub:       editNote,
		editNoteInput: newNoteInput("corrected note"),
		editEntry:     journal.Entry{Task: strPtr("#impl")},
		editTaskInput: "#impl",
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.editSub != editTask {
		t.Errorf("expected editTask after Enter, got %v", m.editSub)
	}
}

func TestEditTaskEnterWithValidTagAdvancesToBlockedDone(t *testing.T) {
	m := updateModel{
		phase:         phaseEditEntry,
		editSub:       editTask,
		editTaskInput: "#impl",
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.editSub != editBlockedDone {
		t.Errorf("expected editBlockedDone after valid tag, got %v", m.editSub)
	}
}

func TestEditTaskEnterWithInvalidTagStays(t *testing.T) {
	m := updateModel{
		phase:         phaseEditEntry,
		editSub:       editTask,
		editTaskInput: "NotAHashtag",
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.editSub != editTask {
		t.Errorf("expected to stay on editTask for invalid tag, got %v", m.editSub)
	}
}

func TestEditEscapeReturnsToMenu(t *testing.T) {
	for _, sub := range []editSub{editPicking, editNote, editTask, editBlockedDone} {
		m := updateModel{phase: phaseEditEntry, editSub: sub, editEntries: nil}
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if m.phase != phaseMenu {
			t.Errorf("expected phaseMenu after Esc from editSub %v, got %v", sub, m.phase)
		}
	}
}

func TestUpdateEntryDoneSetsMenu(t *testing.T) {
	m := updateModel{phase: phaseEditEntry}
	m, _ = m.Update(updateEntryDoneMsg{err: nil})
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after successful edit, got %v", m.phase)
	}
}

func TestEditPickingViewContainsEntries(t *testing.T) {
	m := updateModel{
		phase:   phaseEditEntry,
		editSub: editPicking,
		editEntries: []journal.Entry{
			{
				TS:   time.Now().Add(-time.Hour).Format(time.RFC3339),
				Note: "my note here",
				Task: strPtr("#impl"),
			},
		},
	}
	view := m.View()
	if !strings.Contains(view, "my note here") {
		t.Errorf("expected note in pick view:\n%s", view)
	}
	if !strings.Contains(view, "#impl") {
		t.Errorf("expected task in pick view:\n%s", view)
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

func TestNewTaskSubmitGoesToMenu(t *testing.T) {
	m := updateModel{
		phase:        phaseNewTask,
		newSub:       newFormBlocked,
		selectedTag:  "#impl",
		newNoteInput: newNoteInput("some progress"),
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after submit, got %v", m.phase)
	}
}

func TestNewTaskInvalidTagSetsError(t *testing.T) {
	m := updateModel{
		phase:       phaseNewTask,
		newSub:      newFormTask,
		selectedTag: "no-hash-prefix", // invalid: must start with #
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.tagError == "" {
		t.Error("expected tagError to be set for tag without # prefix")
	}
	if m.newSub != newFormTask {
		t.Errorf("expected to stay on newFormTask, got %v", m.newSub)
	}
}

func TestNewTaskValidTagAdvancesToNote(t *testing.T) {
	m := updateModel{
		phase:       phaseNewTask,
		newSub:      newFormTask,
		selectedTag: "#impl",
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.newSub != newFormNote {
		t.Errorf("expected newFormNote after valid tag, got %v", m.newSub)
	}
	if m.tagError != "" {
		t.Errorf("expected no tag error, got %q", m.tagError)
	}
}

func TestNewTaskNoteEnterAdvancesToBlocked(t *testing.T) {
	m := updateModel{phase: phaseNewTask, newSub: newFormNote}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.newSub != newFormBlocked {
		t.Errorf("expected newFormBlocked, got %v", m.newSub)
	}
}

func TestNewTaskUpFromTaskGoesToGoal(t *testing.T) {
	m := updateModel{phase: phaseNewTask, newSub: newFormTask}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.newSub != newFormGoal {
		t.Errorf("expected newFormGoal after Up from task, got %v", m.newSub)
	}
}

func TestNewTaskUpFromNoteGoesToTask(t *testing.T) {
	m := updateModel{phase: phaseNewTask, newSub: newFormNote}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.newSub != newFormTask {
		t.Errorf("expected newFormTask after Up from note, got %v", m.newSub)
	}
}

func TestNewTaskUpFromBlockedGoesToNote(t *testing.T) {
	m := updateModel{phase: phaseNewTask, newSub: newFormBlocked}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.newSub != newFormNote {
		t.Errorf("expected newFormNote after Up from blocked, got %v", m.newSub)
	}
}

func TestDoneTaskNotInActiveTasks(t *testing.T) {
	// Done tasks should not appear in activeTasks (filtered during Init).
	m := updateModel{}
	msg := taskStatesLoadedMsg{
		activeTasks: []activeTask{},
		username:    "alice",
	}
	m, _ = m.Update(msg)
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu, got %v", m.phase)
	}
	if len(m.activeTasks) != 0 {
		t.Errorf("expected no active tasks, got %d", len(m.activeTasks))
	}
}

func TestGoalsLoadedWithNoGoalsDoesNotError(t *testing.T) {
	m := updateModel{phase: phaseNewTask, newSub: newFormGoal}
	// Empty goals list — previously caused a fatal error, now should be fine
	m, _ = m.Update(goalsLoadedMsg{goals: []goals.Goal{}})
	if m.err != nil {
		t.Errorf("expected no error with empty goals, got: %v", m.err)
	}
	if m.newSub != newFormGoal {
		t.Errorf("expected newSub=newFormGoal after goals load, got %v", m.newSub)
	}
}

func TestGoalPickerIncludesSentinel(t *testing.T) {
	items := goalPickerItems([]goals.Goal{{ID: "ROUTING", State: "open"}})
	if len(items) != 2 {
		t.Fatalf("expected 2 items (sentinel + 1 goal), got %d", len(items))
	}
	if items[0] != noGoalSentinel {
		t.Errorf("expected first item to be %q, got %q", noGoalSentinel, items[0])
	}
	if items[1] != "ROUTING" {
		t.Errorf("expected second item to be %q, got %q", "ROUTING", items[1])
	}
}

func TestGoalPickerSentinelOnlyWhenNoGoals(t *testing.T) {
	items := goalPickerItems([]goals.Goal{})
	if len(items) != 1 || items[0] != noGoalSentinel {
		t.Errorf("expected only sentinel when no goals, got %v", items)
	}
}

func TestSelectingNoGoalSentinelAdvancesToTask(t *testing.T) {
	m := updateModel{
		phase:    phaseNewTask,
		newSub:   newFormGoal,
		allGoals: []goals.Goal{{ID: "ROUTING", State: "open"}},
		goalPicker: pickerModel{
			items:   goalPickerItems([]goals.Goal{{ID: "ROUTING", State: "open"}}),
			matches: goalPickerItems([]goals.Goal{{ID: "ROUTING", State: "open"}}),
		},
	}
	// The picker's first match is the sentinel; Enter selects it
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.newSub != newFormTask {
		t.Errorf("expected newFormTask after selecting no-goal sentinel, got %v", m2.newSub)
	}
	if m2.selectedGoal != "" {
		t.Errorf("expected empty selectedGoal, got %q", m2.selectedGoal)
	}
}

func TestGoalPickerIncludesSentinelOnEntry(t *testing.T) {
	// When goals load, picker should include the sentinel as first item.
	m := updateModel{phase: phaseNewTask, newSub: newFormGoal}
	m, _ = m.Update(goalsLoadedMsg{goals: []goals.Goal{{ID: "PROJ", State: "open"}}})
	if len(m.goalPicker.items) == 0 || m.goalPicker.items[0] != noGoalSentinel {
		t.Errorf("expected sentinel as first picker item, got %v", m.goalPicker.items)
	}
}

func TestExistingTaskInfoDetectsMatch(t *testing.T) {
	m := updateModel{
		selectedGoal: "ROUTING",
		selectedTag:  "#impl",
		activeTasks: []activeTask{
			makeActiveTask("#impl", strPtr("ROUTING"), "fixing the layer", false, 2),
		},
	}
	info := m.existingTaskInfo()
	if info == "" {
		t.Error("expected existing task info, got empty string")
	}
	if !strings.Contains(info, "fixing the layer") {
		t.Errorf("expected last note in info, got %q", info)
	}
}

func TestExistingTaskInfoNoMatchDifferentGoal(t *testing.T) {
	m := updateModel{
		selectedGoal: "OTHER",
		selectedTag:  "#impl",
		activeTasks: []activeTask{
			makeActiveTask("#impl", strPtr("ROUTING"), "fixing the layer", false, 2),
		},
	}
	if info := m.existingTaskInfo(); info != "" {
		t.Errorf("expected no info for different goal, got %q", info)
	}
}

func tabKey() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyTab}
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestAtMentionSuffix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"fixed bug for @rob", "@rob"},
		{"@", "@"},
		{"hello @", "@"},
		{"no mention here", ""},
		{"ends with space ", ""},
		{"@alice-bob", "@alice-bob"},
		{"note @rob extra", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := atMentionSuffix(tc.input)
		if got != tc.want {
			t.Errorf("atMentionSuffix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestUsernamesLoadedMsgStoresUsernames(t *testing.T) {
	m := updateModel{}
	m, _ = m.Update(usernamesLoadedMsg{usernames: []string{"alice", "bob"}})
	if len(m.knownUsernames) != 2 {
		t.Fatalf("expected 2 knownUsernames, got %d", len(m.knownUsernames))
	}
	if m.knownUsernames[0] != "alice" || m.knownUsernames[1] != "bob" {
		t.Errorf("unexpected knownUsernames: %v", m.knownUsernames)
	}
}

func TestUsernamesLoadedMsgIgnoresError(t *testing.T) {
	m := updateModel{knownUsernames: []string{"alice"}}
	m, _ = m.Update(usernamesLoadedMsg{err: errors.New("disk error")})
	if len(m.knownUsernames) != 1 {
		t.Errorf("expected knownUsernames unchanged on error, got %v", m.knownUsernames)
	}
}

func TestTabCompletesAtMentionInTaskUpdateNote(t *testing.T) {
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateNote,
		taskUpdateNote: newNoteInput("fixed for @ali"),
		knownUsernames: []string{"alice", "alicia"},
	}
	m, _ = m.Update(tabKey())
	if m.taskUpdateNote.value != "fixed for @alice" {
		t.Errorf("expected first completion, got %q", m.taskUpdateNote.value)
	}
	if !m.mentionCompletion.active {
		t.Error("expected completion to be active")
	}
}

func TestTabCyclesCandidatesInTaskUpdateNote(t *testing.T) {
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateNote,
		taskUpdateNote: newNoteInput("fixed for @ali"),
		knownUsernames: []string{"alice", "alicia"},
	}
	m, _ = m.Update(tabKey())
	m, _ = m.Update(tabKey())
	if m.taskUpdateNote.value != "fixed for @alicia" {
		t.Errorf("expected second candidate on second Tab, got %q", m.taskUpdateNote.value)
	}
	m, _ = m.Update(tabKey())
	if m.taskUpdateNote.value != "fixed for @alice" {
		t.Errorf("expected wrap back to first candidate, got %q", m.taskUpdateNote.value)
	}
}

func TestTabNoMatchLeavesNoteUnchanged(t *testing.T) {
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateNote,
		taskUpdateNote: newNoteInput("fixed for @xyz"),
		knownUsernames: []string{"alice", "bob"},
	}
	m, _ = m.Update(tabKey())
	if m.taskUpdateNote.value != "fixed for @xyz" {
		t.Errorf("expected note unchanged when no match, got %q", m.taskUpdateNote.value)
	}
	if m.mentionCompletion.active {
		t.Error("expected completion inactive when no match")
	}
}

func TestNonTabKeyResetsCompletion(t *testing.T) {
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateNote,
		taskUpdateNote: newNoteInput("fixed for @robobario"),
		knownUsernames: []string{"robobario"},
		mentionCompletion: mentionCompletion{
			active:     true,
			prefix:     "rob",
			candidates: []string{"robobario"},
			index:      0,
		},
	}
	m, _ = m.Update(runeKey(' '))
	if m.mentionCompletion.active {
		t.Error("expected completion reset after non-tab key")
	}
}

func TestTabCompletesEmptyPrefixMatchesAll(t *testing.T) {
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateNote,
		taskUpdateNote: newNoteInput("ping @"),
		knownUsernames: []string{"alice", "bob"},
	}
	m, _ = m.Update(tabKey())
	if m.taskUpdateNote.value != "ping @alice" {
		t.Errorf("expected first user on empty prefix, got %q", m.taskUpdateNote.value)
	}
}

func TestTabCompletesAtMentionInNewTaskNote(t *testing.T) {
	m := updateModel{
		phase:          phaseNewTask,
		newSub:         newFormNote,
		newNoteInput:   newNoteInput("blocked by @ali"),
		knownUsernames: []string{"alice"},
	}
	m, _ = m.Update(tabKey())
	if m.newNoteInput.value != "blocked by @alice" {
		t.Errorf("expected completion in new task note, got %q", m.newNoteInput.value)
	}
}

func TestTabCompletesAtMentionInEditNote(t *testing.T) {
	m := updateModel{
		phase:          phaseEditEntry,
		editSub:        editNote,
		editNoteInput:  newNoteInput("cc @ali"),
		knownUsernames: []string{"alice"},
	}
	m, _ = m.Update(tabKey())
	if m.editNoteInput.value != "cc @alice" {
		t.Errorf("expected completion in edit note, got %q", m.editNoteInput.value)
	}
}

func TestTabWithNoAtSignLeavesNoteUnchanged(t *testing.T) {
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateNote,
		taskUpdateNote: newNoteInput("no mention"),
		knownUsernames: []string{"alice"},
	}
	m, _ = m.Update(tabKey())
	if m.taskUpdateNote.value != "no mention" {
		t.Errorf("expected note unchanged when no @ suffix, got %q", m.taskUpdateNote.value)
	}
}

func TestTabCompletionStripsLeadingAtFromUsername(t *testing.T) {
	// KnownUsernames may return names with a leading @ if the config Name field
	// includes one (e.g. "@SamBarker"). Completion must produce @SamBarker,
	// not @@SamBarker.
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateNote,
		taskUpdateNote: newNoteInput("ping @"),
		knownUsernames: []string{"@SamBarker"},
	}
	m, _ = m.Update(tabKey())
	if m.taskUpdateNote.value != "ping @SamBarker" {
		t.Errorf("expected @SamBarker (no double @), got %q", m.taskUpdateNote.value)
	}
}

func TestTabCompletionIsCaseInsensitive(t *testing.T) {
	// Prefix typed as uppercase should still match a lowercase stored username.
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateNote,
		taskUpdateNote: newNoteInput("fixed by @S"),
		knownUsernames: []string{"sambarker"},
	}
	m, _ = m.Update(tabKey())
	if m.taskUpdateNote.value != "fixed by @sambarker" {
		t.Errorf("expected case-insensitive match, got %q", m.taskUpdateNote.value)
	}
}

func TestTabCyclingAfterCaseInsensitiveCompletion(t *testing.T) {
	// After completing @ali → @Alice, a second Tab should still cycle (not restart).
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateNote,
		taskUpdateNote: newNoteInput("for @ali"),
		knownUsernames: []string{"Alice", "Alicia"},
	}
	m, _ = m.Update(tabKey()) // → @Alice
	m, _ = m.Update(tabKey()) // → @Alicia (not a fresh start)
	if m.taskUpdateNote.value != "for @Alicia" {
		t.Errorf("expected second candidate after cycling, got %q", m.taskUpdateNote.value)
	}
}

func TestMentionStyledInTaskUpdateNoteView(t *testing.T) {
	m := updateModel{
		phase:              phaseTaskUpdate,
		taskUpdateSub:      taskUpdateNote,
		taskUpdateSelected: activeTask{tag: "#impl"},
		taskUpdateNote:     newNoteInput("fixed with @alice"),
	}
	view := m.viewTaskUpdateForm()
	if !strings.Contains(view, "@alice") {
		t.Errorf("expected @alice in task update note view; got:\n%s", view)
	}
}

func TestMentionStyledInNewTaskNoteView(t *testing.T) {
	m := updateModel{
		phase:        phaseNewTask,
		newSub:       newFormNote,
		newNoteInput: newNoteInput("blocked by @bob"),
	}
	view := m.viewNewTask()
	if !strings.Contains(view, "@bob") {
		t.Errorf("expected @bob in new task note view; got:\n%s", view)
	}
}

func TestMentionStyledInEditNoteView(t *testing.T) {
	m := updateModel{
		phase:         phaseEditEntry,
		editSub:       editNote,
		editNoteInput: newNoteInput("reviewed by @carol"),
		editEntry:     journal.Entry{Task: strPtr("#impl")},
	}
	view := m.viewEditNote()
	if !strings.Contains(view, "@carol") {
		t.Errorf("expected @carol in edit note view; got:\n%s", view)
	}
}

func TestNoteContentWidthNoPropagation(t *testing.T) {
	m := updateModel{width: 0}
	if got := m.noteContentWidth(6); got != 0 {
		t.Errorf("expected 0 when width not set, got %d", got)
	}
}

func TestNoteContentWidthCapsAtWrapWidth(t *testing.T) {
	m := updateModel{width: 200, wrapWidth: 80}
	if got := m.noteContentWidth(6); got != 74 {
		t.Errorf("expected 74 (80-6), got %d", got)
	}
}

func TestNoteContentWidthUsesTerminalWhenNoWrapWidth(t *testing.T) {
	m := updateModel{width: 100, wrapWidth: 0}
	if got := m.noteContentWidth(6); got != 94 {
		t.Errorf("expected 94 (100-6), got %d", got)
	}
}

func TestEditNoteViewWrapsLongNote(t *testing.T) {
	long := "one two three four five six seven eight nine ten"
	m := updateModel{
		phase:         phaseEditEntry,
		editSub:       editNote,
		editNoteInput: newNoteInput(long),
		editEntry:     journal.Entry{},
		width:         20,
	}
	view := m.viewEditNote()
	// With width=20 and prefix "Note: " (6 chars), content width is 14.
	// "one two three" fits in 13 chars; "four five six seven eight nine ten" wraps.
	if !strings.Contains(view, "\n") {
		t.Errorf("expected wrapped (multi-line) note in edit view; got:\n%s", view)
	}
	// Continuation lines must be indented to align with note content start.
	if !strings.Contains(view, "\n      ") {
		t.Errorf("expected continuation indent of 6 spaces; got:\n%s", view)
	}
}

func TestTaskUpdateNoteViewWrapsLongNote(t *testing.T) {
	long := "one two three four five six seven eight nine ten"
	m := updateModel{
		phase:          phaseTaskUpdate,
		taskUpdateSub:  taskUpdateNote,
		taskUpdateSelected: activeTask{tag: "#x"},
		taskUpdateNote: newNoteInput(long),
		width:          20,
	}
	view := m.viewTaskUpdateForm()
	if !strings.Contains(view, "\n      ") {
		t.Errorf("expected wrapped note with 6-space indent; got:\n%s", view)
	}
}

func TestNewTaskNoteViewWrapsLongNote(t *testing.T) {
	long := "one two three four five six seven eight nine ten"
	m := updateModel{
		phase:        phaseNewTask,
		newSub:       newFormNote,
		newNoteInput: newNoteInput(long),
		width:        25,
	}
	view := m.viewNewTask()
	// Prefix "> Note:  " = 9 chars, content width = 16.
	if !strings.Contains(view, "\n         ") {
		t.Errorf("expected wrapped note with 9-space indent; got:\n%s", view)
	}
}

func TestReloadTaskStatesCmdEmptyUsernameReturnsError(t *testing.T) {
	ctx := &cli.AppContext{Username: ""}
	m := updateModel{ctx: ctx}
	cmd := m.reloadTaskStatesCmd()
	msg := cmd()
	loaded, ok := msg.(taskStatesLoadedMsg)
	if !ok {
		t.Fatalf("expected taskStatesLoadedMsg, got %T", msg)
	}
	if !errors.Is(loaded.err, config.ErrNotInitialised) {
		t.Errorf("expected ErrNotInitialised, got %v", loaded.err)
	}
}

func TestMenuIncludesUnblockOption(t *testing.T) {
	m := updateModel{phase: phaseMenu}
	opts := m.menuOptions()
	found := false
	for _, o := range opts {
		if o.phase == phaseUnblockTeammate {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Unblock a teammate' option in menu")
	}
}

func TestBlockedFromOthersLoadedSetsPicker(t *testing.T) {
	m := updateModel{phase: phaseUnblockTeammate, unblockSub: unblockPicking}
	entries := []journal.Entry{
		{Username: "@alice", TS: time.Now().Format(time.RFC3339), Note: "stuck", Blocked: true, Task: strPtr("#impl")},
	}
	m, _ = m.Update(blockedFromOthersLoadedMsg{entries: entries})
	if len(m.unblockPicker.items) != 1 {
		t.Fatalf("expected 1 picker item, got %d", len(m.unblockPicker.items))
	}
	if len(m.unblockByDisplay) != 1 {
		t.Fatalf("expected 1 unblockByDisplay entry, got %d", len(m.unblockByDisplay))
	}
}

func TestUnblockPickingEnterAdvancesToNote(t *testing.T) {
	entries := []journal.Entry{
		{Username: "@alice", TS: time.Now().Format(time.RFC3339), Note: "stuck", Blocked: true, Task: strPtr("#impl")},
	}
	m := updateModel{phase: phaseUnblockTeammate, unblockSub: unblockPicking}
	m, _ = m.Update(blockedFromOthersLoadedMsg{entries: entries})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.unblockSub != unblockNote {
		t.Errorf("expected unblockNote after Enter, got %v", m.unblockSub)
	}
	if m.unblockSelected.Username != "@alice" {
		t.Errorf("expected selected entry from @alice, got %+v", m.unblockSelected)
	}
}

func TestSubmitUnblockGoesToMenu(t *testing.T) {
	m := updateModel{
		phase:      phaseUnblockTeammate,
		unblockSub: unblockNote,
		unblockSelected: journal.Entry{
			Username: "@alice", Blocked: true, Task: strPtr("#impl"),
		},
		unblockNoteInput: newNoteInput("looks fine now"),
		ctx:              &cli.AppContext{},
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.phase != phaseMenu {
		t.Errorf("expected phaseMenu after submit, got %v", m.phase)
	}
}

func TestSubmitUnblockAppendsEntryWithUnblocksField(t *testing.T) {
	dataDir := t.TempDir()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	ctx := &cli.AppContext{
		DataDir:       dataDir,
		Git:           &git.FakeRunner{},
		Username:      "@bob",
		EncryptionKey: key,
	}
	m := updateModel{
		phase:      phaseUnblockTeammate,
		unblockSub: unblockNote,
		username:   "@bob",
		ctx:        ctx,
		unblockSelected: journal.Entry{
			Username: "@alice", Blocked: true, Goal: strPtr("ROUTING"), Task: strPtr("#impl"),
		},
		unblockNoteInput: newNoteInput("looks fine now"),
	}
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a cmd to append the entry")
	}
	msg := cmd()
	if done, ok := msg.(appendDoneMsg); !ok || done.err != nil {
		t.Fatalf("expected successful appendDoneMsg, got %+v", msg)
	}

	entries, err := journal.Collect(dataDir, ctx.Git, 7, "@bob", key)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry from @bob, got %d", len(entries))
	}
	e := entries[0]
	if e.Unblocks == nil || *e.Unblocks != "@alice" {
		t.Errorf("expected Unblocks=@alice, got %v", e.Unblocks)
	}
	if e.Note != "looks fine now" {
		t.Errorf("expected note preserved, got %q", e.Note)
	}
	if e.Goal == nil || *e.Goal != "ROUTING" || e.Task == nil || *e.Task != "#impl" {
		t.Errorf("expected goal/task copied from target, got goal=%v task=%v", e.Goal, e.Task)
	}
}

