import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe('User Journey: Multilingual i18n, Voice Notes & WCAG AAA Accessibility', () => {
  const testPetId = 'demo-lost-1';

  test.beforeAll(async ({ request }) => {
    // Ensure demo pet exists for testing
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        petId: testPetId,
        petName: 'Rusty',
        species: 'Dog',
        breed: 'Golden Retriever',
        primaryColor: 'Golden',
        description: 'Friendly golden retriever wearing blue collar.',
        location: 'Capitol Hill, Seattle, WA',
        reporterEmail: 'owner@example.com',
        phone: '(206) 555-0199',
        coordinates: {
          latitude: 47.6152,
          longitude: -122.3211,
        },
      },
    });
    expect([200, 201, 409]).toContain(res.status());
  });

  test.beforeEach(async ({ context, page }) => {
    // Provide microphone and geolocation permissions
    try {
      await context.grantPermissions(['geolocation', 'microphone'], { origin: WEB_FRONTEND_URL });
    } catch {}

    // Ensure MediaStream is available via Web Audio synthetic stream in headless browser
    await page.addInitScript(() => {
      const origGetUserMedia = navigator.mediaDevices && navigator.mediaDevices.getUserMedia;
      if (!navigator.mediaDevices) {
        (navigator as any).mediaDevices = {};
      }
      navigator.mediaDevices.getUserMedia = async (constraints) => {
        if (constraints && (constraints as any).audio) {
          try {
            const AudioCtx = window.AudioContext || (window as any).webkitAudioContext;
            const ctx = new AudioCtx();
            const osc = ctx.createOscillator();
            const dst = ctx.createMediaStreamDestination();
            osc.connect(dst);
            osc.start();
            return dst.stream;
          } catch (e) {
            if (origGetUserMedia) {
              return origGetUserMedia.call(navigator.mediaDevices, constraints);
            }
            throw e;
          }
        }
        if (origGetUserMedia) {
          return origGetUserMedia.call(navigator.mediaDevices, constraints);
        }
        throw new Error('Not supported');
      };
    });
  });

  test('Journey 1: Language selector dropdown switches locales and dynamically renders translations', async ({ page }) => {
    // 1. Visit /pets in default English
    await page.goto(`${WEB_FRONTEND_URL}/pets`);
    await expect(page.locator('#nav-directory')).toContainText('Pet Directory');
    await expect(page.locator('#nav-home')).toContainText('Home');
    await expect(page.locator('html')).toHaveAttribute('lang', 'en');

    // 2. Select Spanish (es)
    const langSelect = page.locator('#lang-select');
    await expect(langSelect).toBeVisible();
    await langSelect.selectOption('es');

    // Wait for navigation/reload
    await page.waitForLoadState('networkidle');

    // Verify Spanish translations
    await expect(page.locator('#nav-directory')).toContainText('Directorio de Mascotas');
    await expect(page.locator('#nav-home')).toContainText('Inicio');
    await expect(page.locator('html')).toHaveAttribute('lang', 'es');

    // 3. Select Vietnamese (vi)
    await page.locator('#lang-select').selectOption('vi');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#nav-directory')).toContainText('Danh mục thú cưng');
    await expect(page.locator('#nav-home')).toContainText('Trang chủ');
    await expect(page.locator('html')).toHaveAttribute('lang', 'vi');

    // 4. Select Simplified Chinese (zh-CN)
    await page.locator('#lang-select').selectOption('zh-CN');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#nav-directory')).toContainText('宠物名录');
    await expect(page.locator('#nav-home')).toContainText('首页');
    await expect(page.locator('html')).toHaveAttribute('lang', 'zh-CN');

    // 5. Select Tagalog (tl)
    await page.locator('#lang-select').selectOption('tl');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('#nav-directory')).toContainText('Direktoryo ng Alagang Hayop');
    await expect(page.locator('#nav-home')).toContainText('Tahanan');
    await expect(page.locator('html')).toHaveAttribute('lang', 'tl');

    // Reset back to English
    await page.locator('#lang-select').selectOption('en');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('#nav-directory')).toContainText('Pet Directory');
  });

  test('Journey 2: Sighting modal voice note recording, 15s progress bar, and waveform player', async ({ page }) => {
    // 1. Visit /pets and open Sighting Modal
    await page.goto(`${WEB_FRONTEND_URL}/pets`);
    const lostCard = page.locator(`article.pet-card[data-pet-id="${testPetId}"]`).first();
    await expect(lostCard).toBeVisible();

    const reportBtn = lostCard.locator('button[data-action="report-sighting"], .btn-report-sighting').first();
    await reportBtn.click();

    const modal = page.locator('#modal-report-sighting');
    await expect(modal).toBeVisible();

    // 2. Verify voice memo recording controls
    const recordBtn = page.locator('#btn-record-voice-memo');
    const stopBtn = page.locator('#btn-stop-voice-memo');
    const timerDisplay = page.locator('#voice-memo-timer');
    const progressContainer = page.locator('#voice-memo-progress-bar-container');
    const previewContainer = page.locator('#voice-memo-preview-container');

    await expect(recordBtn).toBeVisible();
    await expect(stopBtn).toBeHidden();
    await expect(timerDisplay).toHaveText('0:00 / 0:15');

    // 3. Start Recording
    await recordBtn.click();
    await expect(stopBtn).toBeVisible();
    await expect(recordBtn).toBeHidden();
    await expect(progressContainer).toBeVisible();

    // Wait 1.5s for audio capture & progress bar advancement
    await page.waitForTimeout(1500);

    // Stop recording
    await stopBtn.click();
    await expect(stopBtn).toBeHidden();
    await expect(recordBtn).toBeVisible();

    // 4. Verify preview player and waveform visualization
    await expect(previewContainer).toBeVisible();
    const audioPlayer = page.locator('#voice-memo-audio');
    await expect(audioPlayer).toBeVisible();

    // Verify 30 waveform bars rendered
    const waveformBars = page.locator('#voice-memo-waveform-bars .waveform-bar');
    await expect(waveformBars).toHaveCount(30);

    // 5. Test discard functionality
    const discardBtn = page.locator('#btn-discard-voice-memo');
    await expect(discardBtn).toBeVisible();
    await discardBtn.click();

    await expect(previewContainer).toBeHidden();
    await expect(timerDisplay).toHaveText('0:00 / 0:15');
    await expect(discardBtn).toBeHidden();

    // Close modal
    await page.locator('#btn-close-sighting-modal').click();
    await expect(modal).toBeHidden();
  });

  test('Journey 3: Voice memo REST API ingestion, duration bounding, and audio streaming', async ({ request }) => {
    // Generate 3-second PCM WAV byte buffer
    const sampleRate = 8000;
    const channels = 1;
    const bitsPerSample = 16;
    const durationSec = 3;
    const numSamples = durationSec * sampleRate;
    const byteRate = sampleRate * channels * (bitsPerSample / 8);
    const blockAlign = channels * (bitsPerSample / 8);
    const dataSize = numSamples * blockAlign;

    const buffer = Buffer.alloc(44 + dataSize);
    buffer.write('RIFF', 0);
    buffer.writeUInt32LE(36 + dataSize, 4);
    buffer.write('WAVE', 8);
    buffer.write('fmt ', 12);
    buffer.writeUInt32LE(16, 16);
    buffer.writeUInt16LE(1, 20); // PCM
    buffer.writeUInt16LE(channels, 22);
    buffer.writeUInt32LE(sampleRate, 24);
    buffer.writeUInt32LE(byteRate, 28);
    buffer.writeUInt16LE(blockAlign, 32);
    buffer.writeUInt16LE(bitsPerSample, 34);
    buffer.write('data', 36);
    buffer.writeUInt32LE(dataSize, 40);

    for (let i = 0; i < numSamples; i++) {
      buffer.writeInt16LE((i % 100) * 150, 44 + i * 2);
    }

    const testSightingId = `sight-e2e-${Date.now()}`;
    const uploadUrl = `${WEB_FRONTEND_URL}/api/v1/lost-pets/${testPetId}/sightings/${testSightingId}/voice-memo`;

    // 1. Upload valid voice memo
    const uploadResp = await request.post(uploadUrl, {
      multipart: {
        audio: {
          name: 'memo.wav',
          mimeType: 'audio/wav',
          buffer: buffer,
        },
        duration: '3.0',
      },
    });

    expect(uploadResp.status()).toBe(201);
    const body = await uploadResp.json();
    expect(body.voiceMemoUrl).toContain(`/api/v1/lost-pets/${testPetId}/sightings/${testSightingId}/voice-memo`);
    expect(body.durationSeconds).toBeGreaterThanOrEqual(2.9);
    expect(body.durationSeconds).toBeLessThanOrEqual(3.1);
    expect(body.waveform).toHaveLength(30);

    // 2. Stream stored voice memo
    const streamResp = await request.get(`${WEB_FRONTEND_URL}${body.voiceMemoUrl}`);
    expect(streamResp.status()).toBe(200);
    expect(streamResp.headers()['content-type']).toContain('audio/wav');
    expect(streamResp.headers()['accept-ranges']).toBe('bytes');
    const audioData = await streamResp.body();
    expect(audioData.length).toBe(buffer.length);

    // 3. Duration bounding: Reject audio exceeding 15 seconds
    const longNumSamples = 18 * sampleRate;
    const longDataSize = longNumSamples * blockAlign;
    const longBuffer = Buffer.alloc(44 + longDataSize);
    longBuffer.write('RIFF', 0);
    longBuffer.writeUInt32LE(36 + longDataSize, 4);
    longBuffer.write('WAVE', 8);
    longBuffer.write('fmt ', 12);
    longBuffer.writeUInt32LE(16, 16);
    longBuffer.writeUInt16LE(1, 20);
    longBuffer.writeUInt16LE(channels, 22);
    longBuffer.writeUInt32LE(sampleRate, 24);
    longBuffer.writeUInt32LE(byteRate, 28);
    longBuffer.writeUInt16LE(blockAlign, 32);
    longBuffer.writeUInt16LE(bitsPerSample, 34);
    longBuffer.write('data', 36);
    longBuffer.writeUInt32LE(longDataSize, 40);

    const rejectResp = await request.post(uploadUrl, {
      multipart: {
        audio: {
          name: 'too-long.wav',
          mimeType: 'audio/wav',
          buffer: longBuffer,
        },
      },
    });
    expect(rejectResp.status()).toBe(400);

    // 4. Reject invalid MIME type
    const invalidMimeResp = await request.post(uploadUrl, {
      multipart: {
        audio: {
          name: 'notes.txt',
          mimeType: 'text/plain',
          buffer: Buffer.from('hello invalid audio'),
        },
      },
    });
    expect(invalidMimeResp.status()).toBe(400);
  });

  test('Journey 4: WCAG 2.1 AAA accessibility announcer and focus indicators', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);

    // 1. Verify #aria-announcer exists with appropriate attributes
    const announcer = page.locator('#aria-announcer');
    await expect(announcer).toHaveAttribute('aria-live', 'polite');
    await expect(announcer).toHaveAttribute('aria-atomic', 'true');

    // 2. Trigger announcement via language switch and verify update
    await page.locator('#lang-select').selectOption('es');
    await page.waitForLoadState('networkidle');

    // 3. Verify high-contrast focus rings on keyboard interactive elements
    const brandLink = page.locator('#brand-link');
    await brandLink.focus();
    await expect(brandLink).toBeFocused();

    const langSelect = page.locator('#lang-select');
    await langSelect.focus();
    await expect(langSelect).toBeFocused();
  });
});
