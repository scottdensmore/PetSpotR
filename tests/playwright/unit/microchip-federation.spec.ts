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

test.describe('Universal Microchip Registry Federation & Ownership Resolution Cockpit UI (/microchip-federation)', () => {
  test('1. /microchip-federation renders seeded scan cases (chip-case-101, chip-case-102, chip-case-103), registry waterfall hops, manufacturer lot badges, and lot-trace banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/microchip-federation`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#microchip-federation-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Universal Microchip Registry Federation & Ownership Resolution Cockpit'
    );

    await expect(page.locator('#kpi-chips-total')).toBeVisible();
    await expect(page.locator('#kpi-chips-unregistered')).toBeVisible();
    await expect(page.locator('#kpi-chips-escalated')).toBeVisible();
    await expect(page.locator('#kpi-chips-reunited')).toBeVisible();
    await expect(page.locator('#microchip-federation-live-status')).toBeVisible();

    const card101 = page.locator('#microchip-cases-list .microchip-case-card[data-case-id="chip-case-101"]');
    const card102 = page.locator('#microchip-cases-list .microchip-case-card[data-case-id="chip-case-102"]');
    const card103 = page.locator('#microchip-cases-list .microchip-case-card[data-case-id="chip-case-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-case-status', 'UNREGISTERED_BATCH_TRACED');
    await expect(card101.locator('.badge-chip-status.badge-chip-unregistered')).toContainText('UNREGISTERED_BATCH_TRACED');
    await expect(card101.locator('.badge-chip-standard')).toContainText('ISO_11784_15 • 134.2 kHz');
    await expect(card101.locator('.badge-chip-lot')).toContainText('LOT-ISO900-1180');
    await expect(card101.locator('.registry-hops-list')).toContainText('NO RECORD');
    await expect(card101.locator('.chip-lot-trace-banner')).toContainText('UNREGISTERED CHIP — MANUFACTURER LOT TRACE');
    await expect(card101.locator('.chip-lot-trace-banner')).toContainText('Emerald City Spay/Neuter & Wellness Clinic');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-case-status', 'SECONDARY_CONTACT_ESCALATED');
    await expect(card102.locator('.badge-chip-status.badge-chip-escalated')).toContainText('SECONDARY_CONTACT_ESCALATED');
    await expect(card102.locator('.badge-chip-standard')).toContainText('AVID_9_ENCRYPTED • 125.0 kHz');
    await expect(card102.locator('.badge-chip-lot')).toContainText('LOT-AVID-0412');
    await expect(card102.locator('.registry-hops-list')).toContainText('AVID PETtrac');
    await expect(card102.locator('.registry-hops-list')).toContainText('MATCH');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-case-status', 'REUNION_DISPATCHED_VERIFIED');
    await expect(card103.locator('.badge-chip-status.badge-chip-verified')).toContainText('REUNION_DISPATCHED_VERIFIED');
    await expect(card103.locator('.badge-chip-standard')).toContainText('ISO_11784_15 • 134.2 kHz');
    await expect(card103.locator('.badge-chip-lot')).toContainText('LOT-MERCK-1410');
    await expect(card103.locator('.chip-promoted-sighting-banner')).toContainText('Promoted Sighting: sighting-chip-seed-103');

    await expect(page.locator('#form-scan-microchip')).toBeVisible();
    await expect(page.locator('#btn-sim-unregistered-iso')).toBeVisible();
    await expect(page.locator('#btn-sim-avid9-escalated')).toBeVisible();
    await expect(page.locator('#btn-sim-hex10-legacy')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Simulating a 10-digit legacy hex FDX-A scan (#btn-sim-hex10-legacy) creates a REGISTERED_OWNER_FOUND card with HEX_10_LEGACY • 128.0 kHz', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/microchip-federation`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-hex10-legacy').click();

    const hexCard = page.locator('#microchip-cases-list .microchip-case-card', {
      hasText: 'LOT-FDXA-7F7E',
    }).first();
    await expect(hexCard).toBeVisible();
    await expect(hexCard).toHaveAttribute('data-case-status', 'REGISTERED_OWNER_FOUND');
    await expect(hexCard.locator('.badge-chip-status.badge-chip-registered')).toContainText('REGISTERED_OWNER_FOUND');
    await expect(hexCard.locator('.badge-chip-standard')).toContainText('HEX_10_LEGACY • 128.0 kHz');
  });

  test('3. Simulating an unregistered ISO 15-digit scan (#btn-sim-unregistered-iso) creates an UNREGISTERED_BATCH_TRACED card, and clicking .btn-trace-batch-lot recovers owner contact (Devon & Priya Patel) from the implanting clinic PIMS chart', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/microchip-federation`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-unregistered-iso').click();

    const unregCard = page.locator('#microchip-cases-list .microchip-case-card', {
      hasText: '8291',
    }).first();
    await expect(unregCard).toBeVisible();
    await expect(unregCard).toHaveAttribute('data-case-status', 'UNREGISTERED_BATCH_TRACED');
    await expect(unregCard.locator('.badge-chip-lot')).toContainText('LOT-ISO900-1180');
    await expect(unregCard.locator('.chip-lot-trace-banner')).toContainText('UNREGISTERED CHIP — MANUFACTURER LOT TRACE');

    await unregCard.locator('.btn-trace-batch-lot').click();

    await expect(unregCard).toContainText('Devon & Priya Patel');
    await expect(unregCard).toContainText('+1 (206) •••-7724');
  });

  test('4. Clicking .btn-resolve-chip-reunion transitions a case to REUNION_DISPATCHED_VERIFIED and displays .chip-promoted-sighting-banner (sighting-chip-...)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/microchip-federation`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-avid9-escalated').click();

    const avidCard = page.locator('#microchip-cases-list .microchip-case-card', {
      hasText: 'LOT-AVID-0418',
    }).first();
    await expect(avidCard).toBeVisible();
    await expect(avidCard).toHaveAttribute('data-case-status', 'SECONDARY_CONTACT_ESCALATED');
    await expect(avidCard.locator('.badge-chip-standard')).toContainText('AVID_9_ENCRYPTED • 125.0 kHz');

    await avidCard.locator('.btn-resolve-chip-reunion').click();

    await expect(avidCard).toHaveAttribute('data-case-status', 'REUNION_DISPATCHED_VERIFIED');
    await expect(avidCard.locator('.badge-chip-status.badge-chip-verified')).toContainText('REUNION_DISPATCHED_VERIFIED');
    await expect(avidCard.locator('.chip-promoted-sighting-banner')).toBeVisible();
    await expect(avidCard.locator('.chip-promoted-sighting-banner')).toContainText('Promoted Sighting: sighting-chip-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7:1) on .badge-chip-registered, .badge-chip-unregistered, .badge-chip-escalated, .badge-chip-verified, .badge-chip-standard, .badge-chip-lot, and .chip-lot-trace-banner in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/microchip-federation`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-microchip-federation-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-chip-registered" class="badge badge-chip-status badge-chip-registered">REGISTERED_OWNER_FOUND</span>
        <span id="test-badge-chip-unregistered" class="badge badge-chip-status badge-chip-unregistered">UNREGISTERED_BATCH_TRACED</span>
        <span id="test-badge-chip-escalated" class="badge badge-chip-status badge-chip-escalated">SECONDARY_CONTACT_ESCALATED</span>
        <span id="test-badge-chip-verified" class="badge badge-chip-status badge-chip-verified">REUNION_DISPATCHED_VERIFIED</span>
        <span id="test-badge-chip-standard" class="badge badge-chip-standard">ISO_11784_15 • 134.2 kHz</span>
        <span id="test-badge-chip-lot" class="badge badge-chip-lot">LOT-MERCK-1410</span>
        <div id="test-chip-lot-trace-banner" class="chip-lot-trace-banner">UNREGISTERED CHIP — MANUFACTURER LOT TRACE</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-chip-registered',
        '#test-badge-chip-unregistered',
        '#test-badge-chip-escalated',
        '#test-badge-chip-verified',
        '#test-badge-chip-standard',
        '#test-badge-chip-lot',
        '#test-chip-lot-trace-banner',
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
