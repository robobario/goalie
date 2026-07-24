package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"goalie/internal/cli"
	"goalie/internal/config"
	"goalie/internal/goals"
	"goalie/internal/journal"
	"goalie/internal/slugify"
)

type updatePhase int

const (
	phaseLoading    updatePhase = iota
	phaseMenu                   // top-level action menu
	phaseNewTask
	phaseEditEntry  // editing an existing journal entry
	phaseTaskUpdate // combined active-task picker + log form
	phaseDone
)

type taskUpdateSub int

const (
	taskUpdatePicking taskUpdateSub = iota // fuzzy picker over active tasks
	taskUpdateNote                         // typing the note
	taskUpdateState                        // selecting blocked/unblocked/done
)

type entryState int

const (
	entryBlocked   entryState = iota
	entryUnblocked
	entryDone
)

var entryStateLabels = []string{"Blocked", "Unblocked", "Done"}

type activeTask struct {
	tag   string
	state journal.TaskState
}

type editSub int

const (
	editPicking     editSub = iota // choosing an entry from the list
	editNote                       // editing the note text
	editTask                       // editing the task tag
	editBlockedDone                // setting blocked/done state
)

const noGoalSentinel = "(no goal)"

type newTaskSub int

const (
	newGoalPick newTaskSub = iota
	newTagPick
	newNotes
	newBlocked
	newAnother
)

type goalsLoadedMsg struct {
	goals []goals.Goal
	err   error
}

type taskTagsLoadedMsg struct {
	tags []string
	err  error
}

type taskStatesLoadedMsg struct {
	activeTasks []activeTask
	username    string
	err         error
}

type appendDoneMsg struct {
	err error
}

type editEntriesLoadedMsg struct {
	entries []journal.Entry
	err     error
}

type updateEntryDoneMsg struct {
	err error
}

type updateModel struct {
	ctx      *cli.AppContext
	username string
	phase    updatePhase
	err      error

	activeTasks []activeTask

	menuCursor   int

	// phaseTaskUpdate
	taskUpdateSub       taskUpdateSub
	taskUpdatePicker    pickerModel
	taskUpdateByDisplay map[string]activeTask
	taskUpdateSelected  activeTask
	taskUpdateNote      string
	taskUpdateState     entryState

	editSub      editSub
	editEntries  []journal.Entry
	editCursor   int
	editEntry    journal.Entry
	editNoteInput string
	editTaskInput string
	editBlocked  bool
	editDone     bool

	newSub       newTaskSub
	goalPicker   pickerModel
	taskPicker   pickerModel
	selectedGoal string
	selectedTag  string
	allGoals     []goals.Goal
	newNoteInput string
	tagError     string
	newUnblocked bool

	width  int
	height int
}

func (m updateModel) Init() tea.Cmd {
	return m.reloadTaskStatesCmd()
}

// reloadTaskStatesCmd returns a command that reads the current task states from
// the local journal directory and returns a taskStatesLoadedMsg. It is called
// at startup and after each action that writes journal data.
func (m updateModel) reloadTaskStatesCmd() tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		username := ctx.Username
		if username == "" {
			cfg, err := config.Load()
			if err != nil {
				return taskStatesLoadedMsg{err: err}
			}
			username = slugify.Slugify(cfg.Name)
		}
		journalDir := filepath.Join(ctx.DataDir, "journal")
		states, err := journal.CurrentTaskStates(journalDir, username, ctx.EncryptionKey)
		if err != nil {
			return taskStatesLoadedMsg{err: err}
		}
		// Collect blocked tasks first (sorted by goal then tag), then recent non-blocked.
		var blocked, recent []activeTask
		cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
		for tag, state := range states {
			if state.Done {
				continue
			}
			if state.Blocked {
				blocked = append(blocked, activeTask{tag: tag, state: state})
				continue
			}
			if state.TS == "" {
				continue
			}
			ts, parseErr := time.Parse(time.RFC3339, state.TS)
			if parseErr != nil || ts.Before(cutoff) {
				continue
			}
			recent = append(recent, activeTask{tag: tag, state: state})
		}
		sort.Slice(blocked, func(i, j int) bool {
			gi, gj := "", ""
			if blocked[i].state.Goal != nil {
				gi = *blocked[i].state.Goal
			}
			if blocked[j].state.Goal != nil {
				gj = *blocked[j].state.Goal
			}
			if gi != gj {
				return gi < gj
			}
			return blocked[i].tag < blocked[j].tag
		})
		sort.Slice(recent, func(i, j int) bool {
			return recent[i].state.TS > recent[j].state.TS
		})
		activeTasks := append(blocked, recent...)
		return taskStatesLoadedMsg{activeTasks: activeTasks, username: username}
	}
}

