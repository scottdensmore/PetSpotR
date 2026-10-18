import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Emergency Foster Network Capacity, Quarantine Biosecurity & Medical Foster Matching Cockpit E2E Journey', () => {
  test('Full 6-step operator journey: seeded cockpit load, Parvo suspected puppy evaluation with quarantine alert, HEPA airlock & vet tech competency verification, foster placement authorization, and Ringworm kitten simulation', async ({
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

    // Step 1: Cockpit Load & Seeded Placements
    await page.goto(`${WEB_FRONTEND_URL}/foster-network`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#foster-network-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Emergency Foster Network Capacity & Quarantine Biosecurity Cockpit'
    );

    const card101 = page.locator(
      '#foster-placements-list .foster-placement-card[data-placement-id="foster-match-101"]'
    );
    const card102 = page.locator(
      '#foster-placements-list .foster-placement-card[data-placement-id="foster-match-102"]'
    );
    const card103 = page.locator(
      '#foster-placements-list .foster-placement-card[data-placement-id="foster-match-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-placement-status', 'PENDING_BIOSECURITY_CLEARANCE');
    await expect(card101.locator('.foster-iso-quarantine-banner')).toContainText(
      'BIOSECURITY ISOLATION PROTOCOL REQUIRED'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-placement-status', 'QUARANTINE_VERIFIED_STAGED');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-placement-status', 'ACTIVE_IN_FOSTER_HOME');
    await expect(card103.locator('.foster-authorization-banner')).toContainText(
      'FOSTER BIOSECURITY AUTHORIZED: FOSTER-AUTH-FOSTER-MATCH-103'
    );

    // Step 2: Evaluate Emergency Foster Match for Parvo Suspected Puppy ("Jasper")
    await page.locator('#foster-pet-id').fill('pet-e2e-foster-jasper');
    await page.locator('#foster-pet-name').fill('Jasper');
    await page.locator('#foster-pet-species').selectOption('DOG');
    await page.locator('#foster-host-name').fill('Nurse Maya Lin (CVT)');
    await page.locator('#foster-contagion-flag').selectOption('PARVO_SUSPECTED');
    await page.locator('#foster-medical-need').selectOption('INJECTABLE_SUBQ_FLUIDS');
    await page.locator('#foster-resident-pets').fill('1');

    await page.locator('#btn-create-foster-placement').click();

    const jasperInitialCard = page
      .locator('#foster-placements-list .foster-placement-card', {
        hasText: 'Jasper (DOG · Host: Nurse Maya Lin (CVT) · 1 resident pets)',
      })
      .first();

    await expect(jasperInitialCard).toBeVisible();
    const jasperId = await jasperInitialCard.getAttribute('data-placement-id');
    expect(jasperId).toBeTruthy();

    const jasperCard = page.locator(
      `#foster-placements-list .foster-placement-card[data-placement-id="${jasperId}"]`
    );

    await expect(jasperCard).toHaveAttribute('data-placement-status', 'PENDING_BIOSECURITY_CLEARANCE');
    await expect(jasperCard.locator('.badge-foster-status.badge-foster-pending')).toContainText(
      'PENDING_BIOSECURITY_CLEARANCE'
    );
    await expect(jasperCard.locator('.badge-bio-tier.badge-bio-iso')).toContainText(
      'ISO_QUARANTINE_CONTAGION'
    );
    await expect(jasperCard.locator('.foster-iso-quarantine-banner')).toBeVisible();
    await expect(jasperCard.locator('.foster-iso-quarantine-banner')).toContainText(
      'BIOSECURITY ISOLATION PROTOCOL REQUIRED'
    );

    // Step 3: Verify HEPA Quarantine Rig & Vet Tech Med Cert
    await jasperCard.locator('.btn-verify-foster-bio').click();

    await expect(jasperCard).toHaveAttribute('data-placement-status', 'QUARANTINE_VERIFIED_STAGED');
    await expect(jasperCard.locator('.badge-foster-status.badge-foster-staged')).toContainText(
      'QUARANTINE_VERIFIED_STAGED'
    );
    await expect(jasperCard.locator('.badge-foster-metric')).toContainText(
      'Host Readiness: 100/100'
    );
    await expect(jasperCard.locator('.badge-bio-tier.badge-bio-medical')).toContainText(
      'MEDICAL_RECOVERY_SINGLE_PET'
    );

    // Step 4: Complete Biosecurity Clearance & Authorize Foster Placement
    await jasperCard.locator('.btn-activate-foster-placement').click();

    await expect(jasperCard).toHaveAttribute('data-placement-status', 'ACTIVE_IN_FOSTER_HOME');
    await expect(jasperCard.locator('.badge-foster-status.badge-foster-active')).toContainText(
      'ACTIVE_IN_FOSTER_HOME'
    );
    await expect(jasperCard.locator('.badge-bio-tier.badge-bio-cleared')).toContainText(
      'BIOSECURITY_CLEARED_STANDARD'
    );
    await expect(jasperCard.locator('.foster-authorization-banner')).toBeVisible();
    await expect(jasperCard.locator('.foster-authorization-banner')).toContainText(
      'FOSTER BIOSECURITY AUTHORIZED: FOSTER-AUTH-'
    );

    // Step 5: Simulate Ringworm Kitten & Authorize Placement
    await page.locator('#btn-sim-ringworm-cat').click();

    const pendingMochiCard = page
      .locator('#foster-placements-list .foster-placement-card[data-placement-status="PENDING_BIOSECURITY_CLEARANCE"]', {
        hasText: 'Mochi',
      })
      .first();

    await expect(pendingMochiCard).toBeVisible();
    const mochiId = await pendingMochiCard.getAttribute('data-placement-id');
    expect(mochiId).toBeTruthy();

    const mochiCard = page.locator(
      `#foster-placements-list .foster-placement-card[data-placement-id="${mochiId}"]`
    );
    await expect(mochiCard).toHaveAttribute('data-placement-status', 'PENDING_BIOSECURITY_CLEARANCE');
    await expect(mochiCard.locator('.badge-bio-tier.badge-bio-iso')).toContainText(
      'ISO_QUARANTINE_CONTAGION'
    );

    await mochiCard.locator('.btn-activate-foster-placement').click();

    await expect(mochiCard).toHaveAttribute('data-placement-status', 'ACTIVE_IN_FOSTER_HOME');
    await expect(mochiCard.locator('.badge-foster-status.badge-foster-active')).toContainText(
      'ACTIVE_IN_FOSTER_HOME'
    );
    await expect(mochiCard.locator('.badge-bio-tier.badge-bio-cleared')).toContainText(
      'BIOSECURITY_CLEARED_STANDARD'
    );
    await expect(mochiCard.locator('.foster-authorization-banner')).toBeVisible();
    await expect(mochiCard.locator('.foster-authorization-banner')).toContainText(
      'FOSTER BIOSECURITY AUTHORIZED: FOSTER-AUTH-'
    );

    // Step 6: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
