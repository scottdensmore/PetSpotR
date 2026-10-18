import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Forensic DNA, Coat-Pattern Biometrics & Senior/Blind Pet Cognitive Disorientation Triage Cockpit E2E Journey', () => {
  test('Full 6-step operator journey: seeded cockpit load, 15yo CCD sundowning dog evaluation with entrapment alert, nasal planum ridge matching & 150m culvert sweep, STR DNA forensic extraction confirmation, and cataract-blind cat simulation', async ({
    page,
  }) => {
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

    // Step 1: Cockpit Load & Seeded Cases
    await page.goto(`${WEB_FRONTEND_URL}/biometric-forensics`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#biometric-forensics-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Forensic DNA, Coat-Pattern Biometrics & Senior Cognitive Triage Cockpit'
    );

    const card101 = page.locator(
      '#forensic-cases-list .forensic-case-card[data-case-id="forensic-case-101"]'
    );
    const card102 = page.locator(
      '#forensic-cases-list .forensic-case-card[data-case-id="forensic-case-102"]'
    );
    const card103 = page.locator(
      '#forensic-cases-list .forensic-case-card[data-case-id="forensic-case-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-resolution-status', 'PENDING_BIOMETRIC_SCAN');
    await expect(card101.locator('.forensic-sundowning-alert-banner')).toContainText(
      'SENIOR COGNITIVE SUNDOWNING & ENTRAPMENT ALERT'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-resolution-status', 'DERMATOGLYPHIC_CORRELATED');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute(
      'data-resolution-status',
      'IDENTITY_CONFIRMED_SAFE_EXTRACTION'
    );
    await expect(card103.locator('.forensic-certificate-banner')).toContainText(
      'BIOMETRIC FORENSIC IDENTITY CONFIRMED: CERT-FORENSIC-FORENSIC-CASE-103'
    );

    // Step 2: Evaluate Biometric Forensic Case for 15yo CCD Sundowning Dog ("Bramble")
    await page.locator('#forensic-pet-id').fill('pet-e2e-forensic-bramble');
    await page.locator('#forensic-pet-name').fill('Bramble');
    await page.locator('#forensic-pet-species').selectOption('DOG');
    await page.locator('#forensic-age-years').fill('15');
    await page.locator('#forensic-sensory-condition').selectOption('CCD_SUNDOWNING_DEMENTIA');
    await page.locator('#forensic-modality').selectOption('NASAL_PLANUM_RIDGE_PRINT');
    await page.locator('#forensic-hazard-type').selectOption('RETENTION_POND_CULVERT');

    await page.locator('#btn-create-forensic-case').click();

    const brambleInitialCard = page
      .locator('#forensic-cases-list .forensic-case-card', {
        hasText: 'Bramble (DOG · 15yo · CCD_SUNDOWNING_DEMENTIA)',
      })
      .first();

    await expect(brambleInitialCard).toBeVisible();
    const brambleId = await brambleInitialCard.getAttribute('data-case-id');
    expect(brambleId).toBeTruthy();

    const brambleCard = page.locator(
      `#forensic-cases-list .forensic-case-card[data-case-id="${brambleId}"]`
    );

    await expect(brambleCard).toHaveAttribute('data-resolution-status', 'PENDING_BIOMETRIC_SCAN');
    await expect(brambleCard.locator('.badge-forensic-status.badge-forensic-pending')).toContainText(
      'PENDING_BIOMETRIC_SCAN'
    );
    await expect(
      brambleCard.locator('.badge-entrapment-risk.badge-entrapment-critical')
    ).toContainText('CRITICAL_SUNDOWNING_ENTRAPMENT');
    await expect(brambleCard.locator('.forensic-sundowning-alert-banner')).toBeVisible();
    await expect(brambleCard.locator('.forensic-sundowning-alert-banner')).toContainText(
      'SENIOR COGNITIVE SUNDOWNING & ENTRAPMENT ALERT'
    );

    // Step 3: Correlate Nasal Planum Biometrics & Sweep 150m Culvert
    await brambleCard.locator('.btn-correlate-biometric').click();

    await expect(brambleCard).toHaveAttribute('data-resolution-status', 'DERMATOGLYPHIC_CORRELATED');
    await expect(
      brambleCard.locator('.badge-forensic-status.badge-forensic-correlated')
    ).toContainText('DERMATOGLYPHIC_CORRELATED');
    await expect(brambleCard.locator('.badge-forensic-metric')).toContainText(
      'Biometric Confidence: 100/100'
    );
    await expect(
      brambleCard.locator('.badge-entrapment-risk.badge-entrapment-elevated')
    ).toContainText('ELEVATED_SENSORY_DISORIENTATION');

    // Step 4: Confirm STR DNA / Biometric Identity & Safe Extraction
    await brambleCard.locator('.btn-confirm-forensic-extraction').click();

    await expect(brambleCard).toHaveAttribute(
      'data-resolution-status',
      'IDENTITY_CONFIRMED_SAFE_EXTRACTION'
    );
    await expect(
      brambleCard.locator('.badge-forensic-status.badge-forensic-confirmed')
    ).toContainText('IDENTITY_CONFIRMED_SAFE_EXTRACTION');
    await expect(brambleCard.locator('.badge-entrapment-risk.badge-entrapment-safe')).toContainText(
      'CONTAINED_SAFE_PERIPHERY'
    );
    await expect(brambleCard.locator('.forensic-certificate-banner')).toBeVisible();
    await expect(brambleCard.locator('.forensic-certificate-banner')).toContainText(
      'BIOMETRIC FORENSIC IDENTITY CONFIRMED: CERT-FORENSIC-'
    );

    // Step 5: Simulate Cataract-Blind Cat in Blackberry Bramble & Confirm Safe Extraction
    await page.locator('#btn-sim-blind-cat-bramble').click();

    const pendingPearlCard = page
      .locator(
        '#forensic-cases-list .forensic-case-card[data-resolution-status="PENDING_BIOMETRIC_SCAN"]',
        {
          hasText: 'Pearl',
        }
      )
      .first();

    await expect(pendingPearlCard).toBeVisible();
    const pearlId = await pendingPearlCard.getAttribute('data-case-id');
    expect(pearlId).toBeTruthy();

    const pearlCard = page.locator(
      `#forensic-cases-list .forensic-case-card[data-case-id="${pearlId}"]`
    );
    await expect(pearlCard).toHaveAttribute('data-resolution-status', 'PENDING_BIOMETRIC_SCAN');
    await expect(
      pearlCard.locator('.badge-entrapment-risk.badge-entrapment-critical')
    ).toContainText('CRITICAL_SUNDOWNING_ENTRAPMENT');

    await pearlCard.locator('.btn-confirm-forensic-extraction').click();

    await expect(pearlCard).toHaveAttribute(
      'data-resolution-status',
      'IDENTITY_CONFIRMED_SAFE_EXTRACTION'
    );
    await expect(
      pearlCard.locator('.badge-forensic-status.badge-forensic-confirmed')
    ).toContainText('IDENTITY_CONFIRMED_SAFE_EXTRACTION');
    await expect(pearlCard.locator('.badge-entrapment-risk.badge-entrapment-safe')).toContainText(
      'CONTAINED_SAFE_PERIPHERY'
    );
    await expect(pearlCard.locator('.forensic-certificate-banner')).toBeVisible();
    await expect(pearlCard.locator('.forensic-certificate-banner')).toContainText(
      'BIOMETRIC FORENSIC IDENTITY CONFIRMED: CERT-FORENSIC-'
    );

    // Step 6: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
