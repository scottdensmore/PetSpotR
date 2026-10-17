package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ics"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var (
	lastICSNano      atomic.Int64
	volunteerSeedMu  sync.Mutex
	defaultICSSecret = []byte("petspotr-ics-sar-credential-hmac-key-2026")
)

func nextICSNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastICSNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastICSNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

func getICSCredentialSecretKey() []byte {
	if env := strings.TrimSpace(os.Getenv("PETSPOTR_ICS_CREDENTIAL_SECRET")); env != "" {
		return []byte(env)
	}
	return defaultICSSecret
}

// ICSCommandPageData represents template data passed to templates/ics_command.html.
type ICSCommandPageData struct {
	Volunteers []domain.VolunteerProfile
	Shifts     []domain.ICSShiftEntry
	Locale     string
}

type createICSVolunteerRequest struct {
	Name            string                   `json:"name"`
	Callsign        string                   `json:"callsign"`
	PrimaryRole     domain.SARRoleCredential `json:"primaryRole"`
	Role            domain.SARRoleCredential `json:"role"`
	LicenseNumber   string                   `json:"licenseNumber"`
	License         string                   `json:"license"`
	IssueCredential bool                     `json:"issueCredential"`
}

type issueCredentialRequest struct {
	Role          domain.SARRoleCredential `json:"role"`
	IssuedBy      string                   `json:"issuedBy"`
	LicenseNumber string                   `json:"licenseNumber"`
}

type createICSShiftRequest struct {
	IncidentID        string                   `json:"incidentId"`
	VolunteerID       string                   `json:"volunteerId"`
	AssignedRole      domain.SARRoleCredential `json:"assignedRole"`
	AssignedSectorID  string                   `json:"assignedSectorId"`
	ActiveHours       float64                  `json:"activeHours"`
	DistanceCoveredKm float64                  `json:"distanceCoveredKm"`
}

type updateICSShiftProgressRequest struct {
	AddHours               float64  `json:"addHours"`
	AddDistanceKm          float64  `json:"addDistanceKm"`
	ActiveHours            *float64 `json:"activeHours"`
	DistanceCoveredKm      *float64 `json:"distanceCoveredKm"`
	VerifiedSightings      int      `json:"verifiedSightings"`
	VerifiedSightingsDelta int      `json:"verifiedSightingsDelta"`
	ReunionsAssisted       int      `json:"reunionsAssisted"`
	ReunionsAssistedDelta  int      `json:"reunionsAssistedDelta"`
	NightShift             bool     `json:"nightShift"`
	Checkout               bool     `json:"checkout"`
}

type volunteerProfileEnvelope struct {
	domain.VolunteerProfile
	Volunteer domain.VolunteerProfile `json:"volunteer"`
}

type issueCredentialEnvelope struct {
	domain.VolunteerProfile
	Volunteer  domain.VolunteerProfile `json:"volunteer"`
	Credential domain.SignedCredential `json:"credential"`
}

type icsShiftEnvelope struct {
	domain.ICSShiftEntry
	Shift domain.ICSShiftEntry `json:"shift"`
}

type updateShiftProgressEnvelope struct {
	domain.ICSShiftEntry
	Shift     domain.ICSShiftEntry    `json:"shift"`
	Volunteer domain.VolunteerProfile `json:"volunteer"`
}

