import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Status Epilepticus, Cluster Seizures, Bromethalin/Metaldehyde Neuro-Tremor & Levetiracetam/Propofol CRI Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Nova", staging titration regimen, logging serial neuro ICU check, issuing 24h seizure-free normothermic discharge clearance, and filtering by SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/status-epilepticus-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#status-epilepticus-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Status Epilepticus, Cluster Seizures, Bromethalin/Metaldehyde Neuro-Tremor & Levetiracetam/Propofol CRI Cockpit'
    );

    await expect(
      page.locator('#epilepticus-operations-list .epilepticus-op-card', { hasText: 'Koda (' })
    ).toBeVisible();
    await expect(
      page.locator('#epilepticus-operations-list .epilepticus-op-card', { hasText: 'Mochi (' })
    ).toBeVisible();
    await expect(
      page.locator('#epilepticus-operations-list .epilepticus-op-card', { hasText: 'Bowie (' })
    ).toBeVisible();

    // Step 2: Create new operation "Nova"
    await page.locator('#create-incident-id').fill('INC-SE-9204');
    await page.locator('#create-pet-name').fill('Nova');
    await page.locator('#create-species').selectOption('CANINE');
    await page.locator('#create-breed').fill('Belgian Malinois');
    await page.locator('#create-weight-kg').fill('26.0');
    await page.locator('#create-neuro-etiology').selectOption('BROMETHALIN_CEREBRAL_EDEMA');
    await page.locator('#create-ictal-mins').fill('12');
    await page.locator('#create-cluster-count').fill('4');
    await page.locator('#create-rectal-temp').fill('105.8');
    await page.locator('#create-blood-glucose').fill('58.0');
    await page.locator('#create-mgcs').fill('10');
    await page.locator('#btn-create-epilepticus-op').click();

    const novaCard = page
      .locator('#epilepticus-operations-list .epilepticus-op-card', { hasText: 'Nova (' })
      .first();
    await expect(novaCard).toBeVisible();
    await expect(novaCard).toHaveAttribute('data-status', 'ACTIVE_ICTAL_HYPERTHERMIA_CRITICAL');

    // Step 3: Stage anticonvulsant & osmotherapy titration regimen for Nova
    await page.locator('#stage-weight-kg').fill('26.0');
    await page.locator('#stage-neuro-etiology').selectOption('BROMETHALIN_CEREBRAL_EDEMA');
    await page.locator('#stage-rectal-temp').fill('104.2');
    await page.locator('#stage-mgcs').fill('12');
    await page.locator('#btn-stage-regimen').click();

    await expect(novaCard).toHaveAttribute('data-status', 'MIDAZOLAM_KEPPRA_METHOCARBAMOL_STAGED');
    await expect(page.locator('#regimen-mannitol-g')).toContainText('13.00 g');

    // Step 4: Log serial neuro ICU check for Nova
    await page.locator('#check-rectal-temp').fill('101.4');
    await page.locator('#check-mgcs').fill('17');
    await page.locator('#check-plr').selectOption('NORMAL_BILATERAL');
    await page.locator('#check-blood-glucose').fill('98.0');
    await page.locator('#check-callsign').fill('DR-NEURO-88');
    await page.locator('#check-notes').fill('Cerebral edema improved post-mannitol; normothermic 101.4°F.');
    await page.locator('#check-seizure-observed').uncheck();
    await page.locator('#btn-record-neuro-check').click();

    await expect(page.locator('#neuro-checks-list')).toContainText(
      'Cerebral edema improved post-mannitol; normothermic 101.4°F.'
    );

    // Step 5: Issue 24h Seizure-Free Normothermic Discharge Clearance for Nova
    await page.locator('#clearance-seizure-free-hours').fill('28');
    await page.locator('#clearance-final-mgcs').fill('18');
    await page.locator('#clearance-final-temp').fill('101.0');
    await page.locator('#clearance-vet-license').fill('VET-NEURO-77310');
    await page.locator('#clearance-summary').fill('28h seizure-free normothermic recovery; MGCS 18/18; home intranasal midazolam dispensed.');
    await page.locator('#clearance-ambulatory-menace').check();
    await page.locator('#clearance-rescue-midazolam').check();
    await page.locator('#btn-issue-discharge-clearance').click();

    await expect(novaCard).toHaveAttribute('data-status', 'SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED');
    await expect(page.locator('#epilepticus-clearance-banner')).toBeVisible();
    await expect(page.locator('#epilepticus-clearance-banner')).toContainText('VET-NEURO-77310');

    // Step 6: Filter by SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED and verify Nova and Bowie remain visible
    await page.locator('#filter-status').selectOption('SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED');
    await expect(
      page.locator('#epilepticus-operations-list .epilepticus-op-card', { hasText: 'Nova (' })
    ).toBeVisible();
    await expect(
      page.locator('#epilepticus-operations-list .epilepticus-op-card', { hasText: 'Bowie (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
