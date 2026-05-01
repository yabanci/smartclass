package hass_test

import (
	"testing"

	"smartclass/internal/hass"
	"smartclass/internal/hass/hasstest"
)

// newSvc constructs a hass.Service backed by an in-memory repo + a real
// hass.Client pointed at baseURL. Shared by both default-build and
// `//go:build slow` tests; the slow-only mock-HA helpers (newMockHA,
// mockCounters) live alongside the slow tests in service_slow_test.go.
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
