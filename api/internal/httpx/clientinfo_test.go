package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"classlab/api/internal/httpx"
)

func TestWithClientInfoIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")}

	tests := []struct {
		name       string
		trusted    []netip.Prefix
		remoteAddr string
		realIP     string
		want       string
	}{
		{"no trusted proxy ignores the header", nil, "203.0.113.5:4000", "198.51.100.9", "203.0.113.5"},
		{"trusted proxy: header is used", trusted, "172.18.0.4:4000", "198.51.100.9", "198.51.100.9"},
		{"untrusted peer cannot set the header", trusted, "203.0.113.5:4000", "198.51.100.9", "203.0.113.5"},
		{"trusted proxy without a header", trusted, "172.18.0.4:4000", "", "172.18.0.4"},
		{"trusted proxy with a garbage header", trusted, "172.18.0.4:4000", "not-an-ip", "172.18.0.4"},
		{"ipv6 caller", trusted, "172.18.0.4:4000", "2001:db8::1", "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			h := httpx.WithClientInfo(tt.trusted)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = httpx.ClientInfoFrom(r.Context()).IP
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tt.want {
				t.Fatalf("IP = %q, want %q", got, tt.want)
			}
		})
	}
}
