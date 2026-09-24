# Emergency Veterinary Passport & Crisis Medical Triage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement offline Emergency Veterinary Passports (ECDSA P-256 digital signature, CBOR binary compression, offline QR payload $< 380$ bytes) and a Species-Aware Crisis Medical Triage Engine (VECC/SALT protocol with canine vs. feline baselines, Red/Yellow/Green/Black classification, resuscitation calculations, SSE and P2P mesh relay, and WCAG AAA triage cockpit).

**Architecture:** A pure Go cryptographic and clinical subsystem (`pkg/domain`, `pkg/veterinary`) backed by in-memory/Firestore state store collections, integrated into WebFrontend via REST endpoints (`/api/v1/veterinary/...`), real-time SSE stream (`triage_assessment_created`), and P2P mesh network (`mesh:medical-triage`). Accessible HTML5 triage cockpit (`/triage`) and printable card (`/p/{id}/passport`).

**Tech Stack:** Go 1.26.5 (Standard Library: `crypto/ecdsa`, `crypto/elliptic`, `crypto/sha256`, `compress/zlib`, `encoding/json`), `skip2/go-qrcode`, Leaflet, HTML5 Canvas, Vanilla ES2022 JavaScript, Playwright.

**Spec:** `docs/superpowers/specs/2026-09-23-emergency-veterinary-passport-and-crisis-triage-design.md`

## Global Constraints

- Pinned toolchain: `export GOTOOLCHAIN=go1.26.5`
- Zero CGO Dependencies: 100% Go standard library and existing vendored modules (`skip2/go-qrcode`)
- Offline QR Size Boundary: Signed, compressed passport payload must be $< 380$ bytes to support high error tolerance (Level M)
- Species Baselines: Canine (HR 60–140, RR 10–30) vs Feline (HR 140–220, RR 20–40); temperature 100.0–102.5 °F; CRT $< 2.0$s
- Strict CSP compliance: Zero inline `<script>`, zero `eval()`, zero inline event handlers
- Strict WCAG AAA compliance: All text, icons, and triage badges $\ge 7:1$ contrast ratio in light and dark mode
- Pre-push verification gate: Clean pass on `make verify` and all tests pass with `-race`

---

### Task 1: Domain Models, Store Collections & Medical Schemas

**Files:**
- Create: `pkg/domain/veterinary.go`
- Create: `pkg/domain/veterinary_test.go`
- Modify: `pkg/store/store.go`

**Interfaces:**
- Consumes: `pkg/store/store.go`
- Produces: `domain.TriageCategory`, `domain.MucousMembraneColor`, `domain.AllergySeverity`, `domain.ClinicalAllergy`, `domain.VaccinationRecord`, `domain.ChronicCondition`, `domain.VitalSigns`, `domain.ClinicalTreatment`, `domain.TriageAssessment`, `domain.VeterinaryPassport`, `store.CollectionVeterinaryPassports`, `store.CollectionTriageAssessments`

- [ ] **Step 1: Write failing domain serialization tests**

Create `pkg/domain/veterinary_test.go`:
```go
package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestVeterinaryPassport_Serialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	passport := domain.VeterinaryPassport{
		PassportID:   "vp-12345",
		PetID:        "pet-dog-1",
		PetName:      "Buster",
		Species:      "Dog",
		Breed:        "Labrador",
		MicrochipID:  "985141000123456",
		RabiesTagID:  "RAB-2026-99",
		WeightKg:     28.5,
		EmergencyContact: "555-0199",
		PrimaryClinic:    "Metro Veterinary Trauma",
		IssuedAt:         now,
		ExpiresAt:        now.AddDate(1, 0, 0),
		Vaccinations: []domain.VaccinationRecord{
			{
				VaccineName:      "Rabies 3-Yr",
				AdministeredDate: now.AddDate(-1, 0, 0),
				ExpirationDate:   now.AddDate(2, 0, 0),
				ClinicName:       "Metro Vet",
				Verified:         true,
			},
		},
		Allergies: []domain.ClinicalAllergy{
			{
				Allergen:            "Penicillin",
				Severity:            domain.AllergySeverityAnaphylactic,
				ReactionDescription: "Acute facial angioedema and bronchospasm",
			},
		},
		ChronicConditions: []domain.ChronicCondition{
			{
				ConditionName: "Osteoarthritis",
				DiagnosedDate: now.AddDate(-2, 0, 0),
				Medications:   []string{"Carprofen 75mg PO BID"},
				CriticalFlag:  false,
			},
		},
	}

	data, err := json.Marshal(passport)
	if err != nil {
		t.Fatalf("failed to marshal VeterinaryPassport: %v", err)
	}

	var decoded domain.VeterinaryPassport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal VeterinaryPassport: %v", err)
	}

	if decoded.PassportID != passport.PassportID {
		t.Errorf("expected PassportID %s, got %s", passport.PassportID, decoded.PassportID)
	}
	if len(decoded.Allergies) != 1 || decoded.Allergies[0].Severity != domain.AllergySeverityAnaphylactic {
		t.Errorf("expected 1 anaphylactic allergy, got %+v", decoded.Allergies)
	}
}

func TestTriageAssessment_Serialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	assessment := domain.TriageAssessment{
		AssessmentID: "triage-001",
		PetID:        "pet-dog-1",
		HubID:        "hub-mobile-triage-1",
		MedicID:      "medic-42",
		MedicName:    "Dr. Aris Thorne",
		Species:      "Dog",
		WeightKg:     28.5,
		Category:     domain.TriageCategoryRed,
		Vitals: domain.VitalSigns{
			HeartRateBPM:       195,
			RespiratoryRateBPM: 55,
			TemperatureF:       103.8,
			CapillaryRefillSec: 3.2,
			MucousMembrane:     domain.MMColorCyanotic,
			GlasgowComaScale:   8,
		},
		TraumaNotes: "Suspected smoke inhalation and partial thickness burns on paws",
		AdministeredTreatments: []domain.ClinicalTreatment{
			{
				TreatmentID:    "treat-001",
				MedicationName: "Oxygen 100% via flow-by",
				Dosage:         "5 L/min",
				Route:          domain.RouteTopical,
				AdministeredBy: "Dr. Aris Thorne",
				AdministeredAt: now,
			},
		},
		AssessedAt: now,
	}

	data, err := json.Marshal(assessment)
	if err != nil {
		t.Fatalf("failed to marshal TriageAssessment: %v", err)
	}

	var decoded domain.TriageAssessment
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TriageAssessment: %v", err)
	}

	if decoded.Category != domain.TriageCategoryRed {
		t.Errorf("expected category RED, got %s", decoded.Category)
	}
	if decoded.Vitals.MucousMembrane != domain.MMColorCyanotic {
		t.Errorf("expected cyanotic MM, got %s", decoded.Vitals.MucousMembrane)
	}
}

func TestStoreCollectionConstants(t *testing.T) {
	if store.CollectionVeterinaryPassports != "veterinary_passports" {
		t.Errorf("unexpected CollectionVeterinaryPassports: %s", store.CollectionVeterinaryPassports)
	}
	if store.CollectionTriageAssessments != "triage_assessments" {
		t.Errorf("unexpected CollectionTriageAssessments: %s", store.CollectionTriageAssessments)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/domain/veterinary_test.go`
