package cli

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"goalie/internal/config"
	"goalie/internal/crypto"
	"goalie/internal/display"
	"goalie/internal/git"
	"goalie/internal/meta"
)

func Init(repoURL string, dataDir string, configPath string, branch string, r git.Runner, ctx AppContext) error {
	// Wrap stdin once so sequential prompts share the same buffer and don't lose buffered input.
	sr := bufio.NewReader(ctx.Stdin)

	if _, err := os.Stat(dataDir); err == nil {
		fmt.Fprint(ctx.Stdout, "Goalie data directory already exists.\n")
	} else {
		if err := setupDataDir(repoURL, dataDir, branch, r, ctx, sr); err != nil {
			os.RemoveAll(dataDir)
			return err
		}
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		username, err := promptUsername(sr, ctx)
		if err != nil {
			return err
		}
		if err := config.SaveTo(configPath, &config.Config{Name: username}); err != nil {
			return err
		}
	}

	m, err := meta.Load(dataDir)
	if err != nil {
		return err
	}
	if m.Encrypt {
		key, loadErr := crypto.LoadKey()
		if loadErr != nil {
			if err := promptForKey(sr, ctx, dataDir); err != nil {
				return err
			}
		} else {
			keyCheckPath := filepath.Join(dataDir, "key-check.enc")
			if ok, _ := crypto.VerifyKeyCheck(keyCheckPath, key); ok {
				fmt.Fprint(ctx.Stdout, display.Green("Encryption key verified.", ctx.DisplayCtx())+"\n")
			} else {
				fmt.Fprint(ctx.Stdout, "Warning: your encryption key does not match the team key-check. Run: goalie key import <hex>\n")
			}
		}
	}

	return nil
}

// setupDataDir clones the data branch if it already exists on the remote, or
// creates it from scratch otherwise. Any error it returns means dataDir is
// not in a usable state; the caller must remove it before returning.
func setupDataDir(repoURL, dataDir, branch string, r git.Runner, ctx AppContext, sr *bufio.Reader) error {
	out, err := r.Output([]string{"ls-remote", "--heads", repoURL, branch}, "")
	if err != nil {
		return fmt.Errorf("could not reach %s to check for branch %q — verify the URL and your network access:\n%w", repoURL, branch, err)
	}
	if out != "" {
		if err := r.Run([]string{"clone", "--branch", branch, repoURL, dataDir}, ""); err != nil {
			return fmt.Errorf("failed to clone %s (branch %q) — verify the URL and that you have read access:\n%w", repoURL, branch, err)
		}
	} else {
		if err := r.Run([]string{"init", dataDir}, ""); err != nil {
			return err
		}
		if err := r.Run([]string{"symbolic-ref", "HEAD", "refs/heads/" + branch}, dataDir); err != nil {
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
		encrypt, err := ynPrompt("Enable client-side encryption? (y/n) ", sr, ctx)
		if err != nil {
			return err
		}
		if err := meta.Save(dataDir, meta.Meta{Encrypt: encrypt}); err != nil {
			return err
		}

		addArgs := []string{"add", "goals/.gitkeep", "journal/.gitkeep", "meta.json"}
		var freshKey []byte
		if encrypt {
			freshKey, err = setupEncryptionKey(sr, ctx)
			if err != nil {
				return err
			}
			keyCheckPath := filepath.Join(dataDir, "key-check.enc")
			if err := crypto.WriteKeyCheck(keyCheckPath, freshKey); err != nil {
				return err
			}
			addArgs = append(addArgs, "key-check.enc")
		}

		if err := r.Run(addArgs, dataDir); err != nil {
			return err
		}
		if err := r.Run([]string{"commit", "-m", "chore: initialise goalie data branch"}, dataDir); err != nil {
			return err
		}
		if err := r.Run([]string{"push", "--set-upstream", "origin", branch}, dataDir); err != nil {
			return err
		}

		if encrypt {
			fmt.Fprintf(ctx.Stdout, "Encryption key: %s\nShare with teammates: goalie key import <key>\nkey-check.enc committed to the data branch — teammates must import the same key.\n", hex.EncodeToString(freshKey))
		} else {
			fmt.Fprint(ctx.Stdout, "Data will be stored in plaintext — no encryption key required.\n")
		}
	}
	return nil
}

// promptUsername loops until the user enters a valid GitHub-style handle.
// The '@' prefix is shown as a fixed part of the prompt; the user types only the body.
func promptUsername(r io.Reader, ctx AppContext) (string, error) {
	for {
		fmt.Fprint(ctx.Stdout, display.Bold("Your username: @", ctx.DisplayCtx()))
		line, err := readLine(r)
		if err != nil {
			return "", err
		}
		body := strings.TrimSpace(line)
		username := "@" + body
		if config.ValidUsername(username) {
			return username, nil
		}
		fmt.Fprint(ctx.Stdout, "Username must start with a letter or digit and contain only letters, digits, and hyphens (e.g. @alice or @alice-jones).\n")
	}
}

// promptForKey loops until the user pastes a valid, verified hex key or presses Enter to skip.
func promptForKey(r io.Reader, ctx AppContext, dataDir string) error {
	for {
		fmt.Fprint(ctx.Stdout, display.Bold("Encryption key (paste hex or press Enter to skip): ", ctx.DisplayCtx()))
		line, err := readLine(r)
		if err == io.EOF {
			fmt.Fprint(ctx.Stdout, "No key imported. Run: goalie key import <hex-key> when ready.\n")
			return nil
		}
		if err != nil {
			return err
		}
		hexKey := strings.TrimSpace(line)
		if hexKey == "" {
			fmt.Fprint(ctx.Stdout, "No key imported. Run: goalie key import <hex-key> when ready.\n")
			return nil
		}
		decoded, decodeErr := hex.DecodeString(hexKey)
		if decodeErr != nil || len(decoded) != 32 {
			fmt.Fprint(ctx.Stdout, "Invalid key: must be 64 hex characters (32 bytes). Try again, or press Enter to skip.\n")
			continue
		}
		keyCheckPath := filepath.Join(dataDir, "key-check.enc")
		ok, err := crypto.VerifyKeyCheck(keyCheckPath, decoded)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprint(ctx.Stdout, "Key does not match the team key-check. Try again, or press Enter to skip.\n")
			continue
		}
		if err := crypto.SaveKey(decoded); err != nil {
			return err
		}
		fmt.Fprint(ctx.Stdout, display.Green("Encryption key verified.", ctx.DisplayCtx())+"\n")
		return nil
	}
}

// setupEncryptionKey resolves the key for a fresh encrypted repo.
// If the user already has a local key, it asks whether to reuse it.
// Otherwise a new key is generated and saved.
func setupEncryptionKey(r io.Reader, ctx AppContext) ([]byte, error) {
	existing, err := crypto.LoadKey()
	if err == nil {
		reuse, err := ynPrompt("Use your existing encryption key? (y/n) ", r, ctx)
		if err != nil {
			return nil, err
		}
		if reuse {
			return existing, nil
		}
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	if err := crypto.SaveKey(key); err != nil {
		return nil, err
	}
	return key, nil
}
