/**
 * PetSpotR Canine (K9) Scent-Trailing Telemetry, Atmospheric Wind Plume Dispersion &
 * Handler Cockpit Controller (Milestone 12.4)
 */

(function () {
  'use strict';

  const missionCache = new Map();
  let selectedMissionId = '';
  let sseSource = null;

  /**
   * Escape untrusted text for safe HTML interpolation
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
   * Display toast notification & screen reader announcement
   */
  function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    if (container) {
      const toast = document.createElement('div');
      toast.className = `toast toast-${type}`;
      toast.textContent = message;
      container.appendChild(toast);
      setTimeout(() => {
        toast.remove();
      }, 4000);
    }
    const announcer = document.getElementById('k9-aria-announcer');
    if (announcer) {
      announcer.textContent = message;
    }
  }

  /**
   * Map ScentViabilityRating to WCAG AAA badge CSS class
   */
  function getViabilityBadgeClass(rating) {
    switch (rating) {
      case 'HIGH_VIABILITY':
        return 'badge-viability-high';
      case 'MODERATE_VIABILITY':
        return 'badge-viability-moderate';
      case 'DEGRADED_THERMAL_HAZARD':
        return 'badge-viability-degraded';
      case 'EXPIRED_SCENT_TRAIL':
      default:
        return 'badge-viability-expired';
    }
  }

  /**
   * Project destination coordinate (lat/lng) given origin, bearing (deg), and distance (meters)
   */
  function projectDestination(latDeg, lngDeg, bearingDeg, distanceM) {
    const earthRadiusM = 6371000.0;
    const lat1 = (latDeg * Math.PI) / 180.0;
    const lng1 = (lngDeg * Math.PI) / 180.0;
    const brng = (bearingDeg * Math.PI) / 180.0;
    const angDist = distanceM / earthRadiusM;

    const sinLat2 =
      Math.sin(lat1) * Math.cos(angDist) +
      Math.cos(lat1) * Math.sin(angDist) * Math.cos(brng);
    const lat2 = Math.asin(Math.max(-1.0, Math.min(1.0, sinLat2)));

    const y = Math.sin(brng) * Math.sin(angDist) * Math.cos(lat1);
    const x = Math.cos(angDist) - Math.sin(lat1) * Math.sin(lat2);
    const lng2 = lng1 + Math.atan2(y, x);

    return {
      lat: Number(((lat2 * 180.0) / Math.PI).toFixed(6)),
      lng: Number(((lng2 * 180.0) / Math.PI).toFixed(6)),
    };
  }

  /**
   * Project geographic coordinate (lat, lng) onto the 420x320 SVG radar centered at LKP (210, 160)
   */
  function projectToSvgRadar(lat, lng, originLat, originLng, maxRangeM) {
    const dLat = lat - originLat;
    const dLng = lng - originLng;
    const northMeters = dLat * 111320.0;
    const eastMeters = dLng * 111320.0 * Math.cos((originLat * Math.PI) / 180.0);

    const safeRange = Math.max(maxRangeM || 450.0, 250.0);
    const pxPerMeter = 130.0 / safeRange;

    const rawX = 210.0 + eastMeters * pxPerMeter;
    const rawY = 160.0 - northMeters * pxPerMeter;

    const x = Math.max(15.0, Math.min(405.0, rawX));
    const y = Math.max(15.0, Math.min(305.0, rawY));

    return {
      x: Number(x.toFixed(1)),
      y: Number(y.toFixed(1)),
    };
  }

  /**
   * Build HTML for a single K9 mission card
   */
  function renderMissionCardHtml(m) {
    const missionId = escapeHtml(m.missionId || '');
    const canineName = escapeHtml(m.canineName || '');
    const canineCallsign = escapeHtml(m.canineCallsign || '');
    const handlerName = escapeHtml(m.handlerName || '');
    const petId = escapeHtml(m.petId || '');
    const discipline = escapeHtml(m.discipline || 'MANTRAILING_SCENT_SPECIFIC');
    const scentArticle = escapeHtml(m.scentArticleDesc || '');
    const rating = escapeHtml(m.viabilityRating || 'HIGH_VIABILITY');
    const badgeClass = getViabilityBadgeClass(m.viabilityRating);
    const score = Number(m.viabilityScore || 0);
    const plume = m.plumeCone || {};
    const azimuth = Math.round(Number(plume.downwindAzimuthDeg || 0));
    const intersectionPct = Number(m.plumeIntersectionPct || 0).toFixed(1);
    const pod = Number(m.estimatedPod || 0).toFixed(1);
    const env = m.environment || {};
    const tempC = Number(env.temperatureC || 0).toFixed(1);
    const humidityPct = Math.round(Number(env.relativeHumidityPct || 0));
    const surface = escapeHtml(env.groundSurface || 'FOREST_FLOOR');

    const pawHazardHtml = m.pawThermalHazard
      ? `<div class="k9-paw-hazard-banner" role="alert">🐾 THERMAL PAW HAZARD: ${escapeHtml(m.safetyAdvisory || '')}</div>`
      : '';

    const promotedSightingHtml = m.promotedSightingId
      ? `<div class="k9-promoted-sighting-banner">📍 Promoted to Verified Sighting: ${escapeHtml(m.promotedSightingId)}</div>`
      : '';

    const isSelectedClass = m.missionId === selectedMissionId ? ' k9-mission-card-active' : '';

    return `
      <article class="k9-mission-card${isSelectedClass}" data-mission-id="${missionId}" data-viability-rating="${rating}" role="listitem">
        <div class="k9-mission-card-header">
          <div class="k9-mission-identity">
            <strong class="k9-canine-name">${canineName}</strong>
            <span class="badge badge-k9-callsign">${canineCallsign}</span>
            <span class="badge badge-k9-discipline">${discipline}</span>
          </div>
          <div class="k9-mission-badges">
            <span class="badge badge-viability-rating ${badgeClass}">${rating}</span>
            <span class="badge badge-k9-pod">POD: ${pod}%</span>
          </div>
        </div>

        <div class="k9-mission-meta">
          <span>🐕‍🦺 Handler: <strong>${handlerName}</strong></span>
          <span>🐾 Target Pet: <code>${petId}</code></span>
          <span>🧪 Scent Article: ${scentArticle}</span>
        </div>

        <div class="k9-mission-metrics-row">
          <span>📊 Viability Score: <strong>${score}/100</strong></span>
          <span>🧭 Downwind Azimuth: <strong>${azimuth}°</strong></span>
          <span>🎯 Plume Intersection: <strong>${intersectionPct}%</strong></span>
          <span>🌡️ ${tempC}°C / ${humidityPct}% RH (${surface})</span>
        </div>

        ${pawHazardHtml}
        ${promotedSightingHtml}

        <div class="k9-mission-actions">
          <button type="button" class="btn btn-sm btn-secondary btn-select-k9-mission" data-mission-id="${missionId}">🎯 Inspect Plume &amp; Track</button>
          <a class="btn btn-sm btn-secondary btn-export-k9-gpx" href="/api/v1/k9/missions/${encodeURIComponent(m.missionId || '')}/gpx" download="k9-mission-${missionId}.gpx">📥 Export GPX 1.1</a>
        </div>
      </article>
    `;
  }

  /**
   * Render SVG Gaussian Wind Plume & K9 Track Visualizer for the active mission
   */
  function renderPlumeVisualizer(mission) {
    if (!mission) return;

    const activeBadge = document.getElementById('plume-active-mission-badge');
    const azimuthBadge = document.getElementById('plume-azimuth-badge');
    const podBadge = document.getElementById('mission-pod-badge');
    const pawBanner = document.getElementById('paw-hazard-banner');
    const polygonEl = document.getElementById('svg-plume-polygon');
    const trackLineEl = document.getElementById('svg-k9-track-line');
    const waypointsGroupEl = document.getElementById('svg-k9-waypoints-group');

    const plume = mission.plumeCone || {};
    const azimuthDeg = Math.round(Number(plume.downwindAzimuthDeg || 0));
    const podVal = Number(mission.estimatedPod || 0).toFixed(1);

    if (activeBadge) {
      activeBadge.textContent = `${mission.canineName || mission.missionId} (${mission.missionId})`;
    }
    if (azimuthBadge) {
      azimuthBadge.textContent = `Downwind: ${azimuthDeg}°`;
    }
    if (podBadge) {
      podBadge.textContent = `POD: ${podVal}%`;
      podBadge.className = `badge ${getViabilityBadgeClass(mission.viabilityRating)}`;
    }

    if (pawBanner) {
      if (mission.pawThermalHazard) {
        pawBanner.textContent = `🐾 THERMAL PAW HAZARD: ${mission.safetyAdvisory || 'Hot surface — equip K9 protective booties'}`;
        pawBanner.hidden = false;
      } else {
        pawBanner.textContent = '';
        pawBanner.hidden = true;
      }
    }

    const originLat = Number(mission.lkpOriginLat || plume.originLat || 47.6062);
    const originLng = Number(mission.lkpOriginLng || plume.originLng || -122.3321);
    const maxRangeM = Number(plume.effectiveLengthM || 450.0) * 1.12;

    if (polygonEl) {
      const vertices = Array.isArray(plume.vertices) ? plume.vertices : [];
      if (vertices.length >= 3) {
        const pts = vertices
          .map((v) => {
            const p = projectToSvgRadar(v.lat, v.lng, originLat, originLng, maxRangeM);
            return `${p.x},${p.y}`;
          })
          .join(' ');
        polygonEl.setAttribute('points', pts);
      } else {
        polygonEl.setAttribute('points', '210,160 325,122 335,160 325,198 210,160');
      }
      polygonEl.setAttribute('data-mission-id', mission.missionId || '');
      polygonEl.setAttribute('data-azimuth', String(azimuthDeg));
    }

    const waypoints = Array.isArray(mission.waypoints) ? mission.waypoints : [];
    const trackPoints = ['210,160'];
    const circlesHtml = [];

    waypoints.forEach((wp) => {
      const p = projectToSvgRadar(wp.lat, wp.lng, originLat, originLng, maxRangeM);
      trackPoints.push(`${p.x},${p.y}`);
      const inPlumeAttr = wp.inScentPlume ? 'true' : 'false';
      const safeWpId = escapeHtml(wp.waypointId || '');
      const safeBehavior = escapeHtml(wp.behavior || '');
      circlesHtml.push(
        `<circle class="svg-k9-waypoint" cx="${p.x}" cy="${p.y}" r="5" data-waypoint-id="${safeWpId}" data-in-plume="${inPlumeAttr}" data-behavior="${safeBehavior}"></circle>`
      );
    });

    if (trackLineEl) {
      trackLineEl.setAttribute('points', trackPoints.join(' '));
    }
    if (waypointsGroupEl) {
      waypointsGroupEl.innerHTML = circlesHtml.join('');
    }
  }

  /**
   * Render Waypoints list for the currently selected mission
   */
  function renderWaypointsList(mission) {
    const listEl = document.getElementById('k9-waypoints-list');
    if (!listEl) return;

    if (!mission) {
      listEl.innerHTML = '<p class="k9-empty-hint">No mission selected.</p>';
      return;
    }

    const waypoints = Array.isArray(mission.waypoints) ? mission.waypoints : [];
    if (waypoints.length === 0) {
      listEl.innerHTML = '<p class="k9-empty-hint">No waypoints recorded for this mission yet.</p>';
      return;
    }

    listEl.innerHTML = waypoints
      .map((wp) => {
        const wpId = escapeHtml(wp.waypointId || '');
        const behavior = escapeHtml(wp.behavior || 'CASTING_SEARCH');
        const inPlume = Boolean(wp.inScentPlume);
        const plumeBadgeClass = inPlume ? 'badge-viability-high' : 'badge-viability-degraded';
        const plumeLabel = inPlume ? 'IN PLUME CORRIDOR' : 'OUTSIDE PLUME';
        const coords = `${Number(wp.lat || 0).toFixed(4)}, ${Number(wp.lng || 0).toFixed(4)}`;
        const notes = escapeHtml(wp.notes || '');

        return `
          <div class="k9-waypoint-item" data-waypoint-id="${wpId}" data-in-plume="${inPlume}" role="listitem">
            <div class="k9-waypoint-item-header">
              <span class="badge badge-k9-behavior">${behavior}</span>
              <span class="badge ${plumeBadgeClass}">${plumeLabel}</span>
              <code class="k9-waypoint-coords">${coords}</code>
            </div>
            <p class="k9-waypoint-notes">${notes}</p>
          </div>
        `;
      })
      .join('');
  }

  /**
   * Sync mission select dropdown and render all panels
   */
  function renderAllMissions(missions) {
    missionCache.clear();
    missions.forEach((m) => {
      if (m && m.missionId) {
        missionCache.set(m.missionId, m);
      }
    });

    if (!selectedMissionId || !missionCache.has(selectedMissionId)) {
      if (missionCache.has('mission-k9-101')) {
        selectedMissionId = 'mission-k9-101';
      } else if (missions.length > 0) {
        selectedMissionId = missions[0].missionId;
      }
    }

    const countBadge = document.getElementById('k9-missions-count-badge');
    if (countBadge) {
      countBadge.textContent = `${missions.length} Missions`;
    }

    const listEl = document.getElementById('k9-missions-list');
    if (listEl) {
      if (missions.length === 0) {
        listEl.innerHTML = '<p class="k9-empty-hint" id="k9-missions-empty-msg">No active K9 scent missions dispatched yet.</p>';
      } else {
        listEl.innerHTML = missions.map(renderMissionCardHtml).join('');
      }
    }

    const selectEl = document.getElementById('waypoint-mission-select');
    if (selectEl) {
      selectEl.innerHTML = missions
        .map((m) => {
          const id = escapeHtml(m.missionId || '');
          const name = escapeHtml(m.canineName || '');
          const callsign = escapeHtml(m.canineCallsign || '');
          return `<option value="${id}">${name} (${callsign}) — ${id}</option>`;
        })
        .join('');
      if (selectedMissionId && missionCache.has(selectedMissionId)) {
        selectEl.value = selectedMissionId;
      }
    }

    const activeMission = missionCache.get(selectedMissionId) || missions[0] || null;
    renderPlumeVisualizer(activeMission);
    renderWaypointsList(activeMission);
  }

  /**
   * Fetch all K9 missions from `/api/v1/k9/missions`
   */
  async function fetchMissions() {
    try {
      const res = await fetch('/api/v1/k9/missions', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return;
      const data = await res.json();
      const missions = Array.isArray(data.missions) ? data.missions : [];
      renderAllMissions(missions);
    } catch (_err) {
      // Ignore transient network errors
    }
  }

  /**
   * Select a specific mission for plume inspection and waypoint logging
   */
  function selectMission(missionId) {
    if (!missionId || !missionCache.has(missionId)) return;
    selectedMissionId = missionId;

    const selectEl = document.getElementById('waypoint-mission-select');
    if (selectEl) {
      selectEl.value = missionId;
    }

    const cards = document.querySelectorAll('#k9-missions-list .k9-mission-card');
    cards.forEach((card) => {
      if (card.getAttribute('data-mission-id') === missionId) {
        card.classList.add('k9-mission-card-active');
      } else {
        card.classList.remove('k9-mission-card-active');
      }
    });

    const mission = missionCache.get(missionId);
    renderPlumeVisualizer(mission);
    renderWaypointsList(mission);
  }

  /**
   * Setup theme toggle button (`#theme-toggle`)
   */
  function setupThemeToggle() {
    const btn = document.getElementById('theme-toggle');
    if (!btn) return;
    btn.addEventListener('click', () => {
      const html = document.documentElement;
      const current = html.getAttribute('data-theme') || 'dark';
      const next = current === 'dark' ? 'light' : 'dark';
      html.setAttribute('data-theme', next);
    });
  }

  /**
   * Setup Create K9 Scent Mission form (`#form-create-k9-mission`)
   */
  function setupCreateMissionForm() {
    const form = document.getElementById('form-create-k9-mission');
    if (!form) return;

    form.addEventListener('submit', async (e) => {
      e.preventDefault();

      const petId = (document.getElementById('k9-pet-id')?.value || '').trim();
      const handlerName = (document.getElementById('k9-handler-name')?.value || '').trim();
      const canineName = (document.getElementById('k9-canine-name')?.value || '').trim();
      const canineCallsign = (document.getElementById('k9-canine-callsign')?.value || '').trim();
      const discipline = document.getElementById('k9-discipline')?.value || 'MANTRAILING_SCENT_SPECIFIC';
      const scentArticleDesc = (document.getElementById('k9-scent-article')?.value || '').trim();
      const lkpOriginLat = parseFloat(document.getElementById('k9-lkp-lat')?.value || '47.6062');
      const lkpOriginLng = parseFloat(document.getElementById('k9-lkp-lng')?.value || '-122.3321');
      const windSpeedKmh = parseFloat(document.getElementById('k9-wind-speed')?.value || '14');
      const windFromDeg = parseFloat(document.getElementById('k9-wind-from-deg')?.value || '270');
      const temperatureC = parseFloat(document.getElementById('k9-temp-c')?.value || '15');
      const relativeHumidityPct = parseFloat(document.getElementById('k9-humidity-pct')?.value || '74');
      const groundSurface = document.getElementById('k9-ground-surface')?.value || 'LUSH_VEGETATION';
      const trailAgeHours = parseFloat(document.getElementById('k9-trail-age-hours')?.value || '2.5');

      const submitBtn = document.getElementById('btn-create-k9-mission');
      if (submitBtn) submitBtn.disabled = true;

      try {
        const res = await fetch('/api/v1/k9/missions', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            petId,
            handlerName,
            canineName,
            canineCallsign,
            discipline,
            scentArticleDesc,
            lkpOriginLat,
            lkpOriginLng,
            environment: {
              windSpeedKmh,
              windFromDeg,
              temperatureC,
              relativeHumidityPct,
              groundSurface,
              trailAgeHours,
            },
          }),
        });

        if (res.ok) {
          const created = await res.json();
          const newId = created.missionId || created.mission?.missionId;
          if (newId) {
            selectedMissionId = newId;
          }
          await fetchMissions();
          showToast(`Dispatched K9 scent mission for ${canineName} (${canineCallsign})`, 'success');
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to launch K9 scent mission', 'error');
        }
      } catch (_err) {
        showToast('Network error launching K9 scent mission', 'error');
      } finally {
        if (submitBtn) submitBtn.disabled = false;
      }
    });
  }

  /**
   * Setup Mission Card inspect button delegation and mission dropdown change
   */
  function setupMissionSelectionHandlers() {
    const listEl = document.getElementById('k9-missions-list');
    if (listEl) {
      listEl.addEventListener('click', (e) => {
        const btn = e.target.closest('.btn-select-k9-mission');
        if (!btn) return;
        const missionId = btn.getAttribute('data-mission-id');
        if (missionId) {
          selectMission(missionId);
        }
      });
    }

    const selectEl = document.getElementById('waypoint-mission-select');
    if (selectEl) {
      selectEl.addEventListener('change', () => {
        if (selectEl.value) {
          selectMission(selectEl.value);
        }
      });
    }
  }

  /**
   * Submit a waypoint to `POST /api/v1/k9/missions/{id}/waypoints`
   */
  async function postWaypoint(missionId, payload) {
    const res = await fetch(`/api/v1/k9/missions/${encodeURIComponent(missionId)}/waypoints`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });

    if (res.ok) {
      selectedMissionId = missionId;
      await fetchMissions();
      showToast('Recorded K9 waypoint & updated downwind plume POD', 'success');
      return true;
    } else {
      const errData = await res.json().catch(() => ({}));
      showToast(errData.error || 'Failed to record K9 waypoint', 'error');
      return false;
    }
  }

  /**
   * Setup Waypoint Logger Form (`#form-log-k9-waypoint`) and Quick Simulation button (`#btn-sim-in-plume-alert`)
   */
  function setupWaypointForm() {
    const form = document.getElementById('form-log-k9-waypoint');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const selectEl = document.getElementById('waypoint-mission-select');
        const missionId = selectEl?.value || selectedMissionId;
        if (!missionId) return;

        const lat = parseFloat(document.getElementById('wp-lat')?.value || '47.6062');
        const lng = parseFloat(document.getElementById('wp-lng')?.value || '-122.3285');
        const behavior = document.getElementById('wp-behavior')?.value || 'STRONG_SCENT_PULL';
        const notes = (document.getElementById('wp-notes')?.value || '').trim();
        const promoteToSighting = Boolean(document.getElementById('wp-promote-sighting')?.checked);

        const btn = document.getElementById('btn-log-k9-waypoint');
        if (btn) btn.disabled = true;
        try {
          await postWaypoint(missionId, {
            lat,
            lng,
            behavior,
            notes,
            promoteToSighting,
          });
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const simBtn = document.getElementById('btn-sim-in-plume-alert');
    if (simBtn) {
      simBtn.addEventListener('click', async () => {
        const selectEl = document.getElementById('waypoint-mission-select');
        const missionId = selectEl?.value || selectedMissionId;
        if (!missionId) return;

        const mission = missionCache.get(missionId);
        const originLat = Number(mission?.lkpOriginLat || 47.6062);
        const originLng = Number(mission?.lkpOriginLng || -122.3321);
        const downwindAzimuth = Number(mission?.plumeCone?.downwindAzimuthDeg ?? 90.0);

        const targetCoord = projectDestination(originLat, originLng, downwindAzimuth, 200.0);

        const latInput = document.getElementById('wp-lat');
        const lngInput = document.getElementById('wp-lng');
        const behaviorSelect = document.getElementById('wp-behavior');
        const notesInput = document.getElementById('wp-notes');
        const promoteCheckbox = document.getElementById('wp-promote-sighting');

        if (latInput) latInput.value = String(targetCoord.lat);
        if (lngInput) lngInput.value = String(targetCoord.lng);
        if (behaviorSelect) behaviorSelect.value = 'STRONG_SCENT_PULL';
        if (notesInput) notesInput.value = 'Confirmed downwind scent cone lock';
        if (promoteCheckbox) promoteCheckbox.checked = true;

        simBtn.disabled = true;
        try {
          await postWaypoint(missionId, {
            lat: targetCoord.lat,
            lng: targetCoord.lng,
            behavior: 'STRONG_SCENT_PULL',
            notes: 'Confirmed downwind scent cone lock',
            promoteToSighting: true,
          });
        } finally {
          simBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Subscribe to SSE stream `/api/v1/k9/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/k9/stream');
      const refresh = () => {
        fetchMissions();
      };
      sseSource.addEventListener('k9_mission_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'k9_mission_updated') {
            refresh();
          }
        } catch (_e) {
          // Ignore non-JSON keepalive comments
        }
      };
    } catch (_err) {
      // Ignore SSE errors in headless environments
    }
  }

  /**
   * Initialize controller on DOMContentLoaded
   */
  function init() {
    setupThemeToggle();
    setupCreateMissionForm();
    setupMissionSelectionHandlers();
    setupWaypointForm();
    fetchMissions();
    setupSSEStream();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  window.addEventListener('beforeunload', () => {
    if (sseSource) {
      sseSource.close();
    }
  });
})();
