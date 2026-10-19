import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Hyperbaric Oxygen (HBOT), Carbon Monoxide (CO) Carboxyhemoglobin & Cyanide Smoke Triage Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, structure fire COHb & cyanide lactatemia evaluation, CO-oximetry & hydroxocobalamin HBOT protocol completion, normoxic clearance issuance, and exposure source filter verification', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/hyperbaric-co-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#hyperbaric-co-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Hyperbaric Oxygen (HBOT), Carbon Monoxide (CO) Carboxyhemoglobin & Cyanide Smoke Triage Cockpit'
    );

    const cinderCard = page.locator('#hbot-operations-list .hbot-op-card', {
      hasText: 'Cinder (',
    });
    const dieselCard = page.locator('#hbot-operations-list .hbot-op-card', {
      hasText: 'Diesel (',
    });
    const pebbleCard = page.locator('#hbot-operations-list .hbot-op-card', {
      hasText: 'Pebble (',
    });

    await expect(cinderCard).toBeVisible();
    await expect(dieselCard).toBeVisible();
    await expect(pebbleCard).toBeVisible();

    // Step 2: Evaluate custom smoldering barn smoke & CO operation ("Zephyr")
    await page.locator('#eval-pet-name').fill('Zephyr');
    await page.locator('#eval-species').fill('Dog - Australian Cattle Dog');
    await page.locator('#eval-weight-kg').fill('19.2');
    await page
      .locator('#eval-exposure-source')
      .selectOption('SMOLDERING_BARN_TORF_SMOKE');
    await page.locator('#eval-cohb-pct').fill('29.4');
    await page.locator('#eval-lactate-mmol').fill('9.6');
    await page.locator('#eval-chamber-ata').fill('2.0');
    await page.locator('#eval-singe-soot').check();
    await page.locator('#eval-spo2-warning-noted').check();
    await page.locator('#eval-hydroxocobalamin').check();

    await page.locator('#btn-submit-hbot-eval').click();

    const zephyrInitialCard = page
      .locator('#hbot-operations-list .hbot-op-card', {
        hasText: 'Zephyr (',
      })
      .first();
    await expect(zephyrInitialCard).toBeVisible();
    const zephyrOpId = await zephyrInitialCard.getAttribute('data-op-id');
    expect(zephyrOpId).toBeTruthy();

    const zephyrCard = page.locator(
      `#hbot-operations-list .hbot-op-card[data-op-id="${zephyrOpId}"]`
    );
    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'CARBOXYHEMOGLOBIN_CYANIDE_CRITICAL'
    );
    await expect(zephyrCard.locator('.hbot-cooximetry-alert-banner')).toContainText('98-100%');

    // Step 3: Complete all 4 CO-oximetry, hydroxocobalamin & HBOT washout protocol steps on Zephyr's card
    for (const stepId of [
      'STEP-CO-OXIMETRY-FALSE-SPO2-BYPASS',
      'STEP-HYDROXOCOBALAMIN-CYANIDE-BIND',
      'STEP-HBOT-CHAMBER-2ATA-WASHOUT',
      'STEP-DELAYED-NEURO-SEQUELAE-CHECK',
    ]) {
      const cb = zephyrCard.locator(`.hbot-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'HBOT_CHAMBER_HYDROXOCOBALAMIN_STAGED'
    );

    // Step 4: Issue Normoxic COHb Clearance Token
    await zephyrCard.locator('.hbot-issue-clearance-btn').click();

    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'NORMOXIC_COHB_CLEARED_SAFE'
    );
    await expect(zephyrCard.locator('.hbot-clearance-banner')).toContainText('HBOT-SAFE-');

    // Step 5: Filter by NORMOXIC_COHB_CLEARED_SAFE and verify Zephyr and Pebble remain visible
    await page.locator('#filter-status').selectOption('NORMOXIC_COHB_CLEARED_SAFE');
    await expect(
      page.locator('#hbot-operations-list .hbot-op-card', { hasText: 'Zephyr (' })
    ).toBeVisible();
    await expect(
      page.locator('#hbot-operations-list .hbot-op-card', { hasText: 'Pebble (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
