package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goalie/internal/cli"
	"goalie/internal/crypto"
	"goalie/internal/git"
	"goalie/internal/goals"
)

func newInteractiveCtx(t *testing.T, input string) (cli.AppContext, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	ctx := cli.AppContext{
		DataDir:       t.TempDir(),
		Git:           &git.FakeRunner{},
		Stdin:         strings.NewReader(input),
		Stdout:        stdout,
		Stderr:        stderr,
		Username:      "testuser",
		EncryptionKey: testKey(t),
	}
	return ctx, stdout, stderr
}

func currentWeekJournalFile(username string) string {
	year, week := time.Now().UTC().ISOWeek()
	return fmt.Sprintf("%s-%d-W%02d.jsonl", username, year, week)
}

func lastJournalEntry(t *testing.T, dataDir string, key []byte) map[string]any {
	t.Helper()
	path := filepath.Join(dataDir, "journal", currentWeekJournalFile("testuser"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read journal: %v", err)
	}
	data, err := crypto.Decrypt(key, raw)
	if err != nil {
		t.Fatalf("failed to decrypt journal: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("failed to parse entry: %v", err)
	}
	return entry
}

func addOpenGoal(t *testing.T, dataDir, id string, key []byte) {
	t.Helper()
	goalsDir := filepath.Join(dataDir, "goals")
	if err := os.MkdirAll(goalsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeGoalJSON(t, filepath.Join(goalsDir, goals.GoalFilename(key, id)), map[string]any{
		"id": id, "description": id + " desc", "state": "open", "created": "2026-01-01T00:00:00+00:00",
	}, key)
}

func addClosedGoal(t *testing.T, dataDir, id string, key []byte) {
	t.Helper()
	goalsDir := filepath.Join(dataDir, "goals")
	if err := os.MkdirAll(goalsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeGoalJSON(t, filepath.Join(goalsDir, goals.GoalFilename(key, id)), map[string]any{
		"id": id, "description": id + " desc", "state": "closed", "created": "2026-01-01T00:00:00+00:00",
	}, key)
}

func appendJournalEntries(t *testing.T, dataDir, username string, entries []map[string]any, key []byte) {
	t.Helper()
	journalDir := filepath.Join(dataDir, "journal")
	if err := os.MkdirAll(journalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(journalDir, currentWeekJournalFile(username))

	var existing []byte
	if raw, err := os.ReadFile(path); err == nil {
		dec, decErr := crypto.Decrypt(key, raw)
		if decErr != nil {
			t.Fatalf("failed to decrypt existing journal: %v", decErr)
		}
		existing = dec
	}

	var buf bytes.Buffer
	buf.Write(existing)
	enc := json.NewEncoder(&buf)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			t.Fatalf("failed to marshal entry: %v", err)
		}
	}
	encrypted, err := crypto.Encrypt(key, buf.Bytes())
	if err != nil {
		t.Fatalf("failed to encrypt journal: %v", err)
	}
	if err := os.WriteFile(path, encrypted, 0o644); err != nil {
		t.Fatalf("failed to write journal: %v", err)
	}
}

func TestInteractiveNoGoalsNotBlocked(t *testing.T) {
	ctx, _, _ := newInteractiveCtx(t, "n\nRefactoring auth\n")

	if err := cli.Log(ctx, "", "", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := lastJournalEntry(t, ctx.DataDir, ctx.EncryptionKey)
	if entry["note"] != "Refactoring auth" {
		t.Errorf("expected note 'Refactoring auth', got %v", entry["note"])
	}
	if entry["goal"] != nil {
		t.Errorf("expected nil goal, got %v", entry["goal"])
	}
	if entry["blocked"] != false {
		t.Errorf("expected blocked false, got %v", entry["blocked"])
	}
}

func TestInteractiveWithGoalAndBlocked(t *testing.T) {
	ctx, _, _ := newInteractiveCtx(t, "1\n\ny\nRefactoring auth\n")
	addOpenGoal(t, ctx.DataDir, "AUTH_REWORK", ctx.EncryptionKey)

	if err := cli.Log(ctx, "", "", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := lastJournalEntry(t, ctx.DataDir, ctx.EncryptionKey)
	if entry["goal"] != "AUTH_REWORK" {
		t.Errorf("expected goal 'AUTH_REWORK', got %v", entry["goal"])
	}
	if entry["blocked"] != true {
		t.Errorf("expected blocked true, got %v", entry["blocked"])
	}
	if entry["note"] != "Refactoring auth" {
		t.Errorf("expected note 'Refactoring auth', got %v", entry["note"])
	}
}

func TestInteractiveClosedGoalsNotOffered(t *testing.T) {
	ctx, _, _ := newInteractiveCtx(t, "n\nSome work\n")
	addClosedGoal(t, ctx.DataDir, "CLOSED_GOAL", ctx.EncryptionKey)

	if err := cli.Log(ctx, "", "", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := lastJournalEntry(t, ctx.DataDir, ctx.EncryptionKey)
	if entry["goal"] != nil {
		t.Errorf("expected nil goal, got %v", entry["goal"])
	}
}

func TestInteractiveGoalBlankSkip(t *testing.T) {
	ctx, _, _ := newInteractiveCtx(t, "\nn\nSome work\n")
	addOpenGoal(t, ctx.DataDir, "SOME_GOAL", ctx.EncryptionKey)

	if err := cli.Log(ctx, "", "", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := lastJournalEntry(t, ctx.DataDir, ctx.EncryptionKey)
	if entry["goal"] != nil {
		t.Errorf("expected nil goal, got %v", entry["goal"])
	}
}

func TestInteractiveExistingThreadsOffered(t *testing.T) {
	ctx, stdout, _ := newInteractiveCtx(t, "1\n1\nn\nSome work\n")
	addOpenGoal(t, ctx.DataDir, "AUTH_REWORK", ctx.EncryptionKey)
	appendJournalEntries(t, ctx.DataDir, "testuser", []map[string]any{
		{"ts": "2026-01-01T00:00:00+00:00", "goal": "AUTH_REWORK", "note": "a", "blocked": false, "task": "#impl"},
		{"ts": "2026-01-01T00:00:01+00:00", "goal": "AUTH_REWORK", "note": "b", "blocked": false, "task": "#tests"},
	}, ctx.EncryptionKey)

	if err := cli.Log(ctx, "", "", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := lastJournalEntry(t, ctx.DataDir, ctx.EncryptionKey)
	if entry["task"] != "#impl" {
		t.Errorf("expected thread '#impl', got %v", entry["task"])
	}
	if entry["blocked"] != false {
		t.Errorf("expected blocked false, got %v", entry["blocked"])
	}
	out := stdout.String()
	if !strings.Contains(out, "#impl") {
		t.Errorf("expected '#impl' in output:\n%s", out)
	}
	if !strings.Contains(out, "#tests") {
		t.Errorf("expected '#tests' in output:\n%s", out)
	}
}

func TestInteractiveNewHashtag(t *testing.T) {
	ctx, _, _ := newInteractiveCtx(t, "1\n#new-thing\nn\nSome work\n")
	addOpenGoal(t, ctx.DataDir, "AUTH_REWORK", ctx.EncryptionKey)

	if err := cli.Log(ctx, "", "", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := lastJournalEntry(t, ctx.DataDir, ctx.EncryptionKey)
	if entry["task"] != "#new-thing" {
		t.Errorf("expected thread '#new-thing', got %v", entry["task"])
	}
}

func TestInteractiveBlankThreadStoresNull(t *testing.T) {
	ctx, _, _ := newInteractiveCtx(t, "1\n\nn\nSome work\n")
	addOpenGoal(t, ctx.DataDir, "AUTH_REWORK", ctx.EncryptionKey)

	if err := cli.Log(ctx, "", "", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := lastJournalEntry(t, ctx.DataDir, ctx.EncryptionKey)
	if entry["task"] != nil {
		t.Errorf("expected nil thread, got %v", entry["task"])
	}
}

func TestInteractiveThreadsFromOtherGoalsNotOffered(t *testing.T) {
	ctx, stdout, _ := newInteractiveCtx(t, "1\n1\nn\nSome work\n")
	addOpenGoal(t, ctx.DataDir, "GOAL_A", ctx.EncryptionKey)
	addOpenGoal(t, ctx.DataDir, "GOAL_B", ctx.EncryptionKey)
	appendJournalEntries(t, ctx.DataDir, "testuser", []map[string]any{
		{"ts": "2026-01-01T00:00:00+00:00", "goal": "GOAL_A", "note": "a", "blocked": false, "task": "#impl"},
		{"ts": "2026-01-01T00:00:01+00:00", "goal": "GOAL_B", "note": "b", "blocked": false, "task": "#docs"},
	}, ctx.EncryptionKey)

	if err := cli.Log(ctx, "", "", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := lastJournalEntry(t, ctx.DataDir, ctx.EncryptionKey)
	if entry["task"] != "#impl" {
		t.Errorf("expected thread '#impl', got %v", entry["task"])
	}
	out := stdout.String()
	if strings.Contains(out, "#docs") {
		t.Errorf("unexpected '#docs' in output:\n%s", out)
	}
}
