import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Fireworks/Thunderstorm Acoustic Panic, Bolt-Trajectory Vectoring & Calming Pheromone SAR Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, thunderstorm barometric drop & acoustic panic evaluation, silent no-chase & pheromone lure protocol completion, acoustic containment clearance issuance, and status filter verification', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/acoustic-panic-trajectory`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#acoustic-panic-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Fireworks/Thunderstorm Acoustic Panic, Bolt-Trajectory Vectoring & Calming Pheromone SAR Cockpit'
    );

    const boomerCard = page.locator('#acoustic-operations-list .acoustic-op-card', {
      hasText: 'Boomer (',
    });
    const willowCard = page.locator('#acoustic-operations-list .acoustic-op-card', {
      hasText: 'Willow (',
    });
    const miloCard = page.locator('#acoustic-operations-list .acoustic-op-card', {
      hasText: 'Milo (',
    });

    await expect(boomerCard).toBeVisible();
    await expect(willowCard).toBeVisible();
    await expect(miloCard).toBeVisible();

    // Step 2: Evaluate custom severe thunderstorm barometric drop operation ("Zephyr")
    await page.locator('#eval-pet-name').fill('Zephyr');
    await page.locator('#eval-species').fill('Dog - Weimaraner');
    await page.locator('#eval-weight-kg').fill('29.2');
    await page
      .locator('#eval-trigger-type')
      .selectOption('SEVERE_THUNDERSTORM_BAROMETRIC_DROP');
    await page.locator('#eval-peak-db').fill('124.0');
    await page.locator('#eval-barometric-hpa').fill('15.5');
    await page.locator('#eval-heading-deg').fill('175');
    await page.locator('#eval-barrier-breach').check();
    await page.locator('#eval-no-chase-enforced').check();
    await page.locator('#eval-pheromone-staged').check();

    await page.locator('#btn-submit-acoustic-eval').click();

    const zephyrInitialCard = page
      .locator('#acoustic-operations-list .acoustic-op-card', {
        hasText: 'Zephyr (',
      })
      .first();
    await expect(zephyrInitialCard).toBeVisible();
    const zephyrOpId = await zephyrInitialCard.getAttribute('data-op-id');
    expect(zephyrOpId).toBeTruthy();

    const zephyrCard = page.locator(
      `#acoustic-operations-list .acoustic-op-card[data-op-id="${zephyrOpId}"]`
    );
    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'ADRENAL_FLIGHT_BOLT_CRITICAL'
    );
    await expect(zephyrCard.locator('.acoustic-advisory-alert-banner')).toContainText('NEVER');

    // Step 3: Complete all 4 silent no-chase & pheromone refuge lure protocol steps on Zephyr's card
    for (const stepId of [
      'STEP-SILENT-NO-CHASE-PERIMETER',
      'STEP-SCENT-PHEROMONE-REFUGE-LURE',
      'STEP-BARRIER-LACERATION-CLAW-CHECK',
      'STEP-THUNDERSHIRT-ACOUSTIC-DECOMPRESS',
    ]) {
      const cb = zephyrCard.locator(`.acoustic-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'CALMING_PERIMETER_SNIFF_STAGED'
    );

    // Step 4: Issue Acoustic Containment Clearance Token
    await zephyrCard.locator('.acoustic-issue-clearance-btn').click();

    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'DECOMPRESSED_CONTAINED_SAFE'
    );
    await expect(zephyrCard.locator('.acoustic-clearance-banner')).toContainText('ACOUSTIC-SAFE-');

    // Step 5: Filter by DECOMPRESSED_CONTAINED_SAFE and verify Zephyr and Milo remain visible
    await page.locator('#filter-status').selectOption('DECOMPRESSED_CONTAINED_SAFE');
    await expect(
      page.locator('#acoustic-operations-list .acoustic-op-card', { hasText: 'Zephyr (' })
    ).toBeVisible();
    await expect(
      page.locator('#acoustic-operations-list .acoustic-op-card', { hasText: 'Milo (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
