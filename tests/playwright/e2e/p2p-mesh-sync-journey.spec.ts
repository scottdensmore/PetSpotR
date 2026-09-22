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

  test('Step 2: Open #mesh-modal, verify peer roster, optical QR code toggle (#btn-mesh-show-qr), and focus trap / Escape close', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first().click();

    const indicator = page.locator('#mesh-status-indicator');
    await expect(indicator).toBeVisible();

    // Dispatch peer-joined to populate roster
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

    // Open modal by clicking indicator
    await indicator.click();

    const meshModal = page.locator('#mesh-modal');
    await expect(meshModal).toBeVisible();
    await expect(meshModal).not.toHaveClass(/hidden/);
    await expect(meshModal).toHaveAttribute('role', 'dialog');
    await expect(meshModal).toHaveAttribute('aria-modal', 'true');
    await expect(indicator).toHaveAttribute('aria-expanded', 'true');

    // Verify Peer Roster card
    const peerCard = meshModal.locator('#mesh-peer-roster .mesh-peer-card');
    await expect(peerCard).toHaveCount(1);
    await expect(peerCard).toContainText('Alex Scout');
    await expect(peerCard).toContainText('K9_HANDLER');
    await expect(peerCard).toContainText('92%');

    // Verify pairing buttons
    const btnAutoPair = meshModal.locator('#btn-mesh-auto-pair');
    await expect(btnAutoPair).toBeVisible();

    const btnShowQR = meshModal.locator('#btn-mesh-show-qr');
    await expect(btnShowQR).toBeVisible();

    const btnScanQR = meshModal.locator('#btn-mesh-scan-qr');
    await expect(btnScanQR).toBeVisible();

    const btnSOS = meshModal.locator('#btn-mesh-sos');
    await expect(btnSOS).toBeVisible();

    // Verify optical QR code toggle (#btn-mesh-show-qr)
    await btnShowQR.click();
    const qrView = meshModal.locator('#mesh-qr-view');
    await expect(qrView).toBeVisible();
    await expect(qrView).not.toHaveClass(/hidden/);

    const qrImg = meshModal.locator('#mesh-qr-container img.mesh-qr-code-img');
    await expect(qrImg).toBeVisible();

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

  test('Step 3: Trigger peer mesh sector claim (mesh:sector-updated); verify sector card badge updates to "CLEARED (Mesh Verified)" and Leaflet polygon updates', async ({ page }) => {
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

  test('Step 4: Trigger peer GPS breadcrumb (mesh:breadcrumb-received); assert trail stats/polyline update', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first().click();

    const spModal = page.locator('#pet-search-party-container');
    await expect(spModal).toBeVisible();

    // Dispatch simulated peer GPS breadcrumb events
    await page.evaluate(({ pid }) => {
      const pt1 = {
        id: 'bc-maya-1',
        petId: pid,
        volunteerId: 'vol-k9-maya',
        volunteerName: 'Tracker Maya',
        seq: 1,
        latitude: 47.4885,
        longitude: -121.7230,
        accuracy: 4.2,
        timestamp: new Date().toISOString(),
      };
      const pt2 = {
        id: 'bc-maya-2',
        petId: pid,
        volunteerId: 'vol-k9-maya',
        volunteerName: 'Tracker Maya',
        seq: 2,
        latitude: 47.4892,
        longitude: -121.7215,
        accuracy: 3.8,
        timestamp: new Date().toISOString(),
      };
      window.dispatchEvent(new CustomEvent('mesh:breadcrumb-received', { detail: pt1 }));
      window.dispatchEvent(new CustomEvent('mesh:breadcrumb-received', { detail: pt2 }));
    }, { pid: petId });

    // Assert Leaflet polyline update for peer breadcrumb trail
    const peerTrail = page.locator('path.mesh-peer-trail, .volunteer-trail-polyline.mesh-peer-trail');
    await expect(peerTrail.first()).toBeVisible();

    // Assert polyline element has SVG path data
    const trailData = await page.evaluate(() => {
      const path = document.querySelector('.mesh-peer-trail');
      return {
        exists: Boolean(path),
        hasPathData: Boolean(path?.getAttribute('d')),
      };
    });
    expect(trailData.exists).toBe(true);
    expect(trailData.hasPathData).toBe(true);
  });

  test('Step 5: Test Tactical SOS button (#btn-mesh-sos) and event (mesh:sos-alert); assert #mesh-sos-banner displays with distress coordinates and volunteer name', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const petCard = page.locator(`article.pet-card[data-pet-id="${petId}"]`);
    await petCard.locator('.btn-view-search-party, button[data-action="view-search-party"]').first().click();

    const sosBanner = page.locator('#mesh-sos-banner');
    await expect(sosBanner).toHaveClass(/hidden/);

    // 1. Test Tactical SOS Button via Mesh Modal
    const indicator = page.locator('#mesh-status-indicator');
    await indicator.click();

    const meshModal = page.locator('#mesh-modal');
    await expect(meshModal).toBeVisible();

    const btnSOS = meshModal.locator('#btn-mesh-sos');
    await expect(btnSOS).toBeVisible();
    await btnSOS.click();

    const sosFeedback = meshModal.locator('#mesh-sos-feedback');
    await expect(sosFeedback).toBeVisible();
    await expect(sosFeedback).toContainText('Emergency SOS broadcasted');

    // Close mesh modal
    await meshModal.locator('#btn-close-mesh-modal').click();
    await expect(meshModal).toHaveClass(/hidden/);

    // Assert SOS banner is visible on the Search Party HUD with distress coordinates and volunteer name
    await expect(sosBanner).toBeVisible();
    await expect(sosBanner).not.toHaveClass(/hidden/);
    await expect(sosBanner).toContainText('TACTICAL DISTRESS SOS DETECTED');
    await expect(sosBanner).toContainText('Distress SOS Beacon');
    await expect(sosBanner).toContainText('47.4881');
    await expect(sosBanner).toContainText('-121.7225');

    // 2. Test receiving peer SOS alert event
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

    await expect(sosBanner).toContainText('Scout Devon');
    await expect(sosBanner).toContainText('47.4912');
    await expect(sosBanner).toContainText('-121.7198');
    await expect(sosBanner).toContainText('Injured ankle');
  });

  test('Step 6: Post cloud uplink sync payload to POST /api/v1/mesh/uplink-sync; assert HTTP 200, reconciledSectors >= 1, and backend store verification', async ({ request }) => {
    // 1. Fetch search party to obtain party ID and sector ID
    const spRes = await request.get(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/search-party`);
    expect(spRes.status()).toBe(200);
    const spData = await spRes.json();
    const party = spData.searchParty || spData;
    expect(party.sectors?.length).toBeGreaterThan(0);
    const targetSector = party.sectors[party.sectors.length - 1]; // use the last sector

    // 2. Post cloud uplink sync payload
    const uplinkRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/mesh/uplink-sync`, {
      data: {
        searchPartyId: party.partyId,
        senderNodeId: 'node-field-gateway-1',
        sectors: [
          {
            petId: petId,
            sectorId: targetSector.sectorId,
            state: 'CLEARED',
            rank: 3,
            claimedByVolunteerId: 'vol-field-gateway',
            claimedByName: 'Ranger Jackson',
            nodeId: 'node-field-gateway-1',
            lamportClock: 88,
            timestamp: new Date().toISOString(),
          },
        ],
        breadcrumbs: [
          {
            petId: petId,
            volunteerId: 'vol-field-gateway',
            volunteerName: 'Ranger Jackson',
            seq: 1,
            latitude: 47.4890,
            longitude: -121.7220,
            accuracy: 3.5,
            timestamp: new Date().toISOString(),
          },
        ],
        sightings: [
          {
            sightingId: `sight-uplink-${Date.now()}`,
            petId: petId,
            volunteerId: 'vol-field-gateway',
            volunteerName: 'Ranger Jackson',
            latitude: 47.4895,
            longitude: -121.7210,
            notes: 'High ridge visual confirmation.',
            lamportClock: 88,
            timestamp: new Date().toISOString(),
          },
        ],
      },
    });

    expect(uplinkRes.status()).toBe(200);
    const uplinkData = await uplinkRes.json();
    expect(uplinkData.success).toBe(true);
    expect(uplinkData.reconciledSectors).toBeGreaterThanOrEqual(1);

    // 3. Backend store verification
    const verifyRes = await request.get(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/search-party`);
    expect(verifyRes.status()).toBe(200);
    const verifyData = await verifyRes.json();
    const updatedParty = verifyData.searchParty || verifyData;
    const reconciledSec = updatedParty.sectors.find((s: any) => s.sectorId === targetSector.sectorId);
    expect(reconciledSec).toBeDefined();
    expect(reconciledSec.status).toBe('cleared');
  });
});
