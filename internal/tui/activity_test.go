package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"goalie/internal/journal"
)

func strPtr(s string) *string { return &s }

type fakeNotifier struct {
	sent []struct{ title, message string }
}

func (f *fakeNotifier) Send(title, message string) error {
	f.sent = append(f.sent, struct{ title, message string }{title, message})
	return nil
}

func TestNotifyFiresOnNewBlockedEntryFromOtherUser(t *testing.T) {
	fake := &fakeNotifier{}
	m := activityModel{selfUsername: "@me", notifier: fake, notificationsEnabled: true}
	m, _ = m.Update(entriesLoadedMsg{entries: []journal.Entry{}})
	_, cmd := m.Update(entriesLoadedMsg{entries: []journal.Entry{
		{ID: "1", Username: "@alice", Blocked: true, Note: "stuck on setup"},
	}})
	if cmd == nil {
		t.Fatal("expected a notify cmd for a new blocked entry")
	}
	cmd()
	if len(fake.sent) != 1 || fake.sent[0].title != "Blocked" {
		t.Errorf("expected one Blocked notification, got %+v", fake.sent)
	}
}

func TestNotifyMergesBlockedAndMentionIntoOneNotification(t *testing.T) {
	fake := &fakeNotifier{}
	m := activityModel{selfUsername: "@me", notifier: fake, notificationsEnabled: true}
	m, _ = m.Update(entriesLoadedMsg{entries: []journal.Entry{}})
	_, cmd := m.Update(entriesLoadedMsg{entries: []journal.Entry{
		{ID: "1b", Username: "@alice", Blocked: true, Note: "stuck, @me can you help"},
	}})
	if cmd == nil {
		t.Fatal("expected a notify cmd for an entry that is both blocked and mentions self")
	}
	cmd()
	if len(fake.sent) != 1 {
		t.Fatalf("expected exactly one notification, got %+v", fake.sent)
	}
	if fake.sent[0].title != "Blocked & Mentioned" {
		t.Errorf("expected merged title, got %q", fake.sent[0].title)
	}
}

func TestNotifyFiresOnSelfMentionFromOtherUser(t *testing.T) {
	fake := &fakeNotifier{}
	m := activityModel{selfUsername: "@me", notifier: fake, notificationsEnabled: true}
	m, _ = m.Update(entriesLoadedMsg{entries: []journal.Entry{}})
	_, cmd := m.Update(entriesLoadedMsg{entries: []journal.Entry{
		{ID: "2", Username: "@alice", Note: "hey @me check this"},
	}})
	if cmd == nil {
		t.Fatal("expected a notify cmd for a self-mention")
	}
	cmd()
	if len(fake.sent) != 1 || fake.sent[0].title != "Mentioned" {
		t.Errorf("expected one Mentioned notification, got %+v", fake.sent)
	}
}

func TestNotifySkipsOwnEntries(t *testing.T) {
	fake := &fakeNotifier{}
	m := activityModel{selfUsername: "@me", notifier: fake, notificationsEnabled: true}
	m, _ = m.Update(entriesLoadedMsg{entries: []journal.Entry{}})
	_, cmd := m.Update(entriesLoadedMsg{entries: []journal.Entry{
		{ID: "3", Username: "@me", Blocked: true, Note: "stuck"},
	}})
	if cmd != nil {
		t.Error("expected no notify cmd for the user's own entry")
	}
}

func TestNotifySkipsFirstLoad(t *testing.T) {
	fake := &fakeNotifier{}
	m := activityModel{selfUsername: "@me", notifier: fake, notificationsEnabled: true}
	_, cmd := m.Update(entriesLoadedMsg{entries: []journal.Entry{
		{ID: "4", Username: "@alice", Blocked: true, Note: "already blocked at startup"},
	}})
	if cmd != nil {
		t.Error("expected no notify cmd on the initial load")
	}
}

func TestNotifySkipsWhenDisabled(t *testing.T) {
	fake := &fakeNotifier{}
	m := activityModel{selfUsername: "@me", notifier: fake, notificationsEnabled: false}
	m, _ = m.Update(entriesLoadedMsg{entries: []journal.Entry{}})
	_, cmd := m.Update(entriesLoadedMsg{entries: []journal.Entry{
		{ID: "5", Username: "@alice", Blocked: true, Note: "stuck"},
	}})
	if cmd != nil {
		t.Error("expected no notify cmd when notifications are disabled")
	}
}

