package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBodyLimit_AllowsSmallPayload(t *testing.T) {
	h := BodyLimit(100)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusNoContent)
	}))

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hello"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestBodyLimit_RejectsOverLimitPayload(t *testing.T) {
	h := BodyLimit(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reading the body must surface the MaxBytesReader error.
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 1024)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code,
		"BodyLimit must cause oversized requests to fail when the handler reads the body")
}
