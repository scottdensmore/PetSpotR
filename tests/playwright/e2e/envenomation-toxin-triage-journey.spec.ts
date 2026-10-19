import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Pit-Viper Envenomation, Bufo Toad Bufotoxin, Porcupine Quill & Toxic Flora Triage Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, porcupine retrograde quill & envenomation evaluation, 4-step protocol completion, TOXIN-SAFE clearance issuance, and toxin/status filter verification', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/envenomation-toxin-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#envenomation-toxin-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Pit-Viper Envenomation, Bufo Toad Bufotoxin, Porcupine Quill & Toxic Flora Triage Cockpit'
    );

    const copperCard = page.locator('#toxin-operations-list .toxin-op-card', {
      hasText: 'Copper (',
    });
    const brambleCard = page.locator('#toxin-operations-list .toxin-op-card', {
      hasText: 'Bramble (',
    });
    const mangoCard = page.locator('#toxin-operations-list .toxin-op-card', {
      hasText: 'Mango (',
    });

    await expect(copperCard).toBeVisible();
    await expect(brambleCard).toBeVisible();
    await expect(mangoCard).toBeVisible();

    // Step 2: Evaluate custom critical porcupine retrograde quill operation ("Juniper")
    await page.locator('#eval-pet-name').fill('Juniper');
    await page.locator('#eval-species').fill('Dog - Siberian Husky');
    await page.locator('#eval-weight-kg').fill('23.4');
    await page
      .locator('#eval-toxin-type')
      .selectOption('PORCUPINE_RETROGRADE_QUILLS');
    await page.locator('#eval-edema-spread').fill('1.8');
    await page.locator('#eval-echinocyte-pct').fill('0.0');
    await page.locator('#eval-quill-count').fill('28');
    await page.locator('#eval-antivenom-vials').fill('0');
    await page.locator('#eval-oral-flushed').check();
    await page.locator('#eval-quills-cut-avoided').check();

    await page.locator('#btn-submit-toxin-eval').click();

    const juniperInitialCard = page
      .locator('#toxin-operations-list .toxin-op-card', {
        hasText: 'Juniper (',
      })
      .first();
    await expect(juniperInitialCard).toBeVisible();
    const juniperOpId = await juniperInitialCard.getAttribute('data-op-id');
    expect(juniperOpId).toBeTruthy();

    const juniperCard = page.locator(
      `#toxin-operations-list .toxin-op-card[data-op-id="${juniperOpId}"]`
    );
    await expect(juniperCard).toHaveAttribute(
      'data-status',
      'COAGULOPATHY_NEUROTOXIC_CRITICAL'
    );
    await expect(juniperCard.locator('.toxin-contraindication-banner')).toContainText(
      'NEVER cut porcupine quills'
    );

    // Step 3: Complete all 4 protocol steps on Juniper's card
    for (const stepId of [
      'STEP-DEMARCATION-PERIMETER-MARK',
      'STEP-ORAL-FORWARD-LAVAGE-ANTIVENOM',
      'STEP-SEDATED-QUILL-EXTRACTION',
      'STEP-COAG-ECHINOCYTE-RECHECK',
    ]) {
      const cb = juniperCard.locator(`.toxin-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(juniperCard).toHaveAttribute(
      'data-status',
      'ANTIVENOM_LAVAGE_SEDATION_STAGED'
    );

    // Step 4: Issue Toxin Neutralization Clearance Token
    await juniperCard.locator('.toxin-issue-clearance-btn').click();

    await expect(juniperCard).toHaveAttribute(
      'data-status',
      'STABILIZED_NEUTRALIZED_CLEARED'
    );
    await expect(juniperCard.locator('.toxin-clearance-banner')).toContainText('TOXIN-SAFE-');

    // Step 5: Filter by STABILIZED_NEUTRALIZED_CLEARED and verify Juniper and Mango remain visible
    await page.locator('#filter-status').selectOption('STABILIZED_NEUTRALIZED_CLEARED');
    await expect(
      page.locator('#toxin-operations-list .toxin-op-card', { hasText: 'Juniper (' })
    ).toBeVisible();
    await expect(
      page.locator('#toxin-operations-list .toxin-op-card', { hasText: 'Mango (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