func TestNotifyFiresOnExistingEntryEditedToBlocked(t *testing.T) {
	fake := &fakeNotifier{}
	m := activityModel{selfUsername: "@me", notifier: fake, notificationsEnabled: true}
	m, _ = m.Update(entriesLoadedMsg{entries: []journal.Entry{
		{ID: "7", Username: "@alice", Blocked: false, Note: "working on it"},
	}})
	_, cmd := m.Update(entriesLoadedMsg{entries: []journal.Entry{
		{ID: "7", Username: "@alice", Blocked: true, Note: "working on it, now stuck"},
	}})
	if cmd == nil {
		t.Fatal("expected a notify cmd when an existing entry is edited into blocked state")
	}
	cmd()
	if len(fake.sent) != 1 || fake.sent[0].title != "Blocked" {
		t.Errorf("expected one Blocked notification, got %+v", fake.sent)
	}
}

func TestNotifySkipsEntryThatStaysBlocked(t *testing.T) {
	fake := &fakeNotifier{}
	m := activityModel{selfUsername: "@me", notifier: fake, notificationsEnabled: true}
	m, _ = m.Update(entriesLoadedMsg{entries: []journal.Entry{
		{ID: "8", Username: "@alice", Blocked: true, Note: "stuck on X"},
	}})
	_, cmd := m.Update(entriesLoadedMsg{entries: []journal.Entry{
		{ID: "8", Username: "@alice", Blocked: true, Note: "still stuck on X, tried Y"},
	}})
	if cmd != nil {
		t.Error("expected no notify cmd when an already-blocked entry's note is merely edited")
	}
}

func TestNotifySkipsUnchangedEntries(t *testing.T) {
	fake := &fakeNotifier{}
	entry := journal.Entry{ID: "6", Username: "@alice", Blocked: true, Note: "stuck"}
	m := activityModel{selfUsername: "@me", notifier: fake, notificationsEnabled: true}
	m, _ = m.Update(entriesLoadedMsg{entries: []journal.Entry{entry}})
	_, cmd := m.Update(entriesLoadedMsg{entries: []journal.Entry{entry}})
	if cmd != nil {
		t.Error("expected no notify cmd when the entry ID was already seen")
	}
}

