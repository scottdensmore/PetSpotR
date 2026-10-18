package custody

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

var ErrInvalidHandoverPIN = errors.New("invalid 6-digit handover escrow PIN")

func GenerateHandoverPIN(chainID, petID string, secretKey []byte) string {
	mac := hmac.New(sha256.New, secretKey)
	_, _ = fmt.Fprintf(mac, "PIN|%s|%s", strings.TrimSpace(chainID), strings.TrimSpace(petID))
	sum := mac.Sum(nil)
	val := binary.BigEndian.Uint32(sum[:4])
	pin := 100000 + (val % 900000)
	return fmt.Sprintf("%06d", pin)
}

func VerifyAndCompleteReunionHandover(
	chain domain.PetCustodyChain,
	providedPIN, ownerName, locationLabel string,
	proofMethod domain.OwnershipProofMethod,
	waiverAccepted bool,
	secretKey []byte,
	now time.Time,
) (domain.PetCustodyChain, error) {
	providedPIN = strings.TrimSpace(providedPIN)
	ownerName = strings.TrimSpace(ownerName)
	locationLabel = strings.TrimSpace(locationLabel)

	if chain.HandoverPinCode == "" || subtle.ConstantTimeCompare([]byte(providedPIN), []byte(chain.HandoverPinCode)) != 1 {
		return chain, ErrInvalidHandoverPIN
	}
	if ownerName == "" {
		return chain, errors.New("verifiedOwnerName is required")
	}
	if !proofMethod.IsValid() {
		return chain, errors.New("valid ownershipProof method is required")
	}
	if !waiverAccepted {
		return chain, errors.New("liability waiver must be signed/accepted before release")
	}
	if locationLabel == "" {
		locationLabel = chain.CurrentLocation
	}

	reunionHop := domain.CustodyTransferHop{
		Stage:          domain.CustodyStageReunited,
		FromCustodian:  chain.CurrentCustodian,
		ToCustodian:    ownerName,
		ToRole:         "VERIFIED_PET_OWNER",
		LocationLabel:  locationLabel,
		ConditionNotes: fmt.Sprintf("Dual-party cryptographic reunion completed via %s; liability waiver signed", proofMethod),
		TransferredAt:  now.UTC(),
	}

	updated, err := AppendCustodyHop(chain, reunionHop, secretKey)
	if err != nil {
		return chain, err
	}

	updated.VerifiedOwnerName = ownerName
	updated.OwnershipProof = proofMethod
	updated.WaiverSigned = true

	suffix := strings.ToUpper(updated.ChainID)
	if len(suffix) > 4 {
		suffix = suffix[len(suffix)-4:]
	}
	hashPrefix := strings.ToUpper(updated.LatestEntryHashHex)
	if len(hashPrefix) > 8 {
		hashPrefix = hashPrefix[:8]
	}
	updated.ReceiptCertificateID = fmt.Sprintf("CERT-REUNION-%s-%s", suffix, hashPrefix)

	return updated, nil
}
