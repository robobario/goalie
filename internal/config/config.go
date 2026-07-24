package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"goalie/internal/goalieenv"
)

// UsernameRe matches a valid goalie username: @ followed by a GitHub-style
// handle (alphanumeric and hyphens, starting with alphanumeric, 1–39 chars).
var UsernameRe = regexp.MustCompile(`^@[a-zA-Z0-9][a-zA-Z0-9-]{0,38}$`)

func ValidUsername(s string) bool { return UsernameRe.MatchString(s) }

type Config struct {
	Name string `json:"name"`
}

var ErrNotInitialised = errors.New("goalie not initialised: run 'goalie init <repo-url>' first")

func Load() (*Config, error) {
	home, err := goalieenv.Home()
	if err != nil {
		return nil, err
	}
	return LoadFrom(filepath.Join(home, "config.json"))
}

func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotInitialised
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	home, err := goalieenv.Home()
	if err != nil {
		return err
	}
	return SaveTo(filepath.Join(home, "config.json"), cfg)
}

func SaveTo(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
