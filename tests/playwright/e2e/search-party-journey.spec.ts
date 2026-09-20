import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe.serial('User Journey: Community Search Party & Sector Claiming', () => {
  const petId = `lost-party-journey-${Date.now()}`;
  const petName = 'Barnaby';

  test.beforeAll(async ({ request }) => {
    // Seed lost pet report with valid location coordinates in the live backend
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId,
        petName,
        species: 'Dog',
        breed: 'Beagle',
        primaryColor: 'Tricolor',
        description: 'Friendly beagle participating in community search party journey test.',
        location: 'Capitol Hill, Seattle, WA',
        reporterEmail: 'barnaby-owner@example.com',
        phone: '(206) 555-0199',
        coordinates: {
          latitude: 47.6152,
          longitude: -122.3211,
        },
      },
    });
    expect([200, 201]).toContain(res.status());
  });

  test('Test 1: Initialize search party for a lost pet via POST /api/v1/lost-pets/{petID}/search-party', async ({ request }) => {
    const initRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/search-party`, {
      data: { sectorCount: 4 },
    });
    expect([200, 201]).toContain(initRes.status());

    const party = await initRes.json();
    expect(party.partyId).toBeDefined();
    expect(party.lostPetId).toBe(petId);
    expect(Array.isArray(party.sectors)).toBe(true);
    expect(party.sectors).toHaveLength(4);
    expect(party.coveragePercentage).toBe(0);
    expect(party.activeVolunteersCount).toBe(0);
    expect(party.activeAssignments).toHaveLength(0);

    for (const sector of party.sectors) {
      expect(sector.status).toBe('unassigned');
      expect(sector.name).toBeDefined();
      expect(sector.sectorId).toBeDefined();
      expect(Array.isArray(sector.polygonPoints)).toBe(true);
      expect(sector.polygonPoints.length).toBeGreaterThanOrEqual(3);
    }
  });

  test('Test 2: Navigate to /pets, trigger search party modal/container, verify map, sector cards grid, and coverage progress bar', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await expect(petCard).toBeVisible();

    const spBtn = petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first();
    await expect(spBtn).toBeVisible();
    await spBtn.click();

    const modal = page.locator('#pet-search-party-container');
    await expect(modal).toBeVisible();
    await expect(modal).not.toHaveClass(/hidden/);

    const map = page.locator('#search-party-map');
    await expect(map).toBeVisible();

    const sectorCards = page.locator('#search-party-sector-cards .sector-card');
    await expect(sectorCards).toHaveCount(4);

    const coverageTrack = page.locator('.search-party-coverage-track');
    await expect(coverageTrack).toBeVisible();

    const coverageBar = page.locator('#search-party-coverage-bar');
    await expect(coverageBar).toBeAttached();
    await expect(coverageBar).toHaveAttribute('role', 'progressbar');
    await expect(coverageBar).toHaveAttribute('aria-valuenow', '0');

    const coverageLabel = page.locator('#search-party-coverage-label');
    await expect(coverageLabel).toHaveText('0%');

    const volunteersCount = page.locator('#search-party-volunteers-count');
    await expect(volunteersCount).toHaveText('0');

    const sectorsCount = page.locator('#search-party-sectors-count');
    await expect(sectorsCount).toHaveText('4');
  });

  test('Test 3: Claim a sector via #modal-claim-sector, entering an alias and verifying status updates to Active Search and card displays alias', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await expect(petCard).toBeVisible();

    const spBtn = petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first();
    await spBtn.click();

    const modal = page.locator('#pet-search-party-container');
    await expect(modal).toBeVisible();

    const firstSectorCard = page.locator('#search-party-sector-cards .sector-card').first();
    await expect(firstSectorCard).toBeVisible();
    await expect(firstSectorCard.locator('.sector-card-volunteer')).toContainText('No volunteer assigned');

    const claimBtn = firstSectorCard.locator('.btn-sector-action[data-action="claim-sector"], button[data-action="claim-sector"]').first();
    await expect(claimBtn).toBeVisible();
    await claimBtn.click();

    const claimModal = page.locator('#modal-claim-sector');
    await expect(claimModal).toBeVisible();
    await expect(claimModal).not.toHaveClass(/hidden/);

    const aliasInput = page.locator('#claim-volunteer-alias');
    await expect(aliasInput).toBeVisible();
    await aliasInput.fill('Volunteer Alpha');

    const submitClaimBtn = page.locator('#btn-submit-claim');
    await expect(submitClaimBtn).toBeVisible();
    await submitClaimBtn.click();

    await expect(claimModal).toBeHidden();

    // Verify sector status updates to "Active Search" and card displays "Volunteer Alpha"
    await expect(firstSectorCard.locator('.badge[class*="badge-status-"]')).toContainText('Active Search');
    await expect(firstSectorCard.locator('.sector-card-volunteer')).toContainText('Volunteer Alpha');
    await expect(firstSectorCard.locator('.btn-sector-action')).toHaveAttribute('data-action', 'clear-sector');

    // Verify volunteer counter updated
    await expect(page.locator('#search-party-volunteers-count')).toHaveText('1');
  });

  test('Test 4: Mark sector as cleared with notes via #modal-clear-sector, verifying status transitions to Cleared and coverage increases', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await expect(petCard).toBeVisible();

    const spBtn = petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first();
    await spBtn.click();

    const modal = page.locator('#pet-search-party-container');
    await expect(modal).toBeVisible();

    const firstSectorCard = page.locator('#search-party-sector-cards .sector-card').first();
    await expect(firstSectorCard).toBeVisible();
    await expect(firstSectorCard.locator('.badge[class*="badge-status-"]')).toContainText('Active Search');

    const clearBtn = firstSectorCard.locator('.btn-sector-action[data-action="clear-sector"], button[data-action="clear-sector"]').first();
    await expect(clearBtn).toBeVisible();
    await clearBtn.click();

    const clearModal = page.locator('#modal-clear-sector');
    await expect(clearModal).toBeVisible();
    await expect(clearModal).not.toHaveClass(/hidden/);

    const statusSelect = page.locator('#clear-sector-status');
    await expect(statusSelect).toBeVisible();
    await statusSelect.selectOption('cleared');

    const notesInput = page.locator('#clear-sector-notes');
    await expect(notesInput).toBeVisible();
    await notesInput.fill('Searched all alleyways and yards thoroughly. Area is clear.');

    const submitClearBtn = page.locator('#btn-submit-clear');
    await expect(submitClearBtn).toBeVisible();
    await submitClearBtn.click();

    await expect(clearModal).toBeHidden();

    // Verify sector status transitions to "Cleared" and coverage percentage increases (e.g. 25%)
    await expect(firstSectorCard.locator('.badge[class*="badge-status-"]')).toContainText('Cleared');
    await expect(page.locator('#search-party-coverage-label')).toHaveText('25%');
    await expect(page.locator('#search-party-coverage-bar')).toHaveAttribute('aria-valuenow', '25');
  });

  test('Test 5: Verify REST API response GET /api/v1/lost-pets/{petID}/search-party matches updated status and Zero-PII alias', async ({ request }) => {
    const res = await request.get(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/search-party`);
    expect(res.status()).toBe(200);

    const party = await res.json();
    expect(party.lostPetId).toBe(petId);
    expect(party.coveragePercentage).toBe(25);
    expect(party.sectors).toHaveLength(4);

    // Verify the updated sector status is cleared
    const clearedSector = party.sectors.find((s: { status: string }) => s.status === 'cleared');
    expect(clearedSector).toBeDefined();

    // Verify active assignment contains the Zero-PII alias and updated status
    expect(party.activeAssignments).toHaveLength(1);
    const assignment = party.activeAssignments[0];
    expect(assignment.status).toBe('cleared');
    expect(assignment.volunteerAlias).toBe('Volunteer Alpha');
    expect(assignment.clearanceNotes).toBe('Searched all alleyways and yards thoroughly. Area is clear.');
    expect(assignment.assignmentId).toBeDefined();
    expect(assignment.claimedAt).toBeDefined();

    // Verify Zero-PII guarantee: no email addresses or phone numbers
    expect(assignment.volunteerAlias).not.toContain('@');
    expect(assignment.volunteerAlias).not.toMatch(/\d{3}-\d{3}-\d{4}/);
  });

  test('should initialize a search party, claim a sector, and mark it cleared in a single session', async ({ page, request }) => {
    const flowPetId = `lost-party-flow-${Date.now()}`;
    const flowRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId: flowPetId,
        petName: 'Scout',
        species: 'Dog',
        breed: 'Border Collie',
        primaryColor: 'Black',
        secondaryColor: 'White',
        description: 'Active border collie on search party flow test.',
        location: 'Green Lake Park, Seattle, WA',
        reporterEmail: 'scout-owner@example.com',
        phone: '(206) 555-0188',
        coordinates: {
          latitude: 47.6815,
          longitude: -122.3295,
        },
      },
    });
    expect([200, 201]).toContain(flowRes.status());

    // 1. Visit /pets
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    // 2. Click Initialize Search Party on Scout's card
    const flowCard = page.locator(`article.pet-card[data-pet-id="${flowPetId}"]`);
    await expect(flowCard).toBeVisible();

    const flowSpBtn = flowCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first();
    await flowSpBtn.click();

    // 3. Verify Map and Sectors appear
    const modal = page.locator('#pet-search-party-container');
    await expect(modal).toBeVisible();
    await expect(page.locator('#search-party-map')).toBeVisible();

    const sectorCards = page.locator('#search-party-sector-cards .sector-card');
    await expect(sectorCards).toHaveCount(4);

    // 4. Click a sector, claim it
    const claimCard = sectorCards.first();
    await claimCard.locator('.btn-sector-action[data-action="claim-sector"]').click();

    const claimModal = page.locator('#modal-claim-sector');
    await expect(claimModal).toBeVisible();
    await page.locator('#claim-volunteer-alias').fill('Volunteer Bravo');
    await page.locator('#btn-submit-claim').click();
    await expect(claimModal).toBeHidden();

    // 5. Verify sector card updates to Active Search and shows Volunteer Bravo
    await expect(claimCard.locator('.badge[class*="badge-status-"]')).toContainText('Active Search');
    await expect(claimCard.locator('.sector-card-volunteer')).toContainText('Volunteer Bravo');

    // 6. Mark sector as cleared with notes
    await claimCard.locator('.btn-sector-action[data-action="clear-sector"]').click();
    const clearModal = page.locator('#modal-clear-sector');
    await expect(clearModal).toBeVisible();
    await page.locator('#clear-sector-status').selectOption('cleared');
    await page.locator('#clear-sector-notes').fill('Green Lake perimeter thoroughly searched and clear.');
    await page.locator('#btn-submit-clear').click();
    await expect(clearModal).toBeHidden();

    // 7. Verify coverage percentage increases
    await expect(claimCard.locator('.badge[class*="badge-status-"]')).toContainText('Cleared');
    await expect(page.locator('#search-party-coverage-label')).toHaveText('25%');
    await expect(page.locator('#search-party-coverage-bar')).toHaveAttribute('aria-valuenow', '25');

    // 8. Close modal
    await page.locator('#btn-close-search-party').click();
    await expect(modal).toBeHidden();
  });
});
