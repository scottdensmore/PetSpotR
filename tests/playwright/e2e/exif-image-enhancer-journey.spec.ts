import { test, expect, Page } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8092';

// 1x1 test JPEG payload
const TEST_JPEG_BUFFER = Buffer.from([
  0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01, 0x01, 0x00, 0x48,
  0x00, 0x48, 0x00, 0x00, 0xff, 0xdb, 0x00, 0x43, 0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08,
  0x07, 0x07, 0x07, 0x09, 0x09, 0x08, 0x0a, 0x0c, 0x14, 0x0d, 0x0c, 0x0b, 0x0b, 0x0c, 0x19, 0x12,
  0x13, 0x0f, 0x14, 0x1d, 0x1a, 0x1f, 0x1e, 0x1d, 0x1a, 0x1c, 0x1c, 0x20, 0x24, 0x2e, 0x27, 0x20,
  0x22, 0x2c, 0x23, 0x1c, 0x1c, 0x28, 0x37, 0x29, 0x2c, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1f, 0x27,
  0x39, 0x3d, 0x38, 0x32, 0x3c, 0x2e, 0x33, 0x34, 0x32, 0xff, 0xc0, 0x00, 0x0b, 0x08, 0x00, 0x01,
  0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xff, 0xc4, 0x00, 0x1f, 0x00, 0x00, 0x01, 0x05, 0x01, 0x01,
  0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04,
  0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3f,
  0x00, 0xbf, 0x00, 0xff, 0xd9,
]);

async function setupWizardStep2(
  page: Page,
  options?: {
    gps?: { latitude: number; longitude: number };
    captureTime?: string;
    onExtractMetadataRequest?: () => void;
    onEnhanceRequest?: () => void;
  }
) {
  const gps = options?.gps || { latitude: 47.6062, longitude: -122.3321 };
  const captureTime = options?.captureTime || '2026-09-19T14:30:00Z';

  // Intercept uploads/presigned-url
  await page.route('**/api/v1/uploads/presigned-url', async route => {
    const data = route.request().postDataJSON();
    const fileName = data?.fileName || 'pet.jpg';
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        uploadUrl: `http://localhost:8092/mock-storage/${fileName}`,
        publicUrl: `https://storage.petspotr.io/images/${fileName}`,
        fileName: `images/lost-pets/${fileName}`,
      }),
    });
  });

  // Intercept storage PUT
  await page.route('**/mock-storage/**', async route => {
    await route.fulfill({ status: 200 });
  });

  // Intercept metadata extraction
  await page.route('**/api/v1/images/extract-metadata', async route => {
    if (options?.onExtractMetadataRequest) {
      options.onExtractMetadataRequest();
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        gps,
        captureTime,
        orientation: 1,
        width: 800,
        height: 600,
      }),
    });
  });

  // Intercept image auto-enhancement
  await page.route('**/api/v1/images/enhance', async route => {
    if (options?.onEnhanceRequest) {
      options.onEnhanceRequest();
    }
    await route.fulfill({
      status: 200,
      contentType: 'image/jpeg',
      headers: { 'X-Enhancement-Applied': 'true' },
      body: TEST_JPEG_BUFFER,
    });
  });

  await page.goto(`${WEB_FRONTEND_URL}/report-lost`);
  await expect(page.locator('#lost-pet-form')).toBeVisible();

  // Advance Step 1 -> Step 2
  await page.locator('#petName').fill('Cooper');
  await page.locator('#species').selectOption('Dog');
  await page.locator('#btn-next').click();
  await expect(page.locator('#dropzone')).toBeVisible();
}

