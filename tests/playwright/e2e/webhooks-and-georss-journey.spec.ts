import { test, expect } from '@playwright/test';
import http from 'node:http';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8102';

test.describe.serial('User Journey: Webhooks & GeoRSS Feeds', () => {
  let receiverServer: http.Server;
  let receiverPort: number;
  let receiverUrl: string;
  const receivedRequests: Array<{ method?: string; url?: string; headers: http.IncomingHttpHeaders; body: string }> = [];

  const timestamp = Date.now();
  const testPetId = `lost-georss-${timestamp}`;
  let createdWebhookId: string;
  let unmaskedSecret: string;

  test.beforeAll(async ({ request }) => {
    // 1. Start local receiver for webhook delivery
    receiverServer = http.createServer((req, res) => {
      let body = '';
      req.on('data', chunk => {
        body += chunk;
      });
      req.on('end', () => {
        receivedRequests.push({
          method: req.method,
          url: req.url,
          headers: req.headers,
          body,
        });
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ status: 'received' }));
      });
    });

    await new Promise<void>((resolve, reject) => {
      receiverServer.listen(0, '127.0.0.1', () => {
        const addr = receiverServer.address();
        if (addr && typeof addr === 'object') {
          receiverPort = addr.port;
          receiverUrl = `http://127.0.0.1:${receiverPort}/webhook-receiver`;
          resolve();
        } else {
          reject(new Error('Failed to bind receiver server address'));
        }
      });
    });

    // 2. Seed a lost pet report with explicit coordinates to populate GeoRSS points
    const lostPetResp = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId: testPetId,
        petName: 'Barnaby',
        species: 'Dog',
        breed: 'Golden Retriever',
        primaryColor: 'Golden',
        description: 'Friendly golden retriever lost near Capitol Hill',
        reporterEmail: 'owner-journey@example.com',
        location: 'Capitol Hill, Seattle, WA',
        coordinates: {
          latitude: 47.61,
          longitude: -122.33,
        },
      },
    });
    expect(lostPetResp.status()).toBe(201);

    // 3. Seed a community sighting for this lost pet
    const sightingResp = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${testPetId}/sightings`, {
      data: {
        locationDescription: 'Cal Anderson Park',
        sightedAt: new Date().toISOString(),
        coordinates: {
          latitude: 47.615,
          longitude: -122.32,
        },
        movementDirection: 'Heading north toward Broadway',
        notes: 'Spotted running across the grass',
      },
    });
    expect(sightingResp.status()).toBe(201);
  });

  test.afterAll(async () => {
    if (receiverServer) {
      await new Promise<void>(resolve => receiverServer.close(() => resolve()));
    }
  });

  test('Test 1: Register a new webhook subscription via POST /api/v1/webhooks', async ({ request }) => {
    const resp = await request.post(`${WEB_FRONTEND_URL}/api/v1/webhooks`, {
      data: {
        targetUrl: receiverUrl,
        filterEvents: ['pet_lost'],
        geoFence: {
          centerLat: 47.61,
          centerLng: -122.33,
          radiusMiles: 15.0,
        },
      },
    });

    expect(resp.status()).toBe(201);
    const body = await resp.json();
    expect(body.id).toBeDefined();
    createdWebhookId = body.id;
    expect(body.targetUrl).toBe(receiverUrl);
    expect(body.filterEvents).toEqual(['pet_lost']);
    expect(body.active).toBe(true);
    expect(body.secret).toBeDefined();
    expect(body.secret).not.toBe('••••••••');
    expect(body.secret.length).toBeGreaterThanOrEqual(16);
    unmaskedSecret = body.secret;
  });

  test('Test 2: List webhooks via GET /api/v1/webhooks', async ({ request }) => {
    const resp = await request.get(`${WEB_FRONTEND_URL}/api/v1/webhooks`);
    expect(resp.status()).toBe(200);
    const list = await resp.json();
    expect(Array.isArray(list)).toBe(true);

    const sub = list.find((item: any) => item.id === createdWebhookId);
    expect(sub).toBeDefined();
    expect(sub.targetUrl).toBe(receiverUrl);
    expect(sub.secret).toBe('••••••••');
  });

  test('Test 3: Test ping dispatch via POST /api/v1/webhooks/{id}/test', async ({ request }) => {
    const resp = await request.post(`${WEB_FRONTEND_URL}/api/v1/webhooks/${createdWebhookId}/test`);
    expect(resp.status()).toBe(200);
    const delivery = await resp.json();
    expect(delivery.id).toBeDefined();
    expect(delivery.subscriptionId).toBe(createdWebhookId);
    expect(delivery.eventType).toBe('ping');
    expect(delivery.success).toBe(true);
    expect(delivery.statusCode).toBe(200);
    expect(delivery.responseTimeMs).toBeGreaterThanOrEqual(0);

    // Assert the receiver captured the HTTP request signed with unmasked secret
    expect(receivedRequests.length).toBeGreaterThanOrEqual(1);
    const lastReq = receivedRequests[receivedRequests.length - 1];
    expect(lastReq.headers['x-petspotr-signature']).toBeDefined();
    const payload = JSON.parse(lastReq.body);
    expect(payload.event).toBe('ping');
    expect(payload.subscriptionId).toBe(createdWebhookId);
  });

  test('Test 4: Verify GeoRSS Atom feed GET /feeds/lost-pets.atom', async ({ request }) => {
    const resp = await request.get(`${WEB_FRONTEND_URL}/feeds/lost-pets.atom`);
    expect(resp.status()).toBe(200);
    const contentType = resp.headers()['content-type'] || '';
    expect(contentType).toContain('application/atom+xml');

    const text = await resp.text();
    expect(text).toContain('<feed xmlns="http://www.w3.org/2005/Atom" xmlns:georss="http://www.georss.org/georss">');
    expect(text.replace(/\s+/g, '')).toContain('<author><name>PetSpotR</name></author>');
    expect(text).toContain('<georss:point>');
    expect(text).toContain('47.61 -122.33');
  });

  test('Test 5: Verify GeoRSS Atom feed with proximity query parameters', async ({ request }) => {
    // In-proximity request (Seattle) includes the lost pet
    const matchingResp = await request.get(
      `${WEB_FRONTEND_URL}/feeds/lost-pets.atom?lat=47.61&lng=-122.33&radiusMiles=10`
    );
    expect(matchingResp.status()).toBe(200);
    const matchingContentType = matchingResp.headers()['content-type'] || '';
    expect(matchingContentType).toContain('application/atom+xml');
    const matchingText = await matchingResp.text();
    expect(matchingText).toContain('<feed xmlns="http://www.w3.org/2005/Atom" xmlns:georss="http://www.georss.org/georss">');
    expect(matchingText).toContain(testPetId);

    // Far-away request (Portland) excludes the Seattle pet
    const nonMatchingResp = await request.get(
      `${WEB_FRONTEND_URL}/feeds/lost-pets.atom?lat=45.5152&lng=-122.6784&radiusMiles=10`
    );
    expect(nonMatchingResp.status()).toBe(200);
    const nonMatchingText = await nonMatchingResp.text();
    expect(nonMatchingText).not.toContain(testPetId);
  });

  test('Test 6: Verify community sightings feed GET /feeds/sightings.atom', async ({ request }) => {
    const resp = await request.get(`${WEB_FRONTEND_URL}/feeds/sightings.atom`);
    expect(resp.status()).toBe(200);
    const contentType = resp.headers()['content-type'] || '';
    expect(contentType).toContain('application/atom+xml');

    const text = await resp.text();
    expect(text).toContain('<feed xmlns="http://www.w3.org/2005/Atom" xmlns:georss="http://www.georss.org/georss">');
    expect(text).toContain('<title>PetSpotR - Community Sightings</title>');
    expect(text).toContain('<georss:point>');
    expect(text).toContain('47.615 -122.32');
  });

  test('Test 7: Delete webhook subscription via DELETE /api/v1/webhooks/{id}', async ({ request }) => {
    const delResp = await request.delete(`${WEB_FRONTEND_URL}/api/v1/webhooks/${createdWebhookId}`);
    expect(delResp.status()).toBe(204);

    // Assert webhook no longer returned in list
    const listResp = await request.get(`${WEB_FRONTEND_URL}/api/v1/webhooks`);
    expect(listResp.status()).toBe(200);
    const list = await listResp.json();
    const found = list.find((item: any) => item.id === createdWebhookId);
    expect(found).toBeUndefined();

    // Subsequent test ping returns 404
    const testResp = await request.post(`${WEB_FRONTEND_URL}/api/v1/webhooks/${createdWebhookId}/test`);
    expect(testResp.status()).toBe(404);
  });
});
