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

test.describe('Acute Esophageal Foreign Body, Megaesophagus, Myasthenia Gravis (AChR Antibody / Edrophonium-Neostigmine), Aspiration Pneumonia & Pyridostigmine/Bailey Chair Cockpit UI (/megaesophagus-foreignbody-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /megaesophagus-foreignbody-triage renders seeded operations (MEGA-OP-101, MEGA-OP-102, MEGA-OP-103), Barium/Blind Force contraindication & Upright Bailey Chair alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/megaesophagus-foreignbody-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#megaesophagus-foreignbody-cockpit')).toBeVisible();
    await expect(page.locator('#mega-alert-banner')).toBeVisible();
    await expect(page.locator('#mega-alert-banner')).toContainText('Passive Regurgitation');
    await expect(page.locator('#mega-alert-banner')).toContainText('Liquid Barium');
    await expect(page.locator('#mega-alert-banner')).toContainText('Pyridostigmine Bromide');
    await expect(page.locator('#mega-alert-banner')).toContainText('Bailey Chair');

    await expect(page.locator('#mega-active-workspace')).toBeVisible();
    await expect(page.locator('#active-op-title')).toContainText('Waffles');
    await expect(page.locator('#active-spo2')).toContainText('91.0%');
    await expect(page.locator('#active-regurg-episodes')).toContainText('6 episodes/24h');

    const wafflesCard = page.locator('#mega-operations-grid .mega-op-card[data-op-id="MEGA-OP-101"]');
    const freyaCard = page.locator('#mega-operations-grid .mega-op-card[data-op-id="MEGA-OP-102"]');
    const pippinCard = page.locator('#mega-operations-grid .mega-op-card[data-op-id="MEGA-OP-103"]');

    await expect(wafflesCard).toBeVisible();
    await expect(wafflesCard).toHaveAttribute(
      'data-status',
      'ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE'
    );

    await expect(freyaCard).toBeVisible();
    await expect(freyaCard).toHaveAttribute(
      'data-status',
      'ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED'
    );

    await expect(pippinCard).toBeVisible();
    await expect(pippinCard).toHaveAttribute(
      'data-status',
      'UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED'
    );

    expect(cspErrors).toEqual([]);
  });

  test('2. Staging Flexible Esophagoscopy, Pyridostigmine & Upright Bailey Chair protocol on MEGA-OP-101 transitions status to ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/megaesophagus-foreignbody-triage`);
    await page.waitForLoadState('networkidle');

    const wafflesCard = page.locator('#mega-operations-grid .mega-op-card[data-op-id="MEGA-OP-101"]');
    await wafflesCard.click();

    await expect(page.locator('#active-op-title')).toContainText('MEGA-OP-101');
    await page.locator('#stage-bailey-btn').click();

    await expect(wafflesCard).toHaveAttribute(
      'data-status',
      'ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED'
    );
  });

  test('3. Recording a serial esophageal & aspiration check on MEGA-OP-101 updates #active-spo2, #active-regurg-episodes, and appends to #esophageal-checks-list', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/megaesophagus-foreignbody-triage`);
    await page.waitForLoadState('networkidle');

    const wafflesCard = page.locator('#mega-operations-grid .mega-op-card[data-op-id="MEGA-OP-101"]');
    await wafflesCard.click();

    await page.locator('#check-spo2-input').fill('98.0');
    await page.locator('#check-aa-gradient-input').fill('10.0');
    await page.locator('#check-regurg-input').fill('0');
    await page.locator('#check-upright-input').fill('30');
    await page.locator('#check-esophagitis-input').fill('1');
    await page.locator('#check-recorded-by-input').fill('Dr. Clara Sterling, DACVIM (SAIM)');
    await page
      .locator('#check-note-input')
      .fill('Post-endoscopic bone extraction, Sucralfate/Omeprazole & 30-min Upright Bailey Chair feeding: 0 regurgitation episodes over 24h, SpO2 98.0% on room air.');
    await page.locator('#record-esophageal-check-btn').click();

    await expect(page.locator('#active-spo2')).toContainText('98.0%');
    await expect(page.locator('#active-regurg-episodes')).toContainText('0 episodes/24h');
    await expect(page.locator('#esophageal-checks-list')).toContainText(
      'Post-endoscopic bone extraction, Sucralfate/Omeprazole & 30-min Upright Bailey Chair feeding'
    );
  });

  test('4. Clearing MEGA-OP-101 for Upright Bailey Chair discharge transitions status to UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED and reveals #mega-clearance-banner', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/megaesophagus-foreignbody-triage`);
    await page.waitForLoadState('networkidle');

    const wafflesCard = page.locator('#mega-operations-grid .mega-op-card[data-op-id="MEGA-OP-101"]');
    await wafflesCard.click();

    await page.locator('#clear-discharged-by-input').fill('Dr. Clara Sterling, DACVIM (SAIM)');
    await page
      .locator('#clear-summary-input')
      .fill('Esophageal obstruction resolved, 0 passive regurgitation episodes over 24h on 30-min Upright Bailey Chair feeding protocol, SpO2 98.0% on room air.');
    await page.locator('#clear-mega-btn').click();

    await expect(wafflesCard).toHaveAttribute(
      'data-status',
      'UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED'
    );
    await expect(page.locator('#mega-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Clara Sterling, DACVIM (SAIM)'
    );
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .mega-status-badge, .mega-alert-banner, and .mega-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/megaesophagus-foreignbody-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-mega-badge-test-fixture';
      container.innerHTML = `
        <span id="test-mega-status-badge-base" class="mega-status-badge">ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE</span>
        <span id="test-mega-status-badge-critical" class="mega-status-badge mega-status-critical">ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE</span>
        <span id="test-mega-status-badge-staged" class="mega-status-badge mega-status-staged">ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED</span>
        <span id="test-mega-status-badge-cleared" class="mega-status-badge mega-status-cleared">UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED</span>
        <div id="test-mega-alert-banner" class="mega-alert-banner">CRITICAL ESOPHAGEAL FOREIGN BODY, MEGAESOPHAGUS, MYASTHENIA GRAVIS & BAILEY CHAIR GUARD</div>
        <div id="test-mega-clearance-banner" class="mega-clearance-banner">UPRIGHT BAILEY CHAIR & NORMOTHERMIC CLEARANCE VERIFIED</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-mega-status-badge-base',
        '#test-mega-status-badge-critical',
        '#test-mega-status-badge-staged',
        '#test-mega-status-badge-cleared',
        '#test-mega-alert-banner',
        '#test-mega-clearance-banner',
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
