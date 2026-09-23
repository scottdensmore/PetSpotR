(function () {
  'use strict';

  function renderSpectrogram(canvas, bins) {
    if (!canvas || !bins || bins.length === 0) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const width = canvas.width;
    const height = canvas.height;
    const rows = bins.length;
    const cols = bins[0].length;
    const cellW = width / rows;
    const cellH = height / cols;

    ctx.clearRect(0, 0, width, height);

    for (let r = 0; r < rows; r++) {
      for (let c = 0; c < cols; c++) {
        const val = bins[r][c]; // 0.0 to 1.0
        // Viridis/Plasma inspired accessible colormap
        const red = Math.floor(val * 240);
        const green = Math.floor((1 - Math.abs(val - 0.5) * 2) * 220);
        const blue = Math.floor((1 - val) * 255);

        ctx.fillStyle = `rgb(${red}, ${green}, ${blue})`;
        // Flip Y so high frequencies are on top
        ctx.fillRect(r * cellW, height - (c + 1) * cellH, cellW + 1, cellH + 1);
      }
    }
  }

  function renderDefaultSpectrogram(canvas) {
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    const width = canvas.width;
    const height = canvas.height;
    ctx.clearRect(0, 0, width, height);
    for (let r = 0; r < 32; r++) {
      for (let c = 0; c < 32; c++) {
        const val = 0.5 + 0.4 * Math.sin((r / 32) * Math.PI * 2) * Math.cos((c / 32) * Math.PI);
        const red = Math.floor(val * 240);
        const green = Math.floor((1 - Math.abs(val - 0.5) * 2) * 220);
        const blue = Math.floor((1 - val) * 255);
        ctx.fillStyle = `rgb(${red}, ${green}, ${blue})`;
        ctx.fillRect(r * (width / 32), height - (c + 1) * (height / 32), width / 32 + 1, height / 32 + 1);
      }
    }
  }

  function initSpectrogramViewer(container, explicitBins, explicitAudio) {
    if (!container) return;
    const canvas = container.querySelector('.spectrogram-canvas');
    const playBtn = container.querySelector('.btn-spectrogram-play');
    const audioData = explicitAudio || container.getAttribute('data-audio');
    const binsJson = explicitBins || container.getAttribute('data-bins');
    const audioId = container.getAttribute('data-audio-id');

    let bins = null;
    if (Array.isArray(explicitBins)) {
      bins = explicitBins;
    } else if (binsJson) {
      try {
        bins = typeof binsJson === 'string' ? JSON.parse(binsJson) : binsJson;
      } catch (e) {
        // Fallback default gradient
      }
    }

    if (canvas) {
      if (bins && bins.length > 0) {
        renderSpectrogram(canvas, bins);
      } else if (audioId) {
        fetch(`/api/v1/audio/${encodeURIComponent(audioId)}/spectrogram`)
          .then((r) => r.json())
          .then((fetchedBins) => {
            if (fetchedBins && Array.isArray(fetchedBins)) {
              bins = fetchedBins;
              renderSpectrogram(canvas, fetchedBins);
            } else {
              renderDefaultSpectrogram(canvas);
            }
          })
          .catch(() => renderDefaultSpectrogram(canvas));
      } else {
        renderDefaultSpectrogram(canvas);
      }
    }


    if (playBtn && audioData && !container._listenerAttached) {
      container._listenerAttached = true;
      if (container._audioInstance) {
        try {
          container._audioInstance.pause();
        } catch (e) {}
      }

      const audio = new Audio(audioData);
      container._audioInstance = audio;
      let animId = null;
      let isPlaying = false;

      function drawPlayhead() {
        if (isPlaying && !audio.ended && canvas) {
          if (bins) {
            renderSpectrogram(canvas, bins);
          }
          const ctx = canvas.getContext('2d');
          if (ctx) {
            const duration = audio.duration || 1;
            const progress = (audio.currentTime || 0) / duration;
            const x = Math.min(canvas.width, Math.max(0, progress * canvas.width));
            ctx.strokeStyle = '#38bdf8'; // High contrast cyan scrubber
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.moveTo(x, 0);
            ctx.lineTo(x, canvas.height);
            ctx.stroke();
          }
          animId = requestAnimationFrame(drawPlayhead);
        }
      }

      playBtn.addEventListener('click', () => {
        if (!isPlaying) {
          isPlaying = true;
          audio.play().catch(() => {});
          playBtn.textContent = 'Pause';
          playBtn.setAttribute('aria-label', 'Pause audio');
          animId = requestAnimationFrame(drawPlayhead);
        } else {
          isPlaying = false;
          audio.pause();
          playBtn.textContent = 'Play';
          playBtn.setAttribute('aria-label', 'Play audio');
          if (animId) cancelAnimationFrame(animId);
          if (canvas && bins) renderSpectrogram(canvas, bins);
        }
      });

      audio.addEventListener('ended', () => {
        isPlaying = false;
        playBtn.textContent = 'Play';
        playBtn.setAttribute('aria-label', 'Play audio');
        if (animId) cancelAnimationFrame(animId);
        if (canvas && bins) renderSpectrogram(canvas, bins);
      });


      // Keyboard accessibility (WCAG AAA)
      container.setAttribute('tabindex', '0');
      container.addEventListener('keydown', (e) => {
        if (e.key === ' ' || e.key === 'Spacebar' || e.code === 'Space' || e.key === 'Enter') {
          e.preventDefault();
          playBtn.click();
        } else if (e.key === 'ArrowLeft') {
          e.preventDefault();
          audio.currentTime = Math.max(0, audio.currentTime - 0.5);
        } else if (e.key === 'ArrowRight') {
          e.preventDefault();
          audio.currentTime = Math.min(audio.duration || 0, audio.currentTime + 0.5);
        } else if (e.key === 'Home') {
          e.preventDefault();
          audio.currentTime = 0;
        } else if (e.key === 'End') {
          e.preventDefault();
          audio.currentTime = audio.duration || 0;
        }
      });
    }
  }

  function setupAudioFileInput(fileInputId, containerId, badgeId, uriInputId) {
    const input = document.getElementById(fileInputId);
    const container = document.getElementById(containerId);
    const badge = badgeId ? document.getElementById(badgeId) : null;
    const uriInput = uriInputId ? document.getElementById(uriInputId) : null;

    if (!input || !container) return;

    input.addEventListener('change', async (e) => {
      const file = e.target.files && e.target.files[0];
      if (!file) return;

      const reader = new FileReader();
      reader.onload = async () => {
        const dataUri = reader.result;
        if (uriInput) uriInput.value = dataUri;
        container.setAttribute('data-audio', dataUri);
        container.classList.remove('hidden');

        try {
          const res = await fetch('/api/v1/audio/analyze', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ audioDataUri: dataUri }),
          });
          if (res.ok) {
            const profile = await res.json();
            if (profile.spectrogramBins) {
              container.setAttribute('data-bins', JSON.stringify(profile.spectrogramBins));
              initSpectrogramViewer(container, profile.spectrogramBins, dataUri);
            }
            if (badge && profile.vocalization) {
              badge.classList.remove('hidden');
              const confPct = Math.round((profile.confidenceScore || 0) * 100);
              badge.textContent = `🔊 ${profile.vocalization.replace(/_/g, ' ')} (${confPct}%)`;
              if (profile.vocalization === 'ANIMAL_DISTRESS') {
                badge.classList.add('acoustic-badge-distress');
              } else {
                badge.classList.remove('acoustic-badge-distress');
              }
            }
          }
        } catch (err) {
          console.warn('Spectrogram audio analysis error:', err);
        }
      };
      reader.readAsDataURL(file);
    });
  }

  window.initAudioSpectrogram = initSpectrogramViewer;
  window.renderSpectrogram = renderSpectrogram;
  window.setupAudioFileInput = setupAudioFileInput;

  function initAll() {
    document.querySelectorAll('.spectrogram-widget').forEach((el) => initSpectrogramViewer(el));
    setupAudioFileInput('lost-audio-file', 'lost-spectrogram-container', 'lost-vocalization-badge', 'lost-audio-data-uri');
    setupAudioFileInput('sighting-audio-input', 'sighting-spectrogram-container', 'sighting-acoustic-badge', 'sighting-audio-data-uri');
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initAll);
  } else {
    initAll();
  }
})();
