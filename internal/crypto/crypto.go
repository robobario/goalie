package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"goalie/internal/goalieenv"
)

var errCiphertextTooShort = errors.New("ciphertext too short")

func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

func Encrypt(key, plaintext []byte) ([]byte, error) {
	if key == nil {
		return plaintext, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return []byte(encoded), nil
}

func Decrypt(key, ciphertext []byte) ([]byte, error) {
	if key == nil {
		return ciphertext, nil
	}
	raw, err := base64.StdEncoding.DecodeString(string(ciphertext))
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, errCiphertextTooShort
	}

	nonce, raw := raw[:nonceSize], raw[nonceSize:]
	return gcm.Open(nil, nonce, raw, nil)
}

const keyCheckSentinel = "goalie-key-check-v1"

// WriteKeyCheck encrypts the sentinel and writes it to path.
func WriteKeyCheck(path string, key []byte) error {
	ciphertext, err := Encrypt(key, []byte(keyCheckSentinel))
	if err != nil {
		return err
	}
	return os.WriteFile(path, ciphertext, 0644)
}

// VerifyKeyCheck reads the check file at path and attempts to decrypt it with key.
// Returns (true, nil) if the key is correct or the file does not exist.
// Returns (false, nil) if the key is wrong.
func VerifyKeyCheck(path string, key []byte) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	plaintext, err := Decrypt(key, data)
	if err != nil {
		return false, nil
	}
	return string(plaintext) == keyCheckSentinel, nil
}

func DefaultKeyPath() (string, error) {
	home, err := goalieenv.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "encryption.key"), nil
}

func LoadKey() ([]byte, error) {
	path, err := DefaultKeyPath()
	if err != nil {
		return nil, err
	}
	return LoadKeyFrom(path)
}

func LoadKeyFrom(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(strings.TrimSpace(string(data)))
}

func SaveKey(key []byte) error {
	path, err := DefaultKeyPath()
	if err != nil {
		return err
	}
	return SaveKeyTo(path, key)
}

func SaveKeyTo(path string, key []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	encoded := hex.EncodeToString(key)
	return os.WriteFile(path, []byte(encoded), 0600)
}
