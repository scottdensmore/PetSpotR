import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Tumor Lysis Syndrome (TLS) & Oncologic Hypercalcemia of Malignancy, Hyperkalemia/Hyperphosphatemia/Hypocalcemia, Rasburicase/Allopurinol & Zoledronate/Saline Diuresis Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Oncologic TLS patient "Apollo", staging 0.9% NaCl diuresis/Rasburicase/Zoledronate protocol, recording electrolyte check, clearing for normoelectrolytic discharge, and filtering by ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/oncologic-tls-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#oncologic-tls-cockpit')).toBeVisible();
    await expect(page.locator('#tls-alert-banner')).toBeVisible();

    await expect(
      page.locator('#tls-operations-grid .tls-op-card', { hasText: 'Bauer (' })
    ).toBeVisible();
    await expect(
      page.locator('#tls-operations-grid .tls-op-card', { hasText: 'Penny (' })
    ).toBeVisible();
    await expect(
      page.locator('#tls-operations-grid .tls-op-card', { hasText: 'Otis (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Oncologic TLS patient "Apollo"
    await page.locator('#create-case-id-input').fill('CASE-TLS-104');
    await page.locator('#create-pet-name-input').fill('Apollo');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Golden Retriever');
    await page.locator('#create-weight-input').fill('30.5');
    await page
      .locator('#create-emergency-type-input')
      .selectOption('ACUTE_TUMOR_LYSIS_SYNDROME_LYMPHOMA');
    await page.locator('#create-potassium-input').fill('6.9');
    await page.locator('#create-phosphorus-input').fill('10.8');
    await page.locator('#create-ical-input').fill('0.82');
    await page.locator('#create-uric-acid-input').fill('8.8');
    await page.locator('#create-creatinine-input').fill('2.8');
    await page.locator('#create-caxp-input').fill('92.5');
    await page.locator('#create-cytology-checkbox').check();
    await page.locator('#create-tls-op-btn').click();

    const apolloCard = page
      .locator('#tls-operations-grid .tls-op-card', { hasText: 'Apollo (' })
      .first();
    await expect(apolloCard).toBeVisible();
    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE'
    );
    await expect(page.locator('#active-op-title')).toContainText('Apollo');
    await expect(page.locator('#active-potassium')).toContainText('6.9 mmol/L');
    await expect(page.locator('#active-ionized-calcium')).toContainText('0.82 mmol/L');

    // Step 4: Stage 0.9% NaCl diuresis, Rasburicase & Zoledronate protocol for Apollo
    await page.locator('#stage-diuresis-btn').click();
    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED'
    );

    // Step 5: Record serial electrolyte & renal panel check for Apollo
    await page.locator('#check-potassium-input').fill('4.2');
    await page.locator('#check-phosphorus-input').fill('4.4');
    await page.locator('#check-ical-input').fill('1.26');
    await page.locator('#check-uric-acid-input').fill('1.4');
    await page.locator('#check-creatinine-input').fill('1.2');
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Soren Lindqvist, DACVIM (Oncology)');
    await page
      .locator('#check-note-input')
      .fill('Post-0.9% NaCl diuresis & Rasburicase infusion: Apollo K+ normalized to 4.2 mmol/L, iCa restored to 1.26 mmol/L.');
    await page.locator('#record-electrolyte-check-btn').click();

    await expect(page.locator('#active-potassium')).toContainText('4.2 mmol/L');
    await expect(page.locator('#active-ionized-calcium')).toContainText('1.26 mmol/L');
    await expect(page.locator('#electrolyte-checks-list')).toContainText(
      'Post-0.9% NaCl diuresis & Rasburicase infusion: Apollo K+ normalized'
    );

    // Step 6: Clear Apollo for Normoelectrolytic Discharge
    await page
      .locator('#clear-discharged-by-input')
      .fill('Dr. Soren Lindqvist, DACVIM (Oncology)');
    await page
      .locator('#clear-summary-input')
      .fill('Apollo electrolytes normalized (K+ 4.2 mmol/L, iCa 1.26 mmol/L, Phosphorus 4.4 mg/dL), renal function protected, cleared for oncology ward.');
    await page.locator('#clear-tls-btn').click();

    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED'
    );
    await expect(page.locator('#tls-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Apollo');

    // Step 7: Filter by ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED
    await page
      .locator('#tls-status-filter')
      .selectOption('ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED');
    await expect(
      page.locator('#tls-operations-grid .tls-op-card', { hasText: 'Apollo (' })
    ).toBeVisible();
    await expect(
      page.locator('#tls-operations-grid .tls-op-card', { hasText: 'Otis (' })
    ).toBeVisible();

    await page.locator('#tls-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
