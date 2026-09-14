import { test, expect } from '@playwright/test';

test.describe('User Journey: Match Dashboard & Multimodal AI Comparison', () => {
  test('Side-by-side comparison, multi-photo thumbnail switching, zoom modal, AI scores, and bilateral decision', async ({ page }) => {
    const mockMatches = [
      {
        matchId: 'match-e2e-101',
        foundPetId: 'found-e2e-101',
        matchedPetId: 'lost-e2e-101',
        score: 0.94,
        scores: {
          vector: 0.96,
          trait: 0.92,
          visual: 0.92,
          color: 0.90,
          spatial: 0.95,
          distanceMiles: 0.8,
        },
        status: 'PENDING_REVIEW',
        matchedAt: new Date().toISOString(),
        lostPet: {
          petId: 'lost-e2e-101',
          petName: 'Rusty',
          species: 'Dog',
          breed: 'Golden Retriever',
          primaryColor: 'Golden',
          location: 'Capitol Hill, Seattle, WA',
          imageUrl: 'https://storage.petspotr.io/images/lost/rusty-1.jpg',
          images: [
            {
              object: 'images/lost/rusty-1.jpg',
              url: 'https://storage.petspotr.io/images/lost/rusty-1.jpg',
              tag: 'primary',
            },
            {
              object: 'images/lost/rusty-2.jpg',
              url: 'https://storage.petspotr.io/images/lost/rusty-2.jpg',
              tag: 'coat',
            },
            {
              object: 'images/lost/rusty-3.jpg',
              url: 'https://storage.petspotr.io/images/lost/rusty-3.jpg',
              tag: 'collar',
            },
          ],
        },
        foundPet: {
          petId: 'found-e2e-101',
          species: 'Dog',
          breed: 'Golden Retriever',
          primaryColor: 'Golden',
          location: 'First Hill, Seattle, WA',
          imageUrl: 'https://storage.petspotr.io/images/found/found-1.jpg',
          images: [
            {
              object: 'images/found/found-1.jpg',
              url: 'https://storage.petspotr.io/images/found/found-1.jpg',
              tag: 'primary',
            },
            {
              object: 'images/found/found-2.jpg',
              url: 'https://storage.petspotr.io/images/found/found-2.jpg',
              tag: 'face',
            },
          ],
        },
      },
    ];

    // Intercept candidate matches API
    await page.route('**/api/v1/matches*', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockMatches),
      });
    });

    // Intercept match action endpoint
    let capturedAction: any = null;
    await page.route('**/api/v1/matches/action', async route => {
      capturedAction = route.request().postDataJSON();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'success',
          matchId: capturedAction?.matchId || 'match-e2e-101',
          action: capturedAction?.action || 'confirm',
        }),
      });
    });

    await page.goto('/matches');

    // Verify Match Card renders
    const card = page.locator('.match-card[data-match-id="match-e2e-101"]');
    await expect(card).toBeVisible();
    await expect(card.locator('.match-badge')).toContainText('94% HIGH CONFIDENCE MATCH');
    await expect(card.locator('.match-id')).toContainText('Match ID: match-e2e-101');

    // Verify Lost Pet Panel & Multi-Photo Thumbnail Strip
    const lostPanel = card.locator('.image-panel').first();
    await expect(lostPanel.locator('.pet-name')).toHaveText('Rusty (Golden Retriever)');
    await expect(lostPanel.locator('.pet-location')).toContainText('Capitol Hill, Seattle, WA');

    const lostThumbStrip = lostPanel.locator('.thumbnail-strip');
    await expect(lostThumbStrip).toBeVisible();
    const lostThumbs = lostThumbStrip.locator('.thumbnail-btn');
    await expect(lostThumbs).toHaveCount(3);

    // Initial state: 1st thumbnail active
    await expect(lostThumbs.nth(0)).toHaveAttribute('aria-current', 'true');
    await expect(lostThumbs.nth(1)).toHaveAttribute('aria-current', 'false');
    const lostMainImg = lostPanel.locator('.match-pet-image');
    await expect(lostMainImg).toHaveAttribute('src', 'https://storage.petspotr.io/images/lost/rusty-1.jpg');

    // Click 2nd thumbnail (coat)
    await lostThumbs.nth(1).click();
    await expect(lostThumbs.nth(1)).toHaveAttribute('aria-current', 'true');
    await expect(lostThumbs.nth(0)).toHaveAttribute('aria-current', 'false');
    await expect(lostMainImg).toHaveAttribute('src', 'https://storage.petspotr.io/images/lost/rusty-2.jpg');

    // Verify Zoom Preview modal updates to the active angle
    const zoomBtn = lostPanel.locator('.zoom-btn');
    await expect(zoomBtn).toHaveAttribute('data-src', 'https://storage.petspotr.io/images/lost/rusty-2.jpg');
    await zoomBtn.click();

    const zoomModal = page.locator('#zoom-modal');
    await expect(zoomModal).toBeVisible();
    await expect(page.locator('#zoomed-image')).toHaveAttribute('src', 'https://storage.petspotr.io/images/lost/rusty-2.jpg');

    // Press Escape to dismiss Zoom modal and check focus restoration
    await page.keyboard.press('Escape');
    await expect(zoomModal).toBeHidden();
    await expect(zoomBtn).toBeFocused();

    // Verify Found Pet Panel
    const foundPanel = card.locator('.image-panel').nth(1);
    await expect(foundPanel.locator('.pet-name')).toHaveText('Found Pet (Golden Retriever)');
    await expect(foundPanel.locator('.thumbnail-btn')).toHaveCount(2);

    // Verify Multimodal AI Vector Similarity Breakdown
    const vectorScore = card.locator('.score-progress.score-vector');
    await expect(vectorScore).toBeVisible();
    await expect(vectorScore).toHaveAttribute('value', '96');
    await expect(card.locator('.score-grid')).toContainText('✨ Multimodal AI Vector Match:');
    await expect(card.locator('.score-grid')).toContainText('Discrete Trait Match:');
    await expect(card.locator('.score-grid')).toContainText('Geospatial Proximity');

    // Verify Bilateral Action - Confirm Match
    const confirmBtn = card.locator('button[data-action="confirm"]');
    await expect(confirmBtn).toBeVisible();
    await confirmBtn.click();

    await expect.poll(() => capturedAction).toEqual({
      matchId: 'match-e2e-101',
      action: 'confirm',
    });

    const actionModal = page.locator('#match-action-modal');
    await expect(actionModal).toBeVisible();
    await expect(actionModal.locator('#action-modal-title')).toHaveText(/Match confirmed/i);

    // Close action modal
    await actionModal.locator('.modal-close').click();
    await expect(actionModal).toBeHidden();

    // Verify Bilateral Action - Reject Match
    capturedAction = null;
    const rejectBtn = card.locator('button[data-action="reject"]');
    await expect(rejectBtn).toBeVisible();
    await rejectBtn.click();

    await expect.poll(() => capturedAction).toEqual({
      matchId: 'match-e2e-101',
      action: 'reject',
    });
  });
});
