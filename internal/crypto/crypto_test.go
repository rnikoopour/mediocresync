package crypto

import (
	"bytes"
	"testing"
)

var testKey = bytes.Repeat([]byte{0x01}, 32)

func TestRoundTrip(t *testing.T) {
	plaintext := "super-secret-password"

	ciphertext, err := Encrypt(testKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := Decrypt(testKey, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if got != plaintext {
		t.Errorf("got %q, want %q", got, plaintext)
	}
}

func TestEncryptProducesUniqueOutput(t *testing.T) {
	a, _ := Encrypt(testKey, "same")
	b, _ := Encrypt(testKey, "same")
	if bytes.Equal(a, b) {
		t.Error("two encryptions of the same plaintext should differ (random nonce)")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	ciphertext, _ := Encrypt(testKey, "secret")

	wrongKey := bytes.Repeat([]byte{0x02}, 32)
	_, err := Decrypt(wrongKey, ciphertext)
	if err == nil {
		t.Error("expected error decrypting with wrong key, got nil")
	}
}

func TestDecryptTooShort(t *testing.T) {
	_, err := Decrypt(testKey, []byte("short"))
	if err == nil {
		t.Error("expected error for ciphertext shorter than nonce size")
	}
}

func TestEncryptEmptyString(t *testing.T) {
	ciphertext, err := Encrypt(testKey, "")
	if err != nil {
		t.Fatalf("Encrypt empty string: %v", err)
	}
	got, err := Decrypt(testKey, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt empty string: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}
