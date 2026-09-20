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

  // Sector style definition by status
  function getSectorStyle(status) {
    switch (status) {
      case 'active_search':
        return {
          color: '#4f46e5',
          fillColor: '#6366f1',
          fillOpacity: 0.5,
          weight: 2,
          className: 'sector-polygon sector-active-search',
        };
      case 'cleared':
        return {
          color: '#16a34a',
          fillColor: '#22c55e',
          fillOpacity: 0.4,
          weight: 2,
          className: 'sector-polygon sector-cleared',
        };
      case 'sighting_reported':
        return {
          color: '#dc2626',
          fillColor: '#ef4444',
          fillOpacity: 0.45,
          weight: 3,
          className: 'sector-polygon sector-sighting-reported sector-pulse',
        };
      case 'unassigned':
      default:
        return {
          color: '#d97706',
          fillColor: '#f59e0b',
          fillOpacity: 0.4,
          weight: 2,
          className: 'sector-polygon sector-unassigned',
        };
    }
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

  // Open Search Party Modal
  function openSearchPartyModal(petId, petName) {
    const modal = document.getElementById('pet-search-party-container');
    if (!modal) return;

    currentPetID = petId || '';
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

        const style = getSectorStyle(sector.status);
        const polygon = L.polygon(latLngs, style).addTo(searchPartyMapInstance);
        sectorLayers[sector.sectorId] = polygon;

        // Tooltip
        polygon.bindTooltip(
          `<strong>${escapeHTML(sector.name)}</strong><br>Status: ${formatSectorStatus(sector.status)}<br><em>Click to claim or update</em>`,
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

    // Refresh size on modal display
    setTimeout(() => {
      if (searchPartyMapInstance) {
        searchPartyMapInstance.invalidateSize();
      }
    }, 150);
  }

  // Handle sector click from map polygon or list card
  function handleSectorClick(sectorId) {
    if (!currentParty || !Array.isArray(currentParty.sectors)) return;
    const sector = currentParty.sectors.find((s) => s.sectorId === sectorId);
    if (!sector) return;

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

    party.sectors.forEach((sec) => {
      const card = document.createElement('div');
      card.className = `sector-card sector-card-${sec.status}`;
      card.dataset.sectorId = sec.sectorId;

      const asgn = party.activeAssignments?.find((a) => a.sectorId === sec.sectorId);
      const volunteerText = asgn ? escapeHTML(asgn.volunteerAlias) : 'No volunteer assigned';
      const isUnassigned = sec.status === 'unassigned';
      const btnAction = isUnassigned ? 'claim-sector' : 'clear-sector';
      const btnLabel = isUnassigned ? 'Claim Sector' : 'Update Status';
      const btnClass = isUnassigned ? 'btn-primary' : 'btn-secondary';

      card.innerHTML = `
        <div class="sector-card-header">
          <h4 class="sector-card-name">${escapeHTML(sec.name)}</h4>
          <span class="badge badge-status-${sec.status}">${formatSectorStatus(sec.status)}</span>
        </div>
        <p class="sector-card-volunteer text-secondary">👤 ${volunteerText}</p>
        <div class="sector-card-actions">
          <button type="button" class="btn ${btnClass} btn-sm btn-sector-action" data-action="${btnAction}" data-sector-id="${sec.sectorId}">
            ${btnLabel}
          </button>
        </div>
      `;
      container.appendChild(card);
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
      const newStyle = getSectorStyle(newStatus);
      poly.setStyle(newStyle);

      // Handle pulsing animation class on SVG path element
      const pathEl = poly.getElement();
      if (pathEl) {
        if (newStatus === 'sighting_reported') {
          pathEl.classList.add('sector-pulse');
        } else {
          pathEl.classList.remove('sector-pulse');
        }
      }

      if (currentParty.sectors) {
        const sec = currentParty.sectors.find((s) => s.sectorId === sectorId);
        if (sec) {
          poly.setTooltipContent(
            `<strong>${escapeHTML(sec.name)}</strong><br>Status: ${formatSectorStatus(sec.status)}<br><em>Click to claim or update</em>`
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

      const payload = {
        volunteerAlias: aliasInput ? aliasInput.value.trim() : '',
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
          if (payload.type === 'search_party_updated' || payload.sectorId) {
            applySectorUpdate(
              payload.sectorId,
              payload.status,
              payload.coveragePercentage,
              payload.activeVolunteersCount
            );
          }
        } catch (err) {
          console.warn('Failed to parse search party SSE message:', err);
        }
      };

      evtSource.addEventListener('search_party_updated', onUpdate);
      evtSource.addEventListener('message', onUpdate);
      evtSource.addEventListener('error', () => {
        // EventSource will automatically retry in modern browsers
      });
    } catch (err) {
      console.warn('Failed to initialize search party EventSource:', err);
    }
  }

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
        }
      }
    });

    // Escape key listener to close modals
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
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
})();
