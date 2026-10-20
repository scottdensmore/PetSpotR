package webfrontend

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/cameratrap"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type createCameraNodeRequest struct {
	Name                 string              `json:"name"`
	Vendor               domain.CameraVendor `json:"vendor"`
	OwnerName            string              `json:"ownerName"`
	TargetPetID          string              `json:"targetPetId"`
	Latitude             float64             `json:"latitude"`
	Longitude            float64             `json:"longitude"`
	HeadingDegrees       float64             `json:"headingDegrees"`
	FOVDegrees           float64             `json:"fovDegrees"`
	NightVisionIR        bool                `json:"nightVisionIr"`
	AutoPromoteThreshold float64             `json:"autoPromoteThreshold"`
}

type createCameraNodeResponse struct {
	Camera       domain.CameraNode `json:"camera"`
	WebhookToken string            `json:"webhookToken"`
	WebhookURL   string            `json:"webhookUrl"`
}

type cameraWebhookPayload struct {
	SnapshotDataURI string   `json:"snapshotDataUri"`
	ImageBase64     string   `json:"imageBase64"`
	DetectedSpecies string   `json:"detectedSpecies"`
	ExtractedTraits []string `json:"extractedTraits"`
	MotionZone      string   `json:"motionZone"`
	CapturedAt      string   `json:"capturedAt"`
}

// CamerasPageData represents the template data passed to templates/cameras.html.
type CamerasPageData struct {
	Cameras  []domain.CameraNode
	Captures []domain.CameraCapture
	Locale   string
}

func (s *Server) renderTemplate(w http.ResponseWriter, r *http.Request, templateName string, pageData any) {
	tmpl, err := template.New(templateName).Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/"+templateName)
	if err != nil {
		http.Error(w, "Failed to load template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(self), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, pageData)
	}
}

func (s *Server) handleCamerasPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cameras := s.loadCameraNodes(r.Context(), "")
	captures := s.loadCameraCaptures(r.Context(), "", "", false)

	pageData := CamerasPageData{
		Cameras:  cameras,
		Captures: captures,
		Locale:   LocaleFromContext(r.Context()),
	}
	s.renderTemplate(w, r, "cameras.html", pageData)
}

func (s *Server) handleCreateCameraNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createCameraNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCameraJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	vendor := domain.CameraVendor(strings.ToUpper(strings.TrimSpace(string(req.Vendor))))
	if vendor == "" {
		vendor = domain.CameraVendorRing
	}

	fov := req.FOVDegrees
	if fov <= 0 {
		fov = 110.0
	}

	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		writeCameraJSONError(w, http.StatusInternalServerError, "Failed to generate webhook token")
		return
	}
	webhookToken := "cam_tok_" + hex.EncodeToString(randomBytes[:])
	secretSum := sha256.Sum256([]byte(webhookToken))
	secretHash := hex.EncodeToString(secretSum[:])

	seqNano := nextCockpitNano()
	node := domain.CameraNode{
		ID:                   fmt.Sprintf("cam-%d", seqNano),
		Name:                 strings.TrimSpace(req.Name),
		Vendor:               vendor,
		OwnerName:            strings.TrimSpace(req.OwnerName),
		TargetPetID:          strings.TrimSpace(req.TargetPetID),
		Latitude:             req.Latitude,
		Longitude:            req.Longitude,
		HeadingDegrees:       req.HeadingDegrees,
		FOVDegrees:           fov,
		NightVisionIR:        req.NightVisionIR,
		AutoPromoteThreshold: req.AutoPromoteThreshold,
		WebhookSecretHash:    secretHash,
		WebhookTokenPreview:  webhookToken[:14] + "...",
		Status:               domain.CameraStatusActive,
		CreatedAt:            time.Unix(0, seqNano).UTC(),
	}

	if err := node.Validate(); err != nil {
		writeCameraJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	nodeBytes, err := json.Marshal(node)
	if err != nil {
		writeCameraJSONError(w, http.StatusInternalServerError, "Failed to encode camera node")
		return
	}

	if err := s.stateStore.SaveState(r.Context(), store.CollectionCameraNodes, node.ID, nodeBytes); err != nil {
		writeCameraJSONError(w, http.StatusInternalServerError, "Failed to persist camera node")
		return
	}

	publicNode := node
	publicNode.WebhookSecretHash = ""

	resp := createCameraNodeResponse{
		Camera:       publicNode,
		WebhookToken: webhookToken,
		WebhookURL:   "/api/v1/cameras/" + node.ID + "/webhook",
	}

	writeCockpitJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListCameraNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	targetPetID := strings.TrimSpace(r.URL.Query().Get("targetPetId"))
	cameras := s.loadCameraNodes(r.Context(), targetPetID)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"cameras": cameras,
	})
}

