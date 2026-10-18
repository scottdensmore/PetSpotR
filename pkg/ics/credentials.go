package ics

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// SignRoleCredential computes an HMAC-SHA256 cryptographic signature over a volunteer's role credential.
func SignRoleCredential(volunteerID string, cred domain.SignedCredential, secretKey []byte) (domain.SignedCredential, error) {
	if strings.TrimSpace(volunteerID) == "" {
		return domain.SignedCredential{}, errors.New("volunteerId is required")
	}
	if !cred.Role.IsValid() {
		return domain.SignedCredential{}, errors.New("invalid SAR role credential")
	}
	if len(secretKey) == 0 {
		return domain.SignedCredential{}, errors.New("signing secret key is required")
	}

	payload := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%d|%d",
		strings.TrimSpace(volunteerID),
		strings.TrimSpace(cred.CredentialID),
		cred.Role,
		strings.TrimSpace(cred.IssuedBy),
		strings.TrimSpace(cred.LicenseNumber),
		cred.IssuedAt.UTC().Unix(),
		cred.ExpiresAt.UTC().Unix(),
	)

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(payload))
	cred.SignatureHex = hex.EncodeToString(mac.Sum(nil))
	return cred, nil
}

// VerifyRoleCredential verifies the HMAC-SHA256 signature and expiration of a signed SAR role credential.
func VerifyRoleCredential(volunteerID string, cred domain.SignedCredential, secretKey []byte, now time.Time) (bool, error) {
	expected, err := SignRoleCredential(volunteerID, cred, secretKey)
	if err != nil {
		return false, err
	}

	actualSig := strings.ToLower(strings.TrimSpace(cred.SignatureHex))
	if subtle.ConstantTimeCompare([]byte(actualSig), []byte(expected.SignatureHex)) != 1 {
		return false, errors.New("invalid credential signature")
	}

	if !cred.ExpiresAt.IsZero() && now.UTC().After(cred.ExpiresAt.UTC()) {
		return false, errors.New("credential has expired")
	}

	return true, nil
}

// HasVerifiedRole checks whether a volunteer holds an active, cryptographically verified credential for requiredRole.
func HasVerifiedRole(profile domain.VolunteerProfile, requiredRole domain.SARRoleCredential, secretKey []byte, now time.Time) bool {
	if requiredRole == "" || requiredRole == domain.RoleFieldSearcher {
		return true
	}
	for _, c := range profile.Credentials {
		if c.Role == requiredRole {
			ok, err := VerifyRoleCredential(profile.VolunteerID, c, secretKey, now)
			if err == nil && ok {
				return true
			}
		}
	}
	return false
}
