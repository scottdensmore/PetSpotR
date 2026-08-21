import { expect, test } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

const renderedPages = ['/', '/report-lost', '/report-found', '/matches'];
const primaryNavigation = ['nav-home', 'nav-directory', 'nav-matches'];

for (const pagePath of renderedPages) {
  test(`keeps primary navigation visible and keyboard ordered on phones at ${pagePath}`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`${WEB_FRONTEND_URL}${pagePath}`);

    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);

    const header = page.locator('.glass-nav');
    const brand = page.locator('#brand-link');
    const navigation = page.getByRole('navigation', { name: 'Main Navigation' });
    const actions = page.locator('.nav-actions');

    await expect(header).toBeVisible();
    await expect(navigation).toBeVisible();

    const brandBox = await brand.boundingBox();
    const navigationBox = await navigation.boundingBox();
    const actionsBox = await actions.boundingBox();
    const headerBox = await header.boundingBox();
    const mainBox = await page.locator('#main-content').boundingBox();

    expect(brandBox).not.toBeNull();
    expect(navigationBox).not.toBeNull();
    expect(actionsBox).not.toBeNull();
    expect(headerBox).not.toBeNull();
    expect(mainBox).not.toBeNull();
    expect(navigationBox!.y).toBeGreaterThanOrEqual(brandBox!.y + brandBox!.height);
    expect(actionsBox!.y).toBeGreaterThanOrEqual(navigationBox!.y + navigationBox!.height);
    expect(mainBox!.y).toBeGreaterThanOrEqual(headerBox!.y + headerBox!.height);

    for (const id of primaryNavigation) {
      const link = page.locator(`#${id}`);
      await expect(link).toBeVisible();
      const box = await link.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.height).toBeGreaterThanOrEqual(44);
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(390);
    }

    await brand.focus();
    await expect(brand).toBeFocused();
    for (const id of primaryNavigation) {
      await page.keyboard.press('Tab');
      await expect(page.locator(`#${id}`)).toBeFocused();
    }
    await page.keyboard.press('Tab');
    await expect(page.locator('#theme-toggle')).toBeFocused();

    const scrollY = await page.evaluate(() => {
      window.scrollTo(0, document.documentElement.scrollHeight);
      return window.scrollY;
    });
    expect(scrollY).toBeGreaterThan(0);
    await expect.poll(async () => {
      const box = await header.boundingBox();
      return box ? box.y : 0;
    }).toBeLessThan(headerBox!.y);
  });
}
