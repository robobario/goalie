package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goalie/internal/cli"
	"goalie/internal/crypto"
	"goalie/internal/git"
	"goalie/internal/journal"
)

func ts(deltaDays float64) string {
	d := time.Now().UTC().Add(time.Duration(deltaDays*24) * time.Hour)
	return d.Format(time.RFC3339)
}

func weeklyJournalFile(username string) string {
	year, week := time.Now().UTC().ISOWeek()
	return fmt.Sprintf("%s-%d-W%02d.jsonl", username, year, week)
}

type jsonlEntry map[string]any

func writeJSONL(t *testing.T, path string, entries []jsonlEntry, key []byte) {
	t.Helper()
	var buf []byte
	enc := json.NewEncoder(nopWriter{&buf})
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			t.Fatal(err)
		}
	}
	encrypted, err := crypto.Encrypt(key, buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encrypted, 0o644); err != nil {
		t.Fatal(err)
	}
}

type nopWriter struct{ buf *[]byte }

func (w nopWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

func testKey(t *testing.T) []byte {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func writeGoalJSON(t *testing.T, path string, data map[string]any, key []byte) {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := crypto.Encrypt(key, b)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encrypted, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newCtx(t *testing.T) (cli.AppContext, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	ctx := cli.AppContext{
		DataDir:       t.TempDir(),
		Git:           &git.FakeRunner{},
		Stdout:        stdout,
		Stderr:        stderr,
		Username:      "testuser",
		EncryptionKey: testKey(t),
	}
	return ctx, stdout, stderr
}

func isExitCode(err error, code int) bool {
	var e *cli.ExitError
	return errors.As(err, &e) && e.Code == code
}

// GoalList

func TestGoalListPrintsGoalsWithStateAndDescription(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	goalsDir := filepath.Join(ctx.DataDir, "goals")
	os.MkdirAll(goalsDir, 0o755)
	writeGoalJSON(t, filepath.Join(goalsDir, "ALPHA.json"), map[string]any{
		"id": "ALPHA", "description": "Alpha work", "state": "open",
	}, ctx.EncryptionKey)
	writeGoalJSON(t, filepath.Join(goalsDir, "BETA.json"), map[string]any{
		"id": "BETA", "description": "Beta work", "state": "closed",
	}, ctx.EncryptionKey)

	if err := cli.GoalList(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"ALPHA", "Alpha work", "open", "BETA", "Beta work", "closed"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// GoalAdd

func TestGoalAddInvalidIDLowercaseExitsNonzero(t *testing.T) {
	ctx, _, stderr := newCtx(t)
	os.MkdirAll(ctx.DataDir, 0o755)

	err := cli.GoalAdd(ctx, "my-goal", "some description")

	if !isExitCode(err, 1) {
		t.Fatalf("expected ExitError{1}, got %v", err)
	}
	if !strings.Contains(stderr.String(), "my-goal") {
		t.Errorf("stderr missing 'my-goal': %s", stderr.String())
	}
}

// Unblock

func writeRoutingGoal(t *testing.T, ctx cli.AppContext) {
	t.Helper()
	if err := cli.GoalAdd(ctx, "ROUTING", "Routing work"); err != nil {
		t.Fatalf("failed to set up ROUTING goal fixture: %v", err)
	}
}

func TestUnblockAppendsEntryReferencingTarget(t *testing.T) {
	ctx, _, _ := newCtx(t)
	writeRoutingGoal(t, ctx)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "stuck", "task": "#impl", "goal": "ROUTING", "blocked": true, "done": false},
	}, ctx.EncryptionKey)

	if err := cli.Unblock(ctx, "@alice", "ROUTING", "#impl", "looks fine now"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := journal.Collect(ctx.DataDir, ctx.Git, 7, ctx.Username, ctx.EncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry from %s, got %d", ctx.Username, len(entries))
	}
	e := entries[0]
	if e.Unblocks == nil || *e.Unblocks != "@alice" {
		t.Errorf("expected Unblocks=@alice, got %v", e.Unblocks)
	}
	if e.Note != "looks fine now" {
		t.Errorf("expected note preserved, got %q", e.Note)
	}
	if e.Goal == nil || *e.Goal != "ROUTING" || e.Task == nil || *e.Task != "#impl" {
		t.Errorf("expected goal/task copied from target, got goal=%v task=%v", e.Goal, e.Task)
	}
}

func TestUnblockDefaultsNoteWhenOmitted(t *testing.T) {
	ctx, _, _ := newCtx(t)
	writeRoutingGoal(t, ctx)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "stuck", "task": "#impl", "goal": "ROUTING", "blocked": true, "done": false},
	}, ctx.EncryptionKey)

	if err := cli.Unblock(ctx, "@alice", "ROUTING", "#impl", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := journal.Collect(ctx.DataDir, ctx.Git, 7, ctx.Username, ctx.EncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Note == "" {
		t.Fatalf("expected a default note, got %v", entries)
	}
}

func TestUnblockErrorsWhenTargetNotFound(t *testing.T) {
	ctx, _, stderr := newCtx(t)
	writeRoutingGoal(t, ctx)

	err := cli.Unblock(ctx, "@alice", "ROUTING", "#impl", "")
	if !isExitCode(err, 1) {
		t.Fatalf("expected exit code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "No entry found") {
		t.Errorf("expected 'No entry found' in stderr, got %q", stderr.String())
	}
}

func TestUnblockErrorsWhenTargetNotBlocked(t *testing.T) {
	ctx, _, stderr := newCtx(t)
	writeRoutingGoal(t, ctx)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "still going", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": false},
	}, ctx.EncryptionKey)

	err := cli.Unblock(ctx, "@alice", "ROUTING", "#impl", "")
	if !isExitCode(err, 1) {
		t.Fatalf("expected exit code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "not blocked") {
		t.Errorf("expected 'not blocked' in stderr, got %q", stderr.String())
	}
}

func TestUnblockErrorsWhenTargetAlreadyUnblocked(t *testing.T) {
	ctx, _, stderr := newCtx(t)
	writeRoutingGoal(t, ctx)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-2), "note": "stuck", "task": "#impl", "goal": "ROUTING", "blocked": true, "done": false},
	}, ctx.EncryptionKey)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@carol")), []jsonlEntry{
		{"ts": ts(-1), "note": "reviewed", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": false, "unblocks": "@alice"},
	}, ctx.EncryptionKey)

	err := cli.Unblock(ctx, "@alice", "ROUTING", "#impl", "")
	if !isExitCode(err, 1) {
		t.Fatalf("expected exit code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "not blocked") {
		t.Errorf("expected 'not blocked' in stderr, got %q", stderr.String())
	}
}

