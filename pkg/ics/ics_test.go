package ics

import (
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestSignAndVerifyRoleCredential(t *testing.T) {
	secret := []byte("super-secret-ics-signing-key-2026")
	issuedAt := time.Date(2026, 10, 17, 10, 0, 0, 0, time.UTC)
	expiresAt := issuedAt.Add(365 * 24 * time.Hour)

	baseCred := domain.SignedCredential{
		CredentialID:  "cred-uav-101",
		Role:          domain.RoleUAVPart107Pilot,
		IssuedBy:      "FAA / NASAR Command",
		LicenseNumber: "FAA-107-998877",
		IssuedAt:      issuedAt,
		ExpiresAt:     expiresAt,
	}

	t.Run("guard empty volunteerId", func(t *testing.T) {
		_, err := SignRoleCredential("   ", baseCred, secret)
		if err == nil || !strings.Contains(err.Error(), "volunteerId is required") {
			t.Fatalf("expected volunteerId required error, got %v", err)
		}
	})

	t.Run("guard invalid role", func(t *testing.T) {
		bad := baseCred
		bad.Role = domain.SARRoleCredential("INVALID_ROLE")
		_, err := SignRoleCredential("vol-1", bad, secret)
		if err == nil || !strings.Contains(err.Error(), "invalid SAR role credential") {
			t.Fatalf("expected invalid role error, got %v", err)
		}
	})

	t.Run("guard empty secret key", func(t *testing.T) {
		_, err := SignRoleCredential("vol-1", baseCred, nil)
		if err == nil || !strings.Contains(err.Error(), "signing secret key is required") {
			t.Fatalf("expected secret key required error, got %v", err)
		}
	})

	signed, err := SignRoleCredential("vol-1", baseCred, secret)
	if err != nil {
		t.Fatalf("SignRoleCredential unexpected error: %v", err)
	}
	if signed.SignatureHex == "" {
		t.Fatalf("expected non-empty SignatureHex")
	}

	t.Run("verify valid credential", func(t *testing.T) {
		ok, err := VerifyRoleCredential("vol-1", signed, secret, issuedAt.Add(24*time.Hour))
		if err != nil || !ok {
			t.Fatalf("expected valid credential, got ok=%v err=%v", ok, err)
		}
	})

	t.Run("verify tampered signature", func(t *testing.T) {
		tampered := signed
		tampered.SignatureHex = "deadbeef" + signed.SignatureHex[8:]
		ok, err := VerifyRoleCredential("vol-1", tampered, secret, issuedAt.Add(24*time.Hour))
		if ok || err == nil || !strings.Contains(err.Error(), "invalid credential signature") {
			t.Fatalf("expected invalid credential signature error, got ok=%v err=%v", ok, err)
		}
	})

	t.Run("verify expired credential", func(t *testing.T) {
		ok, err := VerifyRoleCredential("vol-1", signed, secret, expiresAt.Add(time.Second))
		if ok || err == nil || !strings.Contains(err.Error(), "credential has expired") {
			t.Fatalf("expected credential has expired error, got ok=%v err=%v", ok, err)
		}
	})
}

func TestHasVerifiedRole(t *testing.T) {
	secret := []byte("super-secret-ics-signing-key-2026")
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

	uavCred, err := SignRoleCredential("vol-uav", domain.SignedCredential{
		CredentialID:  "cred-uav-1",
		Role:          domain.RoleUAVPart107Pilot,
		IssuedBy:      "FAA",
		LicenseNumber: "107-ABC",
		IssuedAt:      now.Add(-24 * time.Hour),
		ExpiresAt:     now.Add(24 * time.Hour),
	}, secret)
	if err != nil {
		t.Fatalf("SignRoleCredential failed: %v", err)
	}

	profile := domain.VolunteerProfile{
		VolunteerID: "vol-uav",
		Name:        "Alex Mercer",
		Callsign:    "EAGLE-1",
		PrimaryRole: domain.RoleUAVPart107Pilot,
		Credentials: []domain.SignedCredential{uavCred},
	}

	if !HasVerifiedRole(profile, "", secret, now) {
		t.Fatalf("expected empty requiredRole to return true")
	}
	if !HasVerifiedRole(profile, domain.RoleFieldSearcher, secret, now) {
		t.Fatalf("expected FIELD_SEARCHER baseline role to return true without special credential")
	}
	if !HasVerifiedRole(profile, domain.RoleUAVPart107Pilot, secret, now) {
		t.Fatalf("expected verified UAV_PART107_PILOT to return true")
	}
	if HasVerifiedRole(profile, domain.RoleVetMedic, secret, now) {
		t.Fatalf("expected uncredentialed VET_MEDIC gate to return false")
	}
	if HasVerifiedRole(profile, domain.RoleUAVPart107Pilot, secret, now.Add(48*time.Hour)) {
		t.Fatalf("expected expired UAV_PART107_PILOT credential to return false")
	}
}

func TestEvaluateShiftFatigue(t *testing.T) {
	tests := []struct {
		name           string
		activeHours    float64
		distanceKm     float64
		expectedLevel  domain.FatigueAlertLevel
		expectedStatus domain.ICSShiftStatus
		expectedSubstr string
	}{
		{
			name:           "normal patrol",
			activeHours:    2.5,
			distanceKm:     5.0,
			expectedLevel:  domain.FatigueLevelNormal,
			expectedStatus: domain.ShiftStatusOnSectorPatrol,
			expectedSubstr: "Fit for active sector patrol",
		},
		{
			name:           "warning by hours",
			activeHours:    6.5,
			distanceKm:     12.0,
			expectedLevel:  domain.FatigueLevelWarning,
			expectedStatus: domain.ShiftStatusOnSectorPatrol,
			expectedSubstr: "Approaching 6h/18km fatigue threshold",
		},
		{
			name:           "warning by distance",
			activeHours:    4.0,
			distanceKm:     19.0,
			expectedLevel:  domain.FatigueLevelWarning,
			expectedStatus: domain.ShiftStatusOnSectorPatrol,
			expectedSubstr: "Approaching 6h/18km fatigue threshold",
		},
		{
			name:           "mandatory rest by hours",
			activeHours:    8.5,
			distanceKm:     15.0,
			expectedLevel:  domain.FatigueLevelMandatoryRest,
			expectedStatus: domain.ShiftStatusMandatoryRest,
			expectedSubstr: "Mandatory 8h/25km SAR safety ceiling reached",
		},
		{
			name:           "mandatory rest by distance",
			activeHours:    5.0,
			distanceKm:     26.0,
			expectedLevel:  domain.FatigueLevelMandatoryRest,
			expectedStatus: domain.ShiftStatusMandatoryRest,
			expectedSubstr: "Mandatory 8h/25km SAR safety ceiling reached",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			level, status, note := EvaluateShiftFatigue(tc.activeHours, tc.distanceKm)
			if level != tc.expectedLevel {
				t.Fatalf("expected level %s, got %s", tc.expectedLevel, level)
			}
			if status != tc.expectedStatus {
				t.Fatalf("expected status %s, got %s", tc.expectedStatus, status)
			}
			if !strings.Contains(note, tc.expectedSubstr) {
				t.Fatalf("expected note containing %q, got %q", tc.expectedSubstr, note)
			}
		})
	}
}

