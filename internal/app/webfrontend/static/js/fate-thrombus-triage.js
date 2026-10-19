/**
 * PetSpotR Feline Aortic Thromboembolism (FATE / Saddle Thrombus), 5 Ps
 * (Pain, Pallor, Pulselessness, Paresis, Poikilothermy), Paired Glucose/Lactate
 * Limb Gradient & Clopidogrel/Rivaroxaban Cockpit Controller (Milestone 22.5)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'FATE-OP-101';
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
    const liveStatus = document.getElementById('fate-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('fate-error-banner');
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
      case 'ACUTE_SADDLE_THROMBUS_5PS_TRIAGE':
        return 'fate-status-critical';
      case 'METHADONE_CLOPIDOGREL_ANTICOAG_STAGED':
        return 'fate-status-staged';
      case 'REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED':
        return 'fate-status-cleared';
      default:
        return 'fate-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const acuteEl = document.getElementById('kpi-acute-saddle');
    const stagedEl = document.getElementById('kpi-anticoag-staged');
    const clearedEl = document.getElementById('kpi-cleared-reperfused');
    const bilateralEl = document.getElementById('kpi-bilateral-saddle');
    const chfEl = document.getElementById('kpi-concurrent-chf');

    const total = metrics?.total_operations ?? ops.length;
    const acute =
      metrics?.acute_saddle_triage_count ??
      ops.filter((op) => op.status === 'ACUTE_SADDLE_THROMBUS_5PS_TRIAGE').length;
    const staged =
      metrics?.anticoag_staged_count ??
      ops.filter((op) => op.status === 'METHADONE_CLOPIDOGREL_ANTICOAG_STAGED').length;
    const cleared =
      metrics?.cleared_reperfused_count ??
      ops.filter((op) => op.status === 'REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED').length;
    const bilateral =
      metrics?.bilateral_saddle_count ??
      ops.filter((op) => op.distribution_type === 'BILATERAL_AORTIC_TRIFURCATION_SADDLE').length;
    const chf =
      metrics?.concurrent_chf_count ??
      ops.filter((op) => Boolean(op.concurrent_cardiogenic_chf)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (acuteEl) acuteEl.textContent = String(acute);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (bilateralEl) bilateralEl.textContent = String(bilateral);
    if (chfEl) chfEl.textContent = String(chf);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const anti = op.antithrombotic_plan || {};
    const opioidInput = document.getElementById('stage-opioid-protocol-input');
    const clopInput = document.getElementById('stage-clopidogrel-loading-input');
    const fxaInput = document.getElementById('stage-factor-xa-protocol-input');
    const chfInput = document.getElementById('stage-chf-furosemide-input');
    const kInput = document.getElementById('stage-potassium-monitoring-input');
    const pimoInput = document.getElementById('stage-pimobendan-note-input');

    if (opioidInput && anti.full_mu_opioid_cri_protocol) {
      opioidInput.value = anti.full_mu_opioid_cri_protocol;
    }
    if (clopInput && anti.clopidogrel_loading_protocol) {
      clopInput.value = anti.clopidogrel_loading_protocol;
    }
    if (fxaInput && anti.factor_xa_or_heparin_protocol) {
      fxaInput.value = anti.factor_xa_or_heparin_protocol;
    }
    if (chfInput && anti.concurrent_chf_furosemide_plan) {
      chfInput.value = anti.concurrent_chf_furosemide_plan;
    }
    if (kInput && anti.serial_potassium_monitoring) {
      kInput.value = anti.serial_potassium_monitoring;
    }
    if (pimoInput && anti.pimobendan_lvot_note) {
      pimoInput.value = anti.pimobendan_lvot_note;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('fate-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED'
    ) {
      const by = active.discharged_by || 'Attending Cardiologist';
      const summary =
        active.discharge_summary ||
        'Reperfused, normokalemic, and cleared on Clopidogrel + Rivaroxaban dual therapy.';
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
      const weightStr = Number(active.weight_kg ?? 5.0).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Feline'} · ${active.breed || 'Domestic Shorthair'} · ${weightStr} kg)`;
    }

    const distEl = document.getElementById('active-distribution-type');
    if (distEl) {
      distEl.textContent = active.distribution_type || '';
    }
    const la2aoEl = document.getElementById('active-la2ao-ratio');
    if (la2aoEl) {
      la2aoEl.textContent = Number(active.la2ao_ratio ?? 2.0).toFixed(2);
    }
    const chfFlagEl = document.getElementById('active-chf-flag');
    if (chfFlagEl) {
      chfFlagEl.textContent = active.concurrent_cardiogenic_chf ? 'YES' : 'NO';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent = active.status || 'ACUTE_SADDLE_THROMBUS_5PS_TRIAGE';
      badgeEl.className = `fate-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const glucEl = document.getElementById('active-glucose-delta');
    if (glucEl) {
      glucEl.textContent = `${Number(active.glucose_delta_mg_dl ?? 0).toFixed(1)} mg/dL`;
    }
    const lactEl = document.getElementById('active-lactate-delta');
    if (lactEl) {
      lactEl.textContent = `${Number(active.lactate_delta_mmol_l ?? 0).toFixed(1)} mmol/L`;
    }
    const potEl = document.getElementById('active-serum-potassium');
    if (potEl) {
      potEl.textContent = `${Number(active.serum_potassium_mmol_l ?? 4.5).toFixed(1)} mmol/L`;
    }
    const tempEl = document.getElementById('active-limb-temp');
    if (tempEl) {
      tempEl.textContent = `${Number(active.distal_limb_temp_c ?? 30.0).toFixed(1)} °C`;
    }

    const grad = active.gradient_plan || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-five-ps', grad.five_ps_clinical_summary || '');
    setText('plan-glucose-delta', grad.systemic_to_limb_glucose_delta || '');
    setText('plan-lactate-delta', grad.limb_to_systemic_lactate_delta || '');
    setText('plan-la2ao-smoke', grad.left_atrial_smoke_la2ao_ratio || '');
    setText('plan-hyperkalemia-alert', grad.reperfusion_hyperkalemia_alert || '');
    setText('plan-passive-warming', grad.passive_warming_contraindication || '');

    const anti = active.antithrombotic_plan || {};
    setText('plan-opioid-protocol', anti.full_mu_opioid_cri_protocol || '');
    setText('plan-clopidogrel-protocol', anti.clopidogrel_loading_protocol || '');
    setText('plan-factor-xa-protocol', anti.factor_xa_or_heparin_protocol || '');
    setText('plan-chf-furosemide', anti.concurrent_chf_furosemide_plan || '');
    setText('plan-potassium-monitoring', anti.serial_potassium_monitoring || '');
    setText('plan-pimobendan-lvot', anti.pimobendan_lvot_note || '');

    const checks = Array.isArray(active.perfusion_checks) ? active.perfusion_checks : [];
    const checksCountEl = document.getElementById('perfusion-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('perfusion-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const gDelta = Number(chk.glucose_delta_mg_dl ?? 0).toFixed(1);
          const lDelta = Number(chk.lactate_delta_mmol_l ?? 0).toFixed(1);
          const kVal = Number(chk.serum_potassium_mmol_l ?? 4.5).toFixed(1);
          const tVal = Number(chk.distal_limb_temp_c ?? 35.0).toFixed(1);
          const pulse = escapeHtml(chk.femoral_pulse_status || '');
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="fate-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · ΔGlucose ${gDelta} mg/dL · ΔLactate ${lDelta} mmol/L · K+ ${kVal} mmol/L · Limb Temp ${tVal} °C · Pulse: <code>${pulse}</code> — ${note}</li>`;
        })
        .join('');
    }
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const caseId = escapeHtml(op.case_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Feline');
    const breed = escapeHtml(op.breed || 'Domestic Shorthair');
    const weightKg = Number(op.weight_kg ?? 5.0).toFixed(1);
    const status = escapeHtml(op.status || 'ACUTE_SADDLE_THROMBUS_5PS_TRIAGE');
    const badgeClass = getStatusBadgeClass(op.status);
    const dist = escapeHtml(op.distribution_type || '');
    const gDelta = Number(op.glucose_delta_mg_dl ?? 0).toFixed(1);
    const sysG = Number(op.systemic_glucose_mg_dl ?? 0).toFixed(1);
    const limbG = Number(op.affected_limb_glucose_mg_dl ?? 0).toFixed(1);
    const lDelta = Number(op.lactate_delta_mmol_l ?? 0).toFixed(1);
    const kVal = Number(op.serum_potassium_mmol_l ?? 4.5).toFixed(1);
    const tempVal = Number(op.distal_limb_temp_c ?? 30.0).toFixed(1);
    const la2ao = Number(op.la2ao_ratio ?? 2.0).toFixed(2);
    const chfText = op.concurrent_cardiogenic_chf ? 'YES (Furosemide + O2)' : 'NO';

    return `
      <article
        class="trust-screening-card fate-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-distribution="${dist}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="fate-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="fate-card-metrics">
          <span><strong>Distribution:</strong> ${dist}</span>
          <span><strong>ΔGlucose:</strong> ${gDelta} mg/dL (Sys ${sysG} / Limb ${limbG})</span>
          <span><strong>ΔLactate:</strong> ${lDelta} mmol/L</span>
          <span><strong>Serum K+:</strong> ${kVal} mmol/L</span>
          <span><strong>Limb Temp:</strong> ${tempVal} °C</span>
          <span><strong>LA:Ao:</strong> ${la2ao}</span>
          <span><strong>Concurrent CHF:</strong> ${chfText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary fate-select-op-btn" data-op-id="${opId}">
            🔍 Open FATE Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('fate-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('fate-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="fate-ops-empty-msg">No FATE / Saddle Thrombus operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (document.getElementById('fate-status-filter')?.value || '').trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/fate-thrombus/operations?${qs}`
        : '/api/fate-thrombus/operations';

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
      const res = await fetch(`/api/fate-thrombus/operations/${encodeURIComponent(opId)}`, {
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
        statusBadge.className = 'fate-status-badge fate-status-cleared';
      }
      eventSource = new EventSource('/api/fate-thrombus/stream');
      eventSource.addEventListener('fate_thrombus_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('fate-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-fate-op-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (document.getElementById('create-case-id-input')?.value || 'CASE-FATE-104').trim();
        const petName = (document.getElementById('create-pet-name-input')?.value || '').trim();
        const breed = (document.getElementById('create-breed-input')?.value || 'Domestic Shorthair').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-input')?.value || '5.6');
        const distType = (
          document.getElementById('create-distribution-input')?.value ||
          'BILATERAL_AORTIC_TRIFURCATION_SADDLE'
        ).trim();
        const sysGlucose = parseFloat(document.getElementById('create-sys-glucose-input')?.value || '192.0');
        const limbGlucose = parseFloat(document.getElementById('create-limb-glucose-input')?.value || '88.0');
        const sysLactate = parseFloat(document.getElementById('create-sys-lactate-input')?.value || '2.2');
        const limbLactate = parseFloat(document.getElementById('create-limb-lactate-input')?.value || '12.6');
        const potassium = parseFloat(document.getElementById('create-potassium-input')?.value || '4.8');
        const limbTemp = parseFloat(document.getElementById('create-limb-temp-input')?.value || '28.9');
        const la2ao = parseFloat(document.getElementById('create-la2ao-input')?.value || '2.40');
        const concurrentChf = Boolean(document.getElementById('create-chf-checkbox')?.checked);

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species: 'Feline',
          breed,
          weight_kg: isNaN(weightKg) ? 5.6 : weightKg,
          distribution_type: distType,
          systemic_glucose_mg_dl: isNaN(sysGlucose) ? 192.0 : sysGlucose,
          affected_limb_glucose_mg_dl: isNaN(limbGlucose) ? 88.0 : limbGlucose,
          systemic_lactate_mmol_l: isNaN(sysLactate) ? 2.2 : sysLactate,
          affected_limb_lactate_mmol_l: isNaN(limbLactate) ? 12.6 : limbLactate,
          serum_potassium_mmol_l: isNaN(potassium) ? 4.8 : potassium,
          distal_limb_temp_c: isNaN(limbTemp) ? 28.9 : limbTemp,
          la2ao_ratio: isNaN(la2ao) ? 2.4 : la2ao,
          concurrent_cardiogenic_chf: concurrentChf,
        };

        const btn = document.getElementById('create-fate-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/fate-thrombus/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(`Admitted FATE patient ${created.pet_name} (${created.id})`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to admit FATE patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-antithrombotic-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const opioid = (
          document.getElementById('stage-opioid-protocol-input')?.value ||
          'Methadone 0.25 mg/kg IV q4h transitioning to Fentanyl CRI 3–5 mcg/kg/hr IV'
        ).trim();
        const clopidogrel = (
          document.getElementById('stage-clopidogrel-loading-input')?.value ||
          'Clopidogrel 75 mg PO one-time oral loading dose (FAT CAT Trial), followed by 18.75 mg PO q24h'
        ).trim();
        const factorXa = (
          document.getElementById('stage-factor-xa-protocol-input')?.value ||
          'Enoxaparin 0.9 mg/kg SC q8h transitioning to Rivaroxaban 2.5 mg PO q24h dual therapy'
        ).trim();
        const chfPlan = (
          document.getElementById('stage-chf-furosemide-input')?.value || ''
        ).trim();
        const kMonitor = (
          document.getElementById('stage-potassium-monitoring-input')?.value || ''
        ).trim();
        const pimoNote = (
          document.getElementById('stage-pimobendan-note-input')?.value || ''
        ).trim();

        const payload = {
          full_mu_opioid_cri_protocol: opioid,
          clopidogrel_loading_protocol: clopidogrel,
          factor_xa_or_heparin_protocol: factorXa,
          concurrent_chf_furosemide_plan: chfPlan,
          serial_potassium_monitoring: kMonitor,
          pimobendan_lvot_note: pimoNote,
        };

        const btn = document.getElementById('stage-antithrombotic-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/fate-thrombus/operations/${encodeURIComponent(selectedOpId)}/stage-antithrombotic`,
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
              `Staged Methadone / Clopidogrel 75 mg / Anticoagulant protocol for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage antithrombotic protocol';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-perfusion-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const glucoseDelta = parseFloat(
          document.getElementById('check-glucose-delta-input')?.value || '18.0'
        );
        const lactateDelta = parseFloat(
          document.getElementById('check-lactate-delta-input')?.value || '1.5'
        );
        const potassium = parseFloat(
          document.getElementById('check-potassium-input')?.value || '4.4'
        );
        const limbTemp = parseFloat(
          document.getElementById('check-limb-temp-input')?.value || '36.8'
        );
        const pulseStatus = (
          document.getElementById('check-pulse-status-input')?.value ||
          'BILATERAL_DOPPLER_AUDIBLE_WARMING_PADS'
        ).trim();
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Maya Lin, DVM, DACVECC'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          glucose_delta_mg_dl: isNaN(glucoseDelta) ? 18.0 : glucoseDelta,
          lactate_delta_mmol_l: isNaN(lactateDelta) ? 1.5 : lactateDelta,
          serum_potassium_mmol_l: isNaN(potassium) ? 4.4 : potassium,
          distal_limb_temp_c: isNaN(limbTemp) ? 36.8 : limbTemp,
          femoral_pulse_status: pulseStatus,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-perfusion-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/fate-thrombus/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
            showToast(`Recorded limb perfusion & K+ check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record limb perfusion check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-fate-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dischargedBy = (
          document.getElementById('clear-discharged-by-input')?.value ||
          'Dr. Julian Vance, DACVIM (Cardiology)'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          discharged_by: dischargedBy,
          discharge_summary: summary,
        };

        const btn = document.getElementById('clear-fate-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/fate-thrombus/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Dual-Therapy Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to clear FATE operation for discharge';
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
    const gridEl = document.getElementById('fate-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.fate-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.fate-op-card');
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
