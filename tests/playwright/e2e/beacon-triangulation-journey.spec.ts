import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe.serial('User Journey: BLE Collar Beacon Scanning & Proximity Triangulation', () => {
  const petId = `lost-beacon-journey-${Date.now()}`;
  const petName = 'Buster';
  const collarBeacon = {
    protocol: 'ibeacon',
    uuid: 'e2c56db5-dffb-48d2-b060-d0f5a71096e0',
    major: 100,
    minor: 200,
    calibratedRssi: -59,
  };
  const initialCoords = {
    latitude: 47.6300,
    longitude: -122.3150,
  };

  test('Test 1: Create a lost pet with CollarBeaconConfig and initiate search party', async ({ request }) => {
    // 1. Create a lost pet with CollarBeaconConfig
    const createPetRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId,
        petName,
        species: 'Dog',
        breed: 'Border Collie',
        primaryColor: 'Black',
        secondaryColor: 'White',
        description: 'Energetic border collie equipped with BLE smart collar tag.',
        location: 'Volunteer Park, Seattle, WA',
        reporterEmail: 'buster-owner@example.com',
        phone: '(206) 555-0199',
        coordinates: initialCoords,
        collarBeacon,
      },
    });
    expect([200, 201]).toContain(createPetRes.status());

    // 2. Initiate search party via POST /api/v1/lost-pets/${petId}/search-party
    const partyRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/search-party`, {
      data: { sectorCount: 4 },
    });
    expect([200, 201]).toContain(partyRes.status());

    // Verify search party response includes collarBeacon
    const party = await partyRes.json();
    expect(party.lostPetId).toBe(petId);
    expect(party.collarBeacon).toBeDefined();
    expect(party.collarBeacon.protocol).toBe(collarBeacon.protocol);
    expect(party.collarBeacon.uuid).toBe(collarBeacon.uuid);
    expect(party.collarBeacon.major).toBe(collarBeacon.major);
    expect(party.collarBeacon.minor).toBe(collarBeacon.minor);
    expect(party.collarBeacon.calibratedRssi).toBe(collarBeacon.calibratedRssi);
  });

  test('Test 2: Navigate to /pets, open search party modal, verify Radar HUD and controls', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await expect(petCard).toBeVisible();

    const spBtn = petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first();
    await expect(spBtn).toBeVisible();
    await spBtn.click();

    const modal = page.locator('#pet-search-party-container');
    await expect(modal).toBeVisible();
    await expect(modal).not.toHaveClass(/hidden/);

    // Verify #beacon-scanner-container is visible and not hidden
    const scannerContainer = page.locator('#beacon-scanner-container');
    await expect(scannerContainer).toBeVisible();
    await expect(scannerContainer).not.toHaveAttribute('hidden');

    // Verify initial state: #beacon-distance-display is "--", #beacon-proximity-badge has text "Out of Range"
    const distDisplay = page.locator('#beacon-distance-display');
    await expect(distDisplay).toHaveText('--');

    const proxBadge = page.locator('#beacon-proximity-badge');
    await expect(proxBadge).toHaveText('Out of Range');

    // Click #btn-start-beacon-scan: verify button text updates to "🛑 Stop Beacon Scan"
    const startScanBtn = page.locator('#btn-start-beacon-scan');
    await expect(startScanBtn).toHaveText('📡 Start Beacon Scan');
    await startScanBtn.click();
    await expect(startScanBtn).toHaveText('🛑 Stop Beacon Scan');

    // Click #btn-toggle-beacon-audio: verify audio state toggles (aria-pressed="true", text updates)
    const audioBtn = page.locator('#btn-toggle-beacon-audio');
    await expect(audioBtn).toHaveAttribute('aria-pressed', 'false');
    await audioBtn.click();
    await expect(audioBtn).toHaveAttribute('aria-pressed', 'true');
    await expect(audioBtn).toHaveText('🔊 Audio Ping: Active');
  });

  test('Test 3: Inject simulated BLE pings at Far and Near distances and verify HUD updates', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await expect(petCard).toBeVisible();
    await petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first().click();

    const modal = page.locator('#pet-search-party-container');
    await expect(modal).toBeVisible();

    // Ensure scanner is running
    const startScanBtn = page.locator('#btn-start-beacon-scan');
    if ((await startScanBtn.textContent())?.includes('Start')) {
      await startScanBtn.click();
      await expect(startScanBtn).toHaveText('🛑 Stop Beacon Scan');
    }

    // Inject ping 1 (Far): RSSI -82 dBm, distance ~12.5m. Verify #beacon-distance-display displays ~12.5m and badge indicates Far.
    // Verify Leaflet map has marker .beacon-ping-marker.
    await page.evaluate(async () => {
      const mockScanner = (window as any).__mockBluetoothScanner;
      if (mockScanner && typeof mockScanner.injectMockPing === 'function') {
        await mockScanner.injectMockPing({
          rssi: -82,
          distanceMeters: 12.5,
          proximity: 'far',
          observerCoords: { latitude: 47.6300, longitude: -122.3150 },
        });
      }
    });

    const distDisplay = page.locator('#beacon-distance-display');
    await expect(distDisplay).toContainText('12.5m');

    const proxBadge = page.locator('#beacon-proximity-badge');
    await expect(proxBadge).toContainText('Far');

    const beaconPingMarker = page.locator('.beacon-ping-marker');
    await expect(beaconPingMarker.first()).toBeVisible();

    // Inject ping 2 (Near): RSSI -64 dBm, distance ~3.2m. Verify #beacon-distance-display displays ~3.2m and badge indicates Near.
    await page.evaluate(async () => {
      const mockScanner = (window as any).__mockBluetoothScanner;
      if (mockScanner && typeof mockScanner.injectMockPing === 'function') {
        await mockScanner.injectMockPing({
          rssi: -64,
          distanceMeters: 3.2,
          proximity: 'near',
          observerCoords: { latitude: 47.6305, longitude: -122.3145 },
        });
      }
    });

    await expect(distDisplay).toContainText('3.2m');
    await expect(proxBadge).toContainText('Near');

    // Verify #btn-log-beacon-sighting becomes visible and not hidden
    const logBtn = page.locator('#btn-log-beacon-sighting');
    await expect(logBtn).toBeVisible();
    await expect(logBtn).not.toHaveAttribute('hidden');
  });

  test('Test 4: One-Tap Sighting Creation from proximity alert', async ({ page, request }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await expect(petCard).toBeVisible();
    await petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first().click();

    const modal = page.locator('#pet-search-party-container');
    await expect(modal).toBeVisible();

    // Ensure scanner is running
    const startScanBtn = page.locator('#btn-start-beacon-scan');
    if ((await startScanBtn.textContent())?.includes('Start')) {
      await startScanBtn.click();
      await expect(startScanBtn).toHaveText('🛑 Stop Beacon Scan');
    }

    // Inject Near ping so one-tap sighting button is visible
    await page.evaluate(async () => {
      const mockScanner = (window as any).__mockBluetoothScanner;
      if (mockScanner && typeof mockScanner.injectMockPing === 'function') {
        await mockScanner.injectMockPing({
          rssi: -64,
          distanceMeters: 3.2,
          proximity: 'near',
          observerCoords: { latitude: 47.6305, longitude: -122.3145 },
        });
      }
    });

    // Click #btn-log-beacon-sighting
    const logBtn = page.locator('#btn-log-beacon-sighting');
    await expect(logBtn).toBeVisible();
    await logBtn.click();

    // Verify #modal-report-sighting opens and is visible
    const sightingModal = page.locator('#modal-report-sighting');
    await expect(sightingModal).toBeVisible();

    // Verify coordinates in the sighting modal are pre-populated with observer/triangulation coordinates
    const latInput = page.locator('#sighting-lat');
    const lngInput = page.locator('#sighting-lng');
    await expect(latInput).not.toHaveValue('');
    await expect(lngInput).not.toHaveValue('');
    const latVal = parseFloat(await latInput.inputValue());
    const lngVal = parseFloat(await lngInput.inputValue());
    expect(latVal).toBeCloseTo(47.63, 1);
    expect(lngVal).toBeCloseTo(-122.315, 1);

    // Verify sighting notes contain "Collar beacon detected"
    const notesArea = page.locator('#sighting-notes');
    await expect(notesArea).toHaveValue(/Collar beacon detected/);

    // Fill reporter details if needed (expand optional witness summary if present)
    const witnessSummary = page.locator('.sighting-contact-summary');
    if (await witnessSummary.isVisible()) {
      await witnessSummary.click();
      await page.locator('#sighting-witness-name').fill('Volunteer Beacon Scout');
      await page.locator('#sighting-witness-contact').fill('scout@example.com');
    }

    // Submit sighting
    await page.locator('#btn-submit-sighting').click();

    // Verify sighting modal closes
    await expect(sightingModal).toBeHidden();

    // Verify sighting is saved via GET /api/v1/lost-pets/${petId}/sightings
    const sightingsRes = await request.get(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/sightings`);
    expect(sightingsRes.status()).toBe(200);
    const sightings = await sightingsRes.json();
    expect(Array.isArray(sightings)).toBe(true);
    expect(sightings.length).toBeGreaterThanOrEqual(1);
    const matched = sightings.some((s: any) => s.notes && s.notes.includes('Collar beacon detected'));
    expect(matched).toBe(true);
  });

  test('Test 5: Offline Buffering & Outbox Auto-Sync', async ({ page, request, context }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await expect(petCard).toBeVisible();
    await petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first().click();

    const modal = page.locator('#pet-search-party-container');
    await expect(modal).toBeVisible();

    // Ensure scanner is running
    const startScanBtn = page.locator('#btn-start-beacon-scan');
    if ((await startScanBtn.textContent())?.includes('Start')) {
      await startScanBtn.click();
      await expect(startScanBtn).toHaveText('🛑 Stop Beacon Scan');
    }

    // Query baseline observation count from API before going offline
    const preRes = await request.get(`${WEB_FRONTEND_URL}/api/v1/search-parties/${petId}/beacon-triangulation`);
    expect(preRes.status()).toBe(200);
    const preData = await preRes.json();
    const baselineCount = preData.triangulation?.observationCount || 0;

    // Simulate offline: await page.context().setOffline(true)
    await context.setOffline(true);

    // Inject ping 3 via page.evaluate(...) when offline
    await page.evaluate(async () => {
      const mockScanner = (window as any).__mockBluetoothScanner;
      if (mockScanner && typeof mockScanner.injectMockPing === 'function') {
        await mockScanner.injectMockPing({
          rssi: -60,
          distanceMeters: 2.1,
          proximity: 'near',
          observerCoords: { latitude: 47.6308, longitude: -122.3142 },
        });
      }
    });

    // Verify ping is stored in IndexedDB petspotr_beacon_outbox
    await expect.poll(async () => {
      return await page.evaluate(async () => {
        const outbox = (window as any).PetSpotROutbox;
        if (outbox && typeof outbox.getQueuedBeaconPings === 'function') {
          const queued = await outbox.getQueuedBeaconPings();
          return queued.length;
        }
        return 0;
      });
    }, { timeout: 5000 }).toBeGreaterThanOrEqual(1);

    // Restore online: await page.context().setOffline(false) and trigger window.dispatchEvent(new Event('online'))
    await context.setOffline(false);
    await page.evaluate(async () => {
      window.dispatchEvent(new Event('online'));
      if ((window as any).PetSpotROutbox?.flushBeaconPings) {
        await (window as any).PetSpotROutbox.flushBeaconPings();
      }
    });

    // Wait for outbox sync: queued beacon pings flushed to 0
    await expect.poll(async () => {
      return await page.evaluate(async () => {
        const outbox = (window as any).PetSpotROutbox;
        if (outbox && typeof outbox.getQueuedBeaconPings === 'function') {
          const queued = await outbox.getQueuedBeaconPings();
          return queued.length;
        }
        return 0;
      });
    }, { timeout: 10000 }).toBe(0);

    // Verify via API GET /api/v1/search-parties/${petId}/beacon-triangulation that the observation count increased
    await expect.poll(async () => {
      const res = await request.get(`${WEB_FRONTEND_URL}/api/v1/search-parties/${petId}/beacon-triangulation`);
      if (!res.ok()) return 0;
      const data = await res.json();
      return data.triangulation?.observationCount || 0;
    }, { timeout: 10000 }).toBeGreaterThan(baselineCount);
  });
});
