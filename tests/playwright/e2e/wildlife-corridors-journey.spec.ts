import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Predator & Wildlife Conflict Corridor Alerting, Nocturnal Vulnerability Index & Coyote/Raptor Sighting Correlation E2E Journey', () => {
  test('Full 6-step operator journey: seeded cockpit load, nocturnal coyote pack conflict evaluation with small prey alert, ultrasonic/strobe deterrent deployment, safe-haven corridor clearance certificate issuance, and Great Horned Owl raptor ridge simulation', async ({
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

    // Step 1: Cockpit Load & Seeded Reports
    await page.goto(`${WEB_FRONTEND_URL}/wildlife-corridors`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#wildlife-corridors-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Predator & Wildlife Conflict Corridor Alerting Cockpit'
    );

    const card101 = page.locator(
      '#corridor-reports-list .corridor-report-card[data-report-id="corridor-rep-101"]'
    );
    const card102 = page.locator(
      '#corridor-reports-list .corridor-report-card[data-report-id="corridor-rep-102"]'
    );
    const card103 = page.locator(
      '#corridor-reports-list .corridor-report-card[data-report-id="corridor-rep-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-report-phase', 'ACTIVE_PREDATOR_WATCH');
    await expect(card101.locator('.corridor-small-prey-banner')).toContainText(
      'SMALL PREY VULNERABILITY ALERT'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-report-phase', 'HAZING_DETERRENT_DEPLOYED');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-report-phase', 'CORRIDOR_CLEARED');
    await expect(card103.locator('.corridor-clearance-banner')).toContainText(
      'CERT-CORRIDOR-CORRIDOR-REP-103'
    );

    // Step 2: Evaluate Wildlife Corridor Report for Small Cat ("Nori") Near Nocturnal Coyote Pack
    await page.locator('#corridor-pet-id').fill('pet-e2e-corridor-nori');
    await page.locator('#corridor-pet-name').fill('Nori');
    await page.locator('#corridor-pet-species').selectOption('CAT');
    await page.locator('#corridor-pet-weight').fill('3.8');
    await page.locator('#corridor-predator-species').selectOption('COYOTE_PACK');
    await page.locator('#corridor-pack-count').fill('3');
    await page.locator('#corridor-solar-phase').selectOption('NIGHT');
    await page.locator('#corridor-terrain').selectOption('URBAN_RAVINE_GREENBELT');
    await page.locator('#corridor-distance').fill('75');

    await page.locator('#btn-create-corridor-report').click();

    const noriInitialCard = page
      .locator('#corridor-reports-list .corridor-report-card', {
        hasText: 'Nori (CAT · 3.8kg · COYOTE_PACK ×3 @ 75m)',
      })
      .first();

    await expect(noriInitialCard).toBeVisible();
    const noriReportId = await noriInitialCard.getAttribute('data-report-id');
    expect(noriReportId).toBeTruthy();

    const noriCard = page.locator(
      `#corridor-reports-list .corridor-report-card[data-report-id="${noriReportId}"]`
    );

    await expect(noriCard).toHaveAttribute('data-report-phase', 'ACTIVE_PREDATOR_WATCH');
    await expect(noriCard.locator('.badge-corridor-phase.badge-corridor-watch')).toContainText(
      'ACTIVE_PREDATOR_WATCH'
    );
    await expect(noriCard.locator('.badge-predator-threat.badge-threat-extreme')).toContainText(
      'EXTREME_NOCTURNAL_PREDATION_RISK'
    );
    await expect(noriCard.locator('.corridor-small-prey-banner')).toBeVisible();
    await expect(noriCard.locator('.corridor-small-prey-banner')).toContainText(
      'SMALL PREY VULNERABILITY ALERT'
    );

    // Step 3: Deploy Ultrasonic & Amber Strobe Deterrents + Lock Attractants
    await noriCard.locator('.btn-deploy-deterrents').click();

    await expect(noriCard).toHaveAttribute('data-report-phase', 'HAZING_DETERRENT_DEPLOYED');
    await expect(noriCard.locator('.badge-corridor-phase.badge-corridor-hazing')).toContainText(
      'HAZING_DETERRENT_DEPLOYED'
    );
    await expect(noriCard.locator('.badge-corridor-metric')).toContainText(
      'Deterrent Readiness: 100/100'
    );
    await expect(noriCard.locator('.badge-predator-threat.badge-threat-elevated')).toContainText(
      'ELEVATED_CREPUSCULAR_CORRIDOR'
    );

    // Step 4: Complete All Corridor Mitigations & Issue Clearance Certificate
    await noriCard.locator('.btn-clear-corridor').click();

    await expect(noriCard).toHaveAttribute('data-report-phase', 'CORRIDOR_CLEARED');
    await expect(noriCard.locator('.badge-corridor-phase.badge-corridor-cleared')).toContainText(
      'CORRIDOR_CLEARED'
    );
    await expect(noriCard.locator('.badge-predator-threat.badge-threat-safe')).toContainText(
      'MITIGATED_SAFE_ZONE'
    );
    await expect(noriCard.locator('.corridor-clearance-banner')).toBeVisible();
    await expect(noriCard.locator('.corridor-clearance-banner')).toContainText('CERT-CORRIDOR-');

    // Step 5: Simulate Crepuscular Great Horned Owl on Canyon Ridge & Clear Corridor
    await page.locator('#btn-sim-raptor-ridge').click();

    const pendingRaptorCard = page
      .locator('#corridor-reports-list .corridor-report-card[data-report-phase="ACTIVE_PREDATOR_WATCH"]', {
        hasText: 'GREAT_HORNED_OWL',
      })
      .first();

    await expect(pendingRaptorCard).toBeVisible();
    const raptorReportId = await pendingRaptorCard.getAttribute('data-report-id');
    expect(raptorReportId).toBeTruthy();

    const raptorCard = page.locator(
      `#corridor-reports-list .corridor-report-card[data-report-id="${raptorReportId}"]`
    );
    await expect(raptorCard).toHaveAttribute('data-report-phase', 'ACTIVE_PREDATOR_WATCH');
    await expect(raptorCard.locator('.badge-predator-threat.badge-threat-extreme')).toContainText(
      'EXTREME_NOCTURNAL_PREDATION_RISK'
    );

    await raptorCard.locator('.btn-clear-corridor').click();

    await expect(raptorCard).toHaveAttribute('data-report-phase', 'CORRIDOR_CLEARED');
    await expect(raptorCard.locator('.badge-corridor-phase.badge-corridor-cleared')).toContainText(
      'CORRIDOR_CLEARED'
    );
    await expect(raptorCard.locator('.badge-predator-threat.badge-threat-safe')).toContainText(
      'MITIGATED_SAFE_ZONE'
    );
    await expect(raptorCard.locator('.corridor-clearance-banner')).toBeVisible();
    await expect(raptorCard.locator('.corridor-clearance-banner')).toContainText('CERT-CORRIDOR-');

    // Step 6: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
