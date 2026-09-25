import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe.serial('Milestone 11.4: Emergency Veterinary Passport & Crisis Triage Journey', () => {
  const testPetID = `pet-triage-${Date.now()}`;
  let qrPayload = '';
  let assessmentID = '';

  test('Step 1: Issue Emergency Veterinary Passport via API with Rabies and Allergy', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/veterinary/passports`, {
      data: {
        petId: testPetID,
        petName: 'Rusty',
        species: 'Dog',
        breed: 'Golden Retriever',
        microchipId: '985141000998811',
        rabiesTagID: 'RAB-2026-X1',
        weightKg: 30.0,
        vaccinations: [
          { vaccineName: 'Rabies 3-Yr', expirationDate: '2027-09-01T00:00:00Z', verified: true },
        ],
        allergies: [
          { allergen: 'Penicillin', severity: 'ANAPHYLACTIC', reactionDescription: 'Anaphylaxis' },
        ],
      },
    });

    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.passportId).toBeTruthy();
    expect(body.qrPayload).toBeTruthy();
    expect(body.qrDataUri).toContain('data:image/png;base64,');
    qrPayload = body.qrPayload;
  });

  test('Step 2: Verify Offline Passport Payload via POST /api/v1/veterinary/passports/verify', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/veterinary/passports/verify`, {
      data: { qrPayload },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.verified).toBe(true);
    expect(body.passport.petName).toBe('Rusty');
    expect(body.passport.allergies.length).toBe(1);
    expect(body.passport.allergies[0].allergen).toBe('Penicillin');
  });

  test('Step 3: Open /triage Cockpit, input Critical Vitals, assert TRIAGE RED', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#triage-species-dog').click();
    await page.locator('#triage-pet-id').fill(testPetID);
    await page.locator('#triage-weight').fill('30');
    await page.locator('#vitals-hr').fill('215');
    await page.locator('#vitals-rr').fill('65');
    await page.locator('#vitals-crt-3').click();
    await page.locator('#vitals-mm-cyanotic').click();

    const indicator = page.locator('#live-triage-indicator');
    await expect(indicator).toContainText('TRIAGE RED');
  });

  test('Step 4: Submit Triage Assessment and Assert Queue Addition with Red Badge', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/triage`);
    await page.waitForLoadState('networkidle');

    await page.locator('#triage-species-dog').click();
    await page.locator('#triage-pet-id').fill(testPetID);
    await page.locator('#triage-weight').fill('30');
    await page.locator('#vitals-hr').fill('215');
    await page.locator('#vitals-rr').fill('65');
    await page.locator('#vitals-crt-3').click();
    await page.locator('#vitals-mm-cyanotic').click();

    await page.locator('#btn-submit-triage').click();

    const patientCard = page.locator(`.patient-card[data-pet-id="${testPetID}"]`);
    await expect(patientCard).toBeVisible();
    await expect(patientCard.locator('.badge-triage-red')).toBeVisible();

    const idAttr = await patientCard.getAttribute('data-assessment-id');
    expect(idAttr).toBeTruthy();
    assessmentID = idAttr!;
  });

  test('Step 5: Administer Emergency IV Shock Fluid and Pain Treatment', async ({ request, page }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/veterinary/triage/${assessmentID}/treatments`, {
      data: {
        medicationName: 'Lactated Ringers Solution Bolus',
        dosage: '450 mL',
        route: 'IV',
        administeredBy: 'Dr. Aris Thorne',
        notes: 'Administered over 15 minutes for hypovolemic shock',
      },
    });
    expect(res.status()).toBe(200);

    await page.goto(`${WEB_FRONTEND_URL}/triage`);
    const patientCard = page.locator(`.patient-card[data-pet-id="${testPetID}"]`);
    await patientCard.click();
    await expect(page.locator('.treatment-log')).toContainText('Lactated Ringers Solution Bolus');
  });

  test('Step 6: Verify Printable Passport Card layout, QR code, and Critical Allergy Alert', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/p/${testPetID}/passport`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('.passport-card')).toBeVisible();
    await expect(page.locator('.passport-qr-code')).toBeVisible();
    const alertBox = page.locator('.alert-allergy-critical');
    await expect(alertBox).toBeVisible();
    await expect(alertBox).toContainText(/Penicillin/i);
  });
});
