/**
 * PetSpotR Acute Non-Cardiogenic Pulmonary Edema (NCPE), Electrocution (Oral Cord Bite),
 * Upper Airway Obstruction / Post-Ictal Neurogenic Edema, Low-Dose Furosemide Controversy &
 * CPAP/PEEP Mechanical Ventilation Cockpit Controller (Milestone 24.3)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'NCPE-OP-101';
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
    const liveStatus = document.getElementById('ncpe-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('ncpe-error-banner');
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
      case 'ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE':
        return 'ncpe-status-critical';
      case 'OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED':
        return 'ncpe-status-staged';
      case 'BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED':
        return 'ncpe-status-cleared';
      default:
        return 'ncpe-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const acuteEl = document.getElementById('kpi-acute-triage');
    const stagedEl = document.getElementById('kpi-cpap-staged');
    const clearedEl = document.getElementById('kpi-cleared-room-air');
    const electrocutionEl = document.getElementById('kpi-electrocution-count');
    const severePfEl = document.getElementById('kpi-severe-pf-count');

    const total = metrics?.total_operations ?? ops.length;
    const acute =
      metrics?.acute_ncpe_triage_count ??
      ops.filter(
        (op) => op.status === 'ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE'
      ).length;
    const staged =
      metrics?.cpap_oxygen_staged_count ??
      ops.filter((op) => op.status === 'OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED')
        .length;
    const cleared =
      metrics?.cleared_room_air_count ??
      ops.filter(
        (op) => op.status === 'BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED'
      ).length;
    const electrocution =
      metrics?.oral_cord_electrocution_count ??
      ops.filter(
        (op) => op.etiology_class === 'ORAL_ELECTRICAL_CORD_BITE_NEUROGENIC'
      ).length;
    const severePf =
      metrics?.severe_pao2_fio2_under_200_count ??
      ops.filter((op) => Number(op.pao2_fio2_ratio ?? 250) < 200).length;

    if (totalEl) totalEl.textContent = String(total);
    if (acuteEl) acuteEl.textContent = String(acute);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (electrocutionEl) electrocutionEl.textContent = String(electrocution);
    if (severePfEl) severePfEl.textContent = String(severePf);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const vent = op.ventilation_plan || {};
    const oxygenInput = document.getElementById('stage-oxygen-protocol-input');
    const anxInput = document.getElementById('stage-anxiolytic-input');
    const cpapInput = document.getElementById('stage-cpap-peep-input');
    const furoInput = document.getElementById('stage-furosemide-note-input');
    const fluidInput = document.getElementById('stage-fluid-plan-input');
    const oralInput = document.getElementById('stage-oral-analgesia-input');

    if (oxygenInput && vent.oxygen_fio2_or_hfnc_protocol) {
      oxygenInput.value = vent.oxygen_fio2_or_hfnc_protocol;
    }
    if (anxInput && vent.anxiolytic_butorphanol_acepromazine) {
      anxInput.value = vent.anxiolytic_butorphanol_acepromazine;
    }
    if (cpapInput && vent.cpap_or_peep_mechanical_ventilation) {
      cpapInput.value = vent.cpap_or_peep_mechanical_ventilation;
    }
    if (furoInput && vent.low_dose_furosemide_bronchodilator_note) {
      furoInput.value = vent.low_dose_furosemide_bronchodilator_note;
    }
    if (fluidInput && vent.conservative_fluid_maintenance_plan) {
      fluidInput.value = vent.conservative_fluid_maintenance_plan;
    }
    if (oralInput && vent.oral_commissure_burn_analgesia_plan) {
      oralInput.value = vent.oral_commissure_burn_analgesia_plan;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('ncpe-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED'
    ) {
      const by = active.discharged_by || 'Attending Criticalist';
      const summary =
        active.discharge_summary ||
        'Caudodorsal B-lines resolved and room-air normoxemia confirmed.';
      if (summaryTextEl) {
        summaryTextEl.textContent = `${active.id} (${active.pet_name}) — Discharged by ${by}: ${summary}`;
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
      const weightStr = Number(active.weight_kg ?? 6.4).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Canine'} · ${active.breed || 'Mixed Breed'} · ${weightStr} kg)`;
    }

    const etiolEl = document.getElementById('active-etiology-class');
    if (etiolEl) {
      etiolEl.textContent = active.etiology_class || '';
    }
    const la2aoSubEl = document.getElementById('active-la2ao-ratio');
    if (la2aoSubEl) {
      la2aoSubEl.textContent = Number(active.la2ao_ratio ?? 1.2).toFixed(2);
    }
    const escharEl = document.getElementById('active-eschar-flag');
    if (escharEl) {
      escharEl.textContent = active.oral_electrical_eschar_present
        ? 'YES'
        : 'NO';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent =
        active.status || 'ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE';
      badgeEl.className = `ncpe-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const pfEl = document.getElementById('active-pf-ratio');
    if (pfEl) {
      pfEl.textContent = Number(active.pao2_fio2_ratio ?? 0).toFixed(1);
    }
    const blinesEl = document.getElementById('active-blines-count');
    if (blinesEl) {
      blinesEl.textContent = `${Number(active.caudodorsal_blines_count ?? 0)} B-Lines`;
    }
    const spo2El = document.getElementById('active-spo2');
    if (spo2El) {
      spo2El.textContent = `${Number(active.spo2_percent ?? 0).toFixed(1)}%`;
    }
    const rrLaEl = document.getElementById('active-rr-la2ao');
    if (rrLaEl) {
      const rr = Number(active.respiratory_rate_bpm ?? 0);
      const la2ao = Number(active.la2ao_ratio ?? 0).toFixed(2);
      rrLaEl.textContent = `${rr} bpm · LA:Ao ${la2ao}`;
    }

    const assess = active.assessment_plan || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-etiology-patho', assess.etiology_pathophysiology || '');
    setText('plan-pocus-note', assess.caudodorsal_vs_perihilar_pocus_note || '');
    setText('plan-la2ao-ruleout', assess.normal_la2ao_cardiogenic_rule_out || '');
    setText(
      'plan-furosemide-guard',
      assess.high_dose_furosemide_contraindication || ''
    );
    setText(
      'plan-oral-eschar',
      assess.oral_electrical_burn_delayed_necrosis || ''
    );
    setText(
      'plan-fluid-warning',
      assess.fluid_overload_capillary_leak_warning || ''
    );

    const vent = active.ventilation_plan || {};
    setText('plan-oxygen-protocol', vent.oxygen_fio2_or_hfnc_protocol || '');
    setText('plan-anxiolytic', vent.anxiolytic_butorphanol_acepromazine || '');
    setText(
      'plan-low-furosemide',
      vent.low_dose_furosemide_bronchodilator_note || ''
    );
    setText('plan-cpap-peep', vent.cpap_or_peep_mechanical_ventilation || '');
    setText(
      'plan-conservative-fluid',
      vent.conservative_fluid_maintenance_plan || ''
    );
    setText('plan-oral-analgesia', vent.oral_commissure_burn_analgesia_plan || '');

    const checks = Array.isArray(active.respiratory_checks)
      ? active.respiratory_checks
      : [];
    const checksCountEl = document.getElementById('respiratory-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('respiratory-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const pf = Number(chk.pao2_fio2_ratio ?? 0).toFixed(1);
          const spo2 = Number(chk.spo2_percent ?? 0).toFixed(1);
          const rr = Number(chk.respiratory_rate_bpm ?? 0);
          const blines = Number(chk.caudodorsal_blines_count ?? 0);
          const la2ao = Number(chk.la2ao_ratio ?? 0).toFixed(2);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="ncpe-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · PaO2/FiO2 ${pf} · SpO2 ${spo2}% · RR ${rr} bpm · Caudodorsal B-Lines ${blines} · LA:Ao ${la2ao} — ${note}</li>`;
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
    const weightKg = Number(op.weight_kg ?? 6.4).toFixed(1);
    const status = escapeHtml(
      op.status || 'ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE'
    );
    const badgeClass = getStatusBadgeClass(op.status);
    const etiolClass = escapeHtml(op.etiology_class || '');
    const pf = Number(op.pao2_fio2_ratio ?? 0).toFixed(1);
    const spo2 = Number(op.spo2_percent ?? 0).toFixed(1);
    const rr = Number(op.respiratory_rate_bpm ?? 0);
    const blines = Number(op.caudodorsal_blines_count ?? 0);
    const la2ao = Number(op.la2ao_ratio ?? 0).toFixed(2);
    const escharText = op.oral_electrical_eschar_present ? 'YES' : 'NO';

    return `
      <article
        class="trust-screening-card ncpe-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-etiology-class="${etiolClass}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="ncpe-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="ncpe-card-metrics">
          <span><strong>Etiology:</strong> ${etiolClass}</span>
          <span><strong>PaO2/FiO2:</strong> ${pf}</span>
          <span><strong>SpO2:</strong> ${spo2}%</span>
          <span><strong>RR:</strong> ${rr} bpm</span>
          <span><strong>Caudodorsal B-Lines:</strong> ${blines}</span>
          <span><strong>LA:Ao Ratio:</strong> ${la2ao}</span>
          <span><strong>Oral Eschar:</strong> ${escharText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary ncpe-select-op-btn" data-op-id="${opId}">
            🔍 Open NCPE Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('ncpe-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('ncpe-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="ncpe-ops-empty-msg">No NCPE &amp; Electrocution operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('ncpe-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/ncpe-electrocution/operations?${qs}`
        : '/api/ncpe-electrocution/operations';

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
      renderAllOperations(ops, data.metrics);
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
        `/api/ncpe-electrocution/operations/${encodeURIComponent(opId)}`,
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
        statusBadge.className = 'ncpe-status-badge ncpe-status-cleared';
      }
      eventSource = new EventSource('/api/ncpe-electrocution/stream');
      eventSource.addEventListener('ncpe_electrocution_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('ncpe-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-ncpe-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value ||
          'CASE-NCPE-104'
        ).trim();
        const petName = (
          document.getElementById('create-pet-name-input')?.value || ''
        ).trim();
        const species = (
          document.getElementById('create-species-input')?.value || 'Canine'
        ).trim();
        const breed = (
          document.getElementById('create-breed-input')?.value ||
          'Labrador Retriever Puppy'
        ).trim();
        const weightKg = parseFloat(
          document.getElementById('create-weight-input')?.value || '7.2'
        );
        const etiologyClass = (
          document.getElementById('create-etiology-input')?.value ||
          'ORAL_ELECTRICAL_CORD_BITE_NEUROGENIC'
        ).trim();
        const pfRatio = parseFloat(
          document.getElementById('create-pf-ratio-input')?.value || '155.0'
        );
        const spo2 = parseFloat(
          document.getElementById('create-spo2-input')?.value || '85.0'
        );
        const rr = parseInt(
          document.getElementById('create-rr-input')?.value || '72',
          10
        );
        const blines = parseInt(
          document.getElementById('create-blines-input')?.value || '16',
          10
        );
        const la2ao = parseFloat(
          document.getElementById('create-la2ao-input')?.value || '1.20'
        );
        const escharPresent = Boolean(
          document.getElementById('create-eschar-checkbox')?.checked
        );
        const frothPresent = Boolean(
          document.getElementById('create-froth-checkbox')?.checked
        );

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 7.2 : weightKg,
          etiology_class: etiologyClass,
          pao2_fio2_ratio: isNaN(pfRatio) ? 155.0 : pfRatio,
          spo2_percent: isNaN(spo2) ? 85.0 : spo2,
          respiratory_rate_bpm: isNaN(rr) ? 72 : rr,
          caudodorsal_blines_count: isNaN(blines) ? 16 : blines,
          la2ao_ratio: isNaN(la2ao) ? 1.2 : la2ao,
          oral_electrical_eschar_present: escharPresent,
          pink_frothy_airway_fluid: frothPresent,
        };

        const btn = document.getElementById('create-ncpe-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/ncpe-electrocution/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted NCPE / Electrocution patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to admit NCPE / Electrocution patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-cpap-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const oxygen = (
          document.getElementById('stage-oxygen-protocol-input')?.value ||
          'High-Flow Nasal Cannula (HFNC) at 1.5–2.0 L/kg/min warm humidified O2 (FiO2 60%)'
        ).trim();
        const anxiolytic = (
          document.getElementById('stage-anxiolytic-input')?.value ||
          'Butorphanol 0.2 mg/kg IV/IM q4h +/- Acepromazine 0.01 mg/kg IV'
        ).trim();
        const cpap = (
          document.getElementById('stage-cpap-peep-input')?.value ||
          'Helmet CPAP (6 cmH2O PEEP) or Mechanical Ventilation with PEEP 5–10 cmH2O'
        ).trim();
        const furosemide = (
          document.getElementById('stage-furosemide-note-input')?.value || ''
        ).trim();
        const fluid = (
          document.getElementById('stage-fluid-plan-input')?.value || ''
        ).trim();
        const oral = (
          document.getElementById('stage-oral-analgesia-input')?.value || ''
        ).trim();

        const payload = {
          oxygen_fio2_or_hfnc_protocol: oxygen,
          anxiolytic_butorphanol_acepromazine: anxiolytic,
          low_dose_furosemide_bronchodilator_note: furosemide,
          cpap_or_peep_mechanical_ventilation: cpap,
          conservative_fluid_maintenance_plan: fluid,
          oral_commissure_burn_analgesia_plan: oral,
        };

        const btn = document.getElementById('stage-cpap-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/ncpe-electrocution/operations/${encodeURIComponent(selectedOpId)}/stage-cpap`,
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
              `Staged O2 / Butorphanol / CPAP / PEEP protocol for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to stage CPAP ventilation protocol';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-respiratory-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const pfRatio = parseFloat(
          document.getElementById('check-pf-ratio-input')?.value || '395.0'
        );
        const spo2 = parseFloat(
          document.getElementById('check-spo2-input')?.value || '98.0'
        );
        const rr = parseInt(
          document.getElementById('check-rr-input')?.value || '28',
          10
        );
        const blines = parseInt(
          document.getElementById('check-blines-input')?.value || '0',
          10
        );
        const la2ao = parseFloat(
          document.getElementById('check-la2ao-input')?.value || '1.22'
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Nadia Varma, DACVECC'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          pao2_fio2_ratio: isNaN(pfRatio) ? 395.0 : pfRatio,
          spo2_percent: isNaN(spo2) ? 98.0 : spo2,
          respiratory_rate_bpm: isNaN(rr) ? 28 : rr,
          caudodorsal_blines_count: isNaN(blines) ? 0 : blines,
          la2ao_ratio: isNaN(la2ao) ? 1.22 : la2ao,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-respiratory-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/ncpe-electrocution/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
              `Recorded respiratory & VetBLUE B-line check for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record respiratory check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-ncpe-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dischargedBy = (
          document.getElementById('clear-discharged-by-input')?.value ||
          'Dr. Nadia Varma, DACVECC'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          discharged_by: dischargedBy,
          discharge_summary: summary,
        };

        const btn = document.getElementById('clear-ncpe-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/ncpe-electrocution/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Room-Air Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to clear NCPE operation for discharge';
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
    const gridEl = document.getElementById('ncpe-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.ncpe-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.ncpe-op-card');
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
