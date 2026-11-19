package httpx

import (
	"mime"
	"net/http"
	"strconv"
	"time"
)

// attemptLimiter is what LimitUploads needs from a rate limiter (platform/ratelimit.Limiter).
type attemptLimiter interface {
	Allow(key string) (retryAfter time.Duration, ok bool)
}

// UploadLimitOption adjusts LimitUploads.
type UploadLimitOption func(*uploadLimit)

type uploadLimit struct{ skipDryRun bool }

// SkipDryRun leaves requests with ?dry_run=true uncounted. Only for routes that really treat that flag as
// a preview that changes nothing and stores nothing (the CSV imports); on any other route it would let a
// caller dodge the limit by adding the parameter.
func SkipDryRun() UploadLimitOption { return func(o *uploadLimit) { o.skipDryRun = true } }

// LimitUploads caps how often one signed-in user may send a multipart/form-data POST, which is how every
// file goes in: letters, report and check-out photos, CSV imports. It sits behind Authenticate and keys on
// the user, so a shared campus address does not make students wait for each other. A request over the
// limit is answered 429 before the body is read. Ordinary JSON requests are not counted.
func LimitUploads(l attemptLimiter, opts ...UploadLimitOption) func(http.Handler) http.Handler {
	var o uploadLimit
	for _, opt := range opts {
		opt(&o)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || !isMultipart(r) || (o.skipDryRun && r.URL.Query().Get("dry_run") == "true") {
				next.ServeHTTP(w, r)
				return
			}
			// Without a principal (the middleware was placed before Authenticate) fall back to the address,
			// so a wiring mistake limits too much rather than nothing.
			key := "ip|" + ClientInfoFrom(r.Context()).IP
			if p, ok := PrincipalFrom(r.Context()); ok {
				key = "user|" + p.UserID.String()
			}
			if retryAfter, ok := l.Allow(key); !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
				WriteError(w, r, NewCustomError(http.StatusTooManyRequests, "RATE_LIMITED",
					"Terlalu banyak unggahan dalam waktu singkat. Tunggu sebentar, lalu coba lagi.", nil, ErrRateLimited))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isMultipart(r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mt == "multipart/form-data"
}
