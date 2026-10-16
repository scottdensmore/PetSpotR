package webfrontend_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func generateTestCameraFrameDataURI(t *testing.T, nightVision bool, variant int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			if nightVision {
				img.Set(x, y, color.RGBA{R: 25, G: 25, B: 25, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 40, G: 75, B: 45, A: 255})
			}
		}
	}
	offsetX := (variant * 24) % 48
	for y := 36; y < 92; y++ {
		for x := 32 + offsetX; x < 76+offsetX; x++ {
			if nightVision {
				img.Set(x, y, color.RGBA{R: 220, G: 220, B: 220, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 245, G: 190, B: 80, A: 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("failed to encode test jpeg: %v", err)
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestCameraTrap_CreateAndListNodes(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// 1. Validation failures
	invalidReq, _ := json.Marshal(map[string]any{
		"name":     "",
		"vendor":   "RING",
		"latitude": 47.6062,
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/cameras", bytes.NewReader(invalidReq))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for empty camera name, got %d: %s", recInvalid.Code, recInvalid.Body.String())
	}

	// 2. Create first camera node
	createBody1, _ := json.Marshal(map[string]any{
		"name":                 "Alleyway South Gate",
		"vendor":               "ring",
		"ownerName":            "Dana Scully",
		"targetPetId":          "lost-101",
		"latitude":             47.6101,
		"longitude":            -122.3421,
		"headingDegrees":       180.0,
		"fovDegrees":           0, // should default to 110.0
		"nightVisionIr":        true,
		"autoPromoteThreshold": 0.0,
	})
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/cameras", bytes.NewReader(createBody1))
	req1.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec1.Code, rec1.Body.String())
	}

	var createResp1 struct {
		Camera       domain.CameraNode `json:"camera"`
		WebhookToken string            `json:"webhookToken"`
		WebhookURL   string            `json:"webhookUrl"`
	}
	if err := json.Unmarshal(rec1.Body.Bytes(), &createResp1); err != nil {
		t.Fatalf("failed to unmarshal create response: %v", err)
	}

	if !strings.HasPrefix(createResp1.Camera.ID, "cam-") {
		t.Errorf("expected camera ID to start with cam-, got %q", createResp1.Camera.ID)
	}
	if createResp1.Camera.Vendor != domain.CameraVendorRing {
		t.Errorf("expected vendor RING, got %q", createResp1.Camera.Vendor)
	}
	if createResp1.Camera.FOVDegrees != 110.0 {
		t.Errorf("expected default FOVDegrees 110.0, got %v", createResp1.Camera.FOVDegrees)
	}
	if createResp1.Camera.Status != domain.CameraStatusActive {
		t.Errorf("expected status ACTIVE, got %q", createResp1.Camera.Status)
	}
	if !strings.HasPrefix(createResp1.WebhookToken, "cam_tok_") {
		t.Errorf("expected webhookToken to start with cam_tok_, got %q", createResp1.WebhookToken)
	}
	if createResp1.Camera.WebhookTokenPreview != createResp1.WebhookToken[:14]+"..." {
		t.Errorf("expected token preview %q, got %q", createResp1.WebhookToken[:14]+"...", createResp1.Camera.WebhookTokenPreview)
	}
	if createResp1.Camera.WebhookSecretHash != "" {
		t.Errorf("expected WebhookSecretHash to be omitted in response, got %q", createResp1.Camera.WebhookSecretHash)
	}
	expectedURL := "/api/v1/cameras/" + createResp1.Camera.ID + "/webhook"
	if createResp1.WebhookURL != expectedURL {
		t.Errorf("expected webhookUrl %q, got %q", expectedURL, createResp1.WebhookURL)
	}

	time.Sleep(2 * time.Millisecond)

	// 3. Create second camera node for different pet
	createBody2, _ := json.Marshal(map[string]any{
		"name":           "Greenbelt Trail Cam #4",
		"vendor":         "TRAIL_CAM",
		"targetPetId":    "lost-202",
		"latitude":       47.6205,
		"longitude":      -122.3500,
		"headingDegrees": 45.0,
		"fovDegrees":     95.0,
	})
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/cameras", bytes.NewReader(createBody2))
	req2.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for second camera, got %d: %s", rec2.Code, rec2.Body.String())
	}

	// 4. List all cameras (newest first)
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/cameras", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list cameras, got %d", recList.Code)
	}
	var listResp struct {
		Cameras []domain.CameraNode `json:"cameras"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list cameras response: %v", err)
	}
	if len(listResp.Cameras) != 2 {
		t.Fatalf("expected 2 cameras, got %d", len(listResp.Cameras))
	}
	if listResp.Cameras[0].Name != "Greenbelt Trail Cam #4" {
		t.Errorf("expected newest camera first, got %q", listResp.Cameras[0].Name)
	}

	// 5. Filter by targetPetId
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/cameras?targetPetId=lost-101", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered cameras, got %d", recFilter.Code)
	}
	var filterResp struct {
		Cameras []domain.CameraNode `json:"cameras"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filterResp); err != nil {
		t.Fatalf("failed to decode filtered cameras: %v", err)
	}
	if len(filterResp.Cameras) != 1 || filterResp.Cameras[0].ID != createResp1.Camera.ID {
		t.Errorf("unexpected filtered cameras: %+v", filterResp.Cameras)
	}
}

