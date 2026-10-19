import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Vector-Borne & Rickettsial Emergencies (Lyme Nephritis, Anaplasmosis, Ehrlichiosis, Rocky Mountain Spotted Fever, Babesiosis & Cytauxzoonosis), SNAP 4Dx Plus / PCR Triage, Doxycycline, Atovaquone + Azithromycin & Imidocarb Dipropionate Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Vector-Borne patient "Scout", staging antimicrobial protocol, recording hematology/renal check, clearing for PCR-negative discharge, and filtering by PCR_NEGATIVE_PLATELETS_RENAL_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/vector-borne-rickettsial-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#vbr-triage-cockpit')).toBeVisible();
    await expect(page.locator('#vbr-alert-banner')).toBeVisible();

    await expect(
      page.locator('#vbr-operations-grid .vbr-op-card', { hasText: 'Yukon (' })
    ).toBeVisible();
    await expect(
      page.locator('#vbr-operations-grid .vbr-op-card', { hasText: 'Willow (' })
    ).toBeVisible();
    await expect(
      page.locator('#vbr-operations-grid .vbr-op-card', { hasText: 'Boone (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Vector-Borne patient "Scout" via UI form
    await page.locator('#create-case-id-input').fill('CASE-VBR-104');
    await page.locator('#create-pet-name-input').fill('Scout');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Australian Shepherd');
    await page.locator('#create-weight-input').fill('22.5');
    await page
      .locator('#create-syndrome-input')
      .selectOption('EHRLICHIOSIS_ANAPLASMOSIS_MORULAE');
    await page
      .locator('#create-assay-input')
      .selectOption('SNAP_4DX_PLUS_C6_PEPTIDE');
    await page
      .locator('#create-protocol-input')
      .selectOption('DOXYCYCLINE_10MG_KG_WATER_CHASE');
    await page.locator('#create-platelets-input').fill('38.0');
    await page.locator('#create-pcv-input').fill('28.0');
    await page.locator('#create-upc-input').fill('0.4');
    await page.locator('#create-albumin-input').fill('2.6');
    await page.locator('#create-doxy-input').fill('10.0');
    await page.locator('#create-water-chase-checkbox').check();
    await page.locator('#create-atropine-checkbox').check();
    await page.locator('#create-vbr-op-btn').click();

    const scoutCard = page
      .locator('#vbr-operations-grid .vbr-op-card', { hasText: 'Scout (' })
      .first();
    await expect(scoutCard).toBeVisible();
    await expect(scoutCard).toHaveAttribute(
      'data-status',
      'DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED'
    );
    await expect(page.locator('#active-op-title')).toContainText('Scout');
    await expect(page.locator('#active-platelets')).toContainText('38.0 K/µL');

    // Step 4: Stage antimicrobial protocol for Scout
    await page
      .locator('#stage-protocol-input')
      .selectOption('DOXYCYCLINE_10MG_KG_WATER_CHASE');
    await page.locator('#stage-doxy-input').fill('10.0');
    await page.locator('#stage-water-chase-checkbox').check();
    await page.locator('#stage-antimicrobial-btn').click();

    await expect(scoutCard).toHaveAttribute(
      'data-status',
      'DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED'
    );

    // Step 5: Record hematology & renal check for Scout
    await page.locator('#check-platelets-input').fill('188.0');
    await page.locator('#check-pcv-input').fill('39.5');
    await page.locator('#check-upc-input').fill('0.22');
    await page.locator('#check-albumin-input').fill('3.3');
    await page.locator('#check-morulae-cleared-checkbox').check();
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Mara Vance, DACVIM (Infectious Disease)');
    await page
      .locator('#check-note-input')
      .fill('Day 14 Check for Scout: Morulae cleared on buffy coat smear, platelets 188.0 K/µL, PCV 39.5%, UPC 0.22.');
    await page.locator('#record-hematology-check-btn').click();

    await expect(scoutCard).toHaveAttribute(
      'data-status',
      'THROMBOCYTOPENIA_PROTEINURIA_MONITORED'
    );
    await expect(page.locator('#active-platelets')).toContainText('188.0 K/µL');
    await expect(page.locator('#active-upc')).toContainText('0.22');
    await expect(page.locator('#hematology-checks-list')).toContainText(
      'Day 14 Check for Scout: Morulae cleared on buffy coat smear'
    );

    // Step 6: Clear Scout for PCR-Negative & Platelet/Renal Recovery Discharge
    await page.locator('#clear-platelets-input').fill('195.0');
    await page.locator('#clear-pcv-input').fill('40.0');
    await page.locator('#clear-upc-input').fill('0.20');
    await page.locator('#clear-pcr-negative-checkbox').check();
    await page.locator('#clear-normothermic-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Mara Vance, DACVIM (Infectious Disease)');
    await page
      .locator('#clear-summary-input')
      .fill('Scout completed 28-day Doxycycline course; quantitative PCR negative, platelets 195.0 K/µL, PCV 40.0%, UPC 0.20, cleared for discharge.');
    await page.locator('#clear-vbr-btn').click();

    await expect(scoutCard).toHaveAttribute(
      'data-status',
      'PCR_NEGATIVE_PLATELETS_RENAL_CLEARED'
    );
    await expect(page.locator('#vbr-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Scout');

    // Step 7: Filter by PCR_NEGATIVE_PLATELETS_RENAL_CLEARED
    await page
      .locator('#vbr-status-filter')
      .selectOption('PCR_NEGATIVE_PLATELETS_RENAL_CLEARED');
    await expect(
      page.locator('#vbr-operations-grid .vbr-op-card', { hasText: 'Scout (' })
    ).toBeVisible();
    await expect(
      page.locator('#vbr-operations-grid .vbr-op-card', { hasText: 'Boone (' })
    ).toBeVisible();

    await page.locator('#vbr-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
