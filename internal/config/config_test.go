package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidUsername(t *testing.T) {
	valid := []string{"@alice", "@Alice", "@alice-jones", "@a", "@a1b2c3"}
	for _, v := range valid {
		if !ValidUsername(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []string{"alice", "@", "@-alice", "@alice!", "@ alice", ""}
	for _, v := range invalid {
		if ValidUsername(v) {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Name: "my-repo"}

	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.Name != cfg.Name {
		t.Errorf("got Name %q, want %q", got.Name, cfg.Name)
	}
}

func TestLoadFromMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	_, err := LoadFrom(path)
	if !errors.Is(err, ErrNotInitialised) {
		t.Errorf("got %v, want ErrNotInitialised", err)
	}
}

func TestSaveToCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "config.json")
	cfg := &Config{Name: "test"}

	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestWrittenJSONIsValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Name: "valid-json"}

	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("invalid JSON: %v", err)
	}
}
