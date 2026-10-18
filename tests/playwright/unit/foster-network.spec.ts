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

test.describe('Emergency Foster Network Capacity & Quarantine Biosecurity Cockpit UI (/foster-network)', () => {
  test.describe.configure({ mode: 'serial' });
  test('1. /foster-network renders seeded placements (foster-match-101, foster-match-102, foster-match-103), biosecurity checklist items, and quarantine alert banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/foster-network`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#foster-network-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Emergency Foster Network Capacity & Quarantine Biosecurity Cockpit'
    );

    await expect(page.locator('#kpi-foster-total')).toBeVisible();
    await expect(page.locator('#kpi-foster-iso')).toBeVisible();
    await expect(page.locator('#kpi-foster-contagion')).toBeVisible();
    await expect(page.locator('#kpi-foster-active')).toBeVisible();
    await expect(page.locator('#foster-network-live-status')).toBeVisible();

    const card101 = page.locator('#foster-placements-list .foster-placement-card[data-placement-id="foster-match-101"]');
    const card102 = page.locator('#foster-placements-list .foster-placement-card[data-placement-id="foster-match-102"]');
    const card103 = page.locator('#foster-placements-list .foster-placement-card[data-placement-id="foster-match-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-placement-status', 'PENDING_BIOSECURITY_CLEARANCE');
    await expect(card101.locator('.badge-foster-status.badge-foster-pending')).toContainText('PENDING_BIOSECURITY_CLEARANCE');
    await expect(card101.locator('.badge-bio-tier.badge-bio-iso')).toContainText('ISO_QUARANTINE_CONTAGION');
    await expect(card101.locator('.foster-iso-quarantine-banner')).toContainText('BIOSECURITY ISOLATION PROTOCOL REQUIRED');
    await expect(card101.locator('.foster-checks-list')).toContainText('PATHOGEN_ISOLATION');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-placement-status', 'QUARANTINE_VERIFIED_STAGED');
    await expect(card102.locator('.badge-foster-status.badge-foster-staged')).toContainText('QUARANTINE_VERIFIED_STAGED');
    await expect(card102.locator('.badge-bio-tier.badge-bio-medical')).toContainText('MEDICAL_RECOVERY_SINGLE_PET');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-placement-status', 'ACTIVE_IN_FOSTER_HOME');
    await expect(card103.locator('.badge-foster-status.badge-foster-active')).toContainText('ACTIVE_IN_FOSTER_HOME');
    await expect(card103.locator('.badge-bio-tier.badge-bio-cleared')).toContainText('BIOSECURITY_CLEARED_STANDARD');
    await expect(card103.locator('.foster-authorization-banner')).toContainText('FOSTER BIOSECURITY AUTHORIZED: FOSTER-AUTH-FOSTER-MATCH-103');

    await expect(page.locator('#form-create-foster-placement')).toBeVisible();
    await expect(page.locator('#btn-sim-parvo-iso')).toBeVisible();
    await expect(page.locator('#btn-sim-ringworm-cat')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Simulating a Parvo isolation case (#btn-sim-parvo-iso) creates a PENDING_BIOSECURITY_CLEARANCE card with .foster-iso-quarantine-banner and ISO_QUARANTINE_CONTAGION', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/foster-network`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-parvo-iso').click();

    const simCard = page.locator('#foster-placements-list .foster-placement-card', {
      hasText: 'PARVO_SUSPECTED',
    }).first();
    await expect(simCard).toBeVisible();
    await expect(simCard).toHaveAttribute('data-placement-status', 'PENDING_BIOSECURITY_CLEARANCE');
    await expect(simCard.locator('.badge-foster-status.badge-foster-pending')).toContainText('PENDING_BIOSECURITY_CLEARANCE');
    await expect(simCard.locator('.badge-bio-tier.badge-bio-iso')).toContainText('ISO_QUARANTINE_CONTAGION');
    await expect(simCard.locator('.foster-iso-quarantine-banner')).toContainText('BIOSECURITY ISOLATION PROTOCOL REQUIRED');
  });

  test('3. Clicking .btn-verify-foster-bio verifies HEPA quarantine rig and vet tech cert (Host Readiness: 100/100) and transitions placement to QUARANTINE_VERIFIED_STAGED', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/foster-network`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-ringworm-cat').click();

    const ringwormCard = page.locator('#foster-placements-list .foster-placement-card', {
      hasText: 'Mochi',
    }).first();
    await expect(ringwormCard).toBeVisible();
    await expect(ringwormCard).toHaveAttribute('data-placement-status', 'PENDING_BIOSECURITY_CLEARANCE');

    await ringwormCard.locator('.btn-verify-foster-bio').click();

    await expect(ringwormCard).toHaveAttribute('data-placement-status', 'QUARANTINE_VERIFIED_STAGED');
    await expect(ringwormCard.locator('.badge-foster-status.badge-foster-staged')).toContainText('QUARANTINE_VERIFIED_STAGED');
    await expect(ringwormCard.locator('.badge-foster-metric')).toContainText('Host Readiness: 100/100');
    await expect(ringwormCard.locator('.badge-bio-tier.badge-bio-medical')).toContainText('MEDICAL_RECOVERY_SINGLE_PET');
  });

  test('4. Clicking .btn-activate-foster-placement transitions a placement to ACTIVE_IN_FOSTER_HOME (BIOSECURITY_CLEARED_STANDARD) and displays .foster-authorization-banner (FOSTER-AUTH-...)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/foster-network`);
    await page.waitForLoadState('networkidle');

    await page.locator('#btn-sim-ringworm-cat').click();

    const card = page.locator('#foster-placements-list .foster-placement-card', {
      hasText: 'Mochi',
    }).first();
    await expect(card).toBeVisible();

    await card.locator('.btn-activate-foster-placement').click();

    await expect(card).toHaveAttribute('data-placement-status', 'ACTIVE_IN_FOSTER_HOME');
    await expect(card.locator('.badge-foster-status.badge-foster-active')).toContainText('ACTIVE_IN_FOSTER_HOME');
    await expect(card.locator('.badge-bio-tier.badge-bio-cleared')).toContainText('BIOSECURITY_CLEARED_STANDARD');
    await expect(card.locator('.foster-authorization-banner')).toBeVisible();
    await expect(card.locator('.foster-authorization-banner')).toContainText('FOSTER BIOSECURITY AUTHORIZED: FOSTER-AUTH-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7:1) on .badge-foster-pending, .badge-foster-staged, .badge-foster-active, .badge-bio-iso, .badge-bio-medical, .badge-bio-cleared, .badge-foster-metric, .foster-iso-quarantine-banner, and .foster-authorization-banner in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/foster-network`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-foster-network-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-foster-pending" class="badge badge-foster-status badge-foster-pending">PENDING_BIOSECURITY_CLEARANCE</span>
        <span id="test-badge-foster-staged" class="badge badge-foster-status badge-foster-staged">QUARANTINE_VERIFIED_STAGED</span>
        <span id="test-badge-foster-active" class="badge badge-foster-status badge-foster-active">ACTIVE_IN_FOSTER_HOME</span>
        <span id="test-badge-bio-iso" class="badge badge-bio-tier badge-bio-iso">ISO_QUARANTINE_CONTAGION</span>
        <span id="test-badge-bio-medical" class="badge badge-bio-tier badge-bio-medical">MEDICAL_RECOVERY_SINGLE_PET</span>
        <span id="test-badge-bio-cleared" class="badge badge-bio-tier badge-bio-cleared">BIOSECURITY_CLEARED_STANDARD</span>
        <span id="test-badge-foster-metric" class="badge badge-foster-metric">Contagion: PARVO_SUSPECTED • Host Readiness: 100/100</span>
        <div id="test-foster-iso-quarantine-banner" class="foster-iso-quarantine-banner">BIOSECURITY ISOLATION PROTOCOL REQUIRED</div>
        <div id="test-foster-authorization-banner" class="foster-authorization-banner">FOSTER BIOSECURITY AUTHORIZED: FOSTER-AUTH-101</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-foster-pending',
        '#test-badge-foster-staged',
        '#test-badge-foster-active',
        '#test-badge-bio-iso',
        '#test-badge-bio-medical',
        '#test-badge-bio-cleared',
        '#test-badge-foster-metric',
        '#test-foster-iso-quarantine-banner',
        '#test-foster-authorization-banner',
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
