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

test.describe('Maritime, Coastal & Tidal Mudflat Pet Rescue Operations Cockpit UI (/maritime-coastal-rescue)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /maritime-coastal-rescue renders seeded operations (MC-OP-101, MC-OP-102, MC-OP-103), KPI telemetry bar, active workspace, protocol steps, and tidal alert banner with zero CSP errors', async ({
    page,
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

    await page.goto(`${WEB_FRONTEND_URL}/maritime-coastal-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#maritime-coastal-rescue-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Maritime, Coastal & Tidal Mudflat Pet Rescue Operations Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-tide')).toBeVisible();
    await expect(page.locator('#kpi-staged-vessel')).toBeVisible();
    await expect(page.locator('#kpi-extracted-safe')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const card101 = page.locator('#maritime-operations-list .mc-op-card[data-op-id="MC-OP-101"]', {
      hasText: 'Anchor (',
    });
    const card102 = page.locator('#maritime-operations-list .mc-op-card[data-op-id="MC-OP-102"]', {
      hasText: 'Coral (',
    });
    const card103 = page.locator('#maritime-operations-list .mc-op-card[data-op-id="MC-OP-103"]', {
      hasText: 'Skipper (',
    });

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-status', 'TIDAL_FLOOD_ASPIRATION_CRITICAL');
    await expect(card101.locator('.badge-mc-status.badge-mc-critical')).toContainText(
      'TIDAL_FLOOD_ASPIRATION_CRITICAL'
    );
    await expect(card101.locator('.badge-mc-zone').first()).toContainText(
      'TIDAL_MUDFLAT_ESTUARY'
    );
    await expect(card101.locator('.mc-tidal-alert-banner')).toContainText(
      'CRITICAL TIDAL / ASPIRATION EMERGENCY'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-status', 'HOVERCRAFT_MUD_SLED_STAGED');
    await expect(card102.locator('.badge-mc-status.badge-mc-staged')).toContainText(
      'HOVERCRAFT_MUD_SLED_STAGED'
    );

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-status', 'EXTRACTED_HARBOR_DECK_SAFE');
    await expect(card103.locator('.badge-mc-status.badge-mc-safe')).toContainText(
      'EXTRACTED_HARBOR_DECK_SAFE'
    );
    await expect(card103.locator('.mc-clearance-banner')).toContainText('MARITIME-SAFE-');

    expect(cspErrors).toEqual([]);
  });

  test('2. Clicking .mc-scenario-btn[data-scenario="tidal-mudflat-entrapment"] evaluates a tidal mudflat suction & saltwater aspiration emergency', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/maritime-coastal-rescue`);
    await page.waitForLoadState('networkidle');

    await page.locator('.mc-scenario-btn[data-scenario="tidal-mudflat-entrapment"]').click();

    const simCard = page
      .locator('#maritime-operations-list .mc-op-card', {
        hasText: 'Barnacle (',
      })
      .first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-status', 'TIDAL_FLOOD_ASPIRATION_CRITICAL');
    await expect(simCard.locator('.badge-mc-status.badge-mc-critical')).toContainText(
      'TIDAL_FLOOD_ASPIRATION_CRITICAL'
    );
    await expect(simCard.locator('.mc-tidal-alert-banner')).toContainText(
      'CRITICAL TIDAL / ASPIRATION EMERGENCY'
    );
  });

  test('3. Toggling .mc-step-checkbox on MC-OP-101 completes protocol steps and transitions status to HOVERCRAFT_MUD_SLED_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/maritime-coastal-rescue`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#maritime-operations-list .mc-op-card[data-op-id="MC-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-TIDE-SUCTION-RELIEF',
      'STEP-VESSEL-PFD-RIGGING',
      'STEP-SALTWATER-ASPIRATION-O2',
      'STEP-HARBOR-DECK-DECON-WARM',
    ]) {
      const cb = card101.locator(`.mc-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'HOVERCRAFT_MUD_SLED_STAGED');
    await expect(card101.locator('.badge-mc-status.badge-mc-staged')).toContainText(
      'HOVERCRAFT_MUD_SLED_STAGED'
    );
  });

  test('4. Clicking .mc-issue-clearance-btn issues a MARITIME-SAFE-<12HEX> token and transitions operation to EXTRACTED_HARBOR_DECK_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/maritime-coastal-rescue`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#maritime-operations-list .mc-op-card[data-op-id="MC-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.mc-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'EXTRACTED_HARBOR_DECK_SAFE');
    await expect(card102.locator('.badge-mc-status.badge-mc-safe')).toContainText(
      'EXTRACTED_HARBOR_DECK_SAFE'
    );
    await expect(card102.locator('.mc-clearance-banner')).toBeVisible();
    await expect(card102.locator('.mc-clearance-banner')).toContainText(
      'HARBOR DECK EXTRACTION CLEARANCE VERIFIED: MARITIME-SAFE-'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-mc-critical, .badge-mc-staged, .badge-mc-safe, .badge-mc-zone, .badge-mc-flag, .mc-tidal-alert-banner, and .mc-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/maritime-coastal-rescue`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-maritime-rescue-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-mc-critical" class="badge badge-mc-status badge-mc-critical">TIDAL_FLOOD_ASPIRATION_CRITICAL</span>
        <span id="test-badge-mc-staged" class="badge badge-mc-status badge-mc-staged">HOVERCRAFT_MUD_SLED_STAGED</span>
        <span id="test-badge-mc-safe" class="badge badge-mc-status badge-mc-safe">EXTRACTED_HARBOR_DECK_SAFE</span>
        <span id="test-badge-mc-zone" class="badge badge-mc-zone">TIDAL_MUDFLAT_ESTUARY</span>
        <span id="test-badge-mc-flag" class="badge badge-mc-flag">SALTWATER_ASPIRATION_SUSPECTED</span>
        <div id="test-mc-tidal-alert-banner" class="mc-tidal-alert-banner">CRITICAL TIDAL / ASPIRATION EMERGENCY</div>
        <div id="test-mc-clearance-banner" class="mc-clearance-banner">HARBOR DECK EXTRACTION CLEARANCE VERIFIED: MARITIME-SAFE-A1B2C3D4E5F6</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-mc-critical',
        '#test-badge-mc-staged',
        '#test-badge-mc-safe',
        '#test-badge-mc-zone',
        '#test-badge-mc-flag',
        '#test-mc-tidal-alert-banner',
        '#test-mc-clearance-banner',
      ]) {
        const styles = await page.locator(selector).evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { color: cs.color, backgroundColor: cs.backgroundColor };
        });
        expect(styles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
        const ratio = contrastRatio(styles.color, styles.backgroundColor);
        expect(ratio, `${selector} in ${theme} mode should have > 7.1:1 contrast`).toBeGreaterThan(7.1);
      }
    }
  });
});
