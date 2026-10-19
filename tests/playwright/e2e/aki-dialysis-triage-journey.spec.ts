import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Kidney Injury (AKI), Lily/Ethylene Glycol/Grape Nephrotoxicity, Oliguria/Anuria & Furosemide/Diltiazem/Dialysis Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Pinot", staging Fomepizole & Ins-and-Outs diuresis plan, logging closed-system UOP & renal telemetry check, issuing polyuric discharge clearance, and filtering by POLYURIC_CREATININE_PLATEAU_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/aki-dialysis-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#aki-dialysis-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Acute Kidney Injury (AKI), Lily/Ethylene Glycol/Grape Nephrotoxicity, Oliguria/Anuria & Furosemide/Diltiazem/Dialysis Cockpit'
    );

    await expect(
      page.locator('#aki-operations-list .aki-op-card', { hasText: 'Lotus (' })
    ).toBeVisible();
    await expect(
      page.locator('#aki-operations-list .aki-op-card', { hasText: 'Diesel (' })
    ).toBeVisible();
    await expect(
      page.locator('#aki-operations-list .aki-op-card', { hasText: 'Merlot (' })
    ).toBeVisible();

    // Step 2: Create new operation "Pinot"
    await page.locator('#create-incident-id').fill('INC-AKI-9934');
    await page.locator('#create-pet-name').fill('Pinot');
    await page.locator('#create-species').selectOption('CANINE_ETHYLENE_GLYCOL_OXALATE');
    await page.locator('#create-breed').fill('Golden Retriever');
    await page.locator('#create-weight-kg').fill('24.0');
    await page.locator('#create-iris-grade').fill('4');
    await page.locator('#create-creatinine').fill('6.8');
    await page.locator('#create-potassium').fill('6.6');
    await page.locator('#create-uop').fill('0.4');
    await page.locator('#create-weight-gain-pct').fill('6.0');
    await page.locator('#create-oxalate-crystals').check();
    await page.locator('#btn-create-aki-op').click();

    const pinotCard = page
      .locator('#aki-operations-list .aki-op-card', { hasText: 'Pinot (' })
      .first();
    await expect(pinotCard).toBeVisible();
    await expect(pinotCard).toHaveAttribute('data-status', 'OLIGURIC_ANURIC_AZOTEMIA_CRITICAL');

    // Step 3: Stage Fomepizole, Ins-and-Outs & Dialysis plan for Pinot
    await page.locator('#stage-weight-kg').fill('24.0');
    await page.locator('#stage-species').selectOption('CANINE_ETHYLENE_GLYCOL_OXALATE');
    await page.locator('#stage-iris-grade').fill('4');
    await page.locator('#stage-uop').fill('0.4');
    await page.locator('#stage-creatinine').fill('6.8');
    await page.locator('#stage-potassium').fill('6.6');
    await page.locator('#btn-stage-plan').click();

    await expect(pinotCard).toHaveAttribute('data-status', 'FOMEPIZOLE_INS_OUTS_DIURESIS_STAGED');
    await expect(page.locator('#plan-fomepizole')).toContainText('480.0 mg IV');
    await expect(page.locator('#plan-ins-outs-rate')).toContainText('29.60 mL/hr');
    await expect(page.locator('#plan-furosemide-bolus')).toContainText('48.0 mg IV');
    await expect(page.locator('#plan-diltiazem-cri')).toContainText('0.3 mcg/kg/min');

    // Step 4: Log serial closed-system UOP & renal telemetry check for Pinot
    await page.locator('#check-uop').fill('2.6');
    await page.locator('#check-creatinine').fill('1.7');
    await page.locator('#check-potassium').fill('4.3');
    await page.locator('#check-body-weight').fill('24.0');
    await page.locator('#check-fluid-balance').fill('0.0');
    await page.locator('#check-callsign').fill('DR-RENAL-55');
    await page
      .locator('#check-notes')
      .fill('Polyuric conversion achieved (UOP 2.6 mL/kg/hr); Cr normalized to 1.7 mg/dL; K+ 4.3 mEq/L.');
    await page.locator('#btn-record-renal-check').click();

    await expect(page.locator('#renal-checks-list')).toContainText(
      'Polyuric conversion achieved (UOP 2.6 mL/kg/hr); Cr normalized to 1.7 mg/dL; K+ 4.3 mEq/L.'
    );

    // Step 5: Issue Polyuric Recovery & Non-Azotemic Creatinine Plateau Discharge Clearance for Pinot
    await page.locator('#clearance-final-uop').fill('2.6');
    await page.locator('#clearance-final-creatinine').fill('1.6');
    await page.locator('#clearance-final-potassium').fill('4.3');
    await page.locator('#clearance-vet-license').fill('VET-RENAL-88320');
    await page
      .locator('#clearance-summary')
      .fill('Polyuric recovery & non-azotemic creatinine plateau achieved; outpatient SubQ fluid taper prescribed.');
    await page.locator('#clearance-euvolemic').check();
    await page.locator('#clearance-subq-educated').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(pinotCard).toHaveAttribute('data-status', 'POLYURIC_CREATININE_PLATEAU_CLEARED');
    await expect(page.locator('#aki-clearance-banner')).toBeVisible();
    await expect(page.locator('#aki-clearance-banner')).toContainText('VET-RENAL-88320');

    // Step 6: Filter by POLYURIC_CREATININE_PLATEAU_CLEARED and verify Pinot and Merlot remain visible
    await page.locator('#filter-status').selectOption('POLYURIC_CREATININE_PLATEAU_CLEARED');
    await expect(
      page.locator('#aki-operations-list .aki-op-card', { hasText: 'Pinot (' })
    ).toBeVisible();
    await expect(
      page.locator('#aki-operations-list .aki-op-card', { hasText: 'Merlot (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
