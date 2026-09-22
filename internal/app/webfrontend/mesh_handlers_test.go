package webfrontend

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type sseEventFrame struct {
	id    string
	event string
	data  string
}

func readSSEFrameWithTimeout(reader *bufio.Reader, timeout time.Duration) (sseEventFrame, error) {
	var frame sseEventFrame
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return frame, errors.New("timed out waiting for SSE frame")
		}
		line, err := readLineWithTimeout(reader, remaining)
		if err != nil {
			return frame, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(trimmed, ":") {
			// Comment line (e.g. : connected or : ping)
			continue
		}
		if trimmed == "" {
			if frame.event != "" || frame.data != "" {
				return frame, nil
			}
			continue
		}
		if strings.HasPrefix(trimmed, "id: ") {
			frame.id = strings.TrimPrefix(trimmed, "id: ")
		} else if strings.HasPrefix(trimmed, "event: ") {
			frame.event = strings.TrimPrefix(trimmed, "event: ")
		} else if strings.HasPrefix(trimmed, "data: ") {
			frame.data = strings.TrimPrefix(trimmed, "data: ")
		}
	}
}

func TestMeshSignaling_PeerJoinAndRelay(t *testing.T) {
	server := NewServer()
	ts := httptest.NewServer(server)
	defer ts.Close()
	defer server.Close()

	ctxA, cancelA := context.WithCancel(context.Background())
	defer cancelA()

	// 1. Client A subscribes to SSE stream
	reqA, err := http.NewRequestWithContext(ctxA, http.MethodGet, ts.URL+"/api/v1/mesh/signal/events?searchPartyId=party-mesh-1&nodeId=node-alpha", nil)
	if err != nil {
		t.Fatalf("failed to create reqA: %v", err)
	}
	respA, err := ts.Client().Do(reqA)
	if err != nil {
		t.Fatalf("reqA failed: %v", err)
	}
	defer respA.Body.Close()

	if respA.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(respA.Body)
		t.Fatalf("statusA = %d, want %d; body = %s", respA.StatusCode, http.StatusOK, string(body))
	}
	if ct := respA.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	readerA := bufio.NewReader(respA.Body)
	connLineA, err := readLineWithTimeout(readerA, 2*time.Second)
	if err != nil || strings.TrimRight(connLineA, "\r\n") != ": connected" {
		t.Fatalf("expected initial ': connected', got %q (%v)", connLineA, err)
	}

	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()

	// 2. Client B subscribes to SSE stream
	reqB, err := http.NewRequestWithContext(ctxB, http.MethodGet, ts.URL+"/api/v1/mesh/signal/events?searchPartyId=party-mesh-1&nodeId=node-bravo", nil)
	if err != nil {
		t.Fatalf("failed to create reqB: %v", err)
	}
	respB, err := ts.Client().Do(reqB)
	if err != nil {
		t.Fatalf("reqB failed: %v", err)
	}
	defer respB.Body.Close()

	if respB.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(respB.Body)
		t.Fatalf("statusB = %d, want %d; body = %s", respB.StatusCode, http.StatusOK, string(body))
	}

	readerB := bufio.NewReader(respB.Body)
	connLineB, err := readLineWithTimeout(readerB, 2*time.Second)
	if err != nil || strings.TrimRight(connLineB, "\r\n") != ": connected" {
		t.Fatalf("expected initial ': connected', got %q (%v)", connLineB, err)
	}

	// 3. Client A should receive PEER_JOINED for node-bravo
	joinFrame, err := readSSEFrameWithTimeout(readerA, 3*time.Second)
	if err != nil {
		t.Fatalf("client A failed to receive PEER_JOINED: %v", err)
	}
	if !strings.EqualFold(joinFrame.event, "PEER_JOINED") {
		t.Errorf("event = %q, want PEER_JOINED", joinFrame.event)
	}
	if !strings.Contains(joinFrame.data, "node-bravo") {
		t.Errorf("joinFrame.data = %q, want containing node-bravo", joinFrame.data)
	}

	// 4. Client A sends WebRTC OFFER targeting node-bravo
	offerMsg := map[string]any{
		"type":          "OFFER",
		"searchPartyId": "party-mesh-1",
		"senderNodeId":  "node-alpha",
		"targetNodeId":  "node-bravo",
		"sdp":           "v=0\r\no=alice 123 456 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n",
	}
	offerBytes, _ := json.Marshal(offerMsg)
	postReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/mesh/signal/message", bytes.NewReader(offerBytes))
	if err != nil {
		t.Fatalf("failed to create postReq: %v", err)
	}
	postReq.Header.Set("Content-Type", "application/json")
	postResp, err := ts.Client().Do(postReq)
	if err != nil {
		t.Fatalf("post message failed: %v", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(postResp.Body)
		t.Fatalf("postResp status = %d, want %d; body = %s", postResp.StatusCode, http.StatusOK, string(b))
	}

	// 5. Client B should receive the framed OFFER event
	offerFrame, err := readSSEFrameWithTimeout(readerB, 3*time.Second)
	if err != nil {
		t.Fatalf("client B failed to receive OFFER: %v", err)
	}
	if !strings.EqualFold(offerFrame.event, "OFFER") {
		t.Errorf("event = %q, want OFFER", offerFrame.event)
	}
	if !strings.Contains(offerFrame.data, "node-alpha") || !strings.Contains(offerFrame.data, "v=0") {
		t.Errorf("offerFrame.data = %q, want containing node-alpha and sdp", offerFrame.data)
	}

	// 6. Disconnect Client B -> Client A should receive PEER_LEFT for node-bravo
	cancelB()
	leftFrame, err := readSSEFrameWithTimeout(readerA, 3*time.Second)
	if err != nil {
		t.Fatalf("client A failed to receive PEER_LEFT: %v", err)
	}
	if !strings.EqualFold(leftFrame.event, "PEER_LEFT") {
		t.Errorf("event = %q, want PEER_LEFT", leftFrame.event)
	}
	if !strings.Contains(leftFrame.data, "node-bravo") {
		t.Errorf("leftFrame.data = %q, want containing node-bravo", leftFrame.data)
	}
}

