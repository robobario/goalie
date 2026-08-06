package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"goalie/internal/cli"
	"goalie/internal/config"
	"goalie/internal/motd"
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
	helpBarStyle     = lipgloss.NewStyle().Faint(true).MarginTop(1)
	motdStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).MarginBottom(1)
)

type motdLoadedMsg struct {
	text string
}

type Model struct {
	ctx       *cli.AppContext
	activeTab tab
	width     int
	height    int
	activity  activityModel
	update    updateModel
	motd      string
	wrapWidth int
}

func resolveSelfUsername(ctx *cli.AppContext) string {
	if ctx.Username != "" {
		return ctx.Username
	}
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return ""
	}
	return cfg.Name
}

func resolveWrapWidth() int {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return config.DefaultWrapWidth
	}
	return cfg.EffectiveWrapWidth()
}

func initialModel(ctx *cli.AppContext) Model {
	ww := resolveWrapWidth()
	return Model{
		ctx:       ctx,
		activeTab: activityTab,
		wrapWidth: ww,
		activity: activityModel{
			selfUsername: resolveSelfUsername(ctx),
			wrapWidth:    ww,
		},
		update: updateModel{ctx: ctx},
	}
}

func wrapMotd(text string, termWidth, wrapWidth int) string {
	const prefix = "#MOTD - "
	const contIndent = "        " // 8 spaces, matching len(prefix)

	effectiveWidth := termWidth
	if wrapWidth > 0 && (effectiveWidth <= 0 || wrapWidth < effectiveWidth) {
		effectiveWidth = wrapWidth
	}
	if effectiveWidth <= 0 || len(prefix)+len(text) <= effectiveWidth {
		return prefix + text
	}

	maxFirst := effectiveWidth - len(prefix)
	firstChunk, remaining := takeFirstLine(text, maxFirst)

	var sb strings.Builder
	sb.WriteString(prefix + firstChunk)
	contWidth := effectiveWidth - len(contIndent)
	for _, cl := range wrapWords(remaining, contWidth) {
		sb.WriteString("\n" + contIndent + cl)
	}
	return sb.String()
}

func loadMotdCmd(ctx *cli.AppContext) tea.Cmd {
	return func() tea.Msg {
		text, _, _ := motd.Latest(ctx.DataDir, ctx.EncryptionKey)
		return motdLoadedMsg{text: text}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadActivityCmd(m.ctx), loadMotdCmd(m.ctx), m.update.Init())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.activeTab == updateTab && m.update.phase != phaseLoading &&
			msg.String() != "shift+left" && msg.String() != "shift+right" &&
			msg.String() != "ctrl+shift+left" && msg.String() != "ctrl+shift+right" {
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
		case "shift+right", "ctrl+shift+right":
			m.activeTab = (m.activeTab + 1) % 2
			if m.activeTab == activityTab {
				m.activity.loaded = false
				cmds = append(cmds, loadActivityCmd(m.ctx))
			}
		case "shift+left", "ctrl+shift+left":
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
		m.activity.width = msg.Width
	case entriesLoadedMsg:
		var cmd tea.Cmd
		m.activity, cmd = m.activity.Update(msg)
		cmds = append(cmds, cmd)
	case taskStatesLoadedMsg:
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
	case taskTagsLoadedMsg:
		var cmd tea.Cmd
		m.update, cmd = m.update.Update(msg)
		cmds = append(cmds, cmd)
	case usernamesLoadedMsg:
		var cmd tea.Cmd
		m.update, cmd = m.update.Update(msg)
		cmds = append(cmds, cmd)
	case editEntriesLoadedMsg:
		var cmd tea.Cmd
		m.update, cmd = m.update.Update(msg)
		cmds = append(cmds, cmd)
	case updateEntryDoneMsg:
		var cmd tea.Cmd
		m.update, cmd = m.update.Update(msg)
		cmds = append(cmds, cmd)
	case motdLoadedMsg:
		m.motd = msg.text
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

	helpBar := helpBarStyle.Render("Shift-←/→: switch view  q: quit")
	parts := []string{tabBar}
	if m.motd != "" {
		parts = append(parts, motdStyle.Render(wrapMotd(m.motd, m.width, m.wrapWidth)))
	}
	parts = append(parts, body, helpBar)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func Run(ctx *cli.AppContext) error {
	p := tea.NewProgram(initialModel(ctx), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
