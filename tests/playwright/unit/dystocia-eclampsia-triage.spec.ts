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

test.describe('Dystocia, Maternal C-Section, Eclampsia (Hypocalcemia) & Oxytocin Contraindication Triage Cockpit UI (/dystocia-eclampsia-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /dystocia-eclampsia-triage renders seeded operations (DE-OP-101, DE-OP-102, DE-OP-103), KPI telemetry, oxytocin contraindication alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/dystocia-eclampsia-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#dystocia-eclampsia-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Dystocia, Maternal C-Section, Eclampsia (Hypocalcemia) & Oxytocin Contraindication Triage Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-fetal')).toBeVisible();
    await expect(page.locator('#kpi-staged-whelping')).toBeVisible();
    await expect(page.locator('#kpi-delivered-safe')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const penelopeCard = page.locator('#dystocia-operations-list .dystocia-op-card', {
      hasText: 'Penelope (',
    });
    const mapleCard = page.locator('#dystocia-operations-list .dystocia-op-card', {
      hasText: 'Maple (',
    });
    const rosieCard = page.locator('#dystocia-operations-list .dystocia-op-card', {
      hasText: 'Rosie (',
    });

    await expect(penelopeCard).toBeVisible();
    await expect(penelopeCard).toHaveAttribute(
      'data-status',
      'FETAL_DISTRESS_OBSTRUCTIVE_CRITICAL'
    );
    await expect(
      penelopeCard.locator('.badge-dystocia-status.badge-dystocia-critical')
    ).toContainText('FETAL_DISTRESS_OBSTRUCTIVE_CRITICAL');
    await expect(
      penelopeCard.locator('.dystocia-oxytocin-alert-banner')
    ).toContainText('uterine rupture');

    await expect(mapleCard).toBeVisible();
    await expect(mapleCard).toHaveAttribute(
      'data-status',
      'CALCIUM_ECG_WHELPING_STAGED'
    );

    await expect(rosieCard).toBeVisible();
    await expect(rosieCard).toHaveAttribute(
      'data-status',
      'DELIVERED_NORMOCALCEMIC_LACTATING_SAFE'
    );
    await expect(rosieCard.locator('.dystocia-clearance-banner')).toContainText(
      'DYSTOCIA-SAFE-'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Submitting #dystocia-eval-form evaluates a new critical dystocia & eclampsia operation with oxytocin contraindication alert and ECG-monitored calcium protocol', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dystocia-eclampsia-triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#eval-pet-name').fill('Duchess');
    await page.locator('#eval-species').fill('Dog - French Bulldog Dam');
    await page.locator('#eval-weight-kg').fill('11.4');
    await page
      .locator('#eval-presentation-type')
      .selectOption('OBSTRUCTIVE_BRACHYCEPHALIC_FETAL_OVERSIZE');
    await page.locator('#eval-fetal-hr').fill('148');
    await page.locator('#eval-ionized-calcium').fill('0.82');
    await page.locator('#eval-interval-mins').fill('160');
    await page.locator('#eval-uteroverdin').check();
    await page.locator('#eval-obstruction-excluded').uncheck();
    await page.locator('#eval-ecg-calcium').check();

    await page.locator('#btn-submit-dystocia-eval').click();

    const duchessCard = page
      .locator('#dystocia-operations-list .dystocia-op-card', {
        hasText: 'Duchess (',
      })
      .first();
    await expect(duchessCard).toBeVisible();
    await expect(duchessCard).toHaveAttribute(
      'data-status',
      'FETAL_DISTRESS_OBSTRUCTIVE_CRITICAL'
    );
    await expect(
      duchessCard.locator('.dystocia-oxytocin-alert-banner')
    ).toContainText('uterine rupture');
  });

  test('3. Completing all 4 protocol steps on DE-OP-101 transitions status to CALCIUM_ECG_WHELPING_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dystocia-eclampsia-triage`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator(
      '#dystocia-operations-list .dystocia-op-card[data-op-id="DE-OP-101"]'
    );
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-DOPPLER-FETAL-HR-PELVIC-XRAY',
      'STEP-ECG-CALCIUM-GLUCONATE-INFUSION',
      'STEP-OXYTOCIN-GATE-OR-EMERGENCY-CSECTION',
      'STEP-NEONATAL-RESUS-COLOSTRUM-LATCH',
    ]) {
      const cb = card101.locator(`.dystocia-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute(
      'data-status',
      'CALCIUM_ECG_WHELPING_STAGED'
    );
    await expect(
      card101.locator('.badge-dystocia-status.badge-dystocia-staged')
    ).toContainText('CALCIUM_ECG_WHELPING_STAGED');
  });

  test('4. Clicking .dystocia-issue-clearance-btn issues a DYSTOCIA-SAFE-<12HEX> clearance token and transitions operation to DELIVERED_NORMOCALCEMIC_LACTATING_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dystocia-eclampsia-triage`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator(
      '#dystocia-operations-list .dystocia-op-card[data-op-id="DE-OP-102"]'
    );
    await expect(card102).toBeVisible();

    await card102.locator('.dystocia-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute(
      'data-status',
      'DELIVERED_NORMOCALCEMIC_LACTATING_SAFE'
    );
    await expect(
      card102.locator('.badge-dystocia-status.badge-dystocia-safe')
    ).toContainText('DELIVERED_NORMOCALCEMIC_LACTATING_SAFE');
    await expect(card102.locator('.dystocia-clearance-banner')).toContainText(
      'DYSTOCIA-SAFE-'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-dystocia-critical, .badge-dystocia-staged, .badge-dystocia-safe, .badge-dystocia-presentation, .badge-dystocia-flag, .dystocia-oxytocin-alert-banner, and .dystocia-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dystocia-eclampsia-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-dystocia-triage-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-dystocia-critical" class="badge badge-dystocia-status badge-dystocia-critical">FETAL_DISTRESS_OBSTRUCTIVE_CRITICAL</span>
        <span id="test-badge-dystocia-staged" class="badge badge-dystocia-status badge-dystocia-staged">CALCIUM_ECG_WHELPING_STAGED</span>
        <span id="test-badge-dystocia-safe" class="badge badge-dystocia-status badge-dystocia-safe">DELIVERED_NORMOCALCEMIC_LACTATING_SAFE</span>
        <span id="test-badge-dystocia-presentation" class="badge badge-dystocia-presentation">OBSTRUCTIVE_BRACHYCEPHALIC_FETAL_OVERSIZE</span>
        <span id="test-badge-dystocia-flag" class="badge badge-dystocia-flag">OXYTOCIN_CONTRAINDICATED_OBSTRUCTION</span>
        <div id="test-dystocia-oxytocin-alert-banner" class="dystocia-oxytocin-alert-banner">CRITICAL OBSTETRIC CONTRAINDICATION: NEVER administer oxytocin when obstructive dystocia or fetal oversize is present</div>
        <div id="test-dystocia-clearance-banner" class="dystocia-clearance-banner">DELIVERED NORMOCALCEMIC &amp; LACTATING CLEARANCE VERIFIED: DYSTOCIA-SAFE-1A2B3C4D5E6F</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-dystocia-critical',
        '#test-badge-dystocia-staged',
        '#test-badge-dystocia-safe',
        '#test-badge-dystocia-presentation',
        '#test-badge-dystocia-flag',
        '#test-dystocia-oxytocin-alert-banner',
        '#test-dystocia-clearance-banner',
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
