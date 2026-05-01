package hass_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/hass"
)

// Fast hass tests. Tests that need to drive Bootstrap / FinishOnboarding's
// 7.5s retry-loop or token-expiry sleep live in service_slow_test.go behind
// `//go:build slow`. Default `go test` skips those; CI runs both tracks.

func TestBootstrap_AlreadyOnboardedReturnsSentinel(t *testing.T) {
	// Mock where onboarding is "done" immediately — simulates user who ran HA manually.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/onboarding", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"step": "user", "done": true},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := newSvc(t, srv.URL)
	_, err := svc.Bootstrap(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, hass.ErrAlreadyOnboarded), "got %v", err)
}

func TestSetToken_PersistsAfterVerification(t *testing.T) {
	// HA that rejects everything except a token we'll supply.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/states", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer manual-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode([]any{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := newSvc(t, srv.URL)
	require.NoError(t, svc.SetToken(context.Background(), "manual-token"))

	// After SetToken, ListEntities should reuse it without any new bootstrap.
	_, err := svc.ListEntities(context.Background())
	require.NoError(t, err)
}

func TestSetToken_DoesNotRefresh(t *testing.T) {
	// HA that accepts only the manually-supplied long-lived token (no /auth/token).
	mux := http.NewServeMux()
	mux.HandleFunc("/api/states", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer manual-bypass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode([]any{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := newSvc(t, srv.URL)
	require.NoError(t, svc.SetToken(context.Background(), "manual-bypass"))

	c, err := svc.Credentials(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "manual-bypass", c.Token)
	assert.Empty(t, c.RefreshToken, "manual long-lived token has no refresh token")
	assert.True(t, c.ExpiresAt.IsZero(), "manual token never expires from our POV")
}
