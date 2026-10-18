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

test.describe('Cold-Weather Avalanche, Deep-Snow Drift & Thermal Hypothermia Rewarming SAR Cockpit UI (/avalanche-winter-rescue)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /avalanche-winter-rescue renders seeded operations (AW-OP-101, AW-OP-102, AW-OP-103), summary stats, protocol steps, and burial alert banner with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/avalanche-winter-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#avalanche-winter-rescue-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Cold-Weather Avalanche, Deep-Snow Drift & Thermal Hypothermia Rewarming SAR Cockpit'
    );

    await expect(page.locator('#stat-total-winter-ops')).toBeVisible();
    await expect(page.locator('#stat-critical-burial-alerts')).toBeVisible();
    await expect(page.locator('#stat-staged-rewarm-bivies')).toBeVisible();
    await expect(page.locator('#stat-normothermic-safe-count')).toBeVisible();

    const card101 = page.locator('#aw-ops-list .aw-op-card[data-op-id="AW-OP-101"]');
    const card102 = page.locator('#aw-ops-list .aw-op-card[data-op-id="AW-OP-102"]');
    const card103 = page.locator('#aw-ops-list .aw-op-card[data-op-id="AW-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-status', 'BURIAL_HYPOTHERMIA_CRITICAL');
    await expect(card101.locator('.badge-aw-status.badge-aw-critical')).toContainText(
      'BURIAL_HYPOTHERMIA_CRITICAL'
    );
    await expect(card101.locator('.badge-aw-terrain').first()).toContainText(
      'AVALANCHE_DEBRIS_CHUTE'
    );
    await expect(card101.locator('.aw-burial-alert-banner')).toContainText(
      'SNOWPACK ASPHYXIA & HYPOTHERMIC AFTERDROP ALERT'
    );
    await expect(card101.locator('.badge-aw-flag').first()).toContainText(
      'EXTREME_SNOWPACK_ASPHYXIA_AIR_POCKET_RISK'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-status', 'PROBE_TRENCH_REWARMING_STAGED');
    await expect(card102.locator('.badge-aw-status.badge-aw-staged')).toContainText(
      'PROBE_TRENCH_REWARMING_STAGED'
    );

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-status', 'EXTRACTED_NORMOTHERMIC_SAFE');
    await expect(card103.locator('.badge-aw-status.badge-aw-safe')).toContainText(
      'EXTRACTED_NORMOTHERMIC_SAFE'
    );
    await expect(card103.locator('.aw-clearance-banner')).toContainText(
      'WINTER-SAFE-6B2C84E1A904'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Clicking .aw-scenario-btn[data-scenario="avalanche-slab-burial"] evaluates an avalanche debris burial emergency with .aw-burial-alert-banner and EXTREME_SNOWPACK_ASPHYXIA_AIR_POCKET_RISK', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/avalanche-winter-rescue`);
    await page.waitForLoadState('networkidle');

    await page.locator('.aw-scenario-btn[data-scenario="avalanche-slab-burial"]').click();

    const simCard = page
      .locator('#aw-ops-list .aw-op-card', {
        hasText: 'Kodiak',
      })
      .first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-status', 'BURIAL_HYPOTHERMIA_CRITICAL');
    await expect(simCard.locator('.badge-aw-status.badge-aw-critical')).toContainText(
      'BURIAL_HYPOTHERMIA_CRITICAL'
    );
    await expect(simCard.locator('.aw-burial-alert-banner')).toContainText(
      'SNOWPACK ASPHYXIA & HYPOTHERMIC AFTERDROP ALERT'
    );
    await expect(simCard.locator('.badge-aw-flag').first()).toContainText(
      'EXTREME_SNOWPACK_ASPHYXIA_AIR_POCKET_RISK'
    );
  });

  test('3. Toggling .aw-step-checkbox on AW-OP-101 updates protocol readiness to 100% and transitions status to PROBE_TRENCH_REWARMING_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/avalanche-winter-rescue`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#aw-ops-list .aw-op-card[data-op-id="AW-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'step-v-conveyor-trench',
      'step-horizontal-afterdrop-lift',
      'step-active-core-rewarm',
    ]) {
      const cb = card101.locator(`.aw-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'PROBE_TRENCH_REWARMING_STAGED');
    await expect(card101.locator('.badge-aw-status.badge-aw-staged')).toContainText(
      'PROBE_TRENCH_REWARMING_STAGED'
    );
    await expect(card101).toContainText('Rewarming Readiness: 100%');
  });

  test('4. Clicking .aw-issue-clearance-btn issues a WINTER-SAFE-<12HEX> certificate and transitions operation to EXTRACTED_NORMOTHERMIC_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/avalanche-winter-rescue`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#aw-ops-list .aw-op-card[data-op-id="AW-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.aw-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'EXTRACTED_NORMOTHERMIC_SAFE');
    await expect(card102.locator('.badge-aw-status.badge-aw-safe')).toContainText(
      'EXTRACTED_NORMOTHERMIC_SAFE'
    );
    await expect(card102.locator('.aw-clearance-banner')).toBeVisible();
    await expect(card102.locator('.aw-clearance-banner')).toContainText(
      'WINTER NORMOTHERMIC EXTRACTION CLEARANCE VERIFIED: WINTER-SAFE-'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-aw-critical, .badge-aw-staged, .badge-aw-safe, .badge-aw-terrain, .badge-aw-flag, .aw-burial-alert-banner, and .aw-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/avalanche-winter-rescue`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-winter-rescue-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-aw-critical" class="badge badge-aw-status badge-aw-critical">BURIAL_HYPOTHERMIA_CRITICAL</span>
        <span id="test-badge-aw-staged" class="badge badge-aw-status badge-aw-staged">PROBE_TRENCH_REWARMING_STAGED</span>
        <span id="test-badge-aw-safe" class="badge badge-aw-status badge-aw-safe">EXTRACTED_NORMOTHERMIC_SAFE</span>
        <span id="test-badge-aw-terrain" class="badge badge-aw-terrain">AVALANCHE_DEBRIS_CHUTE</span>
        <span id="test-badge-aw-flag" class="badge badge-aw-flag">EXTREME_SNOWPACK_ASPHYXIA_AIR_POCKET_RISK</span>
        <div id="test-aw-burial-alert-banner" class="aw-burial-alert-banner">SNOWPACK ASPHYXIA &amp; HYPOTHERMIC AFTERDROP ALERT</div>
        <div id="test-aw-clearance-banner" class="aw-clearance-banner">WINTER NORMOTHERMIC EXTRACTION CLEARANCE VERIFIED: WINTER-SAFE-6B2C84E1A904</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-aw-critical',
        '#test-badge-aw-staged',
        '#test-badge-aw-safe',
        '#test-badge-aw-terrain',
        '#test-badge-aw-flag',
        '#test-aw-burial-alert-banner',
        '#test-aw-clearance-banner',
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
