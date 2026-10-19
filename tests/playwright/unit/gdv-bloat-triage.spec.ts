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

test.describe('Gastric Dilatation-Volvulus (GDV / Bloat), Splenic Torsion & Trocar Decompression Triage Cockpit UI (/gdv-bloat-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /gdv-bloat-triage renders seeded operations (GDV-OP-101, GDV-OP-102, GDV-OP-103), KPI telemetry, caudal vena cava alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/gdv-bloat-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#gdv-bloat-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Gastric Dilatation-Volvulus (GDV / Bloat), Splenic Torsion & Trocar Decompression Triage Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-gdv')).toBeVisible();
    await expect(page.locator('#kpi-staged-trocar')).toBeVisible();
    await expect(page.locator('#kpi-cleared-safe')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const titanCard = page.locator('#gdv-operations-list .gdv-op-card', {
      hasText: 'Titan (',
    });
    const baronCard = page.locator('#gdv-operations-list .gdv-op-card', {
      hasText: 'Baron (',
    });
    const duchessCard = page.locator('#gdv-operations-list .gdv-op-card', {
      hasText: 'Duchess (',
    });

    await expect(titanCard).toBeVisible();
    await expect(titanCard).toHaveAttribute(
      'data-status',
      'GASTRIC_NECROSIS_VPC_SHOCK_CRITICAL'
    );
    await expect(titanCard.locator('.badge-gdv-status.badge-gdv-critical')).toContainText(
      'GASTRIC_NECROSIS_VPC_SHOCK_CRITICAL'
    );
    await expect(titanCard.locator('.gdv-venacava-alert-banner')).toContainText('caudal vena cava');

    await expect(baronCard).toBeVisible();
    await expect(baronCard).toHaveAttribute(
      'data-status',
      'TROCAR_DECOMPRESSED_DUAL_IV_STAGED'
    );

    await expect(duchessCard).toBeVisible();
    await expect(duchessCard).toHaveAttribute(
      'data-status',
      'GASTROPEXY_SINUS_RHYTHM_CLEARED'
    );
    await expect(duchessCard.locator('.gdv-clearance-banner')).toContainText('GDV-SAFE-');

    expect(cspErrors).toEqual([]);
  });

  test('2. Submitting #gdv-eval-form evaluates a new critical GDV / bloat operation with caudal vena cava alert and trocar decompression regimen', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/gdv-bloat-triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#eval-pet-name').fill('Goliath');
    await page.locator('#eval-species').fill('Dog - German Shepherd');
    await page.locator('#eval-weight-kg').fill('42.0');
    await page.locator('#eval-presentation-type').selectOption('ACUTE_GDV_360_PYLORIC_TORSION');
    await page.locator('#eval-lactate-mmol').fill('8.6');
    await page.locator('#eval-heart-rate').fill('204');
    await page.locator('#eval-girth-cm').fill('116.0');
    await page.locator('#eval-retching').check();
    await page.locator('#eval-cranial-iv').check();
    await page.locator('#eval-vpcs').check();

    await page.locator('#btn-submit-gdv-eval').click();

    const goliathCard = page
      .locator('#gdv-operations-list .gdv-op-card', {
        hasText: 'Goliath (',
      })
      .first();
    await expect(goliathCard).toBeVisible();
    await expect(goliathCard).toHaveAttribute(
      'data-status',
      'GASTRIC_NECROSIS_VPC_SHOCK_CRITICAL'
    );
    await expect(goliathCard.locator('.gdv-venacava-alert-banner')).toContainText('caudal vena cava');
  });

  test('3. Completing all 4 protocol steps on GDV-OP-101 transitions status to TROCAR_DECOMPRESSED_DUAL_IV_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/gdv-bloat-triage`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#gdv-operations-list .gdv-op-card[data-op-id="GDV-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-BILATERAL-CEPHALIC-IV-BOLUS',
      'STEP-TROCAR-OROGASTRIC-DECOMPRESS',
      'STEP-RIGHT-LATERAL-DOUBLE-BUBBLE-XRAY',
      'STEP-LIDOCAINE-VPC-GASTROPEXY-PREP',
    ]) {
      const cb = card101.locator(`.gdv-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'TROCAR_DECOMPRESSED_DUAL_IV_STAGED');
    await expect(card101.locator('.badge-gdv-status.badge-gdv-staged')).toContainText(
      'TROCAR_DECOMPRESSED_DUAL_IV_STAGED'
    );
  });

  test('4. Clicking .gdv-issue-clearance-btn issues a GDV-SAFE-<12HEX> clearance token and transitions operation to GASTROPEXY_SINUS_RHYTHM_CLEARED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/gdv-bloat-triage`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#gdv-operations-list .gdv-op-card[data-op-id="GDV-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.gdv-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'GASTROPEXY_SINUS_RHYTHM_CLEARED');
    await expect(card102.locator('.badge-gdv-status.badge-gdv-safe')).toContainText(
      'GASTROPEXY_SINUS_RHYTHM_CLEARED'
    );
    await expect(card102.locator('.gdv-clearance-banner')).toContainText('GDV-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-gdv-critical, .badge-gdv-staged, .badge-gdv-safe, .badge-gdv-presentation, .badge-gdv-flag, .gdv-venacava-alert-banner, and .gdv-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/gdv-bloat-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-gdv-triage-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-gdv-critical" class="badge badge-gdv-status badge-gdv-critical">GASTRIC_NECROSIS_VPC_SHOCK_CRITICAL</span>
        <span id="test-badge-gdv-staged" class="badge badge-gdv-status badge-gdv-staged">TROCAR_DECOMPRESSED_DUAL_IV_STAGED</span>
        <span id="test-badge-gdv-safe" class="badge badge-gdv-status badge-gdv-safe">GASTROPEXY_SINUS_RHYTHM_CLEARED</span>
        <span id="test-badge-gdv-presentation" class="badge badge-gdv-presentation">ACUTE_GDV_360_PYLORIC_TORSION</span>
        <span id="test-badge-gdv-flag" class="badge badge-gdv-flag">GASTRIC_NECROSIS_HYPERLACTATEMIA</span>
        <div id="test-gdv-venacava-alert-banner" class="gdv-venacava-alert-banner">CRITICAL VASCULAR &amp; RADIOGRAPHIC ALERT: NEVER rely on saphenous or caudal IV catheters in GDV</div>
        <div id="test-gdv-clearance-banner" class="gdv-clearance-banner">GASTROPEXY &amp; SINUS RHYTHM CLEARANCE VERIFIED: GDV-SAFE-1A2B3C4D5E6F</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-gdv-critical',
        '#test-badge-gdv-staged',
        '#test-badge-gdv-safe',
        '#test-badge-gdv-presentation',
        '#test-badge-gdv-flag',
        '#test-gdv-venacava-alert-banner',
        '#test-gdv-clearance-banner',
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
