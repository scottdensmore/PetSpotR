import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing & Acute Hemolytic Transfusion Reaction Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, emergency canine-to-feline xenotransfusion evaluation, 170-micron filter & biologic trial protocol completion, transfusion clearance issuance, and status filter verification', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/transfusion-bloodbank-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#transfusion-bloodbank-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing & Acute Hemolytic Transfusion Reaction Cockpit'
    );

    const saffronCard = page.locator('#bloodbank-operations-list .bloodbank-op-card', {
      hasText: 'Saffron (',
    });
    const rangerCard = page.locator('#bloodbank-operations-list .bloodbank-op-card', {
      hasText: 'Ranger (',
    });
    const cleoCard = page.locator('#bloodbank-operations-list .bloodbank-op-card', {
      hasText: 'Cleo (',
    });

    await expect(saffronCard).toBeVisible();
    await expect(rangerCard).toBeVisible();
    await expect(cleoCard).toBeVisible();

    // Step 2: Evaluate custom emergency canine-to-feline xenotransfusion operation ("Nyx")
    await page.locator('#eval-pet-name').fill('Nyx');
    await page.locator('#eval-species').fill('Cat - Abyssinian');
    await page.locator('#eval-weight-kg').fill('3.5');
    await page
      .locator('#eval-blood-group')
      .selectOption('EMERGENCY_CANINE_TO_FELINE_XENOTRANSFUSION');
    await page.locator('#eval-recipient-pcv').fill('8.5');
    await page.locator('#eval-donor-pcv').fill('46.0');
    await page.locator('#eval-target-pcv').fill('22.0');
    await page.locator('#eval-crossmatch-ok').check();
    await page.locator('#eval-micron-filter').check();
    await page.locator('#eval-lrs-avoided').check();

    await page.locator('#btn-submit-bloodbank-eval').click();

    const nyxInitialCard = page
      .locator('#bloodbank-operations-list .bloodbank-op-card', {
        hasText: 'Nyx (',
      })
      .first();
    await expect(nyxInitialCard).toBeVisible();
    const nyxOpId = await nyxInitialCard.getAttribute('data-op-id');
    expect(nyxOpId).toBeTruthy();

    const nyxCard = page.locator(
      `#bloodbank-operations-list .bloodbank-op-card[data-op-id="${nyxOpId}"]`
    );
    await expect(nyxCard).toHaveAttribute(
      'data-status',
      'ALLOANTIBODY_HEMOLYTIC_ANAPHYLAXIS_CRITICAL'
    );
    await expect(nyxCard.locator('.bloodbank-alloantibody-alert-banner')).toContainText('alloantibodies');

    // Step 3: Complete all 4 blood typing, 170-micron filter + 0.9% NaCl line, 15-min biologic trial & post-PCV check protocol steps on Nyx's card
    for (const stepId of [
      'STEP-AB-DEA11-TYPING-SLIDE-CROSSMATCH',
      'STEP-170MICRON-FILTER-09NACL-ONLY-LINE',
      'STEP-SLOW-025MLKG-15MIN-BIOLOGIC-TRIAL',
      'STEP-POST-PCV-TOTAL-SOLIDS-HEMOLYSIS-CHECK',
    ]) {
      const cb = nyxCard.locator(`.bloodbank-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(nyxCard).toHaveAttribute(
      'data-status',
      'CROSSMATCHED_MICRON_FILTER_RATE_STAGED'
    );

    // Step 4: Issue Transfusion PCV Restoration Clearance Token
    await nyxCard.locator('.bloodbank-issue-clearance-btn').click();

    await expect(nyxCard).toHaveAttribute(
      'data-status',
      'TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE'
    );
    await expect(nyxCard.locator('.bloodbank-clearance-banner')).toContainText('BLOODBANK-SAFE-');

    // Step 5: Filter by TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE and verify Nyx and Cleo remain visible
    await page.locator('#filter-status').selectOption('TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE');
    await expect(
      page.locator('#bloodbank-operations-list .bloodbank-op-card', { hasText: 'Nyx (' })
    ).toBeVisible();
    await expect(
      page.locator('#bloodbank-operations-list .bloodbank-op-card', { hasText: 'Cleo (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
