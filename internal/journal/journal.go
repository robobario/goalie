package journal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"goalie/internal/crypto"
	"goalie/internal/git"
)

type Entry struct {
	ID       string  `json:"id"`
	TS       string  `json:"ts"`
	Goal     *string `json:"goal"`
	Note     string  `json:"note"`
	Blocked  bool    `json:"blocked"`
	Done     bool    `json:"done,omitempty"`
	Task     *string `json:"task"`
	Username string  `json:"-"`
}

type TaskState struct {
	Goal    *string
	Note    string
	Blocked bool
	Done    bool
	TS      string
}

var weekFileSuffix = regexp.MustCompile(`-\d{4}-W\d{2}$`)

func weekFileName(username string, t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%s-%d-W%02d.jsonl", username, year, week)
}

func weekFilesForRange(journalDir, username string, from, to time.Time) []string {
	weekStart := from.UTC()
	for weekStart.Weekday() != time.Monday {
		weekStart = weekStart.AddDate(0, 0, -1)
	}
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, time.UTC)

	seen := make(map[string]bool)
	var paths []string
	for !weekStart.After(to) {
		path := filepath.Join(journalDir, weekFileName(username, weekStart))
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
		weekStart = weekStart.AddDate(0, 0, 7)
	}
	sort.Strings(paths)
	return paths
}

func usernameFromWeeklyFile(file string) (string, bool) {
	base := strings.TrimSuffix(filepath.Base(file), ".jsonl")
	idx := weekFileSuffix.FindStringIndex(base)
	if idx == nil {
		return "", false
	}
	return base[:idx[0]], true
}

// Append pulls, appends an entry to journal/<username>-YYYY-Www.jsonl, commits, and pushes.
func Append(dataDir string, r git.Runner, username string, e Entry, key []byte) error {
	if err := r.Run([]string{"pull"}, dataDir); err != nil {
		return err
	}

	journalDir := filepath.Join(dataDir, "journal")
	if err := os.MkdirAll(journalDir, 0o755); err != nil {
		return err
	}

	now := time.Now().UTC()
	e.ID = uuid.New().String()
	e.TS = now.Format(time.RFC3339)

	fname := weekFileName(username, now)
	path := filepath.Join(journalDir, fname)

	var existing []byte
	if rawData, readErr := os.ReadFile(path); readErr == nil {
		dec, decErr := crypto.Decrypt(key, rawData)
		if decErr != nil {
			return decErr
		}
		existing = dec
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}

	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	fullContent := append(existing, append(line, '\n')...)

	encrypted, err := crypto.Encrypt(key, fullContent)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, encrypted, 0o644); err != nil {
		return err
	}

	rel := "journal/" + fname
	if err := r.Run([]string{"add", rel}, dataDir); err != nil {
		return err
	}
	if err := r.Run([]string{"commit", "-m", "journal: log entry"}, dataDir); err != nil {
		return err
	}
	return git.Push(r, dataDir)
}

// UpdateEntry replaces the entry identified by original.ID in-place within its weekly
// JSONL file (the week is located via original.TS), then commits and pushes.
// Returns an error if original.ID is empty or no matching entry is found.
func UpdateEntry(dataDir string, r git.Runner, username string, original, updated Entry, key []byte) error {
	if original.ID == "" {
		return fmt.Errorf("entry has no ID")
	}

	if err := r.Run([]string{"pull"}, dataDir); err != nil {
		return err
	}

	ts, err := time.Parse(time.RFC3339, original.TS)
	if err != nil {
		return fmt.Errorf("invalid entry timestamp %q: %w", original.TS, err)
	}
	journalDir := filepath.Join(dataDir, "journal")
	fname := weekFileName(username, ts)
	path := filepath.Join(journalDir, fname)

	rawData, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decrypted, err := crypto.Decrypt(key, rawData)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	replaced := false
	scanner := bufio.NewScanner(bytes.NewReader(decrypted))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Entry
		if jsonErr := json.Unmarshal([]byte(line), &e); jsonErr == nil && e.ID == original.ID {
			updatedLine, marshalErr := json.Marshal(updated)
			if marshalErr != nil {
				return marshalErr
			}
			buf.Write(updatedLine)
			replaced = true
		} else {
			buf.WriteString(line)
		}
		buf.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !replaced {
		return fmt.Errorf("entry with ID %q not found in %s", original.ID, fname)
	}

	encrypted, err := crypto.Encrypt(key, buf.Bytes())
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, encrypted, 0o644); err != nil {
		return err
	}

	rel := "journal/" + fname
	if err := r.Run([]string{"add", rel}, dataDir); err != nil {
		return err
	}
	if err := r.Run([]string{"commit", "-m", "journal: edit entry"}, dataDir); err != nil {
		return err
	}
	return git.Push(r, dataDir)
}

