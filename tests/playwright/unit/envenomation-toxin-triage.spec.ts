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

test.describe('Pit-Viper Envenomation, Bufo Toad Bufotoxin, Porcupine Quill & Toxic Flora Triage Cockpit UI (/envenomation-toxin-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /envenomation-toxin-triage renders seeded operations (ET-OP-101, ET-OP-102, ET-OP-103), KPI telemetry, contraindication warning banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/envenomation-toxin-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#envenomation-toxin-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Pit-Viper Envenomation, Bufo Toad Bufotoxin, Porcupine Quill & Toxic Flora Triage Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-toxin')).toBeVisible();
    await expect(page.locator('#kpi-staged-antivenom')).toBeVisible();
    await expect(page.locator('#kpi-stabilized-safe')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const copperCard = page.locator('#toxin-operations-list .toxin-op-card', {
      hasText: 'Copper (',
    });
    const brambleCard = page.locator('#toxin-operations-list .toxin-op-card', {
      hasText: 'Bramble (',
    });
    const mangoCard = page.locator('#toxin-operations-list .toxin-op-card', {
      hasText: 'Mango (',
    });

    await expect(copperCard).toBeVisible();
    await expect(copperCard).toHaveAttribute(
      'data-status',
      'COAGULOPATHY_NEUROTOXIC_CRITICAL'
    );
    await expect(copperCard.locator('.badge-toxin-status.badge-toxin-critical')).toContainText(
      'COAGULOPATHY_NEUROTOXIC_CRITICAL'
    );
    await expect(copperCard.locator('.toxin-contraindication-banner')).toContainText(
      'NEVER cut porcupine quills'
    );

    await expect(brambleCard).toBeVisible();
    await expect(brambleCard).toHaveAttribute(
      'data-status',
      'ANTIVENOM_LAVAGE_SEDATION_STAGED'
    );

    await expect(mangoCard).toBeVisible();
    await expect(mangoCard).toHaveAttribute(
      'data-status',
      'STABILIZED_NEUTRALIZED_CLEARED'
    );
    await expect(mangoCard.locator('.toxin-clearance-banner')).toContainText('TOXIN-SAFE-');

    expect(cspErrors).toEqual([]);
  });

  test('2. Submitting #toxin-eval-form evaluates a new critical pit-viper envenomation operation with echinocytosis and antivenom staging', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/envenomation-toxin-triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#eval-pet-name').fill('Sidewinder');
    await page.locator('#eval-species').fill('Dog - Catahoula Leopard Dog');
    await page.locator('#eval-weight-kg').fill('24.8');
    await page.locator('#eval-toxin-type').selectOption('PIT_VIPER_CROTALID_HEI');
    await page.locator('#eval-edema-spread').fill('7.2');
    await page.locator('#eval-echinocyte-pct').fill('42.0');
    await page.locator('#eval-quill-count').fill('0');
    await page.locator('#eval-antivenom-vials').fill('3');
    await page.locator('#eval-oral-flushed').check();
    await page.locator('#eval-quills-cut-avoided').check();

    await page.locator('#btn-submit-toxin-eval').click();

    const sidewinderCard = page
      .locator('#toxin-operations-list .toxin-op-card', {
        hasText: 'Sidewinder (',
      })
      .first();
    await expect(sidewinderCard).toBeVisible();
    await expect(sidewinderCard).toHaveAttribute(
      'data-status',
      'COAGULOPATHY_NEUROTOXIC_CRITICAL'
    );
    await expect(sidewinderCard.locator('.toxin-contraindication-banner')).toContainText(
      'NEVER cut porcupine quills'
    );
  });

  test('3. Completing all 4 protocol steps on ET-OP-101 transitions status to ANTIVENOM_LAVAGE_SEDATION_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/envenomation-toxin-triage`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#toxin-operations-list .toxin-op-card[data-op-id="ET-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-DEMARCATION-PERIMETER-MARK',
      'STEP-ORAL-FORWARD-LAVAGE-ANTIVENOM',
      'STEP-SEDATED-QUILL-EXTRACTION',
      'STEP-COAG-ECHINOCYTE-RECHECK',
    ]) {
      const cb = card101.locator(`.toxin-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'ANTIVENOM_LAVAGE_SEDATION_STAGED');
    await expect(card101.locator('.badge-toxin-status.badge-toxin-staged')).toContainText(
      'ANTIVENOM_LAVAGE_SEDATION_STAGED'
    );
  });

  test('4. Clicking .toxin-issue-clearance-btn issues a TOXIN-SAFE-<12HEX> clearance token and transitions operation to STABILIZED_NEUTRALIZED_CLEARED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/envenomation-toxin-triage`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#toxin-operations-list .toxin-op-card[data-op-id="ET-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.toxin-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'STABILIZED_NEUTRALIZED_CLEARED');
    await expect(card102.locator('.badge-toxin-status.badge-toxin-safe')).toContainText(
      'STABILIZED_NEUTRALIZED_CLEARED'
    );
    await expect(card102.locator('.toxin-clearance-banner')).toContainText('TOXIN-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-toxin-critical, .badge-toxin-staged, .badge-toxin-safe, .badge-toxin-type, .badge-toxin-flag, .toxin-contraindication-banner, and .toxin-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/envenomation-toxin-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-toxin-triage-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-toxin-critical" class="badge badge-toxin-status badge-toxin-critical">COAGULOPATHY_NEUROTOXIC_CRITICAL</span>
        <span id="test-badge-toxin-staged" class="badge badge-toxin-status badge-toxin-staged">ANTIVENOM_LAVAGE_SEDATION_STAGED</span>
        <span id="test-badge-toxin-safe" class="badge badge-toxin-status badge-toxin-safe">STABILIZED_NEUTRALIZED_CLEARED</span>
        <span id="test-badge-toxin-type" class="badge badge-toxin-type">PIT_VIPER_CROTALID_HEI</span>
        <span id="test-badge-toxin-flag" class="badge badge-toxin-flag">SEVERE_ECHINOCYTOSIS_COAGULOPATHY</span>
        <div id="test-toxin-contraindication-banner" class="toxin-contraindication-banner">CRITICAL CONTRAINDICATIONS: NEVER cut porcupine quills prior to extraction</div>
        <div id="test-toxin-clearance-banner" class="toxin-clearance-banner">ENVENOMATION &amp; TOXIN NEUTRALIZATION CLEARANCE VERIFIED: TOXIN-SAFE-1A2B3C4D5E6F</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-toxin-critical',
        '#test-badge-toxin-staged',
        '#test-badge-toxin-safe',
        '#test-badge-toxin-type',
        '#test-badge-toxin-flag',
        '#test-toxin-contraindication-banner',
        '#test-toxin-clearance-banner',
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
