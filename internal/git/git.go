package git

import (
	"os"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(args []string, cwd string) error
	Output(args []string, cwd string) (string, error)
}

type RealRunner struct{}

func (r *RealRunner) Run(args []string, cwd string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *RealRunner) Output(args []string, cwd string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
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
	if err := r.Run([]string{"push"}, cwd); err == nil {
		return nil
	}
	if err := r.Run([]string{"pull", "--rebase"}, cwd); err != nil {
		return err
	}
	return r.Run([]string{"push"}, cwd)
}
