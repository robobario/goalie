package motd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goalie/internal/git"
)

func TestLatestReturnsEmptyWhenNoDir(t *testing.T) {
	dir := t.TempDir()
	text, ok, err := Latest(filepath.Join(dir, "data"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false when motd dir does not exist")
	}
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
}

func TestLatestReturnsEmptyWhenDirEmpty(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "motd"), 0o755); err != nil {
		t.Fatal(err)
	}
	text, ok, err := Latest(dataDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected ok=false when motd dir is empty")
	}
	if text != "" {
		t.Errorf("expected empty text, got %q", text)
	}
}

func TestLatestReturnsMostRecentByFilename(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	motdPath := filepath.Join(dataDir, "motd")
	if err := os.MkdirAll(motdPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(motdPath, "2024-01-01T120000Z-aaa.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(motdPath, "2024-06-01T120000Z-bbb.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	text, ok, err := Latest(dataDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if text != "new" {
		t.Errorf("expected %q, got %q", "new", text)
	}
}

func TestSavePullsCommitsAndPushes(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	r := &git.FakeRunner{}

	if err := Save(dataDir, r, "hello world", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(r.Calls) < 3 {
		t.Fatalf("expected at least 3 git calls, got %d: %v", len(r.Calls), r.Calls)
	}
	if r.Calls[0][0] != "pull" {
		t.Errorf("first call should be pull, got %v", r.Calls[0])
	}
	if r.Calls[1][0] != "add" {
		t.Errorf("second call should be add, got %v", r.Calls[1])
	}
	if r.Calls[2][0] != "commit" {
		t.Errorf("third call should be commit, got %v", r.Calls[2])
	}
	if r.Calls[3][0] != "push" {
		t.Errorf("fourth call should be push, got %v", r.Calls[3])
	}

	if !strings.HasPrefix(r.Calls[1][1], "motd/") {
		t.Errorf("add path should start with motd/, got %q", r.Calls[1][1])
	}

	files, _ := os.ReadDir(filepath.Join(dataDir, "motd"))
	if len(files) != 1 {
		t.Fatalf("expected 1 motd file, got %d", len(files))
	}
	data, _ := os.ReadFile(filepath.Join(dataDir, "motd", files[0].Name()))
	if string(data) != "hello world" {
		t.Errorf("file content: expected %q, got %q", "hello world", string(data))
	}
}

func TestSaveRebasesOnPushConflict(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	pushErr := errors.New("push rejected")
	r := &git.FakeRunner{
		Errors: map[string][]error{
			"push": {pushErr, nil},
		},
	}

	if err := Save(dataDir, r, "motd text", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var pushCalls int
	var rebaseCalls int
	for _, call := range r.Calls {
		switch call[0] {
		case "push":
			pushCalls++
		case "pull":
			if len(call) > 1 && call[1] == "--rebase" {
				rebaseCalls++
			}
		}
	}
	if pushCalls != 2 {
		t.Errorf("expected 2 push calls (initial + retry), got %d", pushCalls)
	}
	if rebaseCalls != 1 {
		t.Errorf("expected 1 pull --rebase call, got %d", rebaseCalls)
	}
}

func TestSaveCreatesUniqueFiles(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	r := &git.FakeRunner{}

	for i := 0; i < 3; i++ {
		if err := Save(dataDir, r, "text", nil); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}

	files, _ := os.ReadDir(filepath.Join(dataDir, "motd"))
	if len(files) != 3 {
		t.Errorf("expected 3 unique files, got %d", len(files))
	}
}
