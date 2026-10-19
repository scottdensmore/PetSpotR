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

test.describe('Acute Pyometra (Open vs. Closed Cervix), POCUS Fluid-Filled Uterine Horns, E. coli Endotoxemia/SIRS, Stabilization & Ovariohysterectomy (OHE) Cockpit UI (/pyometra-sirs-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /pyometra-sirs-triage renders seeded operations (PYO-OP-101, PYO-OP-102, PYO-OP-103), Closed-Cervix Rupture & Blind Cystocentesis Contraindication alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/pyometra-sirs-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#pyometra-sirs-cockpit')).toBeVisible();
    await expect(page.locator('#pyometra-alert-banner')).toBeVisible();
    await expect(page.locator('#pyometra-alert-banner')).toContainText('Closed-Cervix Uterine Rupture Risk');
    await expect(page.locator('#pyometra-alert-banner')).toContainText('Blind Percutaneous Cystocentesis');
    await expect(page.locator('#pyometra-alert-banner')).toContainText('E. coli Lipopolysaccharide Endotoxin');
    await expect(page.locator('#pyometra-alert-banner')).toContainText('Enrofloxacin/Ampicillin-Sulbactam');

    await expect(page.locator('#pyometra-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Bella');
    await expect(page.locator('#active-uterine-diameter')).toContainText('5.4 cm');
    await expect(page.locator('#active-blood-lactate')).toContainText('4.8 mmol/L');

    const bellaCard = page.locator('#pyometra-operations-grid .pyometra-op-card[data-op-id="PYO-OP-101"]');
    const duchessCard = page.locator('#pyometra-operations-grid .pyometra-op-card[data-op-id="PYO-OP-102"]');
    const daisyCard = page.locator('#pyometra-operations-grid .pyometra-op-card[data-op-id="PYO-OP-103"]');

    await expect(bellaCard).toBeVisible();
    await expect(bellaCard).toHaveAttribute('data-status', 'CLOSED_CERVIX_ENDOTOXIC_SIRS_TRIAGE');

    await expect(duchessCard).toBeVisible();
    await expect(duchessCard).toHaveAttribute('data-status', 'RESUSCITATED_EMPIRIC_ABX_OHE_STAGED');

    await expect(daisyCard).toBeVisible();
    await expect(daisyCard).toHaveAttribute('data-status', 'POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED');

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging IV Crystalloid Resuscitation, Empiric Enrofloxacin/Ampicillin-Sulbactam & Emergent OHE protocol on PYO-OP-101 transitions status to RESUSCITATED_EMPIRIC_ABX_OHE_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pyometra-sirs-triage`);
    await page.waitForLoadState('networkidle');

    const bellaCard = page.locator('#pyometra-operations-grid .pyometra-op-card[data-op-id="PYO-OP-101"]');
    await bellaCard.click();

    await expect(page.locator('#active-op-title')).toContainText('PYO-OP-101');
    await page.locator('#stage-ohe-btn').click();

    await expect(bellaCard).toHaveAttribute('data-status', 'RESUSCITATED_EMPIRIC_ABX_OHE_STAGED');
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'RESUSCITATED_EMPIRIC_ABX_OHE_STAGED'
    );
  });

  test('3. Recording a serial perioperative hemodynamic & lactate check on PYO-OP-101 updates #active-uterine-diameter, #active-blood-lactate, and appends to #hemodynamic-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pyometra-sirs-triage`);
    await page.waitForLoadState('networkidle');

    const bellaCard = page.locator('#pyometra-operations-grid .pyometra-op-card[data-op-id="PYO-OP-101"]');
    await bellaCard.click();

    await page.locator('#check-horn-diameter-input').fill('0.0');
    await page.locator('#check-lactate-input').fill('1.6');
    await page.locator('#check-wbc-input').fill('14.2');
    await page.locator('#check-bands-input').fill('2.0');
    await page.locator('#check-map-input').fill('84');
    await page.locator('#check-recorded-by-input').fill('Dr. Hannah Lin, DACVS');
    await page
      .locator('#check-note-input')
      .fill('Post-OHE ICU telemetry: distended uterine horns excised (0.0 cm), lactate cleared to 1.6 mmol/L, band neutrophilia resolved to 2.0%, MAP 84 mmHg.');
    await page.locator('#record-hemodynamic-check-btn').click();

    await expect(page.locator('#active-uterine-diameter')).toContainText('0.0 cm');
    await expect(page.locator('#active-blood-lactate')).toContainText('1.6 mmol/L');
    await expect(page.locator('#hemodynamic-checks-list')).toContainText(
      'Post-OHE ICU telemetry: distended uterine horns excised (0.0 cm), lactate cleared to 1.6 mmol/L'
    );
  });

  test('4. Clearing PYO-OP-101 for post-op OHE discharge transitions status to POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED and reveals #pyometra-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pyometra-sirs-triage`);
    await page.waitForLoadState('networkidle');

    const bellaCard = page.locator('#pyometra-operations-grid .pyometra-op-card[data-op-id="PYO-OP-101"]');
    await bellaCard.click();

    await page.locator('#clear-discharged-by-input').fill('Dr. Hannah Lin, DACVS');
    await page
      .locator('#clear-summary-input')
      .fill('Emergent Ovariohysterectomy (OHE) completed without peritoneal contamination; normotensive (MAP 84 mmHg), afebrile, lactate 1.6 mmol/L, discharged on oral Amoxicillin-Clavulanate + Enrofloxacin.');
    await page.locator('#clear-pyometra-btn').click();

    await expect(bellaCard).toHaveAttribute(
      'data-status',
      'POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED'
    );
    await expect(page.locator('#pyometra-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Hannah Lin, DACVS'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .pyometra-status-badge, .pyometra-alert-banner, and .pyometra-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pyometra-sirs-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-pyometra-badge-test-fixture';
      container.innerHTML = `
        <span id="test-pyometra-status-badge-base" class="pyometra-status-badge">CLOSED_CERVIX_ENDOTOXIC_SIRS_TRIAGE</span>
        <span id="test-pyometra-status-badge-critical" class="pyometra-status-badge pyometra-status-critical">CLOSED_CERVIX_ENDOTOXIC_SIRS_TRIAGE</span>
        <span id="test-pyometra-status-badge-staged" class="pyometra-status-badge pyometra-status-staged">RESUSCITATED_EMPIRIC_ABX_OHE_STAGED</span>
        <span id="test-pyometra-status-badge-cleared" class="pyometra-status-badge pyometra-status-cleared">POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED</span>
        <div id="test-pyometra-alert-banner" class="pyometra-alert-banner">CRITICAL PYOMETRA, E. COLI ENDOTOXEMIA/SIRS & OHE SAFETY GUARD</div>
        <div id="test-pyometra-clearance-banner" class="pyometra-clearance-banner">POST-OP OVARIOHYSTERECTOMY (OHE) NORMOTENSIVE & AFEBRILE CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-pyometra-status-badge-base',
        '#test-pyometra-status-badge-critical',
        '#test-pyometra-status-badge-staged',
        '#test-pyometra-status-badge-cleared',
        '#test-pyometra-alert-banner',
        '#test-pyometra-clearance-banner',
      ]) {
        const styles = await page.locator(selector).evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { color: cs.color, backgroundColor: cs.backgroundColor };
        });
        expect(styles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
        const ratio = contrastRatio(styles.color, styles.backgroundColor);
        expect(ratio, `${selector} in ${theme} mode should have > 7.1:1 contrast`).toBeGreaterThan(
          7.1
        );
      }
    }
  });
});
