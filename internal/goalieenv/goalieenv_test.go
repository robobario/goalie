package goalieenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHome_usesGOALIE_HOME(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GOALIE_HOME", dir)

	got, err := Home()
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestHome_defaultsToUserHomeDir(t *testing.T) {
	t.Setenv("GOALIE_HOME", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no user home dir available")
	}
	want := filepath.Join(home, ".goalie")

	got, err := Home()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
