import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Livestock, Equine & Exotic Companion Large-Animal Containment & Trailer Dispatch Cockpit E2E Journey', () => {
  test('Full 6-step operator journey: seeded cockpit load, 520kg Warmblood highway stampede evaluation, 12ft portable corral & gooseneck trailer staging, safe trailer load & paddock clearance verification, and 4-head alpaca herd breach simulation', async ({
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

    // Step 1: Cockpit Load & Seeded Dispatches
    await page.goto(`${WEB_FRONTEND_URL}/equine-livestock`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#equine-livestock-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Livestock, Equine & Large-Animal Containment Cockpit'
    );

    const card101 = page.locator(
      '#eq-dispatches-list .eq-dispatch-card[data-dispatch-id="eq-dsp-101"]'
    );
    const card102 = page.locator(
      '#eq-dispatches-list .eq-dispatch-card[data-dispatch-id="eq-dsp-102"]'
    );
    const card103 = page.locator(
      '#eq-dispatches-list .eq-dispatch-card[data-dispatch-id="eq-dsp-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-dispatch-status', 'LOOSE_HERD_CORRIDOR_ALERT');
    await expect(card101.locator('.eq-stampede-alert-banner')).toContainText(
      'HIGHWAY STAMPEDE & SIREN BLACKOUT PROTOCOL'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-dispatch-status', 'PORTABLE_CORRAL_STAGED');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute(
      'data-dispatch-status',
      'LOADED_SAFE_PADDOCK_CUSTODY'
    );
    await expect(card103.locator('.eq-paddock-banner')).toContainText(
      'GOOSENECK TRAILER & PADDOCK CUSTODY VERIFIED: PADDOCK-SAFE-EQ-DSP-103'
    );

    // Step 2: Evaluate Large-Animal Dispatch for 520kg Warmblood ("Apollo")
    await page.locator('#eq-pet-id').fill('pet-e2e-eq-apollo');
    await page.locator('#eq-animal-name').fill('Apollo');
    await page.locator('#eq-animal-category').selectOption('EQUINE_WARMBLOOD');
    await page.locator('#eq-head-count').fill('1');
    await page.locator('#eq-weight-kg').fill('520');
    await page.locator('#eq-breach-location').fill('SR-202 Bridle Trail Highway Crossing');

    await page.locator('#btn-create-eq-dispatch').click();

    const apolloInitialCard = page
      .locator('#eq-dispatches-list .eq-dispatch-card', {
        hasText: 'Apollo (EQUINE_WARMBLOOD · 1 head · 520kg)',
      })
      .first();

    await expect(apolloInitialCard).toBeVisible();
    const apolloId = await apolloInitialCard.getAttribute('data-dispatch-id');
    expect(apolloId).toBeTruthy();

    const apolloCard = page.locator(
      `#eq-dispatches-list .eq-dispatch-card[data-dispatch-id="${apolloId}"]`
    );

    await expect(apolloCard).toHaveAttribute('data-dispatch-status', 'LOOSE_HERD_CORRIDOR_ALERT');
    await expect(apolloCard.locator('.badge-eq-status.badge-eq-loose')).toContainText(
      'LOOSE_HERD_CORRIDOR_ALERT'
    );
    await expect(
      apolloCard.locator('.badge-collision-risk.badge-collision-critical')
    ).toContainText('CRITICAL_HIGHWAY_STAMPEDE_RISK');
    await expect(apolloCard.locator('.eq-stampede-alert-banner')).toBeVisible();
    await expect(apolloCard.locator('.eq-stampede-alert-banner')).toContainText(
      'HIGHWAY STAMPEDE & SIREN BLACKOUT PROTOCOL'
    );

    // Step 3: Stage 12ft Corral Panels & Gooseneck Stock Trailer
    await apolloCard.locator('.btn-stage-eq-corral').click();

    await expect(apolloCard).toHaveAttribute('data-dispatch-status', 'PORTABLE_CORRAL_STAGED');
    await expect(
      apolloCard.locator('.badge-eq-status.badge-eq-corralled')
    ).toContainText('PORTABLE_CORRAL_STAGED');
    await expect(apolloCard.locator('.badge-eq-metric')).toContainText(
      'Trailer Readiness: 100/100'
    );
    await expect(
      apolloCard.locator('.badge-collision-risk.badge-collision-elevated')
    ).toContainText('ELEVATED_PASTURE_PERIMETER_RISK');

    // Step 4: Complete Safe Trailer Load & Paddock Custody
    await apolloCard.locator('.btn-complete-eq-load').click();

    await expect(apolloCard).toHaveAttribute(
      'data-dispatch-status',
      'LOADED_SAFE_PADDOCK_CUSTODY'
    );
    await expect(
      apolloCard.locator('.badge-eq-status.badge-eq-loaded')
    ).toContainText('LOADED_SAFE_PADDOCK_CUSTODY');
    await expect(apolloCard.locator('.badge-collision-risk.badge-collision-safe')).toContainText(
      'CONTAINED_PADDOCK_SAFE'
    );
    await expect(apolloCard.locator('.eq-paddock-banner')).toBeVisible();
    await expect(apolloCard.locator('.eq-paddock-banner')).toContainText(
      'GOOSENECK TRAILER & PADDOCK CUSTODY VERIFIED: PADDOCK-SAFE-'
    );

    // Step 5: Simulate 4-Head Alpaca Herd Breach & Complete Safe Trailer Load
    await page.locator('#btn-sim-alpaca-herd-breach').click();

    const pendingCuscoCard = page
      .locator(
        '#eq-dispatches-list .eq-dispatch-card[data-dispatch-status="LOOSE_HERD_CORRIDOR_ALERT"]',
        {
          hasText: 'Cusco Quartet',
        }
      )
      .first();

    await expect(pendingCuscoCard).toBeVisible();
    const cuscoId = await pendingCuscoCard.getAttribute('data-dispatch-id');
    expect(cuscoId).toBeTruthy();

    const cuscoCard = page.locator(
      `#eq-dispatches-list .eq-dispatch-card[data-dispatch-id="${cuscoId}"]`
    );
    await expect(cuscoCard).toHaveAttribute('data-dispatch-status', 'LOOSE_HERD_CORRIDOR_ALERT');
    await expect(
      cuscoCard.locator('.badge-collision-risk.badge-collision-critical')
    ).toContainText('CRITICAL_HIGHWAY_STAMPEDE_RISK');

    await cuscoCard.locator('.btn-complete-eq-load').click();

    await expect(cuscoCard).toHaveAttribute(
      'data-dispatch-status',
      'LOADED_SAFE_PADDOCK_CUSTODY'
    );
    await expect(
      cuscoCard.locator('.badge-eq-status.badge-eq-loaded')
    ).toContainText('LOADED_SAFE_PADDOCK_CUSTODY');
    await expect(cuscoCard.locator('.badge-collision-risk.badge-collision-safe')).toContainText(
      'CONTAINED_PADDOCK_SAFE'
    );
    await expect(cuscoCard.locator('.eq-paddock-banner')).toBeVisible();
    await expect(cuscoCard.locator('.eq-paddock-banner')).toContainText(
      'GOOSENECK TRAILER & PADDOCK CUSTODY VERIFIED: PADDOCK-SAFE-'
    );

    // Step 6: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
