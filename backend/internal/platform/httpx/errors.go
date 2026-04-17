package httpx

import (
	"errors"
	"net/http"

	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/validation"
)

type DomainError struct {
	Code       string
	HTTPStatus int
	MessageKey string
}

func (e *DomainError) Error() string { return e.Code }

func NewDomainError(code string, status int, msgKey string) *DomainError {
	return &DomainError{Code: code, HTTPStatus: status, MessageKey: msgKey}
}

func WriteError(w http.ResponseWriter, r *http.Request, bundle *i18n.Bundle, err error) {
	lang := i18n.LangFrom(r.Context())

	var de *DomainError
	if errors.As(err, &de) {
		Fail(w, de.HTTPStatus, de.Code, bundle.T(lang, de.MessageKey), nil)
		return
	}

	var ve *validation.Errors
	if errors.As(err, &ve) {
		Fail(w, http.StatusBadRequest, "validation_failed", bundle.T(lang, "validation_failed"), ve.Fields)
		return
	}

	Fail(w, http.StatusInternalServerError, "internal_error", bundle.T(lang, "internal_error"), nil)
}

var (
	ErrUnauthorized = NewDomainError("unauthorized", http.StatusUnauthorized, "unauthorized")
	ErrForbidden    = NewDomainError("forbidden", http.StatusForbidden, "forbidden")
	ErrNotFound     = NewDomainError("not_found", http.StatusNotFound, "not_found")
	ErrBadRequest   = NewDomainError("bad_request", http.StatusBadRequest, "bad_request")
)
