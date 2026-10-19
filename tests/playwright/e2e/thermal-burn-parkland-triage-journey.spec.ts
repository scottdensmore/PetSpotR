import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Smoke Inhalation, Thermal Burn Classification (Superficial/Partial/Full-Thickness TBSA%), Parkland Crystalloid Resuscitation Formula, Silver Sulfadiazine & Escharotomy Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Thermal Burn patient "Phoenix", staging Parkland 8h/16h + SSD + Escharotomy protocol, recording burn fluid & wound check, clearing for granulation bed discharge, and filtering by GRANULATION_BED_NORMOALBUMINEMIC_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/thermal-burn-parkland-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#thermal-burn-parkland-cockpit')).toBeVisible();
    await expect(page.locator('#burn-alert-banner')).toBeVisible();

    await expect(
      page.locator('#burn-operations-grid .burn-op-card', { hasText: 'Ember (' })
    ).toBeVisible();
    await expect(
      page.locator('#burn-operations-grid .burn-op-card', { hasText: 'Cinder (' })
    ).toBeVisible();
    await expect(
      page.locator('#burn-operations-grid .burn-op-card', { hasText: 'Blaze (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Thermal Burn patient "Phoenix"
    await page.locator('#create-case-id-input').fill('CASE-BURN-104');
    await page.locator('#create-pet-name-input').fill('Phoenix');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Belgian Malinois');
    await page.locator('#create-weight-input').fill('25.0');
    await page
      .locator('#create-depth-class-input')
      .selectOption('FULL_THICKNESS_CIRCUMFERENTIAL_ESCHAR');
    await page.locator('#create-tbsa-input').fill('28.0');
    await page.locator('#create-fluid-rate-input').fill('175.0');
    await page.locator('#create-uop-input').fill('0.9');
    await page.locator('#create-albumin-input').fill('1.8');
    await page.locator('#create-pcv-input').fill('56.0');
    await page.locator('#create-eschar-checkbox').check();
    await page.locator('#create-airway-checkbox').check();
    await page.locator('#create-burn-op-btn').click();

    const phoenixCard = page
      .locator('#burn-operations-grid .burn-op-card', { hasText: 'Phoenix (' })
      .first();
    await expect(phoenixCard).toBeVisible();
    await expect(phoenixCard).toHaveAttribute(
      'data-status',
      'ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE'
    );
    await expect(page.locator('#active-op-title')).toContainText('Phoenix');
    await expect(page.locator('#active-tbsa-percent')).toContainText('28.0% TBSA');
    await expect(page.locator('#active-urine-output')).toContainText('0.9 mL/kg/hr');

    // Step 4: Stage Parkland 8h/16h, Silver Sulfadiazine & Escharotomy protocol for Phoenix
    await page.locator('#stage-parkland-btn').click();
    await expect(phoenixCard).toHaveAttribute(
      'data-status',
      'PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED'
    );

    // Step 5: Record serial burn fluid & wound check for Phoenix
    await page.locator('#check-tbsa-input').fill('14.0');
    await page.locator('#check-fluid-rate-input').fill('80.0');
    await page.locator('#check-uop-input').fill('1.8');
    await page.locator('#check-albumin-input').fill('2.6');
    await page.locator('#check-pcv-input').fill('42.0');
    await page.locator('#check-recorded-by-input').fill('Dr. Marcus Thorne, DACVS / Burn Reconstruction');
    await page
      .locator('#check-note-input')
      .fill('Post-escharotomy & Parkland transition: Phoenix chest wall compliance restored, Urine Output 1.8 mL/kg/hr, Albumin 2.6 g/dL, residual TBSA 14.0%.');
    await page.locator('#record-burn-check-btn').click();

    await expect(page.locator('#active-tbsa-percent')).toContainText('14.0% TBSA');
    await expect(page.locator('#active-urine-output')).toContainText('1.8 mL/kg/hr');
    await expect(page.locator('#burn-checks-list')).toContainText(
      'Post-escharotomy & Parkland transition: Phoenix chest wall compliance restored'
    );

    // Step 6: Clear Phoenix for Granulation Bed & Normoalbuminemic Discharge
    await page.locator('#clear-discharged-by-input').fill('Dr. Marcus Thorne, DACVS / Burn Reconstruction');
    await page
      .locator('#clear-summary-input')
      .fill('Phoenix healthy vascularized granulation bed established across residual 14.0% TBSA, Urine Output 1.8 mL/kg/hr, Serum Albumin 2.6 g/dL, cleared for step-down.');
    await page.locator('#clear-burn-btn').click();

    await expect(phoenixCard).toHaveAttribute(
      'data-status',
      'GRANULATION_BED_NORMOALBUMINEMIC_CLEARED'
    );
    await expect(page.locator('#burn-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Phoenix');

    // Step 7: Filter by GRANULATION_BED_NORMOALBUMINEMIC_CLEARED
    await page
      .locator('#burn-status-filter')
      .selectOption('GRANULATION_BED_NORMOALBUMINEMIC_CLEARED');
    await expect(
      page.locator('#burn-operations-grid .burn-op-card', { hasText: 'Phoenix (' })
    ).toBeVisible();
    await expect(
      page.locator('#burn-operations-grid .burn-op-card', { hasText: 'Blaze (' })
    ).toBeVisible();

    await page.locator('#burn-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
