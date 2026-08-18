import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

function validMatchRecord(matchId: string) {
  return {
    matchId,
    foundPetId: `found-${matchId}`,
    matchedPetId: `lost-${matchId}`,
    score: 0.92,
    status: 'PENDING_REVIEW',
    matchedAt: '2026-08-14T12:30:00Z',
    scores: {
      visual: 0.95,
      color: 0.9,
      spatial: 0.88,
      distanceMiles: 2.4,
    },
    lostPet: {
      petId: `lost-${matchId}`,
      petName: 'Buddy',
      breed: 'Golden Retriever',
      imageUrl: 'https://storage.petspotr.io/lost-legitimate.jpg',
      location: 'Capitol Hill, Seattle, WA',
    },
    foundPet: {
      petId: `found-${matchId}`,
      breed: 'Golden Retriever',
      imageUrl: 'https://storage.petspotr.io/found-legitimate.jpg',
      location: 'Green Lake Park, Seattle, WA',
    },
  };
}

test.describe('Match dashboard persisted-data boundary', () => {
  test('requires Google sign-in before rendering participant matches', async ({ page }) => {
    const principal = {
      issuer: 'https://securetoken.google.com/demo-petspotr-auth',
      subject: 'google-match-participant-303',
      email: 'verified-participant@example.com',
      emailVerified: true,
      signInProvider: 'google.com',
    };
    let signedIn = false;
    let csrfCount = 0;
    let matchRequests = 0;

    await page.addInitScript(() => {
      const browserWindow = window as typeof window & {
        petspotrFirebaseAuthAdapterFactory?: () => Promise<{
          signInWithGoogle: () => Promise<string>;
          signOut: () => Promise<void>;
        }>;
      };
      browserWindow.petspotrFirebaseAuthAdapterFactory = async () => ({
        signInWithGoogle: async () => 'firebase-google-match-token',
        signOut: async () => {},
      });
    });
    await page.route('**/api/v1/session/client-config', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          enabled: true,
          provider: 'google.com',
          apiKey: 'fake-api-key',
          authDomain: 'demo-petspotr-auth.firebaseapp.com',
          projectId: 'demo-petspotr-auth',
        }),
      });
    });
    await page.route('**/api/v1/session/csrf', async (route) => {
      csrfCount += 1;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ csrfToken: `match-csrf-${csrfCount}` }),
      });
    });
    await page.route('**/api/v1/session', async (route) => {
      const request = route.request();
      if (request.method() === 'GET') {
        await route.fulfill(signedIn
          ? { status: 200, contentType: 'application/json', body: JSON.stringify(principal) }
          : { status: 401, body: 'Authentication required' });
        return;
      }
      if (request.method() === 'POST') {
        expect(request.headers()['x-csrf-token']).toBe('match-csrf-1');
        expect(request.postDataJSON()).toEqual({ idToken: 'firebase-google-match-token' });
        signedIn = true;
        await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(principal) });
        return;
      }
      expect(request.method()).toBe('DELETE');
      expect(request.headers()['x-csrf-token']).toBe('match-csrf-3');
      signedIn = false;
      await route.fulfill({ status: 204 });
    });
    await page.route('**/api/v1/matches', async (route) => {
      matchRequests += 1;
      expect(signedIn).toBe(true);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([validMatchRecord('participant-match')]),
      });
    });

    await page.goto(`${WEB_FRONTEND_URL}/matches`);
    await expect(page.locator('#identity-panel')).toBeVisible();
    await expect(page.locator('#identity-status')).toContainText('private matches');
    await expect(page.locator('#google-sign-in')).toBeVisible();
    await expect(page.locator('article[data-match-id]')).toHaveCount(0);
    expect(matchRequests).toBe(0);

    await page.locator('#google-sign-in').click();
    await expect(page.locator('#identity-status')).toContainText('verified-participant@example.com');
    await expect(page.locator('article[data-match-id="participant-match"]')).toBeVisible();
    await expect(page.locator('.action-btn, .contact-btn, .reunion-btn')).toHaveCount(0);
    expect(matchRequests).toBe(1);

    await page.reload();
    await expect(page.locator('article[data-match-id="participant-match"]')).toBeVisible();
    expect(matchRequests).toBe(2);
    await page.locator('#identity-sign-out').click();
    await expect(page.locator('article[data-match-id]')).toHaveCount(0);
    await expect(page.locator('#google-sign-in')).toBeFocused();
    expect(matchRequests).toBe(2);

    const unavailablePage = await page.context().newPage();
    let unavailableMatchRequests = 0;
    await unavailablePage.addInitScript(() => {
      const browserWindow = window as typeof window & {
        petspotrFirebaseAuthAdapterFactory?: () => Promise<never>;
      };
      browserWindow.petspotrFirebaseAuthAdapterFactory = async () => {
        throw new Error('adapter unavailable');
      };
    });
    await unavailablePage.route('**/api/v1/session/client-config', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          enabled: true,
          provider: 'google.com',
          apiKey: 'fake-api-key',
          authDomain: 'demo-petspotr-auth.firebaseapp.com',
          projectId: 'demo-petspotr-auth',
        }),
      });
    });
    await unavailablePage.route('**/api/v1/matches', async (route) => {
      unavailableMatchRequests += 1;
      await route.abort();
    });
    await unavailablePage.goto(`${WEB_FRONTEND_URL}/matches`);
    await expect(unavailablePage.locator('#identity-error')).toContainText('temporarily unavailable');
    await expect(unavailablePage.locator('article[data-match-id]')).toHaveCount(0);
    expect(unavailableMatchRequests).toBe(0);
    await unavailablePage.close();
  });

  test('renders hostile match fields as inert data and rejects unsafe attributes', async ({ page }) => {
    const marker = 'stored-match-injection';
    const textPayload = `<img id="${marker}" src="/missing" onerror="document.body.dataset.storedXss='executed'">`;
    const matchIdPayload = 'match-hostile-text" data-injected="true';
    const petIdPayload = 'lost-hostile-text" data-pet-injected="true';
    const longValue = 'legitimate value '.repeat(40);
    const requestedUnsafeImageURLs: string[] = [];
    page.on('request', (request) => {
      if (request.url().includes('attacker.example') || request.url().includes('attacker-pet-images')) {
        requestedUnsafeImageURLs.push(request.url());
      }
    });
    await page.setViewportSize({ width: 390, height: 844 });

    const hostileTextRecord = validMatchRecord(matchIdPayload);
    hostileTextRecord.matchedPetId = petIdPayload;
    hostileTextRecord.lostPet = {
      petId: petIdPayload,
      petName: textPayload,
      breed: `Golden "Retriever" ${textPayload}`,
      imageUrl: 'javascript:document.body.dataset.imageXss="executed"',
      location: `Capitol Hill </p>${textPayload}`,
    };
    hostileTextRecord.foundPet = {
      petId: 'found-hostile-text',
      breed: `Retriever ${textPayload}`,
      imageUrl: 'https://storage.googleapis.com/attacker-pet-images/tracking.png',
      location: `Green Lake ${textPayload}`,
    };

    const invalidScore = { ...validMatchRecord('invalid-score'), score: '0.92' };
    const invalidDate = { ...validMatchRecord('invalid-date'), matchedAt: 'not-a-date' };
    const invalidNestedScore = validMatchRecord('invalid-nested-score');
    invalidNestedScore.scores = { ...invalidNestedScore.scores, visual: '95%' } as unknown as typeof invalidNestedScore.scores;
    const invalidDistance = validMatchRecord('invalid-distance');
    invalidDistance.scores = { ...invalidDistance.scores, distanceMiles: -1 };
    const invalidStatus = { ...validMatchRecord('invalid-status'), status: '<img onerror=alert(1)>' };

    const longLegitimateRecord = validMatchRecord(`match-${'long-id-'.repeat(30)}`);
    longLegitimateRecord.lostPet.petName = longValue;
    longLegitimateRecord.lostPet.breed = longValue;
    longLegitimateRecord.lostPet.location = longValue;
    longLegitimateRecord.foundPet.breed = longValue;
    longLegitimateRecord.foundPet.location = longValue;

    const legitimateRecord = validMatchRecord('match-legitimate');

    await page.route('**/api/v1/matches', async (route) => {
      await route.fulfill({
        contentType: 'application/json',
        json: [
          hostileTextRecord,
          invalidScore,
          invalidDate,
          invalidNestedScore,
          invalidDistance,
          invalidStatus,
          longLegitimateRecord,
          legitimateRecord,
        ],
      });
    });

    await page.goto(`${WEB_FRONTEND_URL}/matches`);

    const hostileCard = page.locator('article').filter({ hasText: textPayload });
    await expect(hostileCard).toContainText(textPayload);
    await expect(hostileCard).toHaveAttribute('data-match-id', matchIdPayload);
    await expect(hostileCard.locator('.action-btn').first()).toHaveAttribute('data-match-id', matchIdPayload);
    await expect(page.locator(`#${marker}`)).toHaveCount(0);
    await expect(page.locator('[data-injected="true"]')).toHaveCount(0);
    await expect(page.locator('[data-pet-injected="true"]')).toHaveCount(0);
    await expect(page.locator('img[src^="javascript:"]')).toHaveCount(0);
    await expect(page.locator('img[src*="attacker-pet-images"]')).toHaveCount(0);
    expect(requestedUnsafeImageURLs).toEqual([]);
    await expect(page.locator('body')).not.toHaveAttribute('data-stored-xss', 'executed');
    await expect(page.locator('body')).not.toHaveAttribute('data-image-xss', 'executed');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);

    for (const invalidId of ['invalid-score', 'invalid-date', 'invalid-nested-score', 'invalid-distance', 'invalid-status']) {
      await expect(page.locator(`article[data-match-id="${invalidId}"]`)).toHaveCount(0);
    }

    const longCard = page.locator('article').filter({ hasText: longValue }).first();
    await expect(longCard).toBeVisible();
    await expect(longCard).toContainText(longValue);

    await hostileCard.locator('.reunion-btn').click();
    await expect(page.locator('#reunion-match-id')).toHaveValue(matchIdPayload);
    await expect(page.locator('#reunion-pet-id')).toHaveValue(petIdPayload);
    await page.locator('#reunion-modal button[type="button"]').click();
    await expect(page.locator('#reunion-modal')).toBeHidden();

    const actionRequestPromise = page.waitForRequest((request) =>
      request.url().endsWith('/api/v1/matches/action') && request.method() === 'POST');
    await hostileCard.locator('[data-action="confirm"]').click();
    const actionRequest = await actionRequestPromise;
    expect(actionRequest.postDataJSON()).toEqual({ matchId: matchIdPayload, action: 'confirm' });

    const legitimateCard = page.locator('article[data-match-id="match-legitimate"]');
    await expect(legitimateCard).toContainText('Buddy (Golden Retriever)');
    await expect(legitimateCard.locator('img[src^="https://storage.petspotr.io/"]')).toHaveCount(2);
    await expect(legitimateCard.locator('.action-btn')).toHaveCount(2);

    await legitimateCard.locator('.zoom-btn').first().click();
    await expect(page.locator('#zoom-modal')).toHaveCSS('display', 'flex');
    await expect(page.locator('#zoomed-image')).toHaveAttribute(
      'src',
      'https://storage.petspotr.io/lost-legitimate.jpg',
    );
  });
});
