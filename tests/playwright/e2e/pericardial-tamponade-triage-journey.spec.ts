import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Pericardial Effusion, Cardiac Tamponade, Electrical Alternans & Right-Sided 4th–5th ICS Pericardiocentesis Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Goliath", staging Right 4th–5th ICS pericardiocentesis plan, logging tap & clot-tube check, issuing decompression discharge clearance, and filtering by DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/pericardial-tamponade-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#pericardial-tamponade-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Pericardial Effusion, Cardiac Tamponade, Electrical Alternans & Right-Sided 4th–5th ICS Pericardiocentesis Cockpit'
    );

    await expect(
      page.locator('#tamponade-operations-list .tamponade-op-card', { hasText: 'Brutus (' })
    ).toBeVisible();
    await expect(
      page.locator('#tamponade-operations-list .tamponade-op-card', { hasText: 'Mugsy (' })
    ).toBeVisible();
    await expect(
      page.locator('#tamponade-operations-list .tamponade-op-card', { hasText: 'Samson (' })
    ).toBeVisible();

    // Step 2: Create new operation "Goliath"
    await page.locator('#create-incident-id').fill('INC-PT-9924');
    await page.locator('#create-pet-name').fill('Goliath');
    await page.locator('#create-species').selectOption('CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA');
    await page.locator('#create-breed').fill('German Shepherd');
    await page.locator('#create-weight-kg').fill('36.0');
    await page.locator('#create-heart-rate').fill('190');
    await page.locator('#create-map').fill('50');
    await page.locator('#create-pulsus-paradoxus').fill('20');
    await page.locator('#create-effusion-pcv').fill('30.0');
    await page.locator('#create-ra-collapse').check();
    await page.locator('#create-electrical-alternans').check();
    await page.locator('#btn-create-tamponade-op').click();

    const goliathCard = page
      .locator('#tamponade-operations-list .tamponade-op-card', { hasText: 'Goliath (' })
      .first();
    await expect(goliathCard).toBeVisible();
    await expect(goliathCard).toHaveAttribute('data-status', 'TAMPONADE_RA_COLLAPSE_ALTERNANS_CRITICAL');

    // Step 3: Stage Right 4th–5th ICS pericardiocentesis plan for Goliath
    await page.locator('#stage-weight-kg').fill('36.0');
    await page.locator('#stage-species').selectOption('CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA');
    await page.locator('#stage-heart-rate').fill('175');
    await page.locator('#stage-map').fill('58');
    await page.locator('#stage-pulsus-paradoxus').fill('18');
    await page.locator('#stage-effusion-pcv').fill('30.0');
    await page.locator('#btn-stage-plan').click();

    await expect(goliathCard).toHaveAttribute('data-status', 'RIGHT_ICS_4_5_PERICARDIOCENTESIS_STAGED');
    await expect(page.locator('#plan-approach-ics')).toContainText('Right 4th-5th ICS Cardiac Notch');
    await expect(page.locator('#plan-lidocaine-infilt')).toContainText('3.60 mL');
    await expect(page.locator('#plan-lidocaine-iv')).toContainText('72.0 mg IV');
    await expect(page.locator('#plan-crystalloid-bolus')).toContainText('360.0 mL IV');

    // Step 4: Log pericardiocentesis tap & clot-tube check for Goliath
    await page.locator('#check-volume-aspirated').fill('280.0');
    await page.locator('#check-post-tap-map').fill('92');
    await page.locator('#check-post-tap-hr').fill('105');
    await page.locator('#check-callsign').fill('DR-CARDIO-44');
    await page.locator('#check-notes').fill('280 mL non-clotting hemorrhagic fluid aspirated via right 4th-5th ICS; immediate normalization of QRS complexes and MAP 92 mmHg.');
    await page.locator('#check-non-clotting-verified').check();
    await page.locator('#btn-record-tap-check').click();

    await expect(page.locator('#tap-checks-list')).toContainText(
      '280 mL non-clotting hemorrhagic fluid aspirated via right 4th-5th ICS; immediate normalization of QRS complexes and MAP 92 mmHg.'
    );

    // Step 5: Issue Pericardiocentesis Decompression Discharge Clearance for Goliath
    await page.locator('#clearance-final-map').fill('94');
    await page.locator('#clearance-vet-license').fill('VET-CARDIO-88120');
    await page.locator('#clearance-summary').fill('Right 4th-5th ICS pericardiocentesis decompression complete; 24h repeat TFAST negative for reaccumulation; Yunnan Baiyao & oncology plan educated.');
    await page.locator('#clearance-ra-collapse-resolved').check();
    await page.locator('#clearance-tfast-no-reaccumulation').check();
    await page.locator('#clearance-sinus-no-vpcs').check();
    await page.locator('#clearance-oncology-educated').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(goliathCard).toHaveAttribute('data-status', 'DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED');
    await expect(page.locator('#tamponade-clearance-banner')).toBeVisible();
    await expect(page.locator('#tamponade-clearance-banner')).toContainText('VET-CARDIO-88120');

    // Step 6: Filter by DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED and verify Goliath and Samson remain visible
    await page.locator('#filter-status').selectOption('DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED');
    await expect(
      page.locator('#tamponade-operations-list .tamponade-op-card', { hasText: 'Goliath (' })
    ).toBeVisible();
    await expect(
      page.locator('#tamponade-operations-list .tamponade-op-card', { hasText: 'Samson (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
