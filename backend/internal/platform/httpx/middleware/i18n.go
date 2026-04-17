package middleware

import (
	"net/http"

	"smartclass/internal/platform/i18n"
)

func Language(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Accept-Language")
		if q := r.URL.Query().Get("lang"); q != "" {
			raw = q
		}
		lang := i18n.ParseLang(raw)
		ctx := i18n.WithLang(r.Context(), lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
