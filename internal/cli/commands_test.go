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

// Status

func TestStatusHidesDoneEntries(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("alice")), []jsonlEntry{
		{"ts": ts(-2), "note": "in progress", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": false},
		{"ts": ts(-1), "note": "all done", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": true},
	}, ctx.EncryptionKey)

	if err := cli.Status(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(stdout.String(), "all done") {
		t.Errorf("expected done entry to be hidden from status:\n%s", stdout.String())
	}
}

func TestStatusShowsNonDoneEntries(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "still going", "task": "#impl", "goal": "ROUTING", "blocked": false, "done": false},
	}, ctx.EncryptionKey)

	if err := cli.Status(ctx); err != nil {
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
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "recent work", "goal": nil, "blocked": false, "task": nil},
	}, ctx.EncryptionKey)

	if err := cli.Status(ctx); err != nil {
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
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "stalled", "goal": nil, "blocked": true, "task": nil},
	}, ctx.EncryptionKey)

	if err := cli.Status(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "[BLOCKED]") {
		t.Errorf("expected '[BLOCKED]' in output:\n%s", stdout.String())
	}
}

func TestStatusNoEntriesPrintsMessage(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)

	if err := cli.Status(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "No entries in the last 7 days.") {
		t.Errorf("expected no-entries message:\n%s", stdout.String())
	}
}

// Summary

func TestSummaryEntriesWithinWindowAreShown(t *testing.T) {
	ctx, stdout, _ := newCtx(t)

	journalDir := filepath.Join(ctx.DataDir, "journal")
	os.MkdirAll(journalDir, 0o755)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("alice")), []jsonlEntry{
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
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("alice")), []jsonlEntry{
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
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("alice")), []jsonlEntry{
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
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("alice")), []jsonlEntry{
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
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("alice")), []jsonlEntry{
		{"ts": ts(-1), "note": "alice work", "goal": nil, "blocked": false},
	}, ctx.EncryptionKey)
	writeJSONL(t, filepath.Join(journalDir, weeklyJournalFile("bob")), []jsonlEntry{
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