func TestActivityViewMultiLineErrorPreserved(t *testing.T) {
	m := activityModel{err: errors.New("line one\nline two\nline three")}
	got := m.View()
	want := "Error: line one\nline two\nline three"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestFilterEntriesEmptyQueryReturnsAll(t *testing.T) {
	entries := []journal.Entry{
		{Note: "foo"},
		{Note: "bar"},
	}
	result := FilterEntries(entries, "")
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
}

func TestFilterEntriesMatchesNote(t *testing.T) {
	entries := []journal.Entry{
		{Note: "deploy the service"},
		{Note: "write documentation"},
		{Note: "fix the build"},
	}
	result := FilterEntries(entries, "deploy")
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Note != "deploy the service" {
		t.Errorf("unexpected entry note: %s", result[0].Note)
	}
}

func TestFilterEntriesMatchesGoalID(t *testing.T) {
	entries := []journal.Entry{
		{Note: "progress update", Goal: strPtr("PROJ-42")},
		{Note: "another update", Goal: strPtr("PROJ-99")},
	}
	result := FilterEntries(entries, "PROJ-42")
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Goal == nil || *result[0].Goal != "PROJ-42" {
		t.Errorf("unexpected goal: %v", result[0].Goal)
	}
}

func TestFilterEntriesMatchesThread(t *testing.T) {
	entries := []journal.Entry{
		{Note: "status update", Task: strPtr("#backend")},
		{Note: "status update", Task: strPtr("#frontend")},
	}
	result := FilterEntries(entries, "#backend")
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Task == nil || *result[0].Task != "#backend" {
		t.Errorf("unexpected thread: %v", result[0].Task)
	}
}

func TestFilterEntriesMatchesUsername(t *testing.T) {
	entries := []journal.Entry{
		{Note: "some work", Username: "@alice"},
		{Note: "other work", Username: "@bob"},
	}
	result := FilterEntries(entries, "alice")
	if len(result) != 1 || result[0].Username != "@alice" {
		t.Errorf("expected alice's entry, got %v", result)
	}
}

func TestFilterEntriesMatchesAtPrefixUsername(t *testing.T) {
	entries := []journal.Entry{
		{Note: "some work", Username: "@alice"},
		{Note: "other work", Username: "@bob"},
	}
	result := FilterEntries(entries, "@alice")
	if len(result) != 1 || result[0].Username != "@alice" {
		t.Errorf("expected alice's entry when searching '@alice', got %v", result)
	}
}

func TestFilterEntriesNoMatchReturnsEmpty(t *testing.T) {
	entries := []journal.Entry{
		{Note: "working on auth"},
		{Note: "fixed pagination"},
	}
	result := FilterEntries(entries, "xyzzy99999")
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

func TestFilterEntriesFuzzyTolerance(t *testing.T) {
	entries := []journal.Entry{
		{Note: "unrelated task"},
		{Note: "bug fix for login", Task: strPtr("#bug-fix")},
	}
	result := FilterEntries(entries, "bugfix")
	if len(result) == 0 {
		t.Error("expected fuzzy match on '#bug-fix' thread for query 'bugfix', got none")
	}
	matched := false
	for _, e := range result {
		if e.Task != nil && *e.Task == "#bug-fix" {
			matched = true
		}
	}
	if !matched {
		t.Error("expected entry with thread '#bug-fix' in results")
	}
}

func TestFormatActivityEntryGoalTaskCombined(t *testing.T) {
	// GOAL_ID#task-tag should appear as one token without a space between them.
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "some work",
		Goal: strPtr("ROUTING"),
		Task: strPtr("#impl"),
	}
	got := formatActivityEntry(e, time.Now(), "", false)
	if !strings.Contains(got, "ROUTING") {
		t.Errorf("expected goal in output; got %q", got)
	}
	if !strings.Contains(got, "#impl") {
		t.Errorf("expected task tag in output; got %q", got)
	}
	// Goal should NOT be wrapped in parentheses anymore.
	if strings.Contains(got, "(ROUTING)") {
		t.Errorf("goal should not be wrapped in parentheses; got %q", got)
	}
}

func TestFormatActivityEntryGoalIncluded(t *testing.T) {
	goal := "ROUTING"
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "some work",
		Goal: strPtr(goal),
	}
	got := formatActivityEntry(e, time.Now(), "", false)
	if !strings.Contains(got, goal) {
		t.Errorf("expected goal %q in entry output; got %q", goal, got)
	}
}

func TestFormatActivityEntryTaskTagIncluded(t *testing.T) {
	tag := "#impl"
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "some work",
		Task: strPtr(tag),
	}
	got := formatActivityEntry(e, time.Now(), "", false)
	if !strings.Contains(got, tag) {
		t.Errorf("expected task tag %q in entry output; got %q", tag, got)
	}
}

func TestFormatActivityEntryDoneShowsLabel(t *testing.T) {
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "all finished",
		Done: true,
		Task: strPtr("#impl"),
	}
	got := formatActivityEntry(e, time.Now(), "", false)
	if !strings.Contains(got, "[done]") {
		t.Errorf("expected '[done]' in done entry; got %q", got)
	}
	if strings.Contains(got, "[BLOCKED]") {
		t.Errorf("expected no '[BLOCKED]' in done entry; got %q", got)
	}
}

func TestFormatActivityEntryBlockedShowsLabel(t *testing.T) {
	e := journal.Entry{
		TS:      time.Now().Format(time.RFC3339),
		Note:    "waiting",
		Blocked: true,
	}
	got := formatActivityEntry(e, time.Now(), "", false)
	if !strings.Contains(got, "[BLOCKED]") {
		t.Errorf("expected '[BLOCKED]' in blocked entry; got %q", got)
	}
	if strings.Contains(got, "[done]") {
		t.Errorf("expected no '[done]' in blocked entry; got %q", got)
	}
}

func TestFormatActivityEntryMentionHighlighted(t *testing.T) {
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "waiting on @bob for review",
	}
	got := formatActivityEntry(e, time.Now(), "@alice", false)
	if !strings.Contains(got, "@bob") {
		t.Errorf("expected @bob in output; got %q", got)
	}
}

func TestFormatActivityEntrySelfMentionPresent(t *testing.T) {
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "ask @alice to approve",
	}
	got := formatActivityEntry(e, time.Now(), "@alice", false)
	if !strings.Contains(got, "@alice") {
		t.Errorf("expected @alice in output; got %q", got)
	}
}

