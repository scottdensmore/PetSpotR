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

test.describe('Acute Feline Infectious Peritonitis (FIP — Effusive Wet & Non-Effusive Dry/Ocular/Neurological), Rivalta Test, Albumin:Globulin Ratio < 0.4, GS-441524 / Remdesivir Nucleoside Analog & Blood-Brain/Ocular Barrier Dosing Cockpit UI (/fip-gs441524-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /fip-gs441524-triage renders seeded operations (FIP-OP-101, FIP-OP-102, FIP-OP-103), Rivalta/A:G Ratio/GS-441524 & Ascites Drainage/L-Lysine/Enrofloxacin guard alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/fip-gs441524-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#fip-gs441524-cockpit')).toBeVisible();
    await expect(page.locator('#fip-alert-banner')).toBeVisible();
    await expect(page.locator('#fip-alert-banner')).toContainText('Rivalta Test Positive');
    await expect(page.locator('#fip-alert-banner')).toContainText('Albumin:Globulin (A:G) Ratio < 0.4');
    await expect(page.locator('#fip-alert-banner')).toContainText('GS-441524');
    await expect(page.locator('#fip-alert-banner')).toContainText('Complete Abdominal Ascites Drainage');
    await expect(page.locator('#fip-alert-banner')).toContainText('L-Lysine');

    await expect(page.locator('#fip-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Bramble');
    await expect(page.locator('#active-ag-ratio')).toContainText('0.28');
    await expect(page.locator('#active-treatment-day')).toContainText('Day 1 / 84');

    const brambleCard = page.locator('#fip-operations-grid .fip-op-card[data-op-id="FIP-OP-101"]');
    const cosmoCard = page.locator('#fip-operations-grid .fip-op-card[data-op-id="FIP-OP-102"]');
    const saffronCard = page.locator('#fip-operations-grid .fip-op-card[data-op-id="FIP-OP-103"]');

    await expect(brambleCard).toBeVisible();
    await expect(brambleCard).toHaveAttribute(
      'data-status',
      'ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE'
    );

    await expect(cosmoCard).toBeVisible();
    await expect(cosmoCard).toHaveAttribute(
      'data-status',
      'GS441524_REMDESIVIR_84DAY_STAGED'
    );

    await expect(saffronCard).toBeVisible();
    await expect(saffronCard).toHaveAttribute(
      'data-status',
      'AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging 84-day GS-441524 / Remdesivir nucleoside antiviral protocol on FIP-OP-101 transitions status to GS441524_REMDESIVIR_84DAY_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/fip-gs441524-triage`);
    await page.waitForLoadState('networkidle');

    const brambleCard = page.locator('#fip-operations-grid .fip-op-card[data-op-id="FIP-OP-101"]');
    await brambleCard.click();

    await expect(page.locator('#active-op-title')).toContainText('FIP-OP-101');
    await page.locator('#stage-antiviral-btn').click();

    await expect(brambleCard).toHaveAttribute(
      'data-status',
      'GS441524_REMDESIVIR_84DAY_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'GS441524_REMDESIVIR_84DAY_STAGED'
    );
  });

  test('3. Recording an 84-day antiviral progress check on FIP-OP-101 updates #active-ag-ratio, #active-treatment-day, and appends to #progress-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/fip-gs441524-triage`);
    await page.waitForLoadState('networkidle');

    const brambleCard = page.locator('#fip-operations-grid .fip-op-card[data-op-id="FIP-OP-101"]');
    await brambleCard.click();

    await page.locator('#check-treatment-day-input').fill('84');
    await page.locator('#check-weight-input').fill('3.5');
    await page.locator('#check-ag-ratio-input').fill('0.72');
    await page.locator('#check-bilirubin-input').fill('0.2');
    await page.locator('#check-hct-input').fill('36.5');
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Linnea Lindholm, DACVIM (Feline Medicine)');
    await page
      .locator('#check-note-input')
      .fill('Day 84 (Week 12) Check: Cavitary effusion completely resolved, weight increased to 3.5 kg, A:G Ratio normalized to 0.72 (> 0.60), Total Bilirubin 0.2 mg/dL, HCT 36.5%.');
    await page.locator('#record-progress-check-btn').click();

    await expect(page.locator('#active-ag-ratio')).toContainText('0.72');
    await expect(page.locator('#active-treatment-day')).toContainText('Day 84 / 84');
    await expect(page.locator('#progress-checks-list')).toContainText(
      'Day 84 (Week 12) Check: Cavitary effusion completely resolved'
    );
  });

  test('4. Clearing FIP-OP-101 for 84-day antiviral cure discharge transitions status to AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED and reveals #fip-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/fip-gs441524-triage`);
    await page.waitForLoadState('networkidle');

    const brambleCard = page.locator('#fip-operations-grid .fip-op-card[data-op-id="FIP-OP-101"]');
    await brambleCard.click();

    await page
      .locator('#clear-discharged-by-input')
      .fill('Dr. Linnea Lindholm, DACVIM (Feline Medicine)');
    await page
      .locator('#clear-summary-input')
      .fill('Completed full 84-day (12-week) GS-441524 nucleoside analog protocol; cavitary effusion resolved, Albumin:Globulin (A:G) Ratio normalized to 0.72 (> 0.60), Total Bilirubin 0.2 mg/dL, HCT 36.5%; cleared as 84-day FIP clinical cure.');
    await page.locator('#clear-fip-btn').click();

    await expect(brambleCard).toHaveAttribute(
      'data-status',
      'AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED'
    );
    await expect(page.locator('#fip-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Linnea Lindholm, DACVIM (Feline Medicine)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .fip-status-badge, .fip-alert-banner, and .fip-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/fip-gs441524-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-fip-badge-test-fixture';
      container.innerHTML = `
        <span id="test-fip-status-badge-base" class="fip-status-badge">ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE</span>
        <span id="test-fip-status-badge-critical" class="fip-status-badge fip-status-critical">ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE</span>
        <span id="test-fip-status-badge-staged" class="fip-status-badge fip-status-staged">GS441524_REMDESIVIR_84DAY_STAGED</span>
        <span id="test-fip-status-badge-cleared" class="fip-status-badge fip-status-cleared">AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED</span>
        <div id="test-fip-alert-banner" class="fip-alert-banner">CRITICAL FIP & GS-441524 ANTIVIRAL GUARD</div>
        <div id="test-fip-clearance-banner" class="fip-clearance-banner">84-DAY GS-441524 ANTIVIRAL CURE & A:G RATIO NORMALIZATION CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-fip-status-badge-base',
        '#test-fip-status-badge-critical',
        '#test-fip-status-badge-staged',
        '#test-fip-status-badge-cleared',
        '#test-fip-alert-banner',
        '#test-fip-clearance-banner',
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
