package goals_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"goalie/internal/crypto"
	"goalie/internal/git"
	"goalie/internal/goals"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func writeGoal(t *testing.T, dir, id string, g goals.Goal, key []byte) {
	t.Helper()
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := crypto.Encrypt(key, data)
	if err != nil {
		t.Fatal(err)
	}
	goalsDir := filepath.Join(dir, "goals")
	if err := os.MkdirAll(goalsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goalsDir, goals.GoalFilename(key, id)), encrypted, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readGoal(t *testing.T, path string, key []byte) goals.Goal {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := crypto.Decrypt(key, data)
	if err != nil {
		t.Fatal(err)
	}
	var g goals.Goal
	if err := json.Unmarshal(decrypted, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestAdd(t *testing.T) {
	t.Run("creates encrypted file with correct fields", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey(t)

		if err := goals.Add(dir, r, "MY_GOAL", "My first goal", key); err != nil {
			t.Fatal(err)
		}

		g := readGoal(t, filepath.Join(dir, "goals", goals.GoalFilename(key, "MY_GOAL")), key)
		if g.ID != "MY_GOAL" {
			t.Errorf("id: got %q, want %q", g.ID, "MY_GOAL")
		}
		if g.Description != "My first goal" {
			t.Errorf("description: got %q, want %q", g.Description, "My first goal")
		}
		if g.State != "open" {
			t.Errorf("state: got %q, want %q", g.State, "open")
		}
		if g.Created == "" {
			t.Error("created must be set")
		}
	})

	t.Run("pull happens before file write", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey(t)

		if err := goals.Add(dir, r, "MY_GOAL", "My first goal", key); err != nil {
			t.Fatal(err)
		}

		pullIdx, addIdx := -1, -1
		for i, call := range r.Calls {
			if len(call) > 0 && call[0] == "pull" && pullIdx == -1 {
				pullIdx = i
			}
			if len(call) > 0 && call[0] == "add" && addIdx == -1 {
				addIdx = i
			}
		}
		if pullIdx == -1 {
			t.Fatal("pull was never called")
		}
		if addIdx == -1 {
			t.Fatal("add was never called")
		}
		if pullIdx >= addIdx {
			t.Errorf("pull (index %d) must precede add (index %d)", pullIdx, addIdx)
		}
	})

	t.Run("returns ErrGoalExists if file already present", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey(t)
		writeGoal(t, dir, "EXISTING", goals.Goal{
			ID: "EXISTING", Description: "Existing goal", State: "open", Created: "2026-01-01T00:00:00Z",
		}, key)

		err := goals.Add(dir, r, "EXISTING", "Existing goal", key)
		if err != goals.ErrGoalExists {
			t.Errorf("got %v, want ErrGoalExists", err)
		}
	})

	t.Run("filename does not contain the goal ID", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey(t)

		if err := goals.Add(dir, r, "SECRET_GOAL", "sensitive", key); err != nil {
			t.Fatal(err)
		}

		entries, err := os.ReadDir(filepath.Join(dir, "goals"))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Name() == ".gitkeep" {
				continue
			}
			if filepath.Ext(e.Name()) == ".json" && len(e.Name()) > 5 {
				name := e.Name()[:len(e.Name())-5]
				if name == "SECRET_GOAL" {
					t.Errorf("goal ID exposed in filename: %s", e.Name())
				}
			}
		}
	})

	t.Run("commit message does not contain the goal ID", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey(t)

		if err := goals.Add(dir, r, "SECRET_GOAL", "sensitive", key); err != nil {
			t.Fatal(err)
		}

		for _, call := range r.Calls {
			if len(call) > 1 && call[0] == "commit" {
				for _, arg := range call[1:] {
					if arg == "SECRET_GOAL" {
						t.Errorf("goal ID exposed in commit args: %v", call)
					}
				}
			}
		}
	})

	t.Run("ValidGoalID rejects lowercase", func(t *testing.T) {
		if goals.ValidGoalID("my-goal") {
			t.Error("expected false for my-goal")
		}
	})

	t.Run("ValidGoalID rejects leading digit", func(t *testing.T) {
		if goals.ValidGoalID("1GOAL") {
			t.Error("expected false for 1GOAL")
		}
	})

	t.Run("ValidGoalID accepts letters digits underscores", func(t *testing.T) {
		if !goals.ValidGoalID("ROUTING_RUNTIME_V2") {
			t.Error("expected true for ROUTING_RUNTIME_V2")
		}
	})
}

