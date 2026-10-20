package users

import (
	"strings"
	"testing"
)

func TestTemporaryPasswordHasRequestedLengthAndAlphabet(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		p, err := NewTemporaryPassword(10)
		if err != nil {
			t.Fatal(err)
		}
		if len(p) != 10 {
			t.Fatalf("length = %d, want 10", len(p))
		}
		for _, r := range p {
			if !strings.ContainsRune(tempPasswordAlphabet, r) {
				t.Fatalf("%q contains %q, which is not in the alphabet", p, r)
			}
		}
		seen[p] = true
	}
	// 56^10 possibilities: a repeat in 200 draws means the source is not random.
	if len(seen) != 200 {
		t.Errorf("only %d distinct passwords in 200 draws", len(seen))
	}
}

func TestTemporaryPasswordAvoidsAmbiguousCharacters(t *testing.T) {
	if strings.ContainsAny(tempPasswordAlphabet, "0O1lI") {
		t.Errorf("alphabet %q contains an ambiguous character", tempPasswordAlphabet)
	}
}

func TestTemporaryPasswordRejectsNonPositiveLength(t *testing.T) {
	for _, n := range []int{0, -1} {
		if _, err := NewTemporaryPassword(n); err == nil {
			t.Errorf("length %d accepted", n)
		}
	}
}
