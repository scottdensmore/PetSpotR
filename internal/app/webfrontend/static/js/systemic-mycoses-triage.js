/**
 * PetSpotR Acute Systemic Mycoses & Fungal Emergencies (Blastomycosis, Histoplasmosis,
 * Coccidioidomycosis, Cryptococcosis & Sinonasal Aspergillosis), Urine/Serum MiraVista Antigen EIA,
 * Itraconazole/Fluconazole, Amphotericin B Lipid Complex Nephrotoxicity Guard &
 * Clotrimazole Trephination Cockpit Controller (Milestone 25.3)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'MYC-OP-101';
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
    const liveStatus = document.getElementById('myc-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('myc-error-banner');
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
      case 'AZOLE_AMPHOTERICIN_TREPHINATION_STAGED':
        return 'myc-status-critical';
      case 'ANTIGEN_EIA_CREA_OPHTHALMIC_MONITORED':
        return 'myc-status-staged';
      case 'TWO_NEGATIVE_ANTIGEN_EIA_CLEARED':
        return 'myc-status-cleared';
      default:
        return 'myc-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const stagedEl = document.getElementById('kpi-azole-staged');
    const monitoredEl = document.getElementById('kpi-antigen-monitored');
    const clearedEl = document.getElementById('kpi-antigen-cleared');
    const blastoHistoEl = document.getElementById('kpi-blasto-histo-count');
    const cribriformEl = document.getElementById('kpi-cribriform-breach-count');

    const total = metrics?.total_operations ?? ops.length;
    const staged =
      metrics?.azole_amphotericin_trephination_staged_count ??
      ops.filter(
        (op) => op.status === 'AZOLE_AMPHOTERICIN_TREPHINATION_STAGED'
      ).length;
    const monitored =
      metrics?.antigen_eia_crea_ophthalmic_monitored_count ??
      ops.filter(
        (op) => op.status === 'ANTIGEN_EIA_CREA_OPHTHALMIC_MONITORED'
      ).length;
    const cleared =
      metrics?.two_negative_antigen_eia_cleared_count ??
      ops.filter((op) => op.status === 'TWO_NEGATIVE_ANTIGEN_EIA_CLEARED')
        .length;
    const blastoHisto =
      metrics?.blastomycosis_histoplasmosis_count ??
      ops.filter(
        (op) =>
          op.syndrome === 'BLASTOMYCOSIS_BROAD_BASED_BUDDING_YEAST' ||
          op.syndrome === 'HISTOPLASMOSIS_INTRACELLULAR_MACROPHAGE'
      ).length;
    const cribriformBreach =
      metrics?.cribriform_breach_contraindicated_count ??
      ops.filter((op) => !op.cribriform_plate_intact_on_ct).length;

    if (totalEl) totalEl.textContent = String(total);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (monitoredEl) monitoredEl.textContent = String(monitored);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (blastoHistoEl) blastoHistoEl.textContent = String(blastoHisto);
    if (cribriformEl) cribriformEl.textContent = String(cribriformBreach);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const stg = op.therapy_staging || {};
    const protocolInput = document.getElementById('stage-protocol-input');
    const itraDoseInput = document.getElementById(
      'stage-itraconazole-dose-input'
    );
    const flucoDoseInput = document.getElementById(
      'stage-fluconazole-dose-input'
    );
    const amphoDoseInput = document.getElementById('stage-ampho-dose-input');
    const azolePlanInput = document.getElementById('stage-azole-plan-input');
    const dieOffPlanInput = document.getElementById('stage-die-off-plan-input');
    const durationInput = document.getElementById(
      'stage-duration-months-input'
    );
    const compoundedCheckbox = document.getElementById(
      'stage-compounded-blocked-checkbox'
    );
    const cribriformCheckbox = document.getElementById(
      'stage-cribriform-intact-checkbox'
    );
    const salineCheckbox = document.getElementById(
      'stage-saline-diuresis-checkbox'
    );
    const dieOffCheckbox = document.getElementById(
      'stage-die-off-dex-checkbox'
    );

    if (protocolInput && (stg.protocol || op.protocol)) {
      protocolInput.value = stg.protocol || op.protocol;
    }
    if (itraDoseInput && stg.itraconazole_mg_kg_day) {
      itraDoseInput.value = String(stg.itraconazole_mg_kg_day);
    }
    if (flucoDoseInput && stg.fluconazole_mg_kg_day) {
      flucoDoseInput.value = String(stg.fluconazole_mg_kg_day);
    }
    if (amphoDoseInput && stg.amphotericin_b_lipid_mg_kg_dose) {
      amphoDoseInput.value = String(stg.amphotericin_b_lipid_mg_kg_dose);
    }
    if (azolePlanInput && stg.systemic_azole_and_polyene_regimen_plan) {
      azolePlanInput.value = stg.systemic_azole_and_polyene_regimen_plan;
    }
    if (dieOffPlanInput && stg.pulmonary_die_off_ards_plan) {
      dieOffPlanInput.value = stg.pulmonary_die_off_ards_plan;
    }
    if (durationInput && stg.minimum_azole_duration_months) {
      durationInput.value = String(stg.minimum_azole_duration_months);
    }
    if (compoundedCheckbox) {
      compoundedCheckbox.checked = Boolean(
        op.compounded_bulk_itraconazole_blocked
      );
    }
    if (cribriformCheckbox) {
      cribriformCheckbox.checked = Boolean(op.cribriform_plate_intact_on_ct);
    }
    if (salineCheckbox) {
      salineCheckbox.checked = Boolean(
        op.amphotericin_lipid_saline_diuresis_verified
      );
    }
    if (dieOffCheckbox) {
      dieOffCheckbox.checked = Boolean(
        op.die_off_dexamethasone_anti_inflammatory_staged
      );
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('myc-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (active && active.status === 'TWO_NEGATIVE_ANTIGEN_EIA_CLEARED') {
      const by =
        active.discharged_by ||
        active.clearance?.cleared_by ||
        'Attending Internist';
      const summary =
        active.discharge_summary ||
        active.clearance?.clearance_summary ||
        'Two consecutive negative MiraVista Antigen EIA tests (0.0 ng/mL) 1 month apart, Creatinine <= 1.8 mg/dL, and 6-month azole course verified.';
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

    const synEl = document.getElementById('active-syndrome');
    if (synEl) {
      synEl.textContent = active.syndrome || '';
    }
    const protoEl = document.getElementById('active-protocol');
    if (protoEl) {
      protoEl.textContent = active.protocol || '';
    }
    const guardEl = document.getElementById('active-compounded-guard');
    if (guardEl) {
      guardEl.textContent = active.compounded_bulk_itraconazole_blocked
        ? 'YES (INNOVATOR/SOLUTION ONLY)'
        : 'NO (UNSAFE)';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent =
        active.status || 'AZOLE_AMPHOTERICIN_TREPHINATION_STAGED';
      badgeEl.className = `myc-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const antigenEl = document.getElementById('active-antigen');
    if (antigenEl) {
      const ag = Number(active.miravista_antigen_ng_ml ?? 0).toFixed(2);
      antigenEl.textContent = `${ag} ng/mL`;
    }
    const creaEl = document.getElementById('active-creatinine');
    if (creaEl) {
      const cr = Number(active.serum_creatinine_mg_dl ?? 0).toFixed(2);
      creaEl.textContent = `${cr} mg/dL`;
    }
    const altEl = document.getElementById('active-alt');
    if (altEl) {
      const alt = Number(active.alt_activity_u_l ?? 0).toFixed(0);
      altEl.textContent = `${alt} U/L`;
    }
    const spo2El = document.getElementById('active-spo2-cribriform');
    if (spo2El) {
      const sp = Number(active.spo2_percent ?? 0).toFixed(1);
      const ctStr = active.cribriform_plate_intact_on_ct
        ? 'CT INTACT'
        : 'CT BREACHED';
      spo2El.textContent = `${sp}% · ${ctStr}`;
    }

    const stg = active.therapy_staging || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-patho-summary', active.pathophysiology_summary || '');
    setText(
      'plan-cytology-antigen',
      active.cytology_antigen_diagnostic_note || ''
    );
    setText('plan-renal-hepatic', active.renal_hepatic_safety_note || '');
    setText(
      'plan-cribriform-guard',
      stg.cribriform_plate_trephination_guard_note || ''
    );

    setText(
      'plan-compounded-guard',
      stg.compounded_itraconazole_guard_note || ''
    );
    setText(
      'plan-ampho-guard',
      stg.amphotericin_nephrotoxicity_guard_note || ''
    );
    setText(
      'plan-azole-regimen',
      `${stg.systemic_azole_and_polyene_regimen_plan || ''} (Minimum Duration: ${Number(stg.minimum_azole_duration_months ?? 6)} Months)`
    );
    setText('plan-die-off-ards', stg.pulmonary_die_off_ards_plan || '');
    setText(
      'plan-ophthalmic-neuro',
      stg.ophthalmic_chorioretinitis_neuro_plan || ''
    );

    const checks = Array.isArray(active.antigen_renal_checks)
      ? active.antigen_renal_checks
      : [];
    const checksCountEl = document.getElementById('antigen-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('antigen-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Internist');
          const ag = Number(chk.miravista_antigen_ng_ml ?? 0).toFixed(2);
          const cr = Number(chk.serum_creatinine_mg_dl ?? 0).toFixed(2);
          const alt = Number(chk.alt_activity_u_l ?? 0).toFixed(0);
          const sp = Number(chk.spo2_percent ?? 0).toFixed(1);
          const chorio = chk.ophthalmic_chorioretinitis_resolved
            ? 'Resolved'
            : 'Active';
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="myc-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · MiraVista EIA ${ag} ng/mL · Creatinine ${cr} mg/dL · ALT ${alt} U/L · SpO2 ${sp}% · Chorioretinitis: ${chorio} — ${note}</li>`;
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
      op.status || 'AZOLE_AMPHOTERICIN_TREPHINATION_STAGED'
    );
    const badgeClass = getStatusBadgeClass(op.status);
    const syndrome = escapeHtml(op.syndrome || '');
    const protocol = escapeHtml(op.protocol || '');
    const ag = Number(op.miravista_antigen_ng_ml ?? 0).toFixed(2);
    const cr = Number(op.serum_creatinine_mg_dl ?? 0).toFixed(2);
    const alt = Number(op.alt_activity_u_l ?? 0).toFixed(0);
    const sp = Number(op.spo2_percent ?? 0).toFixed(1);
    const ctText = op.cribriform_plate_intact_on_ct
      ? 'Intact'
      : 'BREACHED (No Trephination)';

    return `
      <article
        class="trust-screening-card myc-op-card"
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
            <span class="myc-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="myc-card-metrics">
          <span><strong>Syndrome:</strong> ${syndrome}</span>
          <span><strong>Protocol:</strong> ${protocol}</span>
          <span><strong>MiraVista EIA:</strong> ${ag} ng/mL</span>
          <span><strong>Creatinine:</strong> ${cr} mg/dL</span>
          <span><strong>ALT:</strong> ${alt} U/L</span>
          <span><strong>SpO2:</strong> ${sp}%</span>
          <span><strong>Cribriform CT:</strong> ${ctText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary myc-select-op-btn" data-op-id="${opId}">
            🔍 Open Mycoses Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('myc-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('myc-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="myc-ops-empty-msg">No Systemic Mycoses operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('myc-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/systemic-mycoses/operations?${qs}`
        : '/api/systemic-mycoses/operations';

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
        `/api/systemic-mycoses/operations/${encodeURIComponent(opId)}`,
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
        statusBadge.className = 'myc-status-badge myc-status-cleared';
      }
      eventSource = new EventSource('/api/systemic-mycoses/stream');
      eventSource.addEventListener('systemic_mycoses_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('myc-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-myc-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value ||
          'CASE-MYC-104'
        ).trim();
        const petName = (
          document.getElementById('create-pet-name-input')?.value || ''
        ).trim();
        const species = (
          document.getElementById('create-species-input')?.value || 'Canine'
        ).trim();
        const breed = (
          document.getElementById('create-breed-input')?.value ||
          'German Shorthaired Pointer'
        ).trim();
        const weightKg = parseFloat(
          document.getElementById('create-weight-input')?.value || '27.5'
        );
        const syndrome = (
          document.getElementById('create-syndrome-input')?.value ||
          'BLASTOMYCOSIS_BROAD_BASED_BUDDING_YEAST'
        ).trim();
        const protocol = (
          document.getElementById('create-protocol-input')?.value ||
          'ITRACONAZOLE_5MG_KG_BRAND_OR_SOLUTION'
        ).trim();
        const antigen = parseFloat(
          document.getElementById('create-antigen-input')?.value || '12.6'
        );
        const creatinine = parseFloat(
          document.getElementById('create-creatinine-input')?.value || '1.25'
        );
        const alt = parseFloat(
          document.getElementById('create-alt-input')?.value || '74'
        );
        const spo2 = parseFloat(
          document.getElementById('create-spo2-input')?.value || '90.5'
        );
        const cribriformIntact = Boolean(
          document.getElementById('create-cribriform-intact-checkbox')?.checked
        );
        const compoundedBlocked = Boolean(
          document.getElementById('create-compounded-blocked-checkbox')?.checked
        );
        const salineDiuresis = Boolean(
          document.getElementById('create-saline-diuresis-checkbox')?.checked
        );

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 27.5 : weightKg,
          syndrome,
          protocol,
          miravista_antigen_ng_ml: isNaN(antigen) ? 12.6 : antigen,
          serum_creatinine_mg_dl: isNaN(creatinine) ? 1.25 : creatinine,
          alt_activity_u_l: isNaN(alt) ? 74.0 : alt,
          spo2_percent: isNaN(spo2) ? 90.5 : spo2,
          cribriform_plate_intact_on_ct: cribriformIntact,
          compounded_bulk_itraconazole_blocked: compoundedBlocked,
          amphotericin_lipid_saline_diuresis_verified: salineDiuresis,
        };

        const btn = document.getElementById('create-myc-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/systemic-mycoses/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted Systemic Mycoses patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to admit Systemic Mycoses patient';
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
          'ITRACONAZOLE_5MG_KG_BRAND_OR_SOLUTION'
        ).trim();
        const itraDose = parseFloat(
          document.getElementById('stage-itraconazole-dose-input')?.value ||
            '5.0'
        );
        const flucoDose = parseFloat(
          document.getElementById('stage-fluconazole-dose-input')?.value ||
            '20.0'
        );
        const amphoDose = parseFloat(
          document.getElementById('stage-ampho-dose-input')?.value || '2.0'
        );
        const azolePlan = (
          document.getElementById('stage-azole-plan-input')?.value ||
          'FDA-approved innovator Itraconazole 5.0 mg/kg/day PO with high-fat meal + Amphotericin B Lipid Complex 2.0 mg/kg IV 3x/wk with 0.9% NaCl diuresis'
        ).trim();
        const dieOffPlan = (
          document.getElementById('stage-die-off-plan-input')?.value || ''
        ).trim();
        const durationMonths = parseInt(
          document.getElementById('stage-duration-months-input')?.value || '6',
          10
        );
        const compoundedBlocked = Boolean(
          document.getElementById('stage-compounded-blocked-checkbox')?.checked
        );
        const cribriformIntact = Boolean(
          document.getElementById('stage-cribriform-intact-checkbox')?.checked
        );
        const salineDiuresis = Boolean(
          document.getElementById('stage-saline-diuresis-checkbox')?.checked
        );
        const dieOffDex = Boolean(
          document.getElementById('stage-die-off-dex-checkbox')?.checked
        );

        const payload = {
          protocol,
          itraconazole_mg_kg_day: isNaN(itraDose) ? 5.0 : itraDose,
          fluconazole_mg_kg_day: isNaN(flucoDose) ? 20.0 : flucoDose,
          amphotericin_b_lipid_mg_kg_dose: isNaN(amphoDose) ? 2.0 : amphoDose,
          compounded_bulk_itraconazole_blocked: compoundedBlocked,
          cribriform_plate_intact_on_ct: cribriformIntact,
          amphotericin_lipid_saline_diuresis_verified: salineDiuresis,
          die_off_dexamethasone_anti_inflammatory_staged: dieOffDex,
          systemic_azole_and_polyene_regimen_plan: azolePlan,
          pulmonary_die_off_ards_plan: dieOffPlan,
          minimum_azole_duration_months: isNaN(durationMonths)
            ? 6
            : durationMonths,
          staged_by:
            'Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)',
        };

        const btn = document.getElementById('stage-therapy-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/systemic-mycoses/operations/${encodeURIComponent(selectedOpId)}/therapy`,
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
              `Staged Antifungal & Nephrotoxicity Guard Protocol for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to stage Systemic Mycoses therapy';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-antigen-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const antigen = parseFloat(
          document.getElementById('check-antigen-input')?.value || '0.0'
        );
        const creatinine = parseFloat(
          document.getElementById('check-creatinine-input')?.value || '1.15'
        );
        const alt = parseFloat(
          document.getElementById('check-alt-input')?.value || '58'
        );
        const spo2 = parseFloat(
          document.getElementById('check-spo2-input')?.value || '98.0'
        );
        const cribriformIntact = Boolean(
          document.getElementById('check-cribriform-intact-checkbox')?.checked
        );
        const amphoHeld = Boolean(
          document.getElementById('check-ampho-held-checkbox')?.checked
        );
        const chorioResolved = Boolean(
          document.getElementById('check-chorioretinitis-resolved-checkbox')
            ?.checked
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          miravista_antigen_ng_ml: isNaN(antigen) ? 0.0 : antigen,
          serum_creatinine_mg_dl: isNaN(creatinine) ? 1.15 : creatinine,
          alt_activity_u_l: isNaN(alt) ? 58.0 : alt,
          spo2_percent: isNaN(spo2) ? 98.0 : spo2,
          cribriform_plate_intact_on_ct: cribriformIntact,
          amphotericin_dose_held_for_azotemia: amphoHeld,
          ophthalmic_chorioretinitis_resolved: chorioResolved,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-antigen-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/systemic-mycoses/operations/${encodeURIComponent(selectedOpId)}/antigen-check`,
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
              `Recorded MiraVista Antigen EIA & Renal Check for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record antigen/renal check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-myc-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const antigen = parseFloat(
          document.getElementById('clear-antigen-input')?.value || '0.0'
        );
        const creatinine = parseFloat(
          document.getElementById('clear-creatinine-input')?.value || '1.15'
        );
        const twoNegativeVerified = Boolean(
          document.getElementById('clear-two-negative-eia-checkbox')?.checked
        );
        const sixMonthVerified = Boolean(
          document.getElementById('clear-six-month-azole-checkbox')?.checked
        );
        const clearedBy = (
          document.getElementById('clear-cleared-by-input')?.value ||
          'Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          miravista_antigen_ng_ml: isNaN(antigen) ? 0.0 : antigen,
          serum_creatinine_mg_dl: isNaN(creatinine) ? 1.15 : creatinine,
          two_consecutive_negative_eia_verified: twoNegativeVerified,
          minimum_6month_azole_course_verified: sixMonthVerified,
          cleared_by: clearedBy,
          clearance_summary: summary,
        };

        const btn = document.getElementById('clear-myc-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/systemic-mycoses/operations/${encodeURIComponent(selectedOpId)}/clearance`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Two-Negative MiraVista Antigen EIA Cure`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to clear Systemic Mycoses operation';
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
    const gridEl = document.getElementById('myc-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.myc-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        selectOperationById(opId);
        return;
      }
      const card = e.target.closest('.myc-op-card');
      if (card) {
        const opId =
          card.getAttribute('data-op-id') ||
          card.getAttribute('data-operation-id');
        selectOperationById(opId);
      }
    });
  }

  function setupThemeToggle() {
    const toggleBtn = document.getElementById('theme-toggle');
    if (!toggleBtn) return;
    toggleBtn.addEventListener('click', () => {
      const htmlEl = document.documentElement;
      const current = htmlEl.getAttribute('data-theme') || 'dark';
      htmlEl.setAttribute('data-theme', current === 'dark' ? 'light' : 'dark');
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
