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
	if got := Bold("hello", Context{}); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestBoldTTYTrue(t *testing.T) {
	want := "\033[1mhello\033[0m"
	if got := Bold("hello", Context{IsTTY: true}); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRedTTYFalse(t *testing.T) {
	if got := Red("err", Context{}); got != "err" {
		t.Errorf("got %q, want %q", got, "err")
	}
}

func TestTealNoTTY(t *testing.T) {
	if got := Teal("hello", Context{}); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestTealTTY(t *testing.T) {
	want := "\033[36mhello\033[0m"
	if got := Teal("hello", Context{IsTTY: true}); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatMotdNoWrap(t *testing.T) {
	got := FormatMotd("short text", Context{}, 120)
	if got != "#MOTD - short text" {
		t.Errorf("got %q", got)
	}
}

func TestFormatMotdTealTTY(t *testing.T) {
	got := FormatMotd("hi", Context{IsTTY: true}, 120)
	want := "\033[36m#MOTD - hi\033[0m"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatMotdWraps(t *testing.T) {
	// prefix is 8 chars, so with wrapWidth=20 first line gets 12 chars of text
	got := FormatMotd("one two three four", Context{}, 20)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "#MOTD - one two" {
		t.Errorf("line 0: got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "        ") {
		t.Errorf("continuation line should have 8-space indent, got %q", lines[1])
	}
}

func TestFormatMotdNoWrapWidth(t *testing.T) {
	long := strings.Repeat("x", 200)
	got := FormatMotd(long, Context{}, 0)
	if got != "#MOTD - "+long {
		t.Errorf("expected no wrapping when wrapWidth=0")
	}
}

func TestSection(t *testing.T) {
	var buf bytes.Buffer
	Section("Team", &buf, Context{})
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
	if got := Username("@alice", Context{}); got != "@alice" {
		t.Errorf("got %q, want %q", got, "@alice")
	}
}

func TestUsernameBoldTTY(t *testing.T) {
	got := Username("@alice", Context{IsTTY: true})
	if !strings.HasPrefix(got, "\033[1m") || !strings.Contains(got, "@alice") {
		t.Errorf("expected bold @alice, got %q", got)
	}
}

func TestFormatEntryIncludesAtPrefix(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "work", Username: "@alice"}
	got := FormatEntry(e, "", fixedNow, Context{})
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
	got := FormatEntry(e, "", fixedNow, Context{})
	want := "@alice work - yesterday"
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
	got := FormatEntry(e, "", fixedNow, Context{})
	if !strings.HasPrefix(got, "[BLOCKED]") {
		t.Errorf("expected [BLOCKED] prefix, got %q", got)
	}
}

func TestFormatEntryWithThread(t *testing.T) {
	e := journal.Entry{
		TS:       fixedTS,
		Note:     "note",
		Blocked:  false,
		Task:     ptr("feat-x"),
		Username: "@carol",
	}
	got := FormatEntry(e, "", fixedNow, Context{})
	want := "@carol feat-x note - yesterday"
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
	got := FormatStatusEntry(e, "", fixedNow, Context{})
	want := "GOAL note - yesterday"
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
	got := FormatStatusEntry(e, "", fixedNow, Context{})
	want := "[BLOCKED] GOAL note - yesterday"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatSummaryHeader(t *testing.T) {
	got := FormatSummaryHeader("ROUTING", "#impl", "@alice", Context{})
	want := "= ROUTING#impl@alice"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatSummaryHeaderNoGoal(t *testing.T) {
	got := FormatSummaryHeader("(no goal)", "#refactor", "@bob", Context{})
	if !strings.Contains(got, "(no goal)") || !strings.Contains(got, "@bob") {
		t.Errorf("unexpected header: %q", got)
	}
}

func TestFormatSummaryEntryNoStateChange(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "steady progress", Blocked: false}
	got := FormatSummaryEntry(e, "", false, fixedNow, Context{})
	if got != "- steady progress — yesterday" {
		t.Errorf("got %q", got)
	}
}

func TestFormatSummaryEntryBlockedStateChange(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "hit a wall", Blocked: true}
	got := FormatSummaryEntry(e, "", false, fixedNow, Context{})
	if !strings.HasPrefix(got, "- [Blocked]") {
		t.Errorf("expected [Blocked] prefix, got %q", got)
	}
}

func TestFormatSummaryEntryUnblockedStateChange(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "resolved", Blocked: false}
	got := FormatSummaryEntry(e, "", true, fixedNow, Context{})
	if !strings.HasPrefix(got, "- [Unblocked]") {
		t.Errorf("expected [Unblocked] prefix, got %q", got)
	}
}

func TestFormatSummaryEntryBlockedNoChange(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "still stuck", Blocked: true}
	got := FormatSummaryEntry(e, "", true, fixedNow, Context{})
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
	got := FormatStatusEntry(e, "", fixedNow, Context{})
	want := "note - yesterday"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHighlightMentionsNoTTY(t *testing.T) {
	got := HighlightMentions("waiting on @alice", "@alice", Context{})
	if got != "waiting on @alice" {
		t.Errorf("expected unchanged note in non-TTY, got %q", got)
	}
}

func TestHighlightMentionsOtherUserGreen(t *testing.T) {
	got := HighlightMentions("ask @bob about this", "@alice", Context{IsTTY: true})
	if !strings.Contains(got, "\033[1;32m@bob\033[0m") {
		t.Errorf("expected bold+green @bob, got %q", got)
	}
	if strings.Contains(got, "\033[1;92m") {
		t.Errorf("expected no bright-green for non-self mention, got %q", got)
	}
}

func TestHighlightMentionsSelfBrightGreen(t *testing.T) {
	got := HighlightMentions("waiting on @alice for review", "@alice", Context{IsTTY: true})
	if !strings.Contains(got, "\033[1;92m@alice\033[0m") {
		t.Errorf("expected bright-green for self mention, got %q", got)
	}
}

func TestHighlightMentionsMixed(t *testing.T) {
	got := HighlightMentions("@alice waiting on @bob", "@alice", Context{IsTTY: true})
	if !strings.Contains(got, "\033[1;92m@alice\033[0m") {
		t.Errorf("expected bright-green self-mention for @alice, got %q", got)
	}
	if !strings.Contains(got, "\033[1;32m@bob\033[0m") {
		t.Errorf("expected bold+green @bob, got %q", got)
	}
}

func TestHighlightMentionsNoMentions(t *testing.T) {
	got := HighlightMentions("no mentions here", "@alice", Context{IsTTY: true})
	if got != "no mentions here" {
		t.Errorf("expected unchanged note with no mentions, got %q", got)
	}
}

func TestWrapStatusEntryShortNoteFitsOnOneLine(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "short note", Blocked: false}
	got := WrapStatusEntry(e, "", fixedNow, Context{}, 50)
	want := "short note - yesterday"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWrapStatusEntryLongNoteWraps(t *testing.T) {
	// Note is long enough to force wrapping at width 30.
	e := journal.Entry{TS: fixedTS, Note: "one two three four five six seven", Blocked: false}
	got := WrapStatusEntry(e, "", fixedNow, Context{}, 30)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("expected wrapped output, got single line: %q", got)
	}
	// Age suffix appears on last line only.
	if !strings.Contains(lines[len(lines)-1], "yesterday") {
		t.Errorf("expected age on last line, got %q", lines[len(lines)-1])
	}
	for _, l := range lines[:len(lines)-1] {
		if strings.Contains(l, "yesterday") {
			t.Errorf("age appeared on non-last line: %q", l)
		}
	}
}

func TestWrapStatusEntryContinuationLinesIndented(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "one two three four five six seven eight nine ten", Blocked: false}
	got := WrapStatusEntry(e, "", fixedNow, Context{}, 25)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("expected wrapped output")
	}
	for _, l := range lines[1:] {
		if !strings.HasPrefix(l, "  ") {
			t.Errorf("continuation line not indented: %q", l)
		}
	}
}

