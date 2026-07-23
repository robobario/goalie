package crypto

import (
	"encoding/base64"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("hello, goalie")

	encrypted, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatal(err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptCorruptedCiphertext(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := Encrypt(key, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.StdEncoding.DecodeString(string(encrypted))
	if err != nil {
		t.Fatal(err)
	}

	// flip a byte after the 12-byte nonce
	raw[12] ^= 0xff
	corrupted := base64.StdEncoding.EncodeToString(raw)

	_, err = Decrypt(key, []byte(corrupted))
	if err == nil {
		t.Fatal("expected error from corrupted ciphertext")
	}
}

func TestEncryptNilKeyPassthrough(t *testing.T) {
	plaintext := []byte("hello, goalie")
	out, err := Encrypt(nil, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(plaintext) {
		t.Fatalf("got %q, want %q", out, plaintext)
	}
}

func TestDecryptNilKeyPassthrough(t *testing.T) {
	data := []byte(`{"note":"hello"}`)
	out, err := Decrypt(nil, data)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(data) {
		t.Fatalf("got %q, want %q", out, data)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := Encrypt(key, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	wrongKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	_, err = Decrypt(wrongKey, encrypted)
	if err == nil {
		t.Fatal("expected error from wrong key")
	}
}
