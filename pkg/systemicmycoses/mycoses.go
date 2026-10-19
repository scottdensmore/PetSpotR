package systemicmycoses

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// SystemicMycosesService manages Acute Systemic Mycoses & Fungal Emergencies
// (Blastomycosis, Histoplasmosis, Coccidioidomycosis, Cryptococcosis & Sinonasal Aspergillosis),
// Urine/Serum MiraVista Antigen EIA, Itraconazole/Fluconazole, Amphotericin B Lipid Complex
// Nephrotoxicity Guard & Clotrimazole Trephination triage operations in a thread-safe store.
type SystemicMycosesService struct {
	mu    sync.RWMutex
	ops   map[string]*domain.SystemicMycosesOperation
	order []string
	seq   int
}

// NewSystemicMycosesService initializes a SystemicMycosesService with 3 seeded operations (MYC-OP-101, MYC-OP-102, MYC-OP-103).
func NewSystemicMycosesService() *SystemicMycosesService {
	s := &SystemicMycosesService{
		ops:   make(map[string]*domain.SystemicMycosesOperation),
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

// BuildClinicalNarratives constructs the fungal pathophysiology, cytology/MiraVista Antigen EIA diagnostic note,
// and renal/hepatic/cribriform safety telemetry note.
func BuildClinicalNarratives(
	syndrome domain.FungalSyndrome,
	miraVistaAntigenNgML, serumCreatinineMgDL, altActivityUL, spo2Percent float64,
	cribriformPlateIntactOnCT, compoundedBulkItraconazoleBlocked bool,
) (patho, diagNote, safetyNote string) {
	switch syndrome {
	case domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast:
		patho = "Systemic Blastomycosis (Blastomyces dermatitidis / gilchristii) — Inhalation of soil conidia transforming at 37°C into thick-walled, refractile, broad-based budding yeasts (8–15 µm) causing pyogranulomatous nodular interstitial pneumonia ('snowstorm' thorax), granulomatous chorioretinitis/panophthalmitis, osteomyelitis, and cutaneous draining tracts."
	case domain.FungalSyndromeHistoplasmosisIntracellular:
		patho = "Disseminated Histoplasmosis (Histoplasma capsulatum) — Nitrogen/bat-or-bird-guano soil microconidia phagocytosed by mononuclear phagocytes, replicating as 2–4 µm intracellular oval yeasts with a clear basophilic halo inside macrophages across lung, liver, spleen, bone marrow, and large-bowel lamina propria."
	case domain.FungalSyndromeCoccidioidomycosisSpheruleValley:
		patho = "Disseminated Coccidioidomycosis ('Valley Fever' — Coccidioides immitis / posadasii) — Inhaled desert arthroconidia enlarging into 20–200 µm multinucleated endospore-packed spherules causing hilar tracheobronchial lymphadenopathy, lytic/proliferative appendicular osteomyelitis, pericarditis, and CNS granulomas."
	case domain.FungalSyndromeCryptococcosisEncapsulatedCNSOcular:
		patho = "Systemic & Neuro-Ocular Cryptococcosis (Cryptococcus neoformans / gattii) — Narrow-based budding yeast (4–8 µm) surrounded by a thick heteropolysaccharide glucuronoxylomannan (GXM) capsule ('soap-bubble' appearance on India ink/Diff-Quik) invading the nasal cavity ('Roman nose'), cribriform plate, meninges, and optic nerve/retina."
	case domain.FungalSyndromeSinonasalAspergillosisCribriform:
		patho = "Canine Sinonasal Aspergillosis (Aspergillus fumigatus) — Destructive necrotizing fungal plaques eroding nasal turbinates and invading the frontal sinus; requires skull CT prior to topical 1% Clotrimazole trephination to rule out cribriform plate osteolysis."
	default:
		patho = "Acute Systemic Dimorphic Mycosis & Opportunistic Fungal Emergency Triage."
	}

	cribriformStr := "INTACT on Skull CT (Safe for Sinonasal Topical Infusion if Indicated)"
	if !cribriformPlateIntactOnCT {
		cribriformStr = "BREACHED / OSTEOLYTIC ON SKULL CT (ABSOLUTE CONTRAINDICATION to Topical Clotrimazole/Enilconazole Sinus Trephination — Fatal Meningoencephalitis Risk)"
	}

	diagNote = fmt.Sprintf("Fungal Cytology, MiraVista Urine/Serum Antigen EIA (%.2f ng/mL) & Skull CT Cribriform Status: %s — ", miraVistaAntigenNgML, cribriformStr)
	if miraVistaAntigenNgML >= 10.0 {
		diagNote += "HIGH FUNGAL ANTIGEN BURDEN (>= 10.0 ng/mL): Massive systemic galactomannan/GXM antigenemia/antigenuria; monitor SpO2 closely for acute yeast die-off pulmonary ARDS cytokine release during initial 48–72h of antifungal therapy."
	} else if miraVistaAntigenNgML > 0.0 {
		diagNote += "Detectable fungal antigenemia/antigenuria (> 0.0 ng/mL); continue systemic azole therapy and recheck MiraVista Antigen EIA every 60–90 days until two consecutive negative (0.0 ng/mL) tests are achieved."
	} else {
		diagNote += "MiraVista Fungal Antigen EIA Negative (0.00 ng/mL); verify two consecutive negative antigen tests 1 month apart and completion of >= 6 months total azole therapy."
	}

	compoundedStr := "BLOCKED (Only FDA-Approved Innovator Capsules w/ Food or Oral Solution 5 mg/kg/day Permitted)"
	if !compoundedBulkItraconazoleBlocked {
		compoundedStr = "UNBLOCKED WARNING — Block Compounded Bulk Powder Immediately (< 10% Bioavailability)"
	}

	safetyNote = fmt.Sprintf("Renal, Hepatic & Pulmonary Die-Off Telemetry: Serum Creatinine %.2f mg/dL | ALT %.0f U/L | SpO2 %.1f%% | Compounded Bulk Itraconazole: %s — ", serumCreatinineMgDL, altActivityUL, spo2Percent, compoundedStr)
	if serumCreatinineMgDL > 2.5 {
		safetyNote += "NEPHROTOXICITY HOLD ALERT (Creatinine > 2.5 mg/dL): HOLD Amphotericin B Lipid Complex immediately, expand 0.9% NaCl IV diuresis, and rely on non-nephrotoxic azole therapy until azotemia resolves."
	} else if spo2Percent < 92.0 {
		safetyNote += "HYPOXEMIC YEAST DIE-OFF ARDS ALERT (SpO2 < 92%): Administer oxygen supplementation + short-course anti-inflammatory Dexamethasone-SP (0.1–0.2 mg/kg IV q24h for 2–3 days) to blunt macrophage TNF-alpha/IL-6 surge from dying fungal organisms."
	} else {
		safetyNote += "Renal function (Creatinine <= 2.5 mg/dL) and oxygenation (SpO2 >= 92%) stable for systemic antifungal administration."
	}

	return patho, diagNote, safetyNote
}

// BuildDefaultTherapyStaging constructs an evidence-based AntifungalTherapyStaging plan.
func BuildDefaultTherapyStaging(
	weightKg float64,
	syndrome domain.FungalSyndrome,
	protocol domain.AntifungalProtocol,
	serumCreatinineMgDL, spo2Percent float64,
	cribriformPlateIntactOnCT, compoundedBulkItraconazoleBlocked, amphotericinLipidSalineDiuresisVerified bool,
	stagedBy string,
	now time.Time,
) domain.AntifungalTherapyStaging {
	itraconazoleMgKgDay := 5.0
	itraconazoleDailyMg := round1(weightKg * itraconazoleMgKgDay)
	fluconazoleMgKgDay := 20.0 // 10 mg/kg PO q12h = 20 mg/kg/day
	fluconazoleSingleDoseMg := round1(weightKg * 10.0)
	amphoDoseMgKg := 2.0 // 1–3 mg/kg IV 3x/wk for lipid complex
	amphoSingleDoseMg := round1(weightKg * amphoDoseMgKg)
	dexDieOffMg := round2(weightKg * 0.1)

	dieOffStaged := spo2Percent < 92.0 || syndrome == domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast

	compoundedNote := "COMPOUNDED BULK ITRACONAZOLE BIOAVAILABILITY GUARD: Compounded bulk-powder Itraconazole capsules/suspensions are STRICTLY PROHIBITED due to < 10% oral absorption and high rates of fatal relapse in Blastomycosis/Histoplasmosis. Administer ONLY FDA-approved innovator Itraconazole capsules (with a high-fat meal) or pelleted/oral solution at 5 mg/kg/day."

	cribriformNote := "CRIBRIFORM PLATE CT INTEGRITY VERIFIED: Skull CT confirms intact cribriform plate without osteolysis; topical 1% Clotrimazole or Enilconazole frontal sinus trephination/endoscopic debridement is permitted for Sinonasal Aspergillosis."
	if !cribriformPlateIntactOnCT {
		cribriformNote = "ABSOLUTE CLOTRIMAZOLE TREPHINATION CONTRAINDICATION: Skull CT demonstrates cribriform plate osteolysis/breach — topical 1% Clotrimazole or Enilconazole sinus infusion is STRICTLY CONTRAINDICATED because vehicle/drug translocation across the breached cribriform plate into the olfactory bulbs and forebrain causes fatal necrotizing meningoencephalitis. Use systemic oral azole/terbinafine therapy only."
	}

	amphoNote := fmt.Sprintf("AMPHOTERICIN B LIPID COMPLEX NEPHROTOXICITY GUARD: Administer Amphotericin B Lipid Complex (ABLC / Ambisome) %.1f mg/kg (%.1f mg IV over 2h in 5%% Dextrose, 3x/week up to 24–36 mg/kg cumulative dose) ONLY with mandatory 0.9%% NaCl IV pre- and post-infusion diuresis (20 mL/kg over 1h pre/post) to prevent renal tubular vasoconstriction; HOLD dose immediately if Serum Creatinine exceeds 2.5 mg/dL (current %.2f mg/dL).", amphoDoseMgKg, amphoSingleDoseMg, serumCreatinineMgDL)

	dieOffPlan := fmt.Sprintf("Pulmonary Fungal Die-Off ARDS Mitigation: Administer Dexamethasone-SP 0.1 mg/kg IV q24h (%.2f mg IV for %.1f kg patient) for 2–3 days + humidified oxygen cage (FiO2 40–60%%) to attenuate acute pyogranulomatous alveolar capillary leak triggered by synchronous Blastomyces/Histoplasma yeast lysis.", dexDieOffMg, weightKg)
	if spo2Percent >= 92.0 && syndrome != domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast {
		dieOffPlan = fmt.Sprintf("Pulmonary Oxygenation Stable (SpO2 %.1f%% >= 92%%): Reserve short-course Dexamethasone-SP (0.1 mg/kg IV) only if acute tachypnea or hypoxemia develops during initial 72h fungal die-off.", spo2Percent)
	}

	azolePlan := fmt.Sprintf("Systemic Azole & Polyene Protocol: Itraconazole 5 mg/kg/day PO (%.1f mg/day innovator capsule w/ food or oral solution) OR Fluconazole 10 mg/kg PO q12h (%.1f mg PO q12h = 20 mg/kg/day for superior blood-brain, blood-ocular, and urinary tract penetration in Cryptococcosis/Coccidioidomycosis) for a minimum of 6 months and at least 1 month beyond clinical, radiographic, and MiraVista antigen resolution.", itraconazoleDailyMg, fluconazoleSingleDoseMg)
	if protocol == domain.AntifungalProtocolClotrimazole1PctFrontalSinusInfusion && cribriformPlateIntactOnCT {
		azolePlan = "Sinonasal Aspergillosis Trephination Protocol: Meticulous endoscopic/rhinoscopic fungal plaque debridement followed by 1-hour intranasal/frontal sinus infusion of 1% Clotrimazole in polyethylene glycol under general anesthesia with cuffed endotracheal tube and pharyngeal Foley packing."
	}

	ophthalmicPlan := "Neuro-Ophthalmic Chorioretinitis & CNS Surveillance: Perform serial indirect ophthalmoscopy + tonometry for granulomatous chorioretinitis, retinal detachment, and secondary glaucoma; use topical Prednisolone Acetate 1% + Dorzolamide 2% for anterior uveitis/glaucoma and prioritize Fluconazole (10 mg/kg PO q12h) or Voriconazole when CNS or ocular barrier penetration is required."

	if strings.TrimSpace(stagedBy) == "" {
		stagedBy = "Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)"
	}

	return domain.AntifungalTherapyStaging{
		Protocol:                                  protocol,
		ItraconazoleMgKgDay:                       itraconazoleMgKgDay,
		FluconazoleMgKgDay:                        fluconazoleMgKgDay,
		AmphotericinBLipidMgKgDose:                amphoDoseMgKg,
		CompoundedBulkItraconazoleBlocked:         compoundedBulkItraconazoleBlocked,
		CompoundedItraconazoleGuardNote:           compoundedNote,
		CribriformPlateIntactOnCT:                 cribriformPlateIntactOnCT,
		CribriformPlateTrephinationGuardNote:      cribriformNote,
		AmphotericinLipidSalineDiuresisVerified:   amphotericinLipidSalineDiuresisVerified,
		AmphotericinNephrotoxicityGuardNote:       amphoNote,
		DieOffDexamethasoneAntiInflammatoryStaged: dieOffStaged,
		PulmonaryDieOffARDSPlan:                   dieOffPlan,
		SystemicAzoleAndPolyeneRegimenPlan:        azolePlan,
		OphthalmicChorioretinitisNeuroPlan:        ophthalmicPlan,
		MinimumAzoleDurationMonths:                6,
		StagedBy:                                  stagedBy,
		StagedAt:                                  now,
	}
}

func (s *SystemicMycosesService) seed() {
	now := time.Now().UTC()

	// MYC-OP-101: Northwoods — Canine, Golden Retriever, 31.0 kg, AZOLE_AMPHOTERICIN_TREPHINATION_STAGED
	patho101, diag101, safety101 := BuildClinicalNarratives(
		domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast,
		14.8, 1.3, 68.0, 89.5,
		true, true,
	)
	stage101 := BuildDefaultTherapyStaging(
		31.0,
		domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast,
		domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution,
		1.3, 89.5,
		true, true, true,
		"Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
		now.Add(-45*time.Minute),
	)
	op101 := &domain.SystemicMycosesOperation{
		ID:                                      "MYC-OP-101",
		CaseID:                                  "CASE-MYC-101",
		PetName:                                 "Northwoods",
		Species:                                 "Canine",
		Breed:                                   "Golden Retriever",
		WeightKg:                                31.0,
		Status:                                  domain.MycosesStatusAzoleAmphotericinTrephinationStaged,
		Syndrome:                                domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast,
		Protocol:                                domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution,
		MiraVistaAntigenNgML:                    14.8,
		SerumCreatinineMgDL:                     1.3,
		ALTActivityUL:                           68.0,
		SpO2Percent:                             89.5,
		CribriformPlateIntactOnCT:               true,
		CompoundedBulkItraconazoleBlocked:       true,
		AmphotericinLipidSalineDiuresisVerified: true,
		DieOffDexamethasoneAntiInflammatoryStaged: true,
		AmphotericinNephrotoxicityHoldAlert:       false,
		PathophysiologySummary:                    patho101,
		CytologyAntigenDiagnosticNote:             diag101,
		RenalHepaticSafetyNote:                    safety101,
		TherapyStaging:                            stage101,
		AntigenRenalChecks: []domain.MycosesAntigenRenalCheck{
			{
				ID:                                "MYC-CHK-101-1",
				MiraVistaAntigenNgML:              14.8,
				SerumCreatinineMgDL:               1.3,
				ALTActivityUL:                     68.0,
				SpO2Percent:                       89.5,
				CribriformPlateIntactOnCT:         true,
				AmphotericinDoseHeldForAzotemia:   false,
				OphthalmicChorioretinitisResolved: false,
				RecordedBy:                        "Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
				ClinicalNote:                      "Systemic Blastomycosis admission: refractory cough, 'snowstorm' nodular interstitial lung pattern, SpO2 89.5%, fine-needle aspirate showing thick-walled refractile broad-based budding yeasts, MiraVista Blastomyces urine antigen > 14.7 ng/mL (14.8 ng/mL), bilateral granulomatous chorioretinitis; compounded bulk Itraconazole blocked — staged for FDA-approved Itraconazole 5 mg/kg/day + Amphotericin B Lipid Complex with 0.9% NaCl diuresis + Dexamethasone-SP 0.1 mg/kg IV die-off prophylaxis.",
				RecordedAt:                        now.Add(-45 * time.Minute),
			},
		},
		CreatedAt: now.Add(-60 * time.Minute),
		UpdatedAt: now.Add(-45 * time.Minute),
	}
	s.ops[op101.ID] = op101
	s.order = append(s.order, op101.ID)

	// MYC-OP-102: Sonora — Canine, German Shepherd Dog, 34.0 kg, ANTIGEN_EIA_CREA_OPHTHALMIC_MONITORED
	patho102, diag102, safety102 := BuildClinicalNarratives(
		domain.FungalSyndromeSinonasalAspergillosisCribriform,
		6.4, 2.1, 112.0, 95.0,
		false, true,
	)
	stage102 := BuildDefaultTherapyStaging(
		34.0,
		domain.FungalSyndromeSinonasalAspergillosisCribriform,
		domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution,
		2.1, 95.0,
		false, true, true,
		"Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
		now.Add(-90*time.Minute),
	)
	op102 := &domain.SystemicMycosesOperation{
		ID:                                      "MYC-OP-102",
		CaseID:                                  "CASE-MYC-102",
		PetName:                                 "Sonora",
		Species:                                 "Canine",
		Breed:                                   "German Shepherd Dog",
		WeightKg:                                34.0,
		Status:                                  domain.MycosesStatusAntigenEIACreaOphthalmicMonitored,
		Syndrome:                                domain.FungalSyndromeSinonasalAspergillosisCribriform,
		Protocol:                                domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution,
		MiraVistaAntigenNgML:                    6.4,
		SerumCreatinineMgDL:                     2.1,
		ALTActivityUL:                           112.0,
		SpO2Percent:                             95.0,
		CribriformPlateIntactOnCT:               false,
		CompoundedBulkItraconazoleBlocked:       true,
		AmphotericinLipidSalineDiuresisVerified: true,
		DieOffDexamethasoneAntiInflammatoryStaged: false,
		AmphotericinNephrotoxicityHoldAlert:       false,
		PathophysiologySummary:                    patho102,
		CytologyAntigenDiagnosticNote:             diag102,
		RenalHepaticSafetyNote:                    safety102,
		TherapyStaging:                            stage102,
		AntigenRenalChecks: []domain.MycosesAntigenRenalCheck{
			{
				ID:                                "MYC-CHK-102-1",
				MiraVistaAntigenNgML:              6.4,
				SerumCreatinineMgDL:               2.1,
				ALTActivityUL:                     112.0,
				SpO2Percent:                       95.0,
				CribriformPlateIntactOnCT:         false,
				AmphotericinDoseHeldForAzotemia:   false,
				OphthalmicChorioretinitisResolved: true,
				RecordedBy:                        "Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
				ClinicalNote:                      "Sinonasal Aspergillosis with focal cribriform plate osteolysis on skull CT: topical 1% Clotrimazole frontal sinus trephination strictly contraindicated due to meningoencephalitis risk; managed on systemic innovator Itraconazole 5 mg/kg/day + Terbinafine with serial renal (Creatinine 2.1 mg/dL) and hepatic (ALT 112 U/L) monitoring.",
				RecordedAt:                        now.Add(-80 * time.Minute),
			},
		},
		CreatedAt: now.Add(-4 * time.Hour),
		UpdatedAt: now.Add(-80 * time.Minute),
	}
	s.ops[op102.ID] = op102
	s.order = append(s.order, op102.ID)

	// MYC-OP-103: Sedona — Feline, Domestic Shorthair, 4.6 kg, TWO_NEGATIVE_ANTIGEN_EIA_CLEARED
	patho103, diag103, safety103 := BuildClinicalNarratives(
		domain.FungalSyndromeCryptococcosisEncapsulatedCNSOcular,
		0.0, 1.1, 54.0, 98.0,
		true, true,
	)
	stage103 := BuildDefaultTherapyStaging(
		4.6,
		domain.FungalSyndromeCryptococcosisEncapsulatedCNSOcular,
		domain.AntifungalProtocolFluconazole10MgKgBIDCNSOcular,
		1.1, 98.0,
		true, true, true,
		"Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
		now.Add(-12*time.Hour),
	)
	clearedAt := now.Add(-30 * time.Minute)
	clearance103 := &domain.SystemicMycosesClearance{
		MiraVistaAntigenNgML:              0.0,
		SerumCreatinineMgDL:               1.1,
		TwoConsecutiveNegativeEIAVerified: true,
		Minimum6MonthAzoleCourseVerified:  true,
		ClearedBy:                         "Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
		ClearanceSummary:                  "Feline Cryptococcosis ('Roman nose' & chorioretinitis) resolved after 7-month Fluconazole 10 mg/kg PO q12h regimen: two consecutive negative antigen titers (0.0 ng/mL) 1 month apart, Serum Creatinine 1.1 mg/dL (<= 1.8 mg/dL), ALT 54 U/L, and complete neuro-ophthalmic resolution verified; cleared.",
		ClearedAt:                         clearedAt,
	}
	op103 := &domain.SystemicMycosesOperation{
		ID:                                      "MYC-OP-103",
		CaseID:                                  "CASE-MYC-103",
		PetName:                                 "Sedona",
		Species:                                 "Feline",
		Breed:                                   "Domestic Shorthair",
		WeightKg:                                4.6,
		Status:                                  domain.MycosesStatusTwoNegativeAntigenEIACleared,
		Syndrome:                                domain.FungalSyndromeCryptococcosisEncapsulatedCNSOcular,
		Protocol:                                domain.AntifungalProtocolFluconazole10MgKgBIDCNSOcular,
		MiraVistaAntigenNgML:                    0.0,
		SerumCreatinineMgDL:                     1.1,
		ALTActivityUL:                           54.0,
		SpO2Percent:                             98.0,
		CribriformPlateIntactOnCT:               true,
		CompoundedBulkItraconazoleBlocked:       true,
		AmphotericinLipidSalineDiuresisVerified: true,
		DieOffDexamethasoneAntiInflammatoryStaged: false,
		AmphotericinNephrotoxicityHoldAlert:       false,
		PathophysiologySummary:                    patho103,
		CytologyAntigenDiagnosticNote:             diag103,
		RenalHepaticSafetyNote:                    safety103,
		TherapyStaging:                            stage103,
		AntigenRenalChecks: []domain.MycosesAntigenRenalCheck{
			{
				ID:                                "MYC-CHK-103-1",
				MiraVistaAntigenNgML:              0.0,
				SerumCreatinineMgDL:               1.1,
				ALTActivityUL:                     54.0,
				SpO2Percent:                       98.0,
				CribriformPlateIntactOnCT:         true,
				AmphotericinDoseHeldForAzotemia:   false,
				OphthalmicChorioretinitisResolved: true,
				RecordedBy:                        "Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
				ClinicalNote:                      "Month 7 Cryptococcus antigen EIA & latex agglutination check: 0.0 ng/mL (second consecutive negative test 30 days after Month 6 negative test), Creatinine 1.1 mg/dL, ALT 54 U/L, nasal bridge swelling and chorioretinal granulomas completely resolved.",
				RecordedAt:                        now.Add(-40 * time.Minute),
			},
		},
		Clearance:        clearance103,
		DischargedBy:     clearance103.ClearedBy,
		DischargeSummary: clearance103.ClearanceSummary,
		DischargedAt:     &clearedAt,
		CreatedAt:        now.Add(-24 * time.Hour),
		UpdatedAt:        clearedAt,
	}
	s.ops[op103.ID] = op103
	s.order = append(s.order, op103.ID)
}

// ListOperations returns a deep-cloned slice of Systemic Mycoses operations, optionally filtered by statusFilter.
func (s *SystemicMycosesService) ListOperations(statusFilter string) []domain.SystemicMycosesOperation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(statusFilter)
	out := make([]domain.SystemicMycosesOperation, 0, len(s.order))
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		if trimmed != "" && !strings.EqualFold(string(op.Status), trimmed) {
			continue
		}
		out = append(out, cloneMycosesOperation(op))
	}
	return out
}