Expected: FAIL due to missing `pkg/domain/veterinary.go` and store constants.

- [ ] **Step 3: Implement domain models and store constants**

Create `pkg/domain/veterinary.go`:
```go
package domain

import "time"

// TriageCategory defines the emergency medical acuity tag.
type TriageCategory string

const (
	TriageCategoryRed    TriageCategory = "TRIAGE_RED"    // Immediate / Life-Threatening
	TriageCategoryYellow TriageCategory = "TRIAGE_YELLOW" // Delayed / Serious
	TriageCategoryGreen  TriageCategory = "TRIAGE_GREEN"  // Minor / Walking Wounded
	TriageCategoryBlack  TriageCategory = "TRIAGE_BLACK"  // Deceased / Expectant
)

// MucousMembraneColor represents the physical perfusion and oxygenation indicator.
type MucousMembraneColor string

const (
	MMColorPink     MucousMembraneColor = "MM_PINK"     // Normal perfusion
	MMColorPale     MucousMembraneColor = "MM_PALE"     // Shock, vasoconstriction, anemia
	MMColorCyanotic MucousMembraneColor = "MM_CYANOTIC" // Severe hypoxia, respiratory distress
	MMColorIcteric  MucousMembraneColor = "MM_ICTERIC"  // Jaundice, hepatic dysfunction
	MMColorBrickRed MucousMembraneColor = "MM_BRICK_RED" // Sepsis, hyperthermia, vasodilation
)

// AllergySeverity defines the clinical consequence of allergen exposure.
type AllergySeverity string

const (
	AllergySeverityMild         AllergySeverity = "MILD"
	AllergySeverityModerate     AllergySeverity = "MODERATE"
	AllergySeverityAnaphylactic AllergySeverity = "ANAPHYLACTIC"
)

// ClinicalAllergy represents a diagnosed medication or environmental allergen.
type ClinicalAllergy struct {
	Allergen            string          `json:"allergen"`
	Severity            AllergySeverity `json:"severity"`
	ReactionDescription string          `json:"reactionDescription"`
}

// VaccinationRecord represents an immunization history entry.
type VaccinationRecord struct {
	VaccineName      string    `json:"vaccineName"`
	AdministeredDate time.Time `json:"administeredDate"`
	ExpirationDate   time.Time `json:"expirationDate"`
	BatchNumber      string    `json:"batchNumber,omitempty"`
	ClinicName       string    `json:"clinicName"`
	Verified         bool      `json:"verified"`
}

// ChronicCondition documents ongoing illnesses and active treatments.
type ChronicCondition struct {
	ConditionName string    `json:"conditionName"`
	DiagnosedDate time.Time `json:"diagnosedDate"`
	Medications   []string  `json:"medications"`
	CriticalFlag  bool      `json:"criticalFlag"`
}

// VitalSigns represents quantifiable physiological measurements.
type VitalSigns struct {
	HeartRateBPM       int                 `json:"heartRateBpm"`
	RespiratoryRateBPM int                 `json:"respiratoryRateBpm"`
	TemperatureF       float64             `json:"temperatureF"`
	CapillaryRefillSec float64             `json:"capillaryRefillSec"`
	MucousMembrane     MucousMembraneColor `json:"mucousMembrane"`
	GlasgowComaScale   int                 `json:"glasgowComaScale"` // 3 to 18
}

// TreatmentRoute defines the pharmacological administration pathway.
type TreatmentRoute string

const (
	RouteIV      TreatmentRoute = "IV"
	RouteIM      TreatmentRoute = "IM"
	RouteSC      TreatmentRoute = "SC"
	RoutePO      TreatmentRoute = "PO"
	RouteTopical TreatmentRoute = "TOPICAL"
)

// ClinicalTreatment records an emergency procedure or medication administered.
type ClinicalTreatment struct {
	TreatmentID    string         `json:"treatmentId"`
	MedicationName string         `json:"medicationName"`
	Dosage         string         `json:"dosage"`
	Route          TreatmentRoute `json:"route"`
	AdministeredBy string         `json:"administeredBy"`
	AdministeredAt time.Time      `json:"administeredAt"`
	Notes          string         `json:"notes,omitempty"`
}

// TriageAssessment records an emergency clinical evaluation in the field.
type TriageAssessment struct {
	AssessmentID           string              `json:"assessmentId"`
	PetID                  string              `json:"petId"`
	HubID                  string              `json:"hubId,omitempty"`
	MedicID                string              `json:"medicId"`
	MedicName              string              `json:"medicName"`
	Species                string              `json:"species"`
	WeightKg               float64             `json:"weightKg"`
	Category               TriageCategory      `json:"category"`
	Vitals                 VitalSigns          `json:"vitals"`
	TraumaNotes            string              `json:"traumaNotes"`
	AdministeredTreatments []ClinicalTreatment `json:"administeredTreatments"`
	AssessedAt             time.Time           `json:"assessedAt"`
	ReassessedAt           *time.Time          `json:"reassessedAt,omitempty"`
}

// VeterinaryPassport is the verifiable digital health record for an animal.
type VeterinaryPassport struct {
	PassportID        string              `json:"passportId"`
	PetID             string              `json:"petId"`
	PetName           string              `json:"petName"`
	Species           string              `json:"species"`
	Breed             string              `json:"breed"`
	MicrochipID       string              `json:"microchipId,omitempty"`
	RabiesTagID       string              `json:"rabiesTagId,omitempty"`
	BloodType         string              `json:"bloodType,omitempty"`
	WeightKg          float64             `json:"weightKg"`
	Vaccinations      []VaccinationRecord `json:"vaccinations"`
	Allergies         []ClinicalAllergy   `json:"allergies"`
	ChronicConditions []ChronicCondition  `json:"chronicConditions"`
	EmergencyContact  string              `json:"emergencyContact"`
	PrimaryClinic     string              `json:"primaryClinic"`
	PublicKeyHex      string              `json:"publicKeyHex"`
	SignatureHex      string              `json:"signatureHex"`
	IssuedAt          time.Time           `json:"issuedAt"`
	ExpiresAt         time.Time           `json:"expiresAt"`
}
```

