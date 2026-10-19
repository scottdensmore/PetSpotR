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

test.describe('Anticoagulant Rodenticide, Xylitol Hypoglycemia/Hepatotoxicity & Lipid Emulsion (ILE) Poison Control Cockpit UI (/toxicology-poison-triage)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /toxicology-poison-triage renders seeded operations (TP-OP-101, TP-OP-102, TP-OP-103), KPI telemetry, toxicology pitfall alert banner, and active workspace with zero CSP errors', async ({
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

    await page.goto(`${WEB_FRONTEND_URL}/toxicology-poison-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#toxicology-poison-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Anticoagulant Rodenticide, Xylitol Hypoglycemia/Hepatotoxicity & Lipid Emulsion (ILE) Poison Control Cockpit'
    );

    await expect(page.locator('#kpi-total-ops')).toBeVisible();
    await expect(page.locator('#kpi-critical-poison')).toBeVisible();
    await expect(page.locator('#kpi-staged-decon')).toBeVisible();
    await expect(page.locator('#kpi-cleared-safe')).toBeVisible();
    await expect(page.locator('#active-operation-workspace')).toBeVisible();

    const waffleCard = page.locator('#poison-operations-list .poison-op-card', {
      hasText: 'Waffle (',
    });
    const banditCard = page.locator('#poison-operations-list .poison-op-card', {
      hasText: 'Bandit (',
    });
    const tiggerCard = page.locator('#poison-operations-list .poison-op-card', {
      hasText: 'Tigger (',
    });

    await expect(waffleCard).toBeVisible();
    await expect(waffleCard).toHaveAttribute(
      'data-status',
      'COAGULOPATHY_HEPATIC_NEURO_TOXIC_CRITICAL'
    );
    await expect(waffleCard.locator('.badge-poison-status.badge-poison-critical')).toContainText(
      'COAGULOPATHY_HEPATIC_NEURO_TOXIC_CRITICAL'
    );
    await expect(waffleCard.locator('.poison-pitfall-alert-banner')).toContainText('xylitol');

    await expect(banditCard).toBeVisible();
    await expect(banditCard).toHaveAttribute(
      'data-status',
      'VITAMIN_K1_ILE_CHARCOAL_DECON_STAGED'
    );

    await expect(tiggerCard).toBeVisible();
    await expect(tiggerCard).toHaveAttribute(
      'data-status',
      'DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED'
    );
    await expect(tiggerCard.locator('.poison-clearance-banner')).toContainText('POISON-SAFE-');

    expect(cspErrors).toEqual([]);
  });

  test('2. Submitting #poison-eval-form evaluates a new critical toxicology poison operation with pitfall alert and antidote regimen', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/toxicology-poison-triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#eval-pet-name').fill('Mochi');
    await page.locator('#eval-species').fill('Dog - French Bulldog');
    await page.locator('#eval-weight-kg').fill('11.2');
    await page.locator('#eval-agent-type').selectOption('XYLITOL_INSULIN_SURGE_HEPATOTOXIN');
    await page.locator('#eval-dose-mgkg').fill('540.0');
    await page.locator('#eval-pt-ratio').fill('1.9');
    await page.locator('#eval-glucose-mgdl').fill('38.0');
    await page.locator('#eval-ingestion-mins').fill('40');
    await page.locator('#eval-charcoal-avoided').check();
    await page.locator('#eval-k3-avoided').check();

    await page.locator('#btn-submit-poison-eval').click();

    const mochiCard = page
      .locator('#poison-operations-list .poison-op-card', {
        hasText: 'Mochi (',
      })
      .first();
    await expect(mochiCard).toBeVisible();
    await expect(mochiCard).toHaveAttribute(
      'data-status',
      'COAGULOPATHY_HEPATIC_NEURO_TOXIC_CRITICAL'
    );
    await expect(mochiCard.locator('.poison-pitfall-alert-banner')).toContainText('Phytonadione');
  });

  test('3. Completing all 4 protocol steps on TP-OP-101 transitions status to VITAMIN_K1_ILE_CHARCOAL_DECON_STAGED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/toxicology-poison-triage`);
    await page.waitForLoadState('networkidle');

    const card101 = page.locator('#poison-operations-list .poison-op-card[data-op-id="TP-OP-101"]');
    await expect(card101).toBeVisible();

    for (const stepId of [
      'STEP-APOMORPHINE-ROPIFINIROLE-EMESIS-GATE',
      'STEP-PHYTONADIONE-K1-OR-20PCT-ILE-LIPID',
      'STEP-DEXTROSE-NAC-SAME-48HR-IV-DIURESIS',
      'STEP-48HR-PT-RECHECK-RENAL-HEPATIC-PANEL',
    ]) {
      const cb = card101.locator(`.poison-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(card101).toHaveAttribute('data-status', 'VITAMIN_K1_ILE_CHARCOAL_DECON_STAGED');
    await expect(card101.locator('.badge-poison-status.badge-poison-staged')).toContainText(
      'VITAMIN_K1_ILE_CHARCOAL_DECON_STAGED'
    );
  });

  test('4. Clicking .poison-issue-clearance-btn issues a POISON-SAFE-<12HEX> clearance token and transitions operation to DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/toxicology-poison-triage`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#poison-operations-list .poison-op-card[data-op-id="TP-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.poison-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED');
    await expect(card102.locator('.badge-poison-status.badge-poison-safe')).toContainText(
      'DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED'
    );
    await expect(card102.locator('.poison-clearance-banner')).toContainText('POISON-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-poison-critical, .badge-poison-staged, .badge-poison-safe, .badge-poison-agent, .badge-poison-flag, .poison-pitfall-alert-banner, and .poison-clearance-banner in both dark and light themes', async ({
    page,
  }) => {
    await page.goto(`${WEB_FRONTEND_URL}/toxicology-poison-triage`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-poison-triage-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-poison-critical" class="badge badge-poison-status badge-poison-critical">COAGULOPATHY_HEPATIC_NEURO_TOXIC_CRITICAL</span>
        <span id="test-badge-poison-staged" class="badge badge-poison-status badge-poison-staged">VITAMIN_K1_ILE_CHARCOAL_DECON_STAGED</span>
        <span id="test-badge-poison-safe" class="badge badge-poison-status badge-poison-safe">DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED</span>
        <span id="test-badge-poison-agent" class="badge badge-poison-agent">XYLITOL_INSULIN_SURGE_HEPATOTOXIN</span>
        <span id="test-badge-poison-flag" class="badge badge-poison-flag">XYLITOL_HYPOGLYCEMIC_CRISIS</span>
        <div id="test-poison-pitfall-alert-banner" class="poison-pitfall-alert-banner">CRITICAL TOXICOLOGY &amp; ANTIDOTE PITFALL ALERT: Activated charcoal does NOT bind xylitol</div>
        <div id="test-poison-clearance-banner" class="poison-clearance-banner">DECONTAMINATED PT &amp; GLUCOSE NORMAL CLEARANCE VERIFIED: POISON-SAFE-1A2B3C4D5E6F</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-poison-critical',
        '#test-badge-poison-staged',
        '#test-badge-poison-safe',
        '#test-badge-poison-agent',
        '#test-badge-poison-flag',
        '#test-poison-pitfall-alert-banner',
        '#test-poison-clearance-banner',
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
