package ws

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"smartclass/internal/classroom"
	"smartclass/internal/platform/httpx"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/metrics"
	"smartclass/internal/user"
)

// TopicAuthorizer answers "may this principal subscribe to this classroom?"
// Implemented by *classroom.Service.Authorize. We accept the small interface
// rather than the concrete type so the ws package can be tested without
// pulling in the whole classroom service.
type TopicAuthorizer interface {
	Authorize(ctx context.Context, p classroom.Principal, classroomID uuid.UUID, mutate bool) error
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type Handler struct {
	hub          *Hub
	log          *zap.Logger
	bundle       *i18n.Bundle
	authz        TopicAuthorizer
	tickets      TicketStore
	allowedOrigs []string
	upgrader     websocket.Upgrader
	// shutdownCtx is cancelled when the server begins graceful shutdown.
	// writePump selects on it to send a CloseGoingAway frame and exit cleanly.
	// We cannot use r.Context() here because the http.Handler returns
	// immediately after launching the pump goroutines, which would cancel the
	// request context and tear down every WS connection right after the
	// handshake.
	shutdownCtx context.Context
}

// NewHandler builds a WebSocket Handler. Authentication is the *ticket flow*:
// the mobile client first POSTs to /ws/ticket (Bearer-authenticated) and uses
// the returned 60s single-use ticket here. Tokens-in-URL leaked into reverse-
// proxy access logs; tickets are one-shot and short-lived.
//
// allowedOrigs becomes the WebSocket CheckOrigin allow-list. Pass `[]string{"*"}`
// for dev / tests; production should pass cfg.CORS.Origins.
//
// shutdownCtx should be the server's graceful-shutdown context (i.e. the
// context.Background() derivative that is cancelled on SIGTERM). writePump
// selects on it to send a CloseGoingAway frame and exit. Do NOT pass
// r.Context() here — that context is cancelled as soon as Serve returns, which
// would kill every WS connection immediately after the handshake.
func NewHandler(hub *Hub, log *zap.Logger, bundle *i18n.Bundle,
	authz TopicAuthorizer, tickets TicketStore, allowedOrigs []string,
	shutdownCtx ...context.Context) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	sctx := context.Background()
	if len(shutdownCtx) > 0 && shutdownCtx[0] != nil {
		sctx = shutdownCtx[0]
	}
	return &Handler{
		hub: hub, log: log.With(zap.String("subsystem", "ws")),
		bundle: bundle, authz: authz, tickets: tickets, allowedOrigs: allowedOrigs,
		shutdownCtx: sctx,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     makeCheckOrigin(allowedOrigs),
			Subprotocols:    []string{"bearer"},
		},
	}
}

func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ticket")
	if raw == "" {
		httpx.WriteError(w, r, h.bundle, httpx.ErrWSTicketRequired)
		return
	}
	if h.tickets == nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrWSTicketInvalid)
		return
	}
	tkt, err := h.tickets.Consume(r.Context(), raw)
	if err != nil {
		log := h.log
		if log == nil {
			log = zap.NewNop()
		}
		log.Warn("ws: ticket consume failed",
			zap.Error(err),
			zap.String("remote", r.RemoteAddr))
		metrics.WSTicketInvalid.Inc()
		httpx.WriteError(w, r, h.bundle, httpx.ErrWSTicketInvalid)
		return
	}

	p := mw.Principal{UserID: tkt.UserID, Role: tkt.Role}

	topics, err := h.authorizeTopics(r.Context(), p, r.URL.Query()["topic"])
	if err != nil {
		// Fail closed: any unauthorized classroom topic in the request rejects
		// the entire upgrade. Silently dropping just the bad ones would mean
		// clients couldn't tell why their realtime updates went missing.
		httpx.WriteError(w, r, h.bundle, err)
		return
	}

	conn, upgErr := h.upgrader.Upgrade(w, r, nil)
	if upgErr != nil {
		h.log.Warn("ws upgrade", zap.Error(upgErr))
		return
	}

	client := newClient(uuid.NewString(), topics)
	h.hub.Register(client)

	// Pass the server's shutdown context — NOT r.Context() — so writePump
	// stays alive for the lifetime of the connection. r.Context() is cancelled
	// as soon as Serve returns (Go HTTP server cancels it on handler exit),
	// which would close every WS connection immediately after the handshake.
	go h.readPump(conn, client)
	go h.writePump(conn, client, h.shutdownCtx)
}

// makeCheckOrigin returns a CheckOrigin that consults the CORS allow-list.
// Empty origins (non-browser clients: native mobile, CLI, tests) are allowed
// because the ticket validation already gated the request — there's no
// cookie-equivalent CSRF threat. The "*" sentinel keeps the dev/test
// allow-everything mode.
func makeCheckOrigin(allowedOrigins []string) func(*http.Request) bool {
	allowAll := len(allowedOrigins) == 1 && strings.TrimSpace(allowedOrigins[0]) == "*"
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[strings.TrimSpace(o)] = struct{}{}
	}
	return func(r *http.Request) bool {
		if allowAll {
			return true
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		_, ok := allowed[origin]
		return ok
	}
}

// authorizeTopics enforces the WS subscription contract:
//
//   - "user:<self-uuid>:notifications" is auto-added (a user always sees their
//     own notifications).
//   - "classroom:<uuid>:..." is allowed only if the principal has access to
//     that classroom; the check delegates to the classroom service.
//   - any other shape is rejected — strict allowlist, not a denylist.
//
// Without this check a teacher could subscribe to "classroom:<other>:devices"
// and silently observe events from a classroom they have no membership in.
func (h *Handler) authorizeTopics(ctx context.Context, p mw.Principal, requested []string) ([]string, error) {
	out := make([]string, 0, len(requested)+1)
	out = append(out, "user:"+p.UserID.String()+":notifications")

	principal := classroom.Principal{UserID: p.UserID, Role: user.Role(p.Role)}
	for _, raw := range requested {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}

		// Allow the user's own notification topic again as an explicit request
		// (idempotent — common path for clients re-subscribing on reconnect).
		if t == "user:"+p.UserID.String()+":notifications" {
			continue // already in `out`
		}

		// Reject any other "user:" topic — those are someone else's events.
		if strings.HasPrefix(t, "user:") {
			return nil, httpx.ErrForbidden
		}

		if strings.HasPrefix(t, "classroom:") {
			rest := t[len("classroom:"):]
			sep := strings.IndexByte(rest, ':')
			if sep <= 0 {
				return nil, httpx.ErrBadRequest
			}
			classroomID, err := uuid.Parse(rest[:sep])
			if err != nil {
				return nil, httpx.ErrBadRequest
			}
			if h.authz == nil {
				// Misconfiguration — refuse rather than fall open.
				return nil, httpx.ErrForbidden
			}
			if err := h.authz.Authorize(ctx, principal, classroomID, false); err != nil {
				return nil, httpx.ErrForbidden
			}
			out = append(out, t)
			continue
		}

		// Unknown topic prefix → reject.
		return nil, httpx.ErrBadRequest
	}
	return out, nil
}

func (h *Handler) readPump(conn *websocket.Conn, c *Client) {
	defer func() {
		h.hub.Unregister(c)
		_ = conn.Close()
	}()
	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		if _, _, err := conn.NextReader(); err != nil {
			return
		}
	}
}

func (h *Handler) writePump(conn *websocket.Conn, c *Client, ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = conn.Close()
	}()
	for {
		select {
		case <-ctx.Done():
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"))
			return
		case <-c.closed:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		case msg := <-c.send:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
