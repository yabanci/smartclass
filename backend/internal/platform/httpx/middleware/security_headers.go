package middleware

import "net/http"

// SecurityHeaders sets defensive HTTP response headers on every response.
// The HSTS header is set unconditionally; TLS termination is expected at the
// reverse-proxy layer, and setting it over HTTP is harmless but has no effect.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			next.ServeHTTP(w, r)
		})
	}
}
