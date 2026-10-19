import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Feline Infectious Peritonitis (FIP — Effusive Wet & Non-Effusive Dry/Ocular/Neurological), Rivalta Test, Albumin:Globulin Ratio < 0.4, GS-441524 / Remdesivir Nucleoside Analog & Blood-Brain/Ocular Barrier Dosing Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new FIP patient "Mochi", staging 84-day GS-441524/Remdesivir protocol, recording progress check, clearing for 84-day cure discharge, and filtering by AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/fip-gs441524-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#fip-gs441524-cockpit')).toBeVisible();
    await expect(page.locator('#fip-alert-banner')).toBeVisible();

    await expect(
      page.locator('#fip-operations-grid .fip-op-card', { hasText: 'Bramble (' })
    ).toBeVisible();
    await expect(
      page.locator('#fip-operations-grid .fip-op-card', { hasText: 'Cosmo (' })
    ).toBeVisible();
    await expect(
      page.locator('#fip-operations-grid .fip-op-card', { hasText: 'Saffron (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new FIP patient "Mochi"
    await page.locator('#create-case-id-input').fill('CASE-FIP-104');
    await page.locator('#create-pet-name-input').fill('Mochi');
    await page.locator('#create-species-input').selectOption('Feline');
    await page.locator('#create-breed-input').fill('Scottish Fold');
    await page.locator('#create-weight-input').fill('2.8');
    await page
      .locator('#create-phenotype-input')
      .selectOption('EFFUSIVE_CAVITARY_PLEURAL_ASCITES_FIP');
    await page.locator('#create-treatment-day-input').fill('1');
    await page.locator('#create-ag-ratio-input').fill('0.32');
    await page.locator('#create-bilirubin-input').fill('2.1');
    await page.locator('#create-hct-input').fill('23.5');
    await page.locator('#create-gs-dose-input').fill('10.0');
    await page.locator('#create-rivalta-checkbox').check();
    await page.locator('#create-pleural-dyspnea-checkbox').check();
    await page.locator('#create-fip-op-btn').click();

    const mochiCard = page
      .locator('#fip-operations-grid .fip-op-card', { hasText: 'Mochi (' })
      .first();
    await expect(mochiCard).toBeVisible();
    await expect(mochiCard).toHaveAttribute(
      'data-status',
      'ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE'
    );
    await expect(page.locator('#active-op-title')).toContainText('Mochi');
    await expect(page.locator('#active-ag-ratio')).toContainText('0.32');
    await expect(page.locator('#active-treatment-day')).toContainText('Day 1 / 84');

    // Step 4: Stage 84-day GS-441524 / Remdesivir antiviral protocol for Mochi
    await page.locator('#stage-antiviral-btn').click();
    await expect(mochiCard).toHaveAttribute(
      'data-status',
      'GS441524_REMDESIVIR_84DAY_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'GS441524_REMDESIVIR_84DAY_STAGED'
    );

    // Step 5: Record 84-day antiviral progress & biomarker check for Mochi
    await page.locator('#check-treatment-day-input').fill('84');
    await page.locator('#check-weight-input').fill('3.9');
    await page.locator('#check-ag-ratio-input').fill('0.76');
    await page.locator('#check-bilirubin-input').fill('0.2');
    await page.locator('#check-hct-input').fill('37.5');
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Linnea Lindholm, DACVIM (Feline Medicine)');
    await page
      .locator('#check-note-input')
      .fill('Day 84 (Week 12) Check for Mochi: Effusion completely resolved, weight increased to 3.9 kg, A:G Ratio normalized to 0.76 (> 0.60).');
    await page.locator('#record-progress-check-btn').click();

    await expect(page.locator('#active-ag-ratio')).toContainText('0.76');
    await expect(page.locator('#active-treatment-day')).toContainText('Day 84 / 84');
    await expect(page.locator('#progress-checks-list')).toContainText(
      'Day 84 (Week 12) Check for Mochi: Effusion completely resolved'
    );

    // Step 6: Clear Mochi for 84-Day Antiviral Cure Discharge
    await page
      .locator('#clear-discharged-by-input')
      .fill('Dr. Linnea Lindholm, DACVIM (Feline Medicine)');
    await page
      .locator('#clear-summary-input')
      .fill('Mochi completed full 84-day GS-441524 protocol; A:G Ratio normalized to 0.76 (> 0.60), effusion resolved, HCT 37.5%, cleared as 84-day FIP clinical cure.');
    await page.locator('#clear-fip-btn').click();

    await expect(mochiCard).toHaveAttribute(
      'data-status',
      'AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED'
    );
    await expect(page.locator('#fip-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Mochi');

    // Step 7: Filter by AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED
    await page
      .locator('#fip-status-filter')
      .selectOption('AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED');
    await expect(
      page.locator('#fip-operations-grid .fip-op-card', { hasText: 'Mochi (' })
    ).toBeVisible();
    await expect(
      page.locator('#fip-operations-grid .fip-op-card', { hasText: 'Saffron (' })
    ).toBeVisible();

    await page.locator('#fip-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