func TestUnblockRequiresTask(t *testing.T) {
	ctx, _, stderr := newCtx(t)

	err := cli.Unblock(ctx, "@alice", "ROUTING", "", "")
	if !isExitCode(err, 1) {
		t.Fatalf("expected exit code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "Task tag is required") {
		t.Errorf("expected 'Task tag is required' in stderr, got %q", stderr.String())
	}
}

func TestUnblockRequiresUsername(t *testing.T) {
	ctx, _, stderr := newCtx(t)

	err := cli.Unblock(ctx, "", "ROUTING", "#impl", "")
	if !isExitCode(err, 1) {
		t.Fatalf("expected exit code 1, got %v", err)
	}
	if !strings.Contains(stderr.String(), "Username is required") {
		t.Errorf("expected 'Username is required' in stderr, got %q", stderr.String())
	}
}

// Status

func TestStatusHidesOldDoneEntries(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-2), "note": "in progress", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": false},
		{"ts": ts(-6), "note": "all done", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": true},
	}, ctx.EncryptionKey)

	if err := cli.Status(ctx, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(stdout.String(), "all done") {
		t.Errorf("expected old done entry to be hidden from status:\n%s", stdout.String())
	}
}

func TestStatusShowsRecentDoneEntries(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(0), "note": "just finished", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": true},
	}, ctx.EncryptionKey)

	if err := cli.Status(ctx, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "just finished") {
		t.Errorf("expected recent done entry to appear in status:\n%s", stdout.String())
	}
}

