import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Milestone 12.1: Smart Camera Trap & Home Security Webhook / Edge Frame Ingestion Journey', () => {
  test('Complete 6-Step Smart Camera Trap & Edge Frame Ingestion User Journey', async ({ page, request }) => {
    const cameraName = 'Wilderness Ridge Trail Cam #7';
    const targetPetId = 'pet-lost-cam-99';

    // -------------------------------------------------------------------------
    // Step 1: Register Smart Camera Trap Node via UI (#camera-register-form)
    // -------------------------------------------------------------------------
    await test.step('Step 1: Register Smart Camera Trap Node via UI (#camera-register-form)', async () => {
      await page.goto(`${WEB_FRONTEND_URL}/cameras`);
      await page.waitForLoadState('networkidle');

      await page.locator('#camera-name').fill(cameraName);
      await page.locator('#camera-vendor').selectOption('TRAIL_CAM');
      await page.locator('#camera-target-pet').fill(targetPetId);
      await page.locator('#camera-lat').fill('47.6101');
      await page.locator('#camera-lng').fill('-122.3421');
      await page.locator('#camera-auto-promote').fill('0');

      await page.locator('#btn-register-camera').click();

      await expect(page.locator('#camera-webhook-credentials')).toBeVisible();
      await expect(page.locator('#issued-webhook-token')).toContainText('cam_tok_');
      await expect(page.locator('#camera-nodes-list')).toContainText(cameraName);
    });

    // -------------------------------------------------------------------------
    // Step 2: Transmit Night-Vision IR Motion Frame via Simulator (#webhook-simulator-form)
    // -------------------------------------------------------------------------
    await test.step('Step 2: Transmit Night-Vision IR Motion Frame via Simulator (#webhook-simulator-form)', async () => {
      await page.locator('#sim-scene-preset').selectOption('night_ir_dog');
      await page.locator('#sim-species-hint').selectOption('DOG');
      await page.locator('#sim-traits-input').fill('Reflective collar, limping left paw');

      await page.locator('#btn-send-webhook-frame').click();

      const pendingCard = page
        .locator('#camera-captures-grid .camera-capture-card', { hasText: cameraName })
        .filter({ has: page.locator('.badge-capture-pending') })
        .first();

      await expect(pendingCard).toBeVisible();
      await expect(pendingCard.locator('.badge-capture-pending')).toHaveText('PENDING_REVIEW');
      await expect(pendingCard.locator('.capture-viewport .roi-bounding-box')).toBeVisible();
      await expect(pendingCard).toContainText('Reflective collar');
    });

    // -------------------------------------------------------------------------
    // Step 3: Transmit Static Repeat Frame & Verify Perceptual dHash Duplicate Suppression
    // -------------------------------------------------------------------------
    await test.step('Step 3: Transmit Static Repeat Frame & Verify Perceptual dHash Duplicate Suppression', async () => {
      await page.locator('#sim-scene-preset').selectOption('static_repeat');
      await page.locator('#btn-send-webhook-frame').click();

      const duplicateCard = page
        .locator('#camera-captures-grid .camera-capture-card', { hasText: cameraName })
        .filter({ has: page.locator('.badge-capture-duplicate') })
        .first();

      await expect(duplicateCard).toBeVisible();
      await expect(duplicateCard.locator('.badge-capture-duplicate')).toHaveText('DUPLICATE_SUPPRESSED');

      const nodeCard = page.locator('#camera-nodes-list .camera-node-card', { hasText: cameraName }).first();
      await expect(nodeCard).toContainText('Suppressed Duplicates: 1');

      // Toggle #filter-exclude-duplicates checkbox and assert .badge-capture-duplicate cards are hidden, then uncheck to restore
      const filterExcludeCheckbox = page.locator('#filter-exclude-duplicates');
      await filterExcludeCheckbox.check();
      await expect(page.locator('#camera-captures-grid .badge-capture-duplicate')).toHaveCount(0);

      await filterExcludeCheckbox.uncheck();
      await expect(duplicateCard).toBeVisible();
    });

    // -------------------------------------------------------------------------
    // Step 4: Toggle Night-Vision IR Phosphor Display Mode
    // -------------------------------------------------------------------------
    await test.step('Step 4: Toggle Night-Vision IR Phosphor Display Mode', async () => {
      const toggleIrBtn = page.locator('#toggle-ir-phosphor');
      await toggleIrBtn.click();

      await expect(page.locator('#camera-command-center')).toHaveClass(/ir-phosphor-mode/);
      await expect(toggleIrBtn).toHaveAttribute('aria-pressed', 'true');
    });

    // -------------------------------------------------------------------------
    // Step 5: One-Click Promote Pending Capture to Verified Pet Sighting
    // -------------------------------------------------------------------------
    await test.step('Step 5: One-Click Promote Pending Capture to Verified Pet Sighting', async () => {
      const pendingCard = page
        .locator('#camera-captures-grid .camera-capture-card', { hasText: cameraName })
        .filter({ has: page.locator('.badge-capture-pending') })
        .first();

      const captureId = await pendingCard.getAttribute('data-capture-id');
      expect(captureId).toBeTruthy();

      await pendingCard.locator('.btn-promote-capture').click();

      const promotedCard = page.locator(`#camera-captures-grid .camera-capture-card[data-capture-id="${captureId}"]`);
      await expect(promotedCard.locator('.badge-capture-promoted')).toHaveText('PROMOTED');
      await expect(promotedCard.locator('.promoted-sighting-badge')).toBeVisible();

      const sightingsRes = await request.get(`${WEB_FRONTEND_URL}/api/v1/sightings?petId=${targetPetId}`);
      expect(sightingsRes.status()).toBe(200);
      const sightings = await sightingsRes.json();
      expect(Array.isArray(sightings)).toBe(true);

      const matchedSighting = sightings.find(
        (s: any) =>
          s.petId === targetPetId &&
          ((s.locationDescription && s.locationDescription.includes(cameraName)) ||
            (s.notes && s.notes.includes(cameraName)))
      );
      expect(matchedSighting).toBeTruthy();
    });

    // -------------------------------------------------------------------------
    // Step 6: Verify High-Confidence Auto-Promotion via Direct Webhook API
    // -------------------------------------------------------------------------
    await test.step('Step 6: Verify High-Confidence Auto-Promotion via Direct Webhook API', async () => {
      const createRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/cameras`, {
        data: {
          name: 'High-Confidence Frigate Perimeter Cam',
          vendor: 'FRIGATE_RTSP',
          targetPetId: 'pet-auto-promote-1',
          latitude: 47.6205,
          longitude: -122.3493,
          autoPromoteThreshold: 0.5,
          nightVisionIr: true,
        },
      });
      expect(createRes.status()).toBe(201);
      const createdBody = await createRes.json();
      expect(createdBody.camera?.id).toBeTruthy();
      expect(createdBody.webhookToken).toContain('cam_tok_');

      const syntheticFrameDataUri = await page.evaluate(() =>
        (window as any).generateSyntheticEdgeFrame('daylight_cat')
      );
      expect(syntheticFrameDataUri).toContain('data:image/png;base64,');

      const webhookRes = await request.post(
        `${WEB_FRONTEND_URL}/api/v1/cameras/${createdBody.camera.id}/webhook`,
        {
          headers: {
            'X-Camera-Token': createdBody.webhookToken,
          },
          data: {
            snapshotDataUri: syntheticFrameDataUri,
            detectedSpecies: 'DOG',
            extractedTraits: ['Reflective collar', 'Medium build'],
            motionZone: 'perimeter_gate',
          },
        }
      );
      expect(webhookRes.status()).toBe(201);
      const webhookBody = await webhookRes.json();
      expect(webhookBody.verificationStatus).toBe('PROMOTED');
      expect(webhookBody.promotedSightingId).toBeTruthy();
    });
  });
});