func (s *Server) loadCameraNodes(ctx context.Context, targetPetID string) []domain.CameraNode {
	cameras := make([]domain.CameraNode, 0)
	if s.stateStore == nil {
		return cameras
	}
	rawNodes, err := s.stateStore.ListState(ctx, store.CollectionCameraNodes)
	if err != nil {
		return cameras
	}
	for _, b := range rawNodes {
		var node domain.CameraNode
		if err := json.Unmarshal(b, &node); err != nil {
			continue
		}
		if targetPetID != "" && !strings.EqualFold(node.TargetPetID, targetPetID) {
			continue
		}
		node.WebhookSecretHash = ""
		cameras = append(cameras, node)
	}
	sort.Slice(cameras, func(i, j int) bool {
		if !cameras[i].CreatedAt.Equal(cameras[j].CreatedAt) {
			return cameras[i].CreatedAt.After(cameras[j].CreatedAt)
		}
		return cameras[i].ID > cameras[j].ID
	})
	return cameras
}

func (s *Server) handleCameraWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	cameraID := strings.TrimSpace(r.PathValue("id"))
	if cameraID == "" {
		writeCameraJSONError(w, http.StatusNotFound, "Camera node not found")
		return
	}

	camBytes, err := s.stateStore.GetState(r.Context(), store.CollectionCameraNodes, cameraID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCameraJSONError(w, http.StatusNotFound, "Camera node not found")
			return
		}
		writeCameraJSONError(w, http.StatusInternalServerError, "Failed to load camera node")
		return
	}

	var camera domain.CameraNode
	if err := json.Unmarshal(camBytes, &camera); err != nil {
		writeCameraJSONError(w, http.StatusInternalServerError, "Corrupt camera node record")
		return
	}

	if camera.WebhookSecretHash != "" {
		token := strings.TrimSpace(r.Header.Get("X-Camera-Token"))
		if token == "" {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if len(authHeader) > 7 && strings.EqualFold(authHeader[:7], "Bearer ") {
				token = strings.TrimSpace(authHeader[7:])
			}
		}
		if token == "" {
			writeCameraJSONError(w, http.StatusUnauthorized, "Missing X-Camera-Token header")
			return
		}
		sum := sha256.Sum256([]byte(token))
		tokenHash := hex.EncodeToString(sum[:])
		if subtle.ConstantTimeCompare([]byte(tokenHash), []byte(camera.WebhookSecretHash)) != 1 {
			writeCameraJSONError(w, http.StatusUnauthorized, "Invalid camera webhook token")
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, 8*1024*1024)
	var payload cameraWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCameraJSONError(w, http.StatusBadRequest, "Invalid webhook JSON payload")
		return
	}

	snapshotURI := strings.TrimSpace(payload.SnapshotDataURI)
	if snapshotURI == "" && strings.TrimSpace(payload.ImageBase64) != "" {
		snapshotURI = strings.TrimSpace(payload.ImageBase64)
	}

	analysis, err := cameratrap.ProcessEdgeFrame(snapshotURI, camera.LastFrameDHash, camera.TotalCaptures > 0)
	if err != nil {
		writeCameraJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to decode edge frame: %v", err))
		return
	}

	seqNano := nextCockpitNano()
	capturedAt := time.Unix(0, seqNano).UTC()
	if strings.TrimSpace(payload.CapturedAt) != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(payload.CapturedAt)); parseErr == nil {
			capturedAt = parsed.UTC()
		}
	}

	capture := domain.CameraCapture{
		ID:               fmt.Sprintf("cap-%d", seqNano),
		CameraID:         camera.ID,
		CameraName:       camera.Name,
		Vendor:           camera.Vendor,
		TargetPetID:      camera.TargetPetID,
		Latitude:         camera.Latitude,
		Longitude:        camera.Longitude,
		CapturedAt:       capturedAt,
		FrameDHash:       analysis.FrameDHash,
		FrameDHashHex:    analysis.FrameDHashHex,
		HammingDistance:  analysis.HammingDistance,
		IsDuplicate:      analysis.IsDuplicate,
		IsNightVisionIR:  analysis.IsNightVisionIR || camera.NightVisionIR,
		IREnhanced:       analysis.IREnhanced,
		ROI:              analysis.ROI,
		SaliencyScore:    analysis.SaliencyScore,
		SnapshotDataURI:  analysis.SnapshotDataURI,
		CropThumbnailURI: analysis.CropThumbnailURI,
	}

	capture = cameratrap.VerifyCaptureWithOllama(
		r.Context(),
		s.ollamaClient,
		capture,
		payload.DetectedSpecies,
		payload.ExtractedTraits,
	)

	if capture.IsDuplicate {
		capture.VerificationStatus = domain.CaptureStatusDuplicate
		capture.ConfidenceScore = 0.0
	} else {
		capture.VerificationStatus = domain.CaptureStatusPendingReview
		if camera.AutoPromoteThreshold > 0 && capture.ConfidenceScore >= camera.AutoPromoteThreshold {
			if _, promErr := s.promoteCaptureToSighting(r.Context(), &capture); promErr == nil {
				capture.VerificationStatus = domain.CaptureStatusPromoted
			}
		}
	}

	capBytes, err := json.Marshal(capture)
	if err != nil {
		writeCameraJSONError(w, http.StatusInternalServerError, "Failed to encode camera capture")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionCameraCaptures, capture.ID, capBytes); err != nil {
		writeCameraJSONError(w, http.StatusInternalServerError, "Failed to persist camera capture")
		return
	}

	_ = s.stateStore.UpdateState(r.Context(), store.CollectionCameraNodes, camera.ID, func(current []byte) ([]byte, error) {
		var node domain.CameraNode
		if err := json.Unmarshal(current, &node); err != nil {
			return nil, err
		}
		node.TotalCaptures++
		if capture.IsDuplicate {
			node.SuppressedDuplicates++
		} else {
			node.LastFrameDHash = capture.FrameDHash
			node.LastFrameDHashHex = capture.FrameDHashHex
		}
		seenAt := capture.CapturedAt
		node.LastSeenAt = &seenAt
		return json.Marshal(node)
	})

	s.broadcastCameraTrapEvent("camera_capture_received", capture.TargetPetID, map[string]any{
		"type":    "camera_capture_received",
		"capture": capture,
	})

	writeCockpitJSON(w, http.StatusCreated, capture)
}

