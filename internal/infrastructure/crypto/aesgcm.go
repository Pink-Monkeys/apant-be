// Package crypto provides symmetric encryption for secrets stored at rest,
// notably LLM provider API keys persisted in the database.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// Cipher encrypts and decrypts secrets with AES-256-GCM. The 12-byte random
// nonce is prepended to each ciphertext so decryption is self-contained; the
// same key must be used for both operations.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher builds a Cipher from a 32-byte key encoded as a 64-char hex string
// (AES-256). It fails fast on a malformed or wrong-length key so a
// misconfigured LLM_ENCRYPTION_KEY is caught at boot rather than at first use.
func NewCipher(hexKey string) (*Cipher, error) {
	hexKey = strings.TrimSpace(hexKey)
	if hexKey == "" {
		return nil, fmt.Errorf("encryption key is empty")
	}

	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("encryption key must be hex-encoded: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init aes cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init gcm: %w", err)
	}

	return &Cipher{aead: aead}, nil
}

// Encrypt seals plaintext and returns nonce||ciphertext. An empty plaintext
// yields a nil slice so callers can persist "no key set" as NULL/empty.
func (c *Cipher) Encrypt(plaintext string) ([]byte, error) {
	if plaintext == "" {
		return nil, nil
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Seal appends the ciphertext to nonce, so the returned slice is
	// nonce||ciphertext||tag in one contiguous buffer.
	return c.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt reverses Encrypt. An empty input yields an empty string so a stored
// "no key" round-trips cleanly.
func (c *Cipher) Decrypt(sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}

	nonceSize := c.aead.NonceSize()
	if len(sealed) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := sealed[:nonceSize], sealed[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
