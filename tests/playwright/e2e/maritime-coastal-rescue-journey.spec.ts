import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Maritime, Coastal & Tidal Mudflat Pet Rescue Operations Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, offshore vessel overboard evaluation, suction relief & saltwater O2 step completion, harbor deck safe clearance token issuance, and salt marsh airboat simulation', async ({
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

    // Step 1: Cockpit Load & Seeded Operations
    await page.goto(`${WEB_FRONTEND_URL}/maritime-coastal-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#maritime-coastal-rescue-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Maritime, Coastal & Tidal Mudflat Pet Rescue Operations Cockpit'
    );

    const card101 = page.locator('#maritime-operations-list .mc-op-card[data-op-id="MC-OP-101"]', {
      hasText: 'Anchor (',
    });
    const card102 = page.locator('#maritime-operations-list .mc-op-card[data-op-id="MC-OP-102"]', {
      hasText: 'Coral (',
    });
    const card103 = page.locator('#maritime-operations-list .mc-op-card[data-op-id="MC-OP-103"]', {
      hasText: 'Skipper (',
    });

    await expect(card101).toBeVisible();
    await expect(card102).toBeVisible();
    await expect(card103).toBeVisible();

    // Step 2: Evaluate custom offshore vessel overboard operation ("Poseidon")
    await page.locator('#eval-pet-name').fill('Poseidon');
    await page.locator('#eval-species').fill('Dog - Golden Retriever');
    await page.locator('#eval-weight-kg').fill('31.5');
    await page.locator('#eval-maritime-zone').selectOption('OFFSHORE_VESSEL_OVERBOARD');
    await page.locator('#eval-tide-surge-cm').fill('48.0');
    await page.locator('#eval-swell-height-m').fill('2.7');
    await page.locator('#eval-water-temp-c').fill('9.2');
    await page.locator('#eval-mud-suction-cm').fill('0.0');
    await page.locator('#eval-saltwater-aspiration').check();
    await page.locator('#eval-air-lance-deployed').uncheck();
    await page.locator('#eval-pfd-secured').uncheck();

    await page.locator('#btn-evaluate-maritime-op').click();

    const poseidonInitialCard = page
      .locator('#maritime-operations-list .mc-op-card', {
        hasText: 'Poseidon (',
      })
      .first();
    await expect(poseidonInitialCard).toBeVisible();
    const poseidonOpId = await poseidonInitialCard.getAttribute('data-op-id');
    expect(poseidonOpId).toBeTruthy();

    const poseidonCard = page.locator(
      `#maritime-operations-list .mc-op-card[data-op-id="${poseidonOpId}"]`
    );
    await expect(poseidonCard).toHaveAttribute('data-status', 'TIDAL_FLOOD_ASPIRATION_CRITICAL');
    await expect(poseidonCard.locator('.mc-tidal-alert-banner')).toContainText(
      'CRITICAL TIDAL / ASPIRATION EMERGENCY'
    );

    // Step 3: Complete all 4 maritime protocol steps on Poseidon's card
    for (const stepId of [
      'STEP-TIDE-SUCTION-RELIEF',
      'STEP-VESSEL-PFD-RIGGING',
      'STEP-SALTWATER-ASPIRATION-O2',
      'STEP-HARBOR-DECK-DECON-WARM',
    ]) {
      const cb = poseidonCard.locator(`.mc-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(poseidonCard).toHaveAttribute('data-status', 'HOVERCRAFT_MUD_SLED_STAGED');
    await expect(poseidonCard.locator('.badge-mc-status.badge-mc-staged')).toContainText(
      'HOVERCRAFT_MUD_SLED_STAGED'
    );

    // Step 4: Issue Harbor Deck Safe Extraction Clearance
    await poseidonCard.locator('.mc-issue-clearance-btn').click();

    await expect(poseidonCard).toHaveAttribute('data-status', 'EXTRACTED_HARBOR_DECK_SAFE');
    await expect(poseidonCard.locator('.badge-mc-status.badge-mc-safe')).toContainText(
      'EXTRACTED_HARBOR_DECK_SAFE'
    );
    await expect(poseidonCard.locator('.mc-clearance-banner')).toContainText('MARITIME-SAFE-');

    // Step 5: Trigger Salt Marsh Airboat scenario button and issue clearance
    await page.locator('.mc-scenario-btn[data-scenario="salt-marsh-airboat"]').click();

    const marshCard = page
      .locator('#maritime-operations-list .mc-op-card', {
        hasText: 'Marsh (',
      })
      .first();
    await expect(marshCard).toBeVisible();

    await marshCard.locator('.mc-issue-clearance-btn').click();
    await expect(marshCard).toHaveAttribute('data-status', 'EXTRACTED_HARBOR_DECK_SAFE');
    await expect(marshCard.locator('.mc-clearance-banner')).toContainText('MARITIME-SAFE-');

    expect(cspErrors).toEqual([]);
  });
});
