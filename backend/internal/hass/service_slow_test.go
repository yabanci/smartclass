//go:build slow

package hass_test

import (
	"context"
	"encoding/json"
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
)

// All tests in this file exercise the Bootstrap / FinishOnboarding /
// token-refresh paths. FinishOnboarding has a 5-attempt loop with backoff
// (0.5+1+1.5+2+2.5 = 7.5s), and several tests intentionally sleep to drive
// token-expiry behavior — together they account for the original 55s package
// runtime. Default `go test` skips this file; CI runs:
//
//	go test -tags=slow ./internal/hass/...

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

// TestCurrentToken_ReturnsBootstrappedToken verifies that CurrentToken (the
// homeassistant.TokenProvider implementation) returns the same access token
// that Bootstrap stored, without triggering another round-trip to HA.
func TestCurrentToken_ReturnsBootstrappedToken(t *testing.T) {
	ha, _ := newMockHA(t)
	svc := newSvc(t, ha.URL)

	_, err := svc.Bootstrap(context.Background())
	require.NoError(t, err)

	tok, err := svc.CurrentToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "access-1", tok)
}

// TestCurrentToken_RefreshesExpiredToken verifies that CurrentToken transparently
// refreshes the access token when it is about to expire, so device drivers
// always receive a usable token without manual intervention.
func TestCurrentToken_RefreshesExpiredToken(t *testing.T) {
	var authCodeUsed atomic.Bool
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
			if !authCodeUsed.CompareAndSwap(false, true) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "expired-token", "refresh_token": "R",
				"token_type": "Bearer", "expires_in": 1, // expire immediately
			})
		case "refresh_token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "fresh-token", "token_type": "Bearer", "expires_in": 3600,
			})
		}
	})
	for _, p := range []string{"/api/onboarding/core_config", "/api/onboarding/analytics", "/api/onboarding/integration"} {
		mux.HandleFunc(p, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := newSvc(t, srv.URL)
	_, err := svc.Bootstrap(context.Background())
	require.NoError(t, err)

	// Token has expires_in=1; sleep past NeedsRefresh threshold (< 60s remaining).
	time.Sleep(2 * time.Second)

	tok, err := svc.CurrentToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "fresh-token", tok, "CurrentToken must trigger a refresh when token is expired")
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

// silence unused-import lint when fast tests are pruned aggressively.
var _ = hass.ErrAlreadyOnboarded
