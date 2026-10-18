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

test.describe('Humane Live-Trap IoT Telemetry & Rapid Dispatch Cockpit UI (/humane-traps)', () => {
  test('1. /humane-traps renders seeded traps (trap-ht-101, trap-ht-102, trap-ht-103), bait freshness badges, alert flags, and 15-minute SLA banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/humane-traps`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#humane-traps-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Humane Live-Trap IoT Telemetry & Rapid Dispatch Cockpit'
    );

    await expect(page.locator('#kpi-traps-total')).toBeVisible();
    await expect(page.locator('#kpi-traps-urgent')).toBeVisible();
    await expect(page.locator('#kpi-traps-captured')).toBeVisible();
    await expect(page.locator('#kpi-traps-thermal')).toBeVisible();

    const card101 = page.locator('#humane-traps-list .humane-trap-card[data-trap-id="trap-ht-101"]');
    const card102 = page.locator('#humane-traps-list .humane-trap-card[data-trap-id="trap-ht-102"]');
    const card103 = page.locator('#humane-traps-list .humane-trap-card[data-trap-id="trap-ht-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-trap-status', 'DOOR_TRIPPED_URGENT');
    await expect(card101.locator('.badge-trap-status.badge-trap-tripped')).toContainText('DOOR_TRIPPED_URGENT');
    await expect(card101.locator('.badge-bait-state.badge-bait-fresh')).toContainText('Bait: 88% (FRESH_PEAK_SCENT)');
    await expect(card101.locator('.badge-trap-alert[data-flag="URGENT_15MIN_CHECK_SLA"]')).toBeVisible();
    await expect(card101.locator('.trap-sla-urgent-banner')).toContainText('15-MIN HUMANE CHECK SLA ACTIVE');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-trap-status', 'ARMED_ACTIVE');
    await expect(card102.locator('.badge-trap-status.badge-trap-armed')).toContainText('ARMED_ACTIVE');
    await expect(card102.locator('.badge-bait-state.badge-bait-degrading')).toContainText('Bait: 42% (DEGRADING_REFRESH_SOON)');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-trap-status', 'THERMAL_LOCKOUT_DISARMED');
    await expect(card103.locator('.badge-trap-status.badge-trap-thermal')).toContainText('THERMAL_LOCKOUT_DISARMED');
    await expect(card103.locator('.badge-bait-state.badge-bait-spoiled')).toContainText('Bait: 12% (SPOILED_EXPIRED)');
    await expect(card103.locator('.badge-trap-alert[data-flag="HYPERTHERMIA_HEAT_HAZARD"]')).toBeVisible();
    await expect(card103.locator('.badge-trap-alert[data-flag="BAIT_SPOILED_REFRESH_REQUIRED"]')).toBeVisible();

    await expect(page.locator('#form-deploy-humane-trap')).toBeVisible();
    await expect(page.locator('#btn-sim-door-trip-target')).toBeVisible();
    await expect(page.locator('#btn-sim-bycatch-trip')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Deploying a new humane trap via #form-deploy-humane-trap creates an ARMED_ACTIVE card with FRESH_PEAK_SCENT (100% bait freshness)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/humane-traps`);
    await page.waitForLoadState('networkidle');

    const uniqueSuffix = Date.now();
    const trapLabel = `Tomahawk Tru-Catch #415 — Magnolia Bluff Ravine (${uniqueSuffix})`;

    await page.locator('#ht-pet-id').fill('pet-2026-002');
    await page.locator('#ht-pet-name').fill('Luna');
    await page.locator('#ht-target-species').selectOption('CAT');
    await page.locator('#ht-target-weight-kg').fill('4.2');
    await page.locator('#ht-trap-label').fill(trapLabel);
    await page.locator('#ht-trap-type').selectOption('BOX_TRAP_GRAVITY');
    await page.locator('#ht-bait-type').selectOption('SARDINES_IN_OIL');
    await page.locator('#ht-latitude').fill('47.6632');
    await page.locator('#ht-longitude').fill('-122.4085');
    await page.locator('#ht-sector-id').fill('SEC-W-03');
    await page.locator('#ht-internal-temp-c').fill('17.5');

    await page.locator('#btn-deploy-humane-trap').click();

    const newCard = page.locator('#humane-traps-list .humane-trap-card', { hasText: trapLabel });
    await expect(newCard).toBeVisible();
    await expect(newCard).toHaveAttribute('data-trap-status', 'ARMED_ACTIVE');
    await expect(newCard.locator('.badge-trap-status.badge-trap-armed')).toContainText('ARMED_ACTIVE');
    await expect(newCard.locator('.badge-bait-state.badge-bait-fresh')).toContainText(
      'Bait: 100% (FRESH_PEAK_SCENT)'
    );
  });

  test('3. Dispatching a responder on a tripped trap transitions it to RESPONDER_DISPATCHED, and resolving as TARGET_PET_CAPTURED transitions to TARGET_CAPTURED_SECURED and displays .trap-promoted-sighting-banner (sighting-trap-...)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/humane-traps`);
    await page.waitForLoadState('networkidle');

    const uniqueSuffix = Date.now();
    const trapLabel = `Tomahawk Capture Test #${uniqueSuffix}`;

    await page.locator('#ht-trap-label').fill(trapLabel);
    await page.locator('#btn-deploy-humane-trap').click();

    const targetCard = page.locator('#humane-traps-list .humane-trap-card', { hasText: trapLabel });
    await expect(targetCard).toBeVisible();
    await expect(targetCard).toHaveAttribute('data-trap-status', 'ARMED_ACTIVE');

    // Simulate door trip on the first ARMED_ACTIVE trap (our newly deployed trap)
    await page.locator('#btn-sim-door-trip-target').click();
    await expect(targetCard).toHaveAttribute('data-trap-status', 'DOOR_TRIPPED_URGENT');
    await expect(targetCard.locator('.trap-sla-urgent-banner')).toContainText('15-MIN HUMANE CHECK SLA ACTIVE');

    // Dispatch responder
    await targetCard.locator('.btn-dispatch-trap-responder').click();
    await expect(targetCard).toHaveAttribute('data-trap-status', 'RESPONDER_DISPATCHED');
    await expect(targetCard.locator('.badge-trap-status.badge-trap-dispatched')).toContainText('RESPONDER_DISPATCHED');
    await expect(targetCard.locator('.trap-sla-urgent-banner')).toContainText('Maya Lin (SAR Tech)');

    // Resolve as TARGET_PET_CAPTURED
    await targetCard.locator('.btn-resolve-target-captured').click();
    await expect(targetCard).toHaveAttribute('data-trap-status', 'TARGET_CAPTURED_SECURED');
    await expect(targetCard.locator('.badge-trap-status.badge-trap-captured')).toContainText('TARGET_CAPTURED_SECURED');
    await expect(targetCard.locator('.trap-promoted-sighting-banner')).toBeVisible();
    await expect(targetCard.locator('.trap-promoted-sighting-banner')).toContainText('Promoted Sighting: sighting-trap-');
  });

  test('4. Simulating a wildlife bycatch trip (#btn-sim-bycatch-trip) flags WEIGHT_MISMATCH_POSSIBLE_BYCATCH, and clicking .btn-resolve-bycatch-rearm resets the trap to ARMED_ACTIVE with 100% bait freshness', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/humane-traps`);
    await page.waitForLoadState('networkidle');

    const uniqueSuffix = Date.now();
    const trapLabel = `Tomahawk Bycatch Test #${uniqueSuffix}`;

    await page.locator('#ht-trap-label').fill(trapLabel);
    await page.locator('#btn-deploy-humane-trap').click();

    const bycatchCard = page.locator('#humane-traps-list .humane-trap-card', { hasText: trapLabel });
    await expect(bycatchCard).toBeVisible();
    await expect(bycatchCard).toHaveAttribute('data-trap-status', 'ARMED_ACTIVE');

    await page.locator('#btn-sim-bycatch-trip').click();

    await expect(bycatchCard).toHaveAttribute('data-trap-status', 'DOOR_TRIPPED_URGENT');
    await expect(bycatchCard.locator('.badge-trap-alert[data-flag="WEIGHT_MISMATCH_POSSIBLE_BYCATCH"]')).toBeVisible();
    await expect(bycatchCard.locator('.badge-trap-alert[data-flag="URGENT_15MIN_CHECK_SLA"]')).toBeVisible();

    await bycatchCard.locator('.btn-resolve-bycatch-rearm').click();

    await expect(bycatchCard).toHaveAttribute('data-trap-status', 'ARMED_ACTIVE');
    await expect(bycatchCard.locator('.badge-trap-status.badge-trap-armed')).toContainText('ARMED_ACTIVE');
    await expect(bycatchCard.locator('.badge-bait-state.badge-bait-fresh')).toContainText(
      'Bait: 100% (FRESH_PEAK_SCENT)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7:1) on .badge-trap-armed, .badge-trap-tripped, .badge-trap-dispatched, .badge-trap-captured, .badge-trap-thermal, .badge-trap-alert, and .trap-sla-urgent-banner in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/humane-traps`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-humane-trap-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-trap-armed" class="badge badge-trap-status badge-trap-armed">ARMED_ACTIVE</span>
        <span id="test-badge-trap-tripped" class="badge badge-trap-status badge-trap-tripped">DOOR_TRIPPED_URGENT</span>
        <span id="test-badge-trap-dispatched" class="badge badge-trap-status badge-trap-dispatched">RESPONDER_DISPATCHED</span>
        <span id="test-badge-trap-captured" class="badge badge-trap-status badge-trap-captured">TARGET_CAPTURED_SECURED</span>
        <span id="test-badge-trap-thermal" class="badge badge-trap-status badge-trap-thermal">THERMAL_LOCKOUT_DISARMED</span>
        <span id="test-badge-trap-alert" class="badge badge-trap-alert">URGENT_15MIN_CHECK_SLA</span>
        <div id="test-trap-sla-urgent-banner" class="trap-sla-urgent-banner">15-MIN HUMANE CHECK SLA ACTIVE</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-trap-armed',
        '#test-badge-trap-tripped',
        '#test-badge-trap-dispatched',
        '#test-badge-trap-captured',
        '#test-badge-trap-thermal',
        '#test-badge-trap-alert',
        '#test-trap-sla-urgent-banner',
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
