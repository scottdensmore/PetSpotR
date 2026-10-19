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

test.describe('Fireworks/Thunderstorm Acoustic Panic, Bolt-Trajectory Vectoring & Calming Pheromone SAR Cockpit UI (/acoustic-panic-trajectory)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /acoustic-panic-trajectory renders seeded operations (AP-OP-101, AP-OP-102, AP-OP-103), KPI telemetry, adrenal no-chase alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/acoustic-panic-trajectory`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#acoustic-panic-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Fireworks/Thunderstorm Acoustic Panic, Bolt-Trajectory Vectoring & Calming Pheromone SAR Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-bolt')).toBeVisible();
    await expect(page.locator('#kpi-staged-lure')).toBeVisible();
    await expect(page.locator('#kpi-decompressed-safe')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const boomerCard = page.locator('#acoustic-operations-list .acoustic-op-card', {
      hasText: 'Boomer (',
    });
    const willowCard = page.locator('#acoustic-operations-list .acoustic-op-card', {
      hasText: 'Willow (',
    });
    const miloCard = page.locator('#acoustic-operations-list .acoustic-op-card', {
      hasText: 'Milo (',
    });

    await expect(boomerCard).toBeVisible();
    await expect(boomerCard).toHaveAttribute(
      'data-status',
      'ADRENAL_FLIGHT_BOLT_CRITICAL'
    );
    await expect(boomerCard.locator('.badge-acoustic-status.badge-acoustic-critical')).toContainText(
      'ADRENAL_FLIGHT_BOLT_CRITICAL'
    );
    await expect(boomerCard.locator('.acoustic-advisory-alert-banner')).toContainText('NEVER');

    await expect(willowCard).toBeVisible();
    await expect(willowCard).toHaveAttribute(
      'data-status',
      'CALMING_PERIMETER_SNIFF_STAGED'
    );

    await expect(miloCard).toBeVisible();
    await expect(miloCard).toHaveAttribute(
      'data-status',
      'DECOMPRESSED_CONTAINED_SAFE'
    );
    await expect(miloCard.locator('.acoustic-clearance-banner')).toContainText('ACOUSTIC-SAFE-');

    expect(cspErrors).toEqual([]);
  });

  test('2. Submitting #acoustic-eval-form evaluates a new critical acoustic panic operation with silent no-chase advisory and refuge corridor vector', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/acoustic-panic-trajectory`);
    await page.waitForLoadState('networkidle');

    await page.locator('#eval-pet-name').fill('Ranger');
    await page.locator('#eval-species').fill('Dog - Belgian Malinois');
    await page.locator('#eval-weight-kg').fill('28.4');
    await page.locator('#eval-trigger-type').selectOption('JULY4_NYE_FIREWORKS_BARRAGE');
    await page.locator('#eval-peak-db').fill('136.0');
    await page.locator('#eval-barometric-hpa').fill('2.0');
    await page.locator('#eval-heading-deg').fill('225');
    await page.locator('#eval-barrier-breach').check();
    await page.locator('#eval-no-chase-enforced').check();
    await page.locator('#eval-pheromone-staged').check();

    await page.locator('#btn-submit-acoustic-eval').click();

    const rangerCard = page
      .locator('#acoustic-operations-list .acoustic-op-card', {
        hasText: 'Ranger (',
      })
      .first();
    await expect(rangerCard).toBeVisible();
    await expect(rangerCard).toHaveAttribute(
      'data-status',
      'ADRENAL_FLIGHT_BOLT_CRITICAL'
    );
    await expect(rangerCard.locator('.acoustic-advisory-alert-banner')).toContainText('NEVER');
  });

  test('3. Completing all 4 protocol steps on AP-OP-101 transitions status to CALMING_PERIMETER_SNIFF_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/acoustic-panic-trajectory`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#acoustic-operations-list .acoustic-op-card[data-op-id="AP-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-SILENT-NO-CHASE-PERIMETER',
      'STEP-SCENT-PHEROMONE-REFUGE-LURE',
      'STEP-BARRIER-LACERATION-CLAW-CHECK',
      'STEP-THUNDERSHIRT-ACOUSTIC-DECOMPRESS',
    ]) {
      const cb = card101.locator(`.acoustic-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'CALMING_PERIMETER_SNIFF_STAGED');
    await expect(card101.locator('.badge-acoustic-status.badge-acoustic-staged')).toContainText(
      'CALMING_PERIMETER_SNIFF_STAGED'
    );
  });

  test('4. Clicking .acoustic-issue-clearance-btn issues an ACOUSTIC-SAFE-<12HEX> clearance token and transitions operation to DECOMPRESSED_CONTAINED_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/acoustic-panic-trajectory`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#acoustic-operations-list .acoustic-op-card[data-op-id="AP-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.acoustic-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'DECOMPRESSED_CONTAINED_SAFE');
    await expect(card102.locator('.badge-acoustic-status.badge-acoustic-safe')).toContainText(
      'DECOMPRESSED_CONTAINED_SAFE'
    );
    await expect(card102.locator('.acoustic-clearance-banner')).toContainText('ACOUSTIC-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-acoustic-critical, .badge-acoustic-staged, .badge-acoustic-safe, .badge-acoustic-trigger, .badge-acoustic-flag, .acoustic-advisory-alert-banner, and .acoustic-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/acoustic-panic-trajectory`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-acoustic-panic-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-acoustic-critical" class="badge badge-acoustic-status badge-acoustic-critical">ADRENAL_FLIGHT_BOLT_CRITICAL</span>
        <span id="test-badge-acoustic-staged" class="badge badge-acoustic-status badge-acoustic-staged">CALMING_PERIMETER_SNIFF_STAGED</span>
        <span id="test-badge-acoustic-safe" class="badge badge-acoustic-status badge-acoustic-safe">DECOMPRESSED_CONTAINED_SAFE</span>
        <span id="test-badge-acoustic-trigger" class="badge badge-acoustic-trigger">JULY4_NYE_FIREWORKS_BARRAGE</span>
        <span id="test-badge-acoustic-flag" class="badge badge-acoustic-flag">BARRIER_BREACH_CLAW_DENTAL_LACERATION</span>
        <div id="test-acoustic-advisory-alert-banner" class="acoustic-advisory-alert-banner">CRITICAL ADRENAL FLIGHT PROTOCOL: NEVER call out loudly or chase an adrenalized acoustic-panic pet</div>
        <div id="test-acoustic-clearance-banner" class="acoustic-clearance-banner">ACOUSTIC DECOMPRESSION &amp; CONTAINMENT CLEARANCE VERIFIED: ACOUSTIC-SAFE-1A2B3C4D5E6F</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-acoustic-critical',
        '#test-badge-acoustic-staged',
        '#test-badge-acoustic-safe',
        '#test-badge-acoustic-trigger',
        '#test-badge-acoustic-flag',
        '#test-acoustic-advisory-alert-banner',
        '#test-acoustic-clearance-banner',
      ]) {
        const styles = await page.locator(selector).evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { color: cs.color, backgroundColor: cs.backgroundColor };
        });
        expect(styles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
        const ratio = contrastRatio(styles.color, styles.backgroundColor);
        expect(ratio, `${selector} in ${theme} mode should have > 7.1:1 contrast`).toBeGreaterThan(7.1);
      }
    }
  });
});
