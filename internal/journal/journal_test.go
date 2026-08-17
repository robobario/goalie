package journal_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goalie/internal/clock"
	"goalie/internal/crypto"
	"goalie/internal/git"
	"goalie/internal/journal"
)

func relTS(deltaDays float64) string {
	return relTSFrom(time.Now().UTC(), deltaDays)
}

// relTSFrom is like relTS but anchored to an explicit base time, so that
// callers needing multiple relative timestamps to land in the same ISO week
// file as a fixed base don't race time.Now() across a week boundary.
func relTSFrom(base time.Time, deltaDays float64) string {
	d := time.Duration(float64(24*time.Hour) * deltaDays)
	return base.Add(d).Format(time.RFC3339)
}

func strPtr(s string) *string { return &s }

func currentWeekFile(username string) string {
	return weekFileFrom(time.Now().UTC(), username)
}

// weekFileFrom is like currentWeekFile but anchored to an explicit base time.
func weekFileFrom(base time.Time, username string) string {
	year, week := base.ISOWeek()
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

	t.Run("done flag captured in state", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		path := filepath.Join(dir, currentWeekFile("alice"))
		writeEncryptedEntries(t, path, []journal.Entry{
			{TS: "2026-01-01T00:00:00Z", Note: "some work", Task: strPtr("#foo"), Blocked: false},
			{TS: "2026-01-02T00:00:00Z", Note: "all done", Task: strPtr("#foo"), Done: true},
		}, key)

		states, err := journal.CurrentTaskStates(dir, "alice", key)
		if err != nil {
			t.Fatal(err)
		}
		if !states["#foo"].Done {
			t.Error("expected Done=true for #foo after closing entry")
		}
	})

	t.Run("regular entry after done re-opens task", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		path := filepath.Join(dir, currentWeekFile("alice"))
		writeEncryptedEntries(t, path, []journal.Entry{
			{TS: "2026-01-01T00:00:00Z", Note: "done", Task: strPtr("#foo"), Done: true},
			{TS: "2026-01-02T00:00:00Z", Note: "reopened", Task: strPtr("#foo"), Done: false},
		}, key)

		states, err := journal.CurrentTaskStates(dir, "alice", key)
		if err != nil {
			t.Fatal(err)
		}
		if states["#foo"].Done {
			t.Error("expected Done=false after regular entry re-opens the task")
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
		if len(got.ID) != 36 {
			t.Errorf("expected UUID-format ID (36 chars), got %q", got.ID)
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

	t.Run("done entries included (not filtered)", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-1), Note: "all done", Task: strPtr("#impl"), Done: true},
		}, key)

		entries, err := journal.CollectLatest(dir, r, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if !entries[0].Done {
			t.Error("expected Done=true")
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

func TestCollectLatestLocal(t *testing.T) {
	t.Run("only latest entry per user-goal-thread key is returned", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-3), Note: "earlier note", Goal: strPtr("ROUTING"), Task: strPtr("#impl")},
			{TS: relTS(-1), Note: "later note", Goal: strPtr("ROUTING"), Task: strPtr("#impl")},
		}, key)

		entries, err := journal.CollectLatestLocal(dir, 7, key)
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
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-1), Note: "alice note", Goal: strPtr("ROUTING"), Task: strPtr("#impl")},
		}, key)
		writeEntries(t, dir, currentWeekFile("bob"), []journal.Entry{
			{TS: relTS(-1), Note: "bob note", Goal: strPtr("ROUTING"), Task: strPtr("#impl")},
		}, key)

		entries, err := journal.CollectLatestLocal(dir, 7, key)
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
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-6), Note: "within window", Goal: strPtr("ROUTING")},
			{TS: relTS(-8), Note: "outside window", Goal: strPtr("ROUTING")},
		}, key)

		entries, err := journal.CollectLatestLocal(dir, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Note != "within window" {
			t.Errorf("expected only 'within window', got %v", entries)
		}
	})

	t.Run("reads from local files without git runner", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{TS: relTS(-1), Note: "local note", Task: strPtr("#impl")},
		}, key)

		entries, err := journal.CollectLatestLocal(dir, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Note != "local note" {
			t.Errorf("expected 'local note', got %v", entries)
		}
	})
}

