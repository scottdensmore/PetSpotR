package webfrontend

import (
	"strings"
	"testing"
)

func TestBeaconScannerAssets(t *testing.T) {
	t.Parallel()

	// 1. Verify pet-beacon-scanner.js content
	jsData, err := embeddedFiles.ReadFile("static/js/pet-beacon-scanner.js")
	if err != nil {
		t.Fatalf("failed to read static/js/pet-beacon-scanner.js: %v", err)
	}
	jsContent := string(jsData)
	expectedJSSnippets := []string{
		"class PetBeaconScanner",
		"startScan",
		"stopScan",
		"toggleAudio",
		"injectMockPing",
		"requestLEScan",
		"__mockBluetoothScanner",
		"_smoothRssi",
		"_calculateDistance",
		"_determineProximity",
		"_playGeigerTick",
		"AudioContext",
		"petspotr_beacon_audio_muted",
	}
	for _, snippet := range expectedJSSnippets {
		if !strings.Contains(jsContent, snippet) {
			t.Errorf("static/js/pet-beacon-scanner.js missing snippet %q", snippet)
		}
	}

	// 2. Verify styles.css contains radar hud styles
	cssData, err := embeddedFiles.ReadFile("static/css/styles.css")
	if err != nil {
		t.Fatalf("failed to read static/css/styles.css: %v", err)
	}
	cssContent := string(cssData)
	expectedCSSSnippets := []string{
		".beacon-radar-panel",
		".signal-strength-bar",
		".beacon-pulse-indicator",
		".btn-beacon-action",
		"#beacon-scanner-status",
		"#beacon-distance-display",
		"#beacon-proximity-badge",
		".beacon-ping-marker",
	}
	for _, snippet := range expectedCSSSnippets {
		if !strings.Contains(cssContent, snippet) {
			t.Errorf("static/css/styles.css missing snippet %q", snippet)
		}
	}

	// 3. Verify templates contain beacon scanner markup
	templateFiles := []string{
		"templates/searchparty_modal.html",
		"templates/pets.html",
		"templates/finder_landing.html",
	}
	expectedMarkupSnippets := []string{
		`id="beacon-scanner-container"`,
		`id="beacon-scanner-status"`,
		`id="beacon-distance-display"`,
		`id="beacon-rssi-display"`,
		`id="beacon-proximity-badge"`,
		`id="beacon-rssi-meter"`,
		`id="btn-start-beacon-scan"`,
		`id="btn-toggle-beacon-audio"`,
		`id="btn-log-beacon-sighting"`,
		`id="beacon-aria-announcer"`,
	}
	for _, tmpl := range templateFiles {
		data, err := embeddedFiles.ReadFile(tmpl)
		if err != nil {
			t.Fatalf("failed to read template %s: %v", tmpl, err)
		}
		content := string(data)
		for _, snippet := range expectedMarkupSnippets {
			if !strings.Contains(content, snippet) {
				t.Errorf("%s missing expected snippet %s", tmpl, snippet)
			}
		}
	}

	// 4. Verify script inclusions
	scriptTemplates := []string{
		"templates/searchparty_modal.html",
		"templates/pets.html",
		"templates/finder_landing.html",
		"templates/layout.html",
	}
	for _, tmpl := range scriptTemplates {
		data, err := embeddedFiles.ReadFile(tmpl)
		if err != nil {
			t.Fatalf("failed to read template %s: %v", tmpl, err)
		}
		content := string(data)
		if !strings.Contains(content, "/static/js/pet-beacon-scanner.js") {
			t.Errorf("%s missing script inclusion of /static/js/pet-beacon-scanner.js", tmpl)
		}
	}
}

func TestBeaconHUDIntegrationAssets(t *testing.T) {
	t.Parallel()

	// 1. Verify search-party.js and pet-search-party.js content
	for _, jsFile := range []string{"static/js/search-party.js", "static/js/pet-search-party.js"} {
		jsData, err := embeddedFiles.ReadFile(jsFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", jsFile, err)
		}
		content := string(jsData)
		expectedSnippets := []string{
			"PetBeaconScanner",
			"initBeaconScanner",
			"handleBeaconPing",
			"renderBeaconPingOnMap",
			"handleLogBeaconSightingClick",
			"beacon_ping",
			"btn-start-beacon-scan",
			"btn-toggle-beacon-audio",
			"btn-log-beacon-sighting",
			"beacon-confidence-circle",
			"beacon-pulse-indicator",
			"Collar beacon detected nearby",
		}
		for _, snippet := range expectedSnippets {
			if !strings.Contains(content, snippet) {
				t.Errorf("%s missing expected snippet %q", jsFile, snippet)
			}
		}
	}

	// 2. Verify outbox-sync.js beacon store and queue/flush logic
	outboxData, err := embeddedFiles.ReadFile("static/js/outbox-sync.js")
	if err != nil {
		t.Fatalf("failed to read static/js/outbox-sync.js: %v", err)
	}
	outboxContent := string(outboxData)
	expectedOutboxSnippets := []string{
		"petspotr_beacon_outbox",
		"queueBeaconPing",
		"flushBeaconPings",
		"deleteBeaconPing",
		"getQueuedBeaconPings",
		"/beacon-pings",
	}
	for _, snippet := range expectedOutboxSnippets {
		if !strings.Contains(outboxContent, snippet) {
			t.Errorf("static/js/outbox-sync.js missing expected snippet %q", snippet)
		}
	}
}
