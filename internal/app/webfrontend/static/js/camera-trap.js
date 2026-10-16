/**
 * PetSpotR Smart Camera Trap & Edge Frame Command Center Controller
 * Milestone 12.1: Multi-vendor motion webhook ingestion, synthetic edge frame
 * generator, perceptual dHash deduplication, night-vision IR phosphor toggle,
 * ROI bounding-box overlay, and one-click sighting promotion.
 */

(function () {
  'use strict';

  const STORAGE_KEY_TOKENS = 'petspotr_camera_tokens';
  const cameraTokens = new Map();
  let lastTransmittedDataUri = '';
  let sseSource = null;
  let dynamicStyleSheet = null;

  /**
   * Escape untrusted string for safe HTML interpolation
   */
  function escapeHtml(str) {
    if (str === null || str === undefined) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  /**
   * Escape string for safe CSS attribute selector
   */
  function safeCssEscape(str) {
    if (typeof CSS !== 'undefined' && typeof CSS.escape === 'function') {
      return CSS.escape(String(str));
    }
    return String(str).replace(/["\\]/g, '\\$&');
  }

  /**
   * Load saved session tokens from sessionStorage
   */
  function loadStoredTokens() {
    try {
      const raw = sessionStorage.getItem(STORAGE_KEY_TOKENS);
      if (!raw) return;
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === 'object') {
        Object.keys(parsed).forEach((k) => {
          cameraTokens.set(k, parsed[k]);
        });
      }
    } catch (err) {
      // Ignore storage errors
    }
  }

  /**
   * Persist token map to sessionStorage
   */
  function saveStoredToken(cameraId, token) {
    if (!cameraId || !token) return;
    cameraTokens.set(cameraId, token);
    try {
      const obj = {};
      cameraTokens.forEach((val, key) => {
        obj[key] = val;
      });
      sessionStorage.setItem(STORAGE_KEY_TOKENS, JSON.stringify(obj));
    } catch (err) {
      // Ignore storage errors
    }
  }

  /**
   * Show toast notification
   */
  function showToast(message, type) {
    const container = document.getElementById('toast-container');
    if (!container) return;
    const toast = document.createElement('div');
    toast.className = `toast toast-${type || 'info'}`;
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(() => {
      toast.remove();
    }, 4000);
  }

  /**
   * Announce status message to screen readers
   */
  function announce(message) {
    const announcer = document.getElementById('camera-aria-announcer') || document.getElementById('aria-announcer');
    if (announcer) {
      announcer.textContent = message;
    }
  }

  /**
   * Apply ROI coordinates via same-origin CSSOM stylesheet rule so ROI placement
   * works even under strict style-src 'self' CSP.
   */
  function applyDynamicRoiStyle(captureId, leftPct, topPct, widthPct, heightPct) {
    try {
      if (!dynamicStyleSheet) {
        for (let i = 0; i < document.styleSheets.length; i++) {
          const sheet = document.styleSheets[i];
          if (sheet.href && sheet.href.indexOf('/static/css/styles.css') !== -1) {
            dynamicStyleSheet = sheet;
            break;
          }
        }
      }
      if (dynamicStyleSheet && typeof dynamicStyleSheet.insertRule === 'function') {
        const rule = `.camera-capture-card[data-capture-id="${safeCssEscape(captureId)}"] .roi-bounding-box { left: ${leftPct}%; top: ${topPct}%; width: ${widthPct}%; height: ${heightPct}%; }`;
        dynamicStyleSheet.insertRule(rule, dynamicStyleSheet.cssRules.length);
      }
    } catch (err) {
      // Fallback handled by CSS defaults
    }
  }

  /**
   * Generate a deterministic 160x120 synthetic edge frame PNG data URI.
   * @param {string} preset - 'night_ir_dog' | 'daylight_cat' | 'static_repeat'
   * @returns {string} data:image/png;base64,...
   */
  function generateSyntheticEdgeFrame(preset) {
    if (preset === 'static_repeat' && lastTransmittedDataUri) {
      return lastTransmittedDataUri;
    }

    const canvas = document.createElement('canvas');
    canvas.width = 160;
    canvas.height = 120;
    const ctx = canvas.getContext('2d');
    if (!ctx) {
      return '';
    }

    if (preset === 'daylight_cat') {
      // Colorful yard background (high chrominance & luminance -> DetectNightVisionIR = false)
      ctx.fillStyle = '#22c55e';
      ctx.fillRect(0, 0, 160, 120);

      // Subtle fence line
      ctx.fillStyle = '#16a34a';
      ctx.fillRect(0, 0, 160, 24);

      // High-contrast calico cat subject in center-left quadrant (x: 24..72, y: 36..84)
      ctx.fillStyle = '#ffffff';
      ctx.fillRect(24, 36, 48, 48);
      ctx.fillStyle = '#ea580c';
      ctx.fillRect(28, 40, 20, 20);
      ctx.fillStyle = '#1e1b4b';
      ctx.fillRect(48, 52, 20, 26);
      ctx.fillStyle = '#ffffff';
      ctx.fillRect(34, 62, 14, 18);
    } else if (preset === 'static_repeat') {
      // Uniform static monochromatic background with fixed center marker
      ctx.fillStyle = '#181c1a';
      ctx.fillRect(0, 0, 160, 120);
      ctx.fillStyle = '#cbd5e1';
      ctx.fillRect(60, 45, 40, 30);
    } else {
      // Default: 'night_ir_dog'
      // Dark monochromatic IR background (#121614 -> DetectNightVisionIR = true)
      ctx.fillStyle = '#121614';
      ctx.fillRect(0, 0, 160, 120);

      // Bright high-contrast animal silhouette/patch in lower-right quadrant (x: 85..135, y: 55..100)
      ctx.fillStyle = '#e2e8f0';
      ctx.fillRect(85, 55, 50, 45);

      // Internal contrast details (reflective collar & body contour)
      ctx.fillStyle = '#ffffff';
      ctx.fillRect(90, 58, 18, 16);
      ctx.fillStyle = '#1e293b';
      ctx.fillRect(108, 62, 10, 14);
      ctx.fillStyle = '#f8fafc';
      ctx.fillRect(92, 76, 38, 20);
    }

    const dataUri = canvas.toDataURL('image/png');
    lastTransmittedDataUri = dataUri;
    return dataUri;
  }

  /**
   * Map verification status to WCAG AAA badge class
   */
  function getStatusBadgeClass(status) {
    switch (status) {
      case 'PROMOTED':
        return 'badge-capture-promoted';
      case 'DUPLICATE_SUPPRESSED':
        return 'badge-capture-duplicate';
      case 'DISMISSED':
        return 'badge-capture-dismissed';
      case 'PENDING_REVIEW':
      default:
        return 'badge-capture-pending';
    }
  }

  /**
   * Render camera nodes list and update simulator select dropdown
   */
  function renderCameraNodes(cameras, preferredCameraId) {
    const listEl = document.getElementById('camera-nodes-list');
    const selectEl = document.getElementById('sim-camera-select');
    const tokenInput = document.getElementById('sim-token-input');

    if (listEl) {
      if (!Array.isArray(cameras) || cameras.length === 0) {
        listEl.innerHTML = '<p class="camera-empty-hint" id="camera-nodes-empty">No camera nodes registered yet. Register a perimeter node above to begin receiving motion frames.</p>';
      } else {
        listEl.innerHTML = cameras
          .map((cam) => {
            const safeId = escapeHtml(cam.id);
            const safeName = escapeHtml(cam.name);
            const safeVendor = escapeHtml(cam.vendor);
            const safePet = cam.targetPetId ? `<span class="camera-node-pet">🎯 ${escapeHtml(cam.targetPetId)}</span>` : '';
            const lat = Number(cam.latitude || 0).toFixed(4);
            const lng = Number(cam.longitude || 0).toFixed(4);
            const totalCaps = Number(cam.totalCaptures || 0);
            const dupCaps = Number(cam.suppressedDuplicates || 0);

            return `
              <article class="camera-node-card" data-camera-id="${safeId}" role="listitem">
                <div class="camera-node-header">
                  <strong class="camera-node-name">${safeName}</strong>
                  <span class="badge camera-vendor-badge">${safeVendor}</span>
                </div>
                <div class="camera-node-meta">
                  <span>📍 ${lat}, ${lng}</span>
                  ${safePet}
                  <span>📸 Captures: <strong>${totalCaps}</strong></span>
                  <span>🛡️ Suppressed Duplicates: <strong>${dupCaps}</strong></span>
                </div>
              </article>
            `;
          })
          .join('');
      }
    }

    if (selectEl && Array.isArray(cameras)) {
      const currentVal = preferredCameraId || selectEl.value;
      if (cameras.length === 0) {
        selectEl.innerHTML = '<option value="">-- Register a camera node first --</option>';
      } else {
        selectEl.innerHTML = cameras
          .map((cam) => {
            const safeId = escapeHtml(cam.id);
            const safeName = escapeHtml(cam.name);
            const safeVendor = escapeHtml(cam.vendor);
            return `<option value="${safeId}">${safeName} (${safeVendor})</option>`;
          })
          .join('');

        const targetId = cameras.some((c) => c.id === currentVal) ? currentVal : cameras[0].id;
        selectEl.value = targetId;

        if (tokenInput && cameraTokens.has(targetId)) {
          tokenInput.value = cameraTokens.get(targetId);
        }
      }
    }
  }

  /**
   * Fetch registered camera nodes from GET /api/v1/cameras
   */
  async function loadCameraNodes(preferredCameraId) {
    try {
      const res = await fetch('/api/v1/cameras');
      if (!res.ok) return;
      const data = await res.json();
      renderCameraNodes(data.cameras || [], preferredCameraId);
    } catch (err) {
      console.warn('Failed to load camera nodes:', err);
    }
  }

  /**
   * Render edge capture cards in #camera-captures-grid
   */
  function renderCameraCaptures(captures) {
    const gridEl = document.getElementById('camera-captures-grid');
    const countBadge = document.getElementById('captures-count-badge');

    if (countBadge) {
      const count = Array.isArray(captures) ? captures.length : 0;
      countBadge.textContent = `${count} Capture${count === 1 ? '' : 's'}`;
    }

    if (!gridEl) return;

    if (!Array.isArray(captures) || captures.length === 0) {
      gridEl.innerHTML = '<p class="camera-empty-hint" id="camera-captures-empty">No edge motion captures received yet. Use the simulator above or send a webhook POST to ingest frames.</p>';
      return;
    }

    gridEl.innerHTML = captures
      .map((cap) => {
        const safeId = escapeHtml(cap.id);
        const status = cap.verificationStatus || 'PENDING_REVIEW';
        const safeStatus = escapeHtml(status);
        const badgeClass = getStatusBadgeClass(status);
        const safeCamName = escapeHtml(cap.cameraName || cap.cameraId || 'Camera Trap');
        const safeVendor = escapeHtml(cap.vendor || 'RING');
        const safeDHash = escapeHtml(cap.frameDHashHex || '0x0000000000000000');
        const hamming = Number(cap.hammingDistance || 0);
        const confPct = Math.round(Number(cap.confidenceScore || 0) * 100);
        const safeSpecies = escapeHtml(cap.detectedSpecies || 'UNKNOWN');
        const safeSnapshot = escapeHtml(cap.snapshotDataUri || cap.cropThumbnailUri || '');

        const roi = cap.roi || { x: 0.25, y: 0.25, width: 0.5, height: 0.5 };
        const leftPct = Math.max(0, Math.min(100, Number((roi.x || 0) * 100))).toFixed(1);
        const topPct = Math.max(0, Math.min(100, Number((roi.y || 0) * 100))).toFixed(1);
        const widthPct = Math.max(10, Math.min(100, Number((roi.width || 0.5) * 100))).toFixed(1);
        const heightPct = Math.max(10, Math.min(100, Number((roi.height || 0.5) * 100))).toFixed(1);

        applyDynamicRoiStyle(cap.id, leftPct, topPct, widthPct, heightPct);

        const irBadge =
          cap.isNightVisionIr || cap.irEnhanced
            ? '<span class="badge capture-ir-badge">🌙 IR Enhanced</span>'
            : '';

        const traits = Array.isArray(cap.extractedTraits) ? cap.extractedTraits : [];
        const traitsHtml =
          traits.length > 0
            ? `<ul class="capture-traits">${traits.map((t) => `<li>${escapeHtml(t)}</li>`).join('')}</ul>`
            : '<ul class="capture-traits"><li>Motion anomaly detected</li></ul>';

        let actionsHtml = '';
        if (status === 'PENDING_REVIEW') {
          actionsHtml = `
            <div class="capture-actions">
              <button type="button" class="btn btn-primary btn-promote-capture" data-capture-id="${safeId}">Promote to Sighting</button>
              <button type="button" class="btn btn-secondary btn-dismiss-capture" data-capture-id="${safeId}">Dismiss</button>
            </div>
          `;
        } else if (status === 'PROMOTED') {
          const safeSightingId = escapeHtml(cap.promotedSightingId || 'sighted');
          actionsHtml = `
            <div class="promoted-sighting-badge">Sighting ID: ${safeSightingId}</div>
          `;
        }

        return `
          <article class="camera-capture-card" data-capture-id="${safeId}" data-verification-status="${safeStatus}">
            <div class="capture-card-header">
              <div>
                <strong class="capture-camera-title">${safeCamName}</strong>
                <span class="badge camera-vendor-badge">${safeVendor}</span>
              </div>
              <span class="badge badge-capture-status ${badgeClass}">${safeStatus}</span>
            </div>

            <div class="capture-viewport">
              <img class="capture-snapshot-img" src="${safeSnapshot}" alt="Edge capture from ${safeCamName}">
              <div class="roi-bounding-box" data-roi-left="${leftPct}" data-roi-top="${topPct}" data-roi-width="${widthPct}" data-roi-height="${heightPct}" title="Detected Motion ROI"></div>
            </div>

            <div class="capture-metadata-badges">
              <span class="badge capture-dhash-badge">dHash: ${safeDHash} | Hamming: ${hamming}</span>
              <span class="badge capture-confidence-badge">${confPct}% Confidence • ${safeSpecies}</span>
              ${irBadge}
            </div>

            ${traitsHtml}
            ${actionsHtml}
          </article>
        `;
      })
      .join('');

    gridEl.querySelectorAll('.roi-bounding-box').forEach((roiEl) => {
      const l = roiEl.getAttribute('data-roi-left');
      const t = roiEl.getAttribute('data-roi-top');
      const w = roiEl.getAttribute('data-roi-width');
      const h = roiEl.getAttribute('data-roi-height');
      if (l !== null && t !== null && w !== null && h !== null) {
        roiEl.style.left = `${l}%`;
        roiEl.style.top = `${t}%`;
        roiEl.style.width = `${w}%`;
        roiEl.style.height = `${h}%`;
      }
    });
  }

  /**
   * Fetch edge captures from GET /api/v1/cameras/captures
   */
  async function loadCameraCaptures() {
    try {
      const excludeDupEl = document.getElementById('filter-exclude-duplicates');
      const excludeDuplicates = Boolean(excludeDupEl && excludeDupEl.checked);
      const url = excludeDuplicates
        ? '/api/v1/cameras/captures?excludeDuplicates=true'
        : '/api/v1/cameras/captures';

      const res = await fetch(url);
      if (!res.ok) return;
      const data = await res.json();
      renderCameraCaptures(data.captures || []);
    } catch (err) {
      console.warn('Failed to load camera captures:', err);
    }
  }

  /**
   * Handle #camera-register-form submission
   */
  async function handleRegisterCamera(e) {
    e.preventDefault();

    const nameEl = document.getElementById('camera-name');
    const vendorEl = document.getElementById('camera-vendor');
    const targetPetEl = document.getElementById('camera-target-pet');
    const latEl = document.getElementById('camera-lat');
    const lngEl = document.getElementById('camera-lng');
    const headingEl = document.getElementById('camera-heading');
    const fovEl = document.getElementById('camera-fov');
    const autoPromoteEl = document.getElementById('camera-auto-promote');
    const nightVisionEl = document.getElementById('camera-night-vision');

    const name = nameEl ? nameEl.value.trim() : '';
    if (!name) {
      showToast('Camera node name is required', 'error');
      if (nameEl) nameEl.focus();
      return;
    }

    const payload = {
      name,
      vendor: vendorEl ? vendorEl.value : 'RING',
      targetPetId: targetPetEl ? targetPetEl.value.trim() : '',
      latitude: latEl && latEl.value !== '' ? parseFloat(latEl.value) : 47.6062,
      longitude: lngEl && lngEl.value !== '' ? parseFloat(lngEl.value) : -122.3321,
      headingDegrees: headingEl && headingEl.value !== '' ? parseFloat(headingEl.value) : 45,
      fovDegrees: fovEl && fovEl.value !== '' ? parseFloat(fovEl.value) : 110,
      autoPromoteThreshold: autoPromoteEl && autoPromoteEl.value !== '' ? parseFloat(autoPromoteEl.value) : 0,
      nightVisionIr: Boolean(nightVisionEl ? nightVisionEl.checked : true),
    };

    try {
      const res = await fetch('/api/v1/cameras', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        showToast(errData.error || 'Failed to register camera node', 'error');
        return;
      }

      const data = await res.json();
      const camera = data.camera || {};
      const token = data.webhookToken || '';
      const webhookUrl = data.webhookUrl || `/api/v1/cameras/${camera.id}/webhook`;

      if (camera.id && token) {
        saveStoredToken(camera.id, token);
      }

      const credsBanner = document.getElementById('camera-webhook-credentials');
      const urlCode = document.getElementById('issued-webhook-url');
      const tokenCode = document.getElementById('issued-webhook-token');

      if (urlCode) urlCode.textContent = webhookUrl;
      if (tokenCode) tokenCode.textContent = token;
      if (credsBanner) credsBanner.hidden = false;

      await loadCameraNodes(camera.id);

      const simSelect = document.getElementById('sim-camera-select');
      const simToken = document.getElementById('sim-token-input');
      if (simSelect && camera.id) {
        simSelect.value = camera.id;
      }
      if (simToken && token) {
        simToken.value = token;
      }

      showToast(`Registered camera node: ${camera.name}`, 'success');
      announce(`Camera node ${camera.name} registered`);
    } catch (err) {
      console.error('Error registering camera:', err);
      showToast('Network error registering camera node', 'error');
    }
  }

  /**
   * Handle #webhook-simulator-form submission
   */
  async function handleSimulateWebhook(e) {
    e.preventDefault();

    const simSelect = document.getElementById('sim-camera-select');
    const simToken = document.getElementById('sim-token-input');
    const presetSelect = document.getElementById('sim-scene-preset');
    const speciesSelect = document.getElementById('sim-species-hint');
    const traitsInput = document.getElementById('sim-traits-input');

    const cameraId = simSelect ? simSelect.value.trim() : '';
    if (!cameraId) {
      showToast('Please register and select a camera node first', 'warning');
      return;
    }

    const token = (simToken && simToken.value.trim()) || cameraTokens.get(cameraId) || '';
    const preset = presetSelect ? presetSelect.value : 'night_ir_dog';
    const detectedSpecies = speciesSelect ? speciesSelect.value : 'DOG';
    const rawTraits = traitsInput ? traitsInput.value : '';
    const extractedTraits = rawTraits
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);

    const snapshotDataUri = generateSyntheticEdgeFrame(preset);

    try {
      const res = await fetch(`/api/v1/cameras/${encodeURIComponent(cameraId)}/webhook`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Camera-Token': token,
        },
        body: JSON.stringify({
          snapshotDataUri,
          detectedSpecies,
          extractedTraits,
          motionZone: preset,
        }),
      });

      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        showToast(errData.error || `Webhook rejected (${res.status})`, 'error');
        return;
      }

      const capture = await res.json();
      await Promise.all([loadCameraNodes(cameraId), loadCameraCaptures()]);
      showToast(`Edge frame processed (${capture.verificationStatus})`, 'success');
      announce(`Edge frame received with status ${capture.verificationStatus}`);
    } catch (err) {
      console.error('Error sending simulated webhook frame:', err);
      showToast('Failed to transmit edge frame', 'error');
    }
  }

  /**
   * Handle promote or dismiss button clicks inside #camera-captures-grid
   */
  async function handleCapturesGridClick(e) {
    const promoteBtn = e.target.closest('.btn-promote-capture');
    if (promoteBtn) {
      const captureId = promoteBtn.getAttribute('data-capture-id');
      if (!captureId) return;
      promoteBtn.disabled = true;
      try {
        const res = await fetch(`/api/v1/cameras/captures/${encodeURIComponent(captureId)}/promote`, {
          method: 'POST',
        });
        if (res.ok) {
          const data = await res.json();
          await Promise.all([loadCameraCaptures(), loadCameraNodes()]);
          showToast(`Promoted capture to Sighting ${data.sighting?.id || ''}`, 'success');
          announce('Capture promoted to official sighting');
        } else {
          promoteBtn.disabled = false;
          showToast('Failed to promote capture', 'error');
        }
      } catch (err) {
        promoteBtn.disabled = false;
        showToast('Network error promoting capture', 'error');
      }
      return;
    }

    const dismissBtn = e.target.closest('.btn-dismiss-capture');
    if (dismissBtn) {
      const captureId = dismissBtn.getAttribute('data-capture-id');
      if (!captureId) return;
      dismissBtn.disabled = true;
      try {
        const res = await fetch(`/api/v1/cameras/captures/${encodeURIComponent(captureId)}/dismiss`, {
          method: 'POST',
        });
        if (res.ok) {
          await loadCameraCaptures();
          showToast('Capture dismissed', 'info');
          announce('Capture dismissed');
        } else {
          dismissBtn.disabled = false;
          showToast('Failed to dismiss capture', 'error');
        }
      } catch (err) {
        dismissBtn.disabled = false;
        showToast('Network error dismissing capture', 'error');
      }
    }
  }

  /**
   * Setup SSE connection to /api/v1/cameras/stream
   */
  function setupCameraStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/cameras/stream');

      const refreshAll = () => {
        loadCameraCaptures();
        loadCameraNodes();
      };

      sseSource.addEventListener('camera_capture_received', refreshAll);
      sseSource.addEventListener('camera_capture_promoted', refreshAll);
      sseSource.addEventListener('message', (e) => {
        try {
          const parsed = JSON.parse(e.data);
          if (
            parsed &&
            (parsed.type === 'camera_capture_received' || parsed.type === 'camera_capture_promoted')
          ) {
            refreshAll();
          }
        } catch (err) {
          // Ignore heartbeat comments
        }
      });
    } catch (err) {
      console.warn('Could not open camera trap SSE stream:', err);
    }
  }

  /**
   * Initialize Camera Trap Command Center if present on the current page
   */
  function init() {
    const commandCenter = document.getElementById('camera-command-center');
    if (!commandCenter) return;

    loadStoredTokens();

    const registerForm = document.getElementById('camera-register-form');
    if (registerForm) {
      registerForm.addEventListener('submit', handleRegisterCamera);
    }

    const simForm = document.getElementById('webhook-simulator-form');
    if (simForm) {
      simForm.addEventListener('submit', handleSimulateWebhook);
    }

    const simSelect = document.getElementById('sim-camera-select');
    const simToken = document.getElementById('sim-token-input');
    if (simSelect && simToken) {
      simSelect.addEventListener('change', () => {
        const camId = simSelect.value;
        if (cameraTokens.has(camId)) {
          simToken.value = cameraTokens.get(camId);
        }
      });
    }

    const capturesGrid = document.getElementById('camera-captures-grid');
    if (capturesGrid) {
      capturesGrid.addEventListener('click', handleCapturesGridClick);
    }

    const togglePhosphorBtn = document.getElementById('toggle-ir-phosphor');
    if (togglePhosphorBtn) {
      togglePhosphorBtn.addEventListener('click', () => {
        const isActive = commandCenter.classList.toggle('ir-phosphor-mode');
        togglePhosphorBtn.setAttribute('aria-pressed', isActive ? 'true' : 'false');
        togglePhosphorBtn.textContent = isActive
          ? 'Night-Vision IR Phosphor: ON'
          : 'Night-Vision IR Phosphor: OFF';
      });
    }

    const filterExcludeDup = document.getElementById('filter-exclude-duplicates');
    if (filterExcludeDup) {
      filterExcludeDup.addEventListener('change', () => {
        loadCameraCaptures();
      });
    }

    loadCameraNodes();
    loadCameraCaptures();
    setupCameraStream();
  }

  // Expose helper for programmatic testing
  window.generateSyntheticEdgeFrame = generateSyntheticEdgeFrame;

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