func TestExportICS204RosterCSV(t *testing.T) {
	checkedIn := time.Date(2026, 10, 17, 14, 30, 0, 0, time.UTC)
	shifts := []domain.ICSShiftEntry{
		{
			ShiftID:           "shift-1",
			IncidentID:        "inc-101",
			VolunteerID:       "vol-1",
			VolunteerName:     "Dana Scully",
			Callsign:          "MEDIC-1",
			AssignedRole:      domain.RoleVetMedic,
			AssignedSectorID:  "SECTOR-A1",
			Status:            domain.ShiftStatusOnSectorPatrol,
			ActiveHours:       6.25,
			DistanceCoveredKm: 14.5,
			FatigueLevel:      domain.FatigueLevelWarning,
			CheckedInAt:       checkedIn,
		},
	}

	csvOut, err := ExportICS204RosterCSV(shifts)
	if err != nil {
		t.Fatalf("ExportICS204RosterCSV unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(csvOut), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 CSV lines, got %d: %q", len(lines), csvOut)
	}
	expectedHeader := "shift_id,incident_id,volunteer_id,volunteer_name,callsign,assigned_role,assigned_sector_id,status,active_hours,distance_covered_km,fatigue_level,checked_in_at"
	if strings.TrimSpace(lines[0]) != expectedHeader {
		t.Fatalf("expected header %q, got %q", expectedHeader, lines[0])
	}
	expectedRow := "shift-1,inc-101,vol-1,Dana Scully,MEDIC-1,VET_MEDIC,SECTOR-A1,ON_SECTOR_PATROL,6.25,14.50,FATIGUE_WARNING,2026-10-17T14:30:00Z"
	if strings.TrimSpace(lines[1]) != expectedRow {
		t.Fatalf("expected row %q, got %q", expectedRow, lines[1])
	}
}