func TestRenderNoteWithMentionsSelf(t *testing.T) {
	got := renderNoteWithMentions("waiting on @alice", "@alice", false)
	if !strings.Contains(got, "@alice") {
		t.Errorf("expected @alice in output; got %q", got)
	}
}

func TestRenderNoteWithMentionsNoMentions(t *testing.T) {
	got := renderNoteWithMentions("no mentions here", "@alice", false)
	if got != "no mentions here" {
		t.Errorf("expected unchanged note; got %q", got)
	}
}

func TestRenderNoteWithURLHighlighted(t *testing.T) {
	got := renderNoteWithMentions("see https://example.com for details", "", false)
	if !strings.Contains(got, "https://example.com") {
		t.Errorf("expected URL in output; got %q", got)
	}
	// Plain text around the URL must survive.
	if !strings.Contains(got, "see") || !strings.Contains(got, "for details") {
		t.Errorf("surrounding text missing; got %q", got)
	}
}

func TestRenderNoteURLAndMentionTogether(t *testing.T) {
	got := renderNoteWithMentions("@bob see https://example.com", "@alice", false)
	if !strings.Contains(got, "@bob") {
		t.Errorf("expected @bob in output; got %q", got)
	}
	if !strings.Contains(got, "https://example.com") {
		t.Errorf("expected URL in output; got %q", got)
	}
}

func TestWrapWordsFitsOnOneLine(t *testing.T) {
	got := wrapWords("short text", 80)
	if len(got) != 1 || got[0] != "short text" {
		t.Errorf("expected single line, got %v", got)
	}
}

func TestWrapWordsWrapsAtWordBoundary(t *testing.T) {
	got := wrapWords("one two three four five", 11)
	// "one two" = 7, "three four" = 10, "five" = 4
	if len(got) < 2 {
		t.Fatalf("expected multiple lines, got %v", got)
	}
	for _, line := range got {
		if len(line) > 11 {
			t.Errorf("line %q exceeds maxWidth 11", line)
		}
	}
}

func TestWrapWordsZeroWidthNoWrap(t *testing.T) {
	got := wrapWords("some text", 0)
	if len(got) != 1 || got[0] != "some text" {
		t.Errorf("expected no-op for zero width, got %v", got)
	}
}

func TestWrapWordsEmptyString(t *testing.T) {
	got := wrapWords("", 80)
	if len(got) != 1 || got[0] != "" {
		t.Errorf("expected single empty string, got %v", got)
	}
}

func TestRenderActivityEntryUnblockedTargetShowsGreenTag(t *testing.T) {
	e := journal.Entry{
		TS:       time.Now().Format(time.RFC3339),
		Note:     "waiting",
		Username: "@alice",
		Blocked:  true,
		Goal:     strPtr("ROUTING"),
		Task:     strPtr("#impl"),
	}
	targets := map[journal.UnblockTarget]journal.Entry{
		{Username: "@alice", Goal: "ROUTING", Task: "#impl"}: {TS: time.Now().Add(time.Hour).Format(time.RFC3339), Username: "@bob", Note: "reviewed it"},
	}
	got := renderActivityEntry(e, time.Now(), "", false, 0, targets)
	if !strings.Contains(got, "[UNBLOCKED]") {
		t.Errorf("expected '[UNBLOCKED]' in output, got %q", got)
	}
	if strings.Contains(got, "[BLOCKED]") {
		t.Errorf("expected no '[BLOCKED]' once unblocked, got %q", got)
	}
}

func TestRenderActivityEntryUnblockedTargetShowsNestedNote(t *testing.T) {
	fixedTS := time.Now().Format(time.RFC3339)
	e := journal.Entry{
		TS:       fixedTS,
		Note:     "waiting",
		Username: "@alice",
		Blocked:  true,
		Goal:     strPtr("ROUTING"),
		Task:     strPtr("#impl"),
	}
	targets := map[journal.UnblockTarget]journal.Entry{
		{Username: "@alice", Goal: "ROUTING", Task: "#impl"}: {TS: time.Now().Add(time.Hour).Format(time.RFC3339), Username: "@bob", Note: "reviewed it"},
	}
	got := renderActivityEntry(e, time.Now(), "", false, 40, targets)
	if !strings.Contains(got, "└─") {
		t.Errorf("expected nested unblock line, got %q", got)
	}
	if !strings.Contains(got, "@bob") || !strings.Contains(got, "reviewed it") {
		t.Errorf("expected unblocking username and note, got %q", got)
	}
}

