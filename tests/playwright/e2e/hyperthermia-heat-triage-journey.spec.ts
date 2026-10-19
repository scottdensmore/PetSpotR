import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Hyperthermia, Heatstroke & Hot-Asphalt Pad Burn Triage Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, hot-asphalt paw burn & hyperthermia evaluation, 39.4°C cooling cutoff & hydrogel protocol completion, normothermic clearance issuance, and exposure filter verification', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/hyperthermia-heat-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#hyperthermia-heat-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Hyperthermia, Heatstroke & Hot-Asphalt Pad Burn Triage Cockpit'
    );

    const apolloCard = page.locator('#heat-operations-list .heat-op-card', {
      hasText: 'Apollo (',
    });
    const biscuitCard = page.locator('#heat-operations-list .heat-op-card', {
      hasText: 'Biscuit (',
    });
    const daisyCard = page.locator('#heat-operations-list .heat-op-card', {
      hasText: 'Daisy (',
    });

    await expect(apolloCard).toBeVisible();
    await expect(biscuitCard).toBeVisible();
    await expect(daisyCard).toBeVisible();

    // Step 2: Evaluate custom exertional brachycephalic heatstroke operation ("Solstice")
    await page.locator('#eval-pet-name').fill('Solstice');
    await page.locator('#eval-species').fill('Dog - Pug');
    await page.locator('#eval-weight-kg').fill('9.4');
    await page
      .locator('#eval-exposure-type')
      .selectOption('EXERTIONAL_BRACHYCEPHALIC_HEATSTROKE');
    await page.locator('#eval-rectal-temp-c').fill('41.5');
    await page.locator('#eval-surface-temp-c').fill('62.0');
    await page.locator('#eval-exposure-mins').fill('32');
    await page.locator('#eval-paw-sloughing-grade').selectOption('3');
    await page.locator('#eval-brachycephalic-airway').check();
    await page.locator('#eval-convection-fan').check();
    await page.locator('#eval-ice-bath-avoided').check();

    await page.locator('#btn-submit-heat-eval').click();

    const solsticeInitialCard = page
      .locator('#heat-operations-list .heat-op-card', {
        hasText: 'Solstice (',
      })
      .first();
    await expect(solsticeInitialCard).toBeVisible();
    const solsticeOpId = await solsticeInitialCard.getAttribute('data-op-id');
    expect(solsticeOpId).toBeTruthy();

    const solsticeCard = page.locator(
      `#heat-operations-list .heat-op-card[data-op-id="${solsticeOpId}"]`
    );
    await expect(solsticeCard).toHaveAttribute(
      'data-status',
      'HEATSTROKE_DIC_COAGULOPATHY_CRITICAL'
    );
    await expect(solsticeCard.locator('.heat-cutoff-alert-banner')).toContainText('39.4°C');

    // Step 3: Complete all 4 evaporative cooling & paw hydrogel protocol steps on Solstice's card
    for (const stepId of [
      'STEP-CONVECTIVE-COOLING-START',
      'STEP-STOP-COOLING-394C-CUTOFF',
      'STEP-PAW-PAD-HYDROGEL-DEBRIDE',
      'STEP-DIC-COAG-CRYSTALLOID-IV',
    ]) {
      const cb = solsticeCard.locator(`.heat-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(solsticeCard).toHaveAttribute(
      'data-status',
      'EVAPORATIVE_COOLING_394C_STAGED'
    );

    // Step 4: Issue Normothermic Cooling Clearance Token
    await solsticeCard.locator('.heat-issue-clearance-btn').click();

    await expect(solsticeCard).toHaveAttribute(
      'data-status',
      'STABILIZED_NORMOTHERMIC_COOLED'
    );
    await expect(solsticeCard.locator('.heat-clearance-banner')).toContainText('HEAT-SAFE-');

    // Step 5: Filter by STABILIZED_NORMOTHERMIC_COOLED and verify Solstice and Daisy remain visible
    await page.locator('#filter-status').selectOption('STABILIZED_NORMOTHERMIC_COOLED');
    await expect(
      page.locator('#heat-operations-list .heat-op-card', { hasText: 'Solstice (' })
    ).toBeVisible();
    await expect(
      page.locator('#heat-operations-list .heat-op-card', { hasText: 'Daisy (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
