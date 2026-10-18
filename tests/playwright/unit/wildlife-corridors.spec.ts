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

test.describe('Predator & Wildlife Conflict Corridor Alerting Cockpit UI (/wildlife-corridors)', () => {
  test('1. /wildlife-corridors renders seeded reports (corridor-rep-101, corridor-rep-102, corridor-rep-103), mitigation steps, small prey alert banner, and clearance certificate with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/wildlife-corridors`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#wildlife-corridors-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Predator & Wildlife Conflict Corridor Alerting Cockpit'
    );

    await expect(page.locator('#kpi-corridor-total')).toBeVisible();
    await expect(page.locator('#kpi-corridor-extreme')).toBeVisible();
    await expect(page.locator('#kpi-corridor-small-prey')).toBeVisible();
    await expect(page.locator('#kpi-corridor-cleared')).toBeVisible();
    await expect(page.locator('#wildlife-corridors-live-status')).toBeVisible();

    const card101 = page.locator('#corridor-reports-list .corridor-report-card[data-report-id="corridor-rep-101"]');
    const card102 = page.locator('#corridor-reports-list .corridor-report-card[data-report-id="corridor-rep-102"]');
    const card103 = page.locator('#corridor-reports-list .corridor-report-card[data-report-id="corridor-rep-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-report-phase', 'ACTIVE_PREDATOR_WATCH');
    await expect(card101.locator('.badge-corridor-phase.badge-corridor-watch')).toContainText('ACTIVE_PREDATOR_WATCH');
    await expect(card101.locator('.badge-predator-threat.badge-threat-extreme')).toContainText('EXTREME_NOCTURNAL_PREDATION_RISK');
    await expect(card101.locator('.badge-corridor-metric')).toContainText('Vulnerability Index:');
    await expect(card101.locator('.badge-corridor-metric')).toContainText('Deterrent Readiness:');
    await expect(card101.locator('.corridor-small-prey-banner')).toContainText('SMALL PREY VULNERABILITY ALERT');
    await expect(card101.locator('.corridor-mitigation-list')).toContainText('ACOUSTIC_STROBE_DETERRENT');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-report-phase', 'HAZING_DETERRENT_DEPLOYED');
    await expect(card102.locator('.badge-corridor-phase.badge-corridor-hazing')).toContainText('HAZING_DETERRENT_DEPLOYED');
    await expect(card102.locator('.badge-predator-threat.badge-threat-elevated')).toContainText('ELEVATED_CREPUSCULAR_CORRIDOR');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-report-phase', 'CORRIDOR_CLEARED');
    await expect(card103.locator('.badge-corridor-phase.badge-corridor-cleared')).toContainText('CORRIDOR_CLEARED');
    await expect(card103.locator('.badge-predator-threat.badge-threat-safe')).toContainText('MITIGATED_SAFE_ZONE');
    await expect(card103.locator('.corridor-clearance-banner')).toContainText('CERT-CORRIDOR-CORRIDOR-REP-103');

    await expect(page.locator('#form-create-corridor-report')).toBeVisible();
    await expect(page.locator('#btn-sim-coyote-ravine')).toBeVisible();
    await expect(page.locator('#btn-sim-raptor-ridge')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Simulating a nocturnal coyote pack (#btn-sim-coyote-ravine) creates an ACTIVE_PREDATOR_WATCH card with .corridor-small-prey-banner and EXTREME_NOCTURNAL_PREDATION_RISK', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/wildlife-corridors`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-coyote-ravine').click();

    const simCard = page.locator('#corridor-reports-list .corridor-report-card', {
      hasText: '4.0kg',
    }).first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-report-phase', 'ACTIVE_PREDATOR_WATCH');
    await expect(simCard.locator('.badge-corridor-phase.badge-corridor-watch')).toContainText('ACTIVE_PREDATOR_WATCH');
    await expect(simCard.locator('.badge-predator-threat.badge-threat-extreme')).toContainText('EXTREME_NOCTURNAL_PREDATION_RISK');
    await expect(simCard.locator('.corridor-small-prey-banner')).toContainText('SMALL PREY VULNERABILITY ALERT');
  });

  test('3. Clicking .btn-deploy-deterrents deploys ultrasonic/strobe deterrents & locks attractants (Deterrent Readiness: 100/100) and transitions to HAZING_DETERRENT_DEPLOYED', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/wildlife-corridors`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-raptor-ridge').click();

    const raptorCard = page.locator('#corridor-reports-list .corridor-report-card', {
      hasText: 'GREAT_HORNED_OWL',
    }).first();
    await expect(raptorCard).toBeVisible();
    await expect(raptorCard).toHaveAttribute('data-report-phase', 'ACTIVE_PREDATOR_WATCH');
    await expect(raptorCard.locator('.badge-predator-threat.badge-threat-extreme')).toContainText('EXTREME_NOCTURNAL_PREDATION_RISK');

    await raptorCard.locator('.btn-deploy-deterrents').click();

    await expect(raptorCard).toHaveAttribute('data-report-phase', 'HAZING_DETERRENT_DEPLOYED');
    await expect(raptorCard.locator('.badge-corridor-metric')).toContainText('Deterrent Readiness: 100/100');
    await expect(raptorCard.locator('.badge-predator-threat')).toContainText('ELEVATED_CREPUSCULAR_CORRIDOR');
  });

  test('4. Clicking .btn-clear-corridor transitions a report to CORRIDOR_CLEARED (MITIGATED_SAFE_ZONE) and displays .corridor-clearance-banner (CERT-CORRIDOR-...)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/wildlife-corridors`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-raptor-ridge').click();

    const card = page.locator('#corridor-reports-list .corridor-report-card', {
      hasText: 'GREAT_HORNED_OWL',
    }).first();
    await expect(card).toBeVisible();

    await card.locator('.btn-clear-corridor').click();

    await expect(card).toHaveAttribute('data-report-phase', 'CORRIDOR_CLEARED');
    await expect(card.locator('.badge-corridor-phase.badge-corridor-cleared')).toContainText('CORRIDOR_CLEARED');
    await expect(card.locator('.badge-predator-threat.badge-threat-safe')).toContainText('MITIGATED_SAFE_ZONE');
    await expect(card.locator('.corridor-clearance-banner')).toBeVisible();
    await expect(card.locator('.corridor-clearance-banner')).toContainText('CERT-CORRIDOR-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7:1) on .badge-corridor-watch, .badge-corridor-hazing, .badge-corridor-cleared, .badge-threat-extreme, .badge-threat-elevated, .badge-threat-safe, .badge-corridor-metric, .corridor-small-prey-banner, and .corridor-clearance-banner in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/wildlife-corridors`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-wildlife-corridors-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-corridor-watch" class="badge badge-corridor-phase badge-corridor-watch">ACTIVE_PREDATOR_WATCH</span>
        <span id="test-badge-corridor-hazing" class="badge badge-corridor-phase badge-corridor-hazing">HAZING_DETERRENT_DEPLOYED</span>
        <span id="test-badge-corridor-cleared" class="badge badge-corridor-phase badge-corridor-cleared">CORRIDOR_CLEARED</span>
        <span id="test-badge-threat-extreme" class="badge badge-predator-threat badge-threat-extreme">EXTREME_NOCTURNAL_PREDATION_RISK</span>
        <span id="test-badge-threat-elevated" class="badge badge-predator-threat badge-threat-elevated">ELEVATED_CREPUSCULAR_CORRIDOR</span>
        <span id="test-badge-threat-safe" class="badge badge-predator-threat badge-threat-safe">MITIGATED_SAFE_ZONE</span>
        <span id="test-badge-corridor-metric" class="badge badge-corridor-metric">Vulnerability Index: 92/100 • Deterrent Readiness: 0/100</span>
        <div id="test-corridor-small-prey-banner" class="corridor-small-prey-banner">SMALL PREY VULNERABILITY ALERT</div>
        <div id="test-corridor-clearance-banner" class="corridor-clearance-banner">CORRIDOR SAFE-HAVEN VERIFIED: CERT-CORRIDOR-101</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-corridor-watch',
        '#test-badge-corridor-hazing',
        '#test-badge-corridor-cleared',
        '#test-badge-threat-extreme',
        '#test-badge-threat-elevated',
        '#test-badge-threat-safe',
        '#test-badge-corridor-metric',
        '#test-corridor-small-prey-banner',
        '#test-corridor-clearance-banner',
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
