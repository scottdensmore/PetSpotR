import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Wildfire Smoke, Ashfall, Hazmat Plume & Respiratory Decontamination Triage Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, wildfire smoke & COHb evaluation, surfactant wash & O2 nebulization step completion, hazmat decon clearance certificate issuance, and cyanotoxin bloom simulation', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/hazmat-decon`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#hazmat-decon-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Wildfire Smoke, Ashfall, Hazmat Plume & Respiratory Decontamination Triage Cockpit'
    );

    const card101 = page.locator('#hz-ops-list .hz-op-card[data-op-id="HZ-OP-101"]');
    const card102 = page.locator('#hz-ops-list .hz-op-card[data-op-id="HZ-OP-102"]');
    const card103 = page.locator('#hz-ops-list .hz-op-card[data-op-id="HZ-OP-103"]');

    await expect(card101).toBeVisible();
    await expect(card102).toBeVisible();
    await expect(card103).toBeVisible();

    // Step 2: Evaluate custom wildfire smoke & ashfall operation ("Blaze")
    await page.locator('#hz-pet-id').fill('PET-E2E-BLAZE');
    await page.locator('#hz-pet-name').fill('Blaze');
    await page.locator('#hz-species').selectOption('Feline');
    await page.locator('#hz-weight-kg').fill('4.4');
    await page.locator('#hz-exposure-type').selectOption('WILDFIRE_SMOKE_ASHFALL');
    await page.locator('#hz-aqi-index').fill('465');
    await page.locator('#hz-pm25').fill('425');
    await page.locator('#hz-cohb-pct').fill('18.4');
    await page.locator('#hz-coat-pct').fill('75');
    await page.locator('#hz-resp-rate').fill('74');
    await page.locator('#hz-corridor-zone').fill('DECON-CORRIDOR-HOT-04 (North Cascades Fireline)');
    await page.locator('#hz-tox-callsign').fill('TOX-VET-08 (Keller)');

    await page.locator('#btn-evaluate-hz-op').click();

    const blazeInitialCard = page
      .locator('#hz-ops-list .hz-op-card', {
        hasText: 'Blaze',
      })
      .first();
    await expect(blazeInitialCard).toBeVisible();
    const blazeOpId = await blazeInitialCard.getAttribute('data-op-id');
    expect(blazeOpId).toBeTruthy();

    const blazeCard = page.locator(`#hz-ops-list .hz-op-card[data-op-id="${blazeOpId}"]`);
    await expect(blazeCard).toHaveAttribute('data-status', 'RESPIRATORY_DERMAL_CRITICAL');
    await expect(blazeCard.locator('.hz-smoke-alert-banner')).toContainText(
      'RESPIRATORY & DERMAL TOXIN HAZARD ALERT'
    );

    // Step 3: Complete remaining surfactant wash & O2 nebulization protocol steps on Blaze's card
    for (const stepId of ['step-warm-surfactant-wash', 'step-oxygen-nebulizer', 'step-cohb-binder-panel']) {
      const cb = blazeCard.locator(`.hz-step-checkbox[data-step-id="${stepId}"]`);
      if (!(await cb.isChecked())) {
        await cb.check();
      }
    }

    await expect(blazeCard).toHaveAttribute('data-status', 'DECON_WASH_NEBULIZER_STAGED');
    await expect(blazeCard.locator('.badge-hz-status.badge-hz-staged')).toContainText(
      'DECON_WASH_NEBULIZER_STAGED'
    );

    // Step 4: Issue Hazmat & Respiratory Decon Clearance Certificate
    await blazeCard.locator('.hz-issue-clearance-btn').click();

    await expect(blazeCard).toHaveAttribute('data-status', 'DECONTAMINATED_CLEARED_SAFE');
    await expect(blazeCard.locator('.badge-hz-status.badge-hz-cleared')).toContainText(
      'DECONTAMINATED_CLEARED_SAFE'
    );
    await expect(blazeCard.locator('.hz-clearance-banner')).toContainText('HAZMAT-SAFE-');

    // Step 5: Trigger Cyanotoxin Pond Bloom scenario button and issue clearance
    await page.locator('.hz-scenario-btn[data-scenario="cyanotoxin-pond-bloom"]').click();

    const reedCard = page
      .locator('#hz-ops-list .hz-op-card', {
        hasText: 'Reed (',
      })
      .first();
    await expect(reedCard).toBeVisible();

    await reedCard.locator('.hz-issue-clearance-btn').click();
    await expect(reedCard).toHaveAttribute('data-status', 'DECONTAMINATED_CLEARED_SAFE');
    await expect(reedCard.locator('.hz-clearance-banner')).toContainText('HAZMAT-SAFE-');

    expect(cspErrors).toEqual([]);
  });
});
