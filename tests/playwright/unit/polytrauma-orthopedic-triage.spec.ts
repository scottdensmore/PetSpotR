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

test.describe('Acute Bite Wound, Vehicular Polytrauma, Degloving/Shearing Injury, Open Fracture Gustilo-Anderson Classification, Wet-to-Dry/NPWT Vacuum-Assisted Closure & Damage-Control Orthopedics Cockpit UI (/polytrauma-orthopedic-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /polytrauma-orthopedic-triage renders seeded operations (ORTHO-OP-101, ORTHO-OP-102, ORTHO-OP-103), Gustilo-Anderson/BDLD Iceberg/7-8 PSI Lavage/NPWT/Splint alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/polytrauma-orthopedic-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#polytrauma-orthopedic-cockpit')).toBeVisible();
    await expect(page.locator('#ortho-alert-banner')).toBeVisible();
    await expect(page.locator('#ortho-alert-banner')).toContainText('Gustilo-Anderson');
    await expect(page.locator('#ortho-alert-banner')).toContainText('Iceberg Effect');
    await expect(page.locator('#ortho-alert-banner')).toContainText('7–8 PSI Pulsatile Lavage');
    await expect(page.locator('#ortho-alert-banner')).toContainText('NPWT (-125 mmHg)');
    await expect(page.locator('#ortho-alert-banner')).toContainText('Joint Above and Joint Below');

    await expect(page.locator('#ortho-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Ranger');
    await expect(page.locator('#active-att-score')).toContainText('9/18');
    await expect(page.locator('#active-lavage-volume')).toContainText('1500 mL');

    const rangerCard = page.locator('#ortho-operations-grid .ortho-op-card[data-op-id="ORTHO-OP-101"]');
    const peanutCard = page.locator('#ortho-operations-grid .ortho-op-card[data-op-id="ORTHO-OP-102"]');
    const tobyCard = page.locator('#ortho-operations-grid .ortho-op-card[data-op-id="ORTHO-OP-103"]');

    await expect(rangerCard).toBeVisible();
    await expect(rangerCard).toHaveAttribute(
      'data-status',
      'ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE'
    );

    await expect(peanutCard).toBeVisible();
    await expect(peanutCard).toHaveAttribute(
      'data-status',
      'LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED'
    );

    await expect(tobyCard).toBeVisible();
    await expect(tobyCard).toHaveAttribute(
      'data-status',
      'GRANULATION_FIXATION_STABLE_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging 7-8 PSI lavage, NPWT (-125 mmHg) & Robert Jones/ESF fixation on ORTHO-OP-101 transitions status to LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/polytrauma-orthopedic-triage`);
    await page.waitForLoadState('networkidle');

    const rangerCard = page.locator('#ortho-operations-grid .ortho-op-card[data-op-id="ORTHO-OP-101"]');
    await rangerCard.click();

    await expect(page.locator('#active-op-title')).toContainText('ORTHO-OP-101');
    await page.locator('#stage-fixation-btn').click();

    await expect(rangerCard).toHaveAttribute(
      'data-status',
      'LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED'
    );
  });

  test('3. Recording a serial orthopedic & wound check on ORTHO-OP-101 updates #active-att-score, #active-lavage-volume, and appends to #wound-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/polytrauma-orthopedic-triage`);
    await page.waitForLoadState('networkidle');

    const rangerCard = page.locator('#ortho-operations-grid .ortho-op-card[data-op-id="ORTHO-OP-101"]');
    await rangerCard.click();

    await page.locator('#check-att-score-input').fill('3');
    await page.locator('#check-lavage-input').fill('3000');
    await page.locator('#check-npwt-input').fill('-125');
    await page.locator('#check-lactate-input').fill('1.4');
    await page.locator('#check-toe-delta-input').fill('0.3');
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Caleb Thorne, DACVS (Orthopedics)');
    await page
      .locator('#check-note-input')
      .fill('72h post-7–8 PSI LRS lavage (3,000 mL), NPWT (-125 mmHg) & transarticular ESF: healthy granulation bed over talus, ATT score reduced to 3.');
    await page.locator('#record-wound-check-btn').click();

    await expect(page.locator('#active-att-score')).toContainText('3/18');
    await expect(page.locator('#active-lavage-volume')).toContainText('3000 mL');
    await expect(page.locator('#wound-checks-list')).toContainText(
      '72h post-7–8 PSI LRS lavage (3,000 mL), NPWT (-125 mmHg) & transarticular ESF'
    );
  });

  test('4. Clearing ORTHO-OP-101 for stable fixation discharge transitions status to GRANULATION_FIXATION_STABLE_CLEARED and reveals #ortho-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/polytrauma-orthopedic-triage`);
    await page.waitForLoadState('networkidle');

    const rangerCard = page.locator('#ortho-operations-grid .ortho-op-card[data-op-id="ORTHO-OP-101"]');
    await rangerCard.click();

    await page
      .locator('#clear-discharged-by-input')
      .fill('Dr. Caleb Thorne, DACVS (Orthopedics)');
    await page
      .locator('#clear-summary-input')
      .fill('Healthy granulation bed established over exposed cortical bone, transarticular ESF stable, ATT score 3, Lactate 1.4 mmol/L, warm distal digits.');
    await page.locator('#clear-ortho-btn').click();

    await expect(rangerCard).toHaveAttribute(
      'data-status',
      'GRANULATION_FIXATION_STABLE_CLEARED'
    );
    await expect(page.locator('#ortho-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Caleb Thorne, DACVS (Orthopedics)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .ortho-status-badge, .ortho-alert-banner, and .ortho-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/polytrauma-orthopedic-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-ortho-badge-test-fixture';
      container.innerHTML = `
        <span id="test-ortho-status-badge-base" class="ortho-status-badge">ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE</span>
        <span id="test-ortho-status-badge-critical" class="ortho-status-badge ortho-status-critical">ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE</span>
        <span id="test-ortho-status-badge-staged" class="ortho-status-badge ortho-status-staged">LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED</span>
        <span id="test-ortho-status-badge-cleared" class="ortho-status-badge ortho-status-cleared">GRANULATION_FIXATION_STABLE_CLEARED</span>
        <div id="test-ortho-alert-banner" class="ortho-alert-banner">CRITICAL POLYTRAUMA, OPEN FRACTURE & DEGLOVING GUARD</div>
        <div id="test-ortho-clearance-banner" class="ortho-clearance-banner">GRANULATION BED & ORTHOPEDIC FIXATION CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-ortho-status-badge-base',
        '#test-ortho-status-badge-critical',
        '#test-ortho-status-badge-staged',
        '#test-ortho-status-badge-cleared',
        '#test-ortho-alert-banner',
        '#test-ortho-clearance-banner',
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
