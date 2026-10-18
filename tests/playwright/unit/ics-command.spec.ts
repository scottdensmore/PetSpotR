import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

/**
 * Helper to parse rgb/rgba string into [r, g, b]
 */
function parseRgb(rgbStr: string): [number, number, number] {
  const match = rgbStr.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i);
  if (!match) return [0, 0, 0];
  return [parseInt(match[1], 10), parseInt(match[2], 10), parseInt(match[3], 10)];
}

/**
 * Relative luminance per WCAG 2.1
 */
function relativeLuminance([r, g, b]: [number, number, number]): number {
  const sRGB = [r / 255, g / 255, b / 255];
  const [R, G, B] = sRGB.map((c) =>
    c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)
  );
  return 0.2126 * R + 0.7152 * G + 0.0722 * B;
}

/**
 * Contrast ratio per WCAG 2.1
 */
function contrastRatio(fgStr: string, bgStr: string): number {
  const l1 = relativeLuminance(parseRgb(fgStr));
  const l2 = relativeLuminance(parseRgb(bgStr));
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

test.describe('Incident Command (ICS-211 / ICS-204) Volunteer Shift Roster & SAR Gamification Cockpit UI', () => {
  test('1. /ics-command renders seeded SAR responders, credentials, leaderboard, shift check-in form, and roster panel with zero CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/ics-command`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#ics-command-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Incident Command (ICS) Shift Roster, Volunteer Credentialing & SAR Gamification Cockpit'
    );
    await expect(page.locator('#btn-export-ics204-csv')).toHaveAttribute(
      'href',
      '/api/v1/ics/shifts?format=csv'
    );

    // Seeded SAR responders in #volunteers-leaderboard
    await expect(page.locator('#volunteers-leaderboard')).toBeVisible();
    const volunteerCards = page.locator('#volunteers-leaderboard .volunteer-card');
    await expect(volunteerCards.first()).toBeVisible();
    expect(await volunteerCards.count()).toBeGreaterThanOrEqual(3);

    // Forms & panels
    await expect(page.locator('#form-register-volunteer')).toBeVisible();
    await expect(page.locator('#form-issue-credential')).toBeVisible();
    await expect(page.locator('#form-checkin-shift')).toBeVisible();
    await expect(page.locator('#ics-roster-panel')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Registering a new volunteer via #form-register-volunteer adds them to #volunteers-leaderboard and #shift-volunteer-select', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ics-command`);
    await page.waitForLoadState('networkidle');

    const uniqueSuffix = Date.now();
    const volName = `Jordan Vance #${uniqueSuffix}`;
    const volCallsign = `EAGLE-${uniqueSuffix % 1000}`;

    await page.locator('#vol-name').fill(volName);
    await page.locator('#vol-callsign').fill(volCallsign);
    await page.locator('#vol-primary-role').selectOption('FIELD_SEARCHER');
    await page.locator('#vol-license').fill(`SAR-${uniqueSuffix % 1000}`);
    await page.locator('#btn-register-volunteer').click();

    const newCard = page.locator('#volunteers-leaderboard .volunteer-card', { hasText: volName });
    await expect(newCard).toBeVisible();
    await expect(newCard.locator('.badge-callsign')).toContainText(volCallsign);

    const volId = await newCard.getAttribute('data-volunteer-id');
    expect(volId).toBeTruthy();

    const shiftSelectOption = page.locator(`#shift-volunteer-select option[value="${volId}"]`);
    await expect(shiftSelectOption).toContainText(volName);
  });

  test('3. Role-Gate enforcement: attempting to check in an uncredentialed FIELD_SEARCHER into a UAV_PART107_PILOT shift displays #role-gate-error-alert, and issuing a signed UAV_PART107_PILOT credential subsequently allows check-in', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ics-command`);
    await page.waitForLoadState('networkidle');

    // Register a new FIELD_SEARCHER volunteer (does not have UAV_PART107_PILOT credential)
    const uniqueSuffix = Date.now();
    const volName = `Riley Mercer #${uniqueSuffix}`;
    const volCallsign = `SCOUT-${uniqueSuffix % 1000}`;

    await page.locator('#vol-name').fill(volName);
    await page.locator('#vol-callsign').fill(volCallsign);
    await page.locator('#vol-primary-role').selectOption('FIELD_SEARCHER');
    await page.locator('#btn-register-volunteer').click();

    const newCard = page.locator('#volunteers-leaderboard .volunteer-card', { hasText: volName });
    await expect(newCard).toBeVisible();
    const volId = await newCard.getAttribute('data-volunteer-id');
    expect(volId).toBeTruthy();

    // Attempt to check in as UAV_PART107_PILOT without credential
    await page.locator('#shift-incident-id').fill('INC-2026-ROLE-GATE');
    await page.locator('#shift-volunteer-select').selectOption(volId!);
    await page.locator('#shift-assigned-role').selectOption('UAV_PART107_PILOT');
    await page.locator('#shift-sector-id').fill('SECTOR-THERMAL-01');
    await page.locator('#btn-checkin-shift').click();

    const roleGateAlert = page.locator('#role-gate-error-alert');
    await expect(roleGateAlert).toBeVisible();
    await expect(roleGateAlert).toContainText('UAV_PART107_PILOT');

    // Now issue a signed UAV_PART107_PILOT credential via #form-issue-credential
    await page.locator('#cred-volunteer-select').selectOption(volId!);
    await page.locator('#cred-role-select').selectOption('UAV_PART107_PILOT');
    await page.locator('#cred-issued-by').fill('FAA / NASAR Command');
    await page.locator('#cred-license-number').fill(`FAA-107-${uniqueSuffix}`);
    await page.locator('#btn-issue-credential').click();

    await expect(
      newCard.locator('.badge-sar-credential[data-role="UAV_PART107_PILOT"]')
    ).toBeVisible();

    // Retry check-in into UAV_PART107_PILOT shift
    await page.locator('#shift-volunteer-select').selectOption(volId!);
    await page.locator('#shift-assigned-role').selectOption('UAV_PART107_PILOT');
    await page.locator('#btn-checkin-shift').click();

    await expect(roleGateAlert).toBeHidden();
    const shiftCard = page.locator('#ics-shifts-list .ics-shift-card', { hasText: volName });
    await expect(shiftCard).toBeVisible();
    await expect(shiftCard.locator('.badge-assigned-role')).toContainText('UAV_PART107_PILOT');
  });

  test('4. Fatigue Guardrails & Gamification Progression: simulating 6.5h updates shift to FATIGUE_WARNING, simulating 8.5h/52km escalates to MANDATORY_REST, and checkout transitions to CHECKED_OUT and awards XP & merit badges', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ics-command`);
    await page.waitForLoadState('networkidle');

    // Register a fresh FIELD_SEARCHER volunteer and check into a FIELD_SEARCHER shift
    const uniqueSuffix = Date.now();
    const volName = `Casey Rivera #${uniqueSuffix}`;
    const volCallsign = `RANGER-${uniqueSuffix % 1000}`;

    await page.locator('#vol-name').fill(volName);
    await page.locator('#vol-callsign').fill(volCallsign);
    await page.locator('#vol-primary-role').selectOption('FIELD_SEARCHER');
    await page.locator('#btn-register-volunteer').click();

    const volCard = page.locator('#volunteers-leaderboard .volunteer-card', { hasText: volName });
    await expect(volCard).toBeVisible();
    const volId = await volCard.getAttribute('data-volunteer-id');
    expect(volId).toBeTruthy();

    await page.locator('#shift-incident-id').fill('INC-2026-FATIGUE-TEST');
    await page.locator('#shift-volunteer-select').selectOption(volId!);
    await page.locator('#shift-assigned-role').selectOption('FIELD_SEARCHER');
    await page.locator('#shift-sector-id').fill('SECTOR-BRAVO-09');
    await page.locator('#btn-checkin-shift').click();

    const shiftCard = page.locator('#ics-shifts-list .ics-shift-card', { hasText: volName }).first();
    await expect(shiftCard).toBeVisible();
    await expect(shiftCard).toHaveAttribute('data-fatigue-level', 'NORMAL_FIT');
    await expect(shiftCard.locator('.badge-fatigue-normal')).toBeVisible();

    // 1. Simulate 6.5h Patrol -> FATIGUE_WARNING
    await shiftCard.locator('.btn-sim-fatigue-warn').click();
    await expect(shiftCard).toHaveAttribute('data-fatigue-level', 'FATIGUE_WARNING');
    await expect(shiftCard.locator('.badge-fatigue-warning')).toContainText('FATIGUE_WARNING');

    // 2. Simulate 8.5h / 52km Patrol -> MANDATORY_REST
    await shiftCard.locator('.btn-sim-mandatory-rest').click();
    await expect(shiftCard).toHaveAttribute('data-fatigue-level', 'MANDATORY_REST');
    await expect(shiftCard).toHaveAttribute('data-shift-status', 'MANDATORY_REST');
    await expect(shiftCard.locator('.badge-fatigue-rest')).toContainText('MANDATORY_REST');

    // 3. Check Out & Award XP -> CHECKED_OUT + Merit Badges
    await shiftCard.locator('.btn-checkout-shift').click();
    await expect(shiftCard).toHaveAttribute('data-shift-status', 'CHECKED_OUT');
    await expect(shiftCard.locator('.badge-shift-status')).toContainText('CHECKED_OUT');

    // Verify responder card earned FIRST_RESPONDER, GRID_MASTER_50KM, and REUNION_HERO badges
    await expect(volCard.locator('.badge-sar-merit[data-badge="FIRST_RESPONDER"]')).toBeVisible();
    await expect(volCard.locator('.badge-sar-merit[data-badge="GRID_MASTER_50KM"]')).toBeVisible();
    await expect(volCard.locator('.badge-sar-merit[data-badge="REUNION_HERO"]')).toBeVisible();
  });

  test('5. WCAG AAA contrast ratios (> 7:1) on .badge-fatigue-normal, .badge-fatigue-warning, .badge-fatigue-rest, and .badge-sar-rank in both dark and light modes', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/ics-command`);
    await page.waitForLoadState('networkidle');

    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-ics-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-fatigue-normal" class="badge badge-fatigue-level badge-fatigue-normal">NORMAL_FIT</span>
        <span id="test-badge-fatigue-warning" class="badge badge-fatigue-level badge-fatigue-warning">FATIGUE_WARNING</span>
        <span id="test-badge-fatigue-rest" class="badge badge-fatigue-level badge-fatigue-rest">MANDATORY_REST</span>
        <span id="test-badge-sar-rank" class="badge badge-sar-rank">Master Incident Vanguard (Tier 4)</span>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of [
        '#test-badge-fatigue-normal',
        '#test-badge-fatigue-warning',
        '#test-badge-fatigue-rest',
        '#test-badge-sar-rank',
      ]) {
        const styles = await page.locator(selector).evaluate((el) => {
          const cs = window.getComputedStyle(el);
          return { color: cs.color, backgroundColor: cs.backgroundColor };
        });
        expect(styles.backgroundColor).not.toBe('rgba(0, 0, 0, 0)');
        const ratio = contrastRatio(styles.color, styles.backgroundColor);
        expect(ratio, `${selector} in ${theme} mode should have > 7:1 contrast`).toBeGreaterThan(7.0);
      }
    }
  });
});
