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
