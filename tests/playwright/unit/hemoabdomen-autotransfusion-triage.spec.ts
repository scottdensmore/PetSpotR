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

test.describe('Acute Traumatic Hemoabdomen, Autotransfusion, Centesis Non-Clotting & Norepinephrine/TXA Resuscitation Cockpit UI (/hemoabdomen-autotransfusion-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /hemoabdomen-autotransfusion-triage renders seeded operations (HA-OP-101, HA-OP-102, HA-OP-103), KPI telemetry, clot-popping/autotransfusion alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/hemoabdomen-autotransfusion-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#hemoabdomen-autotransfusion-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Acute Traumatic Hemoabdomen, Autotransfusion, Centesis Non-Clotting & Norepinephrine/TXA Resuscitation Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-shock')).toBeVisible();
    await expect(page.locator('#kpi-staged-autotransfusion')).toBeVisible();
    await expect(page.locator('#kpi-cleared-hemostasis')).toBeVisible();
    await expect(page.locator('#kpi-clot-neoplasia-alerts')).toBeVisible();
    await expect(page.locator('#hemoabdomen-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const apolloCard = page.locator(
      '#hemoabdomen-operations-list .hemoabdomen-op-card[data-op-id="HA-OP-101"]'
    );
    const sadieCard = page.locator(
      '#hemoabdomen-operations-list .hemoabdomen-op-card[data-op-id="HA-OP-102"]'
    );
    const rangerCard = page.locator(
      '#hemoabdomen-operations-list .hemoabdomen-op-card[data-op-id="HA-OP-103"]'
    );

    await expect(apolloCard).toBeVisible();
    await expect(apolloCard).toHaveAttribute('data-status', 'HEMORRHAGIC_SHOCK_HEMOABDOMEN_CRITICAL');
    await expect(apolloCard.locator('.hemoabdomen-alert-banner')).toContainText('170 µm');

    await expect(sadieCard).toBeVisible();
    await expect(sadieCard).toHaveAttribute('data-status', 'AUTOTRANSFUSION_TXA_HYPOTENSIVE_STAGED');

    await expect(rangerCard).toBeVisible();
    await expect(rangerCard).toHaveAttribute('data-status', 'HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE');
    await expect(rangerCard.locator('.hemoabdomen-clearance-banner')).toContainText('VET-CA-94810');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting HA-OP-101 Apollo populates the active workspace, displays hemoabdomen pitfall alert banner, and stages resuscitation plan to AUTOTRANSFUSION_TXA_HYPOTENSIVE_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hemoabdomen-autotransfusion-triage`);
    await page.waitForLoadState('networkidle');

    const apolloCard = page.locator(
      '#hemoabdomen-operations-list .hemoabdomen-op-card[data-op-id="HA-OP-101"]'
    );
    await apolloCard.locator('.hemoabdomen-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Apollo');
    await expect(page.locator('#hemoabdomen-alert-banner')).toContainText('90 mL/kg');
    await expect(page.locator('#hemoabdomen-alert-banner')).toContainText('170');

    await page.locator('#stage-weight-kg').fill('34.0');
    await page.locator('#stage-species').selectOption('CANINE_BLUNT_TRAUMA_HEPATIC');
    await page.locator('#stage-effusion-pcv').fill('38.0');
    await page.locator('#stage-map-mmhg').fill('52');
    await page.locator('#stage-non-clotting-verified').check();
    await page.locator('#stage-suspected-neoplasia').uncheck();
    await page.locator('#btn-stage-plan').click();

    await expect(apolloCard).toHaveAttribute('data-status', 'AUTOTRANSFUSION_TXA_HYPOTENSIVE_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'AUTOTRANSFUSION_TXA_HYPOTENSIVE_STAGED'
    );
    await expect(page.locator('#plan-txa-mg')).toContainText('340.0 mg');
    await expect(page.locator('#plan-max-collect-ml')).toContainText('510.0 mL');
  });

  test('3. Recording a serial hemodynamic & autotransfusion check on HA-OP-101 appends check to #hemodynamic-checks-list and updates patient vitals', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hemoabdomen-autotransfusion-triage`);
    await page.waitForLoadState('networkidle');

    const apolloCard = page.locator(
      '#hemoabdomen-operations-list .hemoabdomen-op-card[data-op-id="HA-OP-101"]'
    );
    await apolloCard.locator('.hemoabdomen-select-op-btn').click();

    await page.locator('#check-map-mmhg').fill('68');
    await page.locator('#check-hr-bpm').fill('118');
    await page.locator('#check-peripheral-pcv').fill('33.0');
    await page.locator('#check-total-solids').fill('5.4');
    await page.locator('#check-lactate').fill('2.3');
    await page.locator('#check-autotransfused-ml').fill('350');
    await page.locator('#check-callsign').fill('DR-TRAUMA-14');
    await page
      .locator('#check-notes')
      .fill('350 mL autologous shed blood reinfused via 170 µm filter; MAP 68 mmHg.');
    await page.locator('#btn-record-hemodynamic-check').click();

    await expect(page.locator('#hemodynamic-checks-list')).toContainText(
      '350 mL autologous shed blood reinfused via 170 µm filter; MAP 68 mmHg.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('68 mmHg');
  });

  test('4. Issuing surgical/spontaneous hemostasis clearance on HA-OP-101 transitions status to HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE and reveals #hemoabdomen-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hemoabdomen-autotransfusion-triage`);
    await page.waitForLoadState('networkidle');

    const apolloCard = page.locator(
      '#hemoabdomen-operations-list .hemoabdomen-op-card[data-op-id="HA-OP-101"]'
    );
    await apolloCard.locator('.hemoabdomen-select-op-btn').click();

    await page.locator('#clearance-final-map').fill('84');
    await page.locator('#clearance-final-pcv').fill('34.0');
    await page.locator('#clearance-final-lactate').fill('1.5');
    await page.locator('#clearance-vet-license').fill('VET-SURG-90814');
    await page
      .locator('#clearance-summary')
      .fill('Surgical hemostasis confirmed; repeat AFAST AFS 0/4 free fluid resolved.');
    await page.locator('#clearance-surgical-hemostasis').check();
    await page.locator('#clearance-repeat-afast-resolved').check();
    await page.locator('#btn-issue-hemostasis-clearance').click();

    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE'
    );
    await expect(page.locator('#hemoabdomen-clearance-banner')).toBeVisible();
    await expect(page.locator('#hemoabdomen-clearance-banner')).toContainText('VET-SURG-90814');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .hemoabdomen-alert-banner, and .hemoabdomen-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hemoabdomen-autotransfusion-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-hemoabdomen-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">CANINE_BLUNT_TRAUMA_HEPATIC</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">HEMORRHAGIC_SHOCK_HEMOABDOMEN_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">AUTOTRANSFUSION_TXA_HYPOTENSIVE_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE</span>
        <div id="test-hemoabdomen-alert-banner" class="hemoabdomen-alert-banner">CRITICAL HEMOABDOMEN RESUSCITATION PITFALL ALERT: NEVER administer 90 mL/kg shock boluses</div>
        <div id="test-hemoabdomen-clearance-banner" class="hemoabdomen-clearance-banner">HEMOSTASIS & NORMOTENSIVE CLEARANCE VERIFIED</div>
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
        '#test-hemoabdomen-alert-banner',
        '#test-hemoabdomen-clearance-banner',
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
