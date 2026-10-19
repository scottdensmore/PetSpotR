import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Diabetic Ketoacidosis (DKA), Regular Insulin CRI, Hypokalemia/Hypophosphatemia & Cerebral Edema Triage Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Biscuit", staging KCl/KPhos/Regular Insulin CRI regimen, logging serial q2h glucose & electrolyte check, issuing ketone-cleared SubQ Glargine discharge clearance, and filtering by KETONES_CLEARED_SUBQ_GLARGINE_READY', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/dka-insulin-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#dka-insulin-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Diabetic Ketoacidosis (DKA), Regular Insulin CRI, Hypokalemia/Hypophosphatemia & Cerebral Edema Triage Cockpit'
    );

    await expect(
      page.locator('#dka-operations-list .dka-op-card', { hasText: 'Winston (' })
    ).toBeVisible();
    await expect(
      page.locator('#dka-operations-list .dka-op-card', { hasText: 'Daisy (' })
    ).toBeVisible();
    await expect(
      page.locator('#dka-operations-list .dka-op-card', { hasText: 'Chester (' })
    ).toBeVisible();

    // Step 2: Create new operation "Biscuit"
    await page.locator('#create-incident-id').fill('INC-DKA-9304');
    await page.locator('#create-pet-name').fill('Biscuit');
    await page.locator('#create-species').selectOption('CANINE_DKA_PANCREATITIS');
    await page.locator('#create-breed').fill('Golden Retriever');
    await page.locator('#create-weight-kg').fill('24.0');
    await page.locator('#create-blood-glucose').fill('460.0');
    await page.locator('#create-bhb').fill('5.4');
    await page.locator('#create-venous-ph').fill('7.14');
    await page.locator('#create-potassium').fill('2.9');
    await page.locator('#create-phosphorus').fill('1.7');
    await page.locator('#create-osmolality').fill('324.0');
    await page.locator('#create-dehydration').fill('8.0');
    await page.locator('#btn-create-dka-op').click();

    const biscuitCard = page
      .locator('#dka-operations-list .dka-op-card', { hasText: 'Biscuit (' })
      .first();
    await expect(biscuitCard).toBeVisible();
    await expect(biscuitCard).toHaveAttribute('data-status', 'KETOACIDOSIS_DEHYDRATED_CRITICAL');

    // Step 3: Stage KCl / KPhos / Regular Insulin CRI regimen for Biscuit
    await page.locator('#stage-weight-kg').fill('24.0');
    await page.locator('#stage-species').selectOption('CANINE_DKA_PANCREATITIS');
    await page.locator('#stage-blood-glucose').fill('230.0');
    await page.locator('#stage-potassium').fill('3.6');
    await page.locator('#stage-phosphorus').fill('2.8');
    await page.locator('#stage-dehydration').fill('4.0');
    await page.locator('#btn-stage-regimen').click();

    await expect(biscuitCard).toHaveAttribute('data-status', 'KCL_PHOS_REGULAR_INSULIN_CRI_STAGED');
    await expect(page.locator('#regimen-insulin-cri')).toContainText('2.2 U/kg/day');
    await expect(page.locator('#regimen-dextrose-pct')).toContainText('2.5%');

    // Step 4: Log serial q2h glucose & electrolyte check for Biscuit
    await page.locator('#check-blood-glucose').fill('175.0');
    await page.locator('#check-bhb').fill('1.1');
    await page.locator('#check-potassium').fill('4.1');
    await page.locator('#check-phosphorus').fill('3.6');
    await page.locator('#check-dextrose-percent').fill('2.5');
    await page.locator('#check-callsign').fill('DR-ENDO-55');
    await page.locator('#check-notes').fill('BG falling 35 mg/dL/hr (<= 50 mg/dL/hr limit); mentation bright.');
    await page.locator('#check-mentation-normal').check();
    await page.locator('#btn-record-serial-check').click();

    await expect(page.locator('#serial-checks-list')).toContainText(
      'BG falling 35 mg/dL/hr (<= 50 mg/dL/hr limit); mentation bright.'
    );

    // Step 5: Issue Ketone-Cleared SubQ Glargine Discharge Clearance for Biscuit
    await page.locator('#clearance-final-bhb').fill('0.3');
    await page.locator('#clearance-final-ph').fill('7.40');
    await page.locator('#clearance-final-potassium').fill('4.3');
    await page.locator('#clearance-final-phosphorus').fill('3.9');
    await page.locator('#clearance-vet-license').fill('VET-ENDO-77410');
    await page.locator('#clearance-summary').fill('Ketones cleared (BHB 0.3 mmol/L); eating voluntarily; SubQ insulin transition completed.');
    await page.locator('#clearance-voluntary-eating').check();
    await page.locator('#clearance-subq-transition').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(biscuitCard).toHaveAttribute('data-status', 'KETONES_CLEARED_SUBQ_GLARGINE_READY');
    await expect(page.locator('#dka-clearance-banner')).toBeVisible();
    await expect(page.locator('#dka-clearance-banner')).toContainText('VET-ENDO-77410');

    // Step 6: Filter by KETONES_CLEARED_SUBQ_GLARGINE_READY and verify Biscuit and Chester remain visible
    await page.locator('#filter-status').selectOption('KETONES_CLEARED_SUBQ_GLARGINE_READY');
    await expect(
      page.locator('#dka-operations-list .dka-op-card', { hasText: 'Biscuit (' })
    ).toBeVisible();
    await expect(
      page.locator('#dka-operations-list .dka-op-card', { hasText: 'Chester (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
