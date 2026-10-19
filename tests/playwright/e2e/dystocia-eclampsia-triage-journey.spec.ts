import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Dystocia, Maternal C-Section, Eclampsia (Hypocalcemia) & Oxytocin Contraindication Triage Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, placental detachment & fetal distress evaluation, Doppler fetal HR & ECG calcium gluconate protocol completion, normocalcemic clearance issuance, and status filter verification', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/dystocia-eclampsia-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#dystocia-eclampsia-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Dystocia, Maternal C-Section, Eclampsia (Hypocalcemia) & Oxytocin Contraindication Triage Cockpit'
    );

    const penelopeCard = page.locator('#dystocia-operations-list .dystocia-op-card', {
      hasText: 'Penelope (',
    });
    const mapleCard = page.locator('#dystocia-operations-list .dystocia-op-card', {
      hasText: 'Maple (',
    });
    const rosieCard = page.locator('#dystocia-operations-list .dystocia-op-card', {
      hasText: 'Rosie (',
    });

    await expect(penelopeCard).toBeVisible();
    await expect(mapleCard).toBeVisible();
    await expect(rosieCard).toBeVisible();

    // Step 2: Evaluate custom placental detachment & uteroverdin discharge operation ("Beatrice")
    await page.locator('#eval-pet-name').fill('Beatrice');
    await page.locator('#eval-species').fill('Dog - Pug Dam');
    await page.locator('#eval-weight-kg').fill('8.8');
    await page
      .locator('#eval-presentation-type')
      .selectOption('PLACENTAL_DETACHMENT_UTEROVERDIN_DISCHARGE');
    await page.locator('#eval-fetal-hr').fill('158');
    await page.locator('#eval-ionized-calcium').fill('0.86');
    await page.locator('#eval-interval-mins').fill('135');
    await page.locator('#eval-uteroverdin').check();
    await page.locator('#eval-obstruction-excluded').uncheck();
    await page.locator('#eval-ecg-calcium').check();

    await page.locator('#btn-submit-dystocia-eval').click();

    const beatriceInitialCard = page
      .locator('#dystocia-operations-list .dystocia-op-card', {
        hasText: 'Beatrice (',
      })
      .first();
    await expect(beatriceInitialCard).toBeVisible();
    const beatriceOpId = await beatriceInitialCard.getAttribute('data-op-id');
    expect(beatriceOpId).toBeTruthy();

    const beatriceCard = page.locator(
      `#dystocia-operations-list .dystocia-op-card[data-op-id="${beatriceOpId}"]`
    );
    await expect(beatriceCard).toHaveAttribute(
      'data-status',
      'FETAL_DISTRESS_OBSTRUCTIVE_CRITICAL'
    );
    await expect(
      beatriceCard.locator('.dystocia-oxytocin-alert-banner')
    ).toContainText('uterine rupture');

    // Step 3: Complete all 4 obstetric & calcium gluconate protocol steps on Beatrice's card
    for (const stepId of [
      'STEP-DOPPLER-FETAL-HR-PELVIC-XRAY',
      'STEP-ECG-CALCIUM-GLUCONATE-INFUSION',
      'STEP-OXYTOCIN-GATE-OR-EMERGENCY-CSECTION',
      'STEP-NEONATAL-RESUS-COLOSTRUM-LATCH',
    ]) {
      const cb = beatriceCard.locator(
        `.dystocia-step-checkbox[data-step-id="${stepId}"]`
      );
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(beatriceCard).toHaveAttribute(
      'data-status',
      'CALCIUM_ECG_WHELPING_STAGED'
    );

    // Step 4: Issue Delivered Normocalcemic Clearance Token
    await beatriceCard.locator('.dystocia-issue-clearance-btn').click();

    await expect(beatriceCard).toHaveAttribute(
      'data-status',
      'DELIVERED_NORMOCALCEMIC_LACTATING_SAFE'
    );
    await expect(beatriceCard.locator('.dystocia-clearance-banner')).toContainText(
      'DYSTOCIA-SAFE-'
    );

    // Step 5: Filter by DELIVERED_NORMOCALCEMIC_LACTATING_SAFE and verify Beatrice and Rosie remain visible
    await page
      .locator('#filter-status')
      .selectOption('DELIVERED_NORMOCALCEMIC_LACTATING_SAFE');
    await expect(
      page.locator('#dystocia-operations-list .dystocia-op-card', {
        hasText: 'Beatrice (',
      })
    ).toBeVisible();
    await expect(
      page.locator('#dystocia-operations-list .dystocia-op-card', {
        hasText: 'Rosie (',
      })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
