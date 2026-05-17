package realtime_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/realtime"
)

func TestEvent_VersionRoundTrip(t *testing.T) {
	in := realtime.Event{
		Version: 1,
		Topic:   "user:1:notifications",
		Type:    "notification.created",
		Payload: map[string]any{"id": "abc"},
	}
	raw, err := json.Marshal(in)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"version":1`,
		"every emitted event must include `version: 1` in its JSON — that's the contract for forward-compatibility")

	var out realtime.Event
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, 1, out.Version)
}

func TestEvent_MissingVersion_DefaultsToZero(t *testing.T) {
	// A consumer encountering a legacy event without version sees Version=0,
	// distinguishable from any explicit Version. That lets future consumers
	// say "I don't know how to read v0" cleanly.
	var got realtime.Event
	require.NoError(t, json.Unmarshal([]byte(`{"topic":"t","type":"x"}`), &got))
	assert.Equal(t, 0, got.Version)
}

func TestNewEvent_SetsVersionOne(t *testing.T) {
	e := realtime.NewEvent("user:1:notifications", "x", nil)
	assert.Equal(t, 1, e.Version,
		"NewEvent constructor must always emit version=1 — that's the whole point of having it over a struct literal")
}
