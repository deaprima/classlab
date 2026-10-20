package users

import (
	"crypto/rand"
	"fmt"
)

// tempPasswordAlphabet leaves out 0, O, 1, l and I, which are easy to misread when a password is
// handed over on paper or read aloud (BR-24). 56 characters.
const tempPasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"

// NewTemporaryPassword returns n random letters and digits from crypto/rand. The value is shown once
// and only its hash is ever stored; it must never be logged or written to the audit log.
func NewTemporaryPassword(n int) (string, error) {
	if n < 1 {
		return "", fmt.Errorf("temporary password length must be positive, got %d", n)
	}
	// Bytes at or above this limit are discarded so every character is equally likely; a plain
	// modulo would favour the first 32 characters.
	const limit = 256 - 256%len(tempPasswordAlphabet)

	out := make([]byte, 0, n)
	buf := make([]byte, n*2)
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("reading random bytes: %w", err)
		}
		for _, b := range buf {
			if int(b) >= limit {
				continue
			}
			out = append(out, tempPasswordAlphabet[int(b)%len(tempPasswordAlphabet)])
			if len(out) == n {
				break
			}
		}
	}
	return string(out), nil
}
