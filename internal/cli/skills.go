package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	claudeembed "goalie/claude"
)

func SkillsInstall(ctx AppContext) error {
	dest, err := claudeCommandsDir()
	if err != nil {
		return err
	}
	return installSkills(ctx, dest, false)
}

func SkillsUpdate(ctx AppContext) error {
	dest, err := claudeCommandsDir()
	if err != nil {
		return err
	}
	return installSkills(ctx, dest, true)
}

func SkillsRemove(ctx AppContext) error {
	dest, err := claudeCommandsDir()
	if err != nil {
		return err
	}
	return removeSkills(ctx, dest)
}

func claudeCommandsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "commands"), nil
}

func installSkills(ctx AppContext, dest string, overwrite bool) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(claudeembed.Skills, "commands", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := filepath.Base(path)
		target := filepath.Join(dest, name)
		if !overwrite {
			if _, statErr := os.Stat(target); statErr == nil {
				fmt.Fprintf(ctx.Stdout, "skipping %s (already installed; use 'goalie skills update' to overwrite)\n", name)
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
		fmt.Fprintf(ctx.Stdout, "installed %s → %s\n", name, target)
		return nil
	})
}

func removeSkills(ctx AppContext, dest string) error {
	return fs.WalkDir(claudeembed.Skills, "commands", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := filepath.Base(path)
		target := filepath.Join(dest, name)
		if err := os.Remove(target); err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(ctx.Stdout, "%s not installed\n", name)
				return nil
			}
			return err
		}
		fmt.Fprintf(ctx.Stdout, "removed %s\n", target)
		return nil
	})
}
