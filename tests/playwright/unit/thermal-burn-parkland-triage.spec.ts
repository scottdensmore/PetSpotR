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

test.describe('Acute Smoke Inhalation, Thermal Burn Classification (Superficial/Partial/Full-Thickness TBSA%), Parkland Crystalloid Resuscitation Formula, Silver Sulfadiazine & Escharotomy Cockpit UI (/thermal-burn-parkland-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /thermal-burn-parkland-triage renders seeded operations (BURN-OP-101, BURN-OP-102, BURN-OP-103), Parkland Formula / 24h Colloid Guard / Escharotomy alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/thermal-burn-parkland-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#thermal-burn-parkland-cockpit')).toBeVisible();
    await expect(page.locator('#burn-alert-banner')).toBeVisible();
    await expect(page.locator('#burn-alert-banner')).toContainText('Parkland');
    await expect(page.locator('#burn-alert-banner')).toContainText('COLLOID');
    await expect(page.locator('#burn-alert-banner')).toContainText('Escharotomy');
    await expect(page.locator('#burn-alert-banner')).toContainText('Silver Sulfadiazine');

    await expect(page.locator('#burn-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Ember');
    await expect(page.locator('#active-tbsa-percent')).toContainText('30.0% TBSA');
    await expect(page.locator('#active-urine-output')).toContainText('0.8 mL/kg/hr');

    const emberCard = page.locator('#burn-operations-grid .burn-op-card[data-op-id="BURN-OP-101"]');
    const cinderCard = page.locator('#burn-operations-grid .burn-op-card[data-op-id="BURN-OP-102"]');
    const blazeCard = page.locator('#burn-operations-grid .burn-op-card[data-op-id="BURN-OP-103"]');

    await expect(emberCard).toBeVisible();
    await expect(emberCard).toHaveAttribute(
      'data-status',
      'ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE'
    );

    await expect(cinderCard).toBeVisible();
    await expect(cinderCard).toHaveAttribute('data-status', 'PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED');

    await expect(blazeCard).toBeVisible();
    await expect(blazeCard).toHaveAttribute(
      'data-status',
      'GRANULATION_BED_NORMOALBUMINEMIC_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging Parkland 8h/16h, Silver Sulfadiazine & Escharotomy protocol on BURN-OP-101 transitions status to PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/thermal-burn-parkland-triage`);
    await page.waitForLoadState('networkidle');

    const emberCard = page.locator('#burn-operations-grid .burn-op-card[data-op-id="BURN-OP-101"]');
    await emberCard.click();

    await expect(page.locator('#active-op-title')).toContainText('BURN-OP-101');
    await page.locator('#stage-parkland-btn').click();

    await expect(emberCard).toHaveAttribute('data-status', 'PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED');
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED'
    );
  });

  test('3. Recording a serial burn fluid & wound check on BURN-OP-101 updates #active-tbsa-percent, #active-urine-output, and appends to #burn-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/thermal-burn-parkland-triage`);
    await page.waitForLoadState('networkidle');

    const emberCard = page.locator('#burn-operations-grid .burn-op-card[data-op-id="BURN-OP-101"]');
    await emberCard.click();

    await page.locator('#check-tbsa-input').fill('18.0');
    await page.locator('#check-fluid-rate-input').fill('75.0');
    await page.locator('#check-uop-input').fill('1.7');
    await page.locator('#check-albumin-input').fill('2.5');
    await page.locator('#check-pcv-input').fill('43.0');
    await page.locator('#check-recorded-by-input').fill('Dr. Marcus Thorne, DACVS / Burn Reconstruction');
    await page
      .locator('#check-note-input')
      .fill('Post-escharotomy & Parkland 16h phase: chest excursion unrestricted, Urine Output 1.7 mL/kg/hr, Serum Albumin 2.5 g/dL, healthy granulation tissue forming over 18.0% TBSA.');
    await page.locator('#record-burn-check-btn').click();

    await expect(page.locator('#active-tbsa-percent')).toContainText('18.0% TBSA');
    await expect(page.locator('#active-urine-output')).toContainText('1.7 mL/kg/hr');
    await expect(page.locator('#burn-checks-list')).toContainText(
      'Post-escharotomy & Parkland 16h phase: chest excursion unrestricted'
    );
  });

  test('4. Clearing BURN-OP-101 for granulation bed & normoalbuminemic discharge transitions status to GRANULATION_BED_NORMOALBUMINEMIC_CLEARED and reveals #burn-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/thermal-burn-parkland-triage`);
    await page.waitForLoadState('networkidle');

    const emberCard = page.locator('#burn-operations-grid .burn-op-card[data-op-id="BURN-OP-101"]');
    await emberCard.click();

    await page.locator('#clear-discharged-by-input').fill('Dr. Marcus Thorne, DACVS / Burn Reconstruction');
    await page
      .locator('#clear-summary-input')
      .fill('Healthy vascularized granulation bed established across residual 18.0% TBSA, Urine Output 1.7 mL/kg/hr, Serum Albumin restored to 2.5 g/dL, PCV normalized to 43.0%.');
    await page.locator('#clear-burn-btn').click();

    await expect(emberCard).toHaveAttribute(
      'data-status',
      'GRANULATION_BED_NORMOALBUMINEMIC_CLEARED'
    );
    await expect(page.locator('#burn-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Marcus Thorne, DACVS / Burn Reconstruction'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .burn-status-badge, .burn-alert-banner, and .burn-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/thermal-burn-parkland-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-burn-badge-test-fixture';
      container.innerHTML = `
        <span id="test-burn-status-badge-base" class="burn-status-badge">ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE</span>
        <span id="test-burn-status-badge-critical" class="burn-status-badge burn-status-critical">ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE</span>
        <span id="test-burn-status-badge-staged" class="burn-status-badge burn-status-staged">PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED</span>
        <span id="test-burn-status-badge-cleared" class="burn-status-badge burn-status-cleared">GRANULATION_BED_NORMOALBUMINEMIC_CLEARED</span>
        <div id="test-burn-alert-banner" class="burn-alert-banner">CRITICAL PARKLAND FORMULA, 24-HOUR COLLOID LEAK GUARD, ESCHAROTOMY & WOUND BARRIER PROTOCOL</div>
        <div id="test-burn-clearance-banner" class="burn-clearance-banner">HEALTHY GRANULATION BED & NORMOALBUMINEMIC CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-burn-status-badge-base',
        '#test-burn-status-badge-critical',
        '#test-burn-status-badge-staged',
        '#test-burn-status-badge-cleared',
        '#test-burn-alert-banner',
        '#test-burn-clearance-banner',
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
