package cli

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goalie/internal/config"
	"goalie/internal/crypto"
	"goalie/internal/git"
	"goalie/internal/meta"
)

func TestInit_NoKeyPromptsForKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOALIE_HOME", home)

	dataDir := t.TempDir()
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{}
	var out strings.Builder

	// press Enter to skip the key prompt
	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader("\n"), &out, false); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "No key imported") {
		t.Errorf("expected skip message; got %q", out.String())
	}
}

func TestInit_KeyExistsNoGuidance(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOALIE_HOME", home)

	// 64 hex chars = 32 bytes
	keyHex := strings.Repeat("a1", 32)
	if err := os.WriteFile(filepath.Join(home, "encryption.key"), []byte(keyHex), 0600); err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{}
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader(""), &out, false); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out.String(), "No encryption key found") {
		t.Errorf("unexpected key guidance in output; got %q", out.String())
	}
}

func hasCall(calls [][]string, args ...string) bool {
	for _, call := range calls {
		if len(call) < len(args) {
			continue
		}
		match := true
		for i, a := range args {
			if call[i] != a {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func prewriteConfig(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	// Ensure the stored name is in @username format if not already.
	if len(name) > 0 && name[0] != '@' {
		name = "@" + name
	}
	if err := config.SaveTo(path, &config.Config{Name: name}); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInit_DataBranchExists(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "existing")
	runner := &git.FakeRunner{
		Outputs: map[string][]string{
			"ls-remote": {"abc123\trefs/heads/data\n"},
		},
	}

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader(""), os.Stdout, false); err != nil {
		t.Fatal(err)
	}

	if !hasCall(runner.Calls, "clone", "--branch", "data") {
		t.Errorf("expected clone --branch data call; got %v", runner.Calls)
	}
}

func TestInit_DataBranchDoesNotExist(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "existing")
	runner := &git.FakeRunner{
		Outputs: map[string][]string{
			"ls-remote": {""},
		},
	}

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader("n\n"), os.Stdout, false); err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		desc string
		args []string
	}{
		{"git init", []string{"init", dataDir}},
		{"set data branch", []string{"symbolic-ref", "HEAD", "refs/heads/data"}},
		{"remote add", []string{"remote", "add", "origin", "https://example.com/repo.git"}},
		{"add gitkeep files and meta", []string{"add", "goals/.gitkeep", "journal/.gitkeep", "meta.json"}},
		{"commit", []string{"commit", "-m", "chore: initialise goalie data branch"}},
		{"push with upstream", []string{"push", "--set-upstream", "origin", "data"}},
	}
	for _, c := range checks {
		if !hasCall(runner.Calls, c.args...) {
			t.Errorf("expected %s call; got %v", c.desc, runner.Calls)
		}
	}
	for _, forbidden := range [][]string{{"clone"}, {"checkout", "--orphan"}, {"rm", "-rf"}} {
		if hasCall(runner.Calls, forbidden...) {
			t.Errorf("unexpected call %v; got %v", forbidden, runner.Calls)
		}
	}
}

func TestInit_CustomBranchUsedInGitCalls(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "existing")
	runner := &git.FakeRunner{
		Outputs: map[string][]string{
			"ls-remote": {""},
		},
	}

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data-test", runner, strings.NewReader("n\n"), os.Stdout, false); err != nil {
		t.Fatal(err)
	}

	if !hasCall(runner.Calls, "symbolic-ref", "HEAD", "refs/heads/data-test") {
		t.Errorf("expected symbolic-ref to use custom branch; got %v", runner.Calls)
	}
	if !hasCall(runner.Calls, "push", "--set-upstream", "origin", "data-test") {
		t.Errorf("expected push to use custom branch; got %v", runner.Calls)
	}
}

func TestInit_CustomBranchClone(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "existing")
	runner := &git.FakeRunner{
		Outputs: map[string][]string{
			"ls-remote": {"abc123\trefs/heads/data-test\n"},
		},
	}

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data-test", runner, strings.NewReader(""), os.Stdout, false); err != nil {
		t.Fatal(err)
	}

	if !hasCall(runner.Calls, "clone", "--branch", "data-test") {
		t.Errorf("expected clone with custom branch; got %v", runner.Calls)
	}
}

func TestInit_DataDirAlreadyExists(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := t.TempDir()
	configPath := prewriteConfig(t, "existing")
	runner := &git.FakeRunner{}
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader(""), &out, false); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "already exists") {
		t.Errorf("expected 'already exists' in output; got %q", out.String())
	}
	if hasCall(runner.Calls, "clone") {
		t.Errorf("expected no clone call; got %v", runner.Calls)
	}
}

