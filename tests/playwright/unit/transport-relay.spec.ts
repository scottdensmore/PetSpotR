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

test.describe('Aviation & Cross-Jurisdiction Pet Airlift / Ground Relay Transport Logistics Cockpit UI (/transport-relay)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /transport-relay renders seeded missions (relay-msn-101, relay-msn-102, relay-msn-103), leg checklist items, and hypoxia cabin alert banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/transport-relay`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#transport-relay-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Aviation & Cross-Jurisdiction Pet Airlift / Ground Relay Transport Logistics Cockpit'
    );

    await expect(page.locator('#kpi-relay-total')).toBeVisible();
    await expect(page.locator('#kpi-relay-hypoxia')).toBeVisible();
    await expect(page.locator('#kpi-relay-intransit')).toBeVisible();
    await expect(page.locator('#kpi-relay-delivered')).toBeVisible();
    await expect(page.locator('#transport-relay-live-status')).toBeVisible();

    const card101 = page.locator('#relay-missions-list .relay-mission-card[data-mission-id="relay-msn-101"]');
    const card102 = page.locator('#relay-missions-list .relay-mission-card[data-mission-id="relay-msn-102"]');
    const card103 = page.locator('#relay-missions-list .relay-mission-card[data-mission-id="relay-msn-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-relay-status', 'PRE_FLIGHT_STAGING');
    await expect(card101.locator('.badge-relay-status.badge-relay-staging')).toContainText('PRE_FLIGHT_STAGING');
    await expect(card101.locator('.badge-thermal-risk.badge-thermal-hypoxia')).toContainText('BRACHYCEPHALIC_CABIN_RISK');
    await expect(card101.locator('.relay-hypoxia-alert-banner')).toContainText('BRACHYCEPHALIC & ALTITUDE HYPOXIA ALERT');
    await expect(card101.locator('.relay-legs-list')).toContainText('USDA_APHIS_CVI_CLEARANCE');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-relay-status', 'IN_TRANSIT_ACTIVE_RELAY');
    await expect(card102.locator('.badge-relay-status.badge-relay-transit')).toContainText('IN_TRANSIT_ACTIVE_RELAY');
    await expect(card102.locator('.badge-thermal-risk.badge-thermal-watch')).toContainText('TARMAC_THERMAL_WATCH');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-relay-status', 'DELIVERED_DESTINATION_VERIFIED');
    await expect(card103.locator('.badge-relay-status.badge-relay-delivered')).toContainText('DELIVERED_DESTINATION_VERIFIED');
    await expect(card103.locator('.badge-thermal-risk.badge-thermal-safe')).toContainText('PRESSURIZED_SAFE_ENVELOPE');
    await expect(card103.locator('.relay-manifest-banner')).toContainText('INTERSTATE RELAY MANIFEST VERIFIED: RELAY-MANIFEST-RELAY-MSN-103');

    await expect(page.locator('#form-create-relay-mission')).toBeVisible();
    await expect(page.locator('#btn-sim-brachy-airlift')).toBeVisible();
    await expect(page.locator('#btn-sim-ground-relay')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Simulating a Brachycephalic French Bulldog airlift (#btn-sim-brachy-airlift) creates a PRE_FLIGHT_STAGING card with .relay-hypoxia-alert-banner and BRACHYCEPHALIC_CABIN_RISK', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/transport-relay`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-brachy-airlift').click();

    const simCard = page.locator('#relay-missions-list .relay-mission-card', {
      hasText: 'Louie',
    }).first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-relay-status', 'PRE_FLIGHT_STAGING');
    await expect(simCard.locator('.badge-relay-status.badge-relay-staging')).toContainText('PRE_FLIGHT_STAGING');
    await expect(simCard.locator('.badge-thermal-risk.badge-thermal-hypoxia')).toContainText('BRACHYCEPHALIC_CABIN_RISK');
    await expect(simCard.locator('.relay-hypoxia-alert-banner')).toContainText('BRACHYCEPHALIC & ALTITUDE HYPOXIA ALERT');
  });

  test('3. Clicking .btn-launch-relay-leg clears USDA CVI and IATA crate (Flight Readiness: 100/100) and transitions mission to IN_TRANSIT_ACTIVE_RELAY', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/transport-relay`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-ground-relay').click();

    const groundCard = page.locator('#relay-missions-list .relay-mission-card', {
      hasText: 'Maple',
    }).first();
    await expect(groundCard).toBeVisible();
    await expect(groundCard).toHaveAttribute('data-relay-status', 'PRE_FLIGHT_STAGING');

    await groundCard.locator('.btn-launch-relay-leg').click();

    await expect(groundCard).toHaveAttribute('data-relay-status', 'IN_TRANSIT_ACTIVE_RELAY');
    await expect(groundCard.locator('.badge-relay-status.badge-relay-transit')).toContainText('IN_TRANSIT_ACTIVE_RELAY');
    await expect(groundCard.locator('.badge-relay-metric')).toContainText('Flight Readiness: 100/100');
    await expect(groundCard.locator('.badge-thermal-risk.badge-thermal-watch')).toContainText('TARMAC_THERMAL_WATCH');
  });

  test('4. Clicking .btn-complete-relay-delivery transitions a mission to DELIVERED_DESTINATION_VERIFIED (PRESSURIZED_SAFE_ENVELOPE) and displays .relay-manifest-banner (RELAY-MANIFEST-...)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/transport-relay`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-ground-relay').click();

    const card = page.locator('#relay-missions-list .relay-mission-card', {
      hasText: 'Maple',
    }).first();
    await expect(card).toBeVisible();

    await card.locator('.btn-complete-relay-delivery').click();

    await expect(card).toHaveAttribute('data-relay-status', 'DELIVERED_DESTINATION_VERIFIED');
    await expect(card.locator('.badge-relay-status.badge-relay-delivered')).toContainText('DELIVERED_DESTINATION_VERIFIED');
    await expect(card.locator('.badge-thermal-risk.badge-thermal-safe')).toContainText('PRESSURIZED_SAFE_ENVELOPE');
    await expect(card.locator('.relay-manifest-banner')).toBeVisible();
    await expect(card.locator('.relay-manifest-banner')).toContainText('INTERSTATE RELAY MANIFEST VERIFIED: RELAY-MANIFEST-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7:1) on .badge-relay-staging, .badge-relay-transit, .badge-relay-delivered, .badge-thermal-hypoxia, .badge-thermal-watch, .badge-thermal-safe, .badge-relay-metric, .relay-hypoxia-alert-banner, and .relay-manifest-banner in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/transport-relay`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-transport-relay-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-relay-staging" class="badge badge-relay-status badge-relay-staging">PRE_FLIGHT_STAGING</span>
        <span id="test-badge-relay-transit" class="badge badge-relay-status badge-relay-transit">IN_TRANSIT_ACTIVE_RELAY</span>
        <span id="test-badge-relay-delivered" class="badge badge-relay-status badge-relay-delivered">DELIVERED_DESTINATION_VERIFIED</span>
        <span id="test-badge-thermal-hypoxia" class="badge badge-thermal-risk badge-thermal-hypoxia">BRACHYCEPHALIC_CABIN_RISK</span>
        <span id="test-badge-thermal-watch" class="badge badge-thermal-risk badge-thermal-watch">TARMAC_THERMAL_WATCH</span>
        <span id="test-badge-thermal-safe" class="badge badge-thermal-risk badge-thermal-safe">PRESSURIZED_SAFE_ENVELOPE</span>
        <span id="test-badge-relay-metric" class="badge badge-relay-metric">Transport Stress: 45/100 • Flight Readiness: 100/100</span>
        <div id="test-relay-hypoxia-alert-banner" class="relay-hypoxia-alert-banner">BRACHYCEPHALIC & ALTITUDE HYPOXIA ALERT</div>
        <div id="test-relay-manifest-banner" class="relay-manifest-banner">INTERSTATE RELAY MANIFEST VERIFIED: RELAY-MANIFEST-101</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-relay-staging',
        '#test-badge-relay-transit',
        '#test-badge-relay-delivered',
        '#test-badge-thermal-hypoxia',
        '#test-badge-thermal-watch',
        '#test-badge-thermal-safe',
        '#test-badge-relay-metric',
        '#test-relay-hypoxia-alert-banner',
        '#test-relay-manifest-banner',
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
