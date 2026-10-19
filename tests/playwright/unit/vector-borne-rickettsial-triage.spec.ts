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

test.describe('Acute Vector-Borne & Rickettsial Emergencies (Lyme Nephritis, Anaplasmosis, Ehrlichiosis, Rocky Mountain Spotted Fever, Babesiosis & Cytauxzoonosis), SNAP 4Dx Plus / PCR Triage, Doxycycline, Atovaquone + Azithromycin & Imidocarb Dipropionate Cockpit UI (/vector-borne-rickettsial-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /vector-borne-rickettsial-triage renders seeded operations (VBR-OP-101, VBR-OP-102, VBR-OP-103), Feline Doxycycline Water Chase / Imidocarb Atropine / Atovaquone Fatty Meal / Lyme Nephritis UPC guard alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/vector-borne-rickettsial-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#vbr-triage-cockpit')).toBeVisible();
    await expect(page.locator('#vbr-alert-banner')).toBeVisible();
    await expect(page.locator('#vbr-alert-banner')).toContainText('5–6 mL syringe water bolus');
    await expect(page.locator('#vbr-alert-banner')).toContainText('Atropine 0.02–0.04 mg/kg SC');
    await expect(page.locator('#vbr-alert-banner')).toContainText('Atovaquone');
    await expect(page.locator('#vbr-alert-banner')).toContainText('Lyme Nephritis');

    await expect(page.locator('#vbr-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Yukon');
    await expect(page.locator('#active-platelets')).toContainText('42.0 K/µL');
    await expect(page.locator('#active-upc')).toContainText('4.80');

    const yukonCard = page.locator('#vbr-operations-grid .vbr-op-card[data-op-id="VBR-OP-101"]');
    const willowCard = page.locator('#vbr-operations-grid .vbr-op-card[data-op-id="VBR-OP-102"]');
    const booneCard = page.locator('#vbr-operations-grid .vbr-op-card[data-op-id="VBR-OP-103"]');

    await expect(yukonCard).toBeVisible();
    await expect(yukonCard).toHaveAttribute(
      'data-status',
      'DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED'
    );

    await expect(willowCard).toBeVisible();
    await expect(willowCard).toHaveAttribute(
      'data-status',
      'THROMBOCYTOPENIA_PROTEINURIA_MONITORED'
    );

    await expect(booneCard).toBeVisible();
    await expect(booneCard).toHaveAttribute(
      'data-status',
      'PCR_NEGATIVE_PLATELETS_RENAL_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging antimicrobial & organ-protection regimen on VBR-OP-101 updates protocol and preserves DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/vector-borne-rickettsial-triage`);
    await page.waitForLoadState('networkidle');

    const yukonCard = page.locator('#vbr-operations-grid .vbr-op-card[data-op-id="VBR-OP-101"]');
    await yukonCard.click();

    await expect(page.locator('#active-op-title')).toContainText('VBR-OP-101');
    await page
      .locator('#stage-protocol-input')
      .selectOption('MYCOPHENOLATE_TELMISARTAN_LYME_PLN');
    await page.locator('#stage-doxy-input').fill('10.0');
    await page.locator('#stage-water-chase-checkbox').check();
    await page.locator('#stage-antimicrobial-btn').click();

    await expect(yukonCard).toHaveAttribute(
      'data-status',
      'DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED'
    );
  });

  test('3. Recording a hematology & renal UPC check on VBR-OP-101 transitions status to THROMBOCYTOPENIA_PROTEINURIA_MONITORED and updates vitals', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/vector-borne-rickettsial-triage`);
    await page.waitForLoadState('networkidle');

    const yukonCard = page.locator('#vbr-operations-grid .vbr-op-card[data-op-id="VBR-OP-101"]');
    await yukonCard.click();

    await page.locator('#check-platelets-input').fill('185.0');
    await page.locator('#check-pcv-input').fill('39.0');
    await page.locator('#check-upc-input').fill('0.35');
    await page.locator('#check-albumin-input').fill('3.1');
    await page.locator('#check-morulae-cleared-checkbox').check();
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Mara Vance, DACVIM (Infectious Disease)');
    await page
      .locator('#check-note-input')
      .fill('Day 14 Lyme Nephritis Check: Platelets 185.0 K/µL, PCV 39.0%, UPC improved from 4.80 to 0.35 (< 0.50).');
    await page.locator('#record-hematology-check-btn').click();

    await expect(yukonCard).toHaveAttribute(
      'data-status',
      'THROMBOCYTOPENIA_PROTEINURIA_MONITORED'
    );
    await expect(page.locator('#active-platelets')).toContainText('185.0 K/µL');
    await expect(page.locator('#active-upc')).toContainText('0.35');
    await expect(page.locator('#hematology-checks-list')).toContainText(
      'Day 14 Lyme Nephritis Check: Platelets 185.0 K/µL'
    );
  });

  test('4. Clearing VBR-OP-101 for PCR-negative & platelet/renal recovery discharge transitions status to PCR_NEGATIVE_PLATELETS_RENAL_CLEARED and reveals #vbr-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/vector-borne-rickettsial-triage`);
    await page.waitForLoadState('networkidle');

    const yukonCard = page.locator('#vbr-operations-grid .vbr-op-card[data-op-id="VBR-OP-101"]');
    await yukonCard.click();

    await page.locator('#clear-platelets-input').fill('195.0');
    await page.locator('#clear-pcv-input').fill('40.0');
    await page.locator('#clear-upc-input').fill('0.20');
    await page.locator('#clear-pcr-negative-checkbox').check();
    await page.locator('#clear-normothermic-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Mara Vance, DACVIM (Infectious Disease)');
    await page
      .locator('#clear-summary-input')
      .fill('Quantitative PCR negative, platelets normalized to 195.0 K/µL (>= 150 K/µL), PCV 40.0% (>= 35%), UPC 0.20 (< 0.5), normothermic and eating; cleared for discharge.');
    await page.locator('#clear-vbr-btn').click();

    await expect(yukonCard).toHaveAttribute(
      'data-status',
      'PCR_NEGATIVE_PLATELETS_RENAL_CLEARED'
    );
    await expect(page.locator('#vbr-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Mara Vance, DACVIM (Infectious Disease)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .vbr-status-badge, .vbr-alert-banner, and .vbr-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/vector-borne-rickettsial-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-vbr-badge-test-fixture';
      container.innerHTML = `
        <span id="test-vbr-status-badge-base" class="vbr-status-badge">DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED</span>
        <span id="test-vbr-status-badge-critical" class="vbr-status-badge vbr-status-critical">DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED</span>
        <span id="test-vbr-status-badge-staged" class="vbr-status-badge vbr-status-staged">THROMBOCYTOPENIA_PROTEINURIA_MONITORED</span>
        <span id="test-vbr-status-badge-cleared" class="vbr-status-badge vbr-status-cleared">PCR_NEGATIVE_PLATELETS_RENAL_CLEARED</span>
        <div id="test-vbr-alert-banner" class="vbr-alert-banner">CRITICAL VECTOR-BORNE & RICKETTSIAL SAFETY GUARDS</div>
        <div id="test-vbr-clearance-banner" class="vbr-clearance-banner">PCR-NEGATIVE, PLATELET & RENAL CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-vbr-status-badge-base',
        '#test-vbr-status-badge-critical',
        '#test-vbr-status-badge-staged',
        '#test-vbr-status-badge-cleared',
        '#test-vbr-alert-banner',
        '#test-vbr-clearance-banner',
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
