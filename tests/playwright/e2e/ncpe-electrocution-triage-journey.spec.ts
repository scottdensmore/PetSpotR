import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Non-Cardiogenic Pulmonary Edema (NCPE), Electrocution (Oral Cord Bite), Upper Airway Obstruction / Post-Ictal Neurogenic Edema, Low-Dose Furosemide Controversy & CPAP/PEEP Mechanical Ventilation Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new NCPE patient "Volt", staging O2/Butorphanol/CPAP/PEEP protocol, recording respiratory & VetBLUE B-line check, clearing for room-air discharge, and filtering by BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/ncpe-electrocution-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#ncpe-electrocution-cockpit')).toBeVisible();
    await expect(page.locator('#ncpe-alert-banner')).toBeVisible();

    await expect(
      page.locator('#ncpe-operations-grid .ncpe-op-card', { hasText: 'Sparky (' })
    ).toBeVisible();
    await expect(
      page.locator('#ncpe-operations-grid .ncpe-op-card', { hasText: 'Milo (' })
    ).toBeVisible();
    await expect(
      page.locator('#ncpe-operations-grid .ncpe-op-card', { hasText: 'Luna (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new NCPE / Electrocution patient "Volt"
    await page.locator('#create-case-id-input').fill('CASE-NCPE-104');
    await page.locator('#create-pet-name-input').fill('Volt');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Labrador Retriever Puppy');
    await page.locator('#create-weight-input').fill('7.2');
    await page
      .locator('#create-etiology-input')
      .selectOption('ORAL_ELECTRICAL_CORD_BITE_NEUROGENIC');
    await page.locator('#create-pf-ratio-input').fill('155.0');
    await page.locator('#create-spo2-input').fill('85.0');
    await page.locator('#create-rr-input').fill('72');
    await page.locator('#create-blines-input').fill('16');
    await page.locator('#create-la2ao-input').fill('1.20');
    await page.locator('#create-eschar-checkbox').check();
    await page.locator('#create-froth-checkbox').check();
    await page.locator('#create-ncpe-op-btn').click();

    const voltCard = page
      .locator('#ncpe-operations-grid .ncpe-op-card', { hasText: 'Volt (' })
      .first();
    await expect(voltCard).toBeVisible();
    await expect(voltCard).toHaveAttribute(
      'data-status',
      'ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE'
    );
    await expect(page.locator('#active-op-title')).toContainText('Volt');
    await expect(page.locator('#active-pf-ratio')).toContainText('155.0');
    await expect(page.locator('#active-blines-count')).toContainText('16 B-Lines');

    // Step 4: Stage O2 / Butorphanol / CPAP / PEEP protocol for Volt
    await page.locator('#stage-cpap-btn').click();
    await expect(voltCard).toHaveAttribute(
      'data-status',
      'OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED'
    );

    // Step 5: Record serial respiratory & VetBLUE B-line check for Volt
    await page.locator('#check-pf-ratio-input').fill('405.0');
    await page.locator('#check-spo2-input').fill('99.0');
    await page.locator('#check-rr-input').fill('26');
    await page.locator('#check-blines-input').fill('0');
    await page.locator('#check-la2ao-input').fill('1.20');
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Nadia Varma, DACVECC');
    await page
      .locator('#check-note-input')
      .fill('Post-HFNC & Helmet CPAP (PEEP 6 cmH2O): Volt caudodorsal B-lines completely resolved (0 B-lines), PaO2/FiO2 405, SpO2 99% on room air.');
    await page.locator('#record-respiratory-check-btn').click();

    await expect(page.locator('#active-pf-ratio')).toContainText('405.0');
    await expect(page.locator('#active-blines-count')).toContainText('0 B-Lines');
    await expect(page.locator('#respiratory-checks-list')).toContainText(
      'Post-HFNC & Helmet CPAP (PEEP 6 cmH2O): Volt caudodorsal B-lines completely resolved'
    );

    // Step 6: Clear Volt for Room-Air Discharge
    await page
      .locator('#clear-discharged-by-input')
      .fill('Dr. Nadia Varma, DACVECC');
    await page
      .locator('#clear-summary-input')
      .fill('Volt caudodorsal B-lines resolved (0 B-lines), normoxemic on room air (PaO2/FiO2 405, SpO2 99%, RR 26 bpm), 5–7 day oral eschar follow-up scheduled.');
    await page.locator('#clear-ncpe-btn').click();

    await expect(voltCard).toHaveAttribute(
      'data-status',
      'BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED'
    );
    await expect(page.locator('#ncpe-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Volt');

    // Step 7: Filter by BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED
    await page
      .locator('#ncpe-status-filter')
      .selectOption('BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED');
    await expect(
      page.locator('#ncpe-operations-grid .ncpe-op-card', { hasText: 'Volt (' })
    ).toBeVisible();
    await expect(
      page.locator('#ncpe-operations-grid .ncpe-op-card', { hasText: 'Luna (' })
    ).toBeVisible();

    await page.locator('#ncpe-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
