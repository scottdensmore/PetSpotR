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

test.describe('Avian, Exotic Parrot & Falconry Telemetry Receiver, Flight-Corridor Thermals & Lure-Call Recovery Cockpit UI (/avian-recovery)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /avian-recovery renders seeded operations (AV-OP-101, AV-OP-102, AV-OP-103), summary stats, protocol steps, and thermal alert banner with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/avian-recovery`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#avian-recovery-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Avian, Exotic Parrot & Falconry Telemetry Receiver, Flight-Corridor Thermals & Lure-Call Recovery Cockpit'
    );

    await expect(page.locator('#stat-total-avian-ops')).toBeVisible();
    await expect(page.locator('#stat-critical-thermal-alerts')).toBeVisible();
    await expect(page.locator('#stat-staged-acoustic-lures')).toBeVisible();
    await expect(page.locator('#stat-recovered-aviary-count')).toBeVisible();

    const card101 = page.locator('#av-ops-list .av-op-card[data-op-id="AV-OP-101"]');
    const card102 = page.locator('#av-ops-list .av-op-card[data-op-id="AV-OP-102"]');
    const card103 = page.locator('#av-ops-list .av-op-card[data-op-id="AV-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-status', 'THERMAL_DRIFT_PREDATION_CRITICAL');
    await expect(card101.locator('.badge-av-status.badge-av-critical')).toContainText(
      'THERMAL_DRIFT_PREDATION_CRITICAL'
    );
    await expect(card101.locator('.badge-av-class').first()).toContainText('PSITTACINE_PARROT_MACAW');
    await expect(card101.locator('.av-thermal-alert-banner')).toContainText(
      'FLIGHT-CORRIDOR THERMAL DRIFT & CROP DEPLETION ALERT'
    );
    await expect(card101.locator('.badge-av-flag').first()).toContainText(
      'EXTREME_THERMAL_DOWNWIND_DRIFT_RISK'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-status', 'ACOUSTIC_LURE_PERCH_STAGED');
    await expect(card102.locator('.badge-av-status.badge-av-staged')).toContainText(
      'ACOUSTIC_LURE_PERCH_STAGED'
    );

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-status', 'RECOVERED_AVIARY_SAFE');
    await expect(card103.locator('.badge-av-status.badge-av-recovered')).toContainText(
      'RECOVERED_AVIARY_SAFE'
    );
    await expect(card103.locator('.av-clearance-banner')).toContainText(
      'AVIAN-SAFE-4D8E12A9F603'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Clicking .av-scenario-btn[data-scenario="macaw-thermal-ridge"] evaluates a high-canopy macaw thermal drift emergency with .av-thermal-alert-banner and EXTREME_THERMAL_DOWNWIND_DRIFT_RISK', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/avian-recovery`);
    await page.waitForLoadState('networkidle');

    await page.locator('.av-scenario-btn[data-scenario="macaw-thermal-ridge"]').click();

    const simCard = page
      .locator('#av-ops-list .av-op-card', {
        hasText: 'Kona',
      })
      .first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-status', 'THERMAL_DRIFT_PREDATION_CRITICAL');
    await expect(simCard.locator('.badge-av-status.badge-av-critical')).toContainText(
      'THERMAL_DRIFT_PREDATION_CRITICAL'
    );
    await expect(simCard.locator('.av-thermal-alert-banner')).toContainText(
      'FLIGHT-CORRIDOR THERMAL DRIFT & CROP DEPLETION ALERT'
    );
    await expect(simCard.locator('.badge-av-flag').first()).toContainText(
      'EXTREME_THERMAL_DOWNWIND_DRIFT_RISK'
    );
  });

  test('3. Toggling .av-step-checkbox on AV-OP-101 updates protocol lure readiness to 100% and transitions status to ACOUSTIC_LURE_PERCH_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/avian-recovery`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#av-ops-list .av-op-card[data-op-id="AV-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of ['step-flock-acoustic-lure', 'step-forage-perch-rig', 'step-kevlar-towel-transfer']) {
      const cb = card101.locator(`.av-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'ACOUSTIC_LURE_PERCH_STAGED');
    await expect(card101.locator('.badge-av-status.badge-av-staged')).toContainText(
      'ACOUSTIC_LURE_PERCH_STAGED'
    );
    await expect(card101).toContainText('Lure Readiness: 100%');
  });

  test('4. Clicking .av-issue-clearance-btn issues an AVIAN-SAFE-<12HEX> certificate and transitions operation to RECOVERED_AVIARY_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/avian-recovery`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#av-ops-list .av-op-card[data-op-id="AV-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.av-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'RECOVERED_AVIARY_SAFE');
    await expect(card102.locator('.badge-av-status.badge-av-recovered')).toContainText(
      'RECOVERED_AVIARY_SAFE'
    );
    await expect(card102.locator('.av-clearance-banner')).toBeVisible();
    await expect(card102.locator('.av-clearance-banner')).toContainText(
      'AVIAN RECOVERY CLEARANCE VERIFIED: AVIAN-SAFE-'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-av-critical, .badge-av-staged, .badge-av-recovered, .badge-av-class, .badge-av-flag, .av-thermal-alert-banner, and .av-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/avian-recovery`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-avian-recovery-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-av-critical" class="badge badge-av-status badge-av-critical">THERMAL_DRIFT_PREDATION_CRITICAL</span>
        <span id="test-badge-av-staged" class="badge badge-av-status badge-av-staged">ACOUSTIC_LURE_PERCH_STAGED</span>
        <span id="test-badge-av-recovered" class="badge badge-av-status badge-av-recovered">RECOVERED_AVIARY_SAFE</span>
        <span id="test-badge-av-class" class="badge badge-av-class">PSITTACINE_PARROT_MACAW</span>
        <span id="test-badge-av-flag" class="badge badge-av-flag">EXTREME_THERMAL_DOWNWIND_DRIFT_RISK</span>
        <div id="test-av-thermal-alert-banner" class="av-thermal-alert-banner">FLIGHT-CORRIDOR THERMAL DRIFT &amp; CROP DEPLETION ALERT</div>
        <div id="test-av-clearance-banner" class="av-clearance-banner">AVIAN RECOVERY CLEARANCE VERIFIED: AVIAN-SAFE-4D8E12A9F603</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-av-critical',
        '#test-badge-av-staged',
        '#test-badge-av-recovered',
        '#test-badge-av-class',
        '#test-badge-av-flag',
        '#test-av-thermal-alert-banner',
        '#test-av-clearance-banner',
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
