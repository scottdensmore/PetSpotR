import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8102';

test.describe('User Journey: Community Sighting Timeline & Trajectory Map', () => {
  test.beforeAll(async ({ request }) => {
    // Seed demo-lost-1 pet report in live backend if not already present
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId: 'demo-lost-1',
        petName: 'Rusty',
        species: 'Dog',
        breed: 'Golden Retriever',
        primaryColor: 'Golden',
        description: 'Friendly golden retriever wearing blue collar.',
        location: 'Capitol Hill, Seattle, WA',
        reporterEmail: 'owner@example.com',
        phone: '(206) 555-0199',
        coordinates: {
          latitude: 47.6152,
          longitude: -122.3211,
        },
      },
    });
    expect([200, 201, 409]).toContain(res.status());

    // Seed an initial sighting to ensure baseline trajectory milestones exist
    const sRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets/demo-lost-1/sightings`, {
      data: {
        locationDescription: 'Spotted near Cal Anderson Park',
        sightedAt: new Date(Date.now() - 45 * 60 * 1000).toISOString(),
        coordinates: {
          latitude: 47.6174,
          longitude: -122.3195,
        },
        movementDirection: 'North',
        notes: 'Trotting happily near tennis courts',
      },
    });
    expect([200, 201]).toContain(sRes.status());
  });

  test.beforeEach(async ({ context }) => {
    try {
      await context.grantPermissions(['clipboard-read', 'clipboard-write', 'geolocation']);
    } catch {}
    try {
      await context.grantPermissions(['geolocation'], { origin: WEB_FRONTEND_URL });
    } catch {}
    try {
      await context.setGeolocation({ latitude: 47.6152, longitude: -122.3211 });
    } catch (e) {
      console.warn('Geolocation grant warning:', e);
    }
  });

  test('Journey 1: Report a sighting from /pets and display confirmation feedback', async ({ page }) => {
    // 1. Visit /pets
    await page.goto(`${WEB_FRONTEND_URL}/pets`);
    const lostCard = page.locator('article.pet-card[data-pet-id="demo-lost-1"]');
    await expect(lostCard).toBeVisible();

    // 2. Open Sighting Modal via .btn-report-sighting
    const reportBtn = lostCard.locator('.btn-report-sighting, button[data-action="report-sighting"]').first();
    await expect(reportBtn).toBeVisible();
    await reportBtn.click();

    const modal = page.locator('#modal-report-sighting');
    await expect(modal).toBeVisible();
    await expect(modal).not.toHaveClass(/hidden/);
    await expect(page.locator('#sighting-modal-title')).toBeVisible();

    // 3. Click "📍 Use My Location" and verify GPS status
    const geoBtn = page.locator('#sighting-geolocation-btn');
    await expect(geoBtn).toBeVisible();
    await geoBtn.click();
    await expect(page.locator('#sighting-gps-status')).toContainText(/GPS Acquired|Location Updated/i);

    // 4. Fill location description, direction, notes
    await page.locator('#sighting-location').fill('Near Pike Place Market, 4th & Olive Way');
    await page.locator('#sighting-direction').selectOption('East');
    await page.locator('#sighting-notes').fill('Spotted heading east towards 1st Ave');

    // 5. Submit sighting and verify real API call
    const [sightingReq] = await Promise.all([
      page.waitForRequest(req => req.url().includes('/sightings') && req.method() === 'POST'),
      page.locator('#btn-submit-sighting').click(),
    ]);
    const payload = JSON.parse(sightingReq.postData() || '{}');
    expect(payload.locationDescription).toContain('Pike Place Market');
    expect(payload.movementDirection).toBe('East');
    expect(payload.notes).toContain('heading east');
    if (payload.coordinates) {
      expect(payload.coordinates.latitude).toBeCloseTo(47.6152, 2);
      expect(payload.coordinates.longitude).toBeCloseTo(-122.3211, 2);
    }

    // 6. Verify modal closes and toast appears
    await expect(modal).toBeHidden();
    const toast = page.locator('#toast-container .toast-item, .toast-item');
    await expect(toast).toBeVisible();
    await expect(toast).toContainText(/Sighting reported/i);
  });

  test('Journey 2: Open trajectory map via pet card and inspect milestones, perimeter, and timeline', async ({ page }) => {
    // 1. Visit /pets
    await page.goto(`${WEB_FRONTEND_URL}/pets`);
    const lostCard = page.locator('article.pet-card[data-pet-id="demo-lost-1"]');
    await expect(lostCard).toBeVisible();

    // 2. Open Trajectory modal via .btn-view-trajectory
    const trajBtn = lostCard.locator('.btn-view-trajectory, button[data-action="view-trajectory"]').first();
    await expect(trajBtn).toBeVisible();
    await trajBtn.click();

    // 3. Verify trajectory container and map become visible
    const trajContainer = page.locator('#pet-trajectory-container');
    await expect(trajContainer).toBeVisible();
    await expect(trajContainer).not.toHaveClass(/hidden/);

    const trajMap = page.locator('#pet-trajectory-map');
    await expect(trajMap).toBeVisible();

    // 4. Verify milestone pins in map and legend
    // Pin 0 (Origin)
    const originPin = page.locator('.milestone-pin-origin');
    await expect(originPin.first()).toBeVisible();

    // Sighting milestone pin(s)
    const sightingPin = page.locator('.milestone-pin:not(.milestone-pin-origin)');
    await expect(sightingPin.first()).toBeVisible();

    // Perimeter readout and stats
    const countStat = page.locator('#traj-stat-count');
    await expect(countStat).toBeVisible();
    await expect(countStat).not.toHaveText('0');

    const distStat = page.locator('#traj-stat-distance');
    await expect(distStat).toBeVisible();
    await expect(distStat).toContainText('mi');

    const perimeterStat = page.locator('#traj-stat-perimeter');
    await expect(perimeterStat).toBeVisible();

    // 5. Verify timeline list
    const timelineList = page.locator('#timeline-sightings-list');
    await expect(timelineList).toBeVisible();
    await expect(timelineList.locator('.timeline-item').first()).toBeVisible();
    await expect(timelineList).toContainText(/Origin|Pin 0/i);

    // 6. Close trajectory modal
    const closeBtn = page.locator('#btn-close-trajectory');
    await closeBtn.click();
    await expect(trajContainer).toBeHidden();
  });

  test('Journey 3: Report sighting from finder landing page /p/{petID} and verify trajectory map updates', async ({ page }) => {
    // 1. Visit /p/demo-lost-1
    await page.goto(`${WEB_FRONTEND_URL}/p/demo-lost-1`);
    await expect(page.locator('.finder-landing-container')).toBeVisible();

    // 2. Open sighting modal
    const finderSightingBtn = page.locator('#btn-finder-sighting');
    await expect(finderSightingBtn).toBeVisible();
    await finderSightingBtn.click();

    const modal = page.locator('#modal-report-sighting');
    await expect(modal).toBeVisible();
    await expect(modal).not.toHaveClass(/hidden/);

    // 3. Acquire geolocation and fill fields
    await page.locator('#sighting-geolocation-btn').click();
    await expect(page.locator('#sighting-gps-status')).toContainText(/GPS Acquired|Location Updated/i);

    await page.locator('#sighting-location').fill('Volunteer Park water reservoir');
    await page.locator('#sighting-direction').selectOption('North');
    await page.locator('#sighting-notes').fill('Trotting safely towards Volunteer Park conservatory');

    // 4. Submit sighting
    const [sightingReq] = await Promise.all([
      page.waitForRequest(req => req.url().includes('/api/v1/lost-pets/demo-lost-1/sightings') && req.method() === 'POST'),
      page.locator('#btn-submit-sighting').click(),
    ]);
    const payload = JSON.parse(sightingReq.postData() || '{}');
    expect(payload.locationDescription).toContain('Volunteer Park');
    expect(payload.movementDirection).toBe('North');

    // 5. Verify modal closes and toast appears
    await expect(modal).toBeHidden();
    const toast = page.locator('#toast-container .toast-item, .toast-item');
    await expect(toast).toBeVisible();
    await expect(toast).toContainText(/Sighting reported/i);

    // 6. Open trajectory map on finder landing page
    const viewTrajBtn = page.locator('#btn-finder-trajectory');
    await expect(viewTrajBtn).toBeVisible();
    await viewTrajBtn.click();

    const trajContainer = page.locator('#pet-trajectory-container');
    await expect(trajContainer).toBeVisible();
    await expect(trajContainer).not.toHaveClass(/hidden/);

    const trajMap = page.locator('#pet-trajectory-map');
    await expect(trajMap).toBeVisible();

    // Verify timeline includes newly reported sighting
    const timelineList = page.locator('#timeline-sightings-list');
    await expect(timelineList).toBeVisible();
    await expect(timelineList).toContainText('Volunteer Park');

    // Close modal
    await page.locator('#btn-close-trajectory').click();
    await expect(trajContainer).toBeHidden();
  });

  test('Direct API Assertion: GET /api/v1/lost-pets/{petID}/trajectory returns valid origin and legs', async ({ page }) => {
    const trajResp = await page.request.get(`${WEB_FRONTEND_URL}/api/v1/lost-pets/demo-lost-1/trajectory`);
    expect(trajResp.status()).toBe(200);

    const data = await trajResp.json();
    expect(data.lostPetId).toBe('demo-lost-1');
    expect(data.sightingsCount).toBeGreaterThanOrEqual(1);
    expect(data.originLocation).toBeDefined();
    expect(data.originLocation.latitude).toBeCloseTo(47.6152, 2);
    expect(data.originLocation.longitude).toBeCloseTo(-122.3211, 2);

    expect(Array.isArray(data.legs)).toBe(true);
    expect(data.legs.length).toBeGreaterThanOrEqual(1);

    const leg = data.legs[0];
    expect(leg).toHaveProperty('fromSightingId');
    expect(leg).toHaveProperty('toSightingId');
    expect(leg).toHaveProperty('distanceMiles');
    expect(leg).toHaveProperty('cardinalHeading');

    expect(data.estimatedPerimeter).toBeDefined();
    expect(data.estimatedPerimeter.centerCoordinates).toBeDefined();
    expect(data.estimatedPerimeter.radiusMiles).toBeGreaterThan(0);
    expect(data.estimatedPerimeter.confidenceLevel).toBeDefined();
  });
});
