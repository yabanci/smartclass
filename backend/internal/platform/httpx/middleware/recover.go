package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"go.uber.org/zap"

	"smartclass/internal/platform/httpx"
)

func Recoverer(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					// Populate the ErrorSlot so RequestLogger (which runs as the
					// outer middleware) emits a single ERROR log line that includes
					// request_id, method, path, and the recovered panic value.
					// Recoverer itself does NOT log here to avoid duplicate lines.
					if slot := httpx.ErrorSlotFrom(r.Context()); slot != nil {
						slot.Err = fmt.Errorf("panic: %v\n%s", rec, debug.Stack())
					} else {
						// Fallback: RequestLogger is not in the chain (e.g. tests
						// that use Recoverer standalone); log from here instead.
						logger.Error("panic recovered",
							zap.Any("panic", rec),
							zap.String("path", r.URL.Path),
							zap.ByteString("stack", debug.Stack()),
						)
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"Internal server error"}}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
