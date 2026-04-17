// Package realtime defines a transport-agnostic pub/sub interface the rest of
// the app uses to fan out device/sensor/notification events. A swap from the
// in-process WebSocket hub to Redis PubSub or NATS is a one-file change.
package realtime

import "context"

type Event struct {
	Topic   string         `json:"topic"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

type Broker interface {
	Publish(ctx context.Context, event Event) error
}

type Noop struct{}

func (Noop) Publish(_ context.Context, _ Event) error { return nil }
