import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Aquatic, Floodwater & Ice-Rescue Swiftwater Canine/Feline Extraction Cockpit E2E Journey', () => {
  test('Full 6-step operator journey: seeded cockpit load, 34°F fractured ice-shelf breakthrough dog evaluation with cold-shock alert, NFPA 1670 throw-bag & inflatable sled deployment, shoreline extraction & Bair Hugger rewarming verification, and 8.0-knot flash flood storm channel simulation', async ({
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

    // Step 1: Cockpit Load & Seeded Ops
    await page.goto(`${WEB_FRONTEND_URL}/swiftwater-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#swiftwater-rescue-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Aquatic, Floodwater & Ice-Rescue Swiftwater Canine/Feline Extraction Cockpit'
    );

    const card101 = page.locator(
      '#sw-ops-list .sw-op-card[data-op-id="sw-op-101"]'
    );
    const card102 = page.locator(
      '#sw-ops-list .sw-op-card[data-op-id="sw-op-102"]'
    );
    const card103 = page.locator(
      '#sw-ops-list .sw-op-card[data-op-id="sw-op-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-extraction-status', 'STRANDED_IN_SWIFTWATER');
    await expect(card101.locator('.sw-coldshock-alert-banner')).toContainText(
      'COLD SHOCK & CIRCUM-RESCUE COLLAPSE ALERT'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-extraction-status', 'THROW_BAG_RIGGED_APPROACH');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute(
      'data-extraction-status',
      'EXTRACTED_WARMING_STABLE'
    );
    await expect(card103.locator('.sw-clearance-banner')).toContainText(
      'SWIFTWATER EXTRACTION & REWARMING VERIFIED: SWIFTWATER-SAFE-SW-OP-103'
    );

    // Step 2: Evaluate Swiftwater Rescue Op for Stranded Dog ("Sitka") in Fractured Lake Ice Shelf
    await page.locator('#sw-pet-id').fill('pet-e2e-sw-sitka');
    await page.locator('#sw-pet-name').fill('Sitka');
    await page.locator('#sw-pet-species').selectOption('DOG');
    await page.locator('#sw-waterway-name').fill('Haller Lake Fractured Ice Shelf');
    await page.locator('#sw-hazard-env').selectOption('FRACTURED_LAKE_ICE_SHELF');
    await page.locator('#sw-water-temp').fill('34');
    await page.locator('#sw-current-knots').fill('2.0');

    await page.locator('#btn-create-sw-op').click();

    const sitkaInitialCard = page
      .locator('#sw-ops-list .sw-op-card', {
        hasText: 'Sitka (DOG · Haller Lake Fractured Ice Shelf · FRACTURED_LAKE_ICE_SHELF)',
      })
      .first();

    await expect(sitkaInitialCard).toBeVisible();
    const sitkaId = await sitkaInitialCard.getAttribute('data-op-id');
    expect(sitkaId).toBeTruthy();

    const sitkaCard = page.locator(
      `#sw-ops-list .sw-op-card[data-op-id="${sitkaId}"]`
    );

    await expect(sitkaCard).toHaveAttribute('data-extraction-status', 'STRANDED_IN_SWIFTWATER');
    await expect(sitkaCard.locator('.badge-sw-status.badge-sw-stranded')).toContainText(
      'STRANDED_IN_SWIFTWATER'
    );
    await expect(
      sitkaCard.locator('.badge-hydro-risk.badge-hydro-extreme')
    ).toContainText('EXTREME_ICE_HYPOTHERMIA_RISK');
    await expect(sitkaCard.locator('.sw-coldshock-alert-banner')).toBeVisible();
    await expect(sitkaCard.locator('.sw-coldshock-alert-banner')).toContainText(
      'COLD SHOCK & CIRCUM-RESCUE COLLAPSE ALERT'
    );

    // Step 3: Deploy Throw-Bag Rigging & Inflatable Ice Sled
    await sitkaCard.locator('.btn-deploy-sw-rigging').click();

    await expect(sitkaCard).toHaveAttribute('data-extraction-status', 'THROW_BAG_RIGGED_APPROACH');
    await expect(
      sitkaCard.locator('.badge-sw-status.badge-sw-rigged')
    ).toContainText('THROW_BAG_RIGGED_APPROACH');
    await expect(sitkaCard.locator('.badge-sw-metric')).toContainText(
      'Rigging Readiness: 100/100'
    );
    await expect(
      sitkaCard.locator('.badge-hydro-risk.badge-hydro-elevated')
    ).toContainText('ELEVATED_FLOOD_CURRENT_RISK');

    // Step 4: Complete Shoreline Extraction & Bair Hugger Rewarming
    await sitkaCard.locator('.btn-complete-sw-extraction').click();

    await expect(sitkaCard).toHaveAttribute(
      'data-extraction-status',
      'EXTRACTED_WARMING_STABLE'
    );
    await expect(
      sitkaCard.locator('.badge-sw-status.badge-sw-extracted')
    ).toContainText('EXTRACTED_WARMING_STABLE');
    await expect(sitkaCard.locator('.badge-hydro-risk.badge-hydro-safe')).toContainText(
      'SHORELINE_REWARMED_SAFE'
    );
    await expect(sitkaCard.locator('.sw-clearance-banner')).toBeVisible();
    await expect(sitkaCard.locator('.sw-clearance-banner')).toContainText(
      'SWIFTWATER EXTRACTION & REWARMING VERIFIED: SWIFTWATER-SAFE-'
    );

    // Step 5: Simulate 8.0-Knot Flash Flood Storm Channel Washout & Complete Shoreline Extraction
    await page.locator('#btn-sim-flash-flood-culvert').click();

    const pendingCascadeCard = page
      .locator(
        '#sw-ops-list .sw-op-card[data-extraction-status="STRANDED_IN_SWIFTWATER"]',
        {
          hasText: 'Cascade',
        }
      )
      .first();

    await expect(pendingCascadeCard).toBeVisible();
    const cascadeId = await pendingCascadeCard.getAttribute('data-op-id');
    expect(cascadeId).toBeTruthy();

    const cascadeCard = page.locator(
      `#sw-ops-list .sw-op-card[data-op-id="${cascadeId}"]`
    );
    await expect(cascadeCard).toHaveAttribute('data-extraction-status', 'STRANDED_IN_SWIFTWATER');
    await expect(
      cascadeCard.locator('.badge-hydro-risk.badge-hydro-extreme')
    ).toContainText('EXTREME_ICE_HYPOTHERMIA_RISK');

    await cascadeCard.locator('.btn-complete-sw-extraction').click();

    await expect(cascadeCard).toHaveAttribute(
      'data-extraction-status',
      'EXTRACTED_WARMING_STABLE'
    );
    await expect(
      cascadeCard.locator('.badge-sw-status.badge-sw-extracted')
    ).toContainText('EXTRACTED_WARMING_STABLE');
    await expect(cascadeCard.locator('.badge-hydro-risk.badge-hydro-safe')).toContainText(
      'SHORELINE_REWARMED_SAFE'
    );
    await expect(cascadeCard.locator('.sw-clearance-banner')).toBeVisible();
    await expect(cascadeCard.locator('.sw-clearance-banner')).toContainText(
      'SWIFTWATER EXTRACTION & REWARMING VERIFIED: SWIFTWATER-SAFE-'
    );

    // Step 6: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
