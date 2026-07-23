package goals

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"goalie/internal/crypto"
	"goalie/internal/git"
)

var (
	GoalIDRe    = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	ThreadTagRe = regexp.MustCompile(`^#[a-z][a-z0-9_-]*$`)
)

func ValidGoalID(id string) bool    { return GoalIDRe.MatchString(id) }
func ValidThreadTag(tag string) bool { return ThreadTagRe.MatchString(tag) }

var (
	ErrGoalExists   = errors.New("goal already exists")
	ErrGoalNotFound = errors.New("goal not found")
	ErrGoalClosed   = errors.New("goal is already closed")
)

type Goal struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	State       string `json:"state"`
	Created     string `json:"created"`
}

// GoalFilename returns the filename for a goal. With a key the name is
// HMAC-SHA256 derived so the goal ID is never written to disk or exposed in
// git history. Without a key (unencrypted mode) the plain ID is used so the
// data directory is self-explanatory.
func GoalFilename(key []byte, id string) string {
	if key == nil {
		return id + ".json"
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil)) + ".json"
}

func goalPath(dataDir string, key []byte, id string) string {
	return filepath.Join(dataDir, "goals", GoalFilename(key, id))
}

func Add(dataDir string, r git.Runner, id, description string, key []byte) error {
	if err := r.Run([]string{"pull"}, dataDir); err != nil {
		return err
	}

	path := goalPath(dataDir, key, id)
	if _, err := os.Stat(path); err == nil {
		return ErrGoalExists
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	g := Goal{
		ID:          id,
		Description: description,
		State:       "open",
		Created:     time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	encrypted, err := crypto.Encrypt(key, data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, encrypted, 0o644); err != nil {
		return err
	}

	rel := "goals/" + GoalFilename(key, id)
	if err := r.Run([]string{"add", rel}, dataDir); err != nil {
		return err
	}
	if err := r.Run([]string{"commit", "-m", "goals: add goal"}, dataDir); err != nil {
		return err
	}
	return git.Push(r, dataDir)
}

func Close(dataDir string, r git.Runner, id string, key []byte) error {
	if err := r.Run([]string{"pull"}, dataDir); err != nil {
		return err
	}

	path := goalPath(dataDir, key, id)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrGoalNotFound
		}
		return err
	}

	decrypted, err := crypto.Decrypt(key, data)
	if err != nil {
		return err
	}

	var g Goal
	if err := json.Unmarshal(decrypted, &g); err != nil {
		return err
	}
	if g.State == "closed" {
		return ErrGoalClosed
	}

	g.State = "closed"
	updated, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	encrypted, err := crypto.Encrypt(key, updated)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, encrypted, 0o644); err != nil {
		return err
	}

	rel := "goals/" + GoalFilename(key, id)
	if err := r.Run([]string{"add", rel}, dataDir); err != nil {
		return err
	}
	if err := r.Run([]string{"commit", "-m", "goals: close goal"}, dataDir); err != nil {
		return err
	}
	return git.Push(r, dataDir)
}

func List(dataDir string, key []byte) ([]Goal, error) {
	dir := filepath.Join(dataDir, "goals")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Goal{}, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	goals := make([]Goal, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		decrypted, err := crypto.Decrypt(key, data)
		if err != nil {
			return nil, err
		}
		var g Goal
		if err := json.Unmarshal(decrypted, &g); err != nil {
			return nil, err
		}
		goals = append(goals, g)
	}
	sort.Slice(goals, func(i, j int) bool { return goals[i].ID < goals[j].ID })
	return goals, nil
}

func Exists(dataDir, id string, key []byte) bool {
	_, err := os.Stat(goalPath(dataDir, key, id))
	return err == nil
}