func TestClose(t *testing.T) {
	t.Run("updates state to closed", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey(t)
		writeGoal(t, dir, "MY_GOAL", goals.Goal{
			ID: "MY_GOAL", Description: "My goal", State: "open", Created: "2026-01-01T00:00:00Z",
		}, key)

		if err := goals.Close(dir, r, "MY_GOAL", key); err != nil {
			t.Fatal(err)
		}

		g := readGoal(t, filepath.Join(dir, "goals", goals.GoalFilename(key, "MY_GOAL")), key)
		if g.State != "closed" {
			t.Errorf("state: got %q, want %q", g.State, "closed")
		}
	})

	t.Run("pull happens before write", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey(t)
		writeGoal(t, dir, "MY_GOAL", goals.Goal{
			ID: "MY_GOAL", Description: "My goal", State: "open", Created: "2026-01-01T00:00:00Z",
		}, key)

		if err := goals.Close(dir, r, "MY_GOAL", key); err != nil {
			t.Fatal(err)
		}

		pullIdx, addIdx := -1, -1
		for i, call := range r.Calls {
			if len(call) > 0 && call[0] == "pull" && pullIdx == -1 {
				pullIdx = i
			}
			if len(call) > 0 && call[0] == "add" && addIdx == -1 {
				addIdx = i
			}
		}
		if pullIdx == -1 {
			t.Fatal("pull was never called")
		}
		if addIdx == -1 {
			t.Fatal("add was never called")
		}
		if pullIdx >= addIdx {
			t.Errorf("pull (index %d) must precede add (index %d)", pullIdx, addIdx)
		}
	})

	t.Run("returns ErrGoalNotFound for missing goal", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey(t)

		err := goals.Close(dir, r, "GHOST", key)
		if err != goals.ErrGoalNotFound {
			t.Errorf("got %v, want ErrGoalNotFound", err)
		}
	})

	t.Run("returns ErrGoalClosed for already-closed goal", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}
		key := testKey(t)
		writeGoal(t, dir, "DONE", goals.Goal{
			ID: "DONE", Description: "Done goal", State: "closed", Created: "2026-01-01T00:00:00Z",
		}, key)

		err := goals.Close(dir, r, "DONE", key)
		if err != goals.ErrGoalClosed {
			t.Errorf("got %v, want ErrGoalClosed", err)
		}
	})
}

func TestGoalFilename(t *testing.T) {
	t.Run("nil key uses plain ID", func(t *testing.T) {
		got := goals.GoalFilename(nil, "MY_GOAL")
		if got != "MY_GOAL.json" {
			t.Errorf("got %q, want MY_GOAL.json", got)
		}
	})

	t.Run("with key produces HMAC-derived name", func(t *testing.T) {
		key := testKey(t)
		got := goals.GoalFilename(key, "MY_GOAL")
		if got == "MY_GOAL.json" {
			t.Error("keyed filename must not expose the goal ID")
		}
		if filepath.Ext(got) != ".json" {
			t.Errorf("expected .json extension, got %q", got)
		}
	})

	t.Run("with key is deterministic", func(t *testing.T) {
		key := testKey(t)
		a := goals.GoalFilename(key, "MY_GOAL")
		b := goals.GoalFilename(key, "MY_GOAL")
		if a != b {
			t.Errorf("non-deterministic: %q vs %q", a, b)
		}
	})
}

func TestAddNilKey(t *testing.T) {
	t.Run("creates plaintext file with plain ID filename", func(t *testing.T) {
		dir := t.TempDir()
		r := &git.FakeRunner{}

		if err := goals.Add(dir, r, "PLAINGOAL", "plain goal", nil); err != nil {
			t.Fatal(err)
		}

		path := filepath.Join(dir, "goals", "PLAINGOAL.json")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file at %s: %v", path, err)
		}

		g := readGoal(t, path, nil)
		if g.ID != "PLAINGOAL" {
			t.Errorf("id: got %q, want PLAINGOAL", g.ID)
		}
	})
}

func TestList(t *testing.T) {
	t.Run("returns goals sorted by ID", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey(t)
		writeGoal(t, dir, "BETA", goals.Goal{ID: "BETA", Description: "Beta work", State: "closed"}, key)
		writeGoal(t, dir, "ALPHA", goals.Goal{ID: "ALPHA", Description: "Alpha work", State: "open"}, key)

		gs, err := goals.List(dir, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(gs) != 2 {
			t.Fatalf("got %d goals, want 2", len(gs))
		}
		if gs[0].ID != "ALPHA" {
			t.Errorf("first goal: got %q, want ALPHA", gs[0].ID)
		}
		if gs[1].ID != "BETA" {
			t.Errorf("second goal: got %q, want BETA", gs[1].ID)
		}
		if gs[0].State != "open" {
			t.Errorf("ALPHA state: got %q, want open", gs[0].State)
		}
		if gs[1].State != "closed" {
			t.Errorf("BETA state: got %q, want closed", gs[1].State)
		}
	})

	t.Run("returns empty slice when goals dir does not exist", func(t *testing.T) {
		dir := t.TempDir()
		key := testKey(t)

		gs, err := goals.List(dir, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(gs) != 0 {
			t.Errorf("got %d goals, want 0", len(gs))
		}
	})
}
