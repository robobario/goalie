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
)

func blockedEntry(goal *string, thread, note string) map[string]any {
	e := map[string]any{
		"ts":      "2026-01-01T00:00:00+00:00",
		"goal":    goal,
		"note":    note,
		"blocked": true,
		"task":    thread,
	}
	return e
}

func recentEntry(thread string, goal *string, note string, daysAgo float64) map[string]any {
	ts := time.Now().UTC().Add(time.Duration(-daysAgo*24) * time.Hour).Format(time.RFC3339)
	return map[string]any{
		"ts":      ts,
		"goal":    goal,
		"note":    note,
		"blocked": false,
		"task":    thread,
	}
}

func weekJournalFile(username string) string {
	year, week := time.Now().UTC().ISOWeek()
	return fmt.Sprintf("%s-%d-W%02d.jsonl", username, year, week)
}

func writeJournalEntries(t *testing.T, dataDir, username string, entries []map[string]any, key []byte) {
	t.Helper()
	journalDir := filepath.Join(dataDir, "journal")
	if err := os.MkdirAll(journalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(journalDir, weekJournalFile(username))

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

func journalEntries(t *testing.T, dataDir, username string, key []byte) []map[string]any {
	t.Helper()
	path := filepath.Join(dataDir, "journal", weekJournalFile(username))
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("failed to read journal: %v", err)
	}
	data, err := crypto.Decrypt(key, raw)
	if err != nil {
		t.Fatalf("failed to decrypt journal: %v", err)
	}
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("failed to parse entry: %v", err)
		}
		entries = append(entries, e)
	}
	return entries
}

func runUpdate(t *testing.T, input string, setup func(dataDir string, key []byte)) (string, cli.AppContext) {
	t.Helper()
	ctx, stdout, _ := newInteractiveCtx(t, input)
	if setup != nil {
		setup(ctx.DataDir, ctx.EncryptionKey)
	}
	if err := cli.InteractiveUpdate(&ctx); err != nil {
		t.Fatalf("InteractiveUpdate returned error: %v", err)
	}
	return stdout.String(), ctx
}

func TestInteractiveUpdateGreeting(t *testing.T) {
	out, _ := runUpdate(t, "n\n", nil)
	if !strings.Contains(out, "testuser") {
		t.Errorf("greeting not in output:\n%s", out)
	}
}

func TestInteractiveUpdateNoBlockedThreads(t *testing.T) {
	out, _ := runUpdate(t, "n\n", nil)
	if !strings.Contains(out, "No blocked tasks.") {
		t.Errorf("expected 'No blocked threads.' in output:\n%s", out)
	}
}

func TestInteractiveUpdateBlockedCountShown(t *testing.T) {
	out, _ := runUpdate(t, "n\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			blockedEntry(nil, "#foo", "waiting on infra"),
		}, key)
	})
	if !strings.Contains(out, "1 blocked task(s).") {
		t.Errorf("expected blocked count in output:\n%s", out)
	}
}

func TestInteractiveUpdateBlockedThreadShownInOutput(t *testing.T) {
	goal := "ROUTING"
	out, _ := runUpdate(t, "n\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			blockedEntry(&goal, "#work", "needs review"),
		}, key)
	})
	if !strings.Contains(out, "ROUTING") {
		t.Errorf("expected 'ROUTING' in output:\n%s", out)
	}
	if !strings.Contains(out, "#work") {
		t.Errorf("expected '#work' in output:\n%s", out)
	}
	if !strings.Contains(out, "needs review") {
		t.Errorf("expected 'needs review' in output:\n%s", out)
	}
}

func TestInteractiveUpdateNoGoalBlockedThreadShown(t *testing.T) {
	out, _ := runUpdate(t, "n\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			blockedEntry(nil, "#mythread", "some blocker"),
		}, key)
	})
	if !strings.Contains(out, "#mythread - some blocker") {
		t.Errorf("expected thread shown without goal prefix in output:\n%s", out)
	}
	if strings.Contains(out, "nil") || strings.Contains(out, "<nil>") {
		t.Errorf("nil goal shown in output:\n%s", out)
	}
}

func TestInteractiveUpdateBlockedUnblockedNoNotes(t *testing.T) {
	goal := "ROUTING"
	_, ctx := runUpdate(t, "y\ny\n\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			blockedEntry(&goal, "#work", "needs review"),
		}, key)
	})
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 2 {
		t.Fatalf("expected 2 journal entries, got %d", len(entries))
	}
	e := entries[1]
	if e["blocked"] != false {
		t.Errorf("expected blocked=false, got %v", e["blocked"])
	}
	if e["note"] != "unblocked" {
		t.Errorf("expected note='unblocked', got %v", e["note"])
	}
	if e["task"] != "#work" {
		t.Errorf("expected thread='#work', got %v", e["task"])
	}
}

