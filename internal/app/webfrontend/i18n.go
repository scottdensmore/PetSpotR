package webfrontend

import (
	"context"
	"html/template"
	"net/http"

	"github.com/scottdensmore/petspotr/pkg/i18n"
)

type contextKey string

// LocaleContextKey is the context key for the resolved request locale.
const LocaleContextKey contextKey = "petspotr_locale"

// LocaleFromContext extracts the resolved locale from the request context.
func LocaleFromContext(ctx context.Context) string {
	if ctx == nil {
		return i18n.DefaultLocale
	}
	if val, ok := ctx.Value(LocaleContextKey).(string); ok && val != "" {
		return val
	}
	return i18n.DefaultLocale
}

// templateFuncMap provides i18n and helper functions to HTML templates.
var templateFuncMap = template.FuncMap{
	"t": func(locale, key string, args ...any) string {
		return i18n.Translate(locale, key, args...)
	},
	"tp": func(locale, key string, count int, args ...any) string {
		return i18n.TranslatePlural(locale, key, count, args...)
	},
	"supportedLocales": func() []string {
		return i18n.SupportedLocales
	},
	"localeDisplayName": func(code string) string {
		return i18n.LocaleDisplayName[code]
	},
}

// localeMiddleware inspects query params (?lang=), cookies, and Accept-Language to resolve locale.
func (s *Server) localeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryLang := r.URL.Query().Get("lang")
		var cookieLang string
		if c, err := r.Cookie(i18n.CookieName); err == nil {
			cookieLang = c.Value
		}
		acceptLang := r.Header.Get("Accept-Language")

		resolved := i18n.ResolveLocale(queryLang, cookieLang, acceptLang)

		if queryLang != "" && i18n.IsSupported(i18n.NormalizeLocale(queryLang)) {
			http.SetCookie(w, &http.Cookie{
				Name:     i18n.CookieName,
				Value:    resolved,
				Path:     "/",
				MaxAge:   365 * 24 * 3600,
				HttpOnly: false,
				SameSite: http.SameSiteLaxMode,
			})
		}

		ctx := context.WithValue(r.Context(), LocaleContextKey, resolved)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
