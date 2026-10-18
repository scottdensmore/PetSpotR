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

test.describe('Neonatal Orphan Litter & Incubator Critical Care Triage Cockpit UI (/neonatal-incubator-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /neonatal-incubator-triage renders seeded operations (NI-OP-101, NI-OP-102, NI-OP-103), KPI telemetry, filters, active workspace, and paralytic ileus warming gate note with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/neonatal-incubator-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#neonatal-incubator-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Neonatal Orphan Litter & Incubator Critical Care Triage Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-fading')).toBeVisible();
    await expect(page.locator('#kpi-staged-incubator')).toBeVisible();
    await expect(page.locator('#kpi-stabilized-safe')).toBeVisible();

    await expect(page.locator('#filter-litter-type')).toBeVisible();
    await expect(page.locator('#filter-status')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const card101 = page.locator('#neonatal-operations-list .neonatal-op-card', {
      hasText: 'Clover Litter (',
    });
    const card102 = page.locator('#neonatal-operations-list .neonatal-op-card', {
      hasText: 'Pippin (',
    });
    const card103 = page.locator('#neonatal-operations-list .neonatal-op-card', {
      hasText: 'Buttercup Litter (',
    });

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-status', 'FADING_TRIAD_HYPOTHERMIC_CRITICAL');
    await expect(card101.locator('.badge-ni-status.badge-ni-critical')).toContainText(
      'FADING_TRIAD_HYPOTHERMIC_CRITICAL'
    );
    await expect(card101.locator('.ni-ileus-gate-banner')).toContainText('36.1°C');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-status', 'INCUBATOR_WARMED_DEXTROSE_STAGED');
    await expect(card102.locator('.badge-ni-status.badge-ni-staged')).toContainText(
      'INCUBATOR_WARMED_DEXTROSE_STAGED'
    );

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-status', 'STABILIZED_NURSING_GAINING_SAFE');
    await expect(card103.locator('.badge-ni-status.badge-ni-safe')).toContainText(
      'STABILIZED_NURSING_GAINING_SAFE'
    );
    await expect(card103.locator('.ni-clearance-banner')).toContainText('NEONATAL-SAFE-');

    expect(cspErrors).toEqual([]);
  });

  test('2. Submitting #neonatal-eval-form with invalid weight displays #eval-error-banner, and valid cleft-palate puppy litter creates a FADING_TRIAD_HYPOTHERMIC_CRITICAL operation', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/neonatal-incubator-triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#eval-pet-name').fill('');
    await page.locator('#btn-evaluate-neonatal').click();
    await expect(page.locator('#eval-error-banner')).toBeVisible();

    await page.locator('.ni-scenario-btn[data-scenario="cleft-palate-gavage"]').click();

    const simCard = page
      .locator('#neonatal-operations-list .neonatal-op-card', {
        hasText: 'Barnaby Litter (',
      })
      .first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-status', 'FADING_TRIAD_HYPOTHERMIC_CRITICAL');
    await expect(simCard.locator('.badge-ni-litter').first()).toContainText(
      'NEONATAL_PUPPY_LITTER_CLEFT'
    );
  });

  test('3. Toggling .ni-step-checkbox on NI-OP-101 completes incubator warming gate and transitions status to INCUBATOR_WARMED_DEXTROSE_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/neonatal-incubator-triage`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator(
      '#neonatal-operations-list .neonatal-op-card[data-op-id="NI-OP-101"]'
    );
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-INCUBATOR-WARMING-GATE',
      'STEP-SUBLINGUAL-DEXTROSE-50',
      'STEP-OROGASTRIC-TUBE-GAVAGE',
      'STEP-ANOGENITAL-STIM-WEIGH',
    ]) {
      const cb = card101.locator(`.ni-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'INCUBATOR_WARMED_DEXTROSE_STAGED');
    await expect(card101.locator('.badge-ni-status.badge-ni-staged')).toContainText(
      'INCUBATOR_WARMED_DEXTROSE_STAGED'
    );
  });

  test('4. Clicking .ni-issue-clearance-btn on NI-OP-102 issues a NEONATAL-SAFE-<12HEX> token and transitions operation to STABILIZED_NURSING_GAINING_SAFE', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/neonatal-incubator-triage`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator(
      '#neonatal-operations-list .neonatal-op-card[data-op-id="NI-OP-102"]'
    );
    await expect(card102).toBeVisible();

    await card102.locator('.ni-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'STABILIZED_NURSING_GAINING_SAFE');
    await expect(card102.locator('.badge-ni-status.badge-ni-safe')).toContainText(
      'STABILIZED_NURSING_GAINING_SAFE'
    );
    await expect(card102.locator('.ni-clearance-banner')).toBeVisible();
    await expect(card102.locator('.ni-clearance-banner')).toContainText(
      'NEONATAL NURSING & WEIGHT-GAIN CLEARANCE VERIFIED: NEONATAL-SAFE-'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-ni-critical, .badge-ni-staged, .badge-ni-safe, .badge-ni-litter, .ni-ileus-gate-banner, and .ni-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/neonatal-incubator-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-neonatal-triage-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-ni-critical" class="badge badge-ni-status badge-ni-critical">FADING_TRIAD_HYPOTHERMIC_CRITICAL</span>
        <span id="test-badge-ni-staged" class="badge badge-ni-status badge-ni-staged">INCUBATOR_WARMED_DEXTROSE_STAGED</span>
        <span id="test-badge-ni-safe" class="badge badge-ni-status badge-ni-safe">STABILIZED_NURSING_GAINING_SAFE</span>
        <span id="test-badge-ni-litter" class="badge badge-ni-litter">UNWEANED_KITTEN_LITTER_0_2WK</span>
        <div id="test-ni-ileus-gate-banner" class="ni-ileus-gate-banner">CRITICAL PARALYTIC ILEUS GATE: Never formula-feed below 36.1°C / 97.0°F</div>
        <div id="test-ni-clearance-banner" class="ni-clearance-banner">NEONATAL NURSING &amp; WEIGHT-GAIN CLEARANCE VERIFIED: NEONATAL-SAFE-A1B2C3D4E5F6</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-ni-critical',
        '#test-badge-ni-staged',
        '#test-badge-ni-safe',
        '#test-badge-ni-litter',
        '#test-ni-ileus-gate-banner',
        '#test-ni-clearance-banner',
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
