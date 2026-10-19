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

test.describe('Acute Rabies Quarantine, Lyssavirus Post-Exposure Prophylaxis (PEP), Strict 10-Day Domestic Bite Observation vs. 45-Day/4-Month Unvaccinated Quarantine, Biosecurity Double-Barrier Isolation & Public Health Rabies Control Officer (RCO) Statutory Reporting Cockpit UI (/rabies-quarantine-pep-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /rabies-quarantine-pep-triage renders seeded operations (RBQ-OP-101, RBQ-OP-102, RBQ-OP-103), NASPHV 10-Day Bite Hold Vaccine Prohibition / Double-Barrier PPE / RFFIT / DFA 4°C alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/rabies-quarantine-pep-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#rabies-quarantine-cockpit')).toBeVisible();
    await expect(page.locator('#rbq-alert-banner')).toBeVisible();
    await expect(page.locator('#rbq-alert-banner')).toContainText('10-DAY BITE HOLD VACCINE PROHIBITION');
    await expect(page.locator('#rbq-alert-banner')).toContainText('povidone-iodine');
    await expect(page.locator('#rbq-alert-banner')).toContainText('0.50 IU/mL');
    await expect(page.locator('#rbq-alert-banner')).toContainText('DFA NEUROPATHOLOGY GUARD');

    await expect(page.locator('#rbq-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Koda');
    await expect(page.locator('#active-quarantine-days')).toContainText('Day 2 / 10 days');
    await expect(page.locator('#active-rffit-titer')).toContainText('0.25 IU/mL');

    const kodaCard = page.locator('#rbq-operations-grid .rbq-op-card[data-op-id="RBQ-OP-101"]');
    const vesperCard = page.locator('#rbq-operations-grid .rbq-op-card[data-op-id="RBQ-OP-102"]');
    const artemisCard = page.locator('#rbq-operations-grid .rbq-op-card[data-op-id="RBQ-OP-103"]');

    await expect(kodaCard).toBeVisible();
    await expect(kodaCard).toHaveAttribute(
      'data-status',
      'BIOSECURITY_ISOLATION_RCO_HOLD_STAGED'
    );

    await expect(vesperCard).toBeVisible();
    await expect(vesperCard).toHaveAttribute(
      'data-status',
      'SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED'
    );

    await expect(artemisCard).toBeVisible();
    await expect(artemisCard).toHaveAttribute(
      'data-status',
      'STATUTORY_QUARANTINE_SURVIVED_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging biosecurity double-barrier isolation & Public Health RCO statutory protocol on RBQ-OP-101 transitions status to SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/rabies-quarantine-pep-triage`);
    await page.waitForLoadState('networkidle');

    const kodaCard = page.locator('#rbq-operations-grid .rbq-op-card[data-op-id="RBQ-OP-101"]');
    await kodaCard.click();

    await expect(page.locator('#active-op-title')).toContainText('RBQ-OP-101');
    await page.locator('#stage-quarantine-btn').click();

    await expect(kodaCard).toHaveAttribute(
      'data-status',
      'SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED'
    );
  });

  test('3. Recording a neurological & salivary shedding observation check on RBQ-OP-101 updates #active-quarantine-days, #active-rffit-titer, and appends to #neuro-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/rabies-quarantine-pep-triage`);
    await page.waitForLoadState('networkidle');

    const kodaCard = page.locator('#rbq-operations-grid .rbq-op-card[data-op-id="RBQ-OP-101"]');
    await kodaCard.click();

    await page.locator('#check-day-elapsed-input').fill('10');
    await page.locator('#check-rffit-titer-input').fill('1.45');
    await page.locator('#check-bulbar-absent-checkbox').check();
    await page.locator('#check-dysphagia-absent-checkbox').check();
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)');
    await page
      .locator('#check-note-input')
      .fill('Day 10 of 10 statutory domestic bite observation complete: cranial nerves V, VII, IX, X, XII normal, drinking water freely without dysphagia or foamy hypersalivation, RFFIT titer 1.45 IU/mL.');
    await page.locator('#record-neuro-check-btn').click();

    await expect(page.locator('#active-quarantine-days')).toContainText('Day 10 / 10 days');
    await expect(page.locator('#active-rffit-titer')).toContainText('1.45 IU/mL');
    await expect(page.locator('#neuro-checks-list')).toContainText(
      'Day 10 of 10 statutory domestic bite observation complete'
    );
  });

  test('4. Clearing RBQ-OP-101 for statutory RCO release transitions status to STATUTORY_QUARANTINE_SURVIVED_CLEARED and reveals #rbq-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/rabies-quarantine-pep-triage`);
    await page.waitForLoadState('networkidle');

    const kodaCard = page.locator('#rbq-operations-grid .rbq-op-card[data-op-id="RBQ-OP-101"]');
    await kodaCard.click();

    await page.locator('#clear-day-elapsed-input').fill('10');
    await page.locator('#clear-rffit-titer-input').fill('1.45');
    await page.locator('#clear-rco-release-checkbox').check();
    await page.locator('#clear-exit-vaccine-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)');
    await page
      .locator('#clear-summary-input')
      .fill('Survived full 10-day NASPHV domestic bite observation (Day 10/10) with zero cranial nerve or bulbar palsy, RFFIT 1.45 IU/mL, Public Health RCO statutory release signed, and exit USDA-licensed rabies vaccine administered upon release; cleared.');
    await page.locator('#clear-rbq-btn').click();

    await expect(kodaCard).toHaveAttribute(
      'data-status',
      'STATUTORY_QUARANTINE_SURVIVED_CLEARED'
    );
    await expect(page.locator('#rbq-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .rbq-status-badge, .rbq-alert-banner, and .rbq-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/rabies-quarantine-pep-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-rbq-badge-test-fixture';
      container.innerHTML = `
        <span id="test-rbq-status-badge-base" class="rbq-status-badge">BIOSECURITY_ISOLATION_RCO_HOLD_STAGED</span>
        <span id="test-rbq-status-badge-critical" class="rbq-status-badge rbq-status-critical">BIOSECURITY_ISOLATION_RCO_HOLD_STAGED</span>
        <span id="test-rbq-status-badge-staged" class="rbq-status-badge rbq-status-staged">SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED</span>
        <span id="test-rbq-status-badge-cleared" class="rbq-status-badge rbq-status-cleared">STATUTORY_QUARANTINE_SURVIVED_CLEARED</span>
        <div id="test-rbq-alert-banner" class="rbq-alert-banner">CRITICAL NASPHV COMPENDIUM & PUBLIC HEALTH RCO BIOSECURITY GUARD</div>
        <div id="test-rbq-clearance-banner" class="rbq-clearance-banner">STATUTORY RABIES QUARANTINE SURVIVED, CRANIAL NERVES NORMAL & PUBLIC HEALTH RCO RELEASE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-rbq-status-badge-base',
        '#test-rbq-status-badge-critical',
        '#test-rbq-status-badge-staged',
        '#test-rbq-status-badge-cleared',
        '#test-rbq-alert-banner',
        '#test-rbq-clearance-banner',
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
