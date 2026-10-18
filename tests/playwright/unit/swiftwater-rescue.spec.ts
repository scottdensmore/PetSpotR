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

test.describe('Aquatic, Floodwater & Ice-Rescue Swiftwater Canine/Feline Extraction Cockpit UI (/swiftwater-rescue)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /swiftwater-rescue renders seeded ops (sw-op-101, sw-op-102, sw-op-103), protocol steps, and cold-shock alert banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/swiftwater-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#swiftwater-rescue-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Aquatic, Floodwater & Ice-Rescue Swiftwater Canine/Feline Extraction Cockpit'
    );

    await expect(page.locator('#kpi-sw-total')).toBeVisible();
    await expect(page.locator('#kpi-sw-hypothermia')).toBeVisible();
    await expect(page.locator('#kpi-sw-coldshock')).toBeVisible();
    await expect(page.locator('#kpi-sw-extracted')).toBeVisible();
    await expect(page.locator('#swiftwater-rescue-live-status')).toBeVisible();

    const card101 = page.locator('#sw-ops-list .sw-op-card[data-op-id="sw-op-101"]');
    const card102 = page.locator('#sw-ops-list .sw-op-card[data-op-id="sw-op-102"]');
    const card103 = page.locator('#sw-ops-list .sw-op-card[data-op-id="sw-op-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-extraction-status', 'STRANDED_IN_SWIFTWATER');
    await expect(card101.locator('.badge-sw-status.badge-sw-stranded')).toContainText('STRANDED_IN_SWIFTWATER');
    await expect(card101.locator('.badge-hydro-risk.badge-hydro-extreme')).toContainText('EXTREME_ICE_HYPOTHERMIA_RISK');
    await expect(card101.locator('.sw-coldshock-alert-banner')).toContainText('COLD SHOCK & CIRCUM-RESCUE COLLAPSE ALERT');
    await expect(card101.locator('.sw-steps-list')).toContainText('NFPA_1670_ROPE_RIGGING');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-extraction-status', 'THROW_BAG_RIGGED_APPROACH');
    await expect(card102.locator('.badge-sw-status.badge-sw-rigged')).toContainText('THROW_BAG_RIGGED_APPROACH');
    await expect(card102.locator('.badge-hydro-risk.badge-hydro-elevated')).toContainText('ELEVATED_FLOOD_CURRENT_RISK');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-extraction-status', 'EXTRACTED_WARMING_STABLE');
    await expect(card103.locator('.badge-sw-status.badge-sw-extracted')).toContainText('EXTRACTED_WARMING_STABLE');
    await expect(card103.locator('.badge-hydro-risk.badge-hydro-safe')).toContainText('SHORELINE_REWARMED_SAFE');
    await expect(card103.locator('.sw-clearance-banner')).toContainText('SWIFTWATER EXTRACTION & REWARMING VERIFIED: SWIFTWATER-SAFE-SW-OP-103');

    await expect(page.locator('#form-create-sw-op')).toBeVisible();
    await expect(page.locator('#btn-sim-ice-breakthrough')).toBeVisible();
    await expect(page.locator('#btn-sim-flash-flood-culvert')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Simulating a 33°F fractured pond ice shelf breakthrough (#btn-sim-ice-breakthrough) creates a STRANDED_IN_SWIFTWATER card with .sw-coldshock-alert-banner and EXTREME_ICE_HYPOTHERMIA_RISK', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/swiftwater-rescue`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-ice-breakthrough').click();

    const simCard = page.locator('#sw-ops-list .sw-op-card', {
      hasText: 'Kodiak',
    }).first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-extraction-status', 'STRANDED_IN_SWIFTWATER');
    await expect(simCard.locator('.badge-sw-status.badge-sw-stranded')).toContainText('STRANDED_IN_SWIFTWATER');
    await expect(simCard.locator('.badge-hydro-risk.badge-hydro-extreme')).toContainText('EXTREME_ICE_HYPOTHERMIA_RISK');
    await expect(simCard.locator('.sw-coldshock-alert-banner')).toContainText('COLD SHOCK & CIRCUM-RESCUE COLLAPSE ALERT');
  });

  test('3. Clicking .btn-deploy-sw-rigging deploys throw-bag rigging and inflatable ice sled (Rigging Readiness: 100/100) and transitions op to THROW_BAG_RIGGED_APPROACH', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/swiftwater-rescue`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-flash-flood-culvert').click();

    const floodCard = page.locator('#sw-ops-list .sw-op-card', {
      hasText: 'Cascade',
    }).first();
    await expect(floodCard).toBeVisible();
    await expect(floodCard).toHaveAttribute('data-extraction-status', 'STRANDED_IN_SWIFTWATER');

    await floodCard.locator('.btn-deploy-sw-rigging').click();

    await expect(floodCard).toHaveAttribute('data-extraction-status', 'THROW_BAG_RIGGED_APPROACH');
    await expect(floodCard.locator('.badge-sw-status.badge-sw-rigged')).toContainText('THROW_BAG_RIGGED_APPROACH');
    await expect(floodCard.locator('.badge-sw-metric')).toContainText('Rigging Readiness: 100/100');
    await expect(floodCard.locator('.badge-hydro-risk.badge-hydro-elevated')).toContainText('ELEVATED_FLOOD_CURRENT_RISK');
  });

  test('4. Clicking .btn-complete-sw-extraction transitions an op to EXTRACTED_WARMING_STABLE (SHORELINE_REWARMED_SAFE) and displays .sw-clearance-banner (SWIFTWATER-SAFE-...)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/swiftwater-rescue`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-flash-flood-culvert').click();

    const card = page.locator('#sw-ops-list .sw-op-card', {
      hasText: 'Cascade',
    }).first();
    await expect(card).toBeVisible();

    await card.locator('.btn-complete-sw-extraction').click();

    await expect(card).toHaveAttribute('data-extraction-status', 'EXTRACTED_WARMING_STABLE');
    await expect(card.locator('.badge-sw-status.badge-sw-extracted')).toContainText('EXTRACTED_WARMING_STABLE');
    await expect(card.locator('.badge-hydro-risk.badge-hydro-safe')).toContainText('SHORELINE_REWARMED_SAFE');
    await expect(card.locator('.sw-clearance-banner')).toBeVisible();
    await expect(card.locator('.sw-clearance-banner')).toContainText('SWIFTWATER EXTRACTION & REWARMING VERIFIED: SWIFTWATER-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7:1) on .badge-sw-stranded, .badge-sw-rigged, .badge-sw-extracted, .badge-hydro-extreme, .badge-hydro-elevated, .badge-hydro-safe, .badge-sw-metric, .sw-coldshock-alert-banner, and .sw-clearance-banner in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/swiftwater-rescue`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-swiftwater-rescue-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-sw-stranded" class="badge badge-sw-status badge-sw-stranded">STRANDED_IN_SWIFTWATER</span>
        <span id="test-badge-sw-rigged" class="badge badge-sw-status badge-sw-rigged">THROW_BAG_RIGGED_APPROACH</span>
        <span id="test-badge-sw-extracted" class="badge badge-sw-status badge-sw-extracted">EXTRACTED_WARMING_STABLE</span>
        <span id="test-badge-hydro-extreme" class="badge badge-hydro-risk badge-hydro-extreme">EXTREME_ICE_HYPOTHERMIA_RISK</span>
        <span id="test-badge-hydro-elevated" class="badge badge-hydro-risk badge-hydro-elevated">ELEVATED_FLOOD_CURRENT_RISK</span>
        <span id="test-badge-hydro-safe" class="badge badge-hydro-risk badge-hydro-safe">SHORELINE_REWARMED_SAFE</span>
        <span id="test-badge-sw-metric" class="badge badge-sw-metric">Hypothermia Risk: 80/100 • Rigging Readiness: 100/100</span>
        <div id="test-sw-coldshock-alert-banner" class="sw-coldshock-alert-banner">COLD SHOCK &amp; CIRCUM-RESCUE COLLAPSE ALERT</div>
        <div id="test-sw-clearance-banner" class="sw-clearance-banner">SWIFTWATER EXTRACTION &amp; REWARMING VERIFIED: SWIFTWATER-SAFE-SW-OP-101</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-sw-stranded',
        '#test-badge-sw-rigged',
        '#test-badge-sw-extracted',
        '#test-badge-hydro-extreme',
        '#test-badge-hydro-elevated',
        '#test-badge-hydro-safe',
        '#test-badge-sw-metric',
        '#test-sw-coldshock-alert-banner',
        '#test-sw-clearance-banner',
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
