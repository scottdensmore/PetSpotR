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

test.describe('Wildfire Smoke, Ashfall, Hazmat Plume & Respiratory Decontamination Triage Cockpit UI (/hazmat-decon)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /hazmat-decon renders seeded operations (HZ-OP-101, HZ-OP-102, HZ-OP-103), summary stats, protocol steps, and smoke alert banner with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/hazmat-decon`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#hazmat-decon-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Wildfire Smoke, Ashfall, Hazmat Plume & Respiratory Decontamination Triage Cockpit'
    );

    await expect(page.locator('#stat-total-hazmat-ops')).toBeVisible();
    await expect(page.locator('#stat-critical-smoke-alerts')).toBeVisible();
    await expect(page.locator('#stat-staged-decon-washes')).toBeVisible();
    await expect(page.locator('#stat-cleared-safe-count')).toBeVisible();

    const card101 = page.locator('#hz-ops-list .hz-op-card[data-op-id="HZ-OP-101"]');
    const card102 = page.locator('#hz-ops-list .hz-op-card[data-op-id="HZ-OP-102"]');
    const card103 = page.locator('#hz-ops-list .hz-op-card[data-op-id="HZ-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-status', 'RESPIRATORY_DERMAL_CRITICAL');
    await expect(card101.locator('.badge-hz-status.badge-hz-critical')).toContainText(
      'RESPIRATORY_DERMAL_CRITICAL'
    );
    await expect(card101.locator('.badge-hz-exposure').first()).toContainText('WILDFIRE_SMOKE_ASHFALL');
    await expect(card101.locator('.hz-smoke-alert-banner')).toContainText(
      'RESPIRATORY & DERMAL TOXIN HAZARD ALERT'
    );
    await expect(card101.locator('.badge-hz-flag').first()).toContainText(
      'EXTREME_SMOKE_CYANIDE_HYPOXIA_RISK'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-status', 'DECON_WASH_NEBULIZER_STAGED');
    await expect(card102.locator('.badge-hz-status.badge-hz-staged')).toContainText(
      'DECON_WASH_NEBULIZER_STAGED'
    );

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-status', 'DECONTAMINATED_CLEARED_SAFE');
    await expect(card103.locator('.badge-hz-status.badge-hz-cleared')).toContainText(
      'DECONTAMINATED_CLEARED_SAFE'
    );
    await expect(card103.locator('.hz-clearance-banner')).toContainText(
      'HAZMAT-SAFE-9F4B21E8C305'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Clicking .hz-scenario-btn[data-scenario="wildfire-ash-plume"] evaluates a wildfire ashfall plume emergency with .hz-smoke-alert-banner and EXTREME_SMOKE_CYANIDE_HYPOXIA_RISK', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hazmat-decon`);
    await page.waitForLoadState('networkidle');

    await page.locator('.hz-scenario-btn[data-scenario="wildfire-ash-plume"]').click();

    const simCard = page
      .locator('#hz-ops-list .hz-op-card', {
        hasText: 'Ash',
      })
      .first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-status', 'RESPIRATORY_DERMAL_CRITICAL');
    await expect(simCard.locator('.badge-hz-status.badge-hz-critical')).toContainText(
      'RESPIRATORY_DERMAL_CRITICAL'
    );
    await expect(simCard.locator('.hz-smoke-alert-banner')).toContainText(
      'RESPIRATORY & DERMAL TOXIN HAZARD ALERT'
    );
    await expect(simCard.locator('.badge-hz-flag').first()).toContainText(
      'EXTREME_SMOKE_CYANIDE_HYPOXIA_RISK'
    );
  });

  test('3. Toggling .hz-step-checkbox on HZ-OP-101 updates protocol readiness to 100% and transitions status to DECON_WASH_NEBULIZER_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hazmat-decon`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#hz-ops-list .hz-op-card[data-op-id="HZ-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of ['step-warm-surfactant-wash', 'step-oxygen-nebulizer', 'step-cohb-binder-panel']) {
      const cb = card101.locator(`.hz-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'DECON_WASH_NEBULIZER_STAGED');
    await expect(card101.locator('.badge-hz-status.badge-hz-staged')).toContainText(
      'DECON_WASH_NEBULIZER_STAGED'
    );
    await expect(card101).toContainText('Decon Readiness: 100%');
  });

  test('4. Clicking .hz-issue-clearance-btn issues a HAZMAT-SAFE-<12HEX> certificate and transitions operation to DECONTAMINATED_CLEARED_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hazmat-decon`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#hz-ops-list .hz-op-card[data-op-id="HZ-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.hz-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'DECONTAMINATED_CLEARED_SAFE');
    await expect(card102.locator('.badge-hz-status.badge-hz-cleared')).toContainText(
      'DECONTAMINATED_CLEARED_SAFE'
    );
    await expect(card102.locator('.hz-clearance-banner')).toBeVisible();
    await expect(card102.locator('.hz-clearance-banner')).toContainText(
      'HAZMAT & RESPIRATORY DECON CLEARANCE VERIFIED: HAZMAT-SAFE-'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-hz-critical, .badge-hz-staged, .badge-hz-cleared, .badge-hz-exposure, .badge-hz-flag, .hz-smoke-alert-banner, and .hz-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hazmat-decon`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-hazmat-decon-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-hz-critical" class="badge badge-hz-status badge-hz-critical">RESPIRATORY_DERMAL_CRITICAL</span>
        <span id="test-badge-hz-staged" class="badge badge-hz-status badge-hz-staged">DECON_WASH_NEBULIZER_STAGED</span>
        <span id="test-badge-hz-cleared" class="badge badge-hz-status badge-hz-cleared">DECONTAMINATED_CLEARED_SAFE</span>
        <span id="test-badge-hz-exposure" class="badge badge-hz-exposure">WILDFIRE_SMOKE_ASHFALL</span>
        <span id="test-badge-hz-flag" class="badge badge-hz-flag">EXTREME_SMOKE_CYANIDE_HYPOXIA_RISK</span>
        <div id="test-hz-smoke-alert-banner" class="hz-smoke-alert-banner">RESPIRATORY &amp; DERMAL TOXIN HAZARD ALERT</div>
        <div id="test-hz-clearance-banner" class="hz-clearance-banner">HAZMAT &amp; RESPIRATORY DECON CLEARANCE VERIFIED: HAZMAT-SAFE-9F4B21E8C305</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-hz-critical',
        '#test-badge-hz-staged',
        '#test-badge-hz-cleared',
        '#test-badge-hz-exposure',
        '#test-badge-hz-flag',
        '#test-hz-smoke-alert-banner',
        '#test-hz-clearance-banner',
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