// Collect returns all entries within the last `days` days, optionally filtered
// by a glob pattern on username. An empty userPattern includes all users.
func Collect(dataDir string, r git.Runner, days int, userPattern string, key []byte) ([]Entry, error) {
	if err := r.Run([]string{"pull"}, dataDir); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	journalDir := filepath.Join(dataDir, "journal")

	allFiles, err := filepath.Glob(filepath.Join(journalDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}

	usernameSet := make(map[string]bool)
	for _, file := range allFiles {
		username, ok := usernameFromWeeklyFile(file)
		if !ok {
			continue
		}
		usernameSet[username] = true
	}

	usernames := make([]string, 0, len(usernameSet))
	for u := range usernameSet {
		usernames = append(usernames, u)
	}
	sort.Strings(usernames)

	var entries []Entry
	for _, username := range usernames {
		if userPattern != "" {
			match, err := filepath.Match(userPattern, username)
			if err != nil {
				return nil, err
			}
			if !match {
				continue
			}
		}
		for _, file := range weekFilesForRange(journalDir, username, cutoff, now) {
			fileEntries, err := readEntries(file, username, key)
			if err != nil {
				return nil, err
			}
			for _, e := range fileEntries {
				ts, err := time.Parse(time.RFC3339, e.TS)
				if err != nil {
					continue
				}
				if !ts.Before(cutoff) {
					entries = append(entries, e)
				}
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TS < entries[j].TS
	})
	if entries == nil {
		entries = []Entry{}
	}
	return entries, nil
}

// CollectLatest returns the latest entry per (username, goal, task) key
// within the last `days` days.
func CollectLatest(dataDir string, r git.Runner, days int, key []byte) ([]Entry, error) {
	if err := r.Run([]string{"pull"}, dataDir); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	journalDir := filepath.Join(dataDir, "journal")

	allFiles, err := filepath.Glob(filepath.Join(journalDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}

	usernameSet := make(map[string]bool)
	for _, file := range allFiles {
		username, ok := usernameFromWeeklyFile(file)
		if !ok {
			continue
		}
		usernameSet[username] = true
	}

	usernames := make([]string, 0, len(usernameSet))
	for u := range usernameSet {
		usernames = append(usernames, u)
	}
	sort.Strings(usernames)

	type dedupKey struct {
		username string
		goal     string
		task     string
	}
	latest := make(map[dedupKey]Entry)

	for _, username := range usernames {
		for _, file := range weekFilesForRange(journalDir, username, cutoff, now) {
			fileEntries, err := readEntries(file, username, key)
			if err != nil {
				return nil, err
			}
			for _, e := range fileEntries {
				ts, err := time.Parse(time.RFC3339, e.TS)
				if err != nil {
					continue
				}
				if ts.Before(cutoff) {
					continue
				}
				goalStr := ""
				if e.Goal != nil {
					goalStr = *e.Goal
				}
				taskStr := ""
				if e.Task != nil {
					taskStr = *e.Task
				}
				k := dedupKey{username, goalStr, taskStr}
				if prev, ok := latest[k]; !ok || e.TS > prev.TS {
					latest[k] = e
				}
			}
		}
	}

	result := make([]Entry, 0, len(latest))
	for _, e := range latest {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TS < result[j].TS
	})
	return result, nil
}

// CurrentTaskStates returns the latest state for each task tag found in
// weekly journal files for the given username over the past 4 weeks.
// Entries with nil Task are ignored. Returns an empty map if no files exist.
func CurrentTaskStates(journalDir, username string, key []byte) (map[string]TaskState, error) {
	now := time.Now().UTC()
	from := now.Add(-4 * 7 * 24 * time.Hour)
	files := weekFilesForRange(journalDir, username, from, now)

	states := make(map[string]TaskState)
	for _, path := range files {
		entries, err := readEntries(path, username, key)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Task == nil {
				continue
			}
			states[*e.Task] = TaskState{
				Goal:    e.Goal,
				Note:    e.Note,
				Blocked: e.Blocked,
				Done:    e.Done,
				TS:      e.TS,
			}
		}
	}
	return states, nil
}

// CollectTasks returns all distinct task tags used for a given goalID across all users.
func CollectTasks(dataDir, goalID string, key []byte) ([]string, error) {
	journalDir := filepath.Join(dataDir, "journal")
	files, err := filepath.Glob(filepath.Join(journalDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	for _, file := range files {
		fileEntries, err := readEntries(file, "", key)
		if err != nil {
			return nil, err
		}
		for _, e := range fileEntries {
			if e.Goal != nil && *e.Goal == goalID && e.Task != nil {
				seen[*e.Task] = true
			}
		}
	}

	tasks := make([]string, 0, len(seen))
	for t := range seen {
		tasks = append(tasks, t)
	}
	sort.Strings(tasks)
	return tasks, nil
}

func readEntries(path, username string, key []byte) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	decrypted, err := crypto.Decrypt(key, data)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	scanner := bufio.NewScanner(bytes.NewReader(decrypted))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		e.Username = username
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}
