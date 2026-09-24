import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Crisis Medical Triage Cockpit & Passport UI', () => {
  test('Triage cockpit renders species buttons, vitals HUD, and dynamic acuity tag', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#triage-species-dog')).toBeVisible();
    await expect(page.locator('#triage-species-cat')).toBeVisible();

    // Fill vitals
    await page.locator('#vitals-hr').fill('210');
    await page.locator('#vitals-rr').fill('65');
    await page.locator('#vitals-crt-3').click();
    await page.locator('#vitals-mm-cyanotic').click();

    // Assert dynamic category calculation
    const indicator = page.locator('#live-triage-indicator');
    await expect(indicator).toContainText('TRIAGE RED');
    await expect(indicator).toHaveClass(/badge-triage-red/);
  });

  test('Printable passport card displays QR code and critical allergy banner', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/p/sample-passport/passport`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('.passport-card')).toBeVisible();
    await expect(page.locator('.passport-qr-code')).toBeVisible();
    await expect(page.locator('.alert-allergy-critical')).toBeVisible();
    await expect(page.locator('.alert-allergy-critical')).toContainText(/Penicillin/i);
  });

  test('Calculates live resuscitation dosages and updates upon species / weight change', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#triage-weight').fill('20');
    await page.locator('#triage-species-dog').click();

    // 20 kg Dog: 20 * 15 = 300 mL fluids, 0.20 mg epi
    await expect(page.locator('#dosage-fluids')).toContainText('300 mL');
    await expect(page.locator('#dosage-epinephrine')).toContainText('0.20 mg');

    // Switch to Cat: 20 * 7.5 = 150 mL fluids
    await page.locator('#triage-species-cat').click();
    await expect(page.locator('#dosage-fluids')).toContainText('150 mL');
  });

  test('Submits triage assessment and appends patient card to active triage stream', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/triage`);
    await page.waitForLoadState('networkidle');

    const testPetId = `pet-unit-${Date.now()}`;
    await page.locator('#triage-species-dog').click();
    await page.locator('#triage-pet-id').fill(testPetId);
    await page.locator('#triage-weight').fill('15');
    await page.locator('#vitals-hr').fill('220');
    await page.locator('#vitals-rr').fill('65');
    await page.locator('#vitals-crt-3').click();
    await page.locator('#vitals-mm-cyanotic').click();

    await page.locator('#btn-submit-triage').click();

    const patientCard = page.locator(`.patient-card[data-pet-id="${testPetId}"]`);
    await expect(patientCard).toBeVisible();
    await expect(patientCard.locator('.badge-triage-red')).toBeVisible();
  });
});
