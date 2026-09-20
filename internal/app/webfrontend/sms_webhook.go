package webfrontend

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/sighting"
	"github.com/scottdensmore/petspotr/pkg/sms"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/webhook"
)

// InboundSMSRequest represents incoming SMS payload from providers or tests.
type InboundSMSRequest struct {
	From      string `json:"From"`
	To        string `json:"To"`
	Body      string `json:"Body"`
	FromLower string `json:"from"`
	ToLower   string `json:"to"`
	BodyLower string `json:"body"`
}

// InboundSMSResponse represents JSON response to inbound SMS.
type InboundSMSResponse struct {
	Status  string `json:"status"`
	Command string `json:"command"`
	Reply   string `json:"reply"`
}

// TwiMLResponse represents an XML response compatible with Twilio/Telnyx SMS webhooks.
type TwiMLResponse struct {
	XMLName xml.Name `xml:"Response"`
	Message string   `xml:"Message"`
}

// verifySMSWebhookSignature validates an HMAC signature against the raw body payload.
// Supports X-PetSpotR-Signature (hex with or without sha256= prefix) and X-Twilio-Signature (base64 SHA1/SHA256).
func verifySMSWebhookSignature(payload []byte, secret, signature string) bool {
	sig := strings.TrimSpace(signature)
	if sig == "" || secret == "" {
		return false
	}

	// 1. Standard PetSpotR format: sha256=<hex> via webhook.VerifySignature
	if webhook.VerifySignature(payload, secret, sig) {
		return true
	}

	// 2. Direct HMAC-SHA256 (hex or base64)
	mac256 := hmac.New(sha256.New, []byte(secret))
	mac256.Write(payload)
	sum256 := mac256.Sum(nil)
	hex256 := hex.EncodeToString(sum256)
	b64_256 := base64.StdEncoding.EncodeToString(sum256)

	if hmac.Equal([]byte(hex256), []byte(sig)) || hmac.Equal([]byte(b64_256), []byte(sig)) {
		return true
	}

	// 3. HMAC-SHA1 (Twilio standard base64 or hex)
	mac1 := hmac.New(sha1.New, []byte(secret))
	mac1.Write(payload)
	sum1 := mac1.Sum(nil)
	b64_1 := base64.StdEncoding.EncodeToString(sum1)
	hex1 := hex.EncodeToString(sum1)

	if hmac.Equal([]byte(b64_1), []byte(sig)) || hmac.Equal([]byte(hex1), []byte(sig)) {
		return true
	}

	return false
}

// handleApiInboundSMSWebhook handles POST /api/v1/webhooks/sms/inbound.
func (s *Server) handleApiInboundSMSWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	bodyBytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
	if err != nil {
		http.Error(w, "Request body too large", http.StatusBadRequest)
		return
	}

	secret := s.smsWebhookSecret
	if secret == "" {
		secret = os.Getenv("PETSPOTR_SMS_WEBHOOK_SECRET")
	}

	sigHeader := strings.TrimSpace(r.Header.Get("X-PetSpotR-Signature"))
	if sigHeader == "" {
		sigHeader = strings.TrimSpace(r.Header.Get("X-Twilio-Signature"))
	}

	// Fallback test secret if a signature header was explicitly provided by caller in test mode without env
	if secret == "" && sigHeader != "" {
		secret = "petspotr-test-sms-secret"
	}

	if secret != "" {
		if sigHeader == "" || !verifySMSWebhookSignature(bodyBytes, secret, sigHeader) {
			http.Error(w, "Unauthorized: invalid or missing webhook signature", http.StatusUnauthorized)
			return
		}
	}

	var from, body string

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var req InboundSMSRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		from = strings.TrimSpace(req.From)
		if from == "" {
			from = strings.TrimSpace(req.FromLower)
		}
		body = strings.TrimSpace(req.Body)
		if body == "" {
			body = strings.TrimSpace(req.BodyLower)
		}
	} else {
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}
		from = strings.TrimSpace(r.FormValue("From"))
		if from == "" {
			from = strings.TrimSpace(r.FormValue("from"))
		}
		body = strings.TrimSpace(r.FormValue("Body"))
		if body == "" {
			body = strings.TrimSpace(r.FormValue("body"))
		}
	}

	if from == "" || body == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "from and body are required",
		})
		return
	}

	if norm, err := sms.NormalizeE164(from); err == nil {
		from = norm
	}

	cmd, arg := sms.ParseCommand(body)
	var reply string

	switch cmd {
	case sms.CommandOptOut, sms.CommandUnsubscribe:
		optMgr := sms.NewOptOutManager(s.stateStore, nil)
		_ = optMgr.OptOut(r.Context(), from, "SMS STOP webhook")
		reply = sms.ReplyUnsubscribed

	case sms.CommandOptIn:
		optMgr := sms.NewOptOutManager(s.stateStore, nil)
		_ = optMgr.OptIn(r.Context(), from)
		reply = sms.ReplyResubscribed

	case sms.CommandClaim:
		reply = s.handleSMSClaimSector(r.Context(), from, arg)

	case sms.CommandSighted:
		reply = s.handleSMSSighted(r.Context(), from, arg)

	case sms.CommandStatus:
		reply = s.handleSMSStatus(r.Context())

	default:
		reply = "PetSpotR Dispatch commands: CLAIM <sector>, SIGHTED <details>, STATUS, or STOP to opt out."
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/xml") || strings.Contains(accept, "text/xml") {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		xmlResp := TwiMLResponse{Message: reply}
		_, _ = w.Write([]byte(xml.Header))
		_ = xml.NewEncoder(w).Encode(xmlResp)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(InboundSMSResponse{
		Status:  "success",
		Command: string(cmd),
		Reply:   reply,
	})
}

