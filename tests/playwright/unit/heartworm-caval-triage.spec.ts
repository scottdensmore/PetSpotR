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

test.describe('Acute Heartworm Caval Syndrome (Dirofilaria immitis), Vena Cava Worm Burden Echocardiography, Intravascular Hemolysis (Hemoglobinuria), Jugular Venotomy Extraction & Melarsomine Split-Dose / Doxycycline / Macrocyclic Lactone Thromboembolism Prevention Cockpit UI (/heartworm-caval-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /heartworm-caval-triage renders seeded operations (HWC-OP-101, HWC-OP-102, HWC-OP-103), Melarsomine Contraindication / Ishihara Jugular Extraction / Wolbachia Doxycycline / Sildenafil alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/heartworm-caval-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#heartworm-caval-cockpit')).toBeVisible();
    await expect(page.locator('#hwc-alert-banner')).toBeVisible();
    await expect(page.locator('#hwc-alert-banner')).toContainText('IMMEDIATE MELARSOMINE');
    await expect(page.locator('#hwc-alert-banner')).toContainText('Right External Jugular Venotomy Extraction');
    await expect(page.locator('#hwc-alert-banner')).toContainText('Wolbachia pipientis');
    await expect(page.locator('#hwc-alert-banner')).toContainText('Sildenafil');
    await expect(page.locator('#hwc-alert-banner')).toContainText('Strict Cage Rest');

    await expect(page.locator('#hwc-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Boudreaux');
    await expect(page.locator('#active-tr-jet')).toContainText('3.90 m/s');
    await expect(page.locator('#active-pcv')).toContainText('21.0%');

    const boudreauxCard = page.locator('#hwc-operations-grid .hwc-op-card[data-op-id="HWC-OP-101"]');
    const magnoliaCard = page.locator('#hwc-operations-grid .hwc-op-card[data-op-id="HWC-OP-102"]');
    const cypressCard = page.locator('#hwc-operations-grid .hwc-op-card[data-op-id="HWC-OP-103"]');

    await expect(boudreauxCard).toBeVisible();
    await expect(boudreauxCard).toHaveAttribute(
      'data-status',
      'JUGULAR_VENOTOMY_WOLBACHIA_DOXY_STAGED'
    );

    await expect(magnoliaCard).toBeVisible();
    await expect(magnoliaCard).toHaveAttribute(
      'data-status',
      'PULMONARY_HYPERTENSION_PTE_MONITORED'
    );

    await expect(cypressCard).toBeVisible();
    await expect(cypressCard).toHaveAttribute(
      'data-status',
      'CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging right jugular venotomy extraction & Wolbachia Doxycycline intervention on HWC-OP-101 transitions status to PULMONARY_HYPERTENSION_PTE_MONITORED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/heartworm-caval-triage`);
    await page.waitForLoadState('networkidle');

    const boudreauxCard = page.locator('#hwc-operations-grid .hwc-op-card[data-op-id="HWC-OP-101"]');
    await boudreauxCard.click();

    await expect(page.locator('#active-op-title')).toContainText('HWC-OP-101');
    await page.locator('#stage-intervention-btn').click();

    await expect(boudreauxCard).toHaveAttribute(
      'data-status',
      'PULMONARY_HYPERTENSION_PTE_MONITORED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'PULMONARY_HYPERTENSION_PTE_MONITORED'
    );
  });

  test('3. Recording an echocardiography & hemolysis check on HWC-OP-101 updates #active-tr-jet, #active-worms-extracted, #active-pcv, and appends to #hemodynamic-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/heartworm-caval-triage`);
    await page.waitForLoadState('networkidle');

    const boudreauxCard = page.locator('#hwc-operations-grid .hwc-op-card[data-op-id="HWC-OP-101"]');
    await boudreauxCard.click();

    await page.locator('#check-tr-jet-input').fill('2.35');
    await page.locator('#check-worms-extracted-input').fill('31');
    await page.locator('#check-pcv-input').fill('36.0');
    await page.locator('#check-sildenafil-dose-input').fill('1.5');
    await page.locator('#check-hemoglobinuria-checkbox').uncheck();
    await page.locator('#check-caval-lines-checkbox').uncheck();
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Thibodaux Broussard, DACVIM (Cardiology)');
    await page
      .locator('#check-note-input')
      .fill('Post-jugular venotomy check: 31 adult Dirofilaria immitis extracted; parallel hyperechoic caval lines abolished, urine clear straw-yellow (hemoglobinuria resolved), PCV 36.0%, TR jet improved to 2.35 m/s.');
    await page.locator('#record-hemodynamic-check-btn').click();

    await expect(page.locator('#active-tr-jet')).toContainText('2.35 m/s');
    await expect(page.locator('#active-worms-extracted')).toContainText('31 worms');
    await expect(page.locator('#active-pcv')).toContainText('36.0%');
    await expect(page.locator('#hemodynamic-checks-list')).toContainText(
      'Post-jugular venotomy check: 31 adult Dirofilaria immitis extracted'
    );
  });

  test('4. Clearing HWC-OP-101 for patent caval discharge transitions status to CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED and reveals #hwc-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/heartworm-caval-triage`);
    await page.waitForLoadState('networkidle');

    const boudreauxCard = page.locator('#hwc-operations-grid .hwc-op-card[data-op-id="HWC-OP-101"]');
    await boudreauxCard.click();

    await page.locator('#clear-tr-jet-input').fill('2.35');
    await page.locator('#clear-cage-rest-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Thibodaux Broussard, DACVIM (Cardiology)');
    await page
      .locator('#clear-summary-input')
      .fill('Caval and right atrial lumen 100% patent (0 parallel hyperechoic lines), intravascular hemolysis and hemoglobinuria resolved, PCV 36.0%, TR jet velocity 2.35 m/s (<= 2.8 m/s), 28-day Wolbachia Doxycycline + 3-dose split Melarsomine and 8-week strict cage rest verified; cleared.');
    await page.locator('#clear-hwc-btn').click();

    await expect(boudreauxCard).toHaveAttribute(
      'data-status',
      'CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED'
    );
    await expect(page.locator('#hwc-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Thibodaux Broussard, DACVIM (Cardiology)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .hwc-status-badge, .hwc-alert-banner, and .hwc-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/heartworm-caval-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-hwc-badge-test-fixture';
      container.innerHTML = `
        <span id="test-hwc-status-badge-base" class="hwc-status-badge">JUGULAR_VENOTOMY_WOLBACHIA_DOXY_STAGED</span>
        <span id="test-hwc-status-badge-critical" class="hwc-status-badge hwc-status-critical">JUGULAR_VENOTOMY_WOLBACHIA_DOXY_STAGED</span>
        <span id="test-hwc-status-badge-staged" class="hwc-status-badge hwc-status-staged">PULMONARY_HYPERTENSION_PTE_MONITORED</span>
        <span id="test-hwc-status-badge-cleared" class="hwc-status-badge hwc-status-cleared">CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED</span>
        <div id="test-hwc-alert-banner" class="hwc-alert-banner">CRITICAL HEARTWORM CAVAL SYNDROME & ADULTICIDE GUARD</div>
        <div id="test-hwc-clearance-banner" class="hwc-clearance-banner">CAVAL LUMEN PATENT, HEMOGLOBINURIA RESOLVED & STRICT CAGE-REST CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-hwc-status-badge-base',
        '#test-hwc-status-badge-critical',
        '#test-hwc-status-badge-staged',
        '#test-hwc-status-badge-cleared',
        '#test-hwc-alert-banner',
        '#test-hwc-clearance-banner',
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
