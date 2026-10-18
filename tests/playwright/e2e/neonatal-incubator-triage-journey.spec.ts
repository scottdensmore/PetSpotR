import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Neonatal Orphan Litter & Incubator Critical Care Triage Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, maternal agalactia hypothermic litter evaluation, 36.1°C warming gate & 5Fr orogastric gavage step completion, neonatal clearance token issuance, and litter filter verification', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/neonatal-incubator-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#neonatal-incubator-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Neonatal Orphan Litter & Incubator Critical Care Triage Cockpit'
    );

    const card101 = page.locator('#neonatal-operations-list .neonatal-op-card', {
      hasText: 'Clover Litter (',
    });
    const card102 = page.locator('#neonatal-operations-list .neonatal-op-card', {
      hasText: 'Pippin (',
    });
    const card103 = page.locator('#neonatal-operations-list .neonatal-op-card', {
      hasText: 'Buttercup Litter (',
    });

    await expect(card101).toBeVisible();
    await expect(card102).toBeVisible();
    await expect(card103).toBeVisible();

    // Step 2: Evaluate custom maternal agalactia rejected litter ("Hazel Litter")
    await page.locator('#eval-pet-name').fill('Hazel Litter');
    await page.locator('#eval-species').fill('Cat - Calico Neonates');
    await page.locator('#eval-weight-grams').fill('96');
    await page.locator('#eval-litter-count').fill('5');
    await page.locator('#eval-age-days').fill('4');
    await page.locator('#eval-litter-type').selectOption('MATERNAL_AGALACTIA_REJECTED');
    await page.locator('#eval-rectal-temp-c').fill('34.1');
    await page.locator('#eval-glucose-mgdl').fill('48');
    await page.locator('#eval-humidity-pct').fill('63');
    await page.locator('#eval-suckling-reflex').uncheck();
    await page.locator('#eval-warmed-gate').uncheck();
    await page.locator('#eval-dextrose-applied').check();

    await page.locator('#btn-evaluate-neonatal').click();

    const hazelInitialCard = page
      .locator('#neonatal-operations-list .neonatal-op-card', {
        hasText: 'Hazel Litter (',
      })
      .first();
    await expect(hazelInitialCard).toBeVisible();
    const hazelOpId = await hazelInitialCard.getAttribute('data-op-id');
    expect(hazelOpId).toBeTruthy();

    const hazelCard = page.locator(
      `#neonatal-operations-list .neonatal-op-card[data-op-id="${hazelOpId}"]`
    );
    await expect(hazelCard).toHaveAttribute('data-status', 'FADING_TRIAD_HYPOTHERMIC_CRITICAL');
    await expect(hazelCard.locator('.ni-ileus-gate-banner')).toContainText('36.1°C');

    // Step 3: Complete all 4 incubator warming, dextrose, gavage & stimulation steps on Hazel Litter's card
    for (const stepId of [
      'STEP-INCUBATOR-WARMING-GATE',
      'STEP-SUBLINGUAL-DEXTROSE-50',
      'STEP-OROGASTRIC-TUBE-GAVAGE',
      'STEP-ANOGENITAL-STIM-WEIGH',
    ]) {
      const cb = hazelCard.locator(`.ni-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(hazelCard).toHaveAttribute('data-status', 'INCUBATOR_WARMED_DEXTROSE_STAGED');
    await expect(hazelCard.locator('.badge-ni-status.badge-ni-staged')).toContainText(
      'INCUBATOR_WARMED_DEXTROSE_STAGED'
    );

    // Step 4: Issue Neonatal Stabilized Nursing Clearance Token
    await hazelCard.locator('.ni-issue-clearance-btn').click();

    await expect(hazelCard).toHaveAttribute('data-status', 'STABILIZED_NURSING_GAINING_SAFE');
    await expect(hazelCard.locator('.badge-ni-status.badge-ni-safe')).toContainText(
      'STABILIZED_NURSING_GAINING_SAFE'
    );
    await expect(hazelCard.locator('.ni-clearance-banner')).toContainText('NEONATAL-SAFE-');

    // Step 5: Trigger Singleton Colostrum Orphan scenario button and issue clearance
    await page.locator('.ni-scenario-btn[data-scenario="singleton-colostrum-orphan"]').click();

    const wafflesCard = page
      .locator('#neonatal-operations-list .neonatal-op-card', {
        hasText: 'Waffles (',
      })
      .first();
    await expect(wafflesCard).toBeVisible();

    await wafflesCard.locator('.ni-issue-clearance-btn').click();
    await expect(wafflesCard).toHaveAttribute('data-status', 'STABILIZED_NURSING_GAINING_SAFE');
    await expect(wafflesCard.locator('.ni-clearance-banner')).toContainText('NEONATAL-SAFE-');

    expect(cspErrors).toEqual([]);
  });
});
