package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRealRunner_DisablesInteractivePrompts asserts every git invocation carries
// the env vars that suppress terminal prompts, GUI askpass helpers, and SSH
// host-key/passphrase prompts, so a credential problem fails fast instead of
// hanging or popping a dialog.
func TestRealRunner_DisablesInteractivePrompts(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho \"$GIT_TERMINAL_PROMPT|$GIT_ASKPASS|$SSH_ASKPASS|$GIT_SSH_COMMAND\"\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	r := &RealRunner{}
	out, err := r.Output([]string{"whatever"}, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parts := strings.Split(out, "|")
	if len(parts) != 4 {
		t.Fatalf("unexpected output: %q", out)
	}
	if parts[0] != "0" {
		t.Errorf("GIT_TERMINAL_PROMPT = %q, want 0", parts[0])
	}
	if parts[1] != "" {
		t.Errorf("GIT_ASKPASS = %q, want empty", parts[1])
	}
	if parts[2] != "" {
		t.Errorf("SSH_ASKPASS = %q, want empty", parts[2])
	}
	if !strings.Contains(parts[3], "BatchMode=yes") {
		t.Errorf("GIT_SSH_COMMAND = %q, want BatchMode=yes", parts[3])
	}
}

func TestPushSucceedsFirstAttempt(t *testing.T) {
	r := &FakeRunner{}
	if err := Push(r, "/tmp"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(r.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d: %v", len(r.Calls), r.Calls)
	}
	if r.Calls[0][0] != "push" {
		t.Errorf("expected push, got %v", r.Calls[0])
	}
}

func TestPushRetriesAfterFirstFailure(t *testing.T) {
	pushErr := errors.New("push failed")
	r := &FakeRunner{
		Errors: map[string][]error{
			"push": {pushErr, nil},
		},
	}
	if err := Push(r, "/tmp"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(r.Calls) != 3 {
		t.Fatalf("expected 3 calls, got %d: %v", len(r.Calls), r.Calls)
	}
	expectArgs(t, r.Calls[0], []string{"push"})
	expectArgs(t, r.Calls[1], []string{"pull", "--rebase"})
	expectArgs(t, r.Calls[2], []string{"push"})
}

func TestPushReturnsErrorWhenRetryFails(t *testing.T) {
	pushErr := errors.New("push failed")
	r := &FakeRunner{
		Errors: map[string][]error{
			"push": {pushErr, pushErr},
		},
	}
	if err := Push(r, "/tmp"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRealRunner_Run_CapturesStderr(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	r := &RealRunner{}
	err := r.Run([]string{"push"}, dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fatal") {
		t.Errorf("expected stderr in error, got: %v", err)
	}
}

func TestRealRunner_Output_CapturesStderr(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	r := &RealRunner{}
	_, err := r.Output([]string{"rev-parse", "nonexistent-ref"}, dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "fatal") {
		t.Errorf("expected stderr in error, got: %v", err)
	}
}

func TestRealRunner_Run_Succeeds(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	r := &RealRunner{}
	if err := r.Run([]string{"status"}, dir); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestRealRunner_Output_Succeeds(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	r := &RealRunner{}
	out, err := r.Output([]string{"rev-parse", "--show-toplevel"}, dir)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if out != dir {
		t.Errorf("expected %q, got %q", dir, out)
	}
}

func TestVerifyWriteAccess_Success(t *testing.T) {
	dir := t.TempDir()
	r := &FakeRunner{}

	if err := VerifyWriteAccess(r, dir, "data", true); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if len(r.Calls) != 6 {
		t.Fatalf("expected 6 calls, got %d: %v", len(r.Calls), r.Calls)
	}
	if len(r.Calls[0]) != 2 || r.Calls[0][0] != "add" {
		t.Fatalf("expected add <file>, got %v", r.Calls[0])
	}
	file := r.Calls[0][1]
	if !strings.HasPrefix(file, ".goalie-write-check-") {
		t.Errorf("unexpected file name %q", file)
	}
	expectArgs(t, r.Calls[1], []string{"commit", "-m", "chore: verify write access"})
	expectArgs(t, r.Calls[2], []string{"push", "--set-upstream", "origin", "data"})
	expectArgs(t, r.Calls[3], []string{"rm", file})
	expectArgs(t, r.Calls[4], []string{"commit", "-m", "chore: remove write access check"})
	expectArgs(t, r.Calls[5], []string{"push"})
}

func TestVerifyWriteAccess_NoSetUpstream(t *testing.T) {
	dir := t.TempDir()
	r := &FakeRunner{}

	if err := VerifyWriteAccess(r, dir, "data", false); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	expectArgs(t, r.Calls[2], []string{"push"})
}

func TestVerifyWriteAccess_RetriesPushOnFailure(t *testing.T) {
	dir := t.TempDir()
	pushErr := errors.New("push failed")
	r := &FakeRunner{Errors: map[string][]error{"push": {pushErr, nil}}}

	if err := VerifyWriteAccess(r, dir, "data", false); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	if len(r.Calls) != 8 {
		t.Fatalf("expected 8 calls (extra pull --rebase), got %d: %v", len(r.Calls), r.Calls)
	}
	expectArgs(t, r.Calls[3], []string{"pull", "--rebase"})
	expectArgs(t, r.Calls[4], []string{"push"})
}

func TestVerifyWriteAccess_PermanentPushFailureStopsEarly(t *testing.T) {
	dir := t.TempDir()
	pushErr := errors.New("push failed")
	r := &FakeRunner{Errors: map[string][]error{"push": {pushErr, pushErr}}}

	if err := VerifyWriteAccess(r, dir, "data", false); err == nil {
		t.Fatal("expected error, got nil")
	}

	// add, commit, then push/pull-rebase/push all fail out — never gets to
	// removing the write-check file.
	if len(r.Calls) != 5 {
		t.Fatalf("expected 5 calls, got %d: %v", len(r.Calls), r.Calls)
	}
	for _, call := range r.Calls {
		if len(call) > 0 && call[0] == "rm" {
			t.Errorf("expected no rm call after permanent push failure; got %v", r.Calls)
		}
	}
}

func expectArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("args length: got %v, want %v", got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("args[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
