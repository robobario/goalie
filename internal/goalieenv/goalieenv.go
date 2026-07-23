package goalieenv

import (
	"os"
	"path/filepath"
)

// Home returns the goalie home directory. If the GOALIE_HOME environment
// variable is set it is used directly; otherwise the default ~/.goalie is
// returned.
func Home() (string, error) {
	if h := os.Getenv("GOALIE_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".goalie"), nil
}
