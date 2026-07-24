// Package system_test contains end-to-end tests that build the goalie binary
// and exercise it against a real (bare) git repository.  Each test gets its
// own bare repo in a temp directory so tests are fully isolated.
//
// Run with:
//
//	go test ./tests/system/...
//
// Tests are skipped when -short is passed because they build a binary and
// perform real git operations.
package system_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// If GOALIE_BINARY is set, use that pre-built binary (e.g. a release build).
	// Otherwise build from source so plain `go test ./tests/system/...` works.
	if p := os.Getenv("GOALIE_BINARY"); p != "" {
		abs, err := filepath.Abs(p)
		if err != nil {
			panic(fmt.Sprintf("GOALIE_BINARY path error: %v", err))
		}
		binaryPath = abs
		os.Exit(m.Run())
	}

	tmp, err := os.MkdirTemp("", "goalie-system-*")
	if err != nil {
		panic(err)
	}
	binaryPath = filepath.Join(tmp, "goalie")

	moduleRoot, err := filepath.Abs("../..")
	if err != nil {
		panic(err)
	}

	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/goalie")
	build.Dir = moduleRoot
	if out, err := build.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("build failed: %v\n%s", err, out))
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// setupBareRepo creates a temporary bare git repository and returns its file:// URL.
func setupBareRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	return "file://" + dir
}

// gitHome creates a temp directory with a minimal .gitconfig so git commits
// inside the test don't inherit the developer's real signing settings.
func gitHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	cfg := "[user]\n\tname = Test User\n\temail = test@example.com\n[commit]\n\tgpgsign = false\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	return home
}

// runGoalie runs the goalie binary with the given GOALIE_HOME and optional stdin.
// It fatals if the command exits non-zero.
func runGoalie(t *testing.T, goalieHome, gitHomeDir, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"GOALIE_HOME="+goalieHome,
		"HOME="+gitHomeDir,
	)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goalie %v failed: %v\noutput:\n%s", args, err, out)
	}
	return string(out)
}

// runGoalieMayFail runs the binary but does not fatal on non-zero exit.
func runGoalieMayFail(t *testing.T, goalieHome, gitHomeDir, stdin string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(),
		"GOALIE_HOME="+goalieHome,
		"HOME="+gitHomeDir,
	)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// extractHexKey parses the "Encryption key: <hex>" text from init output.
// The key text may appear mid-line (after an interactive prompt that shares
// the line), so we search by substring rather than line prefix.
func extractHexKey(t *testing.T, output string) string {
	t.Helper()
	const marker = "Encryption key: "
	for _, line := range strings.Split(output, "\n") {
		if idx := strings.Index(line, marker); idx >= 0 {
			return strings.TrimSpace(line[idx+len(marker):])
		}
	}
	t.Fatalf("no encryption key found in output:\n%s", output)
	return ""
}

// ── Unencrypted scenario ─────────────────────────────────────────────────────

func TestScenario_Unencrypted_MultiUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping system test in short mode")
	}

	repo := setupBareRepo(t)
	gh := gitHome(t)

	// GIVEN: user1's GOALIE_HOME is isolated
	user1 := t.TempDir()

	// WHEN: user1 inits a fresh unencrypted repo and gives their name
	// stdin: "n" for encryption prompt, "Alice" for name prompt
	runGoalie(t, user1, gh, "n\nAlice\n", "init", repo)

	// AND: user1 creates a shared goal
	runGoalie(t, user1, gh, "", "goal", "add", "ROUTING", "Implement the routing layer")

	// AND: user1 logs progress on a task
	runGoalie(t, user1, gh, "", "log", "started the work", "--task", "#impl", "--goal", "ROUTING")
	runGoalie(t, user1, gh, "", "log", "blocked on code review", "--task", "#impl", "--goal", "ROUTING", "--blocked")

	// WHEN: user2 joins from the same repo (branch already exists — no encryption prompt)
	// stdin: "Bob" for name prompt only
	user2 := t.TempDir()
	runGoalie(t, user2, gh, "Bob\n", "init", repo)

	// THEN: user2 can see user1's entries in the standup view
	status := runGoalie(t, user2, gh, "", "status")
	if !strings.Contains(status, "@alice") {
		t.Errorf("expected @alice in status; got:\n%s", status)
	}
	if !strings.Contains(status, "ROUTING") {
		t.Errorf("expected ROUTING goal in status; got:\n%s", status)
	}

	// AND: the goal user1 created is visible to user2
	goals := runGoalie(t, user2, gh, "", "goal", "list")
	if !strings.Contains(goals, "ROUTING") {
		t.Errorf("expected ROUTING goal in list; got:\n%s", goals)
	}

	// WHEN: user2 logs their own entry
	runGoalie(t, user2, gh, "", "log", "reviewing the PR", "--task", "#review", "--goal", "ROUTING")

	// THEN: user1 can pull and see user2's entry
	status2 := runGoalie(t, user1, gh, "", "status")
	if !strings.Contains(status2, "@bob") {
		t.Errorf("expected @bob in status; got:\n%s", status2)
	}
}

