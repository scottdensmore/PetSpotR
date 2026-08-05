import { test, expect } from '@playwright/test';

interface SuccessResponse {
  status: string;
  petId: string;
}

interface ErrorResponse {
  error: string;
}

const FOUNDPET_SERVICE_URL = process.env.FOUNDPET_SERVICE_URL || 'http://localhost:8081';

test.describe('User Journey: Found Pet Report & Service Boundary', () => {
  test('should submit a found pet report with image URL and trigger processing pipeline', async ({ request }) => {
    const foundPetPayload = {
      petId: `found-pw-${Date.now()}`,
      imageUrl: 'https://storage.petspotr.io/images/found-pw.jpg',
      foundAt: new Date().toISOString(),
      location: 'Portland, OR',
    };

    const response = await request.post(`${FOUNDPET_SERVICE_URL}/foundPet`, {
      data: foundPetPayload,
    });

    expect(response.status()).toBe(201);
    const body = (await response.json()) as SuccessResponse;
    expect(body.status).toBe('success');
    expect(body.petId).toBe(foundPetPayload.petId);
  });

  test('should reject invalid found pet report with HTTP 400 Bad Request', async ({ request }) => {
    const invalidPayload = {
      petId: '',
      imageUrl: '',
    };

    const response = await request.post(`${FOUNDPET_SERVICE_URL}/foundPet`, {
      data: invalidPayload,
    });

    expect(response.status()).toBe(400);
    const body = (await response.json()) as ErrorResponse;
    expect(body.error).toBeDefined();
  });

  test('should reject HTTP GET requests with 405 Method Not Allowed', async ({ request }) => {
    const response = await request.get(`${FOUNDPET_SERVICE_URL}/foundPet`);

    expect(response.status()).toBe(405);
    const body = (await response.json()) as ErrorResponse;
    expect(body.error).toBe('Method not allowed');
  });
});
