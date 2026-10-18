import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Canine (K9) Scent-Trailing Telemetry, Wind Plume Dispersion & Handler Cockpit E2E Journey', () => {
  test('Full 6-step user journey: seeded K9 missions & plume radar, new mantrailing mission dispatch, downwind in-plume waypoint alert, sighting promotion & POD progression, thermal paw hazard inspection, and GPX 1.1 telemetry export', async ({
    page,
    request,
  }) => {
    const cspErrors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error' && /Content Security Policy/i.test(msg.text())) {
        cspErrors.push(msg.text());
      }
    });
    page.on('pageerror', (err) => {
      if (/Content Security Policy/i.test(err.message)) {
        cspErrors.push(err.message);
      }
    });

    // =========================================================================
    // Step 1 — Navigate to /k9-ops & Verify Seeded Missions & Plume Radar
    // =========================================================================
    await page.goto(`${WEB_FRONTEND_URL}/k9-ops`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#k9-ops-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Canine (K9) Scent-Trailing Telemetry, Wind Plume Dispersion & Handler Cockpit'
    );

    const card101 = page.locator(
      '#k9-missions-list .k9-mission-card[data-mission-id="mission-k9-101"]'
    );
    const card102 = page.locator(
      '#k9-missions-list .k9-mission-card[data-mission-id="mission-k9-102"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toContainText('K9 Atlas');
    await expect(card101).toHaveAttribute('data-viability-rating', 'HIGH_VIABILITY');

    await expect(card102).toBeVisible();
    await expect(card102).toContainText('K9 Freya');
    await expect(card102).toHaveAttribute('data-viability-rating', 'DEGRADED_THERMAL_HAZARD');
    await expect(card102.locator('.k9-paw-hazard-banner')).toBeVisible();
    await expect(card102.locator('.k9-paw-hazard-banner')).toContainText('THERMAL PAW HAZARD');

    const plumeSvg = page.locator('#k9-plume-svg');
    const plumePolygon = page.locator('#svg-plume-polygon');
    await expect(plumeSvg).toBeVisible();
    await expect(plumePolygon).toBeVisible();
    const polygonPoints = await plumePolygon.getAttribute('points');
    expect(polygonPoints).toBeTruthy();
    expect(polygonPoints!.trim().length).toBeGreaterThan(0);

    expect(cspErrors).toEqual([]);

    // =========================================================================
    // Step 2 — Dispatch New K9 Mantrailing Mission with Atmospheric Telemetry
    // =========================================================================
    const uniqueSuffix = Date.now();
    const petId = `pet-e2e-k9-timber-${uniqueSuffix}`;
    const handlerName = 'Capt. Elena Rostova';
    const canineName = `K9 Timber #${uniqueSuffix}`;
    const canineCallsign = 'K9-TIMBER-7';

    await page.locator('#k9-pet-id').fill(petId);
    await page.locator('#k9-handler-name').fill(handlerName);
    await page.locator('#k9-canine-name').fill(canineName);
    await page.locator('#k9-canine-callsign').fill(canineCallsign);
    await page.locator('#k9-discipline').selectOption('MANTRAILING_SCENT_SPECIFIC');
    await page
      .locator('#k9-scent-article')
      .fill('Unwashed fleece collar sealed in sterile glass jar');
    await page.locator('#k9-lkp-lat').fill('47.6062');
    await page.locator('#k9-lkp-lng').fill('-122.3321');
    await page.locator('#k9-wind-speed').fill('14');
    await page.locator('#k9-wind-from-deg').fill('270');
    await page.locator('#k9-temp-c').fill('14');
    await page.locator('#k9-humidity-pct').fill('78');
    await page.locator('#k9-ground-surface').selectOption('LUSH_VEGETATION');
    await page.locator('#k9-trail-age-hours').fill('2.0');

    await page.locator('#btn-create-k9-mission').click();

    const newMissionCard = page.locator('#k9-missions-list .k9-mission-card', {
      hasText: canineName,
    });
    await expect(newMissionCard).toBeVisible();
    await expect(newMissionCard).toHaveAttribute('data-viability-rating', 'HIGH_VIABILITY');
    await expect(newMissionCard.locator('.badge-viability-high')).toContainText('HIGH_VIABILITY');
    await expect(newMissionCard).toContainText(handlerName);
    await expect(newMissionCard).toContainText(canineCallsign);

    const newMissionId = await newMissionCard.getAttribute('data-mission-id');
    expect(newMissionId).toBeTruthy();

    await expect(page.locator('#plume-azimuth-badge')).toContainText('Downwind: 90°');
    await expect(plumePolygon).toHaveAttribute('data-mission-id', newMissionId!);

    // Record initial POD before logging in-plume waypoints
    const initialCardPodText = await newMissionCard.locator('.badge-k9-pod').innerText();
    const initialCardPod = parseFloat(initialCardPodText.replace(/[^0-9.]/g, ''));
    const initialVisualizerPodText = await page.locator('#mission-pod-badge').innerText();
    const initialVisualizerPod = parseFloat(initialVisualizerPodText.replace(/[^0-9.]/g, ''));

    // =========================================================================
    // Step 3 — Record Downwind In-Plume K9 Waypoints & Behavioral Alerts
    // =========================================================================
    await page.locator('#btn-sim-in-plume-alert').click();

    const inPlumeWaypoint = page
      .locator('#k9-waypoints-list .k9-waypoint-item[data-in-plume="true"]')
      .first();
    await expect(inPlumeWaypoint).toBeVisible();
    await expect(inPlumeWaypoint).toContainText('IN PLUME CORRIDOR');
    await expect(inPlumeWaypoint.locator('.badge-k9-behavior')).toContainText('STRONG_SCENT_PULL');

    // =========================================================================
    // Step 4 — Verify Sighting Promotion & POD Progression
    // =========================================================================
    const sightingBanner = newMissionCard.locator('.k9-promoted-sighting-banner');
    await expect(sightingBanner).toBeVisible();
    await expect(sightingBanner).toContainText(/sighting-k9-/);

    const updatedCardPodText = await newMissionCard.locator('.badge-k9-pod').innerText();
    const updatedCardPod = parseFloat(updatedCardPodText.replace(/[^0-9.]/g, ''));
    expect(updatedCardPod).toBeGreaterThan(initialCardPod);

    const updatedVisualizerPodText = await page.locator('#mission-pod-badge').innerText();
    const updatedVisualizerPod = parseFloat(updatedVisualizerPodText.replace(/[^0-9.]/g, ''));
    expect(updatedVisualizerPod).toBeGreaterThan(initialVisualizerPod);

    // =========================================================================
    // Step 5 — Inspect Thermal Hazard Mission (mission-k9-102) in Plume Visualizer
    // =========================================================================
    await card102.locator('.btn-select-k9-mission').click();

    const visualizerPawBanner = page.locator('#paw-hazard-banner');
    await expect(visualizerPawBanner).toBeVisible();
    await expect(visualizerPawBanner).toContainText('THERMAL PAW HAZARD');
    await expect(visualizerPawBanner).toContainText(/asphalt|booties/i);

    // =========================================================================
    // Step 6 — Verify GPX 1.1 Telemetry Export
    // =========================================================================
    const gpxRes = await request.get(
      `${WEB_FRONTEND_URL}/api/v1/k9/missions/${encodeURIComponent(newMissionId!)}/gpx`
    );
    expect(gpxRes.status()).toBe(200);
    expect(gpxRes.headers()['content-type']).toContain('application/gpx+xml');

    const gpxBody = await gpxRes.text();
    expect(gpxBody).toContain('<gpx version="1.1"');
    expect(gpxBody).toContain('<wpt');
    expect(gpxBody).toContain('<trkpt');
    expect(gpxBody).toContain('STRONG_SCENT_PULL');

    expect(cspErrors).toEqual([]);
  });
});
