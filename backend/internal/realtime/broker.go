// Package realtime defines a transport-agnostic pub/sub interface the rest of
// the app uses to fan out device/sensor/notification events. A swap from the
// in-process WebSocket hub to Redis PubSub or NATS is a one-file change.
package realtime

import "context"

// Event is the wire format every broker fans out to subscribers.
//
// Version is the schema version: bumped only on breaking changes (renames,
// type changes). Consumers MUST tolerate unknown fields so additive changes
// don't require a version bump. Producers always set this — see NewEvent.
type Event struct {
	Version int            `json:"version"`
	Topic   string         `json:"topic"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// NewEvent is the canonical constructor — sets Version to the current schema
// version (1) so callers can't accidentally emit Version=0. Use this in
// preference to literal struct construction.
func NewEvent(topic, eventType string, payload map[string]any) Event {
	return Event{Version: 1, Topic: topic, Type: eventType, Payload: payload}
}

type Broker interface {
	Publish(ctx context.Context, event Event) error
}

type Noop struct{}

func (Noop) Publish(_ context.Context, _ Event) error { return nil }