func TestMeshUplinkSync_Success(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := NewServerWithOptions(memStore, ServerOptions{AllowPrivilegedMutations: true})
	ts := httptest.NewServer(server)
	defer ts.Close()
	defer server.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Seed Search Party in state store
	partyID := "party-pet-mesh-101"
	petID := "pet-mesh-101"
	sectorID := "sec-mesh-01"

	party := searchparty.SearchParty{
		PartyID:   partyID,
		LostPetID: petID,
		CenterCoordinates: domain.LocationPoint{
			Latitude:  47.6205,
			Longitude: -122.3493,
		},
		RadiusMeters: 800.0,
		CreatedAt:    now.Add(-1 * time.Hour),
		Sectors: []searchparty.SearchSector{
			{
				SectorID: sectorID,
				Name:     "Alpha Ridge",
				PolygonPoints: []domain.LocationPoint{
					{Latitude: 47.620, Longitude: -122.350},
					{Latitude: 47.625, Longitude: -122.350},
					{Latitude: 47.625, Longitude: -122.345},
					{Latitude: 47.620, Longitude: -122.345},
				},
				Status:       searchparty.SectorStatusUnassigned,
				TotalAreaSqM: 250000,
			},
		},
		ActiveAssignments:     []searchparty.SectorAssignment{},
		CoveragePercentage:    0.0,
		ActiveVolunteersCount: 0,
	}
	partyBytes, _ := json.Marshal(party)
	if err := memStore.SaveState(ctx, store.SearchPartiesCollection, partyID, partyBytes); err != nil {
		t.Fatalf("failed to seed party: %v", err)
	}

	// Prepare MeshBatchDelta
	batch := domain.MeshBatchDelta{
		SearchPartyID: partyID,
		SenderNodeID:  "node-field-alpha",
		Sectors: []domain.MeshSectorDelta{
			{
				PetID:                petID,
				SectorID:             sectorID,
				State:                domain.SectorStateSearching,
				Rank:                 domain.SectorRankSearching,
				ClaimedByVolunteerID: "vol-99",
				ClaimedByName:        "Field Leader Dave",
				NodeID:               "node-field-alpha",
				LamportClock:         2,
				Timestamp:            now,
			},
		},
		Breadcrumbs: []domain.MeshBreadcrumbDelta{
			{
				PetID:         petID,
				VolunteerID:   "vol-99",
				VolunteerName: "Field Leader Dave",
				Seq:           1,
				Latitude:      47.621,
				Longitude:     -122.348,
				Accuracy:      3.5,
				Timestamp:     now,
			},
		},
		Sightings: []domain.MeshSightingDelta{
			{
				SightingID:    "sight-mesh-01",
				PetID:         petID,
				VolunteerID:   "vol-99",
				VolunteerName: "Field Leader Dave",
				Latitude:      47.622,
				Longitude:     -122.347,
				Notes:         "Fresh paw prints heading north-west",
				LamportClock:  1,
				Timestamp:     now,
			},
		},
	}

	batchBytes, _ := json.Marshal(batch)
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/mesh/uplink-sync", bytes.NewReader(batchBytes))
	if err != nil {
		t.Fatalf("failed to create uplink req: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("uplink request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("uplink status = %d, want %d; body = %s", resp.StatusCode, http.StatusOK, string(body))
	}

	var syncResp domain.MeshUplinkSyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		t.Fatalf("failed to decode uplink response: %v", err)
	}

	if !syncResp.Success {
		t.Errorf("syncResp.Success = false, want true")
	}
	if syncResp.ReconciledSectors != 1 {
		t.Errorf("syncResp.ReconciledSectors = %d, want 1", syncResp.ReconciledSectors)
	}
	if syncResp.ReconciledBreadcrumbs != 1 {
		t.Errorf("syncResp.ReconciledBreadcrumbs = %d, want 1", syncResp.ReconciledBreadcrumbs)
	}
	if syncResp.ReconciledSightings != 1 {
		t.Errorf("syncResp.ReconciledSightings = %d, want 1", syncResp.ReconciledSightings)
	}

	// Verify StateStore SearchParty status updated
	savedPartyBytes, err := memStore.GetState(ctx, store.SearchPartiesCollection, partyID)
	if err != nil {
		t.Fatalf("failed to read updated party from store: %v", err)
	}
	var updatedParty searchparty.SearchParty
	if err := json.Unmarshal(savedPartyBytes, &updatedParty); err != nil {
		t.Fatalf("failed to decode updated party: %v", err)
	}

	if len(updatedParty.Sectors) == 0 || updatedParty.Sectors[0].Status != searchparty.SectorStatusActiveSearch {
		t.Errorf("sector status = %q, want %q", updatedParty.Sectors[0].Status, searchparty.SectorStatusActiveSearch)
	}

	// Verify Breadcrumbs stored in store.BreadcrumbsCollection
	rawTrails, err := memStore.ListState(ctx, store.BreadcrumbsCollection)
	if err != nil || len(rawTrails) == 0 {
		t.Fatalf("expected breadcrumb trails in store, found %d (err: %v)", len(rawTrails), err)
	}

	// Verify Sighting stored in store.SightingsCollection
	rawSightings, err := memStore.ListState(ctx, store.SightingsCollection)
	if err != nil || len(rawSightings) == 0 {
		t.Fatalf("expected sightings in store, found %d (err: %v)", len(rawSightings), err)
	}
}

