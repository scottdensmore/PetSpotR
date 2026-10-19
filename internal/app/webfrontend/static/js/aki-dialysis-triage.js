/**
 * PetSpotR Acute Kidney Injury (AKI), Lily/Ethylene Glycol/Grape Nephrotoxicity,
 * Oliguria/Anuria & Furosemide/Diltiazem/Dialysis Triage Cockpit Controller (Milestone 21.5)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'AKI-OP-101';

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
    const liveStatus = document.getElementById('aki-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('aki-error-banner');
    if (!banner) return;
    if (!message) {
      banner.textContent = '';
      banner.hidden = true;
      return;
    }
    banner.textContent = message;
    banner.hidden = false;
  }

  function getStatusPillClass(status) {
    switch (status) {
      case 'OLIGURIC_ANURIC_AZOTEMIA_CRITICAL':
        return 'status-pill-critical';
      case 'FOMEPIZOLE_INS_OUTS_DIURESIS_STAGED':
        return 'status-pill-staged';
      case 'POLYURIC_CREATININE_PLATEAU_CLEARED':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-oliguric');
    const stagedEl = document.getElementById('kpi-staged-fomepizole');
    const clearedEl = document.getElementById('kpi-cleared-polyuric');
    const alertsEl = document.getElementById('kpi-anuric-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.oliguric_azotemia_critical_count ??
      ops.filter((op) => op.status === 'OLIGURIC_ANURIC_AZOTEMIA_CRITICAL').length;
    const staged =
      metrics?.fomepizole_diuresis_staged_count ??
      ops.filter((op) => op.status === 'FOMEPIZOLE_INS_OUTS_DIURESIS_STAGED').length;
    const cleared =
      metrics?.polyuric_cleared_count ??
      ops.filter((op) => op.status === 'POLYURIC_CREATININE_PLATEAU_CLEARED').length;
    const alerts =
      metrics?.anuric_overload_alert_count ??
      ops.filter((op) => Boolean(op.anuric_fluid_overload_fomepizole_alert)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (alertsEl) alertsEl.textContent = String(alerts);
  }

  function syncStageFormWithOperation(op) {
    if (!op || !op.patient_profile) return;
    const p = op.patient_profile;
    const weightEl = document.getElementById('stage-weight-kg');
    const speciesEl = document.getElementById('stage-species');
    const irisEl = document.getElementById('stage-iris-grade');
    const uopEl = document.getElementById('stage-uop');
    const crEl = document.getElementById('stage-creatinine');
    const kEl = document.getElementById('stage-potassium');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (irisEl && p.iris_grade !== undefined) irisEl.value = String(p.iris_grade);
    if (uopEl && p.urine_output_ml_kg_hr !== undefined) {
      uopEl.value = String(p.urine_output_ml_kg_hr);
    }
    if (crEl && p.serum_creatinine_mg_dl !== undefined) {
      crEl.value = String(p.serum_creatinine_mg_dl);
    }
    if (kEl && p.serum_potassium_meq_l !== undefined) {
      kEl.value = String(p.serum_potassium_meq_l);
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('aki-alert-banner');
    const clearanceBanner = document.getElementById('aki-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const plan = active.diuresis_plan || {};
        const iris = Number(p.iris_grade ?? 5);
        const uop = Number(p.urine_output_ml_kg_hr ?? 0.2).toFixed(1);
        const cr = Number(p.serum_creatinine_mg_dl ?? 11.4).toFixed(1);
        const k = Number(p.serum_potassium_meq_l ?? 7.4).toFixed(1);
        const wtGain = Number(p.body_weight_gain_percent ?? 9.5).toFixed(1);
        const pitfall =
          plan.aki_pitfall_alert ||
          "CRITICAL AKI, ANURIC FLUID OVERLOAD & FOMEPIZOLE PITFALL ALERT: NEVER force high-rate IV crystalloid fluids into an oliguric/anuric (< 0.5 mL/kg/hr) patient — switch strictly to 'Ins-and-Outs' fluid matching (urine output + 20 mL/kg/day insensible loss); NEVER use canine-dose Fomepizole (20 mg/kg) in cats (cats require 125 mg/kg IV high-dose Fomepizole within 3 hours); and NEVER delay intermittent hemodialysis when oliguria + K+ > 6.5 mEq/L persists.";
        alertBanner.textContent = `⚠️ AKI, ANURIC FLUID OVERLOAD & FOMEPIZOLE GUARD (${active.id} — ${active.pet_name} · IRIS Grade ${iris} · UOP ${uop} mL/kg/hr · Cr ${cr} mg/dL · K+ ${k} mEq/L · Weight Gain ${wtGain}%): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.discharge_clearance) {
        const clr = active.discharge_clearance;
        const finalUop = Number(clr.final_urine_output_ml_kg_hr ?? 2.4).toFixed(1);
        const finalCr = Number(clr.final_serum_creatinine_mg_dl ?? 1.8).toFixed(1);
        const finalK = Number(clr.final_serum_potassium_meq_l ?? 4.2).toFixed(1);
        clearanceBanner.textContent = `✅ POLYURIC RECOVERY & CREATININE PLATEAU CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): Final UOP ${finalUop} mL/kg/hr · Final Cr ${finalCr} mg/dL · Final K+ ${finalK} mEq/L · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
        clearanceBanner.hidden = false;
      } else {
        clearanceBanner.hidden = true;
      }
    }
  }

  function renderActiveWorkspace(ops) {
    const workspaceEl = document.getElementById('active-operation-workspace');
    if (!workspaceEl) return;

    if (!ops || ops.length === 0) {
      workspaceEl.innerHTML =
        '<p class="trust-empty-hint">No active AKI &amp; Dialysis operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const plan = active.diuresis_plan;
    const checks = Array.isArray(active.renal_checks) ? active.renal_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'FELINE_LILIUM_NEPHROTOXICITY');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 4.5).toFixed(1);
    const status = escapeHtml(active.status || 'OLIGURIC_ANURIC_AZOTEMIA_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const iris = Number(p.iris_grade ?? 5);
    const uop = Number(p.urine_output_ml_kg_hr ?? 0.2).toFixed(1);
    const cr = Number(p.serum_creatinine_mg_dl ?? 11.4).toFixed(1);
    const k = Number(p.serum_potassium_meq_l ?? 7.4).toFixed(1);
    const wtGain = Number(p.body_weight_gain_percent ?? 9.5).toFixed(1);
    const crystalsText = p.calcium_oxalate_monohydrate_crystals ? 'YES' : 'NO';

    const planHtml = plan
      ? `
        <div class="aki-plan-box" id="workspace-plan-box">
          <h4>Calculated Fomepizole (4-MP), Ins-and-Outs Fluid Rate, Furosemide/Diltiazem &amp; Hemodialysis Plan</h4>
          <div class="aki-doses-grid">
            <div><strong>Fomepizole Loading Dose:</strong> <span id="plan-fomepizole">${Number(plan.fomepizole_loading_dose_mg ?? 562.5).toFixed(1)} mg IV</span></div>
            <div><strong>Ins-and-Outs Fluid Rate:</strong> <span id="plan-ins-outs-rate">${Number(plan.ins_and_outs_fluid_rate_ml_hr ?? 4.65).toFixed(2)} mL/hr</span></div>
            <div><strong>Furosemide Conversion Bolus:</strong> <span id="plan-furosemide-bolus">${Number(plan.furosemide_conversion_bolus_mg ?? 9.0).toFixed(1)} mg IV</span></div>
            <div><strong>Diltiazem CRI:</strong> <span id="plan-diltiazem-cri">${Number(plan.diltiazem_cri_03_mcg_kg_min ?? 0.3).toFixed(1)} mcg/kg/min</span></div>
            <div><strong>Intermittent Hemodialysis:</strong> <span id="plan-hemodialysis">${plan.hemodialysis_indicated ? 'INDICATED' : 'STANDBY'}</span></div>
            <div><strong>Zero-Force-Fluid-in-Anuria Guard:</strong> <span id="plan-anuria-fluid-guard">${plan.zero_force_fluid_in_anuria_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="aki-rationale-text" id="workspace-clinical-rationale">${escapeHtml(plan.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-RENAL');
        const cUop = Number(chk.urine_output_ml_kg_hr ?? 2.4).toFixed(1);
        const cCr = Number(chk.serum_creatinine_mg_dl ?? 1.9).toFixed(1);
        const cK = Number(chk.serum_potassium_meq_l ?? 4.2).toFixed(1);
        const cWt = Number(chk.body_weight_kg ?? 4.5).toFixed(1);
        const cBal = Number(chk.fluid_in_out_balance_ml ?? 0.0).toFixed(1);
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="aki-check-item"><strong>${callsign}</strong> · UOP ${cUop} mL/kg/hr · Cr ${cCr} mg/dL · K+ ${cK} mEq/L · Wt ${cWt} kg · Fluid Bal ${cBal} mL — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="aki-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name" id="workspace-pet-title">Active Workspace: ${petName} (${species}${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id" id="workspace-op-code">${opId} · ${incId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="status-pill status-pill-species" id="workspace-species-pill">${species}</span>
            <span class="status-pill ${statusClass}" id="workspace-status-pill">${status}</span>
          </div>
        </div>

        <div class="aki-vitals-summary" id="workspace-vitals-summary">
          <span><strong>IRIS Grade:</strong> ${iris}</span>
          <span><strong>UOP:</strong> ${uop} mL/kg/hr</span>
          <span><strong>Creatinine:</strong> ${cr} mg/dL</span>
          <span><strong>Potassium K+:</strong> ${k} mEq/L</span>
          <span><strong>Weight Gain:</strong> ${wtGain}%</span>
          <span><strong>Oxalate Crystals:</strong> ${crystalsText}</span>
        </div>

        ${planHtml}

        <div class="aki-renal-checks-box">
          <h4>Serial Closed-System UOP, Creatinine, Potassium &amp; Fluid Balance Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="renal-checks-list" class="aki-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const plan = op.diuresis_plan || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'FELINE_LILIUM_NEPHROTOXICITY');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 4.5).toFixed(1);
    const status = escapeHtml(op.status || 'OLIGURIC_ANURIC_AZOTEMIA_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const iris = Number(p.iris_grade ?? 5);
    const uop = Number(p.urine_output_ml_kg_hr ?? 0.2).toFixed(1);
    const cr = Number(p.serum_creatinine_mg_dl ?? 11.4).toFixed(1);
    const k = Number(p.serum_potassium_meq_l ?? 7.4).toFixed(1);
    const wtGain = Number(p.body_weight_gain_percent ?? 9.5).toFixed(1);
    const crystalsYesNo = p.calcium_oxalate_monohydrate_crystals ? 'YES' : 'NO';

    const alertHtml = op.anuric_fluid_overload_fomepizole_alert
      ? `
        <div class="aki-alert-banner">
          ⚠️ ANURIC FLUID OVERLOAD &amp; FOMEPIZOLE GUARD (UOP ${uop} mL/kg/hr · Cr ${cr} mg/dL · K+ ${k} mEq/L): ${escapeHtml(plan.aki_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.discharge_clearance
      ? `
        <div class="aki-clearance-banner">
          ✅ POLYURIC &amp; CREATININE PLATEAU CLEARED (${escapeHtml(op.discharge_clearance.vet_license || '')}): Final UOP ${Number(op.discharge_clearance.final_urine_output_ml_kg_hr ?? 2.8).toFixed(1)} mL/kg/hr · Final Cr ${Number(op.discharge_clearance.final_serum_creatinine_mg_dl ?? 1.4).toFixed(1)} mg/dL · Final K+ ${Number(op.discharge_clearance.final_serum_potassium_meq_l ?? 4.3).toFixed(1)} mEq/L
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card aki-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species}${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${incId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="status-pill status-pill-species">${species}</span>
            <span class="status-pill ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row aki-metrics-row">
          <span class="status-pill status-pill-species">IRIS Grade: ${iris} • UOP: ${uop} mL/kg/hr • Cr: ${cr} mg/dL • K+: ${k} mEq/L • Weight Gain: ${wtGain}% • Oxalate Crystals: ${crystalsYesNo}</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary aki-select-op-btn"
            data-op-id="${opId}"
          >
            🔍 Select Operation (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateSummaryStats(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('aki-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('aki-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="aki-ops-empty-msg">No AKI &amp; Dialysis operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const speciesFilter = (document.getElementById('filter-species')?.value || '').trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);
      if (speciesFilter) params.set('species', speciesFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/aki-dialysis/operations?${qs}`
        : '/api/aki-dialysis/operations';

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
      const res = await fetch(`/api/aki-dialysis/operations/${encodeURIComponent(opId)}`, {
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

  function setupFormsAndFilters() {
    const createForm = document.getElementById('create-aki-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (
          document.getElementById('create-incident-id')?.value || 'INC-AKI-9934'
        ).trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_ETHYLENE_GLYCOL_OXALATE'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const irisGrade = parseInt(document.getElementById('create-iris-grade')?.value || '0', 10);
        const crMgDl = parseFloat(document.getElementById('create-creatinine')?.value || '0');
        const kMeqL = parseFloat(document.getElementById('create-potassium')?.value || '0');
        const uopMlKgHr = parseFloat(document.getElementById('create-uop')?.value || '0');
        const wtGainPct = parseFloat(
          document.getElementById('create-weight-gain-pct')?.value || '0'
        );
        const oxalateCrystals = Boolean(
          document.getElementById('create-oxalate-crystals')?.checked
        );

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            iris_grade: isNaN(irisGrade) ? 0 : irisGrade,
            serum_creatinine_mg_dl: isNaN(crMgDl) ? 0 : crMgDl,
            serum_potassium_meq_l: isNaN(kMeqL) ? 0 : kMeqL,
            urine_output_ml_kg_hr: isNaN(uopMlKgHr) ? 0 : uopMlKgHr,
            body_weight_gain_percent: isNaN(wtGainPct) ? 0 : wtGainPct,
            calcium_oxalate_monohydrate_crystals: oxalateCrystals,
          },
        };

        const btn = document.getElementById('btn-create-aki-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/aki-dialysis/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Created AKI Dialysis operation ${created.id} for ${created.pet_name}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create AKI Dialysis operation';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-plan-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const active = currentOps.find((item) => item.id === selectedOpId) || currentOps[0];
        const baseProfile = active?.patient_profile || {
          species: 'FELINE_LILIUM_NEPHROTOXICITY',
          weight_kg: 4.5,
          iris_grade: 5,
          serum_creatinine_mg_dl: 11.4,
          serum_potassium_meq_l: 7.4,
          urine_output_ml_kg_hr: 0.2,
          body_weight_gain_percent: 9.5,
          calcium_oxalate_monohydrate_crystals: false,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const species = (
          document.getElementById('stage-species')?.value || baseProfile.species
        ).trim();
        const irisGrade = parseInt(
          document.getElementById('stage-iris-grade')?.value || String(baseProfile.iris_grade),
          10
        );
        const uopMlKgHr = parseFloat(
          document.getElementById('stage-uop')?.value ||
            String(baseProfile.urine_output_ml_kg_hr)
        );
        const crMgDl = parseFloat(
          document.getElementById('stage-creatinine')?.value ||
            String(baseProfile.serum_creatinine_mg_dl)
        );
        const kMeqL = parseFloat(
          document.getElementById('stage-potassium')?.value ||
            String(baseProfile.serum_potassium_meq_l)
        );

        const profilePayload = {
          ...baseProfile,
          species,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          iris_grade: isNaN(irisGrade) ? baseProfile.iris_grade : irisGrade,
          urine_output_ml_kg_hr: isNaN(uopMlKgHr)
            ? baseProfile.urine_output_ml_kg_hr
            : uopMlKgHr,
          serum_creatinine_mg_dl: isNaN(crMgDl)
            ? baseProfile.serum_creatinine_mg_dl
            : crMgDl,
          serum_potassium_meq_l: isNaN(kMeqL) ? baseProfile.serum_potassium_meq_l : kMeqL,
        };

        const btn = document.getElementById('btn-stage-plan');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/aki-dialysis/operations/${encodeURIComponent(selectedOpId)}/stage-plan`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(profilePayload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Staged Fomepizole, Ins-and-Outs & Dialysis plan for ${updated.id} → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage diuresis & dialysis plan';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('renal-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const uop = parseFloat(document.getElementById('check-uop')?.value || '2.4');
        const cr = parseFloat(document.getElementById('check-creatinine')?.value || '1.9');
        const k = parseFloat(document.getElementById('check-potassium')?.value || '4.2');
        const wt = parseFloat(document.getElementById('check-body-weight')?.value || '4.5');
        const bal = parseFloat(document.getElementById('check-fluid-balance')?.value || '0.0');
        const callsign = (
          document.getElementById('check-callsign')?.value || 'DR-RENAL-11'
        ).trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();

        const checkPayload = {
          urine_output_ml_kg_hr: isNaN(uop) ? 2.4 : uop,
          serum_creatinine_mg_dl: isNaN(cr) ? 1.9 : cr,
          serum_potassium_meq_l: isNaN(k) ? 4.2 : k,
          body_weight_kg: isNaN(wt) ? 4.5 : wt,
          fluid_in_out_balance_ml: isNaN(bal) ? 0.0 : bal,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-renal-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/aki-dialysis/operations/${encodeURIComponent(selectedOpId)}/renal-check`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(checkPayload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(`Recorded closed-system UOP & renal telemetry check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record renal telemetry check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearanceForm = document.getElementById('discharge-clearance-form');
    if (clearanceForm) {
      clearanceForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const finalUop = parseFloat(
          document.getElementById('clearance-final-uop')?.value || '2.4'
        );
        const finalCr = parseFloat(
          document.getElementById('clearance-final-creatinine')?.value || '1.8'
        );
        const finalK = parseFloat(
          document.getElementById('clearance-final-potassium')?.value || '4.2'
        );
        const vetLicense = (
          document.getElementById('clearance-vet-license')?.value || 'VET-RENAL-99308'
        ).trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const euvolemic = Boolean(document.getElementById('clearance-euvolemic')?.checked);
        const subqEducated = Boolean(
          document.getElementById('clearance-subq-educated')?.checked
        );

        const clearancePayload = {
          final_urine_output_ml_kg_hr: isNaN(finalUop) ? 2.4 : finalUop,
          final_serum_creatinine_mg_dl: isNaN(finalCr) ? 1.8 : finalCr,
          final_serum_potassium_meq_l: isNaN(finalK) ? 4.2 : finalK,
          euvolemic_no_pulmonary_edema: euvolemic,
          outpatient_subq_fluids_educated: subqEducated,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-discharge-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/aki-dialysis/operations/${encodeURIComponent(selectedOpId)}/discharge-clearance`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(clearancePayload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Issued polyuric recovery & creatinine plateau clearance for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to issue discharge clearance';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const filterStatus = document.getElementById('filter-status');
    if (filterStatus) {
      filterStatus.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const filterSpecies = document.getElementById('filter-species');
    if (filterSpecies) {
      filterSpecies.addEventListener('change', () => {
        fetchOperations();
      });
    }
  }

  function setupCardInteractions() {
    const listEl = document.getElementById('aki-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.aki-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.aki-op-card');
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
