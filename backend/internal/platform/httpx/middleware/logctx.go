package middleware

import (
	"context"

	"go.uber.org/zap"
)

// LoggerFromCtx returns base enriched with the request_id from ctx.
// If no request_id is present (e.g., background goroutines), base is returned
// unchanged. Use this in service-layer WARN/ERROR calls so every log line
// that originates from a request carries the same correlation ID as the HTTP
// access log.
func LoggerFromCtx(ctx context.Context, base *zap.Logger) *zap.Logger {
	if id := RequestIDFrom(ctx); id != "" {
		return base.With(zap.String("request_id", id))
	}
	return base
}