func (s *Server) handleListCameraCaptures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	cameraID := strings.TrimSpace(q.Get("cameraId"))
	statusFilter := strings.TrimSpace(q.Get("status"))
	excludeDuplicates := strings.EqualFold(strings.TrimSpace(q.Get("excludeDuplicates")), "true")

	captures := s.loadCameraCaptures(r.Context(), cameraID, statusFilter, excludeDuplicates)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"captures": captures,
	})
}

func (s *Server) loadCameraCaptures(ctx context.Context, cameraID, statusFilter string, excludeDuplicates bool) []domain.CameraCapture {
	captures := make([]domain.CameraCapture, 0)
	if s.stateStore == nil {
		return captures
	}
	rawCaptures, err := s.stateStore.ListState(ctx, store.CollectionCameraCaptures)
	if err != nil {
		return captures
	}
	for _, b := range rawCaptures {
		var c domain.CameraCapture
		if err := json.Unmarshal(b, &c); err != nil {
			continue
		}
		if cameraID != "" && c.CameraID != cameraID {
			continue
		}
		if statusFilter != "" && !strings.EqualFold(string(c.VerificationStatus), statusFilter) {
			continue
		}
		if excludeDuplicates && (c.IsDuplicate || c.VerificationStatus == domain.CaptureStatusDuplicate) {
			continue
		}
		captures = append(captures, c)
	}
	sort.Slice(captures, func(i, j int) bool {
		if !captures[i].CapturedAt.Equal(captures[j].CapturedAt) {
			return captures[i].CapturedAt.After(captures[j].CapturedAt)
		}
		return captures[i].ID > captures[j].ID
	})
	return captures
}

