package veterinary_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/veterinary"
)

func TestEvaluateTriage_CanineCriticalRed(t *testing.T) {
	// Severe hypoxia and shock in dog
	vitals := domain.VitalSigns{
		HeartRateBPM:       215,
		RespiratoryRateBPM: 65,
		TemperatureF:       104.2,
		CapillaryRefillSec: 3.5,
		MucousMembrane:     domain.MMColorCyanotic,
		GlasgowComaScale:   7,
	}
	trauma := veterinary.TraumaIndicators{
		SevereBurns: true,
	}

	category, reasons := veterinary.EvaluateTriage("Dog", vitals, trauma)
	if category != domain.TriageCategoryRed {
		t.Errorf("expected TRIAGE_RED, got %s", category)
	}
	if len(reasons) == 0 {
		t.Error("expected triage evaluation reasons")
	}
}

func TestEvaluateTriage_FelineDelayedYellow(t *testing.T) {
	// Dehydration and fracture in cat
	vitals := domain.VitalSigns{
		HeartRateBPM:       180,
		RespiratoryRateBPM: 32,
		TemperatureF:       98.5,
		CapillaryRefillSec: 2.5,
		MucousMembrane:     domain.MMColorPale,
		GlasgowComaScale:   13,
	}
	trauma := veterinary.TraumaIndicators{
		OpenFracture: true,
	}

	category, _ := veterinary.EvaluateTriage("Cat", vitals, trauma)
	if category != domain.TriageCategoryYellow {
		t.Errorf("expected TRIAGE_YELLOW, got %s", category)
	}
}

func TestEvaluateTriage_StableGreen(t *testing.T) {
	vitals := domain.VitalSigns{
		HeartRateBPM:       90,
		RespiratoryRateBPM: 20,
		TemperatureF:       101.5,
		CapillaryRefillSec: 1.2,
		MucousMembrane:     domain.MMColorPink,
		GlasgowComaScale:   18,
	}
	trauma := veterinary.TraumaIndicators{}

	category, _ := veterinary.EvaluateTriage("Dog", vitals, trauma)
	if category != domain.TriageCategoryGreen {
		t.Errorf("expected TRIAGE_GREEN, got %s", category)
	}
}

func TestEvaluateTriage_DeceasedExpectantBlack(t *testing.T) {
	// Asystole / unresponsive
	trauma := veterinary.TraumaIndicators{
		UnresponsiveAsystole: true,
	}
	vitals := domain.VitalSigns{
		HeartRateBPM:       0,
		RespiratoryRateBPM: 0,
		GlasgowComaScale:   3,
	}

	category, reasons := veterinary.EvaluateTriage("Dog", vitals, trauma)
	if category != domain.TriageCategoryBlack {
		t.Errorf("expected TRIAGE_BLACK, got %s", category)
	}
	if len(reasons) == 0 {
		t.Error("expected triage evaluation reasons")
	}

	// Also without trauma flag, but zero vitals with GCS 3
	category2, _ := veterinary.EvaluateTriage("Feline", vitals, veterinary.TraumaIndicators{})
	if category2 != domain.TriageCategoryBlack {
		t.Errorf("expected TRIAGE_BLACK for unresuscitated vitals, got %s", category2)
	}
}

func TestEvaluateTriage_UnrecordedVitalsWithSevereBurns(t *testing.T) {
	// Living trauma patient entered before vitals are obtained
	trauma := veterinary.TraumaIndicators{
		SevereBurns: true,
	}
	vitals := domain.VitalSigns{} // empty / unrecorded

	category, reasons := veterinary.EvaluateTriage("Dog", vitals, trauma)
	if category != domain.TriageCategoryRed {
		t.Fatalf("expected TRIAGE_RED, got %s", category)
	}
	foundSevereBurns := false
	for _, r := range reasons {
		if r == "Extensive burns (> 30% body surface area)" {
			foundSevereBurns = true
		}
	}
	if !foundSevereBurns {
		t.Errorf("expected Extensive burns reason, got %v", reasons)
	}
}

