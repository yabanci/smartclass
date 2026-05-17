package schedule

import (
	"net/http"
	"strconv"

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
	r.Post("/", h.create)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
}

func (h *Handler) ClassroomRoutes(r chi.Router) {
	r.Get("/", h.week)
	r.Get("/day/{day}", h.day)
	r.Get("/current", h.current)
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
	starts, err := ParseTimeOfDay(req.StartsAt)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, ErrInvalidTime)
		return
	}
	ends, err := ParseTimeOfDay(req.EndsAt)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, ErrInvalidTime)
		return
	}
	var teacher *uuid.UUID
	if req.TeacherID != nil {
		tid, err := uuid.Parse(*req.TeacherID)
		if err != nil {
			httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
			return
		}
		teacher = &tid
	}
	l, err := h.svc.Create(r.Context(), p, CreateInput{
		ClassroomID: cid, Subject: req.Subject, TeacherID: teacher,
		DayOfWeek: DayOfWeek(req.DayOfWeek), StartsAt: starts, EndsAt: ends, Notes: req.Notes,
	})
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, ToDTO(l))
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
	in := UpdateInput{Subject: req.Subject, Notes: req.Notes, ClearTeacher: req.ClearTeacher}
	if req.DayOfWeek != nil {
		d := DayOfWeek(*req.DayOfWeek)
		in.DayOfWeek = &d
	}
	if req.StartsAt != nil {
		s, err := ParseTimeOfDay(*req.StartsAt)
		if err != nil {
			httpx.WriteError(w, r, h.bundle, ErrInvalidTime)
			return
		}
		in.StartsAt = &s
	}
	if req.EndsAt != nil {
		e, err := ParseTimeOfDay(*req.EndsAt)
		if err != nil {
			httpx.WriteError(w, r, h.bundle, ErrInvalidTime)
			return
		}
		in.EndsAt = &e
	}
	if req.TeacherID != nil {
		tid, err := uuid.Parse(*req.TeacherID)
		if err != nil {
			httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
			return
		}
		in.TeacherID = &tid
	}
	l, err := h.svc.Update(r.Context(), p, id, in)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ToDTO(l))
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

func (h *Handler) week(w http.ResponseWriter, r *http.Request) {
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
	week, err := h.svc.Week(r.Context(), p, cid)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	out := make(map[string][]DTO, len(week))
	for d, lessons := range week {
		dto := make([]DTO, 0, len(lessons))
		for _, l := range lessons {
			dto = append(dto, ToDTO(l))
		}
		out[strconv.Itoa(int(d))] = dto
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) day(w http.ResponseWriter, r *http.Request) {
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
	d, err := strconv.Atoi(chi.URLParam(r, "day"))
	if err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	lessons, err := h.svc.Day(r.Context(), p, cid, DayOfWeek(d))
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	out := make([]DTO, 0, len(lessons))
	for _, l := range lessons {
		out = append(out, ToDTO(l))
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) current(w http.ResponseWriter, r *http.Request) {
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
	l, err := h.svc.Current(r.Context(), p, cid)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	if l == nil {
		httpx.JSON(w, http.StatusOK, nil)
		return
	}
	httpx.JSON(w, http.StatusOK, ToDTO(l))
}
