package cli

import "fmt"

// ExitError signals that the command should terminate with a specific exit code.
// The error message has already been written to Stderr by the command function.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
