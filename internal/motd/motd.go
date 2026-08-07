package motd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"goalie/internal/clock"
	"goalie/internal/crypto"
	"goalie/internal/git"
)

// Entry holds a decrypted MOTD together with the timestamp encoded in its filename.
type Entry struct {
	TS      string // RFC3339
	Content string
}

// filenameTimestampLayout matches the format used when writing MOTD files.
const filenameTimestampLayout = "2006-01-02T150405Z"

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

// AllEntries returns every MOTD as an Entry (timestamp + decrypted content),
// in chronological order (ascending by filename), without pulling from the remote.
// The timestamp is parsed from the filename so it is available even though it
// is not stored inside the file itself.
func AllEntries(dataDir string, key []byte) ([]Entry, error) {
	dir := motdDir(dataDir)
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range dirEntries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".txt" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var result []Entry
	for _, name := range names {
		ts, err := parseFilenameTimestamp(name)
		if err != nil {
			return nil, fmt.Errorf("motd filename %q: %w", name, err)
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		decrypted, err := crypto.Decrypt(key, data)
		if err != nil {
			return nil, err
		}
		result = append(result, Entry{TS: ts, Content: string(decrypted)})
	}
	return result, nil
}

// parseFilenameTimestamp extracts the RFC3339 timestamp from a MOTD filename.
// Filename format: "2006-01-02T150405Z-<uuid>.txt"
func parseFilenameTimestamp(filename string) (string, error) {
	base := strings.TrimSuffix(filepath.Base(filename), ".txt")
	const tsLen = len("2006-01-02T150405Z") // 18
	if len(base) < tsLen {
		return "", fmt.Errorf("too short to contain timestamp")
	}
	t, err := time.Parse(filenameTimestampLayout, base[:tsLen])
	if err != nil {
		return "", err
	}
	return t.UTC().Format(time.RFC3339), nil
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
	timestamp := clock.Now().Format("2006-01-02T150405Z")
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
