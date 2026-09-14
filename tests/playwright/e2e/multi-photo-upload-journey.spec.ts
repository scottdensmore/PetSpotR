import { test, expect } from '@playwright/test';

test.describe('User Journey: Multi-Photo Upload & Staging', () => {
  test('Lost pet wizard: staging photos, tagging angles, counter lock at 3 photos, and submitting metadata', async ({ page }) => {
    // Intercept presigned URL generation and direct upload
    await page.route('**/api/v1/uploads/presigned-url', async route => {
      const data = route.request().postDataJSON();
      const fileName = data?.fileName || 'photo.jpg';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          uploadUrl: `http://localhost:8082/mock-storage/${fileName}`,
          publicUrl: `https://storage.petspotr.io/images/${fileName}`,
          fileName: `images/lost-pets/${fileName}`,
        }),
      });
    });

    await page.route('**/mock-storage/**', async route => {
      await route.fulfill({ status: 200 });
    });

    let submittedLostPayload: any = null;
    await page.route('**/api/v1/lost-pets', async route => {
      submittedLostPayload = route.request().postDataJSON();
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'success',
          petId: submittedLostPayload?.petId || 'lost-e2e-test',
        }),
      });
    });

    await page.goto('/report-lost');
    await expect(page.locator('#lost-pet-form')).toBeVisible();

    // Step 1: Pet Details
    await page.locator('#petName').fill('Barnaby');
    await page.locator('#species').selectOption('Dog');
    await page.locator('#breed').fill('Golden Retriever');
    await page.locator('#primaryColor').fill('Golden');
    await page.locator('#description').fill('Friendly dog with red collar');
    await page.locator('#btn-next').click();

    // Step 2: Multi-Photo Dropzone
    const dropzone = page.locator('#dropzone');
    const photoCountBadge = page.locator('#photo-count-badge');
    const stagingContainer = page.locator('#staging-container');
    const photoInput = page.locator('#photoInput');

    await expect(dropzone).toBeVisible();
    await expect(photoCountBadge).toHaveText('0 / 3 photos added');
    await expect(stagingContainer).toBeHidden();

    // Stage first photo
    await photoInput.setInputFiles({
      name: 'front-face.jpg',
      mimeType: 'image/jpeg',
      buffer: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46]),
    });

    await expect(stagingContainer).toBeVisible();
    await expect(photoCountBadge).toHaveText('1 / 3 photos added');
    let cards = stagingContainer.locator('.photo-staging-card');
    await expect(cards).toHaveCount(1);
    await expect(cards.first().locator('.photo-tag-select')).toHaveValue('primary');

    // Stage second and third photos
    await photoInput.setInputFiles([
      {
        name: 'side-coat.jpg',
        mimeType: 'image/jpeg',
        buffer: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46]),
      },
      {
        name: 'collar-detail.jpg',
        mimeType: 'image/jpeg',
        buffer: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46]),
      },
    ]);

    await expect(photoCountBadge).toHaveText('3 / 3 photos added');
    cards = stagingContainer.locator('.photo-staging-card');
    await expect(cards).toHaveCount(3);
    await expect(dropzone).toHaveAttribute('aria-disabled', 'true');

    // Check default tags
    await expect(cards.nth(0).locator('.photo-tag-select')).toHaveValue('primary');
    await expect(cards.nth(1).locator('.photo-tag-select')).toHaveValue('coat');
    await expect(cards.nth(2).locator('.photo-tag-select')).toHaveValue('collar');

    // Change tag on second photo
    await cards.nth(1).locator('.photo-tag-select').selectOption('collar');
    await expect(cards.nth(1).locator('.photo-tag-select')).toHaveValue('collar');

    // Remove third photo
    const removeBtns = stagingContainer.locator('.btn-remove-photo');
    await removeBtns.nth(2).click();

    await expect(photoCountBadge).toHaveText('2 / 3 photos added');
    await expect(dropzone).toHaveAttribute('aria-disabled', 'false');
    await expect(stagingContainer.locator('.photo-staging-card')).toHaveCount(2);

    // Re-add a third photo to verify dropzone re-enables properly
    await photoInput.setInputFiles({
      name: 'back-view.jpg',
      mimeType: 'image/jpeg',
      buffer: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46]),
    });
    await expect(photoCountBadge).toHaveText('3 / 3 photos added');
    await expect(dropzone).toHaveAttribute('aria-disabled', 'true');

    // Proceed to Step 3: Location
    await page.locator('#btn-next').click();
    await page.locator('#location').fill('Portland, OR');

    // Proceed to Step 4: Contact & Review
    await page.locator('#btn-next').click();
    await page.locator('#reporterEmail').fill('owner@example.com');
    await page.locator('#phone').fill('555-123-4567');

    // Submit report
    await page.locator('#btn-submit').click();

    // Verify submitted payload contains multi-photo images array
    await expect.poll(() => submittedLostPayload).not.toBeNull();
    expect(submittedLostPayload.petName).toBe('Barnaby');
    expect(submittedLostPayload.images).toBeDefined();
    expect(submittedLostPayload.images.length).toBe(3);
    expect(submittedLostPayload.images[0].tag).toBe('primary');
    expect(submittedLostPayload.images[0].object).toContain('images/lost-pets/');
  });

  test('Found pet report: multi-photo staging, tag selection, and report submission', async ({ page }) => {
    // Intercept presigned URL generation and direct upload
    await page.route('**/api/v1/uploads/presigned-url', async route => {
      const data = route.request().postDataJSON();
      const fileName = data?.fileName || 'found-photo.jpg';
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          uploadUrl: `http://localhost:8082/mock-storage/${fileName}`,
          publicUrl: `https://storage.petspotr.io/images/${fileName}`,
          fileName: `images/found-pets/${fileName}`,
        }),
      });
    });

    await page.route('**/mock-storage/**', async route => {
      await route.fulfill({ status: 200 });
    });

    // Mock AI feature extraction
    await page.route('**/api/v1/found-pets/extract-features', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          species: 'Dog',
          breed: 'Labrador Retriever',
          primaryColor: 'Yellow',
          secondaryColor: 'Cream',
          distinctiveMarkings: ['Floppy ears', 'Dark nose'],
        }),
      });
    });

    let submittedFoundPayload: any = null;
    await page.route('**/api/v1/found-pets', async route => {
      submittedFoundPayload = route.request().postDataJSON();
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'success',
          petId: submittedFoundPayload?.petId || 'found-e2e-test',
        }),
      });
    });

    await page.goto('/report-found');
    await expect(page.locator('#found-pet-form')).toBeVisible();

    const dropzone = page.locator('#found-dropzone');
    const photoInput = page.locator('#foundPhotoInput');
    const photoCountBadge = page.locator('#photo-count-badge');
    const stagingContainer = page.locator('#staging-container');

    await expect(dropzone).toBeVisible();
    await expect(photoCountBadge).toHaveText('0 / 3 photos added');

    // Stage 2 photos
    await photoInput.setInputFiles([
      {
        name: 'found-angle1.jpg',
        mimeType: 'image/jpeg',
        buffer: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46]),
      },
      {
        name: 'found-angle2.jpg',
        mimeType: 'image/jpeg',
        buffer: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46]),
      },
    ]);

    await expect(stagingContainer).toBeVisible();
    await expect(photoCountBadge).toHaveText('2 / 3 photos added');
    const cards = stagingContainer.locator('.photo-staging-card');
    await expect(cards).toHaveCount(2);

    // Select tags
    await cards.nth(0).locator('.photo-tag-select').selectOption('primary');
    await cards.nth(1).locator('.photo-tag-select').selectOption('coat');

    // Fill form details
    await page.locator('#foundLocation').fill('Hawthorne, Portland, OR');
    await page.locator('#finderEmail').fill('finder@example.com');

    // Submit found pet form
    await page.locator('#btn-submit-found').click();

    // Verify submitted payload
    await expect.poll(() => submittedFoundPayload).not.toBeNull();
    expect(submittedFoundPayload.location).toBe('Hawthorne, Portland, OR');
    expect(submittedFoundPayload.finderEmail).toBe('finder@example.com');
    expect(submittedFoundPayload.images).toBeDefined();
    expect(submittedFoundPayload.images.length).toBe(2);
    expect(submittedFoundPayload.images[0].tag).toBe('primary');
    expect(submittedFoundPayload.images[1].tag).toBe('coat');
  });
});
