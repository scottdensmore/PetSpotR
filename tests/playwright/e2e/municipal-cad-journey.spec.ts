import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Municipal 311 / Animal Control Dispatch CAD Bridge, Roadway Hazard Alerts & Good Samaritan Safe-Capture Routing E2E Journey', () => {
  test('Full 6-step operator journey: seeded cockpit load, 311 freeway median panic call ingestion with DO-NOT-CHASE alert, DOT VMS & ACO L-barrier containment deployment, safe slip-lead custody resolution, and arterial bridge simulation', async ({
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

    // Step 1: Cockpit Load & Seeded CAD Calls
    await page.goto(`${WEB_FRONTEND_URL}/municipal-cad`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#municipal-cad-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Municipal 311 / Animal Control Dispatch CAD Bridge Cockpit'
    );

    const card101 = page.locator(
      '#cad-incidents-list .cad-incident-card[data-incident-id="cad-inc-101"]'
    );
    const card102 = page.locator(
      '#cad-incidents-list .cad-incident-card[data-incident-id="cad-inc-102"]'
    );
    const card103 = page.locator(
      '#cad-incidents-list .cad-incident-card[data-incident-id="cad-inc-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-dispatch-status', 'INBOUND_311_TRIAGE');
    await expect(card101.locator('.cad-no-chase-banner')).toContainText(
      'DO NOT CHASE — GOOD SAMARITAN CALMING PROTOCOL'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute(
      'data-dispatch-status',
      'HIGHWAY_CONTAINMENT_DISPATCHED'
    );

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-dispatch-status', 'RESOLVED_SAFE_CUSTODY');
    await expect(card103.locator('.cad-resolution-banner')).toContainText(
      'SAFE CUSTODY INTERCEPT VERIFIED: CAD-SAFE-CAD-INC-103'
    );

    // Step 2: Ingest Municipal 311 Call for Panicked Dog on I-90 Floating Bridge ("Apollo")
    await page.locator('#cad-call-number').fill('311-SEA-9501');
    await page.locator('#cad-pet-id').fill('pet-e2e-cad-apollo');
    await page.locator('#cad-pet-name').fill('Apollo');
    await page.locator('#cad-pet-species').selectOption('DOG');
    await page.locator('#cad-roadway-name').fill('I-90 Floating Bridge Westbound Center Median');
    await page.locator('#cad-speed-limit').fill('60');
    await page.locator('#cad-lane-count').fill('4');
    await page.locator('#cad-behavior-state').selectOption('BOLTING_INTO_TRAFFIC');

    await page.locator('#btn-create-cad-incident').click();

    const apolloInitialCard = page
      .locator('#cad-incidents-list .cad-incident-card', {
        hasText: 'Apollo (DOG · I-90 Floating Bridge Westbound Center Median · 60 MPH / 4 lanes)',
      })
      .first();

    await expect(apolloInitialCard).toBeVisible();
    const apolloIncidentId = await apolloInitialCard.getAttribute('data-incident-id');
    expect(apolloIncidentId).toBeTruthy();

    const apolloCard = page.locator(
      `#cad-incidents-list .cad-incident-card[data-incident-id="${apolloIncidentId}"]`
    );

    await expect(apolloCard).toHaveAttribute('data-dispatch-status', 'INBOUND_311_TRIAGE');
    await expect(apolloCard.locator('.badge-cad-status.badge-cad-triage')).toContainText(
      'INBOUND_311_TRIAGE'
    );
    await expect(apolloCard.locator('.badge-hazard-severity.badge-hazard-freeway')).toContainText(
      'FREEWAY_MEDIAN_CRITICAL'
    );
    await expect(apolloCard.locator('.cad-no-chase-banner')).toBeVisible();
    await expect(apolloCard.locator('.cad-no-chase-banner')).toContainText(
      'DO NOT CHASE — GOOD SAMARITAN CALMING PROTOCOL'
    );

    // Step 3: Deploy DOT VMS Speed Advisory & ACO L-Barrier Containment
    await apolloCard.locator('.btn-deploy-cad-containment').click();

    await expect(apolloCard).toHaveAttribute(
      'data-dispatch-status',
      'HIGHWAY_CONTAINMENT_DISPATCHED'
    );
    await expect(apolloCard.locator('.badge-cad-status.badge-cad-containment')).toContainText(
      'HIGHWAY_CONTAINMENT_DISPATCHED'
    );
    await expect(apolloCard.locator('.badge-cad-metric')).toContainText(
      'Containment Readiness: 100/100'
    );
    await expect(apolloCard.locator('.badge-hazard-severity.badge-hazard-arterial')).toContainText(
      'ARTERIAL_TRAFFIC_HIGH'
    );

    // Step 4: Confirm Safe Slip-Lead Capture & ACO Custody Handoff
    await apolloCard.locator('.btn-resolve-cad-custody').click();

    await expect(apolloCard).toHaveAttribute('data-dispatch-status', 'RESOLVED_SAFE_CUSTODY');
    await expect(apolloCard.locator('.badge-cad-status.badge-cad-resolved')).toContainText(
      'RESOLVED_SAFE_CUSTODY'
    );
    await expect(apolloCard.locator('.badge-hazard-severity.badge-hazard-safe')).toContainText(
      'CONTAINED_CORRIDOR_SAFE'
    );
    await expect(apolloCard.locator('.cad-resolution-banner')).toBeVisible();
    await expect(apolloCard.locator('.cad-resolution-banner')).toContainText(
      'SAFE CUSTODY INTERCEPT VERIFIED: CAD-SAFE-'
    );

    // Step 5: Simulate Arterial Bridge Sighting & Resolve Safe Custody
    await page.locator('#btn-sim-arterial-bridge').click();

    const pendingBridgeCard = page
      .locator('#cad-incidents-list .cad-incident-card[data-dispatch-status="INBOUND_311_TRIAGE"]', {
        hasText: 'Ballard Bridge Northbound Bascule Span',
      })
      .first();

    await expect(pendingBridgeCard).toBeVisible();
    const bridgeIncidentId = await pendingBridgeCard.getAttribute('data-incident-id');
    expect(bridgeIncidentId).toBeTruthy();

    const bridgeCard = page.locator(
      `#cad-incidents-list .cad-incident-card[data-incident-id="${bridgeIncidentId}"]`
    );
    await expect(bridgeCard).toHaveAttribute('data-dispatch-status', 'INBOUND_311_TRIAGE');

    await bridgeCard.locator('.btn-resolve-cad-custody').click();

    await expect(bridgeCard).toHaveAttribute('data-dispatch-status', 'RESOLVED_SAFE_CUSTODY');
    await expect(bridgeCard.locator('.badge-cad-status.badge-cad-resolved')).toContainText(
      'RESOLVED_SAFE_CUSTODY'
    );
    await expect(bridgeCard.locator('.badge-hazard-severity.badge-hazard-safe')).toContainText(
      'CONTAINED_CORRIDOR_SAFE'
    );
    await expect(bridgeCard.locator('.cad-resolution-banner')).toBeVisible();
    await expect(bridgeCard.locator('.cad-resolution-banner')).toContainText(
      'SAFE CUSTODY INTERCEPT VERIFIED: CAD-SAFE-'
    );

    // Step 6: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
