import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe.serial('User Journey: SMS Alert Preferences & Two-Way Interactive Dispatch', () => {
  const petId = `lost-sms-journey-${Date.now()}`;
  const petName = 'Rusty';
  const testPhone = '+12065550199';

  test.beforeAll(async ({ request }) => {
    // 1. Seed lost pet report
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId,
        petName,
        species: 'Dog',
        breed: 'Golden Retriever',
        primaryColor: 'Golden',
        description: 'Friendly golden retriever participating in SMS dispatch journey test.',
        location: 'Capitol Hill, Seattle, WA',
        reporterEmail: 'rusty-owner@example.com',
        phone: testPhone,
        coordinates: {
          latitude: 47.6150,
          longitude: -122.3200,
        },
      },
    });
    expect([200, 201]).toContain(res.status());

    // 2. Initialize search party
    const partyRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/search-party`, {
      data: { sectorCount: 4 },
    });
    expect([200, 201]).toContain(partyRes.status());
  });

  test('Step 1: Open Notification Preferences, toggle SMS alert, enter phone, and verify via UI and API', async ({ page, request }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    // Open notification drawer
    const drawerBtn = page.locator('#btn-notification-drawer');
    await expect(drawerBtn).toBeVisible();
    await drawerBtn.click();

    // Open preferences modal
    const prefBtn = page.locator('#btn-open-preferences');
    await expect(prefBtn).toBeVisible();
    await prefBtn.click();

    // Modal is visible
    const modal = page.locator('#modal-alert-preferences');
    await expect(modal).toBeVisible();

    // SMS toggle and phone input inside #modal-alert-preferences
    const smsToggle = modal.locator('#pref-sms-enabled');
    const phoneInput = modal.locator('#pref-phone-number');
    const verifyBtn = modal.locator('#btn-verify-sms');

    await expect(smsToggle).toBeVisible();
    await smsToggle.check();
    await expect(smsToggle).toBeChecked();

    await expect(phoneInput).toBeVisible();
    await phoneInput.fill(testPhone);
    await expect(phoneInput).toHaveValue(testPhone);

    // Click verify SMS button
    await expect(verifyBtn).toBeVisible();
    await verifyBtn.click();

    // Feedback message is shown
    const feedback = modal.locator('#pref-sms-feedback');
    await expect(feedback).toBeVisible();
    await expect(feedback).toContainText('Verification');

    // Also verify the verification endpoint directly via API
    const subRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/sms/subscribe`, {
      data: { phone: testPhone },
    });
    expect(subRes.status()).toBe(200);
    const subData = await subRes.json();
    expect(subData.status).toBe('success');
    expect(subData.phone).toBe(testPhone);
  });

  test('Step 2: Two-Way SMS Command STATUS returns active search party and open sectors', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/webhooks/sms/inbound`, {
      data: {
        From: testPhone,
        Body: 'STATUS',
        petId: petId,
      },
    });
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data.status).toBe('success');
    expect(data.command).toBe('STATUS');
    expect(data.reply).toContain('Search Active');
    expect(data.reply).toContain('sector-1');
  });

  test('Step 3: Two-Way SMS Command CLAIM claims a sector and syncs with search party state', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/webhooks/sms/inbound`, {
      data: {
        From: testPhone,
        Body: 'CLAIM sector-1',
        petId: petId,
      },
    });
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data.status).toBe('success');
    expect(data.command).toBe('CLAIM');
    expect(data.reply).toContain('sector-1 claimed!');
    expect(data.reply).toContain(petId);

    // Verify search party state via API
    const partyRes = await request.get(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/search-party`);
    expect(partyRes.status()).toBe(200);
    const party = await partyRes.json();
    const sec1 = party.sectors.find((s: any) => s.sectorId === 'sector-1');
    expect(sec1).toBeDefined();
    expect(sec1.status).toBe('active_search');
    expect(party.activeAssignments.length).toBeGreaterThanOrEqual(1);

    // Duplicate claim returns already claimed
    const dupRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/webhooks/sms/inbound`, {
      data: {
        From: testPhone,
        Body: 'CLAIM sector-1',
        petId: petId,
      },
    });
    expect(dupRes.status()).toBe(200);
    const dupData = await dupRes.json();
    expect(dupData.reply.toLowerCase()).toContain('already claimed');
  });

  test('Step 4: Two-Way SMS Command SIGHTED records community sighting and coordinates', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/webhooks/sms/inbound`, {
      data: {
        From: testPhone,
        Body: 'SIGHTED Near 15th Ave E & Republican St running north',
        petId: petId,
      },
    });
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data.status).toBe('success');
    expect(data.command).toBe('SIGHTED');
    expect(data.reply).toContain('Sighting recorded');

    // Verify sightings endpoint has recorded the sighting
    const sightRes = await request.get(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/sightings`);
    expect(sightRes.status()).toBe(200);
    const sightings = await sightRes.json();
    expect(Array.isArray(sightings)).toBe(true);
    expect(sightings.length).toBeGreaterThanOrEqual(1);
    const logged = sightings.find((s: any) => s.locationDescription.includes('15th Ave'));
    expect(logged).toBeDefined();
  });

  test('Step 5: Two-Way SMS Command STOP and START handle opt-out compliance', async ({ request }) => {
    // STOP
    const stopRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/webhooks/sms/inbound`, {
      data: {
        From: testPhone,
        Body: 'STOP',
      },
    });
    expect(stopRes.status()).toBe(200);
    const stopData = await stopRes.json();
    expect(stopData.reply.toLowerCase()).toContain('unsubscribed');

    // START
    const startRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/webhooks/sms/inbound`, {
      data: {
        From: testPhone,
        Body: 'START',
      },
    });
    expect(startRes.status()).toBe(200);
    const startData = await startRes.json();
    expect(startData.reply.toLowerCase()).toContain('resubscribed');
  });
});