func TestEvaluateTriage_IsolatedApneaAndAsystole(t *testing.T) {
	// Isolated apnea with palpable pulse
	apneaVitals := domain.VitalSigns{
		HeartRateBPM:       120,
		RespiratoryRateBPM: 0,
		GlasgowComaScale:   15,
	}
	catApnea, reasonsApnea := veterinary.EvaluateTriage("Dog", apneaVitals, veterinary.TraumaIndicators{})
	if catApnea != domain.TriageCategoryRed {
		t.Errorf("expected TRIAGE_RED for isolated apnea, got %s", catApnea)
	}
	foundApnea := false
	for _, r := range reasonsApnea {
		if r == "Apnea / respiratory arrest" {
			foundApnea = true
		}
	}
	if !foundApnea {
		t.Errorf("expected Apnea / respiratory arrest reason, got %v", reasonsApnea)
	}

	// Isolated pulseless arrest with agonal or active respiration
	asystoleVitals := domain.VitalSigns{
		HeartRateBPM:       0,
		RespiratoryRateBPM: 16,
		GlasgowComaScale:   14,
	}
	catAsystole, reasonsAsystole := veterinary.EvaluateTriage("Dog", asystoleVitals, veterinary.TraumaIndicators{})
	if catAsystole != domain.TriageCategoryRed {
		t.Errorf("expected TRIAGE_RED for isolated asystole, got %s", catAsystole)
	}
	foundAsystole := false
	for _, r := range reasonsAsystole {
		if r == "Absent heart rate / pulseless arrest" {
			foundAsystole = true
		}
	}
	if !foundAsystole {
		t.Errorf("expected Absent heart rate / pulseless arrest reason, got %v", reasonsAsystole)
	}
}

func TestEvaluateTriage_IctericMucousMembranes(t *testing.T) {
	vitals := domain.VitalSigns{
		HeartRateBPM:       100,
		RespiratoryRateBPM: 24,
		TemperatureF:       101.0,
		CapillaryRefillSec: 1.5,
		MucousMembrane:     domain.MMColorIcteric,
		GlasgowComaScale:   17,
	}
	category, reasons := veterinary.EvaluateTriage("Dog", vitals, veterinary.TraumaIndicators{})
	if category != domain.TriageCategoryYellow {
		t.Errorf("expected TRIAGE_YELLOW for icteric membranes, got %s", category)
	}
	foundIcteric := false
	for _, r := range reasons {
		if r == "Icteric mucous membranes (jaundice / hepatic or hemolytic crisis)" {
			foundIcteric = true
		}
	}
	if !foundIcteric {
		t.Errorf("expected icteric reason, got %v", reasons)
	}
}

func TestEvaluateTriage_CanineVsFelineThresholds(t *testing.T) {
	// Heart rate 80: Normal for dog (50-220), but critical low for cat (< 100)
	vitals := domain.VitalSigns{
		HeartRateBPM:       80,
		RespiratoryRateBPM: 25,
		TemperatureF:       101.0,
		CapillaryRefillSec: 1.5,
		MucousMembrane:     domain.MMColorPink,
		GlasgowComaScale:   18,
	}
	trauma := veterinary.TraumaIndicators{}

	dogCat, _ := veterinary.EvaluateTriage("   Dog   ", vitals, trauma)
	if dogCat != domain.TriageCategoryGreen {
		t.Errorf("expected Dog with HR 80 to be GREEN, got %s", dogCat)
	}

	catCat, catReasons := veterinary.EvaluateTriage("   Cat   ", vitals, trauma)
	if catCat != domain.TriageCategoryRed {
		t.Errorf("expected Cat with HR 80 to be RED (bradycardia), got %s", catCat)
	}
	if len(catReasons) == 0 {
		t.Error("expected reasons for feline critical HR")
	}
}

func TestCalculateEmergencyDosages(t *testing.T) {
	dosages := veterinary.CalculateEmergencyDosages("  Dog  ", 20.0)
	if dosages["ShockFluidsBolus"] != "300 mL IV over 15 min" {
		t.Errorf("unexpected fluid bolus: %s", dosages["ShockFluidsBolus"])
	}
	if dosages["EpinephrineCPR"] != "0.20 mg (0.20 mL of 1:1000) IV/IO" {
		t.Errorf("unexpected epinephrine dose: %s", dosages["EpinephrineCPR"])
	}
	if dosages["Buprenorphine"] != "0.40 mg (1.33 mL of 0.3mg/mL) IV/IM/SL" {
		t.Errorf("unexpected buprenorphine dose: %s", dosages["Buprenorphine"])
	}

	// Test Feline fluid calculation (7.5 mL/kg)
	felineDosages := veterinary.CalculateEmergencyDosages("  feline  ", 4.0)
	if felineDosages["ShockFluidsBolus"] != "30 mL IV over 15 min" {
		t.Errorf("unexpected feline fluid bolus: %s", felineDosages["ShockFluidsBolus"])
	}

	// Test default weight fallback
	defaultDosages := veterinary.CalculateEmergencyDosages("Canine", 0)
	if defaultDosages["ShockFluidsBolus"] != "150 mL IV over 15 min" {
		t.Errorf("unexpected fallback fluid bolus: %s", defaultDosages["ShockFluidsBolus"])
	}
}

