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

test.describe('Feline Urethral Obstruction (Blocked Tom), Hyperkalemic Bradycardia & Calcium Gluconate Cardioprotection Cockpit UI (/urethral-obstruction-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /urethral-obstruction-triage renders seeded operations (UO-OP-101, UO-OP-102, UO-OP-103), KPI telemetry, cardioprotection alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/urethral-obstruction-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#urethral-obstruction-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Feline Urethral Obstruction (Blocked Tom), Hyperkalemic Bradycardia & Calcium Gluconate Cardioprotection Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-hyperk')).toBeVisible();
    await expect(page.locator('#kpi-staged-unblock')).toBeVisible();
    await expect(page.locator('#kpi-cleared-safe')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const sylvesterCard = page.locator('#urethral-operations-list .urethral-op-card', {
      hasText: 'Sylvester (',
    });
    const figaroCard = page.locator('#urethral-operations-list .urethral-op-card', {
      hasText: 'Figaro (',
    });
    const chesterCard = page.locator('#urethral-operations-list .urethral-op-card', {
      hasText: 'Chester (',
    });

    await expect(sylvesterCard).toBeVisible();
    await expect(sylvesterCard).toHaveAttribute(
      'data-status',
      'HYPERKALEMIC_BRADYCARDIA_ATRIAL_STANDSTILL_CRITICAL'
    );
    await expect(sylvesterCard.locator('.badge-urethral-status.badge-urethral-critical')).toContainText(
      'HYPERKALEMIC_BRADYCARDIA_ATRIAL_STANDSTILL_CRITICAL'
    );
    await expect(sylvesterCard.locator('.urethral-cardioprotect-alert-banner')).toContainText('Calcium Gluconate');

    await expect(figaroCard).toBeVisible();
    await expect(figaroCard).toHaveAttribute(
      'data-status',
      'CALCIUM_CARDIOPROTECT_RETROHYDROPROPULSION_STAGED'
    );

    await expect(chesterCard).toBeVisible();
    await expect(chesterCard).toHaveAttribute(
      'data-status',
      'INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED'
    );
    await expect(chesterCard.locator('.urethral-clearance-banner')).toContainText('URETHRAL-SAFE-');

    expect(cspErrors).toEqual([]);
  });

  test('2. Submitting #urethral-eval-form evaluates a new critical urethral obstruction operation with calcium gluconate cardioprotection alert and unblocking protocol', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/urethral-obstruction-triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#eval-pet-name').fill('Garfield');
    await page.locator('#eval-species').fill('Cat - Persian Neutered Male');
    await page.locator('#eval-weight-kg').fill('6.4');
    await page.locator('#eval-obstruction-cause').selectOption('STRUWITE_CRYSTALLINE_MATRIX_PLUG');
    await page.locator('#eval-potassium-mmol').fill('9.4');
    await page.locator('#eval-heart-rate').fill('112');
    await page.locator('#eval-bladder-cm').fill('10.2');
    await page.locator('#eval-absent-pwave').check();
    await page.locator('#eval-calcium-before-sedation').check();
    await page.locator('#eval-manual-expression-avoided').check();

    await page.locator('#btn-submit-urethral-eval').click();

    const garfieldCard = page
      .locator('#urethral-operations-list .urethral-op-card', {
        hasText: 'Garfield (',
      })
      .first();
    await expect(garfieldCard).toBeVisible();
    await expect(garfieldCard).toHaveAttribute(
      'data-status',
      'HYPERKALEMIC_BRADYCARDIA_ATRIAL_STANDSTILL_CRITICAL'
    );
    await expect(garfieldCard.locator('.urethral-cardioprotect-alert-banner')).toContainText('Calcium Gluconate');
  });

  test('3. Completing all 4 protocol steps on UO-OP-101 transitions status to CALCIUM_CARDIOPROTECT_RETROHYDROPROPULSION_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/urethral-obstruction-triage`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#urethral-operations-list .urethral-op-card[data-op-id="UO-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-CALCIUM-GLUCONATE-INSULIN-DEXTROSE',
      'STEP-SACROCOCCYGEAL-BLOCK-RETROHYDROPROPULSION',
      'STEP-CLOSED-COLLECTION-POST-OBSTRUCTIVE-DIURESIS',
      'STEP-PRAZOSIN-NORMO-K-CLEARANCE',
    ]) {
      const cb = card101.locator(`.urethral-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'CALCIUM_CARDIOPROTECT_RETROHYDROPROPULSION_STAGED');
    await expect(card101.locator('.badge-urethral-status.badge-urethral-staged')).toContainText(
      'CALCIUM_CARDIOPROTECT_RETROHYDROPROPULSION_STAGED'
    );
  });

  test('4. Clicking .urethral-issue-clearance-btn issues a URETHRAL-SAFE-<12HEX> clearance token and transitions operation to INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/urethral-obstruction-triage`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#urethral-operations-list .urethral-op-card[data-op-id="UO-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.urethral-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED');
    await expect(card102.locator('.badge-urethral-status.badge-urethral-safe')).toContainText(
      'INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED'
    );
    await expect(card102.locator('.urethral-clearance-banner')).toContainText('URETHRAL-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-urethral-critical, .badge-urethral-staged, .badge-urethral-safe, .badge-urethral-cause, .badge-urethral-flag, .urethral-cardioprotect-alert-banner, and .urethral-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/urethral-obstruction-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-urethral-triage-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-urethral-critical" class="badge badge-urethral-status badge-urethral-critical">HYPERKALEMIC_BRADYCARDIA_ATRIAL_STANDSTILL_CRITICAL</span>
        <span id="test-badge-urethral-staged" class="badge badge-urethral-status badge-urethral-staged">CALCIUM_CARDIOPROTECT_RETROHYDROPROPULSION_STAGED</span>
        <span id="test-badge-urethral-safe" class="badge badge-urethral-status badge-urethral-safe">INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED</span>
        <span id="test-badge-urethral-cause" class="badge badge-urethral-cause">STRUWITE_CRYSTALLINE_MATRIX_PLUG</span>
        <span id="test-badge-urethral-flag" class="badge badge-urethral-flag">ABSENT_PWAVE_ATRIAL_STANDSTILL</span>
        <div id="test-urethral-cardioprotect-alert-banner" class="urethral-cardioprotect-alert-banner">CRITICAL CARDIOPROTECTION &amp; UROABDOMEN ALERT: NEVER induce general anesthesia or heavy sedation before 10% Calcium Gluconate IV</div>
        <div id="test-urethral-clearance-banner" class="urethral-clearance-banner">INDWELLING CLOSED DIURESIS &amp; NORMOKALEMIC CLEARANCE VERIFIED: URETHRAL-SAFE-1A2B3C4D5E6F</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-urethral-critical',
        '#test-badge-urethral-staged',
        '#test-badge-urethral-safe',
        '#test-badge-urethral-cause',
        '#test-badge-urethral-flag',
        '#test-urethral-cardioprotect-alert-banner',
        '#test-urethral-clearance-banner',
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
