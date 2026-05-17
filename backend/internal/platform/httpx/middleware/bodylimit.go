package middleware

import "net/http"

// MaxBodyBytes is the default upper bound for request bodies. Most endpoints
// accept JSON DTOs in the 1–4 KB range; 1 MiB is generous enough for the
// largest payload we issue (HA flow step with embedded credentials), without
// letting a client tie up server memory by sending megabytes of garbage.
const MaxBodyBytes int64 = 1 << 20 // 1 MiB

// BodyLimit caps the request body to limit bytes. Subsequent handlers reading
// r.Body (DecodeJSON, io.ReadAll) will receive an "http: request body too
// large" error after the limit is hit and Go's MaxBytesReader will respond
// with 413 if the limit is exceeded before any body read.
func BodyLimit(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
