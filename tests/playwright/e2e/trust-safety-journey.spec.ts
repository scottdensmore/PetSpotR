import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Milestone 13.2: Multi-Modal False-Positive & Scam/Hoax Sighting Shield (Trust & Safety Engine) Journey', () => {
  test('Complete 6-Step Multi-Modal Sighting Fraud Detection, Quarantine & Moderator Adjudication User Journey', async ({ page }) => {
    const cspErrors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error' && /Content Security Policy/i.test(msg.text())) {
        cspErrors.push(msg.text());
      }
    });
    page.on('pageerror', (err) => {
      if (/Content Security Policy/i.test(err.message)) {
        cspErrors.push(err.message);
      }
    });

    const petId = 'pet-e2e-trust-archie';
    const petName = 'Archie';
    let scamScreeningId = '';
    let teleportScreeningId = '';

    // -------------------------------------------------------------------------
    // Step 1: Navigate to /trust-safety & Verify Seeded Screenings
    // -------------------------------------------------------------------------
    await test.step('Step 1: Navigate to /trust-safety & Verify Seeded Screenings', async () => {
      await page.goto(`${WEB_FRONTEND_URL}/trust-safety`);
      await page.waitForLoadState('networkidle');

      await expect(page.locator('#trust-safety-main')).toBeVisible();
      await expect(page.locator('h1')).toContainText(
        'Multi-Modal False-Positive & Scam/Hoax Sighting Shield (Trust & Safety Cockpit)'
      );

      const card101 = page.locator('#trust-screenings-list .trust-screening-card[data-screening-id="screening-ts-101"]');
      await expect(card101).toBeVisible();
      await expect(card101).toHaveAttribute('data-verdict', 'QUARANTINED_SCAM_RISK');
      await expect(card101.locator('.badge-verdict-quarantined')).toContainText('QUARANTINED_SCAM_RISK');
      await expect(card101.locator('.trust-quarantine-banner')).toContainText('OWNER SHIELD ACTIVE');

      const card102 = page.locator('#trust-screenings-list .trust-screening-card[data-screening-id="screening-ts-102"]');
      await expect(card102).toBeVisible();
      await expect(card102).toHaveAttribute('data-verdict', 'HOLD_FOR_MODERATION');
      await expect(card102.locator('.badge-verdict-hold')).toContainText('HOLD_FOR_MODERATION');

      const card103 = page.locator('#trust-screenings-list .trust-screening-card[data-screening-id="screening-ts-103"]');
      await expect(card103).toBeVisible();
      await expect(card103).toHaveAttribute('data-verdict', 'VERIFIED_AUTHENTIC');
      await expect(card103.locator('.badge-verdict-authentic')).toContainText('VERIFIED_AUTHENTIC');

      expect(cspErrors).toEqual([]);
    });

    // -------------------------------------------------------------------------
    // Step 2: Submit Plausible Authentic Sighting via Form & Verify Auto-Promotion
    // -------------------------------------------------------------------------
    await test.step('Step 2: Submit Plausible Authentic Sighting via Form & Verify Auto-Promotion', async () => {
      await page.locator('#ts-pet-id').fill(petId);
      await page.locator('#ts-pet-name').fill(petName);
      await page.locator('#ts-reporter-handle').fill('sar-spotter-elena');
      await page.locator('#ts-reporter-trust').fill('90');
      await page.locator('#ts-prev-lat').fill('47.6062');
      await page.locator('#ts-prev-lng').fill('-122.3321');
      await page.locator('#ts-sighting-lat').fill('47.6088');
      await page.locator('#ts-sighting-lng').fill('-122.3342');
      await page.locator('#ts-elapsed-min').fill('25');
      await page.locator('#ts-image-integrity').selectOption('CAMERA_EXIF_INTACT');
      await page.locator('#ts-submitted-notes').fill(
        'Spotted Archie walking along Olympic Sculpture Park trail wearing blue reflective collar'
      );

      await page.locator('#btn-submit-trust-screening').click();

      const authenticCard = page
        .locator('#trust-screenings-list .trust-screening-card')
        .filter({ hasText: petId })
        .filter({ hasText: 'sar-spotter-elena' })
        .first();
      await expect(authenticCard).toBeVisible();
      await expect(authenticCard).toContainText(petName);
      await expect(authenticCard).toContainText(petId);
      await expect(authenticCard).toHaveAttribute('data-verdict', 'VERIFIED_AUTHENTIC');
      await expect(authenticCard.locator('.badge-verdict-authentic')).toContainText('VERIFIED_AUTHENTIC');
      await expect(authenticCard.locator('.trust-promoted-banner')).toBeVisible();
      await expect(authenticCard.locator('.trust-promoted-banner')).toContainText('sighting-trust-');
    });

    // -------------------------------------------------------------------------
    // Step 3: Trigger Reward-Extortion + Synthetic GAN Scam Simulation & Verify Auto-Quarantine
    // -------------------------------------------------------------------------
    await test.step('Step 3: Trigger Reward-Extortion + Synthetic GAN Scam Simulation & Verify Auto-Quarantine', async () => {
      await page.locator('#btn-sim-extortion-scam').click();

      const scamCard = page
        .locator('#trust-screenings-list .trust-screening-card')
        .filter({ hasText: petId })
        .filter({ hasText: 'burner-extort-77' })
        .first();
      await expect(scamCard).toBeVisible();
      await expect(scamCard).toContainText(petName);
      await expect(scamCard).toHaveAttribute('data-verdict', 'QUARANTINED_SCAM_RISK');
      await expect(scamCard.locator('.badge-verdict-quarantined')).toContainText('QUARANTINED_SCAM_RISK');
      await expect(scamCard.locator('.badge-anomaly-flag[data-flag="EXTORTION_PAYMENT_DEMAND"]')).toBeVisible();
      await expect(scamCard.locator('.badge-anomaly-flag[data-flag="PIN_PHISHING_ATTEMPT"]')).toBeVisible();
      await expect(scamCard.locator('.badge-anomaly-flag[data-flag="SYNTHETIC_IMAGE_ARTIFACT"]')).toBeVisible();
      await expect(scamCard.locator('.trust-quarantine-banner')).toBeVisible();
      await expect(scamCard.locator('.trust-quarantine-banner')).toContainText('OWNER SHIELD ACTIVE');

      const idAttr = await scamCard.getAttribute('data-screening-id');
      expect(idAttr).toBeTruthy();
      scamScreeningId = idAttr!;
    });

    // -------------------------------------------------------------------------
    // Step 4: Trigger 150 km/h Teleportation Velocity Anomaly Simulation (HOLD_FOR_MODERATION)
    // -------------------------------------------------------------------------
    await test.step('Step 4: Trigger 150 km/h Teleportation Velocity Anomaly Simulation (HOLD_FOR_MODERATION)', async () => {
      await page.locator('#btn-sim-teleport-hoax').click();

      const teleportCard = page
        .locator('#trust-screenings-list .trust-screening-card')
        .filter({ hasText: petId })
        .filter({ hasText: 'highway-spotter-91' })
        .first();
      await expect(teleportCard).toBeVisible();
      await expect(teleportCard).toContainText(petName);
      await expect(teleportCard).toHaveAttribute('data-verdict', 'HOLD_FOR_MODERATION');
      await expect(teleportCard.locator('.badge-verdict-hold')).toContainText('HOLD_FOR_MODERATION');
      await expect(
        teleportCard.locator('.badge-anomaly-flag[data-flag="TELEPORTATION_VELOCITY_ANOMALY"]')
      ).toBeVisible();

      const idAttr = await teleportCard.getAttribute('data-screening-id');
      expect(idAttr).toBeTruthy();
      teleportScreeningId = idAttr!;
    });

    // -------------------------------------------------------------------------
    // Step 5: Moderator Adjudication: Approve Held Sighting & Promote to Live Community Sighting
    // -------------------------------------------------------------------------
    await test.step('Step 5: Moderator Adjudication: Approve Held Sighting & Promote to Live Community Sighting', async () => {
      const teleportCard = page.locator(
        `#trust-screenings-list .trust-screening-card[data-screening-id="${teleportScreeningId}"]`
      );
      await expect(teleportCard).toBeVisible();

      await teleportCard.locator('.btn-approve-screening').click();

      await expect(teleportCard).toHaveAttribute('data-verdict', 'VERIFIED_AUTHENTIC');
      await expect(teleportCard.locator('.badge-verdict-authentic')).toContainText('VERIFIED_AUTHENTIC');
      await expect(teleportCard.locator('.trust-promoted-banner')).toBeVisible();
      await expect(teleportCard.locator('.trust-promoted-banner')).toContainText('sighting-trust-');
    });

    // -------------------------------------------------------------------------
    // Step 6: Moderator Adjudication: Confirm Scam / Hoax & Zero Reporter Reputation
    // -------------------------------------------------------------------------
    await test.step('Step 6: Moderator Adjudication: Confirm Scam / Hoax & Zero Reporter Reputation', async () => {
      const scamCard = page.locator(
        `#trust-screenings-list .trust-screening-card[data-screening-id="${scamScreeningId}"]`
      );
      await expect(scamCard).toBeVisible();

      await scamCard.locator('.btn-reject-screening').click();

      await expect(scamCard).toHaveAttribute('data-verdict', 'REJECTED_CONFIRMED_HOAX');
      await expect(scamCard.locator('.badge-verdict-rejected')).toContainText('REJECTED_CONFIRMED_HOAX');
      await expect(scamCard.locator('.badge-reporter-trust')).toContainText('Trust: 0/100');
      await expect(scamCard.locator('.trust-quarantine-banner')).toContainText('OWNER SHIELD ACTIVE');

      expect(cspErrors).toEqual([]);
    });
  });
});
