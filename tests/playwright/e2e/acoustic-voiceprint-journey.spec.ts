import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

function generateTestWAVBase64(freq: number, durationSec: number): string {
  const sampleRate = 16000;
  const numSamples = Math.floor(sampleRate * durationSec);
  const dataSize = numSamples * 2;
  const totalSize = 36 + dataSize;
  const buffer = new ArrayBuffer(44 + dataSize);
  const view = new DataView(buffer);

  function writeString(offset: number, str: string) {
    for (let i = 0; i < str.length; i++) {
      view.setUint8(offset + i, str.charCodeAt(i));
    }
  }

  writeString(0, 'RIFF');
  view.setUint32(4, totalSize, true);
  writeString(8, 'WAVE');
  writeString(12, 'fmt ');
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true); // PCM
  view.setUint16(22, 1, true); // 1 channel
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  writeString(36, 'data');
  view.setUint32(40, dataSize, true);

  for (let i = 0; i < numSamples; i++) {
    const t = i / sampleRate;
    const sample = Math.floor(0.7 * 32767.0 * Math.sin(2 * Math.PI * freq * t));
    view.setInt16(44 + i * 2, sample, true);
  }

  const bytes = new Uint8Array(buffer);
  let binary = '';
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return 'data:audio/wav;base64,' + btoa(binary);
}

test.describe.serial('Milestone 11.3: Acoustic Biometric Voiceprinting & Vocalization Journey', () => {
  const testPetID = `lost-acoustic-${Date.now()}`;
  const canineBarkWAV = generateTestWAVBase64(360.0, 0.4);
  const ambientNoiseWAV = generateTestWAVBase64(60.0, 2.0);

  test('Step 1: Reference Audio Ingestion & DSP Analysis via POST /api/v1/audio/analyze', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/audio/analyze`, {
      data: { audioDataUri: canineBarkWAV },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.audioId).toBeTruthy();
    expect(body.vocalization).toBe('CANINE_BARK');
    expect(body.voiceprint.vectorDimensions).toBe(32);
    expect(body.spectrogramBins.length).toBe(32);
  });

  test('Step 2: Attach Reference Audio to Lost Pet via POST /api/v1/lost-pets/{id}/audio-profile', async ({ request }) => {
    // Seed lost pet
    await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        id: testPetID,
        petName: 'Rusty',
        species: 'Dog',
        breed: 'Golden Retriever',
        email: 'owner@example.com',
      },
    });

    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${testPetID}/audio-profile`, {
      data: { audioDataUri: canineBarkWAV },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.petId).toBe(testPetID);
    expect(body.vocalization).toBe('CANINE_BARK');
  });

  test('Step 3 & 4: Submit Field Sighting with Audio & Assert Bilateral Acoustic Match', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/sightings`, {
      data: {
        petId: testPetID,
        latitude: 37.7750,
        longitude: -122.4190,
        notes: 'Heard bark near trailhead culvert',
        audioDataUri: canineBarkWAV,
      },
    });
    expect(res.status()).toBe(200);
    const sighting = await res.json();
    expect(sighting.acousticMatch).toBeTruthy();
    expect(sighting.acousticMatch.similarityScore).toBeGreaterThanOrEqual(0.75);
    expect(sighting.acousticMatch.isProbableMatch).toBe(true);
    expect(sighting.acousticMatch.vocalization).toBe('CANINE_BARK');
  });

  test('Step 5: Verify Sighting Timeline Acoustic Match Badge & Spectrogram Canvas in UI', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);
    await page.waitForLoadState('networkidle');

    // Locate acoustic badge
    const badge = page.locator('.acoustic-match-badge').first();
    await expect(badge).toBeVisible();
    await expect(badge).toContainText(/Acoustic Match/i);

    // Verify canvas rendered
    const canvas = page.locator('.spectrogram-canvas').first();
    await expect(canvas).toBeVisible();
  });

  test('Step 6: Negative Control Rejection on Ambient Noise', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/audio/analyze`, {
      data: { audioDataUri: ambientNoiseWAV },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.vocalization).toBe('AMBIENT_NOISE');
  });
});
