package ws

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Ticket is what TicketStore.Issue hands back to the caller — `Raw` is the
// string the WS client puts in `?ticket=`, the rest is metadata used by the
// store and surfaced for observability/logs.
type Ticket struct {
	Raw       string
	UserID    uuid.UUID
	Role      string
	ExpiresAt time.Time
}

// TicketStore mints and consumes single-use WebSocket upgrade tickets.
// Tickets exist because we don't want JWTs in URL query strings (they end up
// in reverse-proxy access logs). The mobile client calls Issue with a Bearer
// JWT, then immediately uses the returned ticket on the WS upgrade. The
// server validates with Consume — once-only, so a ticket leaked into a log
// or shoulder-surfed off the URL bar is useless.
type TicketStore interface {
	Issue(ctx context.Context, userID uuid.UUID, role string) (Ticket, error)
	Consume(ctx context.Context, raw string) (Ticket, error)
}

// ErrTicketUnknown collapses three failure modes into one: ticket never
// issued, expired, or already used. Callers map all three to a single 401
// rather than disclose which condition matched (so an attacker can't probe).
var ErrTicketUnknown = errors.New("ws: ticket unknown or already used")

// MemTicketStore is the single-process implementation. A sync.Map keyed on
// the raw ticket string holds entries with userID, expiry, and an atomic.Bool
// that the Consume CAS uses to enforce once-only redemption.
type MemTicketStore struct {
	ttl     time.Duration
	entries sync.Map // map[string]*ticketEntry
}

type ticketEntry struct {
	userID    uuid.UUID
	role      string
	expiresAt time.Time
	used      atomic.Bool
}

// NewMemTicketStore returns a store with the given TTL. Callers should also
// call Run on it from a goroutine so expired entries get pruned.
func NewMemTicketStore(ttl time.Duration) *MemTicketStore {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &MemTicketStore{ttl: ttl}
}

// Issue generates a fresh random ticket. 24 random bytes encode to 32 URL-safe
// base64 chars without padding — small enough for a query string, large
// enough to resist guessing.
func (s *MemTicketStore) Issue(_ context.Context, userID uuid.UUID, role string) (Ticket, error) {
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return Ticket{}, fmt.Errorf("ws: ticket entropy: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(buf[:])
	expiresAt := time.Now().Add(s.ttl)
	s.entries.Store(raw, &ticketEntry{userID: userID, role: role, expiresAt: expiresAt})
	return Ticket{Raw: raw, UserID: userID, Role: role, ExpiresAt: expiresAt}, nil
}

// Consume marks a ticket used and returns the Ticket (including userID and
// role). Returns ErrTicketUnknown for unknown / expired / already-used tickets
// — single error path so the caller's response is identical regardless of
// which case matched.
func (s *MemTicketStore) Consume(_ context.Context, raw string) (Ticket, error) {
	v, ok := s.entries.Load(raw)
	if !ok {
		return Ticket{}, ErrTicketUnknown
	}
	entry := v.(*ticketEntry)
	if time.Now().After(entry.expiresAt) {
		s.entries.Delete(raw)
		return Ticket{}, ErrTicketUnknown
	}
	if !entry.used.CompareAndSwap(false, true) {
		return Ticket{}, ErrTicketUnknown
	}
	// Drop from the map immediately — once-used is forever-used; keeping
	// it around just wastes memory until cleanup.
	s.entries.Delete(raw)
	return Ticket{Raw: raw, UserID: entry.userID, Role: entry.role, ExpiresAt: entry.expiresAt}, nil
}

// Run starts a background goroutine that prunes expired entries on `interval`.
// Returns a stop function the caller invokes on shutdown.
func (s *MemTicketStore) Run(interval time.Duration) (stop func()) {
	if interval <= 0 {
		interval = time.Minute
	}
	done := make(chan struct{})
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				s.cleanup()
			}
		}
	}()
	return func() { close(done) }
}

func (s *MemTicketStore) cleanup() {
	now := time.Now()
	s.entries.Range(func(k, v any) bool {
		if now.After(v.(*ticketEntry).expiresAt) {
			s.entries.Delete(k)
		}
		return true
	})
}
