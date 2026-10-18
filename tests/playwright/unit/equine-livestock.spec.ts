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

test.describe('Livestock, Equine & Large-Animal Containment Cockpit UI (/equine-livestock)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /equine-livestock renders seeded dispatches (eq-dsp-101, eq-dsp-102, eq-dsp-103), containment steps, and highway stampede alert banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/equine-livestock`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#equine-livestock-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Livestock, Equine & Large-Animal Containment Cockpit'
    );

    await expect(page.locator('#kpi-eq-total')).toBeVisible();
    await expect(page.locator('#kpi-eq-stampede')).toBeVisible();
    await expect(page.locator('#kpi-eq-corralled')).toBeVisible();
    await expect(page.locator('#kpi-eq-loaded')).toBeVisible();
    await expect(page.locator('#equine-livestock-live-status')).toBeVisible();

    const card101 = page.locator('#eq-dispatches-list .eq-dispatch-card[data-dispatch-id="eq-dsp-101"]');
    const card102 = page.locator('#eq-dispatches-list .eq-dispatch-card[data-dispatch-id="eq-dsp-102"]');
    const card103 = page.locator('#eq-dispatches-list .eq-dispatch-card[data-dispatch-id="eq-dsp-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-dispatch-status', 'LOOSE_HERD_CORRIDOR_ALERT');
    await expect(card101.locator('.badge-eq-status.badge-eq-loose')).toContainText('LOOSE_HERD_CORRIDOR_ALERT');
    await expect(card101.locator('.badge-collision-risk.badge-collision-critical')).toContainText('CRITICAL_HIGHWAY_STAMPEDE_RISK');
    await expect(card101.locator('.eq-stampede-alert-banner')).toContainText('HIGHWAY STAMPEDE & SIREN BLACKOUT PROTOCOL');
    await expect(card101.locator('.eq-steps-list')).toContainText('HERD_FLANK_CONTAINMENT');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-dispatch-status', 'PORTABLE_CORRAL_STAGED');
    await expect(card102.locator('.badge-eq-status.badge-eq-corralled')).toContainText('PORTABLE_CORRAL_STAGED');
    await expect(card102.locator('.badge-collision-risk.badge-collision-elevated')).toContainText('ELEVATED_PASTURE_PERIMETER_RISK');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-dispatch-status', 'LOADED_SAFE_PADDOCK_CUSTODY');
    await expect(card103.locator('.badge-eq-status.badge-eq-loaded')).toContainText('LOADED_SAFE_PADDOCK_CUSTODY');
    await expect(card103.locator('.badge-collision-risk.badge-collision-safe')).toContainText('CONTAINED_PADDOCK_SAFE');
    await expect(card103.locator('.eq-paddock-banner')).toContainText('GOOSENECK TRAILER & PADDOCK CUSTODY VERIFIED: PADDOCK-SAFE-EQ-DSP-103');

    await expect(page.locator('#form-create-eq-dispatch')).toBeVisible();
    await expect(page.locator('#btn-sim-spooked-thoroughbred')).toBeVisible();
    await expect(page.locator('#btn-sim-alpaca-herd-breach')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Simulating a 520kg spooked Thoroughbred (#btn-sim-spooked-thoroughbred) creates a LOOSE_HERD_CORRIDOR_ALERT card with .eq-stampede-alert-banner and CRITICAL_HIGHWAY_STAMPEDE_RISK', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/equine-livestock`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-spooked-thoroughbred').click();

    const simCard = page.locator('#eq-dispatches-list .eq-dispatch-card', {
      hasText: 'Thunderbolt',
    }).first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-dispatch-status', 'LOOSE_HERD_CORRIDOR_ALERT');
    await expect(simCard.locator('.badge-eq-status.badge-eq-loose')).toContainText('LOOSE_HERD_CORRIDOR_ALERT');
    await expect(simCard.locator('.badge-collision-risk.badge-collision-critical')).toContainText('CRITICAL_HIGHWAY_STAMPEDE_RISK');
    await expect(simCard.locator('.eq-stampede-alert-banner')).toContainText('HIGHWAY STAMPEDE & SIREN BLACKOUT PROTOCOL');
  });

  test('3. Clicking .btn-stage-eq-corral stages 12ft corral panels and gooseneck stock trailer (Trailer Readiness: 100/100) and transitions dispatch to PORTABLE_CORRAL_STAGED', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/equine-livestock`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-alpaca-herd-breach').click();

    const alpacaCard = page.locator('#eq-dispatches-list .eq-dispatch-card', {
      hasText: 'Cusco Quartet',
    }).first();
    await expect(alpacaCard).toBeVisible();
    await expect(alpacaCard).toHaveAttribute('data-dispatch-status', 'LOOSE_HERD_CORRIDOR_ALERT');

    await alpacaCard.locator('.btn-stage-eq-corral').click();

    await expect(alpacaCard).toHaveAttribute('data-dispatch-status', 'PORTABLE_CORRAL_STAGED');
    await expect(alpacaCard.locator('.badge-eq-status.badge-eq-corralled')).toContainText('PORTABLE_CORRAL_STAGED');
    await expect(alpacaCard.locator('.badge-eq-metric')).toContainText('Trailer Readiness: 100/100');
    await expect(alpacaCard.locator('.badge-collision-risk.badge-collision-elevated')).toContainText('ELEVATED_PASTURE_PERIMETER_RISK');
  });

  test('4. Clicking .btn-complete-eq-load transitions a dispatch to LOADED_SAFE_PADDOCK_CUSTODY (CONTAINED_PADDOCK_SAFE) and displays .eq-paddock-banner (PADDOCK-SAFE-...)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/equine-livestock`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-alpaca-herd-breach').click();

    const card = page.locator('#eq-dispatches-list .eq-dispatch-card', {
      hasText: 'Cusco Quartet',
    }).first();
    await expect(card).toBeVisible();

    await card.locator('.btn-complete-eq-load').click();

    await expect(card).toHaveAttribute('data-dispatch-status', 'LOADED_SAFE_PADDOCK_CUSTODY');
    await expect(card.locator('.badge-eq-status.badge-eq-loaded')).toContainText('LOADED_SAFE_PADDOCK_CUSTODY');
    await expect(card.locator('.badge-collision-risk.badge-collision-safe')).toContainText('CONTAINED_PADDOCK_SAFE');
    await expect(card.locator('.eq-paddock-banner')).toBeVisible();
    await expect(card.locator('.eq-paddock-banner')).toContainText('GOOSENECK TRAILER & PADDOCK CUSTODY VERIFIED: PADDOCK-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7:1) on .badge-eq-loose, .badge-eq-corralled, .badge-eq-loaded, .badge-collision-critical, .badge-collision-elevated, .badge-collision-safe, .badge-eq-metric, .eq-stampede-alert-banner, and .eq-paddock-banner in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/equine-livestock`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-equine-livestock-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-eq-loose" class="badge badge-eq-status badge-eq-loose">LOOSE_HERD_CORRIDOR_ALERT</span>
        <span id="test-badge-eq-corralled" class="badge badge-eq-status badge-eq-corralled">PORTABLE_CORRAL_STAGED</span>
        <span id="test-badge-eq-loaded" class="badge badge-eq-status badge-eq-loaded">LOADED_SAFE_PADDOCK_CUSTODY</span>
        <span id="test-badge-collision-critical" class="badge badge-collision-risk badge-collision-critical">CRITICAL_HIGHWAY_STAMPEDE_RISK</span>
        <span id="test-badge-collision-elevated" class="badge badge-collision-risk badge-collision-elevated">ELEVATED_PASTURE_PERIMETER_RISK</span>
        <span id="test-badge-collision-safe" class="badge badge-collision-risk badge-collision-safe">CONTAINED_PADDOCK_SAFE</span>
        <span id="test-badge-eq-metric" class="badge badge-eq-metric">Collision Risk: 81/100 • Trailer Readiness: 100/100</span>
        <div id="test-eq-stampede-alert-banner" class="eq-stampede-alert-banner">HIGHWAY STAMPEDE &amp; SIREN BLACKOUT PROTOCOL</div>
        <div id="test-eq-paddock-banner" class="eq-paddock-banner">GOOSENECK TRAILER &amp; PADDOCK CUSTODY VERIFIED: PADDOCK-SAFE-EQ-DSP-101</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-eq-loose',
        '#test-badge-eq-corralled',
        '#test-badge-eq-loaded',
        '#test-badge-collision-critical',
        '#test-badge-collision-elevated',
        '#test-badge-collision-safe',
        '#test-badge-eq-metric',
        '#test-eq-stampede-alert-banner',
        '#test-eq-paddock-banner',
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
