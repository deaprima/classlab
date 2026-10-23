package auth

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/users"

	"github.com/go-chi/chi/v5"
)

// RefreshCookieName is scoped to /api/v1/auth, so the browser never sends it to other endpoints.
const (
	RefreshCookieName = "refresh_token"
	refreshCookiePath = "/api/v1/auth"
)

type Handler struct {
	svc          *Service
	cookieSecure bool
}

func NewHandler(svc *Service, cookieSecure bool) *Handler {
	return &Handler{svc: svc, cookieSecure: cookieSecure}
}

// Routes mounts the public auth endpoints and, behind authenticate, the ones that need a session.
func (h *Handler) Routes(authenticate func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Post("/login", h.login)
	r.Post("/refresh", h.refresh)
	r.Group(func(r chi.Router) {
		r.Use(authenticate)
		// These three stay reachable while a temporary password is pending (BR-24); every other route in
		// the API sits behind httpx.RequirePasswordChanged.
		r.Post("/logout", h.logout)
		r.Get("/me", h.me)
		r.Post("/change-password", h.changePassword)
	})
	return r
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type loginResponse struct {
	AccessToken        string     `json:"access_token"`
	ExpiresIn          int        `json:"expires_in"`
	User               users.User `json:"user"`
	MustChangePassword bool       `json:"must_change_password"`
}

type refreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	sess, err := h.svc.Login(r.Context(), req.Identifier, req.Password, clientIP(r))
	if err != nil {
		var limited *RateLimitedError
		if errors.As(err, &limited) {
			w.Header().Set("Retry-After", strconv.Itoa(int(limited.RetryAfter.Seconds())+1))
		}
		httpx.WriteError(w, r, err)
		return
	}

	h.setRefreshCookie(w, sess.RefreshToken, sess.RefreshExpiresAt)
	httpx.JSON(w, http.StatusOK, loginResponse{
		AccessToken: sess.AccessToken, ExpiresIn: sess.ExpiresIn, User: *sess.User, MustChangePassword: sess.User.MustChangePassword,
	})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var raw string
	if c, err := r.Cookie(RefreshCookieName); err == nil {
		raw = c.Value
	}

	sess, err := h.svc.Refresh(r.Context(), raw)
	if err != nil {
		// Drop the cookie on failure so a client holding a dead token stops sending it.
		h.clearRefreshCookie(w)
		httpx.WriteError(w, r, err)
		return
	}

	h.setRefreshCookie(w, sess.RefreshToken, sess.RefreshExpiresAt)
	httpx.JSON(w, http.StatusOK, refreshResponse{AccessToken: sess.AccessToken, ExpiresIn: sess.ExpiresIn})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	var raw string
	if c, err := r.Cookie(RefreshCookieName); err == nil {
		raw = c.Value
	}
	if err := h.svc.Logout(r.Context(), actor, raw); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	u, err := h.svc.Me(r.Context(), actor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

func (h *Handler) setRefreshCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    value,
		Path:     refreshCookiePath,
		Expires:  expires,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// clientIP is the address httpx.WithClientInfo resolved: the socket peer, or the proxy-reported one when
// the peer is a trusted proxy. A client-supplied header alone never counts, or an attacker could dodge the
// login limiter by rotating it.
func clientIP(r *http.Request) string {
	if ip := httpx.ClientInfoFrom(r.Context()).IP; ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	var req changePasswordRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// The refresh cookie tells the service which session to keep alive.
	var raw string
	if c, err := r.Cookie(RefreshCookieName); err == nil {
		raw = c.Value
	}
	if err := h.svc.ChangePassword(r.Context(), actor, raw, req.CurrentPassword, req.NewPassword); err != nil {
		var limited *RateLimitedError
		if errors.As(err, &limited) {
			w.Header().Set("Retry-After", strconv.Itoa(int(limited.RetryAfter.Seconds())+1))
		}
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
