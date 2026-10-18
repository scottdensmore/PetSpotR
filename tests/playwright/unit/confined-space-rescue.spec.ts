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

test.describe('Subterranean, Storm-Drain, Culvert & Confined-Space Acoustic/Fiber-Optic Extraction Cockpit UI (/confined-space-rescue)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /confined-space-rescue renders seeded operations (CS-OP-101, CS-OP-102, CS-OP-103), summary stats, protocol steps, and gas alert banner with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/confined-space-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#confined-space-rescue-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Subterranean, Storm-Drain, Culvert & Confined-Space Acoustic/Fiber-Optic Extraction Cockpit'
    );

    await expect(page.locator('#stat-total-confined-ops')).toBeVisible();
    await expect(page.locator('#stat-critical-gas-alerts')).toBeVisible();
    await expect(page.locator('#stat-rigged-borescopes')).toBeVisible();
    await expect(page.locator('#stat-extracted-safe-count')).toBeVisible();

    const card101 = page.locator('#cs-ops-list .cs-op-card[data-op-id="CS-OP-101"]');
    const card102 = page.locator('#cs-ops-list .cs-op-card[data-op-id="CS-OP-102"]');
    const card103 = page.locator('#cs-ops-list .cs-op-card[data-op-id="CS-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-status', 'ATMOSPHERIC_HAZARD_CRITICAL');
    await expect(card101.locator('.badge-cs-status.badge-cs-critical')).toContainText(
      'ATMOSPHERIC_HAZARD_CRITICAL'
    );
    await expect(card101.locator('.badge-cs-space').first()).toContainText('STORM_DRAIN_CULVERT');
    await expect(card101.locator('.cs-gas-alert-banner')).toContainText(
      'ATMOSPHERIC & CULVERT SURGE HAZARD ALERT'
    );
    await expect(card101.locator('.badge-cs-flag').first()).toContainText(
      'EXTREME_ATMOSPHERIC_ASPHYXIATION_RISK'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-status', 'BORESCOPE_PIPE_GATE_RIGGED');
    await expect(card102.locator('.badge-cs-status.badge-cs-rigged')).toContainText(
      'BORESCOPE_PIPE_GATE_RIGGED'
    );

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-status', 'EXTRACTED_SUBTERRANEAN_SAFE');
    await expect(card103.locator('.badge-cs-status.badge-cs-extracted')).toContainText(
      'EXTRACTED_SUBTERRANEAN_SAFE'
    );
    await expect(card103.locator('.cs-clearance-banner')).toContainText(
      'CONFINED-SAFE-7C3E91A4B802'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Clicking .cs-scenario-btn[data-scenario="culvert-gas-surge"] evaluates a storm-drain culvert emergency with .cs-gas-alert-banner and EXTREME_ATMOSPHERIC_ASPHYXIATION_RISK', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/confined-space-rescue`);
    await page.waitForLoadState('networkidle');

    await page.locator('.cs-scenario-btn[data-scenario="culvert-gas-surge"]').click();

    const simCard = page
      .locator('#cs-ops-list .cs-op-card', {
        hasText: 'Shadow',
      })
      .first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-status', 'ATMOSPHERIC_HAZARD_CRITICAL');
    await expect(simCard.locator('.badge-cs-status.badge-cs-critical')).toContainText(
      'ATMOSPHERIC_HAZARD_CRITICAL'
    );
    await expect(simCard.locator('.cs-gas-alert-banner')).toContainText(
      'ATMOSPHERIC & CULVERT SURGE HAZARD ALERT'
    );
    await expect(simCard.locator('.badge-cs-flag').first()).toContainText(
      'EXTREME_ATMOSPHERIC_ASPHYXIATION_RISK'
    );
  });

  test('3. Toggling .cs-step-checkbox on CS-OP-101 updates protocol readiness to 100% and transitions status to BORESCOPE_PIPE_GATE_RIGGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/confined-space-rescue`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#cs-ops-list .cs-op-card[data-op-id="CS-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of ['step-borescope-loc', 'step-oneway-gate', 'step-haul-extract']) {
      const cb = card101.locator(`.cs-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'BORESCOPE_PIPE_GATE_RIGGED');
    await expect(card101.locator('.badge-cs-status.badge-cs-rigged')).toContainText(
      'BORESCOPE_PIPE_GATE_RIGGED'
    );
    await expect(card101).toContainText('Rigging Readiness: 100%');
  });

  test('4. Clicking .cs-issue-clearance-btn issues a CONFINED-SAFE-<12HEX> certificate and transitions operation to EXTRACTED_SUBTERRANEAN_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/confined-space-rescue`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#cs-ops-list .cs-op-card[data-op-id="CS-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.cs-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'EXTRACTED_SUBTERRANEAN_SAFE');
    await expect(card102.locator('.badge-cs-status.badge-cs-extracted')).toContainText(
      'EXTRACTED_SUBTERRANEAN_SAFE'
    );
    await expect(card102.locator('.cs-clearance-banner')).toBeVisible();
    await expect(card102.locator('.cs-clearance-banner')).toContainText(
      'SUBTERRANEAN EXTRACTION CLEARANCE VERIFIED: CONFINED-SAFE-'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-cs-critical, .badge-cs-rigged, .badge-cs-extracted, .badge-cs-space, .badge-cs-flag, .cs-gas-alert-banner, and .cs-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/confined-space-rescue`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-confined-space-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-cs-critical" class="badge badge-cs-status badge-cs-critical">ATMOSPHERIC_HAZARD_CRITICAL</span>
        <span id="test-badge-cs-rigged" class="badge badge-cs-status badge-cs-rigged">BORESCOPE_PIPE_GATE_RIGGED</span>
        <span id="test-badge-cs-extracted" class="badge badge-cs-status badge-cs-extracted">EXTRACTED_SUBTERRANEAN_SAFE</span>
        <span id="test-badge-cs-space" class="badge badge-cs-space">STORM_DRAIN_CULVERT</span>
        <span id="test-badge-cs-flag" class="badge badge-cs-flag">EXTREME_ATMOSPHERIC_ASPHYXIATION_RISK</span>
        <div id="test-cs-gas-alert-banner" class="cs-gas-alert-banner">ATMOSPHERIC &amp; CULVERT SURGE HAZARD ALERT</div>
        <div id="test-cs-clearance-banner" class="cs-clearance-banner">SUBTERRANEAN EXTRACTION CLEARANCE VERIFIED: CONFINED-SAFE-7C3E91A4B802</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-cs-critical',
        '#test-badge-cs-rigged',
        '#test-badge-cs-extracted',
        '#test-badge-cs-space',
        '#test-badge-cs-flag',
        '#test-cs-gas-alert-banner',
        '#test-cs-clearance-banner',
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
