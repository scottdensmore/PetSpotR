import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

/**
 * Helper to parse rgb/rgba string into [r, g, b]
 */
function parseRgb(rgbStr: string): [number, number, number] {
  const match = rgbStr.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i);
  if (!match) return [0, 0, 0];
  return [parseInt(match[1], 10), parseInt(match[2], 10), parseInt(match[3], 10)];
}

/**
 * Relative luminance per WCAG 2.1
 */
function relativeLuminance([r, g, b]: [number, number, number]): number {
  const sRGB = [r / 255, g / 255, b / 255];
  const [R, G, B] = sRGB.map((c) =>
    c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)
  );
  return 0.2126 * R + 0.7152 * G + 0.0722 * B;
}

/**
 * Contrast ratio per WCAG 2.1
 */
function contrastRatio(fgStr: string, bgStr: string): number {
  const l1 = relativeLuminance(parseRgb(fgStr));
  const l2 = relativeLuminance(parseRgb(bgStr));
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

test.describe('Canine (K9) Scent-Trailing Telemetry, Atmospheric Wind Plume Dispersion & Handler Cockpit UI', () => {
  test('1. /k9-ops renders seeded K9 missions (mission-k9-101, mission-k9-102), SVG Gaussian wind plume visualizer (#k9-plume-svg, #svg-plume-polygon), and paw thermal hazard banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/k9-ops`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#k9-ops-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Canine (K9) Scent-Trailing Telemetry, Wind Plume Dispersion & Handler Cockpit'
    );

    const card101 = page.locator('#k9-missions-list .k9-mission-card[data-mission-id="mission-k9-101"]');
    const card102 = page.locator('#k9-missions-list .k9-mission-card[data-mission-id="mission-k9-102"]');
    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-viability-rating', 'HIGH_VIABILITY');
    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-viability-rating', 'DEGRADED_THERMAL_HAZARD');

    // Paw thermal hazard banner on mission-k9-102 card and in visualizer when mission-k9-102 is inspected
    await expect(card102.locator('.k9-paw-hazard-banner')).toBeVisible();
    await expect(card102.locator('.k9-paw-hazard-banner')).toContainText('THERMAL PAW HAZARD');

    await expect(page.locator('#k9-plume-svg')).toBeVisible();
    await expect(page.locator('#svg-plume-polygon')).toBeVisible();
    await expect(page.locator('#svg-lkp-origin')).toBeVisible();

    // Clicking Inspect Plume on mission-k9-102 also reveals #paw-hazard-banner inside the visualizer
    await card102.locator('.btn-select-k9-mission').click();
    await expect(page.locator('#paw-hazard-banner')).toBeVisible();
    await expect(page.locator('#paw-hazard-banner')).toContainText('THERMAL PAW HAZARD');

    expect(cspErrors).toEqual([]);
  });

  test('2. Launching a new K9 scent mission via #form-create-k9-mission adds the mission card to #k9-missions-list, updates #k9-plume-svg, and computes HIGH_VIABILITY', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/k9-ops`);
    await page.waitForLoadState('networkidle');

    const uniqueSuffix = Date.now();
    const canineName = `K9 Kodiak #${uniqueSuffix}`;
    const callsign = `K9-KODIAK-${uniqueSuffix % 1000}`;

    await page.locator('#k9-pet-id').fill(`pet-lost-luna-${uniqueSuffix}`);
    await page.locator('#k9-handler-name').fill('Sierra Vance');
    await page.locator('#k9-canine-name').fill(canineName);
    await page.locator('#k9-canine-callsign').fill(callsign);
    await page.locator('#k9-discipline').selectOption('MANTRAILING_SCENT_SPECIFIC');
    await page.locator('#k9-scent-article').fill('Unwashed fleece harness in sterile Mason jar');
    await page.locator('#k9-lkp-lat').fill('47.6062');
    await page.locator('#k9-lkp-lng').fill('-122.3321');
    await page.locator('#k9-wind-speed').fill('14');
    await page.locator('#k9-wind-from-deg').fill('270');
    await page.locator('#k9-temp-c').fill('15');
    await page.locator('#k9-humidity-pct').fill('74');
    await page.locator('#k9-ground-surface').selectOption('LUSH_VEGETATION');
    await page.locator('#k9-trail-age-hours').fill('2.5');

    await page.locator('#btn-create-k9-mission').click();

    const newCard = page.locator('#k9-missions-list .k9-mission-card', { hasText: canineName });
    await expect(newCard).toBeVisible();
    await expect(newCard).toHaveAttribute('data-viability-rating', 'HIGH_VIABILITY');
    await expect(newCard.locator('.badge-viability-high')).toContainText('HIGH_VIABILITY');

    const newMissionId = await newCard.getAttribute('data-mission-id');
    expect(newMissionId).toBeTruthy();

    await expect(page.locator('#svg-plume-polygon')).toHaveAttribute('data-mission-id', newMissionId!);
    await expect(page.locator('#plume-azimuth-badge')).toContainText('Downwind: 90°');
  });

  test('3. Logging a downwind K9 waypoint via #btn-sim-in-plume-alert adds an IN PLUME CORRIDOR waypoint to #k9-waypoints-list, increases EstimatedPOD, and displays .k9-promoted-sighting-banner', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/k9-ops`);
    await page.waitForLoadState('networkidle');

    // First create a fresh K9 mission with 0 waypoints so we can observe POD increase cleanly
    const uniqueSuffix = Date.now();
    const canineName = `K9 Zephyr #${uniqueSuffix}`;
    await page.locator('#k9-canine-name').fill(canineName);
    await page.locator('#k9-canine-callsign').fill(`K9-ZEPHYR-${uniqueSuffix % 1000}`);
    await page.locator('#btn-create-k9-mission').click();

    const missionCard = page.locator('#k9-missions-list .k9-mission-card', { hasText: canineName });
    await expect(missionCard).toBeVisible();

    const initialPodText = await missionCard.locator('.badge-k9-pod').innerText();
    const initialPod = parseFloat(initialPodText.replace(/[^0-9.]/g, ''));

    // Trigger #btn-sim-in-plume-alert
    await page.locator('#btn-sim-in-plume-alert').click();

    // Verify waypoint added with IN PLUME CORRIDOR
    const wpItem = page.locator('#k9-waypoints-list .k9-waypoint-item[data-in-plume="true"]').first();
    await expect(wpItem).toBeVisible();
    await expect(wpItem).toContainText('IN PLUME CORRIDOR');
    await expect(wpItem.locator('.badge-k9-behavior')).toContainText('STRONG_SCENT_PULL');

    // Verify Promoted Sighting banner on mission card
    await expect(missionCard.locator('.k9-promoted-sighting-banner')).toBeVisible();
    await expect(missionCard.locator('.k9-promoted-sighting-banner')).toContainText('sighting-k9-');

    // Verify EstimatedPOD increased
    const updatedPodText = await missionCard.locator('.badge-k9-pod').innerText();
    const updatedPod = parseFloat(updatedPodText.replace(/[^0-9.]/g, ''));
    expect(updatedPod).toBeGreaterThan(initialPod);
  });

  test('4. GPX 1.1 export endpoint /api/v1/k9/missions/mission-k9-101/gpx returns 200 OK with valid <gpx version="1.1"> XML containing <wpt> and <trkpt>', async ({ request }) => {
    const res = await request.get(`${WEB_FRONTEND_URL}/api/v1/k9/missions/mission-k9-101/gpx`);
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toContain('application/gpx+xml');

    const body = await res.text();
    expect(body).toContain('<gpx version="1.1"');
    expect(body).toContain('<wpt');
    expect(body).toContain('<trkpt');
  });

  test('5. WCAG AAA (> 7:1) contrast ratio check on .badge-viability-high, .badge-viability-moderate, .badge-viability-degraded, .badge-viability-expired, and .badge-k9-behavior in both dark and light modes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/k9-ops`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-k9-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-viability-high" class="badge badge-viability-rating badge-viability-high">HIGH_VIABILITY</span>
        <span id="test-badge-viability-moderate" class="badge badge-viability-rating badge-viability-moderate">MODERATE_VIABILITY</span>
        <span id="test-badge-viability-degraded" class="badge badge-viability-rating badge-viability-degraded">DEGRADED_THERMAL_HAZARD</span>
        <span id="test-badge-viability-expired" class="badge badge-viability-rating badge-viability-expired">EXPIRED_SCENT_TRAIL</span>
        <span id="test-badge-k9-behavior" class="badge badge-k9-behavior">STRONG_SCENT_PULL</span>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-viability-high',
        '#test-badge-viability-moderate',
        '#test-badge-viability-degraded',
        '#test-badge-viability-expired',
        '#test-badge-k9-behavior',
      ]) {
        const styles = await page.locator(selector).evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { color: cs.color, backgroundColor: cs.backgroundColor };
        });
        expect(styles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
        const ratio = contrastRatio(styles.color, styles.backgroundColor);
        expect(ratio, `${selector} in ${theme} mode should have > 7:1 contrast`).toBeGreaterThan(7.0);
      }
    }
  });
});
