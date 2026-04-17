package sensor

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"smartclass/internal/classroom"
	"smartclass/internal/platform/httpx"
	mw "smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/validation"
	"smartclass/internal/user"
)

type Handler struct {
	svc    *Service
	valid  *validation.Validator
	bundle *i18n.Bundle
}

func NewHandler(svc *Service, valid *validation.Validator, bundle *i18n.Bundle) *Handler {
	return &Handler{svc: svc, valid: valid, bundle: bundle}
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/readings", h.ingest)
}

func (h *Handler) DeviceRoutes(r chi.Router) {
	r.Get("/readings", h.history)
}

func (h *Handler) ClassroomRoutes(r chi.Router) {
	r.Get("/readings/latest", h.latest)
}

func (h *Handler) principal(r *http.Request) (classroom.Principal, bool) {
	p, ok := mw.PrincipalFrom(r.Context())
	if !ok {
		return classroom.Principal{}, false
	}
	return classroom.Principal{UserID: p.UserID, Role: user.Role(p.Role)}, true
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(r)
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	var req IngestRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.valid.Struct(&req); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	items := make([]IngestItem, 0, len(req.Readings))
	for _, it := range req.Readings {
		did, err := uuid.Parse(it.DeviceID)
		if err != nil {
			httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
			return
		}
		items = append(items, IngestItem{
			DeviceID: did, Metric: Metric(it.Metric),
			Value: it.Value, Unit: it.Unit,
			RecordedAt: it.RecordedAt, Raw: it.Raw,
		})
	}
	n, err := h.svc.Ingest(r.Context(), p, items)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, IngestResponse{Accepted: n})
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(r)
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	did, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	metric := Metric(r.URL.Query().Get("metric"))
	from := parseTime(r.URL.Query().Get("from"))
	to := parseTime(r.URL.Query().Get("to"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	list, err := h.svc.History(r.Context(), p, did, metric, from, to, limit)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	out := make([]ReadingDTO, 0, len(list))
	for _, rd := range list {
		out = append(out, ToDTO(rd))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(r)
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	cid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	list, err := h.svc.LatestForClassroom(r.Context(), p, cid)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	out := make([]ReadingDTO, 0, len(list))
	for _, rd := range list {
		out = append(out, ToDTO(rd))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
