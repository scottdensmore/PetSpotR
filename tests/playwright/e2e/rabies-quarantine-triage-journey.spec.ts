import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Rabies Quarantine, Lyssavirus Post-Exposure Prophylaxis (PEP), Strict 10-Day Domestic Bite Observation vs. 45-Day/4-Month Unvaccinated Quarantine, Biosecurity Double-Barrier Isolation & Public Health Rabies Control Officer (RCO) Statutory Reporting Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Rabies Quarantine patient "Zephyr", staging double-barrier biosecurity & RCO statutory protocol, recording Day 10 neurological/salivary shedding check, clearing for statutory RCO release, and filtering by STATUTORY_QUARANTINE_SURVIVED_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/rabies-quarantine-pep-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#rabies-quarantine-cockpit')).toBeVisible();
    await expect(page.locator('#rbq-alert-banner')).toBeVisible();

    await expect(
      page.locator('#rbq-operations-grid .rbq-op-card', { hasText: 'Koda (' })
    ).toBeVisible();
    await expect(
      page.locator('#rbq-operations-grid .rbq-op-card', { hasText: 'Vesper (' })
    ).toBeVisible();
    await expect(
      page.locator('#rbq-operations-grid .rbq-op-card', { hasText: 'Artemis (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Rabies Quarantine patient "Zephyr"
    await page.locator('#create-case-id-input').fill('CASE-RBQ-104');
    await page.locator('#create-pet-name-input').fill('Zephyr');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Belgian Malinois');
    await page.locator('#create-weight-input').fill('29.5');
    await page.locator('#create-rco-filing-input').fill('RCO-STAT-2026-104');
    await page
      .locator('#create-statutory-category-input')
      .selectOption('TEN_DAY_DOMESTIC_BITE_CONFINEMENT');
    await page
      .locator('#create-protocol-input')
      .selectOption('STRICT_TEN_DAY_NO_VACCINE_DURING_BITE_HOLD');
    await page.locator('#create-day-elapsed-input').fill('1');
    await page.locator('#create-required-days-input').fill('10');
    await page.locator('#create-rffit-titer-input').fill('0.40');
    await page.locator('#create-no-vaccine-hold-checkbox').check();
    await page.locator('#create-double-barrier-checkbox').check();
    await page.locator('#create-rbq-op-btn').click();

    const zephyrCard = page
      .locator('#rbq-operations-grid .rbq-op-card', { hasText: 'Zephyr (' })
      .first();
    await expect(zephyrCard).toBeVisible();
    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'BIOSECURITY_ISOLATION_RCO_HOLD_STAGED'
    );
    await expect(page.locator('#active-op-title')).toContainText('Zephyr');
    await expect(page.locator('#active-quarantine-days')).toContainText('Day 1 / 10 days');
    await expect(page.locator('#active-rffit-titer')).toContainText('0.40 IU/mL');

    // Step 4: Stage biosecurity double-barrier isolation & RCO protocol for Zephyr
    await page.locator('#stage-quarantine-btn').click();
    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED'
    );

    // Step 5: Record Day 10 neurological & salivary shedding observation check for Zephyr
    await page.locator('#check-day-elapsed-input').fill('10');
    await page.locator('#check-rffit-titer-input').fill('1.60');
    await page.locator('#check-bulbar-absent-checkbox').check();
    await page.locator('#check-dysphagia-absent-checkbox').check();
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)');
    await page
      .locator('#check-note-input')
      .fill('Day 10 of 10 observation check for Zephyr: cranial nerves V/VII/IX/X/XII normal, zero bulbar palsy or dysphagia, RFFIT 1.60 IU/mL.');
    await page.locator('#record-neuro-check-btn').click();

    await expect(page.locator('#active-quarantine-days')).toContainText('Day 10 / 10 days');
    await expect(page.locator('#active-rffit-titer')).toContainText('1.60 IU/mL');
    await expect(page.locator('#neuro-checks-list')).toContainText(
      'Day 10 of 10 observation check for Zephyr'
    );

    // Step 6: Clear Zephyr for Statutory Quarantine Survival & RCO Release
    await page.locator('#clear-day-elapsed-input').fill('10');
    await page.locator('#clear-rffit-titer-input').fill('1.60');
    await page.locator('#clear-rco-release-checkbox').check();
    await page.locator('#clear-exit-vaccine-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)');
    await page
      .locator('#clear-summary-input')
      .fill('Zephyr survived full 10-day NASPHV domestic bite observation (Day 10/10), cranial nerves intact, RCO release RCO-STAT-2026-104 signed, exit rabies vaccine administered; cleared.');
    await page.locator('#clear-rbq-btn').click();

    await expect(zephyrCard).toHaveAttribute(
      'data-status',
      'STATUTORY_QUARANTINE_SURVIVED_CLEARED'
    );
    await expect(page.locator('#rbq-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Zephyr');

    // Step 7: Filter by STATUTORY_QUARANTINE_SURVIVED_CLEARED
    await page
      .locator('#rbq-status-filter')
      .selectOption('STATUTORY_QUARANTINE_SURVIVED_CLEARED');
    await expect(
      page.locator('#rbq-operations-grid .rbq-op-card', { hasText: 'Zephyr (' })
    ).toBeVisible();
    await expect(
      page.locator('#rbq-operations-grid .rbq-op-card', { hasText: 'Artemis (' })
    ).toBeVisible();

    await page.locator('#rbq-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
