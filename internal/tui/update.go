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
	phaseBlockedReview             // reviewing blocked threads one at a time
	phaseRecentReview              // Task 5 — stub
	phaseNewThread                 // Task 6 — stub
	phaseDone
)

type newThreadSub int

const (
	newGoalPick newThreadSub = iota
	newTagPick
	newNotes
	newBlocked
	newAnother
)

type goalsLoadedMsg struct {
	goals []goals.Goal
	err   error
}

type threadTagsLoadedMsg struct {
	tags []string
	err  error
}

type blockedThread struct {
	tag   string
	state journal.ThreadState
}

type recentSub int

const (
	recentList    recentSub = iota // list shown, cursor moving
	recentNotes                    // capturing notes for selected thread
	recentBlocked                  // y/n for blocked
)

type recentThread struct {
	tag   string
	state journal.ThreadState
}

type threadStatesLoadedMsg struct {
	blocked  []blockedThread
	recent   []recentThread
	username string
	err      error
}

type appendDoneMsg struct {
	err error
}

type updateModel struct {
	ctx      *cli.AppContext
	username string
	phase    updatePhase
	err      error

	blockedThreads  []blockedThread
	blockedIdx      int
	awaitingUnblock bool // showing "Is it now unblocked?" prompt
	inputMode       bool // capturing notes text
	notesInput      string
	nowUnblocked    bool

	recentThreads   []recentThread
	recentCursor    int
	recentSub       recentSub
	updatedTags     map[string]bool
	recentNotes     string
	recentUnblocked bool

	newSub       newThreadSub
	goalPicker   pickerModel
	threadPicker pickerModel
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
				return threadStatesLoadedMsg{err: err}
			}
			username = slugify.Slugify(cfg.Name)
		}
		journalDir := filepath.Join(m.ctx.DataDir, "journal")
		states, err := journal.CurrentThreadStates(journalDir, username, m.ctx.EncryptionKey)
		if err != nil {
			return threadStatesLoadedMsg{err: err}
		}
		var blocked []blockedThread
		for tag, state := range states {
			if state.Blocked {
				blocked = append(blocked, blockedThread{tag: tag, state: state})
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
		var recent []recentThread
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
			recent = append(recent, recentThread{tag: tag, state: state})
		}
		sort.Slice(recent, func(i, j int) bool {
			return recent[i].state.TS > recent[j].state.TS
		})

		return threadStatesLoadedMsg{blocked: blocked, recent: recent, username: username}
	}
}

