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
    await expect(page.locator('.action-btn')).toHaveCount(2);
    await expect(page.locator('.contact-btn, .reunion-btn')).toHaveCount(0);
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

  test('submits participant match decisions with CSRF and accurate feedback', async ({ page }) => {
    const principal = {
      issuer: 'https://securetoken.google.com/demo-petspotr-auth',
      subject: 'google-match-reporter-404',
      email: 'verified-reporter@example.com',
      emailVerified: true,
      signInProvider: 'google.com',
    };
    const decisions: Array<{ headers: Record<string, string>; body: Record<string, unknown> }> = [];
    let matchRequests = 0;
    let releaseFirstDecision: (() => void) | undefined;
    const firstDecisionGate = new Promise<void>((resolve) => {
      releaseFirstDecision = resolve;
    });
    let releaseDecisionRefresh: (() => void) | undefined;
    const decisionRefreshGate = new Promise<void>((resolve) => {
      releaseDecisionRefresh = resolve;
    });

    await page.addInitScript(() => {
      const browserWindow = window as typeof window & {
        petspotrFirebaseAuthAdapterFactory?: () => Promise<{
          signInWithGoogle: () => Promise<string>;
          signOut: () => Promise<void>;
        }>;
      };
      browserWindow.petspotrFirebaseAuthAdapterFactory = async () => ({
        signInWithGoogle: async () => 'unused-token',
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
    await page.route('**/api/v1/session', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(principal) });
    });
    await page.route('**/api/v1/session/csrf', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ csrfToken: 'match-decision-csrf' }),
      });
    });
    await page.route('**/api/v1/matches', async (route) => {
      matchRequests += 1;
      if (matchRequests === 2) await decisionRefreshGate;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([validMatchRecord('decision-match')]),
      });
    });
    await page.route('**/api/v1/matches/action', async (route) => {
      const request = route.request();
      decisions.push({ headers: request.headers(), body: request.postDataJSON() as Record<string, unknown> });
      if (decisions.length === 1) {
        await firstDecisionGate;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ matchId: 'decision-match', status: 'PENDING_REVIEW' }),
        });
        return;
      }
      await route.fulfill({ status: 409, body: 'Match decision conflicts with the accepted decision' });
    });

    await page.goto(`${WEB_FRONTEND_URL}/matches`);
    const card = page.locator('article[data-match-id="decision-match"]');
    await expect(card).toBeVisible();
    await expect(card.locator('.action-btn')).toHaveCount(2);
    await expect(card.locator('.contact-btn, .reunion-btn')).toHaveCount(0);

    const confirmButton = card.locator('[data-action="confirm"]');
    const rejectButton = card.locator('[data-action="reject"]');
    await card.locator('.action-btn').evaluateAll((buttons) => {
      const actionButtons = buttons as HTMLButtonElement[];
      const confirm = actionButtons.find(button => button.dataset.action === 'confirm');
      const reject = actionButtons.find(button => button.dataset.action === 'reject');
      confirm?.click();
      confirm?.click();
      reject?.click();
    });
    await expect(confirmButton).toHaveAttribute('aria-disabled', 'true');
    await expect(rejectButton).toHaveAttribute('aria-disabled', 'true');
    await expect(confirmButton).toHaveText('Saving decision...');
    await expect(page.locator('#match-decision-status')).toHaveText('Saving match decision...');
    await expect.poll(() => decisions.length).toBe(1);
    releaseFirstDecision?.();
    await expect.poll(() => matchRequests).toBe(2);
    await expect(page.locator('#match-action-modal')).toBeHidden();
    releaseDecisionRefresh?.();
    await expect(page.locator('#match-action-modal')).toBeVisible();
    await expect(page.locator('#match-action-modal')).toHaveAttribute('role', 'dialog');
    await expect(page.locator('#match-action-modal')).toHaveAttribute('aria-modal', 'true');
    await expect(page.locator('#action-modal-title')).toHaveText('Decision recorded');
    await expect(page.locator('#action-modal-desc')).toContainText('other participant');
    await expect(page.locator('#match-action-modal .modal-close')).toBeFocused();
    await page.keyboard.press('Tab');
    await expect(page.locator('#match-action-modal .modal-close')).toBeFocused();
    expect(decisions[0]).toEqual({
      headers: expect.objectContaining({ 'x-csrf-token': 'match-decision-csrf' }),
      body: { matchId: 'decision-match', action: 'confirm' },
    });
    await page.keyboard.press('Enter');
    await expect(card.locator('[data-action="confirm"]')).toBeFocused();

    await card.locator('[data-action="reject"]').click();
    await expect(page.locator('#match-action-modal')).toBeVisible();
    await expect(page.locator('#action-modal-title')).toHaveText('Decision not saved');
    await expect(page.locator('#action-modal-desc')).toContainText('already recorded');
    await expect(page.locator('#match-action-modal .modal-close')).toBeFocused();
    expect(decisions[1]).toEqual({
      headers: expect.objectContaining({ 'x-csrf-token': 'match-decision-csrf' }),
      body: { matchId: 'decision-match', action: 'reject' },
    });
    await page.keyboard.press('Enter');
    await expect(card.locator('[data-action="reject"]')).toBeFocused();
  });

  test('discards a pending match decision when the participant logs out', async ({ page }) => {
    const principal = {
      issuer: 'https://securetoken.google.com/demo-petspotr-auth',
      subject: 'google-match-finder-505',
      email: 'verified-finder@example.com',
      emailVerified: true,
      signInProvider: 'google.com',
    };
    let signedIn = true;
    let matchRequests = 0;
    let decisionRequests = 0;
    let releaseDecision: (() => void) | undefined;
    const decisionGate = new Promise<void>((resolve) => {
      releaseDecision = resolve;
    });

    await page.addInitScript(() => {
      const browserWindow = window as typeof window & {
        petspotrFirebaseAuthAdapterFactory?: () => Promise<{
          signInWithGoogle: () => Promise<string>;
          signOut: () => Promise<void>;
        }>;
      };
      browserWindow.petspotrFirebaseAuthAdapterFactory = async () => ({
        signInWithGoogle: async () => 'unused-token',
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
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ csrfToken: 'logout-race-csrf' }),
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
      expect(request.method()).toBe('DELETE');
      signedIn = false;
      await route.fulfill({ status: 204 });
    });
    await page.route('**/api/v1/matches', async (route) => {
      matchRequests += 1;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([validMatchRecord('logout-race-match')]),
      });
    });
    await page.route('**/api/v1/matches/action', async (route) => {
      decisionRequests += 1;
      await decisionGate;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ matchId: 'logout-race-match', status: 'PENDING_REVIEW' }),
      });
    });

    await page.goto(`${WEB_FRONTEND_URL}/matches`);
    await expect(page.locator('article[data-match-id="logout-race-match"]')).toBeVisible();
    await page.locator('[data-match-id="logout-race-match"][data-action="confirm"]').click();
    await expect.poll(() => decisionRequests).toBe(1);
    await page.locator('#identity-sign-out').click();
    await expect(page.locator('article[data-match-id]')).toHaveCount(0);
    await expect(page.locator('#google-sign-in')).toBeFocused();
    releaseDecision?.();
    await expect(page.locator('#match-decision-status')).toBeHidden();
    await expect(page.locator('#match-action-modal')).toBeHidden();
    await expect.poll(() => matchRequests).toBe(1);
    await expect(page.locator('#google-sign-in')).toBeFocused();
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
    await expect(page.locator('#match-action-modal')).toBeVisible();
    await page.locator('#match-action-modal .modal-close').click();
    await expect(page.locator('#match-action-modal')).toBeHidden();

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
