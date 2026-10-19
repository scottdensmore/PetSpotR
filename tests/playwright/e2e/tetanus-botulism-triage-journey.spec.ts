import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Tetanus (Clostridium tetani) & Botulism (Clostridium botulinum), Risus Sardonicus / Sawhorse Stance vs. Ascending Flaccid Lower Motor Neuron Paralysis, Equine Tetanus Antitoxin (TAT) Intradermal Test, Metronidazole (vs. Penicillin GABA Antagonism) & Dark/Quiet Sensory-Isolation Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Tetanus patient "Apollo", staging Metronidazole/Equine TAT/Methocarbamol therapy, recording neuro/ventilation check, clearing for normoreflexic discharge, and filtering by NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/tetanus-botulism-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#tetanus-botulism-cockpit')).toBeVisible();
    await expect(page.locator('#clb-alert-banner')).toBeVisible();

    await expect(
      page.locator('#clb-operations-grid .clb-op-card', { hasText: 'Rex (' })
    ).toBeVisible();
    await expect(
      page.locator('#clb-operations-grid .clb-op-card', { hasText: 'Otis (' })
    ).toBeVisible();
    await expect(
      page.locator('#clb-operations-grid .clb-op-card', { hasText: 'Bella (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Tetanus patient "Apollo" via UI form (without mutating CLB-OP-101)
    await page.locator('#create-case-id-input').fill('CASE-CLB-104');
    await page.locator('#create-pet-name-input').fill('Apollo');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Rottweiler');
    await page.locator('#create-weight-input').fill('34.0');
    await page
      .locator('#create-syndrome-input')
      .selectOption('TETANUS_CLASS_II_GENERALIZED_SAWHORSE');
    await page
      .locator('#create-protocol-input')
      .selectOption('EQUINE_TAT_100_1000_IU_KG_AFTER_ID_TEST');
    await page.locator('#create-burkitt-class-input').fill('2');
    await page.locator('#create-spasm-freq-input').fill('4');
    await page.locator('#create-core-temp-input').fill('39.6');
    await page.locator('#create-paco2-input').fill('46.0');
    await page.locator('#create-hr-input').fill('136');
    await page.locator('#create-metronidazole-input').fill('10.0');
    await page.locator('#create-tat-units-input').fill('400');
    await page.locator('#create-methocarbamol-input').fill('50');
    await page.locator('#create-id-test-checkbox').check();
    await page.locator('#create-penicillin-gaba-checkbox').check();
    await page.locator('#create-earplugs-checkbox').check();
    await page.locator('#create-aminoglycoside-checkbox').check();
    await page.locator('#create-clb-op-btn').click();

    const apolloCard = page
      .locator('#clb-operations-grid .clb-op-card', { hasText: 'Apollo (' })
      .first();
    await expect(apolloCard).toBeVisible();
    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED'
    );
    await expect(page.locator('#active-op-title')).toContainText('Apollo');
    await expect(page.locator('#active-burkitt-class')).toContainText('Class 2/4');
    await expect(page.locator('#active-spasm-frequency')).toContainText('4 spasms/hr');

    // Step 4: Stage Metronidazole, Equine TAT (after ID test), and Methocarbamol protocol for Apollo
    await page.locator('#stage-tat-units-input').fill('500');
    await page.locator('#stage-therapy-btn').click();
    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED'
    );
    await expect(page.locator('#active-metro-tat')).toContainText('500 IU/kg');

    // Step 5: Record serial neurological & ventilation check for Apollo
    await page.locator('#check-burkitt-class-input').fill('1');
    await page.locator('#check-spasm-freq-input').fill('0');
    await page.locator('#check-core-temp-input').fill('38.5');
    await page.locator('#check-paco2-input').fill('38.0');
    await page.locator('#check-hr-input').fill('86');
    await page.locator('#check-gag-reflex-checkbox').check();
    await page.locator('#check-earplugs-checkbox').check();
    await page
      .locator('#check-recorded-by-input')
      .fill('Dr. Ingrid Vance, DACVIM (Neurology)');
    await page
      .locator('#check-note-input')
      .fill('Apollo Day 14 check: Sawhorse stance resolved, 0 spasms/hr, intact pharyngeal gag reflex, PaCO2 38.0 mmHg.');
    await page.locator('#record-neuro-check-btn').click();

    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'SENSORY_ISOLATION_VENTILATION_MONITORED'
    );
    await expect(page.locator('#active-burkitt-class')).toContainText('Class 1/4');
    await expect(page.locator('#active-spasm-frequency')).toContainText('0 spasms/hr');
    await expect(page.locator('#neuro-checks-list')).toContainText(
      'Apollo Day 14 check: Sawhorse stance resolved'
    );

    // Step 6: Clear Apollo for Normoreflexic & Swallowing-Intact Discharge
    await page.locator('#clear-spasm-freq-input').fill('0');
    await page.locator('#clear-paco2-input').fill('38.0');
    await page.locator('#clear-gag-reflex-checkbox').check();
    await page.locator('#clear-ambulatory-checkbox').check();
    await page
      .locator('#clear-cleared-by-input')
      .fill('Dr. Ingrid Vance, DACVIM (Neurology)');
    await page
      .locator('#clear-summary-input')
      .fill('Apollo tetanus resolved: 0 spasms/hr, intact pharyngeal gag reflex, PaCO2 38.0 mmHg, normothermic and ambulatory.');
    await page.locator('#clear-clb-btn').click();

    await expect(apolloCard).toHaveAttribute(
      'data-status',
      'NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED'
    );
    await expect(page.locator('#clb-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Apollo');

    // Step 7: Filter by NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED
    await page
      .locator('#clb-status-filter')
      .selectOption('NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED');
    await expect(
      page.locator('#clb-operations-grid .clb-op-card', { hasText: 'Apollo (' })
    ).toBeVisible();
    await expect(
      page.locator('#clb-operations-grid .clb-op-card', { hasText: 'Bella (' })
    ).toBeVisible();

    await page.locator('#clb-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
