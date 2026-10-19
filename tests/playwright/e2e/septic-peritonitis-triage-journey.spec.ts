import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Septic Peritonitis, Abdominocentesis Blood-Fluid Glucose/Lactate Gradient & Closed-Suction Jackson-Pratt Drain Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Roscoe", staging surgical plan, logging Jackson-Pratt drain check, issuing non-septic JP drain removal clearance, and filtering by NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/septic-peritonitis-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#septic-peritonitis-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Septic Peritonitis, Abdominocentesis Blood-Fluid Glucose/Lactate Gradient & Closed-Suction Jackson-Pratt Drain Cockpit'
    );

    await expect(
      page.locator('#peritonitis-operations-list .peritonitis-op-card', { hasText: 'Baxter (' })
    ).toBeVisible();
    await expect(
      page.locator('#peritonitis-operations-list .peritonitis-op-card', { hasText: 'Nala (' })
    ).toBeVisible();
    await expect(
      page.locator('#peritonitis-operations-list .peritonitis-op-card', { hasText: 'Gumbo (' })
    ).toBeVisible();

    // Step 2: Create new operation "Roscoe"
    await page.locator('#create-incident-id').fill('INC-SP-9504');
    await page.locator('#create-pet-name').fill('Roscoe');
    await page.locator('#create-species').selectOption('CANINE_RUPTURED_PROSTATIC_ABSCESS');
    await page.locator('#create-breed').fill('German Shepherd');
    await page.locator('#create-weight-kg').fill('34.0');
    await page.locator('#create-degenerate-neutrophils').fill('42000');
    await page.locator('#create-blood-glucose').fill('124.0');
    await page.locator('#create-effusion-glucose').fill('46.0');
    await page.locator('#create-blood-lactate').fill('2.2');
    await page.locator('#create-effusion-lactate').fill('6.4');
    await page.locator('#create-intracellular-bacteria').check();
    await page.locator('#btn-create-peritonitis-op').click();

    const roscoeCard = page
      .locator('#peritonitis-operations-list .peritonitis-op-card', { hasText: 'Roscoe (' })
      .first();
    await expect(roscoeCard).toBeVisible();
    await expect(roscoeCard).toHaveAttribute(
      'data-status',
      'SEPTIC_EFFUSION_INTRACELLULAR_BACTERIA_CRITICAL'
    );

    // Step 3: Stage empiric antibiotic, warm 0.9% NaCl lavage & JP drain surgical plan for Roscoe
    await page.locator('#stage-weight-kg').fill('34.0');
    await page.locator('#stage-species').selectOption('CANINE_RUPTURED_PROSTATIC_ABSCESS');
    await page.locator('#stage-blood-glucose').fill('124.0');
    await page.locator('#stage-effusion-glucose').fill('46.0');
    await page.locator('#stage-blood-lactate').fill('2.2');
    await page.locator('#stage-effusion-lactate').fill('6.4');
    await page.locator('#btn-stage-plan').click();

    await expect(roscoeCard).toHaveAttribute(
      'data-status',
      'EMPIRIC_ANTIBIOTIC_LAVAGE_JP_DRAIN_STAGED'
    );
    await expect(page.locator('#plan-glucose-gradient')).toContainText('78.0 mg/dL');
    await expect(page.locator('#plan-lavage-liters')).toContainText('10.20 L');

    // Step 4: Log Jackson-Pratt closed-suction drain check for Roscoe
    await page.locator('#check-effusion-production').fill('1.6');
    await page.locator('#check-serum-albumin').fill('2.6');
    await page.locator('#check-map').fill('86');
    await page.locator('#check-callsign').fill('DR-SURG-88');
    await page
      .locator('#check-notes')
      .fill('Post-omentalization prostatic abscess; JP drain output 1.6 mL/kg/day; bacteria negative.');
    await page.locator('#check-intracellular-bacteria').uncheck();
    await page.locator('#check-degenerate-neutrophils').uncheck();
    await page.locator('#btn-record-drain-check').click();

    await expect(page.locator('#drain-checks-list')).toContainText(
      'Post-omentalization prostatic abscess; JP drain output 1.6 mL/kg/day; bacteria negative.'
    );

    // Step 5: Issue Non-Septic Cytology & JP Drain Removal Clearance for Roscoe
    await page.locator('#clearance-final-output').fill('1.4');
    await page.locator('#clearance-final-albumin').fill('2.7');
    await page.locator('#clearance-vet-license').fill('VET-SURG-77319');
    await page
      .locator('#clearance-summary')
      .fill('Non-septic cytology verified; JP drain output 1.4 mL/kg/day; albumin 2.7 g/dL; JP drains aseptically pulled.');
    await page.locator('#clearance-bacteria-negative').check();
    await page.locator('#clearance-enteral-nutrition').check();
    await page.locator('#clearance-jp-removed').check();
    await page.locator('#btn-issue-drain-clearance').click();

    await expect(roscoeCard).toHaveAttribute(
      'data-status',
      'NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED'
    );
    await expect(page.locator('#peritonitis-clearance-banner')).toBeVisible();
    await expect(page.locator('#peritonitis-clearance-banner')).toContainText('VET-SURG-77319');

    // Step 6: Filter by NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED and verify Roscoe and Gumbo remain visible
    await page
      .locator('#filter-status')
      .selectOption('NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED');
    await expect(
      page.locator('#peritonitis-operations-list .peritonitis-op-card', { hasText: 'Roscoe (' })
    ).toBeVisible();
    await expect(
      page.locator('#peritonitis-operations-list .peritonitis-op-card', { hasText: 'Gumbo (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
