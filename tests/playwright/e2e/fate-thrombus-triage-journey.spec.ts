import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Feline Aortic Thromboembolism (FATE / Saddle Thrombus), 5 Ps, Paired Glucose/Lactate Limb Gradient & Clopidogrel/Rivaroxaban Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new FATE patient "Oliver", staging Methadone/Clopidogrel 75 mg/Rivaroxaban protocol, recording paired glucose/lactate & potassium perfusion check, clearing for dual-therapy discharge, and filtering by REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/fate-thrombus-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#fate-thrombus-cockpit')).toBeVisible();
    await expect(page.locator('#fate-alert-banner')).toBeVisible();

    await expect(
      page.locator('#fate-operations-grid .fate-op-card', { hasText: 'Jasper (' })
    ).toBeVisible();
    await expect(
      page.locator('#fate-operations-grid .fate-op-card', { hasText: 'Cleo (' })
    ).toBeVisible();
    await expect(
      page.locator('#fate-operations-grid .fate-op-card', { hasText: 'Figaro (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new FATE patient "Oliver"
    await page.locator('#create-case-id-input').fill('CASE-FATE-104');
    await page.locator('#create-pet-name-input').fill('Oliver');
    await page.locator('#create-breed-input').fill('Domestic Shorthair');
    await page.locator('#create-weight-input').fill('5.6');
    await page
      .locator('#create-distribution-input')
      .selectOption('BILATERAL_AORTIC_TRIFURCATION_SADDLE');
    await page.locator('#create-sys-glucose-input').fill('192.0');
    await page.locator('#create-limb-glucose-input').fill('88.0');
    await page.locator('#create-sys-lactate-input').fill('2.2');
    await page.locator('#create-limb-lactate-input').fill('12.6');
    await page.locator('#create-potassium-input').fill('4.8');
    await page.locator('#create-limb-temp-input').fill('28.9');
    await page.locator('#create-la2ao-input').fill('2.40');
    await page.locator('#create-chf-checkbox').check();
    await page.locator('#create-fate-op-btn').click();

    const oliverCard = page
      .locator('#fate-operations-grid .fate-op-card', { hasText: 'Oliver (' })
      .first();
    await expect(oliverCard).toBeVisible();
    await expect(oliverCard).toHaveAttribute('data-status', 'ACUTE_SADDLE_THROMBUS_5PS_TRIAGE');
    await expect(page.locator('#active-op-title')).toContainText('Oliver');
    await expect(page.locator('#active-glucose-delta')).toContainText('104.0 mg/dL');

    // Step 4: Stage Methadone/Fentanyl CRI, Clopidogrel 75 mg load & Rivaroxaban/Enoxaparin protocol for Oliver
    await page.locator('#stage-antithrombotic-btn').click();
    await expect(oliverCard).toHaveAttribute(
      'data-status',
      'METHADONE_CLOPIDOGREL_ANTICOAG_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'METHADONE_CLOPIDOGREL_ANTICOAG_STAGED'
    );

    // Step 5: Record serial limb perfusion & potassium check for Oliver
    await page.locator('#check-glucose-delta-input').fill('12.0');
    await page.locator('#check-lactate-delta-input').fill('1.1');
    await page.locator('#check-potassium-input').fill('4.2');
    await page.locator('#check-limb-temp-input').fill('37.5');
    await page.locator('#check-pulse-status-input').fill('STRONG_BILATERAL_FEMORAL_PULSES_RESTORED');
    await page.locator('#check-recorded-by-input').fill('Dr. Maya Lin, DVM, DACVECC');
    await page
      .locator('#check-note-input')
      .fill('Post-Clopidogrel 75 mg & Enoxaparin: bilateral pelvic limb pulses restored, glucose delta 12.0 mg/dL, limb temp 37.5°C, normokalemic K+ 4.2 mmol/L.');
    await page.locator('#record-perfusion-check-btn').click();

    await expect(page.locator('#active-glucose-delta')).toContainText('12.0 mg/dL');
    await expect(page.locator('#active-limb-temp')).toContainText('37.5');
    await expect(page.locator('#perfusion-checks-list')).toContainText(
      'Post-Clopidogrel 75 mg & Enoxaparin: bilateral pelvic limb pulses restored'
    );

    // Step 6: Clear Oliver for Dual-Therapy Discharge
    await page.locator('#clear-discharged-by-input').fill('Dr. Julian Vance, DACVIM (Cardiology)');
    await page
      .locator('#clear-summary-input')
      .fill('Oliver reperfused with bilateral hindlimb motor recovery, normokalemic K+ 4.2 mmol/L, discharged on Clopidogrel 18.75 mg PO q24h + Rivaroxaban 2.5 mg PO q24h.');
    await page.locator('#clear-fate-btn').click();

    await expect(oliverCard).toHaveAttribute(
      'data-status',
      'REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED'
    );
    await expect(page.locator('#fate-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Oliver');

    // Step 7: Filter by REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED
    await page
      .locator('#fate-status-filter')
      .selectOption('REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED');
    await expect(
      page.locator('#fate-operations-grid .fate-op-card', { hasText: 'Oliver (' })
    ).toBeVisible();
    await expect(
      page.locator('#fate-operations-grid .fate-op-card', { hasText: 'Figaro (' })
    ).toBeVisible();

    await page.locator('#fate-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
