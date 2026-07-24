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
	phaseLoading       updatePhase = iota
	phaseMenu                      // top-level action menu
	phaseBlockedReview             // reviewing blocked tasks one at a time
	phaseRecentReview
	phaseNewTask
	phaseEditEntry // editing an existing journal entry
	phaseDone
)

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

type blockedTask struct {
	tag   string
	state journal.TaskState
}

type recentSub int

const (
	recentList    recentSub = iota // list shown, cursor moving
	recentNotes                    // capturing notes for selected task
	recentBlocked                  // y/n for blocked
)

type recentTask struct {
	tag   string
	state journal.TaskState
}

type taskStatesLoadedMsg struct {
	blocked  []blockedTask
	recent   []recentTask
	username string
	err      error
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

	blockedTasks    []blockedTask
	blockedIdx      int
	awaitingUnblock bool // showing "Is it now unblocked?" prompt
	inputMode       bool // capturing notes text
	notesInput      string
	nowUnblocked    bool

	recentTasks     []recentTask
	recentCursor    int
	recentSub       recentSub
	updatedTags     map[string]bool
	recentNotes     string
	recentUnblocked bool
	recentDone      bool

	menuCursor   int

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
	return func() tea.Msg {
		username := m.ctx.Username
		if username == "" {
			cfg, err := config.Load()
			if err != nil {
				return taskStatesLoadedMsg{err: err}
			}
			username = slugify.Slugify(cfg.Name)
		}
		journalDir := filepath.Join(m.ctx.DataDir, "journal")
		states, err := journal.CurrentTaskStates(journalDir, username, m.ctx.EncryptionKey)
		if err != nil {
			return taskStatesLoadedMsg{err: err}
		}
		var blocked []blockedTask
		for tag, state := range states {
			if state.Blocked && !state.Done {
				blocked = append(blocked, blockedTask{tag: tag, state: state})
			}
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

		cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
		var recent []recentTask
		for tag, state := range states {
			if state.Blocked || state.TS == "" {
				continue
			}
			ts, err := time.Parse(time.RFC3339, state.TS)
			if err != nil {
				continue
			}
			if ts.Before(cutoff) {
				continue
			}
			recent = append(recent, recentTask{tag: tag, state: state})
		}
		sort.Slice(recent, func(i, j int) bool {
			return recent[i].state.TS > recent[j].state.TS
		})

		return taskStatesLoadedMsg{blocked: blocked, recent: recent, username: username}
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
		m.blockedTasks = msg.blocked
		m.blockedIdx = 0
		m.recentTasks = msg.recent
		m.recentCursor = 0
		m.updatedTags = make(map[string]bool)
		m.phase = phaseMenu
		m.menuCursor = 0

	case appendDoneMsg:
		if msg.err != nil {
			m.err = msg.err
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
		case phaseBlockedReview:
			if m.inputMode {
				return m.handleInputKey(msg)
			}
			if m.awaitingUnblock {
				return m.handleUnblockKey(msg)
			}
			return m.handleBlockedReviewKey(msg)
		case phaseRecentReview:
			return m.handleRecentReviewKey(msg)
		case phaseNewTask:
			return m.handleNewTaskKey(msg)
		case phaseEditEntry:
			return m.handleEditKey(msg)
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
	if len(m.blockedTasks) > 0 {
		opts = append(opts, menuOption{
			label: fmt.Sprintf("Review blocked tasks (%d pending)", len(m.blockedTasks)),
			phase: phaseBlockedReview,
		})
	}
	if len(m.recentTasks) > 0 {
		opts = append(opts, menuOption{
			label: "Log progress on a recent task",
			phase: phaseRecentReview,
		})
	}
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
		case phaseBlockedReview:
			m.phase = phaseBlockedReview
			m.blockedIdx = 0
			m.inputMode = false
			m.awaitingUnblock = false
		case phaseRecentReview:
			m.phase = phaseRecentReview
			m.recentCursor = 0
			m.recentSub = recentList
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

func (m updateModel) handleInputKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		note := strings.TrimSpace(m.notesInput)
		var cmd tea.Cmd
		if note != "" || m.nowUnblocked {
			entryNote := note
			if entryNote == "" {
				entryNote = "unblocked"
			}
			tag := m.blockedTasks[m.blockedIdx].tag
			item := m.blockedTasks[m.blockedIdx]
			entry := journal.Entry{
				Goal:    item.state.Goal,
				Note:    entryNote,
				Blocked: !m.nowUnblocked,
				Task:    &tag,
			}
			ctx := m.ctx
			username := m.username
			cmd = func() tea.Msg {
				err := journal.Append(ctx.DataDir, ctx.Git, username, entry, ctx.EncryptionKey)
				return appendDoneMsg{err: err}
			}
		}
		m.inputMode = false
		m.notesInput = ""
		m.nowUnblocked = false
		m.blockedIdx++
		if m.blockedIdx >= len(m.blockedTasks) {
			m.phase = phaseMenu
		}
		return m, cmd
	case "backspace":
		if len(m.notesInput) > 0 {
			m.notesInput = m.notesInput[:len(m.notesInput)-1]
		}
	default:
		if len(msg.Runes) == 1 {
			m.notesInput += string(msg.Runes)
		}
	}
	return m, nil
}

func (m updateModel) handleUnblockKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.nowUnblocked = true
		m.awaitingUnblock = false
		m.inputMode = true
	case "n":
		m.nowUnblocked = false
		m.awaitingUnblock = false
		m.inputMode = true
	case "esc":
		m.awaitingUnblock = false
		m.phase = phaseMenu
	}
	return m, nil
}

func (m updateModel) handleBlockedReviewKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.awaitingUnblock = true
	case "n":
		m.blockedIdx++
		if m.blockedIdx >= len(m.blockedTasks) {
			m.phase = phaseMenu
		}
	case "esc":
		m.phase = phaseMenu
	}
	return m, nil
}

