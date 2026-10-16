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

test.describe('Smart Camera Trap & Edge Frame Command Center UI', () => {
  test('1. /cameras page renders the Command Center, registration form, simulator, and IR phosphor toggle without CSP errors', async ({ page }) => {
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

    await page.goto(`${WEB_FRONTEND_URL}/cameras`);
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#camera-command-center')).toBeVisible();
    await expect(page.locator('h1')).toContainText('Smart Camera Trap & Edge Frame Command Center');
    await expect(page.locator('#camera-register-form')).toBeVisible();
    await expect(page.locator('#webhook-simulator-form')).toBeVisible();
    await expect(page.locator('#toggle-ir-phosphor')).toBeVisible();
    await expect(page.locator('#toggle-ir-phosphor')).toHaveAttribute('aria-pressed', 'false');
    await expect(page.locator('#filter-exclude-duplicates')).toBeVisible();

    expect(cspErrors).toEqual([]);
  });

  test('2. Registering a camera node displays the webhook token, adds the node to #camera-nodes-list, and populates #sim-camera-select', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/cameras`);
    await page.waitForLoadState('networkidle');

    const cameraName = `North Woods Trail Cam #${Date.now()}`;
    await page.locator('#camera-name').fill(cameraName);
    await page.locator('#camera-vendor').selectOption('TRAIL_CAM');
    await page.locator('#camera-target-pet').fill('pet-lost-101');
    await page.locator('#camera-lat').fill('47.6062');
    await page.locator('#camera-lng').fill('-122.3321');
    await page.locator('#btn-register-camera').click();

    const credsBanner = page.locator('#camera-webhook-credentials');
    await expect(credsBanner).toBeVisible();
    await expect(page.locator('#issued-webhook-token')).toContainText('cam_tok_');
    await expect(page.locator('#issued-webhook-url')).toContainText('/api/v1/cameras/');

    const nodeCard = page.locator('#camera-nodes-list .camera-node-card', { hasText: cameraName });
    await expect(nodeCard).toBeVisible();
    const cameraId = await nodeCard.getAttribute('data-camera-id');
    expect(cameraId).toBeTruthy();

    await expect(page.locator('#sim-camera-select')).toHaveValue(cameraId!);
    const simTokenVal = await page.locator('#sim-token-input').inputValue();
    expect(simTokenVal).toContain('cam_tok_');
  });

  test('3. Transmitting a simulated edge frame renders a .camera-capture-card with a visible .roi-bounding-box and .badge-capture-pending', async ({ page }) => {
    const cspErrors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error' && /Content Security Policy/i.test(msg.text())) {
        cspErrors.push(msg.text());
      }
    });

    await page.goto(`${WEB_FRONTEND_URL}/cameras`);
    await page.waitForLoadState('networkidle');

    const cameraName = `Edge Sim Cam #${Date.now()}`;
    await page.locator('#camera-name').fill(cameraName);
    await page.locator('#camera-vendor').selectOption('RING');
    await page.locator('#camera-target-pet').fill('pet-lost-101');
    await page.locator('#btn-register-camera').click();

    await expect(page.locator('#camera-webhook-credentials')).toBeVisible();

    await page.locator('#sim-scene-preset').selectOption('night_ir_dog');
    await page.locator('#sim-species-hint').selectOption('DOG');
    await page.locator('#sim-traits-input').fill('Reflective collar, medium build, alert ears');
    await page.locator('#btn-send-webhook-frame').click();

    const captureCard = page.locator('#camera-captures-grid .camera-capture-card').first();
    await expect(captureCard).toBeVisible();
    await expect(captureCard).toHaveAttribute('data-verification-status', 'PENDING_REVIEW');
    await expect(captureCard.locator('.roi-bounding-box')).toBeVisible();
    await expect(captureCard.locator('.roi-bounding-box')).toHaveAttribute('style', /left:.*top:.*width:.*height:/i);
    await expect(captureCard.locator('.badge-capture-pending')).toBeVisible();
    await expect(captureCard.locator('.capture-dhash-badge')).toContainText('dHash:');
    await expect(captureCard.locator('.capture-confidence-badge')).toContainText('DOG');

    // Promote capture to sighting and verify badge transitions to PROMOTED
    await captureCard.locator('.btn-promote-capture').click();
    await expect(captureCard).toHaveAttribute('data-verification-status', 'PROMOTED');
    await expect(captureCard.locator('.badge-capture-promoted')).toBeVisible();
    await expect(captureCard.locator('.promoted-sighting-badge')).toContainText('Sighting ID:');

    expect(cspErrors).toEqual([]);
  });

  test('4. Toggling #toggle-ir-phosphor applies .ir-phosphor-mode and sets aria-pressed="true"', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/cameras`);
    await page.waitForLoadState('networkidle');

    const toggleBtn = page.locator('#toggle-ir-phosphor');
    const commandCenter = page.locator('#camera-command-center');

    await expect(toggleBtn).toHaveAttribute('aria-pressed', 'false');
    await expect(commandCenter).not.toHaveClass(/ir-phosphor-mode/);

    await toggleBtn.click();
    await expect(toggleBtn).toHaveAttribute('aria-pressed', 'true');
    await expect(toggleBtn).toContainText('Night-Vision IR Phosphor: ON');
    await expect(commandCenter).toHaveClass(/ir-phosphor-mode/);

    await toggleBtn.click();
    await expect(toggleBtn).toHaveAttribute('aria-pressed', 'false');
    await expect(toggleBtn).toContainText('Night-Vision IR Phosphor: OFF');
    await expect(commandCenter).not.toHaveClass(/ir-phosphor-mode/);
  });

  test('5. WCAG AAA contrast ratios (> 7:1) on .badge-capture-pending, .badge-capture-promoted, and .badge-capture-duplicate', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/cameras`);
    await page.waitForLoadState('networkidle');

    // Inject sample badges inside the page to measure computed styles in both dark and light themes
    await page.evaluate(() => {
      const container = document.createElement('div');
      container.id = 'wcag-badge-test-fixture';
      container.innerHTML = `
        <span id="test-badge-pending" class="badge-capture-status badge-capture-pending">PENDING_REVIEW</span>
        <span id="test-badge-promoted" class="badge-capture-status badge-capture-promoted">PROMOTED</span>
        <span id="test-badge-duplicate" class="badge-capture-status badge-capture-duplicate">DUPLICATE_SUPPRESSED</span>
        <span id="test-badge-dismissed" class="badge-capture-status badge-capture-dismissed">DISMISSED</span>
      `;
      document.body.appendChild(container);
    });

    for (const theme of ['dark', 'light']) {
      await page.evaluate((t) => {
        document.documentElement.setAttribute('data-theme', t);
      }, theme);

      for (const selector of ['#test-badge-pending', '#test-badge-promoted', '#test-badge-duplicate', '#test-badge-dismissed']) {
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