func TestUpdateEntry(t *testing.T) {
	// fixedNow anchors these fixtures at a literal instant, far from any ISO
	// week or midnight boundary, so the entries' week file and Collect's
	// 7-day window are deterministic regardless of when the suite runs.
	fixedNow := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clock.FakeClock{T: fixedNow}

	t.Run("replaces note in-place", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		original := journal.Entry{ID: "id-1", TS: relTSFrom(fixedNow, -1.0/24), Note: "tpyo", Task: strPtr("#impl")}
		writeEntries(t, dir, weekFileFrom(fixedNow, "alice"), []journal.Entry{original}, key)

		updated := original
		updated.Note = "no typo"
		if err := journal.UpdateEntry(dir, r, "alice", original, updated, key); err != nil {
			t.Fatal(err)
		}

		entries, err := journal.CollectWithClock(dir, r, 7, "alice", key, fakeClock)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].Note != "no typo" {
			t.Errorf("expected 'no typo', got %q", entries[0].Note)
		}
	})

	t.Run("preserves surrounding entries", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		ts1, ts2, ts3 := relTSFrom(fixedNow, -3.0/24), relTSFrom(fixedNow, -2.0/24), relTSFrom(fixedNow, -1.0/24)
		writeEntries(t, dir, weekFileFrom(fixedNow, "alice"), []journal.Entry{
			{ID: "id-a", TS: ts1, Note: "entry one", Task: strPtr("#impl")},
			{ID: "id-b", TS: ts2, Note: "entry two", Task: strPtr("#impl")},
			{ID: "id-c", TS: ts3, Note: "entry three", Task: strPtr("#impl")},
		}, key)

		original := journal.Entry{ID: "id-b", TS: ts2, Note: "entry two", Task: strPtr("#impl")}
		updated := original
		updated.Note = "entry two corrected"
		if err := journal.UpdateEntry(dir, r, "alice", original, updated, key); err != nil {
			t.Fatal(err)
		}

		entries, err := journal.CollectWithClock(dir, r, 7, "alice", key, fakeClock)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(entries))
		}
		notes := map[string]bool{}
		for _, e := range entries {
			notes[e.Note] = true
		}
		if !notes["entry one"] || !notes["entry two corrected"] || !notes["entry three"] {
			t.Errorf("unexpected notes: %v", notes)
		}
		if notes["entry two"] {
			t.Error("old note should be gone")
		}
	})

	t.Run("returns error when entry not found", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		writeEntries(t, dir, currentWeekFile("alice"), []journal.Entry{
			{ID: "id-real", TS: relTS(-1.0 / 24), Note: "something", Task: strPtr("#impl")},
		}, key)

		bogus := journal.Entry{ID: "id-ghost", TS: relTS(-1.0 / 24), Note: "ghost"}
		err := journal.UpdateEntry(dir, r, "alice", bogus, bogus, key)
		if err == nil {
			t.Error("expected error for missing ID, got nil")
		}
	})

	t.Run("returns error when original has no ID", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey()
		noID := journal.Entry{TS: relTS(-1.0 / 24), Note: "old entry"}
		err := journal.UpdateEntry(dir, r, "alice", noID, noID, key)
		if err == nil {
			t.Error("expected error for entry with empty ID, got nil")
		}
	})
}

