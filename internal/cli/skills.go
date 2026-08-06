package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	claudeembed "goalie/claude"
)

func SkillsInstall(ctx AppContext) error {
	dest, err := claudeSkillsDir()
	if err != nil {
		return err
	}
	return installSkills(ctx, dest, false)
}

func SkillsUpdate(ctx AppContext) error {
	dest, err := claudeSkillsDir()
	if err != nil {
		return err
	}
	return installSkills(ctx, dest, true)
}

func SkillsRemove(ctx AppContext) error {
	dest, err := claudeSkillsDir()
	if err != nil {
		return err
	}
	return removeSkills(ctx, dest)
}

func claudeSkillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "skills"), nil
}

func installSkills(ctx AppContext, dest string, overwrite bool) error {
	return fs.WalkDir(claudeembed.Skills, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("skills", path)
		if rel == "." {
			return nil
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !overwrite {
			if _, statErr := os.Stat(target); statErr == nil {
				fmt.Fprintf(ctx.Stdout, "skipping %s (already installed; use 'goalie skills update' to overwrite)\n", rel)
				return nil
			}
		}
		data, err := claudeembed.Skills.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(ctx.Stdout, "installed %s → %s\n", rel, target)
		return nil
	})
}

func removeSkills(ctx AppContext, dest string) error {
	entries, err := claudeembed.Skills.ReadDir("skills")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		target := filepath.Join(dest, entry.Name())
		_, statErr := os.Stat(target)
		if os.IsNotExist(statErr) {
			fmt.Fprintf(ctx.Stdout, "%s not installed\n", entry.Name())
			continue
		}
		if err := os.RemoveAll(target); err != nil {
			return err
		}
		fmt.Fprintf(ctx.Stdout, "removed %s\n", entry.Name())
	}
	return nil
}
