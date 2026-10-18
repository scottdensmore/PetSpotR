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

test.describe('Hyperthermia, Heatstroke & Hot-Asphalt Pad Burn Triage Cockpit UI (/hyperthermia-heat-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /hyperthermia-heat-triage renders seeded operations (HT-OP-101, HT-OP-102, HT-OP-103), KPI telemetry, 39.4°C cutoff alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/hyperthermia-heat-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#hyperthermia-heat-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Hyperthermia, Heatstroke & Hot-Asphalt Pad Burn Triage Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-dic')).toBeVisible();
    await expect(page.locator('#kpi-staged-cooling')).toBeVisible();
    await expect(page.locator('#kpi-stabilized-safe')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const apolloCard = page.locator('#heat-operations-list .heat-op-card', {
      hasText: 'Apollo (',
    });
    const biscuitCard = page.locator('#heat-operations-list .heat-op-card', {
      hasText: 'Biscuit (',
    });
    const daisyCard = page.locator('#heat-operations-list .heat-op-card', {
      hasText: 'Daisy (',
    });

    await expect(apolloCard).toBeVisible();
    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'HEATSTROKE_DIC_COAGULOPATHY_CRITICAL'
    );
    await expect(apolloCard.locator('.badge-heat-status.badge-heat-critical')).toContainText(
      'HEATSTROKE_DIC_COAGULOPATHY_CRITICAL'
    );
    await expect(apolloCard.locator('.heat-cutoff-alert-banner')).toContainText('39.4°C');

    await expect(biscuitCard).toBeVisible();
    await expect(biscuitCard).toHaveAttribute(
      'data-status',
      'EVAPORATIVE_COOLING_394C_STAGED'
    );

    await expect(daisyCard).toBeVisible();
    await expect(daisyCard).toHaveAttribute(
      'data-status',
      'STABILIZED_NORMOTHERMIC_COOLED'
    );
    await expect(daisyCard.locator('.heat-clearance-banner')).toContainText('HEAT-SAFE-');

    expect(cspErrors).toEqual([]);
  });

  test('2. Submitting #heat-eval-form evaluates a new critical hyperthermia operation with 39.4°C cooling cutoff and BOAS airway protocol', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hyperthermia-heat-triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#eval-pet-name').fill('Brutus');
    await page.locator('#eval-species').fill('Dog - English Bulldog');
    await page.locator('#eval-weight-kg').fill('24.5');
    await page.locator('#eval-exposure-type').selectOption('ENCLOSED_VEHICLE_SOLAR_CABIN');
    await page.locator('#eval-rectal-temp-c').fill('41.9');
    await page.locator('#eval-surface-temp-c').fill('66.0');
    await page.locator('#eval-exposure-mins').fill('38');
    await page.locator('#eval-paw-sloughing-grade').selectOption('2');
    await page.locator('#eval-brachycephalic-airway').check();
    await page.locator('#eval-convection-fan').check();
    await page.locator('#eval-ice-bath-avoided').check();

    await page.locator('#btn-submit-heat-eval').click();

    const brutusCard = page
      .locator('#heat-operations-list .heat-op-card', {
        hasText: 'Brutus (',
      })
      .first();
    await expect(brutusCard).toBeVisible();
    await expect(brutusCard).toHaveAttribute(
      'data-status',
      'HEATSTROKE_DIC_COAGULOPATHY_CRITICAL'
    );
    await expect(brutusCard.locator('.heat-cutoff-alert-banner')).toContainText('39.4°C');
  });

  test('3. Completing all 4 protocol steps on HT-OP-101 transitions status to EVAPORATIVE_COOLING_394C_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hyperthermia-heat-triage`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#heat-operations-list .heat-op-card[data-op-id="HT-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-CONVECTIVE-COOLING-START',
      'STEP-STOP-COOLING-394C-CUTOFF',
      'STEP-PAW-PAD-HYDROGEL-DEBRIDE',
      'STEP-DIC-COAG-CRYSTALLOID-IV',
    ]) {
      const cb = card101.locator(`.heat-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'EVAPORATIVE_COOLING_394C_STAGED');
    await expect(card101.locator('.badge-heat-status.badge-heat-staged')).toContainText(
      'EVAPORATIVE_COOLING_394C_STAGED'
    );
  });

  test('4. Clicking .heat-issue-clearance-btn issues a HEAT-SAFE-<12HEX> clearance token and transitions operation to STABILIZED_NORMOTHERMIC_COOLED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hyperthermia-heat-triage`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#heat-operations-list .heat-op-card[data-op-id="HT-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.heat-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'STABILIZED_NORMOTHERMIC_COOLED');
    await expect(card102.locator('.badge-heat-status.badge-heat-safe')).toContainText(
      'STABILIZED_NORMOTHERMIC_COOLED'
    );
    await expect(card102.locator('.heat-clearance-banner')).toContainText('HEAT-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-heat-critical, .badge-heat-staged, .badge-heat-safe, .badge-heat-exposure, .badge-heat-flag, .heat-cutoff-alert-banner, and .heat-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/hyperthermia-heat-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-heat-triage-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-heat-critical" class="badge badge-heat-status badge-heat-critical">HEATSTROKE_DIC_COAGULOPATHY_CRITICAL</span>
        <span id="test-badge-heat-staged" class="badge badge-heat-status badge-heat-staged">EVAPORATIVE_COOLING_394C_STAGED</span>
        <span id="test-badge-heat-safe" class="badge badge-heat-status badge-heat-safe">STABILIZED_NORMOTHERMIC_COOLED</span>
        <span id="test-badge-heat-exposure" class="badge badge-heat-exposure">ENCLOSED_VEHICLE_SOLAR_CABIN</span>
        <span id="test-badge-heat-flag" class="badge badge-heat-flag">HYPERPYREXIA_DIC_COAGULOPATHY_RISK</span>
        <div id="test-heat-cutoff-alert-banner" class="heat-cutoff-alert-banner">CRITICAL CUTOFF: Stop active evaporative cooling strictly at 39.4°C (103.0°F)</div>
        <div id="test-heat-clearance-banner" class="heat-clearance-banner">NORMOTHERMIC COOLING &amp; PAW PAD CLEARANCE VERIFIED: HEAT-SAFE-1A2B3C4D5E6F</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-heat-critical',
        '#test-badge-heat-staged',
        '#test-badge-heat-safe',
        '#test-badge-heat-exposure',
        '#test-badge-heat-flag',
        '#test-heat-cutoff-alert-banner',
        '#test-heat-clearance-banner',
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
