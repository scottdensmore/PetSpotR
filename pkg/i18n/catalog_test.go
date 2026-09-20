package i18n_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/i18n"
)

func TestSupportedLocales(t *testing.T) {
	expected := []string{"en", "es", "vi", "zh-CN", "tl"}
	for _, loc := range expected {
		if !i18n.IsSupported(loc) {
			t.Errorf("expected locale %q to be supported", loc)
		}
	}
	if i18n.IsSupported("fr") {
		t.Errorf("locale 'fr' should not be supported")
	}
}

func TestNormalizeLocale(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"en", "en"},
		{"EN", "en"},
		{"en-US", "en"},
		{"es", "es"},
		{"es-MX", "es"},
		{"vi", "vi"},
		{"vi-VN", "vi"},
		{"zh-CN", "zh-CN"},
		{"zh-cn", "zh-CN"},
		{"zh", "zh-CN"},
		{"tl", "tl"},
		{"tl-PH", "tl"},
		{"unknown", "en"},
		{"", "en"},
	}

	for _, c := range cases {
		got := i18n.NormalizeLocale(c.input)
		if got != c.expected {
			t.Errorf("NormalizeLocale(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestResolveLocale(t *testing.T) {
	cases := []struct {
		query    string
		cookie   string
		accept   string
		expected string
	}{
		{"es", "vi", "zh-CN", "es"},
		{"", "vi", "zh-CN", "vi"},
		{"", "", "zh-CN,zh;q=0.9,en;q=0.8", "zh-CN"},
		{"", "", "tl;q=0.8, es;q=0.9", "es"},
		{"", "", "fr-FR,fr;q=0.9", "en"},
		{"", "", "", "en"},
		{"invalid", "also-invalid", "", "en"},
	}

	for _, c := range cases {
		got := i18n.ResolveLocale(c.query, c.cookie, c.accept)
		if got != c.expected {
			t.Errorf("ResolveLocale(query=%q, cookie=%q, accept=%q) = %q, want %q",
				c.query, c.cookie, c.accept, got, c.expected)
		}
	}
}

func TestTranslate(t *testing.T) {
	// English lookup
	if got := i18n.Translate("en", "nav.home"); got != "Home" {
		t.Errorf("Translate(en, nav.home) = %q, want 'Home'", got)
	}

	// Spanish lookup
	if got := i18n.Translate("es", "nav.home"); got != "Inicio" {
		t.Errorf("Translate(es, nav.home) = %q, want 'Inicio'", got)
	}

	// Vietnamese lookup
	if got := i18n.Translate("vi", "nav.home"); got != "Trang chủ" {
		t.Errorf("Translate(vi, nav.home) = %q, want 'Trang chủ'", got)
	}

	// Simplified Chinese lookup
	if got := i18n.Translate("zh-CN", "nav.home"); got != "首页" {
		t.Errorf("Translate(zh-CN, nav.home) = %q, want '首页'", got)
	}

	// Tagalog lookup
	if got := i18n.Translate("tl", "nav.home"); got != "Tahanan" {
		t.Errorf("Translate(tl, nav.home) = %q, want 'Tahanan'", got)
	}

	// Fallback to English when key not found in locale
	if got := i18n.Translate("es", "test.english_only"); got != "Only in English" {
		t.Errorf("Translate(es, test.english_only) = %q, want 'Only in English'", got)
	}

	// Fallback to key when key missing completely
	if got := i18n.Translate("en", "missing.key"); got != "missing.key" {
		t.Errorf("Translate(en, missing.key) = %q, want 'missing.key'", got)
	}

	// String interpolation
	if got := i18n.Translate("en", "aria.language_changed", "Spanish"); got != "Language changed to Spanish" {
		t.Errorf("Translate(en, aria.language_changed, Spanish) = %q, want 'Language changed to Spanish'", got)
	}
}

func TestTranslatePlural(t *testing.T) {
	if got := i18n.TranslatePlural("en", "sighting.count", 1, 1); got != "1 sighting" {
		t.Errorf("TranslatePlural(en, 1) = %q, want '1 sighting'", got)
	}
	if got := i18n.TranslatePlural("en", "sighting.count", 5, 5); got != "5 sightings" {
		t.Errorf("TranslatePlural(en, 5) = %q, want '5 sightings'", got)
	}
	if got := i18n.TranslatePlural("es", "sighting.count", 1, 1); got != "1 avistamiento" {
		t.Errorf("TranslatePlural(es, 1) = %q, want '1 avistamiento'", got)
	}
	if got := i18n.TranslatePlural("es", "sighting.count", 2, 2); got != "2 avistamientos" {
		t.Errorf("TranslatePlural(es, 2) = %q, want '2 avistamientos'", got)
	}
	// Plural without args should auto-fill count
	if got := i18n.TranslatePlural("en", "sighting.count", 3); got != "3 sightings" {
		t.Errorf("TranslatePlural(en, 3) without args = %q, want '3 sightings'", got)
	}
	// Fallback to default dictionary
	if got := i18n.TranslatePlural("fr", "sighting.count", 2, 2); got != "2 sightings" {
		t.Errorf("TranslatePlural(fr, 2) = %q, want '2 sightings'", got)
	}
	// Fallback to non-pluralized key
	if got := i18n.TranslatePlural("en", "nav.home", 1); got != "Home" {
		t.Errorf("TranslatePlural(en, nav.home) = %q, want 'Home'", got)
	}
}
