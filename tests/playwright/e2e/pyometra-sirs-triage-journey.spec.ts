import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Pyometra (Open vs. Closed Cervix), POCUS Fluid-Filled Uterine Horns, E. coli Endotoxemia/SIRS, Stabilization & Ovariohysterectomy (OHE) Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Pyometra patient "Sadie", staging IV Crystalloid + Empiric Enrofloxacin/Ampicillin-Sulbactam + Emergent OHE protocol, recording perioperative hemodynamic & lactate check, clearing for post-op OHE discharge, and filtering by POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/pyometra-sirs-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#pyometra-sirs-cockpit')).toBeVisible();
    await expect(page.locator('#pyometra-alert-banner')).toBeVisible();

    await expect(
      page.locator('#pyometra-operations-grid .pyometra-op-card', { hasText: 'Bella (' })
    ).toBeVisible();
    await expect(
      page.locator('#pyometra-operations-grid .pyometra-op-card', { hasText: 'Duchess (' })
    ).toBeVisible();
    await expect(
      page.locator('#pyometra-operations-grid .pyometra-op-card', { hasText: 'Daisy (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Pyometra patient "Sadie"
    await page.locator('#create-case-id-input').fill('CASE-PYO-104');
    await page.locator('#create-pet-name-input').fill('Sadie');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Bernese Mountain Dog');
    await page.locator('#create-weight-input').fill('38.2');
    await page
      .locator('#create-cervix-type-input')
      .selectOption('CLOSED_CERVIX_RUPTURE_RISK');
    await page.locator('#create-horn-diameter-input').fill('6.1');
    await page.locator('#create-lactate-input').fill('5.1');
    await page.locator('#create-wbc-input').fill('36.4');
    await page.locator('#create-bands-input').fill('20.0');
    await page.locator('#create-map-input').fill('56');
    await page.locator('#create-pupd-checkbox').check();
    await page.locator('#create-pyometra-op-btn').click();

    const sadieCard = page
      .locator('#pyometra-operations-grid .pyometra-op-card', { hasText: 'Sadie (' })
      .first();
    await expect(sadieCard).toBeVisible();
    await expect(sadieCard).toHaveAttribute('data-status', 'CLOSED_CERVIX_ENDOTOXIC_SIRS_TRIAGE');
    await expect(page.locator('#active-op-title')).toContainText('Sadie');
    await expect(page.locator('#active-uterine-diameter')).toContainText('6.1 cm');
    await expect(page.locator('#active-blood-lactate')).toContainText('5.1 mmol/L');

    // Step 4: Stage IV Crystalloid Resuscitation, Empiric Abx & Emergent OHE protocol for Sadie
    await page.locator('#stage-ohe-btn').click();
    await expect(sadieCard).toHaveAttribute(
      'data-status',
      'RESUSCITATED_EMPIRIC_ABX_OHE_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'RESUSCITATED_EMPIRIC_ABX_OHE_STAGED'
    );

    // Step 5: Record serial perioperative hemodynamic & lactate check for Sadie
    await page.locator('#check-horn-diameter-input').fill('0.0');
    await page.locator('#check-lactate-input').fill('1.3');
    await page.locator('#check-wbc-input').fill('13.5');
    await page.locator('#check-bands-input').fill('1.5');
    await page.locator('#check-map-input').fill('86');
    await page.locator('#check-recorded-by-input').fill('Dr. Hannah Lin, DACVS');
    await page
      .locator('#check-note-input')
      .fill('Post-OHE Sadie recovery: bilateral 6.1 cm uterine horns excised intact (0.0 cm), lactate cleared to 1.3 mmol/L, bands 1.5%, MAP 86 mmHg.');
    await page.locator('#record-hemodynamic-check-btn').click();

    await expect(page.locator('#active-uterine-diameter')).toContainText('0.0 cm');
    await expect(page.locator('#active-blood-lactate')).toContainText('1.3 mmol/L');
    await expect(page.locator('#hemodynamic-checks-list')).toContainText(
      'Post-OHE Sadie recovery: bilateral 6.1 cm uterine horns excised intact'
    );

    // Step 6: Clear Sadie for Post-Op OHE Discharge
    await page.locator('#clear-discharged-by-input').fill('Dr. Hannah Lin, DACVS');
    await page
      .locator('#clear-summary-input')
      .fill('Sadie post-op OHE normotensive (MAP 86 mmHg), afebrile, lactate 1.3 mmol/L, resolved E. coli ADH resistance PU/PD, discharged on oral Amoxicillin-Clavulanate + Enrofloxacin.');
    await page.locator('#clear-pyometra-btn').click();

    await expect(sadieCard).toHaveAttribute(
      'data-status',
      'POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED'
    );
    await expect(page.locator('#pyometra-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Sadie');

    // Step 7: Filter by POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED
    await page
      .locator('#pyometra-status-filter')
      .selectOption('POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED');
    await expect(
      page.locator('#pyometra-operations-grid .pyometra-op-card', { hasText: 'Sadie (' })
    ).toBeVisible();
    await expect(
      page.locator('#pyometra-operations-grid .pyometra-op-card', { hasText: 'Daisy (' })
    ).toBeVisible();

    await page.locator('#pyometra-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
