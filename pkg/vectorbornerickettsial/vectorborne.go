package vectorbornerickettsial

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// VectorBorneService manages Acute Vector-Borne & Rickettsial Emergencies
// (Lyme Nephritis, Anaplasmosis, Ehrlichiosis, Rocky Mountain Spotted Fever,
// Babesiosis & Cytauxzoonosis), SNAP 4Dx Plus / PCR Triage, Doxycycline,
// Atovaquone + Azithromycin & Imidocarb Dipropionate operations in a thread-safe store.
type VectorBorneService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.VectorBorneOperation
	order []string
	seq   int
}

// NewVectorBorneService initializes a VectorBorneService with 3 seeded operations
// (VBR-OP-101, VBR-OP-102, VBR-OP-103) across distinct statuses.
func NewVectorBorneService() *VectorBorneService {
	s := &VectorBorneService{
		ops:   make(map[string]*domain.VectorBorneOperation),
		order: make([]string, 0, 8),
		seq:   103,
	}
	s.seed()
	return s
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// BuildPathophysiologySummary returns the syndrome-specific pathophysiology summary.
func BuildPathophysiologySummary(syndrome domain.VectorBorneSyndrome, assay domain.VectorBorneAssay) string {
	var base string
	switch syndrome {
	case domain.VectorBorneSyndromeLymeNephritis:
		base = "Borrelia burgdorferi C6-Positive Lyme Nephritis — Immune-complex membranoproliferative glomerulonephritis (MPGN) with podocyte foot-process effacement, severe protein-losing nephropathy (UPC >= 0.5), hypoalbuminemia, antithrombin III loss, and thromboembolic/azotemic crisis."
	case domain.VectorBorneSyndromeEhrlichiaAnaplasma:
		base = "Acute Ehrlichiosis (Ehrlichia canis / E. ewingii) & Anaplasmosis (Anaplasma phagocytophilum / A. platys) — Obligate intraleukocytic/intraplatelet rickettsial morulae inducing immune-mediated platelet consumption, severe thrombocytopenia, polyarthritis, and cavitary/mucosal hemorrhage."
	case domain.VectorBorneSyndromeRMSFVasculitis:
		base = "Rocky Mountain Spotted Fever (Rickettsia rickettsii) Endothelial Vasculitis — Direct rickettsial invasion of vascular endothelium causing systemic necrotizing vasculitis, consumptive thrombocytopenia, petechiae/ecchymoses, peripheral/facial edema, and meningoencephalitis."
	case domain.VectorBorneSyndromeBabesiosis:
		base = "Canine Babesiosis (Babesia gibsoni Small Piroplasm / Babesia canis vogeli Large Piroplasm) — Intraerythrocytic protozoal parasitemia causing intravascular and extravascular immune-mediated hemolytic anemia (IMHA), splenomegaly, hemoglobinuria, and thrombocytopenia."
	case domain.VectorBorneSyndromeCytauxzoonosis:
		base = "Acute Feline Cytauxzoonosis (Cytauxzoon felis) — Amblyomma americanum tick-transmitted protozoan where schizont-laden macrophages occlude pulmonary, hepatic, and splenic venules alongside signet-ring piroplasm hemolysis, high fever transitioning to hypothermia, icterus, and pancytopenia."
	default:
		base = "Acute Vector-Borne & Rickettsial Hemoparasitic Emergency — Consumptive thrombocytopenia, hemolytic anemia, endothelial vasculitis, and immune-complex glomerulopathy."
	}
	return fmt.Sprintf("%s [Primary Assay: %s]", base, assay)
}

// BuildSafetyGuardAlerts evaluates clinical safety guards for a Vector-Borne & Rickettsial operation.
func BuildSafetyGuardAlerts(
	species string,
	syndrome domain.VectorBorneSyndrome,
	protocol domain.VectorBorneAntimicrobial,
	plateletCountKUL, pcvPercent, urinePCRatio, serumAlbuminGDL float64,
	doxyMgKgDay, atovaquoneMgKgTID, imidocarbMgKgIM float64,
	waterChaseVerified, atropinePremedVerified bool,
) []string {
	alerts := make([]string, 0, 5)

	isFeline := strings.EqualFold(strings.TrimSpace(species), "Feline") ||
		strings.Contains(strings.ToLower(species), "cat") ||
		strings.Contains(strings.ToLower(species), "felid")

	if doxyMgKgDay > 0 || protocol == domain.VectorBorneAntimicrobialDoxycyclineWaterChase {
		if isFeline {
			alerts = append(alerts, fmt.Sprintf(
				"FELINE DOXYCYCLINE ESOPHAGITIS & STRICTURE GUARD (%.1f mg/kg/day | Water Chase Verified: %t): Oral doxycycline hyclate tablets in cats MUST be followed immediately by a 5–6 mL water bolus or administered as compounded liquid suspension to prevent severe corrosive esophagitis and permanent esophageal stricture.",
				doxyMgKgDay, waterChaseVerified,
			))
		} else {
			alerts = append(alerts, fmt.Sprintf(
				"DOXYCYCLINE ANTI-RICKETTSIAL PROTOCOL (%.1f mg/kg/day PO/IV x 28 days | Water Chase Verified: %t): First-line ribosomal 30S inhibitor for Borrelia, Ehrlichia, Anaplasma, and Rickettsia rickettsii; administer with food or water bolus to minimize GI/esophageal irritation.",
				doxyMgKgDay, waterChaseVerified,
			))
		}
	}

	if atovaquoneMgKgTID > 0 || protocol == domain.VectorBorneAntimicrobialAtovaquoneAzithromycin ||
		syndrome == domain.VectorBorneSyndromeCytauxzoonosis {
		alerts = append(alerts, fmt.Sprintf(
			"ATOVAQUONE + AZITHROMYCIN PROTOZOAL BIOAVAILABILITY GUARD (Atovaquone %.1f mg/kg PO q8h TID + Azithromycin 10 mg/kg PO q24h x 10 days): Atovaquone is highly lipophilic — MUST be administered with a high-fat meal or emulsified cream to maximize enteral absorption against Babesia gibsoni and Cytauxzoon felis.",
			atovaquoneMgKgTID,
		))
	}

	if imidocarbMgKgIM > 0 || protocol == domain.VectorBorneAntimicrobialImidocarbAtropine {
		alerts = append(alerts, fmt.Sprintf(
			"IMIDOCARB DIPROPIONATE CHOLINERGIC TOXICITY GUARD (%.1f mg/kg IM repeated in 14 days | Atropine Pre-Med Verified: %t): Mandatory Atropine 0.02–0.04 mg/kg SC pre-medication 15 minutes prior to Imidocarb injection to prevent acetylcholinesterase-inhibition cholinergic crisis (profuse salivation, lacrimation, vomiting, colic, bradycardia).",
			imidocarbMgKgIM, atropinePremedVerified,
		))
	}

	if urinePCRatio >= 0.5 || syndrome == domain.VectorBorneSyndromeLymeNephritis || protocol == domain.VectorBorneAntimicrobialMycophenolateLymePLN {
		alerts = append(alerts, fmt.Sprintf(
			"LYME NEPHRITIS PROTEIN-LOSING NEPHROPATHY GUARD (UPC Ratio %.2f >= 0.5 | Serum Albumin %.1f g/dL): C6-positive proteinuria requires Doxycycline 10 mg/kg/day + RAAS blockade (Telmisartan 1 mg/kg PO q24h or Benazepril) + Mycophenolate Mofetil (10 mg/kg PO q12h) immunomodulation and Clopidogrel antithrombotic therapy if Albumin < 2.0 g/dL.",
			urinePCRatio, serumAlbuminGDL,
		))
	}

	if plateletCountKUL < 50.0 || pcvPercent < 25.0 {
		alerts = append(alerts, fmt.Sprintf(
			"CRITICAL HEMATOLOGY & BLEEDING GUARD (Platelets %.1f K/µL | PCV %.1f%%): Avoid jugular venipuncture and blind cavitary centesis when Platelets < 50 K/µL; stage crossmatched packed RBC transfusion if PCV < 20%% with clinical hypoxia.",
			plateletCountKUL, pcvPercent,
		))
	}

	return alerts
}

// BuildDefaultAntimicrobialStaging constructs an evidence-based VectorBorneAntimicrobialStaging plan.
func BuildDefaultAntimicrobialStaging(
	species string,
	weightKg float64,
	syndrome domain.VectorBorneSyndrome,
	protocol domain.VectorBorneAntimicrobial,
	doxyMgKgDay, atovaquoneMgKgTID, imidocarbMgKgIM float64,
	urinePCRatio float64,
	waterChaseVerified, atropinePremedVerified bool,
	stagedBy string,
) domain.VectorBorneAntimicrobialStaging {
	if doxyMgKgDay <= 0 && (protocol == domain.VectorBorneAntimicrobialDoxycyclineWaterChase || protocol == domain.VectorBorneAntimicrobialMycophenolateLymePLN) {
		doxyMgKgDay = 10.0
	}
	if atovaquoneMgKgTID <= 0 && protocol == domain.VectorBorneAntimicrobialAtovaquoneAzithromycin {
		atovaquoneMgKgTID = 15.0
	}
	if imidocarbMgKgIM <= 0 && protocol == domain.VectorBorneAntimicrobialImidocarbAtropine {
		imidocarbMgKgIM = 6.6
	}
	azithroMgKgDay := 0.0
	if atovaquoneMgKgTID > 0 || protocol == domain.VectorBorneAntimicrobialAtovaquoneAzithromycin {
		azithroMgKgDay = 10.0
	}

	totalDoxyMg := round1(weightKg * doxyMgKgDay)
	totalAtovaquoneMg := round1(weightKg * atovaquoneMgKgTID)
	totalImidocarbMg := round1(weightKg * imidocarbMgKgIM)
	atropineDoseMg := round2(weightKg * 0.03)

	fattyMealNote := fmt.Sprintf(
		"Atovaquone %.1f mg/kg PO q8h TID (%.1f mg/dose) + Azithromycin %.1f mg/kg PO q24h for 10 days: Administer Atovaquone with a fatty meal (or Milk/Cream slurry via nasoesophageal tube) to prevent 4-fold drop in oral bioavailability.",
		atovaquoneMgKgTID, totalAtovaquoneMg, azithroMgKgDay,
	)
	lymeNote := fmt.Sprintf(
		"Lyme Nephritis PLN Protocol (Current UPC %.2f): Doxycycline %.1f mg/kg/day (%.1f mg/day) + Telmisartan 1.0 mg/kg PO q24h (or Benazepril 0.5 mg/kg PO q12h) + Mycophenolate Mofetil 10 mg/kg PO q12h + Clopidogrel 2 mg/kg PO q24h.",
		urinePCRatio, doxyMgKgDay, totalDoxyMg,
	)
	strictureNote := fmt.Sprintf(
		"Esophageal Stricture Prevention (%s, %.1f kg | Water Chase Verified: %t): Follow every oral Doxycycline dose (%.1f mg/day) with a mandatory 5–6 mL syringe water bolus or use compounded suspension.",
		species, weightKg, waterChaseVerified, totalDoxyMg,
	)
	atropineNote := fmt.Sprintf(
		"Cholinergic Toxicity Prevention (Atropine Pre-Med Verified: %t): Administer Atropine 0.03 mg/kg SC (%.2f mg) 15 minutes prior to Imidocarb Dipropionate %.1f mg/kg IM (%.1f mg IM, repeat day 14).",
		atropinePremedVerified, atropineDoseMg, imidocarbMgKgIM, totalImidocarbMg,
	)

	if strings.TrimSpace(stagedBy) == "" {
		stagedBy = "Dr. Mara Vance, DACVIM (Infectious Disease)"
	}

	return domain.VectorBorneAntimicrobialStaging{
		Protocol:                          protocol,
		DoxycyclineMgKgDay:                round1(doxyMgKgDay),
		AtovaquoneMgKgTID:                 round1(atovaquoneMgKgTID),
		AzithromycinMgKgDay:               round1(azithroMgKgDay),
		ImidocarbMgKgIM:                   round1(imidocarbMgKgIM),
		EsophagealWaterChaseVerified:      waterChaseVerified,
		AtropinePremedVerified:            atropinePremedVerified,
		FattyMealBioavailabilityNote:      fattyMealNote,
		LymeNephritisImmunomodulationNote: lymeNote,
		EsophagealStrictureGuardNote:      strictureNote,
		CholinergicAtropineGuardNote:      atropineNote,
		StagedBy:                          stagedBy,
		ClinicalRationale: fmt.Sprintf(
			"Targeted %s antimicrobial & organ-protection regimen staged for %s (%s, %.1f kg).",
			protocol, syndrome, species, weightKg,
		),
		StagedAt: time.Now().UTC(),
	}
}

func (s *VectorBorneService) seed() {
	now := time.Now().UTC()

	// VBR-OP-101: Yukon — Canine, Golden Retriever, 31.5 kg, DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED
	staging101 := BuildDefaultAntimicrobialStaging(
		"Canine", 31.5,
		domain.VectorBorneSyndromeLymeNephritis,
		domain.VectorBorneAntimicrobialMycophenolateLymePLN,
		10.0, 0.0, 0.0, 4.8,
		true, false,
		"Dr. Mara Vance, DACVIM (Infectious Disease)",
	)
	staging101.StagedAt = now.Add(-35 * time.Minute)
	op101 := &domain.VectorBorneOperation{
		ID:                           "VBR-OP-101",
		CaseID:                       "CASE-VBR-101",
		PetName:                      "Yukon",
		Species:                      "Canine",
		Breed:                        "Golden Retriever",
		WeightKg:                     31.5,
		Status:                       domain.VectorBorneStatusAntimicrobialStaged,
		Syndrome:                     domain.VectorBorneSyndromeLymeNephritis,
		Assay:                        domain.VectorBorneAssaySNAP4DxPlusC6,
		AntimicrobialProtocol:        domain.VectorBorneAntimicrobialMycophenolateLymePLN,
		PlateletCountKUL:             42.0,
		PCVPercent:                   26.5,
		UrinePCRatio:                 4.8,
		SerumAlbuminGDL:              1.6,
		DoxycyclineMgKgDay:           10.0,
		AtovaquoneMgKgTID:            0.0,
		ImidocarbMgKgIM:              0.0,
		LymeNephritisUPCPositive:     true,
		EsophagealWaterChaseVerified: true,
		AtropinePremedVerified:       false,
		PathophysiologySummary: BuildPathophysiologySummary(
			domain.VectorBorneSyndromeLymeNephritis,
			domain.VectorBorneAssaySNAP4DxPlusC6,
		),
		SafetyGuardAlerts: BuildSafetyGuardAlerts(
			"Canine",
			domain.VectorBorneSyndromeLymeNephritis,
			domain.VectorBorneAntimicrobialMycophenolateLymePLN,
			42.0, 26.5, 4.8, 1.6,
			10.0, 0.0, 0.0,
			true, false,
		),
		AntimicrobialStaging: &staging101,
		HematologyChecks: []domain.VectorBorneHematologyCheck{
			{
				ID:                         "VBR-CHK-101-1",
				PlateletCountKUL:           36.0,
				PCVPercent:                 25.0,
				UrinePCRatio:               5.4,
				SerumAlbuminGDL:            1.5,
				MorulaeOrPiroplasmsCleared: false,
				RecordedBy:                 "Dr. Mara Vance, DACVIM (Infectious Disease)",
				ClinicalNote:               "ICU Admission: SNAP 4Dx Plus strongly positive for Borrelia burgdorferi C6 peptide & Anaplasma spp.; severe protein-losing nephropathy (UPC 5.4), hypoalbuminemia (1.5 g/dL), and thrombocytopenia (36 K/µL).",
				RecordedAt:                 now.Add(-60 * time.Minute),
			},
			{
				ID:                         "VBR-CHK-101-2",
				PlateletCountKUL:           42.0,
				PCVPercent:                 26.5,
				UrinePCRatio:               4.8,
				SerumAlbuminGDL:            1.6,
				MorulaeOrPiroplasmsCleared: false,
				RecordedBy:                 "Dr. Mara Vance, DACVIM (Infectious Disease)",
				ClinicalNote:               "Staged Doxycycline 10 mg/kg/day (315 mg/day PO with water bolus) + Telmisartan 1 mg/kg/day + Mycophenolate Mofetil 10 mg/kg q12h + Clopidogrel 2 mg/kg/day for Lyme Nephritis.",
				RecordedAt:                 now.Add(-25 * time.Minute),
			},
		},
		CreatedAt: now.Add(-75 * time.Minute),
		UpdatedAt: now.Add(-25 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// VBR-OP-102: Willow — Feline, Domestic Shorthair, 4.4 kg, THROMBOCYTOPENIA_PROTEINURIA_MONITORED
	staging102 := BuildDefaultAntimicrobialStaging(
		"Feline", 4.4,
		domain.VectorBorneSyndromeCytauxzoonosis,
		domain.VectorBorneAntimicrobialAtovaquoneAzithromycin,
		10.0, 15.0, 0.0, 0.3,
		true, false,
		"Dr. Mara Vance, DACVIM (Infectious Disease)",
	)
	staging102.StagedAt = now.Add(-18 * time.Hour)
	op102 := &domain.VectorBorneOperation{
		ID:                           "VBR-OP-102",
		CaseID:                       "CASE-VBR-102",
		PetName:                      "Willow",
		Species:                      "Feline",
		Breed:                        "Domestic Shorthair",
		WeightKg:                     4.4,
		Status:                       domain.VectorBorneStatusThrombocytopeniaMonitored,
		Syndrome:                     domain.VectorBorneSyndromeCytauxzoonosis,
		Assay:                        domain.VectorBorneAssayBuffyCoatSmear,
		AntimicrobialProtocol:        domain.VectorBorneAntimicrobialAtovaquoneAzithromycin,
		PlateletCountKUL:             95.0,
		PCVPercent:                   29.0,
		UrinePCRatio:                 0.3,
		SerumAlbuminGDL:              2.5,
		DoxycyclineMgKgDay:           10.0,
		AtovaquoneMgKgTID:            15.0,
		ImidocarbMgKgIM:              0.0,
		LymeNephritisUPCPositive:     false,
		EsophagealWaterChaseVerified: true,
		AtropinePremedVerified:       false,
		PathophysiologySummary: BuildPathophysiologySummary(
			domain.VectorBorneSyndromeCytauxzoonosis,
			domain.VectorBorneAssayBuffyCoatSmear,
		),
		SafetyGuardAlerts: BuildSafetyGuardAlerts(
			"Feline",
			domain.VectorBorneSyndromeCytauxzoonosis,
			domain.VectorBorneAntimicrobialAtovaquoneAzithromycin,
			95.0, 29.0, 0.3, 2.5,
			10.0, 15.0, 0.0,
			true, false,
		),
		AntimicrobialStaging: &staging102,
		HematologyChecks: []domain.VectorBorneHematologyCheck{
			{
				ID:                         "VBR-CHK-102-1",
				PlateletCountKUL:           95.0,
				PCVPercent:                 29.0,
				UrinePCRatio:               0.3,
				SerumAlbuminGDL:            2.5,
				MorulaeOrPiroplasmsCleared: false,
				RecordedBy:                 "Dr. Mara Vance, DACVIM (Infectious Disease)",
				ClinicalNote:               "Day 3 Cytauxzoon felis monitoring: Receiving Atovaquone 15 mg/kg PO TID (66 mg q8h with high-fat recovery diet via NE tube) + Azithromycin 10 mg/kg PO q24h + compounded Doxycycline with 6 mL water chase; platelets rebounding to 95 K/µL, PCV 29.0%.",
				RecordedAt:                 now.Add(-90 * time.Minute),
			},
		},
		CreatedAt: now.Add(-48 * time.Hour),
		UpdatedAt: now.Add(-90 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// VBR-OP-103: Boone — Canine, American Pit Bull Terrier, 24.0 kg, PCR_NEGATIVE_PLATELETS_RENAL_CLEARED
	staging103 := BuildDefaultAntimicrobialStaging(
		"Canine", 24.0,
		domain.VectorBorneSyndromeBabesiosis,
		domain.VectorBorneAntimicrobialImidocarbAtropine,
		10.0, 13.5, 6.6, 0.2,
		true, true,
		"Dr. Mara Vance, DACVIM (Infectious Disease)",
	)
	staging103.StagedAt = now.Add(-14 * 24 * time.Hour)
	clearedAt := now.Add(-30 * time.Minute)
	clearance103 := domain.VectorBorneClearance{
		PlateletCountKUL:           215.0,
		PCVPercent:                 41.0,
		UrinePCRatio:               0.2,
		PCRNegativeVerified:        true,
		NormothermicEatingVerified: true,
		ClearedBy:                  "Dr. Mara Vance, DACVIM (Infectious Disease)",
		ClearanceSummary:           "Quantitative Babesia PCR panel negative, platelets normalized to 215 K/µL (>= 150 K/µL), PCV regenerated to 41.0% (>= 35%), UPC 0.2 (< 0.5) after Atropine-premedicated Imidocarb Dipropionate 6.6 mg/kg IM & Atovaquone/Azithromycin protocol.",
		ClearedAt:                  clearedAt,
	}
	op103 := &domain.VectorBorneOperation{
		ID:                           "VBR-OP-103",
		CaseID:                       "CASE-VBR-103",
		PetName:                      "Boone",
		Species:                      "Canine",
		Breed:                        "American Pit Bull Terrier",
		WeightKg:                     24.0,
		Status:                       domain.VectorBorneStatusPCRNegativeCleared,
		Syndrome:                     domain.VectorBorneSyndromeBabesiosis,
		Assay:                        domain.VectorBorneAssayQuantitativePCR,
		AntimicrobialProtocol:        domain.VectorBorneAntimicrobialImidocarbAtropine,
		PlateletCountKUL:             215.0,
		PCVPercent:                   41.0,
		UrinePCRatio:                 0.2,
		SerumAlbuminGDL:              3.4,
		DoxycyclineMgKgDay:           10.0,
		AtovaquoneMgKgTID:            13.5,
		ImidocarbMgKgIM:              6.6,
		LymeNephritisUPCPositive:     false,
		EsophagealWaterChaseVerified: true,
		AtropinePremedVerified:       true,
		PathophysiologySummary: BuildPathophysiologySummary(
			domain.VectorBorneSyndromeBabesiosis,
			domain.VectorBorneAssayQuantitativePCR,
		),
		SafetyGuardAlerts: BuildSafetyGuardAlerts(
			"Canine",
			domain.VectorBorneSyndromeBabesiosis,
			domain.VectorBorneAntimicrobialImidocarbAtropine,
			215.0, 41.0, 0.2, 3.4,
			10.0, 13.5, 6.6,
			true, true,
		),
		AntimicrobialStaging: &staging103,
		HematologyChecks: []domain.VectorBorneHematologyCheck{
			{
				ID:                         "VBR-CHK-103-1",
				PlateletCountKUL:           135.0,
				PCVPercent:                 33.0,
				UrinePCRatio:               0.3,
				SerumAlbuminGDL:            3.1,
				MorulaeOrPiroplasmsCleared: true,
				RecordedBy:                 "Dr. Mara Vance, DACVIM (Infectious Disease)",
				ClinicalNote:               "Day 7 post-Imidocarb Dipropionate (6.6 mg/kg IM after Atropine 0.03 mg/kg SC pre-med) + Atovaquone 13.5 mg/kg TID with fatty meal: no cholinergic side effects, piroplasms absent on buffy coat smear.",
				RecordedAt:                 now.Add(-7 * 24 * time.Hour),
			},
			{
				ID:                         "VBR-CHK-103-2",
				PlateletCountKUL:           215.0,
				PCVPercent:                 41.0,
				UrinePCRatio:               0.2,
				SerumAlbuminGDL:            3.4,
				MorulaeOrPiroplasmsCleared: true,
				RecordedBy:                 "Dr. Mara Vance, DACVIM (Infectious Disease)",
				ClinicalNote:               "Day 14 repeat Imidocarb & Quantitative PCR Verification: PCR negative for Babesia spp., Platelets 215 K/µL, PCV 41.0%, UPC 0.2, normothermic and eating.",
				RecordedAt:                 now.Add(-45 * time.Minute),
			},
		},
		Clearance:        &clearance103,
		DischargedBy:     clearance103.ClearedBy,
		DischargeSummary: clearance103.ClearanceSummary,
		DischargedAt:     &clearedAt,
		CreatedAt:        now.Add(-14 * 24 * time.Hour),
		UpdatedAt:        clearedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Vector-Borne & Rickettsial operations,
// optionally filtered by statusFilter.
func (s *VectorBorneService) ListOperations(statusFilter string) []domain.VectorBorneOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.VectorBorneOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneVectorBorneOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a Vector-Borne & Rickettsial operation by ID.
func (s *VectorBorneService) GetOperation(id string) (*domain.VectorBorneOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("vector-borne rickettsial operation %q not found", id)
	}
	cp := cloneVectorBorneOperation(op)
	return &cp, nil
}

// CalculateOverview computes aggregate metrics across all Vector-Borne & Rickettsial operations.
func (s *VectorBorneService) CalculateOverview() domain.VectorBorneOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ov domain.VectorBorneOverview
	ov.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.VectorBorneStatusAntimicrobialStaged:
			ov.AntimicrobialStagedCount++
		case domain.VectorBorneStatusThrombocytopeniaMonitored:
			ov.ThrombocytopeniaMonitoredCount++
		case domain.VectorBorneStatusPCRNegativeCleared:
			ov.PCRNegativeClearedCount++
		}
		if op.LymeNephritisUPCPositive || op.UrinePCRatio >= 0.5 || op.Syndrome == domain.VectorBorneSyndromeLymeNephritis {
			ov.LymeNephritisProteinuriaCount++
		}
		if op.PlateletCountKUL < 100.0 {
			ov.SevereThrombocytopeniaCount++
		}
		if op.Syndrome == domain.VectorBorneSyndromeBabesiosis || op.Syndrome == domain.VectorBorneSyndromeCytauxzoonosis {
			ov.BabesiaCytauxzoonProtozoalCount++
		}
	}
	return ov
}

// GetOverviewMetrics is an alias for CalculateOverview for handler consistency.
func (s *VectorBorneService) GetOverviewMetrics() domain.VectorBorneOverview {
	return s.CalculateOverview()
}

// CreateOperation validates and creates a new Acute Vector-Borne & Rickettsial Emergency triage operation.
func (s *VectorBorneService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	syndrome domain.VectorBorneSyndrome,
	assay domain.VectorBorneAssay,
	antimicrobialProtocol domain.VectorBorneAntimicrobial,
	plateletCountKUL, pcvPercent, urinePCRatio, serumAlbuminGDL float64,
	doxyMgKgDay, atovaquoneMgKgTID, imidocarbMgKgIM float64,
	waterChaseVerified, atropinePremedVerified bool,
) (*domain.VectorBorneOperation, error) {
	trimmedCase := strings.TrimSpace(caseID)
	if trimmedCase == "" {
		return nil, fmt.Errorf("case_id is required")
	}
	trimmedPet := strings.TrimSpace(petName)
	if trimmedPet == "" {
		return nil, fmt.Errorf("pet_name is required")
	}
	trimmedSpecies := strings.TrimSpace(species)
	if trimmedSpecies == "" {
		if syndrome == domain.VectorBorneSyndromeCytauxzoonosis {
			trimmedSpecies = "Feline"
		} else {
			trimmedSpecies = "Canine"
		}
	}
	trimmedBreed := strings.TrimSpace(breed)
	if trimmedBreed == "" {
		trimmedBreed = "Mixed Breed"
	}
	if weightKg <= 0 || weightKg > 120.0 {
		return nil, fmt.Errorf("weight_kg must be in (0, 120.0], got %v", weightKg)
	}
	if !syndrome.IsValid() {
		return nil, fmt.Errorf("invalid syndrome: %q", syndrome)
	}
	if assay == "" {
		assay = domain.VectorBorneAssaySNAP4DxPlusC6
	}
	if !assay.IsValid() {
		return nil, fmt.Errorf("invalid assay: %q", assay)
	}
	if antimicrobialProtocol == "" {
		switch syndrome {
		case domain.VectorBorneSyndromeLymeNephritis:
			antimicrobialProtocol = domain.VectorBorneAntimicrobialMycophenolateLymePLN
		case domain.VectorBorneSyndromeCytauxzoonosis:
			antimicrobialProtocol = domain.VectorBorneAntimicrobialAtovaquoneAzithromycin
		case domain.VectorBorneSyndromeBabesiosis:
			antimicrobialProtocol = domain.VectorBorneAntimicrobialImidocarbAtropine
		default:
			antimicrobialProtocol = domain.VectorBorneAntimicrobialDoxycyclineWaterChase
		}
	}
	if !antimicrobialProtocol.IsValid() {
		return nil, fmt.Errorf("invalid antimicrobial_protocol: %q", antimicrobialProtocol)
	}
	if plateletCountKUL <= 0 || plateletCountKUL > 1000.0 {
		return nil, fmt.Errorf("platelet_count_kul must be in (0, 1000.0], got %v", plateletCountKUL)
	}
	if pcvPercent <= 0 || pcvPercent > 75.0 {
		return nil, fmt.Errorf("pcv_percent must be in (0, 75.0], got %v", pcvPercent)
	}
	if urinePCRatio < 0 || urinePCRatio > 50.0 {
		return nil, fmt.Errorf("urine_pc_ratio must be in [0, 50.0], got %v", urinePCRatio)
	}
	if serumAlbuminGDL <= 0 || serumAlbuminGDL > 6.0 {
		return nil, fmt.Errorf("serum_albumin_g_dl must be in (0, 6.0], got %v", serumAlbuminGDL)
	}
	if doxyMgKgDay < 0 || doxyMgKgDay > 30.0 {
		return nil, fmt.Errorf("doxycycline_mg_kg_day must be in [0, 30.0], got %v", doxyMgKgDay)
	}
	if atovaquoneMgKgTID < 0 || atovaquoneMgKgTID > 40.0 {
		return nil, fmt.Errorf("atovaquone_mg_kg_tid must be in [0, 40.0], got %v", atovaquoneMgKgTID)
	}
	if imidocarbMgKgIM < 0 || imidocarbMgKgIM > 15.0 {
		return nil, fmt.Errorf("imidocarb_mg_kg_im must be in [0, 15.0], got %v", imidocarbMgKgIM)
	}

	isFeline := strings.EqualFold(trimmedSpecies, "Feline") ||
		strings.Contains(strings.ToLower(trimmedSpecies), "cat") ||
		strings.Contains(strings.ToLower(trimmedSpecies), "felid")
	if isFeline && doxyMgKgDay > 0 && !waterChaseVerified {
		return nil, fmt.Errorf("feline oral doxycycline requires esophageal_water_chase_verified=true (5-6 mL water bolus or compounded suspension) to prevent esophageal stricture")
	}
	if imidocarbMgKgIM > 0 && !atropinePremedVerified {
		return nil, fmt.Errorf("imidocarb dipropionate requires atropine_premed_verified=true (Atropine 0.02-0.04 mg/kg SC 15 min prior) to prevent cholinergic toxicity")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("VBR-OP-%d", s.seq)
	now := time.Now().UTC()

	lymeUPCPositive := urinePCRatio >= 0.5 || syndrome == domain.VectorBorneSyndromeLymeNephritis
	staging := BuildDefaultAntimicrobialStaging(
		trimmedSpecies, weightKg,
		syndrome, antimicrobialProtocol,
		doxyMgKgDay, atovaquoneMgKgTID, imidocarbMgKgIM,
		urinePCRatio,
		waterChaseVerified, atropinePremedVerified,
		"Dr. Mara Vance, DACVIM (Infectious Disease)",
	)

	op := &domain.VectorBorneOperation{
		ID:                           opID,
		CaseID:                       trimmedCase,
		PetName:                      trimmedPet,
		Species:                      trimmedSpecies,
		Breed:                        trimmedBreed,
		WeightKg:                     round1(weightKg),
		Status:                       domain.VectorBorneStatusAntimicrobialStaged,
		Syndrome:                     syndrome,
		Assay:                        assay,
		AntimicrobialProtocol:        antimicrobialProtocol,
		PlateletCountKUL:             round1(plateletCountKUL),
		PCVPercent:                   round1(pcvPercent),
		UrinePCRatio:                 round2(urinePCRatio),
		SerumAlbuminGDL:              round1(serumAlbuminGDL),
		DoxycyclineMgKgDay:           round1(doxyMgKgDay),
		AtovaquoneMgKgTID:            round1(atovaquoneMgKgTID),
		ImidocarbMgKgIM:              round1(imidocarbMgKgIM),
		LymeNephritisUPCPositive:     lymeUPCPositive,
		EsophagealWaterChaseVerified: waterChaseVerified,
		AtropinePremedVerified:       atropinePremedVerified,
		PathophysiologySummary:       BuildPathophysiologySummary(syndrome, assay),
		SafetyGuardAlerts: BuildSafetyGuardAlerts(
			trimmedSpecies, syndrome, antimicrobialProtocol,
			plateletCountKUL, pcvPercent, urinePCRatio, serumAlbuminGDL,
			doxyMgKgDay, atovaquoneMgKgTID, imidocarbMgKgIM,
			waterChaseVerified, atropinePremedVerified,
		),
		AntimicrobialStaging: &staging,
		HematologyChecks:     []domain.VectorBorneHematologyCheck{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneVectorBorneOperation(op)
	return &cp, nil
}

// StageAntimicrobial validates safety guards and stages the anti-rickettsial / anti-protozoal /
// Lyme nephritis regimen on an existing operation.
func (s *VectorBorneService) StageAntimicrobial(
	id string,
	protocol domain.VectorBorneAntimicrobial,
	doxyMgKgDay, atovaquoneMgKgTID, imidocarbMgKgIM float64,
	waterChaseVerified, atropinePremedVerified bool,
	fattyMealNote, lymePLNNote, stagedBy, clinicalRationale string,
) (*domain.VectorBorneOperation, error) {
	if !protocol.IsValid() {
		return nil, fmt.Errorf("invalid antimicrobial_protocol: %q", protocol)
	}
	if doxyMgKgDay < 0 || doxyMgKgDay > 30.0 {
		return nil, fmt.Errorf("doxycycline_mg_kg_day must be in [0, 30.0], got %v", doxyMgKgDay)
	}
	if atovaquoneMgKgTID < 0 || atovaquoneMgKgTID > 40.0 {
		return nil, fmt.Errorf("atovaquone_mg_kg_tid must be in [0, 40.0], got %v", atovaquoneMgKgTID)
	}
	if imidocarbMgKgIM < 0 || imidocarbMgKgIM > 15.0 {
		return nil, fmt.Errorf("imidocarb_mg_kg_im must be in [0, 15.0], got %v", imidocarbMgKgIM)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("vector-borne rickettsial operation %q not found", id)
	}

	isFeline := strings.EqualFold(op.Species, "Feline") ||
		strings.Contains(strings.ToLower(op.Species), "cat") ||
		strings.Contains(strings.ToLower(op.Species), "felid")
	if isFeline && (doxyMgKgDay > 0 || protocol == domain.VectorBorneAntimicrobialDoxycyclineWaterChase) && !waterChaseVerified {
		return nil, fmt.Errorf("feline oral doxycycline requires esophageal_water_chase_verified=true (5-6 mL water bolus or compounded suspension) to prevent esophageal stricture")
	}
	if (imidocarbMgKgIM > 0 || protocol == domain.VectorBorneAntimicrobialImidocarbAtropine) && !atropinePremedVerified {
		return nil, fmt.Errorf("imidocarb dipropionate requires atropine_premed_verified=true (Atropine 0.02-0.04 mg/kg SC 15 min prior) to prevent cholinergic toxicity")
	}

	staging := BuildDefaultAntimicrobialStaging(
		op.Species, op.WeightKg,
		op.Syndrome, protocol,
		doxyMgKgDay, atovaquoneMgKgTID, imidocarbMgKgIM,
		op.UrinePCRatio,
		waterChaseVerified, atropinePremedVerified,
		stagedBy,
	)
	if strings.TrimSpace(fattyMealNote) != "" {
		staging.FattyMealBioavailabilityNote = strings.TrimSpace(fattyMealNote)
	}
	if strings.TrimSpace(lymePLNNote) != "" {
		staging.LymeNephritisImmunomodulationNote = strings.TrimSpace(lymePLNNote)
	}
	if strings.TrimSpace(clinicalRationale) != "" {
		staging.ClinicalRationale = strings.TrimSpace(clinicalRationale)
	}

	op.AntimicrobialProtocol = protocol
	op.DoxycyclineMgKgDay = staging.DoxycyclineMgKgDay
	op.AtovaquoneMgKgTID = staging.AtovaquoneMgKgTID
	op.ImidocarbMgKgIM = staging.ImidocarbMgKgIM
	op.EsophagealWaterChaseVerified = waterChaseVerified
	op.AtropinePremedVerified = atropinePremedVerified
	op.AntimicrobialStaging = &staging
	op.Status = domain.VectorBorneStatusAntimicrobialStaged
	op.Clearance = nil
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.SafetyGuardAlerts = BuildSafetyGuardAlerts(
		op.Species, op.Syndrome, op.AntimicrobialProtocol,
		op.PlateletCountKUL, op.PCVPercent, op.UrinePCRatio, op.SerumAlbuminGDL,
		op.DoxycyclineMgKgDay, op.AtovaquoneMgKgTID, op.ImidocarbMgKgIM,
		op.EsophagealWaterChaseVerified, op.AtropinePremedVerified,
	)
	op.UpdatedAt = time.Now().UTC()

	cp := cloneVectorBorneOperation(op)
	return &cp, nil
}

// RecordHematologyCheck appends a serial VectorBorneHematologyCheck, updates platelet count,
// PCV%, UPC ratio, and serum albumin, and transitions the operation to THROMBOCYTOPENIA_PROTEINURIA_MONITORED.
func (s *VectorBorneService) RecordHematologyCheck(
	id string,
	plateletCountKUL, pcvPercent, urinePCRatio, serumAlbuminGDL float64,
	morulaeOrPiroplasmsCleared bool,
	recordedBy, clinicalNote string,
) (*domain.VectorBorneOperation, error) {
	if plateletCountKUL <= 0 || plateletCountKUL > 1000.0 {
		return nil, fmt.Errorf("platelet_count_kul must be in (0, 1000.0], got %v", plateletCountKUL)
	}
	if pcvPercent <= 0 || pcvPercent > 75.0 {
		return nil, fmt.Errorf("pcv_percent must be in (0, 75.0], got %v", pcvPercent)
	}
	if urinePCRatio < 0 || urinePCRatio > 50.0 {
		return nil, fmt.Errorf("urine_pc_ratio must be in [0, 50.0], got %v", urinePCRatio)
	}
	if serumAlbuminGDL <= 0 || serumAlbuminGDL > 6.0 {
		return nil, fmt.Errorf("serum_albumin_g_dl must be in (0, 6.0], got %v", serumAlbuminGDL)
	}
	trimmedBy := strings.TrimSpace(recordedBy)
	if trimmedBy == "" {
		return nil, fmt.Errorf("recorded_by is required")
	}
	trimmedNote := strings.TrimSpace(clinicalNote)

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("vector-borne rickettsial operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.VectorBorneHematologyCheck{
		ID:                         fmt.Sprintf("VBR-CHK-%s-%d", op.ID, len(op.HematologyChecks)+1),
		PlateletCountKUL:           round1(plateletCountKUL),
		PCVPercent:                 round1(pcvPercent),
		UrinePCRatio:               round2(urinePCRatio),
		SerumAlbuminGDL:            round1(serumAlbuminGDL),
		MorulaeOrPiroplasmsCleared: morulaeOrPiroplasmsCleared,
		RecordedBy:                 trimmedBy,
		ClinicalNote:               trimmedNote,
		RecordedAt:                 now,
	}

	op.HematologyChecks = append(op.HematologyChecks, chk)
	op.PlateletCountKUL = chk.PlateletCountKUL
	op.PCVPercent = chk.PCVPercent
	op.UrinePCRatio = chk.UrinePCRatio
	op.SerumAlbuminGDL = chk.SerumAlbuminGDL
	op.LymeNephritisUPCPositive = op.UrinePCRatio >= 0.5 || op.Syndrome == domain.VectorBorneSyndromeLymeNephritis
	op.Status = domain.VectorBorneStatusThrombocytopeniaMonitored
	op.SafetyGuardAlerts = BuildSafetyGuardAlerts(
		op.Species, op.Syndrome, op.AntimicrobialProtocol,
		op.PlateletCountKUL, op.PCVPercent, op.UrinePCRatio, op.SerumAlbuminGDL,
		op.DoxycyclineMgKgDay, op.AtovaquoneMgKgTID, op.ImidocarbMgKgIM,
		op.EsophagealWaterChaseVerified, op.AtropinePremedVerified,
	)
	op.UpdatedAt = now

	cp := cloneVectorBorneOperation(op)
	return &cp, nil
}

// EvaluateClearance validates strict discharge clearance thresholds:
//   - platelet_count_kul >= 150
//   - pcv_percent >= 35
//   - urine_pc_ratio < 0.5
//   - pcr_negative_verified == true
//   - normothermic_eating_verified == true
//
// and transitions the operation to PCR_NEGATIVE_PLATELETS_RENAL_CLEARED.
func (s *VectorBorneService) EvaluateClearance(
	id string,
	plateletCountKUL, pcvPercent, urinePCRatio float64,
	pcrNegativeVerified, normothermicEatingVerified bool,
	clearedBy, clearanceSummary string,
) (*domain.VectorBorneOperation, error) {
	if plateletCountKUL < 150.0 {
		return nil, fmt.Errorf("clearance requires platelet_count_kul >= 150.0 K/uL, got %v", plateletCountKUL)
	}
	if pcvPercent < 35.0 {
		return nil, fmt.Errorf("clearance requires pcv_percent >= 35.0%%, got %v", pcvPercent)
	}
	if urinePCRatio >= 0.5 || urinePCRatio < 0 {
		return nil, fmt.Errorf("clearance requires urine_pc_ratio < 0.5, got %v", urinePCRatio)
	}
	if !pcrNegativeVerified {
		return nil, fmt.Errorf("clearance requires pcr_negative_verified == true")
	}
	if !normothermicEatingVerified {
		return nil, fmt.Errorf("clearance requires normothermic_eating_verified == true")
	}
	trimmedBy := strings.TrimSpace(clearedBy)
	if trimmedBy == "" {
		return nil, fmt.Errorf("cleared_by is required")
	}
	trimmedSummary := strings.TrimSpace(clearanceSummary)
	if trimmedSummary == "" {
		return nil, fmt.Errorf("clearance_summary is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("vector-borne rickettsial operation %q not found", id)
	}

	now := time.Now().UTC()
	cl := domain.VectorBorneClearance{
		PlateletCountKUL:           round1(plateletCountKUL),
		PCVPercent:                 round1(pcvPercent),
		UrinePCRatio:               round2(urinePCRatio),
		PCRNegativeVerified:        pcrNegativeVerified,
		NormothermicEatingVerified: normothermicEatingVerified,
		ClearedBy:                  trimmedBy,
		ClearanceSummary:           trimmedSummary,
		ClearedAt:                  now,
	}

	op.PlateletCountKUL = cl.PlateletCountKUL
	op.PCVPercent = cl.PCVPercent
	op.UrinePCRatio = cl.UrinePCRatio
	op.LymeNephritisUPCPositive = false
	op.Status = domain.VectorBorneStatusPCRNegativeCleared
	op.Clearance = &cl
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.SafetyGuardAlerts = BuildSafetyGuardAlerts(
		op.Species, op.Syndrome, op.AntimicrobialProtocol,
		op.PlateletCountKUL, op.PCVPercent, op.UrinePCRatio, op.SerumAlbuminGDL,
		op.DoxycyclineMgKgDay, op.AtovaquoneMgKgTID, op.ImidocarbMgKgIM,
		op.EsophagealWaterChaseVerified, op.AtropinePremedVerified,
	)
	op.UpdatedAt = now

	cp := cloneVectorBorneOperation(op)
	return &cp, nil
}

func cloneVectorBorneOperation(op *domain.VectorBorneOperation) domain.VectorBorneOperation {
	cp := *op
	if op.SafetyGuardAlerts != nil {
		cp.SafetyGuardAlerts = make([]string, len(op.SafetyGuardAlerts))
		copy(cp.SafetyGuardAlerts, op.SafetyGuardAlerts)
	}
	if op.AntimicrobialStaging != nil {
		st := *op.AntimicrobialStaging
		cp.AntimicrobialStaging = &st
	}
	if op.HematologyChecks != nil {
		cp.HematologyChecks = make([]domain.VectorBorneHematologyCheck, len(op.HematologyChecks))
		copy(cp.HematologyChecks, op.HematologyChecks)
	}
	if op.Clearance != nil {
		cl := *op.Clearance
		cp.Clearance = &cl
	}
	if op.DischargedAt != nil {
		t := *op.DischargedAt
		cp.DischargedAt = &t
	}
	return cp
}
