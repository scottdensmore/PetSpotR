package veterinary_test

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
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

	// Verify using embedded public key (pub == nil)
	embeddedValid, err := veterinary.VerifyPassport(pass, nil)
	if err != nil {
		t.Fatalf("VerifyPassport with embedded pubkey failed: %v", err)
	}
	if !embeddedValid {
		t.Error("expected valid signature using embedded public key")
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
	if len(decoded.Vaccinations) != 2 || decoded.Vaccinations[0].VaccineName != "Rabies" {
		t.Errorf("expected rabies vaccination, got %+v", decoded.Vaccinations)
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

func TestPassport_SigningValidation(t *testing.T) {
	pass := samplePassport()

	// Nil private key
	_, err := veterinary.SignPassport(pass, nil)
	if err == nil {
		t.Error("expected error when signing with nil private key")
	}

	// Nil public key and empty PublicKeyHex
	pass.SignatureHex = "deadbeef"
	pass.PublicKeyHex = ""
	_, err = veterinary.VerifyPassport(pass, nil)
	if err == nil {
		t.Error("expected error when verifying with nil pub and empty PublicKeyHex")
	}

	// Invalid hex in SignatureHex
	pass.PublicKeyHex = "04"
	pass.SignatureHex = "not-hex"
	_, err = veterinary.VerifyPassport(pass, nil)
	if err == nil {
		t.Error("expected error for invalid hex signature")
	}

	// Invalid hex in PublicKeyHex
	pass.PublicKeyHex = "invalid-hex"
	pass.SignatureHex = "01020304"
	_, err = veterinary.VerifyPassport(pass, nil)
	if err == nil {
		t.Error("expected error for invalid hex public key")
	}
}

func TestPassport_DecodeOfflineErrors(t *testing.T) {
	_, pub, err := veterinary.GeneratePassportKey()
	if err != nil {
		t.Fatalf("GeneratePassportKey failed: %v", err)
	}

	// Missing VP1: prefix
	_, _, err = veterinary.DecodeOfflinePassportPayload("INVALID:12345", pub)
	if err == nil || !strings.Contains(err.Error(), "VP1:") {
		t.Errorf("expected header error, got %v", err)
	}

	// Corrupted base64
	_, _, err = veterinary.DecodeOfflinePassportPayload("VP1:???not-base64???", pub)
	if err == nil || !strings.Contains(err.Error(), "base64") {
		t.Errorf("expected base64 error, got %v", err)
	}

	// Corrupted zlib
	_, _, err = veterinary.DecodeOfflinePassportPayload("VP1:aGVsbG8gd29ybGQ", pub)
	if err == nil || !strings.Contains(err.Error(), "zlib") {
		t.Errorf("expected zlib error, got %v", err)
	}
}

func TestPassport_NilGuards(t *testing.T) {
	priv, pub, err := veterinary.GeneratePassportKey()
	if err != nil {
		t.Fatalf("GeneratePassportKey failed: %v", err)
	}

	// SignPassport with nil passport
	if _, err := veterinary.SignPassport(nil, priv); err == nil || err.Error() != "passport is nil" {
		t.Errorf("expected 'passport is nil' error, got %v", err)
	}

	// VerifyPassport with nil passport
	if valid, err := veterinary.VerifyPassport(nil, pub); valid || err == nil || err.Error() != "passport is nil" {
		t.Errorf("expected 'passport is nil' error, got valid=%v, err=%v", valid, err)
	}

	// EncodeOfflinePassportPayload with nil passport
	if _, err := veterinary.EncodeOfflinePassportPayload(nil, priv); err == nil || err.Error() != "passport is nil" {
		t.Errorf("expected 'passport is nil' error, got %v", err)
	}

	// CanonicalBytes with nil passport
	if b := veterinary.CanonicalBytes(nil); b != nil {
		t.Errorf("expected nil for CanonicalBytes(nil), got %v", b)
	}
}

func TestPassport_StrictSignatureLength(t *testing.T) {
	_, pub, err := veterinary.GeneratePassportKey()
	if err != nil {
		t.Fatalf("GeneratePassportKey failed: %v", err)
	}

	pass := samplePassport()

	// 0 bytes
	pass.SignatureHex = ""
	if _, err := veterinary.VerifyPassport(pass, pub); err == nil || !strings.Contains(err.Error(), "signature must be exactly 64 bytes") {
		t.Errorf("expected strict signature error for 0 bytes, got %v", err)
	}

	// 32 bytes (64 hex characters)
	pass.SignatureHex = strings.Repeat("aa", 32)
	if _, err := veterinary.VerifyPassport(pass, pub); err == nil || !strings.Contains(err.Error(), "signature must be exactly 64 bytes") {
		t.Errorf("expected strict signature error for 32 bytes, got %v", err)
	}

	// 63 bytes (126 hex characters)
	pass.SignatureHex = strings.Repeat("bb", 63)
	if _, err := veterinary.VerifyPassport(pass, pub); err == nil || !strings.Contains(err.Error(), "signature must be exactly 64 bytes") {
		t.Errorf("expected strict signature error for 63 bytes, got %v", err)
	}

	// 65 bytes (130 hex characters)
	pass.SignatureHex = strings.Repeat("cc", 65)
	if _, err := veterinary.VerifyPassport(pass, pub); err == nil || !strings.Contains(err.Error(), "signature must be exactly 64 bytes") {
		t.Errorf("expected strict signature error for 65 bytes, got %v", err)
	}
}

func TestPassport_ZipBombDecompressionLimit(t *testing.T) {
	_, pub, err := veterinary.GeneratePassportKey()
	if err != nil {
		t.Fatalf("GeneratePassportKey failed: %v", err)
	}

	// Construct an oversized payload (> 10KB uncompressed)
	largeData := bytes.Repeat([]byte(`{"i":"large-passport","p":"pet-1"}`), 1000) // ~33 KB
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	if _, err := w.Write(largeData); err != nil {
		t.Fatalf("failed to write zlib: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zlib: %v", err)
	}

	oversizedPayload := "VP1:" + base64.RawURLEncoding.EncodeToString(b.Bytes())
	_, _, err = veterinary.DecodeOfflinePassportPayload(oversizedPayload, pub)
	if err == nil {
		t.Fatal("expected error decoding payload exceeding 10KB, got nil")
	}
	if !strings.Contains(err.Error(), "10KB limit") && !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected 10KB limit error, got %v", err)
	}
}

func TestPassport_VaccineWithColons(t *testing.T) {
	priv, pub, err := veterinary.GeneratePassportKey()
	if err != nil {
		t.Fatalf("GeneratePassportKey failed: %v", err)
	}

	pass := samplePassport()
	vaccineNameWithColons := "Rabies: 3-Year: Booster Edition: Batch-01"
	pass.Vaccinations = []domain.VaccinationRecord{
		{
			VaccineName:    vaccineNameWithColons,
			ExpirationDate: time.Now().UTC().AddDate(3, 0, 0).Truncate(time.Second),
			Verified:       true,
		},
	}

	payload, err := veterinary.EncodeOfflinePassportPayload(pass, priv)
	if err != nil {
		t.Fatalf("EncodeOfflinePassportPayload failed: %v", err)
	}

	decoded, valid, err := veterinary.DecodeOfflinePassportPayload(payload, pub)
	if err != nil {
		t.Fatalf("DecodeOfflinePassportPayload failed: %v", err)
	}
	if !valid {
		t.Fatal("expected valid passport signature")
	}

	if len(decoded.Vaccinations) != 1 {
		t.Fatalf("expected 1 vaccination, got %d", len(decoded.Vaccinations))
	}
	if decoded.Vaccinations[0].VaccineName != vaccineNameWithColons {
		t.Errorf("expected vaccine name %q, got %q", vaccineNameWithColons, decoded.Vaccinations[0].VaccineName)
	}
	if decoded.Vaccinations[0].ExpirationDate.Unix() != pass.Vaccinations[0].ExpirationDate.Unix() {
		t.Errorf("expiration date mismatch: expected %v, got %v", pass.Vaccinations[0].ExpirationDate, decoded.Vaccinations[0].ExpirationDate)
	}
}
