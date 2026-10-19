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

test.describe('Status Epilepticus, Cluster Seizures, Bromethalin/Metaldehyde Neuro-Tremor & Levetiracetam/Propofol CRI Cockpit UI (/status-epilepticus-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /status-epilepticus-triage renders seeded operations (SE-OP-101, SE-OP-102, SE-OP-103), KPI telemetry, neurogenic hyperthermia alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/status-epilepticus-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#status-epilepticus-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Status Epilepticus, Cluster Seizures, Bromethalin/Metaldehyde Neuro-Tremor & Levetiracetam/Propofol CRI Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-ictal')).toBeVisible();
    await expect(page.locator('#kpi-staged-cri')).toBeVisible();
    await expect(page.locator('#kpi-cleared-normothermic')).toBeVisible();
    await expect(page.locator('#kpi-hyperthermia-alerts')).toBeVisible();
    await expect(page.locator('#epilepticus-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const kodaCard = page.locator('#epilepticus-operations-list .epilepticus-op-card[data-op-id="SE-OP-101"]');
    const mochiCard = page.locator('#epilepticus-operations-list .epilepticus-op-card[data-op-id="SE-OP-102"]');
    const bowieCard = page.locator('#epilepticus-operations-list .epilepticus-op-card[data-op-id="SE-OP-103"]');

    await expect(kodaCard).toBeVisible();
    await expect(kodaCard).toHaveAttribute('data-status', 'ACTIVE_ICTAL_HYPERTHERMIA_CRITICAL');
    await expect(kodaCard.locator('.epilepticus-alert-banner')).toContainText('103.0°F');

    await expect(mochiCard).toBeVisible();
    await expect(mochiCard).toHaveAttribute('data-status', 'MIDAZOLAM_KEPPRA_METHOCARBAMOL_STAGED');

    await expect(bowieCard).toBeVisible();
    await expect(bowieCard).toHaveAttribute('data-status', 'SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED');
    await expect(bowieCard.locator('.epilepticus-clearance-banner')).toContainText('VET-CA-88412');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting SE-OP-101 Koda populates the active workspace, displays neuroprotection pitfall alert banner, and stages titration regimen to MIDAZOLAM_KEPPRA_METHOCARBAMOL_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/status-epilepticus-triage`);
    await page.waitForLoadState('networkidle');

    const kodaCard = page.locator('#epilepticus-operations-list .epilepticus-op-card[data-op-id="SE-OP-101"]');
    await kodaCard.locator('.epilepticus-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Koda');
    await expect(page.locator('#epilepticus-alert-banner')).toContainText('103.0°F');
    await expect(page.locator('#epilepticus-alert-banner')).toContainText('diazepam');

    await page.locator('#stage-weight-kg').fill('24.0');
    await page.locator('#stage-neuro-etiology').selectOption('IDIOPATHIC_STATUS_EPILEPTICUS');
    await page.locator('#stage-rectal-temp').fill('105.2');
    await page.locator('#stage-mgcs').fill('11');
    await page.locator('#btn-stage-regimen').click();

    await expect(kodaCard).toHaveAttribute('data-status', 'MIDAZOLAM_KEPPRA_METHOCARBAMOL_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'MIDAZOLAM_KEPPRA_METHOCARBAMOL_STAGED'
    );
    await expect(page.locator('#regimen-midazolam-mg')).toContainText('4.80 mg');
    await expect(page.locator('#regimen-levetiracetam-mg')).toContainText('1440.0 mg');
  });

  test('3. Recording a serial neuro ICU check on SE-OP-101 appends check to #neuro-checks-list and updates patient vitals', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/status-epilepticus-triage`);
    await page.waitForLoadState('networkidle');

    const kodaCard = page.locator('#epilepticus-operations-list .epilepticus-op-card[data-op-id="SE-OP-101"]');
    await kodaCard.locator('.epilepticus-select-op-btn').click();

    await page.locator('#check-rectal-temp').fill('101.6');
    await page.locator('#check-mgcs').fill('16');
    await page.locator('#check-plr').selectOption('NORMAL_BILATERAL');
    await page.locator('#check-blood-glucose').fill('96.0');
    await page.locator('#check-callsign').fill('DR-NEURO-41');
    await page.locator('#check-notes').fill('Active cooling stopped at 103.0°F; now normothermic 101.6°F.');
    await page.locator('#check-seizure-observed').uncheck();
    await page.locator('#btn-record-neuro-check').click();

    await expect(page.locator('#neuro-checks-list')).toContainText(
      'Active cooling stopped at 103.0°F; now normothermic 101.6°F.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('101.6°F');
  });

  test('4. Issuing 24h seizure-free normothermic discharge clearance on SE-OP-101 transitions status to SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED and reveals #epilepticus-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/status-epilepticus-triage`);
    await page.waitForLoadState('networkidle');

    const kodaCard = page.locator('#epilepticus-operations-list .epilepticus-op-card[data-op-id="SE-OP-101"]');
    await kodaCard.locator('.epilepticus-select-op-btn').click();

    await page.locator('#clearance-seizure-free-hours').fill('26');
    await page.locator('#clearance-final-mgcs').fill('18');
    await page.locator('#clearance-final-temp').fill('101.2');
    await page.locator('#clearance-vet-license').fill('VET-NEURO-90411');
    await page.locator('#clearance-summary').fill('26h seizure-free normothermic recovery; home intranasal midazolam dispensed.');
    await page.locator('#clearance-ambulatory-menace').check();
    await page.locator('#clearance-rescue-midazolam').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(kodaCard).toHaveAttribute('data-status', 'SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED');
    await expect(page.locator('#epilepticus-clearance-banner')).toBeVisible();
    await expect(page.locator('#epilepticus-clearance-banner')).toContainText('VET-NEURO-90411');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .epilepticus-alert-banner, and .epilepticus-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/status-epilepticus-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-epilepticus-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-etiology">IDIOPATHIC_STATUS_EPILEPTICUS</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">ACTIVE_ICTAL_HYPERTHERMIA_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">MIDAZOLAM_KEPPRA_METHOCARBAMOL_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED</span>
        <div id="test-epilepticus-alert-banner" class="epilepticus-alert-banner">CRITICAL NEUROPROTECTION PITFALL ALERT: NEVER place fingers in an ictal mouth</div>
        <div id="test-epilepticus-clearance-banner" class="epilepticus-clearance-banner">SEIZURE-FREE 24H NORMOTHERMIC DISCHARGE CLEARANCE VERIFIED</div>
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
        '#test-epilepticus-alert-banner',
        '#test-epilepticus-clearance-banner',
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
