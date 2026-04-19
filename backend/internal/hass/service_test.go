package hass_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/hass"
	"smartclass/internal/hass/hasstest"
)

// newMockHA spins up an HA-shaped HTTP server that:
//   1. reports onboarding as not done until /api/onboarding/users is POSTed;
//   2. returns an auth_code on that POST;
//   3. exchanges that auth_code for an access token at /auth/token;
//   4. issues a long-lived token at /auth/long_lived_access_token;
//   5. serves /api/states with two entities (one supported, one not).
// It records how many times each onboarding endpoint is hit so tests can
// verify idempotency.
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
			"flow_id":  "flow-123",
			"handler":  req["handler"],
			"type":     "form",
			"step_id":  "user",
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
				"title":  "Adopted",
				"entry":  "abc",
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

func TestBootstrap_OnboardsOnce(t *testing.T) {
	ha, counters := newMockHA(t)
	svc := newSvc(t, ha.URL)

	c, err := svc.Bootstrap(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "access-1", c.Token)
	assert.Equal(t, "refresh-rotated", c.RefreshToken)
	assert.False(t, c.ExpiresAt.IsZero())
	assert.True(t, c.Onboarded)

	// Second call should hit the cache, not the upstream.
	prevStatus := atomic.LoadInt32(&counters.onboardingStatus)
	prevCreate := atomic.LoadInt32(&counters.createUser)
	_, err = svc.Bootstrap(context.Background())
	require.NoError(t, err)
	assert.Equal(t, prevStatus, atomic.LoadInt32(&counters.onboardingStatus), "status not re-fetched from cache")
	assert.Equal(t, prevCreate, atomic.LoadInt32(&counters.createUser), "user not re-created from cache")
}

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

func TestListEntities_FiltersToSupportedDomains(t *testing.T) {
	ha, _ := newMockHA(t)
	svc := newSvc(t, ha.URL)
	_, err := svc.Bootstrap(context.Background())
	require.NoError(t, err)

	entities, err := svc.ListEntities(context.Background())
	require.NoError(t, err)
	require.Len(t, entities, 1, "sensor.* filtered out, light.* kept")
	assert.Equal(t, "light.kitchen", entities[0].EntityID)
	assert.Equal(t, "light", entities[0].Domain)
	assert.Equal(t, "Kitchen Light", entities[0].FriendlyName)
}

func TestFlowProxy_StartAndStep(t *testing.T) {
	ha, _ := newMockHA(t)
	svc := newSvc(t, ha.URL)
	_, err := svc.Bootstrap(context.Background())
	require.NoError(t, err)

	step, err := svc.StartFlow(context.Background(), "mqtt")
	require.NoError(t, err)
	assert.Equal(t, "form", step.Type)
	assert.Equal(t, "flow-123", step.FlowID)
	require.Len(t, step.DataSchema, 2)
	assert.Equal(t, "host", step.DataSchema[0].Name)
	assert.True(t, step.DataSchema[0].Required)

	final, err := svc.StepFlow(context.Background(), "flow-123", map[string]any{"host": "192.168.1.1"})
	require.NoError(t, err)
	assert.Equal(t, "create_entry", final.Type)
	assert.Equal(t, "Adopted", final.Result["title"])
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

// Regression: Credentials() used to release the mutex between the
// NeedsRefresh check and the HA token exchange, so N parallel callers of
// Credentials on a near-expired token all triggered independent refreshes.
// With refreshMu + double-check, refresh_token grant hits upstream once.
func TestCredentials_SerializesConcurrentRefresh(t *testing.T) {
	var refreshCount atomic.Int32
	var authCodeCalled atomic.Bool

	mux := http.NewServeMux()
	mux.HandleFunc("/api/onboarding", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"step": "user", "done": false}})
	})
	mux.HandleFunc("/api/onboarding/users", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"auth_code": "c"})
	})
	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.FormValue("grant_type") {
		case "authorization_code":
			if !authCodeCalled.CompareAndSwap(false, true) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			// expires_in=1 forces NeedsRefresh() to return true immediately,
			// so any follow-up Credentials() call enters the refresh path.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "A",
				"refresh_token": "R",
				"token_type":    "Bearer",
				"expires_in":    1,
			})
		case "refresh_token":
			refreshCount.Add(1)
			// Latency so concurrent callers definitely pile up on refreshMu;
			// without serialization each would issue its own /auth/token.
			time.Sleep(50 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "A2",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	for _, p := range []string{
		"/api/onboarding/core_config",
		"/api/onboarding/analytics",
		"/api/onboarding/integration",
	} {
		mux.HandleFunc(p, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := newSvc(t, srv.URL)
	_, err := svc.Bootstrap(context.Background())
	require.NoError(t, err)

	const N = 20
	var wg sync.WaitGroup
	wg.Add(N)
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			if _, err := svc.Credentials(context.Background()); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("concurrent Credentials error: %v", e)
	}

	assert.Equal(t, int32(1), refreshCount.Load(),
		"refresh_token grant should be exchanged exactly once for N parallel callers")
}

func TestStatus_ReflectsConfiguredState(t *testing.T) {
	ha, _ := newMockHA(t)
	svc := newSvc(t, ha.URL)

	st := svc.Status(context.Background())
	// First call triggers bootstrap and succeeds — so Configured=true.
	assert.True(t, st.Configured)
	assert.True(t, st.Onboarded)
	assert.Contains(t, strings.ToLower(st.BaseURL), "127.0.0.1")
}