// GetOperation returns a deep copy of a Systemic Mycoses operation by ID.
func (s *SystemicMycosesService) GetOperation(id string) (*domain.SystemicMycosesOperation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trimmed := strings.TrimSpace(id)
	op, ok := s.ops[trimmed]
	if !ok {
		return nil, fmt.Errorf("systemic mycoses operation %q not found", id)
	}
	cp := cloneMycosesOperation(op)
	return &cp, nil
}

// CalculateOverview computes summary telemetry metrics across all Systemic Mycoses operations.
func (s *SystemicMycosesService) CalculateOverview() domain.SystemicMycosesOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var m domain.SystemicMycosesOverview
	m.TotalOperations = len(s.order)
	for _, id := range s.order {
		op, ok := s.ops[id]
		if !ok {
			continue
		}
		switch op.Status {
		case domain.MycosesStatusAzoleAmphotericinTrephinationStaged:
			m.AzoleAmphotericinTrephinationStagedCount++
		case domain.MycosesStatusAntigenEIACreaOphthalmicMonitored:
			m.AntigenEIACreaOphthalmicMonitoredCount++
		case domain.MycosesStatusTwoNegativeAntigenEIACleared:
			m.TwoNegativeAntigenEIAClearedCount++
		}
		if op.Syndrome == domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast ||
			op.Syndrome == domain.FungalSyndromeHistoplasmosisIntracellular {
			m.BlastomycosisHistoplasmosisCount++
		}
		if !op.CribriformPlateIntactOnCT {
			m.CribriformBreachContraindicatedCount++
		}
		if op.SerumCreatinineMgDL > 2.5 || op.AmphotericinNephrotoxicityHoldAlert {
			m.AzotemiaNephrotoxicityAlertCount++
		}
	}
	return m
}