func TestEvaluateTriage_PartialVitalsNoFalseArrest(t *testing.T) {
	// Case 1: Medic enters ONLY temperature (normal: 101.5 F)
	vitalsTempOnly := domain.VitalSigns{
		TemperatureF: 101.5,
	}
	catTemp, reasonsTemp := veterinary.EvaluateTriage("Dog", vitalsTempOnly, veterinary.TraumaIndicators{})
	if catTemp != domain.TriageCategoryGreen {
		t.Errorf("expected TRIAGE_GREEN for temp-only vitals, got %s (reasons: %v)", catTemp, reasonsTemp)
	}
	for _, r := range reasonsTemp {
		if r == "Absent heart rate / pulseless arrest" || r == "Apnea / respiratory arrest" {
			t.Errorf("unexpected false arrest reason: %s", r)
		}
	}

	// Case 2: Medic enters ONLY capillary refill time (normal: 1.2s)
	vitalsCrtOnly := domain.VitalSigns{
		CapillaryRefillSec: 1.2,
	}
	catCrt, reasonsCrt := veterinary.EvaluateTriage("Dog", vitalsCrtOnly, veterinary.TraumaIndicators{})
	if catCrt != domain.TriageCategoryGreen {
		t.Errorf("expected TRIAGE_GREEN for CRT-only vitals, got %s (reasons: %v)", catCrt, reasonsCrt)
	}

	// Case 3: Medic enters ONLY mucous membrane color
	vitalsMmOnly := domain.VitalSigns{
		MucousMembrane: domain.MMColorPink,
	}
	catMm, reasonsMm := veterinary.EvaluateTriage("Cat", vitalsMmOnly, veterinary.TraumaIndicators{})
	if catMm != domain.TriageCategoryGreen {
		t.Errorf("expected TRIAGE_GREEN for MM-only vitals, got %s (reasons: %v)", catMm, reasonsMm)
	}

	// Case 4: Medic actively assesses heart rate as 0 (pulseless arrest)
	vitalsAssessedPulseless := domain.VitalSigns{
		HeartRateBPM:      0,
		HeartRateAssessed: true,
	}
	catPulse, reasonsPulse := veterinary.EvaluateTriage("Dog", vitalsAssessedPulseless, veterinary.TraumaIndicators{})
	if catPulse != domain.TriageCategoryRed {
		t.Errorf("expected TRIAGE_RED for assessed pulseless arrest, got %s", catPulse)
	}
	hasPulseless := false
	for _, r := range reasonsPulse {
		if r == "Absent heart rate / pulseless arrest" {
			hasPulseless = true
		}
	}
	if !hasPulseless {
		t.Errorf("expected pulseless arrest reason, got %v", reasonsPulse)
	}

	// Case 5: Medic actively assesses respiratory rate as 0 (apnea)
	vitalsAssessedApnea := domain.VitalSigns{
		RespiratoryRateBPM: 0,
		RespRateAssessed:   true,
	}
	catApnea, reasonsApnea := veterinary.EvaluateTriage("Dog", vitalsAssessedApnea, veterinary.TraumaIndicators{})
	if catApnea != domain.TriageCategoryRed {
		t.Errorf("expected TRIAGE_RED for assessed apnea, got %s", catApnea)
	}
	hasApnea := false
	for _, r := range reasonsApnea {
		if r == "Apnea / respiratory arrest" {
			hasApnea = true
		}
	}
	if !hasApnea {
		t.Errorf("expected apnea reason, got %v", reasonsApnea)
	}
}

