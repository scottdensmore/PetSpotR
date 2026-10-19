import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Acute Esophageal Foreign Body, Megaesophagus, Myasthenia Gravis (AChR Antibody / Edrophonium-Neostigmine), Aspiration Pneumonia & Pyridostigmine/Bailey Chair Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, connecting live SSE stream, admitting new Megaesophagus/FB patient "Paddington", staging Endoscopy/Pyridostigmine/Bailey Chair protocol, recording esophageal & aspiration check, clearing for Upright Bailey Chair discharge, and filtering by UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/megaesophagus-foreignbody-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#megaesophagus-foreignbody-cockpit')).toBeVisible();
    await expect(page.locator('#mega-alert-banner')).toBeVisible();

    await expect(
      page.locator('#mega-operations-grid .mega-op-card', { hasText: 'Waffles (' })
    ).toBeVisible();
    await expect(
      page.locator('#mega-operations-grid .mega-op-card', { hasText: 'Freya (' })
    ).toBeVisible();
    await expect(
      page.locator('#mega-operations-grid .mega-op-card', { hasText: 'Pippin (' })
    ).toBeVisible();

    // Step 2: Connect Live SSE Stream
    await page.locator('#connect-sse-btn').click();
    await expect(page.locator('#sse-status-badge')).toContainText('SSE Connected');

    // Step 3: Admit new Megaesophagus / Foreign Body patient "Paddington"
    await page.locator('#create-case-id-input').fill('CASE-MEGA-104');
    await page.locator('#create-pet-name-input').fill('Paddington');
    await page.locator('#create-species-input').selectOption('Canine');
    await page.locator('#create-breed-input').fill('Golden Retriever');
    await page.locator('#create-weight-input').fill('28.2');
    await page
      .locator('#create-pathology-input')
      .selectOption('ACUTE_ESOPHAGEAL_BONE_FISHHOOK_FOREIGN_BODY');
    await page.locator('#create-spo2-input').fill('92.0');
    await page.locator('#create-aa-gradient-input').fill('31.0');
    await page.locator('#create-regurg-input').fill('5');
    await page.locator('#create-upright-input').fill('0');
    await page.locator('#create-esophagitis-input').fill('2');
    await page.locator('#create-achr-input').fill('0.15');
    await page.locator('#create-pneumonia-checkbox').check();
    await page.locator('#create-mega-op-btn').click();

    const paddingtonCard = page
      .locator('#mega-operations-grid .mega-op-card', { hasText: 'Paddington (' })
      .first();
    await expect(paddingtonCard).toBeVisible();
    await expect(paddingtonCard).toHaveAttribute(
      'data-status',
      'ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE'
    );
    await expect(page.locator('#active-op-title')).toContainText('Paddington');
    await expect(page.locator('#active-spo2')).toContainText('92.0%');
    await expect(page.locator('#active-regurg-episodes')).toContainText('5 episodes/24h');

    // Step 4: Stage Flexible Esophagoscopy, Pyridostigmine & Upright Bailey Chair protocol for Paddington
    await page.locator('#stage-bailey-btn').click();
    await expect(paddingtonCard).toHaveAttribute(
      'data-status',
      'ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED'
    );
    await expect(page.locator('#active-op-status-badge')).toContainText(
      'ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED'
    );

    // Step 5: Record serial esophageal & aspiration check for Paddington
    await page.locator('#check-spo2-input').fill('98.5');
    await page.locator('#check-aa-gradient-input').fill('9.5');
    await page.locator('#check-regurg-input').fill('0');
    await page.locator('#check-upright-input').fill('30');
    await page.locator('#check-esophagitis-input').fill('1');
    await page.locator('#check-recorded-by-input').fill('Dr. Clara Sterling, DACVIM (SAIM)');
    await page
      .locator('#check-note-input')
      .fill('Post-endoscopic bone extraction & 30-min Upright Bailey Chair feeding: Paddington 0 regurgitation episodes over 24h, SpO2 98.5%.');
    await page.locator('#record-esophageal-check-btn').click();

    await expect(page.locator('#active-spo2')).toContainText('98.5%');
    await expect(page.locator('#active-regurg-episodes')).toContainText('0 episodes/24h');
    await expect(page.locator('#esophageal-checks-list')).toContainText(
      'Post-endoscopic bone extraction & 30-min Upright Bailey Chair feeding: Paddington 0 regurgitation episodes'
    );

    // Step 6: Clear Paddington for Upright Bailey Chair Discharge
    await page.locator('#clear-discharged-by-input').fill('Dr. Clara Sterling, DACVIM (SAIM)');
    await page
      .locator('#clear-summary-input')
      .fill('Paddington esophageal foreign body resolved, 0 passive regurgitation episodes over 24h on 30-min Upright Bailey Chair feeding protocol, SpO2 98.5% on room air.');
    await page.locator('#clear-mega-btn').click();

    await expect(paddingtonCard).toHaveAttribute(
      'data-status',
      'UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED'
    );
    await expect(page.locator('#mega-clearance-banner')).toBeVisible();
    await expect(page.locator('#clearance-summary-text')).toContainText('Paddington');

    // Step 7: Filter by UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED
    await page
      .locator('#mega-status-filter')
      .selectOption('UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED');
    await expect(
      page.locator('#mega-operations-grid .mega-op-card', { hasText: 'Paddington (' })
    ).toBeVisible();
    await expect(
      page.locator('#mega-operations-grid .mega-op-card', { hasText: 'Pippin (' })
    ).toBeVisible();

    await page.locator('#mega-status-filter').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
