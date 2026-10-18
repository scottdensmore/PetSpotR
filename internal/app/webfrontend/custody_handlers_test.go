package webfrontend_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestCustodyHandlers_SeedingCreateTransferPinGateAndVerify(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Subscribe to SSE "custody" topic and Mesh signaling hub
	custodySSECh, unsubSSE := server.ReunionHub().Subscribe("custody")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("custody", "custody-escrow-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /custody-ledger should render HTML and seed default custody chains
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/custody-ledger", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /custody-ledger, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Cooper") {
		t.Errorf("expected /custody-ledger HTML to contain seeded Cooper custody chain")
	}

	// 2. GET /api/v1/custody/chains should return the 2 seeded custody chains
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/custody/chains", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/custody/chains, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Chains []domain.PetCustodyChain `json:"chains"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal custody chains response: %v", err)
	}
	if len(listResp.Chains) != 2 {
		t.Fatalf("expected 2 seeded custody chains, got %d", len(listResp.Chains))
	}

	chainsByID := make(map[string]domain.PetCustodyChain)
	for _, c := range listResp.Chains {
		chainsByID[c.ChainID] = c
	}

	c101, ok := chainsByID["chain-custody-101"]
	if !ok {
		t.Fatalf("expected seeded chain-custody-101")
	}
	if c101.PetName != "Cooper" || c101.CurrentStage != domain.CustodyStageOwnerEscrow {
		t.Errorf("expected chain-custody-101 Cooper at OWNER_REUNIFICATION_ESCROW, got %s / %s", c101.PetName, c101.CurrentStage)
	}
	if len(c101.Hops) != 3 {
		t.Errorf("expected chain-custody-101 to have 3 seeded hops, got %d", len(c101.Hops))
	}
	if len(c101.HandoverPinCode) != 6 {
		t.Errorf("expected 6-digit HandoverPinCode on chain-custody-101, got %q", c101.HandoverPinCode)
	}
	if !c101.ChainVerifiedIntact {
		t.Errorf("expected chain-custody-101 ChainVerifiedIntact = true")
	}

	c102, ok := chainsByID["chain-custody-102"]
	if !ok {
		t.Fatalf("expected seeded chain-custody-102")
	}
	if c102.PetName != "Milo" || c102.CurrentStage != domain.CustodyStageVetERHold {
		t.Errorf("expected chain-custody-102 Milo at VET_ER_HOLD, got %s / %s", c102.PetName, c102.CurrentStage)
	}
	if len(c102.Hops) != 2 {
		t.Errorf("expected chain-custody-102 to have 2 seeded hops, got %d", len(c102.Hops))
	}

	// 3. POST /api/v1/custody/chains with invalid payload returns 400 Bad Request
	invalidBody, _ := json.Marshal(map[string]any{
		"petId":   "",
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/custody/chains", bytes.NewReader(invalidBody))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid custody chain, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/custody/chains creates a new custody chain with genesis hop
	createPayload, _ := json.Marshal(map[string]any{
		"petId":          "pet-custody-daisy",
		"petName":        "Daisy",
		"species":        "Dog",
		"microchipId":    "985141007766554",
		"initialStage":   "FINDER_INTAKE",
		"fromCustodian":  "Green Lake East Meadow",
		"toCustodian":    "Alex Mercer (Finder)",
		"toRole":         "COMMUNITY_FINDER",
		"locationLabel":  "Green Lake Boathouse Check-In",
		"conditionNotes": "Friendly Golden Retriever, intact collar",
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/custody/chains", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/custody/chains, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createdChain domain.PetCustodyChain
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createdChain); err != nil {
		t.Fatalf("failed to unmarshal created custody chain: %v", err)
	}
	if !strings.HasPrefix(createdChain.ChainID, "chain-custody-") {
		t.Errorf("expected generated chainId starting with 'chain-custody-', got %q", createdChain.ChainID)
	}
	if len(createdChain.HandoverPinCode) != 6 {
		t.Errorf("expected 6-digit HandoverPinCode, got %q", createdChain.HandoverPinCode)
	}
	if len(createdChain.Hops) != 1 {
		t.Fatalf("expected 1 genesis hop on created chain, got %d", len(createdChain.Hops))
	}
	genesisHash := createdChain.Hops[0].EntryHashHex
	if len(genesisHash) != 64 {
		t.Errorf("expected 64-char SHA-256 genesis entryHashHex, got %q", genesisHash)
	}

	// Verify SSE event "custody_chain_updated"
	select {
	case evt := <-custodySSECh:
		if evt.Type != domain.ReunionEventType("custody_chain_updated") {
			t.Errorf("expected custody_chain_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for custody_chain_updated SSE event")
	}

	// 5. POST /api/v1/custody/chains/{id}/transfer appends hop #2
	transferPayload, _ := json.Marshal(map[string]any{
		"stage":          "OWNER_REUNIFICATION_ESCROW",
		"toCustodian":    "Seattle Animal Shelter Reunion Desk",
		"toRole":         "ESCROW_CUSTODIAN",
		"locationLabel":  "2061 15th Ave W, Seattle, WA",
		"conditionNotes": "Identity verified, ready for owner PIN release",
	})
	recTransfer := httptest.NewRecorder()
	reqTransfer := httptest.NewRequest(http.MethodPost, "/api/v1/custody/chains/"+createdChain.ChainID+"/transfer", bytes.NewReader(transferPayload))
	reqTransfer.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTransfer, reqTransfer)
	if recTransfer.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST transfer hop, got %d: %s", recTransfer.Code, recTransfer.Body.String())
	}

	var transferredChain domain.PetCustodyChain
	if err := json.Unmarshal(recTransfer.Body.Bytes(), &transferredChain); err != nil {
		t.Fatalf("failed to unmarshal transferred chain: %v", err)
	}
	if len(transferredChain.Hops) != 2 {
		t.Fatalf("expected 2 hops after transfer, got %d", len(transferredChain.Hops))
	}
	if transferredChain.Hops[1].PrevHashHex != genesisHash {
		t.Errorf("expected hop #2 prevHashHex %q to match genesis entryHashHex %q", transferredChain.Hops[1].PrevHashHex, genesisHash)
	}
	if transferredChain.CurrentStage != domain.CustodyStageOwnerEscrow {
		t.Errorf("expected CurrentStage OWNER_REUNIFICATION_ESCROW, got %s", transferredChain.CurrentStage)
	}

	// 6. POST /api/v1/custody/chains/{id}/reunite with WRONG PIN returns 403 Forbidden
	wrongPinPayload, _ := json.Marshal(map[string]any{
		"handoverPinCode":   "000000",
		"verifiedOwnerName": "Jordan Taylor",
		"ownershipProof":    "MICROCHIP_SCAN_MATCH",
		"locationLabel":     "Seattle Animal Shelter Reunion Desk",
		"waiverAccepted":    true,
	})
	recWrongPin := httptest.NewRecorder()
	reqWrongPin := httptest.NewRequest(http.MethodPost, "/api/v1/custody/chains/"+createdChain.ChainID+"/reunite", bytes.NewReader(wrongPinPayload))
	reqWrongPin.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recWrongPin, reqWrongPin)
	if recWrongPin.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for wrong handover PIN, got %d: %s", recWrongPin.Code, recWrongPin.Body.String())
	}
	if !strings.Contains(recWrongPin.Body.String(), "Handover PIN rejected") {
		t.Errorf("expected error message containing 'Handover PIN rejected', got %s", recWrongPin.Body.String())
	}

	// 7. POST /api/v1/custody/chains/{id}/reunite with VALID PIN completes reunion
	validPinPayload, _ := json.Marshal(map[string]any{
		"handoverPinCode":   createdChain.HandoverPinCode,
		"verifiedOwnerName": "Jordan Taylor",
		"ownershipProof":    "MICROCHIP_SCAN_MATCH",
		"locationLabel":     "Seattle Animal Shelter Reunion Desk",
		"waiverAccepted":    true,
	})
	recReunite := httptest.NewRecorder()
	reqReunite := httptest.NewRequest(http.MethodPost, "/api/v1/custody/chains/"+createdChain.ChainID+"/reunite", bytes.NewReader(validPinPayload))
	reqReunite.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recReunite, reqReunite)
	if recReunite.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid PIN reunion, got %d: %s", recReunite.Code, recReunite.Body.String())
	}

	var reunitedChain domain.PetCustodyChain
	if err := json.Unmarshal(recReunite.Body.Bytes(), &reunitedChain); err != nil {
		t.Fatalf("failed to unmarshal reunited chain: %v", err)
	}
	if reunitedChain.CurrentStage != domain.CustodyStageReunited {
		t.Errorf("expected CurrentStage REUNITED_RELEASED, got %s", reunitedChain.CurrentStage)
	}
	if !reunitedChain.WaiverSigned {
		t.Errorf("expected WaiverSigned = true")
	}
	if !strings.HasPrefix(reunitedChain.ReceiptCertificateID, "CERT-REUNION-") {
		t.Errorf("expected ReceiptCertificateID starting with 'CERT-REUNION-', got %q", reunitedChain.ReceiptCertificateID)
	}
	if len(reunitedChain.Hops) != 3 {
		t.Fatalf("expected 3 hops after reunion handover, got %d", len(reunitedChain.Hops))
	}

	// 8. Attempting transfer on already reunited chain returns 400 or 409
	recPostReunionTransfer := httptest.NewRecorder()
	reqPostReunionTransfer := httptest.NewRequest(http.MethodPost, "/api/v1/custody/chains/"+createdChain.ChainID+"/transfer", bytes.NewReader(transferPayload))
	reqPostReunionTransfer.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recPostReunionTransfer, reqPostReunionTransfer)
	if recPostReunionTransfer.Code != http.StatusBadRequest && recPostReunionTransfer.Code != http.StatusConflict {
		t.Errorf("expected 400 or 409 when transferring already reunited chain, got %d", recPostReunionTransfer.Code)
	}

	// 9. GET /api/v1/custody/chains/{id}/verify verifies SHA-256 chain integrity
	recVerify := httptest.NewRecorder()
	reqVerify := httptest.NewRequest(http.MethodGet, "/api/v1/custody/chains/"+createdChain.ChainID+"/verify", nil)
	server.ServeHTTP(recVerify, reqVerify)
	if recVerify.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET verify, got %d: %s", recVerify.Code, recVerify.Body.String())
	}

	var verifyResp struct {
		ChainID            string `json:"chainId"`
		VerifiedIntact     bool   `json:"verifiedIntact"`
		HopCount           int    `json:"hopCount"`
		LatestEntryHashHex string `json:"latestEntryHashHex"`
		Message            string `json:"message"`
	}
	if err := json.Unmarshal(recVerify.Body.Bytes(), &verifyResp); err != nil {
		t.Fatalf("failed to unmarshal verify response: %v", err)
	}
	if !verifyResp.VerifiedIntact || verifyResp.HopCount != 3 {
		t.Errorf("expected verifiedIntact=true and hopCount=3, got %+v", verifyResp)
	}
	if verifyResp.LatestEntryHashHex != reunitedChain.LatestEntryHashHex {
		t.Errorf("expected latestEntryHashHex %q, got %q", reunitedChain.LatestEntryHashHex, verifyResp.LatestEntryHashHex)
	}
	if !strings.Contains(verifyResp.Message, "SHA-256 hash chain & HMAC signatures verified intact (3 hops)") {
		t.Errorf("unexpected verify message: %q", verifyResp.Message)
	}

	// 10. GET /api/v1/custody/chains/unknown-id/verify returns 404 Not Found
	recVerify404 := httptest.NewRecorder()
	reqVerify404 := httptest.NewRequest(http.MethodGet, "/api/v1/custody/chains/unknown-chain-id/verify", nil)
	server.ServeHTTP(recVerify404, reqVerify404)
	if recVerify404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown custody chain verify, got %d", recVerify404.Code)
	}
}

func TestCustodyHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/custody/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/custody/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/custody/stream, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Errorf("expected text/event-stream Content-Type, got %q", resp.Header.Get("Content-Type"))
	}

	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed reading initial SSE comment: %v", err)
	}
	if !strings.Contains(line, ": connected") {
		t.Errorf("expected ': connected' prologue, got %q", line)
	}
}
