import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8102';

test.describe('User Journey: In-App Notification Center & Alert Preferences', () => {
  test('should open notification drawer and show notifications or empty state', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const bellBtn = page.locator('#btn-notification-drawer');
    await expect(bellBtn).toBeVisible();

    // Drawer starts closed
    const drawer = page.locator('#notification-drawer');
    await expect(drawer).toHaveAttribute('aria-hidden', 'true');
    await expect(page.locator('#drawer-backdrop')).toBeHidden();

    // Click bell button to open drawer
    await bellBtn.click();
    await expect(bellBtn).toHaveAttribute('aria-expanded', 'true');
    await expect(drawer).toHaveAttribute('aria-hidden', 'false');
    await expect(drawer).toHaveClass(/is-open/);
    await expect(page.locator('#drawer-backdrop')).toBeVisible();

    // Drawer header actions are visible
    await expect(page.locator('#btn-mark-all-read')).toBeVisible();
    await expect(page.locator('#btn-open-preferences')).toBeVisible();
    await expect(page.locator('#btn-close-drawer')).toBeVisible();

    // Close drawer via close button
    await page.locator('#btn-close-drawer').dispatchEvent('click');
    await expect(bellBtn).toHaveAttribute('aria-expanded', 'false');
    await expect(drawer).toHaveAttribute('aria-hidden', 'true');
    await expect(drawer).not.toHaveClass(/is-open/);
  });

  test('should open and interact with alert preferences modal', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    // Open notification drawer
    await page.locator('#btn-notification-drawer').click();
    await expect(page.locator('#notification-drawer')).toHaveClass(/is-open/);

    // Click Settings button to open preferences modal
    await page.locator('#btn-open-preferences').click();

    const modal = page.locator('#notification-preferences-modal');
    await expect(modal).toBeVisible();
    await expect(modal).not.toHaveClass(/hidden/);
    await expect(page.locator('#pref-modal-title')).toHaveText('Alert & Zone Preferences');

    // Channel toggles are present and interactable
    const emailToggle = page.locator('#pref-email-enabled');
    const emailInput = page.locator('#pref-email-address');
    await expect(emailToggle).toBeVisible();
    await emailToggle.check();
    await expect(emailToggle).toBeChecked();
    await emailInput.fill('tester@example.com');
    await expect(emailInput).toHaveValue('tester@example.com');

    const smsToggle = page.locator('#pref-sms-enabled');
    const phoneInput = page.locator('#pref-phone-number');
    await expect(smsToggle).toBeVisible();
    await smsToggle.check();
    await phoneInput.fill('+12065550100');
    await expect(phoneInput).toHaveValue('+12065550100');

    // Alert radius selection
    const radiusSelect = page.locator('#pref-radius');
    await expect(radiusSelect).toBeVisible();
    await radiusSelect.selectOption('25');
    await expect(radiusSelect).toHaveValue('25');

    // Mini map is initialized
    await expect(page.locator('#zone-mini-map')).toHaveClass(/leaflet-container/);

    // Save preferences
    await page.locator('#btn-save-preferences').click();

    // Modal closes upon successful save
    await expect(modal).toBeHidden();
  });

  test('should close drawer when pressing Escape key or clicking backdrop', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    // Open drawer
    await page.locator('#btn-notification-drawer').click();
    await expect(page.locator('#notification-drawer')).toHaveClass(/is-open/);

    // Press Escape
    await page.keyboard.press('Escape');
    await expect(page.locator('#notification-drawer')).not.toHaveClass(/is-open/);

    // Reopen and click backdrop
    await page.locator('#btn-notification-drawer').click();
    await expect(page.locator('#notification-drawer')).toHaveClass(/is-open/);
    await page.locator('#drawer-backdrop').click({ force: true });
    await expect(page.locator('#notification-drawer')).not.toHaveClass(/is-open/);
  });
});