func TestScenario_Unencrypted_GoalCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping system test in short mode")
	}

	repo := setupBareRepo(t)
	gh := gitHome(t)
	home := t.TempDir()

	runGoalie(t, home, gh, "n\nAlice\n", "init", repo)

	// Add and list goals
	runGoalie(t, home, gh, "", "goal", "add", "FEAT_A", "Feature A")
	runGoalie(t, home, gh, "", "goal", "add", "FEAT_B", "Feature B")
	goals := runGoalie(t, home, gh, "", "goal", "list")
	if !strings.Contains(goals, "FEAT_A") || !strings.Contains(goals, "FEAT_B") {
		t.Errorf("expected both goals in list; got:\n%s", goals)
	}

	// Close a goal
	runGoalie(t, home, gh, "", "goal", "close", "FEAT_A")
	goals = runGoalie(t, home, gh, "", "goal", "list")
	if !strings.Contains(goals, "closed") {
		t.Errorf("expected FEAT_A to be closed; got:\n%s", goals)
	}
}

// ── Encrypted scenario ───────────────────────────────────────────────────────

func TestScenario_Encrypted_MultiUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping system test in short mode")
	}

	repo := setupBareRepo(t)
	gh := gitHome(t)

	// GIVEN: user1 inits with encryption enabled
	// stdin: "y" for encryption, "Alice" for name
	user1 := t.TempDir()
	out := runGoalie(t, user1, gh, "y\nAlice\n", "init", repo)

	// AND: the hex key is printed for sharing
	hexKey := extractHexKey(t, out)
	if len(hexKey) != 64 {
		t.Fatalf("expected 64-char hex key, got %q (len %d)", hexKey, len(hexKey))
	}

	// AND: user1 adds a goal and logs an entry
	runGoalie(t, user1, gh, "", "goal", "add", "ROUTING", "Implement routing")
	runGoalie(t, user1, gh, "", "log", "working on the routing layer", "--task", "#impl", "--goal", "ROUTING")

	// WHEN: user2 joins, pasting the shared key at the prompt
	// stdin: "Bob" for name, then the hex key at the key prompt
	user2 := t.TempDir()
	runGoalie(t, user2, gh, "Bob\n"+hexKey+"\n", "init", repo)

	// THEN: user2 can read status (decryption works)
	status := runGoalie(t, user2, gh, "", "status")
	if !strings.Contains(status, "@alice") {
		t.Errorf("expected @alice in status; got:\n%s", status)
	}

	// AND: user2 can log an encrypted entry
	runGoalie(t, user2, gh, "", "log", "reviewing the implementation", "--task", "#review", "--goal", "ROUTING")

	// AND: user1 can read user2's entry
	status2 := runGoalie(t, user1, gh, "", "status")
	if !strings.Contains(status2, "@bob") {
		t.Errorf("expected @bob in status after user2 logged; got:\n%s", status2)
	}
}

func TestScenario_Encrypted_WrongKeyRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping system test in short mode")
	}

	repo := setupBareRepo(t)
	gh := gitHome(t)

	// GIVEN: user1 inits with encryption
	user1 := t.TempDir()
	runGoalie(t, user1, gh, "y\nAlice\n", "init", repo)

	// WHEN: user2 tries to init with the wrong key then skips
	user2 := t.TempDir()
	wrongKey := strings.Repeat("ab", 32)
	// stdin: name, wrong key (rejected), then Enter to skip
	out, _ := runGoalieMayFail(t, user2, gh, "Bob\n"+wrongKey+"\n\n", "init", repo)

	// THEN: output warns about the mismatch
	if !strings.Contains(out, "does not match") {
		t.Errorf("expected key mismatch warning; got:\n%s", out)
	}
}
