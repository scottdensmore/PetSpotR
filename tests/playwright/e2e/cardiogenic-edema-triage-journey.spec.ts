import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Cardiogenic Pulmonary Edema, Mitral Valve Chordae Rupture, Furosemide/Pimobendan & Oxygen Cage Cockpit E2E Journey', () => {
  test('Full operator journey: admit "Darcy", stage furosemide/pimobendan/O2 cage regimen, log diuresis telemetry check, issue eupneic discharge clearance, and filter by EUPNEIC_BLINES_RESOLVED_CLEARED', async ({
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

    // Step 1: Load Cockpit & Verify Seeded Operations
    await page.goto(`${WEB_FRONTEND_URL}/cardiogenic-edema-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#cardiogenic-edema-triage-main')).toBeVisible();
    await expect(
      page.locator('#cardiogenic-operations-list .cardiogenic-op-card', { hasText: 'Oliver (' })
    ).toBeVisible();
    await expect(
      page.locator('#cardiogenic-operations-list .cardiogenic-op-card', { hasText: 'Cleo (' })
    ).toBeVisible();
    await expect(
      page.locator('#cardiogenic-operations-list .cardiogenic-op-card', { hasText: 'Teddy (' })
    ).toBeVisible();

    // Step 2: Create new cardiogenic pulmonary edema operation ("Darcy")
    await page.locator('#create-incident-id').fill('INC-CE-9109');
    await page.locator('#create-pet-name').fill('Darcy');
    await page.locator('#create-species').selectOption('CANINE_MMVD_STAGE_C_D');
    await page.locator('#create-weight-kg').fill('8.5');
    await page.locator('#create-resp-rate').fill('82');
    await page.locator('#create-heart-rate').fill('185');
    await page.locator('#create-murmur-grade').selectOption('GRADE_VI_THRILL_CHORDAE_RUPTURE');
    await page.locator('#create-tfast-blines').fill('7');
    await page.locator('#create-la-ao-ratio').fill('2.35');
    await page.locator('#btn-create-cardiogenic-op').click();

    const darcyInitialCard = page
      .locator('#cardiogenic-operations-list .cardiogenic-op-card', { hasText: 'Darcy (' })
      .first();
    await expect(darcyInitialCard).toBeVisible();
    const darcyOpId = await darcyInitialCard.getAttribute('data-op-id');
    expect(darcyOpId).toBeTruthy();

    const darcyCard = page.locator(
      `#cardiogenic-operations-list .cardiogenic-op-card[data-op-id="${darcyOpId}"]`
    );
    await expect(darcyCard).toHaveAttribute('data-status', 'FULMINANT_PULMONARY_EDEMA_CRITICAL');
    await expect(page.locator('#cardiogenic-alert-banner')).toBeVisible();

    // Step 3: Stage Furosemide / Pimobendan / O2 Cage Regimen on Darcy
    await page.locator('#btn-stage-regimen').click();
    await expect(darcyCard).toHaveAttribute(
      'data-status',
      'FUROSEMIDE_PIMOBENDAN_O2_CAGE_STAGED'
    );

    // Step 4: Log Diuresis Telemetry Check on Darcy
    await page.locator('#check-resp-rate').fill('30');
    await page.locator('#check-urine-output').fill('4.1');
    await page.locator('#check-tfast-blines').fill('1');
    await page.locator('#check-sbp').fill('120');
    await page.locator('#check-creatinine').fill('1.2');
    await page.locator('#check-potassium').fill('4.0');
    await page.locator('#check-clinician').fill('DR-CARDIO-33');
    await page.locator('#check-notes').fill('B-lines resolved to <= 1/site; eupneic in O2 cage.');
    await page.locator('#btn-record-diuresis-check').click();

    await expect(page.locator('#workspace-checks-list')).toContainText('DR-CARDIO-33');

    // Step 5: Issue Eupneic & B-Line Resolved Discharge Clearance on Darcy
    await page.locator('#clearance-final-srr').fill('20');
    await page.locator('#clearance-final-blines').fill('0');
    await page.locator('#clearance-spo2').fill('98.5');
    await page.locator('#clearance-vet-license').fill('VET-CARDIO-77410');
    await page.locator('#clearance-renal-stable').check();
    await page.locator('#clearance-srr-educated').check();
    await page
      .locator('#clearance-summary')
      .fill('Darcy eupneic on room air, B-lines cleared, owner educated on daily SRR < 30 bpm.');
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(darcyCard).toHaveAttribute('data-status', 'EUPNEIC_BLINES_RESOLVED_CLEARED');
    await expect(page.locator('#cardiogenic-clearance-banner')).toBeVisible();
    await expect(page.locator('#cardiogenic-clearance-banner')).toContainText('VET-CARDIO-77410');

    // Step 6: Filter by EUPNEIC_BLINES_RESOLVED_CLEARED and verify Darcy and Teddy remain visible
    await page.locator('#filter-status').selectOption('EUPNEIC_BLINES_RESOLVED_CLEARED');
    await expect(
      page.locator('#cardiogenic-operations-list .cardiogenic-op-card', { hasText: 'Darcy (' })
    ).toBeVisible();
    await expect(
      page.locator('#cardiogenic-operations-list .cardiogenic-op-card', { hasText: 'Teddy (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
