import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Universal Microchip Registry Federation, Multi-Standard Scanner Lookup & Ownership Resolution E2E Journey', () => {
  test('Full 6-step operator journey: seeded cockpit load, unregistered ISO 15-digit scan, manufacturer batch lot clinic PIMS trace, reunion sighting promotion, and multi-standard AVID-9 & Hex-10 verification', async ({
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
    await page.goto(`${WEB_FRONTEND_URL}/microchip-federation`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#microchip-federation-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Universal Microchip Registry Federation & Ownership Resolution Cockpit'
    );

    const card101 = page.locator(
      '#microchip-cases-list .microchip-case-card[data-case-id="chip-case-101"]'
    );
    const card102 = page.locator(
      '#microchip-cases-list .microchip-case-card[data-case-id="chip-case-102"]'
    );
    const card103 = page.locator(
      '#microchip-cases-list .microchip-case-card[data-case-id="chip-case-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-case-status', 'UNREGISTERED_BATCH_TRACED');
    await expect(card101.locator('.chip-lot-trace-banner')).toContainText(
      'UNREGISTERED CHIP — MANUFACTURER LOT TRACE'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-case-status', 'SECONDARY_CONTACT_ESCALATED');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-case-status', 'REUNION_DISPATCHED_VERIFIED');
    await expect(card103.locator('.chip-promoted-sighting-banner')).toContainText(
      'Promoted Sighting: sighting-chip-seed-103'
    );

    // Step 2: Scan Unregistered ISO 15-Digit Transponder via Form
    await page.locator('#mc-pet-id').fill('pet-e2e-chip-ollie');
    await page.locator('#mc-pet-name').fill('Ollie');
    await page.locator('#mc-species').selectOption('CAT');
    await page.locator('#mc-raw-chip-input').fill('900-118000-663412');
    await page.locator('#mc-scanned-by-agency').fill('Fremont Shelter Intake Wand');
    await page.locator('#mc-latitude').fill('47.6512');
    await page.locator('#mc-longitude').fill('-122.3501');
    await page.locator('#mc-simulate-mode').selectOption('UNREGISTERED_DEAD_END');

    await page.locator('#btn-scan-microchip').click();

    const ollieCard = page
      .locator('#microchip-cases-list .microchip-case-card', {
        hasText: 'Ollie (CAT · 900 ••••3412)',
      })
      .first();

    await expect(ollieCard).toBeVisible();
    await expect(ollieCard).toHaveAttribute('data-case-status', 'UNREGISTERED_BATCH_TRACED');
    await expect(ollieCard.locator('.badge-chip-status.badge-chip-unregistered')).toContainText(
      'UNREGISTERED_BATCH_TRACED'
    );
    await expect(ollieCard.locator('.badge-chip-standard')).toContainText(
      'ISO_11784_15 • 134.2 kHz'
    );
    await expect(ollieCard.locator('.badge-chip-lot')).toContainText('LOT-ISO900-1180');
    await expect(ollieCard.locator('.chip-lot-trace-banner')).toBeVisible();
    await expect(ollieCard.locator('.chip-lot-trace-banner')).toContainText(
      'UNREGISTERED CHIP — MANUFACTURER LOT TRACE'
    );

    // Step 3: Trace Manufacturer Batch Lot to Implanting Clinic PIMS
    await ollieCard.locator('.btn-trace-batch-lot').click();

    await expect(ollieCard).toContainText('Devon & Priya Patel');
    await expect(ollieCard).toContainText('+1 (206) •••-7724');
    await expect(ollieCard.locator('.chip-lot-trace-banner')).toContainText('PIMS-CHART-9102');

    // Step 4: Verify Ownership & Promote Reunion Sighting
    await ollieCard.locator('.btn-resolve-chip-reunion').click();

    await expect(ollieCard).toHaveAttribute('data-case-status', 'REUNION_DISPATCHED_VERIFIED');
    await expect(ollieCard.locator('.badge-chip-status.badge-chip-verified')).toContainText(
      'REUNION_DISPATCHED_VERIFIED'
    );
    await expect(ollieCard.locator('.chip-promoted-sighting-banner')).toBeVisible();
    await expect(ollieCard.locator('.chip-promoted-sighting-banner')).toContainText(
      'Promoted Sighting: sighting-chip-'
    );

    // Step 5: Multi-Standard AVID-9 & Legacy Hex-10 Verification
    await page.locator('#btn-sim-avid9-escalated').click();

    const pendingAvidCard = page
      .locator(
        '#microchip-cases-list .microchip-case-card[data-case-status="SECONDARY_CONTACT_ESCALATED"]',
        {
          hasText: 'LOT-AVID-0418',
        }
      )
      .first();

    await expect(pendingAvidCard).toBeVisible();
    const avidCaseId = await pendingAvidCard.getAttribute('data-case-id');
    expect(avidCaseId).toBeTruthy();

    const avidCard = page.locator(
      `#microchip-cases-list .microchip-case-card[data-case-id="${avidCaseId}"]`
    );
    await expect(avidCard).toHaveAttribute('data-case-status', 'SECONDARY_CONTACT_ESCALATED');
    await expect(avidCard.locator('.badge-chip-standard')).toContainText(
      'AVID_9_ENCRYPTED • 125.0 kHz'
    );

    await page.locator('#btn-sim-hex10-legacy').click();

    const hexCard = page
      .locator('#microchip-cases-list .microchip-case-card', {
        hasText: 'LOT-FDXA-7F7E',
      })
      .first();

    await expect(hexCard).toBeVisible();
    await expect(hexCard).toHaveAttribute('data-case-status', 'REGISTERED_OWNER_FOUND');
    await expect(hexCard.locator('.badge-chip-standard')).toContainText(
      'HEX_10_LEGACY • 128.0 kHz'
    );

    await avidCard.locator('.btn-resolve-chip-reunion').click();

    await expect(avidCard).toHaveAttribute('data-case-status', 'REUNION_DISPATCHED_VERIFIED');
    await expect(avidCard.locator('.badge-chip-status.badge-chip-verified')).toContainText(
      'REUNION_DISPATCHED_VERIFIED'
    );
    await expect(avidCard.locator('.chip-promoted-sighting-banner')).toBeVisible();
    await expect(avidCard.locator('.chip-promoted-sighting-banner')).toContainText(
      'Promoted Sighting: sighting-chip-'
    );

    // Step 6: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
