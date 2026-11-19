package httpx

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

const maxUserAgentLen = 300

// realIPHeader is set by the reverse proxy to the address it saw the caller connect from. The proxy
// overwrites whatever the caller sent, so it is only believed when the socket peer is a trusted proxy.
const realIPHeader = "X-Real-IP"

// ClientInfo is what the audit log records about the caller of a request (BR-25).
type ClientInfo struct {
	IP        string
	UserAgent string
}

type clientInfoKey struct{}

// WithClientInfo stores the caller's address and user agent in the request context. The address is the
// socket peer, unless that peer is inside one of the trusted proxy ranges: then X-Real-IP is used,
// because behind the proxy every socket peer is the proxy itself and the login limiter would share one
// bucket among all users. With no trusted ranges the header is ignored, as X-Forwarded-For and X-Real-IP
// are client-controlled and would let an attacker dodge the limiter by rotating them.
func WithClientInfo(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ua := r.UserAgent()
			if len(ua) > maxUserAgentLen {
				ua = ua[:maxUserAgentLen]
			}
			ci := ClientInfo{IP: callerIP(r, trusted), UserAgent: ua}
			ctx := context.WithValue(r.Context(), clientInfoKey{}, ci)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func callerIP(r *http.Request, trusted []netip.Prefix) string {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	if len(trusted) == 0 {
		return peer
	}
	peerAddr, err := netip.ParseAddr(peer)
	if err != nil || !inAny(trusted, peerAddr.Unmap()) {
		return peer
	}
	forwarded, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get(realIPHeader)))
	if err != nil {
		return peer
	}
	return forwarded.Unmap().String()
}

func inAny(prefixes []netip.Prefix, a netip.Addr) bool {
	for _, p := range prefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// ClientInfoFrom returns the zero value for contexts that did not come from an HTTP request (jobs, tests).
func ClientInfoFrom(ctx context.Context) ClientInfo {
	ci, _ := ctx.Value(clientInfoKey{}).(ClientInfo)
	return ci
}
