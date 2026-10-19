import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Bite Wound, Vehicular Polytrauma, Degloving/Shearing Injury, Open Fracture Gustilo-Anderson Classification, Wet-to-Dry/NPWT Vacuum-Assisted Closure & Damage-Control Orthopedics Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Polytrauma/Open Fracture patient "Bandit", staging 7-8 PSI lavage/NPWT/Robert Jones & ESF protocol, recording wound check, clearing for stable fixation discharge, and filtering by GRANULATION_FIXATION_STABLE_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/polytrauma-orthopedic-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#polytrauma-orthopedic-cockpit')).toBeVisible();
    await expect(page.locator('#ortho-alert-banner')).toBeVisible();

    await expect(
      page.locator('#ortho-operations-grid .ortho-op-card', { hasText: 'Ranger (' })
    ).toBeVisible();
    await expect(
      page.locator('#ortho-operations-grid .ortho-op-card', { hasText: 'Peanut (' })
    ).toBeVisible();
    await expect(
      page.locator('#ortho-operations-grid .ortho-op-card', { hasText: 'Toby (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Polytrauma / Open Fracture patient "Bandit"
    await page.locator('#create-case-id-input').fill('CASE-ORTHO-104');
    await page.locator('#create-pet-name-input').fill('Bandit');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Australian Shepherd');
    await page.locator('#create-weight-input').fill('22.5');
    await page
      .locator('#create-gustilo-class-input')
      .selectOption('GUSTILO_GRADE_III_SHEARING_DEGLOVING');
    await page.locator('#create-att-score-input').fill('8');
    await page.locator('#create-lavage-input').fill('1800');
    await page.locator('#create-npwt-input').fill('0');
    await page.locator('#create-lactate-input').fill('3.9');
    await page.locator('#create-toe-delta-input').fill('1.6');
    await page.locator('#create-exposed-bone-checkbox').check();
    await page.locator('#create-ortho-op-btn').click();

    const banditCard = page
      .locator('#ortho-operations-grid .ortho-op-card', { hasText: 'Bandit (' })
      .first();
    await expect(banditCard).toBeVisible();
    await expect(banditCard).toHaveAttribute(
      'data-status',
      'ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE'
    );
    await expect(page.locator('#active-op-title')).toContainText('Bandit');
    await expect(page.locator('#active-att-score')).toContainText('8/18');
    await expect(page.locator('#active-lavage-volume')).toContainText('1800 mL');

    // Step 4: Stage 7-8 PSI lavage, NPWT (-125 mmHg), Robert Jones & ESF protocol for Bandit
    await page.locator('#stage-fixation-btn').click();
    await expect(banditCard).toHaveAttribute(
      'data-status',
      'LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED'
    );

    // Step 5: Record serial orthopedic & wound check for Bandit
    await page.locator('#check-att-score-input').fill('2');
    await page.locator('#check-lavage-input').fill('3200');
    await page.locator('#check-npwt-input').fill('-125');
    await page.locator('#check-lactate-input').fill('1.2');
    await page.locator('#check-toe-delta-input').fill('0.3');
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Caleb Thorne, DACVS (Orthopedics)');
    await page
      .locator('#check-note-input')
      .fill('Post-3,200 mL 7-8 PSI warm LRS lavage & -125 mmHg NPWT: Bandit healthy granulation bed covering exposed cortical bone, ATT score 2.');
    await page.locator('#record-wound-check-btn').click();

    await expect(page.locator('#active-att-score')).toContainText('2/18');
    await expect(page.locator('#active-lavage-volume')).toContainText('3200 mL');
    await expect(page.locator('#wound-checks-list')).toContainText(
      'Post-3,200 mL 7-8 PSI warm LRS lavage & -125 mmHg NPWT: Bandit healthy granulation bed'
    );

    // Step 6: Clear Bandit for Granulation & Fixation Stable Discharge
    await page
      .locator('#clear-discharged-by-input')
      .fill('Dr. Caleb Thorne, DACVS (Orthopedics)');
    await page
      .locator('#clear-summary-input')
      .fill('Bandit granulation bed complete, transarticular ESF stable, ATT score 2, Lactate 1.2 mmol/L, warm distal toes (ΔT 0.3°C); cleared for orthopedic ward.');
    await page.locator('#clear-ortho-btn').click();

    await expect(banditCard).toHaveAttribute(
      'data-status',
      'GRANULATION_FIXATION_STABLE_CLEARED'
    );
    await expect(page.locator('#ortho-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Bandit');

    // Step 7: Filter by GRANULATION_FIXATION_STABLE_CLEARED
    await page
      .locator('#ortho-status-filter')
      .selectOption('GRANULATION_FIXATION_STABLE_CLEARED');
    await expect(
      page.locator('#ortho-operations-grid .ortho-op-card', { hasText: 'Bandit (' })
    ).toBeVisible();
    await expect(
      page.locator('#ortho-operations-grid .ortho-op-card', { hasText: 'Toby (' })
    ).toBeVisible();

    await page.locator('#ortho-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
