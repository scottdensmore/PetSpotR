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

test.describe('Ectothermic Reptile, Chelonian & Amphibian Thermal-Dormancy Brisk-Basking & Micro-Habitat Recovery Cockpit UI (/reptile-ectotherm-rescue)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /reptile-ectotherm-rescue renders seeded operations (RE-OP-101, RE-OP-102, RE-OP-103), summary stats, protocol steps, and torpor alert banner with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/reptile-ectotherm-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#reptile-ectotherm-rescue-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Ectothermic Reptile, Chelonian & Amphibian Thermal-Dormancy Brisk-Basking & Micro-Habitat Recovery Cockpit'
    );

    await expect(page.locator('#stat-total-ectotherm-ops')).toBeVisible();
    await expect(page.locator('#stat-critical-torpor-alerts')).toBeVisible();
    await expect(page.locator('#stat-staged-basking-lures')).toBeVisible();
    await expect(page.locator('#stat-thermal-safe-count')).toBeVisible();

    const card101 = page.locator('#re-ops-list .re-op-card[data-op-id="RE-OP-101"]');
    const card102 = page.locator('#re-ops-list .re-op-card[data-op-id="RE-OP-102"]');
    const card103 = page.locator('#re-ops-list .re-op-card[data-op-id="RE-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-status', 'COLD_TORPOR_BRUMATION_CRITICAL');
    await expect(card101.locator('.badge-re-status.badge-re-critical')).toContainText(
      'COLD_TORPOR_BRUMATION_CRITICAL'
    );
    await expect(card101.locator('.badge-re-taxon').first()).toContainText(
      'CHELONIAN_TORTOISE_BURROW'
    );
    await expect(card101.locator('.re-torpor-alert-banner')).toContainText(
      'ECTOTHERMIC COLD-STUN TORPOR & DESICCATION ALERT'
    );
    await expect(card101.locator('.badge-re-flag').first()).toContainText(
      'EXTREME_ECTOTHERMIC_COLD_STUN_TORPOR_RISK'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-status', 'BASKING_LURE_CREVICE_STAGED');
    await expect(card102.locator('.badge-re-status.badge-re-staged')).toContainText(
      'BASKING_LURE_CREVICE_STAGED'
    );

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-status', 'RECOVERED_THERMAL_GRADIENT_SAFE');
    await expect(card103.locator('.badge-re-status.badge-re-safe')).toContainText(
      'RECOVERED_THERMAL_GRADIENT_SAFE'
    );
    await expect(card103.locator('.re-clearance-banner')).toContainText(
      'ECTOTHERM-SAFE-8A3F19C4E207'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Clicking .re-scenario-btn[data-scenario="sulcata-cold-burrow"] evaluates a cold-stunned tortoise burrow emergency with .re-torpor-alert-banner and EXTREME_ECTOTHERMIC_COLD_STUN_TORPOR_RISK', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/reptile-ectotherm-rescue`);
    await page.waitForLoadState('networkidle');

    await page.locator('.re-scenario-btn[data-scenario="sulcata-cold-burrow"]').click();

    const simCard = page
      .locator('#re-ops-list .re-op-card', {
        hasText: 'Goliath (',
      })
      .first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-status', 'COLD_TORPOR_BRUMATION_CRITICAL');
    await expect(simCard.locator('.badge-re-status.badge-re-critical')).toContainText(
      'COLD_TORPOR_BRUMATION_CRITICAL'
    );
    await expect(simCard.locator('.re-torpor-alert-banner')).toContainText(
      'ECTOTHERMIC COLD-STUN TORPOR & DESICCATION ALERT'
    );
    await expect(simCard.locator('.badge-re-flag').first()).toContainText(
      'EXTREME_ECTOTHERMIC_COLD_STUN_TORPOR_RISK'
    );
  });

  test('3. Toggling .re-step-checkbox on RE-OP-101 updates protocol readiness to 100% and transitions status to BASKING_LURE_CREVICE_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/reptile-ectotherm-rescue`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#re-ops-list .re-op-card[data-op-id="RE-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'step-ceramic-ir-basking-lure',
      'step-humidity-hydration-soak',
      'step-gradual-coelomic-rewarm',
    ]) {
      const cb = card101.locator(`.re-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'BASKING_LURE_CREVICE_STAGED');
    await expect(card101.locator('.badge-re-status.badge-re-staged')).toContainText(
      'BASKING_LURE_CREVICE_STAGED'
    );
    await expect(card101).toContainText('Thermal Readiness: 100%');
  });

  test('4. Clicking .re-issue-clearance-btn issues an ECTOTHERM-SAFE-<12HEX> certificate and transitions operation to RECOVERED_THERMAL_GRADIENT_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/reptile-ectotherm-rescue`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#re-ops-list .re-op-card[data-op-id="RE-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.re-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'RECOVERED_THERMAL_GRADIENT_SAFE');
    await expect(card102.locator('.badge-re-status.badge-re-safe')).toContainText(
      'RECOVERED_THERMAL_GRADIENT_SAFE'
    );
    await expect(card102.locator('.re-clearance-banner')).toBeVisible();
    await expect(card102.locator('.re-clearance-banner')).toContainText(
      'ECTOTHERM THERMAL-GRADIENT RECOVERY CLEARANCE VERIFIED: ECTOTHERM-SAFE-'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-re-critical, .badge-re-staged, .badge-re-safe, .badge-re-taxon, .badge-re-flag, .re-torpor-alert-banner, and .re-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/reptile-ectotherm-rescue`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-ectotherm-rescue-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-re-critical" class="badge badge-re-status badge-re-critical">COLD_TORPOR_BRUMATION_CRITICAL</span>
        <span id="test-badge-re-staged" class="badge badge-re-status badge-re-staged">BASKING_LURE_CREVICE_STAGED</span>
        <span id="test-badge-re-safe" class="badge badge-re-status badge-re-safe">RECOVERED_THERMAL_GRADIENT_SAFE</span>
        <span id="test-badge-re-taxon" class="badge badge-re-taxon">CHELONIAN_TORTOISE_BURROW</span>
        <span id="test-badge-re-flag" class="badge badge-re-flag">EXTREME_ECTOTHERMIC_COLD_STUN_TORPOR_RISK</span>
        <div id="test-re-torpor-alert-banner" class="re-torpor-alert-banner">ECTOTHERMIC COLD-STUN TORPOR &amp; DESICCATION ALERT</div>
        <div id="test-re-clearance-banner" class="re-clearance-banner">ECTOTHERM THERMAL-GRADIENT RECOVERY CLEARANCE VERIFIED: ECTOTHERM-SAFE-8A3F19C4E207</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-re-critical',
        '#test-badge-re-staged',
        '#test-badge-re-safe',
        '#test-badge-re-taxon',
        '#test-badge-re-flag',
        '#test-re-torpor-alert-banner',
        '#test-re-clearance-banner',
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
