package meta

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFrom_missing_defaults_to_encrypt(t *testing.T) {
	m, err := LoadFrom("/nonexistent/path/meta.json")
	if err != nil {
		t.Fatal(err)
	}
	if !m.Encrypt {
		t.Error("expected Encrypt=true when file is absent")
	}
}

func TestSaveTo_and_LoadFrom_roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meta.json")

	for _, encrypt := range []bool{true, false} {
		if err := SaveTo(path, Meta{Encrypt: encrypt}); err != nil {
			t.Fatal(err)
		}
		got, err := LoadFrom(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Encrypt != encrypt {
			t.Errorf("Encrypt: got %v, want %v", got.Encrypt, encrypt)
		}
	}
}

func TestLoad_uses_data_dir(t *testing.T) {
	dir := t.TempDir()
	if err := SaveTo(filepath.Join(dir, "meta.json"), Meta{Encrypt: false}); err != nil {
		t.Fatal(err)
	}
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Encrypt {
		t.Error("expected Encrypt=false")
	}
}

func TestSaveTo_creates_parent_dirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "meta.json")
	if err := SaveTo(path, Meta{Encrypt: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
