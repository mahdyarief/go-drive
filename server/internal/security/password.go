// Package security provides link-password hashing and link-token generation.
package security

import (
	"crypto/rand"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// HashLinkPassword returns a bcrypt hash of pw, or "" when pw is empty.
func HashLinkPassword(pw string) (string, error) {
	if pw == "" {
		return "", nil
	}
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyLinkPassword reports whether pw matches the stored bcrypt hash.
// An empty hash means no password is set, so the check always passes.
func VerifyLinkPassword(hash, pw string) bool {
	if hash == "" {
		return true
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

const tokenAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Token32 returns a 32-character URL-safe random token (share/upload links).
// Rejection sampling avoids modulo bias on the alphabet.
func Token32() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	maxByte := byte(256 - (256 % len(tokenAlphabet))) // 248
	for i := range b {
		v := b[i]
		for v >= maxByte {
			if _, err := rand.Read(b[i : i+1]); err != nil {
				return "", err
			}
			v = b[i]
		}
		b[i] = tokenAlphabet[int(v)%len(tokenAlphabet)]
	}
	return string(b), nil
}

// TokenHex16 returns a 32-char hex token (16 random bytes) for tracked links.
func TokenHex16() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
