package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"goalie/internal/cli"
	"goalie/internal/journal"
)

func newModel() Model {
	m := initialModel(&cli.AppContext{})
	m.syncing = false
	return m
}

func TestShiftRightFromActivityLandsOnUpdate(t *testing.T) {
	m := newModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	got := next.(Model)
	if got.activeTab != updateTab {
		t.Errorf("expected updateTab after Shift+Right from activityTab, got %v", got.activeTab)
	}
}

func TestShiftLeftFromUpdateLandsOnActivity(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftLeft})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Errorf("expected activityTab after Shift+Left from updateTab, got %v", got.activeTab)
	}
}

func TestShiftRightFromUpdateWrapsToActivity(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Errorf("expected activityTab after Shift+Right from updateTab, got %v", got.activeTab)
	}
}

func TestShiftLeftFromActivityWrapsToUpdate(t *testing.T) {
	m := newModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftLeft})
	got := next.(Model)
	if got.activeTab != updateTab {
		t.Errorf("expected updateTab after Shift+Left from activityTab, got %v", got.activeTab)
	}
}

func TestCtrlCReturnsQuitCmd(t *testing.T) {
	m := newModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for ctrl+c")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg from ctrl+c cmd, got %T", msg)
	}
}

func TestQReturnsQuitCmd(t *testing.T) {
	m := newModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for q")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg from q cmd, got %T", msg)
	}
}

func TestWindowSizeMsgStoresWidthAndHeight(t *testing.T) {
	m := newModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := next.(Model)
	if got.width != 120 {
		t.Errorf("expected width=120, got %d", got.width)
	}
	if got.height != 40 {
		t.Errorf("expected height=40, got %d", got.height)
	}
}

func TestShiftRightToActivityTabTriggersRefresh(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	m.activity.loaded = true
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Fatalf("expected activityTab, got %v", got.activeTab)
	}
	if got.activity.loaded {
		t.Error("expected activity.loaded=false after switching to activity tab")
	}
	if cmd == nil {
		t.Error("expected a refresh command when switching to activity tab")
	}
}

func TestShiftLeftToActivityTabTriggersRefresh(t *testing.T) {
	m := newModel()
	m.activity.loaded = true
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyShiftLeft})
	got := next.(Model)
	if got.activeTab != updateTab {
		t.Fatalf("expected updateTab, got %v", got.activeTab)
	}
	// shift+left from activityTab goes to updateTab, not activityTab — no refresh
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Error("unexpected quit command")
		}
	}
	// now shift+left back to activityTab
	next2, cmd2 := got.Update(tea.KeyMsg{Type: tea.KeyShiftLeft})
	got2 := next2.(Model)
	if got2.activeTab != activityTab {
		t.Fatalf("expected activityTab, got %v", got2.activeTab)
	}
	if got2.activity.loaded {
		t.Error("expected activity.loaded=false after switching to activity tab")
	}
	if cmd2 == nil {
		t.Error("expected a refresh command when switching to activity tab")
	}
}

func TestShiftRightFromUpdateMenuSwitchesToActivity(t *testing.T) {
	// Regression test: Shift+Right was silently consumed by the update model when
	// phase != phaseLoading, preventing the user from returning to the
	// Activity tab.
	m := newModel()
	m.activeTab = updateTab
	m.update.phase = phaseMenu // simulate fully-loaded update tab at the menu
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Errorf("expected activityTab after Shift+Right from update menu, got %v", got.activeTab)
	}
}

func TestShiftLeftFromUpdateMenuSwitchesToActivity(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	m.update.phase = phaseMenu
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftLeft})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Errorf("expected activityTab after Shift+Left from update menu, got %v", got.activeTab)
	}
}

func TestCtrlShiftRightFromActivityLandsOnUpdate(t *testing.T) {
	m := newModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlShiftRight})
	got := next.(Model)
	if got.activeTab != updateTab {
		t.Errorf("expected updateTab after Ctrl+Shift+Right from activityTab, got %v", got.activeTab)
	}
}

func TestCtrlShiftLeftFromUpdateLandsOnActivity(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlShiftLeft})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Errorf("expected activityTab after Ctrl+Shift+Left from updateTab, got %v", got.activeTab)
	}
}

func TestCtrlShiftRightFromUpdateWrapsToActivity(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlShiftRight})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Errorf("expected activityTab after Ctrl+Shift+Right from updateTab, got %v", got.activeTab)
	}
}

func TestCtrlShiftLeftFromActivityWrapsToUpdate(t *testing.T) {
	m := newModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlShiftLeft})
	got := next.(Model)
	if got.activeTab != updateTab {
		t.Errorf("expected updateTab after Ctrl+Shift+Left from activityTab, got %v", got.activeTab)
	}
}

func TestCtrlShiftRightToActivityTabTriggersRefresh(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	m.activity.loaded = true
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlShiftRight})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Fatalf("expected activityTab, got %v", got.activeTab)
	}
	if got.activity.loaded {
		t.Error("expected activity.loaded=false after switching to activity tab")
	}
	if cmd == nil {
		t.Error("expected a refresh command when switching to activity tab")
	}
}

func TestCtrlShiftRightFromUpdateMenuSwitchesToActivity(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	m.update.phase = phaseMenu
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlShiftRight})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Errorf("expected activityTab after Ctrl+Shift+Right from update menu, got %v", got.activeTab)
	}
}

func TestCtrlShiftLeftFromUpdateMenuSwitchesToActivity(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	m.update.phase = phaseMenu
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlShiftLeft})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Errorf("expected activityTab after Ctrl+Shift+Left from update menu, got %v", got.activeTab)
	}
}

