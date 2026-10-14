// Package password hashes and verifies passwords with argon2id in the PHC string format.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen = 16
	keyLen  = 32
)

var ErrInvalidHash = errors.New("invalid password hash format")

type Hasher struct {
	memoryKiB   uint32
	iterations  uint32
	parallelism uint8
}

func NewHasher(memoryKiB, iterations uint32, parallelism uint8) *Hasher {
	return &Hasher{memoryKiB: memoryKiB, iterations: iterations, parallelism: parallelism}
}

func (h *Hasher) Hash(plain string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, h.iterations, h.memoryKiB, h.parallelism, keyLen)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.memoryKiB, h.iterations, h.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify reads the cost parameters from the stored hash, so changing the config never locks anyone out.
func (h *Hasher) Verify(plain, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrInvalidHash
	}

	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, ErrInvalidHash
	}

	got := argon2.IDKey([]byte(plain), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// BurnVerify spends the time of one verification without any account behind it, so a login for an
// unknown identifier takes as long as one for a real account.
func (h *Hasher) BurnVerify(plain string) {
	argon2.IDKey([]byte(plain), make([]byte, saltLen), h.iterations, h.memoryKiB, h.parallelism, keyLen)
}
