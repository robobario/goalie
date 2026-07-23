package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goalie/internal/config"
	"goalie/internal/git"
	"goalie/internal/meta"
)

func TestInit_NoKeyPrintsGuidance(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dataDir := t.TempDir()
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{}
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, runner, strings.NewReader(""), &out, false); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "No encryption key found. Generate one with: goalie key init") {
		t.Errorf("expected key guidance in output; got %q", out.String())
	}
}

func TestInit_KeyExistsNoGuidance(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	keyDir := filepath.Join(home, ".goalie")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		t.Fatal(err)
	}
	// 64 hex chars = 32 bytes
	keyHex := strings.Repeat("a1", 32)
	if err := os.WriteFile(filepath.Join(keyDir, "encryption.key"), []byte(keyHex), 0600); err != nil {
		t.Fatal(err)
	}

	dataDir := t.TempDir()
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{}
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, runner, strings.NewReader(""), &out, false); err != nil {
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
	if err := config.SaveTo(path, &config.Config{Name: name}); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInit_DataBranchExists(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "existing")
	runner := &git.FakeRunner{
		Outputs: map[string][]string{
			"ls-remote": {"abc123\trefs/heads/data\n"},
		},
	}

	if err := Init("https://example.com/repo.git", dataDir, configPath, runner, strings.NewReader(""), os.Stdout, false); err != nil {
		t.Fatal(err)
	}

	if !hasCall(runner.Calls, "clone", "--branch", "data") {
		t.Errorf("expected clone --branch data call; got %v", runner.Calls)
	}
}

func TestInit_DataBranchDoesNotExist(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "existing")
	runner := &git.FakeRunner{
		Outputs: map[string][]string{
			"ls-remote": {""},
		},
	}

	if err := Init("https://example.com/repo.git", dataDir, configPath, runner, strings.NewReader("n\n"), os.Stdout, false); err != nil {
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

func TestInit_DataDirAlreadyExists(t *testing.T) {
	dataDir := t.TempDir()
	configPath := prewriteConfig(t, "existing")
	runner := &git.FakeRunner{}
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, runner, strings.NewReader(""), &out, false); err != nil {
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
	dataDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")
	runner := &git.FakeRunner{}

	if err := Init("https://example.com/repo.git", dataDir, configPath, runner, strings.NewReader("Alice\n"), os.Stdout, false); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	if cfg.Name != "Alice" {
		t.Errorf("expected Name=Alice; got %q", cfg.Name)
	}
}

func TestInit_NewBranch_MetaEncryptTrue(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{Outputs: map[string][]string{"ls-remote": {""}}}
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, runner, strings.NewReader("y\n"), &out, false); err != nil {
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
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{Outputs: map[string][]string{"ls-remote": {""}}}
	var out strings.Builder

	if err := Init("https://example.com/repo.git", dataDir, configPath, runner, strings.NewReader("n\n"), &out, false); err != nil {
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
	dataDir := filepath.Join(t.TempDir(), "data")
	configPath := prewriteConfig(t, "Alice")
	runner := &git.FakeRunner{Outputs: map[string][]string{"ls-remote": {"abc123\trefs/heads/data\n"}}}

	// stdin is empty — if a prompt were shown, the call would fail with EOF
	if err := Init("https://example.com/repo.git", dataDir, configPath, runner, strings.NewReader(""), os.Stdout, false); err != nil {
		t.Fatal(err)
	}
}

func TestInit_ConfigNotOverwritten(t *testing.T) {
	dataDir := t.TempDir()
	configPath := prewriteConfig(t, "OriginalName")
	runner := &git.FakeRunner{}

	if err := Init("https://example.com/repo.git", dataDir, configPath, runner, strings.NewReader(""), os.Stdout, false); err != nil {
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
	if cfg.Name != "OriginalName" {
		t.Errorf("expected config unchanged with Name=OriginalName; got %q", cfg.Name)
	}
}
