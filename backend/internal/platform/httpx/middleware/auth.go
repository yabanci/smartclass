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
			token := extractToken(r)
			if token == "" {
				httpx.Fail(w, http.StatusUnauthorized, "unauthorized", bundle.T(i18n.LangFrom(r.Context()), "unauthorized"), nil)
				return
			}
			claims, err := issuer.Parse(token)
			if err != nil || claims.Kind != tokens.KindAccess {
				httpx.Fail(w, http.StatusUnauthorized, "unauthorized", bundle.T(i18n.LangFrom(r.Context()), "auth.invalid_token"), nil)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxKeyRole, claims.Role)
			// Mirror the principal into the slot so the outer RequestLogger
			// can include user_id+role on the log line. Downstream
			// PrincipalFrom helper still reads from the immutable ctx values.
			if slot := PrincipalSlotFrom(r.Context()); slot != nil {
				slot.Principal = Principal{UserID: claims.UserID, Role: claims.Role}
				slot.Set = true
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractToken reads the access JWT from the standard Authorization header, and
// falls back to the "access_token" query parameter — required for WebSocket
// upgrades where browsers cannot set custom headers.
func extractToken(r *http.Request) string {
	const prefix = "Bearer "
	if raw := r.Header.Get("Authorization"); strings.HasPrefix(raw, prefix) {
		return strings.TrimPrefix(raw, prefix)
	}
	if q := r.URL.Query().Get("access_token"); q != "" {
		return q
	}
	return ""
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

// WithPrincipalForTest installs a Principal directly in the context, used by
// handler tests that don't go through the real Authn middleware. Production
// code never calls this — use Authn instead.
func WithPrincipalForTest(ctx context.Context, p Principal) context.Context {
	ctx = context.WithValue(ctx, ctxKeyUserID, p.UserID)
	ctx = context.WithValue(ctx, ctxKeyRole, p.Role)
	return ctx
}
