package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"smartclass/internal/platform/httpx"
	"smartclass/internal/platform/i18n"
	"smartclass/internal/platform/tokens"
)

type ctxKey string

const (
	ctxKeyUserID ctxKey = "userID"
	ctxKeyRole   ctxKey = "userRole"
)

type Principal struct {
	UserID uuid.UUID
	Role   string
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	uid, ok1 := ctx.Value(ctxKeyUserID).(uuid.UUID)
	role, ok2 := ctx.Value(ctxKeyRole).(string)
	if !ok1 || !ok2 {
		return Principal{}, false
	}
	return Principal{UserID: uid, Role: role}, true
}

func Authn(issuer tokens.Issuer, bundle *i18n.Bundle) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get("Authorization")
			const p = "Bearer "
			if !strings.HasPrefix(raw, p) {
				httpx.Fail(w, http.StatusUnauthorized, "unauthorized", bundle.T(i18n.LangFrom(r.Context()), "unauthorized"), nil)
				return
			}
			claims, err := issuer.Parse(strings.TrimPrefix(raw, p))
			if err != nil || claims.Kind != tokens.KindAccess {
				httpx.Fail(w, http.StatusUnauthorized, "unauthorized", bundle.T(i18n.LangFrom(r.Context()), "auth.invalid_token"), nil)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxKeyRole, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(bundle *i18n.Bundle, roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFrom(r.Context())
			if !ok {
				httpx.Fail(w, http.StatusUnauthorized, "unauthorized", bundle.T(i18n.LangFrom(r.Context()), "unauthorized"), nil)
				return
			}
			if _, allow := allowed[p.Role]; !allow {
				httpx.Fail(w, http.StatusForbidden, "forbidden", bundle.T(i18n.LangFrom(r.Context()), "forbidden"), nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
