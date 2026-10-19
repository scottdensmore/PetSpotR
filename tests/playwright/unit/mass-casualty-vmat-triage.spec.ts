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

test.describe('Acute Disaster Mass-Casualty Veterinary Triage (START/SALT Veterinary Color-Tagging: Immediate Red, Delayed Yellow, Minimal Green, Expectant Black), Decontamination Corridor, Field Hospital Surge Capacity & FEMA/AVMA VMAT Deployment Command Cockpit UI (/mass-casualty-vmat-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /mass-casualty-vmat-triage renders seeded operations (MCI-OP-101, MCI-OP-102, MCI-OP-103), Hot-to-Cold Zone Cross-Contamination Guard / START-SALT / Reverse-Triage Surge alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/mass-casualty-vmat-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#mass-casualty-vmat-cockpit')).toBeVisible();
    await expect(page.locator('#mci-alert-banner')).toBeVisible();
    await expect(page.locator('#mci-alert-banner')).toContainText(
      'HOT-TO-COLD ZONE CROSS-CONTAMINATION GUARD'
    );
    await expect(page.locator('#mci-alert-banner')).toContainText(
      'START/SALT Color-Tagging'
    );
    await expect(page.locator('#mci-alert-banner')).toContainText(
      'Waterproof RFID Triage Collars'
    );
    await expect(page.locator('#mci-alert-banner')).toContainText(
      'FEMA ICS-214'
    );
    await expect(page.locator('#mci-alert-banner')).toContainText(
      'Reverse-Triage Surge Protocol'
    );

    await expect(page.locator('#mci-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Kodiak');
    await expect(page.locator('#active-ats-score')).toContainText('17/25');
    await expect(page.locator('#active-lactate')).toContainText('6.80 mmol/L');
    await expect(page.locator('#active-spo2')).toContainText('87.5%');

    const kodiakCard = page.locator(
      '#mci-operations-grid .mci-op-card[data-op-id="MCI-OP-101"]'
    );
    const cinderCard = page.locator(
      '#mci-operations-grid .mci-op-card[data-op-id="MCI-OP-102"]'
    );
    const shastaCard = page.locator(
      '#mci-operations-grid .mci-op-card[data-op-id="MCI-OP-103"]'
    );

    await expect(kodiakCard).toBeVisible();
    await expect(kodiakCard).toHaveAttribute(
      'data-status',
      'START_SALT_TAGGED_DECON_STAGED'
    );

    await expect(cinderCard).toBeVisible();
    await expect(cinderCard).toHaveAttribute(
      'data-status',
      'DAMAGE_CONTROL_SURGE_ICU_MONITORED'
    );

    await expect(shastaCard).toBeVisible();
    await expect(shastaCard).toHaveAttribute(
      'data-status',
      'REUNIFIED_EVAC_TRANSPORT_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging warm-zone decontamination, damage-control resuscitation & ICS-214 protocol on MCI-OP-101 transitions status to DAMAGE_CONTROL_SURGE_ICU_MONITORED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/mass-casualty-vmat-triage`);
    await page.waitForLoadState('networkidle');

    const kodiakCard = page.locator(
      '#mci-operations-grid .mci-op-card[data-op-id="MCI-OP-101"]'
    );
    await kodiakCard.click();

    await expect(page.locator('#active-op-title')).toContainText('MCI-OP-101');
    await page.locator('#stage-vmat-btn').click();

    await expect(kodiakCard).toHaveAttribute(
      'data-status',
      'DAMAGE_CONTROL_SURGE_ICU_MONITORED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'DAMAGE_CONTROL_SURGE_ICU_MONITORED'
    );
  });

  test('3. Recording a VMAT surge reassessment check on MCI-OP-101 updates #active-ats-score, #active-lactate, #active-spo2, and appends to #reassessment-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/mass-casualty-vmat-triage`);
    await page.waitForLoadState('networkidle');

    const kodiakCard = page.locator(
      '#mci-operations-grid .mci-op-card[data-op-id="MCI-OP-101"]'
    );
    await kodiakCard.click();

    await page
      .locator('#check-triage-tag-input')
      .selectOption('YELLOW_DELAYED_PRIORITY_2_URGENT_SURGERY');
    await page.locator('#check-ats-score-input').fill('5');
    await page.locator('#check-lactate-input').fill('1.85');
    await page.locator('#check-spo2-input').fill('97.5');
    await page.locator('#check-surge-occupancy-input').fill('78.0');
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)');
    await page
      .locator('#check-note-input')
      .fill(
        'Post-resuscitation VMAT ICU check: arterial hemorrhage controlled, ATS improved to 5/25, blood lactate cleared to 1.85 mmol/L, SpO2 97.5%, surge bed occupancy reduced to 78.0%.'
      );
    await page.locator('#record-reassessment-btn').click();

    await expect(page.locator('#active-ats-score')).toContainText('5/25');
    await expect(page.locator('#active-lactate')).toContainText('1.85 mmol/L');
    await expect(page.locator('#active-spo2')).toContainText('97.5%');
    await expect(page.locator('#reassessment-checks-list')).toContainText(
      'Post-resuscitation VMAT ICU check: arterial hemorrhage controlled'
    );
  });

  test('4. Clearing MCI-OP-101 for reunification / medevac transport transitions status to REUNIFIED_EVAC_TRANSPORT_CLEARED and reveals #mci-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/mass-casualty-vmat-triage`);
    await page.waitForLoadState('networkidle');

    const kodiakCard = page.locator(
      '#mci-operations-grid .mci-op-card[data-op-id="MCI-OP-101"]'
    );
    await kodiakCard.click();

    await page.locator('#clear-lactate-input').fill('1.85');
    await page.locator('#clear-spo2-input').fill('97.5');
    await page.locator('#clear-rfid-verified-checkbox').check();
    await page.locator('#clear-ics214-signed-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)');
    await page
      .locator('#clear-summary-input')
      .fill(
        'Perfusion normalized (lactate 1.85 mmol/L <= 2.5 mmol/L), SpO2 97.5% (>= 95.0%), warm-zone gross decontamination verified, waterproof RFID triage collar & ISO microchip matched, and FEMA ICS-214 chain of custody signed; cleared.'
      );
    await page.locator('#clear-mci-btn').click();

    await expect(kodiakCard).toHaveAttribute(
      'data-status',
      'REUNIFIED_EVAC_TRANSPORT_CLEARED'
    );
    await expect(page.locator('#mci-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .mci-status-badge, .mci-alert-banner, and .mci-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/mass-casualty-vmat-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-mci-badge-test-fixture';
      container.innerHTML = `
        <span id="test-mci-status-badge-base" class="mci-status-badge">START_SALT_TAGGED_DECON_STAGED</span>
        <span id="test-mci-status-badge-critical" class="mci-status-badge mci-status-critical">START_SALT_TAGGED_DECON_STAGED</span>
        <span id="test-mci-status-badge-staged" class="mci-status-badge mci-status-staged">DAMAGE_CONTROL_SURGE_ICU_MONITORED</span>
        <span id="test-mci-status-badge-cleared" class="mci-status-badge mci-status-cleared">REUNIFIED_EVAC_TRANSPORT_CLEARED</span>
        <div id="test-mci-alert-banner" class="mci-alert-banner">CRITICAL MASS-CASUALTY VMAT & DECONTAMINATION GUARD</div>
        <div id="test-mci-clearance-banner" class="mci-clearance-banner">VMAT PERFUSION RESTORED, RFID/MICROCHIP MATCHED & FEMA ICS-214 CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-mci-status-badge-base',
        '#test-mci-status-badge-critical',
        '#test-mci-status-badge-staged',
        '#test-mci-status-badge-cleared',
        '#test-mci-alert-banner',
        '#test-mci-clearance-banner',
      ]) {
        const styles = await page.locator(selector).evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { color: cs.color, backgroundColor: cs.backgroundColor };
        });
        expect(styles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
        const ratio = contrastRatio(styles.color, styles.backgroundColor);
        expect(
          ratio,
          `${selector} in ${theme} mode should have > 7.1:1 contrast`
        ).toBeGreaterThan(7.1);
      }
    }
  });
});
