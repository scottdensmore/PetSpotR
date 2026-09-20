import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe.serial('User Journey: Volunteer GPS Breadcrumbs & Offline Sector Walk Recording', () => {
  const petId = `lost-trail-journey-${Date.now()}`;
  const petName = 'Zeus';
  let sectorId = 'sector-1';

  test.beforeAll(async ({ request }) => {
    // 1. Seed lost pet with valid location coordinates
    const petRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId,
        petName,
        species: 'Dog',
        breed: 'German Shepherd',
        primaryColor: 'Tan',
        secondaryColor: 'Black',
        description: 'Friendly search trial dog testing GPS breadcrumbs.',
        location: 'Volunteer Park, Seattle, WA',
        reporterEmail: 'zeus-owner@example.com',
        phone: '(206) 555-0155',
        coordinates: {
          latitude: 47.6300,
          longitude: -122.3150,
        },
      },
    });
    expect([200, 201]).toContain(petRes.status());

    // 2. Initialize search party with sectors
    const partyRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/search-party`, {
      data: { sectorCount: 4 },
    });
    expect([200, 201]).toContain(partyRes.status());
    const party = await partyRes.json();
    if (party.sectors && party.sectors.length > 0) {
      sectorId = party.sectors[0].sectorId;
    }
  });

  test('Test 1: Verify search party modal opens with Trail Recording controls and distance display', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await expect(petCard).toBeVisible();

    const spBtn = petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first();
    await expect(spBtn).toBeVisible();
    await spBtn.click();

    const modal = page.locator('#pet-search-party-container');
    await expect(modal).toBeVisible();

    // Verify trail recording strip controls
    const trailStrip = page.locator('#search-party-trail-strip');
    await expect(trailStrip).toBeVisible();

    const recordBtn = page.locator('#btn-toggle-record-trail');
    await expect(recordBtn).toBeVisible();
    await expect(page.locator('#record-trail-btn-label')).toHaveText('Record Search Path');

    const distanceWalked = page.locator('#trail-distance-walked');
    await expect(distanceWalked).toBeVisible();
    await expect(distanceWalked).toContainText('0');

    const recordingBadge = page.locator('#recording-active-badge');
    await expect(recordingBadge).toHaveClass(/hidden/);

    const offlineBadge = page.locator('#trail-offline-badge');
    await expect(offlineBadge).toHaveClass(/hidden/);

    // Verify sector trail badge exists in sector cards
    const firstSectorCard = page.locator('#search-party-sector-cards .sector-card').first();
    await expect(firstSectorCard).toBeVisible();
    const trailBadge = firstSectorCard.locator('.badge-trail');
    await expect(trailBadge).toBeVisible();
    await expect(trailBadge).toContainText('0 trails');
  });

  test('Test 2: Claim sector and start recording search path, simulating GPS positions', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await expect(petCard).toBeVisible();
    await petCard.locator('.btn-view-search-party').first().click();

    const modal = page.locator('#pet-search-party-container');
    await expect(modal).toBeVisible();

    // Claim first sector
    const firstSectorCard = page.locator('#search-party-sector-cards .sector-card').first();
    await firstSectorCard.locator('.btn-sector-action[data-action="claim-sector"]').click();

    const claimModal = page.locator('#modal-claim-sector');
    await expect(claimModal).toBeVisible();
    await page.locator('#claim-volunteer-alias').fill('Volunteer Scout One');
    await page.locator('#btn-submit-claim').click();
    await expect(claimModal).toBeHidden();

    // Start Recording
    const recordBtn = page.locator('#btn-toggle-record-trail');
    await recordBtn.click();

    await expect(page.locator('#record-trail-btn-label')).toHaveText('Stop Recording');
    const recordingBadge = page.locator('#recording-active-badge');
    await expect(recordingBadge).toBeVisible();
    await expect(recordingBadge).not.toHaveClass(/hidden/);

    // Simulate GPS breadcrumb points along the sector
    await page.evaluate(() => {
      const sp = (window as unknown as { petspotrSearchParty: { onPositionUpdate: (pos: unknown) => Promise<void> } }).petspotrSearchParty;
      return sp.onPositionUpdate({
        coords: {
          latitude: 47.6305,
          longitude: -122.3145,
          accuracy: 4.2,
        },
        timestamp: Date.now(),
      });
    });

    await page.waitForTimeout(300);

    await page.evaluate(() => {
      const sp = (window as unknown as { petspotrSearchParty: { onPositionUpdate: (pos: unknown) => Promise<void> } }).petspotrSearchParty;
      return sp.onPositionUpdate({
        coords: {
          latitude: 47.6318,
          longitude: -122.3138,
          accuracy: 3.8,
        },
        timestamp: Date.now() + 10000,
      });
    });

    // Verify distance walked counter updated to a positive distance
    const distanceWalked = page.locator('#trail-distance-walked');
    await expect(distanceWalked).not.toHaveText('0.00 km');

    // Verify active volunteer pulsing marker on the map
    const activeMarker = page.locator('.volunteer-pulse-marker');
    await expect(activeMarker).toBeVisible();

    // Verify Leaflet polyline exists
    const polylines = page.locator('.volunteer-trail-polyline, .leaflet-pane svg path[stroke]');
    await expect(polylines.first()).toBeAttached();
  });

  test('Test 3: Offline buffering and synchronization when reconnected', async ({ page, context }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await petCard.locator('.btn-view-search-party').first().click();

    const modal = page.locator('#pet-search-party-container');
    await expect(modal).toBeVisible();

    // Ensure recording is active
    const recordBtn = page.locator('#btn-toggle-record-trail');
    const isRecording = await page.evaluate(() => {
      const sp = (window as unknown as { petspotrSearchParty: { isRecording: () => boolean } }).petspotrSearchParty;
      return sp.isRecording();
    });
    if (!isRecording) {
      await recordBtn.click();
    }

    // 1. Simulate Network Disconnect (Offline mode)
    await context.setOffline(true);
    await page.evaluate(() => window.dispatchEvent(new Event('offline')));

    // 2. Record new GPS coordinates while offline
    await page.evaluate(() => {
      const sp = (window as unknown as { petspotrSearchParty: { onPositionUpdate: (pos: unknown) => Promise<void> } }).petspotrSearchParty;
      return sp.onPositionUpdate({
        coords: {
          latitude: 47.6325,
          longitude: -122.3130,
          accuracy: 4.0,
        },
        timestamp: Date.now() + 20000,
      });
    });

    // 3. Verify IndexedDB contains queued breadcrumb and offline badge appears
    const offlineBadge = page.locator('#trail-offline-badge');
    await expect(offlineBadge).toBeVisible();
    await expect(offlineBadge).toContainText('Buffered Offline');

    const queuedCount = await page.evaluate(async () => {
      const sp = (window as unknown as { petspotrSearchParty: { getQueuedBreadcrumbs: () => Promise<unknown[]> } }).petspotrSearchParty;
      const queued = await sp.getQueuedBreadcrumbs();
      return queued.length;
    });
    expect(queuedCount).toBeGreaterThanOrEqual(1);

    // 4. Restore Network Connectivity (Online mode)
    await context.setOffline(false);
    await page.evaluate(() => window.dispatchEvent(new Event('online')));

    // Flush offline queue
    await page.evaluate(async () => {
      const sp = (window as unknown as { petspotrSearchParty: { flushOfflineBreadcrumbs: () => Promise<void> } }).petspotrSearchParty;
      await sp.flushOfflineBreadcrumbs();
    });

    // Verify offline badge is hidden after successful flush
    await expect(offlineBadge).toHaveClass(/hidden/);

    // Stop recording
    await recordBtn.click();
    await expect(page.locator('#record-trail-btn-label')).toHaveText('Record Search Path');
    await expect(page.locator('#recording-active-badge')).toHaveClass(/hidden/);
  });

  test('Test 4: Verify REST API GET /api/v1/search-parties/{petID}/sectors/{sectorID}/breadcrumbs returns saved trail', async ({ request }) => {
    const res = await request.get(`${WEB_FRONTEND_URL}/api/v1/search-parties/${petId}/sectors/${sectorId}/breadcrumbs`);
    expect(res.status()).toBe(200);

    const trails = await res.json();
    expect(Array.isArray(trails)).toBe(true);
    expect(trails.length).toBeGreaterThanOrEqual(1);

    const trail = trails[0];
    expect(trail.trailId).toBeDefined();
    expect(trail.sectorId).toBe(sectorId);
    expect(trail.volunteerAlias).toBeDefined();
    expect(trail.totalDistanceM).toBeGreaterThan(0);
    expect(trail.points.length).toBeGreaterThanOrEqual(2);

    for (const pt of trail.points) {
      expect(pt.latitude).toBeGreaterThan(45);
      expect(pt.longitude).toBeLessThan(-120);
      expect(pt.timestamp).toBeDefined();
    }
  });
});
