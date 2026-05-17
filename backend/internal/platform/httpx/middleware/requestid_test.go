package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestID_GeneratedWhenAbsent(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	require.NotEmpty(t, seen, "RequestID must populate context with a generated id")
	assert.Equal(t, seen, rr.Header().Get(RequestIDHeader),
		"the same id must be echoed back in the response header for client-side correlation")
}

func TestRequestID_TrustsCallerSuppliedHeader(t *testing.T) {
	const caller = "abc-123-deadbeef"
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(RequestIDHeader, caller)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)

	assert.Equal(t, caller, seen, "an upstream gateway-supplied id must be propagated unchanged")
}

func TestRequestID_RegeneratedWhenCallerSendsAbsurdLength(t *testing.T) {
	long := strings.Repeat("x", 200)
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(RequestIDHeader, long)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)

	assert.NotEqual(t, long, seen, "an oversized caller id (>128 chars) must be rejected and a fresh id generated")
}
