import { test, expect } from '@playwright/test';

test.describe('Audio Spectrogram Component', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('should render spectrogram canvas with non-zero pixel buffer and support play/pause', async ({ page }) => {
    const sampleBins = [
      [0.2, 0.4, 0.6, 0.8],
      [0.3, 0.5, 0.7, 0.9],
      [0.1, 0.3, 0.5, 0.7],
      [0.4, 0.6, 0.8, 1.0],
    ];

    await page.setContent(`
      <div id="spectrogram-container" class="spectrogram-widget" data-audio="data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQAAAAA=" data-bins='${JSON.stringify(sampleBins)}'>
        <canvas id="test-spectrogram" class="spectrogram-canvas" width="320" height="120" role="img" aria-label="Acoustic spectrogram"></canvas>
        <button id="btn-audio-play" class="btn-spectrogram-play" aria-label="Play Audio">Play</button>
      </div>
      <script src="/static/js/audio-spectrogram.js"></script>
    `);

    // Verify canvas element exists and has ARIA attributes
    const canvas = page.locator('#test-spectrogram');
    await expect(canvas).toBeVisible();
    await expect(canvas).toHaveAttribute('role', 'img');
    await expect(canvas).toHaveAttribute('aria-label', /spectrogram/i);

    // Verify window.initAudioSpectrogram function is loaded from audio-spectrogram.js
    const hasInitFn = await page.evaluate(() => typeof (window as any).initAudioSpectrogram === 'function');
    expect(hasInitFn).toBe(true);

    // Verify canvas has non-zero pixel buffer rendered from spectrogram bins
    const hasPixels = await page.evaluate(() => {
      const cvs = document.getElementById('test-spectrogram') as HTMLCanvasElement;
      if (!cvs) return false;
      const ctx = cvs.getContext('2d');
      if (!ctx) return false;
      const imgData = ctx.getImageData(0, 0, cvs.width, cvs.height);
      for (let i = 0; i < imgData.data.length; i += 4) {
        if (imgData.data[i + 3] > 0 && (imgData.data[i] > 0 || imgData.data[i + 1] > 0 || imgData.data[i + 2] > 0)) {
          return true;
        }
      }
      return false;
    });
    expect(hasPixels).toBe(true);

    // Verify play/pause toggle interaction
    const playBtn = page.locator('#btn-audio-play');
    await expect(playBtn).toHaveText('Play');
    await playBtn.click();
    await expect(playBtn).toHaveText('Pause');
    await playBtn.click();
    await expect(playBtn).toHaveText('Play');
  });

  test('should support keyboard navigation controls on spectrogram container', async ({ page }) => {
    await page.setContent(`
      <div id="spectrogram-container" class="spectrogram-widget" data-audio="data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQAAAAA=">
        <canvas id="test-spectrogram" class="spectrogram-canvas" width="320" height="120" role="img" aria-label="Acoustic spectrogram"></canvas>
        <button id="btn-audio-play" class="btn-spectrogram-play" aria-label="Play Audio">Play</button>
      </div>
      <script src="/static/js/audio-spectrogram.js"></script>
    `);

    const container = page.locator('#spectrogram-container');
    const playBtn = page.locator('#btn-audio-play');

    await expect(container).toHaveAttribute('tabindex', '0');

    // Space key should toggle playback
    await container.focus();
    await page.keyboard.press('Space');
    await expect(playBtn).toHaveText('Pause');

    await page.keyboard.press('Space');
    await expect(playBtn).toHaveText('Play');
  });

  test('should render fallback default spectrogram when data-bins is not provided', async ({ page }) => {
    await page.setContent(`
      <div id="spectrogram-fallback" class="spectrogram-widget">
        <canvas id="fallback-canvas" class="spectrogram-canvas" width="320" height="120" role="img" aria-label="Acoustic spectrogram"></canvas>
      </div>
      <script src="/static/js/audio-spectrogram.js"></script>
    `);

    const hasPixels = await page.evaluate(() => {
      const cvs = document.getElementById('fallback-canvas') as HTMLCanvasElement;
      if (!cvs) return false;
      const ctx = cvs.getContext('2d');
      if (!ctx) return false;
      const imgData = ctx.getImageData(0, 0, cvs.width, cvs.height);
      for (let i = 0; i < imgData.data.length; i += 4) {
        if (imgData.data[i + 3] > 0 && (imgData.data[i] > 0 || imgData.data[i + 1] > 0 || imgData.data[i + 2] > 0)) {
          return true;
        }
      }
      return false;
    });
    expect(hasPixels).toBe(true);
  });

  test('should safely handle empty or invalid spectrogram bins without throwing', async ({ page }) => {
    await page.setContent(`
      <canvas id="empty-spectrogram" width="320" height="120"></canvas>
      <script src="/static/js/audio-spectrogram.js"></script>
    `);

    const result = await page.evaluate(() => {
      const cvs = document.getElementById('empty-spectrogram') as HTMLCanvasElement;
      const fn = (window as any).renderSpectrogram;
      if (typeof fn !== 'function') return false;
      try {
        fn(cvs, []);
        fn(cvs, [[]]);
        fn(null, null);
        fn(cvs, null);
        return true;
      } catch (e) {
        return false;
      }
    });

    expect(result).toBe(true);
  });

  test('should update audio instance, pause playback, reset button, and redraw when audio source is re-selected', async ({ page }) => {
    const binsA = [
      [0.2, 0.4],
      [0.6, 0.8],
    ];
    const binsB = [
      [0.9, 0.7],
      [0.5, 0.3],
    ];
    const audioDataA = 'data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQAAAAA=';
    const audioDataB = 'data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQFBQUFB';

    await page.setContent(`
      <div id="spectrogram-container" class="spectrogram-widget" data-audio="${audioDataA}" data-bins='${JSON.stringify(binsA)}'>
        <canvas id="test-spectrogram" class="spectrogram-canvas" width="320" height="120" role="img" aria-label="Acoustic spectrogram"></canvas>
        <button id="btn-audio-play" class="btn-spectrogram-play" aria-label="Play Audio">Play</button>
      </div>
      <script src="/static/js/audio-spectrogram.js"></script>
    `);

    const playBtn = page.locator('#btn-audio-play');
    // Start playback
    await playBtn.click();
    await expect(playBtn).toHaveText('Pause');

    // Simulate re-selecting a new audio file via initAudioSpectrogram
    await page.evaluate(({ binsB, audioDataB }) => {
      const container = document.getElementById('spectrogram-container');
      (window as any).initAudioSpectrogram(container, binsB, audioDataB);
    }, { binsB, audioDataB });

    // Verify play button reset to Play, audio src updated to audioDataB, and isPlaying reset to false
    await expect(playBtn).toHaveText('Play');
    const audioSrc = await page.evaluate(() => {
      const container = document.getElementById('spectrogram-container') as any;
      return {
        src: container._audioInstance?.src,
        isPlaying: container._isPlaying,
      };
    });
    expect(audioSrc.src).toBe(audioDataB);
    expect(audioSrc.isPlaying).toBe(false);
  });

  test('should include audioDataUri in sighting payload when audio is attached', async ({ page }) => {
    await page.setContent(`
      <main data-pet-id="demo-pet-1">
        <form id="form-report-sighting">
          <input type="hidden" id="sighting-pet-id" name="petId" value="demo-pet-1">
          <input type="hidden" id="sighting-lat" name="lat" value="47.6152">
          <input type="hidden" id="sighting-lng" name="lng" value="-122.3211">
          <input type="text" id="sighting-location" name="locationDescription" value="Near park">
          <input type="hidden" id="sighting-audio-data-uri" name="audioDataUri" value="data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQAAAAA=">
          <div id="modal-report-sighting"></div>
          <button type="submit" id="btn-submit-sighting">Submit Sighting</button>
        </form>
      </main>
      <script src="/static/js/sighting-trajectory.js"></script>
    `);

    await page.route('**/api/v1/lost-pets/*/sightings', async (route) => {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ sightingId: 'sighting-123', status: 'CONFIRMED' }),
      });
    });

    const requestPromise = page.waitForRequest((req) => req.url().includes('/sightings') && req.method() === 'POST');
    await page.locator('#btn-submit-sighting').click();
    const request = await requestPromise;
    const payload = request.postDataJSON();

    expect(payload).not.toBeNull();
    expect(payload.audioDataUri).toBe('data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQAAAAA=');
    expect(payload.locationDescription).toBe('Near park');
  });
});
