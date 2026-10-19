import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Immune-Mediated Hemolytic Anemia (IMHA) & Thrombocytopenia (IMTP), Saline Agglutination, Spherocytosis & Thromboprophylaxis Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Poppy", staging Prednisolone/Clopidogrel/pRBC regimen, logging serial q6h hemolysis & PTE respiratory check, issuing 48h stable PCV & reticulocytosis taper clearance, and filtering by PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/imha-thrombosis-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#imha-thrombosis-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Immune-Mediated Hemolytic Anemia (IMHA) & Thrombocytopenia (IMTP), Saline Agglutination, Spherocytosis & Thromboprophylaxis Cockpit'
    );

    await expect(
      page.locator('#imha-operations-list .imha-op-card', { hasText: 'Penny (' })
    ).toBeVisible();
    await expect(
      page.locator('#imha-operations-list .imha-op-card', { hasText: 'Scout (' })
    ).toBeVisible();
    await expect(
      page.locator('#imha-operations-list .imha-op-card', { hasText: 'Maple (' })
    ).toBeVisible();

    // Step 2: Create new operation "Poppy"
    await page.locator('#create-incident-id').fill('INC-IM-9704');
    await page.locator('#create-pet-name').fill('Poppy');
    await page.locator('#create-species').selectOption('CANINE_PRIMARY_IMHA');
    await page.locator('#create-breed').fill('Cocker Spaniel');
    await page.locator('#create-weight-kg').fill('12.5');
    await page.locator('#create-pcv').fill('13.5');
    await page.locator('#create-bilirubin').fill('5.2');
    await page.locator('#create-platelets').fill('92.0');
    await page.locator('#create-spherocytes').fill('16');
    await page.locator('#create-saline-agglutination').check();
    await page.locator('#create-hemoglobinuria').check();
    await page.locator('#btn-create-imha-op').click();

    const poppyCard = page
      .locator('#imha-operations-list .imha-op-card', { hasText: 'Poppy (' })
      .first();
    await expect(poppyCard).toBeVisible();
    await expect(poppyCard).toHaveAttribute('data-status', 'HEMOLYTIC_AGGLUTINATION_CRISIS_CRITICAL');

    // Step 3: Stage Prednisolone / Clopidogrel / pRBC regimen for Poppy
    await page.locator('#stage-weight-kg').fill('12.5');
    await page.locator('#stage-species').selectOption('CANINE_PRIMARY_IMHA');
    await page.locator('#stage-pcv').fill('16.0');
    await page.locator('#stage-bilirubin').fill('3.8');
    await page.locator('#stage-platelets').fill('105');
    await page.locator('#stage-spherocytes').fill('12');
    await page.locator('#btn-stage-regimen').click();

    await expect(poppyCard).toHaveAttribute('data-status', 'PREDNISOLONE_CLOPIDOGREL_PRBC_STAGED');
    await expect(page.locator('#regimen-prednisolone-mg')).toContainText('25.0 mg/day');
    await expect(page.locator('#regimen-clopidogrel-mg')).toContainText('25.0 mg/day');

    // Step 4: Log serial q6h hemolysis & PTE respiratory check for Poppy
    await page.locator('#check-pcv').fill('25.5');
    await page.locator('#check-reticulocytes').fill('124.0');
    await page.locator('#check-bilirubin').fill('1.1');
    await page.locator('#check-respiratory-rate').fill('22');
    await page.locator('#check-callsign').fill('DR-HEME-55');
    await page.locator('#check-notes').fill('PCV stable at 25.5%, regenerative reticulocytosis 124 K/uL, 1:4 saline wash negative.');
    await page.locator('#check-saline-persistent').uncheck();
    await page.locator('#btn-record-hemolysis-check').click();

    await expect(page.locator('#hemolysis-checks-list')).toContainText(
      'PCV stable at 25.5%, regenerative reticulocytosis 124 K/uL, 1:4 saline wash negative.'
    );

    // Step 5: Issue 48h Stable PCV & Reticulocytosis Taper Clearance for Poppy
    await page.locator('#clearance-final-pcv').fill('29.0');
    await page.locator('#clearance-final-reticulocytes').fill('142.0');
    await page.locator('#clearance-vet-license').fill('VET-IM-88410');
    await page.locator('#clearance-summary').fill('PCV stable at 29.0% for 48h; reticulocytes 142 K/uL; 1:4 saline agglutination resolved; outpatient thromboprophylaxis dispensed.');
    await page.locator('#clearance-agglutination-resolved').check();
    await page.locator('#clearance-no-pte-dyspnea').check();
    await page.locator('#clearance-thromboprophylaxis-dispensed').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(poppyCard).toHaveAttribute('data-status', 'PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE');
    await expect(page.locator('#imha-clearance-banner')).toBeVisible();
    await expect(page.locator('#imha-clearance-banner')).toContainText('VET-IM-88410');

    // Step 6: Filter by PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE and verify Poppy and Maple remain visible
    await page.locator('#filter-status').selectOption('PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE');
    await expect(
      page.locator('#imha-operations-list .imha-op-card', { hasText: 'Poppy (' })
    ).toBeVisible();
    await expect(
      page.locator('#imha-operations-list .imha-op-card', { hasText: 'Maple (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
