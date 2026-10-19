import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Hepatic Failure, Hepatic Encephalopathy, Sago Palm / Microcystin / Amanita Hepatotoxicity, N-Acetylcysteine (NAC) & Lactulose/Levetiracetam Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Hepatic patient "Bodie", staging NAC 140 mg/kg / Lactulose / Levetiracetam / Vitamin K1 protocol, recording serial blood ammonia & ALT check, clearing for discharge, and filtering by AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/hepatic-encephalopathy-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#hepatic-encephalopathy-cockpit')).toBeVisible();
    await expect(page.locator('#hepatic-alert-banner')).toBeVisible();

    await expect(
      page.locator('#hepatic-operations-grid .hepatic-op-card', { hasText: 'Kona (' })
    ).toBeVisible();
    await expect(
      page.locator('#hepatic-operations-grid .hepatic-op-card', { hasText: 'Marmalade (' })
    ).toBeVisible();
    await expect(
      page.locator('#hepatic-operations-grid .hepatic-op-card', { hasText: 'Scout (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Hepatic patient "Bodie"
    await page.locator('#create-case-id-input').fill('CASE-HEP-104');
    await page.locator('#create-pet-name-input').fill('Bodie');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Golden Retriever');
    await page.locator('#create-weight-input').fill('29.5');
    await page
      .locator('#create-etiology-input')
      .selectOption('SAGO_PALM_CYCASIN_HEPATOTOXICITY');
    await page.locator('#create-alt-input').fill('3120');
    await page.locator('#create-bilirubin-input').fill('7.2');
    await page.locator('#create-ammonia-input').fill('260.0');
    await page.locator('#create-glucose-input').fill('52.0');
    await page.locator('#create-pt-input').fill('27.8');
    await page.locator('#create-he-grade-input').fill('3');
    await page.locator('#create-seizures-checkbox').check();
    await page.locator('#create-hepatic-op-btn').click();

    const bodieCard = page
      .locator('#hepatic-operations-grid .hepatic-op-card', { hasText: 'Bodie (' })
      .first();
    await expect(bodieCard).toBeVisible();
    await expect(bodieCard).toHaveAttribute(
      'data-status',
      'ACUTE_FULMINANT_HEPATOTOXICITY_HE_TRIAGE'
    );
    await expect(page.locator('#active-op-title')).toContainText('Bodie');
    await expect(page.locator('#active-blood-ammonia')).toContainText('260.0 umol/L');
    await expect(page.locator('#active-alt-value')).toContainText('3120 IU/L');

    // Step 4: Stage NAC 140 mg/kg IV load, Lactulose enema & Levetiracetam protocol for Bodie
    await page.locator('#stage-support-btn').click();
    await expect(bodieCard).toHaveAttribute('data-status', 'NAC_LACTULOSE_VITAMIN_K1_STAGED');
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'NAC_LACTULOSE_VITAMIN_K1_STAGED'
    );

    // Step 5: Record serial hepatic metabolic & ammonia check for Bodie
    await page.locator('#check-alt-input').fill('410');
    await page.locator('#check-bilirubin-input').fill('1.4');
    await page.locator('#check-ammonia-input').fill('38.0');
    await page.locator('#check-glucose-input').fill('102.0');
    await page.locator('#check-pt-input').fill('12.2');
    await page.locator('#check-recorded-by-input').fill('Dr. Priya Nair, DACVECC');
    await page
      .locator('#check-note-input')
      .fill(
        'Post-NAC 140 mg/kg IV & Lactulose enemas: Bodie ammonia normalized to 38.0 umol/L, ALT 410 IU/L, euglycemic 102.0 mg/dL, PT 12.2 sec.'
      );
    await page.locator('#record-metabolic-check-btn').click();

    await expect(page.locator('#active-blood-ammonia')).toContainText('38.0 umol/L');
    await expect(page.locator('#active-alt-value')).toContainText('410 IU/L');
    await expect(page.locator('#metabolic-checks-list')).toContainText(
      'Post-NAC 140 mg/kg IV & Lactulose enemas: Bodie ammonia normalized to 38.0 umol/L'
    );

    // Step 6: Clear Bodie for Hepatic Discharge
    await page.locator('#clear-discharged-by-input').fill('Dr. Priya Nair, DACVECC');
    await page
      .locator('#clear-summary-input')
      .fill(
        'Bodie recovered from Sago Palm hepatotoxicity; blood ammonia normalized (38.0 umol/L), HE resolved, PT 12.2 sec, discharged on oral SAMe/Silybin + Lactulose.'
      );
    await page.locator('#clear-hepatic-btn').click();

    await expect(bodieCard).toHaveAttribute(
      'data-status',
      'AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED'
    );
    await expect(page.locator('#hepatic-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Bodie');

    // Step 7: Filter by AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED
    await page
      .locator('#hepatic-status-filter')
      .selectOption('AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED');
    await expect(
      page.locator('#hepatic-operations-grid .hepatic-op-card', { hasText: 'Bodie (' })
    ).toBeVisible();
    await expect(
      page.locator('#hepatic-operations-grid .hepatic-op-card', { hasText: 'Scout (' })
    ).toBeVisible();

    await page.locator('#hepatic-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
