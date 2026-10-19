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

test.describe('Canine Parvovirus, Panleukopenia, Distemper & Biosecurity Isolation Cohort Cockpit UI (/biosecurity-isolation-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /biosecurity-isolation-triage renders seeded operations (BI-OP-101, BI-OP-102, BI-OP-103), KPI telemetry, quat disinfectant failure alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/biosecurity-isolation-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#biosecurity-isolation-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Canine Parvovirus, Panleukopenia, Distemper & Biosecurity Isolation Cohort Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-sepsis')).toBeVisible();
    await expect(page.locator('#kpi-staged-iso')).toBeVisible();
    await expect(page.locator('#kpi-cleared-safe')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const baxterCard = page.locator('#biosec-operations-list .biosec-op-card', {
      hasText: 'Baxter (',
    });
    const mittensCard = page.locator('#biosec-operations-list .biosec-op-card', {
      hasText: 'Mittens (',
    });
    const otisCard = page.locator('#biosec-operations-list .biosec-op-card', {
      hasText: 'Otis (',
    });

    await expect(baxterCard).toBeVisible();
    await expect(baxterCard).toHaveAttribute(
      'data-status',
      'NEUTROPENIC_SEPSIS_BREACH_CRITICAL'
    );
    await expect(baxterCard.locator('.badge-biosec-status.badge-biosec-critical')).toContainText(
      'NEUTROPENIC_SEPSIS_BREACH_CRITICAL'
    );
    await expect(baxterCard.locator('.biosec-quat-alert-banner')).toContainText(
      'quaternary ammonium'
    );

    await expect(mittensCard).toBeVisible();
    await expect(mittensCard).toHaveAttribute(
      'data-status',
      'NEGATIVE_PRESSURE_ISO_COHORT_STAGED'
    );

    await expect(otisCard).toBeVisible();
    await expect(otisCard).toHaveAttribute(
      'data-status',
      'PCR_NEGATIVE_DECON_CLEARED'
    );
    await expect(otisCard.locator('.biosec-clearance-banner')).toContainText('BIOSEC-SAFE-');

    expect(cspErrors).toEqual([]);
  });

  test('2. Submitting #biosec-eval-form evaluates a new critical CPV-2 parvovirus neutropenic sepsis operation with Virkon/AHP disinfectant alert', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/biosecurity-isolation-triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#eval-pet-name').fill('Koda');
    await page.locator('#eval-species').fill('Dog - Doberman Pinscher Puppy');
    await page.locator('#eval-weight-kg').fill('9.8');
    await page.locator('#eval-pathogen-type').selectOption('CANINE_PARVOVIRUS_CPV2_ENTERITIS');
    await page.locator('#eval-neutrophils').fill('540');
    await page.locator('#eval-pressure-pa').fill('-5.5');
    await page.locator('#eval-contact-mins').fill('10');
    await page.locator('#eval-hemorrhagic').check();
    await page.locator('#eval-footbath').check();
    await page.locator('#eval-ppe-enforced').check();

    await page.locator('#btn-submit-biosec-eval').click();

    const kodaCard = page
      .locator('#biosec-operations-list .biosec-op-card', {
        hasText: 'Koda (',
      })
      .first();
    await expect(kodaCard).toBeVisible();
    await expect(kodaCard).toHaveAttribute(
      'data-status',
      'NEUTROPENIC_SEPSIS_BREACH_CRITICAL'
    );
    await expect(kodaCard.locator('.biosec-quat-alert-banner')).toContainText(
      'quaternary ammonium'
    );
  });

  test('3. Completing all 4 protocol steps on BI-OP-101 transitions status to NEGATIVE_PRESSURE_ISO_COHORT_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/biosecurity-isolation-triage`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#biosec-operations-list .biosec-op-card[data-op-id="BI-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-ANTEROOM-PPE-FOOTBATH-LOCK',
      'STEP-AHP-VIRKON-10MIN-DECON',
      'STEP-NASOJEJUNAL-MAROPITANT-SEPSIS',
      'STEP-PCR-SHEDDING-CLEARANCE-BATH',
    ]) {
      const cb = card101.locator(`.biosec-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'NEGATIVE_PRESSURE_ISO_COHORT_STAGED');
    await expect(card101.locator('.badge-biosec-status.badge-biosec-staged')).toContainText(
      'NEGATIVE_PRESSURE_ISO_COHORT_STAGED'
    );
  });

  test('4. Clicking .biosec-issue-clearance-btn issues a BIOSEC-SAFE-<12HEX> clearance token and transitions operation to PCR_NEGATIVE_DECON_CLEARED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/biosecurity-isolation-triage`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#biosec-operations-list .biosec-op-card[data-op-id="BI-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.biosec-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'PCR_NEGATIVE_DECON_CLEARED');
    await expect(card102.locator('.badge-biosec-status.badge-biosec-safe')).toContainText(
      'PCR_NEGATIVE_DECON_CLEARED'
    );
    await expect(card102.locator('.biosec-clearance-banner')).toContainText('BIOSEC-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-biosec-critical, .badge-biosec-staged, .badge-biosec-safe, .badge-biosec-pathogen, .badge-biosec-flag, .biosec-quat-alert-banner, and .biosec-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/biosecurity-isolation-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-biosec-triage-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-biosec-critical" class="badge badge-biosec-status badge-biosec-critical">NEUTROPENIC_SEPSIS_BREACH_CRITICAL</span>
        <span id="test-badge-biosec-staged" class="badge badge-biosec-status badge-biosec-staged">NEGATIVE_PRESSURE_ISO_COHORT_STAGED</span>
        <span id="test-badge-biosec-safe" class="badge badge-biosec-status badge-biosec-safe">PCR_NEGATIVE_DECON_CLEARED</span>
        <span id="test-badge-biosec-pathogen" class="badge badge-biosec-pathogen">CANINE_PARVOVIRUS_CPV2_ENTERITIS</span>
        <span id="test-badge-biosec-flag" class="badge badge-biosec-flag">SEVERE_NEUTROPENIC_LEUKOPENIA</span>
        <div id="test-biosec-quat-alert-banner" class="biosec-quat-alert-banner">BIOSECURITY DISINFECTANT FAILURE ALERT: Non-enveloped parvoviruses CPV-2 and FPV are completely resistant to standard quaternary ammonium disinfectants</div>
        <div id="test-biosec-clearance-banner" class="biosec-clearance-banner">PCR-NEGATIVE SHEDDING &amp; VIRKON DECON CLEARANCE VERIFIED: BIOSEC-SAFE-1A2B3C4D5E6F</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-biosec-critical',
        '#test-badge-biosec-staged',
        '#test-badge-biosec-safe',
        '#test-badge-biosec-pathogen',
        '#test-badge-biosec-flag',
        '#test-biosec-quat-alert-banner',
        '#test-biosec-clearance-banner',
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
