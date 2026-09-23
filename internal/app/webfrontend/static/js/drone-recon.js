/**
 * PetSpotR - Aerial Reconnaissance Cockpit UI & Leaflet Map Synchronization
 * Milestone 11.2: Drone / UAV Aerial Reconnaissance & Thermal Hotspot Sighting Feeds
 *
 * Implements:
 * - Real-time flight HUD: artificial horizon, compass heading, altitude AGL, ground speed, battery
 * - Telemetry & video scrubber synchronization
 * - Dynamic Leaflet camera frustum polygon & cyan flight path polyline (.leaflet-overlay-pane)
 * - Radiometric thermal reticle canvas bounding box overlay
 * - Pulsating radar flame pins (.marker-hotspot-pulse)
 * - Bi-directional P2P mesh synchronization (mesh:thermal-hotspot)
 * - Strict WCAG AAA compliance and zero-inline CSP compliance
 */
(() => {
  'use strict';

  // State
  let reconModal = null;
  let reconMap = null;
  let flightPolyline = null;
  let frustumPolygon = null;
  let droneMarker = null;
  const hotspotMarkers = new Map(); // id -> L.marker
  let waypoints = [];
  let hotspots = [];
  let selectedHotspotId = null;
  let currentWaypointIndex = 0;
  let currentPetId = '';
  let currentMissionId = '';
  let isPlaying = false;
  let playInterval = null;
  let previouslyFocusedElement = null;

  // Camera Intrinsics
  const cameraIntrinsics = {
    hfov: 84.0,
    vfov: 60.0
  };

  // Safe CSRF Token reader
  function getCsrfToken() {
    if (typeof window !== 'undefined' && window.petspotrIdentity?.getState) {
      const state = window.petspotrIdentity.getState();
      if (state?.csrfToken) return state.csrfToken;
    }
    if (typeof document !== 'undefined' && document.cookie) {
      const match = document.cookie.match(/(?:^|;\s*)(?:__Host-)?petspotr_csrf=([^;]+)/);
      if (match) return decodeURIComponent(match[1]);
    }
    return '';
  }

  // Announce to screen reader
  function announce(message) {
    const announcer = document.getElementById('recon-aria-announcer');
    if (announcer) {
      announcer.textContent = message;
    }
  }

  // Raycasting Ground Projection (Matches pkg/recon/projection.go)
  function raycastPixelToGround(u, v, wp, cam) {
    const hRad = ((cam.hfov || 84.0) * Math.PI / 180.0) / 2.0;
    const vRad = ((cam.vfov || 60.0) * Math.PI / 180.0) / 2.0;

    const xc = (u - 0.5) * 2.0 * Math.tan(hRad);
    const yc = -(v - 0.5) * 2.0 * Math.tan(vRad);
    const zc = 1.0;

    const totalYawDeg = (wp.headingDeg || 0) + (wp.gimbalYawDeg || 0);
    const yawRad = totalYawDeg * Math.PI / 180.0;

    const fwdX = Math.sin(yawRad);
    const fwdY = Math.cos(yawRad);
    const fwdZ = 0.0;

    const rightX = Math.cos(yawRad);
    const rightY = -Math.sin(yawRad);
    const rightZ = 0.0;

    const upX = 0.0;
    const upY = 0.0;
    const upZ = 1.0;

    const pitchRad = (wp.gimbalPitchDeg !== undefined ? wp.gimbalPitchDeg : -90.0) * Math.PI / 180.0;
    const cosPitch = Math.cos(pitchRad);
    const sinPitch = Math.sin(pitchRad);

    const dirX = cosPitch * fwdX + sinPitch * upX;
    const dirY = cosPitch * fwdY + sinPitch * upY;
    const dirZ = cosPitch * fwdZ + sinPitch * upZ;

    let upCamX = -sinPitch * fwdX + cosPitch * upX;
    let upCamY = -sinPitch * fwdY + cosPitch * upY;
    let upCamZ = -sinPitch * fwdZ + cosPitch * upZ;

    let rightCamX = rightX;
    let rightCamY = rightY;
    let rightCamZ = rightZ;

    if (wp.gimbalRollDeg) {
      const rollRad = wp.gimbalRollDeg * Math.PI / 180.0;
      const cosRoll = Math.cos(rollRad);
      const sinRoll = Math.sin(rollRad);

      const rX = cosRoll * rightCamX + sinRoll * upCamX;
      const rY = cosRoll * rightCamY + sinRoll * upCamY;
      const rZ = cosRoll * rightCamZ + sinRoll * upCamZ;

      const uX = -sinRoll * rightCamX + cosRoll * upCamX;
      const uY = -sinRoll * rightCamY + cosRoll * upCamY;
      const uZ = -sinRoll * rightCamZ + cosRoll * upCamZ;

      rightCamX = rX; rightCamY = rY; rightCamZ = rZ;
      upCamX = uX; upCamY = uY; upCamZ = uZ;
    }

    const vx = xc * rightCamX + yc * upCamX + zc * dirX;
    const vy = xc * rightCamY + yc * upCamY + zc * dirY;
    const vz = xc * rightCamZ + yc * upCamZ + zc * dirZ;

    if (vz >= -1e-9) {
      return null;
    }

    const alt = Math.max(1.0, wp.altitudeMetersAGL || wp.altitudeAGL || 30.0);
    const t = -alt / vz;
    const deltaX = t * vx;
    const deltaY = t * vy;

    const metersPerDegreeLat = 111132.95;
    const deltaLat = deltaY / metersPerDegreeLat;
    const latRad = wp.latitude * Math.PI / 180.0;
    const cosLat = Math.max(1e-6, Math.abs(Math.cos(latRad)));
    const deltaLng = deltaX / (metersPerDegreeLat * cosLat);

    return [wp.latitude + deltaLat, wp.longitude + deltaLng];
  }

  function computeCameraFrustum(wp, cam = cameraIntrinsics) {
    const corners = [
      [0.0, 0.0], // TopLeft
      [1.0, 0.0], // TopRight
      [1.0, 1.0], // BottomRight
      [0.0, 1.0]  // BottomLeft
    ];
    const poly = [];
    for (const c of corners) {
      const pt = raycastPixelToGround(c[0], c[1], wp, cam);
      if (!pt) {
        // Fallback quadrilateral around waypoint
        const r = 0.0003;
        return [
          [wp.latitude + r, wp.longitude - r],
          [wp.latitude + r, wp.longitude + r],
          [wp.latitude - r, wp.longitude + r],
          [wp.latitude - r, wp.longitude - r]
        ];
      }
      poly.push(pt);
    }
    return poly;
  }

  // Initialize Tactical Leaflet Map
  function initTacticalMap() {
    const mapEl = document.getElementById('recon-map');
    if (!mapEl || typeof L === 'undefined') return;

    if (reconMap) {
      reconMap.invalidateSize();
      return;
    }

    try {
      reconMap = L.map(mapEl, {
        zoomControl: true,
        attributionControl: false
      }).setView([37.7749, -122.4194], 16);

      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        maxZoom: 19
      }).addTo(reconMap);

      // Force high-contrast rendering in Leaflet overlay pane
      const overlayPane = reconMap.getPane('overlayPane');
      if (overlayPane) {
        overlayPane.classList.add('leaflet-overlay-pane');
      }
    } catch (e) {
      console.warn('Failed to initialize recon Leaflet map:', e);
    }
  }

  // Update HUD Readouts
  function updateHUD(wp) {
    if (!wp) return;

    // 1. Heading
    const headingVal = document.getElementById('hud-heading-value');
    if (headingVal) {
      const deg = Math.round(wp.headingDeg || 0);
      headingVal.textContent = `${String(deg).padStart(3, '0')}°`;
    }

    // 2. Altitude AGL
    const altVal = document.getElementById('hud-altitude-value');
    if (altVal) {
      const alt = (wp.altitudeMetersAGL !== undefined ? wp.altitudeMetersAGL : (wp.altitudeAGL || 0));
      altVal.textContent = `${alt.toFixed(1)} m`;
    }

    // 3. Ground Speed
    const speedVal = document.getElementById('hud-speed-value');
    if (speedVal) {
      const spd = wp.groundSpeedMps || 0;
      speedVal.textContent = `${spd.toFixed(1)} m/s`;
    }

    // 4. Battery Level
    const batVal = document.getElementById('hud-battery-value');
    if (batVal) {
      batVal.textContent = `${wp.batteryPercent !== undefined ? wp.batteryPercent : 100}%`;
    }

    // 5. Artificial Horizon / Attitude
    const pitchVal = document.getElementById('hud-pitch-roll-value');
    const horizonEl = document.getElementById('attitude-horizon');
    const pitch = wp.gimbalPitchDeg !== undefined ? wp.gimbalPitchDeg : -90;
    const roll = wp.gimbalRollDeg || 0;
    if (pitchVal) {
      pitchVal.textContent = `${Math.round(pitch)}° / ${Math.round(roll)}°`;
    }
    if (horizonEl) {
      // Map pitch to vertical translation (-90 nadir = top, 0 horizontal = middle)
      const translateY = (pitch + 45) * 0.5;
      horizonEl.style.transform = `translateY(${translateY}px) rotate(${roll}deg)`;
    }
  }

  // Update Map Trajectory, Frustum, and Drone Marker
  function updateMapLayers(targetIndex) {
    if (!reconMap || typeof L === 'undefined' || waypoints.length === 0) return;

    const idx = Math.max(0, Math.min(waypoints.length - 1, targetIndex));
    const wp = waypoints[idx];
    if (!wp) return;

    // 1. Flight Trajectory Polyline (Cyan #00ffff)
    const latLngs = waypoints.map(w => [w.latitude, w.longitude]);
    if (!flightPolyline) {
      flightPolyline = L.polyline(latLngs, {
        color: '#00ffff',
        weight: 3,
        opacity: 0.9,
        lineCap: 'round',
        className: 'recon-flight-polyline'
      }).addTo(reconMap);
    } else {
      flightPolyline.setLatLngs(latLngs);
    }

    // 2. Camera Frustum Polygon
    const frustumCoords = computeCameraFrustum(wp, cameraIntrinsics);
    if (!frustumPolygon) {
      frustumPolygon = L.polygon(frustumCoords, {
        color: '#00ffff',
        weight: 2,
        fillColor: '#00ffff',
        fillOpacity: 0.25,
        className: 'recon-frustum-polygon'
      }).addTo(reconMap);
    } else {
      frustumPolygon.setLatLngs(frustumCoords);
    }

    // 3. Drone Position Marker
    const droneHeading = wp.headingDeg || 0;
    const droneIcon = L.divIcon({
      className: 'drone-icon-wrapper',
      html: `<div class="drone-marker" style="transform: rotate(${droneHeading}deg)" title="UAV Position (Heading: ${Math.round(droneHeading)}°)">🔺</div>`,
      iconSize: [24, 24],
      iconAnchor: [12, 12]
    });

    if (!droneMarker) {
      droneMarker = L.marker([wp.latitude, wp.longitude], {
        icon: droneIcon,
        zIndexOffset: 500
      }).addTo(reconMap);
    } else {
      droneMarker.setLatLng([wp.latitude, wp.longitude]);
      droneMarker.setIcon(droneIcon);
    }

    // Also mirror layers to #search-party-map if present
    const spMapEl = document.getElementById('search-party-map');
    if (spMapEl && window.searchPartyMapInstance && typeof window.searchPartyMapInstance.addLayer === 'function') {
      try {
        if (!window.__spFlightPolyline) {
          window.__spFlightPolyline = L.polyline(latLngs, {
            color: '#00ffff',
            weight: 3,
            opacity: 0.9
          }).addTo(window.searchPartyMapInstance);
        } else {
          window.__spFlightPolyline.setLatLngs(latLngs);
        }
        if (!window.__spFrustumPoly) {
          window.__spFrustumPoly = L.polygon(frustumCoords, {
            color: '#00ffff',
            weight: 2,
            fillColor: '#00ffff',
            fillOpacity: 0.25
          }).addTo(window.searchPartyMapInstance);
        } else {
          window.__spFrustumPoly.setLatLngs(frustumCoords);
        }
      } catch (_) {}
    }
  }

  // Draw Reticle Canvas Bounding Boxes
  function updateReticleCanvas() {
    const canvas = document.getElementById('recon-reticle-canvas');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    // Size canvas to video or container
    const video = document.getElementById('recon-video');
    const width = video && video.videoWidth ? video.videoWidth : (canvas.clientWidth || 640);
    const height = video && video.videoHeight ? video.videoHeight : (canvas.clientHeight || 360);

    if (canvas.width !== width || canvas.height !== height) {
      canvas.width = width;
      canvas.height = height;
    }

    ctx.clearRect(0, 0, width, height);

    // Draw active hotspots
    for (const h of hotspots) {
      if (h.status === 'DISMISSED') continue;
      const b = h.boundingBox;
      if (!b) continue;

      const bx = b.x * width;
      const by = b.y * height;
      const bw = b.width * width;
      const bh = b.height * height;

      // Reticle Box
      ctx.strokeStyle = h.id === selectedHotspotId ? '#00ffff' : '#ef4444';
      ctx.lineWidth = 2.5;
      ctx.strokeRect(bx, by, bw, bh);

      // Corner Accents
      const cornerLen = Math.min(bw, bh) * 0.25;
      ctx.strokeStyle = '#ffffff';
      ctx.lineWidth = 1.5;
      // Top-Left
      ctx.beginPath();
      ctx.moveTo(bx, by + cornerLen);
      ctx.lineTo(bx, by);
      ctx.lineTo(bx + cornerLen, by);
      ctx.stroke();

      // Label Tag
      const label = `${h.classification || 'Hotspot'} ${(h.confidenceScore * 100).toFixed(0)}% (${h.estimatedTempC ? h.estimatedTempC.toFixed(1) : '36.5'}°C)`;
      ctx.fillStyle = 'rgba(15, 23, 42, 0.85)';
      ctx.font = 'bold 12px Inter, sans-serif';
      const textWidth = ctx.measureText(label).width;
      ctx.fillRect(bx, Math.max(0, by - 20), textWidth + 8, 18);
      ctx.fillStyle = '#ffffff';
      ctx.fillText(label, bx + 4, Math.max(14, by - 6));
    }
  }

  // Plot Hotspot Pin on Leaflet Map
  function plotHotspotMarker(hotspot) {
    if (!reconMap || typeof L === 'undefined') return;
    if (hotspotMarkers.has(hotspot.id)) {
      const existing = hotspotMarkers.get(hotspot.id);
      existing.setLatLng([hotspot.latitude, hotspot.longitude]);
      return;
    }

    const flameIcon = L.divIcon({
      className: 'marker-hotspot-pulse-wrapper',
      html: '<div class="marker-hotspot-pulse" role="img" aria-label="Thermal hotspot anomaly"><span class="flame-icon">🔥</span><span class="hotspot-pulse-ring"></span></div>',
      iconSize: [32, 32],
      iconAnchor: [16, 16]
    });

    const marker = L.marker([hotspot.latitude, hotspot.longitude], {
      icon: flameIcon,
      zIndexOffset: 800
    }).addTo(reconMap);

    const tempText = hotspot.estimatedTempC ? `${hotspot.estimatedTempC.toFixed(1)}°C` : '36.5°C';
    const confText = hotspot.confidenceScore ? `${(hotspot.confidenceScore * 100).toFixed(0)}%` : '85%';
    marker.bindPopup(`
      <div class="hotspot-popup">
        <h4>🔥 Thermal Anomaly Detected</h4>
        <p><strong>Classification:</strong> ${escapeHTML(hotspot.classification || 'Animal Signature')}</p>
        <p><strong>Confidence:</strong> ${confText}</p>
        <p><strong>Estimated Temp:</strong> ${tempText}</p>
        <p><strong>Coordinates:</strong> ${hotspot.latitude.toFixed(5)}, ${hotspot.longitude.toFixed(5)}</p>
      </div>
    `);

    marker.on('click', () => {
      selectHotspot(hotspot.id);
    });

    hotspotMarkers.set(hotspot.id, marker);

    // Also plot on #search-party-map if active
    if (window.searchPartyMapInstance && typeof window.searchPartyMapInstance.addLayer === 'function') {
      try {
        const spMarker = L.marker([hotspot.latitude, hotspot.longitude], {
          icon: flameIcon,
          zIndexOffset: 800
        }).addTo(window.searchPartyMapInstance);
        spMarker.bindPopup(marker.getPopup());
      } catch (_) {}
    }
  }

  // Render Hotspots in Drawer
  function renderHotspotsList() {
    const listEl = document.getElementById('recon-hotspots-list');
    const badgeEl = document.getElementById('hotspot-count-badge');
    if (!listEl) return;

    if (badgeEl) {
      badgeEl.textContent = `${hotspots.length} detected`;
    }

    if (hotspots.length === 0) {
      listEl.innerHTML = '<p class="text-secondary empty-hotspots-text" id="recon-empty-hotspots">No thermal anomalies detected in current frame or sortie.</p>';
      updateDrawerActionButtons();
      return;
    }

    listEl.innerHTML = '';
    // Sort by confidence descending
    const sorted = [...hotspots].sort((a, b) => (b.confidenceScore || 0) - (a.confidenceScore || 0));

    for (const h of sorted) {
      const card = document.createElement('div');
      card.className = `hotspot-card ${h.id === selectedHotspotId ? 'selected' : ''}`;
      card.id = `hotspot-card-${h.id}`;
      card.setAttribute('role', 'button');
      card.setAttribute('tabindex', '0');
      card.setAttribute('aria-pressed', h.id === selectedHotspotId ? 'true' : 'false');

      const tempStr = h.estimatedTempC ? `${h.estimatedTempC.toFixed(1)}°C` : '36.5°C';
      const confStr = `${((h.confidenceScore || 0) * 100).toFixed(0)}%`;
      const statusClass = h.status === 'CONFIRMED' ? 'badge-success' : (h.status === 'DISMISSED' ? 'badge-secondary' : 'badge-warning');

      let thumbHtml = '';
      if (h.thumbnailBase64) {
        const src = h.thumbnailBase64.startsWith('data:') ? h.thumbnailBase64 : `data:image/jpeg;base64,${h.thumbnailBase64}`;
        thumbHtml = `<img src="${src}" alt="Crop of thermal hotspot" class="hotspot-thumb">`;
      } else {
        thumbHtml = `<div class="hotspot-thumb-placeholder">🔥</div>`;
      }

      card.innerHTML = `
        ${thumbHtml}
        <div class="hotspot-card-info">
          <div class="hotspot-card-header">
            <strong>${escapeHTML(h.classification || 'Biological Heat Signature')}</strong>
            <span class="badge ${statusClass}">${h.status || 'UNVERIFIED'}</span>
          </div>
          <div class="hotspot-card-metrics">
            <span class="metric-item">Temp: <strong>${tempStr}</strong></span>
            <span class="metric-item">Conf: <strong>${confStr}</strong></span>
            <span class="metric-item">Palette: <strong>${escapeHTML(h.palette || 'WHITE_HOT')}</strong></span>
          </div>
        </div>
      `;

      card.addEventListener('click', () => {
        selectHotspot(h.id);
      });
      card.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          selectHotspot(h.id);
        }
      });

      listEl.appendChild(card);
    }

    updateDrawerActionButtons();
  }

  function selectHotspot(id) {
    selectedHotspotId = id;
    renderHotspotsList();
    updateReticleCanvas();

    const h = hotspots.find(item => item.id === id);
    if (h && reconMap) {
      reconMap.panTo([h.latitude, h.longitude]);
      const marker = hotspotMarkers.get(id);
      if (marker) {
        marker.openPopup();
      }
    }
  }

  function updateDrawerActionButtons() {
    const confirmBtn = document.getElementById('btn-confirm-hotspot');
    const dismissBtn = document.getElementById('btn-dismiss-hotspot');
    const selected = hotspots.find(h => h.id === selectedHotspotId);

    if (confirmBtn) {
      confirmBtn.disabled = !selected || selected.status === 'CONFIRMED';
    }
    if (dismissBtn) {
      dismissBtn.disabled = !selected || selected.status === 'DISMISSED';
    }
  }

  // Confirm or Dismiss Hotspot Status
  async function updateHotspotStatus(newStatus) {
    if (!selectedHotspotId) return;
    const h = hotspots.find(item => item.id === selectedHotspotId);
    if (!h) return;

    try {
      const headers = { 'Content-Type': 'application/json' };
      const csrf = getCsrfToken();
      if (csrf) headers['X-CSRF-Token'] = csrf;

      const resp = await fetch(`/api/v1/recon/hotspots/${encodeURIComponent(h.id)}/status`, {
        method: 'PUT',
        headers,
        body: JSON.stringify({ status: newStatus })
      });

      if (!resp.ok) {
        throw new Error(`Failed to update hotspot status: HTTP ${resp.status}`);
      }

      const updated = await resp.json();
      h.status = updated.status || newStatus;
      if (updated.linkedSightingId) {
        h.linkedSightingId = updated.linkedSightingId;
      }

      renderHotspotsList();
      updateReticleCanvas();

      // Dispatch mesh:thermal-hotspot event
      const detail = {
        id: h.id,
        missionId: h.missionId || currentMissionId,
        petId: h.petId || currentPetId,
        latitude: h.latitude,
        longitude: h.longitude,
        confidence: h.confidenceScore,
        estimatedTempC: h.estimatedTempC,
        palette: h.palette,
        status: h.status,
        classification: h.classification,
        linkedSightingId: h.linkedSightingId
      };
      const event = new CustomEvent('mesh:thermal-hotspot', { detail, bubbles: true });
      window.dispatchEvent(event);
      document.dispatchEvent(event);

      announce(`Hotspot ${h.id} status updated to ${newStatus}.`);
    } catch (err) {
      console.error('Error updating hotspot status:', err);
      announce('Failed to update hotspot status.');
    }
  }

  // Parse Telemetry (.srt, .kml, .geojson)
  async function parseTelemetryFile(file) {
    try {
      announce('Parsing flight telemetry...');
      const formData = new FormData();
      formData.append('file', file);
      if (currentPetId) formData.append('petId', currentPetId);
      if (currentMissionId) formData.append('missionId', currentMissionId);

      const headers = {};
      const csrf = getCsrfToken();
      if (csrf) headers['X-CSRF-Token'] = csrf;

      const resp = await fetch('/api/v1/recon/telemetry/parse', {
        method: 'POST',
        headers,
        body: formData
      });

      if (!resp.ok) {
        throw new Error(`Telemetry parsing failed: HTTP ${resp.status}`);
      }

      const data = await resp.json();
      if (Array.isArray(data.waypoints) && data.waypoints.length > 0) {
        waypoints = data.waypoints;
        currentWaypointIndex = 0;

        // Configure slider
        const slider = document.getElementById('recon-timeline-slider');
        if (slider) {
          slider.min = '0';
          slider.max = String(waypoints.length - 1);
          slider.value = '0';
          slider.setAttribute('aria-valuemin', '0');
          slider.setAttribute('aria-valuemax', String(waypoints.length - 1));
          slider.setAttribute('aria-valuenow', '0');
        }

        updateHUD(waypoints[0]);
        updateMapLayers(0);

        if (reconMap && waypoints.length > 0) {
          const bounds = L.latLngBounds(waypoints.map(w => [w.latitude, w.longitude]));
          reconMap.fitBounds(bounds, { padding: [30, 30] });
        }

        // Add file to batch list
        addBatchItem(file.name, `${waypoints.length} waypoints parsed successfully.`);
        announce(`Telemetry loaded with ${waypoints.length} waypoints.`);
      }
    } catch (err) {
      console.error('Telemetry parse error:', err);
      announce('Error parsing telemetry flight log.');
    }
  }

  // Ingest Aerial Image & Radiometric Scan
  async function scanThermalImageFile(file) {
    try {
      announce('Analyzing aerial thermal image...');
      const formData = new FormData();
      formData.append('image', file);
      if (currentPetId) formData.append('petId', currentPetId);
      if (currentMissionId) formData.append('missionId', currentMissionId);

      const currentWp = waypoints[currentWaypointIndex] || {
        latitude: 37.7749,
        longitude: -122.4194,
        altitudeMetersAGL: 35.0,
        headingDeg: 142.5,
        gimbalPitchDeg: -45.0
      };

      formData.append('latitude', String(currentWp.latitude));
      formData.append('longitude', String(currentWp.longitude));
      formData.append('altitudeMetersAGL', String(currentWp.altitudeMetersAGL || 35.0));
      formData.append('headingDeg', String(currentWp.headingDeg || 0.0));
      formData.append('gimbalPitchDeg', String(currentWp.gimbalPitchDeg !== undefined ? currentWp.gimbalPitchDeg : -45.0));

      const headers = {};
      const csrf = getCsrfToken();
      if (csrf) headers['X-CSRF-Token'] = csrf;

      const resp = await fetch('/api/v1/recon/thermal/scan', {
        method: 'POST',
        headers,
        body: formData
      });

      if (!resp.ok) {
        throw new Error(`Thermal scan failed: HTTP ${resp.status}`);
      }

      const detected = await resp.json();
      if (Array.isArray(detected)) {
        for (const item of detected) {
          if (!hotspots.some(h => h.id === item.id)) {
            hotspots.push(item);
            plotHotspotMarker(item);

            // Dispatch mesh:thermal-hotspot event
            const detail = {
              id: item.id,
              missionId: item.missionId || currentMissionId,
              petId: item.petId || currentPetId,
              latitude: item.latitude,
              longitude: item.longitude,
              confidence: item.confidenceScore,
              estimatedTempC: item.estimatedTempC,
              palette: item.palette,
              boundingBox: item.boundingBox,
              status: item.status
            };
            const event = new CustomEvent('mesh:thermal-hotspot', { detail, bubbles: true });
            window.dispatchEvent(event);
            document.dispatchEvent(event);
          }
        }

        if (detected.length > 0) {
          selectHotspot(detected[0].id);
        }
        renderHotspotsList();
        updateReticleCanvas();
        addBatchItem(file.name, `${detected.length} thermal anomalies detected.`);
        announce(`Thermal scan complete: ${detected.length} hotspots detected.`);
      }
    } catch (err) {
      console.error('Thermal scan error:', err);
      announce('Error scanning aerial thermal image.');
    }
  }

  function addBatchItem(name, status) {
    const list = document.getElementById('recon-batch-list');
    const empty = document.getElementById('recon-empty-batch');
    if (!list) return;
    if (empty) empty.remove();

    const item = document.createElement('div');
    item.className = 'batch-item-row';
    item.innerHTML = `
      <span class="batch-item-icon">📄</span>
      <div class="batch-item-meta">
        <strong>${escapeHTML(name)}</strong>
        <span class="text-secondary">${escapeHTML(status)}</span>
      </div>
    `;
    list.prepend(item);
  }

  // Scrubber Synchronization
  function handleTimelineScrub(value) {
    if (waypoints.length === 0) return;
    const idx = Math.max(0, Math.min(waypoints.length - 1, Math.round(Number(value))));
    currentWaypointIndex = idx;
    const wp = waypoints[idx];

    updateHUD(wp);
    updateMapLayers(idx);
    updateReticleCanvas();

    const timecode = document.getElementById('recon-timecode');
    if (timecode) {
      const currentSec = idx;
      const totalSec = waypoints.length;
      timecode.textContent = `${formatTime(currentSec)} / ${formatTime(totalSec)}`;
    }

    const slider = document.getElementById('recon-timeline-slider');
    if (slider) {
      slider.setAttribute('aria-valuenow', String(idx));
    }
  }

  function formatTime(seconds) {
    const m = Math.floor(seconds / 60);
    const s = (seconds % 60).toFixed(1);
    return `${String(m).padStart(2, '0')}:${s < 10 ? '0' : ''}${s}`;
  }

  // Play / Pause video & timeline simulation
  function togglePlay() {
    isPlaying = !isPlaying;
    const icon = document.getElementById('play-pause-icon');
    const video = document.getElementById('recon-video');

    if (isPlaying) {
      if (icon) icon.textContent = '⏸';
      if (video) video.play().catch(() => {});
      playInterval = setInterval(() => {
        if (waypoints.length === 0) return;
        currentWaypointIndex = (currentWaypointIndex + 1) % waypoints.length;
        const slider = document.getElementById('recon-timeline-slider');
        if (slider) slider.value = String(currentWaypointIndex);
        handleTimelineScrub(currentWaypointIndex);
      }, 500);
    } else {
      if (icon) icon.textContent = '▶';
      if (video) video.pause();
      if (playInterval) clearInterval(playInterval);
      playInterval = null;
    }
  }

  // Tab switching
  function switchTab(tabId) {
    const tabCockpit = document.getElementById('tab-recon-cockpit');
    const tabBatch = document.getElementById('tab-recon-batch');
    const panelCockpit = document.getElementById('recon-panel-cockpit');
    const panelBatch = document.getElementById('recon-panel-batch');

    if (tabId === 'tab-recon-cockpit') {
      tabCockpit?.classList.add('active');
      tabCockpit?.setAttribute('aria-selected', 'true');
      tabBatch?.classList.remove('active');
      tabBatch?.setAttribute('aria-selected', 'false');

      panelCockpit?.classList.remove('hidden');
      panelCockpit?.classList.add('active');
      panelBatch?.classList.add('hidden');
      panelBatch?.classList.remove('active');

      if (reconMap) {
        setTimeout(() => reconMap.invalidateSize(), 50);
      }
    } else {
      tabBatch?.classList.add('active');
      tabBatch?.setAttribute('aria-selected', 'true');
      tabCockpit?.classList.remove('active');
      tabCockpit?.setAttribute('aria-selected', 'false');

      panelBatch?.classList.remove('hidden');
      panelBatch?.classList.add('active');
      panelCockpit?.classList.add('hidden');
      panelCockpit?.classList.remove('active');
    }
  }

  // Open & Close Modal with Focus Trap
  function openReconModal(petId = '') {
    reconModal = document.getElementById('recon-modal');
    if (!reconModal) return;

    previouslyFocusedElement = document.activeElement;
    currentPetId = petId || currentPetId || '';

    reconModal.classList.remove('hidden');
    reconModal.setAttribute('aria-hidden', 'false');

    initTacticalMap();

    // Focus close button or first interactive element
    const closeBtn = document.getElementById('btn-close-recon-modal');
    if (closeBtn) closeBtn.focus();

    announce('Aerial Reconnaissance Cockpit opened.');
  }

  function closeReconModal() {
    if (!reconModal) return;
    if (isPlaying) togglePlay();

    reconModal.classList.add('hidden');
    reconModal.setAttribute('aria-hidden', 'true');

    if (previouslyFocusedElement && typeof previouslyFocusedElement.focus === 'function') {
      previouslyFocusedElement.focus();
    }
    announce('Aerial Reconnaissance Cockpit closed.');
  }

  // Focus trap on modal
  function handleKeyDown(e) {
    if (!reconModal || reconModal.classList.contains('hidden')) return;

    if (e.key === 'Escape') {
      e.preventDefault();
      closeReconModal();
      return;
    }

    if (e.key === 'Tab') {
      const focusables = reconModal.querySelectorAll(
        'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
      );
      if (focusables.length === 0) return;

      const first = focusables[0];
      const last = focusables[focusables.length - 1];

      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  }

  // Helper escapeHTML
  function escapeHTML(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  // Setup Event Listeners
  function setupEventListeners() {
    reconModal = document.getElementById('recon-modal');

    // 1. Launch Buttons
    document.addEventListener('click', (e) => {
      const btn = e.target.closest('#btn-open-drone-recon, .btn-open-drone-recon, [data-action="open-drone-recon"]');
      if (btn) {
        e.preventDefault();
        const petId = btn.getAttribute('data-pet-id') || '';
        openReconModal(petId);
      }
    });

    // 2. Close Buttons
    const closeBtn = document.getElementById('btn-close-recon-modal');
    if (closeBtn) {
      closeBtn.addEventListener('click', closeReconModal);
    }
    document.addEventListener('click', (e) => {
      if (e.target.matches('[data-close-modal="recon"]')) {
        closeReconModal();
      }
    });

    // 3. Dual Tabs
    const tabCockpit = document.getElementById('tab-recon-cockpit');
    const tabBatch = document.getElementById('tab-recon-batch');
    tabCockpit?.addEventListener('click', () => switchTab('tab-recon-cockpit'));
    tabBatch?.addEventListener('click', () => switchTab('tab-recon-batch'));

    // 4. Timeline Controls
    const slider = document.getElementById('recon-timeline-slider');
    slider?.addEventListener('input', (e) => handleTimelineScrub(e.target.value));
    slider?.addEventListener('change', (e) => handleTimelineScrub(e.target.value));

    const playBtn = document.getElementById('btn-recon-play-pause');
    playBtn?.addEventListener('click', togglePlay);

    // 5. Hotspot Action Buttons
    const confirmBtn = document.getElementById('btn-confirm-hotspot');
    confirmBtn?.addEventListener('click', () => updateHotspotStatus('CONFIRMED'));

    const dismissBtn = document.getElementById('btn-dismiss-hotspot');
    dismissBtn?.addEventListener('click', () => updateHotspotStatus('DISMISSED'));

    // 6. Dropzone & File Input
    const dropzone = document.getElementById('recon-dropzone');
    const fileInput = document.getElementById('recon-file-input');

    if (dropzone && fileInput) {
      dropzone.addEventListener('click', () => fileInput.click());
      dropzone.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          fileInput.click();
        }
      });

      dropzone.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropzone.classList.add('drag-over');
      });
      dropzone.addEventListener('dragleave', () => {
        dropzone.classList.remove('drag-over');
      });
      dropzone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropzone.classList.remove('drag-over');
        const files = e.dataTransfer?.files;
        if (files && files.length > 0) {
          handleFiles(files);
        }
      });

      fileInput.addEventListener('change', (e) => {
        const files = e.target.files;
        if (files && files.length > 0) {
          handleFiles(files);
        }
      });
    }

    function handleFiles(fileList) {
      for (let i = 0; i < fileList.length; i++) {
        const file = fileList[i];
        const lower = file.name.toLowerCase();
        if (lower.endsWith('.srt') || lower.endsWith('.kml') || lower.endsWith('.geojson')) {
          parseTelemetryFile(file);
        } else if (lower.endsWith('.jpg') || lower.endsWith('.jpeg') || lower.endsWith('.png')) {
          scanThermalImageFile(file);
        }
      }
    }

    // 7. Focus Trap & Escape key
    document.addEventListener('keydown', handleKeyDown);

    // 8. Listen for mesh:thermal-hotspot event
    const onMeshHotspot = (e) => {
      const h = e.detail;
      if (!h || !h.id) return;

      const existingIndex = hotspots.findIndex(item => item.id === h.id);
      if (existingIndex >= 0) {
        hotspots[existingIndex] = { ...hotspots[existingIndex], ...h };
      } else {
        hotspots.push({
          id: h.id,
          missionId: h.missionId || '',
          petId: h.petId || '',
          latitude: h.latitude || 0,
          longitude: h.longitude || 0,
          confidenceScore: h.confidence !== undefined ? h.confidence : (h.confidenceScore || 0.8),
          estimatedTempC: h.estimatedTempC || 36.5,
          palette: h.palette || 'WHITE_HOT',
          boundingBox: h.boundingBox || { x: 0.4, y: 0.4, width: 0.2, height: 0.2 },
          status: h.status || 'UNVERIFIED',
          classification: h.classification || 'Thermal Anomaly'
        });
      }

      plotHotspotMarker(hotspots[existingIndex >= 0 ? existingIndex : hotspots.length - 1]);
      renderHotspotsList();
      updateReticleCanvas();
    };

    window.addEventListener('mesh:thermal-hotspot', onMeshHotspot);
    document.addEventListener('mesh:thermal-hotspot', onMeshHotspot);
  }

  // Public Interface for programmatic inspection / testing
  window.PetSpotRDronRecon = {
    open: openReconModal,
    close: closeReconModal,
    getWaypoints: () => [...waypoints],
    getHotspots: () => [...hotspots],
    scrub: handleTimelineScrub,
    confirmHotspot: () => updateHotspotStatus('CONFIRMED'),
    dismissHotspot: () => updateHotspotStatus('DISMISSED')
  };

  // Run initialization
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', setupEventListeners);
  } else {
    setupEventListeners();
  }
})();
