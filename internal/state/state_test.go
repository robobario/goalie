package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFrom_missing_returns_empty(t *testing.T) {
	s, err := LoadFrom("/nonexistent/path/state.json")
	if err != nil {
		t.Fatal(err)
	}
	if s.LastVersionCheck != "" {
		t.Errorf("expected empty LastVersionCheck, got %q", s.LastVersionCheck)
	}
}

func TestSaveTo_and_LoadFrom_roundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	ts := "2026-01-02T03:04:05Z"
	if err := SaveTo(path, &State{LastVersionCheck: ts}); err != nil {
		t.Fatal(err)
	}
	s, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.LastVersionCheck != ts {
		t.Errorf("got %q, want %q", s.LastVersionCheck, ts)
	}
}

func TestVersionCheckDue(t *testing.T) {
	cases := []struct {
		name string
		s    *State
		want bool
	}{
		{"empty", &State{}, true},
		{"invalid", &State{LastVersionCheck: "not-a-date"}, true},
		{"recent", &State{LastVersionCheck: time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)}, false},
		{"stale", &State{LastVersionCheck: time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339)}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := VersionCheckDue(c.s); got != c.want {
				t.Errorf("VersionCheckDue = %v, want %v", got, c.want)
			}
		})
	}
}
