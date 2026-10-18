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

test.describe('Municipal 311 / Animal Control Dispatch CAD Bridge Cockpit UI (/municipal-cad)', () => {
  test('1. /municipal-cad renders seeded incidents (cad-inc-101, cad-inc-102, cad-inc-103), protocol steps, DO-NOT-CHASE alert banner, and safe custody banner with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/municipal-cad`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#municipal-cad-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Municipal 311 / Animal Control Dispatch CAD Bridge Cockpit'
    );

    await expect(page.locator('#kpi-cad-total')).toBeVisible();
    await expect(page.locator('#kpi-cad-critical')).toBeVisible();
    await expect(page.locator('#kpi-cad-no-chase')).toBeVisible();
    await expect(page.locator('#kpi-cad-resolved')).toBeVisible();
    await expect(page.locator('#municipal-cad-live-status')).toBeVisible();

    const card101 = page.locator(
      '#cad-incidents-list .cad-incident-card[data-incident-id="cad-inc-101"]'
    );
    const card102 = page.locator(
      '#cad-incidents-list .cad-incident-card[data-incident-id="cad-inc-102"]'
    );
    const card103 = page.locator(
      '#cad-incidents-list .cad-incident-card[data-incident-id="cad-inc-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-dispatch-status', 'INBOUND_311_TRIAGE');
    await expect(card101.locator('.badge-cad-status.badge-cad-triage')).toContainText(
      'INBOUND_311_TRIAGE'
    );
    await expect(card101.locator('.badge-hazard-severity.badge-hazard-freeway')).toContainText(
      'FREEWAY_MEDIAN_CRITICAL'
    );
    await expect(card101.locator('.badge-cad-metric')).toContainText('Containment Readiness:');
    await expect(card101.locator('.cad-no-chase-banner')).toContainText(
      'DO NOT CHASE — GOOD SAMARITAN CALMING PROTOCOL'
    );
    await expect(card101.locator('.cad-protocol-list')).toContainText('GOOD_SAMARITAN_CALMING');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute(
      'data-dispatch-status',
      'HIGHWAY_CONTAINMENT_DISPATCHED'
    );
    await expect(card102.locator('.badge-cad-status.badge-cad-containment')).toContainText(
      'HIGHWAY_CONTAINMENT_DISPATCHED'
    );
    await expect(card102.locator('.badge-hazard-severity.badge-hazard-arterial')).toContainText(
      'ARTERIAL_TRAFFIC_HIGH'
    );

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-dispatch-status', 'RESOLVED_SAFE_CUSTODY');
    await expect(card103.locator('.badge-cad-status.badge-cad-resolved')).toContainText(
      'RESOLVED_SAFE_CUSTODY'
    );
    await expect(card103.locator('.badge-hazard-severity.badge-hazard-safe')).toContainText(
      'CONTAINED_CORRIDOR_SAFE'
    );
    await expect(card103.locator('.cad-resolution-banner')).toContainText(
      'SAFE CUSTODY INTERCEPT VERIFIED: CAD-SAFE-CAD-INC-103'
    );

    await expect(page.locator('#form-create-cad-incident')).toBeVisible();
    await expect(page.locator('#btn-sim-freeway-median')).toBeVisible();
    await expect(page.locator('#btn-sim-arterial-bridge')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Simulating a 65 MPH freeway median call (#btn-sim-freeway-median) creates an INBOUND_311_TRIAGE card with .cad-no-chase-banner (DO NOT CHASE — GOOD SAMARITAN CALMING PROTOCOL) and FREEWAY_MEDIAN_CRITICAL', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/municipal-cad`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-freeway-median').click();

    const simCard = page
      .locator('#cad-incidents-list .cad-incident-card', {
        hasText: 'I-5 Southbound Ship Canal Bridge Median',
      })
      .first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-dispatch-status', 'INBOUND_311_TRIAGE');
    await expect(simCard.locator('.badge-cad-status.badge-cad-triage')).toContainText(
      'INBOUND_311_TRIAGE'
    );
    await expect(simCard.locator('.badge-hazard-severity.badge-hazard-freeway')).toContainText(
      'FREEWAY_MEDIAN_CRITICAL'
    );
    await expect(simCard.locator('.cad-no-chase-banner')).toContainText(
      'DO NOT CHASE — GOOD SAMARITAN CALMING PROTOCOL'
    );
  });

  test('3. Clicking .btn-deploy-cad-containment activates DOT VMS & ACO L-barrier (Containment Readiness: 100/100) and transitions incident to HIGHWAY_CONTAINMENT_DISPATCHED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/municipal-cad`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-arterial-bridge').click();

    const bridgeCard = page
      .locator('#cad-incidents-list .cad-incident-card', {
        hasText: 'Ballard Bridge Northbound Bascule Span',
      })
      .first();
    await expect(bridgeCard).toBeVisible();
    await expect(bridgeCard).toHaveAttribute('data-dispatch-status', 'INBOUND_311_TRIAGE');

    await bridgeCard.locator('.btn-deploy-cad-containment').click();

    await expect(bridgeCard).toHaveAttribute(
      'data-dispatch-status',
      'HIGHWAY_CONTAINMENT_DISPATCHED'
    );
    await expect(bridgeCard.locator('.badge-cad-status.badge-cad-containment')).toContainText(
      'HIGHWAY_CONTAINMENT_DISPATCHED'
    );
    await expect(bridgeCard.locator('.badge-cad-metric')).toContainText(
      'Containment Readiness: 100/100'
    );
    await expect(bridgeCard.locator('.badge-hazard-severity.badge-hazard-arterial')).toContainText(
      'ARTERIAL_TRAFFIC_HIGH'
    );
  });

  test('4. Clicking .btn-resolve-cad-custody transitions an incident to RESOLVED_SAFE_CUSTODY (CONTAINED_CORRIDOR_SAFE) and displays .cad-resolution-banner (SAFE CUSTODY INTERCEPT VERIFIED: CAD-SAFE-...)', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/municipal-cad`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-arterial-bridge').click();

    const card = page
      .locator('#cad-incidents-list .cad-incident-card', {
        hasText: 'Ballard Bridge Northbound Bascule Span',
      })
      .first();
    await expect(card).toBeVisible();

    await card.locator('.btn-resolve-cad-custody').click();

    await expect(card).toHaveAttribute('data-dispatch-status', 'RESOLVED_SAFE_CUSTODY');
    await expect(card.locator('.badge-cad-status.badge-cad-resolved')).toContainText(
      'RESOLVED_SAFE_CUSTODY'
    );
    await expect(card.locator('.badge-hazard-severity.badge-hazard-safe')).toContainText(
      'CONTAINED_CORRIDOR_SAFE'
    );
    await expect(card.locator('.cad-resolution-banner')).toBeVisible();
    await expect(card.locator('.cad-resolution-banner')).toContainText(
      'SAFE CUSTODY INTERCEPT VERIFIED: CAD-SAFE-'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7:1) on .badge-cad-triage, .badge-cad-containment, .badge-cad-resolved, .badge-hazard-freeway, .badge-hazard-arterial, .badge-hazard-safe, .badge-cad-metric, .cad-no-chase-banner, and .cad-resolution-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/municipal-cad`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-municipal-cad-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-cad-triage" class="badge badge-cad-status badge-cad-triage">INBOUND_311_TRIAGE</span>
        <span id="test-badge-cad-containment" class="badge badge-cad-status badge-cad-containment">HIGHWAY_CONTAINMENT_DISPATCHED</span>
        <span id="test-badge-cad-resolved" class="badge badge-cad-status badge-cad-resolved">RESOLVED_SAFE_CUSTODY</span>
        <span id="test-badge-hazard-freeway" class="badge badge-hazard-severity badge-hazard-freeway">FREEWAY_MEDIAN_CRITICAL</span>
        <span id="test-badge-hazard-arterial" class="badge badge-hazard-severity badge-hazard-arterial">ARTERIAL_TRAFFIC_HIGH</span>
        <span id="test-badge-hazard-safe" class="badge badge-hazard-severity badge-hazard-safe">CONTAINED_CORRIDOR_SAFE</span>
        <span id="test-badge-cad-metric" class="badge badge-cad-metric">Containment Readiness: 100/100</span>
        <div id="test-cad-no-chase-banner" class="cad-no-chase-banner">DO NOT CHASE — GOOD SAMARITAN CALMING PROTOCOL</div>
        <div id="test-cad-resolution-banner" class="cad-resolution-banner">SAFE CUSTODY INTERCEPT VERIFIED: CAD-SAFE-101</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-cad-triage',
        '#test-badge-cad-containment',
        '#test-badge-cad-resolved',
        '#test-badge-hazard-freeway',
        '#test-badge-hazard-arterial',
        '#test-badge-hazard-safe',
        '#test-badge-cad-metric',
        '#test-cad-no-chase-banner',
        '#test-cad-resolution-banner',
      ]) {
        const styles = await page.locator(selector).evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { color: cs.color, backgroundColor: cs.backgroundColor };
        });
        expect(styles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
        const ratio = contrastRatio(styles.color, styles.backgroundColor);
        expect(ratio, `${selector} in ${theme} mode should have > 7:1 contrast`).toBeGreaterThan(
          7.0
        );
      }
    }
  });
});
