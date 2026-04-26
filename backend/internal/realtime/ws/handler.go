package ws

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"smartclass/internal/platform/httpx"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

type Handler struct {
	hub    *Hub
	log    *zap.Logger
	bundle *i18n.Bundle
}

func NewHandler(hub *Hub, log *zap.Logger, bundle *i18n.Bundle) *Handler {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handler{hub: hub, log: log, bundle: bundle}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(_ *http.Request) bool { return true },
	Subprotocols:    []string{"bearer"},
}

func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	p, ok := mw.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}

	topics := parseTopics(r, p.UserID)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("ws upgrade", zap.Error(err))
		return
	}

	client := newClient(uuid.NewString(), topics)
	h.hub.Register(client)

	// Pass the server shutdown context so writePump can exit cleanly when the
	// server is shutting down, rather than waiting for the 60s pong timeout.
	go h.readPump(conn, client)
	go h.writePump(conn, client, r.Context())
}

func parseTopics(r *http.Request, userID uuid.UUID) []string {
	q := r.URL.Query()["topic"]
	topics := make([]string, 0, len(q)+1)
	for _, t := range q {
		t = strings.TrimSpace(t)
		if t != "" {
			topics = append(topics, t)
		}
	}
	topics = append(topics, "user:"+userID.String()+":notifications")
	return topics
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
