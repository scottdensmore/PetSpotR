import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

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

    // 4. Asserts primary CTAs are visible and interactive
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
    const sightingModal = page.locator('#modal-finder-sighting');
    await expect(sightingModal).toBeVisible();
    await expect(sightingModal).not.toHaveClass(/hidden/);
    await expect(page.locator('#sighting-modal-title')).toBeVisible();

    // Close sighting modal
    await page.locator('#modal-finder-sighting .btn-icon').click();
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
