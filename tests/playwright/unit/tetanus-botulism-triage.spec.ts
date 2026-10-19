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

test.describe('Acute Tetanus (Clostridium tetani) & Botulism (Clostridium botulinum), Risus Sardonicus / Sawhorse Stance vs. Ascending Flaccid Lower Motor Neuron Paralysis, Equine Tetanus Antitoxin (TAT) Intradermal Test, Metronidazole (vs. Penicillin GABA Antagonism) & Dark/Quiet Sensory-Isolation Cockpit UI (/tetanus-botulism-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /tetanus-botulism-triage renders seeded operations (CLB-OP-101, CLB-OP-102, CLB-OP-103), Equine TAT intradermal skin test / Metronidazole vs. Penicillin G GABA-A antagonism / Aminoglycoside guard alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/tetanus-botulism-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#tetanus-botulism-cockpit')).toBeVisible();
    await expect(page.locator('#clb-alert-banner')).toBeVisible();
    await expect(page.locator('#clb-alert-banner')).toContainText('Equine Tetanus Antitoxin');
    await expect(page.locator('#clb-alert-banner')).toContainText('0.1–0.2 mL Intradermal Skin Test');
    await expect(page.locator('#clb-alert-banner')).toContainText('Metronidazole');
    await expect(page.locator('#clb-alert-banner')).toContainText('Penicillin G');
    await expect(page.locator('#clb-alert-banner')).toContainText('Aminoglycosides');

    await expect(page.locator('#clb-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Rex');
    await expect(page.locator('#active-burkitt-class')).toContainText('Class 3/4');
    await expect(page.locator('#active-spasm-frequency')).toContainText('6 spasms/hr');

    const rexCard = page.locator('#clb-operations-grid .clb-op-card[data-op-id="CLB-OP-101"]');
    const otisCard = page.locator('#clb-operations-grid .clb-op-card[data-op-id="CLB-OP-102"]');
    const bellaCard = page.locator('#clb-operations-grid .clb-op-card[data-op-id="CLB-OP-103"]');

    await expect(rexCard).toBeVisible();
    await expect(rexCard).toHaveAttribute(
      'data-status',
      'METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED'
    );

    await expect(otisCard).toBeVisible();
    await expect(otisCard).toHaveAttribute(
      'data-status',
      'SENSORY_ISOLATION_VENTILATION_MONITORED'
    );

    await expect(bellaCard).toBeVisible();
    await expect(bellaCard).toHaveAttribute(
      'data-status',
      'NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging Metronidazole / Equine TAT (after ID skin test) / Methocarbamol & dark/quiet sensory-isolation protocol on CLB-OP-101 updates active therapy regimen', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/tetanus-botulism-triage`);
    await page.waitForLoadState('networkidle');

    const rexCard = page.locator('#clb-operations-grid .clb-op-card[data-op-id="CLB-OP-101"]');
    await rexCard.click();

    await expect(page.locator('#active-op-title')).toContainText('CLB-OP-101');
    await page.locator('#stage-tat-units-input').fill('600');
    await page.locator('#stage-therapy-btn').click();

    await expect(rexCard).toHaveAttribute(
      'data-status',
      'METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED'
    );
    await expect(page.locator('#active-metro-tat')).toContainText('600 IU/kg');
  });

  test('3. Recording a neurological & ventilation check on CLB-OP-101 transitions status to SENSORY_ISOLATION_VENTILATION_MONITORED and appends to #neuro-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/tetanus-botulism-triage`);
    await page.waitForLoadState('networkidle');

    const rexCard = page.locator('#clb-operations-grid .clb-op-card[data-op-id="CLB-OP-101"]');
    await rexCard.click();

    await page.locator('#check-burkitt-class-input').fill('1');
    await page.locator('#check-spasm-freq-input').fill('0');
    await page.locator('#check-core-temp-input').fill('38.5');
    await page.locator('#check-paco2-input').fill('38.0');
    await page.locator('#check-hr-input').fill('88');
    await page.locator('#check-gag-reflex-checkbox').check();
    await page.locator('#check-earplugs-checkbox').check();
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Ingrid Vance, DACVIM (Neurology)');
    await page
      .locator('#check-note-input')
      .fill('Day 14 sensory-isolation check: Risus sardonicus and sawhorse rigidity resolved, 0 spasms/hr, intact pharyngeal gag reflex.');
    await page.locator('#record-neuro-check-btn').click();

    await expect(rexCard).toHaveAttribute(
      'data-status',
      'SENSORY_ISOLATION_VENTILATION_MONITORED'
    );
    await expect(page.locator('#active-burkitt-class')).toContainText('Class 1/4');
    await expect(page.locator('#active-spasm-frequency')).toContainText('0 spasms/hr');
    await expect(page.locator('#neuro-checks-list')).toContainText(
      'Day 14 sensory-isolation check: Risus sardonicus and sawhorse rigidity resolved'
    );
  });

  test('4. Clearing CLB-OP-101 for normoreflexic & swallowing-intact discharge transitions status to NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED and reveals #clb-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/tetanus-botulism-triage`);
    await page.waitForLoadState('networkidle');

    const rexCard = page.locator('#clb-operations-grid .clb-op-card[data-op-id="CLB-OP-101"]');
    await rexCard.click();

    await page.locator('#clear-spasm-freq-input').fill('0');
    await page.locator('#clear-paco2-input').fill('38.0');
    await page.locator('#clear-gag-reflex-checkbox').check();
    await page.locator('#clear-ambulatory-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Ingrid Vance, DACVIM (Neurology)');
    await page
      .locator('#clear-summary-input')
      .fill('Completed Metronidazole 10 mg/kg IV q12h and Equine TAT protocol; 0 spasms/hr, intact pharyngeal gag reflex, PaCO2 38.0 mmHg, and ambulatory normothermia verified.');
    await page.locator('#clear-clb-btn').click();

    await expect(rexCard).toHaveAttribute(
      'data-status',
      'NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED'
    );
    await expect(page.locator('#clb-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Ingrid Vance, DACVIM (Neurology)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .clb-status-badge, .clb-alert-banner, and .clb-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/tetanus-botulism-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-clb-badge-test-fixture';
      container.innerHTML = `
        <span id="test-clb-status-badge-base" class="clb-status-badge">METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED</span>
        <span id="test-clb-status-badge-critical" class="clb-status-badge clb-status-critical">METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED</span>
        <span id="test-clb-status-badge-staged" class="clb-status-badge clb-status-staged">SENSORY_ISOLATION_VENTILATION_MONITORED</span>
        <span id="test-clb-status-badge-cleared" class="clb-status-badge clb-status-cleared">NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED</span>
        <div id="test-clb-alert-banner" class="clb-alert-banner">CRITICAL CLOSTRIDIAL NEUROTOXIN ICU GUARD</div>
        <div id="test-clb-clearance-banner" class="clb-clearance-banner">NORMOREFLEXIC, SWALLOWING INTACT & NORMOCAPNIC CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-clb-status-badge-base',
        '#test-clb-status-badge-critical',
        '#test-clb-status-badge-staged',
        '#test-clb-status-badge-cleared',
        '#test-clb-alert-banner',
        '#test-clb-clearance-banner',
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
