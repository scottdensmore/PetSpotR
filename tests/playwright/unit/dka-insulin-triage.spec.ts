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

test.describe('Diabetic Ketoacidosis (DKA), Regular Insulin CRI, Hypokalemia/Hypophosphatemia & Cerebral Edema Triage Cockpit UI (/dka-insulin-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /dka-insulin-triage renders seeded operations (DKA-OP-101, DKA-OP-102, DKA-OP-103), KPI telemetry, hypokalemia & cerebral edema alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/dka-insulin-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#dka-insulin-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Diabetic Ketoacidosis (DKA), Regular Insulin CRI, Hypokalemia/Hypophosphatemia & Cerebral Edema Triage Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-acidosis')).toBeVisible();
    await expect(page.locator('#kpi-staged-insulin-cri')).toBeVisible();
    await expect(page.locator('#kpi-cleared-glargine')).toBeVisible();
    await expect(page.locator('#kpi-hypokalemia-alerts')).toBeVisible();
    await expect(page.locator('#dka-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const winstonCard = page.locator('#dka-operations-list .dka-op-card[data-op-id="DKA-OP-101"]');
    const daisyCard = page.locator('#dka-operations-list .dka-op-card[data-op-id="DKA-OP-102"]');
    const chesterCard = page.locator('#dka-operations-list .dka-op-card[data-op-id="DKA-OP-103"]');

    await expect(winstonCard).toBeVisible();
    await expect(winstonCard).toHaveAttribute('data-status', 'KETOACIDOSIS_DEHYDRATED_CRITICAL');
    await expect(winstonCard.locator('.dka-alert-banner')).toContainText('50 mg/dL/hr');

    await expect(daisyCard).toBeVisible();
    await expect(daisyCard).toHaveAttribute('data-status', 'KCL_PHOS_REGULAR_INSULIN_CRI_STAGED');

    await expect(chesterCard).toBeVisible();
    await expect(chesterCard).toHaveAttribute('data-status', 'KETONES_CLEARED_SUBQ_GLARGINE_READY');
    await expect(chesterCard.locator('.dka-clearance-banner')).toContainText('VET-CA-93104');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting DKA-OP-101 Winston populates the active workspace, displays DKA pitfall alert banner, and stages electrolyte & insulin regimen to KCL_PHOS_REGULAR_INSULIN_CRI_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dka-insulin-triage`);
    await page.waitForLoadState('networkidle');

    const winstonCard = page.locator('#dka-operations-list .dka-op-card[data-op-id="DKA-OP-101"]');
    await winstonCard.locator('.dka-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Winston');
    await expect(page.locator('#dka-alert-banner')).toContainText('3.3 mEq/L');
    await expect(page.locator('#dka-alert-banner')).toContainText('cerebral edema');

    await page.locator('#stage-weight-kg').fill('5.4');
    await page.locator('#stage-species').selectOption('FELINE_DKA_HEPATIC_LIPIDOSIS');
    await page.locator('#stage-blood-glucose').fill('210');
    await page.locator('#stage-potassium').fill('2.8');
    await page.locator('#stage-phosphorus').fill('1.6');
    await page.locator('#stage-dehydration').fill('6.0');
    await page.locator('#btn-stage-regimen').click();

    await expect(winstonCard).toHaveAttribute('data-status', 'KCL_PHOS_REGULAR_INSULIN_CRI_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'KCL_PHOS_REGULAR_INSULIN_CRI_STAGED'
    );
    await expect(page.locator('#regimen-kcl-meq')).toContainText('40 mEq/L');
    await expect(page.locator('#regimen-insulin-cri')).toContainText('1.1 U/kg/day');
    await expect(page.locator('#regimen-dextrose-pct')).toContainText('2.5%');
  });

  test('3. Recording a serial q2h glucose & electrolyte check on DKA-OP-101 appends check to #serial-checks-list and updates patient vitals', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dka-insulin-triage`);
    await page.waitForLoadState('networkidle');

    const winstonCard = page.locator('#dka-operations-list .dka-op-card[data-op-id="DKA-OP-101"]');
    await winstonCard.locator('.dka-select-op-btn').click();

    await page.locator('#check-blood-glucose').fill('185');
    await page.locator('#check-bhb').fill('1.6');
    await page.locator('#check-potassium').fill('3.9');
    await page.locator('#check-phosphorus').fill('3.1');
    await page.locator('#check-dextrose-percent').fill('2.5');
    await page.locator('#check-callsign').fill('DR-ENDO-14');
    await page.locator('#check-notes').fill('BG falling 38 mg/dL/hr; K+ corrected to 3.9 mEq/L; 2.5% dextrose running.');
    await page.locator('#check-mentation-normal').check();
    await page.locator('#btn-record-serial-check').click();

    await expect(page.locator('#serial-checks-list')).toContainText(
      'BG falling 38 mg/dL/hr; K+ corrected to 3.9 mEq/L; 2.5% dextrose running.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('185 mg/dL');
  });

  test('4. Issuing ketone-cleared SubQ Glargine discharge clearance on DKA-OP-101 transitions status to KETONES_CLEARED_SUBQ_GLARGINE_READY and reveals #dka-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dka-insulin-triage`);
    await page.waitForLoadState('networkidle');

    const winstonCard = page.locator('#dka-operations-list .dka-op-card[data-op-id="DKA-OP-101"]');
    await winstonCard.locator('.dka-select-op-btn').click();

    await page.locator('#clearance-final-bhb').fill('0.4');
    await page.locator('#clearance-final-ph').fill('7.39');
    await page.locator('#clearance-final-potassium').fill('4.2');
    await page.locator('#clearance-final-phosphorus').fill('3.8');
    await page.locator('#clearance-vet-license').fill('VET-ENDO-91042');
    await page.locator('#clearance-summary').fill('Ketones cleared; eating voluntarily; transitioned to SubQ Glargine.');
    await page.locator('#clearance-voluntary-eating').check();
    await page.locator('#clearance-subq-transition').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(winstonCard).toHaveAttribute('data-status', 'KETONES_CLEARED_SUBQ_GLARGINE_READY');
    await expect(page.locator('#dka-clearance-banner')).toBeVisible();
    await expect(page.locator('#dka-clearance-banner')).toContainText('VET-ENDO-91042');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .dka-alert-banner, and .dka-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dka-insulin-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-dka-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">FELINE_DKA_HEPATIC_LIPIDOSIS</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">KETOACIDOSIS_DEHYDRATED_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">KCL_PHOS_REGULAR_INSULIN_CRI_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">KETONES_CLEARED_SUBQ_GLARGINE_READY</span>
        <div id="test-dka-alert-banner" class="dka-alert-banner">CRITICAL DKA & CEREBRAL EDEMA PITFALL ALERT: Hold insulin if K+ < 3.3 mEq/L</div>
        <div id="test-dka-clearance-banner" class="dka-clearance-banner">KETONES CLEARED & SUBQ GLARGINE DISCHARGE CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-status-pill-base',
        '#test-status-pill-critical',
        '#test-status-pill-staged',
        '#test-status-pill-cleared',
        '#test-dka-alert-banner',
        '#test-dka-clearance-banner',
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
