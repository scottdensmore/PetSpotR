import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe("Traumatic Brain Injury (TBI), Modified Glasgow Coma Scale (MGCS), Cushing's Reflex & Hypertonic Saline/Mannitol Cockpit E2E Journey", () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Zephyr", staging 7.5% HTS/Mannitol/30° head elevation plan, logging serial MGCS & neuro check, issuing MGCS-recovered normocapnic discharge clearance, and filtering by MGCS_RECOVERED_NORMOCAPNIC_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/traumatic-brain-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#traumatic-brain-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      "Traumatic Brain Injury (TBI), Modified Glasgow Coma Scale (MGCS), Cushing's Reflex & Hypertonic Saline/Mannitol Triage Cockpit"
    );

    await expect(
      page.locator('#tbi-operations-list .tbi-op-card', { hasText: 'Atlas (' })
    ).toBeVisible();
    await expect(
      page.locator('#tbi-operations-list .tbi-op-card', { hasText: 'Milo (' })
    ).toBeVisible();
    await expect(
      page.locator('#tbi-operations-list .tbi-op-card', { hasText: 'Sierra (' })
    ).toBeVisible();

    // Step 2: Create new operation "Zephyr"
    await page.locator('#create-incident-id').fill('INC-TBI-9804');
    await page.locator('#create-pet-name').fill('Zephyr');
    await page.locator('#create-species').selectOption('CANINE_HIGH_VELOCITY_CRANIAL');
    await page.locator('#create-breed').fill('Belgian Malinois');
    await page.locator('#create-weight-kg').fill('29.0');
    await page.locator('#create-mgcs').fill('9');
    await page.locator('#create-map-mmhg').fill('146');
    await page.locator('#create-hr-bpm').fill('48');
    await page.locator('#create-etco2-mmhg').fill('46.0');
    await page.locator('#create-pupils').selectOption('ANISOCORIA_UNILATERAL_MYDRIASIS');
    await page.locator('#create-cushing-reflex').check();
    await page.locator('#btn-create-tbi-op').click();

    const zephyrCard = page
      .locator('#tbi-operations-list .tbi-op-card', { hasText: 'Zephyr (' })
      .first();
    await expect(zephyrCard).toBeVisible();
    await expect(zephyrCard).toHaveAttribute('data-status', 'CUSHING_REFLEX_ICP_HERNIATION_CRITICAL');

    // Step 3: Stage 7.5% HTS / Mannitol / 30° Head Elevation plan for Zephyr
    await page.locator('#stage-weight-kg').fill('29.0');
    await page.locator('#stage-species').selectOption('CANINE_HIGH_VELOCITY_CRANIAL');
    await page.locator('#stage-mgcs').fill('13');
    await page.locator('#stage-map-mmhg').fill('94');
    await page.locator('#stage-hr-bpm').fill('104');
    await page.locator('#stage-etco2-mmhg').fill('38.0');
    await page.locator('#stage-pupils').selectOption('MIOTIC_PINPOINT_BILATERAL');
    await page.locator('#btn-stage-plan').click();

    await expect(zephyrCard).toHaveAttribute('data-status', 'HTS_MANNITOL_30DEG_HEAD_ELEVATION_STAGED');
    await expect(page.locator('#plan-hts-dose')).toContainText('4.0 mL/kg (116.0 mL)');
    await expect(page.locator('#plan-mannitol-grams')).toContainText('14.50 g');

    // Step 4: Log serial MGCS & neuro check for Zephyr
    await page.locator('#check-mgcs').fill('16');
    await page.locator('#check-map-mmhg').fill('96');
    await page.locator('#check-hr-bpm').fill('88');
    await page.locator('#check-etco2-mmhg').fill('37.0');
    await page.locator('#check-pupils').selectOption('RESPONSIVE_ISOCORIC');
    await page.locator('#check-callsign').fill('DR-NEURO-22');
    await page.locator('#check-notes').fill('MGCS improved to 16/18; pupils isocoric and responsive; MAP 96 mmHg.');
    await page.locator('#btn-record-neuro-check').click();

    await expect(page.locator('#neuro-checks-list')).toContainText(
      'MGCS improved to 16/18; pupils isocoric and responsive; MAP 96 mmHg.'
    );

    // Step 5: Issue MGCS-Recovered Normocapnic Discharge Clearance for Zephyr
    await page.locator('#clearance-final-mgcs').fill('17');
    await page.locator('#clearance-final-map').fill('98');
    await page.locator('#clearance-vet-license').fill('VET-NEURO-88120');
    await page.locator('#clearance-summary').fill('MGCS recovered to 17/18; normotensive MAP 98 mmHg; responsive isocoric pupils; intact gag/swallow reflex.');
    await page.locator('#clearance-pupils-isocoric').check();
    await page.locator('#clearance-gag-reflex').check();
    await page.locator('#clearance-ambulatory-normocapnic').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(zephyrCard).toHaveAttribute('data-status', 'MGCS_RECOVERED_NORMOCAPNIC_CLEARED');
    await expect(page.locator('#tbi-clearance-banner')).toBeVisible();
    await expect(page.locator('#tbi-clearance-banner')).toContainText('VET-NEURO-88120');

    // Step 6: Filter by MGCS_RECOVERED_NORMOCAPNIC_CLEARED and verify Zephyr and Sierra remain visible
    await page.locator('#filter-status').selectOption('MGCS_RECOVERED_NORMOCAPNIC_CLEARED');
    await expect(
      page.locator('#tbi-operations-list .tbi-op-card', { hasText: 'Zephyr (' })
    ).toBeVisible();
    await expect(
      page.locator('#tbi-operations-list .tbi-op-card', { hasText: 'Sierra (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
