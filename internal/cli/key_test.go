package cli

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestKeyInit_outputsHex(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var stdout bytes.Buffer
	ctx := AppContext{Stdout: &stdout, Stderr: &bytes.Buffer{}}

	if err := KeyInit(ctx); err != nil {
		t.Fatalf("KeyInit returned error: %v", err)
	}

	out := strings.TrimSpace(stdout.String())
	if len(out) != 64 {
		t.Fatalf("expected 64 hex chars, got %d: %q", len(out), out)
	}
	if _, err := hex.DecodeString(out); err != nil {
		t.Fatalf("output is not valid hex: %v", err)
	}
}

func TestKeyImport_valid(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx := AppContext{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	if err := KeyImport(ctx, strings.Repeat("ab", 32)); err != nil {
		t.Fatalf("KeyImport returned error: %v", err)
	}
}

func TestKeyImport_tooShort(t *testing.T) {
	var stderr bytes.Buffer
	ctx := AppContext{Stdout: &bytes.Buffer{}, Stderr: &stderr}

	err := KeyImport(ctx, "ab")

	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("expected ExitError{Code:1}, got %v", err)
	}
	if !strings.Contains(stderr.String(), "invalid key") {
		t.Fatalf("expected error message on stderr, got %q", stderr.String())
	}
}

func TestKeyImport_tooLong(t *testing.T) {
	var stderr bytes.Buffer
	ctx := AppContext{Stdout: &bytes.Buffer{}, Stderr: &stderr}

	err := KeyImport(ctx, strings.Repeat("ab", 33))

	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("expected ExitError{Code:1}, got %v", err)
	}
}

func TestKeyImport_nonHex(t *testing.T) {
	var stderr bytes.Buffer
	ctx := AppContext{Stdout: &bytes.Buffer{}, Stderr: &stderr}

	err := KeyImport(ctx, strings.Repeat("zz", 32))

	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("expected ExitError{Code:1}, got %v", err)
	}
}
