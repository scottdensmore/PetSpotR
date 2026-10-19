import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

test.describe('Brachycephalic Obstructive Airway Syndrome (BOAS), Laryngeal Paralysis & Temporary Tracheostomy Triage Cockpit E2E Journey', () => {
  test('Full operator journey: seeded cockpit load, creating new BOAS operation "Otis", staging Butorphanol/Dex-SP/Intubation stabilization, logging aseptic tracheostomy care, issuing extubation clearance, and filtering by PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED', async ({
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

    // Step 1: Cockpit Load & Seeded Operations
    await page.goto(`${WEB_FRONTEND_URL}/airway-tracheostomy-triage`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#airway-tracheostomy-triage-main')).toBeVisible();
    await expect(page.locator('h1')).toContainText(
      'Brachycephalic Obstructive Airway Syndrome (BOAS), Laryngeal Paralysis & Temporary Tracheostomy Triage Cockpit'
    );

    await expect(page.locator('#airway-operations-list .airway-op-card[data-op-id="AT-OP-101"]')).toBeVisible();
    await expect(page.locator('#airway-operations-list .airway-op-card[data-op-id="AT-OP-102"]')).toBeVisible();
    await expect(page.locator('#airway-operations-list .airway-op-card[data-op-id="AT-OP-103"]')).toBeVisible();

    // Step 2: Create a new critical BOAS operation ("Otis")
    await page.locator('#create-incident-id').fill('INC-AT-9004');
    await page.locator('#create-pet-name').fill('Otis');
    await page.locator('#create-species').selectOption('CANINE_BRACHYCEPHALIC_BOAS');
    await page.locator('#create-breed').fill('English Bulldog');
    await page.locator('#create-weight-kg').fill('24.0');
    await page.locator('#create-rectal-temp-f').fill('105.4');
    await page.locator('#create-spo2-pct').fill('85.0');
    await page.locator('#create-inspiratory-effort').selectOption('SEVERE_PARADOXICAL_CYANOTIC');
    await page.locator('#create-edema-grade').selectOption('GRADE_III_EVERTED_SACCULES_COLLAPSE');

    await page.locator('#btn-create-airway-op').click();

    const otisInitialCard = page
      .locator('#airway-operations-list .airway-op-card', {
        hasText: 'Otis (',
      })
      .first();
    await expect(otisInitialCard).toBeVisible();
    const otisOpId = await otisInitialCard.getAttribute('data-op-id');
    expect(otisOpId).toBeTruthy();

    const otisCard = page.locator(
      `#airway-operations-list .airway-op-card[data-op-id="${otisOpId}"]`
    );
    await expect(otisCard).toHaveAttribute(
      'data-status',
      'STRIDOR_CYANOSIS_DYSPNEA_CRITICAL'
    );
    await expect(page.locator('#workspace-op-title')).toContainText('Otis');
    await expect(page.locator('#airway-alert-banner')).toContainText('Butorphanol');

    // Step 3: Stage Stabilization Plan on Otis
    await page.locator('#stage-rectal-temp-f').fill('102.5');
    await page.locator('#stage-spo2-pct').fill('94.0');
    await page.locator('#stage-inspiratory-effort').selectOption('MODERATE_STRIDOR_ORTHOPNEA');
    await page.locator('#stage-edema-grade').selectOption('GRADE_III_EVERTED_SACCULES_COLLAPSE');

    await page.locator('#btn-stage-stabilization').click();

    await expect(otisCard).toHaveAttribute(
      'data-status',
      'BUTORPHANOL_DEX_INTUBATION_STAGED'
    );

    // Step 4: Log Aseptic Tracheostomy Care on Otis
    await page.locator('#care-preox-sec').fill('90');
    await page.locator('#care-suction-sec').fill('7');
    await page.locator('#care-saline-ml').fill('1.5');
    await page.locator('#care-post-spo2').fill('97.0');
    await page.locator('#care-tech-callsign').fill('RVT-LEE-09');
    await page.locator('#care-notes').fill('Pre-oxygenated 90s; inner cannula cleaned; 7s suction pass.');
    await page.locator('#care-inner-cannula').check();

    await page.locator('#btn-record-trach-care').click();

    await expect(page.locator('#workspace-care-logs-list')).toContainText('RVT-LEE-09');

    // Step 5: Issue Extubation Clearance on Otis
    await page.locator('#clearance-spo2-pct').fill('97.5');
    await page.locator('#clearance-temp-f').fill('101.0');
    await page.locator('#clearance-vet-license').fill('DVM-WA-77410');
    await page.locator('#clearance-summary').fill(
      'Otis extubated with vigorous gag reflex, SpO2 97.5% on room air, normothermic 101.0°F, chest harness verified.'
    );
    await page.locator('#clearance-gag-vigorous').check();
    await page.locator('#clearance-trach-occluded').check();
    await page.locator('#clearance-harness-only').check();

    await page.locator('#btn-issue-extubation-clearance').click();

    await expect(otisCard).toHaveAttribute(
      'data-status',
      'PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED'
    );
    await expect(page.locator('#airway-clearance-banner')).toBeVisible();
    await expect(page.locator('#airway-clearance-banner')).toContainText('DVM-WA-77410');

    // Step 6: Filter by PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED and verify Otis and Pippin remain visible
    await page.locator('#filter-status').selectOption('PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED');
    await expect(
      page.locator('#airway-operations-list .airway-op-card', { hasText: 'Otis (' })
    ).toBeVisible();
    await expect(
      page.locator('#airway-operations-list .airway-op-card', { hasText: 'Pippin (' })
    ).toBeVisible();

    await page.locator('#filter-status').selectOption('');

    expect(cspErrors).toEqual([]);
  });
});