func (m updateModel) Update(msg tea.Msg) (updateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case threadStatesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.username = msg.username
		m.blockedThreads = msg.blocked
		m.blockedIdx = 0
		m.recentThreads = msg.recent
		m.recentCursor = 0
		m.updatedTags = make(map[string]bool)
		if len(m.blockedThreads) == 0 {
			m.phase = phaseRecentReview
		} else {
			m.phase = phaseBlockedReview
		}

	case appendDoneMsg:
		if msg.err != nil {
			m.err = msg.err
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
		if len(open) == 0 {
			m.err = fmt.Errorf("No open goals — use `goalie goal add` first")
			return m, tea.Quit
		}
		m.allGoals = open
		m.goalPicker = newPicker(goalIDs(open))
		m.newSub = newGoalPick

	case threadTagsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.threadPicker = newPicker(msg.tags).withPrefix("#")

	case tea.KeyMsg:
		switch m.phase {
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
		case phaseNewThread:
			return m.handleNewThreadKey(msg)
		}
	}
	return m, nil
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
			tag := m.blockedThreads[m.blockedIdx].tag
			item := m.blockedThreads[m.blockedIdx]
			entry := journal.Entry{
				Goal:    item.state.Goal,
				Note:    entryNote,
				Blocked: !m.nowUnblocked,
				Thread:  &tag,
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
		if m.blockedIdx >= len(m.blockedThreads) {
			m.phase = phaseRecentReview
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
	}
	return m, nil
}

func (m updateModel) handleBlockedReviewKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	switch msg.String() {
	case "y":
		m.awaitingUnblock = true
	case "n":
		m.blockedIdx++
		if m.blockedIdx >= len(m.blockedThreads) {
			m.phase = phaseRecentReview
		}
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
	case "s":
		return m.enterPhaseNewThread()
	case "up":
		if m.recentCursor > 0 {
			m.recentCursor--
		}
	case "down":
		if m.recentCursor < len(m.recentThreads)-1 {
			m.recentCursor++
		}
	case "enter":
		if len(m.recentThreads) > 0 {
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
		return m.submitRecentEntry()
	case "n":
		m.recentUnblocked = true
		return m.submitRecentEntry()
	}
	return m, nil
}

func (m updateModel) submitRecentEntry() (updateModel, tea.Cmd) {
	item := m.recentThreads[m.recentCursor]
	note := strings.TrimSpace(m.recentNotes)
	isBlocked := !m.recentUnblocked

	entry := journal.Entry{
		Goal:    item.state.Goal,
		Note:    note,
		Blocked: isBlocked,
		Thread:  &item.tag,
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
	m.recentThreads = append(m.recentThreads[:m.recentCursor], m.recentThreads[m.recentCursor+1:]...)
	if m.recentCursor >= len(m.recentThreads) && m.recentCursor > 0 {
		m.recentCursor = len(m.recentThreads) - 1
	}
	m.recentNotes = ""
	m.recentUnblocked = false
	m.recentSub = recentList

	if len(m.recentThreads) == 0 {
		m2, goalsCmd := m.enterPhaseNewThread()
		return m2, tea.Batch(cmd, goalsCmd)
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
	case phaseBlockedReview:
		return m.viewBlockedReview()
	case phaseRecentReview:
		return m.viewRecentReview()
	case phaseNewThread:
		return m.viewNewThread()
	case phaseDone:
		return "All done. Press q to exit."
	}
	return ""
}

func (m updateModel) viewBlockedReview() string {
	if m.blockedIdx >= len(m.blockedThreads) {
		return ""
	}
	item := m.blockedThreads[m.blockedIdx]
	remaining := len(m.blockedThreads) - m.blockedIdx

	if m.inputMode {
		return fmt.Sprintf("Notes: %s_", m.notesInput)
	}

	if m.awaitingUnblock {
		return "Is it now unblocked? [y/n]"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Reviewing blocked threads (%d remaining)\n\n", remaining)
	if item.state.Goal != nil {
		fmt.Fprintf(&sb, "Goal:    %s\n", *item.state.Goal)
	}
	fmt.Fprintf(&sb, "Thread:  %s\n", item.tag)
	fmt.Fprintf(&sb, "Note:    %s\n", item.state.Note)
	fmt.Fprintf(&sb, "Since:   %s\n", ageString(item.state.TS, time.Now().UTC()))
	sb.WriteString("\n[y] Report changes   [n] Skip   [q] Quit")
	return sb.String()
}

func (m updateModel) viewRecentReview() string {
	switch m.recentSub {
	case recentNotes:
		if len(m.recentThreads) == 0 {
			return ""
		}
		item := m.recentThreads[m.recentCursor]
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
		return "Blocked? [y/n]"
	default:
		var sb strings.Builder
		sb.WriteString("Recent threads — select one to update (↑/↓, Enter to select, s to skip all)\n\n")
		now := time.Now().UTC()
		for i, item := range m.recentThreads {
			cursor := "  "
			if i == m.recentCursor {
				cursor = "> "
			}
			goal := ""
			if item.state.Goal != nil {
				goal = *item.state.Goal
			}
			age := ageString(item.state.TS, now)
			if goal != "" {
				fmt.Fprintf(&sb, "%s%-20s %-12s %s\n", cursor, item.tag, goal, age)
			} else {
				fmt.Fprintf(&sb, "%s%-20s %s\n", cursor, item.tag, age)
			}
		}
		return sb.String()
	}
}

func goalIDs(gs []goals.Goal) []string {
	ids := make([]string, 0, len(gs))
	for _, g := range gs {
		ids = append(ids, g.ID)
	}
	return ids
}

func (m updateModel) loadGoalsCmd() tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		gs, err := goals.List(ctx.DataDir, ctx.EncryptionKey)
		return goalsLoadedMsg{goals: gs, err: err}
	}
}

func (m updateModel) enterPhaseNewThread() (updateModel, tea.Cmd) {
	m.phase = phaseNewThread
	m.newSub = newGoalPick
	m.goalPicker = pickerModel{}
	m.threadPicker = pickerModel{prefix: "#"}
	m.selectedGoal = ""
	m.selectedTag = ""
	m.newNoteInput = ""
	m.tagError = ""
	m.newUnblocked = false
	return m, m.loadGoalsCmd()
}

func (m updateModel) handleNewThreadKey(msg tea.KeyMsg) (updateModel, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.newSub {
	case newGoalPick:
		updated, cmd, selected, wasSelected := m.goalPicker.Update(msg)
		m.goalPicker = updated
		if wasSelected && selected != "" {
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
				m.threadPicker = newPicker([]string{}).withPrefix("#")
				ctx := m.ctx
				goalID := selected
				loadTags := func() tea.Msg {
					tags, err := journal.CollectThreads(ctx.DataDir, goalID, ctx.EncryptionKey)
					return threadTagsLoadedMsg{tags: tags, err: err}
				}
				return m, tea.Batch(cmd, loadTags)
			}
		}
		return m, cmd

	case newTagPick:
		updated, cmd, selected, wasSelected := m.threadPicker.Update(msg)
		m.threadPicker = updated
		if wasSelected && selected != "" {
			inList := false
			for _, item := range m.threadPicker.items {
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
				if !goals.ValidThreadTag(selected) {
					m.tagError = "Tag must start with a lowercase letter, e.g. my-thread"
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
			return m.submitNewThread()
		case "n":
			m.newUnblocked = true
			return m.submitNewThread()
		}

	case newAnother:
		switch msg.String() {
		case "y":
			m.newSub = newGoalPick
			m.goalPicker = newPicker(goalIDs(m.allGoals))
			m.threadPicker = pickerModel{prefix: "#"}
			m.selectedGoal = ""
			m.selectedTag = ""
			m.newNoteInput = ""
			m.tagError = ""
			m.newUnblocked = false
		case "n":
			m.phase = phaseDone
		}
	}
	return m, nil
}

func (m updateModel) submitNewThread() (updateModel, tea.Cmd) {
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
		Thread:  &tag,
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

func (m updateModel) viewNewThread() string {
	switch m.newSub {
	case newGoalPick:
		return "Select a goal:\n\n" + m.goalPicker.View()
	case newTagPick:
		var sb strings.Builder
		fmt.Fprintf(&sb, "Goal: %s\n\n", m.selectedGoal)
		sb.WriteString("Select or type a thread tag:\n\n")
		sb.WriteString(m.threadPicker.View())
		if m.tagError != "" {
			sb.WriteString("\n" + m.tagError)
		}
		return sb.String()
	case newNotes:
		return fmt.Sprintf("Thread: %s\n\nNotes: %s_", m.selectedTag, m.newNoteInput)
	case newBlocked:
		return "Blocked? [y/n]"
	case newAnother:
		return "Log another new thread? [y/n]"
	}
	return ""
}
