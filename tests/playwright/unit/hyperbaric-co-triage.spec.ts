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

test.describe('Hyperbaric Oxygen (HBOT), Carbon Monoxide (CO) Carboxyhemoglobin & Cyanide Smoke Triage Cockpit UI (/hyperbaric-co-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /hyperbaric-co-triage renders seeded operations (HB-OP-101, HB-OP-102, HB-OP-103), KPI telemetry, CO-oximetry alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/hyperbaric-co-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#hyperbaric-co-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Hyperbaric Oxygen (HBOT), Carbon Monoxide (CO) Carboxyhemoglobin & Cyanide Smoke Triage Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-cohb')).toBeVisible();
    await expect(page.locator('#kpi-staged-hbot')).toBeVisible();
    await expect(page.locator('#kpi-cleared-safe')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const cinderCard = page.locator('#hbot-operations-list .hbot-op-card', {
      hasText: 'Cinder (',
    });
    const dieselCard = page.locator('#hbot-operations-list .hbot-op-card', {
      hasText: 'Diesel (',
    });
    const pebbleCard = page.locator('#hbot-operations-list .hbot-op-card', {
      hasText: 'Pebble (',
    });

    await expect(cinderCard).toBeVisible();
    await expect(cinderCard).toHaveAttribute(
      'data-status',
      'CARBOXYHEMOGLOBIN_CYANIDE_CRITICAL'
    );
    await expect(cinderCard.locator('.badge-hbot-status.badge-hbot-critical')).toContainText(
      'CARBOXYHEMOGLOBIN_CYANIDE_CRITICAL'
    );
    await expect(cinderCard.locator('.hbot-cooximetry-alert-banner')).toContainText('98-100%');

    await expect(dieselCard).toBeVisible();
    await expect(dieselCard).toHaveAttribute(
      'data-status',
      'HBOT_CHAMBER_HYDROXOCOBALAMIN_STAGED'
    );

    await expect(pebbleCard).toBeVisible();
    await expect(pebbleCard).toHaveAttribute(
      'data-status',
      'NORMOXIC_COHB_CLEARED_SAFE'
    );
    await expect(pebbleCard.locator('.hbot-clearance-banner')).toContainText('HBOT-SAFE-');

    expect(cspErrors).toEqual([]);
  });

  test('2. Submitting #hbot-eval-form evaluates a new critical hyperbaric CO & cyanide smoke operation with CO-oximetry alert and hydroxocobalamin regimen', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hyperbaric-co-triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#eval-pet-name').fill('Phoenix');
    await page.locator('#eval-species').fill('Cat - Siamese');
    await page.locator('#eval-weight-kg').fill('4.6');
    await page.locator('#eval-exposure-source').selectOption('STRUCTURAL_HOUSE_FIRE_SYNTHETICS');
    await page.locator('#eval-cohb-pct').fill('36.8');
    await page.locator('#eval-lactate-mmol').fill('12.4');
    await page.locator('#eval-chamber-ata').fill('2.2');
    await page.locator('#eval-singe-soot').check();
    await page.locator('#eval-spo2-warning-noted').check();
    await page.locator('#eval-hydroxocobalamin').check();

    await page.locator('#btn-submit-hbot-eval').click();

    const phoenixCard = page
      .locator('#hbot-operations-list .hbot-op-card', {
        hasText: 'Phoenix (',
      })
      .first();
    await expect(phoenixCard).toBeVisible();
    await expect(phoenixCard).toHaveAttribute(
      'data-status',
      'CARBOXYHEMOGLOBIN_CYANIDE_CRITICAL'
    );
    await expect(phoenixCard.locator('.hbot-cooximetry-alert-banner')).toContainText('98-100%');
  });

  test('3. Completing all 4 protocol steps on HB-OP-101 transitions status to HBOT_CHAMBER_HYDROXOCOBALAMIN_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hyperbaric-co-triage`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#hbot-operations-list .hbot-op-card[data-op-id="HB-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-CO-OXIMETRY-FALSE-SPO2-BYPASS',
      'STEP-HYDROXOCOBALAMIN-CYANIDE-BIND',
      'STEP-HBOT-CHAMBER-2ATA-WASHOUT',
      'STEP-DELAYED-NEURO-SEQUELAE-CHECK',
    ]) {
      const cb = card101.locator(`.hbot-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'HBOT_CHAMBER_HYDROXOCOBALAMIN_STAGED');
    await expect(card101.locator('.badge-hbot-status.badge-hbot-staged')).toContainText(
      'HBOT_CHAMBER_HYDROXOCOBALAMIN_STAGED'
    );
  });

  test('4. Clicking .hbot-issue-clearance-btn issues an HBOT-SAFE-<12HEX> clearance token and transitions operation to NORMOXIC_COHB_CLEARED_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hyperbaric-co-triage`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#hbot-operations-list .hbot-op-card[data-op-id="HB-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.hbot-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'NORMOXIC_COHB_CLEARED_SAFE');
    await expect(card102.locator('.badge-hbot-status.badge-hbot-safe')).toContainText(
      'NORMOXIC_COHB_CLEARED_SAFE'
    );
    await expect(card102.locator('.hbot-clearance-banner')).toContainText('HBOT-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-hbot-critical, .badge-hbot-staged, .badge-hbot-safe, .badge-hbot-source, .badge-hbot-flag, .hbot-cooximetry-alert-banner, and .hbot-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hyperbaric-co-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-hbot-triage-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-hbot-critical" class="badge badge-hbot-status badge-hbot-critical">CARBOXYHEMOGLOBIN_CYANIDE_CRITICAL</span>
        <span id="test-badge-hbot-staged" class="badge badge-hbot-status badge-hbot-staged">HBOT_CHAMBER_HYDROXOCOBALAMIN_STAGED</span>
        <span id="test-badge-hbot-safe" class="badge badge-hbot-status badge-hbot-safe">NORMOXIC_COHB_CLEARED_SAFE</span>
        <span id="test-badge-hbot-source" class="badge badge-hbot-source">STRUCTURAL_HOUSE_FIRE_SYNTHETICS</span>
        <span id="test-badge-hbot-flag" class="badge badge-hbot-flag">CYANIDE_LACTATEMIA_HISTOTOXIC_CRISIS</span>
        <div id="test-hbot-cooximetry-alert-banner" class="hbot-cooximetry-alert-banner">CRITICAL DIAGNOSTIC ALERT: Standard two-wavelength pulse oximetry SpO2 falsely reads 98-100%</div>
        <div id="test-hbot-clearance-banner" class="hbot-clearance-banner">NORMOXIC COHb &amp; HBOT WASHOUT CLEARANCE VERIFIED: HBOT-SAFE-1A2B3C4D5E6F</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-hbot-critical',
        '#test-badge-hbot-staged',
        '#test-badge-hbot-safe',
        '#test-badge-hbot-source',
        '#test-badge-hbot-flag',
        '#test-hbot-cooximetry-alert-banner',
        '#test-hbot-clearance-banner',
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
