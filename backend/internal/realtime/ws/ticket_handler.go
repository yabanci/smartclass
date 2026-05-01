package ws

import (
	"net/http"
	"time"

	"smartclass/internal/platform/httpx"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
)

// TicketHandler issues short-lived tickets the client uses to authenticate
// the next WebSocket upgrade. The route is mounted inside the authenticated
// chi.Group, so the principal is already validated; we just record (userID,
// expiry) into the store and hand the random ticket string back.
type TicketHandler struct {
	store  TicketStore
	bundle *i18n.Bundle
}

func NewTicketHandler(store TicketStore, bundle *i18n.Bundle) *TicketHandler {
	return &TicketHandler{store: store, bundle: bundle}
}

func (h *TicketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := mw.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	tkt, err := h.store.Issue(r.Context(), p.UserID)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ticketResponse{
		Ticket:    tkt.Raw,
		ExpiresAt: tkt.ExpiresAt.UTC(),
	})
}

type ticketResponse struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expiresAt"`
}
