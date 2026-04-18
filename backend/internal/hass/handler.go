package hass

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"smartclass/internal/classroom"
	"smartclass/internal/device"
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
	r.Get("/status", h.status)
	r.Post("/token", h.setToken)
	r.Get("/integrations", h.integrations)
	r.Post("/flows", h.startFlow)
	r.Post("/flows/{id}/step", h.stepFlow)
	r.Delete("/flows/{id}", h.abortFlow)
	r.Get("/entities", h.entities)
	r.Post("/adopt", h.adopt)
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, h.svc.Status(r.Context()))
}

type setTokenReq struct {
	Token string `json:"token" validate:"required,min=20"`
}

func (h *Handler) setToken(w http.ResponseWriter, r *http.Request) {
	var req setTokenReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.valid.Struct(&req); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	if err := h.svc.SetToken(r.Context(), req.Token); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, h.svc.Status(r.Context()))
}

func (h *Handler) integrations(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListIntegrations(r.Context())
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	if items == nil {
		items = []FlowHandler{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

type startFlowReq struct {
	Handler string `json:"handler" validate:"required"`
}

func (h *Handler) startFlow(w http.ResponseWriter, r *http.Request) {
	var req startFlowReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.valid.Struct(&req); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	step, err := h.svc.StartFlow(r.Context(), req.Handler)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, step)
}

type stepFlowReq struct {
	Data map[string]any `json:"data"`
}

func (h *Handler) stepFlow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	var req stepFlowReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	step, err := h.svc.StepFlow(r.Context(), id, req.Data)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, step)
}

func (h *Handler) abortFlow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.AbortFlow(r.Context(), id); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.Empty(w, http.StatusNoContent)
}

func (h *Handler) entities(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListEntities(r.Context())
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	if items == nil {
		items = []Entity{}
	}
	httpx.JSON(w, http.StatusOK, items)
}

type adoptReq struct {
	EntityID    string `json:"entityId" validate:"required"`
	ClassroomID string `json:"classroomId" validate:"required,uuid"`
	Name        string `json:"name"`
	Brand       string `json:"brand"`
}

func (h *Handler) adopt(w http.ResponseWriter, r *http.Request) {
	p, ok := mw.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	var req adoptReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.valid.Struct(&req); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	cid, err := uuid.Parse(req.ClassroomID)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	d, err := h.svc.Adopt(r.Context(), classroom.Principal{UserID: p.UserID, Role: user.Role(p.Role)}, AdoptInput{
		EntityID:    req.EntityID,
		ClassroomID: cid,
		Name:        req.Name,
		Brand:       req.Brand,
	})
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, device.ToDTO(d))
}
