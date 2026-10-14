package auth

import (
	"testing"
	"time"

	"classlab/api/internal/platform/clock"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const testSecret = "0123456789abcdef0123456789abcdef"

var epoch = time.Date(2026, 10, 14, 3, 0, 0, 0, time.UTC)

func TestTokenRoundTrip(t *testing.T) {
	clk := clock.NewMockClock(epoch)
	issuer := NewTokenIssuer(testSecret, 15*time.Minute, clk)
	id := uuid.New()

	tok, err := issuer.Issue(id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := issuer.Verify(tok)
	if err != nil || got != id {
		t.Fatalf("Verify = %v, %v; want %v, nil", got, err, id)
	}
}

func TestTokenExpiry(t *testing.T) {
	clk := clock.NewMockClock(epoch)
	issuer := NewTokenIssuer(testSecret, 15*time.Minute, clk)
	tok, _ := issuer.Issue(uuid.New())

	clk.Add(15*time.Minute - time.Second)
	if _, err := issuer.Verify(tok); err != nil {
		t.Fatalf("token should still be valid 1s before expiry: %v", err)
	}
	clk.Add(time.Second)
	if _, err := issuer.Verify(tok); err == nil {
		t.Fatal("token should be rejected at expiry")
	}
}

func TestTokenRejectsTamperedOrForeign(t *testing.T) {
	clk := clock.NewMockClock(epoch)
	issuer := NewTokenIssuer(testSecret, 15*time.Minute, clk)

	other := NewTokenIssuer("ffffffffffffffffffffffffffffffff", 15*time.Minute, clk)
	foreign, _ := other.Issue(uuid.New())

	claims := jwt.RegisteredClaims{
		Issuer: "someone-else", Subject: uuid.NewString(),
		ExpiresAt: jwt.NewNumericDate(epoch.Add(time.Hour)),
	}
	wrongIssuer, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))

	claims.Issuer = jwtIssuer
	claims.ExpiresAt = nil
	noExpiry, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))

	claims.ExpiresAt = jwt.NewNumericDate(epoch.Add(time.Hour))
	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte(testSecret))

	// "alg: none" is the classic bypass; the parser must not accept it.
	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)

	for name, tok := range map[string]string{
		"empty":         "",
		"garbage":       "not-a-jwt",
		"wrong secret":  foreign,
		"wrong issuer":  wrongIssuer,
		"no expiry":     noExpiry,
		"other alg":     hs512,
		"alg none":      none,
		"bad subject":   mustSign(t, jwt.RegisteredClaims{Issuer: jwtIssuer, Subject: "not-a-uuid", ExpiresAt: jwt.NewNumericDate(epoch.Add(time.Hour))}),
		"issued future": mustSign(t, jwt.RegisteredClaims{Issuer: jwtIssuer, Subject: uuid.NewString(), IssuedAt: jwt.NewNumericDate(epoch.Add(time.Hour)), ExpiresAt: jwt.NewNumericDate(epoch.Add(2 * time.Hour))}),
	} {
		if _, err := issuer.Verify(tok); err == nil {
			t.Errorf("%s: token was accepted", name)
		}
	}
}

func mustSign(t *testing.T, c jwt.RegisteredClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRefreshTokensAreUniqueAndOnlyHashIsDerivable(t *testing.T) {
	rawA, hashA, err := newRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	rawB, hashB, _ := newRefreshToken()
	if rawA == rawB || hashA == hashB {
		t.Fatal("refresh tokens must be unique")
	}
	if hashA == rawA || hashRefreshToken(rawA) != hashA {
		t.Fatal("stored value must be a deterministic hash of the raw token, not the token itself")
	}
}