func defaultICSVolunteers(now time.Time, secretKey []byte) []domain.VolunteerProfile {
	expires := now.Add(365 * 24 * time.Hour)

	signCred := func(volID, credID string, role domain.SARRoleCredential, issuedBy, license string) domain.SignedCredential {
		c, _ := ics.SignRoleCredential(volID, domain.SignedCredential{
			CredentialID:  credID,
			Role:          role,
			IssuedBy:      issuedBy,
			LicenseNumber: license,
			IssuedAt:      now,
			ExpiresAt:     expires,
		}, secretKey)
		return c
	}

	v1 := domain.VolunteerProfile{
		VolunteerID:       "vol-sar-101",
		Name:              "Capt. Elena Rostova",
		Callsign:          "SIERRA-1",
		PrimaryRole:       domain.RoleICSCommander,
		Credentials:       []domain.SignedCredential{signCred("vol-sar-101", "cred-101-cmd", domain.RoleICSCommander, "FEMA / NIMS Command", "FEMA-ICS-300-8812"), signCred("vol-sar-101", "cred-101-k9", domain.RoleK9Handler, "NASAR K9 Command", "NASAR-K9-I-4402")},
		EarnedBadges:      []domain.SARMeritBadge{},
		CompletedShifts:   12,
		TotalDistanceKm:   64.5,
		TotalHoursLogged:  48.0,
		VerifiedSightings: 4,
		ReunionsAssisted:  2,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	v1 = ics.EvaluateVolunteerProgression(v1, false)

	v2 := domain.VolunteerProfile{
		VolunteerID:       "vol-sar-102",
		Name:              "Marcus Vance",
		Callsign:          "FALCON-7",
		PrimaryRole:       domain.RoleUAVPart107Pilot,
		Credentials:       []domain.SignedCredential{signCred("vol-sar-102", "cred-102-uav", domain.RoleUAVPart107Pilot, "FAA / NASAR Command", "FAA-107-449821")},
		EarnedBadges:      []domain.SARMeritBadge{},
		CompletedShifts:   6,
		TotalDistanceKm:   28.0,
		TotalHoursLogged:  22.0,
		VerifiedSightings: 3,
		ReunionsAssisted:  1,
		CreatedAt:         now.Add(time.Millisecond),
		UpdatedAt:         now.Add(time.Millisecond),
	}
	v2 = ics.EvaluateVolunteerProgression(v2, false)

	v3 := domain.VolunteerProfile{
		VolunteerID:       "vol-sar-103",
		Name:              "Dr. Maya Lin",
		Callsign:          "MEDIC-3",
		PrimaryRole:       domain.RoleVetMedic,
		Credentials:       []domain.SignedCredential{signCred("vol-sar-103", "cred-103-vet", domain.RoleVetMedic, "WA State Veterinary Medical Board", "WA-DVM-88310")},
		EarnedBadges:      []domain.SARMeritBadge{},
		CompletedShifts:   4,
		TotalDistanceKm:   15.0,
		TotalHoursLogged:  18.0,
		VerifiedSightings: 1,
		ReunionsAssisted:  1,
		CreatedAt:         now.Add(2 * time.Millisecond),
		UpdatedAt:         now.Add(2 * time.Millisecond),
	}
	v3 = ics.EvaluateVolunteerProgression(v3, false)

	return []domain.VolunteerProfile{v1, v2, v3}
}

func (s *Server) ensureDefaultICSVolunteers(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	volunteerSeedMu.Lock()
	defer volunteerSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionVolunteerProfiles)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC()
	secretKey := getICSCredentialSecretKey()
	for _, v := range defaultICSVolunteers(now, secretKey) {
		if data, err := json.Marshal(v); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionVolunteerProfiles, v.VolunteerID, data)
		}
	}
}

func (s *Server) loadICSVolunteers(ctx context.Context) []domain.VolunteerProfile {
	volunteers := make([]domain.VolunteerProfile, 0)
	if s.stateStore == nil {
		return volunteers
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionVolunteerProfiles)
	if err != nil {
		return volunteers
	}
	for _, b := range raw {
		var v domain.VolunteerProfile
		if err := json.Unmarshal(b, &v); err == nil {
			if v.Credentials == nil {
				v.Credentials = []domain.SignedCredential{}
			}
			if v.EarnedBadges == nil {
				v.EarnedBadges = []domain.SARMeritBadge{}
			}
			volunteers = append(volunteers, v)
		}
	}
	sort.Slice(volunteers, func(i, j int) bool {
		if volunteers[i].TotalXP != volunteers[j].TotalXP {
			return volunteers[i].TotalXP > volunteers[j].TotalXP
		}
		return volunteers[i].VolunteerID < volunteers[j].VolunteerID
	})
	return volunteers
}

