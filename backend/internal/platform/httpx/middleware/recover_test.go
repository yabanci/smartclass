package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"smartclass/internal/platform/httpx"
)

// TestRecoverer_PanicProducesErrorLogWithRequestID verifies V-3:
// a panicking handler must yield a single ERROR log line that contains both
// the request_id and the recovered panic value. The test wires the chain in
// the same order used by server.go: RequestID → RequestLogger → Recoverer →
// handler, so the ErrorSlot is installed in context before Recoverer fires.
func TestRecoverer_PanicProducesErrorLogWithRequestID(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)

	// Build chain: RequestID → RequestLogger → Recoverer → panicking handler.
	// This mirrors the order in server.go (RequestLogger is outer, Recoverer inner).
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		panic("test-panic-value")
	})
	chain := RequestID(RequestLogger(logger)(Recoverer(logger)(panicHandler)))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	// HTTP response must be 500.
	require.Equal(t, http.StatusInternalServerError, rr.Code)

	// Exactly one ERROR log line (from RequestLogger reading the ErrorSlot).
	// Recoverer must NOT emit a separate log when the slot is populated.
	require.Equal(t, 1, logs.Len(), "exactly one ERROR log line expected — no duplicate from Recoverer")

	entry := logs.All()[0]
	assert.Equal(t, zapcore.ErrorLevel, entry.Level)

	// Must carry request_id (added by RequestID middleware).
	var hasRequestID bool
	var errMsg string
	for _, f := range entry.Context {
		if f.Key == "request_id" && f.String != "" {
			hasRequestID = true
		}
		if f.Key == "error" {
			errMsg = f.Interface.(error).Error()
		}
	}
	assert.True(t, hasRequestID, "ERROR log must contain request_id for correlation")
	assert.True(t, strings.Contains(errMsg, "test-panic-value"),
		"ERROR log must contain the recovered panic value, got: %s", errMsg)
}

// TestRecoverer_StandaloneLogsWhenNoSlot verifies that Recoverer still logs
// (via its own logger fallback) when used without RequestLogger in the chain.
func TestRecoverer_StandaloneLogsWhenNoSlot(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		panic("standalone-panic")
	})
	// No RequestLogger wrapping — ErrorSlot will be nil.
	chain := Recoverer(logger)(panicHandler)

	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, 1, logs.Len(), "standalone Recoverer must still emit an ERROR log")
	entry := logs.All()[0]
	assert.Equal(t, zapcore.ErrorLevel, entry.Level)

	var found bool
	for _, f := range entry.Context {
		if f.Key == "panic" {
			found = true
			break
		}
	}
	assert.True(t, found, "standalone log must carry the panic field")
}

// TestRecoverer_NoErrorSlotPopulatedOnSuccess verifies no ErrorSlot side-effects
// for normal (non-panicking) requests.
func TestRecoverer_NoErrorSlotPopulatedOnSuccess(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	logger := zap.New(core)

	var capturedSlot *httpx.ErrorSlot
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSlot = httpx.ErrorSlotFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	chain := RequestLogger(logger)(Recoverer(logger)(okHandler))

	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ok", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, 0, logs.Len(), "no ERROR logs for a successful request")
	require.NotNil(t, capturedSlot)
	assert.Nil(t, capturedSlot.Err, "ErrorSlot.Err must be nil for non-panicking handler")
}