func (m updateModel) Update(msg tea.Msg) (updateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case taskStatesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.username = msg.username
		m.activeTasks = msg.activeTasks
		m.phase = phaseMenu
		m.menuCursor = 0

	case appendDoneMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			return m, m.reloadTaskStatesCmd()
		}

	case editEntriesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.editEntries = msg.entries
		m.editCursor = 0

	case updateEntryDoneMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.phase = phaseMenu
			return m, m.reloadTaskStatesCmd()
		}

	case goalsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		var open []goals.Goal
		for _, g := range msg.goals {
			if g.State == "open" {
				open = append(open, g)
			}
		}
		m.allGoals = open
		m.goalPicker = newPicker(goalPickerItems(open))
		m.newSub = newGoalPick

	case taskTagsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.taskPicker = newPicker(msg.tags).withPrefix("#")

	case tea.KeyMsg:
		switch m.phase {
		case phaseMenu:
			return m.handleMenuKey(msg)
		case phaseNewTask:
			return m.handleNewTaskKey(msg)
		case phaseEditEntry:
			return m.handleEditKey(msg)
		case phaseTaskUpdate:
			return m.handleTaskUpdateKey(msg)
		}
	}
	return m, nil
}

type menuOption struct {
	label string
	phase updatePhase
}

func (m updateModel) menuOptions() []menuOption {
	var opts []menuOption
	opts = append(opts, menuOption{
		label: "Update a task",
		phase: phaseTaskUpdate,
	})
	opts = append(opts, menuOption{
		label: "Log progress on a new task",
		phase: phaseNewTask,
	})
	opts = append(opts, menuOption{
		label: "Edit a recent entry",
		phase: phaseEditEntry,
	})
	return opts
}

func (m updateModel) handleMenuKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	opts := m.menuOptions()
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down":
		if m.menuCursor < len(opts)-1 {
			m.menuCursor++
		}
	case "enter":
		if len(opts) == 0 {
			return m, tea.Quit
		}
		if m.menuCursor >= len(opts) {
			m.menuCursor = len(opts) - 1
		}
		switch opts[m.menuCursor].phase {
		case phaseTaskUpdate:
			return m.enterPhaseTaskUpdate(), nil
		case phaseNewTask:
			return m.enterPhaseNewTask()
		case phaseEditEntry:
			m.phase = phaseEditEntry
			m.editSub = editPicking
			m.editCursor = 0
			return m, m.loadEditEntriesCmd()
		}
	}
	return m, nil
}

func (m updateModel) viewMenu() string {
	opts := m.menuOptions()
	if len(opts) == 0 {
		return "Nothing to do. Press q to quit."
	}
	var sb strings.Builder
	sb.WriteString("What would you like to do?\n\n")
	for i, opt := range opts {
		cursor := "  "
		if i == m.menuCursor {
			cursor = "> "
		}
		sb.WriteString(cursor + opt.label + "\n")
	}
	sb.WriteString("\n↑/↓ to select, Enter to confirm, q to quit")
	return sb.String()
}

func (m updateModel) View() string {
	if m.err != nil {
		return "Error: " + m.err.Error()
	}
	switch m.phase {
	case phaseLoading:
		return "Loading..."
	case phaseMenu:
		return m.viewMenu()
	case phaseNewTask:
		return m.viewNewTask()
	case phaseEditEntry:
		return m.viewEdit()
	case phaseTaskUpdate:
		return m.viewTaskUpdate()
	case phaseDone:
		return "All done. Press q to exit."
	}
	return ""
}


func (m updateModel) loadEditEntriesCmd() tea.Cmd {
	ctx := m.ctx
	username := m.username
	return func() tea.Msg {
		entries, err := journal.Collect(ctx.DataDir, ctx.Git, 7, username, ctx.EncryptionKey)
		if err != nil {
			return editEntriesLoadedMsg{err: err}
		}
		// Reverse so newest entries appear first.
		for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
			entries[i], entries[j] = entries[j], entries[i]
		}
		return editEntriesLoadedMsg{entries: entries}
	}
}