func TestRenderActivityEntryReblockedAfterUnblockShowsBlockedTag(t *testing.T) {
	// Regression for issue #153: a stale unblock entry must not hide a
	// genuinely new block on the same (username, goal, task) that comes
	// after it.
	e := journal.Entry{
		TS:       time.Now().Format(time.RFC3339),
		Note:     "blocked again",
		Username: "@alice",
		Blocked:  true,
		Goal:     strPtr("ROUTING"),
		Task:     strPtr("#impl"),
	}
	targets := map[journal.UnblockTarget]journal.Entry{
		{Username: "@alice", Goal: "ROUTING", Task: "#impl"}: {TS: time.Now().Add(-time.Hour).Format(time.RFC3339), Username: "@bob"},
	}
	got := renderActivityEntry(e, time.Now(), "", false, 0, targets)
	if !strings.Contains(got, "[BLOCKED]") {
		t.Errorf("expected '[BLOCKED]' for a block newer than the unblock, got %q", got)
	}
	if strings.Contains(got, "[UNBLOCKED]") {
		t.Errorf("expected no '[UNBLOCKED]' tag, got %q", got)
	}
}

func TestRenderActivityEntryDoneTakesPrecedenceOverUnblocked(t *testing.T) {
	e := journal.Entry{
		TS:       time.Now().Format(time.RFC3339),
		Note:     "waiting",
		Username: "@alice",
		Done:     true,
		Blocked:  true,
		Goal:     strPtr("ROUTING"),
		Task:     strPtr("#impl"),
	}
	targets := map[journal.UnblockTarget]journal.Entry{
		{Username: "@alice", Goal: "ROUTING", Task: "#impl"}: {TS: time.Now().Add(time.Hour).Format(time.RFC3339), Username: "@bob", Note: "reviewed it"},
	}
	got := renderActivityEntry(e, time.Now(), "", false, 0, targets)
	if !strings.Contains(got, "[done]") {
		t.Errorf("expected '[done]' to take precedence, got %q", got)
	}
	if strings.Contains(got, "[UNBLOCKED]") {
		t.Errorf("expected no '[UNBLOCKED]' when Done is set, got %q", got)
	}
}

func TestActivityViewShowsUnblockedTagAfterUnblock(t *testing.T) {
	now := time.Now().UTC()
	targetUsername := "@alice"
	blocked := journal.Entry{ID: "blocked-1", Note: "stuck", Username: "@alice", TS: now.Add(-time.Hour).Format(time.RFC3339), Blocked: true, Goal: strPtr("ROUTING"), Task: strPtr("#impl")}
	unblock := journal.Entry{ID: "unblock-1", Note: "looks fine now", Username: "@bob", TS: now.Format(time.RFC3339), Goal: strPtr("ROUTING"), Task: strPtr("#impl"), Unblocks: &targetUsername}
	m := activityModel{
		loaded:  true,
		width:   120,
		entries: []journal.Entry{blocked, unblock},
		unblockedTargets: map[journal.UnblockTarget]journal.Entry{
			{Username: "@alice", Goal: "ROUTING", Task: "#impl"}: unblock,
		},
	}
	m.filtered = m.entries
	view := m.View()
	if !strings.Contains(view, "[UNBLOCKED]") {
		t.Errorf("expected '[UNBLOCKED]' in view:\n%s", view)
	}
	if strings.Contains(view, "[BLOCKED]") {
		t.Errorf("expected no '[BLOCKED]' once unblocked:\n%s", view)
	}
	if !strings.Contains(view, "└─") || !strings.Contains(view, "@bob") || !strings.Contains(view, "looks fine now") {
		t.Errorf("expected nested unblocking note in view:\n%s", view)
	}
}

func TestRenderActivityEntryHeaderContainsAge(t *testing.T) {
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "some note",
		Goal: strPtr("PROJ"),
	}
	got := renderActivityEntry(e, time.Now(), "", false, 0, nil)
	lines := strings.Split(got, "\n")
	// Header is always the first line and must contain the age.
	if !strings.Contains(lines[0], "ago") {
		t.Errorf("expected age on first (header) line, got %q", lines[0])
	}
	// Note must appear on subsequent lines.
	if len(lines) < 2 {
		t.Errorf("expected header + note lines, got %q", got)
	}
}

