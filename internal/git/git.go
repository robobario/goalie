package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Runner interface {
	Run(args []string, cwd string) error
	Output(args []string, cwd string) (string, error)
}

type RealRunner struct{}

// noPromptEnv disables every interactive credential path git might try:
// terminal prompts, GUI askpass helpers, and SSH passphrase/host-key prompts.
// A user with misconfigured or missing credentials must see a fast error,
// never a hang or an unexpected GUI popup.
func noPromptEnv() []string {
	return append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new",
	)
}

func (r *RealRunner) Run(args []string, cwd string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	cmd.Env = noPromptEnv()
	var buf bytes.Buffer
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		if out := strings.TrimSpace(buf.String()); out != "" {
			return fmt.Errorf("%w\n%s", err, out)
		}
		return err
	}
	return nil
}

func (r *RealRunner) Output(args []string, cwd string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	cmd.Env = noPromptEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%w\n%s", err, msg)
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type FakeRunner struct {
	Calls   [][]string
	Errors  map[string][]error
	Outputs map[string][]string
}

func (f *FakeRunner) popError(key string) error {
	errs := f.Errors[key]
	if len(errs) == 0 {
		return nil
	}
	err := errs[0]
	f.Errors[key] = errs[1:]
	return err
}

func (f *FakeRunner) popOutput(key string) string {
	vals := f.Outputs[key]
	if len(vals) == 0 {
		return ""
	}
	val := vals[0]
	f.Outputs[key] = vals[1:]
	return val
}

func (f *FakeRunner) Run(args []string, cwd string) error {
	f.Calls = append(f.Calls, args)
	if len(args) == 0 {
		return nil
	}
	return f.popError(args[0])
}

func (f *FakeRunner) Output(args []string, cwd string) (string, error) {
	f.Calls = append(f.Calls, args)
	if len(args) == 0 {
		return "", nil
	}
	return f.popOutput(args[0]), f.popError(args[0])
}

// Push attempts git push in cwd. On failure, it pulls with rebase then retries.
// Returns an error only if the retry also fails.
func Push(r Runner, cwd string) error {
	return PushArgs(r, cwd, []string{"push"})
}

// PushArgs runs the given push command in cwd. On failure, it pulls with
// rebase then retries the same push command. Returns an error only if the
// retry also fails.
func PushArgs(r Runner, cwd string, args []string) error {
	if err := r.Run(args, cwd); err == nil {
		return nil
	}
	if err := r.Run([]string{"pull", "--rebase"}, cwd); err != nil {
		return err
	}
	return r.Run(args, cwd)
}

// VerifyWriteAccess proves the caller can push to an already-tracked branch
// (e.g. right after a clone) before trusting cwd as usable. It writes a
// uniquely-named throwaway file, commits and pushes it, then removes it with
// a second commit and push. Do not use this on a branch that does not yet
// exist on the remote: unlike a brand-new branch's first push, a failure
// partway through here would leave the file committed-but-unpushed at worst,
// never a remote branch with no real content. The caller is responsible for
// cleaning up cwd if this returns an error.
func VerifyWriteAccess(r Runner, cwd string) error {
	name := ".goalie-write-check-" + uuid.New().String()
	path := filepath.Join(cwd, name)
	if err := os.WriteFile(path, nil, 0644); err != nil {
		return err
	}
	if err := r.Run([]string{"add", name}, cwd); err != nil {
		return err
	}
	if err := r.Run([]string{"commit", "-m", "chore: verify write access"}, cwd); err != nil {
		return err
	}
	if err := Push(r, cwd); err != nil {
		return err
	}
	if err := r.Run([]string{"rm", name}, cwd); err != nil {
		return err
	}
	if err := r.Run([]string{"commit", "-m", "chore: remove write access check"}, cwd); err != nil {
		return err
	}
	return Push(r, cwd)
}

// PushNewBranch pushes a freshly created local branch to origin for the
// first time, establishing upstream tracking as it goes. Unlike Push, its
// retry pulls explicitly from origin/branch: no tracking config exists until
// this call succeeds, so a bare `git pull --rebase` cannot work if the first
// attempt fails (e.g. a race where another user just created the branch).
func PushNewBranch(r Runner, cwd, branch string) error {
	args := []string{"push", "--set-upstream", "origin", branch}
	if err := r.Run(args, cwd); err == nil {
		return nil
	}
	if err := r.Run([]string{"pull", "--rebase", "origin", branch}, cwd); err != nil {
		return err
	}
	return r.Run(args, cwd)
}