func (s *Server) promoteCaptureToSighting(ctx context.Context, capture *domain.CameraCapture) (domain.Sighting, error) {
	if capture == nil {
		return domain.Sighting{}, errors.New("capture is nil")
	}

	now := time.Now().UTC()
	sightingID := generateSightingID()
	photoURL := strings.TrimSpace(capture.CropThumbnailURI)
	if photoURL == "" {
		photoURL = strings.TrimSpace(capture.SnapshotDataURI)
	}

	notes := fmt.Sprintf(
		"[%s Camera Trap: %s] Detected %s (%s)",
		capture.Vendor,
		capture.CameraName,
		capture.DetectedSpecies,
		strings.Join(capture.ExtractedTraits, ", "),
	)

	coords := &domain.LocationPoint{
		Latitude:  capture.Latitude,
		Longitude: capture.Longitude,
	}

	sighting := domain.Sighting{
		ID:                  sightingID,
		SightingID:          sightingID,
		PetID:               capture.TargetPetID,
		LostPetID:           capture.TargetPetID,
		ReportedAt:          now,
		SightedAt:           capture.CapturedAt,
		LocationDescription: fmt.Sprintf("%s (%s)", capture.CameraName, capture.Vendor),
		Coordinates:         coords,
		ImageURL:            photoURL,
		Notes:               notes,
		Status:              domain.SightingStatusActive,
	}

	if s.stateStore != nil {
		sBytes, err := json.Marshal(sighting)
		if err != nil {
			return domain.Sighting{}, err
		}
		if err := s.stateStore.SaveState(ctx, store.SightingsCollection, sighting.ID, sBytes); err != nil {
			return domain.Sighting{}, err
		}
	}

	capture.VerificationStatus = domain.CaptureStatusPromoted
	capture.PromotedSightingID = sighting.ID
	return sighting, nil
}

func (s *Server) handlePromoteCameraCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	captureID := strings.TrimSpace(r.PathValue("id"))
	if captureID == "" {
		writeCameraJSONError(w, http.StatusNotFound, "Capture not found")
		return
	}

	capBytes, err := s.stateStore.GetState(r.Context(), store.CollectionCameraCaptures, captureID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCameraJSONError(w, http.StatusNotFound, "Capture not found")
			return
		}
		writeCameraJSONError(w, http.StatusInternalServerError, "Failed to load capture")
		return
	}

	var capture domain.CameraCapture
	if err := json.Unmarshal(capBytes, &capture); err != nil {
		writeCameraJSONError(w, http.StatusInternalServerError, "Corrupt capture record")
		return
	}

	sighting, err := s.promoteCaptureToSighting(r.Context(), &capture)
	if err != nil {
		writeCameraJSONError(w, http.StatusInternalServerError, "Failed to promote capture to sighting")
		return
	}

	updatedBytes, err := json.Marshal(capture)
	if err != nil {
		writeCameraJSONError(w, http.StatusInternalServerError, "Failed to encode updated capture")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionCameraCaptures, capture.ID, updatedBytes); err != nil {
		writeCameraJSONError(w, http.StatusInternalServerError, "Failed to save updated capture")
		return
	}

	s.broadcastCameraTrapEvent("camera_capture_promoted", capture.TargetPetID, map[string]any{
		"type":     "camera_capture_promoted",
		"capture":  capture,
		"sighting": sighting,
	})

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"capture":  capture,
		"sighting": sighting,
	})
}

func (s *Server) handleDismissCameraCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	captureID := strings.TrimSpace(r.PathValue("id"))
	if captureID == "" {
		writeCameraJSONError(w, http.StatusNotFound, "Capture not found")
		return
	}

	var updatedCapture domain.CameraCapture
	err := s.stateStore.UpdateState(r.Context(), store.CollectionCameraCaptures, captureID, func(current []byte) ([]byte, error) {
		var c domain.CameraCapture
		if err := json.Unmarshal(current, &c); err != nil {
			return nil, err
		}
		c.VerificationStatus = domain.CaptureStatusDismissed
		updatedCapture = c
		return json.Marshal(c)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCameraJSONError(w, http.StatusNotFound, "Capture not found")
			return
		}
		writeCameraJSONError(w, http.StatusInternalServerError, "Failed to dismiss capture")
		return
	}

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"capture": updatedCapture,
	})
}

func (s *Server) handleCamerasStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "cameras")
}

func (s *Server) broadcastCameraTrapEvent(eventType, targetPetID string, payload map[string]any) {
	now := time.Now().UTC()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, nextCockpitNano()),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "cameras",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)

		if targetPetID != "" && targetPetID != "cameras" {
			petEvt := evt
			petEvt.MatchID = targetPetID
			s.reunionHub.Broadcast(petEvt)
		}
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		partyID := targetPetID
		if partyID == "" {
			partyID = "cameras"
		}
		envelope := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: partyID,
			SenderNodeID:  "camera-trap-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(envelope)
	}
}

func writeCameraJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