func TestRenderActivityEntryShortNoteNoBlankLine(t *testing.T) {
	// When the note fits on the first line, no blank continuation line should appear.
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "short note",
		Goal: strPtr("PROJ"),
	}
	got := renderActivityEntry(e, time.Now(), "", false, 120, nil)
	if strings.Contains(got, "\n") {
		t.Errorf("expected single line when note fits, got %q", got)
	}
}

func TestRenderActivityEntryEmptyNoteNoBlankLine(t *testing.T) {
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "",
		Goal: strPtr("PROJ"),
	}
	got := renderActivityEntry(e, time.Now(), "", false, 80, nil)
	if strings.Contains(got, "\n") {
		t.Errorf("expected single header line for empty note, got %q", got)
	}
}

func TestRenderActivityEntryWrapsLongNote(t *testing.T) {
	longNote := strings.Repeat("word ", 30)
	longNote = strings.TrimSpace(longNote)
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: longNote,
	}
	got := renderActivityEntry(e, time.Now(), "", false, 40, nil)
	if !strings.Contains(got, "\n") {
		t.Errorf("expected multi-line output for long note at width 40, got %q", got)
	}
}

func TestRenderActivityEntryAgeOnFirstLine(t *testing.T) {
	longNote := strings.Repeat("word ", 30)
	longNote = strings.TrimSpace(longNote)
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: longNote,
	}
	got := renderActivityEntry(e, time.Now(), "", false, 40, nil)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header + note lines, got %q", got)
	}
	// Age must be on the first (header) line only.
	if !strings.Contains(lines[0], "ago") {
		t.Errorf("expected age on first line; got %q", lines[0])
	}
	for _, line := range lines[1:] {
		if strings.Contains(line, "ago") {
			t.Errorf("age should not appear on note lines; got %q", line)
		}
	}
}

func TestActivityViewWrapsLongEntry(t *testing.T) {
	longNote := strings.Repeat("word ", 40)
	longNote = strings.TrimSpace(longNote)
	m := activityModel{
		loaded:  true,
		width:   40,
		entries: []journal.Entry{{Note: longNote, Username: "@alice", TS: time.Now().Format(time.RFC3339)}},
	}
	m.filtered = m.entries
	view := m.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	// At width 40, a 200-char note must span more than one display line.
	entryLines := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "  ") {
			entryLines++
		}
	}
	if entryLines < 2 {
		t.Errorf("expected at least 2 indented entry lines for wrapped note, got %d; view:\n%s", entryLines, view)
	}
}

func TestActivityViewContinuationIndentDeeper(t *testing.T) {
	longNote := strings.Repeat("word ", 40)
	longNote = strings.TrimSpace(longNote)
	m := activityModel{
		loaded:  true,
		width:   40,
		entries: []journal.Entry{{Note: longNote, Username: "@alice", TS: time.Now().Format(time.RFC3339)}},
	}
	m.filtered = m.entries
	view := m.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	var firstEntry, continuation string
	for _, l := range lines {
		if strings.HasPrefix(l, "    ") && continuation == "" {
			continuation = l
		} else if strings.HasPrefix(l, "  ") && !strings.HasPrefix(l, "    ") && firstEntry == "" {
			firstEntry = l
		}
	}
	if firstEntry == "" {
		t.Fatal("expected a 2-space-indented entry line")
	}
	if continuation == "" {
		t.Fatal("expected a 4-space-indented continuation line")
	}
}

func makeLoadedModel(entries []journal.Entry) activityModel {
	m := activityModel{}
	m, _ = m.Update(entriesLoadedMsg{entries: entries})
	return m
}

func TestUpdateAnyRuneEntersSearchMode(t *testing.T) {
	m := activityModel{loaded: true}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !m.searchMode {
		t.Error("expected searchMode=true after typing a character")
	}
	if m.search != "a" {
		t.Errorf("expected search=%q, got %q", "a", m.search)
	}
}

func TestUpdateEscapeClearsSearchAndExitsSearchMode(t *testing.T) {
	m := activityModel{loaded: true, searchMode: true, search: "hello"}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.searchMode {
		t.Error("expected searchMode=false after escape")
	}
	if m.search != "" {
		t.Errorf("expected empty search after escape, got %q", m.search)
	}
}

