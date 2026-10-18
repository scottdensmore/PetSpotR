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

test.describe('Forensic DNA, Coat-Pattern Biometrics & Senior Cognitive Triage Cockpit UI (/biometric-forensics)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /biometric-forensics renders seeded cases (forensic-case-101, forensic-case-102, forensic-case-103), verification steps, and sundowning alert banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/biometric-forensics`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#biometric-forensics-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Forensic DNA, Coat-Pattern Biometrics & Senior Cognitive Triage Cockpit'
    );

    await expect(page.locator('#kpi-forensic-total')).toBeVisible();
    await expect(page.locator('#kpi-forensic-entrapment')).toBeVisible();
    await expect(page.locator('#kpi-forensic-sundowning')).toBeVisible();
    await expect(page.locator('#kpi-forensic-confirmed')).toBeVisible();
    await expect(page.locator('#biometric-forensics-live-status')).toBeVisible();

    const card101 = page.locator('#forensic-cases-list .forensic-case-card[data-case-id="forensic-case-101"]');
    const card102 = page.locator('#forensic-cases-list .forensic-case-card[data-case-id="forensic-case-102"]');
    const card103 = page.locator('#forensic-cases-list .forensic-case-card[data-case-id="forensic-case-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-resolution-status', 'PENDING_BIOMETRIC_SCAN');
    await expect(card101.locator('.badge-forensic-status.badge-forensic-pending')).toContainText('PENDING_BIOMETRIC_SCAN');
    await expect(card101.locator('.badge-entrapment-risk.badge-entrapment-critical')).toContainText('CRITICAL_SUNDOWNING_ENTRAPMENT');
    await expect(card101.locator('.forensic-sundowning-alert-banner')).toContainText('SENIOR COGNITIVE SUNDOWNING & ENTRAPMENT ALERT');
    await expect(card101.locator('.forensic-steps-list')).toContainText('NASAL_PLANUM_BIOMETRICS');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-resolution-status', 'DERMATOGLYPHIC_CORRELATED');
    await expect(card102.locator('.badge-forensic-status.badge-forensic-correlated')).toContainText('DERMATOGLYPHIC_CORRELATED');
    await expect(card102.locator('.badge-entrapment-risk.badge-entrapment-elevated')).toContainText('ELEVATED_SENSORY_DISORIENTATION');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-resolution-status', 'IDENTITY_CONFIRMED_SAFE_EXTRACTION');
    await expect(card103.locator('.badge-forensic-status.badge-forensic-confirmed')).toContainText('IDENTITY_CONFIRMED_SAFE_EXTRACTION');
    await expect(card103.locator('.badge-entrapment-risk.badge-entrapment-safe')).toContainText('CONTAINED_SAFE_PERIPHERY');
    await expect(card103.locator('.forensic-certificate-banner')).toContainText('BIOMETRIC FORENSIC IDENTITY CONFIRMED: CERT-FORENSIC-FORENSIC-CASE-103');

    await expect(page.locator('#form-create-forensic-case')).toBeVisible();
    await expect(page.locator('#btn-sim-sundowning-senior')).toBeVisible();
    await expect(page.locator('#btn-sim-blind-cat-bramble')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Simulating a 15yo CCD sundowning senior dog (#btn-sim-sundowning-senior) creates a PENDING_BIOMETRIC_SCAN card with .forensic-sundowning-alert-banner and CRITICAL_SUNDOWNING_ENTRAPMENT', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/biometric-forensics`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-sundowning-senior').click();

    const simCard = page.locator('#forensic-cases-list .forensic-case-card', {
      hasText: 'Winston',
    }).first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-resolution-status', 'PENDING_BIOMETRIC_SCAN');
    await expect(simCard.locator('.badge-forensic-status.badge-forensic-pending')).toContainText('PENDING_BIOMETRIC_SCAN');
    await expect(simCard.locator('.badge-entrapment-risk.badge-entrapment-critical')).toContainText('CRITICAL_SUNDOWNING_ENTRAPMENT');
    await expect(simCard.locator('.forensic-sundowning-alert-banner')).toContainText('SENIOR COGNITIVE SUNDOWNING & ENTRAPMENT ALERT');
  });

  test('3. Clicking .btn-correlate-biometric correlates nasal planum biometrics and sweeps 150m culvert (Biometric Confidence: 100/100) and transitions case to DERMATOGLYPHIC_CORRELATED', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/biometric-forensics`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-blind-cat-bramble').click();

    const blindCatCard = page.locator('#forensic-cases-list .forensic-case-card', {
      hasText: 'Pearl',
    }).first();
    await expect(blindCatCard).toBeVisible();
    await expect(blindCatCard).toHaveAttribute('data-resolution-status', 'PENDING_BIOMETRIC_SCAN');

    await blindCatCard.locator('.btn-correlate-biometric').click();

    await expect(blindCatCard).toHaveAttribute('data-resolution-status', 'DERMATOGLYPHIC_CORRELATED');
    await expect(blindCatCard.locator('.badge-forensic-status.badge-forensic-correlated')).toContainText('DERMATOGLYPHIC_CORRELATED');
    await expect(blindCatCard.locator('.badge-forensic-metric')).toContainText('Biometric Confidence: 100/100');
    await expect(blindCatCard.locator('.badge-entrapment-risk.badge-entrapment-elevated')).toContainText('ELEVATED_SENSORY_DISORIENTATION');
  });

  test('4. Clicking .btn-confirm-forensic-extraction transitions a case to IDENTITY_CONFIRMED_SAFE_EXTRACTION (CONTAINED_SAFE_PERIPHERY) and displays .forensic-certificate-banner (CERT-FORENSIC-...)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/biometric-forensics`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-blind-cat-bramble').click();

    const card = page.locator('#forensic-cases-list .forensic-case-card', {
      hasText: 'Pearl',
    }).first();
    await expect(card).toBeVisible();

    await card.locator('.btn-confirm-forensic-extraction').click();

    await expect(card).toHaveAttribute('data-resolution-status', 'IDENTITY_CONFIRMED_SAFE_EXTRACTION');
    await expect(card.locator('.badge-forensic-status.badge-forensic-confirmed')).toContainText('IDENTITY_CONFIRMED_SAFE_EXTRACTION');
    await expect(card.locator('.badge-entrapment-risk.badge-entrapment-safe')).toContainText('CONTAINED_SAFE_PERIPHERY');
    await expect(card.locator('.forensic-certificate-banner')).toBeVisible();
    await expect(card.locator('.forensic-certificate-banner')).toContainText('BIOMETRIC FORENSIC IDENTITY CONFIRMED: CERT-FORENSIC-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7:1) on .badge-forensic-pending, .badge-forensic-correlated, .badge-forensic-confirmed, .badge-entrapment-critical, .badge-entrapment-elevated, .badge-entrapment-safe, .badge-forensic-metric, .forensic-sundowning-alert-banner, and .forensic-certificate-banner in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/biometric-forensics`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-biometric-forensics-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-forensic-pending" class="badge badge-forensic-status badge-forensic-pending">PENDING_BIOMETRIC_SCAN</span>
        <span id="test-badge-forensic-correlated" class="badge badge-forensic-status badge-forensic-correlated">DERMATOGLYPHIC_CORRELATED</span>
        <span id="test-badge-forensic-confirmed" class="badge badge-forensic-status badge-forensic-confirmed">IDENTITY_CONFIRMED_SAFE_EXTRACTION</span>
        <span id="test-badge-entrapment-critical" class="badge badge-entrapment-risk badge-entrapment-critical">CRITICAL_SUNDOWNING_ENTRAPMENT</span>
        <span id="test-badge-entrapment-elevated" class="badge badge-entrapment-risk badge-entrapment-elevated">ELEVATED_SENSORY_DISORIENTATION</span>
        <span id="test-badge-entrapment-safe" class="badge badge-entrapment-risk badge-entrapment-safe">CONTAINED_SAFE_PERIPHERY</span>
        <span id="test-badge-forensic-metric" class="badge badge-forensic-metric">Entrapment Risk: 95/100 • Biometric Confidence: 100/100</span>
        <div id="test-forensic-sundowning-alert-banner" class="forensic-sundowning-alert-banner">SENIOR COGNITIVE SUNDOWNING &amp; ENTRAPMENT ALERT</div>
        <div id="test-forensic-certificate-banner" class="forensic-certificate-banner">BIOMETRIC FORENSIC IDENTITY CONFIRMED: CERT-FORENSIC-101</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-forensic-pending',
        '#test-badge-forensic-correlated',
        '#test-badge-forensic-confirmed',
        '#test-badge-entrapment-critical',
        '#test-badge-entrapment-elevated',
        '#test-badge-entrapment-safe',
        '#test-badge-forensic-metric',
        '#test-forensic-sundowning-alert-banner',
        '#test-forensic-certificate-banner',
      ]) {
        const styles = await page.locator(selector).evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { color: cs.color, backgroundColor: cs.backgroundColor };
        });
        expect(styles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
        const ratio = contrastRatio(styles.color, styles.backgroundColor);
        expect(ratio, `${selector} in ${theme} mode should have > 7:1 contrast`).toBeGreaterThan(7.0);
      }
    }
  });
});
