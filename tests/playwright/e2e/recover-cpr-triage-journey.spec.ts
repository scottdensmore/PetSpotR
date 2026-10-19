import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Cardiopulmonary Resuscitation (RECOVER Guidelines), Open/Closed-Chest Compression, EtCO2 > 15 mmHg & Defibrillation Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Buster", staging RECOVER 2-Min ALS & Defib plan, logging 2-min cycle & EtCO2 check, issuing sustained ROSC & PCA clearance, and filtering by ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/recover-cpr-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#recover-cpr-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Cardiopulmonary Resuscitation (RECOVER Guidelines), Open/Closed-Chest Compression, EtCO2 > 15 mmHg & Defibrillation Cockpit'
    );

    await expect(
      page.locator('#cpr-operations-list .cpr-op-card', { hasText: 'Duke (' })
    ).toBeVisible();
    await expect(
      page.locator('#cpr-operations-list .cpr-op-card', { hasText: 'Gizmo (' })
    ).toBeVisible();
    await expect(
      page.locator('#cpr-operations-list .cpr-op-card', { hasText: 'Pixel (' })
    ).toBeVisible();

    // Step 2: Create new operation "Buster"
    await page.locator('#create-incident-id').fill('INC-CPR-9944');
    await page.locator('#create-pet-name').fill('Buster');
    await page.locator('#create-species').selectOption('CANINE_ROUND_CHEST_CARDIAC_PUMP');
    await page.locator('#create-breed').fill('Golden Retriever');
    await page.locator('#create-weight-kg').fill('28.0');
    await page.locator('#create-arrest-rhythm').selectOption('VFIB_PULSELESS_VT_SHOCKABLE');
    await page.locator('#create-etco2-mmhg').fill('12.5');
    await page.locator('#create-cycles-completed').fill('1');
    await page.locator('#create-reversible-present').check();
    await page.locator('#btn-create-cpr-op').click();

    const busterCard = page
      .locator('#cpr-operations-list .cpr-op-card', { hasText: 'Buster (' })
      .first();
    await expect(busterCard).toBeVisible();
    await expect(busterCard).toHaveAttribute('data-status', 'CARDIOPULMONARY_ARREST_BLS_ALS_CRITICAL');

    // Step 3: Stage RECOVER 2-Min ALS & Defib plan for Buster
    await page.locator('#stage-weight-kg').fill('28.0');
    await page.locator('#stage-species').selectOption('CANINE_ROUND_CHEST_CARDIAC_PUMP');
    await page.locator('#stage-arrest-rhythm').selectOption('VFIB_PULSELESS_VT_SHOCKABLE');
    await page.locator('#stage-etco2-mmhg').fill('19.5');
    await page.locator('#stage-cycles-completed').fill('2');
    await page.locator('#stage-reversible-present').check();
    await page.locator('#btn-stage-plan').click();

    await expect(busterCard).toHaveAttribute('data-status', 'RECOVER_2MIN_CYCLE_EPI_DEFIB_STAGED');
    await expect(page.locator('#plan-hand-placement')).toContainText('Widest Part of Thorax');
    await expect(page.locator('#plan-epi-low-dose')).toContainText('0.28 mg IV/IO');
    await expect(page.locator('#plan-biphasic-defib')).toContainText('112.0 J');
    await expect(page.locator('#plan-amiodarone-dose')).toContainText('140.0 mg IV/IO');

    // Step 4: Log 2-minute BLS/ALS cycle & EtCO2 check for Buster
    await page.locator('#check-cycle-number').fill('3');
    await page.locator('#check-etco2-mmhg').fill('39.0');
    await page.locator('#check-rhythm-pause').fill('VFIB_PULSELESS_VT_SHOCKABLE');
    await page.locator('#check-pause-seconds').fill('6');
    await page.locator('#check-callsign').fill('DR-RECOVER-22');
    await page.locator('#check-notes').fill('Abrupt EtCO2 jump to 39.0 mmHg after 112 J biphasic shock and Amiodarone 140 mg IV.');
    await page.locator('#check-compressor-rotated').check();
    await page.locator('#btn-record-cycle-check').click();

    await expect(page.locator('#cycle-checks-list')).toContainText(
      'Abrupt EtCO2 jump to 39.0 mmHg after 112 J biphasic shock and Amiodarone 140 mg IV.'
    );

    // Step 5: Issue Sustained ROSC & Normocapnic PCA Clearance for Buster
    await page.locator('#clearance-rosc-minutes').fill('30');
    await page.locator('#clearance-final-etco2').fill('39.0');
    await page.locator('#clearance-final-map').fill('94');
    await page.locator('#clearance-vet-license').fill('VET-RECOVER-88120');
    await page.locator('#clearance-summary').fill('Sustained ROSC 30 min; normocapnic EtCO2 39.0 mmHg, MAP 94 mmHg, SpO2 96%.');
    await page.locator('#clearance-spo2-titrated').check();
    await page.locator('#clearance-hypothermia-maintained').check();
    await page.locator('#btn-issue-rosc-clearance').click();

    await expect(busterCard).toHaveAttribute('data-status', 'ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE');
    await expect(page.locator('#cpr-clearance-banner')).toBeVisible();
    await expect(page.locator('#cpr-clearance-banner')).toContainText('VET-RECOVER-88120');

    // Step 6: Filter by ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE and verify Buster and Pixel remain visible
    await page.locator('#filter-status').selectOption('ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE');
    await expect(
      page.locator('#cpr-operations-list .cpr-op-card', { hasText: 'Buster (' })
    ).toBeVisible();
    await expect(
      page.locator('#cpr-operations-list .cpr-op-card', { hasText: 'Pixel (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