func TestInteractiveUpdateBlockedUnblockedWithNotes(t *testing.T) {
	goal := "ROUTING"
	_, ctx := runUpdate(t, "y\ny\ndone\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			blockedEntry(&goal, "#work", "needs review"),
		}, key)
	})
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 2 {
		t.Fatalf("expected 2 journal entries, got %d", len(entries))
	}
	e := entries[1]
	if e["blocked"] != false {
		t.Errorf("expected blocked=false, got %v", e["blocked"])
	}
	if e["note"] != "done" {
		t.Errorf("expected note='done', got %v", e["note"])
	}
}

func TestInteractiveUpdateBlockedStillBlockedWithNotes(t *testing.T) {
	goal := "ROUTING"
	_, ctx := runUpdate(t, "y\nn\nstill waiting\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			blockedEntry(&goal, "#work", "needs review"),
		}, key)
	})
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 2 {
		t.Fatalf("expected 2 journal entries, got %d", len(entries))
	}
	e := entries[1]
	if e["blocked"] != true {
		t.Errorf("expected blocked=true, got %v", e["blocked"])
	}
	if e["note"] != "still waiting" {
		t.Errorf("expected note='still waiting', got %v", e["note"])
	}
}

func TestInteractiveUpdateBlockedNoChangeSkipped(t *testing.T) {
	_, ctx := runUpdate(t, "y\nn\n\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			blockedEntry(nil, "#work", "needs review"),
		}, key)
	})
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 1 {
		t.Errorf("expected 1 journal entry (no change), got %d", len(entries))
	}
}

func TestInteractiveUpdateRecentThreadShown(t *testing.T) {
	out, _ := runUpdate(t, "n\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			recentEntry("#mythread", nil, "some work", 1),
		}, key)
	})
	if !strings.Contains(out, "Your other recently active tasks (last 7d):") {
		t.Errorf("recent threads header not in output:\n%s", out)
	}
	if !strings.Contains(out, "#mythread") {
		t.Errorf("thread not in output:\n%s", out)
	}
}

func TestInteractiveUpdateOldThreadNotShown(t *testing.T) {
	out, _ := runUpdate(t, "n\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			recentEntry("#oldthread", nil, "old work", 10),
		}, key)
	})
	if strings.Contains(out, "Your other recently active tasks (last 7d):") {
		t.Errorf("old thread should not appear in recent section:\n%s", out)
	}
}

func TestInteractiveUpdateBlockedThreadNotInRecentList(t *testing.T) {
	out, _ := runUpdate(t, "n\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			blockedEntry(nil, "#blocked", "some blocker"),
		}, key)
	})
	if strings.Contains(out, "Your other recently active tasks (last 7d):") {
		t.Errorf("blocked thread should not appear in recent section:\n%s", out)
	}
}

func TestInteractiveUpdateRecentThreadUpdateLogged(t *testing.T) {
	_, ctx := runUpdate(t, "y\n1\nn\nreview done\n\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			recentEntry("#work", nil, "some work", 1),
		}, key)
	})
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 2 {
		t.Fatalf("expected 2 journal entries, got %d", len(entries))
	}
	e := entries[1]
	if e["task"] != "#work" {
		t.Errorf("expected thread='#work', got %v", e["task"])
	}
	if e["note"] != "review done" {
		t.Errorf("expected note='review done', got %v", e["note"])
	}
	if e["blocked"] != false {
		t.Errorf("expected blocked=false, got %v", e["blocked"])
	}
	if e["goal"] != nil {
		t.Errorf("expected goal=nil, got %v", e["goal"])
	}
}

func TestInteractiveUpdateRecentThreadUpdateBlocked(t *testing.T) {
	_, ctx := runUpdate(t, "y\n1\ny\nstalled\n\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			recentEntry("#work", nil, "some work", 1),
		}, key)
	})
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 2 {
		t.Fatalf("expected 2 journal entries, got %d", len(entries))
	}
	if entries[1]["blocked"] != true {
		t.Errorf("expected blocked=true, got %v", entries[1]["blocked"])
	}
}

func TestInteractiveUpdateRecentSkipOnNo(t *testing.T) {
	_, ctx := runUpdate(t, "n\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			recentEntry("#work", nil, "some work", 1),
		}, key)
	})
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 1 {
		t.Errorf("expected 1 journal entry (no update), got %d", len(entries))
	}
}

func TestInteractiveUpdateRecentBlankToFinish(t *testing.T) {
	_, ctx := runUpdate(t, "y\n\nn\n", func(dataDir string, key []byte) {
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			recentEntry("#work", nil, "some work", 1),
		}, key)
	})
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 1 {
		t.Errorf("expected 1 journal entry (blank to finish), got %d", len(entries))
	}
}