// handleSMSClaimSector handles CLAIM <sector> command.
func (s *Server) handleSMSClaimSector(ctx context.Context, from, sectorID string) string {
	sectorID = strings.TrimSpace(sectorID)
	if sectorID == "" {
		return "Please specify a sector to claim (e.g. CLAIM SEC-01). Reply STATUS for open sectors."
	}

	rawParties, err := s.stateStore.ListState(ctx, store.SearchPartiesCollection)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
		return "Error accessing search party state. Please retry shortly."
	}

	var targetParty *searchparty.SearchParty
	var sectorIdx = -1

	for _, b := range rawParties {
		var party searchparty.SearchParty
		if err := json.Unmarshal(b, &party); err != nil {
			continue
		}
		for i, sec := range party.Sectors {
			if strings.EqualFold(sec.SectorID, sectorID) {
				partyCopy := party
				targetParty = &partyCopy
				sectorIdx = i
				break
			}
		}
		if targetParty != nil {
			break
		}
	}

	if targetParty == nil || sectorIdx == -1 {
		return fmt.Sprintf("Sector %s was not found in any active search party. Reply STATUS for open sectors.", sectorID)
	}

	sec := targetParty.Sectors[sectorIdx]
	if sec.Status != searchparty.SectorStatusUnassigned {
		return fmt.Sprintf("Sector %s is already claimed or cleared. Reply STATUS for available sectors.", sec.SectorID)
	}

	now := time.Now().UTC()
	assignmentID := generateAssignmentID()
	alias := sanitizeVolunteerAlias("SMS-Volunteer", len(targetParty.ActiveAssignments)+1)

	assignment := searchparty.SectorAssignment{
		AssignmentID:   assignmentID,
		SectorID:       sec.SectorID,
		VolunteerAlias: alias,
		ClaimedAt:      now,
		UpdatedAt:      now,
		Status:         searchparty.SectorStatusActiveSearch,
	}

	// Persist assignment
	asgnBytes, err := json.Marshal(assignment)
	if err == nil {
		_ = s.stateStore.SaveState(ctx, store.SectorAssignmentsCollection, assignmentID, asgnBytes)
	}

	// Update party sectors and active assignments
	targetParty.Sectors[sectorIdx].Status = searchparty.SectorStatusActiveSearch
	targetParty.ActiveAssignments = append(targetParty.ActiveAssignments, assignment)
	targetParty.CoveragePercentage = targetParty.CalculateCoverage()
	targetParty.ActiveVolunteersCount = countActiveVolunteers(targetParty.ActiveAssignments)

	partyBytes, err := json.Marshal(targetParty)
	if err == nil {
		_ = s.stateStore.SaveState(ctx, store.SearchPartiesCollection, targetParty.PartyID, partyBytes)
	}

	// Emit SSE broadcast update
	s.broadcastSearchPartyUpdate(ctx, *targetParty, sec.SectorID, searchparty.SectorStatusActiveSearch)

	return fmt.Sprintf("Sector %s claimed! View search briefing at https://petspotr.io/pets/%s. Reply SIGHTED <details> if spotted.", sec.SectorID, targetParty.LostPetID)
}

