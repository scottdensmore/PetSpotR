package webfrontend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestDroneReconAssets(t *testing.T) {
	t.Parallel()
	server := NewTestServer(t)

	// 1. Verify JS asset is served with correct status
	req := httptest.NewRequest(http.MethodGet, "/static/js/drone-recon.js", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for drone-recon.js, got %d", w.Code)
	}

	// 2. Verify modal template renders in /pets page
	req = httptest.NewRequest(http.MethodGet, "/pets", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /pets, got %d", w.Code)
	}
	petsBody := w.Body.String()
	if !strings.Contains(petsBody, "recon-modal") {
		t.Errorf("expected pets page to include recon-modal")
	}
	if !strings.Contains(petsBody, "btn-open-drone-recon") {
		t.Errorf("expected pets page to include btn-open-drone-recon")
	}
	if !strings.Contains(petsBody, "/static/js/drone-recon.js") {
		t.Errorf("expected pets page to include /static/js/drone-recon.js script tag")
	}

	// 3. Verify drone_modal template markup structure
	requiredMarkupSnippets := []string{
		`id="recon-modal"`,
		`role="dialog"`,
		`aria-modal="true"`,
		`id="tab-recon-cockpit"`,
		`id="tab-recon-batch"`,
		`id="recon-video"`,
		`id="recon-reticle-canvas"`,
		`id="recon-timeline-slider"`,
		`id="recon-dropzone"`,
		`id="recon-hotspot-drawer"`,
		`id="btn-confirm-hotspot"`,
		`id="btn-dismiss-hotspot"`,
		`id="hud-attitude"`,
		`id="hud-heading"`,
		`id="hud-altitude"`,
		`id="hud-speed"`,
		`id="hud-battery"`,
		`id="btn-close-recon-modal"`,
	}
	for _, snippet := range requiredMarkupSnippets {
		if !strings.Contains(petsBody, snippet) {
			t.Errorf("pets page missing required modal snippet: %s", snippet)
		}
	}

	// 4. Verify CSS contains radar flame pulse and cockpit styling
	cssData, err := embeddedFiles.ReadFile("static/css/styles.css")
	if err != nil {
		t.Fatalf("failed to read static/css/styles.css: %v", err)
	}
	cssContent := string(cssData)
	requiredCSS := []string{
		"marker-hotspot-pulse",
		"#recon-modal",
		"recon-cockpit",
	}
	for _, snippet := range requiredCSS {
		if !strings.Contains(cssContent, snippet) {
			t.Errorf("styles.css missing required snippet: %s", snippet)
		}
	}
}

func TestDroneReconFinderLanding(t *testing.T) {
	t.Parallel()
	memStore := store.NewMemoryStore()
	server := NewTestServer(t, memStore)

	pet := domain.LostPetRecord{
		PetID:       "pet-recon-landing-1",
		PetName:     "Kona",
		Species:     "Dog",
		Status:      "LOST",
		ReportedAt:  time.Now().UTC(),
		Location:    "Lake Union, Seattle",
		Description: "Chocolate lab with orange collar",
	}
	pBytes, _ := json.Marshal(pet)
	if err := memStore.SaveState(context.Background(), store.LostPetsCollection, pet.PetID, pBytes); err != nil {
		t.Fatalf("failed to save test pet: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/p/"+pet.PetID, nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /p/%s, got %d", pet.PetID, w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "recon-modal") {
		t.Errorf("expected finder landing page to include recon-modal")
	}
	if !strings.Contains(body, "btn-open-drone-recon") {
		t.Errorf("expected finder landing page to include btn-open-drone-recon")
	}
	if !strings.Contains(body, "/static/js/drone-recon.js") {
		t.Errorf("expected finder landing page to include /static/js/drone-recon.js script tag")
	}
}

func TestDroneReconJSIntegrity(t *testing.T) {
	t.Parallel()
	jsData, err := embeddedFiles.ReadFile("static/js/drone-recon.js")
	if err != nil {
		t.Fatalf("failed to read static/js/drone-recon.js: %v", err)
	}
	jsContent := string(jsData)

	expectedSnippets := []string{
		"'use strict';",
		"raycastPixelToGround",
		"computeCameraFrustum",
		"updateHUD",
		"updateMapLayers",
		"updateReticleCanvas",
		"plotHotspotMarker",
		"marker-hotspot-pulse",
		"mesh:thermal-hotspot",
		"/api/v1/recon/telemetry/parse",
		"/api/v1/recon/thermal/scan",
		"/api/v1/recon/hotspots/",
		"CONFIRMED",
		"DISMISSED",
		"btn-open-drone-recon",
		"btn-close-recon-modal",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(jsContent, snippet) {
			t.Errorf("drone-recon.js missing expected snippet %q", snippet)
		}
	}

	// Verify strict CSP compliance: zero eval
	if strings.Contains(jsContent, "eval(") {
		t.Errorf("drone-recon.js violates CSP: contains eval()")
	}
}

func TestDroneReconTemplateCSP(t *testing.T) {
	t.Parallel()
	tmplData, err := embeddedFiles.ReadFile("templates/drone_modal.html")
	if err != nil {
		t.Fatalf("failed to read templates/drone_modal.html: %v", err)
	}
	tmplContent := string(tmplData)

	// Verify zero inline script tags
	if strings.Contains(tmplContent, "<script") {
		t.Errorf("templates/drone_modal.html violates CSP: contains inline <script>")
	}

	// Verify zero inline event handlers (e.g. onclick, onchange, etc.)
	disallowedHandlers := []string{
		"onclick=", "onchange=", "onsubmit=", "onkeydown=", "onkeyup=", "onmouseover=", "onload=", "onerror=",
	}
	for _, handler := range disallowedHandlers {
		if strings.Contains(strings.ToLower(tmplContent), handler) {
			t.Errorf("templates/drone_modal.html violates CSP: contains inline handler %q", handler)
		}
	}
}
