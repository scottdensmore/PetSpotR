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

test.describe('Working Dog & Service Animal ADA Priority Recovery Cockpit UI (/service-animal-priority)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /service-animal-priority renders seeded cases (sa-case-101, sa-case-102, sa-case-103), recovery steps, and ADA medical standby banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/service-animal-priority`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#service-animal-priority-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Working Dog & Service Animal ADA Priority Recovery Cockpit'
    );

    await expect(page.locator('#kpi-sa-total')).toBeVisible();
    await expect(page.locator('#kpi-sa-critical')).toBeVisible();
    await expect(page.locator('#kpi-sa-standby')).toBeVisible();
    await expect(page.locator('#kpi-sa-restored')).toBeVisible();
    await expect(page.locator('#service-animal-live-status')).toBeVisible();

    const card101 = page.locator('#sa-cases-list .sa-case-card[data-case-id="sa-case-101"]');
    const card102 = page.locator('#sa-cases-list .sa-case-card[data-case-id="sa-case-102"]');
    const card103 = page.locator('#sa-cases-list .sa-case-card[data-case-id="sa-case-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-recovery-status', 'HANDLER_MEDICAL_ESCALATED');
    await expect(card101.locator('.badge-sa-status.badge-sa-escalated')).toContainText('HANDLER_MEDICAL_ESCALATED');
    await expect(card101.locator('.badge-sa-urgency.badge-urgency-critical')).toContainText('LIFE_CRITICAL_MEDICAL_DEPENDENCY');
    await expect(card101.locator('.sa-medical-standby-banner')).toContainText('ADA MEDICAL-ALERT HANDLER STANDBY & SNAG HAZARD');
    await expect(card101.locator('.sa-steps-list')).toContainText('HANDLER_MEDICAL_STANDBY');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-recovery-status', 'TACTICAL_VEST_INTERCEPT_STAGED');
    await expect(card102.locator('.badge-sa-status.badge-sa-staged')).toContainText('TACTICAL_VEST_INTERCEPT_STAGED');
    await expect(card102.locator('.badge-sa-urgency.badge-urgency-mobility')).toContainText('HIGH_MOBILITY_GUIDE_DEPENDENCY');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-recovery-status', 'REUNITED_ADA_TEAM_RESTORED');
    await expect(card103.locator('.badge-sa-status.badge-sa-restored')).toContainText('REUNITED_ADA_TEAM_RESTORED');
    await expect(card103.locator('.badge-sa-urgency.badge-urgency-stabilized')).toContainText('HANDLER_TEAM_STABILIZED');
    await expect(card103.locator('.sa-restoration-banner')).toContainText('ADA SERVICE ANIMAL TEAM RESTORED: ADA-RESTORED-SA-CASE-103');

    await expect(page.locator('#form-create-sa-case')).toBeVisible();
    await expect(page.locator('#btn-sim-diabetic-alert-dog')).toBeVisible();
    await expect(page.locator('#btn-sim-guide-dog-harness')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Simulating a T1D diabetic hypoglycemia alert dog (#btn-sim-diabetic-alert-dog) creates a HANDLER_MEDICAL_ESCALATED card with .sa-medical-standby-banner and LIFE_CRITICAL_MEDICAL_DEPENDENCY', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/service-animal-priority`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-diabetic-alert-dog').click();

    const simCard = page.locator('#sa-cases-list .sa-case-card', {
      hasText: 'Koda',
    }).first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-recovery-status', 'HANDLER_MEDICAL_ESCALATED');
    await expect(simCard.locator('.badge-sa-status.badge-sa-escalated')).toContainText('HANDLER_MEDICAL_ESCALATED');
    await expect(simCard.locator('.badge-sa-urgency.badge-urgency-critical')).toContainText('LIFE_CRITICAL_MEDICAL_DEPENDENCY');
    await expect(simCard.locator('.sa-medical-standby-banner')).toContainText('ADA MEDICAL-ALERT HANDLER STANDBY & SNAG HAZARD');
  });

  test('3. Clicking .btn-stage-sa-intercept stages EMS handler standby and vest intercept (Intercept Readiness: 100/100) and transitions case to TACTICAL_VEST_INTERCEPT_STAGED', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/service-animal-priority`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-guide-dog-harness').click();

    const guideDogCard = page.locator('#sa-cases-list .sa-case-card', {
      hasText: 'Beacon',
    }).first();
    await expect(guideDogCard).toBeVisible();
    await expect(guideDogCard).toHaveAttribute('data-recovery-status', 'HANDLER_MEDICAL_ESCALATED');

    await guideDogCard.locator('.btn-stage-sa-intercept').click();

    await expect(guideDogCard).toHaveAttribute('data-recovery-status', 'TACTICAL_VEST_INTERCEPT_STAGED');
    await expect(guideDogCard.locator('.badge-sa-status.badge-sa-staged')).toContainText('TACTICAL_VEST_INTERCEPT_STAGED');
    await expect(guideDogCard.locator('.badge-sa-metric')).toContainText('Intercept Readiness: 100/100');
    await expect(guideDogCard.locator('.badge-sa-urgency.badge-urgency-mobility')).toContainText('HIGH_MOBILITY_GUIDE_DEPENDENCY');
  });

  test('4. Clicking .btn-restore-sa-team transitions a case to REUNITED_ADA_TEAM_RESTORED (HANDLER_TEAM_STABILIZED) and displays .sa-restoration-banner (ADA-RESTORED-...)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/service-animal-priority`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-guide-dog-harness').click();

    const card = page.locator('#sa-cases-list .sa-case-card', {
      hasText: 'Beacon',
    }).first();
    await expect(card).toBeVisible();

    await card.locator('.btn-restore-sa-team').click();

    await expect(card).toHaveAttribute('data-recovery-status', 'REUNITED_ADA_TEAM_RESTORED');
    await expect(card.locator('.badge-sa-status.badge-sa-restored')).toContainText('REUNITED_ADA_TEAM_RESTORED');
    await expect(card.locator('.badge-sa-urgency.badge-urgency-stabilized')).toContainText('HANDLER_TEAM_STABILIZED');
    await expect(card.locator('.sa-restoration-banner')).toBeVisible();
    await expect(card.locator('.sa-restoration-banner')).toContainText('ADA SERVICE ANIMAL TEAM RESTORED: ADA-RESTORED-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7:1) on .badge-sa-escalated, .badge-sa-staged, .badge-sa-restored, .badge-urgency-critical, .badge-urgency-mobility, .badge-urgency-stabilized, .badge-sa-metric, .sa-medical-standby-banner, and .sa-restoration-banner in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/service-animal-priority`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-service-animal-priority-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-sa-escalated" class="badge badge-sa-status badge-sa-escalated">HANDLER_MEDICAL_ESCALATED</span>
        <span id="test-badge-sa-staged" class="badge badge-sa-status badge-sa-staged">TACTICAL_VEST_INTERCEPT_STAGED</span>
        <span id="test-badge-sa-restored" class="badge badge-sa-status badge-sa-restored">REUNITED_ADA_TEAM_RESTORED</span>
        <span id="test-badge-urgency-critical" class="badge badge-sa-urgency badge-urgency-critical">LIFE_CRITICAL_MEDICAL_DEPENDENCY</span>
        <span id="test-badge-urgency-mobility" class="badge badge-sa-urgency badge-urgency-mobility">HIGH_MOBILITY_GUIDE_DEPENDENCY</span>
        <span id="test-badge-urgency-stabilized" class="badge badge-sa-urgency badge-urgency-stabilized">HANDLER_TEAM_STABILIZED</span>
        <span id="test-badge-sa-metric" class="badge badge-sa-metric">Medical Urgency: 90/100 • Intercept Readiness: 100/100</span>
        <div id="test-sa-medical-standby-banner" class="sa-medical-standby-banner">ADA MEDICAL-ALERT HANDLER STANDBY &amp; SNAG HAZARD</div>
        <div id="test-sa-restoration-banner" class="sa-restoration-banner">ADA SERVICE ANIMAL TEAM RESTORED: ADA-RESTORED-101</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-sa-escalated',
        '#test-badge-sa-staged',
        '#test-badge-sa-restored',
        '#test-badge-urgency-critical',
        '#test-badge-urgency-mobility',
        '#test-badge-urgency-stabilized',
        '#test-badge-sa-metric',
        '#test-sa-medical-standby-banner',
        '#test-sa-restoration-banner',
      ]) {
        const styles = await page.locator(selector).evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { color: cs.color, backgroundColor: cs.backgroundColor };
        });
        expect(styles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
        const ratio = contrastRatio(styles.color, styles.backgroundColor);
        expect(ratio, `${selector} in ${theme} mode should have > 7:1 contrast`).toBeGreaterThan(7.0);
      }
    }
  });
});
