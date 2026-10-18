# Automated Pet Custody Chain, Escrow & Cryptographic Reunion Handover Ledger — Design Specification

- **Milestone:** 13.1
- **Date:** 2026-10-17
- **Status:** Approved
- **Author:** Antigravity & User

---

## 1. Problem Statement & Objectives

When a lost pet is recovered in the field—whether by a community finder, a credentialed SAR responder, a municipal disaster evacuation team, or an emergency veterinary clinic—safe and legally auditable physical custody handover back to the verified owner requires:
1. **Tamper-Evident SHA-256 Hash-Chained Custody Ledger:** Every physical transfer of a recovered animal (`FINDER_INTAKE` -> `FIELD_SAR_ESCORT` -> `VET_ER_HOLD` / `SHELTER_SAFEKEEPING` -> `OWNER_REUNIFICATION_ESCROW` -> `REUNITED_RELEASED`) must be recorded as an immutable link in a SHA-256 hash chain (`PrevHashHex` -> `EntryHashHex`), preventing post-hoc tampering or disputed handovers.
2. **Dual-Party Cryptographic Handover Verification (HMAC-SHA256 + 6-Digit Handover PIN):** Final release to an owner requires verifying both a deterministic 6-digit Handover Escrow PIN (generated via HMAC-SHA256 over the chain ID, pet ID, and secret key) and identity/microchip proof (`MICROCHIP_SCAN_MATCH`, `VET_RECORD_DEED`, `PHOTO_BIOMETRIC_MATCH`, `GOVERNMENT_ID_AFFIDAVIT`). Wrong PIN attempts must be rejected (`403 Forbidden`) without mutating the custody state.
3. **Printable Custody Receipt & Liability Waiver Certificate:** Generates a tamper-evident, printable custody handover receipt & liability release certificate displaying the full SHA-256 hash chain, verified microchip/identity proof, releasing & receiving signatures, and Merkle/chain root digest.
4. **Dedicated Pet Custody Chain & Cryptographic Handover Cockpit (`/custody-ledger`):** An interactive WCAG AAA (`> 7:1` contrast) cockpit with live chain integrity verification (`CHAIN_VERIFIED_INTACT` vs `TAMPER_DETECTED`), custody transfer appendage, Dual-Party PIN Escrow Handover execution, and Printable Custody Waiver Certificate viewer.

---

## 2. Domain Models & State Store Collections

### 2.1 State Store Collections (`pkg/store/store.go`)
- `CollectionPetCustodyChains = "pet_custody_chains"`

### 2.2 Domain Models (`pkg/domain/custody_ledger.go`)