Modify `pkg/store/store.go` to add:
```go
const (
	CollectionVeterinaryPassports = "veterinary_passports"
	CollectionTriageAssessments   = "triage_assessments"
)
```

- [ ] **Step 4: Run test to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./pkg/domain/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/domain/veterinary.go pkg/domain/veterinary_test.go pkg/store/store.go
git commit -m "feat(domain): add veterinary passport, triage assessment, and clinical vital sign models"
```

---

### Task 2: Pure Go ECDSA P-256 Cryptographic Passport Engine & Compact QR Encoder

**Files:**
- Create: `pkg/veterinary/passport.go`
- Create: `pkg/veterinary/passport_test.go`

**Interfaces:**
- Consumes: `pkg/domain/veterinary.go`, `pkg/qrcode`
- Produces: `GeneratePassportKey()`, `SignPassport(pass *domain.VeterinaryPassport, priv *ecdsa.PrivateKey) (string, error)`, `VerifyPassport(pass *domain.VeterinaryPassport, pub *ecdsa.PublicKey) (bool, error)`, `EncodeOfflinePassportPayload(pass *domain.VeterinaryPassport, priv *ecdsa.PrivateKey) (string, error)`, `DecodeOfflinePassportPayload(encoded string, pub *ecdsa.PublicKey) (*domain.VeterinaryPassport, bool, error)`, `GeneratePassportQRCode(payload string) (string, error)`

- [ ] **Step 1: Write failing crypto & compression tests**

Create `pkg/veterinary/passport_test.go`:
```go
package veterinary_test

import (
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/veterinary"
)

func samplePassport() *domain.VeterinaryPassport {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.VeterinaryPassport{
		PassportID:       "vp-test-99",
		PetID:            "pet-test-1",
		PetName:          "Luna",
		Species:          "Dog",
		Breed:            "Border Collie",
		MicrochipID:      "985141000998877",
		RabiesTagID:      "RAB-2026-88",
		WeightKg:         18.2,
		EmergencyContact: "555-0144",
		PrimaryClinic:    "Cascadia Animal Hospital",
		IssuedAt:         now,
		ExpiresAt:        now.AddDate(1, 0, 0),
		Vaccinations: []domain.VaccinationRecord{
			{VaccineName: "Rabies", ExpirationDate: now.AddDate(1, 0, 0), Verified: true},
			{VaccineName: "DHPP", ExpirationDate: now.AddDate(1, 0, 0), Verified: true},
		},
		Allergies: []domain.ClinicalAllergy{
			{Allergen: "Penicillin", Severity: domain.AllergySeverityAnaphylactic},
		},
		ChronicConditions: []domain.ChronicCondition{
			{ConditionName: "Epilepsy", CriticalFlag: true, Medications: []string{"Phenobarbital 30mg BID"}},
		},
	}
}

func TestPassport_SignAndVerify(t *testing.T) {
	priv, pub, err := veterinary.GeneratePassportKey()
	if err != nil {
		t.Fatalf("GeneratePassportKey failed: %v", err)
	}

	pass := samplePassport()
	sig, err := veterinary.SignPassport(pass, priv)
	if err != nil {
		t.Fatalf("SignPassport failed: %v", err)
	}
	if len(sig) == 0 {
		t.Fatal("expected non-empty signature")
	}

	valid, err := veterinary.VerifyPassport(pass, pub)
	if err != nil {
		t.Fatalf("VerifyPassport failed: %v", err)
	}
	if !valid {
		t.Error("expected valid signature")
	}

	// Tamper test
	pass.WeightKg = 99.9
	tamperedValid, _ := veterinary.VerifyPassport(pass, pub)
	if tamperedValid {
		t.Error("expected tampered passport to fail verification")
	}
}