func TestEvaluateVolunteerProgression(t *testing.T) {
	t.Run("Tier 1 Rookie Spotter", func(t *testing.T) {
		p := EvaluateVolunteerProgression(domain.VolunteerProfile{
			VolunteerID:     "vol-t1",
			PrimaryRole:     domain.RoleFieldSearcher,
			CompletedShifts: 1,
			TotalDistanceKm: 2.0,
		}, false)
		if p.RankTitle != "Rookie Spotter (Tier 1)" {
			t.Fatalf("expected Rookie Spotter (Tier 1), got %q (xp=%d)", p.RankTitle, p.TotalXP)
		}
		if len(p.EarnedBadges) != 1 || p.EarnedBadges[0] != domain.BadgeFirstResponder {
			t.Fatalf("expected [FIRST_RESPONDER], got %v", p.EarnedBadges)
		}
	})

	t.Run("Tier 2 Field Tracker with Night Owl and Canine", func(t *testing.T) {
		p := EvaluateVolunteerProgression(domain.VolunteerProfile{
			VolunteerID:      "vol-t2",
			PrimaryRole:      domain.RoleK9Handler,
			CompletedShifts:  3,
			TotalDistanceKm:  10.0,
			TotalHoursLogged: 4.0,
		}, true)
		// xp = 300 + 200 + 100 = 600
		if p.TotalXP != 600 || p.RankTitle != "Field Tracker (Tier 2)" {
			t.Fatalf("expected 600 XP Field Tracker (Tier 2), got xp=%d rank=%q", p.TotalXP, p.RankTitle)
		}
		badgeSet := map[domain.SARMeritBadge]bool{}
		for _, b := range p.EarnedBadges {
			badgeSet[b] = true
		}
		if !badgeSet[domain.BadgeFirstResponder] || !badgeSet[domain.BadgeNightOwlIR] || !badgeSet[domain.BadgeCanineTracker] {
			t.Fatalf("missing expected badges in %v", p.EarnedBadges)
		}
	})

	t.Run("Tier 3 Senior SAR Specialist", func(t *testing.T) {
		p := EvaluateVolunteerProgression(domain.VolunteerProfile{
			VolunteerID:       "vol-t3",
			PrimaryRole:       domain.RoleUAVPart107Pilot,
			CompletedShifts:   5,
			TotalDistanceKm:   25.0,
			TotalHoursLogged:  10.0,
			VerifiedSightings: 2,
		}, false)
		// xp = 500 + 500 + 250 + 300 = 1550
		if p.TotalXP != 1550 || p.RankTitle != "Senior SAR Specialist (Tier 3)" {
			t.Fatalf("expected 1550 XP Senior SAR Specialist (Tier 3), got xp=%d rank=%q", p.TotalXP, p.RankTitle)
		}
	})

	t.Run("Tier 4 Master Incident Vanguard and all 7 badges without duplicates", func(t *testing.T) {
		p := EvaluateVolunteerProgression(domain.VolunteerProfile{
			VolunteerID:  "vol-t4",
			PrimaryRole:  domain.RoleVetMedic,
			EarnedBadges: []domain.SARMeritBadge{domain.BadgeFirstResponder, domain.BadgeNightOwlIR},
			Credentials: []domain.SignedCredential{
				{Role: domain.RoleK9Handler},
				{Role: domain.RoleUAVPart107Pilot},
				{Role: domain.RoleVetMedic},
			},
			CompletedShifts:   6,
			TotalDistanceKm:   55.0,
			TotalHoursLogged:  20.0,
			VerifiedSightings: 3,
			ReunionsAssisted:  2,
		}, true)
		// xp = 6*100(600) + 55*20(1100) + 20*25(500) + 3*150(450) + 2*500(1000) + 3*200(600) = 4250
		if p.TotalXP != 4250 || p.RankTitle != "Master Incident Vanguard (Tier 4)" {
			t.Fatalf("expected 4250 XP Master Incident Vanguard (Tier 4), got xp=%d rank=%q", p.TotalXP, p.RankTitle)
		}
		if len(p.EarnedBadges) != 7 {
			t.Fatalf("expected all 7 unique badges, got %d: %v", len(p.EarnedBadges), p.EarnedBadges)
		}
		seen := map[domain.SARMeritBadge]bool{}
		for _, b := range p.EarnedBadges {
			if seen[b] {
				t.Fatalf("duplicate badge found: %s", b)
			}
			seen[b] = true
		}
	})
}
