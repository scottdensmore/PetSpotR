import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Pancreatitis, Canine/Feline Spec cPL/fPL, POCUS Hypoechoic Pancreas/Hyperechoic Mesentery, Maropitant/Fuzapladib & Early Enteral Nutrition Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, manual SSE stream connection, admitting a new operation, staging Fuzapladib & early enteral nutrition, recording Spec cPL/pain/RER check, clearing for pain-free discharge, and filtering by TOLERATING_ENTERAL_PAIN_FREE_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/pancreatitis-cpl-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#pancreatitis-cpl-cockpit')).toBeVisible();
    await expect(page.locator('#pancreatitis-alert-banner')).toBeVisible();

    const schnitzelCard = page.locator(
      '#pancreatitis-operations-grid .pancreatitis-op-card[data-op-id="PANC-OP-101"]'
    );
    const misoCard = page.locator(
      '#pancreatitis-operations-grid .pancreatitis-op-card[data-op-id="PANC-OP-102"]'
    );
    const biscuitCard = page.locator(
      '#pancreatitis-operations-grid .pancreatitis-op-card[data-op-id="PANC-OP-103"]'
    );

    await expect(schnitzelCard).toBeVisible();
    await expect(misoCard).toBeVisible();
    await expect(biscuitCard).toBeVisible();

    // Step 2: Connect SSE Stream manually
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit a new operation so the seeded PANC-OP-101 stays untouched
    // for specs running in parallel against the same server.
    const petName = `Pretzel ${Date.now()}`;
    await page.locator('#create-pet-name').fill(petName);
    await page.locator('#create-pancreatitis-btn').click();
    await expect(page.locator('#active-op-title')).toContainText(petName);
    const newCard = page
      .locator('#pancreatitis-operations-grid .pancreatitis-op-card')
      .filter({ hasText: petName });
    await expect(newCard).toBeVisible();

    // Stage Fuzapladib, Maropitant & Early Enteral Nutrition
    await page.locator('#stage-nutrition-btn').click();
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'FUZAPLADIB_MAROPITANT_ENTERAL_STAGED'
    );

    // Step 4: Record serial Spec cPL, CMPS-SF pain score & Enteral RER check
    await page.locator('#check-spec-pl-input').fill('195');
    await page.locator('#check-pain-score-input').fill('2');
    await page.locator('#check-rer-percent-input').fill('100');
    await page.locator('#check-vomiting-input').fill('VOLUNTARY_LOW_FAT_100PCT_RER');
    await page.locator('#check-recorded-by-input').fill('Dr. Elena Rostova, DACVECC');
    await page
      .locator('#check-note-input')
      .fill('Spec cPL normalized to 195 ug/L; pain-free (CMPS-SF 2/24); consuming 100% RER.');
    await page.locator('#record-enteral-check-btn').click();

    await expect(page.locator('#active-spec-pl')).toContainText('195');
    await expect(page.locator('#active-enteral-rer')).toContainText('100% RER');
    await expect(page.locator('#enteral-checks-list')).toContainText(
      'Spec cPL normalized to 195 ug/L; pain-free (CMPS-SF 2/24); consuming 100% RER.'
    );

    // Step 5: Clear the new operation for discharge
    await page.locator('#clear-discharged-by-input').fill('Dr. Elena Rostova, DACVECC');
    await page
      .locator('#clear-summary-input')
      .fill(
        'Completed 3-day Fuzapladib sodium & Maropitant/Ondansetron course; tolerating 100% RER ultra-low-fat diet pain-free.'
      );
    await page.locator('#clear-pancreatitis-btn').click();

    await expect(page.locator('#active-op-status-badge')).toContainText(
      'TOLERATING_ENTERAL_PAIN_FREE_CLEARED'
    );
    await expect(page.locator('#pancreatitis-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText(
      'Dr. Elena Rostova, DACVECC'
    );

    // Step 6: Filter by TOLERATING_ENTERAL_PAIN_FREE_CLEARED
    await page
      .locator('#pancreatitis-status-filter')
      .selectOption('TOLERATING_ENTERAL_PAIN_FREE_CLEARED');
    await expect(newCard).toBeVisible();
    await expect(biscuitCard).toBeVisible();

    await page.locator('#pancreatitis-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
