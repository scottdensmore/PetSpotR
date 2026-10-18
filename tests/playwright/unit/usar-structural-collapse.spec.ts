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

test.describe('Seismic Rubble, USAR Structural Collapse & Delsar Acoustic Void-Space Extraction Cockpit UI (/usar-structural-collapse)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /usar-structural-collapse renders seeded operations (USAR-OP-101, USAR-OP-102, USAR-OP-103), summary stats, protocol steps, and collapse alert banner with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/usar-structural-collapse`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#usar-structural-collapse-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Seismic Rubble, Urban Search & Rescue (USAR) Structural Collapse & Delsar Acoustic Void-Space Extraction Cockpit'
    );

    await expect(page.locator('#stat-total-usar-ops')).toBeVisible();
    await expect(page.locator('#stat-critical-collapse-alerts')).toBeVisible();
    await expect(page.locator('#stat-staged-shoring-rigs')).toBeVisible();
    await expect(page.locator('#stat-extracted-usar-count')).toBeVisible();

    const card101 = page.locator('#usar-ops-list .usar-op-card[data-op-id="USAR-OP-101"]');
    const card102 = page.locator('#usar-ops-list .usar-op-card[data-op-id="USAR-OP-102"]');
    const card103 = page.locator('#usar-ops-list .usar-op-card[data-op-id="USAR-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-status', 'SECONDARY_COLLAPSE_CRUSH_CRITICAL');
    await expect(card101.locator('.badge-usar-status.badge-usar-critical')).toContainText(
      'SECONDARY_COLLAPSE_CRUSH_CRITICAL'
    );
    await expect(card101.locator('.badge-usar-void').first()).toContainText(
      'PANCAKE_FLOOR_SLAB_VOID'
    );
    await expect(card101.locator('.usar-collapse-alert-banner')).toContainText(
      'SECONDARY COLLAPSE & CRUSH-SYNDROME HYPERKALEMIA ALERT'
    );
    await expect(card101.locator('.badge-usar-flag').first()).toContainText(
      'EXTREME_UNSHORED_SECONDARY_COLLAPSE_RISK'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-status', 'SHORING_CRIBBING_DELSAR_STAGED');
    await expect(card102.locator('.badge-usar-status.badge-usar-staged')).toContainText(
      'SHORING_CRIBBING_DELSAR_STAGED'
    );

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-status', 'EXTRACTED_USAR_DECON_SAFE');
    await expect(card103.locator('.badge-usar-status.badge-usar-safe')).toContainText(
      'EXTRACTED_USAR_DECON_SAFE'
    );
    await expect(card103.locator('.usar-clearance-banner')).toContainText(
      'USAR-SAFE-5C9D41B8E702'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Clicking .usar-scenario-btn[data-scenario="pancake-slab-aftershock"] evaluates a pancake floor-slab collapse emergency with .usar-collapse-alert-banner and EXTREME_UNSHORED_SECONDARY_COLLAPSE_RISK', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/usar-structural-collapse`);
    await page.waitForLoadState('networkidle');

    await page.locator('.usar-scenario-btn[data-scenario="pancake-slab-aftershock"]').click();

    const simCard = page
      .locator('#usar-ops-list .usar-op-card', {
        hasText: 'Brutus (',
      })
      .first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-status', 'SECONDARY_COLLAPSE_CRUSH_CRITICAL');
    await expect(simCard.locator('.badge-usar-status.badge-usar-critical')).toContainText(
      'SECONDARY_COLLAPSE_CRUSH_CRITICAL'
    );
    await expect(simCard.locator('.usar-collapse-alert-banner')).toContainText(
      'SECONDARY COLLAPSE & CRUSH-SYNDROME HYPERKALEMIA ALERT'
    );
    await expect(simCard.locator('.badge-usar-flag').first()).toContainText(
      'EXTREME_UNSHORED_SECONDARY_COLLAPSE_RISK'
    );
  });

  test('3. Toggling .usar-step-checkbox on USAR-OP-101 updates protocol readiness to 100% and transitions status to SHORING_CRIBBING_DELSAR_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/usar-structural-collapse`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#usar-ops-list .usar-op-card[data-op-id="USAR-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'step-box-cribbing-shoring',
      'step-rotary-core-breach',
      'step-crush-rhabdo-decon',
    ]) {
      const cb = card101.locator(`.usar-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'SHORING_CRIBBING_DELSAR_STAGED');
    await expect(card101.locator('.badge-usar-status.badge-usar-staged')).toContainText(
      'SHORING_CRIBBING_DELSAR_STAGED'
    );
    await expect(card101).toContainText('Shoring Readiness: 100%');
  });

  test('4. Clicking .usar-issue-clearance-btn issues a USAR-SAFE-<12HEX> certificate and transitions operation to EXTRACTED_USAR_DECON_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/usar-structural-collapse`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#usar-ops-list .usar-op-card[data-op-id="USAR-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.usar-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'EXTRACTED_USAR_DECON_SAFE');
    await expect(card102.locator('.badge-usar-status.badge-usar-safe')).toContainText(
      'EXTRACTED_USAR_DECON_SAFE'
    );
    await expect(card102.locator('.usar-clearance-banner')).toBeVisible();
    await expect(card102.locator('.usar-clearance-banner')).toContainText(
      'USAR VOID EXTRACTION & SILICA DECON CLEARANCE VERIFIED: USAR-SAFE-'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-usar-critical, .badge-usar-staged, .badge-usar-safe, .badge-usar-void, .badge-usar-flag, .usar-collapse-alert-banner, and .usar-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/usar-structural-collapse`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-usar-collapse-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-usar-critical" class="badge badge-usar-status badge-usar-critical">SECONDARY_COLLAPSE_CRUSH_CRITICAL</span>
        <span id="test-badge-usar-staged" class="badge badge-usar-status badge-usar-staged">SHORING_CRIBBING_DELSAR_STAGED</span>
        <span id="test-badge-usar-safe" class="badge badge-usar-status badge-usar-safe">EXTRACTED_USAR_DECON_SAFE</span>
        <span id="test-badge-usar-void" class="badge badge-usar-void">PANCAKE_FLOOR_SLAB_VOID</span>
        <span id="test-badge-usar-flag" class="badge badge-usar-flag">EXTREME_UNSHORED_SECONDARY_COLLAPSE_RISK</span>
        <div id="test-usar-collapse-alert-banner" class="usar-collapse-alert-banner">SECONDARY COLLAPSE &amp; CRUSH-SYNDROME HYPERKALEMIA ALERT</div>
        <div id="test-usar-clearance-banner" class="usar-clearance-banner">USAR VOID EXTRACTION &amp; SILICA DECON CLEARANCE VERIFIED: USAR-SAFE-5C9D41B8E702</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-usar-critical',
        '#test-badge-usar-staged',
        '#test-badge-usar-safe',
        '#test-badge-usar-void',
        '#test-badge-usar-flag',
        '#test-usar-collapse-alert-banner',
        '#test-usar-clearance-banner',
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