func (m updateModel) handleEditKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch m.editSub {
	case editPicking:
		return m.handleEditPickingKey(msg)
	case editNote:
		return m.handleEditNoteKey(msg)
	case editTask:
		return m.handleEditTaskKey(msg)
	case editBlockedDone:
		return m.handleEditBlockedDoneKey(msg)
	}
	return m, nil
}

func (m updateModel) handleEditPickingKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.phase = phaseMenu
	case "up":
		if m.editCursor > 0 {
			m.editCursor--
		}
	case "down":
		if m.editCursor < len(m.editEntries)-1 {
			m.editCursor++
		}
	case "enter":
		if len(m.editEntries) > 0 {
			m.editEntry = m.editEntries[m.editCursor]
			m.editNoteInput = m.editEntry.Note
			m.editTaskInput = ""
			if m.editEntry.Task != nil {
				m.editTaskInput = *m.editEntry.Task
			}
			m.editBlocked = m.editEntry.Blocked
			m.editDone = m.editEntry.Done
			m.editSub = editNote
		}
	}
	return m, nil
}

func (m updateModel) handleEditNoteKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.phase = phaseMenu
	case "enter":
		m.editSub = editTask
	case "backspace":
		if len(m.editNoteInput) > 0 {
			m.editNoteInput = m.editNoteInput[:len(m.editNoteInput)-1]
		}
	default:
		if len(msg.Runes) == 1 {
			m.editNoteInput += string(msg.Runes)
		}
	}
	return m, nil
}

func (m updateModel) handleEditTaskKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.phase = phaseMenu
	case "enter":
		if goals.ValidTaskTag(m.editTaskInput) {
			m.editSub = editBlockedDone
		}
	case "backspace":
		if len(m.editTaskInput) > 0 {
			m.editTaskInput = m.editTaskInput[:len(m.editTaskInput)-1]
		}
	default:
		if len(msg.Runes) == 1 {
			m.editTaskInput += string(msg.Runes)
		}
	}
	return m, nil
}

func (m updateModel) handleEditBlockedDoneKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.phase = phaseMenu
	case "y":
		m.editBlocked = true
		m.editDone = false
		return m.submitEdit()
	case "n":
		m.editBlocked = false
		m.editDone = false
		return m.submitEdit()
	case "d":
		m.editBlocked = false
		m.editDone = true
		return m.submitEdit()
	}
	return m, nil
}

func (m updateModel) submitEdit() (updateModel, tea.Cmd) {
	updated := m.editEntry
	updated.Note = strings.TrimSpace(m.editNoteInput)
	updated.Blocked = m.editBlocked
	updated.Done = m.editDone
	task := m.editTaskInput
	updated.Task = &task

	original := m.editEntry
	ctx := m.ctx
	username := m.username
	cmd := func() tea.Msg {
		err := journal.UpdateEntry(ctx.DataDir, ctx.Git, username, original, updated, ctx.EncryptionKey)
		return updateEntryDoneMsg{err: err}
	}
	return m, cmd
}

func (m updateModel) viewEdit() string {
	switch m.editSub {
	case editPicking:
		return m.viewEditPicking()
	case editNote:
		return m.viewEditNote()
	case editTask:
		return m.viewEditTask()
	case editBlockedDone:
		return m.viewEditBlockedDone()
	}
	return ""
}

func (m updateModel) viewEditPicking() string {
	if len(m.editEntries) == 0 {
		return "No entries in the last 7 days.\n\nPress Esc to go back."
	}
	var sb strings.Builder
	sb.WriteString("Select an entry to edit (↑/↓, Enter to select, Esc to go back)\n\n")
	now := time.Now().UTC()
	for i, e := range m.editEntries {
		cursor := "  "
		if i == m.editCursor {
			cursor = "> "
		}
		task := ""
		if e.Task != nil {
			task = *e.Task + " "
		}
		goalPart := ""
		if e.Goal != nil {
			goalPart = "(" + *e.Goal + ") "
		}
		age := ageString(e.TS, now)
		note := e.Note
		if len(note) > 40 {
			note = note[:37] + "..."
		}
		fmt.Fprintf(&sb, "%s%s%s%s — %s\n", cursor, task, goalPart, note, age)
	}
	return sb.String()
}

