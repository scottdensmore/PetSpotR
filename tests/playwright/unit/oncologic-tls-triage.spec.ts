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

test.describe('Acute Tumor Lysis Syndrome (TLS) & Oncologic Hypercalcemia of Malignancy, Hyperkalemia/Hyperphosphatemia/Hypocalcemia, Rasburicase/Allopurinol & Zoledronate/Saline Diuresis Cockpit UI (/oncologic-tls-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /oncologic-tls-triage renders seeded operations (TLS-OP-101, TLS-OP-102, TLS-OP-103), 0.9% NaCl/Rasburicase/Zoledronate & Pre-Biopsy Steroid/NaHCO3 guard alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/oncologic-tls-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#oncologic-tls-cockpit')).toBeVisible();
    await expect(page.locator('#tls-alert-banner')).toBeVisible();
    await expect(page.locator('#tls-alert-banner')).toContainText('0.9% NaCl Calcium-Free Diuresis');
    await expect(page.locator('#tls-alert-banner')).toContainText('Rasburicase');
    await expect(page.locator('#tls-alert-banner')).toContainText('Zoledronate');
    await expect(page.locator('#tls-alert-banner')).toContainText('Prednisone/Dexamethasone');
    await expect(page.locator('#tls-alert-banner')).toContainText('Sodium Bicarbonate');

    await expect(page.locator('#tls-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Bauer');
    await expect(page.locator('#active-potassium')).toContainText('7.2 mmol/L');
    await expect(page.locator('#active-ionized-calcium')).toContainText('0.78 mmol/L');

    const bauerCard = page.locator('#tls-operations-grid .tls-op-card[data-op-id="TLS-OP-101"]');
    const pennyCard = page.locator('#tls-operations-grid .tls-op-card[data-op-id="TLS-OP-102"]');
    const otisCard = page.locator('#tls-operations-grid .tls-op-card[data-op-id="TLS-OP-103"]');

    await expect(bauerCard).toBeVisible();
    await expect(bauerCard).toHaveAttribute(
      'data-status',
      'ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE'
    );

    await expect(pennyCard).toBeVisible();
    await expect(pennyCard).toHaveAttribute(
      'data-status',
      'SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED'
    );

    await expect(otisCard).toBeVisible();
    await expect(otisCard).toHaveAttribute(
      'data-status',
      'ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging 0.9% NaCl diuresis, Rasburicase & Zoledronate protocol on TLS-OP-101 transitions status to SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/oncologic-tls-triage`);
    await page.waitForLoadState('networkidle');

    const bauerCard = page.locator('#tls-operations-grid .tls-op-card[data-op-id="TLS-OP-101"]');
    await bauerCard.click();

    await expect(page.locator('#active-op-title')).toContainText('TLS-OP-101');
    await page.locator('#stage-diuresis-btn').click();

    await expect(bauerCard).toHaveAttribute(
      'data-status',
      'SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED'
    );
  });

  test('3. Recording a serial electrolyte & renal panel check on TLS-OP-101 updates #active-potassium, #active-ionized-calcium, and appends to #electrolyte-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/oncologic-tls-triage`);
    await page.waitForLoadState('networkidle');

    const bauerCard = page.locator('#tls-operations-grid .tls-op-card[data-op-id="TLS-OP-101"]');
    await bauerCard.click();

    await page.locator('#check-potassium-input').fill('4.4');
    await page.locator('#check-phosphorus-input').fill('4.6');
    await page.locator('#check-ical-input').fill('1.24');
    await page.locator('#check-uric-acid-input').fill('1.6');
    await page.locator('#check-creatinine-input').fill('1.3');
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Soren Lindqvist, DACVIM (Oncology)');
    await page
      .locator('#check-note-input')
      .fill('Post-0.9% NaCl diuresis, Rasburicase & Zoledronate: K+ normalized to 4.4 mmol/L, Phosphorus reduced to 4.6 mg/dL, iCa stabilized at 1.24 mmol/L.');
    await page.locator('#record-electrolyte-check-btn').click();

    await expect(page.locator('#active-potassium')).toContainText('4.4 mmol/L');
    await expect(page.locator('#active-ionized-calcium')).toContainText('1.24 mmol/L');
    await expect(page.locator('#electrolyte-checks-list')).toContainText(
      'Post-0.9% NaCl diuresis, Rasburicase & Zoledronate: K+ normalized to 4.4 mmol/L'
    );
  });

  test('4. Clearing TLS-OP-101 for normoelectrolytic discharge transitions status to ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED and reveals #tls-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/oncologic-tls-triage`);
    await page.waitForLoadState('networkidle');

    const bauerCard = page.locator('#tls-operations-grid .tls-op-card[data-op-id="TLS-OP-101"]');
    await bauerCard.click();

    await page
      .locator('#clear-discharged-by-input')
      .fill('Dr. Soren Lindqvist, DACVIM (Oncology)');
    await page
      .locator('#clear-summary-input')
      .fill('Hyperkalemia and hyperphosphatemia resolved (K+ 4.4 mmol/L, Phosphorus 4.6 mg/dL), ionized calcium normalized (iCa 1.24 mmol/L), and renal function protected.');
    await page.locator('#clear-tls-btn').click();

    await expect(bauerCard).toHaveAttribute(
      'data-status',
      'ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED'
    );
    await expect(page.locator('#tls-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Soren Lindqvist, DACVIM (Oncology)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .tls-status-badge, .tls-alert-banner, and .tls-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/oncologic-tls-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-tls-badge-test-fixture';
      container.innerHTML = `
        <span id="test-tls-status-badge-base" class="tls-status-badge">ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE</span>
        <span id="test-tls-status-badge-critical" class="tls-status-badge tls-status-critical">ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE</span>
        <span id="test-tls-status-badge-staged" class="tls-status-badge tls-status-staged">SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED</span>
        <span id="test-tls-status-badge-cleared" class="tls-status-badge tls-status-cleared">ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED</span>
        <div id="test-tls-alert-banner" class="tls-alert-banner">CRITICAL ONCOLOGIC TLS & HYPERCALCEMIA GUARD</div>
        <div id="test-tls-clearance-banner" class="tls-clearance-banner">ELECTROLYTES NORMALIZED & RENAL PROTECTION CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-tls-status-badge-base',
        '#test-tls-status-badge-critical',
        '#test-tls-status-badge-staged',
        '#test-tls-status-badge-cleared',
        '#test-tls-alert-banner',
        '#test-tls-clearance-banner',
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