```go
package domain

import "time"

type CustodyStage string

const (
	CustodyStageFinderIntake   CustodyStage = "FINDER_INTAKE"
	CustodyStageFieldSAREscort CustodyStage = "FIELD_SAR_ESCORT"
	CustodyStageVetERHold      CustodyStage = "VET_ER_HOLD"
	CustodyStageShelterHold    CustodyStage = "SHELTER_SAFEKEEPING"
	CustodyStageOwnerEscrow    CustodyStage = "OWNER_REUNIFICATION_ESCROW"
	CustodyStageReunited       CustodyStage = "REUNITED_RELEASED"
)

type OwnershipProofMethod string

const (
	ProofMicrochipMatch  OwnershipProofMethod = "MICROCHIP_SCAN_MATCH"
	ProofVetRecordDeed   OwnershipProofMethod = "VET_RECORD_DEED"
	ProofPhotoBiometric  OwnershipProofMethod = "PHOTO_BIOMETRIC_MATCH"
	ProofGovIDAffidavit  OwnershipProofMethod = "GOVERNMENT_ID_AFFIDAVIT"
)

type CustodyTransferHop struct {
	HopIndex         int          `json:"hopIndex"`
	Stage            CustodyStage `json:"stage"`
	FromCustodian    string       `json:"fromCustodian"`
	ToCustodian      string       `json:"toCustodian"`
	ToRole           string       `json:"toRole"`
	LocationLabel    string       `json:"locationLabel"`
	ConditionNotes   string       `json:"conditionNotes"`
	TransferredAt    time.Time    `json:"transferredAt"`
	PrevHashHex      string       `json:"prevHashHex"`
	EntryHashHex     string       `json:"entryHashHex"`
	HmacSignatureHex string       `json:"hmacSignatureHex"`
}

type PetCustodyChain struct {
	ChainID               string               `json:"chainId"`
	PetID                 string               `json:"petId"`
	PetName               string               `json:"petName"`
	Species               string               `json:"species"`
	MicrochipID           string               `json:"microchipId,omitempty"`
	CurrentStage          CustodyStage         `json:"currentStage"`
	CurrentCustodian      string               `json:"currentCustodian"`
	CurrentLocation       string               `json:"currentLocation"`
	HandoverPinCode       string               `json:"handoverPinCode"` // 6-digit deterministic escrow PIN
	VerifiedOwnerName     string               `json:"verifiedOwnerName,omitempty"`
	OwnershipProof        OwnershipProofMethod `json:"ownershipProof,omitempty"`
	WaiverSigned          bool                 `json:"waiverSigned"`
	ReceiptCertificateID  string               `json:"receiptCertificateId,omitempty"`
	ChainVerifiedIntact   bool                 `json:"chainVerifiedIntact"`
	LatestEntryHashHex    string               `json:"latestEntryHashHex"`
	Hops                  []CustodyTransferHop `json:"hops"`
	CreatedAt             time.Time            `json:"createdAt"`
	UpdatedAt             time.Time            `json:"updatedAt"`
}
```

---

## 3. Pure-Go SHA-256 Hash-Chain Ledger & Dual-Party Escrow Engine (`pkg/custody`)

### 3.1 Canonical SHA-256 Hash Chain & HMAC Signer (`pkg/custody/ledger.go`)
- `GenesisPrevHash = "0000000000000000000000000000000000000000000000000000000000000000"`
- `ComputeHopHashes(chainID, petID string, hop domain.CustodyTransferHop, secretKey []byte) (entryHashHex string, hmacSigHex string)`:
  - Canonical payload: `fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s|%s|%s|%s|%s", chainID, petID, hop.HopIndex, hop.Stage, hop.FromCustodian, hop.ToCustodian, hop.ToRole, hop.LocationLabel, hop.ConditionNotes, hop.TransferredAt.UTC().Format(time.RFC3339), hop.PrevHashHex)`
  - `entryHashHex` = hex-encoded SHA-256 digest of the canonical payload.
  - `hmacSigHex` = hex-encoded HMAC-SHA256 signature of `entryHashHex` using `secretKey`.
- `AppendCustodyHop(chain domain.PetCustodyChain, nextHop domain.CustodyTransferHop, secretKey []byte) (domain.PetCustodyChain, error)`:
  - Sets `nextHop.HopIndex = len(chain.Hops) + 1`.
  - Sets `nextHop.PrevHashHex = GenesisPrevHash` if first hop, else `chain.Hops[len(chain.Hops)-1].EntryHashHex`.
  - Computes `nextHop.EntryHashHex` and `nextHop.HmacSignatureHex`.
  - Appends `nextHop` to `chain.Hops`, updates `chain.CurrentStage`, `chain.CurrentCustodian`, `chain.CurrentLocation`, `chain.LatestEntryHashHex`, and `chain.ChainVerifiedIntact = true`.
