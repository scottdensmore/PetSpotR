import { test, expect, type Locator, type Page } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

/**
 * Clicks #btn-sim-fireworks-bolt and returns the card for the plan that click
 * created, so parallel specs simulating the same case never share a card.
 */
async function simulateFireworksBolt(page: Page): Promise<Locator> {
  const [response] = await Promise.all([
    page.waitForResponse(
      (res) =>
        res.url().endsWith('/api/v1/reunion-decompression/plans') && res.request().method() === 'POST'
    ),
    page.locator('#btn-sim-fireworks-bolt').click(),
  ]);
  const { planId } = (await response.json()) as { planId: string };
  return page.locator(`#decomp-plans-list .decomp-plan-card[data-plan-id="${planId}"]`);
}

test.describe('Post-Reunion Behavioral Reintegration, Cortisol Decompression Tracker & Escape-Recidivism Prevention Plan E2E Journey', () => {
  test('Full 6-step operator journey: seeded cockpit load, starving cat decompression plan generation with refeeding syndrome alert, perimeter fortification & GPS collar verification, 14-day graduation certificate issuance, and fireworks panic-bolt simulation', async ({
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

    // Step 1: Cockpit Load & Seeded Plans
    await page.goto(`${WEB_FRONTEND_URL}/reunion-decompression`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#reunion-decompression-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Post-Reunion Cortisol Decompression & Escape-Recidivism Cockpit'
    );

    const card101 = page.locator(
      '#decomp-plans-list .decomp-plan-card[data-plan-id="decomp-plan-101"]'
    );
    const card102 = page.locator(
      '#decomp-plans-list .decomp-plan-card[data-plan-id="decomp-plan-102"]'
    );
    const card103 = page.locator(
      '#decomp-plans-list .decomp-plan-card[data-plan-id="decomp-plan-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-plan-phase', 'ACUTE_REFUGE_72H');
    await expect(card101.locator('.decomp-refeeding-warning-banner')).toContainText(
      'REFEEDING SYNDROME ALERT'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-plan-phase', 'CORTISOL_TAPER_DAY_4_7');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-plan-phase', 'GRADUATED_STABLE');
    await expect(card103.locator('.decomp-graduation-banner')).toContainText(
      'CERT-DECOMP-DECOMP-PLAN-103'
    );

    // Step 2: Generate 14-Day Decompression Plan for Reunited Starving Cat ("Cleo")
    await page.locator('#rd-pet-id').fill('pet-e2e-decomp-cleo');
    await page.locator('#rd-pet-name').fill('Cleo');
    await page.locator('#rd-species').selectOption('CAT');
    await page.locator('#rd-escape-vector').selectOption('DOOR_DASH_BOLT');
    await page.locator('#rd-days-missing').fill('10');
    await page.locator('#rd-weight-loss-pct').fill('15.0');

    await page.locator('#btn-create-decomp-plan').click();

    const cleoInitialCard = page
      .locator('#decomp-plans-list .decomp-plan-card', {
        hasText: 'Cleo (CAT · 10d missing · 15.0% weight loss)',
      })
      .first();

    await expect(cleoInitialCard).toBeVisible();
    const cleoPlanId = await cleoInitialCard.getAttribute('data-plan-id');
    expect(cleoPlanId).toBeTruthy();

    const cleoCard = page.locator(
      `#decomp-plans-list .decomp-plan-card[data-plan-id="${cleoPlanId}"]`
    );

    await expect(cleoCard).toHaveAttribute('data-plan-phase', 'ACUTE_REFUGE_72H');
    await expect(cleoCard.locator('.badge-decomp-phase.badge-phase-acute')).toContainText(
      'ACUTE_REFUGE_72H'
    );
    await expect(cleoCard.locator('.badge-recidivism-risk.badge-risk-critical')).toContainText(
      'CRITICAL_FLIGHT_RISK'
    );
    await expect(cleoCard.locator('.decomp-refeeding-warning-banner')).toBeVisible();
    await expect(cleoCard.locator('.decomp-refeeding-warning-banner')).toContainText(
      'REFEEDING SYNDROME ALERT'
    );

    // Step 3: Fortify Escape Perimeter & Equip GPS Collar
    await cleoCard.locator('.btn-remediate-perimeter').click();

    await expect(cleoCard).toHaveAttribute('data-plan-phase', 'CORTISOL_TAPER_DAY_4_7');
    await expect(cleoCard.locator('.badge-decomp-phase.badge-phase-taper')).toContainText(
      'CORTISOL_TAPER_DAY_4_7'
    );
    await expect(cleoCard.locator('.badge-cortisol-metric')).toContainText(
      'Collar Readiness: 100/100'
    );
    await expect(cleoCard.locator('.badge-recidivism-risk.badge-risk-elevated')).toContainText(
      'ELEVATED_PERIMETER_RISK'
    );

    // Step 4: Complete 14-Day Decompression Checklist & Graduate
    await cleoCard.locator('.btn-graduate-decomp-plan').click();

    await expect(cleoCard).toHaveAttribute('data-plan-phase', 'GRADUATED_STABLE');
    await expect(cleoCard.locator('.badge-decomp-phase.badge-phase-graduated')).toContainText(
      'GRADUATED_STABLE'
    );
    await expect(cleoCard.locator('.badge-recidivism-risk.badge-risk-fortified')).toContainText(
      'FORTIFIED_LOW_RISK'
    );
    await expect(cleoCard.locator('.decomp-graduation-banner')).toBeVisible();
    await expect(cleoCard.locator('.decomp-graduation-banner')).toContainText('CERT-DECOMP-');

    // Step 5: Simulate Fireworks Panic-Bolt Dog & Graduate
    const boltCard = await simulateFireworksBolt(page);
    await expect(boltCard).toHaveAttribute('data-plan-phase', 'ACUTE_REFUGE_72H');
    await expect(boltCard.locator('.badge-recidivism-risk.badge-risk-critical')).toContainText(
      'CRITICAL_FLIGHT_RISK'
    );

    await boltCard.locator('.btn-graduate-decomp-plan').click();

    await expect(boltCard).toHaveAttribute('data-plan-phase', 'GRADUATED_STABLE');
    await expect(boltCard.locator('.badge-decomp-phase.badge-phase-graduated')).toContainText(
      'GRADUATED_STABLE'
    );
    await expect(boltCard.locator('.badge-recidivism-risk.badge-risk-fortified')).toContainText(
      'FORTIFIED_LOW_RISK'
    );
    await expect(boltCard.locator('.decomp-graduation-banner')).toBeVisible();
    await expect(boltCard.locator('.decomp-graduation-banner')).toContainText('CERT-DECOMP-');

    // Step 6: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