func (m updateModel) handleRecentReviewKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch m.recentSub {
	case recentList:
		return m.handleRecentListKey(msg)
	case recentNotes:
		return m.handleRecentNotesKey(msg)
	case recentBlocked:
		return m.handleRecentBlockedKey(msg)
	}
	return m, nil
}

func (m updateModel) handleRecentListKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "s", "esc":
		m.phase = phaseMenu
		return m, nil
	case "up":
		if m.recentCursor > 0 {
			m.recentCursor--
		}
	case "down":
		if m.recentCursor < len(m.recentTasks)-1 {
			m.recentCursor++
		}
	case "enter":
		if len(m.recentTasks) > 0 {
			m.recentNotes = ""
			m.recentSub = recentNotes
		}
	}
	return m, nil
}

func (m updateModel) handleRecentNotesKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.recentSub = recentBlocked
	case "backspace":
		if len(m.recentNotes) > 0 {
			m.recentNotes = m.recentNotes[:len(m.recentNotes)-1]
		}
	default:
		if len(msg.Runes) == 1 {
			m.recentNotes += string(msg.Runes)
		}
	}
	return m, nil
}

func (m updateModel) handleRecentBlockedKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.recentUnblocked = false
		m.recentDone = false
		return m.submitRecentEntry()
	case "n":
		m.recentUnblocked = true
		m.recentDone = false
		return m.submitRecentEntry()
	case "d":
		m.recentUnblocked = true
		m.recentDone = true
		return m.submitRecentEntry()
	}
	return m, nil
}

func (m updateModel) submitRecentEntry() (updateModel, tea.Cmd) {
	item := m.recentTasks[m.recentCursor]
	note := strings.TrimSpace(m.recentNotes)
	isBlocked := !m.recentUnblocked

	entry := journal.Entry{
		Goal:    item.state.Goal,
		Note:    note,
		Blocked: isBlocked,
		Done:    m.recentDone,
		Task:    &item.tag,
	}
	ctx := m.ctx
	username := m.username
	var cmd tea.Cmd
	if note != "" {
		cmd = func() tea.Msg {
			err := journal.Append(ctx.DataDir, ctx.Git, username, entry, ctx.EncryptionKey)
			return appendDoneMsg{err: err}
		}
	}

	m.updatedTags[item.tag] = true
	m.recentTasks = append(m.recentTasks[:m.recentCursor], m.recentTasks[m.recentCursor+1:]...)
	if m.recentCursor >= len(m.recentTasks) && m.recentCursor > 0 {
		m.recentCursor = len(m.recentTasks) - 1
	}
	m.recentNotes = ""
	m.recentUnblocked = false
	m.recentDone = false
	m.recentSub = recentList

	if len(m.recentTasks) == 0 {
		m.phase = phaseMenu
		return m, cmd
	}
	return m, cmd
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
	case phaseBlockedReview:
		return m.viewBlockedReview()
	case phaseRecentReview:
		return m.viewRecentReview()
	case phaseNewTask:
		return m.viewNewTask()
	case phaseEditEntry:
		return m.viewEdit()
	case phaseDone:
		return "All done. Press q to exit."
	}
	return ""
}

func (m updateModel) viewBlockedReview() string {
	if m.blockedIdx >= len(m.blockedTasks) {
		return ""
	}
	item := m.blockedTasks[m.blockedIdx]
	remaining := len(m.blockedTasks) - m.blockedIdx

	if m.inputMode {
		return fmt.Sprintf("Notes: %s_", m.notesInput)
	}

	if m.awaitingUnblock {
		return "Is it now unblocked? [y/n]"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Reviewing blocked tasks (%d remaining)\n\n", remaining)
	if item.state.Goal != nil {
		fmt.Fprintf(&sb, "Goal:    %s\n", *item.state.Goal)
	}
	fmt.Fprintf(&sb, "Task:    %s\n", item.tag)
	fmt.Fprintf(&sb, "Note:    %s\n", item.state.Note)
	fmt.Fprintf(&sb, "Since:   %s\n", ageString(item.state.TS, time.Now().UTC()))
	sb.WriteString("\n[y] Report changes   [n] Skip   [q] Quit")
	return sb.String()
}

func (m updateModel) viewRecentReview() string {
	switch m.recentSub {
	case recentNotes:
		if len(m.recentTasks) == 0 {
			return ""
		}
		item := m.recentTasks[m.recentCursor]
		goal := ""
		if item.state.Goal != nil {
			goal = *item.state.Goal
		}
		header := item.tag
		if goal != "" {
			header = fmt.Sprintf("%s (%s)", item.tag, goal)
		}
		return fmt.Sprintf("%s — last note: %s\n\nNotes: %s_", header, item.state.Note, m.recentNotes)
	case recentBlocked:
		return "Blocked? [y] Not blocked? [n] Mark done? [d]"
	default:
		var sb strings.Builder
		sb.WriteString("Recent tasks — select one to update (↑/↓, Enter to select, s to skip all)\n\n")
		now := time.Now().UTC()
		for i, item := range m.recentTasks {
			cursor := "  "
			if i == m.recentCursor {
				cursor = "> "
			}
			goal := ""
			if item.state.Goal != nil {
				goal = *item.state.Goal
			}
			age := ageString(item.state.TS, now)
			label := item.tag
			if item.state.Done {
				label += " (closed)"
			}
			if goal != "" {
				fmt.Fprintf(&sb, "%s%-28s %-12s %s\n", cursor, label, goal, age)
			} else {
				fmt.Fprintf(&sb, "%s%-28s %s\n", cursor, label, age)
			}
		}
		return sb.String()
	}
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
