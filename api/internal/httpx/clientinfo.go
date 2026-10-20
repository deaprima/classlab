package httpx

import (
	"context"
	"net"
	"net/http"
)

const maxUserAgentLen = 300

// ClientInfo is what the audit log records about the caller of a request (BR-25).
type ClientInfo struct {
	IP        string
	UserAgent string
}

type clientInfoKey struct{}

// WithClientInfo stores the caller's address and user agent in the request context. It uses the socket
// address on purpose: X-Forwarded-For is client-controlled. Behind a reverse proxy this needs a
// trusted-proxy setting (Phase 7).
func WithClientInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		ua := r.UserAgent()
		if len(ua) > maxUserAgentLen {
			ua = ua[:maxUserAgentLen]
		}
		ctx := context.WithValue(r.Context(), clientInfoKey{}, ClientInfo{IP: ip, UserAgent: ua})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClientInfoFrom returns the zero value for contexts that did not come from an HTTP request (jobs, tests).
func ClientInfoFrom(ctx context.Context) ClientInfo {
	ci, _ := ctx.Value(clientInfoKey{}).(ClientInfo)
	return ci
}
