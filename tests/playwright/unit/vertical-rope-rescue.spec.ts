import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

function parseRgb(rgbStr: string): [number, number, number] {
  const match = rgbStr.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i);
  if (!match) return [0, 0, 0];
  return [parseInt(match[1], 10), parseInt(match[2], 10), parseInt(match[3], 10)];
}

function relativeLuminance([r, g, b]: [number, number, number]): number {
  const sRGB = [r / 255, g / 255, b / 255];
  const [R, G, B] = sRGB.map((c) =>
    c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)
  );
  return 0.2126 * R + 0.7152 * G + 0.0722 * B;
}

function contrastRatio(fgStr: string, bgStr: string): number {
  const l1 = relativeLuminance(parseRgb(fgStr));
  const l2 = relativeLuminance(parseRgb(bgStr));
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

test.describe('High-Rise, Cliffside, Tree-Canopy & Technical Rope/Arborist Vertical Rescue Cockpit UI (/vertical-rope-rescue)', () => {
  test.describe.configure({ mode: 'serial' });

  test('1. /vertical-rope-rescue renders seeded ops (VR-OP-101, VR-OP-102, VR-OP-103), summary KPI counters, protocol steps, and fall-shock alert banner with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/vertical-rope-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#vertical-rope-rescue-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'High-Rise, Cliffside, Tree-Canopy & Technical Rope/Arborist Vertical Rescue Cockpit'
    );

    await expect(page.locator('#stat-total-vertical-ops')).toBeVisible();
    await expect(page.locator('#stat-critical-fall-alerts')).toBeVisible();
    await expect(page.locator('#stat-rigged-twin-lines')).toBeVisible();
    await expect(page.locator('#stat-lowered-safe-count')).toBeVisible();

    const card101 = page.locator('#vr-operations-list .vr-op-card[data-op-id="VR-OP-101"]');
    const card102 = page.locator('#vr-operations-list .vr-op-card[data-op-id="VR-OP-102"]');
    const card103 = page.locator('#vr-operations-list .vr-op-card[data-op-id="VR-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-status', 'FALL_SHOCK_HAZARD_CRITICAL');
    await expect(card101.locator('.badge-vr-status.badge-vr-critical')).toContainText('FALL_SHOCK_HAZARD_CRITICAL');
    await expect(card101.locator('.badge-vr-env').first()).toContainText('TREE_CANOPY_CROWN');
    await expect(card101.locator('.vr-fall-alert-banner')).toContainText('HIGH-RISE FALL-SHOCK & CANOPY SHEAR ALERT');
    await expect(card101.locator('.badge-vr-flag').first()).toContainText('EXTREME_VERTICAL_FALL_TRAUMA_RISK');

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-status', 'TWIN_TENSION_RIGGED_ASCENT');
    await expect(card102.locator('.badge-vr-status.badge-vr-rigged')).toContainText('TWIN_TENSION_RIGGED_ASCENT');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-status', 'LOWERED_GROUND_SAFE');
    await expect(card103.locator('.badge-vr-status.badge-vr-lowered')).toContainText('LOWERED_GROUND_SAFE');
    await expect(card103.locator('.vr-clearance-banner')).toContainText('VERTICAL-SAFE-5E8A29C1D407');

    await expect(page.locator('.vr-scenario-btn[data-scenario="canopy-crown-gale"]')).toBeVisible();
    await expect(page.locator('.vr-scenario-btn[data-scenario="cliff-ledge-crag"]')).toBeVisible();
    await expect(page.locator('.vr-scenario-btn[data-scenario="highrise-balcony-ledge"]')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Clicking canopy-crown-gale scenario simulator evaluates a critical tree-canopy operation with .vr-fall-alert-banner and EXTREME_VERTICAL_FALL_TRAUMA_RISK', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/vertical-rope-rescue`);
    await page.waitForLoadState('networkidle');

    await page.locator('.vr-scenario-btn[data-scenario="canopy-crown-gale"]').click();

    const willowCard = page.locator('#vr-operations-list .vr-op-card', {
      hasText: 'Willow',
    }).first();

    await expect(willowCard).toBeVisible();
    await expect(willowCard).toHaveAttribute('data-status', 'FALL_SHOCK_HAZARD_CRITICAL');
    await expect(willowCard.locator('.badge-vr-status.badge-vr-critical')).toContainText('FALL_SHOCK_HAZARD_CRITICAL');
    await expect(willowCard.locator('.vr-fall-alert-banner')).toBeVisible();
    await expect(willowCard.locator('.badge-vr-flag', { hasText: 'EXTREME_VERTICAL_FALL_TRAUMA_RISK' })).toBeVisible();
    await expect(willowCard.locator('.badge-vr-flag', { hasText: 'CRITICAL_DEADWOOD_WIND_SHEAR_HAZARD' })).toBeVisible();
  });

  test('3. Completing protocol checkboxes (.vr-step-checkbox) advances rigging readiness to 100% and transitions status to TWIN_TENSION_RIGGED_ASCENT', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/vertical-rope-rescue`);
    await page.waitForLoadState('networkidle');

    await page.locator('.vr-scenario-btn[data-scenario="cliff-ledge-crag"]').click();

    const summitCard = page.locator('#vr-operations-list .vr-op-card', {
      hasText: 'Summit',
    }).first();
    await expect(summitCard).toBeVisible();
    const opId = await summitCard.getAttribute('data-op-id');
    expect(opId).toBeTruthy();

    const cardByOpId = page.locator(`#vr-operations-list .vr-op-card[data-op-id="${opId}"]`);
    const checkboxes = cardByOpId.locator('.vr-step-checkbox');
    const count = await checkboxes.count();
    expect(count).toBe(5);

    for (let i = 0; i < count; i++) {
      const cb = cardByOpId.locator('.vr-step-checkbox').nth(i);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
      await expect(cardByOpId.locator('.vr-step-checkbox').nth(i)).toBeChecked();
    }

    await expect(cardByOpId).toHaveAttribute('data-status', 'TWIN_TENSION_RIGGED_ASCENT');
    await expect(cardByOpId.locator('.badge-vr-status.badge-vr-rigged')).toContainText('TWIN_TENSION_RIGGED_ASCENT');
    await expect(cardByOpId).toContainText('Rigging Readiness: 100%');
  });

  test('4. Clicking .vr-issue-clearance-btn on a fully rigged operation transitions status to LOWERED_GROUND_SAFE and renders .vr-clearance-banner with VERTICAL-SAFE- certificate', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/vertical-rope-rescue`);
    await page.waitForLoadState('networkidle');

    const card102 = page.locator('#vr-operations-list .vr-op-card[data-op-id="VR-OP-102"]');
    await expect(card102).toBeVisible();

    await card102.locator('.vr-issue-clearance-btn').click();

    await expect(card102).toHaveAttribute('data-status', 'LOWERED_GROUND_SAFE');
    await expect(card102.locator('.badge-vr-status.badge-vr-lowered')).toContainText('LOWERED_GROUND_SAFE');
    await expect(card102.locator('.vr-clearance-banner')).toBeVisible();
    await expect(card102.locator('.vr-clearance-banner')).toContainText('VERTICAL-SAFE-');
  });

  test('5. Enforces WCAG AAA contrast ratio (> 7.1:1) on .badge-vr-critical, .badge-vr-rigged, .badge-vr-lowered, .badge-vr-env, .badge-vr-flag, .vr-fall-alert-banner, and .vr-clearance-banner in both dark and light themes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/vertical-rope-rescue`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-vertical-rescue-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-vr-critical" class="badge badge-vr-status badge-vr-critical">FALL_SHOCK_HAZARD_CRITICAL</span>
        <span id="test-badge-vr-rigged" class="badge badge-vr-status badge-vr-rigged">TWIN_TENSION_RIGGED_ASCENT</span>
        <span id="test-badge-vr-lowered" class="badge badge-vr-status badge-vr-lowered">LOWERED_GROUND_SAFE</span>
        <span id="test-badge-vr-env" class="badge badge-vr-env">TREE_CANOPY_CROWN</span>
        <span id="test-badge-vr-flag" class="badge badge-vr-flag">EXTREME_VERTICAL_FALL_TRAUMA_RISK</span>
        <div id="test-vr-fall-alert-banner" class="vr-fall-alert-banner">HIGH-RISE FALL-SHOCK &amp; CANOPY SHEAR ALERT</div>
        <div id="test-vr-clearance-banner" class="vr-clearance-banner">VERTICAL GROUND-SAFE LOWERING CERTIFIED: VERTICAL-SAFE-5E8A29C1D407</div>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-vr-critical',
        '#test-badge-vr-rigged',
        '#test-badge-vr-lowered',
        '#test-badge-vr-env',
        '#test-badge-vr-flag',
        '#test-vr-fall-alert-banner',
        '#test-vr-clearance-banner',
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
