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

test.describe('Tension Pneumothorax, Flail Chest, 7th–9th ICS Thoracocentesis & Heimlich/Pleural Seal Chest Tube Cockpit UI (/pneumothorax-thoracocentesis-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /pneumothorax-thoracocentesis-triage renders seeded operations (PTX-OP-101, PTX-OP-102, PTX-OP-103), KPI telemetry, tension radiograph & cranial rib margin alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/pneumothorax-thoracocentesis-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#pneumothorax-thoracocentesis-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Tension Pneumothorax, Flail Chest, 7th–9th ICS Thoracocentesis & Heimlich/Pleural Seal Chest Tube Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-tension')).toBeVisible();
    await expect(page.locator('#kpi-staged-thoracocentesis')).toBeVisible();
    await expect(page.locator('#kpi-cleared-pleural-seal')).toBeVisible();
    await expect(page.locator('#kpi-tension-radiograph-alerts')).toBeVisible();
    await expect(page.locator('#pneumothorax-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const maverickCard = page.locator(
      '#pneumothorax-operations-list .pneumothorax-op-card[data-op-id="PTX-OP-101"]'
    );
    const banditCard = page.locator(
      '#pneumothorax-operations-list .pneumothorax-op-card[data-op-id="PTX-OP-102"]'
    );
    const tangoCard = page.locator(
      '#pneumothorax-operations-list .pneumothorax-op-card[data-op-id="PTX-OP-103"]'
    );

    await expect(maverickCard).toBeVisible();
    await expect(maverickCard).toHaveAttribute(
      'data-status',
      'TENSION_PNEUMOTHORAX_ABSENT_GLIDE_CRITICAL'
    );
    await expect(maverickCard.locator('.pneumothorax-alert-banner')).toContainText('CRANIAL');

    await expect(banditCard).toBeVisible();
    await expect(banditCard).toHaveAttribute(
      'data-status',
      'ICS_7_9_THORACOCENTESIS_CHEST_TUBE_STAGED'
    );

    await expect(tangoCard).toBeVisible();
    await expect(tangoCard).toHaveAttribute(
      'data-status',
      'PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED'
    );
    await expect(tangoCard.locator('.pneumothorax-clearance-banner')).toContainText('VET-CO-99108');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting PTX-OP-101 Maverick populates the active workspace, displays pneumothorax pitfall alert banner, and stages 7th–9th ICS thoracocentesis & chest tube plan to ICS_7_9_THORACOCENTESIS_CHEST_TUBE_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pneumothorax-thoracocentesis-triage`);
    await page.waitForLoadState('networkidle');

    const maverickCard = page.locator(
      '#pneumothorax-operations-list .pneumothorax-op-card[data-op-id="PTX-OP-101"]'
    );
    await maverickCard.locator('.pneumothorax-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Maverick');
    await expect(page.locator('#pneumothorax-alert-banner')).toContainText('CRANIAL');
    await expect(page.locator('#pneumothorax-alert-banner')).toContainText('radiographs');

    await page.locator('#stage-weight-kg').fill('25.0');
    await page.locator('#stage-species').selectOption('CANINE_SPONTANEOUS_BULLAE');
    await page.locator('#stage-resp-rate').fill('48');
    await page.locator('#stage-spo2').fill('92.0');
    await page.locator('#stage-cumulative-air').fill('62.0');
    await page.locator('#stage-tap-count').fill('3');
    await page.locator('#stage-glide-sign').check();
    await page.locator('#btn-stage-plan').click();

    await expect(maverickCard).toHaveAttribute(
      'data-status',
      'ICS_7_9_THORACOCENTESIS_CHEST_TUBE_STAGED'
    );
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'ICS_7_9_THORACOCENTESIS_CHEST_TUBE_STAGED'
    );
    await expect(page.locator('#plan-needle-landmark')).toContainText('7th-9th ICS Dorsal Third');
    await expect(page.locator('#plan-expected-tap')).toContainText('750.0 mL');
    await expect(page.locator('#plan-chest-tube-indicated')).toContainText('YES');
    await expect(page.locator('#plan-continuous-suction')).toContainText('-15 cmH2O');
    await expect(page.locator('#plan-blood-patch')).toContainText('5.0 mL/kg');
  });

  test('3. Recording a pleural drainage & TFAST glide check on PTX-OP-101 appends check to #drainage-checks-list and updates patient vitals', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pneumothorax-thoracocentesis-triage`);
    await page.waitForLoadState('networkidle');

    const maverickCard = page.locator(
      '#pneumothorax-operations-list .pneumothorax-op-card[data-op-id="PTX-OP-101"]'
    );
    await maverickCard.locator('.pneumothorax-select-op-btn').click();

    await page.locator('#check-air-rate').fill('0.0');
    await page.locator('#check-fluid-rate').fill('1.4');
    await page.locator('#check-spo2').fill('97.0');
    await page.locator('#check-callsign').fill('DR-PULM-11');
    await page
      .locator('#check-notes')
      .fill('Zero air production on -15 cmH2O continuous suction; bilateral TFAST glide sign restored; SpO2 97%.');
    await page.locator('#check-glide-restored').check();
    await page.locator('#btn-record-drainage-check').click();

    await expect(page.locator('#drainage-checks-list')).toContainText(
      'Zero air production on -15 cmH2O continuous suction; bilateral TFAST glide sign restored; SpO2 97%.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('97.0%');
  });

  test('4. Issuing pleural seal & aseptic chest tube removal clearance on PTX-OP-101 transitions status to PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED and reveals #pneumothorax-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pneumothorax-thoracocentesis-triage`);
    await page.waitForLoadState('networkidle');

    const maverickCard = page.locator(
      '#pneumothorax-operations-list .pneumothorax-op-card[data-op-id="PTX-OP-101"]'
    );
    await maverickCard.locator('.pneumothorax-select-op-btn').click();

    await page.locator('#clearance-zero-air-hours').fill('26');
    await page.locator('#clearance-final-fluid').fill('1.2');
    await page.locator('#clearance-room-air-spo2').fill('97.5');
    await page.locator('#clearance-vet-license').fill('VET-PULM-99140');
    await page
      .locator('#clearance-summary')
      .fill('Zero air production for 26 hours; fluid output 1.2 mL/kg/day; bilateral TFAST glide sign confirmed; chest tube aseptically removed.');
    await page.locator('#clearance-glide-confirmed').check();
    await page.locator('#clearance-tube-removed').check();
    await page.locator('#btn-issue-removal-clearance').click();

    await expect(maverickCard).toHaveAttribute(
      'data-status',
      'PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED'
    );
    await expect(page.locator('#pneumothorax-clearance-banner')).toBeVisible();
    await expect(page.locator('#pneumothorax-clearance-banner')).toContainText('VET-PULM-99140');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .pneumothorax-alert-banner, and .pneumothorax-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pneumothorax-thoracocentesis-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-pneumothorax-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">CANINE_SPONTANEOUS_BULLAE</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">TENSION_PNEUMOTHORAX_ABSENT_GLIDE_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">ICS_7_9_THORACOCENTESIS_CHEST_TUBE_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED</span>
        <div id="test-pneumothorax-alert-banner" class="pneumothorax-alert-banner">CRITICAL TENSION PNEUMOTHORAX & INTERCOSTAL VESSEL PITFALL ALERT: Advance along CRANIAL rib margin at 7th-9th ICS</div>
        <div id="test-pneumothorax-clearance-banner" class="pneumothorax-clearance-banner">PLEURAL SEAL & ASEPTIC CHEST TUBE REMOVAL CLEARANCE VERIFIED</div>
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
        '#test-pneumothorax-alert-banner',
        '#test-pneumothorax-clearance-banner',
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
