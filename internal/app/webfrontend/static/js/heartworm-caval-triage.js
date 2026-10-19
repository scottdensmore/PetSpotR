/**
 * PetSpotR Acute Heartworm Caval Syndrome (Dirofilaria immitis), Vena Cava Worm Burden Echocardiography,
 * Intravascular Hemolysis (Hemoglobinuria), Jugular Venotomy Extraction & Melarsomine Split-Dose /
 * Doxycycline / Macrocyclic Lactone Thromboembolism Prevention Cockpit Controller (Milestone 25.1)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'HWC-OP-101';
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
    const liveStatus = document.getElementById('hwc-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('hwc-error-banner');
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
      case 'JUGULAR_VENOTOMY_WOLBACHIA_DOXY_STAGED':
        return 'hwc-status-critical';
      case 'PULMONARY_HYPERTENSION_PTE_MONITORED':
        return 'hwc-status-staged';
      case 'CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED':
        return 'hwc-status-cleared';
      default:
        return 'hwc-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const stagedEl = document.getElementById('kpi-jugular-staged');
    const monitoredEl = document.getElementById('kpi-pte-monitored');
    const clearedEl = document.getElementById('kpi-caval-cleared');
    const classIvEl = document.getElementById('kpi-class-iv-count');
    const severePhEl = document.getElementById('kpi-severe-ph-count');

    const total = metrics?.total_operations ?? ops.length;
    const staged =
      metrics?.jugular_venotomy_wolbachia_staged_count ??
      ops.filter(
        (op) => op.status === 'JUGULAR_VENOTOMY_WOLBACHIA_DOXY_STAGED'
      ).length;
    const monitored =
      metrics?.pulmonary_hypertension_pte_monitored_count ??
      ops.filter(
        (op) => op.status === 'PULMONARY_HYPERTENSION_PTE_MONITORED'
      ).length;
    const cleared =
      metrics?.caval_patent_cleared_count ??
      ops.filter(
        (op) => op.status === 'CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED'
      ).length;
    const classIv =
      metrics?.class_iv_caval_syndrome_count ??
      ops.filter((op) => op.severity_class === 'CLASS_IV_CAVAL_SYNDROME')
        .length;
    const severePh =
      metrics?.severe_pulmonary_hypertension_count ??
      ops.filter((op) => Number(op.tr_jet_velocity_ms ?? 0) > 3.4).length;

    if (totalEl) totalEl.textContent = String(total);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (monitoredEl) monitoredEl.textContent = String(monitored);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (classIvEl) classIvEl.textContent = String(classIv);
    if (severePhEl) severePhEl.textContent = String(severePh);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const stg = op.intervention_staging || {};
    const protocolInput = document.getElementById('stage-protocol-input');
    const doxyDoseInput = document.getElementById('stage-doxy-dose-input');
    const jugularPlanInput = document.getElementById('stage-jugular-plan-input');
    const doxyPlanInput = document.getElementById('stage-doxy-plan-input');
    const lactonePlanInput = document.getElementById('stage-lactone-plan-input');
    const predDoseInput = document.getElementById('stage-pred-dose-input');
    const cageRestInput = document.getElementById('stage-cage-rest-weeks-input');
    const sildenafilInput = document.getElementById(
      'stage-sildenafil-plan-input'
    );
    const melarsomineInput = document.getElementById(
      'stage-melarsomine-note-input'
    );

    if (protocolInput && (stg.protocol || op.protocol)) {
      protocolInput.value = stg.protocol || op.protocol;
    }
    if (doxyDoseInput && stg.wolbachia_doxycycline_mg_kg_day) {
      doxyDoseInput.value = String(stg.wolbachia_doxycycline_mg_kg_day);
    }
    if (jugularPlanInput && stg.right_jugular_venotomy_extraction_plan) {
      jugularPlanInput.value = stg.right_jugular_venotomy_extraction_plan;
    }
    if (doxyPlanInput && stg.wolbachia_doxycycline_28day_plan) {
      doxyPlanInput.value = stg.wolbachia_doxycycline_28day_plan;
    }
    if (lactonePlanInput && stg.macrocyclic_lactone_pretreat_plan) {
      lactonePlanInput.value = stg.macrocyclic_lactone_pretreat_plan;
    }
    if (predDoseInput && stg.prednisone_taper_mg_kg_day) {
      predDoseInput.value = String(stg.prednisone_taper_mg_kg_day);
    }
    if (cageRestInput && stg.strict_cage_rest_weeks) {
      cageRestInput.value = String(stg.strict_cage_rest_weeks);
    }
    if (sildenafilInput && stg.sildenafil_pulmonary_hypertension_plan) {
      sildenafilInput.value = stg.sildenafil_pulmonary_hypertension_plan;
    }
    if (melarsomineInput && stg.split_dose_melarsomine_schedule_note) {
      melarsomineInput.value = stg.split_dose_melarsomine_schedule_note;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('hwc-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED'
    ) {
      const by =
        active.discharged_by ||
        active.clearance?.cleared_by ||
        'Attending Cardiologist';
      const summary =
        active.discharge_summary ||
        active.clearance?.clearance_summary ||
        'Caval lumen patent, hemoglobinuria resolved, TR jet <= 2.8 m/s, strict cage rest verified.';
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
      const weightStr = Number(active.weight_kg ?? 25.0).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Canine'} · ${active.breed || 'Mixed Breed'} · ${weightStr} kg)`;
    }

    const classEl = document.getElementById('active-severity-class');
    if (classEl) {
      classEl.textContent = active.severity_class || '';
    }
    const protoEl = document.getElementById('active-protocol');
    if (protoEl) {
      protoEl.textContent = active.protocol || '';
    }
    const guardEl = document.getElementById('active-melarsomine-guard');
    if (guardEl) {
      guardEl.textContent = active.immediate_melarsomine_contraindicated
        ? 'YES (EXTRACT FIRST)'
        : 'NO (STAGED)';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent =
        active.status || 'JUGULAR_VENOTOMY_WOLBACHIA_DOXY_STAGED';
      badgeEl.className = `hwc-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const trJetEl = document.getElementById('active-tr-jet');
    if (trJetEl) {
      const tr = Number(active.tr_jet_velocity_ms ?? 0).toFixed(2);
      const rvsp = Number(active.estimated_rvsp_mmhg ?? 0).toFixed(1);
      trJetEl.textContent = `${tr} m/s (${rvsp} mmHg)`;
    }
    const wormsEl = document.getElementById('active-worms-extracted');
    if (wormsEl) {
      wormsEl.textContent = `${Number(active.worms_extracted_count ?? 0)} worms`;
    }
    const pcvEl = document.getElementById('active-pcv');
    if (pcvEl) {
      pcvEl.textContent = `${Number(active.pcv_percent ?? 0).toFixed(1)}%`;
    }
    const hbEl = document.getElementById('active-hemoglobinuria');
    if (hbEl) {
      const hbStr = active.free_hemoglobinuria_present ? 'ACTIVE' : 'RESOLVED';
      const linesStr = active.parallel_hyperechoic_caval_lines
        ? 'LINES PRESENT'
        : 'PATENT';
      hbEl.textContent = `${hbStr} · ${linesStr}`;
    }

    const stg = active.intervention_staging || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-patho-summary', active.pathophysiology_summary || '');
    setText('plan-echo-caval', active.echocardiography_caval_note || '');
    setText('plan-hemolysis-note', active.hemolysis_hemoglobinuria_note || '');
    setText(
      'plan-adulticide-guard',
      stg.adulticide_contraindication_guard_note || ''
    );

    setText(
      'plan-jugular-extraction',
      stg.right_jugular_venotomy_extraction_plan || ''
    );
    setText(
      'plan-wolbachia-doxy',
      stg.wolbachia_doxycycline_28day_plan || ''
    );
    setText(
      'plan-lactone-pretreat',
      stg.macrocyclic_lactone_pretreat_plan || ''
    );
    setText(
      'plan-prednisone-taper',
      stg.prednisone_anti_inflammatory_taper_plan || ''
    );
    setText(
      'plan-sildenafil-ph',
      stg.sildenafil_pulmonary_hypertension_plan || ''
    );
    setText(
      'plan-split-melarsomine',
      `${stg.split_dose_melarsomine_schedule_note || ''} (Strict Cage Rest: ${Number(active.strict_cage_rest_weeks ?? 8)} Weeks)`
    );

    const checks = Array.isArray(active.hemodynamic_checks)
      ? active.hemodynamic_checks
      : [];
    const checksCountEl = document.getElementById('hemodynamic-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('hemodynamic-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Cardiologist');
          const tr = Number(chk.tr_jet_velocity_ms ?? 0).toFixed(2);
          const rvsp = Number(chk.estimated_rvsp_mmhg ?? 0).toFixed(1);
          const worms = Number(chk.worms_extracted_count ?? 0);
          const pcv = Number(chk.pcv_percent ?? 0).toFixed(1);
          const hb = chk.free_hemoglobinuria_present ? 'ACTIVE' : 'Resolved';
          const lines = chk.parallel_hyperechoic_caval_lines
            ? 'PRESENT'
            : 'Clear';
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="hwc-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · TR Jet ${tr} m/s (${rvsp} mmHg) · Worms Extracted: ${worms} · PCV ${pcv}% · Hemoglobinuria: ${hb} · Caval Lines: ${lines} — ${note}</li>`;
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
      op.status || 'JUGULAR_VENOTOMY_WOLBACHIA_DOXY_STAGED'
    );
    const badgeClass = getStatusBadgeClass(op.status);
    const severityClass = escapeHtml(op.severity_class || '');
    const protocol = escapeHtml(op.protocol || '');
    const tr = Number(op.tr_jet_velocity_ms ?? 0).toFixed(2);
    const rvsp = Number(op.estimated_rvsp_mmhg ?? 0).toFixed(1);
    const worms = Number(op.worms_extracted_count ?? 0);
    const pcv = Number(op.pcv_percent ?? 0).toFixed(1);
    const hbText = op.free_hemoglobinuria_present
      ? 'ACTIVE (Port-Wine)'
      : 'Resolved';
    const linesText = op.parallel_hyperechoic_caval_lines ? 'PRESENT' : 'Clear';

    return `
      <article
        class="trust-screening-card hwc-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-severity-class="${severityClass}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="hwc-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="hwc-card-metrics">
          <span><strong>Class:</strong> ${severityClass}</span>
          <span><strong>Protocol:</strong> ${protocol}</span>
          <span><strong>TR Jet:</strong> ${tr} m/s (${rvsp} mmHg)</span>
          <span><strong>Worms Extracted:</strong> ${worms}</span>
          <span><strong>PCV:</strong> ${pcv}%</span>
          <span><strong>Hemoglobinuria:</strong> ${hbText}</span>
          <span><strong>Caval Lines:</strong> ${linesText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary hwc-select-op-btn" data-op-id="${opId}">
            🔍 Open Caval Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('hwc-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('hwc-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="hwc-ops-empty-msg">No Heartworm Caval Syndrome operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('hwc-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/heartworm-caval/operations?${qs}`
        : '/api/heartworm-caval/operations';

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
      renderAllOperations(ops, data.metrics || data.overview);
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
        `/api/heartworm-caval/operations/${encodeURIComponent(opId)}`,
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
        statusBadge.className = 'hwc-status-badge hwc-status-cleared';
      }
      eventSource = new EventSource('/api/heartworm-caval/stream');
      eventSource.addEventListener('heartworm_caval_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('hwc-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-hwc-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value ||
          'CASE-HWC-104'
        ).trim();
        const petName = (
          document.getElementById('create-pet-name-input')?.value || ''
        ).trim();
        const species = (
          document.getElementById('create-species-input')?.value || 'Canine'
        ).trim();
        const breed = (
          document.getElementById('create-breed-input')?.value ||
          'Catahoula Leopard Dog'
        ).trim();
        const weightKg = parseFloat(
          document.getElementById('create-weight-input')?.value || '26.0'
        );
        const severityClass = (
          document.getElementById('create-severity-class-input')?.value ||
          'CLASS_IV_CAVAL_SYNDROME'
        ).trim();
        const protocol = (
          document.getElementById('create-protocol-input')?.value ||
          'RIGHT_JUGULAR_ISHIHARA_FORCEPS_EXTRACTION'
        ).trim();
        const trJet = parseFloat(
          document.getElementById('create-tr-jet-input')?.value || '4.05'
        );
        const wormsExtracted = parseInt(
          document.getElementById('create-worms-extracted-input')?.value || '0',
          10
        );
        const pcv = parseFloat(
          document.getElementById('create-pcv-input')?.value || '19.5'
        );
        const hemoglobinuria = Boolean(
          document.getElementById('create-hemoglobinuria-checkbox')?.checked
        );
        const cavalLines = Boolean(
          document.getElementById('create-caval-lines-checkbox')?.checked
        );

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 26.0 : weightKg,
          severity_class: severityClass,
          protocol,
          tr_jet_velocity_ms: isNaN(trJet) ? 4.05 : trJet,
          worms_extracted_count: isNaN(wormsExtracted) ? 0 : wormsExtracted,
          pcv_percent: isNaN(pcv) ? 19.5 : pcv,
          free_hemoglobinuria_present: hemoglobinuria,
          parallel_hyperechoic_caval_lines: cavalLines,
        };

        const btn = document.getElementById('create-hwc-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/heartworm-caval/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted Heartworm Caval patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to admit Heartworm Caval patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-intervention-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const protocol = (
          document.getElementById('stage-protocol-input')?.value ||
          'RIGHT_JUGULAR_ISHIHARA_FORCEPS_EXTRACTION'
        ).trim();
        const doxyDose = parseFloat(
          document.getElementById('stage-doxy-dose-input')?.value || '20.0'
        );
        const jugularPlan = (
          document.getElementById('stage-jugular-plan-input')?.value ||
          'Right external jugular venotomy with flexible Ishihara alligator forceps under fluoroscopic/echocardiographic guidance'
        ).trim();
        const doxyPlan = (
          document.getElementById('stage-doxy-plan-input')?.value ||
          'Doxycycline 10 mg/kg PO q12h for 28 days to eliminate Wolbachia pipientis'
        ).trim();
        const lactonePlan = (
          document.getElementById('stage-lactone-plan-input')?.value ||
          'Macrocyclic lactone preventive Day 1, 30, 60 with Diphenhydramine + Prednisone pretreatment'
        ).trim();
        const predDose = parseFloat(
          document.getElementById('stage-pred-dose-input')?.value || '1.0'
        );
        const cageRestWeeks = parseInt(
          document.getElementById('stage-cage-rest-weeks-input')?.value || '8',
          10
        );
        const sildenafilPlan = (
          document.getElementById('stage-sildenafil-plan-input')?.value || ''
        ).trim();
        const melarsomineNote = (
          document.getElementById('stage-melarsomine-note-input')?.value || ''
        ).trim();

        const payload = {
          protocol,
          right_jugular_venotomy_extraction_plan: jugularPlan,
          immediate_melarsomine_contraindicated: true,
          wolbachia_doxycycline_mg_kg_day: isNaN(doxyDose) ? 20.0 : doxyDose,
          wolbachia_doxycycline_28day_plan: doxyPlan,
          macrocyclic_lactone_pretreat_plan: lactonePlan,
          prednisone_taper_mg_kg_day: isNaN(predDose) ? 1.0 : predDose,
          sildenafil_pulmonary_hypertension_plan: sildenafilPlan,
          split_dose_melarsomine_schedule_note: melarsomineNote,
          strict_cage_rest_weeks: isNaN(cageRestWeeks) ? 8 : cageRestWeeks,
          staged_by: 'Dr. Thibodaux Broussard, DACVIM (Cardiology)',
        };

        const btn = document.getElementById('stage-intervention-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/heartworm-caval/operations/${encodeURIComponent(selectedOpId)}/intervention`,
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
              `Staged Jugular Venotomy & Wolbachia Protocol for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to stage Heartworm Caval intervention';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-hemodynamic-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const trJet = parseFloat(
          document.getElementById('check-tr-jet-input')?.value || '2.35'
        );
        const wormsExtracted = parseInt(
          document.getElementById('check-worms-extracted-input')?.value || '31',
          10
        );
        const pcv = parseFloat(
          document.getElementById('check-pcv-input')?.value || '36.0'
        );
        const sildenafilDose = parseFloat(
          document.getElementById('check-sildenafil-dose-input')?.value || '1.5'
        );
        const hemoglobinuria = Boolean(
          document.getElementById('check-hemoglobinuria-checkbox')?.checked
        );
        const cavalLines = Boolean(
          document.getElementById('check-caval-lines-checkbox')?.checked
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Thibodaux Broussard, DACVIM (Cardiology)'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          tr_jet_velocity_ms: isNaN(trJet) ? 2.35 : trJet,
          worms_extracted_count: isNaN(wormsExtracted) ? 31 : wormsExtracted,
          pcv_percent: isNaN(pcv) ? 36.0 : pcv,
          free_hemoglobinuria_present: hemoglobinuria,
          parallel_hyperechoic_caval_lines: cavalLines,
          sildenafil_mg_kg_dose: isNaN(sildenafilDose) ? 1.5 : sildenafilDose,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-hemodynamic-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/heartworm-caval/operations/${encodeURIComponent(selectedOpId)}/hemodynamic-check`,
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
              `Recorded echocardiography & hemolysis check for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record hemodynamic check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-hwc-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const trJet = parseFloat(
          document.getElementById('clear-tr-jet-input')?.value || '2.35'
        );
        const cageRestVerified = Boolean(
          document.getElementById('clear-cage-rest-checkbox')?.checked
        );
        const clearedBy = (
          document.getElementById('clear-cleared-by-input')?.value ||
          'Dr. Thibodaux Broussard, DACVIM (Cardiology)'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          parallel_hyperechoic_caval_lines: false,
          free_hemoglobinuria_present: false,
          tr_jet_velocity_ms: isNaN(trJet) ? 2.35 : trJet,
          strict_cage_rest_verified: cageRestVerified,
          cleared_by: clearedBy,
          clearance_summary: summary,
        };

        const btn = document.getElementById('clear-hwc-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/heartworm-caval/operations/${encodeURIComponent(selectedOpId)}/clearance`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Patent Vena Cava & Antigen-Negative Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to clear Heartworm Caval operation';
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
    const gridEl = document.getElementById('hwc-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.hwc-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.hwc-op-card');
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
