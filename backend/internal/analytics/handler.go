package analytics

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"smartclass/internal/classroom"
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

func (h *Handler) ClassroomRoutes(r chi.Router) {
	r.Get("/sensors", h.sensors)
	r.Get("/usage", h.usage)
	r.Get("/energy", h.energy)
}

func (h *Handler) principal(r *http.Request) (classroom.Principal, bool) {
	p, ok := mw.PrincipalFrom(r.Context())
	if !ok {
		return classroom.Principal{}, false
	}
	return classroom.Principal{UserID: p.UserID, Role: user.Role(p.Role)}, true
}

func (h *Handler) sensors(w http.ResponseWriter, r *http.Request) {
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
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	bucket := Bucket(r.URL.Query().Get("bucket"))
	if bucket == "" {
		bucket = BucketHour
	}
	from := parseTime(r.URL.Query().Get("from"))
	to := parseTime(r.URL.Query().Get("to"))

	series, err := h.svc.SensorSeries(r.Context(), p, cid, metric, bucket, from, to)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, series)
}

func (h *Handler) usage(w http.ResponseWriter, r *http.Request) {
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
	from := parseTime(r.URL.Query().Get("from"))
	to := parseTime(r.URL.Query().Get("to"))
	items, err := h.svc.DeviceUsage(r.Context(), p, cid, from, to)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, items)
}

func (h *Handler) energy(w http.ResponseWriter, r *http.Request) {
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
	from := parseTime(r.URL.Query().Get("from"))
	to := parseTime(r.URL.Query().Get("to"))
	total, err := h.svc.EnergyTotal(r.Context(), p, cid, from, to)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]float64{"total": total})
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
