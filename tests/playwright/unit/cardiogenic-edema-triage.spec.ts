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

test.describe('Acute Cardiogenic Pulmonary Edema, Mitral Valve Chordae Rupture, Furosemide/Pimobendan & Oxygen Cage Cockpit UI (/cardiogenic-edema-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /cardiogenic-edema-triage renders seeded operations (CE-OP-101 Oliver, CE-OP-102 Cleo, CE-OP-103 Teddy), selects CE-OP-101 in workspace, displays #cardiogenic-alert-banner, and has zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/cardiogenic-edema-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#cardiogenic-edema-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Acute Cardiogenic Pulmonary Edema, Mitral Valve Chordae Rupture, Furosemide/Pimobendan & Oxygen Cage Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-fulminant-critical')).toBeVisible();
    await expect(page.locator('#kpi-furosemide-staged')).toBeVisible();
    await expect(page.locator('#kpi-eupneic-cleared')).toBeVisible();
    await expect(page.locator('#kpi-fluid-guard-locks')).toBeVisible();

    const oliverCard = page.locator('#cardiogenic-operations-list .cardiogenic-op-card[data-op-id="CE-OP-101"]');
    const cleoCard = page.locator('#cardiogenic-operations-list .cardiogenic-op-card[data-op-id="CE-OP-102"]');
    const teddyCard = page.locator('#cardiogenic-operations-list .cardiogenic-op-card[data-op-id="CE-OP-103"]');

    await expect(oliverCard).toBeVisible();
    await expect(oliverCard).toHaveAttribute('data-status', 'FULMINANT_PULMONARY_EDEMA_CRITICAL');
    await expect(cleoCard).toBeVisible();
    await expect(cleoCard).toHaveAttribute('data-status', 'FUROSEMIDE_PIMOBENDAN_O2_CAGE_STAGED');
    await expect(teddyCard).toBeVisible();
    await expect(teddyCard).toHaveAttribute('data-status', 'EUPNEIC_BLINES_RESOLVED_CLEARED');

    // Select CE-OP-101 Oliver explicitly and verify active workspace & #cardiogenic-alert-banner
    await oliverCard.locator('.cardiogenic-select-op-btn').click();
    await expect(page.locator('#active-operation-workspace')).toContainText('Oliver');
    await expect(page.locator('#cardiogenic-alert-banner')).toBeVisible();
    await expect(page.locator('#cardiogenic-alert-banner')).toContainText('CRYSTALLOID');

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging Furosemide/Pimobendan/O2 Cage regimen on CE-OP-101 transitions status to FUROSEMIDE_PIMOBENDAN_O2_CAGE_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/cardiogenic-edema-triage`);
    await page.waitForLoadState('networkidle');

    const oliverCard = page.locator('#cardiogenic-operations-list .cardiogenic-op-card[data-op-id="CE-OP-101"]');
    await oliverCard.locator('.cardiogenic-select-op-btn').click();

    await page.locator('#stage-species').selectOption('CANINE_MMVD_STAGE_C_D');
    await page.locator('#stage-weight-kg').fill('9.2');
    await page.locator('#stage-resp-rate').fill('88');
    await page.locator('#stage-heart-rate').fill('190');
    await page.locator('#stage-murmur-grade').selectOption('GRADE_VI_THRILL_CHORDAE_RUPTURE');
    await page.locator('#stage-tfast-blines').fill('8');
    await page.locator('#stage-la-ao-ratio').fill('2.45');
    await page.locator('#btn-stage-regimen').click();

    await expect(oliverCard).toHaveAttribute('data-status', 'FUROSEMIDE_PIMOBENDAN_O2_CAGE_STAGED');
    await expect(page.locator('#workspace-regimen-box')).toContainText('Furosemide IV Bolus:');
  });

  test('3. Recording a serial diuresis telemetry check on CE-OP-101 appends telemetry to #workspace-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/cardiogenic-edema-triage`);
    await page.waitForLoadState('networkidle');

    const oliverCard = page.locator('#cardiogenic-operations-list .cardiogenic-op-card[data-op-id="CE-OP-101"]');
    await oliverCard.locator('.cardiogenic-select-op-btn').click();

    await page.locator('#check-resp-rate').fill('32');
    await page.locator('#check-urine-output').fill('4.5');
    await page.locator('#check-tfast-blines').fill('2');
    await page.locator('#check-sbp').fill('118');
    await page.locator('#check-creatinine').fill('1.3');
    await page.locator('#check-potassium').fill('3.9');
    await page.locator('#check-clinician').fill('DR-CARDIO-21');
    await page.locator('#check-notes').fill('Marked reduction in perihilar B-lines after furosemide CRI.');
    await page.locator('#btn-record-diuresis-check').click();

    await expect(page.locator('#workspace-checks-list')).toContainText('DR-CARDIO-21');
    await expect(page.locator('#workspace-checks-list')).toContainText('Marked reduction in perihilar B-lines');
  });

  test('4. Issuing discharge clearance on CE-OP-101 transitions status to EUPNEIC_BLINES_RESOLVED_CLEARED and displays #cardiogenic-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/cardiogenic-edema-triage`);
    await page.waitForLoadState('networkidle');

    const oliverCard = page.locator('#cardiogenic-operations-list .cardiogenic-op-card[data-op-id="CE-OP-101"]');
    await oliverCard.locator('.cardiogenic-select-op-btn').click();

    await page.locator('#clearance-final-srr').fill('22');
    await page.locator('#clearance-final-blines').fill('0');
    await page.locator('#clearance-spo2').fill('98.0');
    await page.locator('#clearance-vet-license').fill('VET-CARDIO-99201');
    await page.locator('#clearance-renal-stable').check();
    await page.locator('#clearance-srr-educated').check();
    await page.locator('#clearance-summary').fill('Eupneic on room air, B-lines resolved (0/site), owner educated on daily SRR < 30 bpm.');
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(oliverCard).toHaveAttribute('data-status', 'EUPNEIC_BLINES_RESOLVED_CLEARED');
    await expect(page.locator('#cardiogenic-clearance-banner')).toBeVisible();
    await expect(page.locator('#cardiogenic-clearance-banner')).toContainText('VET-CARDIO-99201');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .cardiogenic-alert-banner, and .cardiogenic-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/cardiogenic-edema-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-cardiogenic-edema-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill" class="status-pill">CANINE_MMVD_STAGE_C_D</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">FULMINANT_PULMONARY_EDEMA_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">FUROSEMIDE_PIMOBENDAN_O2_CAGE_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">EUPNEIC_BLINES_RESOLVED_CLEARED</span>
        <div id="test-cardiogenic-alert-banner" class="cardiogenic-alert-banner">ZERO IV CRYSTALLOID FLUID CONTRAINDICATION GUARD ACTIVE</div>
        <div id="test-cardiogenic-clearance-banner" class="cardiogenic-clearance-banner">EUPNEIC &amp; TFAST B-LINES RESOLVED DISCHARGE CLEARANCE ISSUED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-status-pill',
        '#test-status-pill-critical',
        '#test-status-pill-staged',
        '#test-status-pill-cleared',
        '#test-cardiogenic-alert-banner',
        '#test-cardiogenic-clearance-banner',
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
