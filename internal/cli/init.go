package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"goalie/internal/config"
	"goalie/internal/crypto"
	"goalie/internal/git"
	"goalie/internal/meta"
)

func Init(repoURL string, dataDir string, configPath string, r git.Runner, stdin io.Reader, stdout io.Writer, tty bool) error {
	// Wrap stdin once so sequential prompts share the same buffer and don't lose buffered input.
	sr := bufio.NewReader(stdin)

	if _, err := os.Stat(dataDir); err == nil {
		fmt.Fprint(stdout, "Goalie data directory already exists.\n")
	} else {
		out, err := r.Output([]string{"ls-remote", "--heads", repoURL, "data"}, "")
		if err != nil {
			return err
		}
		if out != "" {
			if err := r.Run([]string{"clone", "--branch", "data", repoURL, dataDir}, ""); err != nil {
				return err
			}
		} else {
			if err := r.Run([]string{"init", dataDir}, ""); err != nil {
				return err
			}
			if err := r.Run([]string{"symbolic-ref", "HEAD", "refs/heads/data"}, dataDir); err != nil {
				return err
			}
			if err := r.Run([]string{"remote", "add", "origin", repoURL}, dataDir); err != nil {
				return err
			}
			for _, dir := range []string{"goals", "journal"} {
				d := filepath.Join(dataDir, dir)
				if err := os.MkdirAll(d, 0755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(d, ".gitkeep"), nil, 0644); err != nil {
					return err
				}
			}
			encrypt, err := ynPrompt("Enable client-side encryption? (y/n) ", sr, stdout, tty)
			if err != nil {
				return err
			}
			if err := meta.Save(dataDir, meta.Meta{Encrypt: encrypt}); err != nil {
				return err
			}
			if err := r.Run([]string{"add", "goals/.gitkeep", "journal/.gitkeep", "meta.json"}, dataDir); err != nil {
				return err
			}
			if err := r.Run([]string{"commit", "-m", "chore: initialise goalie data branch"}, dataDir); err != nil {
				return err
			}
			if err := r.Run([]string{"push", "--set-upstream", "origin", "data"}, dataDir); err != nil {
				return err
			}
		}
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		name, err := requireInput("Your name: ", sr, stdout, tty)
		if err != nil {
			return err
		}
		if err := config.SaveTo(configPath, &config.Config{Name: name}); err != nil {
			return err
		}
	}

	m, err := meta.Load(dataDir)
	if err != nil {
		return err
	}
	if m.Encrypt {
		if _, err := crypto.LoadKey(); err != nil {
			fmt.Fprint(stdout, "No encryption key found. Generate one with: goalie key init\nOr import an existing key with: goalie key import <hex-key>\n")
		}
	} else {
		fmt.Fprint(stdout, "Data will be stored in plaintext — no encryption key required.\n")
	}

	return nil
}
