package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"

	"smartclass/internal/platform/httpx"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not implement http.Hijacker")
	}
	if s.status == 0 {
		s.status = http.StatusSwitchingProtocols
	}
	return h.Hijack()
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func RequestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			// Install a mutable slot so an inner Authn middleware can write
			// the principal, and we read it back here for log enrichment.
			slot := &PrincipalSlot{}
			errSlot := &httpx.ErrorSlot{}
			ctx := WithPrincipalSlot(r.Context(), slot)
			ctx = httpx.WithErrorSlot(ctx, errSlot)
			next.ServeHTTP(rec, r.WithContext(ctx))
			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rec.status),
				zap.Int("bytes", rec.bytes),
				zap.Duration("took", time.Since(start)),
				zap.String("remote", r.RemoteAddr),
			}
			if id := RequestIDFrom(r.Context()); id != "" {
				fields = append(fields, zap.String("request_id", id))
			}
			if slot.Set {
				fields = append(fields,
					zap.Stringer("user_id", slot.Principal.UserID),
					zap.String("role", slot.Principal.Role))
			}
			// Emit ERROR for unhandled errors that produced a 5xx response so
			// that database failures and unexpected panics surface in logs.
			if errSlot.Err != nil {
				logger.Error("http: unhandled error", append(fields, zap.Error(errSlot.Err))...)
				return
			}
			logger.Info("http", fields...)
		})
	}
}