func TestCameraTrap_WebhookAuthAndDeduplication(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Subscribe to SSE "cameras" topic
	sseCh, unsubSSE := server.ReunionHub().Subscribe("cameras")
	defer unsubSSE()

	// Create camera node
	createBody, _ := json.Marshal(map[string]any{
		"name":          "Front Porch Wyze v3",
		"vendor":        "WYZE",
		"targetPetId":   "lost-101",
		"latitude":      47.6150,
		"longitude":     -122.3300,
		"nightVisionIr": true,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/cameras", bytes.NewReader(createBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("failed to create camera: %d", recCreate.Code)
	}
	var created struct {
		Camera       domain.CameraNode `json:"camera"`
		WebhookToken string            `json:"webhookToken"`
		WebhookURL   string            `json:"webhookUrl"`
	}
	_ = json.Unmarshal(recCreate.Body.Bytes(), &created)

	frame1 := generateTestCameraFrameDataURI(t, true, 0)
	webhookBody, _ := json.Marshal(map[string]any{
		"snapshotDataUri": frame1,
		"detectedSpecies": "Dog",
		"extractedTraits": []string{"Golden coat", "Reflective collar"},
		"motionZone":      "driveway",
	})

	// 1. Missing X-Camera-Token -> 401 Unauthorized
	recNoTok := httptest.NewRecorder()
	reqNoTok := httptest.NewRequest(http.MethodPost, created.WebhookURL, bytes.NewReader(webhookBody))
	reqNoTok.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recNoTok, reqNoTok)
	if recNoTok.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized when token is missing, got %d", recNoTok.Code)
	}

	// 2. Invalid X-Camera-Token -> 401 Unauthorized
	recBadTok := httptest.NewRecorder()
	reqBadTok := httptest.NewRequest(http.MethodPost, created.WebhookURL, bytes.NewReader(webhookBody))
	reqBadTok.Header.Set("Content-Type", "application/json")
	reqBadTok.Header.Set("X-Camera-Token", "cam_tok_invalid_secret_value")
	server.ServeHTTP(recBadTok, reqBadTok)
	if recBadTok.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized when token is wrong, got %d", recBadTok.Code)
	}

	// 3. Valid X-Camera-Token initial frame -> 201 Created, IsDuplicate == false
	recHook1 := httptest.NewRecorder()
	reqHook1 := httptest.NewRequest(http.MethodPost, created.WebhookURL, bytes.NewReader(webhookBody))
	reqHook1.Header.Set("Content-Type", "application/json")
	reqHook1.Header.Set("X-Camera-Token", created.WebhookToken)
	server.ServeHTTP(recHook1, reqHook1)
	if recHook1.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for initial frame, got %d: %s", recHook1.Code, recHook1.Body.String())
	}

	var cap1 domain.CameraCapture
	if err := json.Unmarshal(recHook1.Body.Bytes(), &cap1); err != nil {
		t.Fatalf("failed to decode capture 1: %v", err)
	}
	if cap1.IsDuplicate {
		t.Errorf("expected initial frame IsDuplicate == false, got true")
	}
	if cap1.VerificationStatus != domain.CaptureStatusPendingReview {
		t.Errorf("expected PENDING_REVIEW, got %q", cap1.VerificationStatus)
	}
	if cap1.ROI.Width <= 0 || cap1.ROI.Height <= 0 {
		t.Errorf("expected valid ROI dimensions, got %+v", cap1.ROI)
	}
	if !strings.HasPrefix(cap1.CropThumbnailURI, "data:image/jpeg;base64,") {
		t.Errorf("expected JPEG data URI crop thumbnail, got %q", cap1.CropThumbnailURI)
	}
	if !cap1.IsNightVisionIR {
		t.Errorf("expected IsNightVisionIR == true")
	}
	if cap1.ConfidenceScore <= 0 {
		t.Errorf("expected positive ConfidenceScore, got %v", cap1.ConfidenceScore)
	}

	// Verify SSE broadcast on "cameras"
	select {
	case evt := <-sseCh:
		if evt.Type != domain.ReunionEventType("camera_capture_received") {
			t.Errorf("expected SSE event camera_capture_received, got %q", evt.Type)
		}
	case <-time.After(250 * time.Millisecond):
		t.Errorf("timed out waiting for SSE camera_capture_received event")
	}

	// 4. Identical second frame -> IsDuplicate == true, DUPLICATE_SUPPRESSED
	recHook2 := httptest.NewRecorder()
	reqHook2 := httptest.NewRequest(http.MethodPost, created.WebhookURL, bytes.NewReader(webhookBody))
	reqHook2.Header.Set("Content-Type", "application/json")
	reqHook2.Header.Set("Authorization", "Bearer "+created.WebhookToken)
	server.ServeHTTP(recHook2, reqHook2)
	if recHook2.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for second frame, got %d: %s", recHook2.Code, recHook2.Body.String())
	}

	var cap2 domain.CameraCapture
	if err := json.Unmarshal(recHook2.Body.Bytes(), &cap2); err != nil {
		t.Fatalf("failed to decode capture 2: %v", err)
	}
	if !cap2.IsDuplicate {
		t.Errorf("expected identical second frame to be marked IsDuplicate == true")
	}
	if cap2.VerificationStatus != domain.CaptureStatusDuplicate {
		t.Errorf("expected status %q, got %q", domain.CaptureStatusDuplicate, cap2.VerificationStatus)
	}
	if cap2.ConfidenceScore != 0.0 {
		t.Errorf("expected duplicate ConfidenceScore == 0.0, got %v", cap2.ConfidenceScore)
	}

	// Verify camera node counters updated
	recCameras := httptest.NewRecorder()
	reqCameras := httptest.NewRequest(http.MethodGet, "/api/v1/cameras", nil)
	server.ServeHTTP(recCameras, reqCameras)
	var camList struct {
		Cameras []domain.CameraNode `json:"cameras"`
	}
	_ = json.Unmarshal(recCameras.Body.Bytes(), &camList)
	if len(camList.Cameras) != 1 {
		t.Fatalf("expected 1 camera, got %d", len(camList.Cameras))
	}
	if camList.Cameras[0].TotalCaptures != 2 {
		t.Errorf("expected TotalCaptures == 2, got %d", camList.Cameras[0].TotalCaptures)
	}
	if camList.Cameras[0].SuppressedDuplicates != 1 {
		t.Errorf("expected SuppressedDuplicates == 1, got %d", camList.Cameras[0].SuppressedDuplicates)
	}
	if camList.Cameras[0].LastSeenAt == nil {
		t.Errorf("expected LastSeenAt to be set")
	}

	// 5. List captures with and without excludeDuplicates
	recCapsAll := httptest.NewRecorder()
	reqCapsAll := httptest.NewRequest(http.MethodGet, "/api/v1/cameras/captures?cameraId="+created.Camera.ID, nil)
	server.ServeHTTP(recCapsAll, reqCapsAll)
	if recCapsAll.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list captures, got %d", recCapsAll.Code)
	}
	var allCaps struct {
		Captures []domain.CameraCapture `json:"captures"`
	}
	_ = json.Unmarshal(recCapsAll.Body.Bytes(), &allCaps)
	if len(allCaps.Captures) != 2 {
		t.Fatalf("expected 2 total captures, got %d", len(allCaps.Captures))
	}

	recCapsNonDup := httptest.NewRecorder()
	reqCapsNonDup := httptest.NewRequest(http.MethodGet, "/api/v1/cameras/captures?excludeDuplicates=true", nil)
	server.ServeHTTP(recCapsNonDup, reqCapsNonDup)
	var nonDupCaps struct {
		Captures []domain.CameraCapture `json:"captures"`
	}
	_ = json.Unmarshal(recCapsNonDup.Body.Bytes(), &nonDupCaps)
	if len(nonDupCaps.Captures) != 1 || nonDupCaps.Captures[0].ID != cap1.ID {
		t.Errorf("expected 1 non-duplicate capture (%s), got %+v", cap1.ID, nonDupCaps.Captures)
	}
}

