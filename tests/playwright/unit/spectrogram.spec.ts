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
});