func TestInit_ConfigWritten(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")
	runner := &git.FakeRunner{}

	// User types just the body after '@' — the prompt prepends '@' automatically.
	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader("alice\n"), os.Stdout, false); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	if cfg.Name != "@alice" {
		t.Errorf("expected Name=@alice; got %q", cfg.Name)
	}
}

func TestInit_NewBranch_MetaEncryptTrue(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{Outputs: map[string][]string{"ls-remote": {""}}}
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader("y\n"), &out, false); err != nil {
		t.Fatal(err)
	}

	m, err := meta.Load(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Encrypt {
		t.Error("expected Encrypt=true after answering y")
	}
}

func TestInit_NewBranch_MetaEncryptFalse(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{Outputs: map[string][]string{"ls-remote": {""}}}
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader("n\n"), &out, false); err != nil {
		t.Fatal(err)
	}

	m, err := meta.Load(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Encrypt {
		t.Error("expected Encrypt=false after answering n")
	}
	if !strings.Contains(out.String(), "plaintext") {
		t.Errorf("expected plaintext message in output; got %q", out.String())
	}
}

func TestInit_ExistingBranch_NoEncryptionPrompt(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{Outputs: map[string][]string{"ls-remote": {"abc123\trefs/heads/data\n"}}}

	// stdin is empty — if a prompt were shown, the call would fail with EOF
	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader(""), os.Stdout, false); err != nil {
		t.Fatal(err)
	}
}

func TestInit_NewBranch_Encrypt_KeyCheckCommitted(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{Outputs: map[string][]string{"ls-remote": {""}}}

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader("y\n"), os.Stdout, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "key-check.enc")); err != nil {
		t.Errorf("expected key-check.enc to exist in data dir: %v", err)
	}
	if !hasCall(runner.Calls, "add", "goals/.gitkeep", "journal/.gitkeep", "meta.json", "key-check.enc") {
		t.Errorf("expected git add to include key-check.enc; got %v", runner.Calls)
	}
}

func TestInit_NewBranch_Encrypt_PrintsKeyHex(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{Outputs: map[string][]string{"ls-remote": {""}}}
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader("y\n"), &out, false); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "Encryption key:") {
		t.Errorf("expected key hex in output; got %q", out.String())
	}
	if !strings.Contains(out.String(), "goalie key import") {
		t.Errorf("expected import instruction in output; got %q", out.String())
	}
}

func TestInit_NewBranch_Encrypt_ExistingKey_Reuse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOALIE_HOME", home)
	keyHex := strings.Repeat("ab", 32)
	if err := os.WriteFile(filepath.Join(home, "encryption.key"), []byte(keyHex), 0600); err != nil {
		t.Fatal(err)
	}

	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{Outputs: map[string][]string{"ls-remote": {""}}}
	var out strings.Builder

	// "y" for encrypt, "y" for reuse existing key
	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader("y\ny\n"), &out, false); err != nil {
		t.Fatal(err)
	}

	// key file should still hold the original key
	data, err := os.ReadFile(filepath.Join(home, "encryption.key"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != keyHex {
		t.Errorf("expected key to be unchanged; got %q", string(data))
	}
}

func TestInit_NewBranch_Encrypt_ExistingKey_Regenerate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOALIE_HOME", home)
	oldKeyHex := strings.Repeat("ab", 32)
	if err := os.WriteFile(filepath.Join(home, "encryption.key"), []byte(oldKeyHex), 0600); err != nil {
		t.Fatal(err)
	}

	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{Outputs: map[string][]string{"ls-remote": {""}}}

	// "y" for encrypt, "n" to decline reuse → generates a new key
	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader("y\nn\n"), os.Stdout, false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(home, "encryption.key"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) == oldKeyHex {
		t.Error("expected key to be replaced with a newly generated key")
	}
}

