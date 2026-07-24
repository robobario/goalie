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

func TestViewGoalPickerContainsItems(t *testing.T) {
	m := updateModel{
		phase:    phaseNewTask,
		newSub:   newFormGoal,
		allGoals: []goals.Goal{{ID: "ROUTING", State: "open"}},
		goalPicker: pickerModel{
			items:   goalPickerItems([]goals.Goal{{ID: "ROUTING", State: "open"}}),
			matches: goalPickerItems([]goals.Goal{{ID: "ROUTING", State: "open"}}),
		},
	}
	got := m.viewGoalPicker()
	if !strings.Contains(got, "ROUTING") {
		t.Errorf("expected ROUTING in goal picker; got %q", got)
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

func TestMenuOptionsAlwaysHasThreeItems(t *testing.T) {
	// Menu always shows: Update a task, New task, Edit a recent entry.
	m := updateModel{phase: phaseMenu}
	opts := m.menuOptions()
	if len(opts) != 3 {
		t.Fatalf("expected 3 options, got %d", len(opts))
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
		taskUpdateNote: "making progress",
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
	if m.editNoteInput != "fix this tpyo" {
		t.Errorf("expected note pre-filled, got %q", m.editNoteInput)
	}
}

func TestEditNoteEnterAdvancesToTask(t *testing.T) {
	m := updateModel{
		phase:         phaseEditEntry,
		editSub:       editNote,
		editNoteInput: "corrected note",
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
		phase:       phaseNewTask,
		newSub:      newFormBlocked,
		selectedTag: "#impl",
		newNoteInput: "some progress",
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

