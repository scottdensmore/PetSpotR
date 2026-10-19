import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Systemic Mycoses & Fungal Emergencies (Blastomycosis, Histoplasmosis, Coccidioidomycosis, Cryptococcosis & Sinonasal Aspergillosis), Urine/Serum MiraVista Antigen EIA, Itraconazole/Fluconazole, Amphotericin B Lipid Complex Nephrotoxicity Guard & Clotrimazole Trephination Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Systemic Mycoses patient "Voyageur", staging FDA-approved Itraconazole & Amphotericin B protocol, recording MiraVista Antigen EIA & renal check, clearing after two consecutive negative EIA tests, and filtering by TWO_NEGATIVE_ANTIGEN_EIA_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/systemic-mycoses-fungal-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#systemic-mycoses-cockpit')).toBeVisible();
    await expect(page.locator('#myc-alert-banner')).toBeVisible();

    await expect(
      page.locator('#myc-operations-grid .myc-op-card', { hasText: 'Northwoods (' })
    ).toBeVisible();
    await expect(
      page.locator('#myc-operations-grid .myc-op-card', { hasText: 'Sonora (' })
    ).toBeVisible();
    await expect(
      page.locator('#myc-operations-grid .myc-op-card', { hasText: 'Sedona (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Systemic Mycoses patient "Voyageur"
    await page.locator('#create-case-id-input').fill('CASE-MYC-104');
    await page.locator('#create-pet-name-input').fill('Voyageur');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('German Shorthaired Pointer');
    await page.locator('#create-weight-input').fill('27.5');
    await page
      .locator('#create-syndrome-input')
      .selectOption('BLASTOMYCOSIS_BROAD_BASED_BUDDING_YEAST');
    await page
      .locator('#create-protocol-input')
      .selectOption('ITRACONAZOLE_5MG_KG_BRAND_OR_SOLUTION');
    await page.locator('#create-antigen-input').fill('12.6');
    await page.locator('#create-creatinine-input').fill('1.25');
    await page.locator('#create-alt-input').fill('74');
    await page.locator('#create-spo2-input').fill('90.5');
    await page.locator('#create-cribriform-intact-checkbox').check();
    await page.locator('#create-compounded-blocked-checkbox').check();
    await page.locator('#create-saline-diuresis-checkbox').check();
    await page.locator('#create-myc-op-btn').click();

    const voyageurCard = page
      .locator('#myc-operations-grid .myc-op-card', { hasText: 'Voyageur (' })
      .first();
    await expect(voyageurCard).toBeVisible();
    await expect(voyageurCard).toHaveAttribute(
      'data-status',
      'AZOLE_AMPHOTERICIN_TREPHINATION_STAGED'
    );
    await expect(page.locator('#active-op-title')).toContainText('Voyageur');
    await expect(page.locator('#active-antigen')).toContainText('12.60 ng/mL');
    await expect(page.locator('#active-creatinine')).toContainText('1.25 mg/dL');

    // Step 4: Stage FDA-approved Itraconazole & Amphotericin B therapy for Voyageur
    await page.locator('#stage-therapy-btn').click();
    await expect(voyageurCard).toHaveAttribute(
      'data-status',
      'ANTIGEN_EIA_CREA_OPHTHALMIC_MONITORED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'ANTIGEN_EIA_CREA_OPHTHALMIC_MONITORED'
    );

    // Step 5: Record MiraVista Antigen EIA & renal check for Voyageur
    await page.locator('#check-antigen-input').fill('0.0');
    await page.locator('#check-creatinine-input').fill('1.15');
    await page.locator('#check-alt-input').fill('58');
    await page.locator('#check-spo2-input').fill('98.0');
    await page.locator('#check-cribriform-intact-checkbox').check();
    await page.locator('#check-ampho-held-checkbox').uncheck();
    await page.locator('#check-chorioretinitis-resolved-checkbox').check();
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)');
    await page
      .locator('#check-note-input')
      .fill('Month 6 & Month 7 MiraVista check for Voyageur: two consecutive negative urine antigen EIA tests (0.00 ng/mL), Creatinine 1.15 mg/dL, ALT 58 U/L, SpO2 98.0%.');
    await page.locator('#record-antigen-check-btn').click();

    await expect(page.locator('#active-antigen')).toContainText('0.00 ng/mL');
    await expect(page.locator('#active-creatinine')).toContainText('1.15 mg/dL');
    await expect(page.locator('#active-alt')).toContainText('58 U/L');
    await expect(page.locator('#antigen-checks-list')).toContainText(
      'Month 6 & Month 7 MiraVista check for Voyageur: two consecutive negative urine antigen EIA tests'
    );

    // Step 6: Clear Voyageur for Two-Negative MiraVista Antigen EIA & 6-Month Azole Cure
    await page.locator('#clear-antigen-input').fill('0.0');
    await page.locator('#clear-creatinine-input').fill('1.15');
    await page.locator('#clear-two-negative-eia-checkbox').check();
    await page.locator('#clear-six-month-azole-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)');
    await page
      .locator('#clear-summary-input')
      .fill('Voyageur cleared after two consecutive negative MiraVista Blastomyces antigen EIA tests (0.00 ng/mL) 1 month apart, Creatinine 1.15 mg/dL, and 6-month FDA-approved Itraconazole course.');
    await page.locator('#clear-myc-btn').click();

    await expect(voyageurCard).toHaveAttribute(
      'data-status',
      'TWO_NEGATIVE_ANTIGEN_EIA_CLEARED'
    );
    await expect(page.locator('#myc-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Voyageur');

    // Step 7: Filter by TWO_NEGATIVE_ANTIGEN_EIA_CLEARED
    await page
      .locator('#myc-status-filter')
      .selectOption('TWO_NEGATIVE_ANTIGEN_EIA_CLEARED');
    await expect(
      page.locator('#myc-operations-grid .myc-op-card', { hasText: 'Voyageur (' })
    ).toBeVisible();
    await expect(
      page.locator('#myc-operations-grid .myc-op-card', { hasText: 'Sedona (' })
    ).toBeVisible();

    await page.locator('#myc-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
