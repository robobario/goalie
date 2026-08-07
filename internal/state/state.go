package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"goalie/internal/goalieenv"
)

type State struct {
	LastVersionCheck string `json:"last_version_check,omitempty"`
}

func Load() (*State, error) {
	home, err := goalieenv.Home()
	if err != nil {
		return nil, err
	}
	return LoadFrom(filepath.Join(home, "state.json"))
}

func LoadFrom(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func Save(s *State) error {
	home, err := goalieenv.Home()
	if err != nil {
		return err
	}
	return SaveTo(filepath.Join(home, "state.json"), s)
}

func SaveTo(path string, s *State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// VersionCheckDue reports whether the last version check was more than 24 hours ago.
func VersionCheckDue(s *State) bool {
	if s.LastVersionCheck == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, s.LastVersionCheck)
	if err != nil {
		return true
	}
	return time.Since(t) > 24*time.Hour
}
