package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type requestIDKey struct{}

const RequestIDHeader = "X-Request-Id"

// RequestID populates a request-scoped correlation ID. If the caller already
// supplied X-Request-Id (e.g., behind a gateway that injects one), we trust it;
// otherwise we generate a fresh UUID. The same ID is echoed back in the
// response header so clients can attach it to bug reports.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" || len(id) > 128 {
			id = uuid.NewString()
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom returns the request-scoped correlation ID, or "" if none.
func RequestIDFrom(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}
