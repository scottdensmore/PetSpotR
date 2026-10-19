/**
 * PetSpotR Acute Feline Infectious Peritonitis (FIP — Effusive Wet & Non-Effusive Dry/Ocular/Neurological),
 * Rivalta Test, Albumin:Globulin Ratio < 0.4, GS-441524 / Remdesivir Nucleoside Analog &
 * Blood-Brain/Ocular Barrier Dosing Cockpit Controller (Milestone 24.4)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'FIP-OP-101';
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
    const liveStatus = document.getElementById('fip-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('fip-error-banner');
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
      case 'ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE':
        return 'fip-status-critical';
      case 'GS441524_REMDESIVIR_84DAY_STAGED':
        return 'fip-status-staged';
      case 'AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED':
        return 'fip-status-cleared';
      default:
        return 'fip-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const acuteEl = document.getElementById('kpi-acute-triage');
    const stagedEl = document.getElementById('kpi-gs441524-staged');
    const clearedEl = document.getElementById('kpi-cleared-cured');
    const neuroOcularEl = document.getElementById('kpi-neuro-ocular-count');
    const agUnder04El = document.getElementById('kpi-ag-under-04-count');

    const total = metrics?.total_operations ?? ops.length;
    const acute =
      metrics?.acute_fip_triage_count ??
      ops.filter((op) => op.status === 'ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE').length;
    const staged =
      metrics?.gs441524_staged_count ??
      ops.filter((op) => op.status === 'GS441524_REMDESIVIR_84DAY_STAGED').length;
    const cleared =
      metrics?.cleared_84day_cured_count ??
      ops.filter((op) => op.status === 'AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED').length;
    const neuroOcular =
      metrics?.neuro_ocular_high_dose_count ??
      ops.filter(
        (op) =>
          op.phenotype === 'NEUROLOGICAL_ATAXIA_SEIZURE_CNS_FIP' ||
          op.phenotype === 'OCULAR_ANTERIOR_UVEITIS_KERATIC_FIP' ||
          Number(op.daily_gs441524_mg_kg ?? 0) >= 15.0
      ).length;
    const agUnder04 =
      metrics?.ag_ratio_under_04_count ??
      ops.filter((op) => Number(op.albumin_globulin_ratio ?? 1.0) < 0.4).length;

    if (totalEl) totalEl.textContent = String(total);
    if (acuteEl) acuteEl.textContent = String(acute);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (neuroOcularEl) neuroOcularEl.textContent = String(neuroOcular);
    if (agUnder04El) agUnder04El.textContent = String(agUnder04);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const anti = op.antiviral_plan || {};
    const dosingInput = document.getElementById('stage-gs441524-dosing-input');
    const recalcInput = document.getElementById('stage-weight-recalc-input');
    const centesisInput = document.getElementById('stage-centesis-limit-input');
    const ocularInput = document.getElementById('stage-ocular-support-input');
    const hepatoInput = document.getElementById('stage-hepatoprotectant-input');
    const schedInput = document.getElementById('stage-monitoring-plan-input');

    if (dosingInput && anti.gs441524_or_remdesivir_dosing) {
      dosingInput.value = anti.gs441524_or_remdesivir_dosing;
    }
    if (recalcInput && anti.weight_adjusted_dose_recalc_note) {
      recalcInput.value = anti.weight_adjusted_dose_recalc_note;
    }
    if (centesisInput && anti.therapeutic_thoracocentesis_limit) {
      centesisInput.value = anti.therapeutic_thoracocentesis_limit;
    }
    if (ocularInput && anti.ocular_topical_atropine_pred_note) {
      ocularInput.value = anti.ocular_topical_atropine_pred_note;
    }
    if (hepatoInput && anti.hepatoprotectant_b12_support_plan) {
      hepatoInput.value = anti.hepatoprotectant_b12_support_plan;
    }
    if (schedInput && anti.day28_day56_day84_monitoring_plan) {
      schedInput.value = anti.day28_day56_day84_monitoring_plan;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('fip-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED'
    ) {
      const by = active.discharged_by || 'Attending Feline Specialist';
      const summary =
        active.discharge_summary ||
        'Completed 84-day GS-441524 protocol and normalized A:G ratio.';
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
      const weightStr = Number(active.weight_kg ?? 3.0).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Feline'} · ${active.breed || 'Domestic Shorthair'} · ${weightStr} kg)`;
    }

    const phenoEl = document.getElementById('active-phenotype');
    if (phenoEl) {
      phenoEl.textContent = active.phenotype || '';
    }
    const doseEl = document.getElementById('active-gs-dose');
    if (doseEl) {
      doseEl.textContent = `${Number(active.daily_gs441524_mg_kg ?? 10.0).toFixed(1)} mg/kg/day`;
    }
    const rivaltaEl = document.getElementById('active-rivalta-flag');
    if (rivaltaEl) {
      rivaltaEl.textContent = active.rivalta_test_positive
        ? 'POSITIVE'
        : 'NEGATIVE';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent =
        active.status || 'ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE';
      badgeEl.className = `fip-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const agEl = document.getElementById('active-ag-ratio');
    if (agEl) {
      agEl.textContent = Number(active.albumin_globulin_ratio ?? 0).toFixed(2);
    }
    const dayEl = document.getElementById('active-treatment-day');
    if (dayEl) {
      dayEl.textContent = `Day ${Number(active.treatment_day ?? 1)} / 84`;
    }
    const biliEl = document.getElementById('active-bilirubin');
    if (biliEl) {
      biliEl.textContent = `${Number(active.total_bilirubin_mg_dl ?? 0).toFixed(1)} mg/dL`;
    }
    const hctWeightEl = document.getElementById('active-hct-weight');
    if (hctWeightEl) {
      const hct = Number(active.hematocrit_percent ?? 0).toFixed(1);
      const wt = Number(active.weight_kg ?? 0).toFixed(1);
      hctWeightEl.textContent = `${hct}% · ${wt} kg`;
    }

    const bio = active.biomarker_plan || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-phenotype-patho', bio.phenotype_pathophysiology || '');
    setText('plan-rivalta-ag', bio.rivalta_ag_ratio_interpretation || '');
    setText('plan-agp-pcr', bio.alpha1_acid_glycoprotein_fcov_pcr || '');
    setText('plan-ascites-guard', bio.complete_ascites_drainage_contraindication || '');
    setText('plan-barrier-escalation', bio.barrier_penetration_dose_escalation || '');
    setText('plan-toxicity-warning', bio.fluoroquinolone_retinal_toxicity_warning || '');

    const anti = active.antiviral_plan || {};
    setText('plan-gs441524-dosing', anti.gs441524_or_remdesivir_dosing || '');
    setText('plan-weight-recalc', anti.weight_adjusted_dose_recalc_note || '');
    setText('plan-centesis-limit', anti.therapeutic_thoracocentesis_limit || '');
    setText('plan-ocular-support', anti.ocular_topical_atropine_pred_note || '');
    setText('plan-hepatoprotectant', anti.hepatoprotectant_b12_support_plan || '');
    setText('plan-monitoring-schedule', anti.day28_day56_day84_monitoring_plan || '');

    const checks = Array.isArray(active.progress_checks)
      ? active.progress_checks
      : [];
    const checksCountEl = document.getElementById('progress-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('progress-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const day = Number(chk.treatment_day ?? 1);
          const wt = Number(chk.weight_kg ?? 0).toFixed(1);
          const ag = Number(chk.albumin_globulin_ratio ?? 0).toFixed(2);
          const bili = Number(chk.total_bilirubin_mg_dl ?? 0).toFixed(1);
          const hct = Number(chk.hematocrit_percent ?? 0).toFixed(1);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="fip-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · Day ${day}/84 · Weight ${wt} kg · A:G Ratio ${ag} · Bilirubin ${bili} mg/dL · HCT ${hct}% — ${note}</li>`;
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
    const weightKg = Number(op.weight_kg ?? 3.0).toFixed(1);
    const status = escapeHtml(
      op.status || 'ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE'
    );
    const badgeClass = getStatusBadgeClass(op.status);
    const phenotype = escapeHtml(op.phenotype || '');
    const day = Number(op.treatment_day ?? 1);
    const ag = Number(op.albumin_globulin_ratio ?? 0).toFixed(2);
    const bili = Number(op.total_bilirubin_mg_dl ?? 0).toFixed(1);
    const hct = Number(op.hematocrit_percent ?? 0).toFixed(1);
    const gsDose = Number(op.daily_gs441524_mg_kg ?? 10.0).toFixed(1);
    const rivaltaText = op.rivalta_test_positive ? 'POSITIVE' : 'Negative';
    const dyspneaText = op.pleural_effusion_dyspnea_present
      ? 'YES (Thoracocentesis)'
      : 'No';

    return `
      <article
        class="trust-screening-card fip-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-phenotype="${phenotype}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="fip-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="fip-card-metrics">
          <span><strong>Phenotype:</strong> ${phenotype}</span>
          <span><strong>Day:</strong> ${day} / 84</span>
          <span><strong>A:G Ratio:</strong> ${ag}</span>
          <span><strong>Bilirubin:</strong> ${bili} mg/dL</span>
          <span><strong>HCT:</strong> ${hct}%</span>
          <span><strong>GS-441524:</strong> ${gsDose} mg/kg/d</span>
          <span><strong>Rivalta:</strong> ${rivaltaText}</span>
          <span><strong>Pleural Dyspnea:</strong> ${dyspneaText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary fip-select-op-btn" data-op-id="${opId}">
            🔍 Open FIP Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('fip-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('fip-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="fip-ops-empty-msg">No FIP &amp; GS-441524 operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('fip-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/fip-gs441524/operations?${qs}`
        : '/api/fip-gs441524/operations';

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
        `/api/fip-gs441524/operations/${encodeURIComponent(opId)}`,
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
        statusBadge.className = 'fip-status-badge fip-status-cleared';
      }
      eventSource = new EventSource('/api/fip-gs441524/stream');
      eventSource.addEventListener('fip_gs441524_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('fip-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-fip-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value || 'CASE-FIP-104'
        ).trim();
        const petName = (
          document.getElementById('create-pet-name-input')?.value || ''
        ).trim();
        const species = (
          document.getElementById('create-species-input')?.value || 'Feline'
        ).trim();
        const breed = (
          document.getElementById('create-breed-input')?.value ||
          'Scottish Fold'
        ).trim();
        const weightKg = parseFloat(
          document.getElementById('create-weight-input')?.value || '2.8'
        );
        const phenotype = (
          document.getElementById('create-phenotype-input')?.value ||
          'EFFUSIVE_CAVITARY_PLEURAL_ASCITES_FIP'
        ).trim();
        const treatmentDay = parseInt(
          document.getElementById('create-treatment-day-input')?.value || '1',
          10
        );
        const agRatio = parseFloat(
          document.getElementById('create-ag-ratio-input')?.value || '0.32'
        );
        const bilirubin = parseFloat(
          document.getElementById('create-bilirubin-input')?.value || '2.1'
        );
        const hct = parseFloat(
          document.getElementById('create-hct-input')?.value || '23.5'
        );
        const gsDose = parseFloat(
          document.getElementById('create-gs-dose-input')?.value || '10.0'
        );
        const rivaltaPositive = Boolean(
          document.getElementById('create-rivalta-checkbox')?.checked
        );
        const pleuralDyspnea = Boolean(
          document.getElementById('create-pleural-dyspnea-checkbox')?.checked
        );

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 2.8 : weightKg,
          phenotype,
          treatment_day: isNaN(treatmentDay) ? 1 : treatmentDay,
          albumin_globulin_ratio: isNaN(agRatio) ? 0.32 : agRatio,
          total_bilirubin_mg_dl: isNaN(bilirubin) ? 2.1 : bilirubin,
          hematocrit_percent: isNaN(hct) ? 23.5 : hct,
          daily_gs441524_mg_kg: isNaN(gsDose) ? 10.0 : gsDose,
          rivalta_test_positive: rivaltaPositive,
          pleural_effusion_dyspnea_present: pleuralDyspnea,
        };

        const btn = document.getElementById('create-fip-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/fip-gs441524/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted FIP patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to admit FIP patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-antiviral-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dosing = (
          document.getElementById('stage-gs441524-dosing-input')?.value ||
          'GS-441524 Nucleoside Analog 10.0 mg/kg/day for 84 uninterrupted days'
        ).trim();
        const recalc = (
          document.getElementById('stage-weight-recalc-input')?.value ||
          'Weigh weekly on pediatric scale and recalculate GS-441524 dose upward with weight gain; NEVER reduce daily mg dose'
        ).trim();
        const centesis = (
          document.getElementById('stage-centesis-limit-input')?.value ||
          'Immediate thoracocentesis for pleural effusion dyspnea; NEVER perform complete abdominal ascites drainage'
        ).trim();
        const ocular = (
          document.getElementById('stage-ocular-support-input')?.value || ''
        ).trim();
        const hepato = (
          document.getElementById('stage-hepatoprotectant-input')?.value || ''
        ).trim();
        const monitoring = (
          document.getElementById('stage-monitoring-plan-input')?.value || ''
        ).trim();

        const payload = {
          gs441524_or_remdesivir_dosing: dosing,
          weight_adjusted_dose_recalc_note: recalc,
          therapeutic_thoracocentesis_limit: centesis,
          ocular_topical_atropine_pred_note: ocular,
          hepatoprotectant_b12_support_plan: hepato,
          day28_day56_day84_monitoring_plan: monitoring,
        };

        const btn = document.getElementById('stage-antiviral-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/fip-gs441524/operations/${encodeURIComponent(selectedOpId)}/stage-antiviral`,
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
              `Staged 84-Day GS-441524 / Remdesivir Antiviral Protocol for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage antiviral protocol';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-progress-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const treatmentDay = parseInt(
          document.getElementById('check-treatment-day-input')?.value || '84',
          10
        );
        const weightKg = parseFloat(
          document.getElementById('check-weight-input')?.value || '3.5'
        );
        const agRatio = parseFloat(
          document.getElementById('check-ag-ratio-input')?.value || '0.72'
        );
        const bilirubin = parseFloat(
          document.getElementById('check-bilirubin-input')?.value || '0.2'
        );
        const hct = parseFloat(
          document.getElementById('check-hct-input')?.value || '36.5'
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Linnea Lindholm, DACVIM (Feline Medicine)'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          treatment_day: isNaN(treatmentDay) ? 84 : treatmentDay,
          weight_kg: isNaN(weightKg) ? 3.5 : weightKg,
          albumin_globulin_ratio: isNaN(agRatio) ? 0.72 : agRatio,
          total_bilirubin_mg_dl: isNaN(bilirubin) ? 0.2 : bilirubin,
          hematocrit_percent: isNaN(hct) ? 36.5 : hct,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-progress-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/fip-gs441524/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
              `Recorded 84-day antiviral progress check for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record progress check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-fip-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dischargedBy = (
          document.getElementById('clear-discharged-by-input')?.value ||
          'Dr. Linnea Lindholm, DACVIM (Feline Medicine)'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          discharged_by: dischargedBy,
          discharge_summary: summary,
        };

        const btn = document.getElementById('clear-fip-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/fip-gs441524/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for 84-Day Antiviral Cure Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to clear FIP operation for discharge';
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
    const gridEl = document.getElementById('fip-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.fip-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.fip-op-card');
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
