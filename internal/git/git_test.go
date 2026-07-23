package git

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

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