func TestInteractiveUpdateNewThreadSkippedOnNo(t *testing.T) {
	_, ctx := runUpdate(t, "n\n", nil)
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 0 {
		t.Errorf("expected 0 journal entries, got %d", len(entries))
	}
}

func TestInteractiveUpdateNewThreadLogged(t *testing.T) {
	_, ctx := runUpdate(t, "y\n1\n#new-work\nn\nstarting impl\nn\n", func(dataDir string, key []byte) {
		addOpenGoal(t, dataDir, "ROUTING", key)
	})
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 1 {
		t.Fatalf("expected 1 journal entry, got %d", len(entries))
	}
	e := entries[0]
	if e["task"] != "#new-work" {
		t.Errorf("expected thread='#new-work', got %v", e["task"])
	}
	if e["goal"] != "ROUTING" {
		t.Errorf("expected goal='ROUTING', got %v", e["goal"])
	}
	if e["note"] != "starting impl" {
		t.Errorf("expected note='starting impl', got %v", e["note"])
	}
	if e["blocked"] != false {
		t.Errorf("expected blocked=false, got %v", e["blocked"])
	}
}

func TestInteractiveUpdateNewThreadLoggedWithBlocker(t *testing.T) {
	_, ctx := runUpdate(t, "y\n1\n#new-work\ny\nstarting impl\nn\n", func(dataDir string, key []byte) {
		addOpenGoal(t, dataDir, "ROUTING", key)
	})
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 1 {
		t.Fatalf("expected 1 journal entry, got %d", len(entries))
	}
	if entries[0]["blocked"] != true {
		t.Errorf("expected blocked=true, got %v", entries[0]["blocked"])
	}
}

func TestInteractiveUpdateNewThreadWithNoGoal(t *testing.T) {
	_, ctx := runUpdate(t, "y\n\n#new-work\nn\nstarting impl\nn\n", func(dataDir string, key []byte) {
		addOpenGoal(t, dataDir, "ROUTING", key)
	})
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 1 {
		t.Fatalf("expected 1 journal entry, got %d", len(entries))
	}
	e := entries[0]
	if e["goal"] != nil {
		t.Errorf("expected goal=nil, got %v", e["goal"])
	}
	if e["task"] != "#new-work" {
		t.Errorf("expected thread='#new-work', got %v", e["task"])
	}
}

func TestInteractiveUpdateExistingThreadsShownInNewPhase(t *testing.T) {
	out, ctx := runUpdate(t, "y\n1\n1\nn\nwhat I worked on\nn\n", func(dataDir string, key []byte) {
		addOpenGoal(t, dataDir, "ROUTING", key)
		writeJournalEntries(t, dataDir, "testuser", []map[string]any{
			recentEntry("#routing-impl", strPtr("ROUTING"), "old note", 10),
		}, key)
	})
	if !strings.Contains(out, "#routing-impl") {
		t.Errorf("expected '#routing-impl' in output:\n%s", out)
	}
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 2 {
		t.Fatalf("expected 2 journal entries, got %d", len(entries))
	}
	e := entries[1]
	if e["task"] != "#routing-impl" {
		t.Errorf("expected thread='#routing-impl', got %v", e["task"])
	}
	if e["note"] != "what I worked on" {
		t.Errorf("expected note='what I worked on', got %v", e["note"])
	}
}

func TestInteractiveUpdateInvalidHashtagLoops(t *testing.T) {
	_, ctx := runUpdate(t, "y\nbadtag\n#valid\nn\nstarting impl\nn\n", nil)
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 1 {
		t.Fatalf("expected 1 journal entry, got %d", len(entries))
	}
	if entries[0]["task"] != "#valid" {
		t.Errorf("expected thread='#valid', got %v", entries[0]["task"])
	}
}

func TestInteractiveUpdateNewThreadLoopAddsMultiple(t *testing.T) {
	_, ctx := runUpdate(t, "y\n#work1\nn\nfirst task\ny\n#work2\nn\nsecond task\nn\n", nil)
	entries := journalEntries(t, ctx.DataDir, "testuser", ctx.EncryptionKey)
	if len(entries) != 2 {
		t.Errorf("expected 2 journal entries, got %d", len(entries))
	}
}

func TestInteractiveUpdateEmptyGoalsHintShown(t *testing.T) {
	out, _ := runUpdate(t, "y\n#new-work\nn\nsome work\nn\n", nil)
	if !strings.Contains(out, "goalie goal add") {
		t.Errorf("expected 'goalie goal add' hint in output:\n%s", out)
	}
}

func strPtr(s string) *string {
	return &s
}
