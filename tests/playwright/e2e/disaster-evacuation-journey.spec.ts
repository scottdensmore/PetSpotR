import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe('User Journey: Disaster Evacuation Federation, Bulk Intake & Mutual Aid Transfers', () => {
  const targetMicrochipId = '985141000123456';
  const uniquePetName = `Barnaby-${Date.now()}`;
  const lostPetId = `lost-evac-${Date.now()}`;

  test('complete disaster evacuation federation user journey: bulk intake, reconciliation, and mutual aid transfer', async ({ page, request }) => {
    // -------------------------------------------------------------------------
    // 1. Setup: Register lost dog with known microchip ID via POST /api/v1/lost-pets
    // -------------------------------------------------------------------------
    const lostPetPayload = {
      petId: lostPetId,
      petName: uniquePetName,
      species: 'Dog',
      breed: 'Golden Retriever',
      primaryColor: 'Golden',
      description: 'Barnaby displaced during regional disaster evacuation.',
      microchipId: targetMicrochipId,
      location: 'Capitol Hill, Seattle, WA',
      reporterEmail: 'barnaby-owner@example.com',
      phone: '(206) 555-0144',
      coordinates: {
        latitude: 47.6205,
        longitude: -122.3212,
      },
    };

    const createLostPetResp = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: lostPetPayload,
    });
    expect([200, 201]).toContain(createLostPetResp.status());

    // -------------------------------------------------------------------------
    // 2. Dashboard HUD: Navigate to /evacuation and verify metrics & facilities
    // -------------------------------------------------------------------------
    await page.goto(`${WEB_FRONTEND_URL}/evacuation`);
    await expect(page).toHaveTitle(/Disaster Evacuation Operations/i);

    // Verify metrics strip
    const activeHubsMetric = page.locator('#metric-active-hubs');
    await expect(activeHubsMetric).toBeVisible();
    await expect(activeHubsMetric).not.toHaveText('0');
    const activeHubsCount = parseInt((await activeHubsMetric.textContent()) || '0', 10);
    expect(activeHubsCount).toBeGreaterThanOrEqual(2);

    const openCapacityMetric = page.locator('#metric-open-capacity');
    await expect(openCapacityMetric).toBeVisible();
    await expect(openCapacityMetric).not.toHaveText('0');
    const openCapacityVal = parseInt((await openCapacityMetric.textContent()) || '0', 10);
    expect(openCapacityVal).toBeGreaterThan(0);

    const totalEvacuatedMetric = page.locator('#metric-evacuated-total');
    await expect(totalEvacuatedMetric).toBeVisible();

    // Verify Facilities panel card grid
    const hubsCardGrid = page.locator('#hubs-card-grid');
    await expect(hubsCardGrid).toBeVisible();
    await expect(hubsCardGrid.locator('.hub-card')).toHaveCount(activeHubsCount);
    await expect(hubsCardGrid).toContainText('Seattle Center Exhibition Hall');
    await expect(hubsCardGrid).toContainText('Magnuson Park Community Staging');

    // -------------------------------------------------------------------------
    // 3. Bulk Intake: Switch tab, select hub, upload CSV roster, process batch
    // -------------------------------------------------------------------------
    await page.locator('#tab-intake').click();
    const intakePanel = page.locator('#tab-panel-intake');
    await expect(intakePanel).toBeVisible();

    // Select target emergency hub in #intake-hub-select
    const intakeHubSelect = page.locator('#intake-hub-select');
    await expect(intakeHubSelect).toBeVisible();
    await intakeHubSelect.selectOption('hub-seattle-center');

    // Prepare CSV buffer with 3 animals (1 matching chip, 1 avid chip, 1 invalid chip)
    const csvContent = [
      'species,breed,primary_color,gender,microchip_id,address,notes',
      `dog,Golden Retriever,Golden,male,${targetMicrochipId},"2061 15th Ave W, Seattle, WA","Found near Interbay"`,
      'cat,Domestic Shorthair,Tabby,female,456789123,"Mercer St, Seattle, WA","Scanned Avid chip"',
      'dog,Poodle,White,male,invalid-chip,"Rainier Ave, Seattle, WA","Triage check ok"',
    ].join('\n');

    const csvBuffer = Buffer.from(csvContent, 'utf-8');

    // Attach CSV roster via #intake-file-input using setInputFiles
    await page.locator('#intake-file-input').setInputFiles({
      name: 'disaster-roster-batch.csv',
      mimeType: 'text/csv',
      buffer: csvBuffer,
    });

    // Verify file details indicator
    const fileDetails = page.locator('#intake-file-details');
    await expect(fileDetails).toBeVisible();
    await expect(fileDetails).toContainText('disaster-roster-batch.csv');

    // Click #btn-process-batch
    const processBatchBtn = page.locator('#btn-process-batch');
    await expect(processBatchBtn).toBeEnabled();
    await processBatchBtn.click();

    // -------------------------------------------------------------------------
    // 4. Ingestion Table & Summary Assertions
    // -------------------------------------------------------------------------
    const summaryCard = page.locator('#intake-batch-summary');
    await expect(summaryCard).toBeVisible();
    await expect(page.locator('#summary-total-rows')).toHaveText('3');
    await expect(page.locator('#summary-imported-rows')).toHaveText('3');
    await expect(page.locator('#summary-matched-rows')).toHaveText('1');
    await expect(page.locator('#summary-error-rows')).toHaveText('0');

    // Assert results table rows
    const resultRows = page.locator('#intake-results-body tr');
    await expect(resultRows).toHaveCount(3);

    // Assert microchip standard badges
    await expect(page.locator('#intake-results-table .badge-info')).toContainText('ISO 15-Digit');
    await expect(page.locator('#intake-results-table .badge-secondary')).toContainText('Avid 9-Digit');
    await expect(page.locator('#intake-results-table .badge-warning')).toContainText('Invalid Format');

    // Assert exact microchip match badge
    const exactMatchBadge = page.locator('#intake-results-table .badge-microchip-exact');
    await expect(exactMatchBadge).toBeVisible();
    await expect(exactMatchBadge).toContainText('Exact Microchip Match');

    // -------------------------------------------------------------------------
    // 5. Crisis Reunification Dispatch
    // -------------------------------------------------------------------------
    await page.locator('#tab-reunifications').click();
    const reunificationsPanel = page.locator('#tab-panel-reunifications');
    await expect(reunificationsPanel).toBeVisible();

    // Assert #reunifications-grid displays matched pair with exact microchip match badge
    const matchCard = page.locator('#reunifications-grid .crisis-match-card').first();
    await expect(matchCard).toBeVisible();
    await expect(matchCard.locator('.badge-microchip-exact')).toContainText('Exact Microchip Match');
    await expect(matchCard).toContainText(targetMicrochipId);

    // Click owner contact action button
    const contactBtn = matchCard.locator('button.btn-contact-owner');
    await expect(contactBtn).toBeVisible();
    await expect(contactBtn).toBeEnabled();
    await contactBtn.click();

    // Assert button updates to "Contacted ✓" and is disabled
    await expect(contactBtn).toContainText('Contacted ✓');
    await expect(contactBtn).toBeDisabled();
    await expect(matchCard.locator('.reunification-time')).toContainText('Contacted ✓');

    // -------------------------------------------------------------------------
    // 6. Mutual Aid Transfers Lifecycle
    // -------------------------------------------------------------------------
    await page.locator('#tab-transfers').click();
    const transfersPanel = page.locator('#tab-panel-transfers');
    await expect(transfersPanel).toBeVisible();

    // Open modal
    await page.locator('#btn-stage-transfer').click();
    const stageModal = page.locator('#modal-stage-transfer');
    await expect(stageModal).toBeVisible();
    await expect(stageModal).not.toHaveClass(/hidden/);

    // Fill transfer manifest details
    await page.locator('#input-transfer-origin').selectOption('hub-seattle-center');
    await page.locator('#input-transfer-destination').selectOption('hub-magnuson-park');
    await page.locator('#input-transfer-pet-ids').fill('evac-transfer-001');
    await page.locator('#input-transfer-coordinator').fill('Alex Rivera - Transport Command');
    await page.locator('#input-transfer-phone').fill('(206) 555-0188');
    await page.locator('#input-transfer-vehicle').fill('Transport Van 4 - Climate Controlled');

    // Submit modal form
    await page.locator('#btn-submit-transfer').click();

    // Verify modal is dismissed
    await expect(stageModal).toHaveClass(/hidden/);

    // Assert new transfer row appears with status STAGED
    const transferRow = page.locator('#transfers-ledger-body tr').first();
    await expect(transferRow).toBeVisible();
    await expect(transferRow.locator('.transfer-status-staged')).toContainText('Staged');
    await expect(transferRow).toContainText('Seattle Center Exhibition Hall');
    await expect(transferRow).toContainText('Magnuson Park Community Staging');
    await expect(transferRow).toContainText('1 animal(s)');

    // Click action button to transition to IN_TRANSIT
    const transitBtn = transferRow.locator('button.btn-transit-transfer');
    await expect(transitBtn).toBeVisible();
    await transitBtn.click();

    // Assert badge updates to IN_TRANSIT
    await expect(transferRow.locator('.transfer-status-intransit')).toContainText('In Transit');

    // Click action button to transition to RECEIVED
    const receiveBtn = transferRow.locator('button.btn-receive-transfer');
    await expect(receiveBtn).toBeVisible();
    await receiveBtn.click();

    // Assert badge updates to RECEIVED
    await expect(transferRow.locator('.transfer-status-received')).toContainText('Received');

    // Verify ledger status filter
    const statusFilter = page.locator('#transfers-status-filter');
    await statusFilter.selectOption('RECEIVED');
    await expect(page.locator('#transfers-ledger-body tr')).toHaveCount(1);
    await expect(page.locator('#transfers-ledger-body tr .transfer-status-received')).toBeVisible();

    // -------------------------------------------------------------------------
    // 7. Verify Hub Occupancy Updates
    // -------------------------------------------------------------------------
    await page.locator('#tab-hubs').click();
    await expect(page.locator('#tab-panel-hubs')).toBeVisible();

    // Seattle Center: received 3 during intake, transferred 1 out -> occupancy is 2 / 150
    const seattleCenterCard = page.locator('.hub-card', { hasText: 'Seattle Center Exhibition Hall' });
    await expect(seattleCenterCard).toBeVisible();
    await expect(seattleCenterCard.locator('.occupancy-gauge-value')).toContainText('2 / 150');

    // Magnuson Park: received 1 from mutual aid transfer -> occupancy is 1 / 100
    const magnusonParkCard = page.locator('.hub-card', { hasText: 'Magnuson Park Community Staging' });
    await expect(magnusonParkCard).toBeVisible();
    await expect(magnusonParkCard.locator('.occupancy-gauge-value')).toContainText('1 / 100');

    // Total regional evacuated animals is 3
    await expect(page.locator('#metric-evacuated-total')).toHaveText('3');
  });
});