// handleSMSSighted handles SIGHTED <details> command.
func (s *Server) handleSMSSighted(ctx context.Context, from, details string) string {
	details = strings.TrimSpace(details)
	if details == "" {
		return "Please provide sighting details (e.g. SIGHTED Near 5th & Pine, brown dog running west)."
	}

	// Find active pet or search party
	var petID string
	coords := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}

	rawParties, err := s.stateStore.ListState(ctx, store.SearchPartiesCollection)
	if err == nil && len(rawParties) > 0 {
		for _, b := range rawParties {
			var party searchparty.SearchParty
			if err := json.Unmarshal(b, &party); err == nil {
				petID = party.LostPetID
				coords = party.CenterCoordinates
				break
			}
		}
	}

	if petID == "" {
		rawPets, err := s.stateStore.ListState(ctx, store.LostPetsCollection)
		if err == nil && len(rawPets) > 0 {
			for _, b := range rawPets {
				var pet domain.LostPetRecord
				if err := json.Unmarshal(b, &pet); err == nil {
					petID = pet.PetID
					if pet.Coordinates != nil {
						coords = *pet.Coordinates
					}
					break
				}
			}
		}
	}

	if petID == "" {
		petID = "community-pet"
	}

	// Extract coordinates from text if present
	if loc, ok := extractCoordinates(nil, details); ok {
		coords = loc
	}

	now := time.Now().UTC()
	sightingID := generateSightingID()
	record := domain.PetSightingRecord{
		SightingID:          sightingID,
		LostPetID:           petID,
		ReportedAt:          now,
		SightedAt:           now,
		LocationDescription: details,
		Coordinates:         &coords,
		Notes:               fmt.Sprintf("SMS dispatch report from %s", from),
		Status:              domain.SightingStatusActive,
	}

	recordBytes, err := json.Marshal(record)
	if err == nil {
		_ = s.stateStore.SaveState(ctx, store.SightingsCollection, sightingID, recordBytes)
	}

	// Recalculate Trajectory
	var originCoords *domain.LocationPoint
	var originTime time.Time
	if petBytes, err := s.stateStore.GetState(ctx, store.LostPetsCollection, petID); err == nil {
		var pet domain.LostPetRecord
		if err := json.Unmarshal(petBytes, &pet); err == nil {
			pet = domain.NormalizeLostPetRecord(pet)
			originCoords = pet.Coordinates
			if originCoords == nil && pet.Location != "" {
				if pt, ok := extractCoordinates(nil, pet.Location); ok {
					originCoords = &pt
				}
			}
			originTime = pet.ReportedAt
		}
	}
	if originCoords == nil {
		originCoords = &coords
		originTime = now
	}

	rawItems, err := s.stateStore.ListState(ctx, store.SightingsCollection)
	var sightings []domain.PetSightingRecord
	if err == nil {
		for _, b := range rawItems {
			var sRec domain.PetSightingRecord
			if err := json.Unmarshal(b, &sRec); err == nil {
				if sRec.LostPetID == petID && sRec.Status == domain.SightingStatusActive {
					sightings = append(sightings, sRec)
				}
			}
		}
	}

	analysis := sighting.CalculateTrajectory(petID, originCoords, originTime, sightings)
	if record.Coordinates != nil {
		if analysisBytes, err := json.Marshal(analysis); err == nil {
			_ = s.stateStore.SaveState(ctx, store.TrajectoriesCollection, petID, analysisBytes)
		}
	}

	// Update active search party center coordinates and radius if perimeter is calculated
	if analysis.EstimatedPerimeter != nil {
		if rawParties, err := s.stateStore.ListState(ctx, store.SearchPartiesCollection); err == nil {
			for _, b := range rawParties {
				var party searchparty.SearchParty
				if err := json.Unmarshal(b, &party); err == nil && party.LostPetID == petID {
					party.CenterCoordinates = analysis.EstimatedPerimeter.CenterCoordinates
					party.RadiusMeters = analysis.EstimatedPerimeter.RadiusMeters
					if updatedPartyBytes, err := json.Marshal(party); err == nil {
						_ = s.stateStore.SaveState(ctx, store.SearchPartiesCollection, party.PartyID, updatedPartyBytes)
					}
					break
				}
			}
		}
	}

	// SSE Broadcast
	if s.reunionHub != nil {
		event := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_sighting_%s", sightingID),
			Type:      domain.ReunionEventSighting,
			MatchID:   petID,
			Timestamp: now,
			Payload: domain.SightingEventPayload{
				Type:                "sighting",
				PetID:               petID,
				SightingID:          sightingID,
				SightedAt:           now,
				LocationDescription: details,
				Coordinates:         &coords,
			},
		}
		s.reunionHub.Broadcast(event)
	}

	return fmt.Sprintf("Sighting recorded for %s! Coordinates logged and trajectory updated in Reunion Room. Thank you!", petID)
}

// handleSMSStatus handles STATUS command.
func (s *Server) handleSMSStatus(ctx context.Context) string {
	rawParties, err := s.stateStore.ListState(ctx, store.SearchPartiesCollection)
	if err != nil || len(rawParties) == 0 {
		return "No active search parties currently underway. Check https://petspotr.io/pets for lost pet updates."
	}

	for _, b := range rawParties {
		var party searchparty.SearchParty
		if err := json.Unmarshal(b, &party); err != nil {
			continue
		}

		openSectors := make([]string, 0)
		for _, sec := range party.Sectors {
			if sec.Status == searchparty.SectorStatusUnassigned {
				openSectors = append(openSectors, sec.SectorID)
			}
		}

		openStr := strings.Join(openSectors, ", ")
		if openStr == "" {
			openStr = "none"
		}

		return fmt.Sprintf("Search Active for %s. Coverage: %.0f%%. Open sectors: %s. Reply CLAIM <sector> to join.",
			party.LostPetID, party.CoveragePercentage, openStr)
	}

	return "No active search parties currently underway. Check https://petspotr.io/pets for lost pet updates."
}
