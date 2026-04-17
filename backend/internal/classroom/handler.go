package classroom

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Get("/{id}/members", h.members)
	r.Post("/{id}/members", h.assign)
	r.Delete("/{id}/members/{userId}", h.unassign)
}

func (h *Handler) principal(r *http.Request) (Principal, bool) {
	p, ok := mw.PrincipalFrom(r.Context())
	if !ok {
		return Principal{}, false
	}
	return Principal{UserID: p.UserID, Role: user.Role(p.Role)}, true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p, ok := h.principal(r)
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	items, err := h.svc.ListForPrincipal(r.Context(), p, limit, offset)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	out := make([]DTO, 0, len(items))
	for _, c := range items {
		out = append(out, ToDTO(c))
	}
	httpx.JSON(w, http.StatusOK, out)
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
	c, err := h.svc.Create(r.Context(), CreateInput{
		Name: req.Name, Description: req.Description, CreatedBy: p.UserID,
	})
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, ToDTO(c))
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
	if err := h.svc.Authorize(r.Context(), p, id, false); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	c, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToDTO(c))
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
	c, err := h.svc.Update(r.Context(), p, id, UpdateInput{Name: req.Name, Description: req.Description})
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToDTO(c))
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

func (h *Handler) members(w http.ResponseWriter, r *http.Request) {
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
	mems, err := h.svc.Members(r.Context(), p, id)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	out := make([]string, 0, len(mems))
	for _, id := range mems {
		out = append(out, id.String())
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) assign(w http.ResponseWriter, r *http.Request) {
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
	var req AssignRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.valid.Struct(&req); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	uid, err := uuid.Parse(req.UserID)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.svc.Assign(r.Context(), p, id, uid); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.Empty(w, http.StatusNoContent)
}

func (h *Handler) unassign(w http.ResponseWriter, r *http.Request) {
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
	uid, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.svc.Unassign(r.Context(), p, id, uid); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.Empty(w, http.StatusNoContent)
}
