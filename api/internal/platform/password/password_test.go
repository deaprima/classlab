package password_test

import (
	"strings"
	"testing"

	"classlab/api/internal/platform/password"
)

// Cheap parameters keep the suite fast; the format and comparison logic are what is under test.
func newHasher() *password.Hasher { return password.NewHasher(8, 1, 1) }

func TestHashAndVerify(t *testing.T) {
	h := newHasher()
	hash, err := h.Hash("correct horse")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=8,t=1,p=1$") {
		t.Fatalf("unexpected hash format: %q", hash)
	}

	ok, err := h.Verify("correct horse", hash)
	if err != nil || !ok {
		t.Fatalf("Verify(correct) = %v, %v; want true, nil", ok, err)
	}
	ok, err = h.Verify("wrong horse", hash)
	if err != nil || ok {
		t.Fatalf("Verify(wrong) = %v, %v; want false, nil", ok, err)
	}
}

func TestHashIsSalted(t *testing.T) {
	h := newHasher()
	a, _ := h.Hash("same")
	b, _ := h.Hash("same")
	if a == b {
		t.Fatal("two hashes of the same password are identical; salt is not random")
	}
}

func TestVerifyUsesParametersFromStoredHash(t *testing.T) {
	hash, err := password.NewHasher(8, 1, 1).Hash("pw")
	if err != nil {
		t.Fatal(err)
	}
	// A hasher configured with different costs must still verify older hashes.
	ok, err := password.NewHasher(16, 2, 2).Verify("pw", hash)
	if err != nil || !ok {
		t.Fatalf("Verify with different configured cost = %v, %v; want true, nil", ok, err)
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	h := newHasher()
	for _, bad := range []string{
		"",
		"plaintext",
		"$argon2i$v=19$m=8,t=1,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=18$m=8,t=1,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=x,t=1,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=8,t=1,p=1$!!!$aGFzaA",
		"$argon2id$v=19$m=8,t=1,p=1$c2FsdA$",
	} {
		if ok, err := h.Verify("pw", bad); ok || err == nil {
			t.Errorf("Verify(%q) = %v, %v; want false with error", bad, ok, err)
		}
	}
}
