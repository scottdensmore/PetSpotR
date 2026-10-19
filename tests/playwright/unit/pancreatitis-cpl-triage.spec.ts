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

test.describe('Acute Pancreatitis, Canine/Feline Spec cPL/fPL, POCUS Hypoechoic Pancreas/Hyperechoic Mesentery, Maropitant/Fuzapladib & Early Enteral Nutrition Cockpit UI (/pancreatitis-cpl-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /pancreatitis-cpl-triage renders seeded operations (PANC-OP-101, PANC-OP-102, PANC-OP-103), KPI telemetry, outdated NPO myth contraindication alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/pancreatitis-cpl-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#pancreatitis-cpl-cockpit')).toBeVisible();
    await expect(page.locator('#pancreatitis-alert-banner')).toBeVisible();
    await expect(page.locator('#pancreatitis-alert-banner')).toContainText('NPO');
    await expect(page.locator('#pancreatitis-alert-banner')).toContainText('Fuzapladib');
    await expect(page.locator('#pancreatitis-alert-banner')).toContainText('Cobalamin');

    const schnitzelCard = page.locator(
      '#pancreatitis-operations-grid .pancreatitis-op-card[data-op-id="PANC-OP-101"]'
    );
    const misoCard = page.locator(
      '#pancreatitis-operations-grid .pancreatitis-op-card[data-op-id="PANC-OP-102"]'
    );
    const biscuitCard = page.locator(
      '#pancreatitis-operations-grid .pancreatitis-op-card[data-op-id="PANC-OP-103"]'
    );

    await expect(schnitzelCard).toBeVisible();
    await expect(schnitzelCard).toContainText('Schnitzel');
    await expect(schnitzelCard).toHaveAttribute(
      'data-status',
      'ACUTE_NECROTIZING_PANCREATITIS_TRIAGE'
    );

    await expect(misoCard).toBeVisible();
    await expect(misoCard).toContainText('Miso');
    await expect(misoCard).toHaveAttribute('data-status', 'FUZAPLADIB_MAROPITANT_ENTERAL_STAGED');

    await expect(biscuitCard).toBeVisible();
    await expect(biscuitCard).toContainText('Biscuit');
    await expect(biscuitCard).toHaveAttribute(
      'data-status',
      'TOLERATING_ENTERAL_PAIN_FREE_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting PANC-OP-101 Schnitzel populates #pancreatitis-active-workspace and stages Fuzapladib, Maropitant & Early Enteral Nutrition to FUZAPLADIB_MAROPITANT_ENTERAL_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pancreatitis-cpl-triage`);
    await page.waitForLoadState('networkidle');

    const schnitzelCard = page.locator(
      '#pancreatitis-operations-grid .pancreatitis-op-card[data-op-id="PANC-OP-101"]'
    );
    await schnitzelCard.click();

    await expect(page.locator('#active-op-title')).toContainText('Schnitzel');
    await expect(page.locator('#active-spec-pl')).toContainText('940');

    await page.locator('#stage-nutrition-btn').click();

    await expect(page.locator('#active-op-status-badge')).toContainText(
      'FUZAPLADIB_MAROPITANT_ENTERAL_STAGED'
    );
    await expect(schnitzelCard).toHaveAttribute(
      'data-status',
      'FUZAPLADIB_MAROPITANT_ENTERAL_STAGED'
    );
  });

  test('3. Recording an enteral nutrition & pain check on PANC-OP-101 updates #active-spec-pl, #active-enteral-rer, and appends to #enteral-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pancreatitis-cpl-triage`);
    await page.waitForLoadState('networkidle');

    const schnitzelCard = page.locator(
      '#pancreatitis-operations-grid .pancreatitis-op-card[data-op-id="PANC-OP-101"]'
    );
    await schnitzelCard.click();

    await page.locator('#check-spec-pl-input').fill('310');
    await page.locator('#check-pain-score-input').fill('4');
    await page.locator('#check-rer-percent-input').fill('75');
    await page.locator('#check-vomiting-input').fill('ZERO_EMESIS_TOLERATING_NE_TUBE');
    await page.locator('#check-recorded-by-input').fill('Dr. Elena Rostova, DACVECC');
    await page
      .locator('#check-note-input')
      .fill('Spec cPL decreased to 310 ug/L; tolerating 75% RER via 8 Fr NE tube without emesis.');
    await page.locator('#record-enteral-check-btn').click();

    await expect(page.locator('#active-spec-pl')).toContainText('310');
    await expect(page.locator('#active-enteral-rer')).toContainText('75% RER');
    await expect(page.locator('#enteral-checks-list')).toContainText(
      'Spec cPL decreased to 310 ug/L; tolerating 75% RER via 8 Fr NE tube without emesis.'
    );
  });

  test('4. Clearing PANC-OP-101 for discharge transitions status to TOLERATING_ENTERAL_PAIN_FREE_CLEARED and reveals #pancreatitis-clearance-banner with #clearance-summary-text', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pancreatitis-cpl-triage`);
    await page.waitForLoadState('networkidle');

    const schnitzelCard = page.locator(
      '#pancreatitis-operations-grid .pancreatitis-op-card[data-op-id="PANC-OP-101"]'
    );
    await schnitzelCard.click();

    await page.locator('#clear-discharged-by-input').fill('Dr. Elena Rostova, DACVECC');
    await page
      .locator('#clear-summary-input')
      .fill(
        'Completed 3-day Fuzapladib sodium & Maropitant/Ondansetron course; tolerating 100% RER ultra-low-fat diet pain-free.'
      );
    await page.locator('#clear-pancreatitis-btn').click();

    await expect(page.locator('#active-op-status-badge')).toContainText(
      'TOLERATING_ENTERAL_PAIN_FREE_CLEARED'
    );
    await expect(page.locator('#pancreatitis-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Completed 3-day Fuzapladib sodium & Maropitant/Ondansetron course'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .pancreatitis-status-badge, .pancreatitis-alert-banner, and .pancreatitis-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pancreatitis-cpl-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-pancreatitis-test-fixture';
      container.innerHTML = `
        <span id="test-pancreatitis-badge" class="pancreatitis-status-badge">ACUTE_NECROTIZING_PANCREATITIS_TRIAGE</span>
        <div id="test-pancreatitis-alert" class="pancreatitis-alert-banner">CONTRAINDICATED MYTH WARNING: Prolonged NPO gut starvation is strictly contraindicated</div>
        <div id="test-pancreatitis-clearance" class="pancreatitis-clearance-banner">TOLERATING ENTERAL NUTRITION & PAIN-FREE DISCHARGE CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-pancreatitis-badge',
        '#test-pancreatitis-alert',
        '#test-pancreatitis-clearance',
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
