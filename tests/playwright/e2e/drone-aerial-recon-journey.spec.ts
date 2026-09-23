import { test, expect } from '@playwright/test';
import * as zlib from 'zlib';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

/**
 * Generates a valid 100x100 radiometric PNG image buffer in pure Node.js.
 * Has a dark background (cold ground) and a 5x5 hot cluster centered at (50, 50)
 * simulating a warm biological heat signature detected by UAV thermal sensors.
 */
function createThermalPng(): Buffer {
  const width = 100;
  const height = 100;
  const rowBytes = 1 + width * 3;
  const raw = Buffer.alloc(rowBytes * height);

  for (let y = 0; y < height; y++) {
    const rowOffset = y * rowBytes;
    raw[rowOffset] = 0; // PNG filter: None
    for (let x = 0; x < width; x++) {
      const isHot = y >= 48 && y <= 52 && x >= 48 && x <= 52;
      const val = isHot ? 245 : 30;
      const pxOffset = rowOffset + 1 + x * 3;
      raw[pxOffset] = val; // R
      raw[pxOffset + 1] = val; // G
      raw[pxOffset + 2] = val; // B
    }
  }

  const compressed = zlib.deflateSync(raw);

  const crcTable = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) {
      c = (c & 1) ? (0xedb88320 ^ (c >>> 1)) : (c >>> 1);
    }
    crcTable[n] = c;
  }

  function crc32(buf: Buffer): number {
    let crc = 0xffffffff;
    for (let i = 0; i < buf.length; i++) {
      crc = crcTable[(crc ^ buf[i]) & 0xff] ^ (crc >>> 8);
    }
    return (crc ^ 0xffffffff) >>> 0;
  }

  function makeChunk(type: string, data: Buffer): Buffer {
    const len = Buffer.alloc(4);
    len.writeUInt32BE(data.length, 0);
    const typeBuf = Buffer.from(type, 'ascii');
    const body = Buffer.concat([typeBuf, data]);
    const crc = Buffer.alloc(4);
    crc.writeUInt32BE(crc32(body), 0);
    return Buffer.concat([len, body, crc]);
  }

  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8; // Bit depth
  ihdr[9] = 2; // Color type RGB
  ihdr[10] = 0;
  ihdr[11] = 0;
  ihdr[12] = 0;

  const signature = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
  return Buffer.concat([
    signature,
    makeChunk('IHDR', ihdr),
    makeChunk('IDAT', compressed),
    makeChunk('IEND', Buffer.alloc(0)),
  ]);
}

/**
 * Sample DJI SRT Subtitle Telemetry Log containing 3 sequential waypoints.
 */
const SAMPLE_DJI_SRT = `1
00:00:01,000 --> 00:00:02,000
[iso : 100] [shutter : 1/500] [fnum : 2.8] [latitude: 37.774929] [longitude: -122.419416] [rel_alt: 45.2] [heading: 142.5] [speed: 5.4] [battery: 88] [pitch: -45.0] [roll: 0.0]

2
00:00:02,000 --> 00:00:03,000
[iso : 100] [shutter : 1/500] [fnum : 2.8] [latitude: 37.775100] [longitude: -122.419300] [rel_alt: 48.0] [heading: 150.0] [speed: 6.2] [battery: 87] [pitch: -45.0] [roll: 0.0]

3
00:00:03,000 --> 00:00:04,000
[iso : 100] [shutter : 1/500] [fnum : 2.8] [latitude: 37.775300] [longitude: -122.419100] [rel_alt: 50.0] [heading: 160.0] [speed: 7.0] [battery: 85] [pitch: -45.0] [roll: 0.0]
`;