func TestWrapStatusEntryZeroWidthFallback(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "note", Blocked: false}
	wrapped := WrapStatusEntry(e, "", fixedNow, Context{}, 0)
	plain := FormatStatusEntry(e, "", fixedNow, Context{})
	if wrapped != plain {
		t.Errorf("expected fallback to FormatStatusEntry, got %q", wrapped)
	}
}

func TestWrapStatusEntryWithGoalAndTask(t *testing.T) {
	e := journal.Entry{
		TS:      fixedTS,
		Note:    "short",
		Goal:    ptr("GOAL"),
		Task:    ptr("#impl"),
		Blocked: false,
	}
	got := WrapStatusEntry(e, "", fixedNow, Context{}, 50)
	if !strings.Contains(got, "GOAL#impl") {
		t.Errorf("expected concatenated GOAL#impl in output, got %q", got)
	}
}

func TestWrapStatusEntryBlockedPrefix(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "stuck", Blocked: true}
	got := WrapStatusEntry(e, "", fixedNow, Context{}, 50)
	if !strings.HasPrefix(got, "[BLOCKED]") {
		t.Errorf("expected [BLOCKED] prefix, got %q", got)
	}
}

func TestFormatStatusEntryTTYGoalTaskPreservesContent(t *testing.T) {
	e := journal.Entry{
		TS:      fixedTS,
		Note:    "note",
		Goal:    ptr("ROUTING"),
		Task:    ptr("#impl"),
		Blocked: false,
	}
	got := FormatStatusEntry(e, "", fixedNow, Context{IsTTY: true})
	if !strings.Contains(got, "ROUTING") {
		t.Errorf("expected goal ID in TTY output, got %q", got)
	}
	if !strings.Contains(got, "#impl") {
		t.Errorf("expected task tag in TTY output, got %q", got)
	}
	if !strings.Contains(got, "note") || !strings.Contains(got, "yesterday") {
		t.Errorf("expected note and age in TTY output, got %q", got)
	}
}