func TestViewIncludesKeyHelpBar(t *testing.T) {
	m := newModel()
	view := m.View()
	if !strings.Contains(view, "Shift") {
		t.Errorf("expected key-help bar with Shift hint in view; got:\n%s", view)
	}
	if !strings.Contains(view, "q: quit") {
		t.Errorf("expected 'q: quit' in view key-help; got:\n%s", view)
	}
}

func TestInitialModelStartsSyncing(t *testing.T) {
	m := initialModel(&cli.AppContext{})
	if !m.syncing {
		t.Error("expected initial model to start in syncing state")
	}
}

func TestSyncingBlocksKeyInput(t *testing.T) {
	m := initialModel(&cli.AppContext{})
	// syncing: true — key input other than quit must be swallowed
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Error("tab should not change while syncing")
	}
	if cmd != nil {
		t.Error("expected nil cmd while syncing on non-quit key")
	}
}

func TestSyncingAllowsQuit(t *testing.T) {
	m := initialModel(&cli.AppContext{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit cmd while syncing on ctrl+c")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("expected tea.QuitMsg from ctrl+c while syncing")
	}
}

func TestEntriesLoadedWithPulledAtClearsSyncing(t *testing.T) {
	m := initialModel(&cli.AppContext{})
	if !m.syncing {
		t.Fatal("expected syncing=true at startup")
	}
	next, _ := m.Update(entriesLoadedMsg{
		entries:  []journal.Entry{},
		pulledAt: time.Now(),
	})
	got := next.(Model)
	if got.syncing {
		t.Error("expected syncing=false after entriesLoadedMsg with non-zero pulledAt")
	}
}

func TestEntriesLoadedWithoutPulledAtKeepsSyncing(t *testing.T) {
	m := initialModel(&cli.AppContext{})
	m.syncing = true
	next, _ := m.Update(entriesLoadedMsg{
		entries: []journal.Entry{},
		// pulledAt is zero — local read, should not clear syncing
	})
	got := next.(Model)
	if !got.syncing {
		t.Error("expected syncing=true when entriesLoadedMsg has zero pulledAt")
	}
}

func TestTabSwitchToActivityTriggersSyncWhenPullDue(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	m.update.phase = phaseMenu
	// lastPulledAt is zero so pull is due
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	got := next.(Model)
	if !got.syncing {
		t.Error("expected syncing=true when switching to activity and pull is due")
	}
	if cmd == nil {
		t.Error("expected a load command after switching to activity")
	}
}

func TestTabSwitchToActivitySkipsSyncWhenRecentlyPulled(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	m.update.phase = phaseMenu
	m.activity.lastPulledAt = time.Now() // just pulled
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	got := next.(Model)
	if got.syncing {
		t.Error("expected syncing=false when pull was recent")
	}
	if cmd == nil {
		t.Error("expected a local-read command after switching to activity")
	}
}

func TestSyncingViewShowsSyncMessage(t *testing.T) {
	m := initialModel(&cli.AppContext{})
	view := m.View()
	if !strings.Contains(view, "Syncing") {
		t.Errorf("expected 'Syncing' in view while syncing; got:\n%s", view)
	}
}

func TestInitSchedulesTickWhenNotificationsEnabled(t *testing.T) {
	m := initialModel(&cli.AppContext{NotificationsEnabled: true})
	if m.Init() == nil {
		t.Fatal("expected non-nil Init cmd when notifications enabled")
	}
}

func TestTickOnActivityTabTriggersSyncAndReArms(t *testing.T) {
	m := newModel()
	m.activeTab = activityTab
	next, cmd := m.Update(tickMsg{})
	got := next.(Model)
	if !got.syncing {
		t.Error("expected syncing=true after tick while on activity tab")
	}
	if got.activity.loaded {
		t.Error("expected activity.loaded=false after tick-triggered refresh starts")
	}
	if cmd == nil {
		t.Error("expected a batched cmd (re-arm + load) after tick")
	}
}

func TestTickOnUpdateTabSkipsSyncButReArms(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	next, cmd := m.Update(tickMsg{})
	got := next.(Model)
	if got.syncing {
		t.Error("expected syncing to stay false when tick fires while on update tab")
	}
	if cmd == nil {
		t.Error("expected re-arm cmd even when tick is a no-op for the current tab")
	}
}

func TestTickTriggeredRefreshStillFiresNotifications(t *testing.T) {
	// Regression test: model.go resets m.activity.loaded=false before every
	// refresh (for the "Loading..." spinner), including tick-triggered ones.
	// The notify-diff gate must survive that reset instead of mistaking every
	// refresh for the very first load.
	fake := &fakeNotifier{}
	m := initialModel(&cli.AppContext{NotificationsEnabled: true, Username: "@me"})
	m.activity.notifier = fake
	m.syncing = false

	next, _ := m.Update(entriesLoadedMsg{entries: []journal.Entry{}, pulledAt: time.Now()})
	m = next.(Model)

	next, _ = m.Update(tickMsg{})
	m = next.(Model)
	if m.activity.loaded {
		t.Fatal("expected tick to reset activity.loaded for the spinner")
	}

	_, cmd := m.Update(entriesLoadedMsg{
		entries:  []journal.Entry{{ID: "9", Username: "@alice", Blocked: true, Note: "stuck"}},
		pulledAt: time.Now(),
	})
	if cmd == nil {
		t.Fatal("expected a notify cmd to survive a tick-triggered loaded reset")
	}
}

func TestWindowSizeMsgPropagatedToActivityChild(t *testing.T) {
	m := newModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := next.(Model)
	if got.width != 80 || got.height != 24 {
		t.Errorf("top-level model should record width=80 height=24, got %d %d", got.width, got.height)
	}
	if got.activity.width != 80 {
		t.Errorf("activity model should receive width=80 from WindowSizeMsg, got %d", got.activity.width)
	}
}
