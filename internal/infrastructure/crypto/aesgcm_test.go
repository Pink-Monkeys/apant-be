package crypto

import (
	"encoding/hex"
	"testing"
)

// validKey is a deterministic 32-byte (64 hex char) key for tests.
const validKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestNewCipherRejectsBadKeys(t *testing.T) {
	cases := map[string]string{
		"empty":       "",
		"not hex":     "zzzz",
		"wrong length": "00112233",
	}
	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewCipher(key); err == nil {
				t.Fatalf("expected error for key %q, got nil", key)
			}
		})
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c, err := NewCipher(validKey)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	secret := "sk-proj-super-secret-api-key"
	sealed, err := c.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if len(sealed) == 0 {
		t.Fatal("expected non-empty ciphertext")
	}
	if string(sealed) == secret {
		t.Fatal("ciphertext must not equal plaintext")
	}

	got, err := c.Decrypt(sealed)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != secret {
		t.Fatalf("round trip mismatch: got %q want %q", got, secret)
	}
}

func TestEncryptEmptyYieldsNil(t *testing.T) {
	c, _ := NewCipher(validKey)
	sealed, err := c.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if sealed != nil {
		t.Fatalf("expected nil for empty plaintext, got %v", sealed)
	}
	got, err := c.Decrypt(nil)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestNonceIsRandomized(t *testing.T) {
	c, _ := NewCipher(validKey)
	a, _ := c.Encrypt("same-input")
	b, _ := c.Encrypt("same-input")
	if hex.EncodeToString(a) == hex.EncodeToString(b) {
		t.Fatal("two encryptions of the same plaintext must differ (random nonce)")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	c, _ := NewCipher(validKey)
	sealed, _ := c.Encrypt("secret")

	other, _ := NewCipher("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	if _, err := other.Decrypt(sealed); err == nil {
		t.Fatal("expected decryption with wrong key to fail")
	}
}
