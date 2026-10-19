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

test.describe('Acute Non-Cardiogenic Pulmonary Edema (NCPE), Electrocution (Oral Cord Bite), Upper Airway Obstruction / Post-Ictal Neurogenic Edema, Low-Dose Furosemide Controversy & CPAP/PEEP Mechanical Ventilation Cockpit UI (/ncpe-electrocution-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /ncpe-electrocution-triage renders seeded operations (NCPE-OP-101, NCPE-OP-102, NCPE-OP-103), Caudodorsal B-Lines / LA:Ao < 1.5 / High-Dose Furosemide Contraindication / CPAP-PEEP alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/ncpe-electrocution-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#ncpe-electrocution-cockpit')).toBeVisible();
    await expect(page.locator('#ncpe-alert-banner')).toBeVisible();
    await expect(page.locator('#ncpe-alert-banner')).toContainText('Caudodorsal Lung Field B-Lines');
    await expect(page.locator('#ncpe-alert-banner')).toContainText('LA:Ao < 1.5');
    await expect(page.locator('#ncpe-alert-banner')).toContainText('Furosemide');
    await expect(page.locator('#ncpe-alert-banner')).toContainText('Butorphanol');
    await expect(page.locator('#ncpe-alert-banner')).toContainText('PEEP');

    await expect(page.locator('#ncpe-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Sparky');
    await expect(page.locator('#active-pf-ratio')).toContainText('165.0');
    await expect(page.locator('#active-blines-count')).toContainText('14 B-Lines');

    const sparkyCard = page.locator('#ncpe-operations-grid .ncpe-op-card[data-op-id="NCPE-OP-101"]');
    const miloCard = page.locator('#ncpe-operations-grid .ncpe-op-card[data-op-id="NCPE-OP-102"]');
    const lunaCard = page.locator('#ncpe-operations-grid .ncpe-op-card[data-op-id="NCPE-OP-103"]');

    await expect(sparkyCard).toBeVisible();
    await expect(sparkyCard).toHaveAttribute(
      'data-status',
      'ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE'
    );

    await expect(miloCard).toBeVisible();
    await expect(miloCard).toHaveAttribute(
      'data-status',
      'OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED'
    );

    await expect(lunaCard).toBeVisible();
    await expect(lunaCard).toHaveAttribute(
      'data-status',
      'BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging O2 / Butorphanol / CPAP / PEEP protocol on NCPE-OP-101 transitions status to OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ncpe-electrocution-triage`);
    await page.waitForLoadState('networkidle');

    const sparkyCard = page.locator('#ncpe-operations-grid .ncpe-op-card[data-op-id="NCPE-OP-101"]');
    await sparkyCard.click();

    await expect(page.locator('#active-op-title')).toContainText('NCPE-OP-101');
    await page.locator('#stage-cpap-btn').click();

    await expect(sparkyCard).toHaveAttribute(
      'data-status',
      'OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED'
    );
  });

  test('3. Recording a serial respiratory & VetBLUE B-line check on NCPE-OP-101 updates #active-pf-ratio, #active-blines-count, and appends to #respiratory-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ncpe-electrocution-triage`);
    await page.waitForLoadState('networkidle');

    const sparkyCard = page.locator('#ncpe-operations-grid .ncpe-op-card[data-op-id="NCPE-OP-101"]');
    await sparkyCard.click();

    await page.locator('#check-pf-ratio-input').fill('395.0');
    await page.locator('#check-spo2-input').fill('98.0');
    await page.locator('#check-rr-input').fill('28');
    await page.locator('#check-blines-input').fill('0');
    await page.locator('#check-la2ao-input').fill('1.22');
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Nadia Varma, DACVECC');
    await page
      .locator('#check-note-input')
      .fill('Post-HFNC, Helmet CPAP & Butorphanol anxiolysis: caudodorsal B-lines resolved (0 B-lines), PaO2/FiO2 395, SpO2 98% on room air.');
    await page.locator('#record-respiratory-check-btn').click();

    await expect(page.locator('#active-pf-ratio')).toContainText('395.0');
    await expect(page.locator('#active-blines-count')).toContainText('0 B-Lines');
    await expect(page.locator('#respiratory-checks-list')).toContainText(
      'Post-HFNC, Helmet CPAP & Butorphanol anxiolysis: caudodorsal B-lines resolved'
    );
  });

  test('4. Clearing NCPE-OP-101 for room-air discharge transitions status to BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED and reveals #ncpe-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ncpe-electrocution-triage`);
    await page.waitForLoadState('networkidle');

    const sparkyCard = page.locator('#ncpe-operations-grid .ncpe-op-card[data-op-id="NCPE-OP-101"]');
    await sparkyCard.click();

    await page
      .locator('#clear-discharged-by-input')
      .fill('Dr. Nadia Varma, DACVECC');
    await page
      .locator('#clear-summary-input')
      .fill('Caudodorsal VetBLUE B-lines completely resolved (0 B-lines), normoxemic on room air (PaO2/FiO2 395, SpO2 98%), 5–7 day oral eschar monitoring scheduled.');
    await page.locator('#clear-ncpe-btn').click();

    await expect(sparkyCard).toHaveAttribute(
      'data-status',
      'BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED'
    );
    await expect(page.locator('#ncpe-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Nadia Varma, DACVECC'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .ncpe-status-badge, .ncpe-alert-banner, and .ncpe-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ncpe-electrocution-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-ncpe-badge-test-fixture';
      container.innerHTML = `
        <span id="test-ncpe-status-badge-base" class="ncpe-status-badge">ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE</span>
        <span id="test-ncpe-status-badge-critical" class="ncpe-status-badge ncpe-status-critical">ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE</span>
        <span id="test-ncpe-status-badge-staged" class="ncpe-status-badge ncpe-status-staged">OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED</span>
        <span id="test-ncpe-status-badge-cleared" class="ncpe-status-badge ncpe-status-cleared">BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED</span>
        <div id="test-ncpe-alert-banner" class="ncpe-alert-banner">CRITICAL NCPE & ORAL CORD ELECTROCUTION GUARD</div>
        <div id="test-ncpe-clearance-banner" class="ncpe-clearance-banner">CAUDODORSAL B-LINES RESOLVED & ROOM AIR NORMOCAPNIC CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-ncpe-status-badge-base',
        '#test-ncpe-status-badge-critical',
        '#test-ncpe-status-badge-staged',
        '#test-ncpe-status-badge-cleared',
        '#test-ncpe-alert-banner',
        '#test-ncpe-clearance-banner',
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