func TestCameraTrap_PromoteDismissAndAutoPromote(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// 1. Create manual review camera
	createBody, _ := json.Marshal(map[string]any{
		"name":                 "Backyard Nest Cam",
		"vendor":               "NEST",
		"targetPetId":          "lost-101",
		"latitude":             47.6062,
		"longitude":            -122.3321,
		"autoPromoteThreshold": 0.0,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/cameras", bytes.NewReader(createBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	var created struct {
		Camera       domain.CameraNode `json:"camera"`
		WebhookToken string            `json:"webhookToken"`
		WebhookURL   string            `json:"webhookUrl"`
	}
	_ = json.Unmarshal(recCreate.Body.Bytes(), &created)

	// Ingest frame 1
	frame1 := generateTestCameraFrameDataURI(t, false, 0)
	body1, _ := json.Marshal(map[string]any{
		"snapshotDataUri": frame1,
		"detectedSpecies": "Dog",
		"extractedTraits": []string{"Golden retriever build", "Red bandana"},
	})
	recHook1 := httptest.NewRecorder()
	reqHook1 := httptest.NewRequest(http.MethodPost, created.WebhookURL, bytes.NewReader(body1))
	reqHook1.Header.Set("Content-Type", "application/json")
	reqHook1.Header.Set("X-Camera-Token", created.WebhookToken)
	server.ServeHTTP(recHook1, reqHook1)
	if recHook1.Code != http.StatusCreated {
		t.Fatalf("failed to ingest frame 1: %d", recHook1.Code)
	}
	var cap1 domain.CameraCapture
	_ = json.Unmarshal(recHook1.Body.Bytes(), &cap1)

	// Promote cap1
	recPromote := httptest.NewRecorder()
	reqPromote := httptest.NewRequest(http.MethodPost, "/api/v1/cameras/captures/"+cap1.ID+"/promote", nil)
	server.ServeHTTP(recPromote, reqPromote)
	if recPromote.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for promote, got %d: %s", recPromote.Code, recPromote.Body.String())
	}

	var promoteResp struct {
		Capture  domain.CameraCapture `json:"capture"`
		Sighting domain.Sighting      `json:"sighting"`
	}
	if err := json.Unmarshal(recPromote.Body.Bytes(), &promoteResp); err != nil {
		t.Fatalf("failed to unmarshal promote response: %v", err)
	}
	if promoteResp.Capture.VerificationStatus != domain.CaptureStatusPromoted {
		t.Errorf("expected capture status PROMOTED, got %q", promoteResp.Capture.VerificationStatus)
	}
	if promoteResp.Capture.PromotedSightingID == "" || promoteResp.Capture.PromotedSightingID != promoteResp.Sighting.ID {
		t.Errorf("expected matching PromotedSightingID, got capture=%q sighting=%q", promoteResp.Capture.PromotedSightingID, promoteResp.Sighting.ID)
	}
	if !strings.Contains(promoteResp.Sighting.Notes, "[NEST Camera Trap: Backyard Nest Cam] Detected DOG (Golden retriever build, Red bandana)") {
		t.Errorf("unexpected sighting notes: %q", promoteResp.Sighting.Notes)
	}

	// Verify sighting persisted in store.SightingsCollection
	storedSightingBytes, err := memStore.GetState(context.Background(), store.SightingsCollection, promoteResp.Sighting.ID)
	if err != nil {
		t.Fatalf("expected promoted sighting %q in store.SightingsCollection: %v", promoteResp.Sighting.ID, err)
	}
	var storedSighting domain.Sighting
	if err := json.Unmarshal(storedSightingBytes, &storedSighting); err != nil {
		t.Fatalf("failed to unmarshal stored sighting: %v", err)
	}
	if storedSighting.PetID != "lost-101" {
		t.Errorf("expected stored sighting PetID lost-101, got %q", storedSighting.PetID)
	}

	// Ingest distinct frame 2 and dismiss it
	frame2 := generateTestCameraFrameDataURI(t, false, 1)
	body2, _ := json.Marshal(map[string]any{
		"imageBase64":     frame2,
		"detectedSpecies": "Cat",
	})
	recHook2 := httptest.NewRecorder()
	reqHook2 := httptest.NewRequest(http.MethodPost, created.WebhookURL, bytes.NewReader(body2))
	reqHook2.Header.Set("Content-Type", "application/json")
	reqHook2.Header.Set("X-Camera-Token", created.WebhookToken)
	server.ServeHTTP(recHook2, reqHook2)
	var cap2 domain.CameraCapture
	_ = json.Unmarshal(recHook2.Body.Bytes(), &cap2)

	recDismiss := httptest.NewRecorder()
	reqDismiss := httptest.NewRequest(http.MethodPost, "/api/v1/cameras/captures/"+cap2.ID+"/dismiss", nil)
	server.ServeHTTP(recDismiss, reqDismiss)
	if recDismiss.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for dismiss, got %d: %s", recDismiss.Code, recDismiss.Body.String())
	}
	var dismissResp struct {
		Capture domain.CameraCapture `json:"capture"`
	}
	_ = json.Unmarshal(recDismiss.Body.Bytes(), &dismissResp)
	if dismissResp.Capture.VerificationStatus != domain.CaptureStatusDismissed {
		t.Errorf("expected capture status DISMISSED, got %q", dismissResp.Capture.VerificationStatus)
	}

	// 2. Create camera with AutoPromoteThreshold = 0.55 and verify auto-promotion
	autoCreateBody, _ := json.Marshal(map[string]any{
		"name":                 "Auto-Promote Frigate Node",
		"vendor":               "FRIGATE_RTSP",
		"targetPetId":          "lost-105",
		"latitude":             47.6500,
		"longitude":            -122.3500,
		"autoPromoteThreshold": 0.55,
	})
	recAutoCreate := httptest.NewRecorder()
	reqAutoCreate := httptest.NewRequest(http.MethodPost, "/api/v1/cameras", bytes.NewReader(autoCreateBody))
	reqAutoCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recAutoCreate, reqAutoCreate)
	var autoCreated struct {
		Camera       domain.CameraNode `json:"camera"`
		WebhookToken string            `json:"webhookToken"`
		WebhookURL   string            `json:"webhookUrl"`
	}
	_ = json.Unmarshal(recAutoCreate.Body.Bytes(), &autoCreated)

	recAutoHook := httptest.NewRecorder()
	reqAutoHook := httptest.NewRequest(http.MethodPost, autoCreated.WebhookURL, bytes.NewReader(body1))
	reqAutoHook.Header.Set("Content-Type", "application/json")
	reqAutoHook.Header.Set("X-Camera-Token", autoCreated.WebhookToken)
	server.ServeHTTP(recAutoHook, reqAutoHook)
	if recAutoHook.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for auto-promote webhook, got %d: %s", recAutoHook.Code, recAutoHook.Body.String())
	}
	var autoCap domain.CameraCapture
	_ = json.Unmarshal(recAutoHook.Body.Bytes(), &autoCap)
	if autoCap.VerificationStatus != domain.CaptureStatusPromoted {
		t.Errorf("expected auto-promoted status PROMOTED, got %q (confidence=%v)", autoCap.VerificationStatus, autoCap.ConfidenceScore)
	}
	if autoCap.PromotedSightingID == "" {
		t.Errorf("expected auto-promoted capture to have PromotedSightingID")
	}
	if _, err := memStore.GetState(context.Background(), store.SightingsCollection, autoCap.PromotedSightingID); err != nil {
		t.Errorf("expected auto-promoted sighting in store.SightingsCollection: %v", err)
	}
}

