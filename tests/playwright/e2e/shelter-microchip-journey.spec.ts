import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8102';

test.describe('User Journey: Shelter Intake & Microchip Matching Pipeline', () => {
  test('Test 1: Lost pet report wizard microchip entry, live feedback, and masked badge on /pets', async ({ page }) => {
    // 1. Navigate to /report-lost
    await page.goto(`${WEB_FRONTEND_URL}/report-lost`);
    await expect(page.locator('#lost-pet-form')).toBeVisible();

    const microchipInput = page.locator('#lost-pet-microchip');
    const microchipFeedback = page.locator('#microchip-feedback');
    await expect(microchipInput).toBeVisible();

    // 2. Type invalid microchip input format -> verify feedback guidance
    await microchipInput.fill('12345');
    await expect(microchipFeedback).toBeVisible();
    await expect(microchipFeedback).toHaveClass(/invalid/);
    await expect(microchipFeedback).toContainText('Standard microchips are 15 digits (ISO), 9 digits (Avid), or 10 alphanumeric (Euro)');

    // 3. Type valid microchip input -> verify HomeAgain registry detection feedback
    await microchipInput.fill('985141000123456');
    await expect(microchipFeedback).toHaveClass(/valid/);
    await expect(microchipFeedback).toContainText('Valid 15-Digit ISO Microchip • HomeAgain');

    // 4. Fill wizard fields and submit report
    const uniquePetName = `Rusty-${Date.now()}`;
    await page.locator('#petName').fill(uniquePetName);
    await page.locator('#species').selectOption('Dog');
    await page.locator('#breed').fill('Golden Retriever');
    await page.locator('#primaryColor').fill('Golden');

    // Step 1 -> Step 2 (Photo upload)
    await page.locator('#btn-next').click();
    await expect(page.locator('#wizard-step-2')).toBeVisible();

    // Step 2 -> Step 3 (Location)
    await page.locator('#btn-next').click();
    await expect(page.locator('#wizard-step-3')).toBeVisible();
    await page.locator('#location').fill('Capitol Hill, Seattle, WA');

    // Step 3 -> Step 4 (Contact)
    await page.locator('#btn-next').click();
    await expect(page.locator('#wizard-step-4')).toBeVisible();
    await page.locator('#reporterEmail').fill('rusty-owner@example.com');

    // Submit report
    await page.locator('#btn-submit').click();
    await expect(page.locator('#success-modal')).toBeVisible();

    // 5. Navigate to /pets and verify masked microchip badge
    await page.goto(`${WEB_FRONTEND_URL}/pets?query=${encodeURIComponent(uniquePetName)}`);
    const petCard = page.locator('.pet-card', { hasText: uniquePetName });
    await expect(petCard).toBeVisible();

    const chipBadge = petCard.locator('.badge-microchip');
    await expect(chipBadge).toBeVisible();
    await expect(chipBadge).toContainText('HomeAgain ••••3456');
  });

  test('Test 2: Found pet report wizard microchip entry, AKC Reunite / Avid validation, and Shelter Care custody option', async ({ page }) => {
    // 1. Navigate to /report-found
    await page.goto(`${WEB_FRONTEND_URL}/report-found`);
    await expect(page.locator('#found-pet-form')).toBeVisible();

    const foundMicrochipInput = page.locator('#found-pet-microchip');
    const foundMicrochipFeedback = page.locator('#found-microchip-feedback');
    await expect(foundMicrochipInput).toBeVisible();

    // 2. Test AKC Reunite (981010000999999) validation
    await foundMicrochipInput.fill('981010000999999');
    await expect(foundMicrochipFeedback).toBeVisible();
    await expect(foundMicrochipFeedback).toHaveClass(/valid/);
    await expect(foundMicrochipFeedback).toContainText('Valid 15-Digit ISO Microchip • AKC Reunite');

    // 3. Test Avid (123456789) validation
    await foundMicrochipInput.fill('123456789');
    await expect(foundMicrochipFeedback).toHaveClass(/valid/);
    await expect(foundMicrochipFeedback).toContainText('Valid 9-Digit Avid Microchip • Avid Registry');

    // 4. Verify #custodyStatus contains "Shelter Care" and is selectable
    const custodySelect = page.locator('#custodyStatus');
    await expect(custodySelect).toBeVisible();
    const shelterOption = custodySelect.locator('option[value="Shelter Care"]');
    await expect(shelterOption).toBeAttached();
    await expect(shelterOption).toHaveText('In Shelter Care');

    await custodySelect.selectOption('Shelter Care');
    await expect(custodySelect).toHaveValue('Shelter Care');
  });

  test('Test 3: Shelter intake ingestion endpoint POST /api/v1/shelter-intakes/ingest', async ({ request }) => {
    const intakePayload = {
      shelterId: 'shelter-sea-01',
      shelterName: 'Seattle Animal Shelter',
      shelterAddress: '2061 15th Ave W, Seattle, WA 98119',
      shelterPhone: '(206) 386-7387',
      intakeId: 'INT-2026-8819',
      intakeDate: new Date().toISOString(),
      animal: {
        species: 'Dog',
        breed: 'Golden Retriever',
        primaryColor: 'Golden',
        description: 'Found near Interbay',
        microchipId: '985141000123456',
        images: [
          {
            url: 'https://storage.petspotr.io/shelters/intake-8819.jpg',
            view: 'primary',
          },
        ],
      },
      location: {
        address: 'Interbay, Seattle, WA',
        latitude: 47.648,
        longitude: -122.378,
      },
    };

    const response = await request.post(`${WEB_FRONTEND_URL}/api/v1/shelter-intakes/ingest`, {
      data: intakePayload,
    });

    expect(response.status()).toBe(201);
    const body = await response.json();
    expect(body.status).toBe('created');
    expect(body.intakeId).toBe('INT-2026-8819');
    expect(body.shelterId).toBe('shelter-sea-01');
    expect(body.custodyStatus).toBe('Shelter Care');
    expect(body.microchipId).toBe('985141000123456');
    expect(body.microchipRegistry).toBe('HomeAgain');
  });

  test('Test 4: Match comparison dashboard /matches displaying deterministic match banner & shelter alert card', async ({ page }) => {
    const mockDeterministicMatches = [
      {
        matchId: 'match-det-8819',
        foundPetId: 'shelter-shelter-sea-01-INT-2026-8819',
        matchedPetId: 'lost-det-8819',
        score: 1.0,
        deterministicMatch: true,
        matchedMicrochip: 'HomeAgain ••••3456',
        status: 'PENDING_REVIEW',
        matchedAt: new Date().toISOString(),
        scores: {
          vector: 1.0,
          trait: 0.95,
          visual: 0.95,
          color: 0.95,
          spatial: 0.98,
          distanceMiles: 0.8,
          microchipMatch: 1.0,
        },
        lostPet: {
          petId: 'lost-det-8819',
          petName: 'Rusty',
          species: 'Dog',
          breed: 'Golden Retriever',
          primaryColor: 'Golden',
          location: 'Capitol Hill, Seattle, WA',
          imageUrl: 'https://storage.petspotr.io/images/lost/rusty-1.jpg',
          microchipId: '985141000123456',
          microchipRegistry: 'HomeAgain',
        },
        foundPet: {
          petId: 'shelter-shelter-sea-01-INT-2026-8819',
          species: 'Dog',
          breed: 'Golden Retriever',
          primaryColor: 'Golden',
          location: 'Interbay, Seattle, WA',
          custodyStatus: 'Shelter Care',
          shelterId: 'shelter-sea-01',
          shelterName: 'Seattle Animal Shelter',
          intakeId: 'INT-2026-8819',
          shelterPhone: '(206) 386-7387',
          shelterAddress: '2061 15th Ave W, Seattle, WA 98119',
          imageUrl: 'https://storage.petspotr.io/shelters/intake-8819.jpg',
          microchipId: '985141000123456',
          microchipRegistry: 'HomeAgain',
        },
      },
    ];

    await page.route('**/api/v1/matches*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockDeterministicMatches),
      });
    });

    await page.goto(`${WEB_FRONTEND_URL}/matches`);

    // Verify 100% Deterministic Microchip Match banner
    const matchCard = page.locator('.match-card[data-match-id="match-det-8819"]');
    await expect(matchCard).toBeVisible();

    const deterministicBanner = matchCard.locator('.badge-microchip-match');
    await expect(deterministicBanner).toBeVisible();
    await expect(deterministicBanner).toHaveText('🎯 100% Verified Microchip Match');

    // Verify Shelter Alert Card with Seattle Animal Shelter & INT-2026-8819
    const shelterCard = matchCard.locator('.shelter-alert-card');
    await expect(shelterCard).toBeVisible();
    await expect(shelterCard.locator('.shelter-alert-title')).toContainText('Seattle Animal Shelter');
    await expect(shelterCard.locator('.shelter-alert-title')).toContainText('INT-2026-8819');

    // Verify click-to-call button with tel: link
    const callBtn = shelterCard.locator('.btn-shelter-call');
    await expect(callBtn).toBeVisible();
    await expect(callBtn).toHaveAttribute('href', 'tel:(206) 386-7387');
    await expect(callBtn).toContainText('(206) 386-7387');

    // Verify directions button with Google Maps link
    const dirBtn = shelterCard.locator('.btn-shelter-dir');
    await expect(dirBtn).toBeVisible();
    await expect(dirBtn).toHaveAttribute('href', /maps\.google\.com.*Seattle/);
    await expect(dirBtn).toHaveAttribute('target', '_blank');
    await expect(dirBtn).toContainText('Directions to Shelter');
  });

  test('Test 5: Rejection of conflicting microchips from deterministic matching', async ({ page, request }) => {
    // 1. Ingest intake with conflicting microchip 981010000999999 (AKC Reunite)
    const conflictingIntakeId = `INT-CONFLICT-${Date.now()}`;
    const conflictingPayload = {
      shelterId: 'shelter-sea-01',
      shelterName: 'Seattle Animal Shelter',
      shelterAddress: '2061 15th Ave W, Seattle, WA 98119',
      shelterPhone: '(206) 386-7387',
      intakeId: conflictingIntakeId,
      intakeDate: new Date().toISOString(),
      animal: {
        species: 'Dog',
        breed: 'Golden Retriever',
        primaryColor: 'Golden',
        description: 'Pet with conflicting microchip',
        microchipId: '981010000999999',
        images: [{ url: 'https://storage.petspotr.io/shelters/conflict.jpg', view: 'primary' }],
      },
      location: {
        address: 'Interbay, Seattle, WA',
        latitude: 47.648,
        longitude: -122.378,
      },
    };

    const ingestResponse = await request.post(`${WEB_FRONTEND_URL}/api/v1/shelter-intakes/ingest`, {
      data: conflictingPayload,
    });
    expect(ingestResponse.status()).toBe(201);
    const ingestBody = await ingestResponse.json();
    expect(ingestBody.microchipId).toBe('981010000999999');
    expect(ingestBody.microchipRegistry).toBe('AKC Reunite');

    // 2. Verify non-match behavior: conflicting microchip candidates receive score 0.0 / mismatch veto
    // and must never produce a deterministic 100% match banner on /matches
    const conflictingCandidates = [
      {
        matchId: 'match-conflict-999',
        foundPetId: ingestBody.petId,
        matchedPetId: 'lost-det-8819',
        score: 0.0,
        deterministicMatch: false,
        status: 'REJECTED',
        matchedAt: new Date().toISOString(),
        scores: {
          vector: 0.0,
          trait: 0.0,
          spatial: 0.0,
          microchipMatch: 0.0,
        },
        lostPet: {
          petId: 'lost-det-8819',
          petName: 'Rusty',
          microchipId: '985141000123456',
        },
        foundPet: {
          petId: ingestBody.petId,
          microchipId: '981010000999999',
        },
      },
    ];

    await page.route('**/api/v1/matches*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(conflictingCandidates),
      });
    });

    await page.goto(`${WEB_FRONTEND_URL}/matches`);

    // Verify deterministic microchip banner is NOT rendered in match cards
    await expect(page.locator('.match-card .badge-microchip-match')).toHaveCount(0);
    await expect(page.locator('#matches-list-container .badge-microchip-match')).toHaveCount(0);

    // Verify that candidate with score 0.0 does not pass the default threshold (0.70)
    await expect(page.locator('.match-empty')).toBeVisible();
    await expect(page.locator('.match-card')).toHaveCount(0);
  });
});
