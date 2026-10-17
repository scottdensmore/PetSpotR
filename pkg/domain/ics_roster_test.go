package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestICSRosterRoleCredentialIsValid(t *testing.T) {
	validRoles := []SARRoleCredential{
		RoleFieldSearcher,
		RoleK9Handler,
		RoleUAVPart107Pilot,
		RoleVetMedic,
		RoleICSCommander,
	}
	for _, r := range validRoles {
		if !r.IsValid() {
			t.Errorf("expected role %q to be valid", r)
		}
	}

	invalidRoles := []SARRoleCredential{
		"",
		"INVALID_ROLE",
		"field_searcher",
	}
	for _, r := range invalidRoles {
		if r.IsValid() {
			t.Errorf("expected role %q to be invalid", r)
		}
	}
}

func TestICSRosterShiftStatusIsValid(t *testing.T) {
	validStatuses := []ICSShiftStatus{
		ShiftStatusCheckedIn,
		ShiftStatusOnSectorPatrol,
		ShiftStatusMandatoryRest,
		ShiftStatusCheckedOut,
	}
	for _, s := range validStatuses {
		if !s.IsValid() {
			t.Errorf("expected shift status %q to be valid", s)
		}
	}

	invalidStatuses := []ICSShiftStatus{
		"",
		"UNKNOWN_STATUS",
		"checked_in",
	}
	for _, s := range invalidStatuses {
		if s.IsValid() {
			t.Errorf("expected shift status %q to be invalid", s)
		}
	}
}

func TestICSRosterVolunteerProfileValidate(t *testing.T) {
	var nilProfile *VolunteerProfile
	if err := nilProfile.Validate(); err == nil {
		t.Fatal("expected error for nil profile, got nil")
	}

	tests := []struct {
		name    string
		profile VolunteerProfile
		wantErr bool
	}{
		{
			name: "valid profile",
			profile: VolunteerProfile{
				VolunteerID: "vol-1",
				Name:        "Maya Lin",
				Callsign:    "K9-ALPHA-1",
				PrimaryRole: RoleK9Handler,
			},
			wantErr: false,
		},
		{
			name: "empty name",
			profile: VolunteerProfile{
				VolunteerID: "vol-1",
				Name:        "   ",
				Callsign:    "K9-ALPHA-1",
				PrimaryRole: RoleK9Handler,
			},
			wantErr: true,
		},
		{
			name: "empty callsign",
			profile: VolunteerProfile{
				VolunteerID: "vol-1",
				Name:        "Maya Lin",
				Callsign:    "",
				PrimaryRole: RoleK9Handler,
			},
			wantErr: true,
		},
		{
			name: "invalid primary role",
			profile: VolunteerProfile{
				VolunteerID: "vol-1",
				Name:        "Maya Lin",
				Callsign:    "K9-ALPHA-1",
				PrimaryRole: SARRoleCredential("BAD_ROLE"),
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.profile.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.name, err)
			}
		})
	}
}

func TestICSRosterJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 10, 17, 18, 0, 0, 0, time.UTC)
	expires := now.Add(365 * 24 * time.Hour)
	checkedOut := now.Add(4 * time.Hour)

	cred := SignedCredential{
		CredentialID:  "cred-1",
		Role:          RoleUAVPart107Pilot,
		IssuedBy:      "FAA / SAR Command",
		LicenseNumber: "FAA-107-998877",
		IssuedAt:      now,
		ExpiresAt:     expires,
		SignatureHex:  "deadbeefcafebabe",
	}

	profile := VolunteerProfile{
		VolunteerID:       "vol-101",
		Name:              "Alex Rivera",
		Callsign:          "EAGLE-7",
		PrimaryRole:       RoleUAVPart107Pilot,
		Credentials:       []SignedCredential{cred},
		EarnedBadges:      []SARMeritBadge{BadgeFirstResponder, BadgeAerialSpotter, BadgeGridMaster50Km},
		TotalXP:           850,
		RankTitle:         "SAR Specialist",
		CompletedShifts:   5,
		TotalDistanceKm:   62.4,
		TotalHoursLogged:  24.5,
		VerifiedSightings: 3,
		ReunionsAssisted:  1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	profileBytes, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("marshal VolunteerProfile: %v", err)
	}
	var decodedProfile VolunteerProfile
	if err := json.Unmarshal(profileBytes, &decodedProfile); err != nil {
		t.Fatalf("unmarshal VolunteerProfile: %v", err)
	}
	if decodedProfile.VolunteerID != profile.VolunteerID ||
		decodedProfile.Callsign != profile.Callsign ||
		decodedProfile.PrimaryRole != profile.PrimaryRole ||
		len(decodedProfile.Credentials) != 1 ||
		decodedProfile.Credentials[0].LicenseNumber != cred.LicenseNumber ||
		len(decodedProfile.EarnedBadges) != 3 ||
		decodedProfile.TotalXP != profile.TotalXP {
		t.Fatalf("decoded VolunteerProfile mismatch: %+v", decodedProfile)
	}

	shift := ICSShiftEntry{
		ShiftID:           "shift-1",
		IncidentID:        "inc-2026-fire",
		VolunteerID:       "vol-101",
		VolunteerName:     "Alex Rivera",
		Callsign:          "EAGLE-7",
		AssignedRole:      RoleUAVPart107Pilot,
		AssignedSectorID:  "SECTOR-NORTH-4",
		Status:            ShiftStatusCheckedOut,
		ActiveHours:       4.0,
		DistanceCoveredKm: 18.2,
		FatigueLevel:      FatigueLevelNormal,
		FatigueNotes:      "Fit for duty",
		CheckedInAt:       now,
		LastHeartbeatAt:   checkedOut,
		CheckedOutAt:      &checkedOut,
	}

	shiftBytes, err := json.Marshal(shift)
	if err != nil {
		t.Fatalf("marshal ICSShiftEntry: %v", err)
	}
	var decodedShift ICSShiftEntry
	if err := json.Unmarshal(shiftBytes, &decodedShift); err != nil {
		t.Fatalf("unmarshal ICSShiftEntry: %v", err)
	}
	if decodedShift.ShiftID != shift.ShiftID ||
		decodedShift.IncidentID != shift.IncidentID ||
		decodedShift.Status != shift.Status ||
		decodedShift.FatigueLevel != shift.FatigueLevel ||
		decodedShift.CheckedOutAt == nil ||
		!decodedShift.CheckedOutAt.Equal(checkedOut) {
		t.Fatalf("decoded ICSShiftEntry mismatch: %+v", decodedShift)
	}
}