func (s *Server) loadICSShifts(ctx context.Context, incidentIDFilter, statusFilter string) []domain.ICSShiftEntry {
	shifts := make([]domain.ICSShiftEntry, 0)
	if s.stateStore == nil {
		return shifts
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionICSShiftEntries)
	if err != nil {
		return shifts
	}
	for _, b := range raw {
		var sh domain.ICSShiftEntry
		if err := json.Unmarshal(b, &sh); err == nil {
			if incidentIDFilter != "" && !strings.EqualFold(sh.IncidentID, incidentIDFilter) {
				continue
			}
			if statusFilter != "" && !strings.EqualFold(string(sh.Status), statusFilter) {
				continue
			}
			shifts = append(shifts, sh)
		}
	}
	sort.Slice(shifts, func(i, j int) bool {
		if !shifts[i].CheckedInAt.Equal(shifts[j].CheckedInAt) {
			return shifts[i].CheckedInAt.After(shifts[j].CheckedInAt)
		}
		return shifts[i].ShiftID > shifts[j].ShiftID
	})
	return shifts
}

func writeICSJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleICSCommandPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultICSVolunteers(r.Context())
	volunteers := s.loadICSVolunteers(r.Context())
	shifts := s.loadICSShifts(r.Context(), "", "")

	tmpl, err := template.New("ics_command.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/ics_command.html")
	if err != nil {
		http.Error(w, "Failed to load ICS Command template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := ICSCommandPageData{
		Volunteers: volunteers,
		Shifts:     shifts,
		Locale:     LocaleFromContext(r.Context()),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListICSVolunteers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeICSJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultICSVolunteers(r.Context())
	volunteers := s.loadICSVolunteers(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"volunteers": volunteers,
	})
}

