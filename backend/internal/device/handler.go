package device

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"smartclass/internal/classroom"
	"smartclass/internal/devicectl"
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
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Post("/{id}/commands", h.execute)
}

func (h *Handler) ClassroomRoutes(r chi.Router) {
	r.Get("/", h.listByClassroom)
}

func (h *Handler) principal(r *http.Request) (classroom.Principal, bool) {
	p, ok := mw.PrincipalFrom(r.Context())
	if !ok {
		return classroom.Principal{}, false
	}
	return classroom.Principal{UserID: p.UserID, Role: user.Role(p.Role)}, true
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(r)
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	var req CreateRequest
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
	d, err := h.svc.Create(r.Context(), p, CreateInput{
		ClassroomID: cid, Name: req.Name, Type: req.Type,
		Brand: req.Brand, Driver: req.Driver, Config: req.Config,
	})
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, ToDTO(d))
}

func (h *Handler) listByClassroom(w http.ResponseWriter, r *http.Request) {
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
	items, err := h.svc.ListByClassroom(r.Context(), p, cid)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	out := make([]DTO, 0, len(items))
	for _, d := range items {
		out = append(out, ToDTO(d))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(r)
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	d, err := h.svc.Get(r.Context(), p, id)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToDTO(d))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(r)
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	var req UpdateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.valid.Struct(&req); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	d, err := h.svc.Update(r.Context(), p, id, UpdateInput(req))
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToDTO(d))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(r)
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.svc.Delete(r.Context(), p, id); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.Empty(w, http.StatusNoContent)
}

func (h *Handler) execute(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(r)
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	var req CommandRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.valid.Struct(&req); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	d, err := h.svc.Execute(r.Context(), p, id, devicectl.Command{
		Type:  devicectl.CommandType(req.Type),
		Value: req.Value,
	})
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToDTO(d))
}
