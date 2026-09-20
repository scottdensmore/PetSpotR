package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/webhook"
)

func TestSigner_GenerateSignature(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"event":"pet_lost"}`)
	secret := "supersecret"

	sig := webhook.GenerateSignature(payload, secret)
	if sig == "" {
		t.Fatal("expected signature, got empty string")
	}

	if !strings.HasPrefix(sig, "sha256=") {
		t.Fatalf("expected signature to start with 'sha256=', got %q", sig)
	}

	// Verify against crypto/hmac reference computation
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedHex := hex.EncodeToString(mac.Sum(nil))
	expectedSig := "sha256=" + expectedHex

	if sig != expectedSig {
		t.Errorf("GenerateSignature = %q, want %q", sig, expectedSig)
	}
}

func TestSigner_VerifySignature(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"event":"pet_lost","petId":"dog-123"}`)
	secret := "supersecret"

	sig := webhook.GenerateSignature(payload, secret)

	// Valid signature should verify
	if !webhook.VerifySignature(payload, secret, sig) {
		t.Error("expected valid signature to verify successfully")
	}

	// Modified payload should fail verification
	tamperedPayload := []byte(`{"event":"pet_lost","petId":"dog-999"}`)
	if webhook.VerifySignature(tamperedPayload, secret, sig) {
		t.Error("expected tampered payload to fail verification")
	}

	// Wrong secret should fail verification
	if webhook.VerifySignature(payload, "wrongsecret", sig) {
		t.Error("expected wrong secret to fail verification")
	}

	// Invalid signature strings should fail
	invalidSigs := []string{
		"",
		"sha256=",
		"sha256=invalidhex",
		"sha1=123456",
		"randomtext",
	}
	for _, badSig := range invalidSigs {
		if webhook.VerifySignature(payload, secret, badSig) {
			t.Errorf("expected invalid signature %q to fail verification", badSig)
		}
	}
}

func TestSigner_HeaderConstant(t *testing.T) {
	t.Parallel()

	if webhook.SignatureHeader != "X-PetSpotR-Signature" {
		t.Errorf("SignatureHeader = %q, want %q", webhook.SignatureHeader, "X-PetSpotR-Signature")
	}
}
