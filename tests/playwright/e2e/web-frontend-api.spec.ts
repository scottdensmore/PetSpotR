import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8080';

test.describe('API Journey: Web Frontend HTTP Endpoints', () => {
  test('should return 200 OK on GET /healthz', async ({ request }) => {
    const response = await request.get(`${WEB_FRONTEND_URL}/healthz`);
    expect(response.status()).toBe(200);
    const text = await response.text();
    expect(text).toContain('OK');
  });

  test('should return Prometheus metrics on GET /metrics', async ({ request }) => {
    const response = await request.get(`${WEB_FRONTEND_URL}/metrics`);
    expect(response.status()).toBe(200);
    const text = await response.text();
    expect(text).toContain('http_requests_total');
  });

  test('should handle Lost Pet submission via POST /api/v1/lost-pets', async ({ request }) => {
    const payload = {
      petId: `lost-pw-api-${Date.now()}`,
      reporterEmail: 'owner-pw@example.com',
      reportedAt: new Date().toISOString(),
      location: 'Capitol Hill, Seattle, WA',
    };

    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, { data: payload });
    expect(response.status()).toBe(201);
    const body = await response.json();
    expect(body.status).toBe('success');
  });

  test('should handle Found Pet submission via POST /api/v1/found-pets', async ({ request }) => {
    const payload = {
      petId: `found-pw-api-${Date.now()}`,
      imageUrl: 'https://storage.petspotr.io/images/found-pw.jpg',
      foundAt: new Date().toISOString(),
      location: 'Capitol Hill, Seattle, WA',
    };

    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/found-pets`, { data: payload });
    expect(response.status()).toBe(201);
    const body = await response.json();
    expect(body.status).toBe('success');
  });

  test('should return candidate match lists via GET /api/v1/matches', async ({ request }) => {
    const response = await request.get(`${WEB_FRONTEND_URL}/api/v1/matches`);
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(Array.isArray(body.matches)).toBe(true);
  });

  test('should process user match confirmation via POST /api/v1/matches/action', async ({ request }) => {
    const payload = { matchId: 'match-pw-101', action: 'confirm' };
    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/matches/action`, { data: payload });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.status).toBe('CONFIRMED');
  });

  test('should process owner contact initiation via POST /api/v1/reunions/contact', async ({ request }) => {
    const payload = {
      matchId: 'match-pw-101',
      recipientEmail: 'owner-pw@example.com',
      message: 'Hello, I found your pet!',
      channel: 'email',
    };

    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/reunions/contact`, { data: payload });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.status).toBe('SENT');
  });

  test('should process reunion resolution via POST /api/v1/reunions/resolve', async ({ request }) => {
    const payload = {
      matchId: 'match-pw-101',
      petId: 'lost-pw-api-1',
      rating: 5,
      feedback: 'Reunited quickly!',
    };

    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/reunions/resolve`, { data: payload });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.status).toBe('REUNITED');
    expect(body.rating).toBe(5);
  });

  test('should register Web Push subscription via POST /api/v1/push/subscribe', async ({ request }) => {
    const payload = {
      endpoint: 'https://fcm.googleapis.com/fcm/send/pw-sub-token',
      keys: { p256dh: 'key-data', auth: 'auth-data' },
    };

    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/push/subscribe`, { data: payload });
    expect(response.status()).toBe(201);
    const body = await response.json();
    expect(body.status).toBe('subscribed');
  });

  test('should generate pre-signed upload URL via POST /api/v1/uploads/presigned-url', async ({ request }) => {
    const payload = { fileName: 'pet-pw.jpg', contentType: 'image/jpeg' };
    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/uploads/presigned-url`, { data: payload });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.uploadUrl).toBeDefined();
    expect(body.publicUrl).toBeDefined();
  });
});
