package journal_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goalie/internal/crypto"
	"goalie/internal/git"
	"goalie/internal/journal"
)

func relTS(deltaDays float64) string {
	d := time.Duration(float64(24*time.Hour) * deltaDays)
	return time.Now().UTC().Add(d).Format(time.RFC3339)
}

func strPtr(s string) *string { return &s }

func currentWeekFile(username string) string {
	year, week := time.Now().UTC().ISOWeek()
	return fmt.Sprintf("%s-%d-W%02d.jsonl", username, year, week)
}

func testKey() []byte {
	return make([]byte, 32)
}

// writeEntries marshals entries to JSONL, encrypts with key, and writes to
// dataDir/journal/filename.
func writeEntries(t *testing.T, dataDir, filename string, entries []journal.Entry, key []byte) {
	t.Helper()
	journalDir := filepath.Join(dataDir, "journal")
	if err := os.MkdirAll(journalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(journalDir, filename)
	writeEncryptedEntries(t, path, entries, key)
}

// writeEncryptedEntries marshals entries to JSONL, encrypts, and writes to path.
func writeEncryptedEntries(t *testing.T, path string, entries []journal.Entry, key []byte) {
	t.Helper()
	var buf []byte
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		buf = append(buf, data...)
		buf = append(buf, '\n')
	}
	encrypted, err := crypto.Encrypt(key, buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encrypted, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentThreadStates(t *testing.T) {
	t.Run("empty when file doesn't exist", func(t *testing.T) {
		dir := t.TempDir()
		states, err := journal.CurrentTaskStates(dir, "nonexistent", testKey())
		if err != nil {
			t.Fatal(err)
		}
		if len(states) != 0 {
			t.Errorf("expected empty map, got %v", states)
		}
	})

	t.Run("empty when no threaded entries", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		path := filepath.Join(dir, currentWeekFile("alice"))
		writeEncryptedEntries(t, path, []journal.Entry{
			{TS: "2026-01-01T00:00:00Z", Note: "some work"},
		}, key)

		states, err := journal.CurrentTaskStates(dir, "alice", key)
		if err != nil {
			t.Fatal(err)
		}
		if len(states) != 0 {
			t.Errorf("expected empty map, got %v", states)
		}
	})

	t.Run("single thread captured", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		path := filepath.Join(dir, currentWeekFile("alice"))
		writeEncryptedEntries(t, path, []journal.Entry{
			{TS: "2026-01-01T00:00:00Z", Goal: strPtr("GOAL_A"), Note: "some work", Blocked: true, Task: strPtr("#foo")},
		}, key)

		states, err := journal.CurrentTaskStates(dir, "alice", key)
		if err != nil {
			t.Fatal(err)
		}
		s, ok := states["#foo"]
		if !ok {
			t.Fatal("expected #foo in states")
		}
		if s.Note != "some work" {
			t.Errorf("expected note 'some work', got %q", s.Note)
		}
		if !s.Blocked {
			t.Error("expected blocked=true")
		}
		if s.Goal == nil || *s.Goal != "GOAL_A" {
			t.Errorf("expected goal GOAL_A, got %v", s.Goal)
		}
	})

	t.Run("latest entry wins for same thread", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		path := filepath.Join(dir, currentWeekFile("alice"))
		writeEncryptedEntries(t, path, []journal.Entry{
			{TS: "2026-01-01T00:00:00Z", Goal: strPtr("GOAL_A"), Note: "first work", Blocked: true, Task: strPtr("#foo")},
			{TS: "2026-01-02T00:00:00Z", Goal: strPtr("GOAL_A"), Note: "second work", Blocked: false, Task: strPtr("#foo")},
		}, key)

		states, err := journal.CurrentTaskStates(dir, "alice", key)
		if err != nil {
			t.Fatal(err)
		}
		s := states["#foo"]
		if s.Note != "second work" {
			t.Errorf("expected 'second work', got %q", s.Note)
		}
		if s.Blocked {
			t.Error("expected blocked=false for latest entry")
		}
	})

	t.Run("multiple threads tracked independently", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		path := filepath.Join(dir, currentWeekFile("alice"))
		writeEncryptedEntries(t, path, []journal.Entry{
			{TS: "2026-01-01T00:00:00Z", Goal: strPtr("GOAL_A"), Note: "foo work", Blocked: false, Task: strPtr("#foo")},
			{TS: "2026-01-02T00:00:00Z", Goal: strPtr("GOAL_B"), Note: "bar work", Blocked: true, Task: strPtr("#bar")},
		}, key)

		states, err := journal.CurrentTaskStates(dir, "alice", key)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := states["#foo"]; !ok {
			t.Error("expected #foo in states")
		}
		if _, ok := states["#bar"]; !ok {
			t.Error("expected #bar in states")
		}
		if states["#foo"].Goal == nil || *states["#foo"].Goal != "GOAL_A" {
			t.Errorf("expected GOAL_A for #foo")
		}
		if states["#bar"].Goal == nil || *states["#bar"].Goal != "GOAL_B" {
			t.Error("expected GOAL_B for #bar")
		}
		if !states["#bar"].Blocked {
			t.Error("expected blocked=true for #bar")
		}
	})

	t.Run("nil thread entries ignored", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		path := filepath.Join(dir, currentWeekFile("alice"))
		writeEncryptedEntries(t, path, []journal.Entry{
			{TS: "2026-01-01T00:00:00Z", Note: "unthreaded"},
			{TS: "2026-01-02T00:00:00Z", Goal: strPtr("GOAL_A"), Note: "threaded", Task: strPtr("#foo")},
		}, key)

		states, err := journal.CurrentTaskStates(dir, "alice", key)
		if err != nil {
			t.Fatal(err)
		}
		if len(states) != 1 {
			t.Errorf("expected 1 entry, got %d: %v", len(states), states)
		}
		if _, ok := states["#foo"]; !ok {
			t.Error("expected only #foo")
		}
	})
}

