import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8102';

test.describe('User Journey: Pet Recovery Poster & Dynamic Social Sharing', () => {
  test.beforeAll(async ({ request }) => {
    // Seed demo-lost-1 pet report in live backend if not already present
    await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId: 'demo-lost-1',
        petName: 'Rusty',
        species: 'Dog',
        breed: 'Golden Retriever',
        primaryColor: 'Golden',
        description: 'Friendly golden retriever wearing blue collar.',
        location: 'Capitol Hill, Seattle, WA',
        reporterEmail: 'owner@example.com',
        phone: '(206) 555-0199',
      },
    });
  });

  test.beforeEach(async ({ context }) => {
    try {
      await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    } catch {
      // Ignore unsupported permissions across non-chromium browser contexts
    }
    try {
      await context.grantPermissions(['geolocation'], { origin: WEB_FRONTEND_URL });
      await context.setGeolocation({ latitude: 47.6152, longitude: -122.3211 });
    } catch (e) {
      console.warn('Geolocation grant warning:', e);
    }
  });

  test('should open poster customizer, update reward and emergency notes, and verify live preview', async ({ page }) => {
    // 1. Navigate to /pets
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    // 2. Find an active lost pet card
    const lostPetCard = page.locator('article.pet-card[data-status="LOST"], article.pet-card[data-status="lost"]').first();
    await expect(lostPetCard).toBeVisible();

    // 3. Click "📄 Poster & Share" button
    const posterShareBtn = lostPetCard.locator('.btn-poster-share');
    await expect(posterShareBtn).toBeVisible();
    await posterShareBtn.click();

    // 4. Assert #poster-modal is visible and not hidden
    const posterModal = page.locator('#poster-modal');
    await expect(posterModal).toBeVisible();
    await expect(posterModal).not.toHaveClass(/hidden/);

    // 5. Types $1,000 into #poster-reward-input
    const rewardInput = page.locator('#poster-reward-input');
    await expect(rewardInput).toBeVisible();
    await rewardInput.fill('$1,000');

    // 6. Types emergency notes into #poster-emergency-input
    const emergencyInput = page.locator('#poster-emergency-input');
    await expect(emergencyInput).toBeVisible();
    await emergencyInput.fill('Needs daily medication - do not chase!');

    // 7. Toggles #poster-phone-toggle and types (206) 555-0199 into #poster-phone-input
    const phoneToggle = page.locator('#poster-phone-toggle');
    await phoneToggle.check();
    await expect(phoneToggle).toBeChecked();

    const phoneInput = page.locator('#poster-phone-input');
    await expect(phoneInput).toBeVisible();
    await phoneInput.fill('(206) 555-0199');

    // 8. Waits for debounced preview update; asserts #link-standalone-poster href contains reward=, emergency=, and phone=
    const standaloneLink = page.locator('#link-standalone-poster');
    await expect(standaloneLink).toHaveAttribute('href', /reward=/);
    await expect(standaloneLink).toHaveAttribute('href', /emergency=/);
    await expect(standaloneLink).toHaveAttribute('href', /phone=/);

    // 9. Verifies #poster-preview-frame src contains the updated query parameters
    const previewFrame = page.locator('#poster-preview-frame');
    await expect(previewFrame).toHaveAttribute('src', /reward=/);
    await expect(previewFrame).toHaveAttribute('src', /emergency=/);
    await expect(previewFrame).toHaveAttribute('src', /phone=/);

    // 10. Directly visits the standalone flyer URL and asserts flyer DOM elements
    const flyerHref = await standaloneLink.getAttribute('href');
    expect(flyerHref).toBeTruthy();
    const flyerURL = new URL(flyerHref!, WEB_FRONTEND_URL).toString();
    await page.goto(flyerURL);

    // Assert flyer DOM elements: .printable-poster, .poster-header, .poster-reward-banner, .poster-emergency-note, .poster-tear-tabs, .tear-tab
    await expect(page.locator('.printable-poster')).toBeVisible();
    await expect(page.locator('.poster-header')).toBeVisible();
    await expect(page.locator('.poster-header')).toContainText('LOST');

    const rewardBanner = page.locator('.poster-reward-banner');
    await expect(rewardBanner).toBeVisible();
    await expect(rewardBanner).toContainText('$1,000');

    const emergencyNote = page.locator('.poster-emergency-note');
    await expect(emergencyNote).toBeVisible();
    await expect(emergencyNote).toContainText('Needs daily medication - do not chase!');

    const tearTabsContainer = page.locator('.poster-tear-tabs');
    await expect(tearTabsContainer).toBeVisible();

    const tearTabs = page.locator('.tear-tab');
    await expect(tearTabs.first()).toBeVisible();
    await expect(tearTabs.first()).toContainText('(206) 555-0199');

    // 11. Verifies that @page size is letter portrait and print media styles are defined in the stylesheet
    const stylesheetCheck = await page.evaluate(async () => {
      let hasPageRule = false;
      let hasMediaPrint = false;

      for (const sheet of Array.from(document.styleSheets)) {
        try {
          for (const rule of Array.from(sheet.cssRules || [])) {
            const ruleText = rule.cssText.toLowerCase();
            if (rule.type === CSSRule.PAGE_RULE || ruleText.startsWith('@page')) {
              hasPageRule = true;
            }
            if (rule.type === CSSRule.MEDIA_RULE && (rule as CSSMediaRule).conditionText === 'print') {
              hasMediaPrint = true;
            }
          }
        } catch {
          // Cross-origin stylesheet security restrictions if any
        }
      }

      // Verify @page size is letter portrait in stylesheet text
      const res = await fetch('/static/css/styles.css');
      const cssContent = await res.text();
      const hasPageSize = cssContent.includes('@page') && cssContent.includes('size: letter portrait');

      return { hasPageRule, hasMediaPrint, hasPageSize };
    });

    expect(stylesheetCheck.hasPageRule).toBe(true);
    expect(stylesheetCheck.hasMediaPrint).toBe(true);
    expect(stylesheetCheck.hasPageSize).toBe(true);

    // Verify print media behavior
    await page.emulateMedia({ media: 'print' });
    const posterDisplay = await page.locator('.printable-poster').evaluate((el) => window.getComputedStyle(el).display);
    expect(posterDisplay).toBe('flex');
    const toolbarDisplay = await page.locator('.poster-actions-toolbar').evaluate((el) => window.getComputedStyle(el).display);
    expect(toolbarDisplay).toBe('none');
    await page.emulateMedia({ media: 'screen' });
  });

  test('should navigate to mobile finder landing page via shortlink and render action controls', async ({ page }) => {
    // 1. Navigate to /p/demo-lost-1
    await page.goto(`${WEB_FRONTEND_URL}/p/demo-lost-1`);

    // 2. Asserts OpenGraph tags
    const ogImage = page.locator('meta[property="og:image"]');
    await expect(ogImage).toHaveAttribute('content', /share-card\.svg/);

    const ogTitle = page.locator('meta[property="og:title"]');
    await expect(ogTitle).toHaveAttribute('content', /Rusty|Help Find/i);

    const ogDesc = page.locator('meta[property="og:description"]');
    await expect(ogDesc).toHaveAttribute('content', /Rusty|Help find/i);

    // 3. Asserts Twitter card meta tag
    const twitterCard = page.locator('meta[name="twitter:card"][content="summary_large_image"]');
    await expect(twitterCard).toBeAttached();

    // 4. Asserts primary CTAs and camera elements are visible and interactive
    const cameraInput = page.locator('#finder-camera-input');
    await expect(cameraInput).toBeAttached();
    await expect(cameraInput).toHaveAttribute('accept', 'image/*');
    await expect(cameraInput).toHaveAttribute('capture', 'environment');

    const manualFallbackLink = page.locator('.finder-manual-link');
    await expect(manualFallbackLink).toBeVisible();
    await expect(manualFallbackLink).toHaveAttribute('href', /\/report-found\?matchedPetId=demo-lost-1/);

    const btnFound = page.locator('#btn-finder-found');
    const btnMessage = page.locator('#btn-finder-message');
    const btnSighting = page.locator('#btn-finder-sighting');

    await expect(btnFound).toBeVisible();
    await expect(btnFound).toBeEnabled();
    await expect(btnFound).toHaveAttribute('href', /\/report-found/);

    await expect(btnMessage).toBeVisible();
    await expect(btnMessage).toBeEnabled();

    await expect(btnSighting).toBeVisible();
    await expect(btnSighting).toBeEnabled();

    // 5. Clicks #btn-finder-message and verifies the message modal is displayed
    await btnMessage.click();
    const messageModal = page.locator('#modal-finder-message');
    await expect(messageModal).toBeVisible();
    await expect(messageModal).not.toHaveClass(/hidden/);
    await expect(page.locator('#contact-modal-title')).toBeVisible();

    // Close contact modal
    await page.locator('#modal-finder-message .btn-icon').click();
    await expect(messageModal).toBeHidden();

    // 6. Clicks #btn-finder-sighting and verifies the sighting modal is displayed
    await btnSighting.click();
    const sightingModal = page.locator('#modal-report-sighting');
    await expect(sightingModal).toBeVisible();
    await expect(sightingModal).not.toHaveClass(/hidden/);
    await expect(page.locator('#sighting-modal-title')).toBeVisible();

    // Close sighting modal
    await page.locator('#modal-report-sighting #btn-close-sighting-modal').click();
    await expect(sightingModal).toBeHidden();
  });

  test('should handle camera capture, geolocation, and real API submissions on mobile finder landing page', async ({ page }) => {
    // 1. Navigate to /p/demo-lost-1
    await page.goto(`${WEB_FRONTEND_URL}/p/demo-lost-1`);

    // 2. Asserts camera input and accessible fallback link
    const cameraInput = page.locator('#finder-camera-input');
    await expect(cameraInput).toBeAttached();

    const manualFallbackLink = page.locator('.finder-manual-link');
    await expect(manualFallbackLink).toBeVisible();

    // 3. Verifies clicking #btn-finder-found triggers camera input click
    await page.evaluate(() => {
      (window as any).__cameraInputClicked = false;
      document.getElementById('finder-camera-input')?.addEventListener('click', () => {
        (window as any).__cameraInputClicked = true;
      });
    });
    await page.locator('#btn-finder-found').click();
    const wasClicked = await page.evaluate(() => (window as any).__cameraInputClicked);
    expect(wasClicked).toBe(true);

    // 4. Sets input file on #finder-camera-input, verifies modal opens, preview renders, and submits report
    await cameraInput.setInputFiles({
      name: 'captured-pet.jpg',
      mimeType: 'image/jpeg',
      buffer: Buffer.from('fake-jpeg-image-data-for-finder'),
    });

    const cameraModal = page.locator('#modal-finder-camera');
    await expect(cameraModal).toBeVisible();
    await expect(cameraModal).not.toHaveClass(/hidden/);

    const previewImg = page.locator('#camera-preview-img');
    await expect(previewImg).toBeVisible();
    await expect(previewImg).not.toHaveClass(/hidden/);

    const gpsStatus = page.locator('#camera-gps-status');
    await expect(gpsStatus).toBeVisible();
    await expect(gpsStatus).toContainText(/GPS Acquired|Acquiring GPS/i);

    // Submit camera found report and assert real API call
    const [cameraReq] = await Promise.all([
      page.waitForRequest(req => req.url().includes('/api/v1/found-pets') && req.method() === 'POST'),
      page.locator('#btn-submit-camera-report').click(),
    ]);
    const cameraPayload = JSON.parse(cameraReq.postData() || '{}');
    expect(cameraPayload.species).toBe('Dog');
    expect(cameraPayload.imageObject).toContain('images/found-pets/');
    expect(cameraPayload.images).toBeDefined();
    expect(cameraPayload.images.length).toBeGreaterThan(0);
    if (cameraPayload.coordinates) {
      expect(cameraPayload.coordinates.latitude).toBeCloseTo(47.6152, 2);
      expect(cameraPayload.coordinates.longitude).toBeCloseTo(-122.3211, 2);
    }

    const cameraFeedback = page.locator('#camera-feedback');
    await expect(cameraFeedback).toBeVisible();
    await expect(cameraFeedback).toContainText('Found pet report submitted');
    await expect(cameraModal).toBeHidden();

    // 5. Submit private message to owner and assert real API call
    await page.locator('#btn-finder-message').click();
    const messageModal = page.locator('#modal-finder-message');
    await expect(messageModal).toBeVisible();

    await page.locator('#contact-sender-name').fill('Neighbor Sam');
    await page.locator('#contact-sender-phone').fill('sam@example.com');
    await page.locator('#contact-message-body').fill('I spotted your golden retriever near 12th & Pine!');

    const [contactReq] = await Promise.all([
      page.waitForRequest(req => req.url().includes('/api/v1/reunions/contact') && req.method() === 'POST'),
      page.locator('#btn-submit-contact-message').click(),
    ]);
    const contactPayload = JSON.parse(contactReq.postData() || '{}');
    expect(contactPayload.senderEmail).toBe('sam@example.com');
    expect(contactPayload.message).toContain('I spotted your golden retriever');
    expect(contactPayload.matchId).toBe('demo-lost-1');

    const contactFeedback = page.locator('#contact-feedback');
    await expect(contactFeedback).toBeVisible();
    await expect(contactFeedback).toContainText('Message dispatched');
    await expect(messageModal).toBeHidden();

    // 6. Submit quick sighting report and assert real API call
    await page.locator('#btn-finder-sighting').click();
    const sightingModal = page.locator('#modal-report-sighting');
    await expect(sightingModal).toBeVisible();

    await page.locator('#sighting-geolocation-btn').click();
    await expect(page.locator('#sighting-gps-status')).toContainText('GPS Acquired');

    await page.locator('#sighting-location').fill('15th Ave & Pine St');
    await page.locator('#sighting-notes').fill('Trotting safely towards Volunteer Park');

    const [sightingReq] = await Promise.all([
      page.waitForRequest(req => req.url().includes('/api/v1/lost-pets/demo-lost-1/sightings') && req.method() === 'POST'),
      page.locator('#btn-submit-sighting').click(),
    ]);
    const sightingPayload = JSON.parse(sightingReq.postData() || '{}');
    expect(sightingPayload.locationDescription).toBe('15th Ave & Pine St');
    expect(sightingPayload.notes).toContain('Trotting safely');
    if (sightingPayload.coordinates) {
      expect(sightingPayload.coordinates.latitude).toBeCloseTo(47.6152, 2);
      expect(sightingPayload.coordinates.longitude).toBeCloseTo(-122.3211, 2);
    }

    const toast = page.locator('#toast-container .toast-item, .toast-item');
    await expect(toast).toBeVisible();
    await expect(toast).toContainText('Sighting reported');
    await expect(sightingModal).toBeHidden();
  });

  test('should copy share link with toast feedback when Web Share is unsupported', async ({ page }) => {
    // 1. Navigates to /pets
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    // 2. Opens #poster-modal on a lost pet card
    const lostPetCard = page.locator('article.pet-card[data-status="LOST"], article.pet-card[data-status="lost"]').first();
    await expect(lostPetCard).toBeVisible();
    await lostPetCard.locator('.btn-poster-share').click();

    const modal = page.locator('#poster-modal');
    await expect(modal).toBeVisible();
    await expect(modal).not.toHaveClass(/hidden/);

    // 3. Clicks #btn-copy-shortlink
    const btnCopy = page.locator('#btn-copy-shortlink');
    await expect(btnCopy).toBeVisible();
    await btnCopy.click();

    // 4. Asserts toast notification appears with text containing "Link copied to clipboard!"
    const toast = page.locator('#toast-container .toast-item, .toast-item');
    await expect(toast).toBeVisible();
    await expect(toast).toContainText('Link copied to clipboard!');

    // 5. Clicks close button #btn-close-poster-modal -> asserts #poster-modal is hidden
    const btnClose = page.locator('#btn-close-poster-modal');
    await expect(btnClose).toBeVisible();
    await btnClose.click();
    await expect(modal).toBeHidden();
    await expect(modal).toHaveClass(/hidden/);
  });
});
