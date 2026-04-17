package auditlog

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"smartclass/internal/platform/httpx"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/user"
)

type Handler struct {
	svc    *Service
	bundle *i18n.Bundle
}

func NewHandler(svc *Service, bundle *i18n.Bundle) *Handler {
	return &Handler{svc: svc, bundle: bundle}
}

func (h *Handler) Routes(r chi.Router) {
	r.With(requireAdmin(h.bundle)).Get("/", h.list)
}

func requireAdmin(bundle *i18n.Bundle) func(http.Handler) http.Handler {
	return mw.RequireRole(bundle, string(user.RoleAdmin))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := Query{}
	qp := r.URL.Query()
	if v := qp.Get("actorId"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			q.ActorID = &id
		}
	}
	if v := qp.Get("entityType"); v != "" {
		q.EntityType = EntityType(v)
	}
	if v := qp.Get("entityId"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			q.EntityID = &id
		}
	}
	if v := qp.Get("action"); v != "" {
		q.Action = Action(v)
	}
	if v := qp.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.From = &t
		}
	}
	if v := qp.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.To = &t
		}
	}
	q.Limit, _ = strconv.Atoi(qp.Get("limit"))
	q.Offset, _ = strconv.Atoi(qp.Get("offset"))

	entries, err := h.svc.List(r.Context(), q)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	out := make([]DTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, ToDTO(e))
	}
	httpx.JSON(w, http.StatusOK, out)
}
