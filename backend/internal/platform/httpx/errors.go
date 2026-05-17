package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/validation"
)

// errorSlotKey is the context key for the internal error slot written by
// WriteError and read by RequestLogger to emit ERROR-level logs on 5xx.
type errorSlotKey struct{}

// ErrorSlot holds the unhandled error that produced a 5xx response.
// RequestLogger calls ErrorSlotFrom after the handler returns.
type ErrorSlot struct {
	Err error
}

// WithErrorSlot returns a context carrying the slot pointer.
// RequestLogger installs this before calling the handler.
func WithErrorSlot(ctx context.Context, slot *ErrorSlot) context.Context {
	return context.WithValue(ctx, errorSlotKey{}, slot)
}

// ErrorSlotFrom returns the slot, or nil if not installed.
func ErrorSlotFrom(ctx context.Context) *ErrorSlot {
	slot, _ := ctx.Value(errorSlotKey{}).(*ErrorSlot)
	return slot
}

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
		// fmt.Errorf("%w: <raw upstream body>", de) is common for ErrUpstream —
		// the extra context is priceless for debugging (HA's actual response
		// body, network-error text, etc.) but WriteError used to drop it on
		// the floor. Surface it as `details` so the UI can show "Home Assistant
		// не ответил: login_flow 500: {…}" instead of a bare translated string.
		full := err.Error()
		var details any
		if idx := strings.Index(full, ": "); idx > 0 && full[:idx] == de.Code {
			if extra := strings.TrimSpace(full[idx+2:]); extra != "" {
				details = map[string]string{"upstream": extra}
			}
		}
		Fail(w, de.HTTPStatus, de.Code, bundle.T(lang, de.MessageKey), details)
		return
	}

	var ve *validation.Errors
	if errors.As(err, &ve) {
		Fail(w, http.StatusBadRequest, "validation_failed", bundle.T(lang, "validation_failed"), ve.Fields)
		return
	}

	// Unhandled error — will produce 500. Stash it in the context slot so the
	// surrounding RequestLogger middleware can emit an ERROR log with full
	// context (request_id, path, method) without threading a logger into here.
	if slot := ErrorSlotFrom(r.Context()); slot != nil {
		slot.Err = err
	}
	Fail(w, http.StatusInternalServerError, "internal_error", bundle.T(lang, "internal_error"), nil)
}

var (
	ErrUnauthorized = NewDomainError("unauthorized", http.StatusUnauthorized, "unauthorized")
	ErrForbidden    = NewDomainError("forbidden", http.StatusForbidden, "forbidden")
	ErrNotFound     = NewDomainError("not_found", http.StatusNotFound, "not_found")
	ErrBadRequest   = NewDomainError("bad_request", http.StatusBadRequest, "bad_request")
	// WS-specific. Both surface 401 but with distinct body codes so dashboards
	// and clients can tell "client forgot the ticket" from "ticket invalid".
	ErrWSTicketRequired = NewDomainError("ws_ticket_required", http.StatusUnauthorized, "ws.ticket_required")
	ErrWSTicketInvalid  = NewDomainError("ws_ticket_invalid", http.StatusUnauthorized, "ws.ticket_invalid")
)