func TestInit_KeyMismatch_ShowsWarning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOALIE_HOME", home)

	// Set up a data dir that already exists with a key-check.enc encrypted under keyA
	dataDir := t.TempDir()
	keyA := strings.Repeat("aa", 32)
	keyABytes, _ := hex.DecodeString(keyA)

	keyCheckPath := filepath.Join(dataDir, "key-check.enc")
	if err := crypto.WriteKeyCheck(keyCheckPath, keyABytes); err != nil {
		t.Fatal(err)
	}

	// Write meta.json saying encrypt=true
	if err := meta.Save(dataDir, meta.Meta{Encrypt: true}); err != nil {
		t.Fatal(err)
	}

	// Save a different key (keyB) as the user's local key
	keyB := strings.Repeat("bb", 32)
	if err := os.WriteFile(filepath.Join(home, "encryption.key"), []byte(keyB), 0600); err != nil {
		t.Fatal(err)
	}

	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{}
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader(""), &out, false); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "does not match the team key-check") {
		t.Errorf("expected key mismatch warning; got %q", out.String())
	}
}

func encryptedDataDir(t *testing.T, keyHex string) (dataDir string, keyBytes []byte) {
	t.Helper()
	dataDir = t.TempDir()
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		t.Fatal(err)
	}
	if err := meta.Save(dataDir, meta.Meta{Encrypt: true}); err != nil {
		t.Fatal(err)
	}
	if err := crypto.WriteKeyCheck(filepath.Join(dataDir, "key-check.enc"), keyBytes); err != nil {
		t.Fatal(err)
	}
	return dataDir, keyBytes
}

func TestInit_PromptForKey_Skip(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir, _ := encryptedDataDir(t, strings.Repeat("aa", 32))
	configPath := prewriteConfig(t, "Alice")
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", &git.FakeRunner{}, strings.NewReader("\n"), &out, false); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "No key imported") {
		t.Errorf("expected skip message; got %q", out.String())
	}
}

func TestInit_PromptForKey_ValidKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOALIE_HOME", home)
	keyHex := strings.Repeat("aa", 32)
	dataDir, _ := encryptedDataDir(t, keyHex)
	configPath := prewriteConfig(t, "Alice")
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", &git.FakeRunner{}, strings.NewReader(keyHex+"\n"), &out, false); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "Encryption key verified") {
		t.Errorf("expected verification success; got %q", out.String())
	}
	savedKey, err := os.ReadFile(filepath.Join(home, "encryption.key"))
	if err != nil {
		t.Fatalf("expected key to be saved: %v", err)
	}
	if strings.TrimSpace(string(savedKey)) != keyHex {
		t.Errorf("saved key %q does not match expected %q", string(savedKey), keyHex)
	}
}

func TestInit_PromptForKey_InvalidThenValid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOALIE_HOME", home)
	keyHex := strings.Repeat("aa", 32)
	dataDir, _ := encryptedDataDir(t, keyHex)
	configPath := prewriteConfig(t, "Alice")
	var out strings.Builder

	stdin := strings.NewReader("notvalidhex\n" + keyHex + "\n")
	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", &git.FakeRunner{}, stdin, &out, false); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "Invalid key") {
		t.Errorf("expected invalid key message; got %q", out.String())
	}
	if !strings.Contains(out.String(), "Encryption key verified") {
		t.Errorf("expected eventual success; got %q", out.String())
	}
}

func TestInit_PromptForKey_WrongKeyThenSkip(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	keyHex := strings.Repeat("aa", 32)
	dataDir, _ := encryptedDataDir(t, keyHex)
	configPath := prewriteConfig(t, "Alice")
	var out strings.Builder

	wrongKey := strings.Repeat("bb", 32)
	stdin := strings.NewReader(wrongKey + "\n\n")
	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", &git.FakeRunner{}, stdin, &out, false); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "does not match the team key-check") {
		t.Errorf("expected mismatch message; got %q", out.String())
	}
	if !strings.Contains(out.String(), "No key imported") {
		t.Errorf("expected skip message after wrong key; got %q", out.String())
	}
}

func TestInit_UsernameInvalidThenValid(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")
	runner := &git.FakeRunner{}
	var out strings.Builder

	// First input has a space (invalid), second is valid
	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader("bad user\nalice-jones\n"), &out, false); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "@alice-jones" {
		t.Errorf("expected @alice-jones, got %q", cfg.Name)
	}
	if !strings.Contains(out.String(), "must start with a letter") {
		t.Errorf("expected validation error message in output; got %q", out.String())
	}
}

func TestInit_ConfigNotOverwritten(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
	dataDir := t.TempDir()
	configPath := prewriteConfig(t, "OriginalName")
	runner := &git.FakeRunner{}

	if err := Init("https://example.com/repo.git", dataDir, configPath, "data", runner, strings.NewReader(""), os.Stdout, false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "@OriginalName" {
		t.Errorf("expected config unchanged with Name=@OriginalName; got %q", cfg.Name)
	}
}
