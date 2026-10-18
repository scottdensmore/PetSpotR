import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Avian, Exotic Parrot & Falconry Telemetry Receiver, Flight-Corridor Thermals & Lure-Call Recovery Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, high-canopy macaw thermal drift & crop depletion evaluation, VHF telemetry & acoustic lure step completion, avian safe recovery certificate issuance, and cockatiel courtyard perch simulation', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/avian-recovery`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#avian-recovery-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Avian, Exotic Parrot & Falconry Telemetry Receiver, Flight-Corridor Thermals & Lure-Call Recovery Cockpit'
    );

    const card101 = page.locator('#av-ops-list .av-op-card[data-op-id="AV-OP-101"]');
    const card102 = page.locator('#av-ops-list .av-op-card[data-op-id="AV-OP-102"]');
    const card103 = page.locator('#av-ops-list .av-op-card[data-op-id="AV-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card102).toBeVisible();
    await expect(card103).toBeVisible();

    // Step 2: Evaluate custom psittacine macaw operation ("Zazu")
    await page.locator('#av-pet-id').fill('PET-E2E-ZAZU');
    await page.locator('#av-pet-name').fill('Zazu');
    await page.locator('#av-species').selectOption('Avian');
    await page.locator('#av-weight-grams').fill('1040');
    await page.locator('#av-class-type').selectOption('PSITTACINE_PARROT_MACAW');
    await page.locator('#av-perch-altitude').fill('27.5');
    await page.locator('#av-thermal-updraft').fill('23.0');
    await page.locator('#av-ambient-temp').fill('7.0');
    await page.locator('#av-hours-aloft').fill('20.5');
    await page.locator('#av-telemetry-dbm').fill('-80');
    await page.locator('#av-corridor-bearing').fill('CORRIDOR-SSE-168° (Magnolia Bluff Conifer Ridge)');
    await page.locator('#av-tech-callsign').fill('AVIAN-TECH-09 (Vega)');

    await page.locator('#btn-evaluate-av-op').click();

    const zazuInitialCard = page
      .locator('#av-ops-list .av-op-card', {
        hasText: 'Zazu',
      })
      .first();
    await expect(zazuInitialCard).toBeVisible();
    const zazuOpId = await zazuInitialCard.getAttribute('data-op-id');
    expect(zazuOpId).toBeTruthy();

    const zazuCard = page.locator(`#av-ops-list .av-op-card[data-op-id="${zazuOpId}"]`);
    await expect(zazuCard).toHaveAttribute('data-status', 'THERMAL_DRIFT_PREDATION_CRITICAL');
    await expect(zazuCard.locator('.av-thermal-alert-banner')).toContainText(
      'FLIGHT-CORRIDOR THERMAL DRIFT & CROP DEPLETION ALERT'
    );

    // Step 3: Complete remaining acoustic lure & zero-keel-compression protocol steps on Zazu's card
    for (const stepId of ['step-flock-acoustic-lure', 'step-forage-perch-rig', 'step-kevlar-towel-transfer']) {
      const cb = zazuCard.locator(`.av-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(zazuCard).toHaveAttribute('data-status', 'ACOUSTIC_LURE_PERCH_STAGED');
    await expect(zazuCard.locator('.badge-av-status.badge-av-staged')).toContainText(
      'ACOUSTIC_LURE_PERCH_STAGED'
    );

    // Step 4: Issue Avian Aviary Safe Recovery Certificate
    await zazuCard.locator('.av-issue-clearance-btn').click();

    await expect(zazuCard).toHaveAttribute('data-status', 'RECOVERED_AVIARY_SAFE');
    await expect(zazuCard.locator('.badge-av-status.badge-av-recovered')).toContainText(
      'RECOVERED_AVIARY_SAFE'
    );
    await expect(zazuCard.locator('.av-clearance-banner')).toContainText('AVIAN-SAFE-');

    // Step 5: Trigger Cockatiel Courtyard Perch scenario button and issue clearance
    await page.locator('.av-scenario-btn[data-scenario="cockatiel-courtyard-perch"]').click();

    const peachesCard = page
      .locator('#av-ops-list .av-op-card', {
        hasText: 'Peaches (',
      })
      .first();
    await expect(peachesCard).toBeVisible();

    await peachesCard.locator('.av-issue-clearance-btn').click();
    await expect(peachesCard).toHaveAttribute('data-status', 'RECOVERED_AVIARY_SAFE');
    await expect(peachesCard.locator('.av-clearance-banner')).toContainText('AVIAN-SAFE-');

    expect(cspErrors).toEqual([]);
  });
});
