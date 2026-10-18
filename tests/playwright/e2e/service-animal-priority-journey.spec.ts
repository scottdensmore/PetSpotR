import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Working Dog & Service Animal ADA Priority Recovery Cockpit E2E Journey', () => {
  test('Full 6-step operator journey: seeded cockpit load, T1D diabetic alert dog evaluation with medical standby banner, EMS standby & non-confrontational vest intercept staging, protected ADA team restoration, and blind guide dog simulation', async ({
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

    // Step 1: Cockpit Load & Seeded Cases
    await page.goto(`${WEB_FRONTEND_URL}/service-animal-priority`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#service-animal-priority-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Working Dog & Service Animal ADA Priority Recovery Cockpit'
    );

    const card101 = page.locator(
      '#sa-cases-list .sa-case-card[data-case-id="sa-case-101"]'
    );
    const card102 = page.locator(
      '#sa-cases-list .sa-case-card[data-case-id="sa-case-102"]'
    );
    const card103 = page.locator(
      '#sa-cases-list .sa-case-card[data-case-id="sa-case-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-recovery-status', 'HANDLER_MEDICAL_ESCALATED');
    await expect(card101.locator('.sa-medical-standby-banner')).toContainText(
      'ADA MEDICAL-ALERT HANDLER STANDBY & SNAG HAZARD'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-recovery-status', 'TACTICAL_VEST_INTERCEPT_STAGED');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute(
      'data-recovery-status',
      'REUNITED_ADA_TEAM_RESTORED'
    );
    await expect(card103.locator('.sa-restoration-banner')).toContainText(
      'ADA SERVICE ANIMAL TEAM RESTORED: ADA-RESTORED-SA-CASE-103'
    );

    // Step 2: Evaluate Service Animal Priority Case for T1D Diabetic Alert Dog ("Atlas")
    await page.locator('#sa-pet-id').fill('pet-e2e-sa-atlas');
    await page.locator('#sa-pet-name').fill('Atlas');
    await page.locator('#sa-pet-species').selectOption('DOG');
    await page.locator('#sa-handler-name').fill('Devon Brooks (T1D Handler)');
    await page.locator('#sa-task-type').selectOption('DIABETIC_HYPOGLYCEMIA_ALERT');
    await page.locator('#sa-vest-state').selectOption('ADA_CAPE_ENTANGLED_HAZARD');
    await page.locator('#sa-hours-separated').fill('7');

    await page.locator('#btn-create-sa-case').click();

    const atlasInitialCard = page
      .locator('#sa-cases-list .sa-case-card', {
        hasText: 'Atlas (DOG · DIABETIC_HYPOGLYCEMIA_ALERT · Handler: Devon Brooks (T1D Handler))',
      })
      .first();

    await expect(atlasInitialCard).toBeVisible();
    const atlasId = await atlasInitialCard.getAttribute('data-case-id');
    expect(atlasId).toBeTruthy();

    const atlasCard = page.locator(
      `#sa-cases-list .sa-case-card[data-case-id="${atlasId}"]`
    );

    await expect(atlasCard).toHaveAttribute('data-recovery-status', 'HANDLER_MEDICAL_ESCALATED');
    await expect(atlasCard.locator('.badge-sa-status.badge-sa-escalated')).toContainText(
      'HANDLER_MEDICAL_ESCALATED'
    );
    await expect(
      atlasCard.locator('.badge-sa-urgency.badge-urgency-critical')
    ).toContainText('LIFE_CRITICAL_MEDICAL_DEPENDENCY');
    await expect(atlasCard.locator('.sa-medical-standby-banner')).toBeVisible();
    await expect(atlasCard.locator('.sa-medical-standby-banner')).toContainText(
      'ADA MEDICAL-ALERT HANDLER STANDBY & SNAG HAZARD'
    );

    // Step 3: Stage EMS Handler Standby & Non-Confrontational Vest Intercept
    await atlasCard.locator('.btn-stage-sa-intercept').click();

    await expect(atlasCard).toHaveAttribute('data-recovery-status', 'TACTICAL_VEST_INTERCEPT_STAGED');
    await expect(
      atlasCard.locator('.badge-sa-status.badge-sa-staged')
    ).toContainText('TACTICAL_VEST_INTERCEPT_STAGED');
    await expect(atlasCard.locator('.badge-sa-metric')).toContainText(
      'Intercept Readiness: 100/100'
    );
    await expect(
      atlasCard.locator('.badge-sa-urgency.badge-urgency-mobility')
    ).toContainText('HIGH_MOBILITY_GUIDE_DEPENDENCY');

    // Step 4: Restore Protected ADA Service Animal Team
    await atlasCard.locator('.btn-restore-sa-team').click();

    await expect(atlasCard).toHaveAttribute(
      'data-recovery-status',
      'REUNITED_ADA_TEAM_RESTORED'
    );
    await expect(
      atlasCard.locator('.badge-sa-status.badge-sa-restored')
    ).toContainText('REUNITED_ADA_TEAM_RESTORED');
    await expect(atlasCard.locator('.badge-sa-urgency.badge-urgency-stabilized')).toContainText(
      'HANDLER_TEAM_STABILIZED'
    );
    await expect(atlasCard.locator('.sa-restoration-banner')).toBeVisible();
    await expect(atlasCard.locator('.sa-restoration-banner')).toContainText(
      'ADA SERVICE ANIMAL TEAM RESTORED: ADA-RESTORED-'
    );

    // Step 5: Simulate Blind Guide Dog in High-Vis Working Harness & Restore ADA Team
    await page.locator('#btn-sim-guide-dog-harness').click();

    const escalatedBeaconCard = page
      .locator(
        '#sa-cases-list .sa-case-card[data-recovery-status="HANDLER_MEDICAL_ESCALATED"]',
        {
          hasText: 'Beacon',
        }
      )
      .first();

    await expect(escalatedBeaconCard).toBeVisible();
    const beaconId = await escalatedBeaconCard.getAttribute('data-case-id');
    expect(beaconId).toBeTruthy();

    const beaconCard = page.locator(
      `#sa-cases-list .sa-case-card[data-case-id="${beaconId}"]`
    );
    await expect(beaconCard).toHaveAttribute('data-recovery-status', 'HANDLER_MEDICAL_ESCALATED');
    await expect(
      beaconCard.locator('.badge-sa-urgency.badge-urgency-critical')
    ).toContainText('LIFE_CRITICAL_MEDICAL_DEPENDENCY');

    await beaconCard.locator('.btn-restore-sa-team').click();

    await expect(beaconCard).toHaveAttribute(
      'data-recovery-status',
      'REUNITED_ADA_TEAM_RESTORED'
    );
    await expect(
      beaconCard.locator('.badge-sa-status.badge-sa-restored')
    ).toContainText('REUNITED_ADA_TEAM_RESTORED');
    await expect(beaconCard.locator('.badge-sa-urgency.badge-urgency-stabilized')).toContainText(
      'HANDLER_TEAM_STABILIZED'
    );
    await expect(beaconCard.locator('.sa-restoration-banner')).toBeVisible();
    await expect(beaconCard.locator('.sa-restoration-banner')).toContainText(
      'ADA SERVICE ANIMAL TEAM RESTORED: ADA-RESTORED-'
    );

    // Step 6: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
