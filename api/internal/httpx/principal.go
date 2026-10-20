package httpx

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// Role mirrors the user_role database enum.
type Role string

const (
	RoleMahasiswa  Role = "MAHASISWA"
	RoleOrganisasi Role = "ORGANISASI"
	RoleTU         Role = "TU"
	RoleKalab      Role = "KALAB"
	RoleOB         Role = "OB"
	RoleAdmin      Role = "ADMIN"
)

func (r Role) Valid() bool {
	switch r {
	case RoleMahasiswa, RoleOrganisasi, RoleTU, RoleKalab, RoleOB, RoleAdmin:
		return true
	}
	return false
}

// Principal is the authenticated caller, resolved fresh from the database on every request.
type Principal struct {
	UserID uuid.UUID
	Role   Role
	// MustChangePassword is true while the account still holds a temporary password (BR-24).
	MustChangePassword bool
}

func (p Principal) HasRole(roles ...Role) bool {
	return slices.Contains(roles, p.Role)
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// Authenticator turns a bearer token into a Principal; any failure must wrap ErrUnauthenticated.
type Authenticator interface {
	Authenticate(ctx context.Context, bearerToken string) (Principal, error)
}

// Authenticate rejects requests without a valid bearer token and stores the Principal in the context.
func Authenticate(a Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
			token = strings.TrimSpace(token)
			if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
				w.Header().Set("WWW-Authenticate", "Bearer")
				WriteError(w, r, ErrUnauthenticated)
				return
			}

			p, err := a.Authenticate(r.Context(), token)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				WriteError(w, r, err)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
		})
	}
}

// RequireRoles must be mounted after Authenticate. Services re-check ownership and roles themselves.
func RequireRoles(roles ...Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFrom(r.Context())
			if !ok {
				WriteError(w, r, ErrUnauthenticated)
				return
			}
			if !p.HasRole(roles...) {
				WriteError(w, r, ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePasswordChanged refuses callers who still hold a temporary password (BR-24). Mount it after
// Authenticate on every route except /auth/me, /auth/change-password and /auth/logout.
func RequirePasswordChanged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := PrincipalFrom(r.Context()); ok && p.MustChangePassword {
			WriteError(w, r, ErrPasswordChangeRequired)
			return
		}
		next.ServeHTTP(w, r)
	})
}
