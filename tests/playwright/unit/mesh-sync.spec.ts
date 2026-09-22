import { test, expect } from '@playwright/test';

test.describe('Client WebRTC Mesh Controller & Offline IndexedDB Engine', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to base page to load frontend environment
    await page.goto('/');
  });

  test('should initialize petSpotRMesh and create IndexedDB database with required stores', async ({ page }) => {
    const dbInfo = await page.evaluate(async () => {
      if (!window.petSpotRMesh) {
        throw new Error('window.petSpotRMesh is not defined');
      }

      await window.petSpotRMesh.init({
        dbName: 'petspotr_mesh_db',
        partyId: 'party-test-1',
        volunteerInfo: {
          volunteerId: 'vol-101',
          volunteerName: 'Taylor Ranger',
          role: 'SEARCHER'
        }
      });

      // Verify IndexedDB database structure
      return new Promise<{ name: string; stores: string[] }>((resolve, reject) => {
        const req = indexedDB.open('petspotr_mesh_db');
        req.onsuccess = () => {
          const db = req.result;
          const stores = Array.from(db.objectStoreNames);
          db.close();
          resolve({ name: db.name, stores });
        };
        req.onerror = () => reject(req.error);
      });
    });

    expect(dbInfo.name).toBe('petspotr_mesh_db');
    expect(dbInfo.stores).toContain('mesh_sectors');
    expect(dbInfo.stores).toContain('mesh_breadcrumbs');
    expect(dbInfo.stores).toContain('mesh_sightings');
    expect(dbInfo.stores).toContain('mesh_outbox');
  });

  test('should strictly enforce monotonic sector rank progression and CRDT tie-breaking', async ({ page }) => {
    const crdtResults = await page.evaluate(async () => {
      const mesh = window.petSpotRMesh;
      if (!mesh) throw new Error('window.petSpotRMesh is not defined');

      await mesh.init({ dbName: 'petspotr_mesh_db_crdt', partyId: 'party-crdt' });

      // Step 1: Initial claim (CLAIMED: rank 1, lamportClock 1)
      await mesh.mergeSectorDelta({
        petId: 'pet-alpha',
        sectorId: 'sector-nw-1',
        state: 'CLAIMED',
        rank: 1,
        claimedByVolunteerId: 'vol-1',
        claimedByName: 'Alice',
        nodeId: 'node-alice',
        lamportClock: 1,
        timestamp: '2026-09-22T08:00:00Z'
      });
      let s1 = mesh.getSector('sector-nw-1');

      // Step 2: Progress to SEARCHING (rank 2, lamportClock 2)
      await mesh.mergeSectorDelta({
        petId: 'pet-alpha',
        sectorId: 'sector-nw-1',
        state: 'SEARCHING',
        rank: 2,
        claimedByVolunteerId: 'vol-1',
        claimedByName: 'Alice',
        nodeId: 'node-alice',
        lamportClock: 2,
        timestamp: '2026-09-22T08:05:00Z'
      });
      let s2 = mesh.getSector('sector-nw-1');

      // Step 3: Progress to CLEARED (rank 3, lamportClock 3)
      await mesh.mergeSectorDelta({
        petId: 'pet-alpha',
        sectorId: 'sector-nw-1',
        state: 'CLEARED',
        rank: 3,
        claimedByVolunteerId: 'vol-1',
        claimedByName: 'Alice',
        nodeId: 'node-alice',
        lamportClock: 3,
        timestamp: '2026-09-22T08:10:00Z'
      });
      let s3 = mesh.getSector('sector-nw-1');

      // Step 4: Regressive update attempts with higher Lamport clocks MUST be rejected
      // Attempt A: Incoming CLAIMED (rank 1) with lamportClock 10
      const regressiveClaimedAccepted = await mesh.mergeSectorDelta({
        petId: 'pet-alpha',
        sectorId: 'sector-nw-1',
        state: 'CLAIMED',
        rank: 1,
        claimedByVolunteerId: 'vol-2',
        claimedByName: 'Bob',
        nodeId: 'node-bob',
        lamportClock: 10,
        timestamp: '2026-09-22T08:20:00Z'
      });
      let sAfterRegressiveClaimed = mesh.getSector('sector-nw-1');

      // Attempt B: Incoming SEARCHING (rank 2) with lamportClock 20
      const regressiveSearchingAccepted = await mesh.mergeSectorDelta({
        petId: 'pet-alpha',
        sectorId: 'sector-nw-1',
        state: 'SEARCHING',
        rank: 2,
        claimedByVolunteerId: 'vol-3',
        claimedByName: 'Charlie',
        nodeId: 'node-charlie',
        lamportClock: 20,
        timestamp: '2026-09-22T08:25:00Z'
      });
      let sAfterRegressiveSearching = mesh.getSector('sector-nw-1');

      // Step 5: Equal rank tie-breaking
      // Sector B: test Lamport clock precedence
      await mesh.mergeSectorDelta({
        petId: 'pet-alpha',
        sectorId: 'sector-se-2',
        state: 'CLAIMED',
        rank: 1,
        claimedByVolunteerId: 'vol-1',
        claimedByName: 'Alice',
        nodeId: 'node-alice',
        lamportClock: 2,
        timestamp: '2026-09-22T08:00:00Z'
      });
      // Higher clock (5 vs 2) at same rank should win
      await mesh.mergeSectorDelta({
        petId: 'pet-alpha',
        sectorId: 'sector-se-2',
        state: 'CLAIMED',
        rank: 1,
        claimedByVolunteerId: 'vol-2',
        claimedByName: 'Bob',
        nodeId: 'node-bob',
        lamportClock: 5,
        timestamp: '2026-09-22T08:01:00Z'
      });
      let sClockWinner = mesh.getSector('sector-se-2');

      // Stale clock (3 vs 5) should be rejected
      const staleClockAccepted = await mesh.mergeSectorDelta({
        petId: 'pet-alpha',
        sectorId: 'sector-se-2',
        state: 'CLAIMED',
        rank: 1,
        claimedByVolunteerId: 'vol-3',
        claimedByName: 'Charlie',
        nodeId: 'node-charlie',
        lamportClock: 3,
        timestamp: '2026-09-22T08:02:00Z'
      });
      let sClockAfterStale = mesh.getSector('sector-se-2');

      // Step 6: Equal rank AND equal Lamport clock: Lexicographical NodeID tie-breaker
      // "node-zulu" vs "node-alpha": "node-zulu" > "node-alpha"
      await mesh.mergeSectorDelta({
        petId: 'pet-alpha',
        sectorId: 'sector-ne-3',
        state: 'CLAIMED',
        rank: 1,
        claimedByVolunteerId: 'vol-alpha',
        claimedByName: 'Alpha',
        nodeId: 'node-alpha',
        lamportClock: 8,
        timestamp: '2026-09-22T08:00:00Z'
      });
      const nodeZuluWon = await mesh.mergeSectorDelta({
        petId: 'pet-alpha',
        sectorId: 'sector-ne-3',
        state: 'CLAIMED',
        rank: 1,
        claimedByVolunteerId: 'vol-zulu',
        claimedByName: 'Zulu',
        nodeId: 'node-zulu',
        lamportClock: 8,
        timestamp: '2026-09-22T08:00:00Z'
      });
      let sNodeWinner = mesh.getSector('sector-ne-3');

      // Reverse attempt: "node-bravo" < "node-zulu" at same clock 8 should be rejected
      const nodeBravoAccepted = await mesh.mergeSectorDelta({
        petId: 'pet-alpha',
        sectorId: 'sector-ne-3',
        state: 'CLAIMED',
        rank: 1,
        claimedByVolunteerId: 'vol-bravo',
        claimedByName: 'Bravo',
        nodeId: 'node-bravo',
        lamportClock: 8,
        timestamp: '2026-09-22T08:00:00Z'
      });
      let sNodeAfterLesser = mesh.getSector('sector-ne-3');

      return {
        s1State: s1?.state,
        s2State: s2?.state,
        s3State: s3?.state,
        regressiveClaimedAccepted,
        sAfterRegressiveClaimedState: sAfterRegressiveClaimed?.state,
        regressiveSearchingAccepted,
        sAfterRegressiveSearchingState: sAfterRegressiveSearching?.state,
        sClockWinnerNodeId: sClockWinner?.nodeId,
        staleClockAccepted,
        sClockAfterStaleNodeId: sClockAfterStale?.nodeId,
        nodeZuluWon,
        sNodeWinnerNodeId: sNodeWinner?.nodeId,
        nodeBravoAccepted,
        sNodeAfterLesserNodeId: sNodeAfterLesser?.nodeId
      };
    });

    expect(crdtResults.s1State).toBe('CLAIMED');
    expect(crdtResults.s2State).toBe('SEARCHING');
    expect(crdtResults.s3State).toBe('CLEARED');
    expect(crdtResults.regressiveClaimedAccepted).toBe(false);
    expect(crdtResults.sAfterRegressiveClaimedState).toBe('CLEARED');
    expect(crdtResults.regressiveSearchingAccepted).toBe(false);
    expect(crdtResults.sAfterRegressiveSearchingState).toBe('CLEARED');

    expect(crdtResults.sClockWinnerNodeId).toBe('node-bob');
    expect(crdtResults.staleClockAccepted).toBe(false);
    expect(crdtResults.sClockAfterStaleNodeId).toBe('node-bob');

    expect(crdtResults.nodeZuluWon).toBe(true);
    expect(crdtResults.sNodeWinnerNodeId).toBe('node-zulu');
    expect(crdtResults.nodeBravoAccepted).toBe(false);
    expect(crdtResults.sNodeAfterLesserNodeId).toBe('node-zulu');
  });

  test('should queue mutations in outbox when offline and drain on simulated online event', async ({ page }) => {
    // Intercept uplink endpoint to track sync requests
    let uplinkCallCount = 0;
    let lastUplinkPayload: any = null;

    await page.route('**/api/v1/mesh/uplink-sync', async (route) => {
      uplinkCallCount++;
      lastUplinkPayload = route.request().postDataJSON();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          reconciledSectors: lastUplinkPayload?.sectors?.length || 0,
          reconciledBreadcrumbs: lastUplinkPayload?.breadcrumbs?.length || 0,
          reconciledSightings: lastUplinkPayload?.sightings?.length || 0,
          syncedAt: new Date().toISOString()
        })
      });
    });

    const outboxResult = await page.evaluate(async () => {
      const mesh = window.petSpotRMesh;
      if (!mesh) throw new Error('window.petSpotRMesh is not defined');

      await mesh.init({
        dbName: 'petspotr_mesh_db_outbox',
        partyId: 'party-outbox',
        volunteerInfo: { volunteerId: 'vol-42', volunteerName: 'Sam Scout', role: 'SEARCHER' }
      });

      // Simulate offline mode by setting internal flag
      mesh.setOnlineStatus(false);

      // Record mutations while offline
      await mesh.claimSector('pet-outbox-1', 'sector-1', 'CLAIMED');
      await mesh.recordBreadcrumb('pet-outbox-1', 47.6062, -122.3321);

      // Query IndexedDB mesh_outbox
      const pendingBeforeSync = await mesh.getPendingOutbox();

      // Trigger simulated online event
      window.dispatchEvent(new Event('online'));

      // Wait for uplink drain promise
      const drainResult = await mesh.syncUplink();

      const pendingAfterSync = await mesh.getPendingOutbox();

      return {
        pendingBeforeCount: pendingBeforeSync.length,
        pendingAfterCount: pendingAfterSync.length,
        drainSuccess: drainResult?.success
      };
    });

    expect(outboxResult.pendingBeforeCount).toBeGreaterThanOrEqual(2);
    expect(outboxResult.drainSuccess).toBe(true);
    expect(outboxResult.pendingAfterCount).toBe(0);
    expect(uplinkCallCount).toBeGreaterThanOrEqual(1);
    expect(lastUplinkPayload?.searchPartyId).toBe('party-outbox');
    expect(lastUplinkPayload?.sectors?.length).toBeGreaterThanOrEqual(1);
    expect(lastUplinkPayload?.breadcrumbs?.length).toBeGreaterThanOrEqual(1);
  });

  test('should compress and decompress optical QR SDP payloads losslessly', async ({ page }) => {
    const sdpResult = await page.evaluate(async () => {
      const mesh = window.petSpotRMesh;
      if (!mesh) throw new Error('window.petSpotRMesh is not defined');

      const mockSDPOffer = {
        type: 'offer',
        sdp: 'v=0\r\no=- 424242 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=group:BUNDLE data\r\nm=application 9 UDP/DTLS/SCTP webrtc-datachannel\r\nc=IN IP4 192.168.1.100\r\na=candidate:1 1 UDP 2130706431 192.168.1.100 54321 typ host\r\n',
        nodeId: 'node-initiator-1',
        searchPartyId: 'party-optical-99'
      };

      const compressed = await mesh.compressSDP(mockSDPOffer);
      const decompressed = await mesh.decompressSDP(compressed);

      return {
        compressed,
        decompressed,
        isUriSafe: /^[A-Za-z0-9_-]+$/.test(compressed),
        isShorter: compressed.length < JSON.stringify(mockSDPOffer).length
      };
    });

    expect(sdpResult.isUriSafe).toBe(true);
    expect(sdpResult.decompressed.type).toBe('offer');
    expect(sdpResult.decompressed.nodeId).toBe('node-initiator-1');
    expect(sdpResult.decompressed.searchPartyId).toBe('party-optical-99');
    expect(sdpResult.decompressed.sdp).toContain('webrtc-datachannel');
  });

  test('should dispatch custom DOM events on peer and state mutations', async ({ page }) => {
    const eventResults = await page.evaluate(async () => {
      const mesh = window.petSpotRMesh;
      if (!mesh) throw new Error('window.petSpotRMesh is not defined');

      await mesh.init({ dbName: 'petspotr_mesh_db_events', partyId: 'party-events' });

      const eventsFired: Record<string, any> = {};

      const eventNames = [
        'mesh:peer-joined',
        'mesh:peer-left',
        'mesh:sector-updated',
        'mesh:breadcrumb-received',
        'mesh:sos-alert'
      ];

      eventNames.forEach((name) => {
        window.addEventListener(name, (e: any) => {
          eventsFired[name] = e.detail;
        });
      });

      // Trigger operations that fire events
      mesh.handlePeerJoined({
        nodeId: 'peer-event-1',
        volunteerId: 'vol-peer-1',
        volunteerName: 'Morgan',
        role: 'K9_HANDLER'
      });

      await mesh.claimSector('pet-ev-1', 'sector-south-1', 'CLAIMED');
      await mesh.recordBreadcrumb('pet-ev-1', 45.5152, -122.6784);
      await mesh.broadcastSOS('Need assistance on steep slope');

      mesh.handlePeerLeft('peer-event-1');

      return eventsFired;
    });

    expect(eventResults['mesh:peer-joined']).toBeDefined();
    expect(eventResults['mesh:peer-joined'].nodeId).toBe('peer-event-1');
    expect(eventResults['mesh:peer-joined'].role).toBe('K9_HANDLER');

    expect(eventResults['mesh:sector-updated']).toBeDefined();
    expect(eventResults['mesh:sector-updated'].sectorId).toBe('sector-south-1');
    expect(eventResults['mesh:sector-updated'].state).toBe('CLAIMED');

    expect(eventResults['mesh:breadcrumb-received']).toBeDefined();
    expect(eventResults['mesh:breadcrumb-received'].latitude).toBeCloseTo(45.5152, 3);

    expect(eventResults['mesh:sos-alert']).toBeDefined();
    expect(eventResults['mesh:sos-alert'].message).toContain('steep slope');

    expect(eventResults['mesh:peer-left']).toBeDefined();
    expect(eventResults['mesh:peer-left'].nodeId).toBe('peer-event-1');
  });
});
