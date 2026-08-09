import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

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
    expect(Array.isArray(body)).toBe(true);
    expect(body.length).toBeGreaterThan(0);
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
      senderEmail: 'owner-pw@example.com',
      message: 'Hello, I found your pet!',
    };

    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/reunions/contact`, { data: payload });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.status).toBe('sent');
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

  test('should extract visual features via POST /api/v1/found-pets/extract-features', async ({ request }) => {
    const payload = { imageUrl: 'https://storage.petspotr.io/found-test.jpg' };
    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/found-pets/extract-features`, { data: payload });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.breed).toBeDefined();
    expect(body.primaryColor).toBeDefined();
  });

  test('should return push notification test payload via POST /api/v1/push/test', async ({ request }) => {
    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/push/test`);
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.title).toContain('Match');
    expect(body.url).toBe('/matches');
  });

  test('should serve PWA service worker script via GET /sw.js with correct Content-Type', async ({ request }) => {
    const response = await request.get(`${WEB_FRONTEND_URL}/sw.js`);
    expect(response.status()).toBe(200);
    const contentType = response.headers()['content-type'];
    expect(contentType).toContain('javascript');
  });

  test('should render HTML pages and DOM elements in browser', async ({ page }) => {
    const pages = ['/', '/report-lost', '/report-found', '/matches'];
    for (const pagePath of pages) {
      const response = await page.goto(`${WEB_FRONTEND_URL}${pagePath}`);
      expect(response?.status()).toBe(200);
      await expect(page.locator('body')).toBeVisible();
    }
  });

  test('should reject lost pet report with missing email via POST /api/v1/lost-pets', async ({ request }) => {
    const payload = { petName: 'NoEmail', location: 'Seattle, WA' };
    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, { data: payload });
    expect(response.status()).toBe(400);
  });

  test('should reject found pet report missing imageUrl via POST /api/v1/found-pets', async ({ request }) => {
    const payload = { location: 'Seattle, WA' };
    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/found-pets`, { data: payload });
    expect(response.status()).toBe(400);
  });

  test('should reject reunion contact missing message via POST /api/v1/reunions/contact', async ({ request }) => {
    const payload = { matchId: 'm-1', senderEmail: 'test@example.com' };
    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/reunions/contact`, { data: payload });
    expect(response.status()).toBe(400);
  });

  test('should reject reunion resolve missing matchId via POST /api/v1/reunions/resolve', async ({ request }) => {
    const payload = { petId: 'p-1', rating: 5 };
    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/reunions/resolve`, { data: payload });
    expect(response.status()).toBe(400);
  });

  test('should support pagination query params on GET /api/v1/lost-pets?limit=1&offset=0', async ({ request }) => {
    const response = await request.get(`${WEB_FRONTEND_URL}/api/v1/lost-pets?limit=1&offset=0`);
    expect(response.status()).toBe(200);
    expect(response.headers()['x-total-count']).toBeDefined();
    const data = await response.json();
    expect(Array.isArray(data)).toBe(true);
    expect(data.length).toBeLessThanOrEqual(1);
  });

  test('should support species and spatial radius filtering on GET /api/v1/found-pets', async ({ request }) => {
    const response = await request.get(`${WEB_FRONTEND_URL}/api/v1/found-pets?species=Dog&lat=47.6150&lng=-122.3200&radiusMiles=10`);
    expect(response.status()).toBe(200);
    expect(response.headers()['x-total-count']).toBeDefined();
    const data = await response.json();
    expect(Array.isArray(data)).toBe(true);
  });
});
