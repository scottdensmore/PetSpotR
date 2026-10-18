import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Ectothermic Reptile, Chelonian & Amphibian Thermal-Dormancy Brisk-Basking & Micro-Habitat Recovery Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, amphibian axolotl cold-stun & desiccation evaluation, 5.5mm borescope & ceramic IR basking lure step completion, ectotherm thermal-gradient clearance certificate issuance, and dragon sunroom crevice simulation', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/reptile-ectotherm-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#reptile-ectotherm-rescue-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Ectothermic Reptile, Chelonian & Amphibian Thermal-Dormancy Brisk-Basking & Micro-Habitat Recovery Cockpit'
    );

    const card101 = page.locator('#re-ops-list .re-op-card[data-op-id="RE-OP-101"]');
    const card102 = page.locator('#re-ops-list .re-op-card[data-op-id="RE-OP-102"]');
    const card103 = page.locator('#re-ops-list .re-op-card[data-op-id="RE-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card102).toBeVisible();
    await expect(card103).toBeVisible();

    // Step 2: Evaluate custom amphibian salamander / axolotl operation ("Xochitl")
    await page.locator('#re-pet-id').fill('PET-E2E-XOCHITL');
    await page.locator('#re-pet-name').fill('Xochitl');
    await page.locator('#re-species').selectOption('Amphibian');
    await page.locator('#re-weight-kg').fill('0.35');
    await page.locator('#re-taxon-type').selectOption('AMPHIBIAN_SALAMANDER_AXOLOTL');
    await page.locator('#re-burrow-depth').fill('45');
    await page.locator('#re-substrate-temp').fill('11.5');
    await page.locator('#re-optimum-temp').fill('18.5');
    await page.locator('#re-humidity-pct').fill('32');
    await page.locator('#re-hours-dormant').fill('38');
    await page.locator('#re-habitat-grid').fill('DRAIN-SUMP-05D (Sub-Basement French Drain Sump)');
    await page.locator('#re-herp-callsign').fill('HERP-TECH-04 (Morales)');

    await page.locator('#btn-evaluate-re-op').click();

    const xochitlInitialCard = page
      .locator('#re-ops-list .re-op-card', {
        hasText: 'Xochitl (',
      })
      .first();
    await expect(xochitlInitialCard).toBeVisible();
    const xochitlOpId = await xochitlInitialCard.getAttribute('data-op-id');
    expect(xochitlOpId).toBeTruthy();

    const xochitlCard = page.locator(`#re-ops-list .re-op-card[data-op-id="${xochitlOpId}"]`);
    await expect(xochitlCard).toHaveAttribute('data-status', 'COLD_TORPOR_BRUMATION_CRITICAL');
    await expect(xochitlCard.locator('.re-torpor-alert-banner')).toContainText(
      'ECTOTHERMIC COLD-STUN TORPOR & DESICCATION ALERT'
    );

    // Step 3: Complete remaining ceramic IR lure & coelomic rewarming protocol steps on Xochitl's card
    for (const stepId of [
      'step-ceramic-ir-basking-lure',
      'step-humidity-hydration-soak',
      'step-gradual-coelomic-rewarm',
    ]) {
      const cb = xochitlCard.locator(`.re-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(xochitlCard).toHaveAttribute('data-status', 'BASKING_LURE_CREVICE_STAGED');
    await expect(xochitlCard.locator('.badge-re-status.badge-re-staged')).toContainText(
      'BASKING_LURE_CREVICE_STAGED'
    );

    // Step 4: Issue Ectotherm Thermal-Gradient Recovery Clearance Certificate
    await xochitlCard.locator('.re-issue-clearance-btn').click();

    await expect(xochitlCard).toHaveAttribute('data-status', 'RECOVERED_THERMAL_GRADIENT_SAFE');
    await expect(xochitlCard.locator('.badge-re-status.badge-re-safe')).toContainText(
      'RECOVERED_THERMAL_GRADIENT_SAFE'
    );
    await expect(xochitlCard.locator('.re-clearance-banner')).toContainText('ECTOTHERM-SAFE-');

    // Step 5: Trigger Dragon Sunroom Crevice scenario button and issue clearance
    await page.locator('.re-scenario-btn[data-scenario="dragon-sunroom-crevice"]').click();

    const solsticeCard = page
      .locator('#re-ops-list .re-op-card', {
        hasText: 'Solstice (',
      })
      .first();
    await expect(solsticeCard).toBeVisible();

    await solsticeCard.locator('.re-issue-clearance-btn').click();
    await expect(solsticeCard).toHaveAttribute('data-status', 'RECOVERED_THERMAL_GRADIENT_SAFE');
    await expect(solsticeCard.locator('.re-clearance-banner')).toContainText('ECTOTHERM-SAFE-');

    expect(cspErrors).toEqual([]);
  });
});