func TestEvaluateTriage_CanineModerateDistressYellow(t *testing.T) {
	tests := []struct {
		name          string
		vitals        domain.VitalSigns
		expectedMsg   string
		shouldTrigger bool
	}{
		{
			name: "Canine HR in [50, 60) lower moderate bradycardia",
			vitals: domain.VitalSigns{
				HeartRateBPM:       55,
				RespiratoryRateBPM: 20,
			},
			expectedMsg:   "Canine abnormal heart rate / moderate distress",
			shouldTrigger: true,
		},
		{
			name: "Canine HR in (140, 220] moderate tachycardia",
			vitals: domain.VitalSigns{
				HeartRateBPM:       180,
				RespiratoryRateBPM: 20,
			},
			expectedMsg:   "Canine abnormal heart rate / moderate distress",
			shouldTrigger: true,
		},
		{
			name: "Canine RR in [8, 10) lower moderate bradypnea",
			vitals: domain.VitalSigns{
				HeartRateBPM:       100,
				RespiratoryRateBPM: 9,
			},
			expectedMsg:   "Canine abnormal respiratory rate / tachypnea",
			shouldTrigger: true,
		},
		{
			name: "Canine RR in (30, 60] moderate tachypnea",
			vitals: domain.VitalSigns{
				HeartRateBPM:       100,
				RespiratoryRateBPM: 45,
			},
			expectedMsg:   "Canine abnormal respiratory rate / tachypnea",
			shouldTrigger: true,
		},
		{
			name: "Canine baseline normal vitals HR 100 RR 20 (Green)",
			vitals: domain.VitalSigns{
				HeartRateBPM:       100,
				RespiratoryRateBPM: 20,
			},
			expectedMsg:   "",
			shouldTrigger: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cat, reasons := veterinary.EvaluateTriage("Dog", tc.vitals, veterinary.TraumaIndicators{})
			if tc.shouldTrigger {
				if cat != domain.TriageCategoryYellow {
					t.Errorf("expected TRIAGE_YELLOW, got %s (reasons: %v)", cat, reasons)
				}
				found := false
				for _, r := range reasons {
					if r == tc.expectedMsg {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected reason %q, got %v", tc.expectedMsg, reasons)
				}
			} else {
				if cat != domain.TriageCategoryGreen {
					t.Errorf("expected TRIAGE_GREEN, got %s (reasons: %v)", cat, reasons)
				}
			}
		})
	}
}

func TestEvaluateTriage_FelineModerateDistressYellow(t *testing.T) {
	tests := []struct {
		name          string
		vitals        domain.VitalSigns
		expectedMsg   string
		shouldTrigger bool
	}{
		{
			name: "Feline HR in [100, 140) lower moderate bradycardia",
			vitals: domain.VitalSigns{
				HeartRateBPM:       120,
				RespiratoryRateBPM: 30,
			},
			expectedMsg:   "Feline abnormal heart rate / moderate distress",
			shouldTrigger: true,
		},
		{
			name: "Feline HR in (220, 260] moderate tachycardia",
			vitals: domain.VitalSigns{
				HeartRateBPM:       240,
				RespiratoryRateBPM: 30,
			},
			expectedMsg:   "Feline abnormal heart rate / moderate distress",
			shouldTrigger: true,
		},
		{
			name: "Feline RR in [12, 20) lower moderate bradypnea",
			vitals: domain.VitalSigns{
				HeartRateBPM:       180,
				RespiratoryRateBPM: 16,
			},
			expectedMsg:   "Feline abnormal respiratory rate / tachypnea",
			shouldTrigger: true,
		},
		{
			name: "Feline RR in (40, 80] moderate tachypnea",
			vitals: domain.VitalSigns{
				HeartRateBPM:       180,
				RespiratoryRateBPM: 55,
			},
			expectedMsg:   "Feline abnormal respiratory rate / tachypnea",
			shouldTrigger: true,
		},
		{
			name: "Feline baseline normal vitals HR 180 RR 30 (Green)",
			vitals: domain.VitalSigns{
				HeartRateBPM:       180,
				RespiratoryRateBPM: 30,
			},
			expectedMsg:   "",
			shouldTrigger: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cat, reasons := veterinary.EvaluateTriage("Cat", tc.vitals, veterinary.TraumaIndicators{})
			if tc.shouldTrigger {
				if cat != domain.TriageCategoryYellow {
					t.Errorf("expected TRIAGE_YELLOW, got %s (reasons: %v)", cat, reasons)
				}
				found := false
				for _, r := range reasons {
					if r == tc.expectedMsg {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected reason %q, got %v", tc.expectedMsg, reasons)
				}
			} else {
				if cat != domain.TriageCategoryGreen {
					t.Errorf("expected TRIAGE_GREEN, got %s (reasons: %v)", cat, reasons)
				}
			}
		})
	}
}
