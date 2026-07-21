package cli

import (
	"encoding/hex"
	"fmt"

	"goalie/internal/crypto"
)

func KeyInit(ctx AppContext) error {
	key, err := crypto.GenerateKey()
	if err != nil {
		return err
	}
	if err := crypto.SaveKey(key); err != nil {
		return err
	}
	fmt.Fprintln(ctx.Stdout, hex.EncodeToString(key))
	return nil
}

func KeyImport(ctx AppContext, hexKey string) error {
	decoded, err := hex.DecodeString(hexKey)
	if err != nil || len(decoded) != 32 {
		fmt.Fprintln(ctx.Stderr, "invalid key: must be exactly 64 hex characters (32 bytes)")
		return &ExitError{Code: 1}
	}
	return crypto.SaveKey(decoded)
}
