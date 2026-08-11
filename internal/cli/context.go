package cli

import (
	"io"

	"goalie/internal/config"
	"goalie/internal/git"
)

type AppContext struct {
	DataDir       string
	Git           git.Runner
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	IsTTY         bool
	Username      string
	EncryptionKey []byte
	SchemaVersion string

	// Derived from config at startup; zero means use package default.
	Config     *config.Config
	HyperLinks bool
	WrapWidth  int
	StatusDays int
}

func (ctx AppContext) EffectiveWrapWidth() int {
	if ctx.WrapWidth == 0 {
		return config.DefaultWrapWidth
	}
	return ctx.WrapWidth
}

func (ctx AppContext) EffectiveStatusDays() int {
	if ctx.StatusDays == 0 {
		return config.DefaultStatusDays
	}
	return ctx.StatusDays
}