func TestAppend(t *testing.T) {
	t.Run("appends correct fields to JSONL file", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		e := journal.Entry{Note: "Implementing TLS"}

		if err := journal.Append(dir, r, "alice-example", e, key); err != nil {
			t.Fatal(err)
		}

		year, week := time.Now().UTC().ISOWeek()
		filename := fmt.Sprintf("alice-example-%d-W%02d.jsonl", year, week)
		path := filepath.Join(dir, "journal", filename)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		decrypted, err := crypto.Decrypt(key, raw)
		if err != nil {
			t.Fatal(err)
		}
		var got journal.Entry
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(decrypted))), &got); err != nil {
			t.Fatal(err)
		}
		if got.Note != "Implementing TLS" {
			t.Errorf("expected note 'Implementing TLS', got %q", got.Note)
		}
		if got.Goal != nil {
			t.Errorf("expected nil goal, got %v", got.Goal)
		}
		if got.Blocked {
			t.Error("expected blocked=false")
		}
		if got.Task != nil {
			t.Errorf("expected nil thread, got %v", got.Task)
		}
		if got.TS == "" {
			t.Error("expected non-empty TS")
		}
		if _, err := time.Parse(time.RFC3339, got.TS); err != nil {
			t.Errorf("TS is not valid RFC3339: %v", err)
		}
	})

	t.Run("pull happens before file write", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		e := journal.Entry{Note: "some work"}

		if err := journal.Append(dir, r, "alice-example", e, testKey()); err != nil {
			t.Fatal(err)
		}

		var cmds []string
		for _, call := range r.Calls {
			if len(call) > 0 {
				cmds = append(cmds, call[0])
			}
		}
		pullIdx, addIdx := -1, -1
		for i, cmd := range cmds {
			switch cmd {
			case "pull":
				if pullIdx == -1 {
					pullIdx = i
				}
			case "add":
				if addIdx == -1 {
					addIdx = i
				}
			}
		}
		if pullIdx == -1 {
			t.Fatal("pull not called")
		}
		if addIdx == -1 {
			t.Fatal("add not called")
		}
		if pullIdx >= addIdx {
			t.Errorf("pull (index %d) must come before add (index %d)", pullIdx, addIdx)
		}
	})

	t.Run("commits and pushes after write", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		e := journal.Entry{Note: "some work"}

		if err := journal.Append(dir, r, "alice-example", e, testKey()); err != nil {
			t.Fatal(err)
		}

		cmds := make(map[string]bool)
		for _, call := range r.Calls {
			if len(call) > 0 {
				cmds[call[0]] = true
			}
		}
		for _, expected := range []string{"add", "commit", "push"} {
			if !cmds[expected] {
				t.Errorf("expected git %s to be called", expected)
			}
		}
	})
}

