package versiontrack

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"goalie/internal/git"
	"goalie/internal/version"
)

type versionFile struct {
	SchemaVersion string `json:"schema_version"`
}

func versionsDir(dataDir string) string {
	return filepath.Join(dataDir, "versions")
}

// HighestRecorded returns the highest schema version found in the versions/
// directory, or "" if the directory is empty or absent.
func HighestRecorded(dataDir string) (string, error) {
	dir := versionsDir(dataDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	highest := ""
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var vf versionFile
		if err := json.Unmarshal(data, &vf); err != nil {
			continue
		}
		if version.Compare(vf.SchemaVersion, highest) > 0 {
			highest = vf.SchemaVersion
		}
	}
	return highest, nil
}

// Record pulls the data repo, checks whether the running schema version is the
// highest seen, and if so consolidates: deletes all existing version files,
// writes a new UUID-named file, commits, and pushes (with rebase-retry).
// If the running version is not the highest it does nothing.
// Returns the highest schema version found after the pull (which may be higher
// than the running version).
func Record(dataDir string, r git.Runner, schemaVersion string) (string, error) {
	if version.Major(schemaVersion) < 0 {
		// Non-release version string — skip recording.
		return "", nil
	}

	if err := r.Run([]string{"pull"}, dataDir); err != nil {
		return "", err
	}

	highest, err := HighestRecorded(dataDir)
	if err != nil {
		return "", err
	}

	if version.Compare(schemaVersion, highest) < 0 {
		// Running version is behind; nothing to write.
		return highest, nil
	}

	dir := versionsDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	// Remove existing version files.
	existing, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return "", err
	}
	for _, f := range existing {
		if err := os.Remove(f); err != nil {
			return "", err
		}
	}

	// Write new UUID-named version file.
	id, err := randomID()
	if err != nil {
		return "", err
	}
	newFile := filepath.Join(dir, id+".json")
	data, err := json.MarshalIndent(versionFile{SchemaVersion: schemaVersion}, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(newFile, data, 0o644); err != nil {
		return "", err
	}

	// Stage: remove old files, add new file.
	addArgs := []string{"add", filepath.Join("versions", id+".json")}
	for _, f := range existing {
		rel, relErr := filepath.Rel(dataDir, f)
		if relErr != nil {
			rel = f
		}
		addArgs = append(addArgs, rel)
	}
	if err := r.Run(addArgs, dataDir); err != nil {
		return "", err
	}

	msg := fmt.Sprintf("chore: record schema version %s", schemaVersion)
	if err := r.Run([]string{"commit", "-m", msg}, dataDir); err != nil {
		return "", err
	}

	return schemaVersion, git.Push(r, dataDir)
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
