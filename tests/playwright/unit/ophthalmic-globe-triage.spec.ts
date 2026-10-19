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

test.describe('Acute Ophthalmic Emergencies, Descemetoceles, Proptosed Globe, Acute Congestive Glaucoma (IOP > 40 mmHg) & Mannitol/Latanoprost Cockpit UI (/ophthalmic-globe-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /ophthalmic-globe-triage renders seeded operations (OG-OP-101, OG-OP-102, OG-OP-103), KPI telemetry, Latanoprost/lens-luxation/steroid pitfall alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/ophthalmic-globe-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#ophthalmic-globe-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Acute Ophthalmic Emergencies, Descemetoceles, Proptosed Globe, Acute Congestive Glaucoma (IOP > 40 mmHg) & Mannitol/Latanoprost Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-globe')).toBeVisible();
    await expect(page.locator('#kpi-staged-latanoprost')).toBeVisible();
    await expect(page.locator('#kpi-cleared-iop')).toBeVisible();
    await expect(page.locator('#kpi-latanoprost-alerts')).toBeVisible();
    await expect(page.locator('#ophthalmic-alert-banner')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const busterCard = page.locator('#ophthalmic-operations-list .ophthalmic-op-card[data-op-id="OG-OP-101"]');
    const pugsleyCard = page.locator('#ophthalmic-operations-list .ophthalmic-op-card[data-op-id="OG-OP-102"]');
    const mistyCard = page.locator('#ophthalmic-operations-list .ophthalmic-op-card[data-op-id="OG-OP-103"]');

    await expect(busterCard).toBeVisible();
    await expect(busterCard).toHaveAttribute('data-status', 'VISION_THREATENING_IOP_DESCEMETOCELE_CRITICAL');
    await expect(busterCard.locator('.ophthalmic-alert-banner')).toContainText('Latanoprost');

    await expect(pugsleyCard).toBeVisible();
    await expect(pugsleyCard).toHaveAttribute('data-status', 'LATANOPROST_MANNITOL_TARSORRHAPHY_STAGED');

    await expect(mistyCard).toBeVisible();
    await expect(mistyCard).toHaveAttribute('data-status', 'IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED');
    await expect(mistyCard.locator('.ophthalmic-clearance-banner')).toContainText('VET-OPHTHO-99503');

    expect(cspErrors).toEqual([]);
  });

  test('2. Selecting OG-OP-101 Buster populates the active workspace, displays Latanoprost anterior lens luxation contraindication alert banner, and stages Mannitol IV & Dorzolamide/Timolol plan to LATANOPROST_MANNITOL_TARSORRHAPHY_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ophthalmic-globe-triage`);
    await page.waitForLoadState('networkidle');

    const busterCard = page.locator('#ophthalmic-operations-list .ophthalmic-op-card[data-op-id="OG-OP-101"]');
    await busterCard.locator('.ophthalmic-select-op-btn').click();

    await expect(page.locator('#workspace-pet-title')).toContainText('Buster');
    await expect(page.locator('#ophthalmic-alert-banner')).toContainText('Latanoprost');
    await expect(page.locator('#ophthalmic-alert-banner')).toContainText('corticosteroids');

    await page.locator('#stage-weight-kg').fill('12.5');
    await page.locator('#stage-species').selectOption('CANINE_PRIMARY_CONGESTIVE_GLAUCOMA');
    await page.locator('#stage-iop-mmhg').fill('64');
    await page.locator('#stage-fluorescein-pattern').selectOption('FLUORESCEIN_NEGATIVE_INTACT');
    await page.locator('#stage-muscles-avulsed').fill('0');
    await page.locator('#stage-anterior-luxation').check();
    await page.locator('#stage-consensual-plr').check();
    await page.locator('#btn-stage-plan').click();

    await expect(busterCard).toHaveAttribute('data-status', 'LATANOPROST_MANNITOL_TARSORRHAPHY_STAGED');
    await expect(page.locator('#workspace-status-pill')).toContainText(
      'LATANOPROST_MANNITOL_TARSORRHAPHY_STAGED'
    );
    await expect(page.locator('#plan-latanoprost')).toContainText('CONTRAINDICATED');
    await expect(page.locator('#plan-dorzolamide')).toContainText('q8h');
    await expect(page.locator('#plan-mannitol')).toContainText('12.5 g IV');
  });

  test('3. Recording a rebound tonometry, Seidel & dazzle reflex check on OG-OP-101 appends check to #tonometry-checks-list and updates patient IOP', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ophthalmic-globe-triage`);
    await page.waitForLoadState('networkidle');

    const busterCard = page.locator('#ophthalmic-operations-list .ophthalmic-op-card[data-op-id="OG-OP-101"]');
    await busterCard.locator('.ophthalmic-select-op-btn').click();

    await page.locator('#check-iop-mmhg').fill('18');
    await page.locator('#check-callsign').fill('DR-OPHTHO-11');
    await page.locator('#check-notes').fill('Post-Mannitol 12.5 g IV & Dorzolamide/Timolol q8h tonometry: IOP reduced from 64 to 18 mmHg; Seidel negative; dazzle reflex intact; rigid E-collar verified.');
    await page.locator('#check-seidel-negative').check();
    await page.locator('#check-dazzle-present').check();
    await page.locator('#check-ecollar-verified').check();
    await page.locator('#btn-record-tonometry-check').click();

    await expect(page.locator('#tonometry-checks-list')).toContainText(
      'Post-Mannitol 12.5 g IV & Dorzolamide/Timolol q8h tonometry: IOP reduced from 64 to 18 mmHg; Seidel negative; dazzle reflex intact; rigid E-collar verified.'
    );
    await expect(page.locator('#workspace-vitals-summary')).toContainText('18 mmHg');
  });

  test('4. Issuing IOP normalized & globe protected discharge clearance on OG-OP-101 transitions status to IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED and reveals #ophthalmic-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ophthalmic-globe-triage`);
    await page.waitForLoadState('networkidle');

    const busterCard = page.locator('#ophthalmic-operations-list .ophthalmic-op-card[data-op-id="OG-OP-101"]');
    await busterCard.locator('.ophthalmic-select-op-btn').click();

    await page.locator('#clearance-final-iop').fill('16');
    await page.locator('#clearance-vet-license').fill('VET-OPHTHO-99508');
    await page.locator('#clearance-summary').fill('IOP normalized at 16 mmHg; globe tectonic integrity confirmed; hard plastic E-collar fitted and owner educated on 5-minute spacing between topical ophthalmic drops.');
    await page.locator('#clearance-tectonic-integrity').check();
    await page.locator('#clearance-ecollar-fitted').check();
    await page.locator('#clearance-drop-spacing-educated').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(busterCard).toHaveAttribute('data-status', 'IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED');
    await expect(page.locator('#ophthalmic-clearance-banner')).toBeVisible();
    await expect(page.locator('#ophthalmic-clearance-banner')).toContainText('VET-OPHTHO-99508');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .status-pill, .status-pill-critical, .status-pill-staged, .status-pill-cleared, .ophthalmic-alert-banner, and .ophthalmic-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ophthalmic-globe-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-ophthalmic-badge-test-fixture';
      container.innerHTML = `
        <span id="test-status-pill-base" class="status-pill status-pill-species">CANINE_PRIMARY_CONGESTIVE_GLAUCOMA</span>
        <span id="test-status-pill-critical" class="status-pill status-pill-critical">VISION_THREATENING_IOP_DESCEMETOCELE_CRITICAL</span>
        <span id="test-status-pill-staged" class="status-pill status-pill-staged">LATANOPROST_MANNITOL_TARSORRHAPHY_STAGED</span>
        <span id="test-status-pill-cleared" class="status-pill status-pill-cleared">IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED</span>
        <div id="test-ophthalmic-alert-banner" class="ophthalmic-alert-banner">CRITICAL OPHTHALMIC GLOBE & GLAUCOMA PITFALL ALERT: Do NOT administer Latanoprost in anterior lens luxation</div>
        <div id="test-ophthalmic-clearance-banner" class="ophthalmic-clearance-banner">IOP NORMALIZED & GLOBE PROTECTED CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-status-pill-base',
        '#test-status-pill-critical',
        '#test-status-pill-staged',
        '#test-status-pill-cleared',
        '#test-ophthalmic-alert-banner',
        '#test-ophthalmic-clearance-banner',
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
