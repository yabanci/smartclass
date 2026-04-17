package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smartclass/internal/realtime"
)

func TestHub_PublishFansOutToSubscribers(t *testing.T) {
	hub := NewHub(nil)
	a := newClient("a", []string{"classroom:1:devices"})
	b := newClient("b", []string{"classroom:1:devices"})
	c := newClient("c", []string{"classroom:2:devices"})
	hub.Register(a)
	hub.Register(b)
	hub.Register(c)

	err := hub.Publish(context.Background(), realtime.Event{
		Topic: "classroom:1:devices", Type: "device.on",
	})
	require.NoError(t, err)

	assertReceives(t, a, "device.on")
	assertReceives(t, b, "device.on")
	assertNoMessage(t, c)
}

func TestHub_Unregister(t *testing.T) {
	hub := NewHub(nil)
	a := newClient("a", []string{"t"})
	hub.Register(a)
	require.Equal(t, 1, hub.ClientCount())

	hub.Unregister(a)
	require.Equal(t, 0, hub.ClientCount())

	err := hub.Publish(context.Background(), realtime.Event{Topic: "t", Type: "x"})
	require.NoError(t, err)
}

func TestHub_SlowClientDropped(t *testing.T) {
	hub := NewHub(nil)
	a := newClient("a", []string{"t"})
	hub.Register(a)

	for i := 0; i < 200; i++ {
		_ = hub.Publish(context.Background(), realtime.Event{Topic: "t", Type: "x"})
	}
	assert.LessOrEqual(t, len(a.send), 64)
}

func assertReceives(t *testing.T, c *Client, wantType string) {
	t.Helper()
	select {
	case msg := <-c.send:
		var e realtime.Event
		require.NoError(t, json.Unmarshal(msg, &e))
		assert.Equal(t, wantType, e.Type)
	case <-time.After(time.Second):
		t.Fatalf("client %s did not receive", c.ID)
	}
}

func assertNoMessage(t *testing.T, c *Client) {
	t.Helper()
	select {
	case msg := <-c.send:
		t.Fatalf("client %s unexpected: %s", c.ID, string(msg))
	case <-time.After(50 * time.Millisecond):
	}
}
