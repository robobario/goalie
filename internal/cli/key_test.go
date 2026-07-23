package cli

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeyInit_outputsHex(t *testing.T) {
	t.Setenv("GOALIE_HOME", t.TempDir())
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
	t.Setenv("GOALIE_HOME", t.TempDir())
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

func TestKeyInit_ExistingKey_Declined(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOALIE_HOME", home)
	writeExistingKey(t, home, strings.Repeat("aa", 32))

	var stdout bytes.Buffer
	ctx := AppContext{Stdin: strings.NewReader("n\n"), Stdout: &stdout, Stderr: &bytes.Buffer{}}

	if err := KeyInit(ctx); err != nil {
		t.Fatalf("KeyInit returned error: %v", err)
	}

	// Key file should remain unchanged
	data, err := os.ReadFile(filepath.Join(home, "encryption.key"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != strings.Repeat("aa", 32) {
		t.Errorf("expected key unchanged after declining; got %q", string(data))
	}
}

func TestKeyInit_ExistingKey_Confirmed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOALIE_HOME", home)
	writeExistingKey(t, home, strings.Repeat("aa", 32))

	var stdout bytes.Buffer
	ctx := AppContext{Stdin: strings.NewReader("y\n"), Stdout: &stdout, Stderr: &bytes.Buffer{}}

	if err := KeyInit(ctx); err != nil {
		t.Fatalf("KeyInit returned error: %v", err)
	}

	// Key should have been replaced
	data, err := os.ReadFile(filepath.Join(home, "encryption.key"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) == strings.Repeat("aa", 32) {
		t.Error("expected key to be replaced after confirming")
	}
}

func TestKeyImport_ExistingKey_Declined(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOALIE_HOME", home)
	oldKey := strings.Repeat("aa", 32)
	writeExistingKey(t, home, oldKey)

	newKey := strings.Repeat("bb", 32)
	ctx := AppContext{Stdin: strings.NewReader("n\n"), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	if err := KeyImport(ctx, newKey); err != nil {
		t.Fatalf("KeyImport returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, "encryption.key"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != oldKey {
		t.Errorf("expected key unchanged after declining; got %q", string(data))
	}
}

func TestKeyImport_ExistingKey_Confirmed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOALIE_HOME", home)
	writeExistingKey(t, home, strings.Repeat("aa", 32))

	newKey := strings.Repeat("bb", 32)
	ctx := AppContext{Stdin: strings.NewReader("y\n"), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	if err := KeyImport(ctx, newKey); err != nil {
		t.Fatalf("KeyImport returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, "encryption.key"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != newKey {
		t.Errorf("expected key to be replaced; got %q", string(data))
	}
}

func writeExistingKey(t *testing.T, goalieHome, keyHex string) {
	t.Helper()
	if err := os.MkdirAll(goalieHome, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goalieHome, "encryption.key"), []byte(keyHex), 0600); err != nil {
		t.Fatal(err)
	}
}
