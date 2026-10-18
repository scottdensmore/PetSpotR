package microchip

import (
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

type FederatedScanInput struct {
	CaseID          string
	PetID           string
	PetName         string
	Species         string
	RawChipInput    string
	ScannerLocation domain.GeoPoint
	ScannedByAgency string
	SimulateMode    string // "" | "REGISTERED_PRIMARY" | "UNREGISTERED_DEAD_END" | "OUTDATED_PRIMARY_PHONE"
}

type LotTraceInput struct {
	ImplantingClinicName    string
	ImplantingClinicPhone   string
	ImplantPatientChartID   string
	RecoveredOwnerName      string
	RecoveredOwnerPhoneMask string
}

type OwnershipResolutionInput struct {
	VerifiedOwnerName      string
	VerifiedOwnerPhoneMask string
	VerificationMethod     string // e.g. "PRIMARY_REGISTRY_MATCH", "CLINIC_PIMS_LOT_TRACE", "SECONDARY_EMERGENCY_CONTACT"
	Notes                  string
}

func DecodeTransponderManufacturer(rawChip string) (normalized string, masked string, std domain.MicrochipStandardType, freqKhz float64, mfgCode string, mfgName string, lotID string, err error) {
	res := ValidateAndNormalize(rawChip)
	if !res.Valid {
		return "", "", "", 0, "", "", "", fmt.Errorf("%s", res.ErrorMessage)
	}
	normalized = res.NormalizedID

	switch res.Standard {
	case StandardISO15:
		std = domain.ChipStandardISO11784_15
		freqKhz = 134.2
		prefix := normalized[:3]
		switch prefix {
		case "985":
			mfgCode = "985"
			mfgName = "HomeAgain / Merck Animal Health"
			lotID = "LOT-MERCK-" + normalized[3:7]
		case "981":
			mfgCode = "981"
			mfgName = "AKC Reunite / Datamars"
			lotID = "LOT-DATAMARS-" + normalized[3:7]
		case "982":
			mfgCode = "982"
			mfgName = "24PetWatch / Allflex"
			lotID = "LOT-ALLFLEX-" + normalized[3:7]
		case "941":
			mfgCode = "941"
			mfgName = "Michelson Found Animals / Felixcan"
			lotID = "LOT-FELIX-" + normalized[3:7]
		case "900":
			mfgCode = "900"
			mfgName = "Unallocated ISO Shared Lot (Wholesale)"
			lotID = "LOT-ISO900-" + normalized[3:7]
		default:
			mfgCode = prefix
			mfgName = "ISO 11784/11785 Global Manufacturer"
			lotID = "LOT-ISO-" + normalized[3:7]
		}
	case StandardAvid9:
		std = domain.ChipStandardAVID9
		freqKhz = 125.0
		mfgCode = "AVID"
		mfgName = "AVID Identification Systems (PETtrac)"
		lotID = "LOT-AVID-" + normalized[:4]
	case StandardEuro10:
		std = domain.ChipStandardHex10Legacy
		freqKhz = 128.0
		mfgCode = "FDX-A"
		mfgName = "Legacy FDX-A / Trovan-Banfield Hex"
		lotID = "LOT-FDXA-" + normalized[:4]
	default:
		return "", "", "", 0, "", "", "", fmt.Errorf("unsupported microchip standard %q", res.Standard)
	}

	masked = fmt.Sprintf("%s ••••%s", mfgCode, normalized[len(normalized)-4:])
	return normalized, masked, std, freqKhz, mfgCode, mfgName, lotID, nil
}

func EvaluateFederatedScan(input FederatedScanInput, now time.Time) (*domain.MicrochipScanCase, error) {
	normalized, masked, std, freqKhz, mfgCode, mfgName, lotID, err := DecodeTransponderManufacturer(input.RawChipInput)
	if err != nil {
		return nil, err
	}

	species := strings.ToUpper(strings.TrimSpace(input.Species))
	if species == "" {
		species = "DOG"
	}
	agency := strings.TrimSpace(input.ScannedByAgency)
	if agency == "" {
		agency = "Universal RFID Field Scanner"
	}
	simMode := strings.ToUpper(strings.TrimSpace(input.SimulateMode))

	scanCase := &domain.MicrochipScanCase{
		CaseID:            strings.TrimSpace(input.CaseID),
		PetID:             strings.TrimSpace(input.PetID),
		PetName:           strings.TrimSpace(input.PetName),
		Species:           species,
		RawChipInput:      strings.TrimSpace(input.RawChipInput),
		NormalizedChipID:  normalized,
		MaskedChipDisplay: masked,
		ChipStandard:      std,
		FrequencyKhz:      freqKhz,
		ManufacturerCode:  mfgCode,
		ManufacturerName:  mfgName,
		ManufacturerLotID: lotID,
		ScannerLocation:   input.ScannerLocation,
		ScannedByAgency:   agency,
		CreatedAt:         now.UTC(),
		UpdatedAt:         now.UTC(),
	}

	if simMode == "UNREGISTERED_DEAD_END" || (simMode == "" && mfgCode == "900") {
		scanCase.RegistryHops = []domain.RegistryQueryHop{
			{RegistryName: "HomeAgain", ManufacturerISO: "985", Responded: true, RecordFound: false, StatusNote: "No owner registration found"},
			{RegistryName: "AKC Reunite", ManufacturerISO: "981", Responded: true, RecordFound: false, StatusNote: "No owner registration found"},
			{RegistryName: "24PetWatch", ManufacturerISO: "982", Responded: true, RecordFound: false, StatusNote: "No owner registration found"},
			{RegistryName: "Michelson Found Animals", ManufacturerISO: "941", Responded: true, RecordFound: false, StatusNote: "No owner registration found"},
		}
		scanCase.Status = domain.ChipStatusUnregisteredBatchTraced
		scanCase.ImplantingClinicName = "Emerald City Spay/Neuter & Wellness Clinic"
		scanCase.ImplantingClinicPhone = "(206) 555-0148"
		scanCase.ImplantPatientChartID = "PIMS-CHART-" + normalized[len(normalized)-4:]
		scanCase.ResolutionNotes = fmt.Sprintf(
			"UNREGISTERED TRANSPONDER: 0/4 consumer registries matched. Traced manufacturer batch %s to implanting clinic (%s).",
			lotID,
			scanCase.ImplantingClinicName,
		)
	} else if simMode == "OUTDATED_PRIMARY_PHONE" || (simMode == "" && std == domain.ChipStandardAVID9) {
		scanCase.RegistryHops = []domain.RegistryQueryHop{
			{
				RegistryName:    "AVID PETtrac",
				ManufacturerISO: "AVID",
				Responded:       true,
				RecordFound:     true,
				LastUpdatedYear: 2019,
				StatusNote:      "Primary phone disconnected; secondary emergency contact on file",
			},
			{
				RegistryName:    "AAHA Universal Lookup",
				ManufacturerISO: "ISO-GLOBAL",
				Responded:       true,
				RecordFound:     false,
				StatusNote:      "Referred to issuing registry AVID PETtrac",
			},
			{
				RegistryName:    "HomeAgain Cross-Enrollment",
				ManufacturerISO: "985",
				Responded:       true,
				RecordFound:     false,
				StatusNote:      "No secondary cross-enrollment found",
			},
		}
		scanCase.Status = domain.ChipStatusSecondaryContactEscalated
		scanCase.WinningRegistry = "AVID PETtrac"
		scanCase.ResolvedOwnerName = "Elena Vance (Secondary: Marcus Vance)"
		scanCase.ResolvedOwnerPhoneMask = "+1 (206) •••-8841"
		scanCase.ResolutionNotes = "PRIMARY PHONE DISCONNECTED (2019 record): Escalated to secondary emergency contact on AVID PETtrac record."
	} else {
		winningRegistry := "HomeAgain Cross-Enrollment"
		switch mfgCode {
		case "985":
			winningRegistry = "HomeAgain"
		case "981":
			winningRegistry = "AKC Reunite"
		case "982":
			winningRegistry = "24PetWatch"
		}

		hops := []domain.RegistryQueryHop{
			{RegistryName: "HomeAgain", ManufacturerISO: "985", Responded: true, RecordFound: false, StatusNote: "No owner registration found"},
			{RegistryName: "AKC Reunite", ManufacturerISO: "981", Responded: true, RecordFound: false, StatusNote: "No owner registration found"},
			{RegistryName: "24PetWatch", ManufacturerISO: "982", Responded: true, RecordFound: false, StatusNote: "No owner registration found"},
			{RegistryName: "Michelson Found Animals", ManufacturerISO: "941", Responded: true, RecordFound: false, StatusNote: "No owner registration found"},
		}
		matchedInBaseHops := false
		for i := range hops {
			if hops[i].RegistryName == winningRegistry {
				hops[i].RecordFound = true
				hops[i].LastUpdatedYear = 2026
				hops[i].StatusNote = "Active owner registration verified"
				matchedInBaseHops = true
			}
		}
		if !matchedInBaseHops {
			hops[0] = domain.RegistryQueryHop{
				RegistryName:    winningRegistry,
				ManufacturerISO: mfgCode,
				Responded:       true,
				RecordFound:     true,
				LastUpdatedYear: 2026,
				StatusNote:      "Active owner registration verified",
			}
		}

		scanCase.RegistryHops = hops
		scanCase.Status = domain.ChipStatusRegisteredOwnerFound
		scanCase.WinningRegistry = winningRegistry
		scanCase.ResolvedOwnerName = "Jordan & Taylor Brooks"
		scanCase.ResolvedOwnerPhoneMask = "+1 (206) •••-3910"
		scanCase.ResolutionNotes = fmt.Sprintf("ACTIVE REGISTRATION FOUND in %s (%s, %.1f kHz).", winningRegistry, std, freqKhz)
	}

	if err := scanCase.Validate(); err != nil {
		return nil, err
	}
	return scanCase, nil
}

func TraceManufacturerBatchLot(scanCase *domain.MicrochipScanCase, input LotTraceInput, now time.Time) error {
	if scanCase == nil {
		return fmt.Errorf("microchip: scanCase is required")
	}

	clinicName := strings.TrimSpace(input.ImplantingClinicName)
	if clinicName == "" {
		clinicName = "Northwest Veterinary Specialists"
	}
	clinicPhone := strings.TrimSpace(input.ImplantingClinicPhone)
	if clinicPhone == "" {
		clinicPhone = "(206) 555-0192"
	}
	chartID := strings.TrimSpace(input.ImplantPatientChartID)
	if chartID == "" {
		suffix := scanCase.NormalizedChipID
		if len(suffix) > 4 {
			suffix = suffix[len(suffix)-4:]
		}
		chartID = "PIMS-LOT-" + suffix
	}
	ownerName := strings.TrimSpace(input.RecoveredOwnerName)
	if ownerName == "" {
		ownerName = "Devon & Priya Patel (Recovered via Clinic PIMS)"
	}
	ownerPhone := strings.TrimSpace(input.RecoveredOwnerPhoneMask)
	if ownerPhone == "" {
		ownerPhone = "+1 (206) •••-7724"
	}

	scanCase.ImplantingClinicName = clinicName
	scanCase.ImplantingClinicPhone = clinicPhone
	scanCase.ImplantPatientChartID = chartID
	scanCase.ResolvedOwnerName = ownerName
	scanCase.ResolvedOwnerPhoneMask = ownerPhone
	scanCase.Status = domain.ChipStatusUnregisteredBatchTraced
	scanCase.ResolutionNotes = fmt.Sprintf(
		"LOT TRACE COMPLETE (%s): Implanting clinic %s (Chart %s) identified owner %s (%s).",
		scanCase.ManufacturerLotID,
		scanCase.ImplantingClinicName,
		scanCase.ImplantPatientChartID,
		scanCase.ResolvedOwnerName,
		scanCase.ResolvedOwnerPhoneMask,
	)
	scanCase.UpdatedAt = now.UTC()

	return scanCase.Validate()
}

func ResolveMicrochipOwnership(scanCase *domain.MicrochipScanCase, input OwnershipResolutionInput, now time.Time) error {
	if scanCase == nil {
		return fmt.Errorf("microchip: scanCase is required")
	}
	notes := strings.TrimSpace(input.Notes)
	if notes == "" {
		return fmt.Errorf("microchip: resolution notes are required")
	}

	if vName := strings.TrimSpace(input.VerifiedOwnerName); vName != "" {
		scanCase.ResolvedOwnerName = vName
	}
	if vPhone := strings.TrimSpace(input.VerifiedOwnerPhoneMask); vPhone != "" {
		scanCase.ResolvedOwnerPhoneMask = vPhone
	}
	if strings.TrimSpace(scanCase.ResolvedOwnerName) == "" {
		scanCase.ResolvedOwnerName = "Verified Pet Owner"
	}
	if strings.TrimSpace(scanCase.ResolvedOwnerPhoneMask) == "" {
		scanCase.ResolvedOwnerPhoneMask = "+1 (206) •••-9001"
	}

	method := strings.TrimSpace(input.VerificationMethod)
	if method == "" {
		method = "PRIMARY_REGISTRY_MATCH"
	}

	scanCase.Status = domain.ChipStatusReunionDispatchedVerified
	scanCase.ResolutionNotes = fmt.Sprintf(
		"REUNION DISPATCHED (%s): Verified owner %s (%s) — %s",
		method,
		scanCase.ResolvedOwnerName,
		scanCase.ResolvedOwnerPhoneMask,
		notes,
	)
	scanCase.UpdatedAt = now.UTC()

	return scanCase.Validate()
}
