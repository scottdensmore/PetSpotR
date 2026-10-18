package ics

import (
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	FatigueWarningHours      = 6.0
	MandatoryRestHours       = 8.0
	FatigueWarningDistanceKm = 18.0
	MandatoryRestDistanceKm  = 25.0
)

// EvaluateShiftFatigue evaluates active SAR shift hours and distance covered against ICS safety thresholds.
func EvaluateShiftFatigue(activeHours float64, distanceKm float64) (domain.FatigueAlertLevel, domain.ICSShiftStatus, string) {
	if activeHours >= MandatoryRestHours || distanceKm >= MandatoryRestDistanceKm {
		return domain.FatigueLevelMandatoryRest, domain.ShiftStatusMandatoryRest, "Mandatory 8h/25km SAR safety ceiling reached — stand down for rehab & rest"
	}
	if activeHours >= FatigueWarningHours || distanceKm >= FatigueWarningDistanceKm {
		return domain.FatigueLevelWarning, domain.ShiftStatusOnSectorPatrol, "Approaching 6h/18km fatigue threshold — schedule relief team"
	}
	return domain.FatigueLevelNormal, domain.ShiftStatusOnSectorPatrol, "Fit for active sector patrol"
}

// ExportICS204RosterCSV exports shift entries as an RFC 4180 ICS-204 Assignment List CSV string.
func ExportICS204RosterCSV(shifts []domain.ICSShiftEntry) (string, error) {
	var sb strings.Builder
	w := csv.NewWriter(&sb)

	header := []string{
		"shift_id",
		"incident_id",
		"volunteer_id",
		"volunteer_name",
		"callsign",
		"assigned_role",
		"assigned_sector_id",
		"status",
		"active_hours",
		"distance_covered_km",
		"fatigue_level",
		"checked_in_at",
	}
	if err := w.Write(header); err != nil {
		return "", err
	}

	for _, s := range shifts {
		row := []string{
			s.ShiftID,
			s.IncidentID,
			s.VolunteerID,
			s.VolunteerName,
			s.Callsign,
			string(s.AssignedRole),
			s.AssignedSectorID,
			string(s.Status),
			fmt.Sprintf("%.2f", s.ActiveHours),
			fmt.Sprintf("%.2f", s.DistanceCoveredKm),
			string(s.FatigueLevel),
			s.CheckedInAt.UTC().Format(time.RFC3339),
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return sb.String(), nil
}
