import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Hypoadrenocorticism (Addisonian Crisis), Na:K Ratio < 27, Dexamethasone-SP & DOCP Triage Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Louie", staging 0.9% NaCl/Dex-SP/ACTH Stim regimen, logging electrolyte & post-ACTH cortisol check, issuing DOCP & Prednisone discharge clearance, and filtering by DOCP_PREDNISONE_EUKALEMIC_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/addisonian-crisis-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#addisonian-crisis-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Hypoadrenocorticism (Addisonian Crisis), Na:K Ratio < 27, Dexamethasone-SP & DOCP Triage Cockpit'
    );

    await expect(
      page.locator('#addisonian-operations-list .addisonian-op-card', { hasText: 'Curly (' })
    ).toBeVisible();
    await expect(
      page.locator('#addisonian-operations-list .addisonian-op-card', { hasText: 'Pippa (' })
    ).toBeVisible();
    await expect(
      page.locator('#addisonian-operations-list .addisonian-op-card', { hasText: 'Archie (' })
    ).toBeVisible();

    // Step 2: Create new operation "Louie"
    await page.locator('#create-incident-id').fill('INC-AC-9604');
    await page.locator('#create-pet-name').fill('Louie');
    await page.locator('#create-species').selectOption('CANINE_TYPICAL_ADDISONS');
    await page.locator('#create-breed').fill('Bearded Collie');
    await page.locator('#create-weight-kg').fill('22.0');
    await page.locator('#create-sodium').fill('123.0');
    await page.locator('#create-potassium').fill('7.5');
    await page.locator('#create-blood-glucose').fill('56.0');
    await page.locator('#create-heart-rate').fill('54');
    await page.locator('#create-systolic-bp').fill('70');
    await page.locator('#create-resting-cortisol').fill('0.2');
    await page.locator('#btn-create-addisonian-op').click();

    const louieCard = page
      .locator('#addisonian-operations-list .addisonian-op-card', { hasText: 'Louie (' })
      .first();
    await expect(louieCard).toBeVisible();
    await expect(louieCard).toHaveAttribute('data-status', 'ADDISONIAN_SHOCK_HYPERKALEMIA_CRITICAL');

    // Step 3: Stage 0.9% NaCl / Dexamethasone-SP / ACTH Stim regimen for Louie
    await page.locator('#stage-weight-kg').fill('22.0');
    await page.locator('#stage-species').selectOption('CANINE_TYPICAL_ADDISONS');
    await page.locator('#stage-sodium').fill('129.0');
    await page.locator('#stage-potassium').fill('6.0');
    await page.locator('#stage-heart-rate').fill('84');
    await page.locator('#stage-systolic-bp').fill('94');
    await page.locator('#btn-stage-regimen').click();

    await expect(louieCard).toHaveAttribute('data-status', 'DEX_SP_NACL_ACTH_STIM_STAGED');
    await expect(page.locator('#regimen-nacl-bolus')).toContainText('440.0 mL');
    await expect(page.locator('#regimen-dex-sp')).toContainText('5.50 mg IV');
    await expect(page.locator('#regimen-cosyntropin')).toContainText('110.0 mcg IV');

    // Step 4: Log serial electrolyte & post-ACTH cortisol check for Louie
    await page.locator('#check-sodium').fill('138.0');
    await page.locator('#check-potassium').fill('4.8');
    await page.locator('#check-sodium-rise').fill('7.5');
    await page.locator('#check-post-acth-cortisol').fill('0.3');
    await page.locator('#check-heart-rate').fill('90');
    await page.locator('#check-callsign').fill('DR-ENDO-55');
    await page.locator('#check-notes').fill('Na+ rose 7.5 mEq/L/24h (<= 10.0 mEq/L/day target); post-ACTH cortisol 0.3 mcg/dL.');
    await page.locator('#btn-record-electrolyte-check').click();

    await expect(page.locator('#electrolyte-checks-list')).toContainText(
      'Na+ rose 7.5 mEq/L/24h (<= 10.0 mEq/L/day target); post-ACTH cortisol 0.3 mcg/dL.'
    );

    // Step 5: Issue DOCP & Oral Prednisone Eukalemic Discharge Clearance for Louie
    await page.locator('#clearance-final-ratio').fill('31.2');
    await page.locator('#clearance-final-potassium').fill('4.5');
    await page.locator('#clearance-vet-license').fill('VET-ENDO-88120');
    await page.locator('#clearance-summary').fill('Post-ACTH cortisol confirmed < 2.0 mcg/dL; DOCP 2.2 mg/kg SubQ + oral Prednisone initiated.');
    await page.locator('#clearance-cortisol-confirmed').check();
    await page.locator('#clearance-docp-administered').check();
    await page.locator('#clearance-prednisone-started').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(louieCard).toHaveAttribute('data-status', 'DOCP_PREDNISONE_EUKALEMIC_CLEARED');
    await expect(page.locator('#addisonian-clearance-banner')).toBeVisible();
    await expect(page.locator('#addisonian-clearance-banner')).toContainText('VET-ENDO-88120');

    // Step 6: Filter by DOCP_PREDNISONE_EUKALEMIC_CLEARED and verify Louie and Archie remain visible
    await page.locator('#filter-status').selectOption('DOCP_PREDNISONE_EUKALEMIC_CLEARED');
    await expect(
      page.locator('#addisonian-operations-list .addisonian-op-card', { hasText: 'Louie (' })
    ).toBeVisible();
    await expect(
      page.locator('#addisonian-operations-list .addisonian-op-card', { hasText: 'Archie (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
