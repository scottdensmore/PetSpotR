import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe('User Journey: Shelter Analytics Dashboard & Reconciliation Exports', () => {
  test('should navigate to shelter analytics dashboard, inspect KPIs, and download exports', async ({ page }) => {
    // 1. Visit /shelters/analytics
    await page.goto(`${WEB_FRONTEND_URL}/shelters/analytics`);

    // 2. Assert page title and heading
    await expect(page.locator('h1')).toContainText('Shelter Partner Analytics');

    // 3. Assert KPI cards are visible
    await expect(page.locator('#kpi-rto-rate')).toBeVisible();
    await expect(page.locator('#kpi-turnaround')).toBeVisible();
    await expect(page.locator('#kpi-microchip-rate')).toBeVisible();
    await expect(page.locator('#kpi-deterministic-ratio')).toBeVisible();

    // 4. Assert shelter filter dropdown exists and date range filter exists
    const shelterFilter = page.locator('#shelter-filter');
    await expect(shelterFilter).toBeVisible();
    const dateRangeFilter = page.locator('#date-range-filter');
    await expect(dateRangeFilter).toBeVisible();

    // 5. Verify CSV export button link and content
    const csvExportBtn = page.locator('#btn-export-csv');
    await expect(csvExportBtn).toBeVisible();
    const csvHref = await csvExportBtn.getAttribute('href');
    expect(csvHref).toContain('/api/v1/shelters/analytics/export.csv');

    // 6. Verify GeoJSON export button link
    const geojsonExportBtn = page.locator('#btn-export-geojson');
    await expect(geojsonExportBtn).toBeVisible();
    const geojsonHref = await geojsonExportBtn.getAttribute('href');
    expect(geojsonHref).toContain('/api/v1/shelters/analytics/export.geojson');

    // 7. Verify direct CSV API response
    const csvResponse = await page.request.get(`${WEB_FRONTEND_URL}/api/v1/shelters/analytics/export.csv`);
    expect(csvResponse.status()).toBe(200);
    expect(csvResponse.headers()['content-type']).toContain('text/csv');
    const csvText = await csvResponse.text();
    expect(csvText).toContain('IntakeID,ShelterID');

    // 8. Verify direct GeoJSON API response
    const geojsonResponse = await page.request.get(`${WEB_FRONTEND_URL}/api/v1/shelters/analytics/export.geojson`);
    expect(geojsonResponse.status()).toBe(200);
    expect(geojsonResponse.headers()['content-type']).toContain('application/geo+json');
    const geojsonBody = await geojsonResponse.json();
    expect(geojsonBody.type).toBe('FeatureCollection');

    // 9. Test filter interaction: change #date-range-filter to 7d, verify export URLs update
    await dateRangeFilter.selectOption('7d');
    await expect(csvExportBtn).toHaveAttribute('href', /\/api\/v1\/shelters\/analytics\/export\.csv\?.*range=7d/);
    await expect(geojsonExportBtn).toHaveAttribute('href', /\/api\/v1\/shelters\/analytics\/export\.geojson\?.*range=7d/);
  });

  test('should navigate to shelter analytics via navigation bar link from another page', async ({ page }) => {
    // Navigate to /pets first
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    // Locate and click Shelter Analytics navigation link
    const navAnalytics = page.locator('#nav-analytics');
    await expect(navAnalytics).toBeVisible();
    await navAnalytics.click();

    // Confirm navigation to /shelters/analytics
    await expect(page).toHaveURL(`${WEB_FRONTEND_URL}/shelters/analytics`);
    await expect(page.locator('h1')).toContainText('Shelter Partner Analytics');
  });

  test('should update export URLs and UI metrics when shelter filter is updated', async ({ page }) => {
    const mockReport = {
      overallKpis: {
        returnToOwnerRate: 0.85,
        medianIntakeToReunionHours: 24.5,
        medianIntakeToMatchHours: 2.1,
        microchipScanRate: 0.75,
        deterministicMatchRatio: 0.60,
      },
      shelterBreakdown: [
        {
          shelterId: 'shelter-sea-01',
          shelterName: 'Seattle Animal Shelter',
          intakeCount: 20,
          activeCareCount: 3,
          reunitedCount: 17,
          returnToOwnerRate: 0.85,
          microchipScanRate: 0.75,
        },
      ],
      availableShelters: [
        { id: 'shelter-sea-01', name: 'Seattle Animal Shelter' },
        { id: 'shelter-bel-02', name: 'Bellevue Humane Society' },
      ],
    };

    await page.route('**/api/v1/shelters/analytics*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(mockReport),
      });
    });

    await page.goto(`${WEB_FRONTEND_URL}/shelters/analytics`);

    // Verify populated KPIs
    await expect(page.locator('#kpi-rto-rate')).toHaveText('85.0%');
    await expect(page.locator('#kpi-turnaround')).toHaveText('24.5h');
    await expect(page.locator('#kpi-microchip-rate')).toHaveText('75.0%');
    await expect(page.locator('#kpi-deterministic-ratio')).toHaveText('60.0%');

    // Verify shelter dropdown option is populated and select shelter-sea-01
    const shelterFilter = page.locator('#shelter-filter');
    await expect(shelterFilter.locator('option[value="shelter-sea-01"]')).toBeAttached();
    await shelterFilter.selectOption('shelter-sea-01');

    // Verify export button hrefs include shelterId
    const csvExportBtn = page.locator('#btn-export-csv');
    const geojsonExportBtn = page.locator('#btn-export-geojson');
    await expect(csvExportBtn).toHaveAttribute('href', /shelterId=shelter-sea-01/);
    await expect(geojsonExportBtn).toHaveAttribute('href', /shelterId=shelter-sea-01/);

    // Also change date range to 7d and verify both query parameters exist
    await page.locator('#date-range-filter').selectOption('7d');
    const updatedCsvHref = await csvExportBtn.getAttribute('href');
    expect(updatedCsvHref).toContain('shelterId=shelter-sea-01');
    expect(updatedCsvHref).toContain('range=7d');

    const updatedGeojsonHref = await geojsonExportBtn.getAttribute('href');
    expect(updatedGeojsonHref).toContain('shelterId=shelter-sea-01');
    expect(updatedGeojsonHref).toContain('range=7d');
  });
});