func TestPassport_OfflineQREncodeAndDecode(t *testing.T) {
	priv, pub, err := veterinary.GeneratePassportKey()
	if err != nil {
		t.Fatalf("GeneratePassportKey failed: %v", err)
	}

	pass := samplePassport()
	payload, err := veterinary.EncodeOfflinePassportPayload(pass, priv)
	if err != nil {
		t.Fatalf("EncodeOfflinePassportPayload failed: %v", err)
	}

	if !strings.HasPrefix(payload, "VP1:") {
		t.Errorf("expected VP1: prefix, got %s", payload[:4])
	}
	if len(payload) > 380 {
		t.Errorf("payload length %d exceeds max constraint 380 bytes", len(payload))
	}

	decoded, valid, err := veterinary.DecodeOfflinePassportPayload(payload, pub)
	if err != nil {
		t.Fatalf("DecodeOfflinePassportPayload failed: %v", err)
	}
	if !valid {
		t.Error("expected decoded passport to be valid")
	}
	if decoded.PassportID != pass.PassportID || decoded.PetID != pass.PetID {
		t.Errorf("decoded mismatch: got %+v", decoded)
	}
	if len(decoded.Allergies) != 1 || decoded.Allergies[0].Allergen != "Penicillin" {
		t.Errorf("expected penicillin allergy, got %+v", decoded.Allergies)
	}

	// QR Data URI generation
	dataUri, err := veterinary.GeneratePassportQRCode(payload)
	if err != nil {
		t.Fatalf("GeneratePassportQRCode failed: %v", err)
	}
	if !strings.HasPrefix(dataUri, "data:image/png;base64,") {
		t.Errorf("unexpected dataUri prefix: %s", dataUri[:30])
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/veterinary/...`
Expected: FAIL due to missing `pkg/veterinary` package.

- [ ] **Step 3: Implement cryptographic passport engine**

Create `pkg/veterinary/passport.go`:
```go
package veterinary

import (
	"bytes"
	"compress/zlib"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"

	skip2 "github.com/skip2/go-qrcode"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// GeneratePassportKey creates an ECDSA P-256 keypair for passport signing.
func GeneratePassportKey() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv, &priv.PublicKey, nil
}

// CanonicalBytes formats essential clinical fields for signing.
func CanonicalBytes(pass *domain.VeterinaryPassport) []byte {
	return []byte(fmt.Sprintf("%s|%s|%s|%s|%s|%.2f|%d|%d",
		pass.PassportID,
		pass.PetID,
		pass.Species,
		pass.MicrochipID,
		pass.RabiesTagID,
		pass.WeightKg,
		pass.IssuedAt.Unix(),
		pass.ExpiresAt.Unix(),
	))
}

// SignPassport computes an ECDSA P-256 SHA-256 signature for the passport.
func SignPassport(pass *domain.VeterinaryPassport, priv *ecdsa.PrivateKey) (string, error) {
	if priv == nil {
		return "", errors.New("private key is required")
	}
	hash := sha256.Sum256(CanonicalBytes(pass))
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		return "", err
	}

	sigBytes := append(r.Bytes(), s.Bytes())
	pass.SignatureHex = hex.EncodeToString(sigBytes)
	pass.PublicKeyHex = hex.EncodeToString(elliptic.Marshal(priv.Curve, priv.PublicKey.X, priv.PublicKey.Y))
	return pass.SignatureHex, nil
}

// VerifyPassport checks the ECDSA signature against the provided public key.
func VerifyPassport(pass *domain.VeterinaryPassport, pub *ecdsa.PublicKey) (bool, error) {
	if pub == nil {
		if pass.PublicKeyHex == "" {
			return false, errors.New("public key is required")
		}
		pubBytes, err := hex.DecodeString(pass.PublicKeyHex)
		if err != nil {
			return false, err
		}
		curve := elliptic.P256()
		x, y := elliptic.Unmarshal(curve, pubBytes)
		if x == nil || y == nil {
			return false, errors.New("invalid public key curve encoding")
		}
		pub = &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
	}

	sigBytes, err := hex.DecodeString(pass.SignatureHex)
	if err != nil || len(sigBytes) == 0 {
		return false, errors.New("invalid or empty signature")
	}

	mid := len(sigBytes) / 2
	r := new(big.Int).SetBytes(sigBytes[:mid])
	s := new(big.Int).SetBytes(sigBytes[mid:])

	hash := sha256.Sum256(CanonicalBytes(pass))
	return ecdsa.Verify(pub, hash[:], r, s), nil
}

// CompactPassportPayload represents a compressed binary-friendly DTO.
type CompactPassportPayload struct {
	ID        string                     `json:"i"`
	PetID     string                     `json:"p"`
	Name      string                     `json:"n"`
	Species   string                     `json:"s"`
	Chip      string                     `json:"c,omitempty"`
	Rabies    string                     `json:"r,omitempty"`
	Weight    float64                    `json:"w"`
	Allergies []string                   `json:"a,omitempty"`
	Vaccines  []string                   `json:"v,omitempty"`
	Issued    int64                      `json:"t"`
	Expires   int64                      `json:"x"`
	Sig       string                     `json:"g"`
	PubKey    string                     `json:"k"`
}

// EncodeOfflinePassportPayload generates an ultra-compact VP1: string.
func EncodeOfflinePassportPayload(pass *domain.VeterinaryPassport, priv *ecdsa.PrivateKey) (string, error) {
	if pass.SignatureHex == "" {
		if _, err := SignPassport(pass, priv); err != nil {
			return "", err
		}
	}

	var allergies []string
	for _, a := range pass.Allergies {
		allergies = append(allergies, fmt.Sprintf("%s:%s", a.Allergen, a.Severity))
	}

	var vaccines []string
	for _, v := range pass.Vaccinations {
		vaccines = append(vaccines, fmt.Sprintf("%s:%d", v.VaccineName, v.ExpirationDate.Unix()))
	}

	compact := CompactPassportPayload{
		ID:        pass.PassportID,
		PetID:     pass.PetID,
		Name:      pass.PetName,
		Species:   pass.Species,
		Chip:      pass.MicrochipID,
		Rabies:    pass.RabiesTagID,
		Weight:    pass.WeightKg,
		Allergies: allergies,
		Vaccines:  vaccines,
		Issued:    pass.IssuedAt.Unix(),
		Expires:   pass.ExpiresAt.Unix(),
		Sig:       pass.SignatureHex,
		PubKey:    pass.PublicKeyHex,
	}

	jsonData, err := json.Marshal(compact)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	if _, err := w.Write(jsonData); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	return "VP1:" + base64.RawURLEncoding.EncodeToString(b.Bytes()), nil
}

// DecodeOfflinePassportPayload parses and verifies an incoming VP1: string.
func DecodeOfflinePassportPayload(payload string, pub *ecdsa.PublicKey) (*domain.VeterinaryPassport, bool, error) {
	if !strings.HasPrefix(payload, "VP1:") {
		return nil, false, errors.New("invalid passport header: expected VP1:")
	}

	rawBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(payload, "VP1:"))
	if err != nil {
		return nil, false, fmt.Errorf("base64 decode error: %w", err)
	}

	r, err := zlib.NewReader(bytes.NewReader(rawBytes))
	if err != nil {
		return nil, false, fmt.Errorf("zlib decompress error: %w", err)
	}
	defer r.Close()

	decompressed, err := io.ReadAll(r)
	if err != nil {
		return nil, false, fmt.Errorf("read error: %w", err)
	}

	var compact CompactPassportPayload
	if err := json.Unmarshal(decompressed, &compact); err != nil {
		return nil, false, fmt.Errorf("json unmarshal error: %w", err)
	}

	var allergies []domain.ClinicalAllergy
	for _, aStr := range compact.Allergies {
		parts := strings.Split(aStr, ":")
		if len(parts) == 2 {
			allergies = append(allergies, domain.ClinicalAllergy{
				Allergen: parts[0],
				Severity: domain.AllergySeverity(parts[1]),
			})
		}
	}

	var vaccines []domain.VaccinationRecord
	for _, vStr := range compact.Vaccines {
		parts := strings.Split(vStr, ":")
		if len(parts) == 2 {
			var exp int64
			fmt.Sscanf(parts[1], "%d", &exp)
			vaccines = append(vaccines, domain.VaccinationRecord{
				VaccineName:    parts[0],
				ExpirationDate: time.Unix(exp, 0).UTC(),
				Verified:       true,
			})
		}
	}

	pass := &domain.VeterinaryPassport{
		PassportID:   compact.ID,
		PetID:        compact.PetID,
		PetName:      compact.Name,
		Species:      compact.Species,
		MicrochipID:  compact.Chip,
		RabiesTagID:  compact.Rabies,
		WeightKg:     compact.Weight,
		Allergies:    allergies,
		Vaccinations: vaccines,
		IssuedAt:     time.Unix(compact.Issued, 0).UTC(),
		ExpiresAt:    time.Unix(compact.Expires, 0).UTC(),
		SignatureHex: compact.Sig,
		PublicKeyHex: compact.PubKey,
	}

	valid, verifyErr := VerifyPassport(pass, pub)
	return pass, valid, verifyErr
}

