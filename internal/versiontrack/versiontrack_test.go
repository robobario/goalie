package versiontrack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"goalie/internal/git"
)

func writeVersionFile(t *testing.T, dir, schemaVersion string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.MarshalIndent(versionFile{SchemaVersion: schemaVersion}, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "test-"+schemaVersion+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHighestRecorded_empty(t *testing.T) {
	h, err := HighestRecorded(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if h != "" {
		t.Errorf("expected empty, got %q", h)
	}
}

func TestHighestRecorded_single(t *testing.T) {
	dataDir := t.TempDir()
	writeVersionFile(t, filepath.Join(dataDir, "versions"), "1.2.3")
	h, err := HighestRecorded(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if h != "1.2.3" {
		t.Errorf("got %q, want 1.2.3", h)
	}
}

func TestHighestRecorded_picks_highest(t *testing.T) {
	dataDir := t.TempDir()
	dir := filepath.Join(dataDir, "versions")
	writeVersionFile(t, dir, "1.0.0")
	writeVersionFile(t, dir, "2.0.0")
	writeVersionFile(t, dir, "1.9.0")
	h, err := HighestRecorded(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if h != "2.0.0" {
		t.Errorf("got %q, want 2.0.0", h)
	}
}

func TestRecord_consolidates_when_highest(t *testing.T) {
	dataDir := t.TempDir()
	dir := filepath.Join(dataDir, "versions")
	writeVersionFile(t, dir, "1.0.0")
	writeVersionFile(t, dir, "0.9.0")

	r := &git.FakeRunner{}
	highest, err := Record(dataDir, r, "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if highest != "2.0.0" {
		t.Errorf("got %q, want 2.0.0", highest)
	}

	// Only one version file should remain.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var jsonFiles []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			jsonFiles = append(jsonFiles, e.Name())
		}
	}
	if len(jsonFiles) != 1 {
		t.Errorf("expected 1 version file, got %d: %v", len(jsonFiles), jsonFiles)
	}

	// Remaining file must contain the new version.
	data, err := os.ReadFile(filepath.Join(dir, jsonFiles[0]))
	if err != nil {
		t.Fatal(err)
	}
	var vf versionFile
	if err := json.Unmarshal(data, &vf); err != nil {
		t.Fatal(err)
	}
	if vf.SchemaVersion != "2.0.0" {
		t.Errorf("version file has %q, want 2.0.0", vf.SchemaVersion)
	}
}

func TestRecord_does_nothing_when_behind(t *testing.T) {
	dataDir := t.TempDir()
	dir := filepath.Join(dataDir, "versions")
	writeVersionFile(t, dir, "2.0.0")

	r := &git.FakeRunner{}
	highest, err := Record(dataDir, r, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if highest != "2.0.0" {
		t.Errorf("got %q, want 2.0.0", highest)
	}

	// Version files should be untouched.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var jsonFiles []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			jsonFiles = append(jsonFiles, e.Name())
		}
	}
	if len(jsonFiles) != 1 {
		t.Errorf("expected original 1 file, got %d", len(jsonFiles))
	}
}

func TestRecord_skips_non_release_version(t *testing.T) {
	dataDir := t.TempDir()
	r := &git.FakeRunner{}
	highest, err := Record(dataDir, r, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if highest != "" {
		t.Errorf("expected empty highest for dev build, got %q", highest)
	}
	// No git calls should have been made.
	if len(r.Calls) != 0 {
		t.Errorf("expected no git calls for dev build, got %v", r.Calls)
	}
}
