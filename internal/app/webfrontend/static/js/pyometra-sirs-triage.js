/**
 * PetSpotR Acute Pyometra (Open vs. Closed Cervix), POCUS Fluid-Filled Uterine Horns,
 * E. coli Endotoxemia/SIRS, Stabilization & Ovariohysterectomy (OHE) Cockpit Controller (Milestone 23.1)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'PYO-OP-101';
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
    const liveStatus = document.getElementById('pyometra-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('pyometra-error-banner');
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
      case 'CLOSED_CERVIX_ENDOTOXIC_SIRS_TRIAGE':
        return 'pyometra-status-critical';
      case 'RESUSCITATED_EMPIRIC_ABX_OHE_STAGED':
        return 'pyometra-status-staged';
      case 'POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED':
        return 'pyometra-status-cleared';
      default:
        return 'pyometra-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const closedSirsEl = document.getElementById('kpi-closed-sirs');
    const stagedEl = document.getElementById('kpi-ohe-staged');
    const clearedEl = document.getElementById('kpi-cleared-ohe');
    const ruptureEl = document.getElementById('kpi-rupture-risk');
    const effusionEl = document.getElementById('kpi-peritoneal-effusion');

    const total = metrics?.total_operations ?? ops.length;
    const closedSirs =
      metrics?.closed_cervix_sirs_triage_count ??
      ops.filter((op) => op.status === 'CLOSED_CERVIX_ENDOTOXIC_SIRS_TRIAGE').length;
    const staged =
      metrics?.ohe_staged_count ??
      ops.filter((op) => op.status === 'RESUSCITATED_EMPIRIC_ABX_OHE_STAGED').length;
    const cleared =
      metrics?.cleared_postop_ohe_count ??
      ops.filter((op) => op.status === 'POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED').length;
    const rupture =
      metrics?.closed_cervix_rupture_risk_count ??
      ops.filter((op) => op.cervix_type === 'CLOSED_CERVIX_RUPTURE_RISK').length;
    const effusion =
      metrics?.peritoneal_effusion_alert_count ??
      ops.filter((op) => Boolean(op.pocus_free_peritoneal_fluid)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (closedSirsEl) closedSirsEl.textContent = String(closedSirs);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (ruptureEl) ruptureEl.textContent = String(rupture);
    if (effusionEl) effusionEl.textContent = String(effusion);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const ohe = op.ohe_plan || {};
    const crystInput = document.getElementById('stage-crystalloid-plan-input');
    const abxInput = document.getElementById('stage-empiric-abx-input');
    const oheInput = document.getElementById('stage-ohe-handling-input');
    const lavageInput = document.getElementById('stage-lavage-plan-input');
    const analgesiaInput = document.getElementById('stage-analgesia-plan-input');
    const sepsisInput = document.getElementById('stage-sepsis-monitoring-input');

    if (crystInput && ohe.crystalloid_resuscitation_plan) {
      crystInput.value = ohe.crystalloid_resuscitation_plan;
    }
    if (abxInput && ohe.empiric_bactericidal_abx_plan) {
      abxInput.value = ohe.empiric_bactericidal_abx_plan;
    }
    if (oheInput && ohe.surgical_ohe_handling_protocol) {
      oheInput.value = ohe.surgical_ohe_handling_protocol;
    }
    if (lavageInput && ohe.abdominal_lavage_plan) {
      lavageInput.value = ohe.abdominal_lavage_plan;
    }
    if (analgesiaInput && ohe.perioperative_analgesia_plan) {
      analgesiaInput.value = ohe.perioperative_analgesia_plan;
    }
    if (sepsisInput && ohe.postop_sepsis_monitoring) {
      sepsisInput.value = ohe.postop_sepsis_monitoring;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('pyometra-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED'
    ) {
      const by = active.discharged_by || 'Attending Surgeon';
      const summary =
        active.discharge_summary ||
        'Post-op Ovariohysterectomy (OHE) normotensive, afebrile, and cleared for discharge.';
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
      const weightStr = Number(active.weight_kg ?? 20.0).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Canine'} · ${active.breed || 'Mixed Breed'} · ${weightStr} kg)`;
    }

    const cervixEl = document.getElementById('active-cervix-type');
    if (cervixEl) {
      cervixEl.textContent = active.cervix_type || '';
    }
    const fluidEl = document.getElementById('active-free-fluid-flag');
    if (fluidEl) {
      fluidEl.textContent = active.pocus_free_peritoneal_fluid ? 'YES' : 'NO';
    }
    const pupdEl = document.getElementById('active-pupd-flag');
    if (pupdEl) {
      pupdEl.textContent = active.polyuria_polydipsia_present ? 'YES' : 'NO';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent = active.status || 'CLOSED_CERVIX_ENDOTOXIC_SIRS_TRIAGE';
      badgeEl.className = `pyometra-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const diamEl = document.getElementById('active-uterine-diameter');
    if (diamEl) {
      diamEl.textContent = `${Number(active.uterine_horn_diameter_cm ?? 0).toFixed(1)} cm`;
    }
    const lactEl = document.getElementById('active-blood-lactate');
    if (lactEl) {
      lactEl.textContent = `${Number(active.blood_lactate_mmol_l ?? 0).toFixed(1)} mmol/L`;
    }
    const wbcEl = document.getElementById('active-wbc-bands');
    if (wbcEl) {
      wbcEl.textContent = `${Number(active.wbc_count_k_ul ?? 15.0).toFixed(1)} K/µL (${Number(active.band_neutrophil_percent ?? 0).toFixed(1)}% Bands)`;
    }
    const mapEl = document.getElementById('active-map-mmhg');
    if (mapEl) {
      mapEl.textContent = `${Number(active.map_mm_hg ?? 70)} mmHg`;
    }

    const sirs = active.sirs_plan || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-cervix-summary', sirs.cervix_patency_summary || '');
    setText('plan-pocus-finding', sirs.pocus_uterine_horn_finding || '');
    setText('plan-ecoli-adh', sirs.e_coli_adh_resistance_polyuria_note || '');
    setText('plan-cystocentesis-guard', sirs.cystocentesis_contraindication || '');
    setText('plan-prostaglandin-warning', sirs.prostaglandin_luteolysis_warning || '');
    setText('plan-sirs-summary', sirs.sirs_lactate_shock_summary || '');

    const ohe = active.ohe_plan || {};
    setText('plan-crystalloid', ohe.crystalloid_resuscitation_plan || '');
    setText('plan-empiric-abx', ohe.empiric_bactericidal_abx_plan || '');
    setText('plan-ohe-handling', ohe.surgical_ohe_handling_protocol || '');
    setText('plan-abdominal-lavage', ohe.abdominal_lavage_plan || '');
    setText('plan-analgesia', ohe.perioperative_analgesia_plan || '');
    setText('plan-sepsis-monitoring', ohe.postop_sepsis_monitoring || '');

    const checks = Array.isArray(active.hemodynamic_checks) ? active.hemodynamic_checks : [];
    const checksCountEl = document.getElementById('hemodynamic-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('hemodynamic-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const diam = Number(chk.uterine_horn_diameter_cm ?? 0).toFixed(1);
          const lact = Number(chk.blood_lactate_mmol_l ?? 0).toFixed(1);
          const wbc = Number(chk.wbc_count_k_ul ?? 15.0).toFixed(1);
          const bands = Number(chk.band_neutrophil_percent ?? 0).toFixed(1);
          const mapVal = Number(chk.map_mm_hg ?? 75);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="pyometra-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · Uterine Horn ${diam} cm · Lactate ${lact} mmol/L · WBC ${wbc} K/µL (Bands ${bands}%) · MAP <code>${mapVal} mmHg</code> — ${note}</li>`;
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
    const weightKg = Number(op.weight_kg ?? 20.0).toFixed(1);
    const status = escapeHtml(op.status || 'CLOSED_CERVIX_ENDOTOXIC_SIRS_TRIAGE');
    const badgeClass = getStatusBadgeClass(op.status);
    const cervix = escapeHtml(op.cervix_type || '');
    const diam = Number(op.uterine_horn_diameter_cm ?? 0).toFixed(1);
    const lact = Number(op.blood_lactate_mmol_l ?? 0).toFixed(1);
    const wbc = Number(op.wbc_count_k_ul ?? 15.0).toFixed(1);
    const bands = Number(op.band_neutrophil_percent ?? 0).toFixed(1);
    const mapVal = Number(op.map_mm_hg ?? 70);
    const fluidText = op.pocus_free_peritoneal_fluid ? 'YES (Rupture Alert)' : 'NO';
    const pupdText = op.polyuria_polydipsia_present ? 'YES (E. coli ADH Block)' : 'NO';

    return `
      <article
        class="trust-screening-card pyometra-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-cervix-type="${cervix}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="pyometra-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="pyometra-card-metrics">
          <span><strong>Cervix:</strong> ${cervix}</span>
          <span><strong>Uterine Horn:</strong> ${diam} cm</span>
          <span><strong>Lactate:</strong> ${lact} mmol/L</span>
          <span><strong>WBC:</strong> ${wbc} K/µL (Bands ${bands}%)</span>
          <span><strong>MAP:</strong> ${mapVal} mmHg</span>
          <span><strong>Free Fluid:</strong> ${fluidText}</span>
          <span><strong>PU/PD:</strong> ${pupdText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary pyometra-select-op-btn" data-op-id="${opId}">
            🔍 Open Pyometra Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('pyometra-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('pyometra-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="pyometra-ops-empty-msg">No Pyometra / Endotoxic SIRS operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (document.getElementById('pyometra-status-filter')?.value || '').trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/pyometra-sirs/operations?${qs}`
        : '/api/pyometra-sirs/operations';

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
      const res = await fetch(`/api/pyometra-sirs/operations/${encodeURIComponent(opId)}`, {
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
        statusBadge.className = 'pyometra-status-badge pyometra-status-cleared';
      }
      eventSource = new EventSource('/api/pyometra-sirs/stream');
      eventSource.addEventListener('pyometra_sirs_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('pyometra-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-pyometra-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (document.getElementById('create-case-id-input')?.value || 'CASE-PYO-104').trim();
        const petName = (document.getElementById('create-pet-name-input')?.value || '').trim();
        const species = (document.getElementById('create-species-input')?.value || 'Canine').trim();
        const breed = (document.getElementById('create-breed-input')?.value || 'Bernese Mountain Dog').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-input')?.value || '38.2');
        const cervixType = (
          document.getElementById('create-cervix-type-input')?.value ||
          'CLOSED_CERVIX_RUPTURE_RISK'
        ).trim();
        const hornDiameter = parseFloat(document.getElementById('create-horn-diameter-input')?.value || '6.1');
        const lactate = parseFloat(document.getElementById('create-lactate-input')?.value || '5.1');
        const wbc = parseFloat(document.getElementById('create-wbc-input')?.value || '36.4');
        const bands = parseFloat(document.getElementById('create-bands-input')?.value || '20.0');
        const mapMmHg = parseInt(document.getElementById('create-map-input')?.value || '56', 10);
        const freeFluid = Boolean(document.getElementById('create-peritoneal-fluid-checkbox')?.checked);
        const pupd = Boolean(document.getElementById('create-pupd-checkbox')?.checked);

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 38.2 : weightKg,
          cervix_type: cervixType,
          uterine_horn_diameter_cm: isNaN(hornDiameter) ? 6.1 : hornDiameter,
          blood_lactate_mmol_l: isNaN(lactate) ? 5.1 : lactate,
          wbc_count_k_ul: isNaN(wbc) ? 36.4 : wbc,
          band_neutrophil_percent: isNaN(bands) ? 20.0 : bands,
          map_mm_hg: isNaN(mapMmHg) ? 56 : mapMmHg,
          pocus_free_peritoneal_fluid: freeFluid,
          polyuria_polydipsia_present: pupd,
        };

        const btn = document.getElementById('create-pyometra-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/pyometra-sirs/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(`Admitted Pyometra patient ${created.pet_name} (${created.id})`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to admit Pyometra patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-ohe-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const crystalloid = (
          document.getElementById('stage-crystalloid-plan-input')?.value ||
          'Normosol-R 20 mL/kg IV shock bolus over 15 min targeting MAP ≥ 65 mmHg'
        ).trim();
        const empiricAbx = (
          document.getElementById('stage-empiric-abx-input')?.value ||
          'Enrofloxacin 10 mg/kg slow IV diluted q24h + Ampicillin-Sulbactam 30 mg/kg IV q8h'
        ).trim();
        const oheHandling = (
          document.getElementById('stage-ohe-handling-input')?.value ||
          'Atraumatic midline laparotomy OHE: pack off friable distended uterine horns with moistened laparotomy sponges and double-ligate cervical stump'
        ).trim();
        const lavage = (
          document.getElementById('stage-lavage-plan-input')?.value || ''
        ).trim();
        const analgesia = (
          document.getElementById('stage-analgesia-plan-input')?.value || ''
        ).trim();
        const sepsisMonitor = (
          document.getElementById('stage-sepsis-monitoring-input')?.value || ''
        ).trim();

        const payload = {
          crystalloid_resuscitation_plan: crystalloid,
          empiric_bactericidal_abx_plan: empiricAbx,
          surgical_ohe_handling_protocol: oheHandling,
          abdominal_lavage_plan: lavage,
          perioperative_analgesia_plan: analgesia,
          postop_sepsis_monitoring: sepsisMonitor,
        };

        const btn = document.getElementById('stage-ohe-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pyometra-sirs/operations/${encodeURIComponent(selectedOpId)}/stage-ohe`,
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
              `Staged IV Crystalloid Resuscitation, Empiric Abx & Emergent OHE for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage OHE stabilization protocol';
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

        const hornDiameter = parseFloat(
          document.getElementById('check-horn-diameter-input')?.value || '0.0'
        );
        const lactate = parseFloat(
          document.getElementById('check-lactate-input')?.value || '1.5'
        );
        const wbc = parseFloat(
          document.getElementById('check-wbc-input')?.value || '14.8'
        );
        const bands = parseFloat(
          document.getElementById('check-bands-input')?.value || '2.5'
        );
        const mapMmHg = parseInt(
          document.getElementById('check-map-input')?.value || '84',
          10
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Hannah Lin, DACVS'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          uterine_horn_diameter_cm: isNaN(hornDiameter) ? 0.0 : hornDiameter,
          blood_lactate_mmol_l: isNaN(lactate) ? 1.5 : lactate,
          wbc_count_k_ul: isNaN(wbc) ? 14.8 : wbc,
          band_neutrophil_percent: isNaN(bands) ? 2.5 : bands,
          map_mm_hg: isNaN(mapMmHg) ? 84 : mapMmHg,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-hemodynamic-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pyometra-sirs/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
            showToast(`Recorded perioperative hemodynamic & lactate check for ${updated.id}`, 'success');
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

    const clearForm = document.getElementById('clear-pyometra-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dischargedBy = (
          document.getElementById('clear-discharged-by-input')?.value ||
          'Dr. Hannah Lin, DACVS'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          discharged_by: dischargedBy,
          discharge_summary: summary,
        };

        const btn = document.getElementById('clear-pyometra-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pyometra-sirs/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Post-Op OHE Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to clear Pyometra operation for discharge';
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
    const gridEl = document.getElementById('pyometra-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.pyometra-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.pyometra-op-card');
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
