import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Traumatic Hemoabdomen, Autotransfusion, Centesis Non-Clotting & Norepinephrine/TXA Resuscitation Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Titan", staging resuscitation plan, logging serial hemodynamic check, issuing hemostasis clearance, and filtering by HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/hemoabdomen-autotransfusion-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#hemoabdomen-autotransfusion-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Acute Traumatic Hemoabdomen, Autotransfusion, Centesis Non-Clotting & Norepinephrine/TXA Resuscitation Cockpit'
    );

    await expect(
      page.locator('#hemoabdomen-operations-list .hemoabdomen-op-card', { hasText: 'Apollo (' })
    ).toBeVisible();
    await expect(
      page.locator('#hemoabdomen-operations-list .hemoabdomen-op-card', { hasText: 'Sadie (' })
    ).toBeVisible();
    await expect(
      page.locator('#hemoabdomen-operations-list .hemoabdomen-op-card', { hasText: 'Ranger (' })
    ).toBeVisible();

    // Step 2: Create new operation "Titan"
    await page.locator('#create-incident-id').fill('INC-HA-9404');
    await page.locator('#create-pet-name').fill('Titan');
    await page.locator('#create-species').selectOption('CANINE_BLUNT_TRAUMA_HEPATIC');
    await page.locator('#create-breed').fill('Rottweiler');
    await page.locator('#create-weight-kg').fill('38.0');
    await page.locator('#create-afast-score').fill('4');
    await page.locator('#create-effusion-pcv').fill('40.0');
    await page.locator('#create-peripheral-pcv').fill('30.0');
    await page.locator('#create-map-mmhg').fill('50');
    await page.locator('#create-lactate').fill('6.8');
    await page.locator('#create-non-clotting-verified').check();
    await page.locator('#create-suspected-neoplasia').uncheck();
    await page.locator('#btn-create-hemoabdomen-op').click();

    const titanCard = page
      .locator('#hemoabdomen-operations-list .hemoabdomen-op-card', { hasText: 'Titan (' })
      .first();
    await expect(titanCard).toBeVisible();
    await expect(titanCard).toHaveAttribute('data-status', 'HEMORRHAGIC_SHOCK_HEMOABDOMEN_CRITICAL');

    // Step 3: Stage permissive hypotensive, TXA & autotransfusion resuscitation plan for Titan
    await page.locator('#stage-weight-kg').fill('38.0');
    await page.locator('#stage-species').selectOption('CANINE_BLUNT_TRAUMA_HEPATIC');
    await page.locator('#stage-effusion-pcv').fill('40.0');
    await page.locator('#stage-map-mmhg').fill('58');
    await page.locator('#stage-non-clotting-verified').check();
    await page.locator('#stage-suspected-neoplasia').uncheck();
    await page.locator('#btn-stage-plan').click();

    await expect(titanCard).toHaveAttribute('data-status', 'AUTOTRANSFUSION_TXA_HYPOTENSIVE_STAGED');
    await expect(page.locator('#plan-txa-mg')).toContainText('380.0 mg');
    await expect(page.locator('#plan-max-collect-ml')).toContainText('570.0 mL');

    // Step 4: Log serial hemodynamic & autotransfusion check for Titan
    await page.locator('#check-map-mmhg').fill('72');
    await page.locator('#check-hr-bpm').fill('112');
    await page.locator('#check-peripheral-pcv').fill('33.0');
    await page.locator('#check-total-solids').fill('5.6');
    await page.locator('#check-lactate').fill('2.2');
    await page.locator('#check-autotransfused-ml').fill('420');
    await page.locator('#check-callsign').fill('DR-TRAUMA-14');
    await page
      .locator('#check-notes')
      .fill('420 mL autologous shed blood reinfused via 170 µm filter; MAP 72 mmHg.');
    await page.locator('#btn-record-hemodynamic-check').click();

    await expect(page.locator('#hemodynamic-checks-list')).toContainText(
      '420 mL autologous shed blood reinfused via 170 µm filter; MAP 72 mmHg.'
    );

    // Step 5: Issue Surgical / Spontaneous Hemostasis Normotensive Clearance for Titan
    await page.locator('#clearance-final-map').fill('84');
    await page.locator('#clearance-final-pcv').fill('34.0');
    await page.locator('#clearance-final-lactate').fill('1.5');
    await page.locator('#clearance-vet-license').fill('VET-SURG-77219');
    await page
      .locator('#clearance-summary')
      .fill('Surgical hemostasis confirmed; repeat AFAST AFS 0/4 free fluid resolved.');
    await page.locator('#clearance-surgical-hemostasis').check();
    await page.locator('#clearance-repeat-afast-resolved').check();
    await page.locator('#btn-issue-hemostasis-clearance').click();

    await expect(titanCard).toHaveAttribute(
      'data-status',
      'HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE'
    );
    await expect(page.locator('#hemoabdomen-clearance-banner')).toBeVisible();
    await expect(page.locator('#hemoabdomen-clearance-banner')).toContainText('VET-SURG-77219');

    // Step 6: Filter by HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE and verify Titan and Ranger remain visible
    await page.locator('#filter-status').selectOption('HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE');
    await expect(
      page.locator('#hemoabdomen-operations-list .hemoabdomen-op-card', { hasText: 'Titan (' })
    ).toBeVisible();
    await expect(
      page.locator('#hemoabdomen-operations-list .hemoabdomen-op-card', { hasText: 'Ranger (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