// GeneratePassportQRCode outputs a base64 PNG data URI from a payload.
func GeneratePassportQRCode(payload string) (string, error) {
	png, err := skip2.Encode(payload, skip2.Medium, 256)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}
```

- [ ] **Step 4: Run test to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./pkg/veterinary/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/veterinary/passport.go pkg/veterinary/passport_test.go
git commit -m "feat(veterinary): implement ECDSA P-256 signed passport with compact Zlib QR encoding"
```

---

### Task 3: Species-Aware Clinical Triage Engine & Resuscitation Calculator

**Files:**
- Create: `pkg/veterinary/triage.go`
- Create: `pkg/veterinary/triage_test.go`

**Interfaces:**
- Consumes: `pkg/domain/veterinary.go`
- Produces: `TraumaIndicators`, `EvaluateTriage(species string, vitals domain.VitalSigns, trauma TraumaIndicators) (domain.TriageCategory, []string)`, `CalculateEmergencyDosages(species string, weightKg float64) map[string]string`

- [ ] **Step 1: Write failing triage engine tests**

Create `pkg/veterinary/triage_test.go`:
```go
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

func TestCalculateEmergencyDosages(t *testing.T) {
	dosages := veterinary.CalculateEmergencyDosages("Dog", 20.0)
	if dosages["ShockFluidsBolus"] != "300 mL IV over 15 min" {
		t.Errorf("unexpected fluid bolus: %s", dosages["ShockFluidsBolus"])
	}
	if dosages["EpinephrineCPR"] != "0.20 mg (0.20 mL of 1:1000) IV/IO" {
		t.Errorf("unexpected epinephrine dose: %s", dosages["EpinephrineCPR"])
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/veterinary/triage_test.go`
Expected: FAIL due to missing functions.

- [ ] **Step 3: Implement clinical triage evaluator and dosage calculator**

Create `pkg/veterinary/triage.go`:
```go
package veterinary

import (
	"fmt"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// TraumaIndicators represents physical trauma observed during rapid triage.
type TraumaIndicators struct {
	ArterialHemorrhage bool `json:"arterialHemorrhage"`
	PenetratingChest   bool `json:"penetratingChest"`
	SevereBurns        bool `json:"severeBurns"` // > 30% BSA
	ModerateBurns      bool `json:"moderateBurns"` // 10-30% BSA
	OpenFracture       bool `json:"openFracture"`
	UnresponsiveAsystole bool `json:"unresponsiveAsystole"`
}

// EvaluateTriage evaluates vitals and trauma indicators into an acuity tag.
func EvaluateTriage(species string, vitals domain.VitalSigns, trauma TraumaIndicators) (domain.TriageCategory, []string) {
	var reasons []string
	isCat := strings.EqualFold(species, "cat") || strings.EqualFold(species, "feline")

	// 1. Check Deceased / Expectant (Black)
	if trauma.UnresponsiveAsystole || (vitals.HeartRateBPM == 0 && vitals.RespiratoryRateBPM == 0 && vitals.GlasgowComaScale <= 3) {
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
	if vitals.GlasgowComaScale >= 9 && vitals.GlasgowComaScale <= 14 {
		reasons = append(reasons, fmt.Sprintf("Depressed mentation (GCS %d)", vitals.GlasgowComaScale))
	}
	if vitals.TemperatureF > 0 && (vitals.TemperatureF < 99.5 || vitals.TemperatureF > 103.5) {
		reasons = append(reasons, fmt.Sprintf("Abnormal temperature (%.1f F)", vitals.TemperatureF))
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
	isCat := strings.EqualFold(species, "cat") || strings.EqualFold(species, "feline")

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
```

- [ ] **Step 4: Run test to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./pkg/veterinary/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/veterinary/triage.go pkg/veterinary/triage_test.go
git commit -m "feat(veterinary): implement species-aware clinical triage evaluator and emergency dosage calculator"
```

---

### Task 4: WebFrontend REST Endpoints, Mobile Triage Hub Hooks & SSE/Mesh Distribution

**Files:**
- Create: `internal/app/webfrontend/veterinary_handlers.go`
- Create: `internal/app/webfrontend/veterinary_handlers_test.go`
- Modify: `internal/app/webfrontend/server.go`

**Interfaces:**
- Consumes: `pkg/domain/veterinary.go`, `pkg/veterinary`, `pkg/store`
- Produces: `/api/v1/veterinary/passports`, `/api/v1/veterinary/passports/verify`, `/api/v1/veterinary/triage`, `/api/v1/veterinary/triage/{petId}`, `/api/v1/veterinary/triage/{assessmentId}/treatments`

- [ ] **Step 1: Write failing handler tests**

Create `internal/app/webfrontend/veterinary_handlers_test.go`:
```go
package webfrontend_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestVeterinaryEndpoints_Lifecycle(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewServer(webfrontend.Config{
		Store: memStore,
	})

	// 1. Issue passport
	reqBody, _ := json.Marshal(map[string]any{
		"petId":       "pet-vet-001",
		"petName":     "Milo",
		"species":     "Cat",
		"breed":       "Siamese",
		"microchipId": "985141000445566",
		"weightKg":    4.2,
		"allergies": []map[string]any{
			{"allergen": "Amoxicillin", "severity": "MODERATE"},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/passports", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var passportRes map[string]any
	json.Unmarshal(rec.Body.Bytes(), &passportRes)
	qrPayload, ok := passportRes["qrPayload"].(string)
	if !ok || len(qrPayload) == 0 {
		t.Fatalf("missing qrPayload in response: %+v", passportRes)
	}

	// 2. Verify passport payload
	verifyBody, _ := json.Marshal(map[string]string{
		"qrPayload": qrPayload,
	})
	recVerify := httptest.NewRecorder()
	reqVerify := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/passports/verify", bytes.NewReader(verifyBody))
	reqVerify.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recVerify, reqVerify)

	if recVerify.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for verify, got %d: %s", recVerify.Code, recVerify.Body.String())
	}

	// 3. Submit Emergency Triage Assessment
	triageBody, _ := json.Marshal(map[string]any{
		"petId":     "pet-vet-001",
		"medicId":   "medic-9",
		"medicName": "Sarah Jenks, RVT",
		"species":   "Cat",
		"weightKg":  4.2,
		"vitals": map[string]any{
			"heartRateBpm":       240,
			"respiratoryRateBpm": 60,
			"temperatureF":       103.5,
			"capillaryRefillSec": 3.2,
			"mucousMembrane":     "MM_CYANOTIC",
			"glasgowComaScale":   8,
		},
		"traumaNotes": "Smoke inhalation and lethargy",
	})

	recTriage := httptest.NewRecorder()
	reqTriage := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/triage", bytes.NewReader(triageBody))
	reqTriage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTriage, reqTriage)

	if recTriage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for triage, got %d: %s", recTriage.Code, recTriage.Body.String())
	}

	var triageRes domain.TriageAssessment
	json.Unmarshal(recTriage.Body.Bytes(), &triageRes)
	if triageRes.Category != domain.TriageCategoryRed {
		t.Errorf("expected TRIAGE_RED, got %s", triageRes.Category)
	}

	// 4. Append Clinical Treatment
	treatBody, _ := json.Marshal(map[string]any{
		"medicationName": "Oxygen Flow-By & Butorphanol",
		"dosage":         "0.8 mg",
		"route":          "IM",
		"administeredBy": "Sarah Jenks, RVT",
	})
	recTreat := httptest.NewRecorder()
	reqTreat := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/triage/"+triageRes.AssessmentID+"/treatments", bytes.NewReader(treatBody))
	reqTreat.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTreat, reqTreat)

	if recTreat.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for treatment, got %d: %s", recTreat.Code, recTreat.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./internal/app/webfrontend/veterinary_handlers_test.go`
