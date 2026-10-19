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

test.describe('Hypoadrenocorticism (Addisonian Crisis), Na:K Ratio < 27, Dexamethasone-SP & DOCP Triage Cockpit UI (/addisonian-crisis-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /addisonian-crisis-triage renders seeded operations (AC-OP-101, AC-OP-102, AC-OP-103), KPI telemetry, osmotic demyelination & cortisol assay alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/addisonian-crisis-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#addisonian-crisis-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Hypoadrenocorticism (Addisonian Crisis), Na:K Ratio < 27, Dexamethasone-SP & DOCP Triage Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-shock')).toBeVisible();
    await expect(page.locator('#kpi-staged-dex-sp')).toBeVisible();
    await expect(page.locator('#kpi-cleared-docp')).toBeVisible();
    await expect(page.locator('#kpi-myelinolysis-alerts')).toBeVisible();
    await expect(page.locator('#addisonian-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const curlyCard = page.locator('#addisonian-operations-list .addisonian-op-card[data-op-id="AC-OP-101"]');
    const pippaCard = page.locator('#addisonian-operations-list .addisonian-op-card[data-op-id="AC-OP-102"]');
    const archieCard = page.locator('#addisonian-operations-list .addisonian-op-card[data-op-id="AC-OP-103"]');

    await expect(curlyCard).toBeVisible();
    await expect(curlyCard).toHaveAttribute('data-status', 'ADDISONIAN_SHOCK_HYPERKALEMIA_CRITICAL');
    await expect(curlyCard.locator('.addisonian-alert-banner')).toContainText('Dexamethasone-SP');

    await expect(pippaCard).toBeVisible();
    await expect(pippaCard).toHaveAttribute('data-status', 'DEX_SP_NACL_ACTH_STIM_STAGED');

    await expect(archieCard).toBeVisible();
    await expect(archieCard).toHaveAttribute('data-status', 'DOCP_PREDNISONE_EUKALEMIC_CLEARED');
    await expect(archieCard.locator('.addisonian-clearance-banner')).toContainText('VET-CA-96108');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting AC-OP-101 Curly populates the active workspace, displays Addisonian pitfall alert banner, and stages 0.9% NaCl / Dexamethasone-SP / ACTH Stim regimen to DEX_SP_NACL_ACTH_STIM_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/addisonian-crisis-triage`);
    await page.waitForLoadState('networkidle');

    const curlyCard = page.locator('#addisonian-operations-list .addisonian-op-card[data-op-id="AC-OP-101"]');
    await curlyCard.locator('.addisonian-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Curly');
    await expect(page.locator('#addisonian-alert-banner')).toContainText('Dexamethasone-SP');
    await expect(page.locator('#addisonian-alert-banner')).toContainText('myelinolysis');

    await page.locator('#stage-weight-kg').fill('23.5');
    await page.locator('#stage-species').selectOption('CANINE_TYPICAL_ADDISONS');
    await page.locator('#stage-sodium').fill('124.0');
    await page.locator('#stage-potassium').fill('7.8');
    await page.locator('#stage-heart-rate').fill('52');
    await page.locator('#stage-systolic-bp').fill('68');
    await page.locator('#btn-stage-regimen').click();

    await expect(curlyCard).toHaveAttribute('data-status', 'DEX_SP_NACL_ACTH_STIM_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'DEX_SP_NACL_ACTH_STIM_STAGED'
    );
    await expect(page.locator('#regimen-nak-ratio')).toContainText('15.9');
    await expect(page.locator('#regimen-nacl-bolus')).toContainText('470.0 mL');
    await expect(page.locator('#regimen-dex-sp')).toContainText('5.88 mg IV');
    await expect(page.locator('#regimen-cosyntropin')).toContainText('117.5 mcg IV');
  });

  test('3. Recording an electrolyte & post-ACTH cortisol check on AC-OP-101 appends check to #electrolyte-checks-list and updates patient vitals', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/addisonian-crisis-triage`);
    await page.waitForLoadState('networkidle');

    const curlyCard = page.locator('#addisonian-operations-list .addisonian-op-card[data-op-id="AC-OP-101"]');
    await curlyCard.locator('.addisonian-select-op-btn').click();

    await page.locator('#check-sodium').fill('131.0');
    await page.locator('#check-potassium').fill('5.2');
    await page.locator('#check-sodium-rise').fill('7.0');
    await page.locator('#check-post-acth-cortisol').fill('0.3');
    await page.locator('#check-heart-rate').fill('88');
    await page.locator('#check-callsign').fill('DR-ENDO-19');
    await page.locator('#check-notes').fill('Post-ACTH cortisol 0.3 mcg/dL; Na+ rose 7.0 mEq/L/24h; sinus rhythm restored.');
    await page.locator('#btn-record-electrolyte-check').click();

    await expect(page.locator('#electrolyte-checks-list')).toContainText(
      'Post-ACTH cortisol 0.3 mcg/dL; Na+ rose 7.0 mEq/L/24h; sinus rhythm restored.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('131.0 mEq/L');
  });

  test('4. Issuing DOCP & oral Prednisone eukalemic discharge clearance on AC-OP-101 transitions status to DOCP_PREDNISONE_EUKALEMIC_CLEARED and reveals #addisonian-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/addisonian-crisis-triage`);
    await page.waitForLoadState('networkidle');

    const curlyCard = page.locator('#addisonian-operations-list .addisonian-op-card[data-op-id="AC-OP-101"]');
    await curlyCard.locator('.addisonian-select-op-btn').click();

    await page.locator('#clearance-final-ratio').fill('31.5');
    await page.locator('#clearance-final-potassium').fill('4.4');
    await page.locator('#clearance-vet-license').fill('VET-ENDO-96108');
    await page.locator('#clearance-summary').fill('Post-ACTH cortisol confirmed < 2.0 mcg/dL; eukalemic; DOCP 2.2 mg/kg SubQ + oral Prednisone started.');
    await page.locator('#clearance-cortisol-confirmed').check();
    await page.locator('#clearance-docp-administered').check();
    await page.locator('#clearance-prednisone-started').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(curlyCard).toHaveAttribute('data-status', 'DOCP_PREDNISONE_EUKALEMIC_CLEARED');
    await expect(page.locator('#addisonian-clearance-banner')).toBeVisible();
    await expect(page.locator('#addisonian-clearance-banner')).toContainText('VET-ENDO-96108');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .addisonian-alert-banner, and .addisonian-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/addisonian-crisis-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-addisonian-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">CANINE_TYPICAL_ADDISONS</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">ADDISONIAN_SHOCK_HYPERKALEMIA_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">DEX_SP_NACL_ACTH_STIM_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">DOCP_PREDNISONE_EUKALEMIC_CLEARED</span>
        <div id="test-addisonian-alert-banner" class="addisonian-alert-banner">CRITICAL ADDISONIAN CRISIS & OSMOTIC DEMYELINATION PITFALL ALERT: Use Dexamethasone-SP 0.25 mg/kg IV</div>
        <div id="test-addisonian-clearance-banner" class="addisonian-clearance-banner">DOCP & ORAL PREDNISONE EUKALEMIC DISCHARGE CLEARANCE VERIFIED</div>
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
        '#test-addisonian-alert-banner',
        '#test-addisonian-clearance-banner',
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
