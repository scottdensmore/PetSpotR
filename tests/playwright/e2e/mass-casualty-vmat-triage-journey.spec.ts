import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Disaster Mass-Casualty Veterinary Triage (START/SALT Veterinary Color-Tagging: Immediate Red, Delayed Yellow, Minimal Green, Expectant Black), Decontamination Corridor, Field Hospital Surge Capacity & FEMA/AVMA VMAT Deployment Command Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Mass-Casualty VMAT patient "Cascade", staging warm-zone decon & ICS-214 protocol, recording VMAT surge reassessment check, clearing for reunification / medevac transport, and filtering by REUNIFIED_EVAC_TRANSPORT_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/mass-casualty-vmat-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#mass-casualty-vmat-cockpit')).toBeVisible();
    await expect(page.locator('#mci-alert-banner')).toBeVisible();

    await expect(
      page.locator('#mci-operations-grid .mci-op-card', { hasText: 'Kodiak (' })
    ).toBeVisible();
    await expect(
      page.locator('#mci-operations-grid .mci-op-card', { hasText: 'Cinder (' })
    ).toBeVisible();
    await expect(
      page.locator('#mci-operations-grid .mci-op-card', { hasText: 'Shasta (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Mass-Casualty VMAT patient "Cascade"
    await page.locator('#create-case-id-input').fill('CASE-MCI-104');
    await page.locator('#create-pet-name-input').fill('Cascade');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('German Shepherd (SAR K9)');
    await page.locator('#create-weight-input').fill('32.0');
    await page
      .locator('#create-triage-tag-input')
      .selectOption('RED_IMMEDIATE_PRIORITY_1_LIFE_THREAT');
    await page
      .locator('#create-protocol-input')
      .selectOption('HOT_WARM_COLD_ZONE_DAWN_DECON_CORRIDOR');
    await page.locator('#create-ats-score-input').fill('18');
    await page.locator('#create-lactate-input').fill('6.4');
    await page.locator('#create-spo2-input').fill('88.0');
    await page.locator('#create-surge-occupancy-input').fill('92.0');
    await page.locator('#create-rfid-collar-input').fill('VMAT-RFID-9104');
    await page.locator('#create-ics214-input').fill('ICS214-FEMA-2026-104');
    await page.locator('#create-gross-decon-checkbox').check();
    await page.locator('#create-contagious-segregated-checkbox').check();
    await page.locator('#create-mci-op-btn').click();

    const cascadeCard = page
      .locator('#mci-operations-grid .mci-op-card', { hasText: 'Cascade (' })
      .first();
    await expect(cascadeCard).toBeVisible();
    await expect(cascadeCard).toHaveAttribute(
      'data-status',
      'START_SALT_TAGGED_DECON_STAGED'
    );
    await expect(page.locator('#active-op-title')).toContainText('Cascade');
    await expect(page.locator('#active-ats-score')).toContainText('18/25');
    await expect(page.locator('#active-lactate')).toContainText('6.40 mmol/L');
    await expect(page.locator('#active-spo2')).toContainText('88.0%');

    // Step 4: Stage warm-zone decon, damage-control resuscitation & ICS-214 protocol for Cascade
    await page.locator('#stage-vmat-btn').click();
    await expect(cascadeCard).toHaveAttribute(
      'data-status',
      'DAMAGE_CONTROL_SURGE_ICU_MONITORED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'DAMAGE_CONTROL_SURGE_ICU_MONITORED'
    );

    // Step 5: Record post-resuscitation VMAT surge reassessment check for Cascade
    await page
      .locator('#check-triage-tag-input')
      .selectOption('YELLOW_DELAYED_PRIORITY_2_URGENT_SURGERY');
    await page.locator('#check-ats-score-input').fill('4');
    await page.locator('#check-lactate-input').fill('1.75');
    await page.locator('#check-spo2-input').fill('98.0');
    await page.locator('#check-surge-occupancy-input').fill('76.0');
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)');
    await page
      .locator('#check-note-input')
      .fill(
        'Post-resuscitation check for Cascade: arterial hemorrhage controlled, ATS improved to 4/25, lactate 1.75 mmol/L, SpO2 98.0%, surge occupancy 76.0%.'
      );
    await page.locator('#record-reassessment-btn').click();

    await expect(page.locator('#active-ats-score')).toContainText('4/25');
    await expect(page.locator('#active-lactate')).toContainText('1.75 mmol/L');
    await expect(page.locator('#active-spo2')).toContainText('98.0%');
    await expect(page.locator('#reassessment-checks-list')).toContainText(
      'Post-resuscitation check for Cascade: arterial hemorrhage controlled'
    );

    // Step 6: Clear Cascade for Owner Reunification / Medevac Transport
    await page.locator('#clear-lactate-input').fill('1.75');
    await page.locator('#clear-spo2-input').fill('98.0');
    await page.locator('#clear-rfid-verified-checkbox').check();
    await page.locator('#clear-ics214-signed-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)');
    await page
      .locator('#clear-summary-input')
      .fill(
        'Cascade perfusion normalized (lactate 1.75 mmol/L <= 2.5 mmol/L), SpO2 98.0% (>= 95.0%), warm-zone decon verified, RFID collar VMAT-RFID-9104 matched, and FEMA ICS-214 signed; cleared.'
      );
    await page.locator('#clear-mci-btn').click();

    await expect(cascadeCard).toHaveAttribute(
      'data-status',
      'REUNIFIED_EVAC_TRANSPORT_CLEARED'
    );
    await expect(page.locator('#mci-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Cascade');

    // Step 7: Filter by REUNIFIED_EVAC_TRANSPORT_CLEARED
    await page
      .locator('#mci-status-filter')
      .selectOption('REUNIFIED_EVAC_TRANSPORT_CLEARED');
    await expect(
      page.locator('#mci-operations-grid .mci-op-card', { hasText: 'Cascade (' })
    ).toBeVisible();
    await expect(
      page.locator('#mci-operations-grid .mci-op-card', { hasText: 'Shasta (' })
    ).toBeVisible();

    await page.locator('#mci-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