Expected: FAIL due to missing handlers and routes.

- [ ] **Step 3: Implement veterinary handlers and wire into server**

Create `internal/app/webfrontend/veterinary_handlers.go`:
```go
package webfrontend

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/veterinary"
)

var (
	masterPrivKey *ecdsa.PrivateKey
	masterPubKey  *ecdsa.PublicKey
	keyOnce       sync.Once
)

func getMasterKeys() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	var err error
	keyOnce.Do(func() {
		masterPrivKey, masterPubKey, err = veterinary.GeneratePassportKey()
	})
	return masterPrivKey, masterPubKey, err
}

func (s *Server) handleCreateVeterinaryPassport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PetID             string                     `json:"petId"`
		PetName           string                     `json:"petName"`
		Species           string                     `json:"species"`
		Breed             string                     `json:"breed"`
		MicrochipID       string                     `json:"microchipId"`
		RabiesTagID       string                     `json:"rabiesTagId"`
		BloodType         string                     `json:"bloodType"`
		WeightKg          float64                    `json:"weightKg"`
		Vaccinations      []domain.VaccinationRecord `json:"vaccinations"`
		Allergies         []domain.ClinicalAllergy   `json:"allergies"`
		ChronicConditions []domain.ChronicCondition  `json:"chronicConditions"`
		EmergencyContact  string                     `json:"emergencyContact"`
		PrimaryClinic     string                     `json:"primaryClinic"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.PetID == "" || req.PetName == "" {
		http.Error(w, "petId and petName are required", http.StatusBadRequest)
		return
	}

	priv, _, err := getMasterKeys()
	if err != nil {
		http.Error(w, "Failed to initialize crypto keys", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	passport := domain.VeterinaryPassport{
		PassportID:        fmt.Sprintf("vp-%d", now.UnixNano()),
		PetID:             req.PetID,
		PetName:           req.PetName,
		Species:           req.Species,
		Breed:             req.Breed,
		MicrochipID:       req.MicrochipID,
		RabiesTagID:       req.RabiesTagID,
		BloodType:         req.BloodType,
		WeightKg:          req.WeightKg,
		Vaccinations:      req.Vaccinations,
		Allergies:         req.Allergies,
		ChronicConditions: req.ChronicConditions,
		EmergencyContact:  req.EmergencyContact,
		PrimaryClinic:     req.PrimaryClinic,
		IssuedAt:          now,
		ExpiresAt:         now.AddDate(1, 0, 0),
	}

	qrPayload, err := veterinary.EncodeOfflinePassportPayload(&passport, priv)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode passport: %v", err), http.StatusInternalServerError)
		return
	}

	qrDataUri, err := veterinary.GeneratePassportQRCode(qrPayload)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate QR code: %v", err), http.StatusInternalServerError)
		return
	}

	if s.store != nil {
		passBytes, _ := json.Marshal(passport)
		_ = s.store.Put(r.Context(), store.CollectionVeterinaryPassports, passport.PassportID, passBytes)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"passportId": passport.PassportID,
		"passport":   passport,
		"qrPayload":  qrPayload,
		"qrDataUri":  qrDataUri,
		"verified":   true,
	})
}

func (s *Server) handleVerifyVeterinaryPassport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		QRPayload string `json:"qrPayload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.QRPayload == "" {
		http.Error(w, "qrPayload is required", http.StatusBadRequest)
		return
	}

	_, pub, err := getMasterKeys()
	if err != nil {
		http.Error(w, "Crypto error", http.StatusInternalServerError)
		return
	}

	passport, valid, err := veterinary.DecodeOfflinePassportPayload(req.QRPayload, pub)
	if err != nil || !valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"verified": false,
			"error":    "Signature invalid or payload corrupt",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"verified": true,
		"passport": passport,
	})
}

func (s *Server) handleCreateTriageAssessment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PetID       string                      `json:"petId"`
		HubID       string                      `json:"hubId"`
		MedicID     string                      `json:"medicId"`
		MedicName   string                      `json:"medicName"`
		Species     string                      `json:"species"`
		WeightKg    float64                     `json:"weightKg"`
		Vitals      domain.VitalSigns           `json:"vitals"`
		Trauma      veterinary.TraumaIndicators `json:"trauma"`
		TraumaNotes string                      `json:"traumaNotes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	category, reasons := veterinary.EvaluateTriage(req.Species, req.Vitals, req.Trauma)
	dosages := veterinary.CalculateEmergencyDosages(req.Species, req.WeightKg)

	now := time.Now().UTC()
	assessment := domain.TriageAssessment{
		AssessmentID:           fmt.Sprintf("triage-%d", now.UnixNano()),
		PetID:                  req.PetID,
		HubID:                  req.HubID,
		MedicID:                req.MedicID,
		MedicName:              req.MedicName,
		Species:                req.Species,
		WeightKg:               req.WeightKg,
		Category:               category,
		Vitals:                 req.Vitals,
		TraumaNotes:            req.TraumaNotes,
		AdministeredTreatments: []domain.ClinicalTreatment{},
		AssessedAt:             now,
	}

	if s.store != nil {
		assessBytes, _ := json.Marshal(assessment)
		_ = s.store.Put(r.Context(), store.CollectionTriageAssessments, assessment.AssessmentID, assessBytes)
	}

	// SSE Broadcast
	s.broadcastSSE("triage_assessment_created", map[string]any{
		"assessmentId": assessment.AssessmentID,
		"petId":        assessment.PetID,
		"category":     assessment.Category,
		"species":      assessment.Species,
		"reasons":      reasons,
		"dosages":      dosages,
		"assessedAt":   assessment.AssessedAt,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assessment)
}

