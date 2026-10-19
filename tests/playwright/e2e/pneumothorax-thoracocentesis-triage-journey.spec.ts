import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Tension Pneumothorax, Flail Chest, 7th–9th ICS Thoracocentesis & Heimlich/Pleural Seal Chest Tube Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new operation "Goose", staging 7th–9th ICS thoracocentesis & chest tube plan, logging pleural drainage check, issuing removal clearance, and filtering by PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/pneumothorax-thoracocentesis-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#pneumothorax-thoracocentesis-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Tension Pneumothorax, Flail Chest, 7th–9th ICS Thoracocentesis & Heimlich/Pleural Seal Chest Tube Cockpit'
    );

    await expect(
      page.locator('#pneumothorax-operations-list .pneumothorax-op-card', { hasText: 'Maverick (' })
    ).toBeVisible();
    await expect(
      page.locator('#pneumothorax-operations-list .pneumothorax-op-card', { hasText: 'Bandit (' })
    ).toBeVisible();
    await expect(
      page.locator('#pneumothorax-operations-list .pneumothorax-op-card', { hasText: 'Tango (' })
    ).toBeVisible();

    // Step 2: Create new operation "Goose"
    await page.locator('#create-incident-id').fill('INC-PTX-9914');
    await page.locator('#create-pet-name').fill('Goose');
    await page.locator('#create-species').selectOption('CANINE_SPONTANEOUS_BULLAE');
    await page.locator('#create-breed').fill('Alaskan Malamute');
    await page.locator('#create-weight-kg').fill('34.0');
    await page.locator('#create-resp-rate').fill('68');
    await page.locator('#create-spo2').fill('85.0');
    await page.locator('#create-cumulative-air').fill('54.0');
    await page.locator('#create-tap-count').fill('3');
    await page.locator('#btn-create-pneumothorax-op').click();

    const gooseCard = page
      .locator('#pneumothorax-operations-list .pneumothorax-op-card', { hasText: 'Goose (' })
      .first();
    await expect(gooseCard).toBeVisible();
    await expect(gooseCard).toHaveAttribute(
      'data-status',
      'TENSION_PNEUMOTHORAX_ABSENT_GLIDE_CRITICAL'
    );

    // Step 3: Stage 7th–9th ICS thoracocentesis & chest tube plan for Goose
    await page.locator('#stage-weight-kg').fill('34.0');
    await page.locator('#stage-species').selectOption('CANINE_SPONTANEOUS_BULLAE');
    await page.locator('#stage-resp-rate').fill('42');
    await page.locator('#stage-spo2').fill('94.0');
    await page.locator('#stage-cumulative-air').fill('54.0');
    await page.locator('#stage-tap-count').fill('3');
    await page.locator('#stage-glide-sign').check();
    await page.locator('#btn-stage-plan').click();

    await expect(gooseCard).toHaveAttribute(
      'data-status',
      'ICS_7_9_THORACOCENTESIS_CHEST_TUBE_STAGED'
    );
    await expect(page.locator('#plan-expected-tap')).toContainText('1020.0 mL');
    await expect(page.locator('#plan-chest-tube-indicated')).toContainText('YES');
    await expect(page.locator('#plan-blood-patch')).toContainText('5.0 mL/kg');

    // Step 4: Log serial pleural drainage & TFAST glide check for Goose
    await page.locator('#check-air-rate').fill('0.0');
    await page.locator('#check-fluid-rate').fill('1.3');
    await page.locator('#check-spo2').fill('97.5');
    await page.locator('#check-callsign').fill('DR-PULM-34');
    await page
      .locator('#check-notes')
      .fill('Zero air leak on -15 cmH2O continuous suction; bilateral TFAST glide sign restored.');
    await page.locator('#check-glide-restored').check();
    await page.locator('#btn-record-drainage-check').click();

    await expect(page.locator('#drainage-checks-list')).toContainText(
      'Zero air leak on -15 cmH2O continuous suction; bilateral TFAST glide sign restored.'
    );

    // Step 5: Issue Pleural Seal & Aseptic Chest Tube Removal Clearance for Goose
    await page.locator('#clearance-zero-air-hours').fill('26');
    await page.locator('#clearance-final-fluid').fill('1.2');
    await page.locator('#clearance-room-air-spo2').fill('97.5');
    await page.locator('#clearance-vet-license').fill('VET-PULM-99140');
    await page
      .locator('#clearance-summary')
      .fill('Zero air production for 26 hours; bilateral TFAST glide sign confirmed; chest tube aseptically removed.');
    await page.locator('#clearance-glide-confirmed').check();
    await page.locator('#clearance-tube-removed').check();
    await page.locator('#btn-issue-removal-clearance').click();

    await expect(gooseCard).toHaveAttribute(
      'data-status',
      'PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED'
    );
    await expect(page.locator('#pneumothorax-clearance-banner')).toBeVisible();
    await expect(page.locator('#pneumothorax-clearance-banner')).toContainText('VET-PULM-99140');

    // Step 6: Filter by PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED and verify Goose and Tango remain visible
    await page
      .locator('#filter-status')
      .selectOption('PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED');
    await expect(
      page.locator('#pneumothorax-operations-list .pneumothorax-op-card', { hasText: 'Goose (' })
    ).toBeVisible();
    await expect(
      page.locator('#pneumothorax-operations-list .pneumothorax-op-card', { hasText: 'Tango (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
