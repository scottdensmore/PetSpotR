import { test, expect, type Locator, type Page } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

/**
 * Clicks #btn-sim-fireworks-bolt and returns the card for the plan that click
 * created, so parallel specs simulating the same case never share a card.
 */
async function simulateFireworksBolt(page: Page): Promise<Locator> {
  const [response] = await Promise.all([
    page.waitForResponse(
      (res) =>
        res.url().endsWith('/api/v1/reunion-decompression/plans') && res.request().method() === 'POST'
    ),
    page.locator('#btn-sim-fireworks-bolt').click(),
  ]);
  const { planId } = (await response.json()) as { planId: string };
  return page.locator(`#decomp-plans-list .decomp-plan-card[data-plan-id="${planId}"]`);
}

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

test.describe('Post-Reunion Cortisol Decompression & Escape-Recidivism Cockpit UI (/reunion-decompression)', () => {
  test('1. /reunion-decompression renders seeded plans (decomp-plan-101, decomp-plan-102, decomp-plan-103), checklist items, perimeter findings, and refeeding alert banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/reunion-decompression`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#reunion-decompression-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Post-Reunion Cortisol Decompression & Escape-Recidivism Cockpit'
    );

    await expect(page.locator('#kpi-decomp-total')).toBeVisible();
    await expect(page.locator('#kpi-decomp-critical')).toBeVisible();
    await expect(page.locator('#kpi-decomp-refeeding')).toBeVisible();
    await expect(page.locator('#kpi-decomp-graduated')).toBeVisible();
    await expect(page.locator('#reunion-decompression-live-status')).toBeVisible();

    const card101 = page.locator('#decomp-plans-list .decomp-plan-card[data-plan-id="decomp-plan-101"]');
    const card102 = page.locator('#decomp-plans-list .decomp-plan-card[data-plan-id="decomp-plan-102"]');
    const card103 = page.locator('#decomp-plans-list .decomp-plan-card[data-plan-id="decomp-plan-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-plan-phase', 'ACUTE_REFUGE_72H');
    await expect(card101.locator('.badge-decomp-phase.badge-phase-acute')).toContainText('ACUTE_REFUGE_72H');
    await expect(card101.locator('.badge-recidivism-risk.badge-risk-critical')).toContainText('CRITICAL_FLIGHT_RISK');
    await expect(card101.locator('.badge-cortisol-metric')).toContainText('Cortisol:');
    await expect(card101.locator('.badge-cortisol-metric')).toContainText('Collar Readiness:');
    await expect(card101.locator('.decomp-refeeding-warning-banner')).toContainText('REFEEDING SYNDROME ALERT');
    await expect(card101.locator('.decomp-checklist-list')).toContainText('CRITICAL REFEEDING PROTOCOL');
    await expect(card101.locator('.decomp-perimeter-list')).toContainText('SINGLE_DOOR_BOLT_CORRIDOR');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-plan-phase', 'CORTISOL_TAPER_DAY_4_7');
    await expect(card102.locator('.badge-decomp-phase.badge-phase-taper')).toContainText('CORTISOL_TAPER_DAY_4_7');
    await expect(card102.locator('.badge-recidivism-risk.badge-risk-elevated')).toContainText('ELEVATED_PERIMETER_RISK');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-plan-phase', 'GRADUATED_STABLE');
    await expect(card103.locator('.badge-decomp-phase.badge-phase-graduated')).toContainText('GRADUATED_STABLE');
    await expect(card103.locator('.badge-recidivism-risk.badge-risk-fortified')).toContainText('FORTIFIED_LOW_RISK');
    await expect(card103.locator('.decomp-graduation-banner')).toContainText('CERT-DECOMP-DECOMP-PLAN-103');

    await expect(page.locator('#form-create-decomp-plan')).toBeVisible();
    await expect(page.locator('#btn-sim-starvation-refeeding')).toBeVisible();
    await expect(page.locator('#btn-sim-fireworks-bolt')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Simulating a starvation refeeding case (#btn-sim-starvation-refeeding) creates an ACUTE_REFUGE_72H card with .decomp-refeeding-warning-banner (REFEEDING SYNDROME ALERT) and CRITICAL_FLIGHT_RISK', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/reunion-decompression`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-starvation-refeeding').click();

    const simCard = page.locator('#decomp-plans-list .decomp-plan-card', {
      hasText: '11d missing',
    }).first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-plan-phase', 'ACUTE_REFUGE_72H');
    await expect(simCard.locator('.badge-decomp-phase.badge-phase-acute')).toContainText('ACUTE_REFUGE_72H');
    await expect(simCard.locator('.badge-recidivism-risk.badge-risk-critical')).toContainText('CRITICAL_FLIGHT_RISK');
    await expect(simCard.locator('.decomp-refeeding-warning-banner')).toContainText('REFEEDING SYNDROME ALERT');
  });

  test('3. Clicking .btn-remediate-perimeter fortifies perimeter findings, equips GPS collar + double leash (Collar Readiness: 100/100), and reduces recidivism risk', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/reunion-decompression`);
    await page.waitForLoadState('networkidle');

    const boltCard = await simulateFireworksBolt(page);
    await expect(boltCard).toBeVisible();
    await expect(boltCard).toHaveAttribute('data-plan-phase', 'ACUTE_REFUGE_72H');
    await expect(boltCard.locator('.badge-recidivism-risk.badge-risk-critical')).toContainText('CRITICAL_FLIGHT_RISK');

    await boltCard.locator('.btn-remediate-perimeter').click();

    await expect(boltCard).toHaveAttribute('data-plan-phase', 'CORTISOL_TAPER_DAY_4_7');
    await expect(boltCard.locator('.badge-cortisol-metric')).toContainText('Collar Readiness: 100/100');
    await expect(boltCard.locator('.badge-recidivism-risk')).toContainText('ELEVATED_PERIMETER_RISK');
  });

  test('4. Clicking .btn-graduate-decomp-plan transitions a plan to GRADUATED_STABLE (FORTIFIED_LOW_RISK) and displays .decomp-graduation-banner (CERT-DECOMP-...)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/reunion-decompression`);
    await page.waitForLoadState('networkidle');

    const card = await simulateFireworksBolt(page);
    await expect(card).toBeVisible();

    await card.locator('.btn-graduate-decomp-plan').click();

    await expect(card).toHaveAttribute('data-plan-phase', 'GRADUATED_STABLE');
    await expect(card.locator('.badge-decomp-phase.badge-phase-graduated')).toContainText('GRADUATED_STABLE');
    await expect(card.locator('.badge-recidivism-risk.badge-risk-fortified')).toContainText('FORTIFIED_LOW_RISK');
    await expect(card.locator('.decomp-graduation-banner')).toBeVisible();
    await expect(card.locator('.decomp-graduation-banner')).toContainText('CERT-DECOMP-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7:1) on .badge-phase-acute, .badge-phase-taper, .badge-phase-graduated, .badge-risk-critical, .badge-risk-fortified, .badge-cortisol-metric, and .decomp-refeeding-warning-banner in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/reunion-decompression`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-reunion-decompression-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-phase-acute" class="badge badge-decomp-phase badge-phase-acute">ACUTE_REFUGE_72H</span>
        <span id="test-badge-phase-taper" class="badge badge-decomp-phase badge-phase-taper">CORTISOL_TAPER_DAY_4_7</span>
        <span id="test-badge-phase-reintegration" class="badge badge-decomp-phase badge-phase-reintegration">ROUTINE_REINTEGRATION_DAY_8_14</span>
        <span id="test-badge-phase-graduated" class="badge badge-decomp-phase badge-phase-graduated">GRADUATED_STABLE</span>
        <span id="test-badge-risk-critical" class="badge badge-recidivism-risk badge-risk-critical">CRITICAL_FLIGHT_RISK</span>
        <span id="test-badge-risk-elevated" class="badge badge-recidivism-risk badge-risk-elevated">ELEVATED_PERIMETER_RISK</span>
        <span id="test-badge-risk-fortified" class="badge badge-recidivism-risk badge-risk-fortified">FORTIFIED_LOW_RISK</span>
        <span id="test-badge-cortisol-metric" class="badge badge-cortisol-metric">Cortisol: 82/100 • Collar Readiness: 20/100</span>
        <div id="test-decomp-refeeding-warning-banner" class="decomp-refeeding-warning-banner">REFEEDING SYNDROME ALERT</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-phase-acute',
        '#test-badge-phase-taper',
        '#test-badge-phase-reintegration',
        '#test-badge-phase-graduated',
        '#test-badge-risk-critical',
        '#test-badge-risk-elevated',
        '#test-badge-risk-fortified',
        '#test-badge-cortisol-metric',
        '#test-decomp-refeeding-warning-banner',
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
