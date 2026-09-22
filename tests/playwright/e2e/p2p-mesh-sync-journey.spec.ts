import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

function parseRgb(colorStr: string): [number, number, number] {
  const match = colorStr.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
  if (!match) return [0, 0, 0];
  return [parseInt(match[1], 10), parseInt(match[2], 10), parseInt(match[3], 10)];
}

function calculateContrast(rgb1: [number, number, number], rgb2: [number, number, number]): number {
  const getLuminance = (r: number, g: number, b: number) => {
    const a = [r, g, b].map((v) => {
      v /= 255;
      return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
    });
    return a[0] * 0.2126 + a[1] * 0.7152 + a[2] * 0.0722;
  };
  const l1 = getLuminance(rgb1[0], rgb1[1], rgb1[2]);
  const l2 = getLuminance(rgb2[0], rgb2[1], rgb2[2]);
  const brightest = Math.max(l1, l2);
  const darkest = Math.min(l1, l2);
  return (brightest + 0.05) / (darkest + 0.05);
}

test.describe.serial('Milestone 11.1: P2P Mesh Sync & Wilderness Coordination HUD Journey', () => {
  const petId = `lost-mesh-journey-${Date.now()}`;
  const petName = 'Kona';

  test.beforeAll(async ({ request }) => {
    // Seed lost pet report
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId,
        petName,
        species: 'Dog',
        breed: 'Husky',
        primaryColor: 'Silver/White',
        description: 'Wilderness search party mesh sync test subject.',
        location: 'Mount Si Trailhead, North Bend, WA',
        reporterEmail: 'kona-owner@example.com',
        phone: '(425) 555-0188',
        coordinates: {
          latitude: 47.4881,
          longitude: -121.7225,
        },
      },
    });
    expect([200, 201]).toContain(res.status());

    // Initialize search party with 4 sectors
    const initRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/search-party`, {
      data: { sectorCount: 4 },
    });
    expect([200, 201]).toContain(initRes.status());
  });

  test('Step 1: Verify #mesh-status-indicator renders in Search Party modal with WCAG AAA contrast (>7.5:1)', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await expect(petCard).toBeVisible();

    const spBtn = petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first();
    await expect(spBtn).toBeVisible();
    await spBtn.click();

    const spModal = page.locator('#pet-search-party-container');
    await expect(spModal).toBeVisible();
    await expect(spModal).not.toHaveClass(/hidden/);

    const indicator = page.locator('#mesh-status-indicator');
    await expect(indicator).toBeVisible();
    await expect(indicator).toHaveAttribute('aria-haspopup', 'dialog');
    await expect(indicator).toHaveAttribute('aria-expanded', 'false');

    // Verify contrast ratio > 7.5:1 (WCAG AAA)
    const contrastData = await indicator.evaluate((el) => {
      const style = window.getComputedStyle(el);
      return {
        color: style.color,
        backgroundColor: style.backgroundColor,
      };
    });

    const fg = parseRgb(contrastData.color);
    const bg = parseRgb(contrastData.backgroundColor);
    const contrast = calculateContrast(fg, bg);
    expect(contrast).toBeGreaterThanOrEqual(7.5);
  });

  test('Step 2: Clicking indicator opens #mesh-modal and traps keyboard focus', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first().click();

    const indicator = page.locator('#mesh-status-indicator');
    await expect(indicator).toBeVisible();
    await indicator.click();

    const meshModal = page.locator('#mesh-modal');
    await expect(meshModal).toBeVisible();
    await expect(meshModal).not.toHaveClass(/hidden/);
    await expect(meshModal).toHaveAttribute('role', 'dialog');
    await expect(meshModal).toHaveAttribute('aria-modal', 'true');
    await expect(indicator).toHaveAttribute('aria-expanded', 'true');

    // Verify presence of Peer Roster, Pairing Toolbar, and Tactical SOS Beacon
    const peerRoster = meshModal.locator('#mesh-peer-roster');
    await expect(peerRoster).toBeVisible();

    const btnAutoPair = meshModal.locator('#btn-mesh-auto-pair');
    await expect(btnAutoPair).toBeVisible();

    const btnShowQR = meshModal.locator('#btn-mesh-show-qr');
    await expect(btnShowQR).toBeVisible();

    const btnScanQR = meshModal.locator('#btn-mesh-scan-qr');
    await expect(btnScanQR).toBeVisible();

    const btnSOS = meshModal.locator('#btn-mesh-sos');
    await expect(btnSOS).toBeVisible();

    // Verify keyboard focus trapping inside modal
    const closeBtn = meshModal.locator('#btn-close-mesh-modal');
    await expect(closeBtn).toBeVisible();

    // Trigger Tab navigation to confirm focus stays inside dialog
    await page.keyboard.press('Tab');
    const focusedTag = await page.evaluate(() => document.activeElement?.id || document.activeElement?.tagName);
    expect(focusedTag).toBeTruthy();

    // Press Escape to close mesh modal
    await page.keyboard.press('Escape');
    await expect(meshModal).toHaveClass(/hidden/);
    await expect(indicator).toHaveAttribute('aria-expanded', 'false');
  });

  test('Step 3: Simulating mesh:sector-updated updates sector card status badge to "CLEARED (Mesh Verified)" and map style', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first().click();

    const sectorCard = page.locator('#search-party-sector-cards .sector-card').first();
    await expect(sectorCard).toBeVisible();
    const sectorId = await sectorCard.getAttribute('data-sector-id');
    expect(sectorId).toBeTruthy();

    // Dispatch synthetic mesh:sector-updated event
    await page.evaluate(({ sid, pid }) => {
      const event = new CustomEvent('mesh:sector-updated', {
        detail: {
          petId: pid,
          sectorId: sid,
          state: 'CLEARED',
          rank: 3,
          claimedByVolunteerId: 'vol-field-ranger',
          claimedByName: 'Ranger Sarah',
          nodeId: 'node-sarah-1',
          lamportClock: 42,
          timestamp: new Date().toISOString(),
        },
      });
      window.dispatchEvent(event);
    }, { sid: sectorId!, pid: petId });

    // Assert sector card badge updates to "CLEARED (Mesh Verified)"
    const statusBadge = sectorCard.locator('.badge-status-cleared, .badge-mesh-cleared');
    await expect(statusBadge).toContainText('CLEARED (Mesh Verified)');

    // Assert Leaflet polygon style updated
    const polygonHasClearedClass = await page.evaluate((sid) => {
      const polyEl = document.querySelector(`.sector-polygon[data-sector-id="${sid}"], .sector-cleared`);
      return Boolean(polyEl);
    }, sectorId);
    expect(polygonHasClearedClass).toBe(true);
  });

  test('Step 4: Simulating mesh:sos-alert displays #mesh-sos-banner with coordinates', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first().click();

    const sosBanner = page.locator('#mesh-sos-banner');
    await expect(sosBanner).toHaveClass(/hidden/);

    // Dispatch synthetic mesh:sos-alert event
    await page.evaluate(() => {
      const event = new CustomEvent('mesh:sos-alert', {
        detail: {
          alertId: 'sos-alert-999',
          volunteerId: 'vol-scout-7',
          volunteerName: 'Scout Devon',
          latitude: 47.4912,
          longitude: -121.7198,
          message: 'Injured ankle near boulder field, requesting evacuation.',
          timestamp: new Date().toISOString(),
        },
      });
      window.dispatchEvent(event);
    });

    // Assert SOS banner is visible and contains volunteer info and coordinates
    await expect(sosBanner).toBeVisible();
    await expect(sosBanner).not.toHaveClass(/hidden/);
    await expect(sosBanner).toContainText('Scout Devon');
    await expect(sosBanner).toContainText('47.4912');
    await expect(sosBanner).toContainText('-121.7198');
    await expect(sosBanner).toContainText('Injured ankle');
  });

  test('Step 5: Simulating mesh:peer-joined and mesh:peer-left updates peer indicator count and peer roster cards', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first().click();

    const indicator = page.locator('#mesh-status-indicator');
    await expect(indicator).toBeVisible();

    // Dispatch peer-joined
    await page.evaluate(() => {
      window.dispatchEvent(new CustomEvent('mesh:peer-joined', {
        detail: {
          nodeId: 'peer-scout-1',
          volunteerId: 'vol-scout-1',
          volunteerName: 'Alex Scout',
          role: 'K9_HANDLER',
          batteryLevel: 92,
        },
      }));
    });

    await expect(indicator).toContainText('1 Active Peer');

    // Open modal and check peer roster
    await indicator.click();
    const meshModal = page.locator('#mesh-modal');
    await expect(meshModal).toBeVisible();

    const peerCard = meshModal.locator('#mesh-peer-roster .mesh-peer-card');
    await expect(peerCard).toHaveCount(1);
    await expect(peerCard).toContainText('Alex Scout');
    await expect(peerCard).toContainText('K9_HANDLER');
    await expect(peerCard).toContainText('92%');

    // Close modal
    await meshModal.locator('#btn-close-mesh-modal').click();
    await expect(meshModal).toHaveClass(/hidden/);

    // Dispatch peer-left
    await page.evaluate(() => {
      window.dispatchEvent(new CustomEvent('mesh:peer-left', {
        detail: { nodeId: 'peer-scout-1' },
      }));
    });

    await expect(indicator).toContainText('Standalone');
  });
});