func TestStatusShowsNonDoneEntries(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "still going", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": false},
	}, ctx.EncryptionKey)

	if err := cli.Status(ctx, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "still going") {
		t.Errorf("expected non-done entry in status:\n%s", stdout.String())
	}
}

func TestStatusEntriesWithinWindowAreShown(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "recent work", "goal": nil, "blocked": false, "task": nil},
	}, ctx.EncryptionKey)

	if err := cli.Status(ctx, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "recent work") {
		t.Errorf("expected 'recent work' in output:\n%s", stdout.String())
	}
}

func TestStatusBlockedEntryShowsBlockedPrefix(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "stalled", "goal": nil, "blocked": true, "task": nil},
	}, ctx.EncryptionKey)

	if err := cli.Status(ctx, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "[BLOCKED]") {
		t.Errorf("expected '[BLOCKED]' in output:\n%s", stdout.String())
	}
}

func TestStatusOrdersDoneEntriesAfterLivingEntries(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "just finished", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": true},
		{"ts": ts(0), "note": "still going", "task": "#docs", "goal": "ROUTING", "blocked": false, "done": false},
	}, ctx.EncryptionKey)

	if err := cli.Status(ctx, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	livingIdx := strings.Index(out, "still going")
	doneIdx := strings.Index(out, "just finished")
	if livingIdx == -1 || doneIdx == -1 {
		t.Fatalf("expected both entries in output:\n%s", out)
	}
	if doneIdx < livingIdx {
		t.Errorf("expected done entry after living entry, got:\n%s", out)
	}
}

func TestStatusShowsUnblockedTagAfterUnblock(t *testing.T) {
	ctx, stdout, _ := newCtx(t)
	writeRoutingGoal(t, ctx)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "stuck", "task": "#impl", "goal": "ROUTING", "blocked": true, "done": false},
	}, ctx.EncryptionKey)

	if err := cli.Unblock(ctx, "@alice", "ROUTING", "#impl", "looks fine now"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cli.Status(ctx, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "[UNBLOCKED]") {
		t.Errorf("expected '[UNBLOCKED]' in output:\n%s", out)
	}
	if strings.Contains(out, "[BLOCKED]") {
		t.Errorf("expected no '[BLOCKED]' once unblocked:\n%s", out)
	}
	if !strings.Contains(out, "└─") || !strings.Contains(out, ctx.Username) || !strings.Contains(out, "looks fine now") {
		t.Errorf("expected nested unblocking note in output:\n%s", out)
	}
}

func TestStatusStaysUnblockedAfterUnblockerUpdatesSameTask(t *testing.T) {
	// Regression: an unblocking entry is appended under the acting user's
	// own identity for the target's (goal, task). If that user later logs
	// an unrelated update to that same (goal, task), the unblocking entry
	// must not be lost from the target's [UNBLOCKED] status just because it
	// no longer survives CollectLatest's per-(username, goal, task) dedup.
	// Timestamps are explicit (rather than relying on cli.Unblock/cli.Log's
	// real wall-clock time) so the ordering that triggers the dedup can't
	// collide with itself down to the second.
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-2), "note": "stuck", "task": "#impl", "goal": "ROUTING", "blocked": true, "done": false},
	}, ctx.EncryptionKey)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile(ctx.Username)), []jsonlEntry{
		{"ts": ts(-1), "note": "looks fine now", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": false, "unblocks": "@alice"},
		{"ts": ts(0), "note": "just checking in", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": false},
	}, ctx.EncryptionKey)

	if err := cli.Status(ctx, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "[UNBLOCKED]") {
		t.Errorf("expected '[UNBLOCKED]' to survive the unblocker's own later update:\n%s", out)
	}
	if strings.Contains(out, "[BLOCKED]") {
		t.Errorf("expected no '[BLOCKED]', got:\n%s", out)
	}
	if !strings.Contains(out, "└─") || !strings.Contains(out, "looks fine now") {
		t.Errorf("expected the nested unblocking note to survive, got:\n%s", out)
	}
}

func TestStatusNoEntriesPrintsMessage(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)

	if err := cli.Status(ctx, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "No entries in the last 8 days.") {
		t.Errorf("expected no-entries message:\n%s", stdout.String())
	}
}

