import { test, expect } from '@playwright/test';

interface SuccessResponse {
  status: string;
  petId: string;
}

interface ErrorResponse {
  error: string;
}

const LOSTPET_SERVICE_URL = process.env.LOSTPET_SERVICE_URL || process.env.BASE_URL || 'http://localhost:8080';

test.describe('User Journey: Lost Pet Submission', () => {
  test('should successfully post a lost pet report and receive HTTP 201 status', async ({ request }) => {
    const lostPetPayload = {
      petId: `lost-pw-${Date.now()}`,
      reporterEmail: 'owner-playwright@example.com',
      reportedAt: new Date().toISOString(),
      location: 'Portland, OR',
    };

    const response = await request.post(`${LOSTPET_SERVICE_URL}/lostPet`, {
      data: lostPetPayload,
    });

    expect(response.status()).toBe(201);
    const body = (await response.json()) as SuccessResponse;
    expect(body.status).toBe('success');
    expect(body.petId).toBe(lostPetPayload.petId);
  });

  test('should reject invalid lost pet report with HTTP 400 Bad Request', async ({ request }) => {
    const invalidPayload = {
      petId: '',
      reporterEmail: '',
    };

    const response = await request.post(`${LOSTPET_SERVICE_URL}/lostPet`, {
      data: invalidPayload,
    });

    expect(response.status()).toBe(400);
    const body = (await response.json()) as ErrorResponse;
    expect(body.error).toBeDefined();
  });
});
