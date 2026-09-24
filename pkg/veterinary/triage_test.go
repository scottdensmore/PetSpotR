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
