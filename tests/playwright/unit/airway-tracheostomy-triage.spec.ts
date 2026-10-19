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

test.describe('Brachycephalic Obstructive Airway Syndrome (BOAS), Laryngeal Paralysis & Temporary Tracheostomy Triage Cockpit UI (/airway-tracheostomy-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /airway-tracheostomy-triage renders seeded operations (AT-OP-101 Barnaby, AT-OP-102 Yukon, AT-OP-103 Pippin), KPI telemetry, #airway-alert-banner, and selects AT-OP-101 in active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/airway-tracheostomy-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#airway-tracheostomy-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Brachycephalic Obstructive Airway Syndrome (BOAS), Laryngeal Paralysis & Temporary Tracheostomy Triage Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-stridor')).toBeVisible();
    await expect(page.locator('#kpi-staged-intubation')).toBeVisible();
    await expect(page.locator('#kpi-patent-extubated')).toBeVisible();

    const barnabyCard = page.locator('#airway-operations-list .airway-op-card[data-op-id="AT-OP-101"]');
    const yukonCard = page.locator('#airway-operations-list .airway-op-card[data-op-id="AT-OP-102"]');
    const pippinCard = page.locator('#airway-operations-list .airway-op-card[data-op-id="AT-OP-103"]');

    await expect(barnabyCard).toBeVisible();
    await expect(barnabyCard).toHaveAttribute('data-status', 'STRIDOR_CYANOSIS_DYSPNEA_CRITICAL');
    await expect(yukonCard).toBeVisible();
    await expect(yukonCard).toHaveAttribute('data-status', 'BUTORPHANOL_DEX_INTUBATION_STAGED');
    await expect(pippinCard).toBeVisible();
    await expect(pippinCard).toHaveAttribute('data-status', 'PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED');

    await barnabyCard.locator('.airway-select-op-btn').click();
    await expect(page.locator('#workspace-op-title')).toContainText('Barnaby');
    await expect(page.locator('#airway-alert-banner')).toBeVisible();
    await expect(page.locator('#airway-alert-banner')).toContainText('Butorphanol');

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging Butorphanol + Dexamethasone-SP + Intubation plan on AT-OP-101 transitions status to BUTORPHANOL_DEX_INTUBATION_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/airway-tracheostomy-triage`);
    await page.waitForLoadState('networkidle');

    const barnabyCard = page.locator('#airway-operations-list .airway-op-card[data-op-id="AT-OP-101"]');
    await barnabyCard.locator('.airway-select-op-btn').click();

    await page.locator('#stage-rectal-temp-f').fill('102.6');
    await page.locator('#stage-spo2-pct').fill('93.5');
    await page.locator('#stage-inspiratory-effort').selectOption('MODERATE_STRIDOR_ORTHOPNEA');
    await page.locator('#stage-edema-grade').selectOption('GRADE_III_EVERTED_SACCULES_COLLAPSE');

    await page.locator('#btn-stage-stabilization').click();

    await expect(barnabyCard).toHaveAttribute('data-status', 'BUTORPHANOL_DEX_INTUBATION_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText('BUTORPHANOL_DEX_INTUBATION_STAGED');
  });

  test('3. Recording an aseptic tracheostomy care log (<=10s suction, >=60s pre-oxygenation) on AT-OP-101 appends log to workspace', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/airway-tracheostomy-triage`);
    await page.waitForLoadState('networkidle');

    const barnabyCard = page.locator('#airway-operations-list .airway-op-card[data-op-id="AT-OP-101"]');
    await barnabyCard.locator('.airway-select-op-btn').click();

    await page.locator('#care-preox-sec').fill('90');
    await page.locator('#care-suction-sec').fill('8');
    await page.locator('#care-saline-ml').fill('1.5');
    await page.locator('#care-post-spo2').fill('96.5');
    await page.locator('#care-tech-callsign').fill('RVT-GARCIA-12');
    await page.locator('#care-notes').fill('Inner cannula scrubbed sterilely; 8s atraumatic suction pass.');
    await page.locator('#care-inner-cannula').check();

    await page.locator('#btn-record-trach-care').click();

    await expect(page.locator('#workspace-care-logs-list')).toContainText('RVT-GARCIA-12');
    await expect(page.locator('#workspace-care-logs-list')).toContainText('96.5%');
  });

  test('4. Issuing extubation clearance on AT-OP-101 transitions status to PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED and displays #airway-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/airway-tracheostomy-triage`);
    await page.waitForLoadState('networkidle');

    const barnabyCard = page.locator('#airway-operations-list .airway-op-card[data-op-id="AT-OP-101"]');
    await barnabyCard.locator('.airway-select-op-btn').click();

    await page.locator('#clearance-spo2-pct').fill('98.0');
    await page.locator('#clearance-temp-f').fill('100.9');
    await page.locator('#clearance-vet-license').fill('DVM-WA-88301');
    await page.locator('#clearance-summary').fill(
      'Barnaby extubated smoothly with vigorous gag reflex, SpO2 98.0% on room air, normothermic 100.9°F, chest harness verified.'
    );
    await page.locator('#clearance-gag-vigorous').check();
    await page.locator('#clearance-trach-occluded').check();
    await page.locator('#clearance-harness-only').check();

    await page.locator('#btn-issue-extubation-clearance').click();

    await expect(barnabyCard).toHaveAttribute('data-status', 'PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED');
    await expect(page.locator('#airway-clearance-banner')).toBeVisible();
    await expect(page.locator('#airway-clearance-banner')).toContainText('DVM-WA-88301');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .airway-alert-banner, and .airway-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/airway-tracheostomy-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-airway-triage-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-critical" class="status-pill status-pill-critical" data-status="STRIDOR_CYANOSIS_DYSPNEA_CRITICAL">STRIDOR_CYANOSIS_DYSPNEA_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged" data-status="BUTORPHANOL_DEX_INTUBATION_STAGED">BUTORPHANOL_DEX_INTUBATION_STAGED</span>
        <span id="test-status-pill-safe" class="status-pill status-pill-safe" data-status="PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED">PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED</span>
        <div id="test-airway-alert-banner" class="airway-alert-banner">CRITICAL AIRWAY &amp; TRACHEOSTOMY HAZARD: Never apply forceful physical restraint without Butorphanol sedation</div>
        <div id="test-airway-clearance-banner" class="airway-clearance-banner">PATENT AIRWAY &amp; NORMOTHERMIC EXTUBATION CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-status-pill-critical',
        '#test-status-pill-staged',
        '#test-status-pill-safe',
        '#test-airway-alert-banner',
        '#test-airway-clearance-banner',
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
