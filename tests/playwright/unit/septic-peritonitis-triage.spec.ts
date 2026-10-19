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

test.describe('Septic Peritonitis, Abdominocentesis Blood-Fluid Glucose/Lactate Gradient & Closed-Suction Jackson-Pratt Drain Cockpit UI (/septic-peritonitis-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /septic-peritonitis-triage renders seeded operations (SP-OP-101, SP-OP-102, SP-OP-103), KPI telemetry, septic gradient pitfall alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/septic-peritonitis-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#septic-peritonitis-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Septic Peritonitis, Abdominocentesis Blood-Fluid Glucose/Lactate Gradient & Closed-Suction Jackson-Pratt Drain Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-septic')).toBeVisible();
    await expect(page.locator('#kpi-staged-lavage')).toBeVisible();
    await expect(page.locator('#kpi-cleared-jp-removed')).toBeVisible();
    await expect(page.locator('#kpi-septic-gradient-alerts')).toBeVisible();
    await expect(page.locator('#peritonitis-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const baxterCard = page.locator(
      '#peritonitis-operations-list .peritonitis-op-card[data-op-id="SP-OP-101"]'
    );
    const nalaCard = page.locator(
      '#peritonitis-operations-list .peritonitis-op-card[data-op-id="SP-OP-102"]'
    );
    const gumboCard = page.locator(
      '#peritonitis-operations-list .peritonitis-op-card[data-op-id="SP-OP-103"]'
    );

    await expect(baxterCard).toBeVisible();
    await expect(baxterCard).toHaveAttribute(
      'data-status',
      'SEPTIC_EFFUSION_INTRACELLULAR_BACTERIA_CRITICAL'
    );
    await expect(baxterCard.locator('.peritonitis-alert-banner')).toContainText('chlorhexidine');

    await expect(nalaCard).toBeVisible();
    await expect(nalaCard).toHaveAttribute(
      'data-status',
      'EMPIRIC_ANTIBIOTIC_LAVAGE_JP_DRAIN_STAGED'
    );

    await expect(gumboCard).toBeVisible();
    await expect(gumboCard).toHaveAttribute(
      'data-status',
      'NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED'
    );
    await expect(gumboCard.locator('.peritonitis-clearance-banner')).toContainText(
      'VET-SURG-66109'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting SP-OP-101 Baxter populates the active workspace, displays antiseptic lavage pitfall alert banner, and stages surgical plan to EMPIRIC_ANTIBIOTIC_LAVAGE_JP_DRAIN_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/septic-peritonitis-triage`);
    await page.waitForLoadState('networkidle');

    const baxterCard = page.locator(
      '#peritonitis-operations-list .peritonitis-op-card[data-op-id="SP-OP-101"]'
    );
    await baxterCard.locator('.peritonitis-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Baxter');
    await expect(page.locator('#peritonitis-alert-banner')).toContainText('chlorhexidine');
    await expect(page.locator('#peritonitis-alert-banner')).toContainText('povidone-iodine');

    await page.locator('#stage-weight-kg').fill('31.0');
    await page.locator('#stage-species').selectOption('CANINE_GI_DEHISCENCE_PERFORATION');
    await page.locator('#stage-blood-glucose').fill('118.0');
    await page.locator('#stage-effusion-glucose').fill('42.0');
    await page.locator('#stage-blood-lactate').fill('2.4');
    await page.locator('#stage-effusion-lactate').fill('6.8');
    await page.locator('#btn-stage-plan').click();

    await expect(baxterCard).toHaveAttribute(
      'data-status',
      'EMPIRIC_ANTIBIOTIC_LAVAGE_JP_DRAIN_STAGED'
    );
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'EMPIRIC_ANTIBIOTIC_LAVAGE_JP_DRAIN_STAGED'
    );
    await expect(page.locator('#plan-glucose-gradient')).toContainText('76.0 mg/dL');
    await expect(page.locator('#plan-lactate-gradient')).toContainText('4.40 mmol/L');
    await expect(page.locator('#plan-amp-sulbactam-mg')).toContainText('930.0 mg');
    await expect(page.locator('#plan-lavage-liters')).toContainText('9.30 L');
  });

  test('3. Recording a Jackson-Pratt drain check on SP-OP-101 appends check to #drain-checks-list and updates patient cytology status', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/septic-peritonitis-triage`);
    await page.waitForLoadState('networkidle');

    const baxterCard = page.locator(
      '#peritonitis-operations-list .peritonitis-op-card[data-op-id="SP-OP-101"]'
    );
    await baxterCard.locator('.peritonitis-select-op-btn').click();

    await page.locator('#check-effusion-production').fill('1.8');
    await page.locator('#check-serum-albumin').fill('2.4');
    await page.locator('#check-map').fill('82');
    await page.locator('#check-callsign').fill('DR-SURG-14');
    await page
      .locator('#check-notes')
      .fill('JP suction bulbs compressed; serosanguinous output reduced to 1.8 mL/kg/day.');
    await page.locator('#check-intracellular-bacteria').uncheck();
    await page.locator('#check-degenerate-neutrophils').uncheck();
    await page.locator('#btn-record-drain-check').click();

    await expect(page.locator('#drain-checks-list')).toContainText(
      'JP suction bulbs compressed; serosanguinous output reduced to 1.8 mL/kg/day.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('NEGATIVE');
  });

  test('4. Issuing non-septic JP drain removal clearance on SP-OP-101 transitions status to NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED and reveals #peritonitis-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/septic-peritonitis-triage`);
    await page.waitForLoadState('networkidle');

    const baxterCard = page.locator(
      '#peritonitis-operations-list .peritonitis-op-card[data-op-id="SP-OP-101"]'
    );
    await baxterCard.locator('.peritonitis-select-op-btn').click();

    await page.locator('#clearance-final-output').fill('1.5');
    await page.locator('#clearance-final-albumin').fill('2.5');
    await page.locator('#clearance-vet-license').fill('VET-SURG-90512');
    await page
      .locator('#clearance-summary')
      .fill('Non-septic cytology verified; JP drain output 1.5 mL/kg/day; JP drains aseptically pulled.');
    await page.locator('#clearance-bacteria-negative').check();
    await page.locator('#clearance-enteral-nutrition').check();
    await page.locator('#clearance-jp-removed').check();
    await page.locator('#btn-issue-drain-clearance').click();

    await expect(baxterCard).toHaveAttribute(
      'data-status',
      'NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED'
    );
    await expect(page.locator('#peritonitis-clearance-banner')).toBeVisible();
    await expect(page.locator('#peritonitis-clearance-banner')).toContainText('VET-SURG-90512');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .peritonitis-alert-banner, and .peritonitis-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/septic-peritonitis-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-peritonitis-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">CANINE_GI_DEHISCENCE_PERFORATION</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">SEPTIC_EFFUSION_INTRACELLULAR_BACTERIA_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">EMPIRIC_ANTIBIOTIC_LAVAGE_JP_DRAIN_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED</span>
        <div id="test-peritonitis-alert-banner" class="peritonitis-alert-banner">CRITICAL SEPTIC PERITONITIS PITFALL ALERT: NEVER add chlorhexidine or povidone-iodine to abdominal lavage fluid</div>
        <div id="test-peritonitis-clearance-banner" class="peritonitis-clearance-banner">NON-SEPTIC CYTOLOGY & JP DRAIN REMOVAL CLEARANCE VERIFIED</div>
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
        '#test-peritonitis-alert-banner',
        '#test-peritonitis-clearance-banner',
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