func TestFormatStatusEntryTTYBlockedPreservesContent(t *testing.T) {
	e := journal.Entry{TS: fixedTS, Note: "stuck", Blocked: true}
	got := FormatStatusEntry(e, "", fixedNow, Context{IsTTY: true})
	if !strings.Contains(got, "[BLOCKED]") {
		t.Errorf("expected [BLOCKED] text in TTY output, got %q", got)
	}
	if !strings.Contains(got, "stuck") {
		t.Errorf("expected note in TTY output, got %q", got)
	}
}

func TestHighlightStatusNoteTokensURLPreserved(t *testing.T) {
	got := highlightStatusNoteTokens("see https://example.com for details", "", Context{IsTTY: true})
	if !strings.Contains(got, "https://example.com") {
		t.Errorf("expected URL preserved in output, got %q", got)
	}
}

func TestHighlightStatusNoteTokensNoTTY(t *testing.T) {
	note := "see https://example.com and @alice"
	got := highlightStatusNoteTokens(note, "@alice", Context{})
	if got != note {
		t.Errorf("expected unchanged note in non-TTY, got %q", got)
	}
}

func TestHighlightStatusNoteTokensMentionPreserved(t *testing.T) {
	got := highlightStatusNoteTokens("waiting on @alice", "@alice", Context{IsTTY: true})
	if !strings.Contains(got, "@alice") {
		t.Errorf("expected @alice preserved in output, got %q", got)
	}
}

func TestHighlightStatusNoteTokensGitHubHyperLink(t *testing.T) {
	url := "https://github.com/owner/repo/pull/42"
	got := highlightStatusNoteTokens("see "+url, "", Context{IsTTY: true, HyperLinks: true})
	if !strings.Contains(got, "owner/repo#42") {
		t.Errorf("expected compressed label in %q", got)
	}
	if !strings.Contains(got, url) {
		t.Errorf("expected full URL as OSC 8 target in %q", got)
	}
	if !strings.Contains(got, "\033]8;;") {
		t.Errorf("expected OSC 8 escape in %q", got)
	}
}

func TestHighlightStatusNoteTokensNonGitHubHyperLink(t *testing.T) {
	url := "https://www.example.com/thing/x/y/lonoooooooog"
	got := highlightStatusNoteTokens("see "+url, "", Context{IsTTY: true, HyperLinks: true})
	if !strings.Contains(got, "www.example.com/thin...") {
		t.Errorf("expected truncated label in %q", got)
	}
	if !strings.Contains(got, url) {
		t.Errorf("expected full URL as OSC 8 target in %q", got)
	}
}

func TestWrapStatusEntryURLCompressedFitsOnOneLine(t *testing.T) {
	// Raw URL is 37 chars; compressed to "owner/repo#42" (13 chars).
	// With IsTTY=true and HyperLinks=true, width uses compressed length,
	// so a note that would exceed the budget at raw length fits on one line.
	url := "https://github.com/owner/repo/pull/42"
	// "see " (4) + compressed (13) + " - yesterday" (12) = 29; fits in 30.
	// Raw would be 4 + 37 + 12 = 53, which exceeds 30.
	e := journal.Entry{TS: fixedTS, Note: "see " + url, Blocked: false}
	got := WrapStatusEntry(e, "", fixedNow, Context{IsTTY: true, HyperLinks: true}, 30)
	lines := strings.Split(got, "\n")
	if len(lines) != 1 {
		t.Errorf("expected single line with IsTTY=true HyperLinks=true at width 30, got %d lines: %q", len(lines), got)
	}
}

func TestWrapStatusEntryNoTTYHyperLinksUsesRawWidth(t *testing.T) {
	// IsTTY=false: URL compression does not apply regardless of HyperLinks value.
	// Width calculation must use the raw URL length so the output is correct.
	url := "https://github.com/owner/repo/pull/42"
	// Raw: "see " (4) + url (37) + " - yesterday" (12) = 53, exceeds 30.
	e := journal.Entry{TS: fixedTS, Note: "see " + url, Blocked: false}
	got := WrapStatusEntry(e, "", fixedNow, Context{HyperLinks: true}, 30)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("expected wrap when IsTTY=false even with HyperLinks=true, got single line: %q", got)
	}
}

func TestWrapStatusEntryURLRawLengthUsedWhenNoHyperLinks(t *testing.T) {
	// Same note, but HyperLinks=false: display width uses full URL length.
	// Raw: "see " (4) + url (37) + " - yesterday" (12) = 53, exceeds 30; wraps.
	url := "https://github.com/owner/repo/pull/42"
	e := journal.Entry{TS: fixedTS, Note: "see " + url, Blocked: false}
	got := WrapStatusEntry(e, "", fixedNow, Context{}, 30)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("expected wrapped output with HyperLinks=false at width 30, got: %q", got)
	}
}

func TestFormatSummaryEntryURLCompressed(t *testing.T) {
	url := "https://github.com/owner/repo/pull/99"
	e := journal.Entry{TS: fixedTS, Note: "see " + url, Blocked: false}
	got := FormatSummaryEntry(e, "", false, fixedNow, Context{IsTTY: true, HyperLinks: true})
	if !strings.Contains(got, "owner/repo#99") {
		t.Errorf("expected compressed GitHub URL in summary, got %q", got)
	}
}
