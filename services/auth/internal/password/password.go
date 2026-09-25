// Package password provides POC-grade password hashing primitives.
package password

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// Hash stretches salt+password with an iterative SHA-256 loop. This is
// intentionally simple for the POC; replace with bcrypt/argon2id before
// production use.
func Hash(password, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt + password))
	sum := h.Sum(nil)
	for i := 0; i < 9999; i++ {
		h.Reset()
		h.Write(sum)
		sum = h.Sum(nil)
	}
	return hex.EncodeToString(sum)
}

func NewSalt() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
