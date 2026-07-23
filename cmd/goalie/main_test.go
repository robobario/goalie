package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	dir := t.TempDir()
	bin := dir + "/goalie"

	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "--version")
	cmd.Env = append(os.Environ(), "HOME="+dir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !strings.Contains(string(out), "dev") {
		t.Errorf("expected output to contain %q, got: %s", "dev", out)
	}
}
