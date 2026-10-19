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

test.describe('Cardiopulmonary Resuscitation (RECOVER Guidelines), Open/Closed-Chest Compression, EtCO2 > 15 mmHg & Defibrillation Cockpit UI (/recover-cpr-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /recover-cpr-triage renders seeded operations (CPR-OP-101, CPR-OP-102, CPR-OP-103), KPI telemetry, RECOVER anti-hyperventilation & low-dose Epinephrine alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/recover-cpr-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#recover-cpr-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Cardiopulmonary Resuscitation (RECOVER Guidelines), Open/Closed-Chest Compression, EtCO2 > 15 mmHg & Defibrillation Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-arrest')).toBeVisible();
    await expect(page.locator('#kpi-staged-cycle')).toBeVisible();
    await expect(page.locator('#kpi-cleared-rosc')).toBeVisible();
    await expect(page.locator('#kpi-pitfall-alerts')).toBeVisible();
    await expect(page.locator('#cpr-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const dukeCard = page.locator('#cpr-operations-list .cpr-op-card[data-op-id="CPR-OP-101"]');
    const gizmoCard = page.locator('#cpr-operations-list .cpr-op-card[data-op-id="CPR-OP-102"]');
    const pixelCard = page.locator('#cpr-operations-list .cpr-op-card[data-op-id="CPR-OP-103"]');

    await expect(dukeCard).toBeVisible();
    await expect(dukeCard).toHaveAttribute('data-status', 'CARDIOPULMONARY_ARREST_BLS_ALS_CRITICAL');
    await expect(dukeCard.locator('.cpr-alert-banner')).toContainText('10 breaths/min');

    await expect(gizmoCard).toBeVisible();
    await expect(gizmoCard).toHaveAttribute('data-status', 'RECOVER_2MIN_CYCLE_EPI_DEFIB_STAGED');

    await expect(pixelCard).toBeVisible();
    await expect(pixelCard).toHaveAttribute('data-status', 'ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE');
    await expect(pixelCard.locator('.cpr-clearance-banner')).toContainText('VET-RECOVER-99403');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting CPR-OP-101 Duke populates the active workspace, displays RECOVER pitfall alert banner, and stages 2-min ALS & Defib plan to RECOVER_2MIN_CYCLE_EPI_DEFIB_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/recover-cpr-triage`);
    await page.waitForLoadState('networkidle');

    const dukeCard = page.locator('#cpr-operations-list .cpr-op-card[data-op-id="CPR-OP-101"]');
    await dukeCard.locator('.cpr-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Duke');
    await expect(page.locator('#cpr-alert-banner')).toContainText('10 breaths/min');
    await expect(page.locator('#cpr-alert-banner')).toContainText('EVERY OTHER');

    await page.locator('#stage-weight-kg').fill('30.0');
    await page.locator('#stage-species').selectOption('CANINE_KEEL_CHEST_THORACIC_PUMP');
    await page.locator('#stage-arrest-rhythm').selectOption('VFIB_PULSELESS_VT_SHOCKABLE');
    await page.locator('#stage-etco2-mmhg').fill('16.5');
    await page.locator('#stage-cycles-completed').fill('2');
    await page.locator('#stage-reversible-present').check();
    await page.locator('#btn-stage-plan').click();

    await expect(dukeCard).toHaveAttribute('data-status', 'RECOVER_2MIN_CYCLE_EPI_DEFIB_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'RECOVER_2MIN_CYCLE_EPI_DEFIB_STAGED'
    );
    await expect(page.locator('#plan-hand-placement')).toContainText(
      'Directly Over Heart (4th-6th ICS)'
    );
    await expect(page.locator('#plan-epi-low-dose')).toContainText('0.30 mg IV/IO');
    await expect(page.locator('#plan-atropine-dose')).toContainText('1.20 mg IV/IO');
    await expect(page.locator('#plan-biphasic-defib')).toContainText('120.0 J');
    await expect(page.locator('#plan-amiodarone-dose')).toContainText('150.0 mg IV/IO');
  });

  test('3. Recording a 2-minute BLS/ALS cycle & EtCO2 check on CPR-OP-101 appends check to #cycle-checks-list and updates patient capnography EtCO2', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/recover-cpr-triage`);
    await page.waitForLoadState('networkidle');

    const dukeCard = page.locator('#cpr-operations-list .cpr-op-card[data-op-id="CPR-OP-101"]');
    await dukeCard.locator('.cpr-select-op-btn').click();

    await page.locator('#check-cycle-number').fill('2');
    await page.locator('#check-etco2-mmhg').fill('22.5');
    await page.locator('#check-rhythm-pause').fill('VFIB_PULSELESS_VT_SHOCKABLE');
    await page.locator('#check-pause-seconds').fill('6');
    await page.locator('#check-callsign').fill('DR-RECOVER-01');
    await page.locator('#check-notes').fill('Cycle 2 completed with compressor rotation; EtCO2 improved from 11.0 to 22.5 mmHg (> 15 mmHg target); 60 J biphasic shock delivered with <= 6s pause.');
    await page.locator('#check-compressor-rotated').check();
    await page.locator('#btn-record-cycle-check').click();

    await expect(page.locator('#cycle-checks-list')).toContainText(
      'Cycle 2 completed with compressor rotation; EtCO2 improved from 11.0 to 22.5 mmHg (> 15 mmHg target); 60 J biphasic shock delivered with <= 6s pause.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('22.5 mmHg');
  });

  test('4. Issuing sustained ROSC & normocapnic PCA clearance on CPR-OP-101 transitions status to ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE and reveals #cpr-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/recover-cpr-triage`);
    await page.waitForLoadState('networkidle');

    const dukeCard = page.locator('#cpr-operations-list .cpr-op-card[data-op-id="CPR-OP-101"]');
    await dukeCard.locator('.cpr-select-op-btn').click();

    await page.locator('#clearance-rosc-minutes').fill('30');
    await page.locator('#clearance-final-etco2').fill('39.0');
    await page.locator('#clearance-final-map').fill('92');
    await page.locator('#clearance-vet-license').fill('VET-RECOVER-99410');
    await page.locator('#clearance-summary').fill('Sustained ROSC 30 min after biphasic defibrillation and low-dose Epinephrine; normocapnic EtCO2 39.0 mmHg, MAP 92 mmHg, SpO2 titrated 94-98% avoiding hyperoxia.');
    await page.locator('#clearance-spo2-titrated').check();
    await page.locator('#clearance-hypothermia-maintained').check();
    await page.locator('#btn-issue-rosc-clearance').click();

    await expect(dukeCard).toHaveAttribute('data-status', 'ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE');
    await expect(page.locator('#cpr-clearance-banner')).toBeVisible();
    await expect(page.locator('#cpr-clearance-banner')).toContainText('VET-RECOVER-99410');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .cpr-alert-banner, and .cpr-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/recover-cpr-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-cpr-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">CANINE_KEEL_CHEST_THORACIC_PUMP</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">CARDIOPULMONARY_ARREST_BLS_ALS_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">RECOVER_2MIN_CYCLE_EPI_DEFIB_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE</span>
        <div id="test-cpr-alert-banner" class="cpr-alert-banner">CRITICAL RECOVER 2012/2024 BLS/ALS PITFALL ALERT: Do NOT hyperventilate > 10 breaths/min</div>
        <div id="test-cpr-clearance-banner" class="cpr-clearance-banner">ROSC ACHIEVED & NORMOCAPNIC PCA CLEARANCE VERIFIED</div>
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
        '#test-cpr-alert-banner',
        '#test-cpr-clearance-banner',
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
