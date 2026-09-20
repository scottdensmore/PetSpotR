package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SignatureHeader is the standard HTTP header used to convey the HMAC-SHA256 signature.
const SignatureHeader = "X-PetSpotR-Signature"

// HeaderSignature is an alias for SignatureHeader.
const HeaderSignature = SignatureHeader

// GenerateSignature computes an HMAC-SHA256 hex digest over the payload using the provided secret,
// returning the signature formatted as "sha256=<hex_encoded_hash>".
func GenerateSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature validates a signature against the payload and secret using constant-time comparison.
func VerifySignature(payload []byte, secret, signature string) bool {
	expected := GenerateSignature(payload, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}
