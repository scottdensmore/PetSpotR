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

  test('should sign in with Google, submit an owned lost report, and log out', async ({ page }) => {
    const principal = {
      issuer: 'https://securetoken.google.com/demo-petspotr-auth',
      subject: 'google-owner-101',
      email: 'verified-owner@example.com',
      emailVerified: true,
      signInProvider: 'google.com',
    };
    let signedIn = false;
    let csrfCount = 0;
    let submittedReport: Record<string, unknown> | undefined;
    let logoutCount = 0;

    await page.addInitScript(() => {
      const browserWindow = window as typeof window & {
        releasePetspotrGoogleSignIn?: () => void;
        petspotrFirebaseAuthAdapterFactory?: () => Promise<{
          signInWithGoogle: () => Promise<string>;
          signOut: () => Promise<void>;
        }>;
      };
      browserWindow.petspotrFirebaseAuthAdapterFactory = async () => ({
        signInWithGoogle: () => new Promise((resolve) => {
          browserWindow.releasePetspotrGoogleSignIn = () => resolve('firebase-google-id-token');
        }),
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
          authEmulatorUrl: 'http://127.0.0.1:9099',
        }),
      });
    });
    await page.route('**/api/v1/session/csrf', async (route) => {
      csrfCount += 1;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ csrfToken: `csrf-${csrfCount}` }),
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
        expect(request.headers()['x-csrf-token']).toBe('csrf-1');
        expect(request.postDataJSON()).toEqual({ idToken: 'firebase-google-id-token' });
        signedIn = true;
        await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(principal) });
        return;
      }
      expect(request.method()).toBe('DELETE');
      expect(request.headers()['x-csrf-token']).toBe('csrf-3');
      logoutCount += 1;
      signedIn = false;
      await route.fulfill({ status: 204 });
    });
    await page.route('**/api/v1/lost-pets', async (route) => {
      expect(route.request().headers()['x-csrf-token']).toBe('csrf-2');
      submittedReport = route.request().postDataJSON() as Record<string, unknown>;
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'success', petId: submittedReport.petId }),
      });
    });

    await page.goto(`${WEB_FRONTEND_URL}/report-lost`);
    await expect(page.locator('#identity-panel')).toBeVisible();
    await expect(page.locator('#identity-status')).toContainText('Sign in');
    const signInButton = page.locator('#google-sign-in');
    await signInButton.click();
    await expect(page.locator('#identity-status')).toContainText('Signing in');
    await expect(signInButton).toBeFocused();
    await page.evaluate(() => {
      const browserWindow = window as typeof window & { releasePetspotrGoogleSignIn?: () => void };
      browserWindow.releasePetspotrGoogleSignIn?.();
    });
    await expect(page.locator('#identity-status')).toContainText('verified-owner@example.com');
    await expect(page.locator('#identity-sign-out')).toBeFocused();
    await expect(page.locator('#reporterEmail')).toHaveValue('verified-owner@example.com');
    await expect(page.locator('#reporterEmail')).toHaveAttribute('readonly', '');

    await page.locator('#petName').fill('Buddy');
    await page.locator('#btn-next').click();
    await page.locator('#btn-next').click();
    await page.locator('#location').fill('Seattle, WA');
    await page.locator('#btn-next').click();
    await page.locator('#btn-submit').click();
    await expect(page.locator('#success-modal')).toBeVisible();
    expect(submittedReport).toMatchObject({
      reporterEmail: 'verified-owner@example.com',
      petName: 'Buddy',
      location: 'Seattle, WA',
    });

    await page.reload();
    await expect(page.locator('#identity-status')).toContainText('verified-owner@example.com');
    await page.locator('#identity-sign-out').click();
    await expect(page.locator('#identity-status')).toContainText('Sign in');
    await expect(page.locator('#google-sign-in')).toBeFocused();
    expect(logoutCount).toBe(1);
  });

  test('should block identity-enabled submission when session configuration is unavailable', async ({ page }) => {
    await page.route('**/api/v1/session/client-config', async (route) => {
      await route.fulfill({ status: 503, body: 'temporarily unavailable' });
    });

    await page.goto(`${WEB_FRONTEND_URL}/report-lost`);
    await expect(page.locator('#identity-panel')).toBeVisible();
    await expect(page.locator('#identity-error')).toContainText('temporarily unavailable');
    await expect(page.locator('#google-sign-in')).toBeHidden();
    const errorCode = await page.evaluate(async () => {
      const identityWindow = window as typeof window & {
        petspotrIdentity: { requireSession: () => Promise<unknown> };
      };
      try {
        await identityWindow.petspotrIdentity.requireSession();
        return '';
      } catch (error) {
        return (error as Error & { code?: string }).code ?? '';
      }
    });
    expect(errorCode).toBe('identity-unavailable');
  });

  test('should wait for the Firebase adapter before enabling Google sign-in', async ({ page }) => {
    await page.addInitScript(() => {
      type Adapter = {
        signInWithGoogle: () => Promise<string>;
        signOut: () => Promise<void>;
      };
      const browserWindow = window as typeof window & {
        releasePetspotrFirebaseAdapter?: () => void;
        petspotrFirebaseAuthAdapterFactory?: () => Promise<Adapter>;
      };
      browserWindow.petspotrFirebaseAuthAdapterFactory = () => new Promise((resolve) => {
        browserWindow.releasePetspotrFirebaseAdapter = () => resolve({
          signInWithGoogle: async () => 'unused-token',
          signOut: async () => {},
        });
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
      await route.fulfill({ status: 401, body: 'Authentication required' });
    });

    await page.goto(`${WEB_FRONTEND_URL}/report-lost`);
    await expect.poll(() => page.evaluate(() => {
      const browserWindow = window as typeof window & { releasePetspotrFirebaseAdapter?: () => void };
      return typeof browserWindow.releasePetspotrFirebaseAdapter;
    })).toBe('function');
    await expect(page.locator('#identity-panel')).toBeHidden();
    await page.evaluate(() => {
      const browserWindow = window as typeof window & { releasePetspotrFirebaseAdapter?: () => void };
      browserWindow.releasePetspotrFirebaseAdapter?.();
    });
    await expect(page.locator('#identity-panel')).toBeVisible();
    await expect(page.locator('#google-sign-in')).toBeVisible();
  });

  test('should disable Google sign-in when the Firebase adapter cannot initialize', async ({ page }) => {
    await page.addInitScript(() => {
      const browserWindow = window as typeof window & {
        petspotrFirebaseAuthAdapterFactory?: () => Promise<never>;
      };
      browserWindow.petspotrFirebaseAuthAdapterFactory = async () => {
        throw new Error('Firebase SDK unavailable');
      };
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
      await route.fulfill({ status: 401, body: 'Authentication required' });
    });

    await page.goto(`${WEB_FRONTEND_URL}/report-lost`);
    await expect(page.locator('#identity-panel')).toBeVisible();
    await expect(page.locator('#identity-error')).toContainText('temporarily unavailable');
    await expect(page.locator('#google-sign-in')).toBeHidden();
    const errorCode = await page.evaluate(async () => {
      const identityWindow = window as typeof window & {
        petspotrIdentity: { requireSession: () => Promise<unknown> };
      };
      try {
        await identityWindow.petspotrIdentity.requireSession();
        return '';
      } catch (error) {
        return (error as Error & { code?: string }).code ?? '';
      }
    });
    expect(errorCode).toBe('identity-unavailable');
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

  test('should sign in with Google, submit an owned found report, and log out', async ({ page }) => {
    const principal = {
      issuer: 'https://securetoken.google.com/demo-petspotr-auth',
      subject: 'google-finder-202',
      email: 'verified-finder@example.com',
      emailVerified: true,
      signInProvider: 'google.com',
    };
    let signedIn = false;
    let csrfCount = 0;
    let submittedReport: Record<string, unknown> | undefined;

    await page.addInitScript(() => {
      const browserWindow = window as typeof window & {
        petspotrFirebaseAuthAdapterFactory?: () => Promise<{
          signInWithGoogle: () => Promise<string>;
          signOut: () => Promise<void>;
        }>;
      };
      browserWindow.petspotrFirebaseAuthAdapterFactory = async () => ({
        signInWithGoogle: async () => 'firebase-google-finder-token',
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
        body: JSON.stringify({ csrfToken: `found-csrf-${csrfCount}` }),
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
        expect(request.headers()['x-csrf-token']).toBe('found-csrf-1');
        expect(request.postDataJSON()).toEqual({ idToken: 'firebase-google-finder-token' });
        signedIn = true;
        await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(principal) });
        return;
      }
      expect(request.method()).toBe('DELETE');
      expect(request.headers()['x-csrf-token']).toBe('found-csrf-3');
      signedIn = false;
      await route.fulfill({ status: 204 });
    });
    await page.route('**/api/v1/found-pets', async (route) => {
      expect(route.request().headers()['x-csrf-token']).toBe('found-csrf-2');
      submittedReport = route.request().postDataJSON() as Record<string, unknown>;
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'success', petId: submittedReport.petId }),
      });
    });

    await page.goto(`${WEB_FRONTEND_URL}/report-found`);
    await expect(page.locator('#identity-panel')).toBeVisible();
    await page.locator('#foundLocation').fill('Green Lake Park, Seattle, WA');
    await page.locator('#finderEmail').fill('spoofed-finder@example.com');
    let signInDialogMessage = '';
    const signInDialogHandled = page.waitForEvent('dialog').then(async (dialog) => {
      signInDialogMessage = dialog.message();
      await dialog.accept();
    });
    await Promise.all([
      signInDialogHandled,
      page.locator('#btn-submit-found').click(),
    ]);
    expect(signInDialogMessage).toContain('Sign in with Google');
    expect(submittedReport).toBeUndefined();
    await expect(page.locator('#google-sign-in')).toBeFocused();

    await page.locator('#google-sign-in').click();
    await expect(page.locator('#identity-status')).toContainText('verified-finder@example.com');
    await expect(page.locator('#finderEmail')).toHaveValue('verified-finder@example.com');
    await expect(page.locator('#finderEmail')).toHaveAttribute('readonly', '');

    await page.locator('#btn-submit-found').click();
    await expect(page.locator('#found-success-modal')).toBeVisible();
    expect(submittedReport).toMatchObject({
      finderEmail: 'verified-finder@example.com',
      location: 'Green Lake Park, Seattle, WA',
    });

    await page.reload();
    await expect(page.locator('#identity-status')).toContainText('verified-finder@example.com');
    await page.locator('#identity-sign-out').click();
    await expect(page.locator('#identity-status')).toContainText('Sign in');
    await expect(page.locator('#finderEmail')).toHaveValue('');
    await expect(page.locator('#finderEmail')).not.toHaveAttribute('readonly', '');
  });

  test('should reuse the found report identity after a transient browser retry', async ({ page }) => {
    const submissions: Array<Record<string, unknown>> = [];
    await page.route('**/api/v1/found-pets/extract-features', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          species: 'Dog',
          breed: 'Golden Retriever',
          primaryColor: 'Golden',
          secondaryColor: 'Cream',
          distinctiveMarkings: ['White chest patch'],
        }),
      });
    });
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
    await page.locator('#foundPhotoInput').setInputFiles({
      name: 'found-pet.png',
      mimeType: 'image/png',
      buffer: Buffer.from('found-pet-image'),
    });
    await expect(page.locator('#foundBreed')).toHaveValue('Golden Retriever');
    await page.locator('#foundLocation').fill('Seattle, WA');
    await page.locator('#finderEmail').fill('finder@example.com');
    await page.locator('#custodyStatus').selectOption('Local Shelter');

    await page.locator('#found-pet-form button[type="submit"]').click();
    await expect.poll(() => submissions.length).toBe(1);
    await page.locator('#found-pet-form button[type="submit"]').click();
    await expect(page.locator('#found-success-modal')).toBeVisible();

    expect(submissions).toHaveLength(2);
    expect(submissions[0].petId).toMatch(/^found-[0-9a-f-]+$/);
    expect(submissions[1].petId).toBe(submissions[0].petId);
    expect(submissions[1].foundAt).toBe(submissions[0].foundAt);
    expect(submissions[1]).toMatchObject({
      finderEmail: 'finder@example.com',
      species: 'Dog',
      breed: 'Golden Retriever',
      primaryColor: 'Golden',
      secondaryColor: 'Cream',
      distinctiveMarkings: ['White chest patch'],
      custodyStatus: 'Local Shelter',
    });
  });

  test('should clear stale traits when replacement image analysis fails', async ({ page }) => {
    let extractionAttempts = 0;
    let submission: Record<string, unknown> | undefined;
    await page.route('**/api/v1/found-pets/extract-features', async (route) => {
      extractionAttempts += 1;
      if (extractionAttempts === 2) {
        await route.fulfill({ status: 503, body: 'temporarily unavailable' });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          species: 'Dog',
          breed: 'Golden Retriever',
          primaryColor: 'Golden',
          secondaryColor: 'Cream',
          distinctiveMarkings: ['White chest patch'],
        }),
      });
    });
    await page.route('**/api/v1/found-pets', async (route) => {
      submission = route.request().postDataJSON() as Record<string, unknown>;
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'success', petId: submission.petId }),
      });
    });

    await page.goto(`${WEB_FRONTEND_URL}/report-found`);
    await page.locator('#foundPhotoInput').setInputFiles({
      name: 'first-pet.png',
      mimeType: 'image/png',
      buffer: Buffer.from('first-pet-image'),
    });
    await expect(page.locator('#foundBreed')).toHaveValue('Golden Retriever');

    await page.locator('#foundPhotoInput').setInputFiles({
      name: 'replacement-pet.png',
      mimeType: 'image/png',
      buffer: Buffer.from('replacement-pet-image'),
    });
    await expect(page.locator('#ai-extraction-status')).toHaveText(
      'Image analysis failed. Choose the pet traits manually or try another image.',
    );
    await expect(page.locator('#foundSpecies')).toHaveValue('');
    await expect(page.locator('#foundBreed')).toHaveValue('');
    await expect(page.locator('#foundPrimaryColor')).toHaveValue('');
    await expect(page.locator('#foundSecondaryColor')).toHaveValue('');
    await expect(page.locator('#chip-breed')).toHaveText('Breed: Not analyzed');
    await expect(page.locator('#chip-color')).toHaveText('Colors: Not analyzed');

    await page.locator('#foundSpecies').selectOption('Cat');
    await page.locator('#foundLocation').fill('Seattle, WA');
    await page.locator('#finderEmail').fill('finder@example.com');
    await page.locator('#found-pet-form button[type="submit"]').click();
    await expect(page.locator('#found-success-modal')).toBeVisible();

    expect(submission).toMatchObject({
      species: 'Cat',
      breed: '',
      primaryColor: '',
      secondaryColor: '',
      distinctiveMarkings: [],
    });
  });

  test('should ignore a stale analysis response after replacing the image', async ({ page }) => {
    let extractionAttempts = 0;
    let releaseFirstExtraction: (() => void) | undefined;
    const firstExtractionReleased = new Promise<void>((resolve) => {
      releaseFirstExtraction = resolve;
    });
    await page.route('**/api/v1/found-pets/extract-features', async (route) => {
      extractionAttempts += 1;
      if (extractionAttempts === 1) {
        await firstExtractionReleased;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            species: 'Dog',
            breed: 'Stale Golden Retriever',
            primaryColor: 'Stale Golden',
            secondaryColor: 'Stale Cream',
            distinctiveMarkings: ['Stale white chest patch'],
          }),
        });
        return;
      }
      await route.fulfill({ status: 503, body: 'replacement analysis unavailable' });
    });

    await page.goto(`${WEB_FRONTEND_URL}/report-found`);
    await page.locator('#foundPhotoInput').setInputFiles({
      name: 'slow-first-pet.png',
      mimeType: 'image/png',
      buffer: Buffer.from('slow-first-pet-image'),
    });
    await expect.poll(() => extractionAttempts).toBe(1);

    await page.locator('#foundPhotoInput').setInputFiles({
      name: 'replacement-pet.png',
      mimeType: 'image/png',
      buffer: Buffer.from('replacement-pet-image'),
    });
    await expect(page.locator('#ai-extraction-status')).toHaveText(
      'Image analysis failed. Choose the pet traits manually or try another image.',
    );

    releaseFirstExtraction?.();
    await expect.poll(() => page.locator('#foundBreed').inputValue()).toBe('');
    await expect(page.locator('#foundPrimaryColor')).toHaveValue('');
    await expect(page.locator('#foundSecondaryColor')).toHaveValue('');
    await expect(page.locator('#chip-breed')).toHaveText('Breed: Not analyzed');
    await expect(page.locator('#ai-extraction-status')).toHaveText(
      'Image analysis failed. Choose the pet traits manually or try another image.',
    );
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
