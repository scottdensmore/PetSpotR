package veterinary

import (
	"fmt"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// TraumaIndicators represents physical trauma observed during rapid triage.
type TraumaIndicators struct {
	ArterialHemorrhage   bool `json:"arterialHemorrhage"`
	PenetratingChest     bool `json:"penetratingChest"`
	SevereBurns          bool `json:"severeBurns"`   // > 30% BSA
	ModerateBurns        bool `json:"moderateBurns"` // 10-30% BSA
	OpenFracture         bool `json:"openFracture"`
	UnresponsiveAsystole bool `json:"unresponsiveAsystole"`
}

// EvaluateTriage evaluates vitals and trauma indicators into an acuity tag.
func EvaluateTriage(species string, vitals domain.VitalSigns, trauma TraumaIndicators) (domain.TriageCategory, []string) {
	var reasons []string
	cleanSpecies := strings.TrimSpace(species)
	isCat := strings.EqualFold(cleanSpecies, "cat") || strings.EqualFold(cleanSpecies, "feline")

	hasLivingTrauma := trauma.ArterialHemorrhage ||
		trauma.PenetratingChest ||
		trauma.SevereBurns ||
		trauma.ModerateBurns ||
		trauma.OpenFracture

	// 1. Check Deceased / Expectant (Black)
	if trauma.UnresponsiveAsystole || (!hasLivingTrauma && vitals.HeartRateBPM == 0 && vitals.RespiratoryRateBPM == 0 && vitals.GlasgowComaScale == 3) {
		return domain.TriageCategoryBlack, []string{"Absence of heartbeat, respiration, and cortical responsiveness"}
	}

	// 2. Check Immediate / Critical (Red)
	if trauma.ArterialHemorrhage {
		reasons = append(reasons, "Active arterial hemorrhage")
	}
	if trauma.PenetratingChest {
		reasons = append(reasons, "Penetrating thoracic injury / open pneumothorax")
	}
	if trauma.SevereBurns {
		reasons = append(reasons, "Extensive burns (> 30% body surface area)")
	}
	if vitals.MucousMembrane == domain.MMColorCyanotic {
		reasons = append(reasons, "Cyanotic mucous membranes (severe hypoxia)")
	}
	if vitals.MucousMembrane == domain.MMColorBrickRed {
		reasons = append(reasons, "Brick-red mucous membranes (severe sepsis / hyperdynamic shock)")
	}
	if vitals.CapillaryRefillSec >= 3.0 {
		reasons = append(reasons, fmt.Sprintf("Critical hypoperfusion (CRT %.1fs >= 3.0s)", vitals.CapillaryRefillSec))
	}
	if vitals.GlasgowComaScale > 0 && vitals.GlasgowComaScale <= 8 {
		reasons = append(reasons, fmt.Sprintf("Severe mentation impairment (GCS %d <= 8)", vitals.GlasgowComaScale))
	}

	if vitals.HeartRateBPM == 0 && (vitals.HeartRateAssessed || vitals.RespiratoryRateBPM > 0) {
		reasons = append(reasons, "Absent heart rate / pulseless arrest")
	}
	if vitals.RespiratoryRateBPM == 0 && (vitals.RespRateAssessed || vitals.HeartRateBPM > 0) {
		reasons = append(reasons, "Apnea / respiratory arrest")
	}

	// Species-specific vital limits
	if isCat {
		if vitals.HeartRateBPM > 0 && (vitals.HeartRateBPM < 100 || vitals.HeartRateBPM > 260) {
			reasons = append(reasons, fmt.Sprintf("Feline critical heart rate (%d BPM)", vitals.HeartRateBPM))
		}
		if vitals.RespiratoryRateBPM > 0 && (vitals.RespiratoryRateBPM < 12 || vitals.RespiratoryRateBPM > 80) {
			reasons = append(reasons, fmt.Sprintf("Feline critical respiratory rate (%d BPM)", vitals.RespiratoryRateBPM))
		}
	} else {
		if vitals.HeartRateBPM > 0 && (vitals.HeartRateBPM < 50 || vitals.HeartRateBPM > 220) {
			reasons = append(reasons, fmt.Sprintf("Canine critical heart rate (%d BPM)", vitals.HeartRateBPM))
		}
		if vitals.RespiratoryRateBPM > 0 && (vitals.RespiratoryRateBPM < 8 || vitals.RespiratoryRateBPM > 60) {
			reasons = append(reasons, fmt.Sprintf("Canine critical respiratory rate (%d BPM)", vitals.RespiratoryRateBPM))
		}
	}

	if vitals.TemperatureF > 0 && (vitals.TemperatureF < 96.0 || vitals.TemperatureF > 105.0) {
		reasons = append(reasons, fmt.Sprintf("Critical body temperature (%.1f F)", vitals.TemperatureF))
	}

	if len(reasons) > 0 {
		return domain.TriageCategoryRed, reasons
	}

	// 3. Check Delayed / Serious (Yellow)
	if trauma.OpenFracture {
		reasons = append(reasons, "Open skeletal fracture without active arterial bleeding")
	}
	if trauma.ModerateBurns {
		reasons = append(reasons, "Moderate thermal burns (10-30% body surface area)")
	}
	if vitals.CapillaryRefillSec >= 2.0 {
		reasons = append(reasons, fmt.Sprintf("Elevated CRT (%.1fs)", vitals.CapillaryRefillSec))
	}
	if vitals.MucousMembrane == domain.MMColorPale {
		reasons = append(reasons, "Pale mucous membranes (early shock / blood loss)")
	}
	if vitals.MucousMembrane == domain.MMColorIcteric {
		reasons = append(reasons, "Icteric mucous membranes (jaundice / hepatic or hemolytic crisis)")
	}
	if vitals.GlasgowComaScale >= 9 && vitals.GlasgowComaScale <= 14 {
		reasons = append(reasons, fmt.Sprintf("Depressed mentation (GCS %d)", vitals.GlasgowComaScale))
	}
	if vitals.TemperatureF > 0 && (vitals.TemperatureF < 99.5 || vitals.TemperatureF > 103.5) {
		reasons = append(reasons, fmt.Sprintf("Abnormal temperature (%.1f F)", vitals.TemperatureF))
	}

	// Species-specific moderate distress (Yellow)
	if isCat {
		if vitals.HeartRateBPM > 0 && ((vitals.HeartRateBPM >= 100 && vitals.HeartRateBPM < 140) || (vitals.HeartRateBPM > 220 && vitals.HeartRateBPM <= 260)) {
			reasons = append(reasons, "Feline abnormal heart rate / moderate distress")
		}
		if vitals.RespiratoryRateBPM > 0 && ((vitals.RespiratoryRateBPM >= 12 && vitals.RespiratoryRateBPM < 20) || (vitals.RespiratoryRateBPM > 40 && vitals.RespiratoryRateBPM <= 80)) {
			reasons = append(reasons, "Feline abnormal respiratory rate / tachypnea")
		}
	} else {
		if vitals.HeartRateBPM > 0 && ((vitals.HeartRateBPM >= 50 && vitals.HeartRateBPM < 60) || (vitals.HeartRateBPM > 140 && vitals.HeartRateBPM <= 220)) {
			reasons = append(reasons, "Canine abnormal heart rate / moderate distress")
		}
		if vitals.RespiratoryRateBPM > 0 && ((vitals.RespiratoryRateBPM >= 8 && vitals.RespiratoryRateBPM < 10) || (vitals.RespiratoryRateBPM > 30 && vitals.RespiratoryRateBPM <= 60)) {
			reasons = append(reasons, "Canine abnormal respiratory rate / tachypnea")
		}
	}

	if len(reasons) > 0 {
		return domain.TriageCategoryYellow, reasons
	}

	// 4. Default to Walking Wounded (Green)
	return domain.TriageCategoryGreen, []string{"Stable physiological vitals within baseline bounds"}
}

// CalculateEmergencyDosages computes weight-adjusted intervention dosages.
func CalculateEmergencyDosages(species string, weightKg float64) map[string]string {
	if weightKg <= 0 {
		weightKg = 10.0 // Default baseline fallback
	}
	cleanSpecies := strings.TrimSpace(species)
	isCat := strings.EqualFold(cleanSpecies, "cat") || strings.EqualFold(cleanSpecies, "feline")

	var fluidMl float64
	if isCat {
		fluidMl = weightKg * 7.5
	} else {
		fluidMl = weightKg * 15.0
	}

	epiMg := weightKg * 0.01
	buprenorphineMg := weightKg * 0.02

	return map[string]string{
		"ShockFluidsBolus": fmt.Sprintf("%.0f mL IV over 15 min", fluidMl),
		"EpinephrineCPR":   fmt.Sprintf("%.2f mg (%.2f mL of 1:1000) IV/IO", epiMg, epiMg),
		"Buprenorphine":    fmt.Sprintf("%.2f mg (%.2f mL of 0.3mg/mL) IV/IM/SL", buprenorphineMg, buprenorphineMg/0.3),
	}
}
