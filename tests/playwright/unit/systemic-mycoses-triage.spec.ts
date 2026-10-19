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

test.describe('Acute Systemic Mycoses & Fungal Emergencies (Blastomycosis, Histoplasmosis, Coccidioidomycosis, Cryptococcosis & Sinonasal Aspergillosis), Urine/Serum MiraVista Antigen EIA, Itraconazole/Fluconazole, Amphotericin B Lipid Complex Nephrotoxicity Guard & Clotrimazole Trephination Cockpit UI (/systemic-mycoses-fungal-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /systemic-mycoses-fungal-triage renders seeded operations (MYC-OP-101, MYC-OP-102, MYC-OP-103), Clotrimazole Cribriform / Compounded Itraconazole / Amphotericin B Nephrotoxicity / Die-Off ARDS alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/systemic-mycoses-fungal-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#systemic-mycoses-cockpit')).toBeVisible();
    await expect(page.locator('#myc-alert-banner')).toBeVisible();
    await expect(page.locator('#myc-alert-banner')).toContainText('CLOTRIMAZOLE TREPHINATION CONTRAINDICATION');
    await expect(page.locator('#myc-alert-banner')).toContainText('COMPOUNDED BULK-POWDER ITRACONAZOLE');
    await expect(page.locator('#myc-alert-banner')).toContainText('AMPHOTERICIN B LIPID COMPLEX NEPHROTOXICITY GUARD');
    await expect(page.locator('#myc-alert-banner')).toContainText('Dexamethasone-SP');

    await expect(page.locator('#myc-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Northwoods');
    await expect(page.locator('#active-antigen')).toContainText('14.80 ng/mL');
    await expect(page.locator('#active-creatinine')).toContainText('1.30 mg/dL');

    const northwoodsCard = page.locator('#myc-operations-grid .myc-op-card[data-op-id="MYC-OP-101"]');
    const sonoraCard = page.locator('#myc-operations-grid .myc-op-card[data-op-id="MYC-OP-102"]');
    const sedonaCard = page.locator('#myc-operations-grid .myc-op-card[data-op-id="MYC-OP-103"]');

    await expect(northwoodsCard).toBeVisible();
    await expect(northwoodsCard).toHaveAttribute(
      'data-status',
      'AZOLE_AMPHOTERICIN_TREPHINATION_STAGED'
    );

    await expect(sonoraCard).toBeVisible();
    await expect(sonoraCard).toHaveAttribute(
      'data-status',
      'ANTIGEN_EIA_CREA_OPHTHALMIC_MONITORED'
    );

    await expect(sedonaCard).toBeVisible();
    await expect(sedonaCard).toHaveAttribute(
      'data-status',
      'TWO_NEGATIVE_ANTIGEN_EIA_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging FDA-approved Itraconazole & Amphotericin B Lipid Complex therapy on MYC-OP-101 transitions status to ANTIGEN_EIA_CREA_OPHTHALMIC_MONITORED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/systemic-mycoses-fungal-triage`);
    await page.waitForLoadState('networkidle');

    const northwoodsCard = page.locator('#myc-operations-grid .myc-op-card[data-op-id="MYC-OP-101"]');
    await northwoodsCard.click();

    await expect(page.locator('#active-op-title')).toContainText('MYC-OP-101');
    await page.locator('#stage-therapy-btn').click();

    await expect(northwoodsCard).toHaveAttribute(
      'data-status',
      'ANTIGEN_EIA_CREA_OPHTHALMIC_MONITORED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'ANTIGEN_EIA_CREA_OPHTHALMIC_MONITORED'
    );
  });

  test('3. Recording a MiraVista Antigen EIA & renal check on MYC-OP-101 updates #active-antigen, #active-creatinine, #active-alt, and appends to #antigen-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/systemic-mycoses-fungal-triage`);
    await page.waitForLoadState('networkidle');

    const northwoodsCard = page.locator('#myc-operations-grid .myc-op-card[data-op-id="MYC-OP-101"]');
    await northwoodsCard.click();

    await page.locator('#check-antigen-input').fill('0.0');
    await page.locator('#check-creatinine-input').fill('1.15');
    await page.locator('#check-alt-input').fill('58');
    await page.locator('#check-spo2-input').fill('98.0');
    await page.locator('#check-cribriform-intact-checkbox').check();
    await page.locator('#check-ampho-held-checkbox').uncheck();
    await page.locator('#check-chorioretinitis-resolved-checkbox').check();
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)');
    await page
      .locator('#check-note-input')
      .fill('Month 6 & Month 7 MiraVista Blastomyces urine antigen EIA both 0.00 ng/mL, Serum Creatinine 1.15 mg/dL, ALT 58 U/L, SpO2 98.0%, thoracic radiographs and chorioretinitis completely resolved.');
    await page.locator('#record-antigen-check-btn').click();

    await expect(page.locator('#active-antigen')).toContainText('0.00 ng/mL');
    await expect(page.locator('#active-creatinine')).toContainText('1.15 mg/dL');
    await expect(page.locator('#active-alt')).toContainText('58 U/L');
    await expect(page.locator('#antigen-checks-list')).toContainText(
      'Month 6 & Month 7 MiraVista Blastomyces urine antigen EIA both 0.00 ng/mL'
    );
  });

  test('4. Clearing MYC-OP-101 after two consecutive negative MiraVista Antigen EIA tests transitions status to TWO_NEGATIVE_ANTIGEN_EIA_CLEARED and reveals #myc-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/systemic-mycoses-fungal-triage`);
    await page.waitForLoadState('networkidle');

    const northwoodsCard = page.locator('#myc-operations-grid .myc-op-card[data-op-id="MYC-OP-101"]');
    await northwoodsCard.click();

    await page.locator('#clear-antigen-input').fill('0.0');
    await page.locator('#clear-creatinine-input').fill('1.15');
    await page.locator('#clear-two-negative-eia-checkbox').check();
    await page.locator('#clear-six-month-azole-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)');
    await page
      .locator('#clear-summary-input')
      .fill('Two consecutive negative MiraVista urine/serum antigen EIA tests (0.00 ng/mL) 30 days apart, Serum Creatinine 1.15 mg/dL (<= 1.8 mg/dL), ALT 58 U/L, SpO2 98.0%, and 6-month FDA-approved Itraconazole 5 mg/kg/day regimen verified; cleared.');
    await page.locator('#clear-myc-btn').click();

    await expect(northwoodsCard).toHaveAttribute(
      'data-status',
      'TWO_NEGATIVE_ANTIGEN_EIA_CLEARED'
    );
    await expect(page.locator('#myc-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .myc-status-badge, .myc-alert-banner, and .myc-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/systemic-mycoses-fungal-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-myc-badge-test-fixture';
      container.innerHTML = `
        <span id="test-myc-status-badge-base" class="myc-status-badge">AZOLE_AMPHOTERICIN_TREPHINATION_STAGED</span>
        <span id="test-myc-status-badge-critical" class="myc-status-badge myc-status-critical">AZOLE_AMPHOTERICIN_TREPHINATION_STAGED</span>
        <span id="test-myc-status-badge-staged" class="myc-status-badge myc-status-staged">ANTIGEN_EIA_CREA_OPHTHALMIC_MONITORED</span>
        <span id="test-myc-status-badge-cleared" class="myc-status-badge myc-status-cleared">TWO_NEGATIVE_ANTIGEN_EIA_CLEARED</span>
        <div id="test-myc-alert-banner" class="myc-alert-banner">CRITICAL SYSTEMIC MYCOSES & ANTIFUNGAL SAFETY GUARD</div>
        <div id="test-myc-clearance-banner" class="myc-clearance-banner">TWO CONSECUTIVE NEGATIVE MIRAVISTA ANTIGEN EIA & 6-MONTH AZOLE CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-myc-status-badge-base',
        '#test-myc-status-badge-critical',
        '#test-myc-status-badge-staged',
        '#test-myc-status-badge-cleared',
        '#test-myc-alert-banner',
        '#test-myc-clearance-banner',
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
