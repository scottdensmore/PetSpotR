import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('High-Rise, Cliffside, Tree-Canopy & Technical Rope/Arborist Vertical Rescue Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, high-rise balcony parapet feline evaluation, NFPA 2500 / ANSI Z133 5-step twin-tension rigging progression, and cryptographic ground-safe clearance certification', async ({
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

    // Step 1: Load Cockpit & Verify Seeded Operations
    await page.goto(`${WEB_FRONTEND_URL}/vertical-rope-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#vertical-rope-rescue-main')).toBeVisible();
    await expect(page.locator('#stat-total-vertical-ops')).toBeVisible();
    await expect(page.locator('#vr-operations-list .vr-op-card[data-op-id="VR-OP-101"]')).toBeVisible();
    await expect(page.locator('#vr-operations-list .vr-op-card[data-op-id="VR-OP-102"]')).toBeVisible();
    await expect(page.locator('#vr-operations-list .vr-op-card[data-op-id="VR-OP-103"]')).toBeVisible();

    // Step 2: Trigger High-Rise Balcony Parapet Scenario Simulator
    await page.locator('.vr-scenario-btn[data-scenario="highrise-balcony-ledge"]').click();

    const pixelInitialCard = page
      .locator('#vr-operations-list .vr-op-card', {
        hasText: 'Pixel',
      })
      .first();

    await expect(pixelInitialCard).toBeVisible();
    const pixelOpId = await pixelInitialCard.getAttribute('data-op-id');
    expect(pixelOpId).toBeTruthy();

    const pixelCard = page.locator(
      `#vr-operations-list .vr-op-card[data-op-id="${pixelOpId}"]`
    );

    await expect(pixelCard).toHaveAttribute('data-status', 'FALL_SHOCK_HAZARD_CRITICAL');
    await expect(pixelCard.locator('.vr-fall-alert-banner')).toBeVisible();
    await expect(
      pixelCard.locator('.badge-vr-flag', { hasText: 'EXTREME_VERTICAL_FALL_TRAUMA_RISK' })
    ).toBeVisible();
    await expect(
      pixelCard.locator('.badge-vr-flag', { hasText: 'FELINE_PANIC_LEAP_SYNDROME' })
    ).toBeVisible();

    // Step 3: Complete All 5 Vertical Protocol Steps
    const stepCount = await pixelCard.locator('.vr-step-checkbox').count();
    expect(stepCount).toBe(5);

    for (let i = 0; i < stepCount; i++) {
      const cb = pixelCard.locator('.vr-step-checkbox').nth(i);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
      await expect(pixelCard.locator('.vr-step-checkbox').nth(i)).toBeChecked();
    }

    await expect(pixelCard).toHaveAttribute('data-status', 'TWIN_TENSION_RIGGED_ASCENT');
    await expect(pixelCard.locator('.badge-vr-status.badge-vr-rigged')).toContainText(
      'TWIN_TENSION_RIGGED_ASCENT'
    );
    await expect(pixelCard).toContainText('Rigging Readiness: 100%');

    // Step 4: Issue Ground-Safe Vertical Clearance Certificate
    await pixelCard.locator('.vr-issue-clearance-btn').click();

    await expect(pixelCard).toHaveAttribute('data-status', 'LOWERED_GROUND_SAFE');
    await expect(pixelCard.locator('.badge-vr-status.badge-vr-lowered')).toContainText(
      'LOWERED_GROUND_SAFE'
    );
    await expect(pixelCard.locator('.vr-clearance-banner')).toBeVisible();
    await expect(pixelCard.locator('.vr-clearance-banner')).toContainText('VERTICAL-SAFE-');

    // Step 5: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