test.describe('User Journey: EXIF Auto-Extraction and Image Enhancer', () => {
  test('Test 1: Upload a test JPEG photo and trigger EXIF metadata extraction on /report-lost', async ({ page }) => {
    let extractMetadataCalled = false;
    await setupWizardStep2(page, {
      onExtractMetadataRequest: () => {
        extractMetadataCalled = true;
      },
    });

    await page.locator('#photoInput').setInputFiles({
      name: 'cooper.jpg',
      mimeType: 'image/jpeg',
      buffer: TEST_JPEG_BUFFER,
    });

    await expect(page.locator('#exif-toast')).toBeVisible();
    expect(extractMetadataCalled).toBe(true);
  });

  test('Test 2: Verify the EXIF pre-fill toast appears with location details and Apply/Keep Manual buttons', async ({ page }) => {
    await setupWizardStep2(page, {
      gps: { latitude: 47.6062, longitude: -122.3321 },
    });

    await page.locator('#photoInput').setInputFiles({
      name: 'cooper.jpg',
      mimeType: 'image/jpeg',
      buffer: TEST_JPEG_BUFFER,
    });

    const toast = page.locator('#exif-toast');
    await expect(toast).toBeVisible();

    // Verify exact content elements
    await expect(toast.locator('.exif-toast-icon')).toHaveText('📍');
    await expect(toast.locator('#exif-coords-display')).toHaveText('47.6062, -122.3321');
    await expect(toast).toContainText('Found photo location:');
    await expect(toast).toContainText('Pre-fill report?');
    await expect(page.locator('#btn-exif-apply')).toBeVisible();
    await expect(page.locator('#btn-exif-dismiss')).toBeVisible();
    await expect(page.locator('#btn-exif-apply')).toHaveText('Apply');
    await expect(page.locator('#btn-exif-dismiss')).toHaveText('Keep Manual');
  });

  test('Test 3: Click [Apply] and verify coordinate input fields (#latitude, #longitude, #location) and lost date/time are populated', async ({ page }) => {
    await setupWizardStep2(page, {
      gps: { latitude: 47.6062, longitude: -122.3321 },
      captureTime: '2026-09-19T14:30:00Z',
    });

    // Check fields are initially empty
    await expect(page.locator('#latitude')).toHaveValue('');
    await expect(page.locator('#longitude')).toHaveValue('');
    await expect(page.locator('#location')).toHaveValue('');

    await page.locator('#photoInput').setInputFiles({
      name: 'cooper.jpg',
      mimeType: 'image/jpeg',
      buffer: TEST_JPEG_BUFFER,
    });

    const toast = page.locator('#exif-toast');
    await expect(toast).toBeVisible();

    // Click [Apply]
    await page.locator('#btn-exif-apply').click();

    // Toast should dismiss
    await expect(toast).toBeHidden();

    // Coordinate inputs and location should populate
    await expect(page.locator('#latitude')).toHaveValue('47.6062');
    await expect(page.locator('#longitude')).toHaveValue('-122.3321');
    await expect(page.locator('#location')).toHaveValue('47.6062, -122.3321');

    // Date/time field should populate
    const lostDateValue = await page.locator('#lostDate').inputValue();
    expect(lostDateValue).toBeTruthy();
    expect(lostDateValue.length).toBeGreaterThan(0);

    // Map pin placeholder in Step 3 reflects EXIF location
    const mapPin = page.locator('#wizard-step-3 .map-placeholder span');
    await expect(mapPin).toContainText('47.6062');
    await expect(mapPin).toContainText('-122.3321');
    await expect(mapPin).toContainText('(From Photo EXIF)');
  });

  test('Test 4: Toggle "✨ Auto-Enhance", verify /api/v1/images/enhance is called, slider UI is displayed, and user can slide to compare and click [Use Enhanced]', async ({ page }) => {
    let enhanceApiCalled = false;
    await setupWizardStep2(page, {
      onEnhanceRequest: () => {
        enhanceApiCalled = true;
      },
    });

    await page.locator('#photoInput').setInputFiles({
      name: 'cooper.jpg',
      mimeType: 'image/jpeg',
      buffer: TEST_JPEG_BUFFER,
    });

    const autoEnhanceContainer = page.locator('#auto-enhance-control');
    await expect(autoEnhanceContainer).toBeVisible();

    const btnAutoEnhance = page.locator('#btn-auto-enhance');
    await expect(btnAutoEnhance).toBeVisible();
    await expect(btnAutoEnhance).toContainText('Auto-Enhance');

    // Toggle Auto-Enhance
    await btnAutoEnhance.click();

    // Verify /api/v1/images/enhance was invoked
    expect(enhanceApiCalled).toBe(true);

    // Verify slider view is displayed
    const sliderView = page.locator('#enhance-slider-view');
    await expect(sliderView).toBeVisible();

    const imgBefore = page.locator('#enhance-img-before');
    const imgAfter = page.locator('#enhance-img-after');
    const imgAfterWrap = page.locator('#enhance-img-after-wrap');
    const slider = page.locator('#enhance-slider');

    await expect(imgBefore).toBeVisible();
    await expect(imgAfter).toBeVisible();
    await expect(slider).toBeVisible();

    // Slide to compare (e.g. 70%)
    await slider.fill('70');
    await slider.dispatchEvent('input');

    await expect(imgAfterWrap).toHaveAttribute('style', /70%/);

    // Verify badges and action buttons
    await expect(sliderView.locator('.badge-secondary')).toHaveText('Original');
    await expect(sliderView.locator('.badge-primary')).toHaveText('Enhanced (Auto-Contrast)');

    const btnUseEnhanced = page.locator('#btn-accept-enhanced');
    const btnRevert = page.locator('#btn-revert-enhanced');
    await expect(btnUseEnhanced).toBeVisible();
    await expect(btnRevert).toBeVisible();

    // Click [Use Enhanced]
    await btnUseEnhanced.click();

    // Verify status confirms enhanced version selected
    const statusText = page.locator('#auto-enhance-status');
    await expect(statusText).toContainText('Enhanced version selected');

    // Verify staged preview thumbnail updated
    const thumb = page.locator('.photo-staging-thumb.lost-preview-image').first();
    await expect(thumb).toBeVisible();
    const thumbSrc = await thumb.getAttribute('src');
    expect(thumbSrc).toBeTruthy();
    expect(thumbSrc).toMatch(/^blob:/);

    // Verify clicking "[Use Enhanced]" updates the staged image data to the enhanced JPEG file
    await expect.poll(async () => {
      return await page.evaluate(() => {
        const img = (window as any).petspotrStagedImages?.[0];
        if (!img) return null;
        return {
          name: img.file?.name,
          type: img.file?.type,
          isEnhanced: img.isEnhanced,
        };
      });
    }).toEqual({
      name: 'cooper.jpg',
      type: 'image/jpeg',
      isEnhanced: true,
    });
  });

  test('Test 5: Verify [Keep Manual] dismisses the toast without modifying coordinates', async ({ page }) => {
    await setupWizardStep2(page, {
      gps: { latitude: 47.6062, longitude: -122.3321 },
    });

    // Check fields are empty initially
    await expect(page.locator('#latitude')).toHaveValue('');
    await expect(page.locator('#longitude')).toHaveValue('');
    await expect(page.locator('#location')).toHaveValue('');
    await expect(page.locator('#lostDate')).toHaveValue('');

    await page.locator('#photoInput').setInputFiles({
      name: 'cooper.jpg',
      mimeType: 'image/jpeg',
      buffer: TEST_JPEG_BUFFER,
    });

    const toast = page.locator('#exif-toast');
    await expect(toast).toBeVisible();

    // Click [Keep Manual]
    const btnDismiss = page.locator('#btn-exif-dismiss');
    await expect(btnDismiss).toBeVisible();
    await btnDismiss.click();

    // Toast dismissed
    await expect(toast).toBeHidden();

    // Coordinates and location MUST remain unmodified / empty
    await expect(page.locator('#latitude')).toHaveValue('');
    await expect(page.locator('#longitude')).toHaveValue('');
    await expect(page.locator('#location')).toHaveValue('');
    await expect(page.locator('#lostDate')).toHaveValue('');
  });

  test('Complete End-to-End User Journey: upload photo, apply EXIF geolocation, use enhanced image, and proceed through report', async ({ page }) => {
    let enhanceCalled = false;
    await setupWizardStep2(page, {
      gps: { latitude: 47.6105, longitude: -122.3421 },
      captureTime: '2026-09-18T10:15:00Z',
      onEnhanceRequest: () => {
        enhanceCalled = true;
      },
    });

    // Upload photo
    await page.locator('#photoInput').setInputFiles({
      name: 'cooper-journey.jpg',
      mimeType: 'image/jpeg',
      buffer: TEST_JPEG_BUFFER,
    });

    // Toast appears
    const toast = page.locator('#exif-toast');
    await expect(toast).toBeVisible();
    await expect(toast).toContainText('47.6105, -122.3421');

    // Click Apply
    await page.locator('#btn-exif-apply').click();
    await expect(toast).toBeHidden();

    // Auto-Enhance photo
    await page.locator('#btn-auto-enhance').click();
    await expect.poll(() => enhanceCalled).toBe(true);

    const slider = page.locator('#enhance-slider');
    await expect(slider).toBeVisible();
    await slider.fill('60');
    await slider.dispatchEvent('input');

    await page.locator('#btn-accept-enhanced').click();
    await expect(page.locator('#auto-enhance-status')).toContainText('Enhanced version selected');

    // Advance to Step 3 (Location Picker)
    await page.locator('#btn-next').click();
    await expect(page.locator('#wizard-step-3')).toBeVisible();

    // Verify pre-filled location and coordinates
    await expect(page.locator('#location')).toHaveValue('47.6105, -122.3421');
    await expect(page.locator('#latitude')).toHaveValue('47.6105');
    await expect(page.locator('#longitude')).toHaveValue('-122.3421');
    const lostDate = await page.locator('#lostDate').inputValue();
    expect(lostDate).toBeTruthy();

    // Advance to Step 4 (Owner Contact)
    await page.locator('#btn-next').click();
    await expect(page.locator('#wizard-step-4')).toBeVisible();
    await page.locator('#reporterEmail').fill('cooper.owner@example.com');

    // Intercept final submission
    let submittedPayload: any = null;
    await page.route('**/api/v1/lost-pets', async route => {
      submittedPayload = route.request().postDataJSON();
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          petId: 'lost-cooper-test',
          status: 'INDEXED',
        }),
      });
    });

    // Submit report
    await page.locator('#btn-submit').click();
    await expect(page.locator('#success-modal')).toBeVisible();
    expect(submittedPayload).toBeTruthy();
    expect(submittedPayload.petName).toBe('Cooper');
    expect(submittedPayload.location).toBe('47.6105, -122.3421');

    // Verify staged image data reflects the enhanced JPEG file
    await expect.poll(async () => {
      return await page.evaluate(() => {
        const img = (window as any).petspotrStagedImages?.[0];
        if (!img) return null;
        return {
          name: img.file?.name,
          type: img.file?.type,
          isEnhanced: img.isEnhanced,
        };
      });
    }).toEqual({
      name: 'cooper-journey.jpg',
      type: 'image/jpeg',
      isEnhanced: true,
    });

    // Verify upload payload reflects the enhanced JPEG file in images array and imageObject
    expect(submittedPayload.images).toBeDefined();
    expect(submittedPayload.images.length).toBe(1);
    expect(submittedPayload.images[0].object).toContain('cooper-journey.jpg');
    expect(submittedPayload.imageObject).toContain('cooper-journey.jpg');
  });
});
