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

test.describe('Acute Coagulopathy & Disseminated Intravascular Coagulation (DIC), PT/aPTT Prolongation, D-Dimer/FDP Elevation, Fresh Frozen Plasma (FFP) & Tranexamic Acid (TXA) Cockpit UI (/dic-coagulopathy-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /dic-coagulopathy-triage renders seeded operations (DIC-OP-101, DIC-OP-102, DIC-OP-103), FFP/Cryoprecipitate/TXA & Jugular Venipuncture guard alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/dic-coagulopathy-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#dic-coagulopathy-cockpit')).toBeVisible();
    await expect(page.locator('#dic-alert-banner')).toBeVisible();
    await expect(page.locator('#dic-alert-banner')).toContainText('Fresh Frozen Plasma');
    await expect(page.locator('#dic-alert-banner')).toContainText('Cryoprecipitate');
    await expect(page.locator('#dic-alert-banner')).toContainText('Tranexamic Acid');
    await expect(page.locator('#dic-alert-banner')).toContainText('Jugular Venipuncture');

    await expect(page.locator('#dic-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Flash');
    await expect(page.locator('#active-d-dimer')).toContainText('1850.0 ng/mL');
    await expect(page.locator('#active-fibrinogen')).toContainText('68.0 mg/dL');

    const flashCard = page.locator('#dic-operations-grid .dic-op-card[data-op-id="DIC-OP-101"]');
    const rocketCard = page.locator('#dic-operations-grid .dic-op-card[data-op-id="DIC-OP-102"]');
    const willowCard = page.locator('#dic-operations-grid .dic-op-card[data-op-id="DIC-OP-103"]');

    await expect(flashCard).toBeVisible();
    await expect(flashCard).toHaveAttribute(
      'data-status',
      'ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE'
    );

    await expect(rocketCard).toBeVisible();
    await expect(rocketCard).toHaveAttribute('data-status', 'FFP_CRYOPRECIPITATE_TXA_STAGED');

    await expect(willowCard).toBeVisible();
    await expect(willowCard).toHaveAttribute(
      'data-status',
      'HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging FFP, Cryoprecipitate & Tranexamic Acid (TXA) protocol on DIC-OP-101 transitions status to FFP_CRYOPRECIPITATE_TXA_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dic-coagulopathy-triage`);
    await page.waitForLoadState('networkidle');

    const flashCard = page.locator('#dic-operations-grid .dic-op-card[data-op-id="DIC-OP-101"]');
    await flashCard.click();

    await expect(page.locator('#active-op-title')).toContainText('DIC-OP-101');
    await page.locator('#stage-therapy-btn').click();

    await expect(flashCard).toHaveAttribute('data-status', 'FFP_CRYOPRECIPITATE_TXA_STAGED');
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'FFP_CRYOPRECIPITATE_TXA_STAGED'
    );
  });

  test('3. Recording a serial coagulation panel check on DIC-OP-101 updates #active-d-dimer, #active-fibrinogen, and appends to #coag-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dic-coagulopathy-triage`);
    await page.waitForLoadState('networkidle');

    const flashCard = page.locator('#dic-operations-grid .dic-op-card[data-op-id="DIC-OP-101"]');
    await flashCard.click();

    await page.locator('#check-platelets-input').fill('145.0');
    await page.locator('#check-pt-input').fill('11.6');
    await page.locator('#check-aptt-input').fill('72.0');
    await page.locator('#check-ddimer-input').fill('240.0');
    await page.locator('#check-fibrinogen-input').fill('195.0');
    await page.locator('#check-recorded-by-input').fill('Dr. Callum Vance, DACVECC');
    await page
      .locator('#check-note-input')
      .fill('Post-FFP 15 mL/kg, Cryoprecipitate & TXA 10 mg/kg IV: gingival petechiae resolved, Fibrinogen restored to 195.0 mg/dL, D-Dimer decreased to 240.0 ng/mL.');
    await page.locator('#record-coag-check-btn').click();

    await expect(page.locator('#active-d-dimer')).toContainText('240.0 ng/mL');
    await expect(page.locator('#active-fibrinogen')).toContainText('195.0 mg/dL');
    await expect(page.locator('#coag-checks-list')).toContainText(
      'Post-FFP 15 mL/kg, Cryoprecipitate & TXA 10 mg/kg IV: gingival petechiae resolved'
    );
  });

  test('4. Clearing DIC-OP-101 for normocoagulable discharge transitions status to HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED and reveals #dic-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dic-coagulopathy-triage`);
    await page.waitForLoadState('networkidle');

    const flashCard = page.locator('#dic-operations-grid .dic-op-card[data-op-id="DIC-OP-101"]');
    await flashCard.click();

    await page.locator('#clear-discharged-by-input').fill('Dr. Callum Vance, DACVECC');
    await page
      .locator('#clear-summary-input')
      .fill('Spontaneous mucosal bleeding completely arrested, PT/aPTT normalized (11.6s / 72.0s), Fibrinogen restored to 195.0 mg/dL, D-Dimer reduced to 240.0 ng/mL.');
    await page.locator('#clear-dic-btn').click();

    await expect(flashCard).toHaveAttribute(
      'data-status',
      'HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED'
    );
    await expect(page.locator('#dic-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Callum Vance, DACVECC'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .dic-status-badge, .dic-alert-banner, and .dic-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/dic-coagulopathy-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-dic-badge-test-fixture';
      container.innerHTML = `
        <span id="test-dic-status-badge-base" class="dic-status-badge">ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE</span>
        <span id="test-dic-status-badge-critical" class="dic-status-badge dic-status-critical">ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE</span>
        <span id="test-dic-status-badge-staged" class="dic-status-badge dic-status-staged">FFP_CRYOPRECIPITATE_TXA_STAGED</span>
        <span id="test-dic-status-badge-cleared" class="dic-status-badge dic-status-cleared">HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED</span>
        <div id="test-dic-alert-banner" class="dic-alert-banner">CRITICAL DIC, FFP, CRYOPRECIPITATE, TRANEXAMIC ACID (TXA) & JUGULAR VENIPUNCTURE GUARD</div>
        <div id="test-dic-clearance-banner" class="dic-clearance-banner">HEMOSTASIS RESTORED & NORMOCOAGULABLE CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-dic-status-badge-base',
        '#test-dic-status-badge-critical',
        '#test-dic-status-badge-staged',
        '#test-dic-status-badge-cleared',
        '#test-dic-alert-banner',
        '#test-dic-clearance-banner',
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
