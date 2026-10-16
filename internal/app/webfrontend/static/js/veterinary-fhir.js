/**
 * PetSpotR Veterinary Clinic & Shelter HL7 FHIR R4 / PIMS Exchange Controller
 * Milestone 12.2: Bidirectional Pre-Arrival ER Handoffs, HL7 FHIR R4 Bundle
 * inspection, PIMS CSV exports, and inbound stabilization/discharge reconciliation.
 */

(function () {
  'use strict';

  const handoffCache = new Map();
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
   * Escape string for CSS attribute selector
   */
  function safeCssEscape(str) {
    if (typeof CSS !== 'undefined' && typeof CSS.escape === 'function') {
      return CSS.escape(str);
    }
    return String(str).replace(/["\\]/g, '\\$&');
  }

  /**
   * Display toast notification
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
    const announcer = document.getElementById('fhir-aria-announcer');
    if (announcer) {
      announcer.textContent = message;
    }
  }

  /**
   * Map HandoffStatus to WCAG AAA badge CSS class
   */
  function getHandoffStatusBadgeClass(status) {
    switch (status) {
      case 'EN_ROUTE_PRE_ARRIVAL':
        return 'badge-handoff-enroute';
      case 'IN_CLINIC_TREATMENT':
        return 'badge-handoff-treatment';
      case 'STABILIZED':
        return 'badge-handoff-stabilized';
      case 'CLEARED_FOR_REUNIFICATION':
        return 'badge-handoff-cleared';
      default:
        return 'badge-handoff-enroute';
    }
  }

  /**
   * Map TriageCategory to WCAG AAA badge CSS class
   */
  function getTriageBadgeClass(category) {
    switch (category) {
      case 'TRIAGE_RED':
        return 'badge-triage-red';
      case 'TRIAGE_YELLOW':
        return 'badge-triage-yellow';
      case 'TRIAGE_GREEN':
        return 'badge-triage-green';
      case 'TRIAGE_BLACK':
        return 'badge-triage-black';
      default:
        return 'badge-triage-yellow';
    }
  }

  /**
   * Render or update a clinic card in #clinics-list and #handoff-clinic-select
   */
  function addOrUpdateClinic(clinic, selectAfterAdd = false) {
    if (!clinic || !clinic.clinicId) return;

    const clinicsList = document.getElementById('clinics-list');
    if (clinicsList) {
      const emptyMsg = document.getElementById('clinics-empty-msg');
      if (emptyMsg) emptyMsg.remove();

      const safeId = escapeHtml(clinic.clinicId);
      const safeName = escapeHtml(clinic.name || '');
      const safePims = escapeHtml(clinic.pims || 'IDEXX_CORNERSTONE');
      const safePhone = escapeHtml(clinic.phone || 'N/A');
      const safeAddress = escapeHtml(clinic.address || 'N/A');
      const erBadgeHtml = clinic.emergency24Hr !== false
        ? '<span class="badge badge-er-24hr">24/7 ER</span>'
        : '';

      const innerHtml = `
        <div class="clinic-card-header">
          <strong class="clinic-card-name">${safeName}</strong>
          <div class="clinic-card-badges">
            <span class="badge badge-pims-provider">${safePims}</span>
            ${erBadgeHtml}
          </div>
        </div>
        <div class="clinic-card-meta">
          <span>📞 ${safePhone}</span>
          <span>📍 ${safeAddress}</span>
        </div>
      `;

      let card = clinicsList.querySelector(`.clinic-partner-card[data-clinic-id="${safeCssEscape(clinic.clinicId)}"]`);
      if (card) {
        card.innerHTML = innerHtml;
      } else {
        card = document.createElement('article');
        card.className = 'clinic-partner-card';
        card.setAttribute('data-clinic-id', clinic.clinicId);
        card.setAttribute('role', 'listitem');
        card.innerHTML = innerHtml;
        clinicsList.appendChild(card);
      }

      const countBadge = document.getElementById('clinics-count-badge');
      if (countBadge) {
        const total = clinicsList.querySelectorAll('.clinic-partner-card').length;
        countBadge.textContent = `${total} Clinics`;
      }
    }

    const clinicSelect = document.getElementById('handoff-clinic-select');
    if (clinicSelect) {
      let opt = clinicSelect.querySelector(`option[value="${safeCssEscape(clinic.clinicId)}"]`);
      const label = `${clinic.name} (${clinic.pims || 'IDEXX_CORNERSTONE'})`;
      if (opt) {
        opt.textContent = label;
      } else {
        opt = document.createElement('option');
        opt.value = clinic.clinicId;
        opt.textContent = label;
        clinicSelect.appendChild(opt);
      }
      if (selectAfterAdd) {
        clinicSelect.value = clinic.clinicId;
      }
    }
  }

  /**
   * Render or update a clinical handoff card in #handoffs-list
   */
  function addOrUpdateHandoffCard(handoff) {
    if (!handoff || !handoff.handoffId) return;
    handoffCache.set(handoff.handoffId, handoff);

    const handoffsList = document.getElementById('handoffs-list');
    if (!handoffsList) return;

    const emptyMsg = document.getElementById('handoffs-empty-msg');
    if (emptyMsg) emptyMsg.remove();

    const rawId = handoff.handoffId;
    const status = handoff.status || 'EN_ROUTE_PRE_ARRIVAL';
    const statusBadgeClass = getHandoffStatusBadgeClass(status);
    const triageCategory = handoff.triageCategory || 'TRIAGE_RED';
    const triageBadgeClass = getTriageBadgeClass(triageCategory);

    const safeId = escapeHtml(rawId);
    const safeStatus = escapeHtml(status);
    const safeTriage = escapeHtml(triageCategory);
    const safePetId = escapeHtml(handoff.petId || '');
    const safeClinicName = escapeHtml(handoff.clinicName || handoff.clinicId || '');
    const safePims = escapeHtml(handoff.pims || 'IDEXX_CORNERSTONE');
    const safeTransport = escapeHtml(handoff.transportUnit || 'SAR MedEvac Unit #1');
    const safeEta = Number(handoff.etaMinutes) || 12;
    const rawDigest = handoff.fhirBundle?.bundleDigestHex || handoff.bundleDigestHex || '';
    const safeDigest = escapeHtml(rawDigest ? `${rawDigest.slice(0, 24)}...` : 'N/A');

    const verifiedChip = handoff.verifiedMicrochipId || handoff.pimsRecord?.microchipId || 'UNVERIFIED';
    const clearanceBannerHtml = handoff.readyForReunion
      ? `<div class="reunion-clearance-banner" role="status">✅ CLEARED FOR OWNER REUNIFICATION — Verified Microchip: ${escapeHtml(verifiedChip)}</div>`
      : '';

    const vetSummaryHtml = handoff.attendingVet || handoff.dischargeSummary
      ? `<div class="handoff-discharge-meta">
           ${handoff.attendingVet ? `<div><strong>Attending Vet:</strong> ${escapeHtml(handoff.attendingVet)}</div>` : ''}
           ${handoff.dischargeSummary ? `<div><strong>Clinical Summary:</strong> ${escapeHtml(handoff.dischargeSummary)}</div>` : ''}
         </div>`
      : '';

    const cardHtml = `
      <div class="handoff-card-header">
        <div class="handoff-card-badges">
          <span class="badge-handoff-status ${statusBadgeClass}">${safeStatus}</span>
          <span class="badge ${triageBadgeClass}">${safeTriage}</span>
        </div>
        <strong class="handoff-pet-id">${safePetId}</strong>
      </div>

      <div class="handoff-card-meta">
        <div><strong>Clinic:</strong> ${safeClinicName} (<span class="badge badge-pims-provider">${safePims}</span>)</div>
        <div><strong>Transport:</strong> ${safeTransport} • <strong>ETA:</strong> ${safeEta} min</div>
        <div class="handoff-digest-preview"><strong>SHA-256 Digest:</strong> <code>${safeDigest}</code></div>
      </div>

      ${vetSummaryHtml}
      ${clearanceBannerHtml}

      <div class="handoff-card-actions">
        <button type="button" class="btn btn-xs btn-primary btn-inspect-fhir" data-handoff-id="${safeId}">Inspect FHIR R4 Bundle</button>
        <a href="/api/v1/veterinary/handoffs/${encodeURIComponent(rawId)}/fhir?format=csv" class="btn btn-xs btn-secondary btn-export-pims-csv" download>Download PIMS CSV</a>
        <button type="button" class="btn btn-xs btn-secondary btn-select-pims-sim" data-handoff-id="${safeId}">Simulate Clinic Update</button>
      </div>
    `;

    let card = handoffsList.querySelector(`.handoff-card[data-handoff-id="${safeCssEscape(rawId)}"]`);
    if (card) {
      card.setAttribute('data-handoff-status', status);
      card.innerHTML = cardHtml;
    } else {
      card = document.createElement('article');
      card.className = 'handoff-card';
      card.setAttribute('data-handoff-id', rawId);
      card.setAttribute('data-handoff-status', status);
      card.setAttribute('role', 'article');
      card.innerHTML = cardHtml;
      handoffsList.prepend(card);
    }

    const countBadge = document.getElementById('handoffs-count-badge');
    if (countBadge) {
      const total = handoffsList.querySelectorAll('.handoff-card').length;
      countBadge.textContent = `${total} Handoffs`;
    }
  }

  /**
   * Populate the HL7 FHIR R4 Bundle Inspector Panel
   */
  function renderFHIRBundleInspector(bundle) {
    if (!bundle) return;

    const digestBadge = document.getElementById('fhir-digest-badge');
    if (digestBadge) {
      digestBadge.textContent = bundle.bundleDigestHex
        ? `SHA-256: ${bundle.bundleDigestHex}`
        : 'SHA-256: verified';
    }

    const counts = {
      Patient: 0,
      Encounter: 0,
      Observation: 0,
      AllergyIntolerance: 0,
      Immunization: 0,
      MedicationAdministration: 0,
    };

    if (Array.isArray(bundle.entry)) {
      bundle.entry.forEach((entry) => {
        const rt = entry?.resource?.resourceType;
        if (rt && Object.prototype.hasOwnProperty.call(counts, rt)) {
          counts[rt] += 1;
        }
      });
    }

    const summaryEl = document.getElementById('fhir-resource-summary');
    if (summaryEl) {
      summaryEl.innerHTML = Object.entries(counts)
        .map(([k, v]) => `<span class="badge badge-secondary">${escapeHtml(k)}: ${v}</span>`)
        .join('');
    }

    const jsonViewer = document.getElementById('fhir-bundle-json');
    if (jsonViewer) {
      jsonViewer.textContent = JSON.stringify(bundle, null, 2);
    }
  }

  /**
   * Inspect a handoff's FHIR bundle by ID
   */
  async function inspectHandoffFHIR(handoffId) {
    if (!handoffId) return;

    const cached = handoffCache.get(handoffId);
    if (cached && cached.fhirBundle && cached.fhirBundle.resourceType === 'Bundle') {
      renderFHIRBundleInspector(cached.fhirBundle);
    }

    try {
      const res = await fetch(`/api/v1/veterinary/handoffs/${encodeURIComponent(handoffId)}/fhir`);
      if (res.ok) {
        const bundle = await res.json();
        renderFHIRBundleInspector(bundle);
      }
    } catch (err) {
      console.warn('Failed to fetch FHIR bundle:', err);
    }
  }

  /**
   * Select a handoff into the Inbound PIMS Webhook Simulator
   */
  function selectHandoffForSimulation(handoffId) {
    const inputHandoffId = document.getElementById('pims-sim-handoff-id');
    if (inputHandoffId && handoffId) {
      inputHandoffId.value = handoffId;
    }
    const cached = handoffCache.get(handoffId);
    const chipInput = document.getElementById('pims-sim-microchip');
    if (chipInput && cached) {
      const chip = cached.verifiedMicrochipId || cached.pimsRecord?.microchipId;
      if (chip) chipInput.value = chip;
    }
  }

  /**
   * Handle Register Partner Clinic form submit
   */
  async function handleRegisterClinic(e) {
    e.preventDefault();

    const nameInput = document.getElementById('clinic-name');
    const name = nameInput?.value?.trim() || '';
    if (!name) {
      showToast('Clinic name is required', 'error');
      nameInput?.focus();
      return;
    }

    const pims = document.getElementById('clinic-pims')?.value || 'IDEXX_CORNERSTONE';
    const phone = document.getElementById('clinic-phone')?.value?.trim() || '';
    const address = document.getElementById('clinic-address')?.value?.trim() || '';
    const latitude = parseFloat(document.getElementById('clinic-lat')?.value) || 47.6092;
    const longitude = parseFloat(document.getElementById('clinic-lng')?.value) || -122.3261;

    try {
      const res = await fetch('/api/v1/veterinary/clinics', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name,
          pims,
          emergency24Hr: true,
          phone,
          address,
          latitude,
          longitude,
        }),
      });

      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`);
      }

      const data = await res.json();
      const clinic = data.clinic || data;
      addOrUpdateClinic(clinic, true);
      showToast(`Registered partner clinic: ${clinic.name}`, 'success');
      if (nameInput) nameInput.value = '';
    } catch (err) {
      console.error('Failed to register partner clinic:', err);
      showToast('Failed to register partner clinic', 'error');
    }
  }

  /**
   * Handle Dispatch Outbound Pre-Arrival ER Handoff form submit
   */
  async function handleDispatchHandoff(e) {
    e.preventDefault();

    const petIdInput = document.getElementById('handoff-pet-id');
    const petId = petIdInput?.value?.trim() || '';
    const assessmentId = document.getElementById('handoff-assessment-id')?.value?.trim() || '';
    const clinicId = document.getElementById('handoff-clinic-select')?.value || '';
    const transportUnit = document.getElementById('handoff-transport-unit')?.value?.trim() || 'SAR MedEvac Unit #1';
    const etaMinutes = parseInt(document.getElementById('handoff-eta-minutes')?.value, 10) || 12;

    if (!petId && !assessmentId) {
      showToast('Pet ID or Assessment ID is required', 'error');
      petIdInput?.focus();
      return;
    }

    try {
      const res = await fetch('/api/v1/veterinary/handoffs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          petId,
          assessmentId,
          clinicId,
          transportUnit,
          etaMinutes,
        }),
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || `HTTP ${res.status}`);
      }

      const data = await res.json();
      const handoff = data.handoff || data;
      addOrUpdateHandoffCard(handoff);
      selectHandoffForSimulation(handoff.handoffId);

      if (handoff.fhirBundle) {
        renderFHIRBundleInspector(handoff.fhirBundle);
      } else {
        await inspectHandoffFHIR(handoff.handoffId);
      }

      showToast(`Dispatched Pre-Arrival ER Handoff ${handoff.handoffId}`, 'success');
    } catch (err) {
      console.error('Failed to dispatch ER handoff:', err);
      showToast(`Failed to dispatch ER handoff: ${err.message}`, 'error');
    }
  }

  /**
   * Handle Inbound PIMS Webhook Simulator form submit
   */
  async function handleSendPIMSWebhook(e) {
    e.preventDefault();

    const handoffIdInput = document.getElementById('pims-sim-handoff-id');
    const handoffId = handoffIdInput?.value?.trim() || '';
    if (!handoffId) {
      showToast('Target Handoff ID is required', 'error');
      handoffIdInput?.focus();
      return;
    }

    const status = document.getElementById('pims-sim-status')?.value || 'IN_CLINIC_TREATMENT';
    const verifiedMicrochipId = document.getElementById('pims-sim-microchip')?.value?.trim() || '';
    const attendingVet = document.getElementById('pims-sim-vet')?.value?.trim() || '';
    const dischargeSummary = document.getElementById('pims-sim-summary')?.value?.trim() || '';

    try {
      const res = await fetch(`/api/v1/veterinary/handoffs/${encodeURIComponent(handoffId)}/pims-webhook`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          status,
          verifiedMicrochipId,
          attendingVet,
          dischargeSummary,
        }),
      });

      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`);
      }

      const data = await res.json();
      const updated = data.handoff || data;
      addOrUpdateHandoffCard(updated);
      if (updated.fhirBundle) {
        renderFHIRBundleInspector(updated.fhirBundle);
      }
      showToast(`Inbound PIMS update applied: ${updated.status}`, 'success');
    } catch (err) {
      console.error('Failed to transmit PIMS webhook update:', err);
      showToast('Failed to transmit PIMS webhook update', 'error');
    }
  }

  /**
   * Load clinics and handoffs from API on initial page load
   */
  async function loadInitialPortalData() {
    try {
      const clinicsRes = await fetch('/api/v1/veterinary/clinics');
      if (clinicsRes.ok) {
        const data = await clinicsRes.json();
        const clinics = data.clinics || [];
        clinics.forEach((c) => addOrUpdateClinic(c, false));
      }
    } catch (err) {
      console.warn('Could not refresh clinics list:', err);
    }

    try {
      const handoffsRes = await fetch('/api/v1/veterinary/handoffs');
      if (handoffsRes.ok) {
        const data = await handoffsRes.json();
        const handoffs = data.handoffs || [];
        // Iterate in reverse so newest ends up at top when prepended
        for (let i = handoffs.length - 1; i >= 0; i--) {
          addOrUpdateHandoffCard(handoffs[i]);
        }
        if (handoffs.length > 0) {
          const latest = handoffs[0];
          const simInput = document.getElementById('pims-sim-handoff-id');
          if (simInput && !simInput.value) {
            selectHandoffForSimulation(latest.handoffId);
          }
          if (latest.fhirBundle) {
            renderFHIRBundleInspector(latest.fhirBundle);
          }
        }
      }
    } catch (err) {
      console.warn('Could not refresh handoffs list:', err);
    }
  }

  /**
   * Setup real-time SSE stream listener for handoff dispatch and PIMS updates
   */
  function setupSSE() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/veterinary/triage/stream');

      const handleHandoffEvent = (payload) => {
        if (!payload) return;
        const handoff = payload.handoff || payload;
        if (handoff && handoff.handoffId) {
          addOrUpdateHandoffCard(handoff);
        }
      };

      sseSource.addEventListener('clinical_handoff_dispatched', (e) => {
        try {
          handleHandoffEvent(JSON.parse(e.data));
        } catch (err) {
          console.warn('Failed to parse clinical_handoff_dispatched SSE:', err);
        }
      });

      sseSource.addEventListener('clinical_handoff_updated', (e) => {
        try {
          handleHandoffEvent(JSON.parse(e.data));
        } catch (err) {
          console.warn('Failed to parse clinical_handoff_updated SSE:', err);
        }
      });

      sseSource.addEventListener('message', (e) => {
        try {
          const evt = JSON.parse(e.data);
          if (
            (evt.type === 'clinical_handoff_dispatched' || evt.type === 'clinical_handoff_updated') &&
            evt.payload
          ) {
            handleHandoffEvent(evt.payload);
          }
        } catch (err) {
          // ignore heartbeats
        }
      });
    } catch (err) {
      console.warn('Failed to connect to veterinary SSE stream:', err);
    }
  }

  /**
   * Initialize FHIR & PIMS Exchange Portal
   */
  function init() {
    const portalMain = document.getElementById('fhir-portal-main');
    if (!portalMain) return;

    // Pre-populate Pet ID and Assessment ID from query parameters if present
    const params = new URLSearchParams(window.location.search);
    const petIdParam = params.get('petId');
    const assessmentIdParam = params.get('assessmentId');

    const petIdInput = document.getElementById('handoff-pet-id');
    if (petIdInput && petIdParam) {
      petIdInput.value = petIdParam;
    }

    const assessmentIdInput = document.getElementById('handoff-assessment-id');
    if (assessmentIdInput && assessmentIdParam) {
      assessmentIdInput.value = assessmentIdParam;
    }

    const formRegisterClinic = document.getElementById('form-register-clinic');
    if (formRegisterClinic) {
      formRegisterClinic.addEventListener('submit', handleRegisterClinic);
    }

    const formDispatchHandoff = document.getElementById('form-dispatch-handoff');
    if (formDispatchHandoff) {
      formDispatchHandoff.addEventListener('submit', handleDispatchHandoff);
    }

    const formPimsWebhookSim = document.getElementById('form-pims-webhook-sim');
    if (formPimsWebhookSim) {
      formPimsWebhookSim.addEventListener('submit', handleSendPIMSWebhook);
    }

    const handoffsList = document.getElementById('handoffs-list');
    if (handoffsList) {
      handoffsList.addEventListener('click', (e) => {
        const inspectBtn = e.target.closest('.btn-inspect-fhir');
        if (inspectBtn) {
          const handoffId = inspectBtn.getAttribute('data-handoff-id');
          inspectHandoffFHIR(handoffId);
          selectHandoffForSimulation(handoffId);
          return;
        }

        const simBtn = e.target.closest('.btn-select-pims-sim');
        if (simBtn) {
          const handoffId = simBtn.getAttribute('data-handoff-id');
          selectHandoffForSimulation(handoffId);
          document.getElementById('pims-sim-status')?.focus();
        }
      });
    }

    loadInitialPortalData();
    setupSSE();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
