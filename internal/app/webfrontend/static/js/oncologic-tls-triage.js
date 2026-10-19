/**
 * PetSpotR Acute Tumor Lysis Syndrome (TLS) & Oncologic Hypercalcemia of Malignancy,
 * Hyperkalemia/Hyperphosphatemia/Hypocalcemia, Rasburicase/Allopurinol &
 * Zoledronate/Saline Diuresis Cockpit Controller (Milestone 23.4)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'TLS-OP-101';
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
    const liveStatus = document.getElementById('tls-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('tls-error-banner');
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
      case 'ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE':
        return 'tls-status-critical';
      case 'SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED':
        return 'tls-status-staged';
      case 'ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED':
        return 'tls-status-cleared';
      default:
        return 'tls-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const acuteEl = document.getElementById('kpi-acute-triage');
    const stagedEl = document.getElementById('kpi-diuresis-staged');
    const clearedEl = document.getElementById('kpi-cleared-normoelectrolytic');
    const tlsEl = document.getElementById('kpi-tls-count');
    const hypercalcemiaEl = document.getElementById('kpi-hypercalcemia-count');

    const total = metrics?.total_operations ?? ops.length;
    const acute =
      metrics?.acute_metabolic_triage_count ??
      ops.filter((op) => op.status === 'ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE').length;
    const staged =
      metrics?.diuresis_staged_count ??
      ops.filter((op) => op.status === 'SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED').length;
    const cleared =
      metrics?.cleared_normoelectrolytic_count ??
      ops.filter((op) => op.status === 'ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED').length;
    const tlsCount =
      metrics?.tumor_lysis_syndrome_count ??
      ops.filter((op) => op.emergency_type === 'ACUTE_TUMOR_LYSIS_SYNDROME_LYMPHOMA').length;
    const hypercalcemia =
      metrics?.hypercalcemia_malignancy_count ??
      ops.filter((op) => op.emergency_type === 'PARANEOPLASTIC_HYPERCALCEMIA_PTHRP').length;

    if (totalEl) totalEl.textContent = String(total);
    if (acuteEl) acuteEl.textContent = String(acute);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (tlsEl) tlsEl.textContent = String(tlsCount);
    if (hypercalcemiaEl) hypercalcemiaEl.textContent = String(hypercalcemia);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const diur = op.diuresis_plan || {};
    const salineInput = document.getElementById('stage-saline-plan-input');
    const rasbInput = document.getElementById('stage-rasburicase-input');
    const zolInput = document.getElementById('stage-zoledronate-input');
    const cardioInput = document.getElementById('stage-cardioprotection-input');
    const binderInput = document.getElementById('stage-binder-diuretic-input');
    const schedInput = document.getElementById('stage-electrolyte-schedule-input');

    if (salineInput && diur.zero_calcium_saline_diuresis_plan) {
      salineInput.value = diur.zero_calcium_saline_diuresis_plan;
    }
    if (rasbInput && diur.rasburicase_allopurinol_protocol) {
      rasbInput.value = diur.rasburicase_allopurinol_protocol;
    }
    if (zolInput && diur.zoledronate_calcitonin_protocol) {
      zolInput.value = diur.zoledronate_calcitonin_protocol;
    }
    if (cardioInput && diur.hyperkalemia_cardioprotection_plan) {
      cardioInput.value = diur.hyperkalemia_cardioprotection_plan;
    }
    if (binderInput && diur.phosphate_binder_or_loop_diuretic) {
      binderInput.value = diur.phosphate_binder_or_loop_diuretic;
    }
    if (schedInput && diur.serial_electrolyte_ecg_schedule) {
      schedInput.value = diur.serial_electrolyte_ecg_schedule;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('tls-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED'
    ) {
      const by = active.discharged_by || 'Attending Oncologist';
      const summary =
        active.discharge_summary ||
        'Electrolytes normalized and renal function protected.';
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

    const emergEl = document.getElementById('active-emergency-type');
    if (emergEl) {
      emergEl.textContent = active.emergency_type || '';
    }
    const caxpEl = document.getElementById('active-caxp-product');
    if (caxpEl) {
      caxpEl.textContent = Number(active.ca_x_p_product ?? 60.0).toFixed(1);
    }
    const cytoEl = document.getElementById('active-cytology-flag');
    if (cytoEl) {
      cytoEl.textContent = active.definitive_cytology_confirmed
        ? 'YES'
        : 'NO (HOLD STEROIDS)';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent =
        active.status || 'ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE';
      badgeEl.className = `tls-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const kEl = document.getElementById('active-potassium');
    if (kEl) {
      kEl.textContent = `${Number(active.serum_potassium_mmol_l ?? 0).toFixed(1)} mmol/L`;
    }
    const phosEl = document.getElementById('active-phosphorus');
    if (phosEl) {
      phosEl.textContent = `${Number(active.serum_phosphorus_mg_dl ?? 0).toFixed(1)} mg/dL`;
    }
    const icaEl = document.getElementById('active-ionized-calcium');
    if (icaEl) {
      icaEl.textContent = `${Number(active.ionized_calcium_mmol_l ?? 0).toFixed(2)} mmol/L`;
    }
    const uricCrEl = document.getElementById('active-uric-creatinine');
    if (uricCrEl) {
      const ua = Number(active.uric_acid_mg_dl ?? 0).toFixed(1);
      const cr = Number(active.serum_creatinine_mg_dl ?? 0).toFixed(1);
      uricCrEl.textContent = `${ua} / ${cr} mg/dL`;
    }

    const met = active.metabolic_plan || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-emergency-patho', met.emergency_pathophysiology || '');
    setText('plan-caxp-note', met.calcium_phosphorus_product_note || '');
    setText('plan-ical-potassium-alert', met.ionized_calcium_or_potassium_alert || '');
    setText('plan-steroid-guard', met.steroid_before_biopsy_contraindication || '');
    setText('plan-bicarb-note', met.sodium_bicarbonate_urine_note || '');
    setText('plan-urate-risk', met.urate_oxalate_nephropathy_risk || '');

    const diur = active.diuresis_plan || {};
    setText('plan-saline-diuresis', diur.zero_calcium_saline_diuresis_plan || '');
    setText('plan-rasburicase-protocol', diur.rasburicase_allopurinol_protocol || '');
    setText('plan-zoledronate-protocol', diur.zoledronate_calcitonin_protocol || '');
    setText('plan-cardioprotection', diur.hyperkalemia_cardioprotection_plan || '');
    setText('plan-binder-diuretic', diur.phosphate_binder_or_loop_diuretic || '');
    setText('plan-electrolyte-schedule', diur.serial_electrolyte_ecg_schedule || '');

    const checks = Array.isArray(active.electrolyte_checks)
      ? active.electrolyte_checks
      : [];
    const checksCountEl = document.getElementById('electrolyte-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('electrolyte-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const k = Number(chk.serum_potassium_mmol_l ?? 0).toFixed(1);
          const p = Number(chk.serum_phosphorus_mg_dl ?? 0).toFixed(1);
          const ica = Number(chk.ionized_calcium_mmol_l ?? 0).toFixed(2);
          const ua = Number(chk.uric_acid_mg_dl ?? 0).toFixed(1);
          const cr = Number(chk.serum_creatinine_mg_dl ?? 0).toFixed(1);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="tls-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · K+ ${k} mmol/L · Phosphorus ${p} mg/dL · iCa ${ica} mmol/L · Uric Acid ${ua} mg/dL · Creatinine ${cr} mg/dL — ${note}</li>`;
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
    const status = escapeHtml(
      op.status || 'ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE'
    );
    const badgeClass = getStatusBadgeClass(op.status);
    const emergType = escapeHtml(op.emergency_type || '');
    const k = Number(op.serum_potassium_mmol_l ?? 0).toFixed(1);
    const p = Number(op.serum_phosphorus_mg_dl ?? 0).toFixed(1);
    const ica = Number(op.ionized_calcium_mmol_l ?? 0).toFixed(2);
    const ua = Number(op.uric_acid_mg_dl ?? 0).toFixed(1);
    const cr = Number(op.serum_creatinine_mg_dl ?? 0).toFixed(1);
    const caxp = Number(op.ca_x_p_product ?? 0).toFixed(1);
    const cytoText = op.definitive_cytology_confirmed
      ? 'Confirmed'
      : 'Pending (Hold Steroids)';

    return `
      <article
        class="trust-screening-card tls-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-emergency-type="${emergType}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="tls-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="tls-card-metrics">
          <span><strong>Emergency:</strong> ${emergType}</span>
          <span><strong>K+:</strong> ${k} mmol/L</span>
          <span><strong>Phosphorus:</strong> ${p} mg/dL</span>
          <span><strong>iCa:</strong> ${ica} mmol/L</span>
          <span><strong>Uric Acid:</strong> ${ua} mg/dL</span>
          <span><strong>Creatinine:</strong> ${cr} mg/dL</span>
          <span><strong>Ca x P:</strong> ${caxp}</span>
          <span><strong>Cytology:</strong> ${cytoText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary tls-select-op-btn" data-op-id="${opId}">
            🔍 Open TLS Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('tls-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('tls-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="tls-ops-empty-msg">No Oncologic TLS &amp; Hypercalcemia operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('tls-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/oncologic-tls/operations?${qs}`
        : '/api/oncologic-tls/operations';

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
        `/api/oncologic-tls/operations/${encodeURIComponent(opId)}`,
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
        statusBadge.className = 'tls-status-badge tls-status-cleared';
      }
      eventSource = new EventSource('/api/oncologic-tls/stream');
      eventSource.addEventListener('oncologic_tls_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('tls-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-tls-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value || 'CASE-TLS-104'
        ).trim();
        const petName = (
          document.getElementById('create-pet-name-input')?.value || ''
        ).trim();
        const species = (
          document.getElementById('create-species-input')?.value || 'Canine'
        ).trim();
        const breed = (
          document.getElementById('create-breed-input')?.value ||
          'Golden Retriever'
        ).trim();
        const weightKg = parseFloat(
          document.getElementById('create-weight-input')?.value || '30.5'
        );
        const emergencyType = (
          document.getElementById('create-emergency-type-input')?.value ||
          'ACUTE_TUMOR_LYSIS_SYNDROME_LYMPHOMA'
        ).trim();
        const potassium = parseFloat(
          document.getElementById('create-potassium-input')?.value || '6.9'
        );
        const phosphorus = parseFloat(
          document.getElementById('create-phosphorus-input')?.value || '10.8'
        );
        const ical = parseFloat(
          document.getElementById('create-ical-input')?.value || '0.82'
        );
        const uricAcid = parseFloat(
          document.getElementById('create-uric-acid-input')?.value || '8.8'
        );
        const creatinine = parseFloat(
          document.getElementById('create-creatinine-input')?.value || '2.8'
        );
        const caxp = parseFloat(
          document.getElementById('create-caxp-input')?.value || '92.5'
        );
        const cytologyConfirmed = Boolean(
          document.getElementById('create-cytology-checkbox')?.checked
        );

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 30.5 : weightKg,
          emergency_type: emergencyType,
          serum_potassium_mmol_l: isNaN(potassium) ? 6.9 : potassium,
          serum_phosphorus_mg_dl: isNaN(phosphorus) ? 10.8 : phosphorus,
          ionized_calcium_mmol_l: isNaN(ical) ? 0.82 : ical,
          uric_acid_mg_dl: isNaN(uricAcid) ? 8.8 : uricAcid,
          serum_creatinine_mg_dl: isNaN(creatinine) ? 2.8 : creatinine,
          ca_x_p_product: isNaN(caxp) ? 92.5 : caxp,
          definitive_cytology_confirmed: cytologyConfirmed,
        };

        const btn = document.getElementById('create-tls-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/oncologic-tls/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted Oncologic TLS patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to admit Oncologic TLS patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-diuresis-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const saline = (
          document.getElementById('stage-saline-plan-input')?.value ||
          '0.9% NaCl Calcium-Free IV Diuresis at 150 mL/hr (2.5× maintenance)'
        ).trim();
        const rasburicase = (
          document.getElementById('stage-rasburicase-input')?.value ||
          'Rasburicase 0.2 mg/kg IV over 30 min + Allopurinol 10 mg/kg PO q8h'
        ).trim();
        const zoledronate = (
          document.getElementById('stage-zoledronate-input')?.value ||
          'Zoledronate 0.25 mg/kg IV over 15 min + Calcitonin 4–6 IU/kg SC q8–12h'
        ).trim();
        const cardio = (
          document.getElementById('stage-cardioprotection-input')?.value || ''
        ).trim();
        const binder = (
          document.getElementById('stage-binder-diuretic-input')?.value || ''
        ).trim();
        const schedule = (
          document.getElementById('stage-electrolyte-schedule-input')?.value || ''
        ).trim();

        const payload = {
          zero_calcium_saline_diuresis_plan: saline,
          rasburicase_allopurinol_protocol: rasburicase,
          zoledronate_calcitonin_protocol: zoledronate,
          hyperkalemia_cardioprotection_plan: cardio,
          phosphate_binder_or_loop_diuretic: binder,
          serial_electrolyte_ecg_schedule: schedule,
        };

        const btn = document.getElementById('stage-diuresis-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/oncologic-tls/operations/${encodeURIComponent(selectedOpId)}/stage-diuresis`,
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
              `Staged 0.9% NaCl Diuresis / Rasburicase / Zoledronate for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage diuresis therapy';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-electrolyte-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const potassium = parseFloat(
          document.getElementById('check-potassium-input')?.value || '4.4'
        );
        const phosphorus = parseFloat(
          document.getElementById('check-phosphorus-input')?.value || '4.6'
        );
        const ical = parseFloat(
          document.getElementById('check-ical-input')?.value || '1.24'
        );
        const uricAcid = parseFloat(
          document.getElementById('check-uric-acid-input')?.value || '1.6'
        );
        const creatinine = parseFloat(
          document.getElementById('check-creatinine-input')?.value || '1.3'
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Soren Lindqvist, DACVIM (Oncology)'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          serum_potassium_mmol_l: isNaN(potassium) ? 4.4 : potassium,
          serum_phosphorus_mg_dl: isNaN(phosphorus) ? 4.6 : phosphorus,
          ionized_calcium_mmol_l: isNaN(ical) ? 1.24 : ical,
          uric_acid_mg_dl: isNaN(uricAcid) ? 1.6 : uricAcid,
          serum_creatinine_mg_dl: isNaN(creatinine) ? 1.3 : creatinine,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-electrolyte-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/oncologic-tls/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
              `Recorded electrolyte & renal check for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record electrolyte check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-tls-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dischargedBy = (
          document.getElementById('clear-discharged-by-input')?.value ||
          'Dr. Soren Lindqvist, DACVIM (Oncology)'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          discharged_by: dischargedBy,
          discharge_summary: summary,
        };

        const btn = document.getElementById('clear-tls-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/oncologic-tls/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Normoelectrolytic Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to clear Oncologic TLS operation for discharge';
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
    const gridEl = document.getElementById('tls-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.tls-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.tls-op-card');
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
