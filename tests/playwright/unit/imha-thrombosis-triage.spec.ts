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

test.describe('Immune-Mediated Hemolytic Anemia (IMHA) & Thrombocytopenia (IMTP), Saline Agglutination, Spherocytosis & Thromboprophylaxis Cockpit UI (/imha-thrombosis-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /imha-thrombosis-triage renders seeded operations (IM-OP-101, IM-OP-102, IM-OP-103), KPI telemetry, pulmonary thromboembolism & zero-jugular alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/imha-thrombosis-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#imha-thrombosis-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Immune-Mediated Hemolytic Anemia (IMHA) & Thrombocytopenia (IMTP), Saline Agglutination, Spherocytosis & Thromboprophylaxis Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-hemolytic')).toBeVisible();
    await expect(page.locator('#kpi-staged-regimen')).toBeVisible();
    await expect(page.locator('#kpi-cleared-stable')).toBeVisible();
    await expect(page.locator('#kpi-pte-alerts')).toBeVisible();
    await expect(page.locator('#imha-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const pennyCard = page.locator('#imha-operations-list .imha-op-card[data-op-id="IM-OP-101"]');
    const scoutCard = page.locator('#imha-operations-list .imha-op-card[data-op-id="IM-OP-102"]');
    const mapleCard = page.locator('#imha-operations-list .imha-op-card[data-op-id="IM-OP-103"]');

    await expect(pennyCard).toBeVisible();
    await expect(pennyCard).toHaveAttribute('data-status', 'HEMOLYTIC_AGGLUTINATION_CRISIS_CRITICAL');
    await expect(pennyCard.locator('.imha-alert-banner')).toContainText('jugular');

    await expect(scoutCard).toBeVisible();
    await expect(scoutCard).toHaveAttribute('data-status', 'PREDNISOLONE_CLOPIDOGREL_PRBC_STAGED');

    await expect(mapleCard).toBeVisible();
    await expect(mapleCard).toHaveAttribute('data-status', 'PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE');
    await expect(mapleCard.locator('.imha-clearance-banner')).toContainText('VET-IM-97103');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting IM-OP-101 Penny populates the active workspace, displays PTE & zero-jugular alert banner, and stages immunosuppressive & thromboprophylaxis regimen to PREDNISOLONE_CLOPIDOGREL_PRBC_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/imha-thrombosis-triage`);
    await page.waitForLoadState('networkidle');

    const pennyCard = page.locator('#imha-operations-list .imha-op-card[data-op-id="IM-OP-101"]');
    await pennyCard.locator('.imha-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Penny');
    await expect(page.locator('#imha-alert-banner')).toContainText('30 K/µL');
    await expect(page.locator('#imha-alert-banner')).toContainText('1:4 saline');

    await page.locator('#stage-weight-kg').fill('11.5');
    await page.locator('#stage-species').selectOption('CANINE_PRIMARY_IMHA');
    await page.locator('#stage-pcv').fill('15.0');
    await page.locator('#stage-bilirubin').fill('4.8');
    await page.locator('#stage-platelets').fill('95');
    await page.locator('#stage-spherocytes').fill('14');
    await page.locator('#btn-stage-regimen').click();

    await expect(pennyCard).toHaveAttribute('data-status', 'PREDNISOLONE_CLOPIDOGREL_PRBC_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'PREDNISOLONE_CLOPIDOGREL_PRBC_STAGED'
    );
    await expect(page.locator('#regimen-prednisolone-mg')).toContainText('23.0 mg/day');
    await expect(page.locator('#regimen-clopidogrel-mg')).toContainText('23.0 mg/day');
    await expect(page.locator('#regimen-prbc-ml')).toContainText('172.5 mL');
  });

  test('3. Recording a serial q6h hemolysis & PTE respiratory check on IM-OP-101 appends check to #hemolysis-checks-list and updates patient vitals', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/imha-thrombosis-triage`);
    await page.waitForLoadState('networkidle');

    const pennyCard = page.locator('#imha-operations-list .imha-op-card[data-op-id="IM-OP-101"]');
    await pennyCard.locator('.imha-select-op-btn').click();

    await page.locator('#check-pcv').fill('23.0');
    await page.locator('#check-reticulocytes').fill('110.0');
    await page.locator('#check-bilirubin').fill('1.8');
    await page.locator('#check-respiratory-rate').fill('24');
    await page.locator('#check-callsign').fill('DR-HEME-11');
    await page.locator('#check-notes').fill('Post-pRBC PCV rose to 23.0%, regenerative reticulocytosis 110 K/uL, plasma clearing, eupneic.');
    await page.locator('#check-saline-persistent').uncheck();
    await page.locator('#btn-record-hemolysis-check').click();

    await expect(page.locator('#hemolysis-checks-list')).toContainText(
      'Post-pRBC PCV rose to 23.0%, regenerative reticulocytosis 110 K/uL, plasma clearing, eupneic.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('23.0%');
  });

  test('4. Issuing 48h stable PCV & reticulocytosis taper clearance on IM-OP-101 transitions status to PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE and reveals #imha-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/imha-thrombosis-triage`);
    await page.waitForLoadState('networkidle');

    const pennyCard = page.locator('#imha-operations-list .imha-op-card[data-op-id="IM-OP-101"]');
    await pennyCard.locator('.imha-select-op-btn').click();

    await page.locator('#clearance-final-pcv').fill('28.5');
    await page.locator('#clearance-final-reticulocytes').fill('138.0');
    await page.locator('#clearance-vet-license').fill('VET-IM-97410');
    await page.locator('#clearance-summary').fill('PCV stable at 28.5% for 48h; reticulocytes 138 K/uL; 1:4 saline agglutination resolved; Clopidogrel + Rivaroxaban & slow Prednisolone taper dispensed.');
    await page.locator('#clearance-agglutination-resolved').check();
    await page.locator('#clearance-no-pte-dyspnea').check();
    await page.locator('#clearance-thromboprophylaxis-dispensed').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(pennyCard).toHaveAttribute('data-status', 'PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE');
    await expect(page.locator('#imha-clearance-banner')).toBeVisible();
    await expect(page.locator('#imha-clearance-banner')).toContainText('VET-IM-97410');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .imha-alert-banner, and .imha-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/imha-thrombosis-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-imha-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">CANINE_PRIMARY_IMHA</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">HEMOLYTIC_AGGLUTINATION_CRISIS_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">PREDNISOLONE_CLOPIDOGREL_PRBC_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE</span>
        <div id="test-imha-alert-banner" class="imha-alert-banner">CRITICAL IMHA & PULMONARY THROMBOEMBOLISM PITFALL ALERT: Zero jugular venipuncture</div>
        <div id="test-imha-clearance-banner" class="imha-clearance-banner">48H STABLE PCV & REGENERATIVE RETICULOCYTOSIS DISCHARGE CLEARANCE VERIFIED</div>
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
        '#test-imha-alert-banner',
        '#test-imha-clearance-banner',
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
