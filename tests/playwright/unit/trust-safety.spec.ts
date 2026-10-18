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

test.describe('Trust & Safety Moderator Quarantine Cockpit UI (/trust-safety)', () => {
  test('1. /trust-safety renders seeded trust screenings (screening-ts-101, screening-ts-102, screening-ts-103), anomaly flags, and owner quarantine shield banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/trust-safety`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#trust-safety-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Multi-Modal False-Positive & Scam/Hoax Sighting Shield (Trust & Safety Cockpit)'
    );

    const card101 = page.locator('#trust-screenings-list .trust-screening-card[data-screening-id="screening-ts-101"]');
    const card102 = page.locator('#trust-screenings-list .trust-screening-card[data-screening-id="screening-ts-102"]');
    const card103 = page.locator('#trust-screenings-list .trust-screening-card[data-screening-id="screening-ts-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-verdict', 'QUARANTINED_SCAM_RISK');
    await expect(card101.locator('.trust-quarantine-banner')).toBeVisible();
    await expect(card101.locator('.trust-quarantine-banner')).toContainText(
      'OWNER SHIELD ACTIVE: Extortion/Hoax blocked — distressed owner notification suppressed'
    );
    await expect(card101.locator('.badge-anomaly-flag[data-flag="EXTORTION_PAYMENT_DEMAND"]')).toBeVisible();

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-verdict', 'HOLD_FOR_MODERATION');
    await expect(card102.locator('.badge-anomaly-flag[data-flag="TELEPORTATION_VELOCITY_ANOMALY"]')).toBeVisible();

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-verdict', 'VERIFIED_AUTHENTIC');
    await expect(card103.locator('.trust-promoted-banner')).toContainText('sighting-trust-seed-103');

    await expect(page.locator('#form-submit-trust-screening')).toBeVisible();
    await expect(page.locator('#btn-sim-extortion-scam')).toBeVisible();
    await expect(page.locator('#btn-sim-teleport-hoax')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Clicking #btn-sim-extortion-scam creates a QUARANTINED_SCAM_RISK card with EXTORTION_PAYMENT_DEMAND, PIN_PHISHING_ATTEMPT, and SYNTHETIC_IMAGE_ARTIFACT flags and displays .trust-quarantine-banner', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/trust-safety`);
    await page.waitForLoadState('networkidle');

    const uniqueSuffix = Date.now();
    const petName = `DaisyScam #${uniqueSuffix}`;
    const petId = `pet-trust-daisy-${uniqueSuffix}`;

    await page.locator('#ts-pet-id').fill(petId);
    await page.locator('#ts-pet-name').fill(petName);

    await page.locator('#btn-sim-extortion-scam').click();

    const scamCard = page.locator('#trust-screenings-list .trust-screening-card', { hasText: petName });
    await expect(scamCard).toBeVisible();
    await expect(scamCard).toHaveAttribute('data-verdict', 'QUARANTINED_SCAM_RISK');
    await expect(scamCard.locator('.badge-anomaly-flag[data-flag="EXTORTION_PAYMENT_DEMAND"]')).toBeVisible();
    await expect(scamCard.locator('.badge-anomaly-flag[data-flag="PIN_PHISHING_ATTEMPT"]')).toBeVisible();
    await expect(scamCard.locator('.badge-anomaly-flag[data-flag="SYNTHETIC_IMAGE_ARTIFACT"]')).toBeVisible();
    await expect(scamCard.locator('.trust-quarantine-banner')).toBeVisible();
    await expect(scamCard.locator('.trust-quarantine-banner')).toContainText(
      'OWNER SHIELD ACTIVE: Extortion/Hoax blocked — distressed owner notification suppressed'
    );
  });

  test('3. Clicking #btn-sim-teleport-hoax creates a HOLD_FOR_MODERATION card with TELEPORTATION_VELOCITY_ANOMALY, and clicking .btn-approve-screening transitions it to VERIFIED_AUTHENTIC and displays .trust-promoted-banner (sighting-trust-...)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/trust-safety`);
    await page.waitForLoadState('networkidle');

    const uniqueSuffix = Date.now();
    const petName = `DaisyTeleport #${uniqueSuffix}`;
    const petId = `pet-trust-teleport-${uniqueSuffix}`;

    await page.locator('#ts-pet-id').fill(petId);
    await page.locator('#ts-pet-name').fill(petName);

    await page.locator('#btn-sim-teleport-hoax').click();

    const holdCard = page.locator('#trust-screenings-list .trust-screening-card', { hasText: petName });
    await expect(holdCard).toBeVisible();
    await expect(holdCard).toHaveAttribute('data-verdict', 'HOLD_FOR_MODERATION');
    await expect(holdCard.locator('.badge-anomaly-flag[data-flag="TELEPORTATION_VELOCITY_ANOMALY"]')).toBeVisible();

    await holdCard.locator('.btn-approve-screening').click();

    await expect(holdCard).toHaveAttribute('data-verdict', 'VERIFIED_AUTHENTIC');
    await expect(holdCard.locator('.trust-promoted-banner')).toBeVisible();
    await expect(holdCard.locator('.trust-promoted-banner')).toContainText('sighting-trust-');
  });

  test('4. Clicking .btn-reject-screening on a quarantined scam card transitions it to REJECTED_CONFIRMED_HOAX and zeroes the reporter trust score (Trust: 0/100)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/trust-safety`);
    await page.waitForLoadState('networkidle');

    const uniqueSuffix = Date.now();
    const petName = `DaisyReject #${uniqueSuffix}`;
    const petId = `pet-trust-reject-${uniqueSuffix}`;

    await page.locator('#ts-pet-id').fill(petId);
    await page.locator('#ts-pet-name').fill(petName);

    await page.locator('#btn-sim-extortion-scam').click();

    const scamCard = page.locator('#trust-screenings-list .trust-screening-card', { hasText: petName });
    await expect(scamCard).toBeVisible();
    await expect(scamCard).toHaveAttribute('data-verdict', 'QUARANTINED_SCAM_RISK');

    await scamCard.locator('.btn-reject-screening').click();

    await expect(scamCard).toHaveAttribute('data-verdict', 'REJECTED_CONFIRMED_HOAX');
    await expect(scamCard.locator('.badge-reporter-trust')).toContainText('Trust: 0/100');
    await expect(scamCard.locator('.trust-quarantine-banner')).toBeVisible();
  });

  test('5. WCAG AAA (> 7:1) contrast ratio check on .badge-verdict-authentic, .badge-verdict-hold, .badge-verdict-quarantined, .badge-verdict-rejected, and .badge-anomaly-flag in both dark and light modes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/trust-safety`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-trust-safety-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-verdict-authentic" class="badge badge-trust-verdict badge-verdict-authentic">VERIFIED_AUTHENTIC</span>
        <span id="test-badge-verdict-hold" class="badge badge-trust-verdict badge-verdict-hold">HOLD_FOR_MODERATION</span>
        <span id="test-badge-verdict-quarantined" class="badge badge-trust-verdict badge-verdict-quarantined">QUARANTINED_SCAM_RISK</span>
        <span id="test-badge-verdict-rejected" class="badge badge-trust-verdict badge-verdict-rejected">REJECTED_CONFIRMED_HOAX</span>
        <span id="test-badge-anomaly-flag" class="badge badge-anomaly-flag">EXTORTION_PAYMENT_DEMAND</span>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-verdict-authentic',
        '#test-badge-verdict-hold',
        '#test-badge-verdict-quarantined',
        '#test-badge-verdict-rejected',
        '#test-badge-anomaly-flag',
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
