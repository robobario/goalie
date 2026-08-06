package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func skillsTestCtx(t *testing.T) AppContext {
	t.Helper()
	return AppContext{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
}

func TestSkillsInstall_WritesFiles(t *testing.T) {
	dest := t.TempDir()
	ctx := skillsTestCtx(t)

	if err := installSkills(ctx, dest, false); err != nil {
		t.Fatalf("installSkills: %v", err)
	}

	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one skill directory to be installed")
	}
	for _, e := range entries {
		if !e.IsDir() {
			t.Errorf("expected skill subdirectory, got file: %s", e.Name())
		}
		skillFile := filepath.Join(dest, e.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			t.Errorf("expected SKILL.md in %s: %v", e.Name(), err)
		}
	}
}

func TestSkillsInstall_SkipsExisting(t *testing.T) {
	dest := t.TempDir()
	ctx := skillsTestCtx(t)

	if err := installSkills(ctx, dest, false); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// Overwrite SKILL.md in the first skill directory with sentinel content.
	entries, _ := os.ReadDir(dest)
	target := filepath.Join(dest, entries[0].Name(), "SKILL.md")
	if err := os.WriteFile(target, []byte("sentinel"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := installSkills(ctx, dest, false); err != nil {
		t.Fatalf("second install: %v", err)
	}

	got, _ := os.ReadFile(target)
	if string(got) != "sentinel" {
		t.Error("install without overwrite should not replace existing file")
	}
}

func TestSkillsUpdate_OverwritesExisting(t *testing.T) {
	dest := t.TempDir()
	ctx := skillsTestCtx(t)

	if err := installSkills(ctx, dest, false); err != nil {
		t.Fatalf("first install: %v", err)
	}

	entries, _ := os.ReadDir(dest)
	target := filepath.Join(dest, entries[0].Name(), "SKILL.md")
	if err := os.WriteFile(target, []byte("sentinel"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := installSkills(ctx, dest, true); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := os.ReadFile(target)
	if string(got) == "sentinel" {
		t.Error("install with overwrite should replace existing file")
	}
}

func TestSkillsRemove_RemovesInstalledFiles(t *testing.T) {
	dest := t.TempDir()
	ctx := skillsTestCtx(t)

	if err := installSkills(ctx, dest, false); err != nil {
		t.Fatalf("install: %v", err)
	}

	if err := removeSkills(ctx, dest); err != nil {
		t.Fatalf("removeSkills: %v", err)
	}

	entries, _ := os.ReadDir(dest)
	if len(entries) != 0 {
		t.Errorf("expected dest to be empty after remove, got %d entries", len(entries))
	}
}

func TestSkillsRemove_GracefulWhenNotInstalled(t *testing.T) {
	dest := t.TempDir()
	ctx := skillsTestCtx(t)

	if err := removeSkills(ctx, dest); err != nil {
		t.Errorf("removeSkills on empty dir should not error: %v", err)
	}
}