func (s *Server) handleCreateICSVolunteer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeICSJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createICSVolunteerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeICSJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultICSVolunteers(r.Context())

	primaryRole := domain.SARRoleCredential(strings.ToUpper(strings.TrimSpace(string(req.PrimaryRole))))
	if primaryRole == "" {
		primaryRole = domain.SARRoleCredential(strings.ToUpper(strings.TrimSpace(string(req.Role))))
	}
	if primaryRole == "" {
		primaryRole = domain.RoleFieldSearcher
	}

	licenseNumber := strings.TrimSpace(req.LicenseNumber)
	if licenseNumber == "" {
		licenseNumber = strings.TrimSpace(req.License)
	}

	seqNano := nextICSNano()
	now := time.Unix(0, seqNano).UTC()
	volunteerID := fmt.Sprintf("vol-%d", seqNano)

	profile := domain.VolunteerProfile{
		VolunteerID:  volunteerID,
		Name:         strings.TrimSpace(req.Name),
		Callsign:     strings.ToUpper(strings.TrimSpace(req.Callsign)),
		PrimaryRole:  primaryRole,
		Credentials:  []domain.SignedCredential{},
		EarnedBadges: []domain.SARMeritBadge{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := profile.Validate(); err != nil {
		writeICSJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.IssueCredential || profile.PrimaryRole == domain.RoleFieldSearcher {
		if licenseNumber == "" {
			licenseNumber = fmt.Sprintf("SAR-%s-%d", profile.Callsign, seqNano%10000)
		}
		signedCred, err := ics.SignRoleCredential(profile.VolunteerID, domain.SignedCredential{
			CredentialID:  fmt.Sprintf("cred-%d", nextICSNano()),
			Role:          profile.PrimaryRole,
			IssuedBy:      "ICS Command Authority",
			LicenseNumber: licenseNumber,
			IssuedAt:      now,
			ExpiresAt:     now.Add(365 * 24 * time.Hour),
		}, getICSCredentialSecretKey())
		if err == nil {
			profile.Credentials = append(profile.Credentials, signedCred)
		}
	}

	profile = ics.EvaluateVolunteerProgression(profile, false)
	if profile.EarnedBadges == nil {
		profile.EarnedBadges = []domain.SARMeritBadge{}
	}

	profileBytes, err := json.Marshal(profile)
	if err != nil {
		writeICSJSONError(w, http.StatusInternalServerError, "Failed to encode volunteer profile")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionVolunteerProfiles, profile.VolunteerID, profileBytes); err != nil {
		writeICSJSONError(w, http.StatusInternalServerError, "Failed to save volunteer profile")
		return
	}

	s.broadcastICSEvent("ics_volunteer_updated", map[string]any{
		"type":      "ics_volunteer_updated",
		"volunteer": profile,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(volunteerProfileEnvelope{
		VolunteerProfile: profile,
		Volunteer:        profile,
	})
}

func (s *Server) handleIssueVolunteerCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeICSJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	volunteerID := strings.TrimSpace(r.PathValue("id"))
	if volunteerID == "" {
		writeICSJSONError(w, http.StatusBadRequest, "volunteer id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req issueCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeICSJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultICSVolunteers(r.Context())

	req.Role = domain.SARRoleCredential(strings.ToUpper(strings.TrimSpace(string(req.Role))))
	if !req.Role.IsValid() {
		writeICSJSONError(w, http.StatusBadRequest, "Invalid SAR role credential")
		return
	}

	issuedBy := strings.TrimSpace(req.IssuedBy)
	if issuedBy == "" {
		issuedBy = "ICS Command Authority"
	}
	licenseNumber := strings.TrimSpace(req.LicenseNumber)

	seqNano := nextICSNano()
	now := time.Now().UTC()
	signedCred, err := ics.SignRoleCredential(volunteerID, domain.SignedCredential{
		CredentialID:  fmt.Sprintf("cred-%d", seqNano),
		Role:          req.Role,
		IssuedBy:      issuedBy,
		LicenseNumber: licenseNumber,
		IssuedAt:      now,
		ExpiresAt:     now.Add(365 * 24 * time.Hour),
	}, getICSCredentialSecretKey())
	if err != nil {
		writeICSJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var updatedProfile domain.VolunteerProfile
	err = s.stateStore.UpdateState(r.Context(), store.CollectionVolunteerProfiles, volunteerID, func(current []byte) ([]byte, error) {
		var p domain.VolunteerProfile
		if err := json.Unmarshal(current, &p); err != nil {
			return nil, fmt.Errorf("corrupt volunteer profile: %w", err)
		}
		p.Credentials = append(p.Credentials, signedCred)
		p.UpdatedAt = now
		p = ics.EvaluateVolunteerProgression(p, false)
		if p.EarnedBadges == nil {
			p.EarnedBadges = []domain.SARMeritBadge{}
		}
		updatedProfile = p
		return json.Marshal(p)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeICSJSONError(w, http.StatusNotFound, "Volunteer not found")
			return
		}
		writeICSJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update volunteer credentials: %v", err))
		return
	}

	s.broadcastICSEvent("ics_volunteer_updated", map[string]any{
		"type":       "ics_volunteer_updated",
		"volunteer":  updatedProfile,
		"credential": signedCred,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(issueCredentialEnvelope{
		VolunteerProfile: updatedProfile,
		Volunteer:        updatedProfile,
		Credential:       signedCred,
	})
}

func (s *Server) handleCreateICSShift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeICSJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createICSShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeICSJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultICSVolunteers(r.Context())

	volunteerID := strings.TrimSpace(req.VolunteerID)
	if volunteerID == "" {
		writeICSJSONError(w, http.StatusBadRequest, "volunteerId is required")
		return
	}

	volBytes, err := s.stateStore.GetState(r.Context(), store.CollectionVolunteerProfiles, volunteerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeICSJSONError(w, http.StatusNotFound, "Volunteer not found")
			return
		}
		writeICSJSONError(w, http.StatusInternalServerError, "Failed to load volunteer")
		return
	}

	var volunteer domain.VolunteerProfile
	if err := json.Unmarshal(volBytes, &volunteer); err != nil {
		writeICSJSONError(w, http.StatusInternalServerError, "Corrupt volunteer profile")
		return
	}

	assignedRole := domain.SARRoleCredential(strings.ToUpper(strings.TrimSpace(string(req.AssignedRole))))
	if assignedRole == "" {
		assignedRole = volunteer.PrimaryRole
	}
	if !assignedRole.IsValid() {
		writeICSJSONError(w, http.StatusBadRequest, "Invalid assigned SAR role")
		return
	}

	incidentID := strings.TrimSpace(req.IncidentID)
	if incidentID == "" {
		incidentID = "INC-2026-WILDERNESS"
	}
	sectorID := strings.TrimSpace(req.AssignedSectorID)
	if sectorID == "" {
		sectorID = "SECTOR-ALPHA-1"
	}

	now := time.Now().UTC()
	if !ics.HasVerifiedRole(volunteer, assignedRole, getICSCredentialSecretKey(), now) {
		writeICSJSONError(
			w,
			http.StatusForbidden,
			fmt.Sprintf("volunteer %s (%s) lacks a verified cryptographic credential for role %s", volunteer.Name, volunteer.Callsign, assignedRole),
		)
		return
	}

	fatigueLevel, status, notes := ics.EvaluateShiftFatigue(req.ActiveHours, req.DistanceCoveredKm)
	seqNano := nextICSNano()
	checkedInAt := time.Unix(0, seqNano).UTC()

	shift := domain.ICSShiftEntry{
		ShiftID:           fmt.Sprintf("shift-%d", seqNano),
		IncidentID:        incidentID,
		VolunteerID:       volunteer.VolunteerID,
		VolunteerName:     volunteer.Name,
		Callsign:          volunteer.Callsign,
		AssignedRole:      assignedRole,
		AssignedSectorID:  sectorID,
		Status:            status,
		ActiveHours:       req.ActiveHours,
		DistanceCoveredKm: req.DistanceCoveredKm,
		FatigueLevel:      fatigueLevel,
		FatigueNotes:      notes,
		CheckedInAt:       checkedInAt,
		LastHeartbeatAt:   checkedInAt,
	}

	shiftBytes, err := json.Marshal(shift)
	if err != nil {
		writeICSJSONError(w, http.StatusInternalServerError, "Failed to encode shift entry")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionICSShiftEntries, shift.ShiftID, shiftBytes); err != nil {
		writeICSJSONError(w, http.StatusInternalServerError, "Failed to save shift entry")
		return
	}

	s.broadcastICSEvent("ics_shift_updated", map[string]any{
		"type":  "ics_shift_updated",
		"shift": shift,
	}, checkedInAt)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(icsShiftEnvelope{
		ICSShiftEntry: shift,
		Shift:         shift,
	})
}

func (s *Server) handleListICSShifts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeICSJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	q := r.URL.Query()
	incidentIDFilter := strings.TrimSpace(q.Get("incidentId"))
	statusFilter := strings.TrimSpace(q.Get("status"))
	shifts := s.loadICSShifts(r.Context(), incidentIDFilter, statusFilter)

	if strings.EqualFold(strings.TrimSpace(q.Get("format")), "csv") {
		csvContent, err := ics.ExportICS204RosterCSV(shifts)
		if err != nil {
			writeICSJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to format ICS-204 CSV: %v", err))
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\"ics-204-shift-roster.csv\"")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(csvContent))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"shifts": shifts,
	})
}

func (s *Server) handleUpdateICSShiftProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeICSJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	shiftID := strings.TrimSpace(r.PathValue("id"))
	if shiftID == "" {
		writeICSJSONError(w, http.StatusBadRequest, "shift id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req updateICSShiftProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeICSJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	now := time.Now().UTC()
	var updatedShift domain.ICSShiftEntry
	var hoursDelta, distDelta float64

	err := s.stateStore.UpdateState(r.Context(), store.CollectionICSShiftEntries, shiftID, func(current []byte) ([]byte, error) {
		var sh domain.ICSShiftEntry
		if err := json.Unmarshal(current, &sh); err != nil {
			return nil, fmt.Errorf("corrupt shift entry: %w", err)
		}

		if req.AddHours > 0 {
			hoursDelta = req.AddHours
			sh.ActiveHours += req.AddHours
		} else if req.ActiveHours != nil {
			if *req.ActiveHours > sh.ActiveHours {
				hoursDelta = *req.ActiveHours - sh.ActiveHours
			}
			sh.ActiveHours = *req.ActiveHours
		}

		if req.AddDistanceKm > 0 {
			distDelta = req.AddDistanceKm
			sh.DistanceCoveredKm += req.AddDistanceKm
		} else if req.DistanceCoveredKm != nil {
			if *req.DistanceCoveredKm > sh.DistanceCoveredKm {
				distDelta = *req.DistanceCoveredKm - sh.DistanceCoveredKm
			}
			sh.DistanceCoveredKm = *req.DistanceCoveredKm
		}

		fatigueLevel, status, notes := ics.EvaluateShiftFatigue(sh.ActiveHours, sh.DistanceCoveredKm)
		sh.FatigueLevel = fatigueLevel
		sh.FatigueNotes = notes
		sh.LastHeartbeatAt = now

		if req.Checkout {
			sh.Status = domain.ShiftStatusCheckedOut
			checkoutTime := now
			sh.CheckedOutAt = &checkoutTime
		} else {
			sh.Status = status
		}

		updatedShift = sh
		return json.Marshal(sh)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeICSJSONError(w, http.StatusNotFound, "Shift not found")
			return
		}
		writeICSJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update shift: %v", err))
		return
	}

	sightingsDelta := req.VerifiedSightings + req.VerifiedSightingsDelta
	reunionsDelta := req.ReunionsAssisted + req.ReunionsAssistedDelta

	var updatedVolunteer domain.VolunteerProfile
	if updatedShift.VolunteerID != "" {
		_ = s.stateStore.UpdateState(r.Context(), store.CollectionVolunteerProfiles, updatedShift.VolunteerID, func(current []byte) ([]byte, error) {
			var p domain.VolunteerProfile
			if err := json.Unmarshal(current, &p); err != nil {
				return nil, fmt.Errorf("corrupt volunteer profile: %w", err)
			}

			p.TotalHoursLogged += hoursDelta
			p.TotalDistanceKm += distDelta
			p.VerifiedSightings += sightingsDelta
			p.ReunionsAssisted += reunionsDelta
			if req.Checkout {
				p.CompletedShifts++
			}
			p.UpdatedAt = now
			p = ics.EvaluateVolunteerProgression(p, req.NightShift)
			if p.EarnedBadges == nil {
				p.EarnedBadges = []domain.SARMeritBadge{}
			}
			updatedVolunteer = p
			return json.Marshal(p)
		})
	}

	s.broadcastICSEvent("ics_shift_updated", map[string]any{
		"type":      "ics_shift_updated",
		"shift":     updatedShift,
		"volunteer": updatedVolunteer,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updateShiftProgressEnvelope{
		ICSShiftEntry: updatedShift,
		Shift:         updatedShift,
		Volunteer:     updatedVolunteer,
	})
}

func (s *Server) handleICSStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	if s.reunionHub == nil {
		return
	}

	subCh, unsubscribe := s.reunionHub.Subscribe("ics")
	defer unsubscribe()

	pingInterval := s.reunionPingInterval
	if pingInterval <= 0 {
		pingInterval = defaultReunionPingInterval
	}
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-subCh:
			if !ok {
				return
			}
			if err := formatSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) broadcastICSEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextICSNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "ics",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "ics",
			SenderNodeID:  "ics-command-authority",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
