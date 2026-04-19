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

// Regression: Publish used to panic with "send on closed channel" when a
// Unregister raced with an in-flight broadcast. Now Publish sees c.closed
// and skips. Run under -race to fail if the invariant regresses.
func TestHub_ConcurrentPublishUnregister_NoPanic(t *testing.T) {
	hub := NewHub(nil)
	const N = 50
	clients := make([]*Client, N)
	for i := 0; i < N; i++ {
		clients[i] = newClient(string(rune('a'+i)), []string{"t"})
		hub.Register(clients[i])
	}

	done := make(chan struct{})
	// publisher
	go func() {
		for i := 0; i < 500; i++ {
			_ = hub.Publish(context.Background(), realtime.Event{Topic: "t", Type: "x"})
		}
		close(done)
	}()
	// racer: unregister all while publisher runs
	for _, c := range clients {
		hub.Unregister(c)
	}
	<-done

	// Double-unregister is a no-op, not a panic.
	hub.Unregister(clients[0])
}

// Close signal must idempotently unblock writePump-style consumers.
func TestClient_Close_Idempotent(t *testing.T) {
	c := newClient("x", nil)
	c.close()
	c.close() // must not panic
	select {
	case <-c.closed:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("closed channel was not signalled")
	}
}
