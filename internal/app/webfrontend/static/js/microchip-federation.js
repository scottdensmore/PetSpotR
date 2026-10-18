/**
 * PetSpotR Universal Microchip Registry Federation, Multi-Standard Scanner Lookup &
 * Ownership Resolution Cockpit Controller (Milestone 13.4)
 */

(function () {
  'use strict';

  let sseSource = null;
  let currentCases = [];

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
   * Display toast notification & update live status banner
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
    const liveStatus = document.getElementById('microchip-federation-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  /**
   * Map FederationResolutionStatus to WCAG AAA badge CSS class
   */
  function getStatusBadgeClass(status) {
    switch (status) {
      case 'REGISTERED_OWNER_FOUND':
        return 'badge-chip-registered';
      case 'UNREGISTERED_BATCH_TRACED':
        return 'badge-chip-unregistered';
      case 'SECONDARY_CONTACT_ESCALATED':
        return 'badge-chip-escalated';
      case 'REUNION_DISPATCHED_VERIFIED':
        return 'badge-chip-verified';
      default:
        return 'badge-chip-registered';
    }
  }

  /**
   * Update KPI summary counters
   */
  function updateKpiCounters(cases) {
    const totalEl = document.getElementById('kpi-chips-total');
    const unregEl = document.getElementById('kpi-chips-unregistered');
    const escalatedEl = document.getElementById('kpi-chips-escalated');
    const reunitedEl = document.getElementById('kpi-chips-reunited');

    const total = cases.length;
    const unregistered = cases.filter(
      (c) => c.status === 'UNREGISTERED_BATCH_TRACED'
    ).length;
    const escalated = cases.filter(
      (c) => c.status === 'SECONDARY_CONTACT_ESCALATED'
    ).length;
    const reunited = cases.filter(
      (c) => c.status === 'REUNION_DISPATCHED_VERIFIED'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (unregEl) unregEl.textContent = String(unregistered);
    if (escalatedEl) escalatedEl.textContent = String(escalated);
    if (reunitedEl) reunitedEl.textContent = String(reunited);
  }

  /**
   * Render a single MicrochipScanCase card
   */
  function renderCaseCardHtml(c) {
    const caseId = escapeHtml(c.caseId || '');
    const petName = escapeHtml(c.petName || '');
    const species = escapeHtml(c.species || 'CAT');
    const maskedChip = escapeHtml(c.maskedChipDisplay || '');
    const status = escapeHtml(c.status || 'REGISTERED_OWNER_FOUND');
    const statusClass = getStatusBadgeClass(c.status);
    const mfgName = escapeHtml(c.manufacturerName || '');
    const agency = escapeHtml(c.scannedByAgency || '');
    const chipStandard = escapeHtml(c.chipStandard || 'ISO_11784_15');
    const freqKhz = Number(c.frequencyKhz ?? 134.2).toFixed(1);
    const lotId = escapeHtml(c.manufacturerLotId || '');
    const winningRegistry = escapeHtml(c.winningRegistry || '');
    const hops = Array.isArray(c.registryHops) ? c.registryHops : [];
    const notes = escapeHtml(c.resolutionNotes || '');

    const winningRegistryHtml = winningRegistry
      ? `<span class="badge badge-winning-registry">Registry: ${winningRegistry}</span>`
      : '';

    const hopsHtml = hops
      .map((hop) => {
        const regName = escapeHtml(hop.registryName || '');
        const mfgIso = escapeHtml(hop.manufacturerIso || '');
        const matched = Boolean(hop.recordFound);
        const hopBadgeClass = matched ? 'badge-hop-match' : 'badge-hop-miss';
        const hopVerdict = matched ? 'MATCH' : 'NO RECORD';
        const hopNote = escapeHtml(hop.statusNote || '');
        return `
          <div class="registry-hop-item">
            <span class="registry-hop-name">${regName} (${mfgIso})</span>
            <span class="badge ${hopBadgeClass}">${hopVerdict}</span>
            <span class="registry-hop-note text-secondary">${hopNote}</span>
          </div>
        `;
      })
      .join('');

    let lotTraceBannerHtml = '';
    if (c.status === 'UNREGISTERED_BATCH_TRACED') {
      const clinicName = escapeHtml(
        c.implantingClinicName || 'Emerald City Spay/Neuter & Wellness Clinic'
      );
      const chartSuffix = c.implantPatientChartId
        ? ` (Chart: ${escapeHtml(c.implantPatientChartId)})`
        : '';
      const phoneSuffix = c.implantingClinicPhone
        ? ` · ${escapeHtml(c.implantingClinicPhone)}`
        : '';
      lotTraceBannerHtml = `
        <div class="chip-lot-trace-banner">
          🔬 UNREGISTERED CHIP — MANUFACTURER LOT TRACE: ${clinicName}${chartSuffix}${phoneSuffix}
        </div>
      `;
    }

    const ownerContactHtml = c.resolvedOwnerName
      ? `
        <div class="microchip-owner-contact-row">
          <strong>Resolved Owner Contact:</strong> ${escapeHtml(c.resolvedOwnerName)} (${escapeHtml(c.resolvedOwnerPhoneMask || '')})
        </div>
      `
      : '';

    const promotedBannerHtml = c.promotedSightingId
      ? `<div class="chip-promoted-sighting-banner trust-promoted-banner">📍 Promoted Sighting: ${escapeHtml(c.promotedSightingId)}</div>`
      : '';

    let actionsHtml = '';
    if (c.status === 'UNREGISTERED_BATCH_TRACED') {
      actionsHtml += `<button type="button" class="btn btn-sm btn-secondary btn-trace-batch-lot" data-case-id="${caseId}">🏥 Trace Manufacturer Batch Lot via Clinic PIMS</button>`;
    }
    if (c.status !== 'REUNION_DISPATCHED_VERIFIED') {
      actionsHtml += `<button type="button" class="btn btn-sm btn-primary btn-resolve-chip-reunion" data-case-id="${caseId}">✅ Verify Owner &amp; Dispatch Reunion Sighting</button>`;
    }

    return `
      <article class="trust-screening-card microchip-case-card" data-case-id="${caseId}" data-case-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${maskedChip})</strong>
            <code class="trust-pet-id">${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-chip-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="humane-trap-label-row">
          <strong>${mfgName}</strong>
          <span class="text-secondary">(${agency})</span>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-chip-standard">${chipStandard} • ${freqKhz} kHz</span>
          <span class="badge badge-chip-lot">Lot: ${lotId}</span>
          ${winningRegistryHtml}
        </div>

        <div class="registry-hops-list" aria-label="Federated Registry Waterfall Hops">
          ${hopsHtml}
        </div>

        ${lotTraceBannerHtml}
        ${ownerContactHtml}
        ${promotedBannerHtml}

        <p class="trust-screening-notes">${notes}</p>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  /**
   * Render all microchip scan cases in `#microchip-cases-list`
   */
  function renderAllCases(cases) {
    currentCases = cases;
    updateKpiCounters(cases);

    const countBadge = document.getElementById('microchip-cases-count-badge');
    if (countBadge) {
      countBadge.textContent = `${cases.length} Cases`;
    }

    const listEl = document.getElementById('microchip-cases-list');
    if (!listEl) return;

    if (cases.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="microchip-cases-empty-msg">No microchip federation scan cases recorded yet.</p>';
      return;
    }

    listEl.innerHTML = cases.map(renderCaseCardHtml).join('');
  }

  /**
   * Fetch all microchip scan cases from `/api/v1/microchip-federation/cases`
   */
  async function fetchCases() {
    try {
      const res = await fetch('/api/v1/microchip-federation/cases', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const cases = Array.isArray(data.cases) ? data.cases : [];
      renderAllCases(cases);
      return cases;
    } catch (_err) {
      return [];
    }
  }

  /**
   * Submit a federated microchip scan request to `/api/v1/microchip-federation/cases`
   */
  async function submitScanRequest(payload) {
    const res = await fetch('/api/v1/microchip-federation/cases', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchCases();
      showToast(
        `Scanned chip ${created.maskedChipDisplay} (${created.chipStandard} • ${Number(created.frequencyKhz).toFixed(1)} kHz) → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to run federated microchip scan', 'error');
    return null;
  }

  /**
   * Setup Universal Multi-Standard RFID Scanner Lookup Form and Simulation Presets
   */
  function setupScanFormAndSimulators() {
    const form = document.getElementById('form-scan-microchip');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petId = (document.getElementById('mc-pet-id')?.value || 'pet-2026-002').trim();
        const petName = (document.getElementById('mc-pet-name')?.value || 'Luna').trim();
        const species = (document.getElementById('mc-species')?.value || 'CAT').trim();
        const rawChipInput = (
          document.getElementById('mc-raw-chip-input')?.value || '985-141000-774123'
        ).trim();
        const scannedByAgency = (
          document.getElementById('mc-scanned-by-agency')?.value ||
          'Ballard Emergency Vet RFID Wand'
        ).trim();
        const latitude = parseFloat(document.getElementById('mc-latitude')?.value || '47.6640');
        const longitude = parseFloat(document.getElementById('mc-longitude')?.value || '-122.3980');
        const simulateMode = (
          document.getElementById('mc-simulate-mode')?.value || 'REGISTERED_PRIMARY'
        ).trim();

        const submitBtn = document.getElementById('btn-scan-microchip');
        if (submitBtn) submitBtn.disabled = true;

        try {
          await submitScanRequest({
            petId,
            petName,
            species,
            rawChipInput,
            scannedByAgency,
            latitude: Number.isNaN(latitude) ? 47.664 : latitude,
            longitude: Number.isNaN(longitude) ? -122.398 : longitude,
            simulateMode,
          });
        } catch (_err) {
          showToast('Network error scanning microchip', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simUnregisteredBtn = document.getElementById('btn-sim-unregistered-iso');
    if (simUnregisteredBtn) {
      simUnregisteredBtn.addEventListener('click', async () => {
        simUnregisteredBtn.disabled = true;
        try {
          const rawInput = document.getElementById('mc-raw-chip-input');
          const modeSelect = document.getElementById('mc-simulate-mode');
          if (rawInput) rawInput.value = '900-118000-558291';
          if (modeSelect) modeSelect.value = 'UNREGISTERED_DEAD_END';

          await submitScanRequest({
            petId: 'pet-2026-002',
            petName: 'Luna',
            species: 'CAT',
            rawChipInput: '900-118000-558291',
            scannedByAgency: 'Ballard Emergency Vet RFID Wand',
            latitude: 47.664,
            longitude: -122.398,
            simulateMode: 'UNREGISTERED_DEAD_END',
          });
        } finally {
          simUnregisteredBtn.disabled = false;
        }
      });
    }

    const simAvidBtn = document.getElementById('btn-sim-avid9-escalated');
    if (simAvidBtn) {
      simAvidBtn.addEventListener('click', async () => {
        simAvidBtn.disabled = true;
        try {
          const rawInput = document.getElementById('mc-raw-chip-input');
          const modeSelect = document.getElementById('mc-simulate-mode');
          if (rawInput) rawInput.value = '041-882-910';
          if (modeSelect) modeSelect.value = 'OUTDATED_PRIMARY_PHONE';

          await submitScanRequest({
            petId: 'pet-2026-001',
            petName: 'Buster',
            species: 'DOG',
            rawChipInput: '041-882-910',
            scannedByAgency: 'SAR Field Unit K9-04 (Halo Wand)',
            latitude: 47.6812,
            longitude: -122.3405,
            simulateMode: 'OUTDATED_PRIMARY_PHONE',
          });
        } finally {
          simAvidBtn.disabled = false;
        }
      });
    }

    const simHexBtn = document.getElementById('btn-sim-hex10-legacy');
    if (simHexBtn) {
      simHexBtn.addEventListener('click', async () => {
        simHexBtn.disabled = true;
        try {
          const rawInput = document.getElementById('mc-raw-chip-input');
          const modeSelect = document.getElementById('mc-simulate-mode');
          if (rawInput) rawInput.value = '7F7E4A19B2';
          if (modeSelect) modeSelect.value = 'REGISTERED_PRIMARY';

          await submitScanRequest({
            petId: 'pet-2026-003',
            petName: 'Cooper',
            species: 'DOG',
            rawChipInput: '7F7E4A19B2',
            scannedByAgency: 'Ballard Emergency Vet RFID Wand',
            latitude: 47.664,
            longitude: -122.398,
            simulateMode: 'REGISTERED_PRIMARY',
          });
        } finally {
          simHexBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup card action buttons in `#microchip-cases-list`
   */
  function setupCaseCardActions() {
    const listEl = document.getElementById('microchip-cases-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const traceBtn = e.target.closest('.btn-trace-batch-lot');
      const resolveBtn = e.target.closest('.btn-resolve-chip-reunion');

      const btn = traceBtn || resolveBtn;
      if (!btn) return;

      const caseId = btn.getAttribute('data-case-id');
      if (!caseId) return;

      btn.disabled = true;
      try {
        if (traceBtn) {
          const res = await fetch(
            `/api/v1/microchip-federation/cases/${encodeURIComponent(caseId)}/trace-lot`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                implantingClinicName: 'Emerald City Spay/Neuter & Wellness Clinic',
                implantingClinicPhone: '(206) 555-0148',
                implantPatientChartId: 'PIMS-CHART-9102',
                recoveredOwnerName: 'Devon & Priya Patel',
                recoveredOwnerPhoneMask: '+1 (206) •••-7724',
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchCases();
            showToast(
              `Recovered owner contact (${updated.resolvedOwnerName}) from clinic PIMS lot trace for ${caseId}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to trace manufacturer batch lot', 'error');
          }
        } else if (resolveBtn) {
          const res = await fetch(
            `/api/v1/microchip-federation/cases/${encodeURIComponent(caseId)}/resolve`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                verificationMethod: 'PRIMARY_REGISTRY_MATCH',
                notes:
                  'Verified pet owner identity via registry/clinic callback and dispatched reunion sighting.',
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchCases();
            showToast(
              `Verified ownership for ${caseId} → ${updated.status} (${updated.promotedSightingId})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to resolve microchip ownership', 'error');
          }
        }
      } catch (_err) {
        showToast('Network error updating microchip case', 'error');
      } finally {
        btn.disabled = false;
      }
    });
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
   * Subscribe to SSE stream `/api/v1/microchip-federation/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/microchip-federation/stream');
      const refresh = () => {
        fetchCases();
      };
      sseSource.addEventListener('microchip_case_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'microchip_case_updated') {
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
    setupScanFormAndSimulators();
    setupCaseCardActions();
    fetchCases();
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
