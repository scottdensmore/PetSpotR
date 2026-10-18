import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Aviation & Cross-Jurisdiction Pet Airlift / Ground Relay Transport Logistics Cockpit E2E Journey', () => {
  test('Full 6-step operator journey: seeded cockpit load, brachycephalic airlift mission planning with hypoxia alert, USDA CVI & IATA crate pre-flight clearance launch, destination manifest verification, and ground relay simulation', async ({
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

    // Step 1: Cockpit Load & Seeded Missions
    await page.goto(`${WEB_FRONTEND_URL}/transport-relay`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#transport-relay-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Aviation & Cross-Jurisdiction Pet Airlift / Ground Relay Transport Logistics Cockpit'
    );

    const card101 = page.locator(
      '#relay-missions-list .relay-mission-card[data-mission-id="relay-msn-101"]'
    );
    const card102 = page.locator(
      '#relay-missions-list .relay-mission-card[data-mission-id="relay-msn-102"]'
    );
    const card103 = page.locator(
      '#relay-missions-list .relay-mission-card[data-mission-id="relay-msn-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-relay-status', 'PRE_FLIGHT_STAGING');
    await expect(card101.locator('.relay-hypoxia-alert-banner')).toContainText(
      'BRACHYCEPHALIC & ALTITUDE HYPOXIA ALERT'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-relay-status', 'IN_TRANSIT_ACTIVE_RELAY');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-relay-status', 'DELIVERED_DESTINATION_VERIFIED');
    await expect(card103.locator('.relay-manifest-banner')).toContainText(
      'INTERSTATE RELAY MANIFEST VERIFIED: RELAY-MANIFEST-RELAY-MSN-103'
    );

    // Step 2: Plan Brachycephalic Airlift Mission ("Buster")
    await page.locator('#relay-pet-id').fill('pet-e2e-relay-buster');
    await page.locator('#relay-pet-name').fill('Buster');
    await page.locator('#relay-pet-species').selectOption('DOG');
    await page.locator('#relay-transport-mode').selectOption('GENERAL_AVIATION_AIRLIFT');
    await page.locator('#relay-origin-hub').fill('KRNO Reno-Tahoe FBO');
    await page.locator('#relay-destination-hub').fill('KBFI Boeing Field Seattle');
    await page.locator('#relay-distance-miles').fill('580');
    await page.locator('#relay-altitude-feet').fill('8000');
    await page.locator('#relay-brachycephalic').check();

    await page.locator('#btn-create-relay-mission').click();

    const busterInitialCard = page
      .locator('#relay-missions-list .relay-mission-card', {
        hasText: 'Buster (DOG · KRNO Reno-Tahoe FBO',
      })
      .first();

    await expect(busterInitialCard).toBeVisible();
    const busterId = await busterInitialCard.getAttribute('data-mission-id');
    expect(busterId).toBeTruthy();

    const busterCard = page.locator(
      `#relay-missions-list .relay-mission-card[data-mission-id="${busterId}"]`
    );

    await expect(busterCard).toHaveAttribute('data-relay-status', 'PRE_FLIGHT_STAGING');
    await expect(busterCard.locator('.badge-relay-status.badge-relay-staging')).toContainText(
      'PRE_FLIGHT_STAGING'
    );
    await expect(busterCard.locator('.badge-thermal-risk.badge-thermal-hypoxia')).toContainText(
      'BRACHYCEPHALIC_CABIN_RISK'
    );
    await expect(busterCard.locator('.relay-hypoxia-alert-banner')).toBeVisible();
    await expect(busterCard.locator('.relay-hypoxia-alert-banner')).toContainText(
      'BRACHYCEPHALIC & ALTITUDE HYPOXIA ALERT'
    );

    // Step 3: Clear USDA CVI & IATA Crate to Launch Relay
    await busterCard.locator('.btn-launch-relay-leg').click();

    await expect(busterCard).toHaveAttribute('data-relay-status', 'IN_TRANSIT_ACTIVE_RELAY');
    await expect(busterCard.locator('.badge-relay-status.badge-relay-transit')).toContainText(
      'IN_TRANSIT_ACTIVE_RELAY'
    );
    await expect(busterCard.locator('.badge-relay-metric')).toContainText(
      'Flight Readiness: 100/100'
    );
    await expect(busterCard.locator('.badge-thermal-risk.badge-thermal-watch')).toContainText(
      'TARMAC_THERMAL_WATCH'
    );

    // Step 4: Complete Final FBO / Ground Handoff & Verify Manifest
    await busterCard.locator('.btn-complete-relay-delivery').click();

    await expect(busterCard).toHaveAttribute('data-relay-status', 'DELIVERED_DESTINATION_VERIFIED');
    await expect(busterCard.locator('.badge-relay-status.badge-relay-delivered')).toContainText(
      'DELIVERED_DESTINATION_VERIFIED'
    );
    await expect(busterCard.locator('.badge-thermal-risk.badge-thermal-safe')).toContainText(
      'PRESSURIZED_SAFE_ENVELOPE'
    );
    await expect(busterCard.locator('.relay-manifest-banner')).toBeVisible();
    await expect(busterCard.locator('.relay-manifest-banner')).toContainText(
      'INTERSTATE RELAY MANIFEST VERIFIED: RELAY-MANIFEST-'
    );

    // Step 5: Simulate Ground Relay & Complete Delivery
    await page.locator('#btn-sim-ground-relay').click();

    const pendingMapleCard = page
      .locator('#relay-missions-list .relay-mission-card[data-relay-status="PRE_FLIGHT_STAGING"]', {
        hasText: 'Maple',
      })
      .first();

    await expect(pendingMapleCard).toBeVisible();
    const mapleId = await pendingMapleCard.getAttribute('data-mission-id');
    expect(mapleId).toBeTruthy();

    const mapleCard = page.locator(
      `#relay-missions-list .relay-mission-card[data-mission-id="${mapleId}"]`
    );
    await expect(mapleCard).toHaveAttribute('data-relay-status', 'PRE_FLIGHT_STAGING');

    await mapleCard.locator('.btn-complete-relay-delivery').click();

    await expect(mapleCard).toHaveAttribute('data-relay-status', 'DELIVERED_DESTINATION_VERIFIED');
    await expect(mapleCard.locator('.badge-relay-status.badge-relay-delivered')).toContainText(
      'DELIVERED_DESTINATION_VERIFIED'
    );
    await expect(mapleCard.locator('.badge-thermal-risk.badge-thermal-safe')).toContainText(
      'PRESSURIZED_SAFE_ENVELOPE'
    );
    await expect(mapleCard.locator('.relay-manifest-banner')).toBeVisible();
    await expect(mapleCard.locator('.relay-manifest-banner')).toContainText(
      'INTERSTATE RELAY MANIFEST VERIFIED: RELAY-MANIFEST-'
    );

    // Step 6: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
