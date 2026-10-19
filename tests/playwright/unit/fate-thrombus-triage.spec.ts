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

test.describe('Feline Aortic Thromboembolism (FATE / Saddle Thrombus), 5 Ps, Paired Glucose/Lactate Limb Gradient & Clopidogrel/Rivaroxaban Cockpit UI (/fate-thrombus-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /fate-thrombus-triage renders seeded operations (FATE-OP-101, FATE-OP-102, FATE-OP-103), 5 Ps & Clopidogrel 75 mg alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/fate-thrombus-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#fate-thrombus-cockpit')).toBeVisible();
    await expect(page.locator('#fate-alert-banner')).toBeVisible();
    await expect(page.locator('#fate-alert-banner')).toContainText('5 Ps');
    await expect(page.locator('#fate-alert-banner')).toContainText('30 mg/dL');
    await expect(page.locator('#fate-alert-banner')).toContainText('Clopidogrel 75 mg');
    await expect(page.locator('#fate-alert-banner')).toContainText('Buprenorphine');
    await expect(page.locator('#fate-alert-banner')).toContainText('heating pads');
    await expect(page.locator('#fate-alert-banner')).toContainText('Reperfusion Hyperkalemia');

    await expect(page.locator('#fate-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Jasper');
    await expect(page.locator('#active-glucose-delta')).toContainText('93.0 mg/dL');
    await expect(page.locator('#active-limb-temp')).toContainText('29.2');

    const jasperCard = page.locator('#fate-operations-grid .fate-op-card[data-op-id="FATE-OP-101"]');
    const cleoCard = page.locator('#fate-operations-grid .fate-op-card[data-op-id="FATE-OP-102"]');
    const figaroCard = page.locator('#fate-operations-grid .fate-op-card[data-op-id="FATE-OP-103"]');

    await expect(jasperCard).toBeVisible();
    await expect(jasperCard).toHaveAttribute('data-status', 'ACUTE_SADDLE_THROMBUS_5PS_TRIAGE');

    await expect(cleoCard).toBeVisible();
    await expect(cleoCard).toHaveAttribute('data-status', 'METHADONE_CLOPIDOGREL_ANTICOAG_STAGED');

    await expect(figaroCard).toBeVisible();
    await expect(figaroCard).toHaveAttribute('data-status', 'REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED');

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging Methadone/Fentanyl CRI, Clopidogrel 75 mg loading & Rivaroxaban/Enoxaparin protocol on FATE-OP-101 transitions status to METHADONE_CLOPIDOGREL_ANTICOAG_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/fate-thrombus-triage`);
    await page.waitForLoadState('networkidle');

    const jasperCard = page.locator('#fate-operations-grid .fate-op-card[data-op-id="FATE-OP-101"]');
    await jasperCard.click();

    await expect(page.locator('#active-op-title')).toContainText('FATE-OP-101');
    await page.locator('#stage-antithrombotic-btn').click();

    await expect(jasperCard).toHaveAttribute('data-status', 'METHADONE_CLOPIDOGREL_ANTICOAG_STAGED');
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'METHADONE_CLOPIDOGREL_ANTICOAG_STAGED'
    );
  });

  test('3. Recording a serial limb perfusion & potassium check on FATE-OP-101 updates #active-glucose-delta, #active-limb-temp, and appends to #perfusion-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/fate-thrombus-triage`);
    await page.waitForLoadState('networkidle');

    const jasperCard = page.locator('#fate-operations-grid .fate-op-card[data-op-id="FATE-OP-101"]');
    await jasperCard.click();

    await page.locator('#check-glucose-delta-input').fill('16.5');
    await page.locator('#check-lactate-delta-input').fill('1.4');
    await page.locator('#check-potassium-input').fill('4.3');
    await page.locator('#check-limb-temp-input').fill('37.1');
    await page.locator('#check-pulse-status-input').fill('BILATERAL_DOPPLER_AUDIBLE_WARM_PADS');
    await page.locator('#check-recorded-by-input').fill('Dr. Maya Lin, DVM, DACVECC');
    await page
      .locator('#check-note-input')
      .fill('Collateral pelvic limb reperfusion confirmed; glucose delta reduced to 16.5 mg/dL, limb temp 37.1°C, normokalemic K+ 4.3 mmol/L.');
    await page.locator('#record-perfusion-check-btn').click();

    await expect(page.locator('#active-glucose-delta')).toContainText('16.5 mg/dL');
    await expect(page.locator('#active-limb-temp')).toContainText('37.1');
    await expect(page.locator('#perfusion-checks-list')).toContainText(
      'Collateral pelvic limb reperfusion confirmed; glucose delta reduced to 16.5 mg/dL'
    );
  });

  test('4. Clearing FATE-OP-101 for dual-therapy discharge transitions status to REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED and reveals #fate-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/fate-thrombus-triage`);
    await page.waitForLoadState('networkidle');

    const jasperCard = page.locator('#fate-operations-grid .fate-op-card[data-op-id="FATE-OP-101"]');
    await jasperCard.click();

    await page.locator('#clear-discharged-by-input').fill('Dr. Julian Vance, DACVIM (Cardiology)');
    await page
      .locator('#clear-summary-input')
      .fill('Bilateral pelvic limb perfusion restored, normokalemia verified (K+ 4.3 mmol/L), discharged on Clopidogrel 18.75 mg PO q24h + Rivaroxaban 2.5 mg PO q24h.');
    await page.locator('#clear-fate-btn').click();

    await expect(jasperCard).toHaveAttribute(
      'data-status',
      'REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED'
    );
    await expect(page.locator('#fate-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Julian Vance, DACVIM (Cardiology)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .fate-status-badge, .fate-alert-banner, and .fate-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/fate-thrombus-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-fate-badge-test-fixture';
      container.innerHTML = `
        <span id="test-fate-status-badge-base" class="fate-status-badge">ACUTE_SADDLE_THROMBUS_5PS_TRIAGE</span>
        <span id="test-fate-status-badge-critical" class="fate-status-badge fate-status-critical">ACUTE_SADDLE_THROMBUS_5PS_TRIAGE</span>
        <span id="test-fate-status-badge-staged" class="fate-status-badge fate-status-staged">METHADONE_CLOPIDOGREL_ANTICOAG_STAGED</span>
        <span id="test-fate-status-badge-cleared" class="fate-status-badge fate-status-cleared">REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED</span>
        <div id="test-fate-alert-banner" class="fate-alert-banner">CRITICAL FATE / SADDLE THROMBUS 5 Ps & REPERFUSION GUARD</div>
        <div id="test-fate-clearance-banner" class="fate-clearance-banner">REPERFUSED, NORMOKALEMIC & DUAL ANTITHROMBOTIC THERAPY CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-fate-status-badge-base',
        '#test-fate-status-badge-critical',
        '#test-fate-status-badge-staged',
        '#test-fate-status-badge-cleared',
        '#test-fate-alert-banner',
        '#test-fate-clearance-banner',
      ]) {
        const styles = await page.locator(selector).evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { color: cs.color, backgroundColor: cs.backgroundColor };
        });
        expect(styles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
        const ratio = contrastRatio(styles.color, styles.backgroundColor);
        expect(ratio, `${selector} in ${theme} mode should have > 7.1:1 contrast`).toBeGreaterThan(
          7.1
        );
      }
    }
  });
});