func TestMeshUplinkSync_MonotonicConflictResolution(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := NewServerWithOptions(memStore, ServerOptions{AllowPrivilegedMutations: true})
	ts := httptest.NewServer(server)
	defer ts.Close()
	defer server.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	partyID := "party-pet-mesh-202"
	petID := "pet-mesh-202"
	sectorID := "sec-mesh-cleared"

	// Seed Search Party with sector CLEARED (Rank 3)
	party := searchparty.SearchParty{
		PartyID:   partyID,
		LostPetID: petID,
		CenterCoordinates: domain.LocationPoint{
			Latitude:  47.6205,
			Longitude: -122.3493,
		},
		RadiusMeters: 800.0,
		CreatedAt:    now.Add(-2 * time.Hour),
		Sectors: []searchparty.SearchSector{
			{
				SectorID: sectorID,
				Name:     "Cleared Sector",
				PolygonPoints: []domain.LocationPoint{
					{Latitude: 47.620, Longitude: -122.350},
					{Latitude: 47.625, Longitude: -122.350},
					{Latitude: 47.625, Longitude: -122.345},
					{Latitude: 47.620, Longitude: -122.345},
				},
				Status:       searchparty.SectorStatusCleared, // CLEARED (Rank 3)
				TotalAreaSqM: 250000,
			},
		},
		ActiveAssignments: []searchparty.SectorAssignment{
			{
				AssignmentID:   "asgn-cleared",
				SectorID:       sectorID,
				VolunteerAlias: "Searcher One",
				ClaimedAt:      now.Add(-1 * time.Hour),
				Status:         searchparty.SectorStatusCleared,
			},
		},
		CoveragePercentage:    100.0,
		ActiveVolunteersCount: 0,
	}
	partyBytes, _ := json.Marshal(party)
	if err := memStore.SaveState(ctx, store.SearchPartiesCollection, partyID, partyBytes); err != nil {
		t.Fatalf("failed to seed party: %v", err)
	}

	// Post stale CLAIMED (Rank 1) sector delta
	batch := domain.MeshBatchDelta{
		SearchPartyID: partyID,
		SenderNodeID:  "node-stale",
		Sectors: []domain.MeshSectorDelta{
			{
				PetID:                petID,
				SectorID:             sectorID,
				State:                domain.SectorStateClaimed, // CLAIMED (Rank 1)
				Rank:                 domain.SectorRankClaimed,
				ClaimedByVolunteerID: "vol-stale",
				ClaimedByName:        "Stale Volunteer",
				NodeID:               "node-stale",
				LamportClock:         5, // even with high Lamport clock, lower rank must not overwrite
				Timestamp:            now,
			},
		},
	}

	batchBytes, _ := json.Marshal(batch)
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/mesh/uplink-sync", bytes.NewReader(batchBytes))
	if err != nil {
		t.Fatalf("failed to create req: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("uplink request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d; body = %s", resp.StatusCode, http.StatusOK, string(body))
	}

	var syncResp domain.MeshUplinkSyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Assert server rejected stale mutation
	if syncResp.ReconciledSectors != 0 {
		t.Errorf("ReconciledSectors = %d, want 0 (stale update rejected)", syncResp.ReconciledSectors)
	}

	// Assert server returned its latest deltas containing CLEARED
	if syncResp.ServerLatestDeltas == nil || len(syncResp.ServerLatestDeltas.Sectors) == 0 {
		t.Fatalf("ServerLatestDeltas = nil or empty, want server latest state")
	}
	serverSec := syncResp.ServerLatestDeltas.Sectors[0]
	if serverSec.SectorID != sectorID {
		t.Errorf("serverSec.SectorID = %q, want %q", serverSec.SectorID, sectorID)
	}
	if serverSec.Rank != domain.SectorRankCleared || !strings.EqualFold(serverSec.State, domain.SectorStateCleared) {
		t.Errorf("serverSec state = %q (rank %d), want CLEARED (rank 3)", serverSec.State, serverSec.Rank)
	}

	// Assert server retained CLEARED in StateStore
	savedPartyBytes, err := memStore.GetState(ctx, store.SearchPartiesCollection, partyID)
	if err != nil {
		t.Fatalf("failed to get party: %v", err)
	}
	var retainedParty searchparty.SearchParty
	_ = json.Unmarshal(savedPartyBytes, &retainedParty)
	if retainedParty.Sectors[0].Status != searchparty.SectorStatusCleared {
		t.Errorf("retained sector status = %q, want %q", retainedParty.Sectors[0].Status, searchparty.SectorStatusCleared)
	}
}

func TestMeshQRSignaling_SVGGeneration(t *testing.T) {
	server := NewServer()
	ts := httptest.NewServer(server)
	defer ts.Close()
	defer server.Close()

	// 1. Success case with valid data payload
	sdpPayload := "v=0\r\no=- 20518 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=fingerprint:sha-256 12:34:56"
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/mesh/qr-signaling?data="+url.QueryEscape(sdpPayload), nil)
	if err != nil {
		t.Fatalf("failed to create req: %v", err)
	}

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d; body = %s", resp.StatusCode, http.StatusOK, string(body))
	}

	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/svg+xml") {
		t.Errorf("Content-Type = %q, want image/svg+xml", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	bodyStr := string(body)
	if !strings.HasPrefix(bodyStr, "<svg") || !strings.Contains(bodyStr, "</svg>") {
		t.Errorf("expected valid SVG XML payload, got:\n%s", bodyStr)
	}

	// 2. Empty data parameter returns 400 Bad Request
	emptyReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/mesh/qr-signaling?data=", nil)
	emptyResp, err := ts.Client().Do(emptyReq)
	if err != nil {
		t.Fatalf("empty req failed: %v", err)
	}
	defer emptyResp.Body.Close()
	if emptyResp.StatusCode != http.StatusBadRequest {
		t.Errorf("status for empty data = %d, want %d", emptyResp.StatusCode, http.StatusBadRequest)
	}
}

func TestMeshSignaling_ValidationAndCORS(t *testing.T) {
	server := NewServer()
	ts := httptest.NewServer(server)
	defer ts.Close()
	defer server.Close()

	// 1. Missing searchPartyId on events returns 400
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/mesh/signal/events?nodeId=node-1", nil)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("req failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	// 2. Missing nodeId on events returns 400
	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/mesh/signal/events?searchPartyId=party-1", nil)
	resp2, err := ts.Client().Do(req2)
	if err != nil {
		t.Fatalf("req2 failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp2.StatusCode, http.StatusBadRequest)
	}

	// 3. OPTIONS preflight returns 204 and CORS headers
	optReq, _ := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/mesh/signal/message", nil)
	optResp, err := ts.Client().Do(optReq)
	if err != nil {
		t.Fatalf("options req failed: %v", err)
	}
	defer optResp.Body.Close()
	if optResp.StatusCode != http.StatusNoContent && optResp.StatusCode != http.StatusOK {
		t.Errorf("options status = %d, want 204 or 200", optResp.StatusCode)
	}
	if origin := optResp.Header.Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", origin)
	}
}
