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

test.describe('Acute Kidney Injury (AKI), Lily/Ethylene Glycol/Grape Nephrotoxicity, Oliguria/Anuria & Furosemide/Diltiazem/Dialysis Cockpit UI (/aki-dialysis-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /aki-dialysis-triage renders seeded operations (AKI-OP-101, AKI-OP-102, AKI-OP-103), KPI telemetry, anuric fluid overload & Fomepizole alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/aki-dialysis-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#aki-dialysis-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Acute Kidney Injury (AKI), Lily/Ethylene Glycol/Grape Nephrotoxicity, Oliguria/Anuria & Furosemide/Diltiazem/Dialysis Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-oliguric')).toBeVisible();
    await expect(page.locator('#kpi-staged-fomepizole')).toBeVisible();
    await expect(page.locator('#kpi-cleared-polyuric')).toBeVisible();
    await expect(page.locator('#kpi-anuric-alerts')).toBeVisible();
    await expect(page.locator('#aki-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const lotusCard = page.locator('#aki-operations-list .aki-op-card[data-op-id="AKI-OP-101"]');
    const dieselCard = page.locator('#aki-operations-list .aki-op-card[data-op-id="AKI-OP-102"]');
    const merlotCard = page.locator('#aki-operations-list .aki-op-card[data-op-id="AKI-OP-103"]');

    await expect(lotusCard).toBeVisible();
    await expect(lotusCard).toHaveAttribute('data-status', 'OLIGURIC_ANURIC_AZOTEMIA_CRITICAL');
    await expect(lotusCard.locator('.aki-alert-banner')).toContainText('Ins-and-Outs');

    await expect(dieselCard).toBeVisible();
    await expect(dieselCard).toHaveAttribute('data-status', 'FOMEPIZOLE_INS_OUTS_DIURESIS_STAGED');

    await expect(merlotCard).toBeVisible();
    await expect(merlotCard).toHaveAttribute('data-status', 'POLYURIC_CREATININE_PLATEAU_CLEARED');
    await expect(merlotCard.locator('.aki-clearance-banner')).toContainText('VET-RENAL-99303');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting AKI-OP-101 Lotus populates the active workspace, displays AKI anuric fluid overload & Fomepizole pitfall alert banner, and stages Fomepizole, Ins-and-Outs & Dialysis plan to FOMEPIZOLE_INS_OUTS_DIURESIS_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/aki-dialysis-triage`);
    await page.waitForLoadState('networkidle');

    const lotusCard = page.locator('#aki-operations-list .aki-op-card[data-op-id="AKI-OP-101"]');
    await lotusCard.locator('.aki-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Lotus');
    await expect(page.locator('#aki-alert-banner')).toContainText('Ins-and-Outs');
    await expect(page.locator('#aki-alert-banner')).toContainText('125 mg/kg');

    await page.locator('#stage-weight-kg').fill('4.5');
    await page.locator('#stage-species').selectOption('FELINE_LILIUM_NEPHROTOXICITY');
    await page.locator('#stage-iris-grade').fill('5');
    await page.locator('#stage-uop').fill('0.2');
    await page.locator('#stage-creatinine').fill('11.4');
    await page.locator('#stage-potassium').fill('7.4');
    await page.locator('#btn-stage-plan').click();

    await expect(lotusCard).toHaveAttribute('data-status', 'FOMEPIZOLE_INS_OUTS_DIURESIS_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'FOMEPIZOLE_INS_OUTS_DIURESIS_STAGED'
    );
    await expect(page.locator('#plan-fomepizole')).toContainText('562.5 mg IV');
    await expect(page.locator('#plan-ins-outs-rate')).toContainText('4.65 mL/hr');
    await expect(page.locator('#plan-furosemide-bolus')).toContainText('9.0 mg IV');
    await expect(page.locator('#plan-diltiazem-cri')).toContainText('0.3 mcg/kg/min');
    await expect(page.locator('#plan-hemodialysis')).toContainText('INDICATED');
  });

  test('3. Recording a closed-system UOP & renal telemetry check on AKI-OP-101 appends check to #renal-checks-list and updates patient vitals', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/aki-dialysis-triage`);
    await page.waitForLoadState('networkidle');

    const lotusCard = page.locator('#aki-operations-list .aki-op-card[data-op-id="AKI-OP-101"]');
    await lotusCard.locator('.aki-select-op-btn').click();

    await page.locator('#check-uop').fill('2.4');
    await page.locator('#check-creatinine').fill('1.9');
    await page.locator('#check-potassium').fill('4.2');
    await page.locator('#check-body-weight').fill('4.5');
    await page.locator('#check-fluid-balance').fill('0.0');
    await page.locator('#check-callsign').fill('DR-RENAL-11');
    await page
      .locator('#check-notes')
      .fill(
        'Post-hemodialysis & Furosemide/Diltiazem conversion: polyuric phase achieved (UOP 2.4 mL/kg/hr), Cr decreased from 11.4 to 1.9 mg/dL, K+ normalized to 4.2 mEq/L.'
      );
    await page.locator('#btn-record-renal-check').click();

    await expect(page.locator('#renal-checks-list')).toContainText(
      'Post-hemodialysis & Furosemide/Diltiazem conversion: polyuric phase achieved (UOP 2.4 mL/kg/hr), Cr decreased from 11.4 to 1.9 mg/dL, K+ normalized to 4.2 mEq/L.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('2.4 mL/kg/hr');
  });

  test('4. Issuing polyuric recovery & creatinine plateau discharge clearance on AKI-OP-101 transitions status to POLYURIC_CREATININE_PLATEAU_CLEARED and reveals #aki-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/aki-dialysis-triage`);
    await page.waitForLoadState('networkidle');

    const lotusCard = page.locator('#aki-operations-list .aki-op-card[data-op-id="AKI-OP-101"]');
    await lotusCard.locator('.aki-select-op-btn').click();

    await page.locator('#clearance-final-uop').fill('2.4');
    await page.locator('#clearance-final-creatinine').fill('1.8');
    await page.locator('#clearance-final-potassium').fill('4.2');
    await page.locator('#clearance-vet-license').fill('VET-RENAL-99308');
    await page
      .locator('#clearance-summary')
      .fill(
        'Polyuric conversion & non-azotemic creatinine plateau achieved (UOP 2.4 mL/kg/hr, Cr 1.8 mg/dL, K+ 4.2 mEq/L); euvolemic without pulmonary edema; owner educated on outpatient SubQ fluid taper.'
      );
    await page.locator('#clearance-euvolemic').check();
    await page.locator('#clearance-subq-educated').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(lotusCard).toHaveAttribute('data-status', 'POLYURIC_CREATININE_PLATEAU_CLEARED');
    await expect(page.locator('#aki-clearance-banner')).toBeVisible();
    await expect(page.locator('#aki-clearance-banner')).toContainText('VET-RENAL-99308');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .aki-alert-banner, and .aki-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/aki-dialysis-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-aki-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">FELINE_LILIUM_NEPHROTOXICITY</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">OLIGURIC_ANURIC_AZOTEMIA_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">FOMEPIZOLE_INS_OUTS_DIURESIS_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">POLYURIC_CREATININE_PLATEAU_CLEARED</span>
        <div id="test-aki-alert-banner" class="aki-alert-banner">CRITICAL AKI, ANURIC FLUID OVERLOAD & FOMEPIZOLE PITFALL ALERT: Do NOT force IV crystalloids in anuria</div>
        <div id="test-aki-clearance-banner" class="aki-clearance-banner">POLYURIC RECOVERY & CREATININE PLATEAU CLEARANCE VERIFIED</div>
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
        '#test-aki-alert-banner',
        '#test-aki-clearance-banner',
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
