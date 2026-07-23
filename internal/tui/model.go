package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"goalie/internal/cli"
)

type tab int

const (
	activityTab tab = iota
	updateTab
)

var (
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Underline(true).Padding(0, 2)
	inactiveTabStyle = lipgloss.NewStyle().Padding(0, 2)
	tabBarStyle      = lipgloss.NewStyle().MarginBottom(1)
)

type Model struct {
	ctx       *cli.AppContext
	activeTab tab
	width     int
	height    int
	activity  activityModel
	update    updateModel
}

func initialModel(ctx *cli.AppContext) Model {
	return Model{
		ctx:       ctx,
		activeTab: activityTab,
		activity:  activityModel{},
		update:    updateModel{ctx: ctx},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadActivityCmd(m.ctx), m.update.Init())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.activeTab == updateTab && (m.update.inputMode || m.update.phase == phaseNewThread) {
			var cmd tea.Cmd
			m.update, cmd = m.update.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}
		if m.activeTab == activityTab && m.activity.searchMode {
			var cmd tea.Cmd
			m.activity, cmd = m.activity.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % 2
			if m.activeTab == activityTab {
				m.activity.loaded = false
				cmds = append(cmds, loadActivityCmd(m.ctx))
			}
		case "shift+tab":
			m.activeTab = (m.activeTab - 1 + 2) % 2
			if m.activeTab == activityTab {
				m.activity.loaded = false
				cmds = append(cmds, loadActivityCmd(m.ctx))
			}
		default:
			if m.activeTab == activityTab {
				var cmd tea.Cmd
				m.activity, cmd = m.activity.Update(msg)
				cmds = append(cmds, cmd)
			} else if m.activeTab == updateTab {
				var cmd tea.Cmd
				m.update, cmd = m.update.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case entriesLoadedMsg:
		var cmd tea.Cmd
		m.activity, cmd = m.activity.Update(msg)
		cmds = append(cmds, cmd)
	case threadStatesLoadedMsg:
		var cmd tea.Cmd
		m.update, cmd = m.update.Update(msg)
		cmds = append(cmds, cmd)
	case appendDoneMsg:
		var cmd tea.Cmd
		m.update, cmd = m.update.Update(msg)
		cmds = append(cmds, cmd)
	case goalsLoadedMsg:
		var cmd tea.Cmd
		m.update, cmd = m.update.Update(msg)
		cmds = append(cmds, cmd)
	case threadTagsLoadedMsg:
		var cmd tea.Cmd
		m.update, cmd = m.update.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	var activityHeader, updateHeader string
	if m.activeTab == activityTab {
		activityHeader = activeTabStyle.Render("Activity")
		updateHeader = inactiveTabStyle.Render("Update")
	} else {
		activityHeader = inactiveTabStyle.Render("Activity")
		updateHeader = activeTabStyle.Render("Update")
	}
	tabBar := tabBarStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, activityHeader, updateHeader))

	var body string
	if m.activeTab == activityTab {
		body = m.activity.View()
	} else {
		body = m.update.View()
	}

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, body)
}

func Run(ctx *cli.AppContext) error {
	p := tea.NewProgram(initialModel(ctx), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
