/**
 * PetSpotR Acute Tetanus (Clostridium tetani) & Botulism (Clostridium botulinum),
 * Risus Sardonicus / Sawhorse Stance vs. Ascending Flaccid Lower Motor Neuron Paralysis,
 * Equine Tetanus Antitoxin (TAT) Intradermal Test, Metronidazole (vs. Penicillin GABA Antagonism) &
 * Dark/Quiet Sensory-Isolation Cockpit Controller (Milestone 25.2)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'CLB-OP-101';
  let eventSource = null;

  function escapeHtml(str) {
    if (str === null || str === undefined) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

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
    const liveStatus = document.getElementById('clb-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('clb-error-banner');
    if (!banner) return;
    if (!message) {
      banner.textContent = '';
      banner.hidden = true;
      return;
    }
    banner.textContent = message;
    banner.hidden = false;
  }

  function getStatusBadgeClass(status) {
    switch (status) {
      case 'METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED':
        return 'clb-status-critical';
      case 'SENSORY_ISOLATION_VENTILATION_MONITORED':
        return 'clb-status-staged';
      case 'NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED':
        return 'clb-status-cleared';
      default:
        return 'clb-status-critical';
    }
  }

  function updateKPIs(ops, overview) {
    const totalEl = document.getElementById('kpi-total-ops');
    const stagedEl = document.getElementById('kpi-tat-staged');
    const monitoredEl = document.getElementById('kpi-isolation-monitored');
    const clearedEl = document.getElementById('kpi-normoreflexic-cleared');
    const severeEl = document.getElementById('kpi-severe-burkitt');
    const botulismEl = document.getElementById('kpi-botulism-lmn');

    const total = overview?.total_operations ?? ops.length;
    const staged =
      overview?.metronidazole_tat_staged_count ??
      ops.filter((op) => op.status === 'METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED').length;
    const monitored =
      overview?.sensory_isolation_monitored_count ??
      ops.filter((op) => op.status === 'SENSORY_ISOLATION_VENTILATION_MONITORED').length;
    const cleared =
      overview?.normoreflexic_cleared_count ??
      ops.filter((op) => op.status === 'NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED').length;
    const severe =
      overview?.severe_burkitt_class_iii_iv_count ??
      ops.filter((op) => Number(op.burkitt_severity_class ?? 0) >= 3).length;
    const botulism =
      overview?.botulism_lmn_paralysis_count ??
      ops.filter(
        (op) => op.syndrome === 'BOTULISM_ASCENDING_FLACCID_LMN_QUADRIPLEGIA'
      ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (monitoredEl) monitoredEl.textContent = String(monitored);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (severeEl) severeEl.textContent = String(severe);
    if (botulismEl) botulismEl.textContent = String(botulism);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const protoInput = document.getElementById('stage-protocol-input');
    const metroInput = document.getElementById('stage-metronidazole-input');
    const tatInput = document.getElementById('stage-tat-units-input');
    const methoInput = document.getElementById('stage-methocarbamol-input');

    if (protoInput && op.protocol) {
      protoInput.value = op.protocol;
    }
    if (metroInput && op.metronidazole_mg_kg_bid !== undefined) {
      metroInput.value = String(op.metronidazole_mg_kg_bid);
    }
    if (tatInput && op.equine_tat_units_kg !== undefined) {
      tatInput.value = String(op.equine_tat_units_kg);
    }
    if (methoInput && op.methocarbamol_mg_kg_iv !== undefined) {
      methoInput.value = String(op.methocarbamol_mg_kg_iv);
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('clb-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (active && active.status === 'NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED') {
      const by =
        active.cleared_by ||
        active.clearance?.cleared_by ||
        'Attending Veterinary Neurologist';
      const summary =
        active.clearance_summary ||
        active.clearance?.clearance_summary ||
        '0 spasms/hr, intact pharyngeal gag reflex, normocapnia, and ambulatory normothermia verified.';
      if (summaryTextEl) {
        summaryTextEl.textContent = `${active.id} (${active.pet_name}) — Cleared by ${by}: ${summary}`;
      }
      clearanceBanner.hidden = false;
    } else {
      clearanceBanner.hidden = true;
    }
  }

  function renderActiveWorkspace(ops) {
    if (!ops || ops.length === 0) {
      updateClearanceBanner(null);
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;

    syncFormsWithActiveOperation(active);
    updateClearanceBanner(active);

    const titleEl = document.getElementById('active-op-title');
    if (titleEl) {
      const weightStr = Number(active.weight_kg ?? 32.0).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Canine'} · ${active.breed || 'Mixed Breed'} · ${weightStr} kg)`;
    }

    const synEl = document.getElementById('active-syndrome');
    if (synEl) {
      synEl.textContent = active.syndrome || '';
    }
    const protoEl = document.getElementById('active-protocol');
    if (protoEl) {
      protoEl.textContent = active.protocol || '';
    }
    const gagEl = document.getElementById('active-gag-reflex');
    if (gagEl) {
      gagEl.textContent = active.gag_reflex_intact ? 'INTACT' : 'ABSENT';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent =
        active.status || 'METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED';
      badgeEl.className = `clb-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const burkittEl = document.getElementById('active-burkitt-class');
    if (burkittEl) {
      burkittEl.textContent = `Class ${Number(active.burkitt_severity_class ?? 1)}/4`;
    }
    const spasmEl = document.getElementById('active-spasm-frequency');
    if (spasmEl) {
      spasmEl.textContent = `${Number(active.spasm_frequency_per_hr ?? 0)} spasms/hr`;
    }
    const paco2TempEl = document.getElementById('active-paco2-temp');
    if (paco2TempEl) {
      const paco2 = Number(active.paco2_mmhg ?? 40.0).toFixed(1);
      const temp = Number(active.core_temp_celsius ?? 38.5).toFixed(1);
      paco2TempEl.textContent = `${paco2} mmHg / ${temp}°C`;
    }
    const metroTatEl = document.getElementById('active-metro-tat');
    if (metroTatEl) {
      const metro = Number(active.metronidazole_mg_kg_bid ?? 10.0).toFixed(1);
      const tat = Number(active.equine_tat_units_kg ?? 0).toFixed(0);
      metroTatEl.textContent = `${metro} mg/kg BID / ${tat} IU/kg`;
    }

    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-pathophysiology', active.pathophysiology_summary || '');
    setText('plan-penicillin-gaba', active.penicillin_gaba_antagonism_alert || '');
    setText('plan-tat-guard', active.equine_tat_intradermal_test_guard || '');
    setText('plan-aminoglycoside-guard', active.aminoglycoside_botulism_alert || '');
    setText('plan-sensory-isolation', active.sensory_isolation_cockpit_plan || '');

    const checks = Array.isArray(active.neuro_checks) ? active.neuro_checks : [];
    const checksCountEl = document.getElementById('neuro-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('neuro-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const bClass = Number(chk.burkitt_severity_class ?? 1);
          const spasms = Number(chk.spasm_frequency_per_hr ?? 0);
          const gag = chk.gag_reflex_intact ? 'INTACT' : 'ABSENT';
          const temp = Number(chk.core_temp_celsius ?? 38.5).toFixed(1);
          const paco2 = Number(chk.paco2_mmhg ?? 40.0).toFixed(1);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="clb-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · Burkitt Class ${bClass}/4 · Spasms ${spasms}/hr · Gag ${gag} · Temp ${temp}°C · PaCO2 ${paco2} mmHg — ${note}</li>`;
        })
        .join('');
    }
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const caseId = escapeHtml(op.case_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const breed = escapeHtml(op.breed || 'Mixed Breed');
    const weightKg = Number(op.weight_kg ?? 32.0).toFixed(1);
    const status = escapeHtml(
      op.status || 'METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED'
    );
    const badgeClass = getStatusBadgeClass(op.status);
    const syndrome = escapeHtml(op.syndrome || '');
    const burkitt = Number(op.burkitt_severity_class ?? 1);
    const spasms = Number(op.spasm_frequency_per_hr ?? 0);
    const gagText = op.gag_reflex_intact ? 'INTACT' : 'ABSENT';
    const temp = Number(op.core_temp_celsius ?? 38.5).toFixed(1);
    const paco2 = Number(op.paco2_mmhg ?? 40.0).toFixed(1);
    const metro = Number(op.metronidazole_mg_kg_bid ?? 10.0).toFixed(1);
    const tat = Number(op.equine_tat_units_kg ?? 0).toFixed(0);

    return `
      <article
        class="trust-screening-card clb-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-syndrome="${syndrome}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="clb-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="clb-card-metrics">
          <span><strong>Syndrome:</strong> ${syndrome}</span>
          <span><strong>Burkitt Class:</strong> Class ${burkitt}/4</span>
          <span><strong>Spasms:</strong> ${spasms}/hr</span>
          <span><strong>Gag Reflex:</strong> ${gagText}</span>
          <span><strong>Core Temp:</strong> ${temp}°C</span>
          <span><strong>PaCO2:</strong> ${paco2} mmHg</span>
          <span><strong>Metronidazole:</strong> ${metro} mg/kg BID</span>
          <span><strong>Equine TAT:</strong> ${tat} IU/kg</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary clb-select-op-btn" data-op-id="${opId}">
            🔍 Open Neurotoxin Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, overview) {
    currentOps = ops;
    updateKPIs(ops, overview);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('clb-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('clb-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="clb-ops-empty-msg">No Tetanus &amp; Botulism operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('clb-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/tetanus-botulism/operations?${qs}`
        : '/api/tetanus-botulism/operations';

      const res = await fetch(url, {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const ops = Array.isArray(data)
        ? data
        : Array.isArray(data.operations)
        ? data.operations
        : Array.isArray(data.ops)
        ? data.ops
        : [];
      renderAllOperations(ops, data.overview || data.metrics);
      return ops;
    } catch (_err) {
      return [];
    }
  }

  async function selectOperationById(opId) {
    if (!opId) return;
    selectedOpId = opId;
    try {
      const res = await fetch(
        `/api/tetanus-botulism/operations/${encodeURIComponent(opId)}`,
        {
          headers: { Accept: 'application/json' },
        }
      );
      if (res.ok) {
        const op = await res.json();
        const idx = currentOps.findIndex((item) => item.id === op.id);
        if (idx >= 0) {
          currentOps[idx] = op;
        } else {
          currentOps.unshift(op);
        }
      }
    } catch (_err) {
      // Fallback to cached currentOps
    }
    renderActiveWorkspace(currentOps);
  }

  function setupSSEButton() {
    const connectBtn = document.getElementById('connect-sse-btn');
    const statusBadge = document.getElementById('sse-status-badge');
    if (!connectBtn) return;

    connectBtn.addEventListener('click', () => {
      if (eventSource) {
        eventSource.close();
        eventSource = null;
      }
      if (statusBadge) {
        statusBadge.textContent = 'SSE Connected';
        statusBadge.className = 'clb-status-badge clb-status-cleared';
      }
      eventSource = new EventSource('/api/tetanus-botulism/stream');
      eventSource.addEventListener('tetanus_botulism_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('clb-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-clb-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value ||
          'CASE-CLB-104'
        ).trim();
        const petName = (
          document.getElementById('create-pet-name-input')?.value || ''
        ).trim();
        const species = (
          document.getElementById('create-species-input')?.value || 'Canine'
        ).trim();
        const breed = (
          document.getElementById('create-breed-input')?.value || 'Rottweiler'
        ).trim();
        const weightKg = parseFloat(
          document.getElementById('create-weight-input')?.value || '34.0'
        );
        const syndrome = (
          document.getElementById('create-syndrome-input')?.value ||
          'TETANUS_CLASS_II_GENERALIZED_SAWHORSE'
        ).trim();
        const protocol = (
          document.getElementById('create-protocol-input')?.value ||
          'EQUINE_TAT_100_1000_IU_KG_AFTER_ID_TEST'
        ).trim();
        const burkittClass = parseInt(
          document.getElementById('create-burkitt-class-input')?.value || '2',
          10
        );
        const spasmFreq = parseInt(
          document.getElementById('create-spasm-freq-input')?.value || '4',
          10
        );
        const coreTemp = parseFloat(
          document.getElementById('create-core-temp-input')?.value || '39.6'
        );
        const paCO2 = parseFloat(
          document.getElementById('create-paco2-input')?.value || '46.0'
        );
        const hrBPM = parseInt(
          document.getElementById('create-hr-input')?.value || '136',
          10
        );
        const metroDose = parseFloat(
          document.getElementById('create-metronidazole-input')?.value || '10.0'
        );
        const tatDose = parseFloat(
          document.getElementById('create-tat-units-input')?.value || '400'
        );
        const methoDose = parseFloat(
          document.getElementById('create-methocarbamol-input')?.value || '50'
        );
        const gagIntact = Boolean(
          document.getElementById('create-gag-reflex-checkbox')?.checked
        );
        const idVerified = Boolean(
          document.getElementById('create-id-test-checkbox')?.checked
        );
        const penAcknowledged = Boolean(
          document.getElementById('create-penicillin-gaba-checkbox')?.checked
        );
        const earplugsActive = Boolean(
          document.getElementById('create-earplugs-checkbox')?.checked
        );
        const aminoGuard = Boolean(
          document.getElementById('create-aminoglycoside-checkbox')?.checked
        );

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 34.0 : weightKg,
          syndrome,
          protocol,
          burkitt_severity_class: isNaN(burkittClass) ? 2 : burkittClass,
          gag_reflex_intact: gagIntact,
          spasm_frequency_per_hr: isNaN(spasmFreq) ? 4 : spasmFreq,
          core_temp_celsius: isNaN(coreTemp) ? 39.6 : coreTemp,
          paco2_mmhg: isNaN(paCO2) ? 46.0 : paCO2,
          heart_rate_bpm: isNaN(hrBPM) ? 136 : hrBPM,
          metronidazole_mg_kg_bid: isNaN(metroDose) ? 10.0 : metroDose,
          equine_tat_units_kg: isNaN(tatDose) ? 400 : tatDose,
          methocarbamol_mg_kg_iv: isNaN(methoDose) ? 50 : methoDose,
          intradermal_skin_test_negative_verified: idVerified,
          penicillin_gaba_warning_acknowledged: penAcknowledged,
          dark_quiet_cotton_earplugs_active: earplugsActive,
          aminoglycoside_contraindicated_in_botulism: aminoGuard,
        };

        const btn = document.getElementById('create-clb-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/tetanus-botulism/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted Tetanus / Botulism patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to admit Tetanus / Botulism patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-therapy-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const protocol = (
          document.getElementById('stage-protocol-input')?.value ||
          'EQUINE_TAT_100_1000_IU_KG_AFTER_ID_TEST'
        ).trim();
        const metroDose = parseFloat(
          document.getElementById('stage-metronidazole-input')?.value || '10.0'
        );
        const tatDose = parseFloat(
          document.getElementById('stage-tat-units-input')?.value || '500'
        );
        const methoDose = parseFloat(
          document.getElementById('stage-methocarbamol-input')?.value || '55'
        );
        const woundNote = (
          document.getElementById('stage-wound-note-input')?.value || ''
        ).trim();
        const stagedBy = (
          document.getElementById('stage-staged-by-input')?.value ||
          'Dr. Ingrid Vance, DACVIM (Neurology)'
        ).trim();
        const idVerified = Boolean(
          document.getElementById('stage-id-test-checkbox')?.checked
        );
        const penAcknowledged = Boolean(
          document.getElementById('stage-penicillin-gaba-checkbox')?.checked
        );
        const earplugsActive = Boolean(
          document.getElementById('stage-earplugs-checkbox')?.checked
        );
        const aminoGuard = Boolean(
          document.getElementById('stage-aminoglycoside-checkbox')?.checked
        );

        const payload = {
          protocol,
          metronidazole_mg_kg_bid: isNaN(metroDose) ? 10.0 : metroDose,
          equine_tat_units_kg: isNaN(tatDose) ? 500 : tatDose,
          methocarbamol_mg_kg_iv: isNaN(methoDose) ? 55 : methoDose,
          intradermal_skin_test_negative_verified: idVerified,
          penicillin_gaba_warning_acknowledged: penAcknowledged,
          dark_quiet_cotton_earplugs_active: earplugsActive,
          aminoglycoside_contraindicated_in_botulism: aminoGuard,
          wound_infiltration_prior_to_debridement_note: woundNote,
          staged_by: stagedBy,
        };

        const btn = document.getElementById('stage-therapy-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/tetanus-botulism/operations/${encodeURIComponent(selectedOpId)}/therapy`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(payload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Staged Metronidazole / Equine TAT / Methocarbamol protocol for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to stage Tetanus / Botulism therapy';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-neuro-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const burkittClass = parseInt(
          document.getElementById('check-burkitt-class-input')?.value || '1',
          10
        );
        const spasmFreq = parseInt(
          document.getElementById('check-spasm-freq-input')?.value || '0',
          10
        );
        const coreTemp = parseFloat(
          document.getElementById('check-core-temp-input')?.value || '38.5'
        );
        const paCO2 = parseFloat(
          document.getElementById('check-paco2-input')?.value || '38.0'
        );
        const hrBPM = parseInt(
          document.getElementById('check-hr-input')?.value || '88',
          10
        );
        const gagIntact = Boolean(
          document.getElementById('check-gag-reflex-checkbox')?.checked
        );
        const earplugsActive = Boolean(
          document.getElementById('check-earplugs-checkbox')?.checked
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Ingrid Vance, DACVIM (Neurology)'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          burkitt_severity_class: isNaN(burkittClass) ? 1 : burkittClass,
          spasm_frequency_per_hr: isNaN(spasmFreq) ? 0 : spasmFreq,
          gag_reflex_intact: gagIntact,
          core_temp_celsius: isNaN(coreTemp) ? 38.5 : coreTemp,
          paco2_mmhg: isNaN(paCO2) ? 38.0 : paCO2,
          heart_rate_bpm: isNaN(hrBPM) ? 88 : hrBPM,
          dark_quiet_cotton_earplugs_active: earplugsActive,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-neuro-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/tetanus-botulism/operations/${encodeURIComponent(selectedOpId)}/neuro-check`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(payload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Recorded neurological & ventilation check for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record neurological check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-clb-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const spasmFreq = parseInt(
          document.getElementById('clear-spasm-freq-input')?.value || '0',
          10
        );
        const paCO2 = parseFloat(
          document.getElementById('clear-paco2-input')?.value || '38.0'
        );
        const gagIntact = Boolean(
          document.getElementById('clear-gag-reflex-checkbox')?.checked
        );
        const ambulatory = Boolean(
          document.getElementById('clear-ambulatory-checkbox')?.checked
        );
        const clearedBy = (
          document.getElementById('clear-cleared-by-input')?.value ||
          'Dr. Ingrid Vance, DACVIM (Neurology)'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          spasm_frequency_per_hr: isNaN(spasmFreq) ? 0 : spasmFreq,
          gag_reflex_intact: gagIntact,
          paco2_mmhg: isNaN(paCO2) ? 38.0 : paCO2,
          ambulatory_normothermic_verified: ambulatory,
          cleared_by: clearedBy,
          clearance_summary: summary,
        };

        const btn = document.getElementById('clear-clb-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/tetanus-botulism/operations/${encodeURIComponent(selectedOpId)}/clearance`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(payload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Cleared ${updated.id} (${updated.pet_name}) for Normoreflexic & Swallowing-Intact Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error ||
              'Failed to clear Tetanus / Botulism operation for discharge';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }
  }

  function setupCardInteractions() {
    const gridEl = document.getElementById('clb-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.clb-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.clb-op-card');
      if (card) {
        const opId = card.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
      }
    });
  }

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

  function init() {
    setupThemeToggle();
    setupSSEButton();
    setupFormsAndFilters();
    setupCardInteractions();
    fetchOperations();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