func (m updateModel) viewEditNote() string {
	task := ""
	if m.editEntry.Task != nil {
		task = *m.editEntry.Task
	}
	header := "Editing entry"
	if task != "" {
		header = "Editing: " + task
	}
	return fmt.Sprintf("%s\n\nNote: %s_\n\nEnter to continue, Esc to cancel", header, m.editNoteInput)
}

func (m updateModel) viewEditTask() string {
	return fmt.Sprintf("Note: %s\n\nTask tag: %s_\n\nEnter to confirm (#hashtag required), Esc to cancel",
		strings.TrimSpace(m.editNoteInput), m.editTaskInput)
}

func (m updateModel) viewEditBlockedDone() string {
	return fmt.Sprintf("Note: %s\nTask: %s\n\nBlocked? [y]  Not blocked? [n]  Done? [d]  (Esc to cancel)",
		strings.TrimSpace(m.editNoteInput), m.editTaskInput)
}

func goalIDs(gs []goals.Goal) []string {
	ids := make([]string, 0, len(gs))
	for _, g := range gs {
		ids = append(ids, g.ID)
	}
	return ids
}

// goalPickerItems returns the picker list for goal selection: sentinel first, then goal IDs.
func goalPickerItems(gs []goals.Goal) []string {
	return append([]string{noGoalSentinel}, goalIDs(gs)...)
}

func (m updateModel) loadGoalsCmd() tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		gs, err := goals.List(ctx.DataDir, ctx.EncryptionKey)
		return goalsLoadedMsg{goals: gs, err: err}
	}
}

func (m updateModel) enterPhaseNewTask() (updateModel, tea.Cmd) {
	m.phase = phaseNewTask
	m.newSub = newGoalPick
	m.goalPicker = pickerModel{}
	m.taskPicker = pickerModel{prefix: "#"}
	m.selectedGoal = ""
	m.selectedTag = ""
	m.newNoteInput = ""
	m.tagError = ""
	m.newUnblocked = false
	return m, m.loadGoalsCmd()
}

// ── phaseTaskUpdate ──────────────────────────────────────────────────────────

func (m updateModel) enterPhaseTaskUpdate() updateModel {
	m.phase = phaseTaskUpdate
	m.taskUpdateSub = taskUpdatePicking
	m.taskUpdateNote = ""
	m.taskUpdateState = entryUnblocked

	now := time.Now().UTC()
	displays := make([]string, 0, len(m.activeTasks))
	byDisplay := make(map[string]activeTask, len(m.activeTasks))

	for _, at := range m.activeTasks {
		d := formatActiveTask(at.tag, at.state, at.state.Blocked, now)
		displays = append(displays, d)
		byDisplay[d] = at
	}

	m.taskUpdatePicker = newPicker(displays)
	m.taskUpdateByDisplay = byDisplay
	return m
}

func formatActiveTask(tag string, state journal.TaskState, blocked bool, now time.Time) string {
	prefix := ""
	if blocked {
		prefix = "[BLOCKED] "
	}
	goal := ""
	if state.Goal != nil {
		goal = *state.Goal
	}
	age := ageString(state.TS, now)
	if goal != "" {
		return fmt.Sprintf("%s%s%s %s — %s", prefix, goal, tag, state.Note, age)
	}
	return fmt.Sprintf("%s%s %s — %s", prefix, tag, state.Note, age)
}

func (m updateModel) handleTaskUpdateKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.taskUpdateSub == taskUpdatePicking {
			m.phase = phaseMenu
			return m, nil
		}
		m.taskUpdateSub = taskUpdatePicking
		return m, nil
	}
	switch m.taskUpdateSub {
	case taskUpdatePicking:
		return m.handleTaskUpdatePickingKey(msg)
	case taskUpdateNote:
		return m.handleTaskUpdateNoteKey(msg)
	case taskUpdateState:
		return m.handleTaskUpdateStateKey(msg)
	}
	return m, nil
}

func (m updateModel) handleTaskUpdatePickingKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	updated, cmd, selected, wasSelected := m.taskUpdatePicker.Update(msg)
	m.taskUpdatePicker = updated
	if wasSelected && selected != "" {
		if task, ok := m.taskUpdateByDisplay[selected]; ok {
			m.taskUpdateSelected = task
			m.taskUpdateNote = ""
			if task.state.Blocked {
				m.taskUpdateState = entryBlocked
			} else {
				m.taskUpdateState = entryUnblocked
			}
			m.taskUpdateSub = taskUpdateNote
		}
	}
	return m, cmd
}

func (m updateModel) handleTaskUpdateNoteKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "enter", "down":
		m.taskUpdateSub = taskUpdateState
	case "backspace":
		if len(m.taskUpdateNote) > 0 {
			m.taskUpdateNote = m.taskUpdateNote[:len(m.taskUpdateNote)-1]
		}
	default:
		if len(msg.Runes) == 1 {
			m.taskUpdateNote += string(msg.Runes)
		}
	}
	return m, nil
}

func (m updateModel) handleTaskUpdateStateKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "up":
		m.taskUpdateSub = taskUpdateNote
	case "left":
		if m.taskUpdateState > 0 {
			m.taskUpdateState--
		}
	case "right":
		if int(m.taskUpdateState) < len(entryStateLabels)-1 {
			m.taskUpdateState++
		}
	case "enter":
		return m.submitTaskUpdate()
	}
	return m, nil
}

func (m updateModel) submitTaskUpdate() (updateModel, tea.Cmd) {
	task := m.taskUpdateSelected
	note := strings.TrimSpace(m.taskUpdateNote)
	isBlocked := m.taskUpdateState == entryBlocked
	isDone := m.taskUpdateState == entryDone

	entry := journal.Entry{
		Goal:    task.state.Goal,
		Note:    note,
		Blocked: isBlocked,
		Done:    isDone,
		Task:    &task.tag,
	}
	ctx := m.ctx
	username := m.username
	cmd := func() tea.Msg {
		err := journal.Append(ctx.DataDir, ctx.Git, username, entry, ctx.EncryptionKey)
		return appendDoneMsg{err: err}
	}
	m.phase = phaseMenu
	return m, cmd
}

func (m updateModel) viewTaskUpdate() string {
	switch m.taskUpdateSub {
	case taskUpdatePicking:
		return m.viewTaskUpdatePicking()
	case taskUpdateNote, taskUpdateState:
		return m.viewTaskUpdateForm()
	}
	return ""
}

func (m updateModel) viewTaskUpdatePicking() string {
	if len(m.taskUpdatePicker.items) == 0 {
		return "No active tasks.\n\nPress Esc to go back."
	}
	var sb strings.Builder
	sb.WriteString("Select a task to update (type to filter, Enter to select, Esc to go back)\n\n")
	items := m.taskUpdatePicker.matches
	if len(items) == 0 {
		items = m.taskUpdatePicker.items
	}
	cursor := m.taskUpdatePicker.cursor
	for i, item := range items {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		if strings.HasPrefix(item, "[BLOCKED] ") {
			body := strings.TrimPrefix(item, "[BLOCKED] ")
			sb.WriteString(prefix + blockedStyle.Render("[BLOCKED]") + " " + body + "\n")
		} else {
			sb.WriteString(prefix + item + "\n")
		}
	}
	if m.taskUpdatePicker.query != "" {
		sb.WriteString("\nFilter: " + m.taskUpdatePicker.query + "_")
	}
	return sb.String()
}

func (m updateModel) viewTaskUpdateForm() string {
	task := m.taskUpdateSelected
	goal := ""
	if task.state.Goal != nil {
		goal = *task.state.Goal
	}

	var sb strings.Builder
	header := task.tag
	if goal != "" {
		header = goal + header
	}
	sb.WriteString(header + "\n\n")

	// Note field
	noteLine := "Note: " + m.taskUpdateNote
	if m.taskUpdateSub == taskUpdateNote {
		noteLine += "_"
	}
	sb.WriteString(noteLine + "\n\n")

	// State selector
	var stateParts []string
	for i, label := range entryStateLabels {
		if entryState(i) == m.taskUpdateState {
			stateParts = append(stateParts, "> "+label+" <")
		} else {
			stateParts = append(stateParts, "  "+label+"  ")
		}
	}
	stateLine := "State:  " + strings.Join(stateParts, "  ")
	if m.taskUpdateSub == taskUpdateState {
		sb.WriteString("[" + stateLine + "]\n\n")
	} else {
		sb.WriteString(stateLine + "\n\n")
	}

	if m.taskUpdateSub == taskUpdateNote {
		sb.WriteString("↑/↓ or Enter to move to State, Esc to go back to list")
	} else {
		sb.WriteString("←/→ to change state, Enter to submit, ↑ to edit note")
	}
	return sb.String()
}

