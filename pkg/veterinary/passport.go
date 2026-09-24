package veterinary

import (
	"bytes"
	"compress/zlib"
	"crypto/ecdh"
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
	"time"

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

func encodePublicKeyHex(pub *ecdsa.PublicKey) (string, error) {
	if pub == nil {
		return "", errors.New("public key is required")
	}
	ecdhPub, err := pub.ECDH()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ecdhPub.Bytes()), nil
}

func decodePublicKeyHex(pubHex string) (*ecdsa.PublicKey, error) {
	pubBytes, err := hex.DecodeString(pubHex)
	if err != nil {
		return nil, err
	}
	ecdhPub, err := ecdh.P256().NewPublicKey(pubBytes)
	if err != nil {
		return nil, errors.New("invalid public key curve encoding")
	}
	raw := ecddhBytes(ecdhPub)
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(raw[1:33]),
		Y:     new(big.Int).SetBytes(raw[33:65]),
	}, nil
}

func ecddhBytes(pub *ecdh.PublicKey) []byte {
	return pub.Bytes()
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

	rBytes := make([]byte, 32)
	sBytes := make([]byte, 32)
	r.FillBytes(rBytes)
	s.FillBytes(sBytes)
	sigBytes := append(rBytes, sBytes...)
	pass.SignatureHex = hex.EncodeToString(sigBytes)

	pubHex, err := encodePublicKeyHex(&priv.PublicKey)
	if err != nil {
		return "", err
	}
	pass.PublicKeyHex = pubHex
	return pass.SignatureHex, nil
}

// VerifyPassport checks the ECDSA signature against the provided public key.
func VerifyPassport(pass *domain.VeterinaryPassport, pub *ecdsa.PublicKey) (bool, error) {
	if pub == nil {
		if pass.PublicKeyHex == "" {
			return false, errors.New("public key is required")
		}
		decodedPub, err := decodePublicKeyHex(pass.PublicKeyHex)
		if err != nil {
			return false, err
		}
		pub = decodedPub
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
	ID        string   `json:"i"`
	PetID     string   `json:"p"`
	Name      string   `json:"n"`
	Species   string   `json:"s"`
	Chip      string   `json:"c,omitempty"`
	Rabies    string   `json:"r,omitempty"`
	Weight    float64  `json:"w"`
	Allergies []string `json:"a,omitempty"`
	Vaccines  []string `json:"v,omitempty"`
	Issued    int64    `json:"t"`
	Expires   int64    `json:"x"`
	Sig       string   `json:"g"`
	PubKey    string   `json:"k,omitempty"`
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
		return nil, false, errors.New("invalid passport header: expected VP1: prefix")
	}

	rawBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(payload, "VP1:"))
	if err != nil {
		return nil, false, fmt.Errorf("base64 decode error: %w", err)
	}

	r, err := zlib.NewReader(bytes.NewReader(rawBytes))
	if err != nil {
		return nil, false, fmt.Errorf("zlib decompress error: %w", err)
	}
	defer func() {
		_ = r.Close()
	}()

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
			_, _ = fmt.Sscanf(parts[1], "%d", &exp)
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
	if pub != nil && pass.PublicKeyHex == "" {
		pubHex, err := encodePublicKeyHex(pub)
		if err == nil {
			pass.PublicKeyHex = pubHex
		}
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