func TestUpdateBackspaceToEmptyExitsSearchMode(t *testing.T) {
	m := activityModel{loaded: true, searchMode: true, search: "a"}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.searchMode {
		t.Error("expected searchMode=false after backspacing to empty")
	}
	if m.search != "" {
		t.Errorf("expected empty search, got %q", m.search)
	}
}

func TestUpdateTypingFiltersEntries(t *testing.T) {
	entries := []journal.Entry{
		{Note: "deploy service", TS: time.Now().Format(time.RFC3339)},
		{Note: "write docs", TS: time.Now().Format(time.RFC3339)},
	}
	m := makeLoadedModel(entries)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	if m.search != "dep" {
		t.Errorf("expected search=%q, got %q", "dep", m.search)
	}
	if len(m.filtered) == 0 {
		t.Error("expected at least one filtered entry matching 'dep'")
	}
	for _, e := range m.filtered {
		if e.Note != "deploy service" {
			t.Errorf("unexpected entry in filtered: %s", e.Note)
		}
	}
}

func TestActivityViewHidesOldNonDoneEntryWhenNonDoneDaysSet(t *testing.T) {
	oldTS := time.Now().UTC().AddDate(0, 0, -10).Format(time.RFC3339)
	m := activityModel{
		loaded:      true,
		nonDoneDays: 8,
		entries: []journal.Entry{
			{Note: "old active task", Username: "@alice", TS: oldTS, Done: false},
		},
	}
	m.filtered = m.entries
	view := m.View()
	if strings.Contains(view, "old active task") {
		t.Errorf("expected old non-done entry hidden when nonDoneDays=8; got view:\n%s", view)
	}
}

func TestActivityViewShowsRecentNonDoneEntryWhenNonDoneDaysSet(t *testing.T) {
	recentTS := time.Now().UTC().AddDate(0, 0, -3).Format(time.RFC3339)
	m := activityModel{
		loaded:      true,
		width:       120,
		nonDoneDays: 8,
		entries: []journal.Entry{
			{Note: "active task", Username: "@alice", TS: recentTS, Done: false},
		},
	}
	m.filtered = m.entries
	view := m.View()
	if !strings.Contains(view, "active task") {
		t.Errorf("expected recent non-done entry visible when nonDoneDays=8; got view:\n%s", view)
	}
}

func TestActivityViewShowsOldNonDoneEntryWhenNonDoneDaysZero(t *testing.T) {
	oldTS := time.Now().UTC().AddDate(0, 0, -20).Format(time.RFC3339)
	m := activityModel{
		loaded:      true,
		width:       120,
		nonDoneDays: 0,
		entries: []journal.Entry{
			{Note: "old task no limit", Username: "@alice", TS: oldTS, Done: false},
		},
	}
	m.filtered = m.entries
	view := m.View()
	if !strings.Contains(view, "old task no limit") {
		t.Errorf("expected old non-done entry visible when nonDoneDays=0; got view:\n%s", view)
	}
}

func TestActivityViewHidesOldDoneEntry(t *testing.T) {
	oldTS := time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)
	m := activityModel{
		loaded: true,
		entries: []journal.Entry{
			{Note: "old done task", Username: "@alice", TS: oldTS, Done: true},
		},
	}
	m.filtered = m.entries
	view := m.View()
	if strings.Contains(view, "old done task") {
		t.Errorf("expected old done entry to be hidden; got view:\n%s", view)
	}
}

func TestActivityViewShowsRecentDoneEntry(t *testing.T) {
	recentTS := time.Now().UTC().Format(time.RFC3339)
	m := activityModel{
		loaded: true,
		width:  120,
		entries: []journal.Entry{
			{Note: "just finished", Username: "@alice", TS: recentTS, Done: true},
		},
	}
	m.filtered = m.entries
	view := m.View()
	if !strings.Contains(view, "just finished") {
		t.Errorf("expected recent done entry to be visible; got view:\n%s", view)
	}
}

func TestActivityViewOrdersDoneEntriesAfterLivingEntries(t *testing.T) {
	recentTS := time.Now().UTC().Format(time.RFC3339)
	m := activityModel{
		loaded: true,
		width:  120,
		entries: []journal.Entry{
			{Note: "just finished", Username: "@alice", TS: recentTS, Done: true},
			{Note: "still going", Username: "@alice", TS: recentTS, Done: false},
		},
	}
	m.filtered = m.entries
	view := m.View()
	livingIdx := strings.Index(view, "still going")
	doneIdx := strings.Index(view, "just finished")
	if livingIdx == -1 || doneIdx == -1 {
		t.Fatalf("expected both entries in view:\n%s", view)
	}
	if doneIdx < livingIdx {
		t.Errorf("expected done entry after living entry, got view:\n%s", view)
	}
}

