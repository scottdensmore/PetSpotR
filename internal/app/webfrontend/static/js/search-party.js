/**
 * PetSpotR - Search Party Sector Overlay & Volunteer Claim Controller
 *
 * Manages interactive Leaflet sector overlay with dynamic status coloring:
 *   - Amber (opacity ~0.4) for unassigned sectors
 *   - Blue/Indigo (opacity ~0.5) for active_search sectors
 *   - Green (opacity ~0.4) for cleared sectors
 *   - Red border pulse animation for sighting_reported sectors
 *
 * Handles sector click interactions to open Volunteer Claim Modal (#modal-claim-sector)
 * and Sector Status / Clearance Modal (#modal-clear-sector).
 *
 * Listens to SSE stream (ReunionHub) for search_party_updated events to dynamically
 * update polygon styles, volunteer counters, and coverage progress bar.
 * Strict CSP compliant: Zero inline scripts or eval.
 */
(() => {
  'use strict';

  // State
  let searchPartyMapInstance = null;
  let currentParty = null;
  let currentPetID = '';
  let activeEventSource = null;
  const sectorLayers = {}; // sectorId -> L.polygon
  const trailLayers = {}; // trailId -> L.polyline
  const sectorTrailsCache = {}; // sectorId -> array of VolunteerBreadcrumbTrail
  let activeVolunteerMarker = null;

  // Breadcrumbs recording state
  let isRecordingTrail = false;
  let activeWatchId = null;
  let activeTrailId = null;
  let activeSectorId = null;
  let selectedSectorId = null;
  let currentTrailPoints = [];
  let currentTrailDistanceMeters = 0;

  // Urgency prioritization & barrier state
  let sortSectorsByUrgency = false;
  let activeBarriers = [];

  // BLE Beacon Scanner state
  let beaconScannerInstance = null;
  let latestBeaconPing = null;
  let latestTriangulation = null;
  let beaconPingMarker = null;
  let beaconAccuracyCircle = null;

  // IndexedDB offline buffer constants
  const DB_NAME = 'petspotr_breadcrumbs_db';
  const DB_VERSION = 1;
  const STORE_NAME = 'search_party_breadcrumbs';
  let dbInstance = null;

  // Safe IndexedDB factory access
  function getIndexedDB() {
    if (typeof indexedDB !== 'undefined') return indexedDB;
    if (typeof window !== 'undefined' && window.indexedDB) return window.indexedDB;
    if (typeof globalThis !== 'undefined' && globalThis.indexedDB) return globalThis.indexedDB;
    return null;
  }

  // Open IndexedDB database
  function openBreadcrumbsDB() {
    if (dbInstance) return Promise.resolve(dbInstance);
    const idb = getIndexedDB();
    if (!idb) return Promise.resolve(null);

    return new Promise((resolve) => {
      try {
        const req = idb.open(DB_NAME, DB_VERSION);
        req.onupgradeneeded = function (e) {
          const db = e.target.result;
          if (!db.objectStoreNames.contains(STORE_NAME)) {
            db.createObjectStore(STORE_NAME, { keyPath: 'id' });
          }
        };
        req.onsuccess = function (e) {
          dbInstance = e.target.result;
          resolve(dbInstance);
        };
        req.onerror = function () {
          resolve(null);
        };
      } catch (_) {
        resolve(null);
      }
    });
  }

  // Queue breadcrumb batch in IndexedDB
  async function queueOfflineBreadcrumbs(batch) {
    const db = await openBreadcrumbsDB();
    if (!db) return;
    return new Promise((resolve) => {
      try {
        const tx = db.transaction(STORE_NAME, 'readwrite');
        const store = tx.objectStore(STORE_NAME);
        store.put(batch);
        tx.oncomplete = () => resolve();
        tx.onerror = () => resolve();
      } catch (_) {
        resolve();
      }
    });
  }

  // Retrieve all queued breadcrumb batches
  async function getQueuedBreadcrumbs() {
    const db = await openBreadcrumbsDB();
    if (!db) return [];
    return new Promise((resolve) => {
      try {
        const tx = db.transaction(STORE_NAME, 'readonly');
        const store = tx.objectStore(STORE_NAME);
        const req = store.getAll();
        req.onsuccess = () => resolve(req.result || []);
        req.onerror = () => resolve([]);
      } catch (_) {
        resolve([]);
      }
    });
  }

  // Remove flushed breadcrumb from IndexedDB
  async function removeQueuedBreadcrumb(id) {
    const db = await openBreadcrumbsDB();
    if (!db) return;
    return new Promise((resolve) => {
      try {
        const tx = db.transaction(STORE_NAME, 'readwrite');
        const store = tx.objectStore(STORE_NAME);
        store.delete(id);
        tx.oncomplete = () => resolve();
        tx.onerror = () => resolve();
      } catch (_) {
        resolve();
      }
    });
  }

  // Update offline badge in UI
  async function updateOfflineBadge() {
    const badge = document.getElementById('trail-offline-badge');
    if (!badge) return;
    const queued = await getQueuedBreadcrumbs();
    if (queued && queued.length > 0) {
      badge.textContent = `Buffered Offline (${queued.length})`;
      badge.classList.remove('hidden');
    } else {
      badge.classList.add('hidden');
    }
  }

  // Auto-flush queued breadcrumbs when online
  async function flushOfflineBreadcrumbs() {
    if (typeof navigator !== 'undefined' && !navigator.onLine) return;
    const queued = await getQueuedBreadcrumbs();
    if (!queued || queued.length === 0) {
      void updateOfflineBadge();
      return;
    }

    const csrf = await getCsrfToken();
    let flushedCount = 0;

    for (const item of queued) {
      try {
        const headers = { 'Content-Type': 'application/json' };
        if (csrf) headers['X-CSRF-Token'] = csrf;

        const res = await fetch(
          `/api/v1/search-parties/${encodeURIComponent(item.petId)}/sectors/${encodeURIComponent(item.sectorId)}/breadcrumbs`,
          {
            method: 'POST',
            headers,
            body: JSON.stringify(item.payload),
          }
        );

        if (res.ok) {
          await removeQueuedBreadcrumb(item.id);
          flushedCount++;
        } else if (res.status >= 400 && res.status < 500) {
          console.warn(`Breadcrumb batch rejected with client error ${res.status}, discarding item:`, item.id);
          await removeQueuedBreadcrumb(item.id);
        }
      } catch (err) {
        console.warn('Failed to flush breadcrumb batch:', err);
        break; // Stop flushing if connection drop persists
      }
    }

    await updateOfflineBadge();
    if (flushedCount > 0) {
      showToast(`Synced ${flushedCount} buffered search trail batch(es).`);
    }
  }

  // Generate UUID v4 for offline queue keys
  function generateUUID() {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
      const r = (Math.random() * 16) | 0;
      const v = c === 'x' ? r : (r & 0x3) | 0x8;
      return v.toString(16);
    });
  }

  // Spatial Distance Helpers
  function haversineMeters(lat1, lon1, lat2, lon2) {
    const R = 6371000.0;
    const dLat = ((lat2 - lat1) * Math.PI) / 180.0;
    const dLon = ((lon2 - lon1) * Math.PI) / 180.0;
    const a =
      Math.sin(dLat / 2) * Math.sin(dLat / 2) +
      Math.cos((lat1 * Math.PI) / 180.0) *
        Math.cos((lat2 * Math.PI) / 180.0) *
        Math.sin(dLon / 2) *
        Math.sin(dLon / 2);
    const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
    return R * c;
  }

  function calculateLocalTrailDistance(points) {
    if (!Array.isArray(points) || points.length < 2) return 0;
    let total = 0;
    for (let i = 0; i < points.length - 1; i++) {
      total += haversineMeters(
        points[i].latitude,
        points[i].longitude,
        points[i + 1].latitude,
        points[i + 1].longitude
      );
    }
    return Math.round(total * 100) / 100;
  }

  function formatDistance(meters) {
    if (typeof meters !== 'number' || isNaN(meters)) return '0.00 km';
    if (meters >= 1000) {
      return (meters / 1000).toFixed(2) + ' km';
    }
    return Math.round(meters) + ' m';
  }

  function updateDistanceDisplay(meters) {
    const el = document.getElementById('trail-distance-walked');
    if (el) {
      el.textContent = formatDistance(meters);
    }
  }

  // Volunteer Color Differentiation
  const TRAIL_COLORS = [
    '#4f46e5', // Indigo
    '#059669', // Emerald
    '#d97706', // Amber
    '#db2777', // Pink
    '#0891b2', // Cyan
    '#7c3aed', // Purple
    '#ea580c', // Orange
    '#2563eb', // Blue
  ];

  function getTrailColor(key) {
    if (!key) return TRAIL_COLORS[0];
    let hash = 0;
    for (let i = 0; i < key.length; i++) {
      hash = (hash << 5) - hash + key.charCodeAt(i);
      hash |= 0;
    }
    const idx = Math.abs(hash) % TRAIL_COLORS.length;
    return TRAIL_COLORS[idx];
  }

  // HTML entity escaper for safe DOM insertion
  function escapeHTML(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  // Format sector status for UI display
  function formatSectorStatus(status) {
    switch (status) {
      case 'unassigned':
        return 'Unassigned';
      case 'active_search':
        return 'Active Search';
      case 'cleared':
        return 'Cleared';
      case 'sighting_reported':
        return 'Sighting Reported';
      default:
        return status ? escapeHTML(status) : 'Unknown';
    }
  }

  // Sector style definition by status and urgency
  function getSectorStyle(status, urgencyLevel) {
    let baseStyle;
    switch (status) {
      case 'active_search':
        baseStyle = {
          color: '#4f46e5',
          fillColor: '#6366f1',
          fillOpacity: 0.5,
          weight: 2,
          className: 'sector-polygon sector-active-search',
        };
        break;
      case 'cleared':
        baseStyle = {
          color: '#16a34a',
          fillColor: '#22c55e',
          fillOpacity: 0.4,
          weight: 2,
          className: 'sector-polygon sector-cleared',
        };
        break;
      case 'sighting_reported':
        baseStyle = {
          color: '#dc2626',
          fillColor: '#ef4444',
          fillOpacity: 0.45,
          weight: 3,
          className: 'sector-polygon sector-sighting-reported sector-pulse',
        };
        break;
      case 'unassigned':
      default:
        baseStyle = {
          color: '#d97706',
          fillColor: '#f59e0b',
          fillOpacity: 0.4,
          weight: 2,
          className: 'sector-polygon sector-unassigned',
        };
        break;
    }

    if (urgencyLevel === 'CRITICAL') {
      baseStyle.color = '#e11d48';
      baseStyle.weight = 3;
      baseStyle.className += ' sector-urgency-critical';
    } else if (urgencyLevel === 'HIGH') {
      baseStyle.color = '#d97706';
      baseStyle.weight = 3;
      baseStyle.className += ' sector-urgency-high';
    }

    return baseStyle;
  }

  // CSRF token retriever
  async function getCsrfToken() {
    let token = window.petspotrIdentity?.getState?.()?.csrfToken;
    if (token) return token;
    const match = document.cookie.match(/(?:^|;\s*)(?:__Host-)?petspotr_csrf(?:_sec)?=([^;]+)/);
    if (match) return decodeURIComponent(match[1]);
    try {
      const res = await fetch('/api/v1/session/csrf', { credentials: 'same-origin', cache: 'no-store' });
      if (res.ok) {
        const data = await res.json();
        return data.csrfToken || '';
      }
    } catch (_) {}
    return '';
  }

  // Display non-blocking toast notification
  function showToast(message, isError = false) {
    let toastContainer = document.getElementById('toast-container');
    if (!toastContainer) {
      toastContainer = document.createElement('div');
      toastContainer.id = 'toast-container';
      toastContainer.className = 'toast-container';
      toastContainer.setAttribute('role', 'region');
      toastContainer.setAttribute('aria-live', 'polite');
      document.body.appendChild(toastContainer);
    }

    const toast = document.createElement('div');
    toast.className = `toast-item ${isError ? 'toast-error' : 'toast-success'}`;
    toast.textContent = message;
    toastContainer.appendChild(toast);

    setTimeout(() => {
      toast.classList.add('toast-fadeout');
      setTimeout(() => {
        if (toast.parentNode) toast.parentNode.removeChild(toast);
      }, 400);
    }, 4500);
  }

  // =========================================================================
  // BLE Collar Beacon Radar HUD, Leaflet Pulse & Sighting Auto-Fill
  // =========================================================================

  // Clear beacon layers from Leaflet map
  function clearBeaconMapLayers() {
    if (searchPartyMapInstance) {
      if (beaconPingMarker) {
        searchPartyMapInstance.removeLayer(beaconPingMarker);
        beaconPingMarker = null;
      }
      if (beaconAccuracyCircle) {
        searchPartyMapInstance.removeLayer(beaconAccuracyCircle);
        beaconAccuracyCircle = null;
      }
    }
  }

  // Reset HUD readouts and hide sighting button
  function resetBeaconHUD() {
    const distEl = document.getElementById('beacon-distance-display');
    if (distEl) distEl.textContent = '--';

    const rssiEl = document.getElementById('beacon-rssi-display');
    if (rssiEl) rssiEl.textContent = '-- dBm';

    const badgeEl = document.getElementById('beacon-proximity-badge');
    if (badgeEl) {
      badgeEl.className = 'badge';
      badgeEl.textContent = 'Out of Range';
    }

    const meter = document.getElementById('beacon-rssi-meter');
    if (meter) {
      meter.dataset.strength = 'out_of_range';
      const fill = meter.querySelector('.meter-fill');
      if (fill) fill.style.width = '0%';
    }

    const btnLog = document.getElementById('btn-log-beacon-sighting');
    if (btnLog) {
      btnLog.hidden = true;
      btnLog.setAttribute('hidden', '');
    }

    const announcer = document.getElementById('beacon-aria-announcer');
    if (announcer) announcer.textContent = '';
  }

  // Determine volunteer coordinates for beacon ping
  function getBeaconObserverCoords() {
    if (currentTrailPoints && currentTrailPoints.length > 0) {
      const last = currentTrailPoints[currentTrailPoints.length - 1];
      return { latitude: last.latitude, longitude: last.longitude };
    }
    if (activeVolunteerMarker) {
      const latLng = activeVolunteerMarker.getLatLng();
      return { latitude: latLng.lat, longitude: latLng.lng };
    }
    if (currentParty?.centerCoordinates && typeof currentParty.centerCoordinates.latitude === 'number') {
      return {
        latitude: currentParty.centerCoordinates.latitude,
        longitude: currentParty.centerCoordinates.longitude,
      };
    }
    return new Promise((resolve) => {
      if (typeof navigator !== 'undefined' && 'geolocation' in navigator) {
        navigator.geolocation.getCurrentPosition(
          (pos) => resolve({ latitude: pos.coords.latitude, longitude: pos.coords.longitude }),
          () => resolve({ latitude: 0, longitude: 0 }),
          { timeout: 3000, maximumAge: 60000 }
        );
      } else {
        resolve({ latitude: 0, longitude: 0 });
      }
    });
  }

  // Draw or update pulsing beacon pin and confidence circle on map
  function renderBeaconPingOnMap(ping) {
    if (!searchPartyMapInstance || !ping || !ping.observerCoords) return;
    const { latitude, longitude } = ping.observerCoords;
    if (typeof latitude !== 'number' || typeof longitude !== 'number' || (latitude === 0 && longitude === 0)) {
      return;
    }

    const latLng = [latitude, longitude];

    // Pulsing glowing Leaflet marker
    if (beaconPingMarker) {
      beaconPingMarker.setLatLng(latLng);
    } else {
      const beaconIcon = L.divIcon({
        className: 'beacon-pulse-indicator',
        html: '<div class="beacon-ping-marker"><div class="beacon-ping-dot"></div></div>',
        iconSize: [28, 28],
        iconAnchor: [14, 14],
      });

      beaconPingMarker = L.marker(latLng, {
        icon: beaconIcon,
        title: 'Beacon Ping Detection',
        zIndexOffset: 1200,
      }).addTo(searchPartyMapInstance);

      beaconPingMarker.bindTooltip(
        `<strong>📡 Collar Beacon Ping</strong><br>Distance: ~${(ping.distanceMeters ?? 0).toFixed(1)}m (${ping.rssi} dBm)`,
        { sticky: true }
      );
    }

    if (beaconPingMarker && ping.distanceMeters != null) {
      beaconPingMarker.setTooltipContent(
        `<strong>📡 Collar Beacon Ping</strong><br>Distance: ~${ping.distanceMeters.toFixed(1)}m (${ping.rssi} dBm)`
      );
    }

    // Confidence / distance circle (emerald green for near/immediate, cyan for far)
    const radiusMeters = (latestTriangulation && typeof latestTriangulation.accuracyRadiusMeters === 'number' && latestTriangulation.accuracyRadiusMeters > 0)
      ? latestTriangulation.accuracyRadiusMeters
      : Math.max(1, ping.distanceMeters || 5);

    const circleColor = (ping.proximity === 'immediate' || ping.proximity === 'near') ? '#10b981' : '#06b6d4';

    if (beaconAccuracyCircle) {
      beaconAccuracyCircle.setLatLng(latLng);
      beaconAccuracyCircle.setRadius(radiusMeters);
      beaconAccuracyCircle.setStyle({
        color: circleColor,
        fillColor: circleColor,
      });
    } else {
      beaconAccuracyCircle = L.circle(latLng, {
        radius: radiusMeters,
        color: circleColor,
        fillColor: circleColor,
        fillOpacity: 0.2,
        weight: 2,
        dashArray: '4, 6',
        className: 'beacon-confidence-circle',
      }).addTo(searchPartyMapInstance);
    }
  }

  // Update map when server triangulation result arrives
  function handleTriangulationUpdate(triangulation) {
    if (!triangulation) return;
    latestTriangulation = triangulation;

    if (triangulation.estimatedCoordinates && typeof triangulation.estimatedCoordinates.latitude === 'number') {
      const estCoords = triangulation.estimatedCoordinates;
      if (estCoords.latitude !== 0 || estCoords.longitude !== 0) {
        const radius = (typeof triangulation.accuracyRadiusMeters === 'number' && triangulation.accuracyRadiusMeters > 0)
          ? triangulation.accuracyRadiusMeters
          : 10;
        if (beaconAccuracyCircle) {
          beaconAccuracyCircle.setLatLng([estCoords.latitude, estCoords.longitude]);
          beaconAccuracyCircle.setRadius(radius);
        } else if (searchPartyMapInstance) {
          beaconAccuracyCircle = L.circle([estCoords.latitude, estCoords.longitude], {
            radius: radius,
            color: '#10b981',
            fillColor: '#10b981',
            fillOpacity: 0.25,
            weight: 2,
            dashArray: '4, 6',
            className: 'beacon-confidence-circle',
          }).addTo(searchPartyMapInstance);
        }
      }
    }
  }

  // Transmit ping to REST backend or queue offline
  async function transmitBeaconPing(ping) {
    const petId = currentPetID || currentParty?.lostPetId;
    if (!petId) return;

    const payload = {
      volunteerAlias: ping.volunteerAlias || getVolunteerAlias() || 'Volunteer Alpha',
      observerCoords: ping.observerCoords,
      rssi: ping.rssi,
      txPower1m: ping.txPower1m,
      recordedAt: ping.recordedAt,
    };

    if (typeof navigator !== 'undefined' && !navigator.onLine) {
      if (window.PetSpotROutbox && typeof window.PetSpotROutbox.queueBeaconPing === 'function') {
        await window.PetSpotROutbox.queueBeaconPing(petId, payload);
      }
      return;
    }

    try {
      const csrf = await getCsrfToken();
      const headers = { 'Content-Type': 'application/json' };
      if (csrf) headers['X-CSRF-Token'] = csrf;

      const res = await fetch(`/api/v1/search-parties/${encodeURIComponent(petId)}/beacon-pings`, {
        method: 'POST',
        headers,
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        const respData = await res.json();
        if (respData.triangulation) {
          handleTriangulationUpdate(respData.triangulation);
        }
      } else if (res.status >= 500 || res.status === 0) {
        if (window.PetSpotROutbox && typeof window.PetSpotROutbox.queueBeaconPing === 'function') {
          await window.PetSpotROutbox.queueBeaconPing(petId, payload);
        }
      }
    } catch (err) {
      console.warn('Error transmitting beacon ping, queuing offline:', err);
      if (window.PetSpotROutbox && typeof window.PetSpotROutbox.queueBeaconPing === 'function') {
        await window.PetSpotROutbox.queueBeaconPing(petId, payload);
      }
    }
  }

  // Handle ping callback from PetBeaconScanner
  async function handleBeaconPing(ping) {
    if (!ping) return;
    latestBeaconPing = ping;

    // 1. Distance display
    const distEl = document.getElementById('beacon-distance-display');
    if (distEl) {
      const dist = typeof ping.distanceMeters === 'number' ? ping.distanceMeters.toFixed(1) : '--';
      distEl.textContent = `~${dist}m (${ping.proximity || 'unknown'})`;
    }

    // 2. RSSI display
    const rssiEl = document.getElementById('beacon-rssi-display');
    if (rssiEl) {
      rssiEl.textContent = `${ping.rssi} dBm`;
    }

    // 3. Proximity badge
    const badgeEl = document.getElementById('beacon-proximity-badge');
    if (badgeEl) {
      let badgeClass = 'badge';
      let badgeText = 'Out of Range';
      switch (ping.proximity) {
        case 'immediate':
          badgeClass = 'badge badge-success';
          badgeText = 'Immediate (<1m)';
          break;
        case 'near':
          badgeClass = 'badge badge-primary';
          badgeText = 'Near (<5m)';
          break;
        case 'far':
          badgeClass = 'badge badge-warning';
          badgeText = 'Far (<30m)';
          break;
        default:
          badgeClass = 'badge';
          badgeText = 'Out of Range';
      }
      badgeEl.className = badgeClass;
      badgeEl.textContent = badgeText;
    }

    // 4. RSSI meter fill (0% at -100 dBm to 100% at -50 dBm)
    const meter = document.getElementById('beacon-rssi-meter');
    if (meter) {
      meter.dataset.strength = ping.proximity || 'out_of_range';
      const fill = meter.querySelector('.meter-fill');
      if (fill) {
        const clampedRssi = Math.max(-100, Math.min(ping.rssi, -50));
        const percent = Math.round(((clampedRssi - (-100)) / ((-50) - (-100))) * 100);
        fill.style.width = `${percent}%`;
      }
    }

    // 5. ARIA announcement
    const announcer = document.getElementById('beacon-aria-announcer');
    if (announcer) {
      const dist = typeof ping.distanceMeters === 'number' ? ping.distanceMeters.toFixed(1) : '';
      announcer.textContent = `Collar beacon detected ${dist} meters away`;
    }

    // 6. Sighting button unhide when near or immediate (<5m)
    const btnLog = document.getElementById('btn-log-beacon-sighting');
    if (btnLog) {
      if (ping.proximity === 'near' || ping.proximity === 'immediate' || (typeof ping.distanceMeters === 'number' && ping.distanceMeters < 5.0)) {
        btnLog.hidden = false;
        btnLog.removeAttribute('hidden');
      }
    }

    // 7. Leaflet map rendering
    renderBeaconPingOnMap(ping);

    // 8. Backend transmission / offline queue
    await transmitBeaconPing(ping);
  }

  // Handle scanner status changes
  function handleBeaconStatusChange(status, detail) {
    const statusEl = document.getElementById('beacon-scanner-status');
    const btnScan = document.getElementById('btn-start-beacon-scan');

    if (statusEl && detail) {
      statusEl.textContent = detail;
    }

    if (btnScan) {
      if (status === 'scanning') {
        btnScan.textContent = '🛑 Stop Beacon Scan';
      } else if (status === 'stopped' || status === 'unsupported' || status === 'error') {
        btnScan.textContent = '📡 Start Beacon Scan';
      }
    }
  }

  // Handle Log Sighting button click
  function handleLogBeaconSightingClick() {
    const petId = currentPetID || currentParty?.lostPetId || '';
    const petName = currentParty?.lostPetName || 'Lost Pet';

    let lat = 47.6062;
    let lng = -122.3321;

    if (latestTriangulation?.estimatedCoordinates &&
        (latestTriangulation.estimatedCoordinates.latitude !== 0 || latestTriangulation.estimatedCoordinates.longitude !== 0)) {
      lat = latestTriangulation.estimatedCoordinates.latitude;
      lng = latestTriangulation.estimatedCoordinates.longitude;
    } else if (latestBeaconPing?.observerCoords &&
               (latestBeaconPing.observerCoords.latitude !== 0 || latestBeaconPing.observerCoords.longitude !== 0)) {
      lat = latestBeaconPing.observerCoords.latitude;
      lng = latestBeaconPing.observerCoords.longitude;
    } else if (currentParty?.centerCoordinates &&
               (currentParty.centerCoordinates.latitude !== 0 || currentParty.centerCoordinates.longitude !== 0)) {
      lat = currentParty.centerCoordinates.latitude;
      lng = currentParty.centerCoordinates.longitude;
    }

    const dist = latestBeaconPing?.distanceMeters != null ? latestBeaconPing.distanceMeters.toFixed(1) : '3.2';
    const rssi = latestBeaconPing?.rssi != null ? latestBeaconPing.rssi : '-64';
    const notesText = `Collar beacon detected nearby (~${dist}m, RSSI: ${rssi} dBm)`;

    // If PetSpotRSightingTrajectory helper exists, call it first
    if (window.PetSpotRSightingTrajectory && typeof window.PetSpotRSightingTrajectory.openSightingModal === 'function') {
      window.PetSpotRSightingTrajectory.openSightingModal(petId, petName, lat, lng);
    }

    // Pre-populate sighting modal fields
    const sightingModal = document.getElementById('modal-report-sighting');
    if (sightingModal) {
      sightingModal.classList.remove('hidden');

      const petIdInput = document.getElementById('sighting-pet-id');
      if (petIdInput) petIdInput.value = petId;

      const latInput = document.getElementById('sighting-lat');
      if (latInput) latInput.value = lat;

      const lngInput = document.getElementById('sighting-lng');
      if (lngInput) lngInput.value = lng;

      const locInput = document.getElementById('sighting-location');
      if (locInput) locInput.value = notesText;

      const notesInput = document.getElementById('sighting-notes');
      if (notesInput) notesInput.value = notesText;

      const gpsStatus = document.getElementById('sighting-gps-status');
      if (gpsStatus) gpsStatus.textContent = `📍 Beacon detection (${lat.toFixed(4)}, ${lng.toFixed(4)})`;

      if (locInput) {
        locInput.focus();
      }
    }
  }

  // Wire buttons in Radar HUD (initial text & attributes; click events handled via delegated listener)
  function setupBeaconHUDControls() {
    const btnScan = document.getElementById('btn-start-beacon-scan');
    const btnAudio = document.getElementById('btn-toggle-beacon-audio');
    const btnLog = document.getElementById('btn-log-beacon-sighting');
    const statusText = document.getElementById('beacon-scanner-status');

    if (statusText) {
      statusText.textContent = 'Scanner Ready';
    }

    if (btnScan) {
      btnScan.textContent = '📡 Start Beacon Scan';
    }

    if (btnAudio && beaconScannerInstance) {
      const isMuted = beaconScannerInstance.isAudioMuted;
      btnAudio.textContent = isMuted ? '🔇 Audio Ping: Muted' : '🔊 Audio Ping: Active';
      btnAudio.setAttribute('aria-pressed', isMuted ? 'false' : 'true');
    }

    if (btnLog) {
      btnLog.hidden = true;
      btnLog.setAttribute('hidden', '');
    }
  }

  // Initialize or teardown beacon scanner based on collarBeaconConfig presence
  function initBeaconScanner(party) {
    const container = document.getElementById('beacon-scanner-container');
    if (!container) return;

    const beaconConfig = party?.collarBeacon || party?.collarBeaconConfig || party?.lostPet?.collarBeaconConfig || null;

    if (beaconScannerInstance) {
      beaconScannerInstance.destroy?.();
      beaconScannerInstance = null;
    }
    latestBeaconPing = null;
    latestTriangulation = null;
    clearBeaconMapLayers();
    resetBeaconHUD();

    if (!beaconConfig) {
      container.hidden = true;
      return;
    }

    container.hidden = false;

    const petId = party?.lostPetId || currentPetID;
    const volunteerAlias = getVolunteerAlias() || 'Volunteer Alpha';

    if (typeof PetBeaconScanner === 'undefined') {
      console.warn('PetBeaconScanner is not defined.');
      return;
    }

    beaconScannerInstance = new PetBeaconScanner({
      petId: petId,
      beaconConfig: beaconConfig,
      volunteerAlias: volunteerAlias,
      onPing: handleBeaconPing,
      onTriangulationUpdate: handleTriangulationUpdate,
      onStatusChange: handleBeaconStatusChange,
      getObserverCoords: getBeaconObserverCoords,
    });

    setupBeaconHUDControls();
  }

  // Open Search Party Modal
  function openSearchPartyModal(petId, petName) {
    const modal = document.getElementById('pet-search-party-container');
    if (!modal) return;

    currentPetID = petId || '';
    latestBeaconPing = null;
    latestTriangulation = null;
    clearBeaconMapLayers();
    resetBeaconHUD();

    sortSectorsByUrgency = false;
    const sortBtn = document.getElementById('btn-sort-sectors-urgency');
    if (sortBtn) {
      sortBtn.setAttribute('aria-pressed', 'false');
      sortBtn.classList.remove('active');
    }

    const titleEl = document.getElementById('search-party-modal-title');
    const badgeEl = document.getElementById('search-party-pet-badge');

    if (titleEl) {
      titleEl.textContent = petName ? `🚩 Search Party: ${petName}` : '🚩 Search Party Sector Grid';
    }
    if (badgeEl) {
      badgeEl.textContent = petName || 'Lost Pet';
    }

    modal.classList.remove('hidden');
    void loadAndRenderSearchParty(petId, petName);
    connectSearchPartySSE(petId);
  }

  // Close Search Party Modal
  function closeSearchPartyModal() {
    stopRecordingTrail();
    if (beaconScannerInstance) {
      beaconScannerInstance.stopScan();
    }
    clearBeaconMapLayers();
    activeBarriers = [];
    closeMeshModal();
    const modal = document.getElementById('pet-search-party-container');
    if (modal) {
      modal.classList.add('hidden');
    }
    if (activeEventSource) {
      activeEventSource.close();
      activeEventSource = null;
    }
  }

  // Fetch or initialize active search party
  async function loadAndRenderSearchParty(petId, petName) {
    latestBeaconPing = null;
    latestTriangulation = null;
    clearBeaconMapLayers();
    resetBeaconHUD();

    // Fetch trajectory barriers asynchronously for physical barrier intersection checks
    fetch(`/api/v1/lost-pets/${encodeURIComponent(petId)}/trajectory`)
      .then((r) => (r.ok ? r.json() : null))
      .then((trajData) => {
        if (trajData?.predictiveModel?.barriersEncountered) {
          activeBarriers = trajData.predictiveModel.barriersEncountered;
        }
      })
      .catch(() => {});

    const statusOverlay = document.getElementById('search-party-map-status');
    if (statusOverlay) {
      statusOverlay.textContent = 'Loading search party sectors...';
      statusOverlay.classList.add('visible');
    }

    try {
      let res = await fetch(`/api/v1/lost-pets/${encodeURIComponent(petId)}/search-party`, {
        cache: 'no-store',
      });

      // If not yet created, auto-initialize search party
      if (res.status === 404) {
        const csrf = await getCsrfToken();
        const headers = { 'Content-Type': 'application/json' };
        if (csrf) headers['X-CSRF-Token'] = csrf;

        res = await fetch(`/api/v1/lost-pets/${encodeURIComponent(petId)}/search-party`, {
          method: 'POST',
          headers,
          body: JSON.stringify({ sectorCount: 4 }),
        });
      }

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `Failed to load search party (status ${res.status})`);
      }

      const party = await res.json();
      currentParty = party;

      if (statusOverlay) {
        statusOverlay.classList.remove('visible');
      }

      updateSummaryUI(party);
      renderSearchPartyMap(party);
      renderSectorCards(party);
      initBeaconScanner(party);
      void loadAllSectorTrails(party);
      void updateOfflineBadge();
      updateMeshUI();
    } catch (err) {
      console.error('Error loading search party:', err);
      if (statusOverlay) {
        statusOverlay.textContent = `⚠️ ${err.message || 'Unable to load search party'}`;
        statusOverlay.classList.add('visible');
      }
    }
  }

  // Update coverage bar, volunteer counter, and totals
  function updateSummaryUI(party) {
    if (!party) return;

    const volCountEl = document.getElementById('search-party-volunteers-count');
    if (volCountEl) {
      volCountEl.textContent = String(party.activeVolunteersCount ?? 0);
    }

    const secCountEl = document.getElementById('search-party-sectors-count');
    if (secCountEl) {
      secCountEl.textContent = String(party.sectors ? party.sectors.length : 0);
    }

    const coverage = typeof party.coveragePercentage === 'number' ? party.coveragePercentage : 0;
    const roundedCoverage = Math.round(coverage);

    const coverageLabelEl = document.getElementById('search-party-coverage-label');
    if (coverageLabelEl) {
      coverageLabelEl.textContent = `${roundedCoverage}%`;
    }

    const coverageBars = document.querySelectorAll('.search-party-coverage-bar, #search-party-coverage-bar');
    coverageBars.forEach((bar) => {
      const clamped = Math.min(100, Math.max(0, coverage));
      bar.style.width = `${clamped}%`;
      bar.setAttribute('aria-valuenow', String(roundedCoverage));
    });
  }

  // Render Leaflet map with colored sector polygons
  function renderSearchPartyMap(party) {
    const mapElement = document.getElementById('search-party-map');
    if (!mapElement || typeof L === 'undefined') return;

    // Reset map instance cleanly
    if (searchPartyMapInstance) {
      searchPartyMapInstance.remove();
      searchPartyMapInstance = null;
    }
    // Clear sector layer cache
    for (const key in sectorLayers) {
      delete sectorLayers[key];
    }
    for (const key in trailLayers) {
      delete trailLayers[key];
    }
    activeVolunteerMarker = null;
    beaconPingMarker = null;
    beaconAccuracyCircle = null;

    searchPartyMapInstance = L.map(mapElement, {
      scrollWheelZoom: false,
    });

    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap contributors',
    }).addTo(searchPartyMapInstance);

    const allPoints = [];

    // Add Center Marker
    if (party.centerCoordinates && typeof party.centerCoordinates.latitude === 'number') {
      const centerPt = [party.centerCoordinates.latitude, party.centerCoordinates.longitude];
      allPoints.push(centerPt);

      const centerIcon = L.divIcon({
        className: 'sp-center-icon-wrapper',
        html: '<div class="sp-center-marker" title="Search Party Center">🐾</div>',
        iconSize: [28, 28],
        iconAnchor: [14, 14],
      });
      L.marker(centerPt, { icon: centerIcon, title: 'Search Party Center' })
        .addTo(searchPartyMapInstance)
        .bindTooltip('<strong>Search Origin / Last Known</strong>');
    }

    // Render Sector Polygons
    if (Array.isArray(party.sectors)) {
      party.sectors.forEach((sector) => {
        if (!Array.isArray(sector.polygonPoints) || sector.polygonPoints.length < 3) return;

        const latLngs = sector.polygonPoints.map((pt) => {
          allPoints.push([pt.latitude, pt.longitude]);
          return [pt.latitude, pt.longitude];
        });

        const style = getSectorStyle(sector.status, sector.urgencyLevel);
        const polygon = L.polygon(latLngs, style).addTo(searchPartyMapInstance);
        sectorLayers[sector.sectorId] = polygon;
        const pathEl = polygon.getElement();
        if (pathEl) {
          pathEl.setAttribute('data-sector-id', sector.sectorId);
          if (sector.status === 'cleared') {
            pathEl.classList.add('sector-cleared');
          }
        }

        // Tooltip
        const urgencyLabel = sector.urgencyLevel === 'CRITICAL' ? ' [🔥 Critical]' : (sector.urgencyLevel === 'HIGH' ? ' [⚡ High Priority]' : '');
        polygon.bindTooltip(
          `<strong>${escapeHTML(sector.name)}${urgencyLabel}</strong><br>Status: ${formatSectorStatus(sector.status)}<br><em>Click to claim or update</em>`,
          { sticky: true }
        );

        // Click handler to trigger claim or clear modal
        polygon.on('click', () => {
          handleSectorClick(sector.sectorId);
        });
      });
    }

    // Fit map bounds
    if (allPoints.length > 0) {
      searchPartyMapInstance.fitBounds(L.latLngBounds(allPoints), { padding: [30, 30] });
    } else if (party.centerCoordinates) {
      searchPartyMapInstance.setView([party.centerCoordinates.latitude, party.centerCoordinates.longitude], 14);
    }

    // Re-render beacon ping or triangulation on newly initialized map
    if (latestBeaconPing) {
      renderBeaconPingOnMap(latestBeaconPing);
    }
    if (latestTriangulation) {
      handleTriangulationUpdate(latestTriangulation);
    }

    // Refresh size on modal display
    setTimeout(() => {
      if (searchPartyMapInstance) {
        searchPartyMapInstance.invalidateSize();
      }
    }, 150);
  }

  // Point in sector polygon geometry test
  function pointInSectorPolygon(pt, poly) {
    if (!poly || poly.length < 3) return false;
    let inside = false;
    for (let i = 0, j = poly.length - 1; i < poly.length; j = i++) {
      const xi = poly[i].longitude ?? poly[i].lng ?? poly[i][1];
      const yi = poly[i].latitude ?? poly[i].lat ?? poly[i][0];
      const xj = poly[j].longitude ?? poly[j].lng ?? poly[j][1];
      const yj = poly[j].latitude ?? poly[j].lat ?? poly[j][0];
      const intersect = ((yi > pt.lat) !== (yj > pt.lat)) &&
        (pt.lng < (xj - xi) * (pt.lat - yi) / (yj - yi) + xi);
      if (intersect) inside = !inside;
    }
    return inside;
  }

  // Check if sector intersects an impassable barrier, hiding cluster, or has terrain advisory
  function sectorHasBarrierAdvisory(sector) {
    if (!sector) return false;
    if (sector.hasBarrier === true) return true;
    if (Array.isArray(sector.barriers) && sector.barriers.length > 0) return true;
    if (Array.isArray(sector.impassableBarriers) && sector.impassableBarriers.length > 0) return true;
    if (Array.isArray(sector.terrainFeatures) && sector.terrainFeatures.length > 0) return true;
    if (typeof sector.terrainAdvisory === 'string' && sector.terrainAdvisory.trim() !== '') return true;
    if (sector.hasTerrainBarrier === true) return true;
    if (typeof sector.notes === 'string' && /barrier|slope|freeway|highway|water|traffic|roadway|steep/i.test(sector.notes)) return true;
    if (Array.isArray(sector.hidingClusterIds) && sector.hidingClusterIds.length > 0) return true;

    // Geometric intersection check against loaded active barriers
    if (Array.isArray(activeBarriers) && activeBarriers.length > 0 && Array.isArray(sector.polygonPoints) && sector.polygonPoints.length >= 3) {
      for (const b of activeBarriers) {
        const isHazard = b.type === 'freeway' || b.type === 'waterway' || b.type === 'steep_slope' || (b.frictionCost && b.frictionCost >= 2.0);
        if (!isHazard) continue;
        if (Array.isArray(b.geometry)) {
          for (const coord of b.geometry) {
            const lat = typeof coord?.latitude === 'number' ? coord.latitude : (Array.isArray(coord) ? coord[0] : coord?.lat);
            const lng = typeof coord?.longitude === 'number' ? coord.longitude : (Array.isArray(coord) ? coord[1] : coord?.lng);
            if (typeof lat === 'number' && typeof lng === 'number') {
              if (pointInSectorPolygon({ lat, lng }, sector.polygonPoints)) {
                return true;
              }
            }
          }
        }
      }
    }
    return false;
  }

  // Update sector barrier safety advisory display
  function updateBarrierAdvisory(sector) {
    const advisoryEl = document.getElementById('sector-barrier-advisory');
    if (!advisoryEl) return;
    if (sector && sectorHasBarrierAdvisory(sector)) {
      advisoryEl.textContent = '⚠️ Safety Advisory: Sector intersects physical terrain barrier. Caution advised near steep slopes or roadways.';
      advisoryEl.classList.remove('hidden');
    } else {
      advisoryEl.textContent = '';
      advisoryEl.classList.add('hidden');
    }
  }

  // Toggle sorting sectors by priority score / urgency
  function toggleSortSectorsUrgency() {
    sortSectorsByUrgency = !sortSectorsByUrgency;
    const sortBtn = document.getElementById('btn-sort-sectors-urgency');
    if (sortBtn) {
      sortBtn.setAttribute('aria-pressed', sortSectorsByUrgency ? 'true' : 'false');
      sortBtn.classList.toggle('active', sortSectorsByUrgency);
    }
    if (currentParty) {
      renderSectorCards(currentParty);
    }
  }

  // Handle sector click from map polygon or list card
  function handleSectorClick(sectorId) {
    if (!currentParty || !Array.isArray(currentParty.sectors)) return;
    const sector = currentParty.sectors.find((s) => s.sectorId === sectorId);
    if (!sector) return;

    selectedSectorId = sectorId;
    const secLabel = document.getElementById('trail-active-sector-label');
    if (secLabel) {
      secLabel.textContent = sector.name;
    }

    updateBarrierAdvisory(sector);

    if (sector.status === 'unassigned') {
      openClaimModal(sector);
    } else {
      openClearModal(sector);
    }
  }

  // Open Claim Sector Modal
  function openClaimModal(sector) {
    const modal = document.getElementById('modal-claim-sector');
    if (!modal) return;

    const partyIdInput = document.getElementById('claim-party-id');
    const sectorIdInput = document.getElementById('claim-sector-id');
    const petIdInput = document.getElementById('claim-pet-id');
    const nameEl = document.getElementById('claim-sector-name');
    const detailsEl = document.getElementById('claim-sector-details');
    const feedbackEl = document.getElementById('claim-modal-feedback');

    if (partyIdInput) partyIdInput.value = currentParty?.partyId || '';
    if (sectorIdInput) sectorIdInput.value = sector.sectorId;
    if (petIdInput) petIdInput.value = currentParty?.lostPetId || '';
    if (nameEl) nameEl.textContent = sector.name;

    const areaSqMi = sector.totalAreaSqM ? (sector.totalAreaSqM / 2589988.11).toFixed(2) : '—';
    const priority = typeof sector.priorityScore === 'number' ? sector.priorityScore.toFixed(1) : '—';
    if (detailsEl) {
      detailsEl.textContent = `Area: ${areaSqMi} sq mi · Priority: ${priority} · Status: Unassigned`;
    }

    updateBarrierAdvisory(sector);

    if (feedbackEl) {
      feedbackEl.textContent = '';
      feedbackEl.classList.add('hidden');
    }

    modal.classList.remove('hidden');
    const aliasInput = document.getElementById('claim-volunteer-alias');
    if (aliasInput) {
      aliasInput.focus();
    }
  }

  // Close Claim Sector Modal
  function closeClaimModal() {
    const modal = document.getElementById('modal-claim-sector');
    if (modal) {
      modal.classList.add('hidden');
    }
    updateBarrierAdvisory(null);
  }

  // Open Clear / Status Update Modal
  function openClearModal(sector) {
    const modal = document.getElementById('modal-clear-sector');
    if (!modal) return;

    const partyIdInput = document.getElementById('clear-party-id');
    const sectorIdInput = document.getElementById('clear-sector-id');
    const petIdInput = document.getElementById('clear-pet-id');
    const nameEl = document.getElementById('clear-sector-name');
    const detailsEl = document.getElementById('clear-sector-details');
    const statusSelect = document.getElementById('clear-sector-status');
    const notesInput = document.getElementById('clear-sector-notes');
    const feedbackEl = document.getElementById('clear-modal-feedback');

    if (partyIdInput) partyIdInput.value = currentParty?.partyId || '';
    if (sectorIdInput) sectorIdInput.value = sector.sectorId;
    if (petIdInput) petIdInput.value = currentParty?.lostPetId || '';
    if (nameEl) nameEl.textContent = sector.name;

    const asgn = currentParty?.activeAssignments?.find((a) => a.sectorId === sector.sectorId);
    const volunteerName = asgn ? asgn.volunteerAlias : 'None';
    if (detailsEl) {
      detailsEl.textContent = `Current Status: ${formatSectorStatus(sector.status)} · Volunteer: ${escapeHTML(volunteerName)}`;
    }

    if (statusSelect) {
      statusSelect.value = sector.status || 'active_search';
    }
    if (notesInput) {
      notesInput.value = '';
    }
    if (feedbackEl) {
      feedbackEl.textContent = '';
      feedbackEl.classList.add('hidden');
    }

    modal.classList.remove('hidden');
    if (statusSelect) {
      statusSelect.focus();
    }
  }

  // Close Clear / Status Update Modal
  function closeClearModal() {
    const modal = document.getElementById('modal-clear-sector');
    if (modal) {
      modal.classList.add('hidden');
    }
  }

  // Render sector cards list in the legend/summary section
  function renderSectorCards(party) {
    const container = document.getElementById('search-party-sector-cards');
    if (!container) return;

    container.innerHTML = '';
    if (!party || !Array.isArray(party.sectors)) return;

    let sectors = party.sectors.slice();
    if (sortSectorsByUrgency) {
      sectors.sort((a, b) => {
        const scoreA = typeof a.priorityScore === 'number' ? a.priorityScore : 0;
        const scoreB = typeof b.priorityScore === 'number' ? b.priorityScore : 0;
        return scoreB - scoreA;
      });
    }

    sectors.forEach((sec) => {
      const card = document.createElement('div');
      card.className = `sector-card sector-card-${sec.status}`;
      card.dataset.sectorId = sec.sectorId;

      const asgn = party.activeAssignments?.find((a) => a.sectorId === sec.sectorId);
      const volunteerText = asgn ? escapeHTML(asgn.volunteerAlias) : 'No volunteer assigned';
      const isUnassigned = sec.status === 'unassigned';
      const btnAction = isUnassigned ? 'claim-sector' : 'clear-sector';
      const btnLabel = isUnassigned ? 'Claim Sector' : 'Update Status';
      const btnClass = isUnassigned ? 'btn-primary' : 'btn-secondary';

      let urgencyBadgeHtml = '';
      if (sec.urgencyLevel === 'CRITICAL') {
        urgencyBadgeHtml = '<span class="badge badge-critical" data-testid="sector-urgency-critical">🔥 Critical Search Zone</span>';
      } else if (sec.urgencyLevel === 'HIGH') {
        urgencyBadgeHtml = '<span class="badge badge-high" data-testid="sector-urgency-high">⚡ High Priority</span>';
      }

      card.innerHTML = `
        <div class="sector-card-header">
          <div class="sector-card-title-group">
            <h4 class="sector-card-name">${escapeHTML(sec.name)}</h4>
            ${urgencyBadgeHtml}
          </div>
          <span class="badge ${sec.meshVerified ? 'badge-status-cleared badge-mesh-cleared' : `badge-status-${sec.status}`}">${sec.meshVerified ? 'CLEARED (Mesh Verified)' : formatSectorStatus(sec.status)}</span>
        </div>
        <p class="sector-card-volunteer text-secondary">👤 ${volunteerText}</p>
        <div class="sector-card-trail-info">
          <span class="badge badge-trail" id="sector-trail-badge-${sec.sectorId}">👣 0 trails</span>
        </div>
        <div class="sector-card-actions">
          <button type="button" class="btn ${btnClass} btn-sm btn-sector-action" data-action="${btnAction}" data-sector-id="${sec.sectorId}">
            ${btnLabel}
          </button>
        </div>
      `;
      container.appendChild(card);
      updateSectorTrailBadge(sec.sectorId);
    });
  }

  // Dynamically update polygon and UI state for a single sector
  function applySectorUpdate(sectorId, newStatus, coverage, volunteerCount) {
    if (!currentParty) return;

    if (Array.isArray(currentParty.sectors)) {
      const sec = currentParty.sectors.find((s) => s.sectorId === sectorId);
      if (sec) {
        sec.status = newStatus;
      }
    }

    if (typeof coverage === 'number') {
      currentParty.coveragePercentage = coverage;
    }
    if (typeof volunteerCount === 'number') {
      currentParty.activeVolunteersCount = volunteerCount;
    }

    // Update Polygon on Leaflet map
    const poly = sectorLayers[sectorId];
    if (poly) {
      const sec = currentParty?.sectors ? currentParty.sectors.find((s) => s.sectorId === sectorId) : null;
      const newStyle = getSectorStyle(newStatus, sec?.urgencyLevel);
      poly.setStyle(newStyle);

      // Handle pulsing animation and cleared classes on SVG path element
      const pathEl = poly.getElement();
      if (pathEl) {
        pathEl.setAttribute('data-sector-id', sectorId);
        if (newStatus === 'cleared') {
          pathEl.classList.add('sector-cleared');
        } else {
          pathEl.classList.remove('sector-cleared');
        }
        if (newStatus === 'sighting_reported') {
          pathEl.classList.add('sector-pulse');
        } else {
          pathEl.classList.remove('sector-pulse');
        }
      }

      if (currentParty.sectors) {
        if (sec) {
          const urgencyLabel = sec.urgencyLevel === 'CRITICAL' ? ' [🔥 Critical]' : (sec.urgencyLevel === 'HIGH' ? ' [⚡ High Priority]' : '');
          poly.setTooltipContent(
            `<strong>${escapeHTML(sec.name)}${urgencyLabel}</strong><br>Status: ${formatSectorStatus(sec.status)}<br><em>Click to claim or update</em>`
          );
        }
      }
    }

    updateSummaryUI(currentParty);
    renderSectorCards(currentParty);
  }

  // Handle Claim Sector Form Submission
  async function handleClaimSubmit(e) {
    e.preventDefault();
    const form = e.target;
    const partyId = form.partyId?.value;
    const sectorId = form.sectorId?.value;
    const aliasInput = form.volunteerAlias;
    const feedbackEl = document.getElementById('claim-modal-feedback');
    const submitBtn = document.getElementById('btn-submit-claim');

    if (!partyId || !sectorId) {
      if (feedbackEl) {
        feedbackEl.textContent = 'Missing search party or sector ID';
        feedbackEl.className = 'form-feedback form-feedback-error';
        feedbackEl.classList.remove('hidden');
      }
      return;
    }

    if (submitBtn) submitBtn.disabled = true;

    try {
      const csrf = await getCsrfToken();
      const headers = { 'Content-Type': 'application/json' };
      if (csrf) headers['X-CSRF-Token'] = csrf;

      const alias = aliasInput ? aliasInput.value.trim() : '';
      const payload = {
        volunteerAlias: alias,
      };

      const res = await fetch(
        `/api/v1/search-parties/${encodeURIComponent(partyId)}/sectors/${encodeURIComponent(sectorId)}/claim`,
        {
          method: 'POST',
          headers,
          body: JSON.stringify(payload),
        }
      );

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `Claim failed (${res.status})`);
      }

      const data = await res.json();
      closeClaimModal();
      showToast('Sector claimed successfully! Search status is now active.');
      if (window.petspotrAnnounce) {
        window.petspotrAnnounce('Search party updated: sector claimed');
      }
      if (window.petSpotRMesh?.claimSector) {
        window.petSpotRMesh.claimSector(currentPetID || partyId, sectorId, 'CLAIMED', {
          volunteerName: alias || data.volunteerAlias,
        }).catch(() => {});
      }

      if (currentParty) {
        if (!Array.isArray(currentParty.activeAssignments)) {
          currentParty.activeAssignments = [];
        }
        currentParty.activeAssignments = currentParty.activeAssignments.filter((a) => a.sectorId !== sectorId);
        currentParty.activeAssignments.push({
          assignmentId: data.assignmentId,
          sectorId: sectorId,
          volunteerAlias: data.volunteerAlias || alias,
          status: 'active_search',
          claimedAt: new Date().toISOString(),
        });
      }

      applySectorUpdate(sectorId, 'active_search', data.coveragePercentage, data.activeVolunteersCount);
    } catch (err) {
      console.error('Claim sector error:', err);
      if (feedbackEl) {
        feedbackEl.textContent = err.message || 'Failed to claim sector';
        feedbackEl.className = 'form-feedback form-feedback-error';
        feedbackEl.classList.remove('hidden');
      }
    } finally {
      if (submitBtn) submitBtn.disabled = false;
    }
  }

  // Handle Clear / Status Update Form Submission
  async function handleClearSubmit(e) {
    e.preventDefault();
    const form = e.target;
    const partyId = form.partyId?.value;
    const sectorId = form.sectorId?.value;
    const statusSelect = form.status;
    const notesInput = form.clearanceNotes;
    const feedbackEl = document.getElementById('clear-modal-feedback');
    const submitBtn = document.getElementById('btn-submit-clear');

    if (!partyId || !sectorId || !statusSelect) {
      if (feedbackEl) {
        feedbackEl.textContent = 'Missing required fields';
        feedbackEl.className = 'form-feedback form-feedback-error';
        feedbackEl.classList.remove('hidden');
      }
      return;
    }

    if (submitBtn) submitBtn.disabled = true;

    try {
      const csrf = await getCsrfToken();
      const headers = { 'Content-Type': 'application/json' };
      if (csrf) headers['X-CSRF-Token'] = csrf;

      const payload = {
        status: statusSelect.value,
        clearanceNotes: notesInput ? notesInput.value.trim() : '',
      };

      const res = await fetch(
        `/api/v1/search-parties/${encodeURIComponent(partyId)}/sectors/${encodeURIComponent(sectorId)}/status`,
        {
          method: 'POST',
          headers,
          body: JSON.stringify(payload),
        }
      );

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `Update failed (${res.status})`);
      }

      const data = await res.json();
      closeClearModal();
      showToast('Sector status updated successfully.');

      applySectorUpdate(sectorId, payload.status, data.coveragePercentage, data.activeVolunteersCount);
    } catch (err) {
      console.error('Update sector status error:', err);
      if (feedbackEl) {
        feedbackEl.textContent = err.message || 'Failed to update sector status';
        feedbackEl.className = 'form-feedback form-feedback-error';
        feedbackEl.classList.remove('hidden');
      }
    } finally {
      if (submitBtn) submitBtn.disabled = false;
    }
  }

  // Draw or update a polyline on the Leaflet map for a breadcrumb trail
  function drawTrailPolyline(trail) {
    if (!searchPartyMapInstance || !trail || !Array.isArray(trail.points) || trail.points.length < 2) return;

    const latLngs = trail.points.map((pt) => [pt.latitude, pt.longitude]);
    const trailId = trail.trailId;

    if (trailLayers[trailId]) {
      trailLayers[trailId].setLatLngs(latLngs);
    } else {
      const color = getTrailColor(trail.volunteerAlias || trailId);
      const poly = L.polyline(latLngs, {
        color: color,
        weight: 4,
        opacity: 0.85,
        smoothFactor: 1,
        className: 'volunteer-trail-polyline',
      }).addTo(searchPartyMapInstance);

      poly.bindTooltip(
        `<strong>${escapeHTML(trail.volunteerAlias || 'Volunteer')}</strong><br>Distance: ${formatDistance(trail.totalDistanceM)}<br>Points: ${trail.points.length}`,
        { sticky: true }
      );
      trailLayers[trailId] = poly;
    }
  }

  // Update or create pulsing marker for active volunteer position
  function updateActiveVolunteerMarker(lat, lng) {
    if (!searchPartyMapInstance) return;

    const pulsingIcon = L.divIcon({
      className: 'volunteer-pulse-marker-wrapper',
      html: '<div class="volunteer-pulse-marker"><span class="pulse-ring"></span><span class="volunteer-pin">📍</span></div>',
      iconSize: [24, 24],
      iconAnchor: [12, 12],
    });

    if (activeVolunteerMarker) {
      activeVolunteerMarker.setLatLng([lat, lng]);
    } else {
      activeVolunteerMarker = L.marker([lat, lng], {
        icon: pulsingIcon,
        title: 'Active Search Position',
        zIndexOffset: 1000,
      }).addTo(searchPartyMapInstance);
      activeVolunteerMarker.bindTooltip('<strong>Active Search Position</strong>', { permanent: false });
    }
  }

  // Load existing trails for a sector from backend REST API
  async function loadSectorTrails(petId, sectorId) {
    try {
      const res = await fetch(
        `/api/v1/search-parties/${encodeURIComponent(petId)}/sectors/${encodeURIComponent(sectorId)}/breadcrumbs`,
        { cache: 'no-store' }
      );
      if (!res.ok) return [];
      const trails = await res.json();
      sectorTrailsCache[sectorId] = trails || [];

      if (Array.isArray(trails)) {
        trails.forEach((tr) => drawTrailPolyline(tr));
      }
      updateSectorTrailBadge(sectorId);
      return trails;
    } catch (err) {
      console.warn(`Failed to load trails for sector ${sectorId}:`, err);
      return [];
    }
  }

  // Load trails across all sectors
  async function loadAllSectorTrails(party) {
    if (!party || !Array.isArray(party.sectors)) return;
    const petId = party.lostPetId || currentPetID;
    for (const sec of party.sectors) {
      await loadSectorTrails(petId, sec.sectorId);
    }
  }

  // Update sector card badge with trail stats
  function updateSectorTrailBadge(sectorId) {
    const badge = document.getElementById(`sector-trail-badge-${sectorId}`);
    if (!badge) return;
    const trails = sectorTrailsCache[sectorId] || [];
    if (trails.length === 0) {
      badge.textContent = '👣 0 trails';
      return;
    }
    let totalDist = 0;
    trails.forEach((tr) => {
      totalDist += tr.totalDistanceM || 0;
    });
    badge.textContent = `👣 ${trails.length} trail${trails.length > 1 ? 's' : ''} (${formatDistance(totalDist)})`;
  }

  // Get active volunteer alias
  function getVolunteerAlias() {
    if (activeSectorId && currentParty?.activeAssignments) {
      const asgn = currentParty.activeAssignments.find((a) => a.sectorId === activeSectorId);
      if (asgn && asgn.volunteerAlias) return asgn.volunteerAlias;
    }
    const aliasInput = document.getElementById('claim-volunteer-alias');
    if (aliasInput && aliasInput.value.trim()) return aliasInput.value.trim();
    return 'Volunteer Alpha';
  }

  // Start recording volunteer GPS breadcrumbs
  function startRecordingTrail(targetSectorId) {
    if (!currentParty) return;

    const sectorId =
      targetSectorId ||
      selectedSectorId ||
      currentParty.activeAssignments?.[0]?.sectorId ||
      currentParty.sectors?.[0]?.sectorId;

    if (!sectorId) {
      showToast('Please select a sector to record your search trail', true);
      return;
    }

    const sector = currentParty.sectors?.find((s) => s.sectorId === sectorId);
    activeSectorId = sectorId;
    activeTrailId = `trail-${sectorId}-${Date.now()}`;
    currentTrailPoints = [];
    currentTrailDistanceMeters = 0;
    isRecordingTrail = true;

    // Update UI elements
    const btnLabel = document.getElementById('record-trail-btn-label');
    const btnIcon = document.getElementById('record-trail-icon');
    const recordBtn = document.getElementById('btn-toggle-record-trail');
    const recBadge = document.getElementById('recording-active-badge');
    const secLabel = document.getElementById('trail-active-sector-label');

    if (btnLabel) btnLabel.textContent = 'Stop Recording';
    if (btnIcon) btnIcon.textContent = '⏹';
    if (recordBtn) {
      recordBtn.classList.remove('btn-secondary');
      recordBtn.classList.add('btn-danger');
    }
    if (recBadge) recBadge.classList.remove('hidden');
    if (secLabel && sector) secLabel.textContent = sector.name;

    updateDistanceDisplay(0);

    // Start geolocation watch
    if (typeof navigator !== 'undefined' && 'geolocation' in navigator) {
      activeWatchId = navigator.geolocation.watchPosition(
        onPositionUpdate,
        (err) => {
          console.warn('Geolocation watch error:', err);
        },
        { enableHighAccuracy: true, timeout: 10000, maximumAge: 0 }
      );
    } else {
      showToast('Geolocation is not supported by your browser', true);
    }

    showToast(`Started recording path for ${sector ? sector.name : sectorId}`);
  }

  // Stop recording volunteer GPS breadcrumbs
  function stopRecordingTrail() {
    if (!isRecordingTrail) return;

    if (activeWatchId !== null && typeof navigator !== 'undefined' && navigator.geolocation) {
      navigator.geolocation.clearWatch(activeWatchId);
      activeWatchId = null;
    }

    isRecordingTrail = false;

    // Update UI elements
    const btnLabel = document.getElementById('record-trail-btn-label');
    const btnIcon = document.getElementById('record-trail-icon');
    const recordBtn = document.getElementById('btn-toggle-record-trail');
    const recBadge = document.getElementById('recording-active-badge');

    if (btnLabel) btnLabel.textContent = 'Record Search Path';
    if (btnIcon) btnIcon.textContent = '⏺';
    if (recordBtn) {
      recordBtn.classList.remove('btn-danger');
      recordBtn.classList.add('btn-secondary');
    }
    if (recBadge) recBadge.classList.add('hidden');

    if (activeVolunteerMarker && searchPartyMapInstance) {
      searchPartyMapInstance.removeLayer(activeVolunteerMarker);
      activeVolunteerMarker = null;
    }

    void flushOfflineBreadcrumbs();
    showToast(`Stopped recording. Total distance walked: ${formatDistance(currentTrailDistanceMeters)}`);
  }

  // Toggle recording state
  function toggleRecordingTrail() {
    if (isRecordingTrail) {
      stopRecordingTrail();
    } else {
      startRecordingTrail();
    }
  }

  // Handle GPS position update from navigator.geolocation.watchPosition
  async function onPositionUpdate(pos) {
    if (!isRecordingTrail || !pos || !pos.coords) return;

    const lat = pos.coords.latitude;
    const lng = pos.coords.longitude;
    const accuracy = pos.coords.accuracy || 0;
    const timestamp = new Date(pos.timestamp || Date.now()).toISOString();

    const pt = {
      latitude: lat,
      longitude: lng,
      timestamp: timestamp,
      accuracyMeters: accuracy,
    };

    currentTrailPoints.push(pt);
    currentTrailDistanceMeters = calculateLocalTrailDistance(currentTrailPoints);
    updateDistanceDisplay(currentTrailDistanceMeters);

    updateActiveVolunteerMarker(lat, lng);

    // Update local polyline live
    const localTrail = {
      trailId: activeTrailId,
      searchPartyId: currentParty?.partyId || '',
      sectorId: activeSectorId,
      volunteerAlias: getVolunteerAlias(),
      points: currentTrailPoints,
      totalDistanceM: currentTrailDistanceMeters,
    };
    drawTrailPolyline(localTrail);

    // Send breadcrumb batch to backend or offline queue
    await sendBreadcrumbBatch([pt]);
  }

  // Transmit breadcrumbs batch or buffer in IndexedDB
  async function sendBreadcrumbBatch(points) {
    if (!currentParty || !activeSectorId || !points || points.length === 0) return;

    const payload = {
      trailId: activeTrailId,
      volunteerAlias: getVolunteerAlias(),
      points: points,
    };

    const petId = currentPetID || currentParty.lostPetId;

    if (typeof navigator !== 'undefined' && !navigator.onLine) {
      await queueOfflineBreadcrumbs({
        id: generateUUID(),
        petId: petId,
        sectorId: activeSectorId,
        payload: payload,
        queuedAt: new Date().toISOString(),
      });
      await updateOfflineBadge();
      return;
    }

    try {
      const csrf = await getCsrfToken();
      const headers = { 'Content-Type': 'application/json' };
      if (csrf) headers['X-CSRF-Token'] = csrf;

      const res = await fetch(
        `/api/v1/search-parties/${encodeURIComponent(petId)}/sectors/${encodeURIComponent(activeSectorId)}/breadcrumbs`,
        {
          method: 'POST',
          headers,
          body: JSON.stringify(payload),
        }
      );

      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`);
      }

      await updateOfflineBadge();
      updateSectorTrailBadge(activeSectorId);
    } catch (err) {
      console.warn('Network error sending breadcrumbs, buffering offline:', err);
      await queueOfflineBreadcrumbs({
        id: generateUUID(),
        petId: petId,
        sectorId: activeSectorId,
        payload: payload,
        queuedAt: new Date().toISOString(),
      });
      await updateOfflineBadge();
    }
  }

  // Connect to SSE stream for live search party updates
  function connectSearchPartySSE(petId, matchId) {
    if (typeof EventSource !== 'function') return;

    if (activeEventSource) {
      activeEventSource.close();
      activeEventSource = null;
    }

    const streamParam = matchId || petId;
    if (!streamParam) return;

    const sseUrl = `/api/v1/reunions/events?matchId=${encodeURIComponent(streamParam)}`;

    try {
      const evtSource = new EventSource(sseUrl);
      activeEventSource = evtSource;

      const onUpdate = (e) => {
        try {
          const payload = JSON.parse(e.data);
          if (!payload) return;
          if (payload.type === 'beacon_ping') {
            onBeaconPing(e);
            return;
          }
          if (payload.type === 'search_party_updated' || payload.sectorId) {
            applySectorUpdate(
              payload.sectorId,
              payload.status,
              payload.coveragePercentage,
              payload.activeVolunteersCount
            );
            if (window.petspotrAnnounce) {
              window.petspotrAnnounce('Search party updated: sector status changed');
            }
          }
        } catch (err) {
          console.warn('Failed to parse search party SSE message:', err);
        }
      };

      const onBeaconPing = (e) => {
        try {
          const payload = JSON.parse(e.data);
          if (!payload) return;
          if (payload.petId && currentPetID && payload.petId !== currentPetID) {
            return;
          }
          const ping = payload.ping || (payload.type === 'beacon_ping' ? payload : null);
          if (ping && (ping.observerCoords || ping.distanceMeters != null)) {
            const distEl = document.getElementById('beacon-distance-display');
            if (distEl && typeof ping.distanceMeters === 'number') {
              distEl.textContent = `~${ping.distanceMeters.toFixed(1)}m (${ping.proximity || 'remote'})`;
            }
            const rssiEl = document.getElementById('beacon-rssi-display');
            if (rssiEl && typeof ping.rssi === 'number') {
              rssiEl.textContent = `${ping.rssi} dBm`;
            }
            renderBeaconPingOnMap(ping);
          }
          if (payload.triangulation) {
            handleTriangulationUpdate(payload.triangulation);
          }
        } catch (err) {
          console.warn('Failed to parse beacon_ping SSE message:', err);
        }
      };

      const onBreadcrumb = (e) => {
        try {
          const payload = JSON.parse(e.data);
          if (!payload) return;
          const trail = payload.trail || (payload.trailId ? payload : null);
          if (trail && Array.isArray(trail.points)) {
            drawTrailPolyline(trail);
            const secId = trail.sectorId || payload.sectorId;
            if (secId) {
              if (!sectorTrailsCache[secId]) {
                sectorTrailsCache[secId] = [];
              }
              const existingIdx = sectorTrailsCache[secId].findIndex((t) => t.trailId === trail.trailId);
              if (existingIdx >= 0) {
                sectorTrailsCache[secId][existingIdx] = trail;
              } else {
                sectorTrailsCache[secId].push(trail);
              }
              updateSectorTrailBadge(secId);
            }
          }
        } catch (err) {
          console.warn('Failed to parse breadcrumb SSE message:', err);
        }
      };

      evtSource.addEventListener('search_party_updated', onUpdate);
      evtSource.addEventListener('breadcrumb_updated', onBreadcrumb);
      evtSource.addEventListener('beacon_ping', onBeaconPing);
      evtSource.addEventListener('message', onUpdate);
      evtSource.addEventListener('error', () => {
        // EventSource will automatically retry in modern browsers
      });
    } catch (err) {
      console.warn('Failed to initialize search party EventSource:', err);
    }
  }

  /* ==========================================================================
     Milestone 11.1: Wilderness Mesh Network & HUD Integration
     ========================================================================== */

  const meshPeersMap = new Map();

  function openMeshModal() {
    const modal = document.getElementById('mesh-modal');
    const indicator = document.getElementById('mesh-status-indicator');
    if (!modal) return;

    modal.classList.remove('hidden');
    if (indicator) {
      indicator.setAttribute('aria-expanded', 'true');
    }
    updateMeshUI();

    const closeBtn = document.getElementById('btn-close-mesh-modal');
    if (closeBtn) {
      closeBtn.focus();
    }
  }

  function closeMeshModal() {
    const modal = document.getElementById('mesh-modal');
    const indicator = document.getElementById('mesh-status-indicator');
    if (!modal) return;

    modal.classList.add('hidden');
    if (indicator) {
      indicator.setAttribute('aria-expanded', 'false');
      indicator.focus();
    }
  }

  function toggleMeshModal() {
    const modal = document.getElementById('mesh-modal');
    if (!modal) return;
    if (modal.classList.contains('hidden')) {
      openMeshModal();
    } else {
      closeMeshModal();
    }
  }

  function updateMeshUI() {
    if (window.petSpotRMesh?.getPeers) {
      const livePeers = window.petSpotRMesh.getPeers();
      if (Array.isArray(livePeers)) {
        livePeers.forEach((p) => {
          if (p && p.nodeId) meshPeersMap.set(p.nodeId, p);
        });
      }
    }

    const peers = Array.from(meshPeersMap.values());
    const count = peers.length;

    const indicator = document.getElementById('mesh-status-indicator');
    if (indicator) {
      if (count > 0) {
        indicator.className = 'mesh-pill mesh-connected';
        indicator.innerHTML = `<span class="mesh-status-dot active"></span><span class="mesh-status-label">⚡ Mesh: ${count} Active ${count === 1 ? 'Peer' : 'Peers'}</span>`;
      } else {
        indicator.className = 'mesh-pill mesh-pill-standalone';
        indicator.innerHTML = `<span class="mesh-status-dot"></span><span class="mesh-status-label">⚡ Mesh: Standalone (0 peers)</span>`;
      }
    }

    const modalPill = document.getElementById('mesh-modal-status-pill');
    if (modalPill) {
      if (count > 0) {
        modalPill.className = 'badge mesh-pill mesh-connected';
        modalPill.innerHTML = `<span class="mesh-status-dot active"></span><span class="mesh-status-text">Mesh Active (${count} ${count === 1 ? 'Peer' : 'Peers'})</span>`;
      } else {
        modalPill.className = 'badge mesh-pill mesh-pill-standalone';
        modalPill.innerHTML = `<span class="mesh-status-dot"></span><span class="mesh-status-text">Standalone Mode (0 peers)</span>`;
      }
    }

    const roster = document.getElementById('mesh-peer-roster');
    if (roster) {
      if (count === 0) {
        roster.innerHTML = `
          <div class="mesh-empty-roster" id="mesh-empty-roster">
            <p class="text-secondary">No remote peers connected yet. Use Auto-Pair on local Wi-Fi or Optical QR exchange to pair with field searchers.</p>
          </div>`;
      } else {
        roster.innerHTML = '';
        peers.forEach((peer) => {
          const card = document.createElement('div');
          card.className = 'mesh-peer-card';
          card.dataset.nodeId = peer.nodeId;
          const roleBadgeClass = peer.role === 'INCIDENT_COMMANDER' ? 'badge-primary' : (peer.role === 'K9_HANDLER' ? 'badge-warning' : 'badge-secondary');
          const battery = peer.batteryLevel != null ? `${peer.batteryLevel}%` : '100%';
          card.innerHTML = `
            <div class="mesh-peer-card-header">
              <div class="mesh-peer-identity">
                <span class="mesh-peer-dot active"></span>
                <strong class="mesh-peer-name">${escapeHTML(peer.volunteerName || 'Volunteer')}</strong>
              </div>
              <span class="badge ${roleBadgeClass}">${escapeHTML(peer.role || 'SEARCHER')}</span>
            </div>
            <div class="mesh-peer-telemetry">
              <span class="mesh-peer-stat">🔋 ${escapeHTML(battery)}</span>
              <span class="mesh-peer-stat">⚡ &lt; 20ms</span>
              <span class="mesh-peer-stat mesh-node-id">${escapeHTML(peer.nodeId ? peer.nodeId.substring(0, 12) : '')}</span>
            </div>
          `;
          roster.appendChild(card);
        });
      }
    }
  }

  function handleMeshPeerJoined(peer) {
    if (!peer || !peer.nodeId) return;
    meshPeersMap.set(peer.nodeId, peer);
    updateMeshUI();
  }

  function handleMeshPeerLeft(detail) {
    const nodeId = typeof detail === 'string' ? detail : detail?.nodeId;
    if (nodeId) {
      meshPeersMap.delete(nodeId);
      updateMeshUI();
    }
  }

  function handleMeshSectorUpdated(detail) {
    if (!detail || !detail.sectorId) return;
    const sectorId = detail.sectorId;

    if (currentParty && Array.isArray(currentParty.sectors)) {
      let targetSector = currentParty.sectors.find((s) => s.sectorId === sectorId);
      if (!targetSector && currentParty.sectors.length > 0) {
        targetSector = currentParty.sectors[0];
      }
      if (targetSector) {
        if (detail.state === 'CLEARED') {
          targetSector.status = 'cleared';
          targetSector.meshVerified = true;
        } else if (detail.state === 'CLAIMED' || detail.state === 'SEARCHING') {
          targetSector.status = 'active_search';
        } else if (detail.state === 'UNCLAIMED') {
          targetSector.status = 'unassigned';
        }

        if (detail.claimedByName) {
          if (!currentParty.activeAssignments) currentParty.activeAssignments = [];
          let asgn = currentParty.activeAssignments.find((a) => a.sectorId === sectorId);
          if (!asgn) {
            currentParty.activeAssignments.push({
              sectorId: targetSector.sectorId,
              volunteerAlias: detail.claimedByName,
            });
          } else if (detail.claimedByName !== 'Anonymous Searcher' || !asgn.volunteerAlias) {
            asgn.volunteerAlias = detail.claimedByName;
          }
        }

        const clearedCount = currentParty.sectors.filter((s) => s.status === 'cleared').length;
        currentParty.coveragePercentage = Math.round((clearedCount / currentParty.sectors.length) * 100);
        updateSummaryUI(currentParty);
        renderSectorCards(currentParty);
      }
    }

    // Update Leaflet polygon on map
    const poly = sectorLayers[sectorId];
    if (poly) {
      const isCleared = detail.state === 'CLEARED';
      const style = getSectorStyle(isCleared ? 'cleared' : 'active_search');
      poly.setStyle(style);
      const pathEl = poly.getElement();
      if (pathEl) {
        pathEl.setAttribute('data-sector-id', sectorId);
        if (isCleared) {
          pathEl.classList.add('sector-cleared');
        }
      }
    }
  }

  function handleMeshBreadcrumbReceived(bc) {
    if (!bc || !bc.latitude || !bc.longitude || !searchPartyMapInstance) return;
    const peerTrailId = `mesh-trail-${bc.volunteerId || 'peer'}`;
    let poly = trailLayers[peerTrailId];
    const newPt = [Number(bc.latitude), Number(bc.longitude)];
    if (poly) {
      poly.addLatLng(newPt);
    } else {
      const color = getTrailColor(bc.volunteerName || bc.volunteerId || 'mesh');
      poly = L.polyline([newPt], {
        color: color,
        weight: 4,
        opacity: 0.85,
        smoothFactor: 1,
        className: 'volunteer-trail-polyline mesh-peer-trail',
      }).addTo(searchPartyMapInstance);
      poly.bindTooltip(
        `<strong>${escapeHTML(bc.volunteerName || 'Mesh Peer')}</strong> (Mesh Live Trail)`,
        { sticky: true }
      );
      trailLayers[peerTrailId] = poly;
    }
  }

  function handleMeshSOSAlert(sos) {
    if (!sos) return;
    const banner = document.getElementById('mesh-sos-banner');
    const textEl = document.getElementById('mesh-sos-text');
    if (banner && textEl) {
      const volName = sos.volunteerName || 'Field Volunteer';
      const volId = sos.volunteerId || 'anon';
      const msg = sos.message || 'Emergency assistance requested';
      const lat = typeof sos.latitude === 'number' ? sos.latitude.toFixed(4) : (sos.latitude || '0.0000');
      const lng = typeof sos.longitude === 'number' ? sos.longitude.toFixed(4) : (sos.longitude || '0.0000');

      textEl.textContent = `🚨 ${volName} (${volId}): "${msg}" at coordinates [${lat}, ${lng}]`;
      banner.classList.remove('hidden');

      const announcer = document.getElementById('aria-announcer') || document.getElementById('beacon-aria-announcer');
      if (announcer) {
        announcer.textContent = `Emergency distress alert: ${volName} at ${lat}, ${lng}. Message: ${msg}`;
      }
    }
  }

  async function handleAutoPair() {
    if (!window.petSpotRMesh) return;
    const btn = document.getElementById('btn-mesh-auto-pair');
    if (btn) btn.disabled = true;

    try {
      await window.petSpotRMesh.init({
        partyId: currentParty?.partyId || currentPetID || 'default-party',
        volunteerInfo: {
          volunteerId: 'vol-' + Math.random().toString(36).substring(2, 9),
          volunteerName: 'Local Field Searcher',
          role: 'SEARCHER',
        },
      });
      updateMeshUI();
    } catch (err) {
      console.warn('Auto-pair error:', err);
    } finally {
      if (btn) btn.disabled = false;
    }
  }

  async function handleShowQROffer() {
    const qrView = document.getElementById('mesh-qr-view');
    const qrContainer = document.getElementById('mesh-qr-container');
    const cameraContainer = document.getElementById('mesh-camera-container');
    if (!qrView || !qrContainer) return;

    if (cameraContainer) cameraContainer.classList.add('hidden');
    qrView.classList.remove('hidden');
    qrContainer.innerHTML = '<p class="text-secondary">Generating optical QR offer...</p>';

    try {
      let compressedOffer = '';
      if (window.petSpotRMesh?.generateOpticalOffer) {
        compressedOffer = await window.petSpotRMesh.generateOpticalOffer();
      } else {
        compressedOffer = 'offline-mock-optical-offer';
      }

      const img = document.createElement('img');
      img.src = `/api/v1/mesh/qr-signaling?data=${encodeURIComponent(compressedOffer)}&size=256`;
      img.alt = 'Optical WebRTC Pairing QR Code';
      img.className = 'mesh-qr-code-img';
      img.width = 256;
      img.height = 256;

      qrContainer.innerHTML = `
        <div class="mesh-qr-display-box">
          <p class="mesh-qr-title"><strong>Scan with Peer Camera</strong></p>
          <div class="mesh-qr-image-wrapper"></div>
          <p class="mesh-qr-hint text-secondary">Compact WebRTC SDP offer compressed for link-local radio pairing.</p>
        </div>`;
      qrContainer.querySelector('.mesh-qr-image-wrapper')?.appendChild(img);
    } catch (err) {
      console.warn('Optical QR generation error:', err);
      qrContainer.innerHTML = `<p class="form-feedback form-feedback-error">Failed to generate QR offer: ${escapeHTML(err.message || 'Unknown error')}</p>`;
    }
  }

  async function handleScanQROffer() {
    const qrView = document.getElementById('mesh-qr-view');
    const cameraContainer = document.getElementById('mesh-camera-container');
    const statusEl = document.getElementById('mesh-camera-status');
    if (!qrView || !cameraContainer) return;

    qrView.classList.remove('hidden');
    cameraContainer.classList.remove('hidden');

    const video = document.getElementById('mesh-camera-video');
    if (!video) return;

    if (navigator.mediaDevices && navigator.mediaDevices.getUserMedia) {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: 'environment' },
        });
        video.srcObject = stream;
        video.play();
        if (statusEl) statusEl.textContent = 'Optical scanner active. Align QR code in viewfinder.';
      } catch (err) {
        if (statusEl) statusEl.textContent = `Camera unavailable (${err.name || 'permission denied'}). Use manual auto-pair.`;
      }
    } else {
      if (statusEl) statusEl.textContent = 'Camera API not supported in this environment.';
    }
  }

  async function handleBroadcastSOS() {
    const btn = document.getElementById('btn-mesh-sos');
    const feedback = document.getElementById('mesh-sos-feedback');
    if (btn) btn.disabled = true;

    let coords = { latitude: 0, longitude: 0 };
    if (currentTrailPoints.length > 0) {
      const lastPt = currentTrailPoints[currentTrailPoints.length - 1];
      coords = { latitude: lastPt.latitude, longitude: lastPt.longitude };
    } else if (currentParty?.centerCoordinates) {
      coords = {
        latitude: currentParty.centerCoordinates.latitude,
        longitude: currentParty.centerCoordinates.longitude,
      };
    }

    try {
      if (window.petSpotRMesh?.broadcastSOS) {
        await window.petSpotRMesh.broadcastSOS('Distress SOS Beacon: Field volunteer requests urgent support.', coords);
      } else {
        handleMeshSOSAlert({
          alertId: 'sos-' + Date.now(),
          volunteerName: 'Local Volunteer',
          latitude: coords.latitude,
          longitude: coords.longitude,
          message: 'Distress SOS Beacon: Field volunteer requests urgent support.',
        });
      }
      if (feedback) {
        feedback.textContent = '🚨 Emergency SOS broadcasted to all active mesh peers!';
        feedback.className = 'form-feedback form-feedback-success';
        feedback.classList.remove('hidden');
      }
    } catch (err) {
      if (feedback) {
        feedback.textContent = 'Failed to broadcast SOS: ' + err.message;
        feedback.className = 'form-feedback form-feedback-error';
        feedback.classList.remove('hidden');
      }
    } finally {
      setTimeout(() => {
        if (btn) btn.disabled = false;
      }, 2000);
    }
  }

  // Register window custom event listeners for mesh synchronization
  window.addEventListener('mesh:peer-joined', (e) => {
    handleMeshPeerJoined(e.detail);
  });
  window.addEventListener('mesh:peer-left', (e) => {
    handleMeshPeerLeft(e.detail);
  });
  window.addEventListener('mesh:sector-updated', (e) => {
    handleMeshSectorUpdated(e.detail);
  });
  window.addEventListener('mesh:breadcrumb-received', (e) => {
    handleMeshBreadcrumbReceived(e.detail);
  });
  window.addEventListener('mesh:sos-alert', (e) => {
    handleMeshSOSAlert(e.detail);
  });

  // Event Delegation initialization
  document.addEventListener('DOMContentLoaded', () => {
    // Wire claim sector form
    const claimForm = document.getElementById('form-claim-sector');
    if (claimForm) {
      claimForm.addEventListener('submit', handleClaimSubmit);
    }

    // Wire clear sector form
    const clearForm = document.getElementById('form-clear-sector');
    if (clearForm) {
      clearForm.addEventListener('submit', handleClearSubmit);
    }

    // Global click listener for search party triggers and modals
    document.addEventListener('click', (e) => {
      // 0a. Sort Sectors by Urgency Button
      const sortUrgencyBtn = e.target.closest('#btn-sort-sectors-urgency');
      if (sortUrgencyBtn) {
        e.preventDefault();
        toggleSortSectorsUrgency();
        return;
      }

      // 0b. Toggle Record Trail Button
      const recordBtn = e.target.closest('[data-action="toggle-record-trail"], #btn-toggle-record-trail');
      if (recordBtn) {
        e.preventDefault();
        toggleRecordingTrail();
        return;
      }

      // 0b. Beacon Radar Action Buttons
      const beaconScanBtn = e.target.closest('#btn-start-beacon-scan');
      if (beaconScanBtn) {
        e.preventDefault();
        if (beaconScannerInstance) {
          if (beaconScannerInstance.isScanning) {
            beaconScannerInstance.stopScan();
            beaconScanBtn.textContent = '📡 Start Beacon Scan';
          } else {
            beaconScannerInstance.startScan().then(() => {
              beaconScanBtn.textContent = '🛑 Stop Beacon Scan';
            }).catch(() => {});
          }
        }
        return;
      }

      const beaconAudioBtn = e.target.closest('#btn-toggle-beacon-audio');
      if (beaconAudioBtn) {
        e.preventDefault();
        if (beaconScannerInstance) {
          const muted = beaconScannerInstance.toggleAudio();
          beaconAudioBtn.textContent = muted ? '🔇 Audio Ping: Muted' : '🔊 Audio Ping: Active';
          beaconAudioBtn.setAttribute('aria-pressed', muted ? 'false' : 'true');
        }
        return;
      }

      const beaconLogBtn = e.target.closest('#btn-log-beacon-sighting');
      if (beaconLogBtn) {
        e.preventDefault();
        handleLogBeaconSightingClick();
        return;
      }

      // 0c. Mesh Modal and HUD Actions
      const meshIndicatorBtn = e.target.closest('#mesh-status-indicator');
      if (meshIndicatorBtn) {
        e.preventDefault();
        toggleMeshModal();
        return;
      }

      if (e.target.closest('#btn-close-mesh-modal, [data-close-modal="mesh"]')) {
        e.preventDefault();
        closeMeshModal();
        return;
      }

      if (e.target.closest('#btn-close-mesh-sos')) {
        e.preventDefault();
        const banner = document.getElementById('mesh-sos-banner');
        if (banner) banner.classList.add('hidden');
        return;
      }

      if (e.target.closest('#btn-mesh-auto-pair')) {
        e.preventDefault();
        void handleAutoPair();
        return;
      }

      if (e.target.closest('#btn-mesh-show-qr')) {
        e.preventDefault();
        void handleShowQROffer();
        return;
      }

      if (e.target.closest('#btn-mesh-scan-qr')) {
        e.preventDefault();
        void handleScanQROffer();
        return;
      }

      if (e.target.closest('#btn-mesh-sos')) {
        e.preventDefault();
        void handleBroadcastSOS();
        return;
      }

      // 1. Open Search Party Trigger
      const searchPartyBtn = e.target.closest('[data-action="view-search-party"]');
      if (searchPartyBtn) {
        e.preventDefault();
        const petId = searchPartyBtn.dataset.petId || document.querySelector('main')?.dataset.petId;
        const petName = searchPartyBtn.dataset.petName || document.querySelector('main')?.dataset.petName;
        if (petId) {
          openSearchPartyModal(petId, petName);
        }
        return;
      }

      // 2. Close Search Party Modal
      if (e.target.closest('[data-close-modal="search-party"]')) {
        e.preventDefault();
        closeSearchPartyModal();
        return;
      }

      // 3. Close Claim Modal
      if (e.target.closest('[data-close-modal="claim-sector"]')) {
        e.preventDefault();
        closeClaimModal();
        return;
      }

      // 4. Close Clear Modal
      if (e.target.closest('[data-close-modal="clear-sector"]')) {
        e.preventDefault();
        closeClearModal();
        return;
      }

      // 5. Card Action buttons (claim or update)
      const sectorActionBtn = e.target.closest('.btn-sector-action');
      if (sectorActionBtn) {
        e.preventDefault();
        const sectorId = sectorActionBtn.dataset.sectorId;
        if (sectorId) {
          handleSectorClick(sectorId);
        }
        return;
      }

      // 6. Click on backdrop to dismiss modals
      if (e.target.classList.contains('modal-overlay')) {
        if (e.target.id === 'pet-search-party-container') {
          closeSearchPartyModal();
        } else if (e.target.id === 'modal-claim-sector') {
          closeClaimModal();
        } else if (e.target.id === 'modal-clear-sector') {
          closeClearModal();
        } else if (e.target.id === 'mesh-modal') {
          closeMeshModal();
        }
      }
    });

    // Mesh modal focus trapping
    const meshModalEl = document.getElementById('mesh-modal');
    if (meshModalEl) {
      meshModalEl.addEventListener('keydown', (e) => {
        if (e.key === 'Tab') {
          const focusables = meshModalEl.querySelectorAll(
            'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
          );
          if (focusables.length === 0) return;
          const first = focusables[0];
          const last = focusables[focusables.length - 1];

          if (e.shiftKey) {
            if (document.activeElement === first) {
              e.preventDefault();
              last.focus();
            }
          } else {
            if (document.activeElement === last) {
              e.preventDefault();
              first.focus();
            }
          }
        }
      });
    }

    // Escape key listener to close modals
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        const meshModal = document.getElementById('mesh-modal');
        if (meshModal && !meshModal.classList.contains('hidden')) {
          closeMeshModal();
          return;
        }
        const claimModal = document.getElementById('modal-claim-sector');
        if (claimModal && !claimModal.classList.contains('hidden')) {
          closeClaimModal();
          return;
        }
        const clearModal = document.getElementById('modal-clear-sector');
        if (clearModal && !clearModal.classList.contains('hidden')) {
          closeClearModal();
          return;
        }
        const spModal = document.getElementById('pet-search-party-container');
        if (spModal && !spModal.classList.contains('hidden')) {
          closeSearchPartyModal();
        }
      }
    });
  });

  // Online / Offline synchronization listeners
  window.addEventListener('online', () => {
    void flushOfflineBreadcrumbs();
  });
  window.addEventListener('offline', () => {
    void updateOfflineBadge();
  });

  // Export for testability and headless automation
  window.petspotrSearchParty = {
    startRecordingTrail,
    stopRecordingTrail,
    toggleRecordingTrail,
    isRecording: () => isRecordingTrail,
    getQueuedBreadcrumbs,
    flushOfflineBreadcrumbs,
    drawTrailPolyline,
    calculateLocalTrailDistance,
    openBreadcrumbsDB,
    updateActiveVolunteerMarker,
    onPositionUpdate,
    sendBreadcrumbBatch,
    // Beacon methods
    openSearchParty: openSearchPartyModal,
    openSearchPartyModal,
    initBeaconScanner,
    handleBeaconPing,
    handleTriangulationUpdate,
    renderBeaconPingOnMap,
    handleLogBeaconSightingClick,
    getScanner: () => beaconScannerInstance,
    getLatestPing: () => latestBeaconPing,
    getLatestTriangulation: () => latestTriangulation,
    // Mesh coordination methods
    openMeshModal,
    closeMeshModal,
    toggleMeshModal,
    updateMeshUI,
    handleMeshPeerJoined,
    handleMeshPeerLeft,
    handleMeshSectorUpdated,
    handleMeshBreadcrumbReceived,
    handleMeshSOSAlert,
    handleAutoPair,
    handleShowQROffer,
    handleScanQROffer,
    handleBroadcastSOS,
    getMeshPeers: () => Array.from(meshPeersMap.values()),
  };

  // Window-level helper aliases
  if (typeof window !== 'undefined') {
    window.openSearchParty = openSearchPartyModal;
    window.openSearchPartyModal = openSearchPartyModal;
    window.initSearchParty = openSearchPartyModal;
  }
})();
