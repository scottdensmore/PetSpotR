package webfrontend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/i18n"
)

func TestLocaleResolutionMiddleware(t *testing.T) {
	srv := NewDemoServer()

	t.Run("Resolves locale from query param and sets cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pets?lang=es", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		// Check cookie is set
		cookies := rec.Result().Cookies()
		var foundCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == i18n.CookieName {
				foundCookie = c
				break
			}
		}
		if foundCookie == nil {
			t.Fatalf("expected %q cookie to be set", i18n.CookieName)
		}
		if foundCookie.Value != "es" {
			t.Errorf("cookie value = %q, want 'es'", foundCookie.Value)
		}

		// Check body is localized to Spanish
		body := rec.Body.String()
		if !strings.Contains(body, "Directorio de Mascotas") {
			t.Errorf("expected Spanish translation 'Directorio de Mascotas' in body")
		}
	})

	t.Run("Resolves locale from cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pets", nil)
		req.AddCookie(&http.Cookie{
			Name:  i18n.CookieName,
			Value: "vi",
		})
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "Danh mục thú cưng") {
			t.Errorf("expected Vietnamese translation 'Danh mục thú cưng' in body")
		}
	})

	t.Run("Resolves locale from Accept-Language header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pets", nil)
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "宠物名录") {
			t.Errorf("expected Simplified Chinese translation '宠物名录' in body")
		}
	})

	t.Run("Defaults to English when no locale or invalid locale provided", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pets?lang=invalid", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "Pet Directory") {
			t.Errorf("expected English default 'Pet Directory' in body")
		}
	})
}