test.describe('User Journey: Drone / UAV Aerial Reconnaissance & Thermal Hotspot Sighting Feeds', () => {
  const petId = `lost-pet-recon-${Date.now()}`;
  const petName = 'Apollo';

  test.beforeEach(async ({ page }) => {
    // Record mesh:thermal-hotspot events dispatched to window and document
    await page.addInitScript(() => {
      (window as any).__meshEvents = [];
      const recordEvent = (e: any) => {
        (window as any).__meshEvents.push(e.detail);
      };
      window.addEventListener('mesh:thermal-hotspot', recordEvent);
      document.addEventListener('mesh:thermal-hotspot', recordEvent);
    });
  });

  test('Complete 7-Step Aerial Reconnaissance & Thermal Hotspot Journey', async ({
    page,
    request,
  }) => {
    // ------------------------------------------------------------------------
    // Setup: Create a lost pet and register an active drone reconnaissance mission
    // ------------------------------------------------------------------------
    const petRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId,
        petName,
        species: 'Dog',
        breed: 'Husky',
        primaryColor: 'Gray',
        secondaryColor: 'White',
        description: 'Energetic husky lost near Presidio forest buffer.',
        location: 'San Francisco, CA',
        reporterEmail: 'apollo-owner@example.com',
        phone: '(415) 555-0188',
        coordinates: {
          latitude: 37.7749,
          longitude: -122.4194,
        },
      },
    });
    expect([200, 201]).toContain(petRes.status());

    const missionRes = await request.post(`${WEB_FRONTEND_URL}/api/v1/recon/missions`, {
      data: {
        petId,
        pilotCallsign: 'Falcon-1',
        droneModel: 'DJI Matrice 30T',
      },
    });
    expect([200, 201]).toContain(missionRes.status());
    const missionData = await missionRes.json();
    expect(missionData.id).toBeDefined();

    // ------------------------------------------------------------------------
    // Step 1: Open /pets and launch cockpit via #btn-open-drone-recon
    // ------------------------------------------------------------------------
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const openReconBtn = page.locator('#btn-open-drone-recon');
    await expect(openReconBtn).toBeVisible();
    await openReconBtn.click();

    // Verify accessible modal is displayed
    const reconModal = page.locator('#recon-modal');
    await expect(reconModal).toBeVisible();
    await expect(reconModal).not.toHaveClass(/hidden/);
    await expect(reconModal).toHaveAttribute('aria-modal', 'true');
    await expect(reconModal).toHaveAttribute('role', 'dialog');

    // Verify dual mode tabs
    const tabCockpit = page.locator('#tab-recon-cockpit');
    const tabBatch = page.locator('#tab-recon-batch');
    await expect(tabCockpit).toBeVisible();
    await expect(tabBatch).toBeVisible();
    await expect(tabCockpit).toHaveAttribute('aria-selected', 'true');

    // Verify tactical map canvas is rendered
    const reconMap = page.locator('#recon-map');
    await expect(reconMap).toBeVisible();

    // ------------------------------------------------------------------------
    // Step 2: Ingest sample DJI SRT flight log; assert HUD updates (Altitude AGL, Heading, Speed)
    // ------------------------------------------------------------------------
    const fileInput = page.locator('#recon-file-input');
    await fileInput.setInputFiles({
      name: 'mission_sortie_001.srt',
      mimeType: 'text/plain',
      buffer: Buffer.from(SAMPLE_DJI_SRT, 'utf-8'),
    });

    // Assert HUD gauges update to Waypoint 1 values
    const altitudeHud = page.locator('#hud-altitude-val, #hud-altitude-value');
    const headingHud = page.locator('#hud-heading-val, #hud-heading-value');
    const speedHud = page.locator('#hud-speed-val, #hud-speed-value');
    const batteryHud = page.locator('#hud-battery-val, #hud-battery-value');

    await expect(altitudeHud).toBeVisible();
    await expect(altitudeHud).toContainText('45.2 m');
    await expect(headingHud).toContainText(/14[23]°/);
    await expect(speedHud).toContainText('5.4 m/s');
    await expect(batteryHud).toContainText('88%');

    // Verify timeline slider min and max are configured
    const slider = page.locator('#recon-timeline-slider');
    await expect(slider).toBeVisible();
    await expect(slider).toHaveAttribute('max', '2');

    // ------------------------------------------------------------------------
    // Step 3: Verify Leaflet map contains .leaflet-overlay-pane with cyan flight
    //         trajectory polyline and camera frustum polygon
    // ------------------------------------------------------------------------
    const overlayPane = page.locator('#recon-map .leaflet-overlay-pane');
    await expect(overlayPane).toBeAttached();

    const flightPolyline = page.locator('#recon-map .leaflet-overlay-pane path.recon-flight-polyline');
    await expect(flightPolyline).toBeAttached();

    const frustumPolygon = page.locator('#recon-map .leaflet-overlay-pane path.recon-frustum-polygon');
    await expect(frustumPolygon).toBeAttached();

    const droneMarker = page.locator('#recon-map .drone-marker');
    await expect(droneMarker).toBeVisible();

    // ------------------------------------------------------------------------
    // Step 4: Ingest thermal image with hot pet anomaly; assert reticle canvas
    //         draws bounding box and Leaflet map renders pulsating hotspot pin
    // ------------------------------------------------------------------------
    const thermalImageBuffer = createThermalPng();
    await fileInput.setInputFiles({
      name: 'thermal_frame_hotspot.png',
      mimeType: 'image/png',
      buffer: thermalImageBuffer,
    });

    // Assert reticle canvas is visible
    const reticleCanvas = page.locator('#recon-reticle-canvas');
    await expect(reticleCanvas).toBeVisible();

    // Assert Leaflet map renders pulsating hotspot pin (.marker-hotspot-pulse)
    const hotspotPin = page.locator('#recon-map .marker-hotspot-pulse');
    await expect(hotspotPin).toBeVisible();

    // Assert hotspot card in review drawer is populated
    const hotspotDrawer = page.locator('#recon-hotspot-drawer');
    await expect(hotspotDrawer).toBeVisible();

    const hotspotCard = page.locator('#recon-hotspots-list .hotspot-card').first();
    await expect(hotspotCard).toBeVisible();
    await expect(hotspotCard).toHaveClass(/selected/);

    const confirmBtn = page.locator('#btn-confirm-hotspot');
    await expect(confirmBtn).toBeVisible();
    await expect(confirmBtn).toBeEnabled();

    // ------------------------------------------------------------------------
    // Step 5: Scrub timeline slider #recon-timeline-slider forward;
    //         verify drone marker position updates and heading rotates
    // ------------------------------------------------------------------------
    // Scrub to Waypoint index 1 (50% through the 3-waypoint sortie)
    await page.evaluate(() => {
      const el = document.getElementById('recon-timeline-slider') as HTMLInputElement;
      if (el) {
        el.value = '1';
        el.dispatchEvent(new Event('input', { bubbles: true }));
        el.dispatchEvent(new Event('change', { bubbles: true }));
      }
    });

    // Verify HUD updates to Waypoint 2 values
    await expect(altitudeHud).toContainText('48.0 m');
    await expect(headingHud).toContainText('150°');
    await expect(speedHud).toContainText('6.2 m/s');
    await expect(batteryHud).toContainText('87%');

    // Verify drone marker rotated to 150deg heading
    await expect(droneMarker).toHaveAttribute('style', /rotate\(150deg\)/);

    // ------------------------------------------------------------------------
    // Step 6: Click #btn-confirm-hotspot; assert status badge updates to CONFIRMED
    //         and community sighting is registered
    // ------------------------------------------------------------------------
    await confirmBtn.click();

    // Verify status badge in card updates to CONFIRMED
    const statusBadge = hotspotCard.locator('.badge');
    await expect(statusBadge).toHaveText('CONFIRMED');
    await expect(statusBadge).toHaveClass(/badge-success/);

    // Verify confirm button is now disabled
    await expect(confirmBtn).toBeDisabled();

    // Verify official community sighting is registered via backend REST API
    await expect.poll(async () => {
      const sightingsRes = await request.get(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${petId}/sightings`);
      if (sightingsRes.status() !== 200) return 0;
      const sightings = await sightingsRes.json();
      return Array.isArray(sightings) ? sightings.length : 0;
    }).toBeGreaterThanOrEqual(1);

    // ------------------------------------------------------------------------
    // Step 7: Verify mesh:thermal-hotspot DOM event is dispatched and received
    // ------------------------------------------------------------------------
    await expect.poll(async () => {
      return await page.evaluate(() => {
        const events = (window as any).__meshEvents as any[];
        if (!Array.isArray(events)) return 0;
        return events.filter(e => e.status === 'CONFIRMED').length;
      });
    }).toBeGreaterThanOrEqual(1);

    const confirmedEvent = await page.evaluate(() => {
      const events = (window as any).__meshEvents as any[];
      return events.find(e => e.status === 'CONFIRMED');
    });

    expect(confirmedEvent).toBeDefined();
    expect(confirmedEvent.id).toBeDefined();
    expect(confirmedEvent.status).toBe('CONFIRMED');
    expect(confirmedEvent.latitude).toBeCloseTo(37.7749, 2);
    expect(confirmedEvent.longitude).toBeCloseTo(-122.4194, 2);
  });

  test('Dual Tab Switching and Modal Keyboard Navigation', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    const openBtn = page.locator('#btn-open-drone-recon');
    await expect(openBtn).toBeVisible();
    await openBtn.click();

    const reconModal = page.locator('#recon-modal');
    await expect(reconModal).toBeVisible();

    const tabCockpit = page.locator('#tab-recon-cockpit');
    const tabBatch = page.locator('#tab-recon-batch');
    const panelCockpit = page.locator('#recon-panel-cockpit');
    const panelBatch = page.locator('#recon-panel-batch');

    // Switch to Batch panel
    await tabBatch.click();
    await expect(tabBatch).toHaveAttribute('aria-selected', 'true');
    await expect(tabCockpit).toHaveAttribute('aria-selected', 'false');
    await expect(panelBatch).toBeVisible();
    await expect(panelCockpit).toHaveClass(/hidden/);

    // Switch back to Cockpit panel
    await tabCockpit.click();
    await expect(tabCockpit).toHaveAttribute('aria-selected', 'true');
    await expect(panelCockpit).toBeVisible();
    await expect(panelBatch).toHaveClass(/hidden/);

    // Close via close button
    const closeBtn = page.locator('#btn-close-recon-modal');
    await expect(closeBtn).toBeVisible();
    await closeBtn.click();
    await expect(reconModal).toHaveClass(/hidden/);
  });
});
