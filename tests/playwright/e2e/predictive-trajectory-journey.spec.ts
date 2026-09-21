import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe('User Journey: Predictive Trajectory Modeling & Sector Prioritization', () => {
  const petId = `lost-dog-traj-${Date.now()}`;
  const petName = 'Buster';

  test('should complete the end-to-end predictive trajectory and sector prioritization user journey', async ({
    page,
    request,
  }) => {
    // ------------------------------------------------------------------------
    // Setup: Create lost pet report (dog, Seattle coordinates e.g. 47.6062, -122.3321)
    // ------------------------------------------------------------------------
    const petRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId,
        petName,
        species: 'Dog',
        breed: 'Golden Retriever',
        primaryColor: 'Golden',
        description: 'Friendly golden retriever lost in Seattle downtown area.',
        location: 'Downtown Seattle, WA',
        reporterEmail: 'buster-owner@example.com',
        phone: '(206) 555-0144',
        coordinates: {
          latitude: 47.6062,
          longitude: -122.3321,
        },
      },
    });
    expect([200, 201]).toContain(petRes.status());

    // ------------------------------------------------------------------------
    // Step 1: Add multiple chronological sightings with coordinates and direction headings
    // ------------------------------------------------------------------------
    const sighting1Res = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/sightings`, {
      data: {
        locationDescription: 'Near Pike Place Market',
        sightedAt: new Date(Date.now() - 90 * 60 * 1000).toISOString(),
        coordinates: {
          latitude: 47.609,
          longitude: -122.338,
        },
        movementDirection: 'Northwest',
        notes: 'Trotting northwest towards Belltown',
      },
    });
    expect([200, 201]).toContain(sighting1Res.status());

    const sighting2Res = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/sightings`, {
      data: {
        locationDescription: 'Near Olympic Sculpture Park',
        sightedAt: new Date(Date.now() - 30 * 60 * 1000).toISOString(),
        coordinates: {
          latitude: 47.616,
          longitude: -122.353,
        },
        movementDirection: 'North',
        notes: 'Seen near greenway corridor heading north',
      },
    });
    expect([200, 201]).toContain(sighting2Res.status());

    // Verify Direct API response has predictive model isochrones and barriers
    const trajApiRes = await request.get(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/trajectory`);
    expect(trajApiRes.status()).toBe(200);
    const trajData = await trajApiRes.json();
    expect(trajData.lostPetId).toBe(petId);
    expect(trajData.predictiveModel).toBeDefined();
    expect(Array.isArray(trajData.predictiveModel.isochrones)).toBe(true);
    expect(trajData.predictiveModel.isochrones.length).toBe(3);
    expect(trajData.predictiveModel.species).toBe('dog');

    // ------------------------------------------------------------------------
    // Step 2: Open Trajectory Modal, verify #pet-trajectory-container is displayed,
    // layer toggle radio buttons, default friction layer, toggle layers, verify legend chips
    // ------------------------------------------------------------------------
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await expect(petCard).toBeVisible();

    const trajBtn = petCard.locator('.btn-view-trajectory, button[data-action="view-trajectory"]').first();
    await expect(trajBtn).toBeVisible();
    await trajBtn.click();

    // Verify modal container is visible
    const trajModal = page.locator('#pet-trajectory-container');
    await expect(trajModal).toBeVisible();
    await expect(trajModal).not.toHaveClass(/hidden/);

    // Verify layer toggle radio buttons are present
    const toggleRadiogroup = page.locator('.trajectory-layer-toggle');
    await expect(toggleRadiogroup).toBeVisible();

    const btnFriction = page.locator('#btn-layer-friction');
    const btnStandard = page.locator('#btn-layer-standard');
    const btnClusters = page.locator('#btn-layer-clusters');

    await expect(btnFriction).toBeVisible();
    await expect(btnStandard).toBeVisible();
    await expect(btnClusters).toBeVisible();

    // Verify Friction Isochrones is checked by default
    await expect(btnFriction).toHaveAttribute('aria-checked', 'true');
    await expect(btnStandard).toHaveAttribute('aria-checked', 'false');
    await expect(btnClusters).toHaveAttribute('aria-checked', 'false');

    // Toggle to Standard Perimeter and verify ARIA state
    await btnStandard.click();
    await expect(btnStandard).toHaveAttribute('aria-checked', 'true');
    await expect(btnFriction).toHaveAttribute('aria-checked', 'false');
    await expect(btnClusters).toHaveAttribute('aria-checked', 'false');

    // Toggle to Hiding Clusters and verify ARIA state
    await btnClusters.click();
    await expect(btnClusters).toHaveAttribute('aria-checked', 'true');
    await expect(btnStandard).toHaveAttribute('aria-checked', 'false');
    await expect(btnFriction).toHaveAttribute('aria-checked', 'false');

    // Toggle back to Friction Isochrones
    await btnFriction.click();
    await expect(btnFriction).toHaveAttribute('aria-checked', 'true');

    // Verify legend chips (.legend-chip-core, .legend-chip-active, .legend-chip-containment)
    await expect(page.locator('.legend-chip-core')).toBeVisible();
    await expect(page.locator('.legend-chip-active')).toBeVisible();
    await expect(page.locator('.legend-chip-containment')).toBeVisible();

    // Close Trajectory Modal
    await page.locator('#btn-close-trajectory').click();
    await expect(trajModal).toBeHidden();

    // ------------------------------------------------------------------------
    // Step 3: Open Search Party modal for the pet, verify sector urgency badges,
    // click #btn-sort-sectors-urgency and verify sorting by priority
    // ------------------------------------------------------------------------
    const searchPartyBtn = petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first();
    await expect(searchPartyBtn).toBeVisible();
    await searchPartyBtn.click();

    const searchPartyModal = page.locator('#pet-search-party-container');
    await expect(searchPartyModal).toBeVisible();
    await expect(searchPartyModal).not.toHaveClass(/hidden/);

    // Wait for sector cards to render
    const sectorCards = page.locator('#search-party-sector-cards .sector-card');
    await expect(sectorCards).toHaveCount(4);

    // Verify presence of sector urgency badges (critical or high)
    const urgencyBadges = page.locator('[data-testid="sector-urgency-critical"], .badge-critical, [data-testid="sector-urgency-high"], .badge-high');
    await expect(urgencyBadges.first()).toBeVisible();

    // Verify sort button is present and not pressed initially
    const sortBtn = page.locator('#btn-sort-sectors-urgency');
    await expect(sortBtn).toBeVisible();
    await expect(sortBtn).toHaveAttribute('aria-pressed', 'false');

    // Click sort by urgency
    await sortBtn.click();
    await expect(sortBtn).toHaveAttribute('aria-pressed', 'true');

    // Verify the top sector card after sorting is critical or high priority
    const firstSectorCard = sectorCards.first();
    await expect(
      firstSectorCard.locator('[data-testid="sector-urgency-critical"], .badge-critical, [data-testid="sector-urgency-high"], .badge-high')
    ).toBeVisible();

    // ------------------------------------------------------------------------
    // Step 4: Select a sector, verify safety advisory if barrier is intersected,
    // claim the sector, verify status
    // ------------------------------------------------------------------------
    const claimBtn = firstSectorCard.locator('.btn-sector-action[data-action="claim-sector"], button[data-action="claim-sector"]').first();
    await expect(claimBtn).toBeVisible();
    await claimBtn.click();

    const claimModal = page.locator('#modal-claim-sector');
    await expect(claimModal).toBeVisible();
    await expect(claimModal).not.toHaveClass(/hidden/);

    // Verify safety advisory element exists; if barrier is intersected, verify advisory content
    const barrierAdvisory = page.locator('#sector-barrier-advisory');
    await expect(barrierAdvisory).toBeAttached();
    if (await barrierAdvisory.isVisible()) {
      await expect(barrierAdvisory).toContainText(/Safety Advisory/i);
    }

    // Fill volunteer alias and submit claim
    const aliasInput = page.locator('#claim-volunteer-alias');
    await expect(aliasInput).toBeVisible();
    await aliasInput.fill('Volunteer Alpha');

    const submitClaimBtn = page.locator('#btn-submit-claim');
    await expect(submitClaimBtn).toBeVisible();
    await submitClaimBtn.click();

    // Verify claim modal closes
    await expect(claimModal).toBeHidden();

    // Verify sector status transitions to Active Search and card reflects volunteer alias
    await expect(firstSectorCard.locator('.badge[class*="badge-status-"]')).toContainText('Active Search');
    await expect(firstSectorCard.locator('.sector-card-volunteer')).toContainText('Volunteer Alpha');
    await expect(firstSectorCard.locator('.btn-sector-action')).toHaveAttribute('data-action', 'clear-sector');

    // Verify volunteer counter in search party header increments
    await expect(page.locator('#search-party-volunteers-count')).toHaveText('1');

    // Close Search Party Modal
    await page.locator('#btn-close-search-party').click();
    await expect(searchPartyModal).toBeHidden();
  });
});