func TestKnownUsernames(t *testing.T) {
	t.Run("returns sorted usernames from journal filenames", func(t *testing.T) {
		dir := t.TempDir()
		journalDir := filepath.Join(dir, "journal")
		if err := os.MkdirAll(journalDir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{
			"zeus-2025-W01.jsonl",
			"alice-2025-W01.jsonl",
			"alice-2025-W02.jsonl",
			"bob-2024-W52.jsonl",
			"not-a-weekly-file.jsonl",
		} {
			f, err := os.Create(filepath.Join(journalDir, name))
			if err != nil {
				t.Fatal(err)
			}
			f.Close()
		}

		got, err := journal.KnownUsernames(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"alice", "bob", "zeus"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("returns empty slice when journal dir is empty", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "journal"), 0o755); err != nil {
			t.Fatal(err)
		}

		got, err := journal.KnownUsernames(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %v", got)
		}
	})
}

func TestPriorBusinessDayStart(t *testing.T) {
	midnight := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "Tuesday returns Monday",
			now:  time.Date(2026, 8, 4, 15, 0, 0, 0, time.UTC), // Tuesday
			want: midnight(2026, 8, 3),                          // Monday
		},
		{
			name: "Monday returns Friday",
			now:  time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC), // Monday
			want: midnight(2026, 7, 31),                        // Friday
		},
		{
			name: "Wednesday returns Tuesday",
			now:  time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), // Wednesday
			want: midnight(2026, 8, 4),                         // Tuesday
		},
		{
			name: "Saturday returns Friday",
			now:  time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC), // Saturday
			want: midnight(2026, 7, 31),                         // Friday
		},
		{
			name: "Sunday returns Friday",
			now:  time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC), // Sunday
			want: midnight(2026, 7, 31),                         // Friday
		},
		{
			// Regression for issue #151: at Fri 09:11 NZST, the prior business
			// day is local Thursday, not the UTC-calendar Wednesday.
			name: "NZST evening crossing UTC date uses local weekday",
			now:  time.Date(2026, 8, 14, 9, 11, 0, 0, time.FixedZone("NZST", 12*60*60)), // Friday NZST
			want: time.Date(2026, 8, 13, 0, 0, 0, 0, time.FixedZone("NZST", 12*60*60)),  // Thursday NZST midnight
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := journal.PriorBusinessDayStart(tc.now)
			if !got.Equal(tc.want) {
				t.Errorf("PriorBusinessDayStart(%v) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

// TestPriorBusinessDayStart_BoundaryIsInclusive documents the semantics that
// internal/cli/commands.go and internal/tui/activity.go rely on: a [done]
// entry is hidden only when ts.Before(cutoff), so an entry timestamped
// exactly at the cutoff is not hidden.
func TestPriorBusinessDayStart_BoundaryIsInclusive(t *testing.T) {
	now := time.Date(2026, 8, 14, 9, 11, 0, 0, time.FixedZone("NZST", 12*60*60))
	cutoff := journal.PriorBusinessDayStart(now)

	ts := cutoff
	if ts.Before(cutoff) {
		t.Errorf("entry exactly at cutoff (%v) should not be considered before it", ts)
	}
}

func TestSortForDisplay(t *testing.T) {
	t.Run("blocked before living before done, most recent first within tier", func(t *testing.T) {
		entries := []journal.Entry{
			{ID: "done-old", TS: "2026-01-01T00:00:00Z", Done: true},
			{ID: "living-old", TS: "2026-01-02T00:00:00Z"},
			{ID: "blocked-old", TS: "2026-01-03T00:00:00Z", Blocked: true},
			{ID: "done-new", TS: "2026-01-04T00:00:00Z", Done: true},
			{ID: "living-new", TS: "2026-01-05T00:00:00Z"},
			{ID: "blocked-new", TS: "2026-01-06T00:00:00Z", Blocked: true},
		}
		journal.SortForDisplay(entries)

		var ids []string
		for _, e := range entries {
			ids = append(ids, e.ID)
		}
		want := []string{"blocked-new", "blocked-old", "living-new", "living-old", "done-new", "done-old"}
		if len(ids) != len(want) {
			t.Fatalf("got %v, want %v", ids, want)
		}
		for i := range want {
			if ids[i] != want[i] {
				t.Errorf("got %v, want %v", ids, want)
				break
			}
		}
	})

	t.Run("blocked and done entry sorts as blocked", func(t *testing.T) {
		entries := []journal.Entry{
			{ID: "done", TS: "2026-01-01T00:00:00Z", Done: true},
			{ID: "blocked-and-done", TS: "2026-01-01T00:00:00Z", Blocked: true, Done: true},
		}
		journal.SortForDisplay(entries)

		if entries[0].ID != "blocked-and-done" {
			t.Errorf("expected blocked-and-done first, got %v", entries)
		}
	})
}

func TestCollectLatestAndUnblockedLocal(t *testing.T) {
	t.Run("entry with Unblocks records itself as the target's (username, goal, task) unblocking entry", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		username := "@alice"
		writeEntries(t, dir, currentWeekFile("@bob"), []journal.Entry{
			{ID: "unblock-1", TS: relTS(0), Username: "@bob", Goal: strPtr("ROUTING"), Task: strPtr("#impl"), Unblocks: &username},
		}, key)

		_, targets, err := journal.CollectLatestAndUnblockedLocal(dir, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		want := journal.UnblockTarget{Username: "@alice", Goal: "ROUTING", Task: "#impl"}
		if targets[want].ID != "unblock-1" {
			t.Errorf("expected %v to map to unblock-1, got %v", want, targets[want])
		}
	})

	t.Run("entries without Unblocks are ignored", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		writeEntries(t, dir, currentWeekFile("@bob"), []journal.Entry{
			{ID: "normal", TS: relTS(0), Username: "@bob", Goal: strPtr("ROUTING"), Task: strPtr("#impl")},
		}, key)

		_, targets, err := journal.CollectLatestAndUnblockedLocal(dir, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(targets) != 0 {
			t.Errorf("expected no targets, got %v", targets)
		}
	})

	t.Run("nil goal and task normalize to empty string", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		username := "@alice"
		writeEntries(t, dir, currentWeekFile("@bob"), []journal.Entry{
			{ID: "unblock-1", TS: relTS(0), Username: "@bob", Unblocks: &username},
		}, key)

		_, targets, err := journal.CollectLatestAndUnblockedLocal(dir, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		want := journal.UnblockTarget{Username: "@alice", Goal: "", Task: ""}
		if targets[want].ID != "unblock-1" {
			t.Errorf("expected %v to map to unblock-1, got %v", want, targets[want])
		}
	})

	t.Run("keeps the most recent entry when multiple entries unblock the same target", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey()
		username := "@alice"
		writeEntries(t, dir, currentWeekFile("@bob"), []journal.Entry{
			{ID: "unblock-1", TS: relTS(-1), Username: "@bob", Task: strPtr("#impl"), Unblocks: &username},
		}, key)
		writeEntries(t, dir, currentWeekFile("@carol"), []journal.Entry{
			{ID: "unblock-2", TS: relTS(0), Username: "@carol", Task: strPtr("#impl"), Unblocks: &username},
		}, key)

		_, targets, err := journal.CollectLatestAndUnblockedLocal(dir, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		want := journal.UnblockTarget{Username: "@alice", Goal: "", Task: "#impl"}
		if targets[want].ID != "unblock-2" {
			t.Errorf("expected the most recent unblock entry, got %v", targets[want])
		}
	})

	t.Run("acting user's own later unrelated update does not drop the unblocking entry", func(t *testing.T) {
		// Regression for issue #153: an unblocking entry is appended under
		// the acting user's own identity for the target's (goal, task), so
		// their own later update to that same (goal, task) must not make
		// the Unblocks signal vanish just because it no longer survives the
		// per-(username, goal, task) dedup.
		dir := t.TempDir()
		key := testKey()
		username := "@alice"
		writeEntries(t, dir, currentWeekFile("@bob"), []journal.Entry{
			{ID: "unblock-1", TS: relTS(-1), Username: "@bob", Goal: strPtr("ROUTING"), Task: strPtr("#impl"), Unblocks: &username},
			{ID: "self-update", TS: relTS(0), Username: "@bob", Goal: strPtr("ROUTING"), Task: strPtr("#impl")},
		}, key)

		entries, targets, err := journal.CollectLatestAndUnblockedLocal(dir, 7, key)
		if err != nil {
			t.Fatal(err)
		}
		want := journal.UnblockTarget{Username: "@alice", Goal: "ROUTING", Task: "#impl"}
		if targets[want].ID != "unblock-1" {
			t.Errorf("expected %v to still map to unblock-1, got %v", want, targets[want])
		}
		if len(entries) != 1 || entries[0].ID != "self-update" {
			t.Errorf("expected @bob's deduped latest entry to be self-update, got %v", entries)
		}
	})
}

func TestUnblockingEntry(t *testing.T) {
	t.Run("blocked entry with a later unblock entry returns it", func(t *testing.T) {
		e := journal.Entry{Username: "@alice", TS: "2024-01-01T00:00:00Z", Task: strPtr("#impl"), Blocked: true}
		unblock := journal.Entry{ID: "unblock-1", TS: "2024-01-02T00:00:00Z", Username: "@bob", Note: "reviewed"}
		targets := map[journal.UnblockTarget]journal.Entry{
			{Username: "@alice", Task: "#impl"}: unblock,
		}
		got, ok := journal.UnblockingEntry(e, targets)
		if !ok {
			t.Fatal("expected entry to be unblocked")
		}
		if got.ID != "unblock-1" {
			t.Errorf("expected unblock-1, got %v", got)
		}
	})

	t.Run("a new block after the most recent unblock returns not-ok", func(t *testing.T) {
		// Regression for issue #153: Alice's task was unblocked, then blocked
		// again later for an unrelated reason. The stale unblock entry must
		// not hide the genuinely new block.
		e := journal.Entry{Username: "@alice", TS: "2024-01-03T00:00:00Z", Task: strPtr("#impl"), Blocked: true}
		targets := map[journal.UnblockTarget]journal.Entry{
			{Username: "@alice", Task: "#impl"}: {ID: "unblock-1", TS: "2024-01-02T00:00:00Z", Username: "@bob"},
		}
		if _, ok := journal.UnblockingEntry(e, targets); ok {
			t.Error("expected a block newer than the unblock to remain blocked")
		}
	})

	t.Run("no matching target returns not-ok", func(t *testing.T) {
		e := journal.Entry{Username: "@alice", TS: "2024-01-01T00:00:00Z", Task: strPtr("#impl"), Blocked: true}
		if _, ok := journal.UnblockingEntry(e, map[journal.UnblockTarget]journal.Entry{}); ok {
			t.Error("expected no target match to return not-ok")
		}
	})
}

func TestIsUnblocked(t *testing.T) {
	t.Run("blocked entry with a later unblock entry is unblocked", func(t *testing.T) {
		e := journal.Entry{Username: "@alice", TS: "2024-01-01T00:00:00Z", Task: strPtr("#impl"), Blocked: true}
		targets := map[journal.UnblockTarget]journal.Entry{
			{Username: "@alice", Task: "#impl"}: {TS: "2024-01-02T00:00:00Z"},
		}
		if !journal.IsUnblocked(e, targets) {
			t.Error("expected entry to be unblocked")
		}
	})

	t.Run("a new block after the most recent unblock is not unblocked", func(t *testing.T) {
		// Regression for issue #153: Alice's task was unblocked, then blocked
		// again later for an unrelated reason. The stale unblock entry must
		// not hide the genuinely new block.
		e := journal.Entry{Username: "@alice", TS: "2024-01-03T00:00:00Z", Task: strPtr("#impl"), Blocked: true}
		targets := map[journal.UnblockTarget]journal.Entry{
			{Username: "@alice", Task: "#impl"}: {TS: "2024-01-02T00:00:00Z"},
		}
		if journal.IsUnblocked(e, targets) {
			t.Error("expected a block newer than the unblock to remain blocked")
		}
	})

	t.Run("no matching target is not unblocked", func(t *testing.T) {
		e := journal.Entry{Username: "@alice", TS: "2024-01-01T00:00:00Z", Task: strPtr("#impl"), Blocked: true}
		if journal.IsUnblocked(e, map[journal.UnblockTarget]journal.Entry{}) {
			t.Error("expected no target match to mean not unblocked")
		}
	})
}

func TestTargetOf(t *testing.T) {
	e := journal.Entry{Username: "@alice", Goal: strPtr("ROUTING"), Task: strPtr("#impl")}
	got := journal.TargetOf(e)
	want := journal.UnblockTarget{Username: "@alice", Goal: "ROUTING", Task: "#impl"}
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
