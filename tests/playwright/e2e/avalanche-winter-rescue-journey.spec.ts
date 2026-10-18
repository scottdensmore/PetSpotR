import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Cold-Weather Avalanche, Deep-Snow Drift & Thermal Hypothermia Rewarming SAR Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, glacial crevasse & hypothermia evaluation, 457 kHz beacon probe & V-trench step completion, winter normothermic clearance certificate issuance, and blizzard drift shelter simulation', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/avalanche-winter-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#avalanche-winter-rescue-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Cold-Weather Avalanche, Deep-Snow Drift & Thermal Hypothermia Rewarming SAR Cockpit'
    );

    const card101 = page.locator('#aw-ops-list .aw-op-card[data-op-id="AW-OP-101"]');
    const card102 = page.locator('#aw-ops-list .aw-op-card[data-op-id="AW-OP-102"]');
    const card103 = page.locator('#aw-ops-list .aw-op-card[data-op-id="AW-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card102).toBeVisible();
    await expect(card103).toBeVisible();

    // Step 2: Evaluate custom glacial crevasse ledge operation ("Denali")
    await page.locator('#aw-pet-id').fill('PET-E2E-DENALI');
    await page.locator('#aw-pet-name').fill('Denali');
    await page.locator('#aw-species').selectOption('Canine');
    await page.locator('#aw-weight-kg').fill('26.5');
    await page.locator('#aw-terrain-type').selectOption('GLACIAL_CREVASSE_LEDGE');
    await page.locator('#aw-burial-depth').fill('75');
    await page.locator('#aw-wind-chill').fill('-21.5');
    await page.locator('#aw-core-temp').fill('33.9');
    await page.locator('#aw-minutes-buried').fill('21');
    await page.locator('#aw-danger-scale').fill('4');
    await page.locator('#aw-sector-grid').fill('CREVASSE-NISQUALLY-08C (Rainier Muir Snowfield)');
    await page.locator('#aw-patrol-callsign').fill('PATROL-ALPINE-07 (Voss)');

    await page.locator('#btn-evaluate-aw-op').click();

    const denaliInitialCard = page
      .locator('#aw-ops-list .aw-op-card', {
        hasText: 'Denali',
      })
      .first();
    await expect(denaliInitialCard).toBeVisible();
    const denaliOpId = await denaliInitialCard.getAttribute('data-op-id');
    expect(denaliOpId).toBeTruthy();

    const denaliCard = page.locator(`#aw-ops-list .aw-op-card[data-op-id="${denaliOpId}"]`);
    await expect(denaliCard).toHaveAttribute('data-status', 'BURIAL_HYPOTHERMIA_CRITICAL');
    await expect(denaliCard.locator('.aw-burial-alert-banner')).toContainText(
      'SNOWPACK ASPHYXIA & HYPOTHERMIC AFTERDROP ALERT'
    );

    // Step 3: Complete remaining V-trench & core rewarming protocol steps on Denali's card
    for (const stepId of [
      'step-v-conveyor-trench',
      'step-horizontal-afterdrop-lift',
      'step-active-core-rewarm',
    ]) {
      const cb = denaliCard.locator(`.aw-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(denaliCard).toHaveAttribute('data-status', 'PROBE_TRENCH_REWARMING_STAGED');
    await expect(denaliCard.locator('.badge-aw-status.badge-aw-staged')).toContainText(
      'PROBE_TRENCH_REWARMING_STAGED'
    );

    // Step 4: Issue Winter Normothermic Extraction Clearance Certificate
    await denaliCard.locator('.aw-issue-clearance-btn').click();

    await expect(denaliCard).toHaveAttribute('data-status', 'EXTRACTED_NORMOTHERMIC_SAFE');
    await expect(denaliCard.locator('.badge-aw-status.badge-aw-safe')).toContainText(
      'EXTRACTED_NORMOTHERMIC_SAFE'
    );
    await expect(denaliCard.locator('.aw-clearance-banner')).toContainText('WINTER-SAFE-');

    // Step 5: Trigger Blizzard Drift Shelter scenario button and issue clearance
    await page.locator('.aw-scenario-btn[data-scenario="blizzard-drift-shelter"]').click();

    const lumiCard = page
      .locator('#aw-ops-list .aw-op-card', {
        hasText: 'Lumi (',
      })
      .first();
    await expect(lumiCard).toBeVisible();

    await lumiCard.locator('.aw-issue-clearance-btn').click();
    await expect(lumiCard).toHaveAttribute('data-status', 'EXTRACTED_NORMOTHERMIC_SAFE');
    await expect(lumiCard.locator('.aw-clearance-banner')).toContainText('WINTER-SAFE-');

    expect(cspErrors).toEqual([]);
  });
});