// GetOverviewMetrics is an alias for CalculateOverview for handler consistency.
func (s *SystemicMycosesService) GetOverviewMetrics() domain.SystemicMycosesOverview {
	return s.CalculateOverview()
}

// CreateOperation validates and creates a new Acute Systemic Mycoses & Fungal Emergencies triage operation.
func (s *SystemicMycosesService) CreateOperation(
	caseID, petName, species, breed string,
	weightKg float64,
	syndrome domain.FungalSyndrome,
	protocol domain.AntifungalProtocol,
	miraVistaAntigenNgML, serumCreatinineMgDL, altActivityUL, spo2Percent float64,
	cribriformPlateIntactOnCT, compoundedBulkItraconazoleBlocked, amphotericinLipidSalineDiuresisVerified bool,
) (*domain.SystemicMycosesOperation, error) {
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
		trimmedSpecies = "Canine"
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
	if protocol == "" {
		protocol = domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution
	}
	if !protocol.IsValid() {
		return nil, fmt.Errorf("invalid protocol: %q", protocol)
	}
	if miraVistaAntigenNgML < 0 || miraVistaAntigenNgML > 50.0 {
		return nil, fmt.Errorf("miravista_antigen_ng_ml must be in [0.0, 50.0], got %v", miraVistaAntigenNgML)
	}
	if serumCreatinineMgDL <= 0 || serumCreatinineMgDL > 15.0 {
		return nil, fmt.Errorf("serum_creatinine_mg_dl must be in (0, 15.0], got %v", serumCreatinineMgDL)
	}
	if altActivityUL <= 0 || altActivityUL > 5000.0 {
		return nil, fmt.Errorf("alt_activity_u_l must be in (0, 5000.0], got %v", altActivityUL)
	}
	if spo2Percent < 50.0 || spo2Percent > 100.0 {
		return nil, fmt.Errorf("spo2_percent must be in [50.0, 100.0], got %v", spo2Percent)
	}

	// Safety Guard 1: Topical 1% Clotrimazole frontal sinus infusion is strictly contraindicated if CT shows cribriform plate breach
	if protocol == domain.AntifungalProtocolClotrimazole1PctFrontalSinusInfusion && !cribriformPlateIntactOnCT {
		return nil, fmt.Errorf("topical 1%% clotrimazole frontal sinus infusion is strictly contraindicated when cribriform_plate_intact_on_ct is false (cribriform plate osteolysis/breach causes fatal meningoencephalitis)")
	}
	// Safety Guard 2: Compounded bulk-powder Itraconazole is strictly prohibited (< 10% oral bioavailability)
	if !compoundedBulkItraconazoleBlocked {
		return nil, fmt.Errorf("compounded_bulk_itraconazole_blocked must be true: compounded bulk-powder itraconazole has < 10%% oral bioavailability and causes fatal treatment failure; use only FDA-approved innovator capsules with food or oral solution 5 mg/kg/day")
	}
	// Safety Guard 3: Amphotericin B Lipid Complex requires mandatory 0.9% NaCl diuresis and Serum Creatinine <= 2.5 mg/dL
	if protocol == domain.AntifungalProtocolAmphotericinBLipidComplexSaline {
		if !amphotericinLipidSalineDiuresisVerified {
			return nil, fmt.Errorf("amphotericin_lipid_saline_diuresis_verified must be true for Amphotericin B Lipid Complex protocol (mandatory 0.9%% NaCl pre/post-infusion diuresis)")
		}
		if serumCreatinineMgDL > 2.5 {
			return nil, fmt.Errorf("amphotericin B lipid complex is contraindicated when serum_creatinine_mg_dl > 2.5 mg/dL (got %.2f mg/dL); hold dose to prevent acute renal tubular necrosis", serumCreatinineMgDL)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	opID := fmt.Sprintf("MYC-OP-%d", s.seq)
	now := time.Now().UTC()

	patho, diagNote, safetyNote := BuildClinicalNarratives(
		syndrome,
		miraVistaAntigenNgML,
		serumCreatinineMgDL,
		altActivityUL,
		spo2Percent,
		cribriformPlateIntactOnCT,
		compoundedBulkItraconazoleBlocked,
	)
	staging := BuildDefaultTherapyStaging(
		weightKg,
		syndrome,
		protocol,
		serumCreatinineMgDL,
		spo2Percent,
		cribriformPlateIntactOnCT,
		compoundedBulkItraconazoleBlocked,
		amphotericinLipidSalineDiuresisVerified,
		"Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
		now,
	)

	op := &domain.SystemicMycosesOperation{
		ID:                                      opID,
		CaseID:                                  trimmedCase,
		PetName:                                 trimmedPet,
		Species:                                 trimmedSpecies,
		Breed:                                   trimmedBreed,
		WeightKg:                                round1(weightKg),
		Status:                                  domain.MycosesStatusAzoleAmphotericinTrephinationStaged,
		Syndrome:                                syndrome,
		Protocol:                                protocol,
		MiraVistaAntigenNgML:                    round2(miraVistaAntigenNgML),
		SerumCreatinineMgDL:                     round2(serumCreatinineMgDL),
		ALTActivityUL:                           round1(altActivityUL),
		SpO2Percent:                             round1(spo2Percent),
		CribriformPlateIntactOnCT:               cribriformPlateIntactOnCT,
		CompoundedBulkItraconazoleBlocked:       compoundedBulkItraconazoleBlocked,
		AmphotericinLipidSalineDiuresisVerified: amphotericinLipidSalineDiuresisVerified,
		DieOffDexamethasoneAntiInflammatoryStaged: staging.DieOffDexamethasoneAntiInflammatoryStaged,
		AmphotericinNephrotoxicityHoldAlert:       serumCreatinineMgDL > 2.5,
		PathophysiologySummary:                    patho,
		CytologyAntigenDiagnosticNote:             diagNote,
		RenalHepaticSafetyNote:                    safetyNote,
		TherapyStaging:                            staging,
		AntigenRenalChecks:                        []domain.MycosesAntigenRenalCheck{},
		CreatedAt:                                 now,
		UpdatedAt:                                 now,
	}

	s.ops[op.ID] = op
	s.order = append(s.order, op.ID)

	cp := cloneMycosesOperation(op)
	return &cp, nil
}

// StageTherapy stages systemic Itraconazole (5 mg/kg/day innovator capsules or oral solution),
// Fluconazole (10 mg/kg PO q12h for CNS/Ocular penetration), Amphotericin B Lipid Complex
// (1–3 mg/kg IV 3x/wk with 0.9% NaCl pre/post-diuresis), or 1% Clotrimazole frontal sinus trephination,
// enforcing all four clinical safety guards.
func (s *SystemicMycosesService) StageTherapy(
	id string,
	protocol domain.AntifungalProtocol,
	itraconazoleMgKgDay, fluconazoleMgKgDay, amphotericinBLipidMgKgDose float64,
	compoundedBulkItraconazoleBlocked, cribriformPlateIntactOnCT, amphotericinLipidSalineDiuresisVerified, dieOffDexamethasoneStaged bool,
	azoleRegimenPlan, pulmonaryDieOffPlan, ophthalmicNeuroPlan string,
	minimumAzoleDurationMonths int,
	stagedBy string,
) (*domain.SystemicMycosesOperation, error) {
	if protocol == "" {
		protocol = domain.AntifungalProtocolItraconazole5MgKgBrandOrSolution
	}
	if !protocol.IsValid() {
		return nil, fmt.Errorf("invalid protocol: %q", protocol)
	}
	if !compoundedBulkItraconazoleBlocked {
		return nil, fmt.Errorf("compounded_bulk_itraconazole_blocked must be true: compounded bulk-powder itraconazole has < 10%% oral bioavailability and causes fatal treatment failure; use only FDA-approved innovator capsules with food or oral solution 5 mg/kg/day")
	}
	if protocol == domain.AntifungalProtocolClotrimazole1PctFrontalSinusInfusion && !cribriformPlateIntactOnCT {
		return nil, fmt.Errorf("topical 1%% clotrimazole frontal sinus infusion is strictly contraindicated when cribriform_plate_intact_on_ct is false (cribriform plate breach causes fatal meningoencephalitis)")
	}
	if protocol == domain.AntifungalProtocolAmphotericinBLipidComplexSaline && !amphotericinLipidSalineDiuresisVerified {
		return nil, fmt.Errorf("amphotericin_lipid_saline_diuresis_verified must be true for Amphotericin B Lipid Complex protocol (mandatory 0.9%% NaCl pre/post-infusion diuresis)")
	}
	if itraconazoleMgKgDay == 0 {
		itraconazoleMgKgDay = 5.0
	}
	if itraconazoleMgKgDay < 2.5 || itraconazoleMgKgDay > 15.0 {
		return nil, fmt.Errorf("itraconazole_mg_kg_day must be in [2.5, 15.0] (standard 5 mg/kg/day), got %v", itraconazoleMgKgDay)
	}
	if fluconazoleMgKgDay == 0 {
		fluconazoleMgKgDay = 20.0
	}
	if fluconazoleMgKgDay < 5.0 || fluconazoleMgKgDay > 40.0 {
		return nil, fmt.Errorf("fluconazole_mg_kg_day must be in [5.0, 40.0] (standard 10 mg/kg PO q12h = 20 mg/kg/day), got %v", fluconazoleMgKgDay)
	}
	if amphotericinBLipidMgKgDose == 0 {
		amphotericinBLipidMgKgDose = 2.0
	}
	if amphotericinBLipidMgKgDose < 0.5 || amphotericinBLipidMgKgDose > 5.0 {
		return nil, fmt.Errorf("amphotericin_b_lipid_mg_kg_dose must be in [0.5, 5.0] mg/kg (standard 1.0–3.0 mg/kg IV 3x/wk for lipid complex), got %v", amphotericinBLipidMgKgDose)
	}
	if minimumAzoleDurationMonths == 0 {
		minimumAzoleDurationMonths = 6
	}
	if minimumAzoleDurationMonths < 6 || minimumAzoleDurationMonths > 24 {
		return nil, fmt.Errorf("minimum_azole_duration_months must be in [6, 24] months to prevent fungal relapse, got %d", minimumAzoleDurationMonths)
	}
	trimmedAzolePlan := strings.TrimSpace(azoleRegimenPlan)
	if trimmedAzolePlan == "" {
		return nil, fmt.Errorf("systemic_azole_and_polyene_regimen_plan is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	op, ok := s.ops[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("systemic mycoses operation %q not found", id)
	}

	// Check operation's CT cribriform integrity and renal status
	if protocol == domain.AntifungalProtocolClotrimazole1PctFrontalSinusInfusion && !op.CribriformPlateIntactOnCT && !cribriformPlateIntactOnCT {
		return nil, fmt.Errorf("topical 1%% clotrimazole frontal sinus infusion is strictly contraindicated on %s because CT shows cribriform plate osteolysis/breach", op.ID)
	}
	if protocol == domain.AntifungalProtocolAmphotericinBLipidComplexSaline && op.SerumCreatinineMgDL > 2.5 {
		return nil, fmt.Errorf("cannot stage Amphotericin B Lipid Complex on %s while serum_creatinine_mg_dl is %.2f mg/dL (> 2.5 mg/dL nephrotoxicity threshold); hold polyene and use azole therapy", op.ID, op.SerumCreatinineMgDL)
	}
	// Pulmonary Blastomycosis Die-Off ARDS Guard: if SpO2 < 92%, ensure anti-inflammatory Dexamethasone-SP is staged
	if op.SpO2Percent < 92.0 && op.Syndrome == domain.FungalSyndromeBlastomycosisBroadBasedBuddingYeast {
		dieOffDexamethasoneStaged = true
	}

	now := time.Now().UTC()
	defaults := BuildDefaultTherapyStaging(
		op.WeightKg,
		op.Syndrome,
		protocol,
		op.SerumCreatinineMgDL,
		op.SpO2Percent,
		cribriformPlateIntactOnCT,
		compoundedBulkItraconazoleBlocked,
		amphotericinLipidSalineDiuresisVerified,
		stagedBy,
		now,
	)

	trimmedDieOff := strings.TrimSpace(pulmonaryDieOffPlan)
	if trimmedDieOff == "" {
		trimmedDieOff = defaults.PulmonaryDieOffARDSPlan
	}
	trimmedOphthalmic := strings.TrimSpace(ophthalmicNeuroPlan)
	if trimmedOphthalmic == "" {
		trimmedOphthalmic = defaults.OphthalmicChorioretinitisNeuroPlan
	}
	trimmedBy := strings.TrimSpace(stagedBy)
	if trimmedBy == "" {
		trimmedBy = defaults.StagedBy
	}

	op.Protocol = protocol
	op.CribriformPlateIntactOnCT = cribriformPlateIntactOnCT
	op.CompoundedBulkItraconazoleBlocked = compoundedBulkItraconazoleBlocked
	op.AmphotericinLipidSalineDiuresisVerified = amphotericinLipidSalineDiuresisVerified
	op.DieOffDexamethasoneAntiInflammatoryStaged = dieOffDexamethasoneStaged
	op.TherapyStaging = domain.AntifungalTherapyStaging{
		Protocol:                                  protocol,
		ItraconazoleMgKgDay:                       round1(itraconazoleMgKgDay),
		FluconazoleMgKgDay:                        round1(fluconazoleMgKgDay),
		AmphotericinBLipidMgKgDose:                round1(amphotericinBLipidMgKgDose),
		CompoundedBulkItraconazoleBlocked:         compoundedBulkItraconazoleBlocked,
		CompoundedItraconazoleGuardNote:           defaults.CompoundedItraconazoleGuardNote,
		CribriformPlateIntactOnCT:                 cribriformPlateIntactOnCT,
		CribriformPlateTrephinationGuardNote:      defaults.CribriformPlateTrephinationGuardNote,
		AmphotericinLipidSalineDiuresisVerified:   amphotericinLipidSalineDiuresisVerified,
		AmphotericinNephrotoxicityGuardNote:       defaults.AmphotericinNephrotoxicityGuardNote,
		DieOffDexamethasoneAntiInflammatoryStaged: dieOffDexamethasoneStaged,
		PulmonaryDieOffARDSPlan:                   trimmedDieOff,
		SystemicAzoleAndPolyeneRegimenPlan:        trimmedAzolePlan,
		OphthalmicChorioretinitisNeuroPlan:        trimmedOphthalmic,
		MinimumAzoleDurationMonths:                minimumAzoleDurationMonths,
		StagedBy:                                  trimmedBy,
		StagedAt:                                  now,
	}
	op.Status = domain.MycosesStatusAntigenEIACreaOphthalmicMonitored
	op.Clearance = nil
	op.DischargedBy = ""
	op.DischargeSummary = ""
	op.DischargedAt = nil
	op.UpdatedAt = now

	cp := cloneMycosesOperation(op)
	return &cp, nil
}

// RecordAntigenRenalCheck appends a serial MycosesAntigenRenalCheck and updates MiraVista Antigen EIA,
// Serum Creatinine, ALT, SpO2, and cribriform integrity telemetry on the operation.
func (s *SystemicMycosesService) RecordAntigenRenalCheck(
	id string,
	miraVistaAntigenNgML, serumCreatinineMgDL, altActivityUL, spo2Percent float64,
	cribriformPlateIntactOnCT, amphotericinDoseHeldForAzotemia, ophthalmicChorioretinitisResolved bool,
	recordedBy, clinicalNote string,
) (*domain.SystemicMycosesOperation, error) {
	if miraVistaAntigenNgML < 0 || miraVistaAntigenNgML > 50.0 {
		return nil, fmt.Errorf("miravista_antigen_ng_ml must be in [0.0, 50.0], got %v", miraVistaAntigenNgML)
	}
	if serumCreatinineMgDL <= 0 || serumCreatinineMgDL > 15.0 {
		return nil, fmt.Errorf("serum_creatinine_mg_dl must be in (0, 15.0], got %v", serumCreatinineMgDL)
	}
	if altActivityUL <= 0 || altActivityUL > 5000.0 {
		return nil, fmt.Errorf("alt_activity_u_l must be in (0, 5000.0], got %v", altActivityUL)
	}
	if spo2Percent < 50.0 || spo2Percent > 100.0 {
		return nil, fmt.Errorf("spo2_percent must be in [50.0, 100.0], got %v", spo2Percent)
	}
	// Safety Guard: If Serum Creatinine > 2.5 mg/dL, automatically enforce Amphotericin B dose hold
	if serumCreatinineMgDL > 2.5 {
		amphotericinDoseHeldForAzotemia = true
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
		return nil, fmt.Errorf("systemic mycoses operation %q not found", id)
	}

	now := time.Now().UTC()
	chk := domain.MycosesAntigenRenalCheck{
		ID:                                fmt.Sprintf("MYC-CHK-%s-%d", op.ID, len(op.AntigenRenalChecks)+1),
		MiraVistaAntigenNgML:              round2(miraVistaAntigenNgML),
		SerumCreatinineMgDL:               round2(serumCreatinineMgDL),
		ALTActivityUL:                     round1(altActivityUL),
		SpO2Percent:                       round1(spo2Percent),
		CribriformPlateIntactOnCT:         cribriformPlateIntactOnCT,
		AmphotericinDoseHeldForAzotemia:   amphotericinDoseHeldForAzotemia,
		OphthalmicChorioretinitisResolved: ophthalmicChorioretinitisResolved,
		RecordedBy:                        trimmedBy,
		ClinicalNote:                      trimmedNote,
		RecordedAt:                        now,
	}

	op.AntigenRenalChecks = append(op.AntigenRenalChecks, chk)
	op.MiraVistaAntigenNgML = chk.MiraVistaAntigenNgML
	op.SerumCreatinineMgDL = chk.SerumCreatinineMgDL
	op.ALTActivityUL = chk.ALTActivityUL
	op.SpO2Percent = chk.SpO2Percent
	op.CribriformPlateIntactOnCT = chk.CribriformPlateIntactOnCT
	op.AmphotericinNephrotoxicityHoldAlert = chk.SerumCreatinineMgDL > 2.5 || chk.AmphotericinDoseHeldForAzotemia

	patho, diagNote, safetyNote := BuildClinicalNarratives(
		op.Syndrome,
		op.MiraVistaAntigenNgML,
		op.SerumCreatinineMgDL,
		op.ALTActivityUL,
		op.SpO2Percent,
		op.CribriformPlateIntactOnCT,
		op.CompoundedBulkItraconazoleBlocked,
	)
	op.PathophysiologySummary = patho
	op.CytologyAntigenDiagnosticNote = diagNote
	op.RenalHepaticSafetyNote = safetyNote
	if op.Status == domain.MycosesStatusAzoleAmphotericinTrephinationStaged {
		op.Status = domain.MycosesStatusAntigenEIACreaOphthalmicMonitored
	}
	op.UpdatedAt = now

	cp := cloneMycosesOperation(op)
	return &cp, nil
}

// EvaluateClearance verifies that MiraVista Urine/Serum Antigen EIA is 0.0 ng/mL,
// Serum Creatinine is <= 1.8 mg/dL, two consecutive negative antigen EIA tests 1 month apart are verified,
// and a minimum 6-month azole course (continuing >= 1 month past resolution) is verified,
// transitioning status to TWO_NEGATIVE_ANTIGEN_EIA_CLEARED.
func (s *SystemicMycosesService) EvaluateClearance(
	id string,
	miraVistaAntigenNgML, serumCreatinineMgDL float64,
	twoConsecutiveNegativeEIAVerified, minimum6MonthAzoleCourseVerified bool,
	clearedBy, clearanceSummary string,
) (*domain.SystemicMycosesOperation, error) {
	if miraVistaAntigenNgML != 0.0 {
		return nil, fmt.Errorf("cannot clear systemic mycoses operation: miravista_antigen_ng_ml must be 0.0 ng/mL (got %.2f ng/mL; persistent fungal antigenemia/antigenuria)", miraVistaAntigenNgML)
	}
	if serumCreatinineMgDL <= 0 || serumCreatinineMgDL > 1.8 {
		return nil, fmt.Errorf("cannot clear systemic mycoses operation: serum_creatinine_mg_dl must be in (0, 1.8] mg/dL, got %.2f mg/dL", serumCreatinineMgDL)
	}
	if !twoConsecutiveNegativeEIAVerified {
		return nil, fmt.Errorf("cannot clear systemic mycoses operation: two_consecutive_negative_eia_verified must be true (two negative MiraVista antigen EIA tests 1 month apart required)")
	}
	if !minimum6MonthAzoleCourseVerified {
		return nil, fmt.Errorf("cannot clear systemic mycoses operation: minimum_6month_azole_course_verified must be true (minimum 6-month azole course and >= 1 month past clinical/radiographic resolution required)")
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
		return nil, fmt.Errorf("systemic mycoses operation %q not found", id)
	}

	now := time.Now().UTC()
	clearance := &domain.SystemicMycosesClearance{
		MiraVistaAntigenNgML:              0.0,
		SerumCreatinineMgDL:               round2(serumCreatinineMgDL),
		TwoConsecutiveNegativeEIAVerified: true,
		Minimum6MonthAzoleCourseVerified:  true,
		ClearedBy:                         trimmedBy,
		ClearanceSummary:                  trimmedSummary,
		ClearedAt:                         now,
	}

	op.MiraVistaAntigenNgML = 0.0
	op.SerumCreatinineMgDL = clearance.SerumCreatinineMgDL
	op.AmphotericinNephrotoxicityHoldAlert = false
	op.Status = domain.MycosesStatusTwoNegativeAntigenEIACleared
	op.Clearance = clearance
	op.DischargedBy = trimmedBy
	op.DischargeSummary = trimmedSummary
	op.DischargedAt = &now
	op.UpdatedAt = now

	patho, diagNote, safetyNote := BuildClinicalNarratives(
		op.Syndrome,
		op.MiraVistaAntigenNgML,
		op.SerumCreatinineMgDL,
		op.ALTActivityUL,
		op.SpO2Percent,
		op.CribriformPlateIntactOnCT,
		op.CompoundedBulkItraconazoleBlocked,
	)
	op.PathophysiologySummary = patho
	op.CytologyAntigenDiagnosticNote = diagNote
	op.RenalHepaticSafetyNote = safetyNote

	cp := cloneMycosesOperation(op)
	return &cp, nil
}

func cloneMycosesOperation(op *domain.SystemicMycosesOperation) domain.SystemicMycosesOperation {
	cp := *op
	if op.AntigenRenalChecks != nil {
		cp.AntigenRenalChecks = make([]domain.MycosesAntigenRenalCheck, len(op.AntigenRenalChecks))
		copy(cp.AntigenRenalChecks, op.AntigenRenalChecks)
	}
	if op.Clearance != nil {
		cl := *op.Clearance
		cp.Clearance = &cl
	}
	if op.DischargedAt != nil {
		dt := *op.DischargedAt
		cp.DischargedAt = &dt
	}
	return cp
}
