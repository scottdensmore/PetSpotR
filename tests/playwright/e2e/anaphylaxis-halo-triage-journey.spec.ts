import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Anaphylaxis, Hemoabdomen Halo Sign (Gallbladder Wall Edema), Epinephrine IM/CRI & Venous Pool Resuscitation Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Peanut", staging Epinephrine IM/CRI & FFP plan, logging 24h biphasic & Halo Sign check, issuing 24h biphasic discharge clearance, and filtering by BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/anaphylaxis-halo-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#anaphylaxis-halo-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Acute Anaphylaxis, Hemoabdomen Halo Sign (Gallbladder Wall Edema), Epinephrine IM/CRI & Venous Pool Resuscitation Cockpit'
    );

    await expect(
      page.locator('#anaphylaxis-operations-list .anaphylaxis-op-card', { hasText: 'Waffle (' })
    ).toBeVisible();
    await expect(
      page.locator('#anaphylaxis-operations-list .anaphylaxis-op-card', { hasText: 'Bramble (' })
    ).toBeVisible();
    await expect(
      page.locator('#anaphylaxis-operations-list .anaphylaxis-op-card', { hasText: 'Clover (' })
    ).toBeVisible();

    // Step 2: Create new operation "Peanut"
    await page.locator('#create-incident-id').fill('INC-AH-9904');
    await page.locator('#create-pet-name').fill('Peanut');
    await page.locator('#create-species').selectOption('CANINE_HYMENOPTERA_HEMOABDOMEN');
    await page.locator('#create-breed').fill('French Bulldog');
    await page.locator('#create-weight-kg').fill('11.5');
    await page.locator('#create-gb-wall-mm').fill('5.8');
    await page.locator('#create-map-mmhg').fill('48');
    await page.locator('#create-alt-ul').fill('1850');
    await page.locator('#create-pcv-pct').fill('66.0');
    await page.locator('#create-pt-sec').fill('24.5');
    await page.locator('#create-halo-sign-present').check();
    await page.locator('#btn-create-anaphylaxis-op').click();

    const peanutCard = page
      .locator('#anaphylaxis-operations-list .anaphylaxis-op-card', { hasText: 'Peanut (' })
      .first();
    await expect(peanutCard).toBeVisible();
    await expect(peanutCard).toHaveAttribute('data-status', 'ANAPHYLACTIC_SHOCK_HALO_SIGN_CRITICAL');

    // Step 3: Stage Epinephrine IM/CRI, Crystalloid & FFP plan for Peanut
    await page.locator('#stage-weight-kg').fill('11.5');
    await page.locator('#stage-species').selectOption('CANINE_HYMENOPTERA_HEMOABDOMEN');
    await page.locator('#stage-gb-wall-mm').fill('5.8');
    await page.locator('#stage-map-mmhg').fill('48');
    await page.locator('#stage-pcv-pct').fill('66.0');
    await page.locator('#stage-pt-sec').fill('24.5');
    await page.locator('#btn-stage-plan').click();

    await expect(peanutCard).toHaveAttribute('data-status', 'EPINEPHRINE_IM_CRI_CRYSTALLOID_FFP_STAGED');
    await expect(page.locator('#plan-epi-im')).toContainText('0.12 mg IM');
    await expect(page.locator('#plan-crystalloid-bolus')).toContainText('230.0 mL IV');
    await expect(page.locator('#plan-ffp-ml')).toContainText('172.5 mL IV');

    // Step 4: Log serial 24h biphasic & gallbladder Halo Sign check for Peanut
    await page.locator('#check-map-mmhg').fill('86');
    await page.locator('#check-gb-wall-mm').fill('2.2');
    await page.locator('#check-pcv-pct').fill('46.0');
    await page.locator('#check-alt-ul').fill('390');
    await page.locator('#check-callsign').fill('DR-ECC-55');
    await page.locator('#check-notes').fill('AFAST gallbladder Halo Sign resolved (2.2 mm); MAP 86 mmHg off CRI.');
    await page.locator('#check-biphasic-relapse').uncheck();
    await page.locator('#btn-record-biphasic-check').click();

    await expect(page.locator('#biphasic-checks-list')).toContainText(
      'AFAST gallbladder Halo Sign resolved (2.2 mm); MAP 86 mmHg off CRI.'
    );

    // Step 5: Issue 24h Biphasic Normotensive & Halo Sign Resolved Discharge Clearance for Peanut
    await page.locator('#clearance-observation-hours').fill('26');
    await page.locator('#clearance-final-map').fill('88');
    await page.locator('#clearance-vet-license').fill('VET-ECC-88120');
    await page.locator('#clearance-summary').fill('26h biphasic observation completed without relapse; Halo Sign resolved; EpiPen prescribed.');
    await page.locator('#clearance-halo-resolved').check();
    await page.locator('#clearance-coagulopathy-normalized').check();
    await page.locator('#clearance-epipen-educated').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(peanutCard).toHaveAttribute('data-status', 'BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED');
    await expect(page.locator('#anaphylaxis-clearance-banner')).toBeVisible();
    await expect(page.locator('#anaphylaxis-clearance-banner')).toContainText('VET-ECC-88120');

    // Step 6: Filter by BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED and verify Peanut and Clover remain visible
    await page.locator('#filter-status').selectOption('BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED');
    await expect(
      page.locator('#anaphylaxis-operations-list .anaphylaxis-op-card', { hasText: 'Peanut (' })
    ).toBeVisible();
    await expect(
      page.locator('#anaphylaxis-operations-list .anaphylaxis-op-card', { hasText: 'Clover (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
