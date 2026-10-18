package ics

import (
	"github.com/scottdensmore/petspotr/pkg/domain"
)

// EvaluateVolunteerProgression calculates SAR XP, rank title, and merit badges for a volunteer profile.
func EvaluateVolunteerProgression(profile domain.VolunteerProfile, nightShift bool) domain.VolunteerProfile {
	xp := profile.CompletedShifts*100 +
		int(profile.TotalDistanceKm*20.0) +
		int(profile.TotalHoursLogged*25.0) +
		profile.VerifiedSightings*150 +
		profile.ReunionsAssisted*500 +
		len(profile.Credentials)*200

	profile.TotalXP = xp

	switch {
	case xp >= 3000:
		profile.RankTitle = "Master Incident Vanguard (Tier 4)"
	case xp >= 1500:
		profile.RankTitle = "Senior SAR Specialist (Tier 3)"
	case xp >= 500:
		profile.RankTitle = "Field Tracker (Tier 2)"
	default:
		profile.RankTitle = "Rookie Spotter (Tier 1)"
	}

	seen := make(map[domain.SARMeritBadge]bool)
	var badges []domain.SARMeritBadge
	addBadge := func(b domain.SARMeritBadge) {
		if b == "" || seen[b] {
			return
		}
		seen[b] = true
		badges = append(badges, b)
	}

	for _, existing := range profile.EarnedBadges {
		addBadge(existing)
	}

	hasRole := func(r domain.SARRoleCredential) bool {
		if profile.PrimaryRole == r {
			return true
		}
		for _, c := range profile.Credentials {
			if c.Role == r {
				return true
			}
		}
		return false
	}

	if profile.CompletedShifts >= 1 {
		addBadge(domain.BadgeFirstResponder)
	}
	if nightShift {
		addBadge(domain.BadgeNightOwlIR)
	}
	if profile.TotalDistanceKm >= 50.0 {
		addBadge(domain.BadgeGridMaster50Km)
	}
	if hasRole(domain.RoleK9Handler) {
		addBadge(domain.BadgeCanineTracker)
	}
	if hasRole(domain.RoleUAVPart107Pilot) {
		addBadge(domain.BadgeAerialSpotter)
	}
	if hasRole(domain.RoleVetMedic) {
		addBadge(domain.BadgeTriageLifesaver)
	}
	if profile.ReunionsAssisted >= 1 {
		addBadge(domain.BadgeReunionHero)
	}

	profile.EarnedBadges = badges
	return profile
}
