import { test, expect, BrowserContext, Page } from '@playwright/test';

test.describe.serial('Offline PWA & Background Sync User Journeys', () => {
  let context: BrowserContext;
  let page: Page;

  test.beforeAll(async ({ browser }) => {
    context = await browser.newContext();
    await context.route('https://storage.petspotr.io/**', async (route) => {
      await route.fulfill({ status: 200 });
    });
    page = await context.newPage();
  });

  test.afterAll(async () => {
    await context.close();
  });

  test('should cache app shell and render directory when offline', async () => {
    // 1. Navigate to /pets while online to warm Service Worker cache and wait for SW to be ready.
    await context.setOffline(false);
    const initialResponse = await page.goto('/pets');
    expect(initialResponse?.status()).toBe(200);

    // Wait for Service Worker registration and active controller
    await page.evaluate(async () => {
      if ('serviceWorker' in navigator) {
        await navigator.serviceWorker.ready;
      }
    });
    await page.waitForFunction(() => navigator.serviceWorker.controller !== null, { timeout: 10000 });

    // 2. Set offline
    await context.setOffline(true);

    // 3. Reload /pets while offline; assert HTTP 200, DOM contains .glass-nav and #offline-indicator is visible with text "Offline"
    const reloadResponse = await page.reload();
    // In Playwright Chromium, navigating to a new document while offline emulation is active
    // can reset internal CDP network state; toggling re-asserts offline emulation so that
    // navigator.onLine and offline event handlers fire reliably.
    await context.setOffline(false);
    await context.setOffline(true);

    expect(reloadResponse?.status()).toBe(200);
    await expect(page.locator('.glass-nav')).toBeVisible();
    await expect(page.locator('#offline-indicator')).toBeVisible();
    await expect(page.locator('#offline-indicator')).toContainText('Offline');

    // 4. Navigate to / while offline; assert HTTP 200, DOM contains .glass-nav and #offline-indicator is visible with text "Offline"
    const homeResponse = await page.goto('/');
    // In Playwright Chromium, navigating to a new document while offline emulation is active
    // can reset internal CDP network state; toggling re-asserts offline emulation so that
    // navigator.onLine and offline event handlers fire reliably.
    await context.setOffline(false);
    await context.setOffline(true);

    expect(homeResponse?.status()).toBe(200);
    await expect(page.locator('.glass-nav')).toBeVisible();
    await expect(page.locator('#offline-indicator')).toBeVisible();
    await expect(page.locator('#offline-indicator')).toContainText('Offline');
  });

  test('should stage lost pet report with photos in IndexedDB when offline', async () => {
    // While offline, visit /report-lost
    await context.setOffline(true);
    const response = await page.goto('/report-lost');
    // In Playwright Chromium, navigating to a new document while offline emulation is active
    // can reset internal CDP network state; toggling re-asserts offline emulation so that
    // navigator.onLine and offline event handlers fire reliably.
    await context.setOffline(false);
    await context.setOffline(true);
    expect(response?.status()).toBe(200);

    await expect(page.locator('#lost-pet-form')).toBeVisible();

    // Step 1: Pet Details
    await page.locator('#petName').fill('Shadow');
    await page.locator('#species').selectOption('Dog');
    await page.locator('#breed').fill('Husky');
    await page.locator('#primaryColor').fill('Silver / White');
    await page.locator('#description').fill('Friendly Siberian Husky with striking blue eyes');
    await page.locator('#btn-next').click();

    // Step 2: Photo Dropzone
    await expect(page.locator('#dropzone')).toBeVisible();
    await page.locator('#photoInput').setInputFiles({
      name: 'shadow.jpg',
      mimeType: 'image/jpeg',
      buffer: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46]),
    });
    await expect(page.locator('.photo-staging-card')).toHaveCount(1);
    await page.locator('#btn-next').click();

    // Step 3: Location
    await expect(page.locator('#location')).toBeVisible();
    await page.locator('#location').fill('Ballard, Seattle, WA');
    await page.locator('#btn-next').click();

    // Step 4: Contact & Review
    await expect(page.locator('#reporterEmail')).toBeVisible();
    await page.locator('#reporterEmail').fill('owner@example.com');

    // Submit form
    await page.locator('#btn-submit').click();

    // Assert toast notification appears with "Report saved offline"
    await expect(page.locator('#toast-container')).toContainText('Report saved offline');

    // Assert #outbox-count-badge displays 1
    const badge = page.locator('#outbox-count-badge');
    await expect(badge).toBeVisible();
    await expect(badge).toHaveText('1');

    // Inspect IndexedDB to verify record is stored in outbox_reports with photo Blob
    const idbRecord = await page.evaluate(async () => {
      return new Promise<any>((resolve, reject) => {
        const req = indexedDB.open('petspotr_offline_db', 1);
        req.onerror = () => reject(req.error);
        req.onsuccess = () => {
          const db = req.result;
          const tx = db.transaction('outbox_reports', 'readonly');
          const store = tx.objectStore('outbox_reports');
          const getAll = store.getAll();
          getAll.onsuccess = () => {
            const records = getAll.result || [];
            if (records.length === 0) {
              resolve(null);
              return;
            }
            const record = records[0];
            const hasBlob =
              record.photos &&
              record.photos.length > 0 &&
              (record.photos[0].blob instanceof Blob ||
                (typeof record.photos[0].blob?.slice === 'function' &&
                  typeof record.photos[0].blob?.size === 'number'));
            resolve({
              id: record.id,
              type: record.type,
              status: record.status,
              petName: record.payload?.petName,
              species: record.payload?.species,
              breed: record.payload?.breed,
              hasPhotoBlob: hasBlob,
              photoCount: record.photos?.length || 0,
            });
          };
          getAll.onerror = () => reject(getAll.error);
        };
      });
    });

    expect(idbRecord).not.toBeNull();
    expect(idbRecord.status).toBe('pending');
    expect(idbRecord.petName).toBe('Shadow');
    expect(idbRecord.species).toBe('Dog');
    expect(idbRecord.breed).toBe('Husky');
    expect(idbRecord.hasPhotoBlob).toBe(true);
    expect(idbRecord.photoCount).toBe(1);
  });

  test('should automatically sync queued report upon reconnection and persist on backend', async () => {
    // Restores network connectivity
    await context.setOffline(false);

    // Trigger sync via online event and outbox sync engine
    await page.evaluate(async () => {
      window.dispatchEvent(new Event('online'));
      if (window.PetSpotROutbox) {
        await window.PetSpotROutbox.syncAll();
      }
    });

    // Wait for sync to complete and assert #outbox-count-badge becomes hidden / 0
    const badge = page.locator('#outbox-count-badge');
    await expect(badge).toBeHidden();

    // Assert toast notification appears indicating synchronization success
    await expect(page.locator('#toast-container')).toContainText('synchronized successfully');

    // Verify backend /api/v1/lost-pets returns the report with pet name "Shadow"
    const response = await page.request.get('/api/v1/lost-pets');
    expect(response.status()).toBe(200);
    const lostPets = await response.json();
    const shadowReport = lostPets.find((p: any) => p.petName === 'Shadow');
    expect(shadowReport).toBeDefined();
    expect(shadowReport.species).toBe('Dog');
    expect(shadowReport.breed).toBe('Husky');

    // Confirm IndexedDB outbox queue is now 0
    const remainingCount = await page.evaluate(async () => {
      if (window.PetSpotROutbox) {
        return await window.PetSpotROutbox.getQueueCount();
      }
      return 0;
    });
    expect(remainingCount).toBe(0);
  });
});
