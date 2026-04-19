// Package ws implements realtime.Broker on top of an in-process WebSocket hub.
// Clients open /api/v1/ws with a Bearer JWT, optionally subscribing to topics
// via ?topic=classroom:<id>:devices&topic=....
//
// The hub is intentionally thin: a map of topics → set of clients, fan-out on
// Publish. To scale out, replace this type with a Redis PubSub / NATS backed
// implementation — the realtime.Broker interface stays identical.
package ws

import (
	"context"
	"encoding/json"
	"sync"

	"go.uber.org/zap"

	"smartclass/internal/realtime"
)

type Client struct {
	ID        string
	send      chan []byte
	topics    map[string]struct{}
	closed    chan struct{}
	closeOnce sync.Once
}

func newClient(id string, topics []string) *Client {
	set := make(map[string]struct{}, len(topics))
	for _, t := range topics {
		set[t] = struct{}{}
	}
	return &Client{
		ID:     id,
		send:   make(chan []byte, 64),
		topics: set,
		closed: make(chan struct{}),
	}
}

// close signals writePump to exit. Safe to call multiple times. We never
// close c.send — concurrent Publish goroutines would panic on send-to-closed.
// writePump exits via <-c.closed instead of the ok-idiom on c.send.
func (c *Client) close() {
	c.closeOnce.Do(func() {
		close(c.closed)
	})
}

type Hub struct {
	mu     sync.RWMutex
	byID   map[string]*Client
	byTopic map[string]map[string]*Client
	log    *zap.Logger
}

func NewHub(log *zap.Logger) *Hub {
	if log == nil {
		log = zap.NewNop()
	}
	return &Hub{
		byID:    map[string]*Client{},
		byTopic: map[string]map[string]*Client{},
		log:     log,
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.byID[c.ID] = c
	for t := range c.topics {
		if h.byTopic[t] == nil {
			h.byTopic[t] = map[string]*Client{}
		}
		h.byTopic[t][c.ID] = c
	}
	h.log.Debug("ws: client registered", zap.String("id", c.ID), zap.Int("topics", len(c.topics)))
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	delete(h.byID, c.ID)
	for t := range c.topics {
		if subs, ok := h.byTopic[t]; ok {
			delete(subs, c.ID)
			if len(subs) == 0 {
				delete(h.byTopic, t)
			}
		}
	}
	h.mu.Unlock()
	c.close()
	h.log.Debug("ws: client unregistered", zap.String("id", c.ID))
}

func (h *Hub) Publish(_ context.Context, event realtime.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	h.mu.RLock()
	subs := h.byTopic[event.Topic]
	clients := make([]*Client, 0, len(subs))
	for _, c := range subs {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		select {
		case <-c.closed:
			// client is shutting down — skip to avoid sending on a channel
			// whose consumer (writePump) has already exited.
		case c.send <- payload:
		default:
			h.log.Warn("ws: dropping slow client", zap.String("id", c.ID))
		}
	}
	return nil
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.byID)
}
