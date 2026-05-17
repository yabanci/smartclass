package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"smartclass/internal/platform/httpx"
	"smartclass/internal/platform/httpx/middleware"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/tokens"
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
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/refresh", h.refresh)
}

// AuthenticatedRoutes mounts endpoints that require a valid access token.
// Logout lives here because the server uses the bearer token to identify
// whose refresh tokens to revoke; we do not accept a userID in the body
// because that would let any caller revoke any user's sessions.
func (h *Handler) AuthenticatedRoutes(r chi.Router) {
	r.Post("/logout", h.logout)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	p, ok := middleware.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, r, h.bundle, httpx.ErrUnauthorized)
		return
	}
	if err := h.svc.Logout(r.Context(), p.UserID); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.valid.Struct(&req); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	res, err := h.svc.Register(r.Context(), RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		FullName: req.FullName,
		Role:     user.Role(req.Role),
		Language: req.Language,
		Phone:    ptrStr(req.Phone),
	})
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"user":   user.ToDTO(res.User),
		"tokens": pairToDTO(res.Tokens),
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.valid.Struct(&req); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	res, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"user":   user.ToDTO(res.User),
		"tokens": pairToDTO(res.Tokens),
	})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, h.bundle, httpx.ErrBadRequest)
		return
	}
	if err := h.valid.Struct(&req); err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	res, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		httpx.WriteError(w, r, h.bundle, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"user":   user.ToDTO(res.User),
		"tokens": pairToDTO(res.Tokens),
	})
}

func pairToDTO(p tokens.Pair) TokenPairDTO {
	return TokenPairDTO{
		Access:           p.Access,
		Refresh:          p.Refresh,
		AccessExpiresAt:  p.AccessExpiresAt,
		RefreshExpiresAt: p.RefreshExpiresAt,
		TokenType:        "Bearer",
	}
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
