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

test.describe('Acute Anaphylaxis, Hemoabdomen Halo Sign (Gallbladder Wall Edema), Epinephrine IM/CRI & Venous Pool Resuscitation Cockpit UI (/anaphylaxis-halo-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /anaphylaxis-halo-triage renders seeded operations (AH-OP-101, AH-OP-102, AH-OP-103), KPI telemetry, gallbladder Halo Sign cholecystectomy alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/anaphylaxis-halo-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#anaphylaxis-halo-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Acute Anaphylaxis, Hemoabdomen Halo Sign (Gallbladder Wall Edema), Epinephrine IM/CRI & Venous Pool Resuscitation Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-shock')).toBeVisible();
    await expect(page.locator('#kpi-staged-epinephrine')).toBeVisible();
    await expect(page.locator('#kpi-cleared-biphasic')).toBeVisible();
    await expect(page.locator('#kpi-hepatic-alerts')).toBeVisible();
    await expect(page.locator('#anaphylaxis-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const waffleCard = page.locator('#anaphylaxis-operations-list .anaphylaxis-op-card[data-op-id="AH-OP-101"]');
    const brambleCard = page.locator('#anaphylaxis-operations-list .anaphylaxis-op-card[data-op-id="AH-OP-102"]');
    const cloverCard = page.locator('#anaphylaxis-operations-list .anaphylaxis-op-card[data-op-id="AH-OP-103"]');

    await expect(waffleCard).toBeVisible();
    await expect(waffleCard).toHaveAttribute('data-status', 'ANAPHYLACTIC_SHOCK_HALO_SIGN_CRITICAL');
    await expect(waffleCard.locator('.anaphylaxis-alert-banner')).toContainText('Halo Sign');

    await expect(brambleCard).toBeVisible();
    await expect(brambleCard).toHaveAttribute('data-status', 'EPINEPHRINE_IM_CRI_CRYSTALLOID_FFP_STAGED');

    await expect(cloverCard).toBeVisible();
    await expect(cloverCard).toHaveAttribute('data-status', 'BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED');
    await expect(cloverCard.locator('.anaphylaxis-clearance-banner')).toContainText('VET-ECC-99103');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting AH-OP-101 Waffle populates the active workspace, displays Anaphylaxis Halo Sign pitfall alert banner, and stages Epinephrine IM/CRI, Crystalloid & FFP plan to EPINEPHRINE_IM_CRI_CRYSTALLOID_FFP_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/anaphylaxis-halo-triage`);
    await page.waitForLoadState('networkidle');

    const waffleCard = page.locator('#anaphylaxis-operations-list .anaphylaxis-op-card[data-op-id="AH-OP-101"]');
    await waffleCard.locator('.anaphylaxis-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Waffle');
    await expect(page.locator('#anaphylaxis-alert-banner')).toContainText('Halo Sign');
    await expect(page.locator('#anaphylaxis-alert-banner')).toContainText('Diphenhydramine');

    await page.locator('#stage-weight-kg').fill('6.8');
    await page.locator('#stage-species').selectOption('CANINE_HYMENOPTERA_HEMOABDOMEN');
    await page.locator('#stage-gb-wall-mm').fill('6.4');
    await page.locator('#stage-map-mmhg').fill('46');
    await page.locator('#stage-pcv-pct').fill('68.0');
    await page.locator('#stage-pt-sec').fill('28.5');
    await page.locator('#btn-stage-plan').click();

    await expect(waffleCard).toHaveAttribute('data-status', 'EPINEPHRINE_IM_CRI_CRYSTALLOID_FFP_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'EPINEPHRINE_IM_CRI_CRYSTALLOID_FFP_STAGED'
    );
    await expect(page.locator('#plan-epi-im')).toContainText('0.07 mg IM');
    await expect(page.locator('#plan-epi-cri')).toContainText('0.34 mcg/min');
    await expect(page.locator('#plan-crystalloid-bolus')).toContainText('136.0 mL IV');
    await expect(page.locator('#plan-ffp-ml')).toContainText('102.0 mL IV');
  });

  test('3. Recording a 24h biphasic & gallbladder Halo Sign check on AH-OP-101 appends check to #biphasic-checks-list and updates patient vitals', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/anaphylaxis-halo-triage`);
    await page.waitForLoadState('networkidle');

    const waffleCard = page.locator('#anaphylaxis-operations-list .anaphylaxis-op-card[data-op-id="AH-OP-101"]');
    await waffleCard.locator('.anaphylaxis-select-op-btn').click();

    await page.locator('#check-map-mmhg').fill('84');
    await page.locator('#check-gb-wall-mm').fill('2.3');
    await page.locator('#check-pcv-pct').fill('47.0');
    await page.locator('#check-alt-ul').fill('420');
    await page.locator('#check-callsign').fill('DR-ECC-41');
    await page.locator('#check-notes').fill('AFAST gallbladder wall edema resolved from 6.4 mm to 2.3 mm; MAP 84 mmHg off Epinephrine CRI; PCV normalized to 47%.');
    await page.locator('#check-biphasic-relapse').uncheck();
    await page.locator('#btn-record-biphasic-check').click();

    await expect(page.locator('#biphasic-checks-list')).toContainText(
      'AFAST gallbladder wall edema resolved from 6.4 mm to 2.3 mm; MAP 84 mmHg off Epinephrine CRI; PCV normalized to 47%.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('2.3 mm');
  });

  test('4. Issuing 24h biphasic normotensive & Halo Sign resolved discharge clearance on AH-OP-101 transitions status to BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED and reveals #anaphylaxis-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/anaphylaxis-halo-triage`);
    await page.waitForLoadState('networkidle');

    const waffleCard = page.locator('#anaphylaxis-operations-list .anaphylaxis-op-card[data-op-id="AH-OP-101"]');
    await waffleCard.locator('.anaphylaxis-select-op-btn').click();

    await page.locator('#clearance-observation-hours').fill('26');
    await page.locator('#clearance-final-map').fill('88');
    await page.locator('#clearance-vet-license').fill('VET-ECC-99108');
    await page.locator('#clearance-summary').fill('26h biphasic observation completed without relapse; AFAST gallbladder Halo Sign resolved; PT/PCV normalized; owner educated on EpiPen auto-injector.');
    await page.locator('#clearance-halo-resolved').check();
    await page.locator('#clearance-coagulopathy-normalized').check();
    await page.locator('#clearance-epipen-educated').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(waffleCard).toHaveAttribute('data-status', 'BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED');
    await expect(page.locator('#anaphylaxis-clearance-banner')).toBeVisible();
    await expect(page.locator('#anaphylaxis-clearance-banner')).toContainText('VET-ECC-99108');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .anaphylaxis-alert-banner, and .anaphylaxis-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/anaphylaxis-halo-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-anaphylaxis-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">CANINE_HYMENOPTERA_HEMOABDOMEN</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">ANAPHYLACTIC_SHOCK_HALO_SIGN_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">EPINEPHRINE_IM_CRI_CRYSTALLOID_FFP_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED</span>
        <div id="test-anaphylaxis-alert-banner" class="anaphylaxis-alert-banner">CRITICAL ANAPHYLAXIS, GALLBLADDER HALO SIGN & HEMOABDOMEN PITFALL ALERT: Do NOT perform cholecystectomy</div>
        <div id="test-anaphylaxis-clearance-banner" class="anaphylaxis-clearance-banner">24H BIPHASIC NORMOTENSIVE & HALO SIGN RESOLVED CLEARANCE VERIFIED</div>
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
        '#test-anaphylaxis-alert-banner',
        '#test-anaphylaxis-clearance-banner',
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
