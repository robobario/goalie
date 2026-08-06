package motd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"goalie/internal/crypto"
	"goalie/internal/git"
)

func motdDir(dataDir string) string {
	return filepath.Join(dataDir, "motd")
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Latest returns the content of the most recent MOTD file, or ("", false, nil) if none exists.
func Latest(dataDir string, key []byte) (string, bool, error) {
	dir := motdDir(dataDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".txt" {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return "", false, nil
	}
	sort.Strings(names)
	latest := names[len(names)-1]

	data, err := os.ReadFile(filepath.Join(dir, latest))
	if err != nil {
		return "", false, err
	}
	decrypted, err := crypto.Decrypt(key, data)
	if err != nil {
		return "", false, err
	}
	return string(decrypted), true, nil
}

// Save pulls, writes a new timestamped MOTD file, commits, and pushes with rebase-retry on conflict.
func Save(dataDir string, r git.Runner, text string, key []byte) error {
	if err := r.Run([]string{"pull"}, dataDir); err != nil {
		return err
	}

	dir := motdDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	id, err := randomID()
	if err != nil {
		return err
	}
	timestamp := time.Now().UTC().Format("2006-01-02T150405Z")
	filename := fmt.Sprintf("%s-%s.txt", timestamp, id)
	path := filepath.Join(dir, filename)

	encrypted, err := crypto.Encrypt(key, []byte(text))
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, encrypted, 0o644); err != nil {
		return err
	}

	rel := filepath.Join("motd", filename)
	if err := r.Run([]string{"add", rel}, dataDir); err != nil {
		return err
	}
	if err := r.Run([]string{"commit", "-m", "motd: update"}, dataDir); err != nil {
		return err
	}
	return git.Push(r, dataDir)
}
