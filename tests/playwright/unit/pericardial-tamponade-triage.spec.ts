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

test.describe('Pericardial Effusion, Cardiac Tamponade, Electrical Alternans & Right-Sided 4th–5th ICS Pericardiocentesis Cockpit UI (/pericardial-tamponade-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /pericardial-tamponade-triage renders seeded operations (PT-OP-101, PT-OP-102, PT-OP-103), KPI telemetry, furosemide & left coronary artery alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/pericardial-tamponade-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#pericardial-tamponade-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Pericardial Effusion, Cardiac Tamponade, Electrical Alternans & Right-Sided 4th–5th ICS Pericardiocentesis Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-tamponade')).toBeVisible();
    await expect(page.locator('#kpi-staged-centesis')).toBeVisible();
    await expect(page.locator('#kpi-cleared-decompressed')).toBeVisible();
    await expect(page.locator('#kpi-furosemide-coronary-alerts')).toBeVisible();
    await expect(page.locator('#tamponade-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const brutusCard = page.locator('#tamponade-operations-list .tamponade-op-card[data-op-id="PT-OP-101"]');
    const mugsyCard = page.locator('#tamponade-operations-list .tamponade-op-card[data-op-id="PT-OP-102"]');
    const samsonCard = page.locator('#tamponade-operations-list .tamponade-op-card[data-op-id="PT-OP-103"]');

    await expect(brutusCard).toBeVisible();
    await expect(brutusCard).toHaveAttribute('data-status', 'TAMPONADE_RA_COLLAPSE_ALTERNANS_CRITICAL');
    await expect(brutusCard.locator('.tamponade-alert-banner')).toContainText('Furosemide');

    await expect(mugsyCard).toBeVisible();
    await expect(mugsyCard).toHaveAttribute('data-status', 'RIGHT_ICS_4_5_PERICARDIOCENTESIS_STAGED');

    await expect(samsonCard).toBeVisible();
    await expect(samsonCard).toHaveAttribute('data-status', 'DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED');
    await expect(samsonCard.locator('.tamponade-clearance-banner')).toContainText('VET-CARDIO-99204');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting PT-OP-101 Brutus populates the active workspace, displays Tamponade pitfall alert banner, and stages Right 4th–5th ICS pericardiocentesis plan to RIGHT_ICS_4_5_PERICARDIOCENTESIS_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pericardial-tamponade-triage`);
    await page.waitForLoadState('networkidle');

    const brutusCard = page.locator('#tamponade-operations-list .tamponade-op-card[data-op-id="PT-OP-101"]');
    await brutusCard.locator('.tamponade-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Brutus');
    await expect(page.locator('#tamponade-alert-banner')).toContainText('Furosemide');
    await expect(page.locator('#tamponade-alert-banner')).toContainText('coronary');

    await page.locator('#stage-weight-kg').fill('33.0');
    await page.locator('#stage-species').selectOption('CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA');
    await page.locator('#stage-heart-rate').fill('185');
    await page.locator('#stage-map').fill('52');
    await page.locator('#stage-pulsus-paradoxus').fill('18');
    await page.locator('#stage-effusion-pcv').fill('28.0');
    await page.locator('#btn-stage-plan').click();

    await expect(brutusCard).toHaveAttribute('data-status', 'RIGHT_ICS_4_5_PERICARDIOCENTESIS_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'RIGHT_ICS_4_5_PERICARDIOCENTESIS_STAGED'
    );
    await expect(page.locator('#plan-approach-ics')).toContainText('Right 4th-5th ICS Cardiac Notch');
    await expect(page.locator('#plan-lidocaine-infilt')).toContainText('3.30 mL');
    await expect(page.locator('#plan-lidocaine-iv')).toContainText('66.0 mg IV');
    await expect(page.locator('#plan-crystalloid-bolus')).toContainText('330.0 mL IV');
  });

  test('3. Recording a pericardiocentesis tap check on PT-OP-101 appends check to #tap-checks-list and updates patient vitals', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pericardial-tamponade-triage`);
    await page.waitForLoadState('networkidle');

    const brutusCard = page.locator('#tamponade-operations-list .tamponade-op-card[data-op-id="PT-OP-101"]');
    await brutusCard.locator('.tamponade-select-op-btn').click();

    await page.locator('#check-volume-aspirated').fill('260.0');
    await page.locator('#check-post-tap-map').fill('88');
    await page.locator('#check-post-tap-hr').fill('108');
    await page.locator('#check-callsign').fill('DR-CARDIO-11');
    await page.locator('#check-notes').fill('260 mL non-clotting port-wine effusion aspirated via right 4th-5th ICS; RA diastolic collapse and electrical alternans resolved.');
    await page.locator('#check-non-clotting-verified').check();
    await page.locator('#btn-record-tap-check').click();

    await expect(page.locator('#tap-checks-list')).toContainText(
      '260 mL non-clotting port-wine effusion aspirated via right 4th-5th ICS; RA diastolic collapse and electrical alternans resolved.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('88 mmHg');
  });

  test('4. Issuing pericardiocentesis decompression discharge clearance on PT-OP-101 transitions status to DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED and reveals #tamponade-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pericardial-tamponade-triage`);
    await page.waitForLoadState('networkidle');

    const brutusCard = page.locator('#tamponade-operations-list .tamponade-op-card[data-op-id="PT-OP-101"]');
    await brutusCard.locator('.tamponade-select-op-btn').click();

    await page.locator('#clearance-final-map').fill('94');
    await page.locator('#clearance-vet-license').fill('VET-CARDIO-99210');
    await page.locator('#clearance-summary').fill('Right 4th-5th ICS pericardiocentesis decompression complete; 24h repeat TFAST negative for reaccumulation; sinus rhythm without VPCs.');
    await page.locator('#clearance-ra-collapse-resolved').check();
    await page.locator('#clearance-tfast-no-reaccumulation').check();
    await page.locator('#clearance-sinus-no-vpcs').check();
    await page.locator('#clearance-oncology-educated').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(brutusCard).toHaveAttribute('data-status', 'DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED');
    await expect(page.locator('#tamponade-clearance-banner')).toBeVisible();
    await expect(page.locator('#tamponade-clearance-banner')).toContainText('VET-CARDIO-99210');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .tamponade-alert-banner, and .tamponade-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pericardial-tamponade-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-tamponade-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">TAMPONADE_RA_COLLAPSE_ALTERNANS_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">RIGHT_ICS_4_5_PERICARDIOCENTESIS_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED</span>
        <div id="test-tamponade-alert-banner" class="tamponade-alert-banner">CRITICAL PERICARDIAL TAMPONADE, FUROSEMIDE PRELOAD & LEFT CORONARY ARTERY PITFALL ALERT: Use Right 4th-5th ICS Cardiac Notch</div>
        <div id="test-tamponade-clearance-banner" class="tamponade-clearance-banner">PERICARDIOCENTESIS DECOMPRESSION & SINUS RHYTHM DISCHARGE CLEARANCE VERIFIED</div>
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
        '#test-tamponade-alert-banner',
        '#test-tamponade-clearance-banner',
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
