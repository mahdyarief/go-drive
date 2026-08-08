// Package crypto provides AES-256-GCM encryption for secrets at rest
// (store credentials, S3 API keys, plugin secrets).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// key returns the 32-byte AES key from SECRETS_ENCRYPTION_KEY, falling back
// to a key derived from AUTHULA_SECRET (so existing deploys work without a
// new env var). The key is fixed for the lifetime of the process.
func key() []byte {
	secret := os.Getenv("SECRETS_ENCRYPTION_KEY")
	if secret == "" {
		secret = os.Getenv("AUTHULA_SECRET")
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// Encrypt encrypts plaintext and returns a base64-encoded
// "nonce|ciphertext" string safe for storage in a text column.
func Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(key())
	if err != nil {
		return "", fmt.Errorf("crypto: creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: creating gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt reverses Encrypt.
func Decrypt(encoded string) (string, error) {
	block, err := aes.NewCipher(key())
	if err != nil {
		return "", fmt.Errorf("crypto: creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: creating gcm: %w", err)
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto: decoding ciphertext: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("crypto: ciphertext too short")
	}

	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypting: %w", err)
	}
	return string(plaintext), nil
}
