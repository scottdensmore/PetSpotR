package domain

import (
	"errors"
	"strings"
	"time"
)

type SARRoleCredential string

const (
	RoleFieldSearcher   SARRoleCredential = "FIELD_SEARCHER"
	RoleK9Handler       SARRoleCredential = "K9_HANDLER"
	RoleUAVPart107Pilot SARRoleCredential = "UAV_PART107_PILOT"
	RoleVetMedic        SARRoleCredential = "VET_MEDIC"
	RoleICSCommander    SARRoleCredential = "ICS_COMMANDER"
)

func (r SARRoleCredential) IsValid() bool {
	switch r {
	case RoleFieldSearcher, RoleK9Handler, RoleUAVPart107Pilot, RoleVetMedic, RoleICSCommander:
		return true
	default:
		return false
	}
}

type SARMeritBadge string

const (
	BadgeFirstResponder  SARMeritBadge = "FIRST_RESPONDER"
	BadgeNightOwlIR      SARMeritBadge = "NIGHT_OWL_IR"
	BadgeGridMaster50Km  SARMeritBadge = "GRID_MASTER_50KM"
	BadgeCanineTracker   SARMeritBadge = "CANINE_TRACKER"
	BadgeAerialSpotter   SARMeritBadge = "AERIAL_SPOTTER"
	BadgeTriageLifesaver SARMeritBadge = "TRIAGE_LIFESAVER"
	BadgeReunionHero     SARMeritBadge = "REUNION_HERO"
)

type ICSShiftStatus string

const (
	ShiftStatusCheckedIn      ICSShiftStatus = "CHECKED_IN"
	ShiftStatusOnSectorPatrol ICSShiftStatus = "ON_SECTOR_PATROL"
	ShiftStatusMandatoryRest  ICSShiftStatus = "MANDATORY_REST"
	ShiftStatusCheckedOut     ICSShiftStatus = "CHECKED_OUT"
)

func (s ICSShiftStatus) IsValid() bool {
	switch s {
	case ShiftStatusCheckedIn, ShiftStatusOnSectorPatrol, ShiftStatusMandatoryRest, ShiftStatusCheckedOut:
		return true
	default:
		return false
	}
}

type FatigueAlertLevel string

const (
	FatigueLevelNormal        FatigueAlertLevel = "NORMAL_FIT"
	FatigueLevelWarning       FatigueAlertLevel = "FATIGUE_WARNING"
	FatigueLevelMandatoryRest FatigueAlertLevel = "MANDATORY_REST"
)

type SignedCredential struct {
	CredentialID  string            `json:"credentialId"`
	Role          SARRoleCredential `json:"role"`
	IssuedBy      string            `json:"issuedBy"`
	LicenseNumber string            `json:"licenseNumber,omitempty"`
	IssuedAt      time.Time         `json:"issuedAt"`
	ExpiresAt     time.Time         `json:"expiresAt"`
	SignatureHex  string            `json:"signatureHex"`
}

type VolunteerProfile struct {
	VolunteerID       string             `json:"volunteerId"`
	Name              string             `json:"name"`
	Callsign          string             `json:"callsign"`
	PrimaryRole       SARRoleCredential  `json:"primaryRole"`
	Credentials       []SignedCredential `json:"credentials"`
	EarnedBadges      []SARMeritBadge    `json:"earnedBadges"`
	TotalXP           int                `json:"totalXp"`
	RankTitle         string             `json:"rankTitle"`
	CompletedShifts   int                `json:"completedShifts"`
	TotalDistanceKm   float64            `json:"totalDistanceKm"`
	TotalHoursLogged  float64            `json:"totalHoursLogged"`
	VerifiedSightings int                `json:"verifiedSightings"`
	ReunionsAssisted  int                `json:"reunionsAssisted"`
	CreatedAt         time.Time          `json:"createdAt"`
	UpdatedAt         time.Time          `json:"updatedAt"`
}

func (v *VolunteerProfile) Validate() error {
	if v == nil {
		return errors.New("volunteer profile is nil")
	}
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("volunteer name is required")
	}
	if strings.TrimSpace(v.Callsign) == "" {
		return errors.New("volunteer callsign is required")
	}
	if !v.PrimaryRole.IsValid() {
		return errors.New("invalid primary SAR role")
	}
	return nil
}

type ICSShiftEntry struct {
	ShiftID           string            `json:"shiftId"`
	IncidentID        string            `json:"incidentId"`
	VolunteerID       string            `json:"volunteerId"`
	VolunteerName     string            `json:"volunteerName"`
	Callsign          string            `json:"callsign"`
	AssignedRole      SARRoleCredential `json:"assignedRole"`
	AssignedSectorID  string            `json:"assignedSectorId"`
	Status            ICSShiftStatus    `json:"status"`
	ActiveHours       float64           `json:"activeHours"`
	DistanceCoveredKm float64           `json:"distanceCoveredKm"`
	FatigueLevel      FatigueAlertLevel `json:"fatigueLevel"`
	FatigueNotes      string            `json:"fatigueNotes,omitempty"`
	CheckedInAt       time.Time         `json:"checkedInAt"`
	LastHeartbeatAt   time.Time         `json:"lastHeartbeatAt"`
	CheckedOutAt      *time.Time        `json:"checkedOutAt,omitempty"`
}