func (s *Server) handleAppendTriageTreatment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	assessmentID := r.PathValue("assessmentId")
	if assessmentID == "" {
		http.Error(w, "assessmentId required", http.StatusBadRequest)
		return
	}

	var req struct {
		MedicationName string                `json:"medicationName"`
		Dosage         string                `json:"dosage"`
		Route          domain.TreatmentRoute `json:"route"`
		AdministeredBy string                `json:"administeredBy"`
		Notes          string                `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if s.store == nil {
		http.Error(w, "Store not initialized", http.StatusInternalServerError)
		return
	}

	data, err := s.store.Get(r.Context(), store.CollectionTriageAssessments, assessmentID)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	var assessment domain.TriageAssessment
	if err := json.Unmarshal(data, &assessment); err != nil {
		http.Error(w, "Data corrupt", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	treatment := domain.ClinicalTreatment{
		TreatmentID:    fmt.Sprintf("treat-%d", now.UnixNano()),
		MedicationName: req.MedicationName,
		Dosage:         req.Dosage,
		Route:          req.Route,
		AdministeredBy: req.AdministeredBy,
		AdministeredAt: now,
		Notes:          req.Notes,
	}

	assessment.AdministeredTreatments = append(assessment.AdministeredTreatments, treatment)
	assessment.ReassessedAt = &now

	updatedBytes, _ := json.Marshal(assessment)
	_ = s.store.Put(r.Context(), store.CollectionTriageAssessments, assessmentID, updatedBytes)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assessment)
}
```

In `internal/app/webfrontend/server.go`, register routes:
```go
mux.HandleFunc("POST /api/v1/veterinary/passports", s.handleCreateVeterinaryPassport)
mux.HandleFunc("POST /api/v1/veterinary/passports/verify", s.handleVerifyVeterinaryPassport)
mux.HandleFunc("POST /api/v1/veterinary/triage", s.handleCreateTriageAssessment)
mux.HandleFunc("POST /api/v1/veterinary/triage/{assessmentId}/treatments", s.handleAppendTriageTreatment)
```

- [ ] **Step 4: Run test to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./internal/app/webfrontend/veterinary_handlers_test.go`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/veterinary_handlers.go internal/app/webfrontend/veterinary_handlers_test.go internal/app/webfrontend/server.go
git commit -m "feat(webfrontend): add veterinary passport, triage assessment, and treatment REST endpoints"
```

---

### Task 5: Crisis Triage Cockpit UI, Printable Passport Card & WCAG AAA CSS

**Files:**
- Create: `internal/app/webfrontend/templates/triage.html`
- Create: `internal/app/webfrontend/templates/passport.html`
- Create: `internal/app/webfrontend/static/js/veterinary-triage.js`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Modify: `internal/app/webfrontend/layout.html`
- Create: `tests/playwright/unit/veterinary-triage.spec.ts`

**Interfaces:**
- Consumes: Task 4 endpoints, `/triage`, `/p/{id}/passport`
- Produces: Visual Triage HUD, live gauge meters, emergency dosage display, printable passport card layout, and high-contrast WCAG AAA badges.

- [ ] **Step 1: Write Playwright unit tests for Triage UI & Passport**

Create `tests/playwright/unit/veterinary-triage.spec.ts`:
```ts
import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Crisis Medical Triage Cockpit & Passport UI', () => {
  test('Triage cockpit renders species buttons, vitals HUD, and dynamic acuity tag', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#triage-species-dog')).toBeVisible();
    await expect(page.locator('#triage-species-cat')).toBeVisible();

    // Fill vitals
    await page.locator('#vitals-hr').fill('210');
    await page.locator('#vitals-rr').fill('65');
    await page.locator('#vitals-crt-3').click();
    await page.locator('#vitals-mm-cyanotic').click();

    // Assert dynamic category calculation
    const indicator = page.locator('#live-triage-indicator');
    await expect(indicator).toContainText('TRIAGE RED');
    await expect(indicator).toHaveClass(/badge-triage-red/);
  });

  test('Printable passport card displays QR code and critical allergy banner', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/p/sample-passport/passport`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('.passport-card')).toBeVisible();
    await expect(page.locator('.passport-qr-code')).toBeVisible();
  });
});
```

- [ ] **Step 2: Run test to verify failure**

Run: `cd tests/playwright && npx playwright test tests/playwright/unit/veterinary-triage.spec.ts`
Expected: FAIL due to missing `/triage` template.

- [ ] **Step 3: Implement triage.html, passport.html, JS controller, and CSS**

Implement:
- `triage.html` with clean form, live vital sign meters, emergency dosage bar, and active patient triage list.
- `passport.html` with high-resolution QR, vaccination table, emergency contacts, and `.alert-allergy-critical`.
- `veterinary-triage.js` providing real-time calculation, SSE listener, and offline outbox.
- `styles.css` with high contrast badges:
  ```css
  .badge-triage-red {
    background-color: #8A0000;
    color: #FFFFFF;
    border: 1px solid #FF8080;
  }
  .badge-triage-yellow {
    background-color: #594200;
    color: #FFFBEA;
    border: 1px solid #FFD700;
  }
  .badge-triage-green {
    background-color: #0F5132;
    color: #FFFFFF;
    border: 1px solid #75B798;
  }
  .badge-triage-black {
    background-color: #121212;
    color: #FFFFFF;
    border: 1px solid #666666;
  }
  .alert-allergy-critical {
    background-color: #FFF0F0;
    border-left: 6px solid #8A0000;
    color: #8A0000;
    font-weight: bold;
    padding: 0.75rem 1rem;
    margin: 1rem 0;
  }
  ```

- [ ] **Step 4: Run Playwright unit tests to verify pass**

Run: `cd tests/playwright && npx playwright test tests/playwright/unit/veterinary-triage.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/templates/triage.html internal/app/webfrontend/templates/passport.html internal/app/webfrontend/static/js/veterinary-triage.js internal/app/webfrontend/static/css/styles.css tests/playwright/unit/veterinary-triage.spec.ts
git commit -m "feat(ui): add crisis medical triage cockpit, printable passport card, and WCAG AAA triage styles"
```

---

### Task 6: Automated Playwright E2E User Journey & Full Repository Verification

**Files:**
- Create: `tests/playwright/e2e/veterinary-triage-journey.spec.ts`

**Interfaces:**
- Consumes: All Milestone 11.4 endpoints, templates, and UI components

- [ ] **Step 1: Write comprehensive 6-step E2E journey test**

Create `tests/playwright/e2e/veterinary-triage-journey.spec.ts`:
```ts
import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe.serial('Milestone 11.4: Emergency Veterinary Passport & Crisis Triage Journey', () => {
  const testPetID = `pet-triage-${Date.now()}`;
  let qrPayload = '';
  let assessmentID = '';

  test('Step 1: Issue Emergency Veterinary Passport via API with Rabies and Allergy', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/veterinary/passports`, {
      data: {
        petId: testPetID,
        petName: 'Rusty',
        species: 'Dog',
        breed: 'Golden Retriever',
        microchipId: '985141000998811',
        rabiesTagID: 'RAB-2026-X1',
        weightKg: 30.0,
        vaccinations: [
          { vaccineName: 'Rabies 3-Yr', expirationDate: '2027-09-01T00:00:00Z', verified: true },
        ],
        allergies: [
          { allergen: 'Penicillin', severity: 'ANAPHYLACTIC', reactionDescription: 'Anaphylaxis' },
        ],
      },
    });

    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.passportId).toBeTruthy();
    expect(body.qrPayload).toBeTruthy();
    expect(body.qrDataUri).toContain('data:image/png;base64,');
    qrPayload = body.qrPayload;
  });

  test('Step 2: Verify Offline Passport Payload via POST /api/v1/veterinary/passports/verify', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/veterinary/passports/verify`, {
      data: { qrPayload },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.verified).toBe(true);
    expect(body.passport.petName).toBe('Rusty');
    expect(body.passport.allergies.length).toBe(1);
    expect(body.passport.allergies[0].allergen).toBe('Penicillin');
  });

  test('Step 3: Open /triage Cockpit, input Critical Vitals, assert TRIAGE_RED', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#triage-species-dog').click();
    await page.locator('#triage-pet-id').fill(testPetID);
    await page.locator('#triage-weight').fill('30');
    await page.locator('#vitals-hr').fill('215');
    await page.locator('#vitals-rr').fill('65');
    await page.locator('#vitals-crt-3').click();
    await page.locator('#vitals-mm-cyanotic').click();

    const indicator = page.locator('#live-triage-indicator');
    await expect(indicator).toContainText('TRIAGE RED');
  });

  test('Step 4: Submit Triage Assessment and Assert Queue Addition with Red Badge', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#triage-species-dog').click();
    await page.locator('#triage-pet-id').fill(testPetID);
    await page.locator('#triage-weight').fill('30');
    await page.locator('#vitals-hr').fill('215');
    await page.locator('#vitals-rr').fill('65');
    await page.locator('#vitals-crt-3').click();
    await page.locator('#vitals-mm-cyanotic').click();

    await page.locator('#btn-submit-triage').click();

    const patientCard = page.locator(`.patient-card[data-pet-id="${testPetID}"]`);
    await expect(patientCard).toBeVisible();
    await expect(patientCard.locator('.badge-triage-red')).toBeVisible();

    const idAttr = await patientCard.getAttribute('data-assessment-id');
    expect(idAttr).toBeTruthy();
    assessmentID = idAttr!;
  });

  test('Step 5: Administer Emergency IV Shock Fluid and Pain Treatment', async ({ request, page }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/veterinary/triage/${assessmentID}/treatments`, {
      data: {
        medicationName: 'Lactated Ringers Solution Bolus',
        dosage: '450 mL',
        route: 'IV',
        administeredBy: 'Dr. Aris Thorne',
        notes: 'Administered over 15 minutes for hypovolemic shock',
      },
    });
    expect(res.status()).toBe(200);

    await page.goto(`${WEB_FRONTEND_URL}/triage`);
    const patientCard = page.locator(`.patient-card[data-pet-id="${testPetID}"]`);
    await patientCard.click();
    await expect(page.locator('.treatment-log')).toContainText('Lactated Ringers Solution Bolus');
  });

  test('Step 6: Verify Printable Passport Card layout, QR code, and Critical Allergy Alert', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/p/${testPetID}/passport`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('.passport-card')).toBeVisible();
    await expect(page.locator('.passport-qr-code')).toBeVisible();
    const alertBox = page.locator('.alert-allergy-critical');
    await expect(alertBox).toBeVisible();
    await expect(alertBox).toContainText(/Penicillin/i);
  });
});
```

- [ ] **Step 2: Run Playwright journey test**

Run: `cd tests/playwright && npx playwright test tests/playwright/e2e/veterinary-triage-journey.spec.ts`
Expected: PASS.

- [ ] **Step 3: Run full repository verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: Clean pass with 0 linter issues, OpenTofu valid, yamllint clean, and 100% race-free Go tests passing.

- [ ] **Step 4: Commit**

```bash
git add tests/playwright/e2e/veterinary-triage-journey.spec.ts
git commit -m "test(playwright): add E2E journey tests for emergency veterinary passport and crisis medical triage"
```

---

### Task 7: Final Whole-Branch Code Review & Verification Gate

**Description:**
Conduct rigorous whole-branch code review with the `pro` model across all files implemented in Tasks 1–6, addressing any findings in a single coordinated fix wave, followed by pre-push repository verification and GitHub Pull Request creation.
