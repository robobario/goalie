package meta

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Meta struct {
	Encrypt bool `json:"encrypt"`
}

func Load(dataDir string) (Meta, error) {
	return LoadFrom(filepath.Join(dataDir, "meta.json"))
}

// LoadFrom reads Meta from path. If the file does not exist it returns
// Meta{Encrypt: true} so that repos initialised before this field existed
// continue to require a key.
func LoadFrom(path string) (Meta, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Meta{Encrypt: true}, nil
	}
	if err != nil {
		return Meta{}, err
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return Meta{}, err
	}
	return m, nil
}

func Save(dataDir string, m Meta) error {
	return SaveTo(filepath.Join(dataDir, "meta.json"), m)
}

func SaveTo(path string, m Meta) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
