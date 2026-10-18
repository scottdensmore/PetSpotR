package custody

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

const GenesisPrevHash = "0000000000000000000000000000000000000000000000000000000000000000"

func ComputeHopHashes(chainID, petID string, hop domain.CustodyTransferHop, secretKey []byte) (entryHashHex string, hmacSigHex string) {
	canonical := fmt.Sprintf(
		"%s|%s|%d|%s|%s|%s|%s|%s|%s|%s|%s",
		chainID,
		petID,
		hop.HopIndex,
		hop.Stage,
		hop.FromCustodian,
		hop.ToCustodian,
		hop.ToRole,
		hop.LocationLabel,
		hop.ConditionNotes,
		hop.TransferredAt.UTC().Format(time.RFC3339),
		hop.PrevHashHex,
	)
	sum := sha256.Sum256([]byte(canonical))
	entryHashHex = hex.EncodeToString(sum[:])

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(entryHashHex))
	hmacSigHex = hex.EncodeToString(mac.Sum(nil))

	return entryHashHex, hmacSigHex
}

func AppendCustodyHop(chain domain.PetCustodyChain, nextHop domain.CustodyTransferHop, secretKey []byte) (domain.PetCustodyChain, error) {
	if len(secretKey) == 0 {
		return chain, errors.New("secretKey is required")
	}
	if strings.TrimSpace(chain.ChainID) == "" {
		return chain, errors.New("chainId is required")
	}
	if strings.TrimSpace(chain.PetID) == "" {
		return chain, errors.New("petId is required")
	}

	nextHop.HopIndex = len(chain.Hops) + 1
	if nextHop.TransferredAt.IsZero() {
		nextHop.TransferredAt = time.Now().UTC()
	}

	if len(chain.Hops) == 0 {
		nextHop.PrevHashHex = GenesisPrevHash
	} else {
		nextHop.PrevHashHex = chain.Hops[len(chain.Hops)-1].EntryHashHex
	}

	if err := nextHop.Validate(); err != nil {
		return chain, err
	}

	nextHop.EntryHashHex, nextHop.HmacSignatureHex = ComputeHopHashes(chain.ChainID, chain.PetID, nextHop, secretKey)

	newHops := make([]domain.CustodyTransferHop, len(chain.Hops), len(chain.Hops)+1)
	copy(newHops, chain.Hops)
	newHops = append(newHops, nextHop)
	chain.Hops = newHops

	chain.CurrentStage = nextHop.Stage
	chain.CurrentCustodian = nextHop.ToCustodian
	chain.CurrentLocation = nextHop.LocationLabel
	chain.LatestEntryHashHex = nextHop.EntryHashHex
	chain.ChainVerifiedIntact = true
	chain.UpdatedAt = nextHop.TransferredAt

	return chain, nil
}

func VerifyCustodyChainIntegrity(chain domain.PetCustodyChain, secretKey []byte) (bool, int, string) {
	if len(chain.Hops) == 0 {
		return false, 0, "Custody chain has no recorded transfer hops"
	}

	expectedPrev := GenesisPrevHash
	for i, hop := range chain.Hops {
		if hop.HopIndex != i+1 {
			return false, i + 1, fmt.Sprintf("Hop #%d has out-of-sequence index %d", i+1, hop.HopIndex)
		}
		if hop.PrevHashHex != expectedPrev {
			return false, hop.HopIndex, fmt.Sprintf("Hop #%d prevHash mismatch: expected %s, got %s", hop.HopIndex, expectedPrev, hop.PrevHashHex)
		}

		expectedEntryHash, expectedHMAC := ComputeHopHashes(chain.ChainID, chain.PetID, hop, secretKey)
		if subtle.ConstantTimeCompare([]byte(hop.EntryHashHex), []byte(expectedEntryHash)) != 1 {
			return false, hop.HopIndex, fmt.Sprintf("Hop #%d SHA-256 entryHash tamper detected", hop.HopIndex)
		}
		if subtle.ConstantTimeCompare([]byte(hop.HmacSignatureHex), []byte(expectedHMAC)) != 1 {
			return false, hop.HopIndex, fmt.Sprintf("Hop #%d HMAC-SHA256 signature verification failed", hop.HopIndex)
		}

		expectedPrev = hop.EntryHashHex
	}

	if chain.LatestEntryHashHex != expectedPrev {
		return false, len(chain.Hops), "Chain head latestEntryHashHex does not match final hop digest"
	}

	return true, len(chain.Hops), fmt.Sprintf("SHA-256 hash chain & HMAC signatures verified intact (%d hops)", len(chain.Hops))
}
