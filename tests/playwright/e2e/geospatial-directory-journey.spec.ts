import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8102';

test.describe('User Journey: Geospatial Directory & Mapping', () => {
  test('should render public pet directory with filter controls', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    await expect(page.locator('h1')).toHaveText('Public Pet Directory');
    await expect(page.locator('#filter-species')).toBeVisible();
    await expect(page.locator('#filter-status')).toBeVisible();
    await expect(page.locator('#filter-query')).toBeVisible();
    await expect(page.locator('#btn-geolocation')).toBeVisible();
    await expect(page.locator('#btn-view-grid')).toBeVisible();
    await expect(page.locator('#btn-view-map')).toBeVisible();
  });

  test('should switch between Grid view and Leaflet Map view', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    // Initial state: Grid view active, Map view hidden
    await expect(page.locator('#btn-view-grid')).toHaveClass(/active/);
    await expect(page.locator('#btn-view-grid')).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('#pets-grid')).toBeVisible();
    await expect(page.locator('#pets-map-container')).toBeHidden();

    // Click Map button to switch view
    await page.locator('#btn-view-map').click();

    // Map view is now active
    await expect(page.locator('#btn-view-map')).toHaveClass(/active/);
    await expect(page.locator('#btn-view-map')).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('#btn-view-grid')).not.toHaveClass(/active/);
    await expect(page.locator('#btn-view-grid')).toHaveAttribute('aria-pressed', 'false');
    await expect(page.locator('#pets-map-container')).toBeVisible();

    // Verify Leaflet map initialized inside container
    await expect(page.locator('#pets-map')).toHaveClass(/leaflet-container/);

    // Switch back to Grid view
    await page.locator('#btn-view-grid').click();
    await expect(page.locator('#btn-view-grid')).toHaveClass(/active/);
    await expect(page.locator('#pets-grid')).toBeVisible();
    await expect(page.locator('#pets-map-container')).toBeHidden();
  });

  test('should support proximity radius filter controls', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    // Radius select starts disabled until location is set
    await expect(page.locator('#filter-radius')).toBeDisabled();

    // Set coordinates via URL parameters
    await page.goto(`${WEB_FRONTEND_URL}/pets?lat=47.6150&lng=-122.3200&radiusMiles=25`);

    // Proximity controls become active with Location Set label
    await expect(page.locator('#geo-btn-label')).toHaveText('Location Set');
    await expect(page.locator('#filter-radius')).toBeEnabled();
    await expect(page.locator('#filter-radius')).toHaveValue('25');
    await expect(page.locator('#btn-clear-geo')).toBeVisible();

    // Clear location
    await page.locator('#btn-clear-geo').click();
    await expect(page).toHaveURL(new RegExp(`${WEB_FRONTEND_URL}/pets\\?species=.*`));
  });
});