- `VerifyCustodyChainIntegrity(chain domain.PetCustodyChain, secretKey []byte) (bool, int, string)`:
  - Verifies every hop in `chain.Hops` sequentially:
    1. `hop.HopIndex == i + 1`
    2. `hop.PrevHashHex == expectedPrevHash`
    3. Recomputed `entryHashHex` matches `hop.EntryHashHex`
    4. Recomputed `hmacSigHex` matches `hop.HmacSignatureHex` via `subtle.ConstantTimeCompare`
  - Returns `(true, len(chain.Hops), "SHA-256 hash chain & HMAC signatures verified intact")` or `(false, failedHopIdx, reason)`.

### 3.2 Deterministic 6-Digit Handover PIN & Reunion Release (`pkg/custody/escrow.go`)
- `GenerateHandoverPIN(chainID, petID string, secretKey []byte) string`:
  - Computes HMAC-SHA256 over `"PIN|" + chainID + "|" + petID` and maps the first 4 bytes (`binary.BigEndian.Uint32`) modulo `900000 + 100000` into a deterministic 6-digit numeric string (e.g. `"482915"`).
- `VerifyAndCompleteReunionHandover(chain domain.PetCustodyChain, providedPIN, ownerName, locationLabel string, proofMethod domain.OwnershipProofMethod, waiverAccepted bool, secretKey []byte, now time.Time) (domain.PetCustodyChain, error)`:
  - Verifies `subtle.ConstantTimeCompare([]byte(strings.TrimSpace(providedPIN)), []byte(chain.HandoverPinCode)) == 1` (returns `ErrInvalidHandoverPIN` if mismatch).
  - Verifies `waiverAccepted == true` and `proofMethod.IsValid()`.
  - Appends a final `CustodyStageReunited` hop (`FromCustodian: chain.CurrentCustodian`, `ToCustodian: ownerName`, `ToRole: "VERIFIED_PET_OWNER"`), sets `chain.VerifiedOwnerName = ownerName`, `chain.OwnershipProof = proofMethod`, `chain.WaiverSigned = true`, and generates `chain.ReceiptCertificateID = fmt.Sprintf("CERT-REUNION-%s-%s", strings.ToUpper(chain.ChainID[len(chain.ChainID)-4:]), chain.LatestEntryHashHex[:8])`.

---

## 4. REST API & Custody Ledger Cockpit (`internal/app/webfrontend/custody_handlers.go`)

1. `GET /custody-ledger` — Renders the Automated Pet Custody Chain, Escrow & Cryptographic Reunion Handover Ledger Cockpit (`templates/custody_ledger.html`).
2. `GET /api/v1/custody/chains` — Lists all custody chains (seeds 2 default custody chains on first lookup: `chain-custody-101` in `OWNER_REUNIFICATION_ESCROW` ready for final handover, and `chain-custody-102` in `VET_ER_HOLD`).
3. `POST /api/v1/custody/chains` — Initiates a new SHA-256 hash-chained custody record (`FINDER_INTAKE` or `FIELD_SAR_ESCORT` genesis hop) with a deterministic 6-digit `HandoverPinCode`.
4. `POST /api/v1/custody/chains/{id}/transfer` — Appends a new custody transfer hop (`FIELD_SAR_ESCORT`, `VET_ER_HOLD`, `SHELTER_SAFEKEEPING`, `OWNER_REUNIFICATION_ESCROW`), recomputing the SHA-256 hash chain and broadcasting `custody_chain_updated`.
5. `POST /api/v1/custody/chains/{id}/reunite` — Executes Dual-Party Cryptographic Reunion Handover (`403 Forbidden` if `handoverPinCode` does not match), appends the `REUNITED_RELEASED` hop, issues the `ReceiptCertificateID`, and broadcasts `custody_chain_updated`.
6. `GET /api/v1/custody/chains/{id}/verify` — Runs `custody.VerifyCustodyChainIntegrity` and returns JSON `{"chainId": ..., "verifiedIntact": true, "hopCount": ..., "latestEntryHashHex": ..., "message": ...}`.
7. `GET /api/v1/custody/stream` — Public SSE stream on topic `"custody"` for real-time ledger updates.
