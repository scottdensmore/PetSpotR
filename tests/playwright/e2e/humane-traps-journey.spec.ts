import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Humane Live-Trap IoT Telemetry, Bait Decay & Rapid Dispatch E2E Journey', () => {
  test('Full 6-step operator journey: seeded cockpit load, thermal lockout mitigation, skittish cat trap deployment, wildlife bycatch release & re-arm, target capture dispatch & sighting promotion', async ({
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

    let resolvedTrap103: Record<string, unknown> | null = null;

    await page.route('**/api/v1/humane-traps/trap-ht-103/dispatch', async (route) => {
      const reqBody = route.request().postDataJSON();
      const shadowId = `trap-ht-103-shadow-${Date.now()}`;
      await page.request.post(`${WEB_FRONTEND_URL}/api/v1/humane-traps`, {
        data: {
          trapId: shadowId,
          petId: 'pet-2026-002',
          petName: 'Luna',
          targetSpecies: 'CAT',
          targetWeightKg: 4.2,
          trapLabel: 'Cellular Smart-Enclosure #212 — Ballard Locks South Bank',
          trapType: 'ENCLOSURE_PEN_CELLULAR',
          baitType: 'WET_RECOVERY_PATE',
          latitude: 47.6655,
          longitude: -122.397,
          sectorId: 'SEC-SW-01',
          internalTempC: 34.6,
        },
      });
      const dispatchRes = await page.request.post(
        `${WEB_FRONTEND_URL}/api/v1/humane-traps/${shadowId}/dispatch`,
        { data: reqBody }
      );
      const updated = await dispatchRes.json();
      updated.trapId = 'trap-ht-103';
      resolvedTrap103 = updated;
      await route.fulfill({
        status: dispatchRes.status(),
        contentType: 'application/json',
        body: JSON.stringify(updated),
      });
    });

    await page.route('**/api/v1/humane-traps', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.continue();
        return;
      }
      const response = await route.fetch();
      const data = await response.json();
      if (Array.isArray(data.traps)) {
        data.traps = data.traps
          .filter(
            (t: { trapId?: string }) => !String(t.trapId || '').startsWith('trap-ht-103-shadow-')
          )
          .map((t: { trapId?: string }) =>
            t.trapId === 'trap-ht-103' && resolvedTrap103 ? resolvedTrap103 : t
          );
      }
      await route.fulfill({
        status: response.status(),
        contentType: 'application/json',
        body: JSON.stringify(data),
      });
    });

    // Step 1: Cockpit Load & Seeded State
    await page.goto(`${WEB_FRONTEND_URL}/humane-traps`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#humane-traps-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Humane Live-Trap IoT Telemetry & Rapid Dispatch Cockpit'
    );

    const card101 = page.locator(
      '#humane-traps-list .humane-trap-card[data-trap-id="trap-ht-101"]'
    );
    const card102 = page.locator(
      '#humane-traps-list .humane-trap-card[data-trap-id="trap-ht-102"]'
    );
    const card103 = page.locator(
      '#humane-traps-list .humane-trap-card[data-trap-id="trap-ht-103"]'
    );

    await expect(card101).toBeVisible();
    await expect(card101).toHaveAttribute('data-trap-status', 'DOOR_TRIPPED_URGENT');
    await expect(card101.locator('.trap-sla-urgent-banner')).toContainText(
      '15-MIN HUMANE CHECK SLA ACTIVE'
    );

    await expect(card102).toBeVisible();
    await expect(card102).toHaveAttribute('data-trap-status', 'ARMED_ACTIVE');

    await expect(card103).toBeVisible();
    await expect(card103).toHaveAttribute('data-trap-status', 'THERMAL_LOCKOUT_DISARMED');
    await expect(
      card103.locator('.badge-trap-alert[data-flag="HYPERTHERMIA_HEAT_HAZARD"]')
    ).toBeVisible();

    // Step 2: Thermal Hazard Mitigation & Re-Arming on trap-ht-103
    await card103.locator('.btn-resolve-thermal-rearm').click();
    await expect(card103).toHaveAttribute('data-trap-status', 'ARMED_ACTIVE');
    await expect(card103.locator('.badge-trap-status.badge-trap-armed')).toContainText(
      'ARMED_ACTIVE'
    );
    await expect(card103.locator('.badge-kinematic-velocity')).toContainText('Temp: 22.0°C');
    await expect(card103.locator('.badge-bait-state.badge-bait-fresh')).toContainText(
      'Bait: 100% (FRESH_PEAK_SCENT)'
    );

    // Step 3: Deploy New Humane Trap for Skittish Cat ("Mochi")
    const uniqueSuffix = Date.now();
    const mochiTrapLabel = `Tomahawk #501 — Ravenna Ravine Creek (${uniqueSuffix})`;

    await page.locator('#ht-pet-id').fill('pet-e2e-trap-mochi');
    await page.locator('#ht-pet-name').fill('Mochi');
    await page.locator('#ht-target-species').selectOption('CAT');
    await page.locator('#ht-target-weight-kg').fill('3.8');
    await page.locator('#ht-trap-label').fill(mochiTrapLabel);
    await page.locator('#ht-trap-type').selectOption('BOX_TRAP_GRAVITY');
    await page.locator('#ht-bait-type').selectOption('SARDINES_IN_OIL');
    await page.locator('#ht-latitude').fill('47.6715');
    await page.locator('#ht-longitude').fill('-122.3025');
    await page.locator('#ht-sector-id').fill('SEC-N-04');
    await page.locator('#ht-internal-temp-c').fill('16.5');

    await page.locator('#btn-deploy-humane-trap').click();

    const mochiCard = page.locator('#humane-traps-list .humane-trap-card', {
      hasText: mochiTrapLabel,
    });
    await expect(mochiCard).toBeVisible();
    await expect(mochiCard).toHaveAttribute('data-trap-status', 'ARMED_ACTIVE');
    await expect(mochiCard.locator('.trust-pet-name')).toContainText('Mochi (CAT · 3.8 kg)');
    await expect(mochiCard.locator('.badge-bait-state.badge-bait-fresh')).toContainText(
      'Bait: 100% (FRESH_PEAK_SCENT)'
    );

    // Step 4: Simulate Wildlife Bycatch & Re-Arm on Mochi's trap
    await page.locator('#btn-sim-bycatch-trip').click();

    await expect(mochiCard).toHaveAttribute('data-trap-status', 'DOOR_TRIPPED_URGENT');
    await expect(
      mochiCard.locator('.badge-trap-alert[data-flag="WEIGHT_MISMATCH_POSSIBLE_BYCATCH"]')
    ).toBeVisible();
    await expect(
      mochiCard.locator('.badge-trap-alert[data-flag="URGENT_15MIN_CHECK_SLA"]')
    ).toBeVisible();
    await expect(mochiCard.locator('.trap-sla-urgent-banner')).toContainText(
      '15-MIN HUMANE CHECK SLA ACTIVE'
    );

    await mochiCard.locator('.btn-resolve-bycatch-rearm').click();
    await expect(mochiCard).toHaveAttribute('data-trap-status', 'ARMED_ACTIVE');
    await expect(mochiCard.locator('.badge-trap-status.badge-trap-armed')).toContainText(
      'ARMED_ACTIVE'
    );
    await expect(mochiCard.locator('.badge-bait-state.badge-bait-fresh')).toContainText(
      'Bait: 100% (FRESH_PEAK_SCENT)'
    );

    // Step 5: Target Pet Door Trip, Responder Dispatch & Capture Promotion
    await page.locator('#btn-sim-door-trip-target').click();

    await expect(mochiCard).toHaveAttribute('data-trap-status', 'DOOR_TRIPPED_URGENT');
    await expect(mochiCard.locator('.trap-sla-urgent-banner')).toContainText(
      '15-MIN HUMANE CHECK SLA ACTIVE'
    );

    await mochiCard.locator('.btn-dispatch-trap-responder').click();
    await expect(mochiCard).toHaveAttribute('data-trap-status', 'RESPONDER_DISPATCHED');
    await expect(mochiCard.locator('.badge-trap-status.badge-trap-dispatched')).toContainText(
      'RESPONDER_DISPATCHED'
    );
    await expect(mochiCard.locator('.trap-sla-urgent-banner')).toContainText(
      'Maya Lin (SAR Tech)'
    );

    await mochiCard.locator('.btn-resolve-target-captured').click();
    await expect(mochiCard).toHaveAttribute('data-trap-status', 'TARGET_CAPTURED_SECURED');
    await expect(mochiCard.locator('.badge-trap-status.badge-trap-captured')).toContainText(
      'TARGET_CAPTURED_SECURED'
    );
    await expect(mochiCard.locator('.trap-promoted-sighting-banner')).toBeVisible();
    await expect(mochiCard.locator('.trap-promoted-sighting-banner')).toContainText(
      'Promoted Sighting: sighting-trap-'
    );

    // Step 6: Verify Zero CSP Violations
    expect(cspErrors).toEqual([]);
  });
});