func (m updateModel) handleNewTaskKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.phase = phaseMenu
		return m, nil
	}
	switch m.newSub {
	case newGoalPick:
		updated, cmd, selected, wasSelected := m.goalPicker.Update(msg)
		m.goalPicker = updated
		if wasSelected && selected != "" {
			if selected == noGoalSentinel {
				m.selectedGoal = ""
				m.newSub = newTagPick
				m.taskPicker = newPicker([]string{}).withPrefix("#")
				return m, cmd
			}
			validGoal := false
			for _, g := range m.allGoals {
				if g.ID == selected {
					validGoal = true
					break
				}
			}
			if validGoal {
				m.selectedGoal = selected
				m.newSub = newTagPick
				m.taskPicker = newPicker([]string{}).withPrefix("#")
				ctx := m.ctx
				goalID := selected
				loadTags := func() tea.Msg {
					tags, err := journal.CollectTasks(ctx.DataDir, goalID, ctx.EncryptionKey)
					return taskTagsLoadedMsg{tags: tags, err: err}
				}
				return m, tea.Batch(cmd, loadTags)
			}
		}
		return m, cmd

	case newTagPick:
		updated, cmd, selected, wasSelected := m.taskPicker.Update(msg)
		m.taskPicker = updated
		if wasSelected && selected != "" {
			inList := false
			for _, item := range m.taskPicker.items {
				if item == selected {
					inList = true
					break
				}
			}
			if inList {
				m.selectedTag = selected
				m.tagError = ""
				m.newSub = newNotes
			} else {
				if !goals.ValidTaskTag(selected) {
					m.tagError = "Tag must start with a lowercase letter, e.g. my-task"
				} else {
					m.selectedTag = selected
					m.tagError = ""
					m.newSub = newNotes
				}
			}
		}
		return m, cmd

	case newNotes:
		switch msg.String() {
		case "enter":
			m.newSub = newBlocked
		case "backspace":
			if len(m.newNoteInput) > 0 {
				m.newNoteInput = m.newNoteInput[:len(m.newNoteInput)-1]
			}
		default:
			if len(msg.Runes) == 1 {
				m.newNoteInput += string(msg.Runes)
			}
		}

	case newBlocked:
		switch msg.String() {
		case "y":
			m.newUnblocked = false
			return m.submitNewTask()
		case "n":
			m.newUnblocked = true
			return m.submitNewTask()
		}

	case newAnother:
		switch msg.String() {
		case "y":
			m.newSub = newGoalPick
			m.goalPicker = newPicker(goalPickerItems(m.allGoals))
			m.taskPicker = pickerModel{prefix: "#"}
			m.selectedGoal = ""
			m.selectedTag = ""
			m.newNoteInput = ""
			m.tagError = ""
			m.newUnblocked = false
		case "n", "esc":
			m.phase = phaseMenu
		}
	}
	return m, nil
}

func (m updateModel) submitNewTask() (updateModel, tea.Cmd) {
	note := strings.TrimSpace(m.newNoteInput)
	isBlocked := !m.newUnblocked
	tag := m.selectedTag
	var goalPtr *string
	if m.selectedGoal != "" {
		g := m.selectedGoal
		goalPtr = &g
	}
	entry := journal.Entry{
		Goal:    goalPtr,
		Note:    note,
		Blocked: isBlocked,
		Task:    &tag,
	}
	ctx := m.ctx
	username := m.username
	cmd := func() tea.Msg {
		err := journal.Append(ctx.DataDir, ctx.Git, username, entry, ctx.EncryptionKey)
		return appendDoneMsg{err: err}
	}
	m.newSub = newAnother
	return m, cmd
}

func (m updateModel) viewNewTask() string {
	switch m.newSub {
	case newGoalPick:
		return "Select a goal (or choose '" + noGoalSentinel + "'):\n\n" + m.goalPicker.View()
	case newTagPick:
		var sb strings.Builder
		fmt.Fprintf(&sb, "Goal: %s\n\n", m.selectedGoal)
		sb.WriteString("Select or type a task tag:\n\n")
		sb.WriteString(m.taskPicker.View())
		if m.tagError != "" {
			sb.WriteString("\n" + m.tagError)
		}
		return sb.String()
	case newNotes:
		return fmt.Sprintf("Task: %s\n\nNotes: %s_", m.selectedTag, m.newNoteInput)
	case newBlocked:
		return "Blocked? [y/n]"
	case newAnother:
		return "Log another new task? [y/n]"
	}
	return ""
}
