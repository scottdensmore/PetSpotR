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

test.describe('Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing & Acute Hemolytic Transfusion Reaction Cockpit UI (/transfusion-bloodbank-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /transfusion-bloodbank-triage renders seeded operations (TB-OP-101, TB-OP-102, TB-OP-103), KPI telemetry, alloantibody/calcium alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/transfusion-bloodbank-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#transfusion-bloodbank-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing & Acute Hemolytic Transfusion Reaction Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-hemolytic')).toBeVisible();
    await expect(page.locator('#kpi-staged-crossmatch')).toBeVisible();
    await expect(page.locator('#kpi-cleared-safe')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const saffronCard = page.locator('#bloodbank-operations-list .bloodbank-op-card', {
      hasText: 'Saffron (',
    });
    const rangerCard = page.locator('#bloodbank-operations-list .bloodbank-op-card', {
      hasText: 'Ranger (',
    });
    const cleoCard = page.locator('#bloodbank-operations-list .bloodbank-op-card', {
      hasText: 'Cleo (',
    });

    await expect(saffronCard).toBeVisible();
    await expect(saffronCard).toHaveAttribute(
      'data-status',
      'ALLOANTIBODY_HEMOLYTIC_ANAPHYLAXIS_CRITICAL'
    );
    await expect(saffronCard.locator('.badge-bloodbank-status.badge-bloodbank-critical')).toContainText(
      'ALLOANTIBODY_HEMOLYTIC_ANAPHYLAXIS_CRITICAL'
    );
    await expect(saffronCard.locator('.bloodbank-alloantibody-alert-banner')).toContainText('alloantibodies');

    await expect(rangerCard).toBeVisible();
    await expect(rangerCard).toHaveAttribute(
      'data-status',
      'CROSSMATCHED_MICRON_FILTER_RATE_STAGED'
    );

    await expect(cleoCard).toBeVisible();
    await expect(cleoCard).toHaveAttribute(
      'data-status',
      'TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE'
    );
    await expect(cleoCard.locator('.bloodbank-clearance-banner')).toContainText('BLOODBANK-SAFE-');

    expect(cspErrors).toEqual([]);
  });

  test('2. Submitting #bloodbank-eval-form evaluates a new critical Feline Type B / Xenotransfusion operation with alloantibody/calcium alert and transfusion regimen', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/transfusion-bloodbank-triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#eval-pet-name').fill('Bramble');
    await page.locator('#eval-species').fill('Cat - British Shorthair');
    await page.locator('#eval-weight-kg').fill('4.1');
    await page.locator('#eval-blood-group').selectOption('FELINE_TYPE_B_ALLOANTIBODY_RISK');
    await page.locator('#eval-recipient-pcv').fill('9.5');
    await page.locator('#eval-donor-pcv').fill('42.0');
    await page.locator('#eval-target-pcv').fill('24.0');
    await page.locator('#eval-crossmatch-ok').check();
    await page.locator('#eval-micron-filter').check();
    await page.locator('#eval-lrs-avoided').check();

    await page.locator('#btn-submit-bloodbank-eval').click();

    const brambleCard = page
      .locator('#bloodbank-operations-list .bloodbank-op-card', {
        hasText: 'Bramble (',
      })
      .first();
    await expect(brambleCard).toBeVisible();
    await expect(brambleCard).toHaveAttribute(
      'data-status',
      'ALLOANTIBODY_HEMOLYTIC_ANAPHYLAXIS_CRITICAL'
    );
    await expect(brambleCard.locator('.bloodbank-alloantibody-alert-banner')).toContainText('alloantibodies');
  });

  test('3. Completing all 4 protocol steps on TB-OP-101 transitions status to CROSSMATCHED_MICRON_FILTER_RATE_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/transfusion-bloodbank-triage`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#bloodbank-operations-list .bloodbank-op-card[data-op-id="TB-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-AB-DEA11-TYPING-SLIDE-CROSSMATCH',
      'STEP-170MICRON-FILTER-09NACL-ONLY-LINE',
      'STEP-SLOW-025MLKG-15MIN-BIOLOGIC-TRIAL',
      'STEP-POST-PCV-TOTAL-SOLIDS-HEMOLYSIS-CHECK',
    ]) {
      const cb = card101.locator(`.bloodbank-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'CROSSMATCHED_MICRON_FILTER_RATE_STAGED');
    await expect(card101.locator('.badge-bloodbank-status.badge-bloodbank-staged')).toContainText(
      'CROSSMATCHED_MICRON_FILTER_RATE_STAGED'
    );
  });

  test('4. Clicking .bloodbank-issue-clearance-btn issues a BLOODBANK-SAFE-<12HEX> clearance token and transitions operation to TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/transfusion-bloodbank-triage`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#bloodbank-operations-list .bloodbank-op-card[data-op-id="TB-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.bloodbank-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE');
    await expect(card102.locator('.badge-bloodbank-status.badge-bloodbank-safe')).toContainText(
      'TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE'
    );
    await expect(card102.locator('.bloodbank-clearance-banner')).toContainText('BLOODBANK-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-bloodbank-critical, .badge-bloodbank-staged, .badge-bloodbank-safe, .badge-bloodbank-group, .badge-bloodbank-flag, .bloodbank-alloantibody-alert-banner, and .bloodbank-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/transfusion-bloodbank-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-bloodbank-triage-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-bloodbank-critical" class="badge badge-bloodbank-status badge-bloodbank-critical">ALLOANTIBODY_HEMOLYTIC_ANAPHYLAXIS_CRITICAL</span>
        <span id="test-badge-bloodbank-staged" class="badge badge-bloodbank-status badge-bloodbank-staged">CROSSMATCHED_MICRON_FILTER_RATE_STAGED</span>
        <span id="test-badge-bloodbank-safe" class="badge badge-bloodbank-status badge-bloodbank-safe">TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE</span>
        <span id="test-badge-bloodbank-group" class="badge badge-bloodbank-group">FELINE_TYPE_B_ALLOANTIBODY_RISK</span>
        <span id="test-badge-bloodbank-flag" class="badge badge-bloodbank-flag">NATURAL_ANTI_A_ALLOANTIBODY_HAZARD</span>
        <div id="test-bloodbank-alloantibody-alert-banner" class="bloodbank-alloantibody-alert-banner">CRITICAL IMMUNOHEMATOLOGY &amp; COAGULATION ALERT: Cats possess potent naturally occurring anti-A and anti-B alloantibodies</div>
        <div id="test-bloodbank-clearance-banner" class="bloodbank-clearance-banner">TRANSFUSION PCV RESTORATION &amp; NORMOTHERMIC CLEARANCE VERIFIED: BLOODBANK-SAFE-1A2B3C4D5E6F</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-bloodbank-critical',
        '#test-badge-bloodbank-staged',
        '#test-badge-bloodbank-safe',
        '#test-badge-bloodbank-group',
        '#test-badge-bloodbank-flag',
        '#test-bloodbank-alloantibody-alert-banner',
        '#test-bloodbank-clearance-banner',
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
