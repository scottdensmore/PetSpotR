import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Seismic Rubble, USAR Structural Collapse & Delsar Acoustic Void-Space Extraction Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, cantilever beam overhang & crush-syndrome evaluation, Delsar geophone & box-cribbing shoring step completion, USAR extraction clearance certificate issuance, and V-shape masonry pocket simulation', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/usar-structural-collapse`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#usar-structural-collapse-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Seismic Rubble, Urban Search & Rescue (USAR) Structural Collapse & Delsar Acoustic Void-Space Extraction Cockpit'
    );

    const card101 = page.locator('#usar-ops-list .usar-op-card[data-op-id="USAR-OP-101"]');
    const card102 = page.locator('#usar-ops-list .usar-op-card[data-op-id="USAR-OP-102"]');
    const card103 = page.locator('#usar-ops-list .usar-op-card[data-op-id="USAR-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card102).toBeVisible();
    await expect(card103).toBeVisible();

    // Step 2: Evaluate custom cantilever beam overhang operation ("Samson")
    await page.locator('#usar-pet-id').fill('PET-E2E-SAMSON');
    await page.locator('#usar-pet-name').fill('Samson');
    await page.locator('#usar-species').selectOption('Canine');
    await page.locator('#usar-weight-kg').fill('29.5');
    await page.locator('#usar-void-type').selectOption('CANTILEVER_BEAM_OVERHANG');
    await page.locator('#usar-rubble-overburden').fill('2.4');
    await page.locator('#usar-aftershock-pga').fill('0.31');
    await page.locator('#usar-silica-dust').fill('7.4');
    await page.locator('#usar-hours-entrapped').fill('16.0');
    await page.locator('#usar-stability-idx').fill('26');
    await page.locator('#usar-structure-grid').fill('SECTOR-DELTA-07 (Cantilevered Overpass Pier 3)');
    await page.locator('#usar-spec-callsign').fill('USAR-TF1-SPEC-05 (Vance)');

    await page.locator('#btn-evaluate-usar-op').click();

    const samsonInitialCard = page
      .locator('#usar-ops-list .usar-op-card', {
        hasText: 'Samson (',
      })
      .first();
    await expect(samsonInitialCard).toBeVisible();
    const samsonOpId = await samsonInitialCard.getAttribute('data-op-id');
    expect(samsonOpId).toBeTruthy();

    const samsonCard = page.locator(`#usar-ops-list .usar-op-card[data-op-id="${samsonOpId}"]`);
    await expect(samsonCard).toHaveAttribute('data-status', 'SECONDARY_COLLAPSE_CRUSH_CRITICAL');
    await expect(samsonCard.locator('.usar-collapse-alert-banner')).toContainText(
      'SECONDARY COLLAPSE & CRUSH-SYNDROME HYPERKALEMIA ALERT'
    );

    // Step 3: Complete remaining box-cribbing & crush-syndrome protocol steps on Samson's card
    for (const stepId of [
      'step-box-cribbing-shoring',
      'step-rotary-core-breach',
      'step-crush-rhabdo-decon',
    ]) {
      const cb = samsonCard.locator(`.usar-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(samsonCard).toHaveAttribute('data-status', 'SHORING_CRIBBING_DELSAR_STAGED');
    await expect(samsonCard.locator('.badge-usar-status.badge-usar-staged')).toContainText(
      'SHORING_CRIBBING_DELSAR_STAGED'
    );

    // Step 4: Issue USAR Void Extraction & Silica Decon Clearance Certificate
    await samsonCard.locator('.usar-issue-clearance-btn').click();

    await expect(samsonCard).toHaveAttribute('data-status', 'EXTRACTED_USAR_DECON_SAFE');
    await expect(samsonCard.locator('.badge-usar-status.badge-usar-safe')).toContainText(
      'EXTRACTED_USAR_DECON_SAFE'
    );
    await expect(samsonCard.locator('.usar-clearance-banner')).toContainText('USAR-SAFE-');

    // Step 5: Trigger V-Shape Masonry Pocket scenario button and issue clearance
    await page.locator('.usar-scenario-btn[data-scenario="vshape-masonry-pocket"]').click();

    const cleoCard = page
      .locator('#usar-ops-list .usar-op-card', {
        hasText: 'Cleo (',
      })
      .first();
    await expect(cleoCard).toBeVisible();

    await cleoCard.locator('.usar-issue-clearance-btn').click();
    await expect(cleoCard).toHaveAttribute('data-status', 'EXTRACTED_USAR_DECON_SAFE');
    await expect(cleoCard.locator('.usar-clearance-banner')).toContainText('USAR-SAFE-');

    expect(cspErrors).toEqual([]);
  });
});
