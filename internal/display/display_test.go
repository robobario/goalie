package display

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"goalie/internal/journal"
)

func ptr(s string) *string { return &s }

var (
	fixedTS  = "2024-01-15T10:00:00Z"
	fixedNow = time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC) // 1 day after fixedTS
)

func TestBoldTTYFalse(t *testing.T) {
	if got := Bold("hello", false); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestBoldTTYTrue(t *testing.T) {
	want := "\033[1mhello\033[0m"
	if got := Bold("hello", true); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRedTTYFalse(t *testing.T) {
	if got := Red("err", false); got != "err" {
		t.Errorf("got %q, want %q", got, "err")
	}
}

func TestSection(t *testing.T) {
	var buf bytes.Buffer
	Section("Team", &buf, false)
	out := buf.String()
	// leading newline
	if !strings.HasPrefix(out, "\n") {
		t.Errorf("expected leading newline, got %q", out)
	}
	// contains the title
	if !strings.Contains(out, "── Team ") {
		t.Errorf("expected title in section, got %q", out)
	}
	// total dashes: width=44, title len=4, fixed chars=4 ("── " + " ") → 36 dashes
	dashes := strings.Count(out, "─")
	// "── " contributes 2 dashes, then 36 trailing = 38 total
	if dashes != 38 {
		t.Errorf("expected 38 '─' runes, got %d in %q", dashes, out)
	}
}

func TestUsernamePlainText(t *testing.T) {
	if got := Username("@alice", false); got != "@alice" {
		t.Errorf("got %q, want %q", got, "@alice")
	}
}

func TestUsernameBoldTTY(t *testing.T) {
	got := Username("@alice", true)
	if !strings.HasPrefix(got, "\033[1m") || !strings.Contains(got, "@alice") {
		t.Errorf("expected bold @alice, got %q", got)
	}
}

func TestFormatEntryIncludesAtPrefix(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "work", Username: "@alice"}
	got := FormatEntry(e, fixedNow, false)
	if !strings.HasPrefix(got, "@alice") {
		t.Errorf("expected @alice prefix, got %q", got)
	}
}

func TestFormatEntryUnblockedNoThread(t *testing.T) {
	e := journal.Entry{
		TS:       fixedTS,
		Note:     "work",
		Blocked:  false,
		Username: "@alice",
	}
	got := FormatEntry(e, fixedNow, false)
	want := "@alice work - 1d ago"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatEntryBlocked(t *testing.T) {
	e := journal.Entry{
		TS:       fixedTS,
		Note:     "stuck",
		Blocked:  true,
		Username: "@bob",
	}
	got := FormatEntry(e, fixedNow, false)
	if !strings.HasPrefix(got, "[BLOCKED]") {
		t.Errorf("expected [BLOCKED] prefix, got %q", got)
	}
}

func TestFormatEntryWithThread(t *testing.T) {
	e := journal.Entry{
		TS:       fixedTS,
		Note:     "note",
		Blocked:  false,
		Task:   ptr("feat-x"),
		Username: "@carol",
	}
	got := FormatEntry(e, fixedNow, false)
	want := "@carol feat-x note - 1d ago"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatStatusEntryWithGoalNoThread(t *testing.T) {
	e := journal.Entry{
		TS:      fixedTS,
		Note:    "note",
		Blocked: false,
		Goal:    ptr("GOAL"),
	}
	got := FormatStatusEntry(e, fixedNow, false)
	want := "(GOAL) note - 1d ago"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatStatusEntryBlockedWithGoal(t *testing.T) {
	e := journal.Entry{
		TS:      fixedTS,
		Note:    "note",
		Blocked: true,
		Goal:    ptr("GOAL"),
	}
	got := FormatStatusEntry(e, fixedNow, false)
	want := "[BLOCKED](GOAL) note - 1d ago"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatSummaryHeader(t *testing.T) {
	got := FormatSummaryHeader("ROUTING", "#impl", "@alice", false)
	want := "= ROUTING#impl@alice"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatSummaryHeaderNoGoal(t *testing.T) {
	got := FormatSummaryHeader("(no goal)", "#refactor", "@bob", false)
	if !strings.Contains(got, "(no goal)") || !strings.Contains(got, "@bob") {
		t.Errorf("unexpected header: %q", got)
	}
}

func TestFormatSummaryEntryNoStateChange(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "steady progress", Blocked: false}
	got := FormatSummaryEntry(e, false, fixedNow, false)
	if got != "- steady progress — 1d ago" {
		t.Errorf("got %q", got)
	}
}

func TestFormatSummaryEntryBlockedStateChange(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "hit a wall", Blocked: true}
	got := FormatSummaryEntry(e, false, fixedNow, false)
	if !strings.HasPrefix(got, "- [Blocked]") {
		t.Errorf("expected [Blocked] prefix, got %q", got)
	}
}

func TestFormatSummaryEntryUnblockedStateChange(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "resolved", Blocked: false}
	got := FormatSummaryEntry(e, true, fixedNow, false)
	if !strings.HasPrefix(got, "- [Unblocked]") {
		t.Errorf("expected [Unblocked] prefix, got %q", got)
	}
}

func TestFormatSummaryEntryBlockedNoChange(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "still stuck", Blocked: true}
	got := FormatSummaryEntry(e, true, fixedNow, false)
	if strings.Contains(got, "[Blocked]") || strings.Contains(got, "[Unblocked]") {
		t.Errorf("expected no label when state unchanged, got %q", got)
	}
	if !strings.Contains(got, "still stuck") {
		t.Errorf("expected note in output, got %q", got)
	}
}

func TestFormatStatusEntryNoGoalNoThread(t *testing.T) {
	e := journal.Entry{
		TS:      fixedTS,
		Note:    "note",
		Blocked: false,
	}
	got := FormatStatusEntry(e, fixedNow, false)
	want := "note - 1d ago"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