func TestStatusDaysFlagOverridesDefault(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)

	if err := cli.Status(ctx, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "No entries in the last 3 days.") {
		t.Errorf("expected 3-day message:\n%s", stdout.String())
	}
}

// Summary

func TestSummaryEntriesWithinWindowAreShown(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "recent work", "goal": nil, "blocked": false},
	}, ctx.EncryptionKey)

	if err := cli.Summary(ctx, 7, "*"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "recent work") {
		t.Errorf("expected 'recent work' in output:\n%s", stdout.String())
	}
}

func TestSummaryGroupsEntriesByGoalAndTask(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-3), "note": "started", "goal": "ROUTING", "task": "#impl", "blocked": false},
		{"ts": ts(-2), "note": "blocked on review", "goal": "ROUTING", "task": "#impl", "blocked": true},
		{"ts": ts(-1), "note": "unblocked", "goal": "ROUTING", "task": "#impl", "blocked": false},
	}, ctx.EncryptionKey)

	if err := cli.Summary(ctx, 7, "*"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()

	if !strings.Contains(out, "ROUTING#impl@alice") {
		t.Errorf("expected group header; got:\n%s", out)
	}
	if !strings.Contains(out, "started") {
		t.Errorf("expected first note; got:\n%s", out)
	}
	if !strings.Contains(out, "[Blocked]") {
		t.Errorf("expected [Blocked] label; got:\n%s", out)
	}
	if !strings.Contains(out, "[Unblocked]") {
		t.Errorf("expected [Unblocked] label; got:\n%s", out)
	}
}

func TestSummaryNoGoalUsesPlaceholder(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "some work", "goal": nil, "task": "#refactor", "blocked": false},
	}, ctx.EncryptionKey)

	if err := cli.Summary(ctx, 7, "*"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "(no goal)") {
		t.Errorf("expected '(no goal)' placeholder in header; got:\n%s", out)
	}
}

func TestSummaryStateChangeOnlyShowsLabel(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-4), "note": "steady progress", "goal": "GOAL", "task": "#impl", "blocked": false},
		{"ts": ts(-3), "note": "still going", "goal": "GOAL", "task": "#impl", "blocked": false},
		{"ts": ts(-2), "note": "hit a wall", "goal": "GOAL", "task": "#impl", "blocked": true},
		{"ts": ts(-1), "note": "resolved", "goal": "GOAL", "task": "#impl", "blocked": false},
	}, ctx.EncryptionKey)

	if err := cli.Summary(ctx, 7, "*"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()

	blockedCount := strings.Count(out, "[Blocked]")
	unblockedCount := strings.Count(out, "[Unblocked]")
	if blockedCount != 1 {
		t.Errorf("expected exactly 1 [Blocked] label, got %d; output:\n%s", blockedCount, out)
	}
	if unblockedCount != 1 {
		t.Errorf("expected exactly 1 [Unblocked] label, got %d; output:\n%s", unblockedCount, out)
	}
}

func TestSummaryUserArgFiltersByName(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "alice work", "goal": nil, "blocked": false},
	}, ctx.EncryptionKey)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("@bob")), []jsonlEntry{
		{"ts": ts(-1), "note": "bob work", "goal": nil, "blocked": false},
	}, ctx.EncryptionKey)

	if err := cli.Summary(ctx, 7, "bob"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "bob work") {
		t.Errorf("expected 'bob work' in output:\n%s", out)
	}
	if strings.Contains(out, "alice work") {
		t.Errorf("unexpected 'alice work' in output:\n%s", out)
	}
}

// requireDataDir

func TestRequireDataDirMissingPrintsMessage(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	ctx := cli.AppContext{
		DataDir: "/tmp/goalie-does-not-exist-" + t.Name(),
		Git:     &git.FakeRunner{},
		Stdout:  stdout,
		Stderr:  stderr,
	}

	err := cli.GoalList(ctx)
	if !isExitCode(err, 1) {
		t.Fatalf("expected ExitError{1}, got %v", err)
	}
	if !strings.Contains(stderr.String(), "goalie init") {
		t.Errorf("expected init hint in stderr:\n%s", stderr.String())
	}
}