func TestCameraTrap_StreamAndPage(t *testing.T) {
	memStore := store.NewMemoryStore()
	hub := webfrontend.NewReunionHub()
	server := webfrontend.NewServerWithOptions(memStore, webfrontend.ServerOptions{
		ReunionHub:               hub,
		AllowPrivilegedMutations: true,
	})
	ts := httptest.NewServer(server)
	t.Cleanup(func() {
		ts.Close()
		server.Close()
	})

	// 1. GET /cameras page
	respPage, err := ts.Client().Get(ts.URL + "/cameras")
	if err != nil {
		t.Fatalf("failed to GET /cameras: %v", err)
	}
	_ = respPage.Body.Close()
	if respPage.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /cameras, got %d", respPage.StatusCode)
	}

	// 2. Connect to SSE /api/v1/cameras/stream
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reqStream, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/cameras/stream", nil)
	if err != nil {
		t.Fatalf("failed to create stream request: %v", err)
	}
	respStream, err := ts.Client().Do(reqStream)
	if err != nil {
		t.Fatalf("failed to connect to cameras stream: %v", err)
	}
	defer respStream.Body.Close()

	if respStream.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for stream, got %d", respStream.StatusCode)
	}

	reader := bufio.NewReader(respStream.Body)
	firstLine, err := readLineWithTimeout(reader, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read initial SSE comment: %v", err)
	}
	if strings.TrimRight(firstLine, "\r\n") != ": connected" {
		t.Errorf("expected ': connected', got %q", firstLine)
	}
}
