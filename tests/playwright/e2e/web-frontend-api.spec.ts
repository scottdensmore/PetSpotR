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

  test('should reuse the lost report identity after a transient browser retry', async ({ page }) => {
    const submissions: Array<Record<string, unknown>> = [];
    await page.route('**/api/v1/lost-pets', async (route) => {
      submissions.push(route.request().postDataJSON() as Record<string, unknown>);
      if (submissions.length === 1) {
        await route.fulfill({ status: 503, body: 'temporarily unavailable' });
        return;
      }
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'success', petId: submissions[1].petId }),
      });
    });
    page.on('dialog', (dialog) => dialog.accept());

    await page.goto(`${WEB_FRONTEND_URL}/report-lost`);
    await page.locator('#petName').fill('Buddy');
    await page.locator('#btn-next').click();
    await page.locator('#btn-next').click();
    await page.locator('#location').fill('Seattle, WA');
    await page.locator('#btn-next').click();
    await page.locator('#reporterEmail').fill('owner@example.com');

    await page.locator('#btn-submit').click();
    await expect.poll(() => submissions.length).toBe(1);
    await page.locator('#btn-submit').click();
    await expect(page.locator('#success-modal')).toBeVisible();

    expect(submissions).toHaveLength(2);
    expect(submissions[0].petId).toMatch(/^lost-[0-9a-f-]+$/);
    expect(submissions[1].petId).toBe(submissions[0].petId);
    expect(submissions[1].reportedAt).toBe(submissions[0].reportedAt);
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

  test('should reuse the found report identity after a transient browser retry', async ({ page }) => {
    const submissions: Array<Record<string, unknown>> = [];
    await page.route('**/api/v1/found-pets', async (route) => {
      submissions.push(route.request().postDataJSON() as Record<string, unknown>);
      if (submissions.length === 1) {
        await route.fulfill({ status: 503, body: 'temporarily unavailable' });
        return;
      }
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'success', petId: submissions[1].petId }),
      });
    });
    page.on('dialog', (dialog) => dialog.accept());

    await page.goto(`${WEB_FRONTEND_URL}/report-found`);
    await page.locator('#foundLocation').fill('Seattle, WA');
    await page.locator('#finderEmail').fill('finder@example.com');

    await page.locator('#found-pet-form button[type="submit"]').click();
    await expect.poll(() => submissions.length).toBe(1);
    await page.locator('#found-pet-form button[type="submit"]').click();
    await expect(page.locator('#found-success-modal')).toBeVisible();

    expect(submissions).toHaveLength(2);
    expect(submissions[0].petId).toMatch(/^found-[0-9a-f-]+$/);
    expect(submissions[1].petId).toBe(submissions[0].petId);
    expect(submissions[1].foundAt).toBe(submissions[0].foundAt);
  });

  test('should return candidate match lists via GET /api/v1/matches', async ({ request }) => {
    const response = await request.get(`${WEB_FRONTEND_URL}/api/v1/matches`);
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(Array.isArray(body)).toBe(true);
  });

  test('should reject match confirmation for an unknown match', async ({ request }) => {
    const payload = { matchId: 'match-pw-101', action: 'confirm' };
    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/matches/action`, { data: payload });
    expect(response.status()).toBe(404);
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

  test('should reject reunion resolution for an unknown match', async ({ request }) => {
    const payload = {
      matchId: 'match-pw-101',
      petId: 'lost-pw-api-1',
      rating: 5,
      feedback: 'Reunited quickly!',
    };

    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/reunions/resolve`, { data: payload });
    expect(response.status()).toBe(404);
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

  test('should keep home navigation actions visible and keyboard accessible on phones', async ({ page, context }) => {
    await context.clearPermissions();
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`${WEB_FRONTEND_URL}/`);

    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);

    const actionIds = ['theme-toggle', 'btn-enable-push', 'btn-report-lost', 'btn-report-found'];
    for (const id of actionIds) {
      const action = page.locator(`#${id}`);
      await expect(action).toBeVisible();
      const box = await action.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(390);
    }

    await page.locator('#brand-link').focus();
    const keyboardActionIds: string[] = [];
    for (const id of actionIds) {
      if (!(await page.locator(`#${id}`).isDisabled())) keyboardActionIds.push(id);
    }
    for (const id of keyboardActionIds) {
      await page.keyboard.press('Tab');
      await expect(page.locator(`#${id}`)).toBeFocused();
    }
  });

  test('should keep the complete home header visible through the desktop boundary', async ({ page, context }) => {
    await context.clearPermissions();

    const headerIds = [
      'brand-link',
      'nav-home',
      'nav-directory',
      'nav-matches',
      'theme-toggle',
      'btn-enable-push',
      'btn-report-lost',
      'btn-report-found',
    ];

    for (const width of [769, 900, 1100, 1101, 1119, 1120, 1280]) {
      await page.setViewportSize({ width, height: 900 });
      await page.goto(`${WEB_FRONTEND_URL}/`);

      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      const headerBox = await page.locator('.glass-nav').boundingBox();
      expect(headerBox).not.toBeNull();

      for (const id of headerIds) {
        const item = page.locator(`#${id}`);
        await expect(item).toBeVisible();
        const box = await item.boundingBox();
        expect(box).not.toBeNull();
        expect(box!.x).toBeGreaterThanOrEqual(0);
        expect(box!.x + box!.width).toBeLessThanOrEqual(width);
        expect(box!.y).toBeGreaterThanOrEqual(headerBox!.y);
        expect(box!.y + box!.height).toBeLessThanOrEqual(headerBox!.y + headerBox!.height);
      }
    }

    await page.setViewportSize({ width: 769, height: 900 });
    await page.goto(`${WEB_FRONTEND_URL}/`);
    await page.locator('#brand-link').focus();

    for (const id of headerIds.slice(1)) {
      if (await page.locator(`#${id}`).isDisabled()) continue;
      await page.keyboard.press('Tab');
      await expect(page.locator(`#${id}`)).toBeFocused();
    }
  });

  test('should enforce the browser security policy without breaking frontend journeys', async ({ page }) => {
    const expectedCSP = "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob: https://storage.petspotr.io; connect-src 'self'; worker-src 'self'";
    const cspViolations: string[] = [];
    page.on('console', (message) => {
      if (message.type() === 'error' && message.text().includes('Content Security Policy')) {
        cspViolations.push(message.text());
      }
    });

    for (const pagePath of ['/', '/report-lost', '/report-found', '/matches']) {
      const response = await page.goto(`${WEB_FRONTEND_URL}${pagePath}`);
      expect(response?.headers()['content-security-policy']).toBe(expectedCSP);
      expect(response?.headers()['x-content-type-options']).toBe('nosniff');
      expect(response?.headers()['referrer-policy']).toBe('no-referrer');
      expect(response?.headers()['permissions-policy']).toBe('camera=(), geolocation=(), microphone=()');
      expect(response?.headers()['x-frame-options']).toBe('DENY');
      await expect(page.locator('body')).toBeVisible();
      await expect(page.locator('[style], [onclick], [onsubmit]')).toHaveCount(0);
    }

    await page.evaluate(() => {
      const script = document.createElement('script');
      script.textContent = "document.body.dataset.inlineScript = 'executed'";
      document.body.append(script);
    });
    await expect(page.locator('body')).not.toHaveAttribute('data-inline-script', 'executed');

    expect(cspViolations).toHaveLength(1);
    expect(cspViolations[0]).toContain('script-src');
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