func TestCollect(t *testing.T) {
	t.Run("entries within window are returned", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-1), Note: "recent work"},
		}, key)

		entries, err := journal.Collect(dir, r, 7, "", key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Note != "recent work" {
			t.Errorf("expected 1 entry 'recent work', got %v", entries)
		}
	})

	t.Run("entries older than days are excluded", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-10), Note: "old work"},
		}, key)

		entries, err := journal.Collect(dir, r, 7, "", key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %v", entries)
		}
	})

	t.Run("entries sorted by TS ascending across users", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-3), Note: "earlier"},
			{TS: relTS(-1), Note: "later"},
		}, key)
		writeEntries(t, dir, currentWeekFile("bob"), []journal.Entry{
			{TS: relTS(-2), Note: "middle"},
		}, key)

		entries, err := journal.Collect(dir, r, 7, "", key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}
		notes := []string{entries[0].Note, entries[1].Note, entries[2].Note}
		if notes[0] != "earlier" || notes[1] != "middle" || notes[2] != "later" {
			t.Errorf("unexpected order: %v", notes)
		}
	})

	t.Run("user pattern exact match filters correctly", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-1), Note: "alice work"},
		}, key)
		writeEntries(t, dir, currentWeekFile("bob"), []journal.Entry{
			{TS: relTS(-1), Note: "bob work"},
		}, key)

		entries, err := journal.Collect(dir, r, 7, "bob", key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Note != "bob work" {
			t.Errorf("expected only bob's entry, got %v", entries)
		}
	})

	t.Run("user glob pattern matches multiple users", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice-smith"), []journal.Entry{
			{TS: relTS(-1), Note: "alice smith work"},
		}, key)
		writeEntries(t, dir, currentWeekFile("alice-jones"), []journal.Entry{
			{TS: relTS(-1), Note: "alice jones work"},
		}, key)
		writeEntries(t, dir, currentWeekFile("bob"), []journal.Entry{
			{TS: relTS(-1), Note: "bob work"},
		}, key)

		entries, err := journal.Collect(dir, r, 7, "alice*", key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		for _, e := range entries {
			if e.Note == "bob work" {
				t.Error("bob's entry should be excluded")
			}
		}
	})

	t.Run("wildcard pattern includes all users", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-1), Note: "alice work"},
		}, key)
		writeEntries(t, dir, currentWeekFile("bob"), []journal.Entry{
			{TS: relTS(-1), Note: "bob work"},
		}, key)

		entries, err := journal.Collect(dir, r, 7, "*", key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 {
			t.Errorf("expected 2 entries with '*' pattern, got %d", len(entries))
		}
	})

	t.Run("returns empty slice when no entries", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		os.MkdirAll(filepath.Join(dir, "journal"), 0o755)

		entries, err := journal.Collect(dir, r, 7, "", testKey())
		if err != nil {
			t.Fatal(err)
		}
		if entries == nil {
			t.Error("expected non-nil empty slice, got nil")
		}
		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("username populated from filename", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice-example"), []journal.Entry{
			{TS: relTS(-1), Note: "work"},
		}, key)

		entries, err := journal.Collect(dir, r, 7, "", key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Username != "alice-example" {
			t.Errorf("expected username 'alice-example', got %q", entries[0].Username)
		}
	})
}

func TestCollectLatest(t *testing.T) {
	t.Run("only latest entry per user-goal-thread key is returned", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-3), Note: "earlier note", Goal: strPtr("ROUTING"), Task: strPtr("#impl")},
			{TS: relTS(-1), Note: "later note", Goal: strPtr("ROUTING"), Task: strPtr("#impl")},
		}, key)

		entries, err := journal.CollectLatest(dir, r, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].Note != "later note" {
			t.Errorf("expected 'later note', got %q", entries[0].Note)
		}
	})

	t.Run("different users same thread both returned", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-1), Note: "alice note", Goal: strPtr("ROUTING"), Task: strPtr("#impl")},
		}, key)
		writeEntries(t, dir, currentWeekFile("bob"), []journal.Entry{
			{TS: relTS(-1), Note: "bob note", Goal: strPtr("ROUTING"), Task: strPtr("#impl")},
		}, key)

		entries, err := journal.CollectLatest(dir, r, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		notes := map[string]bool{entries[0].Note: true, entries[1].Note: true}
		if !notes["alice note"] || !notes["bob note"] {
			t.Errorf("expected both alice and bob notes, got %v", notes)
		}
	})

	t.Run("entries outside window excluded", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-6), Note: "within window", Goal: strPtr("ROUTING")},
			{TS: relTS(-8), Note: "outside window", Goal: strPtr("ROUTING")},
		}, key)

		entries, err := journal.CollectLatest(dir, r, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Note != "within window" {
			t.Errorf("expected only 'within window', got %v", entries)
		}
	})

	t.Run("nil thread entries deduplicated per user", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-3), Note: "first entry"},
			{TS: relTS(-1), Note: "second entry"},
		}, key)

		entries, err := journal.CollectLatest(dir, r, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry after dedup, got %d", len(entries))
		}
		if entries[0].Note != "second entry" {
			t.Errorf("expected 'second entry', got %q", entries[0].Note)
		}
	})
}
