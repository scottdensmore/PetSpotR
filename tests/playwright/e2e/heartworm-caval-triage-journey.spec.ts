import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Heartworm Caval Syndrome (Dirofilaria immitis), Vena Cava Worm Burden Echocardiography, Intravascular Hemolysis (Hemoglobinuria), Jugular Venotomy Extraction & Melarsomine Split-Dose / Doxycycline / Macrocyclic Lactone Thromboembolism Prevention Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Heartworm Caval patient "Thibodaux", staging right jugular venotomy & Wolbachia protocol, recording echocardiography/hemolysis check, clearing for patent discharge, and filtering by CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/heartworm-caval-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#heartworm-caval-cockpit')).toBeVisible();
    await expect(page.locator('#hwc-alert-banner')).toBeVisible();

    await expect(
      page.locator('#hwc-operations-grid .hwc-op-card', { hasText: 'Boudreaux (' })
    ).toBeVisible();
    await expect(
      page.locator('#hwc-operations-grid .hwc-op-card', { hasText: 'Magnolia (' })
    ).toBeVisible();
    await expect(
      page.locator('#hwc-operations-grid .hwc-op-card', { hasText: 'Cypress (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Heartworm Caval patient "Thibodaux"
    await page.locator('#create-case-id-input').fill('CASE-HWC-104');
    await page.locator('#create-pet-name-input').fill('Thibodaux');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Catahoula Leopard Dog');
    await page.locator('#create-weight-input').fill('26.0');
    await page
      .locator('#create-severity-class-input')
      .selectOption('CLASS_IV_CAVAL_SYNDROME');
    await page
      .locator('#create-protocol-input')
      .selectOption('RIGHT_JUGULAR_ISHIHARA_FORCEPS_EXTRACTION');
    await page.locator('#create-tr-jet-input').fill('4.05');
    await page.locator('#create-worms-extracted-input').fill('0');
    await page.locator('#create-pcv-input').fill('19.5');
    await page.locator('#create-hemoglobinuria-checkbox').check();
    await page.locator('#create-caval-lines-checkbox').check();
    await page.locator('#create-hwc-op-btn').click();

    const thibodauxCard = page
      .locator('#hwc-operations-grid .hwc-op-card', { hasText: 'Thibodaux (' })
      .first();
    await expect(thibodauxCard).toBeVisible();
    await expect(thibodauxCard).toHaveAttribute(
      'data-status',
      'JUGULAR_VENOTOMY_WOLBACHIA_DOXY_STAGED'
    );
    await expect(page.locator('#active-op-title')).toContainText('Thibodaux');
    await expect(page.locator('#active-tr-jet')).toContainText('4.05 m/s');
    await expect(page.locator('#active-pcv')).toContainText('19.5%');

    // Step 4: Stage right jugular venotomy extraction & Wolbachia Doxycycline intervention for Thibodaux
    await page.locator('#stage-intervention-btn').click();
    await expect(thibodauxCard).toHaveAttribute(
      'data-status',
      'PULMONARY_HYPERTENSION_PTE_MONITORED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'PULMONARY_HYPERTENSION_PTE_MONITORED'
    );

    // Step 5: Record post-venotomy echocardiography & hemolysis check for Thibodaux
    await page.locator('#check-tr-jet-input').fill('2.35');
    await page.locator('#check-worms-extracted-input').fill('34');
    await page.locator('#check-pcv-input').fill('37.0');
    await page.locator('#check-sildenafil-dose-input').fill('1.5');
    await page.locator('#check-hemoglobinuria-checkbox').uncheck();
    await page.locator('#check-caval-lines-checkbox').uncheck();
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Thibodaux Broussard, DACVIM (Cardiology)');
    await page
      .locator('#check-note-input')
      .fill('Post-venotomy check for Thibodaux: 34 adult Dirofilaria immitis extracted; parallel hyperechoic caval lines abolished, hemoglobinuria resolved, TR jet 2.35 m/s.');
    await page.locator('#record-hemodynamic-check-btn').click();

    await expect(page.locator('#active-tr-jet')).toContainText('2.35 m/s');
    await expect(page.locator('#active-worms-extracted')).toContainText('34 worms');
    await expect(page.locator('#active-pcv')).toContainText('37.0%');
    await expect(page.locator('#hemodynamic-checks-list')).toContainText(
      'Post-venotomy check for Thibodaux: 34 adult Dirofilaria immitis extracted'
    );

    // Step 6: Clear Thibodaux for Caval Patent & Antigen-Negative Discharge
    await page.locator('#clear-tr-jet-input').fill('2.35');
    await page.locator('#clear-cage-rest-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Thibodaux Broussard, DACVIM (Cardiology)');
    await page
      .locator('#clear-summary-input')
      .fill('Thibodaux caval lumen 100% patent, 34 adult worms extracted, hemoglobinuria resolved, TR jet 2.35 m/s (<= 2.8 m/s), 8-week strict cage rest verified; cleared.');
    await page.locator('#clear-hwc-btn').click();

    await expect(thibodauxCard).toHaveAttribute(
      'data-status',
      'CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED'
    );
    await expect(page.locator('#hwc-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Thibodaux');

    // Step 7: Filter by CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED
    await page
      .locator('#hwc-status-filter')
      .selectOption('CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED');
    await expect(
      page.locator('#hwc-operations-grid .hwc-op-card', { hasText: 'Thibodaux (' })
    ).toBeVisible();
    await expect(
      page.locator('#hwc-operations-grid .hwc-op-card', { hasText: 'Cypress (' })
    ).toBeVisible();

    await page.locator('#hwc-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