func TestActivityViewWrapWidthCapsAtConfiguredWidth(t *testing.T) {
	// Terminal is 200 wide, wrapWidth set to 40. Lines should wrap at 40, not 200.
	longNote := strings.Repeat("word ", 30)
	longNote = strings.TrimSpace(longNote)
	m := activityModel{
		loaded:    true,
		width:     200,
		wrapWidth: 40,
		entries:   []journal.Entry{{Note: longNote, Username: "@alice", TS: time.Now().Format(time.RFC3339)}},
	}
	m.filtered = m.entries
	view := m.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	entryLines := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "  ") {
			entryLines++
		}
	}
	if entryLines < 2 {
		t.Errorf("expected wrapping at wrapWidth=40 even with width=200, got %d entry lines; view:\n%s", entryLines, view)
	}
}

func TestActivityViewWrapWidthDoesNotExceedTerminalWidth(t *testing.T) {
	// wrapWidth > terminal width: terminal width governs.
	longNote := strings.Repeat("word ", 30)
	longNote = strings.TrimSpace(longNote)
	m := activityModel{
		loaded:    true,
		width:     40,
		wrapWidth: 200,
		entries:   []journal.Entry{{Note: longNote, Username: "@alice", TS: time.Now().Format(time.RFC3339)}},
	}
	m.filtered = m.entries
	view := m.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	entryLines := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "  ") {
			entryLines++
		}
	}
	if entryLines < 2 {
		t.Errorf("expected wrapping at terminal width=40 when wrapWidth=200, got %d entry lines; view:\n%s", entryLines, view)
	}
}

func TestRenderNoteWrappedNoWrap(t *testing.T) {
	got := renderNoteWrapped("hello world", "", false, 20)
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

func TestRenderNoteWrappedBreaksAtWordBoundary(t *testing.T) {
	got := renderNoteWrapped("one two three", "", false, 7)
	want := "one two\nthree"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderNoteWrappedZeroMaxWidth(t *testing.T) {
	got := renderNoteWrapped("one two three", "", false, 0)
	want := renderNoteWithMentions("one two three", "", false)
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderNoteWithMentionsGitHubHyperLink(t *testing.T) {
	url := "https://github.com/owner/repo/pull/42"
	got := renderNoteWithMentions("see "+url, "", true)
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

func TestRenderActivityEntryURLCompressedFitsOnFirstLine(t *testing.T) {
	// Raw URL is 37 chars; compressed to "owner/repo#42" (13 chars).
	// Header is "• - now - " (~10 chars). maxNoteOnFirstLine ~= availableWidth - 10.
	// With hyperLinks=true and availableWidth=30, the compressed note should fit.
	url := "https://github.com/owner/repo/pull/42"
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: url,
	}
	got := renderActivityEntry(e, time.Now(), "", true, 40, nil)
	lines := strings.Split(got, "\n")
	if len(lines) > 1 {
		t.Errorf("expected single line with hyperLinks=true at width 40, got %d lines: %q", len(lines), got)
	}
}

func TestRenderNoteWithMentionsNonGitHubHyperLink(t *testing.T) {
	url := "https://www.example.com/thing/x/y/lonoooooooog"
	got := renderNoteWithMentions("see "+url, "", true)
	if !strings.Contains(got, "www.example.com/thin...") {
		t.Errorf("expected truncated label in %q", got)
	}
	if !strings.Contains(got, url) {
		t.Errorf("expected full URL as OSC 8 target in %q", got)
	}
}

func TestFormatActivityEntryGitHubHyperLink(t *testing.T) {
	url := "https://github.com/owner/repo/issues/7"
	e := journal.Entry{
		TS:   time.Now().Format(time.RFC3339),
		Note: "see " + url,
	}
	got := formatActivityEntry(e, time.Now(), "", true)
	if !strings.Contains(got, "owner/repo#7") {
		t.Errorf("expected compressed label in %q", got)
	}
	if !strings.Contains(got, "\033]8;;") {
		t.Errorf("expected OSC 8 escape in %q", got)
	}
}
