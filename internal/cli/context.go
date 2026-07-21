package cli

import (
	"io"

	"goalie/internal/git"
)

type AppContext struct {
	DataDir       string
	Git           git.Runner
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	IsTTY         bool
	Username      string // if non-empty, skip config.Load
	EncryptionKey []byte
}
