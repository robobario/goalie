package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"goalie/internal/cli"
)

func newModel() Model {
	return initialModel(&cli.AppContext{})
}

func TestTabFromActivityLandsOnUpdate(t *testing.T) {
	m := newModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)
	if got.activeTab != updateTab {
		t.Errorf("expected updateTab after Tab from activityTab, got %v", got.activeTab)
	}
}

func TestShiftTabFromUpdateLandsOnActivity(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Errorf("expected activityTab after Shift+Tab from updateTab, got %v", got.activeTab)
	}
}

func TestTabFromUpdateWrapsToActivity(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)
	if got.activeTab != activityTab {
		t.Errorf("expected activityTab after Tab from updateTab, got %v", got.activeTab)
	}
}

func TestShiftTabFromActivityWrapsToUpdate(t *testing.T) {
	m := newModel()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	got := next.(Model)
	if got.activeTab != updateTab {
		t.Errorf("expected updateTab after Shift+Tab from activityTab, got %v", got.activeTab)
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

func TestTabToActivityTabTriggersRefresh(t *testing.T) {
	m := newModel()
	m.activeTab = updateTab
	m.activity.loaded = true
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
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

func TestShiftTabToActivityTabTriggersRefresh(t *testing.T) {
	m := newModel()
	m.activity.loaded = true
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	got := next.(Model)
	if got.activeTab != updateTab {
		t.Fatalf("expected updateTab, got %v", got.activeTab)
	}
	// shift+tab from activityTab goes to updateTab, not activityTab — no refresh
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Error("unexpected quit command")
		}
	}
	// now tab back to activityTab
	next2, cmd2 := got.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
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

func TestWindowSizeMsgNotForwardedToActivityChild(t *testing.T) {
	// activityModel has no width/height field; this test documents that
	// WindowSizeMsg is stored on the top-level Model only and is not
	// propagated to the child activity model.
	m := newModel()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := next.(Model)
	if got.width != 80 || got.height != 24 {
		t.Errorf("top-level model should record width=80 height=24, got %d %d", got.width, got.height)
	}
}
