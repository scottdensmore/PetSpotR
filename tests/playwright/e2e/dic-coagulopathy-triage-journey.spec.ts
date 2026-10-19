import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Coagulopathy & Disseminated Intravascular Coagulation (DIC), PT/aPTT Prolongation, D-Dimer/FDP Elevation, Fresh Frozen Plasma (FFP) & Tranexamic Acid (TXA) Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new DIC patient "Zephyr", staging FFP/Cryoprecipitate/TXA protocol, recording coagulation panel check, clearing for normocoagulable discharge, and filtering by HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/dic-coagulopathy-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#dic-coagulopathy-cockpit')).toBeVisible();
    await expect(page.locator('#dic-alert-banner')).toBeVisible();

    await expect(
      page.locator('#dic-operations-grid .dic-op-card', { hasText: 'Flash (' })
    ).toBeVisible();
    await expect(
      page.locator('#dic-operations-grid .dic-op-card', { hasText: 'Rocket (' })
    ).toBeVisible();
    await expect(
      page.locator('#dic-operations-grid .dic-op-card', { hasText: 'Willow (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new DIC patient "Zephyr"
    await page.locator('#create-case-id-input').fill('CASE-DIC-104');
    await page.locator('#create-pet-name-input').fill('Zephyr');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Retired Racing Greyhound');
    await page.locator('#create-weight-input').fill('29.5');
    await page
      .locator('#create-phenotype-input')
      .selectOption('HYPO_COAGULABLE_HEMORRHAGIC_CONSUMPTIVE_DIC');
    await page.locator('#create-platelets-input').fill('42.0');
    await page.locator('#create-pt-input').fill('23.2');
    await page.locator('#create-aptt-input').fill('138.0');
    await page.locator('#create-ddimer-input').fill('1720.0');
    await page.locator('#create-fibrinogen-input').fill('74.0');
    await page.locator('#create-at3-input').fill('51.0');
    await page.locator('#create-schistocytes-checkbox').check();
    await page.locator('#create-bleeding-checkbox').check();
    await page.locator('#create-dic-op-btn').click();

    const zephyrCard = page
      .locator('#dic-operations-grid .dic-op-card', { hasText: 'Zephyr (' })
      .first();
    await expect(zephyrCard).toBeVisible();
    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE'
    );
    await expect(page.locator('#active-op-title')).toContainText('Zephyr');
    await expect(page.locator('#active-d-dimer')).toContainText('1720.0 ng/mL');
    await expect(page.locator('#active-fibrinogen')).toContainText('74.0 mg/dL');

    // Step 4: Stage FFP, Cryoprecipitate & Tranexamic Acid (TXA) protocol for Zephyr
    await page.locator('#stage-therapy-btn').click();
    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'FFP_CRYOPRECIPITATE_TXA_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'FFP_CRYOPRECIPITATE_TXA_STAGED'
    );

    // Step 5: Record serial coagulation panel check for Zephyr
    await page.locator('#check-platelets-input').fill('158.0');
    await page.locator('#check-pt-input').fill('11.2');
    await page.locator('#check-aptt-input').fill('68.0');
    await page.locator('#check-ddimer-input').fill('210.0');
    await page.locator('#check-fibrinogen-input').fill('205.0');
    await page.locator('#check-recorded-by-input').fill('Dr. Callum Vance, DACVECC');
    await page
      .locator('#check-note-input')
      .fill('Post-FFP, Cryoprecipitate & TXA infusion: Zephyr mucosal bleeding resolved, Fibrinogen 205.0 mg/dL, D-Dimer 210.0 ng/mL.');
    await page.locator('#record-coag-check-btn').click();

    await expect(page.locator('#active-d-dimer')).toContainText('210.0 ng/mL');
    await expect(page.locator('#active-fibrinogen')).toContainText('205.0 mg/dL');
    await expect(page.locator('#coag-checks-list')).toContainText(
      'Post-FFP, Cryoprecipitate & TXA infusion: Zephyr mucosal bleeding resolved'
    );

    // Step 6: Clear Zephyr for Normocoagulable Discharge
    await page.locator('#clear-discharged-by-input').fill('Dr. Callum Vance, DACVECC');
    await page
      .locator('#clear-summary-input')
      .fill('Zephyr hemostasis restored, PT/aPTT normalized (11.2s / 68.0s), Fibrinogen 205.0 mg/dL, D-Dimer 210.0 ng/mL, cleared for step-down recovery.');
    await page.locator('#clear-dic-btn').click();

    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED'
    );
    await expect(page.locator('#dic-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Zephyr');

    // Step 7: Filter by HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED
    await page
      .locator('#dic-status-filter')
      .selectOption('HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED');
    await expect(
      page.locator('#dic-operations-grid .dic-op-card', { hasText: 'Zephyr (' })
    ).toBeVisible();
    await expect(
      page.locator('#dic-operations-grid .dic-op-card', { hasText: 'Willow (' })
    ).toBeVisible();

    await page.locator('#dic-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
