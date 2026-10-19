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

test.describe("Traumatic Brain Injury (TBI), Modified Glasgow Coma Scale (MGCS), Cushing's Reflex & Hypertonic Saline/Mannitol Cockpit UI (/traumatic-brain-triage)", () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /traumatic-brain-triage renders seeded operations (TBI-OP-101, TBI-OP-102, TBI-OP-103), KPI telemetry, Cushing & steroid alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/traumatic-brain-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#traumatic-brain-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      "Traumatic Brain Injury (TBI), Modified Glasgow Coma Scale (MGCS), Cushing's Reflex & Hypertonic Saline/Mannitol Triage Cockpit"
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-cushing-critical')).toBeVisible();
    await expect(page.locator('#kpi-staged-hts-mannitol')).toBeVisible();
    await expect(page.locator('#kpi-cleared-mgcs')).toBeVisible();
    await expect(page.locator('#kpi-cushing-steroid-alerts')).toBeVisible();
    await expect(page.locator('#tbi-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const atlasCard = page.locator('#tbi-operations-list .tbi-op-card[data-op-id="TBI-OP-101"]');
    const miloCard = page.locator('#tbi-operations-list .tbi-op-card[data-op-id="TBI-OP-102"]');
    const sierraCard = page.locator('#tbi-operations-list .tbi-op-card[data-op-id="TBI-OP-103"]');

    await expect(atlasCard).toBeVisible();
    await expect(atlasCard).toHaveAttribute('data-status', 'CUSHING_REFLEX_ICP_HERNIATION_CRITICAL');
    await expect(atlasCard.locator('.tbi-alert-banner')).toContainText('corticosteroids');

    await expect(miloCard).toBeVisible();
    await expect(miloCard).toHaveAttribute('data-status', 'HTS_MANNITOL_30DEG_HEAD_ELEVATION_STAGED');

    await expect(sierraCard).toBeVisible();
    await expect(sierraCard).toHaveAttribute('data-status', 'MGCS_RECOVERED_NORMOCAPNIC_CLEARED');
    await expect(sierraCard.locator('.tbi-clearance-banner')).toContainText('VET-CA-98412');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting TBI-OP-101 Atlas populates the active workspace, displays TBI pitfall alert banner, and stages 7.5% HTS / Mannitol / 30° head elevation plan to HTS_MANNITOL_30DEG_HEAD_ELEVATION_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/traumatic-brain-triage`);
    await page.waitForLoadState('networkidle');

    const atlasCard = page.locator('#tbi-operations-list .tbi-op-card[data-op-id="TBI-OP-101"]');
    await atlasCard.locator('.tbi-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Atlas');
    await expect(page.locator('#tbi-alert-banner')).toContainText('corticosteroids');
    await expect(page.locator('#tbi-alert-banner')).toContainText('jugular');

    await page.locator('#stage-weight-kg').fill('42.0');
    await page.locator('#stage-species').selectOption('CANINE_BLUNT_VEHICULAR_TBI');
    await page.locator('#stage-mgcs').fill('12');
    await page.locator('#stage-map-mmhg').fill('104');
    await page.locator('#stage-hr-bpm').fill('82');
    await page.locator('#stage-etco2-mmhg').fill('38.0');
    await page.locator('#stage-pupils').selectOption('MIOTIC_PINPOINT_BILATERAL');
    await page.locator('#btn-stage-plan').click();

    await expect(atlasCard).toHaveAttribute('data-status', 'HTS_MANNITOL_30DEG_HEAD_ELEVATION_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'HTS_MANNITOL_30DEG_HEAD_ELEVATION_STAGED'
    );
    await expect(page.locator('#plan-hts-dose')).toContainText('4.0 mL/kg (168.0 mL)');
    await expect(page.locator('#plan-mannitol-grams')).toContainText('21.00 g');
    await expect(page.locator('#plan-head-elevation')).toContainText('30°');
  });

  test('3. Recording a serial MGCS & neuro check on TBI-OP-101 appends check to #neuro-checks-list and updates patient vitals', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/traumatic-brain-triage`);
    await page.waitForLoadState('networkidle');

    const atlasCard = page.locator('#tbi-operations-list .tbi-op-card[data-op-id="TBI-OP-101"]');
    await atlasCard.locator('.tbi-select-op-btn').click();

    await page.locator('#check-mgcs').fill('15');
    await page.locator('#check-map-mmhg').fill('96');
    await page.locator('#check-hr-bpm').fill('88');
    await page.locator('#check-etco2-mmhg').fill('37.0');
    await page.locator('#check-pupils').selectOption('RESPONSIVE_ISOCORIC');
    await page.locator('#check-callsign').fill('DR-NEURO-11');
    await page.locator('#check-notes').fill("Cushing's reflex resolved; MGCS improved to 15/18; pupils isocoric.");
    await page.locator('#btn-record-neuro-check').click();

    await expect(page.locator('#neuro-checks-list')).toContainText(
      "Cushing's reflex resolved; MGCS improved to 15/18; pupils isocoric."
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('15/18');
  });

  test('4. Issuing MGCS-recovered normocapnic discharge clearance on TBI-OP-101 transitions status to MGCS_RECOVERED_NORMOCAPNIC_CLEARED and reveals #tbi-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/traumatic-brain-triage`);
    await page.waitForLoadState('networkidle');

    const atlasCard = page.locator('#tbi-operations-list .tbi-op-card[data-op-id="TBI-OP-101"]');
    await atlasCard.locator('.tbi-select-op-btn').click();

    await page.locator('#clearance-final-mgcs').fill('17');
    await page.locator('#clearance-final-map').fill('96');
    await page.locator('#clearance-vet-license').fill('VET-NEURO-98104');
    await page.locator('#clearance-summary').fill('MGCS recovered to 17/18; normotensive MAP 96 mmHg; responsive isocoric pupils; intact gag/swallow reflex.');
    await page.locator('#clearance-pupils-isocoric').check();
    await page.locator('#clearance-gag-reflex').check();
    await page.locator('#clearance-ambulatory-normocapnic').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(atlasCard).toHaveAttribute('data-status', 'MGCS_RECOVERED_NORMOCAPNIC_CLEARED');
    await expect(page.locator('#tbi-clearance-banner')).toBeVisible();
    await expect(page.locator('#tbi-clearance-banner')).toContainText('VET-NEURO-98104');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .tbi-alert-banner, and .tbi-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/traumatic-brain-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-tbi-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">CANINE_BLUNT_VEHICULAR_TBI</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">CUSHING_REFLEX_ICP_HERNIATION_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">HTS_MANNITOL_30DEG_HEAD_ELEVATION_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">MGCS_RECOVERED_NORMOCAPNIC_CLEARED</span>
        <div id="test-tbi-alert-banner" class="tbi-alert-banner">CRITICAL TBI PITFALL ALERT: Never administer corticosteroids or compress jugular veins</div>
        <div id="test-tbi-clearance-banner" class="tbi-clearance-banner">MGCS RECOVERED & NORMOCAPNIC DISCHARGE CLEARANCE VERIFIED</div>
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
        '#test-tbi-alert-banner',
        '#test-tbi-clearance-banner',
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
