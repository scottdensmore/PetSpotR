/**
 * PetSpotR Acute Coagulopathy & Disseminated Intravascular Coagulation (DIC),
 * PT/aPTT Prolongation, D-Dimer/FDP Elevation, Fresh Frozen Plasma (FFP) &
 * Tranexamic Acid (TXA) Cockpit Controller (Milestone 23.3)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'DIC-OP-101';
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
    const liveStatus = document.getElementById('dic-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('dic-error-banner');
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
      case 'ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE':
        return 'dic-status-critical';
      case 'FFP_CRYOPRECIPITATE_TXA_STAGED':
        return 'dic-status-staged';
      case 'HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED':
        return 'dic-status-cleared';
      default:
        return 'dic-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const overtEl = document.getElementById('kpi-overt-dic');
    const stagedEl = document.getElementById('kpi-ffp-txa-staged');
    const clearedEl = document.getElementById('kpi-cleared-normocoagulable');
    const consumptiveEl = document.getElementById('kpi-consumptive-hemorrhagic');
    const greyhoundEl = document.getElementById('kpi-hyperfibrinolysis-greyhound');

    const total = metrics?.total_operations ?? ops.length;
    const overt =
      metrics?.overt_dic_triage_count ??
      ops.filter((op) => op.status === 'ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE').length;
    const staged =
      metrics?.ffp_txa_staged_count ??
      ops.filter((op) => op.status === 'FFP_CRYOPRECIPITATE_TXA_STAGED').length;
    const cleared =
      metrics?.cleared_normocoagulable_count ??
      ops.filter((op) => op.status === 'HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED').length;
    const consumptive =
      metrics?.consumptive_hemorrhagic_count ??
      ops.filter((op) => op.phase_phenotype === 'HYPO_COAGULABLE_HEMORRHAGIC_CONSUMPTIVE_DIC')
        .length;
    const greyhound =
      metrics?.hyperfibrinolysis_greyhound_count ??
      ops.filter(
        (op) => op.phase_phenotype === 'HYPERFIBRINOLYSIS_GREYHOUND_TRAUMA_COAGULOPATHY'
      ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (overtEl) overtEl.textContent = String(overt);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (consumptiveEl) consumptiveEl.textContent = String(consumptive);
    if (greyhoundEl) greyhoundEl.textContent = String(greyhound);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const comp = op.component_plan || {};
    const ffpInput = document.getElementById('stage-ffp-protocol-input');
    const cryoInput = document.getElementById('stage-cryo-protocol-input');
    const txaInput = document.getElementById('stage-txa-plan-input');
    const hepInput = document.getElementById('stage-heparin-phase-input');
    const trigInput = document.getElementById('stage-trigger-control-input');
    const schedInput = document.getElementById('stage-coag-schedule-input');

    if (ffpInput && comp.fresh_frozen_plasma_protocol) {
      ffpInput.value = comp.fresh_frozen_plasma_protocol;
    }
    if (cryoInput && comp.cryoprecipitate_vwf_protocol) {
      cryoInput.value = comp.cryoprecipitate_vwf_protocol;
    }
    if (txaInput && comp.tranexamic_acid_aminocaproic_plan) {
      txaInput.value = comp.tranexamic_acid_aminocaproic_plan;
    }
    if (hepInput && comp.antithrombin_heparin_phase_plan) {
      hepInput.value = comp.antithrombin_heparin_phase_plan;
    }
    if (trigInput && comp.underlying_trigger_control_plan) {
      trigInput.value = comp.underlying_trigger_control_plan;
    }
    if (schedInput && comp.serial_coag_panel_schedule) {
      schedInput.value = comp.serial_coag_panel_schedule;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('dic-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED'
    ) {
      const by = active.discharged_by || 'Attending Criticalist';
      const summary =
        active.discharge_summary ||
        'Hemostasis restored and normocoagulable profile verified.';
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
      const weightStr = Number(active.weight_kg ?? 25.0).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Canine'} · ${active.breed || 'Mixed Breed'} · ${weightStr} kg)`;
    }

    const phenoEl = document.getElementById('active-phase-phenotype');
    if (phenoEl) {
      phenoEl.textContent = active.phase_phenotype || '';
    }
    const at3El = document.getElementById('active-at3-percent');
    if (at3El) {
      at3El.textContent = `${Number(active.antithrombin_percent ?? 80.0).toFixed(1)}%`;
    }
    const schistoEl = document.getElementById('active-schistocytes-flag');
    if (schistoEl) {
      schistoEl.textContent = active.schistocytes_present ? 'YES' : 'NO';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent = active.status || 'ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE';
      badgeEl.className = `dic-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const pltEl = document.getElementById('active-platelet-count');
    if (pltEl) {
      pltEl.textContent = `${Number(active.platelet_count_k_ul ?? 0).toFixed(1)} K/µL`;
    }
    const ptApttEl = document.getElementById('active-pt-aptt');
    if (ptApttEl) {
      const pt = Number(active.pt_seconds ?? 0).toFixed(1);
      const aptt = Number(active.aptt_seconds ?? 0).toFixed(1);
      ptApttEl.textContent = `${pt}s / ${aptt}s`;
    }
    const ddimerEl = document.getElementById('active-d-dimer');
    if (ddimerEl) {
      ddimerEl.textContent = `${Number(active.d_dimer_ng_ml ?? 0).toFixed(1)} ng/mL`;
    }
    const fibEl = document.getElementById('active-fibrinogen');
    if (fibEl) {
      fibEl.textContent = `${Number(active.fibrinogen_mg_dl ?? 0).toFixed(1)} mg/dL`;
    }

    const prof = active.profile_plan || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-phenotype-patho', prof.phenotype_pathophysiology || '');
    setText('plan-pt-aptt-summary', prof.pt_aptt_prolongation_summary || '');
    setText('plan-ddimer-fib-at3', prof.d_dimer_fibrinogen_atiii_note || '');
    setText('plan-teg-interpretation', prof.teg_viscoelastic_interpretation || '');
    setText('plan-txa-indication', prof.txa_fibrinolysis_indication_note || '');
    setText('plan-jugular-guard', prof.jugular_venipuncture_contraindication || '');

    const comp = active.component_plan || {};
    setText('plan-ffp-protocol', comp.fresh_frozen_plasma_protocol || '');
    setText('plan-cryo-protocol', comp.cryoprecipitate_vwf_protocol || '');
    setText('plan-txa-protocol', comp.tranexamic_acid_aminocaproic_plan || '');
    setText('plan-heparin-phase', comp.antithrombin_heparin_phase_plan || '');
    setText('plan-trigger-control', comp.underlying_trigger_control_plan || '');
    setText('plan-coag-schedule', comp.serial_coag_panel_schedule || '');

    const checks = Array.isArray(active.coag_checks) ? active.coag_checks : [];
    const checksCountEl = document.getElementById('coag-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('coag-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const plt = Number(chk.platelet_count_k_ul ?? 0).toFixed(1);
          const pt = Number(chk.pt_seconds ?? 0).toFixed(1);
          const aptt = Number(chk.aptt_seconds ?? 0).toFixed(1);
          const ddimer = Number(chk.d_dimer_ng_ml ?? 0).toFixed(1);
          const fib = Number(chk.fibrinogen_mg_dl ?? 0).toFixed(1);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="dic-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · Platelets ${plt} K/µL · PT ${pt}s · aPTT ${aptt}s · D-Dimer ${ddimer} ng/mL · Fibrinogen ${fib} mg/dL — ${note}</li>`;
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
    const weightKg = Number(op.weight_kg ?? 25.0).toFixed(1);
    const status = escapeHtml(op.status || 'ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE');
    const badgeClass = getStatusBadgeClass(op.status);
    const pheno = escapeHtml(op.phase_phenotype || '');
    const plt = Number(op.platelet_count_k_ul ?? 0).toFixed(1);
    const pt = Number(op.pt_seconds ?? 0).toFixed(1);
    const aptt = Number(op.aptt_seconds ?? 0).toFixed(1);
    const ddimer = Number(op.d_dimer_ng_ml ?? 0).toFixed(1);
    const fib = Number(op.fibrinogen_mg_dl ?? 0).toFixed(1);
    const at3 = Number(op.antithrombin_percent ?? 80.0).toFixed(1);
    const schistoText = op.schistocytes_present ? 'Schisto+' : 'Schisto-';
    const bleedText = op.spontaneous_mucosal_bleeding ? 'Bleeding+' : 'Bleeding-';

    return `
      <article
        class="trust-screening-card dic-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-phenotype="${pheno}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="dic-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="dic-card-metrics">
          <span><strong>Phenotype:</strong> ${pheno}</span>
          <span><strong>Platelets:</strong> ${plt} K/µL</span>
          <span><strong>PT / aPTT:</strong> ${pt}s / ${aptt}s</span>
          <span><strong>D-Dimer:</strong> ${ddimer} ng/mL</span>
          <span><strong>Fibrinogen:</strong> ${fib} mg/dL</span>
          <span><strong>ATIII:</strong> ${at3}%</span>
          <span><strong>Schistocytes / Bleeding:</strong> ${schistoText} / ${bleedText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary dic-select-op-btn" data-op-id="${opId}">
            🔍 Open DIC Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('dic-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('dic-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="dic-ops-empty-msg">No DIC &amp; Coagulopathy operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (document.getElementById('dic-status-filter')?.value || '').trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/dic-coagulopathy/operations?${qs}`
        : '/api/dic-coagulopathy/operations';

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
      const res = await fetch(`/api/dic-coagulopathy/operations/${encodeURIComponent(opId)}`, {
        headers: { Accept: 'application/json' },
      });
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
        statusBadge.className = 'dic-status-badge dic-status-cleared';
      }
      eventSource = new EventSource('/api/dic-coagulopathy/stream');
      eventSource.addEventListener('dic_coagulopathy_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('dic-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-dic-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (document.getElementById('create-case-id-input')?.value || 'CASE-DIC-104').trim();
        const petName = (document.getElementById('create-pet-name-input')?.value || '').trim();
        const species = (document.getElementById('create-species-input')?.value || 'Canine').trim();
        const breed = (document.getElementById('create-breed-input')?.value || 'Retired Racing Greyhound').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-input')?.value || '29.5');
        const phenotype = (
          document.getElementById('create-phenotype-input')?.value ||
          'HYPO_COAGULABLE_HEMORRHAGIC_CONSUMPTIVE_DIC'
        ).trim();
        const platelets = parseFloat(document.getElementById('create-platelets-input')?.value || '42.0');
        const ptSec = parseFloat(document.getElementById('create-pt-input')?.value || '23.2');
        const apttSec = parseFloat(document.getElementById('create-aptt-input')?.value || '138.0');
        const ddimer = parseFloat(document.getElementById('create-ddimer-input')?.value || '1720.0');
        const fibrinogen = parseFloat(document.getElementById('create-fibrinogen-input')?.value || '74.0');
        const at3 = parseFloat(document.getElementById('create-at3-input')?.value || '51.0');
        const schistocytes = Boolean(document.getElementById('create-schistocytes-checkbox')?.checked);
        const bleeding = Boolean(document.getElementById('create-bleeding-checkbox')?.checked);

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 29.5 : weightKg,
          phase_phenotype: phenotype,
          platelet_count_k_ul: isNaN(platelets) ? 42.0 : platelets,
          pt_seconds: isNaN(ptSec) ? 23.2 : ptSec,
          aptt_seconds: isNaN(apttSec) ? 138.0 : apttSec,
          d_dimer_ng_ml: isNaN(ddimer) ? 1720.0 : ddimer,
          fibrinogen_mg_dl: isNaN(fibrinogen) ? 74.0 : fibrinogen,
          antithrombin_percent: isNaN(at3) ? 51.0 : at3,
          schistocytes_present: schistocytes,
          spontaneous_mucosal_bleeding: bleeding,
        };

        const btn = document.getElementById('create-dic-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/dic-coagulopathy/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(`Admitted DIC patient ${created.pet_name} (${created.id})`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to admit DIC patient';
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

        const ffp = (
          document.getElementById('stage-ffp-protocol-input')?.value ||
          'Fresh Frozen Plasma (FFP) 15 mL/kg IV over 3–4h via 170–260 µm filter'
        ).trim();
        const cryo = (
          document.getElementById('stage-cryo-protocol-input')?.value ||
          'Cryoprecipitate 1 unit per 10 kg IV for severe hypofibrinogenemia (< 100 mg/dL)'
        ).trim();
        const txa = (
          document.getElementById('stage-txa-plan-input')?.value ||
          'Tranexamic Acid (TXA) 10 mg/kg IV slow infusion over 20 min q8h'
        ).trim();
        const heparin = (
          document.getElementById('stage-heparin-phase-input')?.value || ''
        ).trim();
        const trigger = (
          document.getElementById('stage-trigger-control-input')?.value || ''
        ).trim();
        const schedule = (
          document.getElementById('stage-coag-schedule-input')?.value || ''
        ).trim();

        const payload = {
          fresh_frozen_plasma_protocol: ffp,
          cryoprecipitate_vwf_protocol: cryo,
          tranexamic_acid_aminocaproic_plan: txa,
          antithrombin_heparin_phase_plan: heparin,
          underlying_trigger_control_plan: trigger,
          serial_coag_panel_schedule: schedule,
        };

        const btn = document.getElementById('stage-therapy-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/dic-coagulopathy/operations/${encodeURIComponent(selectedOpId)}/stage-therapy`,
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
              `Staged FFP / Cryoprecipitate / TXA therapy for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage component therapy';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-coag-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const platelets = parseFloat(
          document.getElementById('check-platelets-input')?.value || '145.0'
        );
        const ptSec = parseFloat(
          document.getElementById('check-pt-input')?.value || '11.6'
        );
        const apttSec = parseFloat(
          document.getElementById('check-aptt-input')?.value || '72.0'
        );
        const ddimer = parseFloat(
          document.getElementById('check-ddimer-input')?.value || '240.0'
        );
        const fibrinogen = parseFloat(
          document.getElementById('check-fibrinogen-input')?.value || '195.0'
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Callum Vance, DACVECC'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          platelet_count_k_ul: isNaN(platelets) ? 145.0 : platelets,
          pt_seconds: isNaN(ptSec) ? 11.6 : ptSec,
          aptt_seconds: isNaN(apttSec) ? 72.0 : apttSec,
          d_dimer_ng_ml: isNaN(ddimer) ? 240.0 : ddimer,
          fibrinogen_mg_dl: isNaN(fibrinogen) ? 195.0 : fibrinogen,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-coag-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/dic-coagulopathy/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
            showToast(`Recorded coagulation panel check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record coagulation check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-dic-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dischargedBy = (
          document.getElementById('clear-discharged-by-input')?.value ||
          'Dr. Callum Vance, DACVECC'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          discharged_by: dischargedBy,
          discharge_summary: summary,
        };

        const btn = document.getElementById('clear-dic-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/dic-coagulopathy/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Normocoagulable Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to clear DIC operation for discharge';
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
    const gridEl = document.getElementById('dic-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.dic-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.dic-op-card');
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
