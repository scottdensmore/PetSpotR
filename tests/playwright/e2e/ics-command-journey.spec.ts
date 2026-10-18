import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Incident Command (ICS-211 / ICS-204), Cryptographic Role Credentialing & SAR Gamification E2E Journey', () => {
  test('Full 6-step user journey: seeded responders, role-gate 403 block, signed credential issuance, shift check-in, fatigue escalation, XP & merit badge checkout, and ICS-204 CSV export', async ({
    page,
    request,
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

    // =========================================================================
    // Step 1 — Navigate to /ics-command & Verify Seeded Responders
    // =========================================================================
    await page.goto(`${WEB_FRONTEND_URL}/ics-command`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#ics-command-main')).toBeVisible();
    await expect(page.locator('#volunteers-leaderboard')).toBeVisible();

    for (const seededId of ['vol-sar-101', 'vol-sar-102', 'vol-sar-103']) {
      const seededCard = page.locator(
        `#volunteers-leaderboard .volunteer-card[data-volunteer-id="${seededId}"]`
      );
      await expect(seededCard).toBeVisible();
      await expect(seededCard.locator('.badge-sar-credential').first()).toBeVisible();
      await expect(seededCard.locator('.badge-sar-rank')).toBeVisible();
    }

    expect(cspErrors).toEqual([]);

    // =========================================================================
    // Step 2 — Register Uncredentialed Field Volunteer & Verify Role Gate (403 Forbidden)
    // =========================================================================
    const uniqueSuffix = Date.now();
    const volName = `Lt. Taylor Brooks #${uniqueSuffix}`;
    const volCallsign = `HAWK-${uniqueSuffix % 1000}`;

    await page.locator('#vol-name').fill(volName);
    await page.locator('#vol-callsign').fill(volCallsign);
    await page.locator('#vol-primary-role').selectOption('FIELD_SEARCHER');
    await page.locator('#vol-license').fill(`SAR-FIELD-${uniqueSuffix % 1000}`);
    await page.locator('#btn-register-volunteer').click();

    const volCard = page.locator('#volunteers-leaderboard .volunteer-card', { hasText: volName });
    await expect(volCard).toBeVisible();
    await expect(volCard.locator('.badge-callsign')).toContainText(volCallsign);
    await expect(volCard.locator('.badge-sar-rank')).toContainText('Rookie Spotter (Tier 1)');
    await expect(volCard.locator('.badge-xp-total')).toContainText('200 XP');

    const volunteerId = await volCard.getAttribute('data-volunteer-id');
    expect(volunteerId).toBeTruthy();

    // Attempt to check in as UAV_PART107_PILOT without the signed credential
    const incidentId = `INC-2026-E2E-${uniqueSuffix % 10000}`;
    await page.locator('#shift-incident-id').fill(incidentId);
    await page.locator('#shift-volunteer-select').selectOption(volunteerId!);
    await page.locator('#shift-assigned-role').selectOption('UAV_PART107_PILOT');
    await page.locator('#shift-sector-id').fill('SECTOR-BRAVO-09');
    await page.locator('#btn-checkin-shift').click();

    const roleGateAlert = page.locator('#role-gate-error-alert');
    await expect(roleGateAlert).toBeVisible();
    await expect(roleGateAlert).toContainText('ICS-211 Role-Gate Blocked');
    await expect(roleGateAlert).toContainText('UAV_PART107_PILOT');

    // Ensure no unauthorized shift card was created for this responder
    await expect(page.locator('#ics-shifts-list .ics-shift-card', { hasText: volName })).toHaveCount(0);

    // =========================================================================
    // Step 3 — Issue Cryptographically Signed UAV_PART107_PILOT Credential & Check In to ICS-211 Shift
    // =========================================================================
    await page.locator('#cred-volunteer-select').selectOption(volunteerId!);
    await page.locator('#cred-role-select').selectOption('UAV_PART107_PILOT');
    await page.locator('#cred-issued-by').fill('FAA / NASAR Command');
    await page.locator('#cred-license-number').fill('FAA-107-E2E-7788');
    await page.locator('#btn-issue-credential').click();

    // Verify signed UAV_PART107_PILOT credential badge, AERIAL_SPOTTER merit badge, and increased XP
    const uavCredBadge = volCard.locator('.badge-sar-credential[data-role="UAV_PART107_PILOT"]');
    await expect(uavCredBadge).toBeVisible();
    await expect(uavCredBadge).toContainText('FAA-107-E2E-7788');
    await expect(uavCredBadge).toHaveAttribute('title', /HMAC-SHA256:\s+[0-9a-f]{64}/i);
    await expect(volCard.locator('.badge-sar-merit[data-badge="AERIAL_SPOTTER"]')).toBeVisible();
    await expect(volCard.locator('.badge-xp-total')).toContainText('400 XP');

    // Re-submit #form-checkin-shift for UAV_PART107_PILOT on SECTOR-BRAVO-09
    await page.locator('#shift-incident-id').fill(incidentId);
    await page.locator('#shift-volunteer-select').selectOption(volunteerId!);
    await page.locator('#shift-assigned-role').selectOption('UAV_PART107_PILOT');
    await page.locator('#shift-sector-id').fill('SECTOR-BRAVO-09');
    await page.locator('#btn-checkin-shift').click();

    await expect(roleGateAlert).toBeHidden();
    const shiftCard = page.locator('#ics-shifts-list .ics-shift-card', { hasText: volName }).first();
    await expect(shiftCard).toBeVisible();
    await expect(shiftCard).toHaveAttribute('data-fatigue-level', 'NORMAL_FIT');
    await expect(shiftCard).toHaveAttribute('data-shift-status', 'ON_SECTOR_PATROL');
    await expect(shiftCard.locator('.badge-fatigue-normal')).toContainText('NORMAL_FIT');
    await expect(shiftCard.locator('.badge-shift-status')).toContainText('ON_SECTOR_PATROL');
    await expect(shiftCard.locator('.badge-assigned-role')).toContainText('UAV_PART107_PILOT');

    const shiftId = await shiftCard.getAttribute('data-shift-id');
    expect(shiftId).toBeTruthy();

    // =========================================================================
    // Step 4 — Simulate Telemetry Progression & Verify Automated Fatigue Guardrails
    // =========================================================================
    await shiftCard.locator('.btn-sim-fatigue-warn').click();
    await expect(shiftCard).toHaveAttribute('data-fatigue-level', 'FATIGUE_WARNING');
    await expect(shiftCard.locator('.badge-fatigue-warning')).toContainText('FATIGUE_WARNING');
    await expect(shiftCard.locator('.shift-hours')).toContainText('6.5h active');

    await shiftCard.locator('.btn-sim-mandatory-rest').click();
    await expect(shiftCard).toHaveAttribute('data-fatigue-level', 'MANDATORY_REST');
    await expect(shiftCard).toHaveAttribute('data-shift-status', 'MANDATORY_REST');
    await expect(shiftCard.locator('.badge-fatigue-rest')).toContainText('MANDATORY_REST');
    await expect(shiftCard.locator('.badge-shift-status')).toContainText('MANDATORY_REST');
    await expect(shiftCard.locator('.shift-hours')).toContainText('8.5h active');
    await expect(shiftCard.locator('.shift-distance')).toContainText('52.0 km covered');

    // =========================================================================
    // Step 5 — Complete Shift Check-Out & Verify SAR Gamification XP, Tier Rank & Merit Badges
    // =========================================================================
    await shiftCard.locator('.btn-checkout-shift').click();
    await expect(shiftCard).toHaveAttribute('data-shift-status', 'CHECKED_OUT');
    await expect(shiftCard.locator('.badge-shift-status')).toContainText('CHECKED_OUT');

    // Verify upgraded XP, rank title, and all 5 earned merit badges
    await expect(volCard.locator('.badge-sar-rank')).toContainText('Senior SAR Specialist (Tier 3)');
    await expect(volCard.locator('.badge-xp-total')).toContainText('2402 XP');
    for (const expectedBadge of [
      'FIRST_RESPONDER',
      'AERIAL_SPOTTER',
      'NIGHT_OWL_IR',
      'GRID_MASTER_50KM',
      'REUNION_HERO',
    ]) {
      await expect(
        volCard.locator(`.badge-sar-merit[data-badge="${expectedBadge}"]`)
      ).toBeVisible();
    }

    // =========================================================================
    // Step 6 — Verify NIMS ICS-204 / ICS-211 Roster CSV Export
    // =========================================================================
    const csvLink = page.locator('#btn-export-ics204-csv');
    await expect(csvLink).toHaveAttribute('href', '/api/v1/ics/shifts?format=csv');

    const csvResponse = await request.get(`${WEB_FRONTEND_URL}/api/v1/ics/shifts?format=csv`);
    expect(csvResponse.status()).toBe(200);
    expect(csvResponse.headers()['content-type']).toContain('text/csv');

    const csvBody = await csvResponse.text();
    const lines = csvBody.trim().split('\n');
    expect(lines[0].trim()).toBe(
      'shift_id,incident_id,volunteer_id,volunteer_name,callsign,assigned_role,assigned_sector_id,status,active_hours,distance_covered_km,fatigue_level,checked_in_at'
    );

    const matchingRow = lines.find((line) => line.includes(shiftId!));
    expect(matchingRow).toBeTruthy();
    expect(matchingRow).toContain(incidentId);
    expect(matchingRow).toContain(volunteerId!);
    expect(matchingRow).toContain(volCallsign);
    expect(matchingRow).toContain('UAV_PART107_PILOT');
    expect(matchingRow).toContain('SECTOR-BRAVO-09');
    expect(matchingRow).toContain('CHECKED_OUT');
    expect(matchingRow).toContain('8.50');
    expect(matchingRow).toContain('52.00');
    expect(matchingRow).toContain('MANDATORY_REST');

    expect(cspErrors).toEqual([]);
  });
});
