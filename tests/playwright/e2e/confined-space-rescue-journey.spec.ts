import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Subterranean, Storm-Drain, Culvert & Confined-Space Acoustic/Fiber-Optic Extraction Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, culvert H2S & inflow surge evaluation, fiber-optic borescope & one-way pipe-gate step completion, subterranean extraction clearance certificate issuance, and chimney flue drop simulation', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/confined-space-rescue`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#confined-space-rescue-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Subterranean, Storm-Drain, Culvert & Confined-Space Acoustic/Fiber-Optic Extraction Cockpit'
    );

    const card101 = page.locator('#cs-ops-list .cs-op-card[data-op-id="CS-OP-101"]');
    const card102 = page.locator('#cs-ops-list .cs-op-card[data-op-id="CS-OP-102"]');
    const card103 = page.locator('#cs-ops-list .cs-op-card[data-op-id="CS-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card102).toBeVisible();
    await expect(card103).toBeVisible();

    // Step 2: Evaluate custom subterranean utility pipe operation ("Echo")
    await page.locator('#cs-pet-id').fill('PET-E2E-ECHO');
    await page.locator('#cs-pet-name').fill('Echo');
    await page.locator('#cs-species').selectOption('Feline');
    await page.locator('#cs-weight-kg').fill('3.9');
    await page.locator('#cs-space-type').selectOption('SUBTERRANEAN_UTILITY_PIPE');
    await page.locator('#cs-pipe-diameter').fill('32');
    await page.locator('#cs-depth-meters').fill('3.7');
    await page.locator('#cs-oxygen-percent').fill('18.3');
    await page.locator('#cs-h2s-ppm').fill('12.5');
    await page.locator('#cs-inflow-risk').fill('76');
    await page.locator('#cs-sector-grid').fill('MH-SOUTH-14D (Georgetown Utility Vault)');
    await page.locator('#cs-tech-callsign').fill('TECH-SUB-09 (Lin)');

    await page.locator('#btn-evaluate-cs-op').click();

    const echoInitialCard = page
      .locator('#cs-ops-list .cs-op-card', {
        hasText: 'Echo',
      })
      .first();
    await expect(echoInitialCard).toBeVisible();
    const echoOpId = await echoInitialCard.getAttribute('data-op-id');
    expect(echoOpId).toBeTruthy();

    const echoCard = page.locator(`#cs-ops-list .cs-op-card[data-op-id="${echoOpId}"]`);
    await expect(echoCard).toHaveAttribute('data-status', 'ATMOSPHERIC_HAZARD_CRITICAL');
    await expect(echoCard.locator('.cs-gas-alert-banner')).toContainText(
      'ATMOSPHERIC & CULVERT SURGE HAZARD ALERT'
    );

    // Step 3: Complete remaining borescope & pipe-gate protocol steps on Echo's card
    for (const stepId of ['step-borescope-loc', 'step-oneway-gate', 'step-haul-extract']) {
      const cb = echoCard.locator(`.cs-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(echoCard).toHaveAttribute('data-status', 'BORESCOPE_PIPE_GATE_RIGGED');
    await expect(echoCard.locator('.badge-cs-status.badge-cs-rigged')).toContainText(
      'BORESCOPE_PIPE_GATE_RIGGED'
    );

    // Step 4: Issue Subterranean Extraction Clearance Certificate
    await echoCard.locator('.cs-issue-clearance-btn').click();

    await expect(echoCard).toHaveAttribute('data-status', 'EXTRACTED_SUBTERRANEAN_SAFE');
    await expect(echoCard.locator('.badge-cs-status.badge-cs-extracted')).toContainText(
      'EXTRACTED_SUBTERRANEAN_SAFE'
    );
    await expect(echoCard.locator('.cs-clearance-banner')).toContainText('CONFINED-SAFE-');

    // Step 5: Trigger Chimney Flue Drop scenario button and issue clearance
    await page.locator('.cs-scenario-btn[data-scenario="chimney-flue-drop"]').click();

    const sootyCard = page
      .locator('#cs-ops-list .cs-op-card', {
        hasText: 'Sooty',
      })
      .first();
    await expect(sootyCard).toBeVisible();

    await sootyCard.locator('.cs-issue-clearance-btn').click();
    await expect(sootyCard).toHaveAttribute('data-status', 'EXTRACTED_SUBTERRANEAN_SAFE');
    await expect(sootyCard.locator('.cs-clearance-banner')).toContainText('CONFINED-SAFE-');

    expect(cspErrors).toEqual([]);
  });
});
