package hass_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"smartclass/internal/hass"
	"smartclass/internal/hass/hasstest"
)

// newMockHA spins up an HA-shaped HTTP server that:
//  1. reports onboarding as not done until /api/onboarding/users is POSTed;
//  2. returns an auth_code on that POST;
//  3. exchanges that auth_code for an access token at /auth/token;
//  4. issues a long-lived token at /auth/long_lived_access_token;
//  5. serves /api/states with two entities (one supported, one not).
//
// It records how many times each onboarding endpoint is hit so tests can
// verify idempotency. Shared between the default-build fast tests and the
// `//go:build slow` Bootstrap-bound tests.
func newMockHA(t *testing.T) (*httptest.Server, *mockCounters) {
	t.Helper()
	c := &mockCounters{}
	mux := http.NewServeMux()
	var ownerCreated atomic.Bool

	mux.HandleFunc("/api/onboarding", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&c.onboardingStatus, 1)
		resp := []map[string]any{
			{"step": "user", "done": ownerCreated.Load()},
			{"step": "core_config", "done": false},
			{"step": "analytics", "done": false},
			{"step": "integration", "done": false},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/onboarding/users", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&c.createUser, 1)
		if ownerCreated.Load() {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		ownerCreated.Store(true)
		_ = json.NewEncoder(w).Encode(map[string]string{"auth_code": "code-xyz"})
	})

	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		switch r.FormValue("grant_type") {
		case "authorization_code":
			if r.FormValue("code") != "code-xyz" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "access-1",
				"refresh_token": "refresh-rotated",
				"token_type":    "Bearer",
				"expires_in":    1800,
			})
		case "refresh_token":
			if r.FormValue("refresh_token") == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-2",
				"token_type":   "Bearer",
				"expires_in":   1800,
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	mux.HandleFunc("/api/onboarding/core_config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/onboarding/analytics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/onboarding/integration", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/states", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer access-1" && auth != "Bearer access-2" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"entity_id":  "light.kitchen",
				"state":      "off",
				"attributes": map[string]any{"friendly_name": "Kitchen Light"},
			},
			{
				"entity_id":  "sensor.temperature",
				"state":      "21",
				"attributes": map[string]any{"friendly_name": "Temperature"},
			},
		})
	})

	mux.HandleFunc("/api/config/config_entries/flow", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"flow_id": "flow-123",
			"handler": req["handler"],
			"type":    "form",
			"step_id": "user",
			"data_schema": []map[string]any{
				{"type": "string", "name": "host", "required": true},
				{"type": "integer", "name": "port", "optional": true, "default": 1883},
			},
		})
	})

	mux.HandleFunc("/api/config/config_entries/flow/flow-123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"flow_id": "flow-123",
			"type":    "create_entry",
			"result": map[string]any{
				"title": "Adopted",
				"entry": "abc",
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, c
}

type mockCounters struct {
	onboardingStatus int32
	createUser       int32
}

func newSvc(t *testing.T, baseURL string) *hass.Service {
	t.Helper()
	client := hass.NewClient(baseURL, nil)
	repo := hasstest.New()
	return hass.NewService(hass.Config{
		BaseURL:       baseURL,
		OwnerName:     "Test",
		OwnerUsername: "tester",
		OwnerPassword: "tester1234",
		Language:      "kz",
	}, repo, client, nil, nil)
}
