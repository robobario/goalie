package system_test

import (
	"strings"
	"testing"
)

func TestExportImportRoundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping system test in short mode")
	}

	srcRepo := setupBareRepo(t)
	gh := gitHome(t)

	// --- Source: populate all entity types ---

	alice := t.TempDir()
	bob := t.TempDir()

	// Alice inits the source repo (unencrypted).
	runGoalie(t, alice, gh, "n\nalice\n", "init", srcRepo)

	// Bob joins the source repo.
	runGoalie(t, bob, gh, "bob\n", "init", srcRepo)

	// Two goals: one that stays open, one that gets closed.
	runGoalie(t, alice, gh, "", "goal", "add", "OPEN_GOAL", "An open goal")
	runGoalie(t, alice, gh, "", "goal", "add", "CLOSED_GOAL", "A goal to close")
	runGoalie(t, alice, gh, "", "goal", "close", "CLOSED_GOAL")

	// Alice: plain entry, blocked entry, done entry.
	runGoalie(t, alice, gh, "", "log", "plain note", "--goal", "OPEN_GOAL", "--task", "#plain")
	runGoalie(t, alice, gh, "", "log", "stuck on something", "--goal", "OPEN_GOAL", "--task", "#stuck", "--blocked")
	runGoalie(t, alice, gh, "", "log", "finished task", "--goal", "OPEN_GOAL", "--task", "#done", "--done")

	// Bob: plain entry.
	runGoalie(t, bob, gh, "", "log", "bob working", "--goal", "OPEN_GOAL", "--task", "#bobwork")

	// Set a MOTD.
	runGoalie(t, alice, gh, "", "motd", "set", "hello team")

	// --- Export from the source ---

	exported := runGoalie(t, alice, gh, "", "export")
	if exported == "" {
		t.Fatal("export produced no output")
	}

	// Spot-check the export contains expected event types.
	for _, want := range []string{"create_goal", "close_goal", "log_entry", "set_motd"} {
		if !strings.Contains(exported, want) {
			t.Errorf("export missing event type %q; output:\n%s", want, exported)
		}
	}
	if !strings.Contains(exported, "OPEN_GOAL") {
		t.Errorf("export missing OPEN_GOAL; output:\n%s", exported)
	}
	if !strings.Contains(exported, "CLOSED_GOAL") {
		t.Errorf("export missing CLOSED_GOAL; output:\n%s", exported)
	}

	// --- Destination: init fresh repo and import ---

	dstRepo := setupBareRepo(t)
	dst := t.TempDir()
	runGoalie(t, dst, gh, "n\nimporter\n", "init", dstRepo)
	runGoalie(t, dst, gh, exported, "import")

	// --- Verify goals ---

	goalList := runGoalie(t, dst, gh, "", "goal", "list")

	if !strings.Contains(goalList, "OPEN_GOAL") {
		t.Errorf("imported goal OPEN_GOAL not found; goal list:\n%s", goalList)
	}
	if !strings.Contains(goalList, "CLOSED_GOAL") {
		t.Errorf("imported goal CLOSED_GOAL not found; goal list:\n%s", goalList)
	}
	if !strings.Contains(goalList, "closed") {
		t.Errorf("CLOSED_GOAL should be in closed state; goal list:\n%s", goalList)
	}

	// --- Verify journal entries via status ---

	status := runGoalie(t, dst, gh, "", "status")

	if !strings.Contains(status, "@alice") {
		t.Errorf("imported alice entries not visible in status; output:\n%s", status)
	}
	if !strings.Contains(status, "@bob") {
		t.Errorf("imported bob entries not visible in status; output:\n%s", status)
	}

	// --- Verify MOTD ---

	motdOut := runGoalie(t, dst, gh, "", "motd")
	if !strings.Contains(motdOut, "hello team") {
		t.Errorf("imported MOTD not found; output:\n%s", motdOut)
	}
}

func TestExportImportRoundtrip_Encrypted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping system test in short mode")
	}

	srcRepo := setupBareRepo(t)
	gh := gitHome(t)

	alice := t.TempDir()

	// Alice inits with encryption.
	out := runGoalie(t, alice, gh, "y\nalice\n", "init", srcRepo)
	hexKey := extractHexKey(t, out)

	runGoalie(t, alice, gh, "", "goal", "add", "FEAT", "An encrypted goal")
	runGoalie(t, alice, gh, "", "log", "working on it", "--goal", "FEAT", "--task", "#impl")
	runGoalie(t, alice, gh, "", "motd", "set", "secret message")

	exported := runGoalie(t, alice, gh, "", "export")

	dstRepo := setupBareRepo(t)
	dst := t.TempDir()
	runGoalie(t, dst, gh, "y\nimporter\n"+hexKey+"\n", "init", dstRepo)

	// Import using the same key via key import.
	runGoalie(t, dst, gh, exported, "import")

	goalList := runGoalie(t, dst, gh, "", "goal", "list")
	if !strings.Contains(goalList, "FEAT") {
		t.Errorf("imported encrypted goal FEAT not found; goal list:\n%s", goalList)
	}

	status := runGoalie(t, dst, gh, "", "status")
	if !strings.Contains(status, "@alice") {
		t.Errorf("imported encrypted entries not visible in status; output:\n%s", status)
	}

	motdOut := runGoalie(t, dst, gh, "", "motd")
	if !strings.Contains(motdOut, "secret message") {
		t.Errorf("imported encrypted MOTD not found; output:\n%s", motdOut)
	}
}
