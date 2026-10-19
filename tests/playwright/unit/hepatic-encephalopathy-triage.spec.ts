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

test.describe('Acute Hepatic Failure, Hepatic Encephalopathy, Sago Palm / Microcystin / Amanita Hepatotoxicity, N-Acetylcysteine (NAC) & Lactulose/Levetiracetam Cockpit UI (/hepatic-encephalopathy-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /hepatic-encephalopathy-triage renders seeded operations (HEP-OP-101, HEP-OP-102, HEP-OP-103), NAC 140 mg/kg & Lactulose/Levetiracetam alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/hepatic-encephalopathy-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#hepatic-encephalopathy-cockpit')).toBeVisible();
    await expect(page.locator('#hepatic-alert-banner')).toBeVisible();
    await expect(page.locator('#hepatic-alert-banner')).toContainText('140 mg/kg');
    await expect(page.locator('#hepatic-alert-banner')).toContainText('0.2-micron');
    await expect(page.locator('#hepatic-alert-banner')).toContainText('Lactulose');
    await expect(page.locator('#hepatic-alert-banner')).toContainText('Diazepam');
    await expect(page.locator('#hepatic-alert-banner')).toContainText('Levetiracetam');
    await expect(page.locator('#hepatic-alert-banner')).toContainText('Vitamin K1');

    await expect(page.locator('#hepatic-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Kona');
    await expect(page.locator('#active-blood-ammonia')).toContainText('245.0 umol/L');
    await expect(page.locator('#active-alt-value')).toContainText('2840 IU/L');

    const konaCard = page.locator(
      '#hepatic-operations-grid .hepatic-op-card[data-op-id="HEP-OP-101"]'
    );
    const marmaladeCard = page.locator(
      '#hepatic-operations-grid .hepatic-op-card[data-op-id="HEP-OP-102"]'
    );
    const scoutCard = page.locator(
      '#hepatic-operations-grid .hepatic-op-card[data-op-id="HEP-OP-103"]'
    );

    await expect(konaCard).toBeVisible();
    await expect(konaCard).toHaveAttribute(
      'data-status',
      'ACUTE_FULMINANT_HEPATOTOXICITY_HE_TRIAGE'
    );

    await expect(marmaladeCard).toBeVisible();
    await expect(marmaladeCard).toHaveAttribute(
      'data-status',
      'NAC_LACTULOSE_VITAMIN_K1_STAGED'
    );

    await expect(scoutCard).toBeVisible();
    await expect(scoutCard).toHaveAttribute(
      'data-status',
      'AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging N-Acetylcysteine (NAC) 140 mg/kg IV load, Lactulose retention enema & Levetiracetam protocol on HEP-OP-101 transitions status to NAC_LACTULOSE_VITAMIN_K1_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hepatic-encephalopathy-triage`);
    await page.waitForLoadState('networkidle');

    const konaCard = page.locator(
      '#hepatic-operations-grid .hepatic-op-card[data-op-id="HEP-OP-101"]'
    );
    await konaCard.click();

    await expect(page.locator('#active-op-title')).toContainText('HEP-OP-101');
    await page.locator('#stage-support-btn').click();

    await expect(konaCard).toHaveAttribute('data-status', 'NAC_LACTULOSE_VITAMIN_K1_STAGED');
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'NAC_LACTULOSE_VITAMIN_K1_STAGED'
    );
  });

  test('3. Recording a serial hepatic metabolic & ammonia check on HEP-OP-101 updates #active-blood-ammonia, #active-alt-value, and appends to #metabolic-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hepatic-encephalopathy-triage`);
    await page.waitForLoadState('networkidle');

    const konaCard = page.locator(
      '#hepatic-operations-grid .hepatic-op-card[data-op-id="HEP-OP-101"]'
    );
    await konaCard.click();

    await page.locator('#check-alt-input').fill('460');
    await page.locator('#check-bilirubin-input').fill('1.5');
    await page.locator('#check-ammonia-input').fill('39.5');
    await page.locator('#check-glucose-input').fill('99.0');
    await page.locator('#check-pt-input').fill('12.4');
    await page.locator('#check-recorded-by-input').fill('Dr. Priya Nair, DACVECC');
    await page
      .locator('#check-note-input')
      .fill(
        'Following NAC 140 mg/kg IV loading & Lactulose retention enemas: blood ammonia normalized to 39.5 umol/L, ALT reduced to 460 IU/L, euglycemic 99.0 mg/dL, PT 12.4 sec.'
      );
    await page.locator('#record-metabolic-check-btn').click();

    await expect(page.locator('#active-blood-ammonia')).toContainText('39.5 umol/L');
    await expect(page.locator('#active-alt-value')).toContainText('460 IU/L');
    await expect(page.locator('#metabolic-checks-list')).toContainText(
      'Following NAC 140 mg/kg IV loading & Lactulose retention enemas: blood ammonia normalized to 39.5 umol/L'
    );
  });

  test('4. Clearing HEP-OP-101 for hepatic discharge transitions status to AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED and reveals #hepatic-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hepatic-encephalopathy-triage`);
    await page.waitForLoadState('networkidle');

    const konaCard = page.locator(
      '#hepatic-operations-grid .hepatic-op-card[data-op-id="HEP-OP-101"]'
    );
    await konaCard.click();

    await page.locator('#clear-discharged-by-input').fill('Dr. Priya Nair, DACVECC');
    await page
      .locator('#clear-summary-input')
      .fill(
        'Sago palm cycasin hepatotoxicity resolved; blood ammonia normalized (39.5 umol/L), HE Grade 0, PT 12.4 sec, discharged on oral SAMe/Silybin + Lactulose.'
      );
    await page.locator('#clear-hepatic-btn').click();

    await expect(konaCard).toHaveAttribute(
      'data-status',
      'AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED'
    );
    await expect(page.locator('#hepatic-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Dr. Priya Nair, DACVECC');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .hepatic-status-badge, .hepatic-alert-banner, and .hepatic-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hepatic-encephalopathy-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-hepatic-badge-test-fixture';
      container.innerHTML = `
        <span id="test-hepatic-status-badge-base" class="hepatic-status-badge">ACUTE_FULMINANT_HEPATOTOXICITY_HE_TRIAGE</span>
        <span id="test-hepatic-status-badge-critical" class="hepatic-status-badge hepatic-status-critical">ACUTE_FULMINANT_HEPATOTOXICITY_HE_TRIAGE</span>
        <span id="test-hepatic-status-badge-staged" class="hepatic-status-badge hepatic-status-staged">NAC_LACTULOSE_VITAMIN_K1_STAGED</span>
        <span id="test-hepatic-status-badge-cleared" class="hepatic-status-badge hepatic-status-cleared">AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED</span>
        <div id="test-hepatic-alert-banner" class="hepatic-alert-banner">CRITICAL FULMINANT HEPATOTOXICITY & HEPATIC ENCEPHALOPATHY GUARD</div>
        <div id="test-hepatic-clearance-banner" class="hepatic-clearance-banner">AMMONIA NORMALIZED, HEPATIC ENCEPHALOPATHY RESOLVED & SYNTHETIC FUNCTION STABLE CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-hepatic-status-badge-base',
        '#test-hepatic-status-badge-critical',
        '#test-hepatic-status-badge-staged',
        '#test-hepatic-status-badge-cleared',
        '#test-hepatic-alert-banner',
        '#test-hepatic-clearance-banner',
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
