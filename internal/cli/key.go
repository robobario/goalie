package cli

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"

	"goalie/internal/crypto"
)

const overwriteWarning = "You already have an encryption key. Replacing it may prevent you from reading or writing existing team data. Continue? (y/n) "

func KeyInit(ctx AppContext) error {
	if keyFileExists() {
		ok, err := ynPrompt(overwriteWarning, bufio.NewReader(ctx.Stdin), ctx.Stdout, ctx.IsTTY)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
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
	if keyFileExists() {
		ok, err := ynPrompt(overwriteWarning, bufio.NewReader(ctx.Stdin), ctx.Stdout, ctx.IsTTY)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	return crypto.SaveKey(decoded)
}

func keyFileExists() bool {
	path, err := crypto.DefaultKeyPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
