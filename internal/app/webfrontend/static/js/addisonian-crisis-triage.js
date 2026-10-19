/**
 * PetSpotR Hypoadrenocorticism (Addisonian Crisis), Na:K Ratio < 27,
 * Dexamethasone-SP & DOCP Triage Cockpit Controller (Milestone 20.4)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'AC-OP-101';

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
    const liveStatus = document.getElementById('addisonian-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('addisonian-error-banner');
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
      case 'ADDISONIAN_SHOCK_HYPERKALEMIA_CRITICAL':
        return 'status-pill-critical';
      case 'DEX_SP_NACL_ACTH_STIM_STAGED':
        return 'status-pill-staged';
      case 'DOCP_PREDNISONE_EUKALEMIC_CLEARED':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-shock');
    const stagedEl = document.getElementById('kpi-staged-dex-sp');
    const clearedEl = document.getElementById('kpi-cleared-docp');
    const myelinolysisEl = document.getElementById('kpi-myelinolysis-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.critical_shock_count ??
      ops.filter((op) => op.status === 'ADDISONIAN_SHOCK_HYPERKALEMIA_CRITICAL').length;
    const staged =
      metrics?.dex_sp_acth_staged_count ??
      ops.filter((op) => op.status === 'DEX_SP_NACL_ACTH_STIM_STAGED').length;
    const cleared =
      metrics?.docp_eukalemic_cleared_count ??
      ops.filter((op) => op.status === 'DOCP_PREDNISONE_EUKALEMIC_CLEARED').length;
    const alerts =
      metrics?.myelinolysis_assay_alert_count ??
      ops.filter((op) => Boolean(op.osmotic_myelinolysis_assay_alert)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (myelinolysisEl) myelinolysisEl.textContent = String(alerts);
  }

  function syncStageFormWithOperation(op) {
    if (!op || !op.patient_profile) return;
    const p = op.patient_profile;
    const weightEl = document.getElementById('stage-weight-kg');
    const speciesEl = document.getElementById('stage-species');
    const naEl = document.getElementById('stage-sodium');
    const kEl = document.getElementById('stage-potassium');
    const hrEl = document.getElementById('stage-heart-rate');
    const sbpEl = document.getElementById('stage-systolic-bp');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (naEl && p.serum_sodium_meq_l !== undefined) naEl.value = String(p.serum_sodium_meq_l);
    if (kEl && p.serum_potassium_meq_l !== undefined) kEl.value = String(p.serum_potassium_meq_l);
    if (hrEl && p.heart_rate_bpm !== undefined) hrEl.value = String(p.heart_rate_bpm);
    if (sbpEl && p.systolic_blood_pressure_mmhg !== undefined) {
      sbpEl.value = String(p.systolic_blood_pressure_mmhg);
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('addisonian-alert-banner');
    const clearanceBanner = document.getElementById('addisonian-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const reg = active.resuscitation_regimen || {};
        const na = Number(p.serum_sodium_meq_l ?? 124).toFixed(1);
        const k = Number(p.serum_potassium_meq_l ?? 7.8).toFixed(1);
        const ratio = Number(reg.sodium_potassium_ratio ?? 15.9).toFixed(1);
        const maxRise = Number(reg.max_sodium_rise_rate_meq_l_day ?? 10.0).toFixed(1);
        const pitfall =
          reg.addisonian_pitfall_alert ||
          'CRITICAL ADDISONIAN CRISIS & OSMOTIC DEMYELINATION PITFALL ALERT: NEVER administer Prednisone, Prednisolone, or Hydrocortisone before completing the ACTH stimulation test (they cross-react with the cortisol immunoassay and invalidate diagnosis — administer Dexamethasone-SP 0.25 mg/kg IV instead); and NEVER raise serum Sodium > 10–12 mEq/L/24h in chronic hyponatremia (< 125 mEq/L), which causes fatal osmotic demyelination syndrome / central pontine myelinolysis.';
        alertBanner.textContent = `⚠️ ADDISONIAN CRISIS, CORTISOL ASSAY & OSMOTIC DEMYELINATION GUARD (${active.id} — ${active.pet_name} · Na:K ${ratio} · Na+ ${na} mEq/L · K+ ${k} mEq/L · Max Na+ Rise <= ${maxRise} mEq/L/day): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.discharge_clearance) {
        const clr = active.discharge_clearance;
        const ratio = Number(clr.final_sodium_potassium_ratio ?? 32.0).toFixed(1);
        const k = Number(clr.final_serum_potassium_meq_l ?? 4.5).toFixed(1);
        clearanceBanner.textContent = `✅ DOCP & ORAL PREDNISONE EUKALEMIC DISCHARGE CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): Na:K Ratio ${ratio} · K+ ${k} mEq/L · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
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
        '<p class="trust-empty-hint">No active Addisonian Crisis &amp; DOCP operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const reg = active.resuscitation_regimen;
    const checks = Array.isArray(active.electrolyte_checks) ? active.electrolyte_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_TYPICAL_ADDISONS');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 23.5).toFixed(1);
    const status = escapeHtml(active.status || 'ADDISONIAN_SHOCK_HYPERKALEMIA_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const na = Number(p.serum_sodium_meq_l ?? 124.0).toFixed(1);
    const k = Number(p.serum_potassium_meq_l ?? 7.8).toFixed(1);
    const bg = Number(p.blood_glucose_mg_dl ?? 54.0).toFixed(0);
    const hr = Number(p.heart_rate_bpm ?? 52);
    const sbp = Number(p.systolic_blood_pressure_mmhg ?? 68);
    const cortisol = Number(p.resting_cortisol_mcg_dl ?? 0.2).toFixed(1);

    const regimenHtml = reg
      ? `
        <div class="addisonian-regimen-box" id="workspace-regimen-box">
          <h4>Calculated Na:K Ratio, 0.9% NaCl Shock Bolus, Dexamethasone-SP &amp; Cosyntropin ACTH Stim Regimen</h4>
          <div class="addisonian-doses-grid">
            <div><strong>Na:K Ratio:</strong> <span id="regimen-nak-ratio">${Number(reg.sodium_potassium_ratio ?? 15.9).toFixed(1)}</span></div>
            <div><strong>0.9% NaCl Bolus (20 mL/kg):</strong> <span id="regimen-nacl-bolus">${Number(reg.isotonic_nacl_09_bolus_ml ?? 470.0).toFixed(1)} mL</span></div>
            <div><strong>Dexamethasone-SP (0.25 mg/kg):</strong> <span id="regimen-dex-sp">${Number(reg.dexamethasone_sp_025_mg_kg_mg ?? 5.88).toFixed(2)} mg IV</span></div>
            <div><strong>Cosyntropin ACTH Stim:</strong> <span id="regimen-cosyntropin">${Number(reg.cosyntropin_acth_stim_dose_mcg ?? 117.5).toFixed(1)} mcg IV</span></div>
            <div><strong>10% Calcium Gluconate:</strong> <span id="regimen-ca-gluconate">${Number(reg.calcium_gluconate_10_pct_ml ?? 11.75).toFixed(2)} mL IV</span></div>
            <div><strong>Max Na+ Rise Rate:</strong> <span id="regimen-max-na-rise">&le; ${Number(reg.max_sodium_rise_rate_meq_l_day ?? 10.0).toFixed(1)} mEq/L/day</span></div>
            <div><strong>Zero Prednisolone Pre-ACTH Guard:</strong> <span id="regimen-pred-guard">${reg.zero_prednisolone_pre_acth_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="addisonian-rationale-text" id="workspace-clinical-rationale">${escapeHtml(reg.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-ENDO');
        const cNa = Number(chk.serum_sodium_meq_l ?? 131.0).toFixed(1);
        const cK = Number(chk.serum_potassium_meq_l ?? 5.2).toFixed(1);
        const cRise = Number(chk.sodium_rise_24hr_meq_l ?? 7.0).toFixed(1);
        const cCortisol = Number(chk.post_acth_cortisol_mcg_dl ?? 0.3).toFixed(1);
        const cHr = Number(chk.heart_rate_bpm ?? 88);
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="addisonian-check-item"><strong>${callsign}</strong> · Na+ ${cNa} mEq/L · K+ ${cK} mEq/L · 24h Na+ Rise ${cRise} mEq/L · Post-ACTH Cortisol ${cCortisol} mcg/dL · HR ${cHr} bpm — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="addisonian-workspace-card" data-workspace-op-id="${opId}">
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

        <div class="addisonian-vitals-summary" id="workspace-vitals-summary">
          <span><strong>Serum Na+:</strong> ${na} mEq/L</span>
          <span><strong>Serum K+:</strong> ${k} mEq/L</span>
          <span><strong>BG:</strong> ${bg} mg/dL</span>
          <span><strong>HR:</strong> ${hr} bpm</span>
          <span><strong>Systolic BP:</strong> ${sbp} mmHg</span>
          <span><strong>Resting Cortisol:</strong> ${cortisol} mcg/dL</span>
        </div>

        ${regimenHtml}

        <div class="addisonian-electrolyte-checks-box">
          <h4>Serial Electrolyte, 24h Na+ Rise &amp; Post-ACTH Cortisol Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="electrolyte-checks-list" class="addisonian-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const reg = op.resuscitation_regimen || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_TYPICAL_ADDISONS');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 23.5).toFixed(1);
    const status = escapeHtml(op.status || 'ADDISONIAN_SHOCK_HYPERKALEMIA_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const na = Number(p.serum_sodium_meq_l ?? 124.0).toFixed(1);
    const k = Number(p.serum_potassium_meq_l ?? 7.8).toFixed(1);
    const ratio = Number(reg.sodium_potassium_ratio ?? 15.9).toFixed(1);
    const hr = Number(p.heart_rate_bpm ?? 52);
    const sbp = Number(p.systolic_blood_pressure_mmhg ?? 68);

    const alertHtml = op.osmotic_myelinolysis_assay_alert
      ? `
        <div class="addisonian-alert-banner">
          ⚠️ OSMOTIC DEMYELINATION &amp; CORTISOL ASSAY GUARD (Na+ ${na} mEq/L · K+ ${k} mEq/L · Max Na+ Rise &le; 10.0 mEq/L/day): ${escapeHtml(reg.addisonian_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.discharge_clearance
      ? `
        <div class="addisonian-clearance-banner">
          ✅ DOCP &amp; PREDNISONE EUKALEMIC CLEARED (${escapeHtml(op.discharge_clearance.vet_license || '')}): Na:K ${Number(op.discharge_clearance.final_sodium_potassium_ratio ?? 32.0).toFixed(1)} · K+ ${Number(op.discharge_clearance.final_serum_potassium_meq_l ?? 4.5).toFixed(1)} mEq/L
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card addisonian-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
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

        <div class="trust-kinematics-row addisonian-metrics-row">
          <span class="status-pill status-pill-species">Na+: ${na} mEq/L • K+: ${k} mEq/L • Na:K: ${ratio} • HR: ${hr} bpm • SBP: ${sbp} mmHg</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary addisonian-select-op-btn"
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

    const countBadge = document.getElementById('addisonian-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('addisonian-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="addisonian-ops-empty-msg">No Addisonian Crisis &amp; DOCP operations match the current filters.</p>';
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
        ? `/api/addisonian-crisis/operations?${qs}`
        : '/api/addisonian-crisis/operations';

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
      const res = await fetch(`/api/addisonian-crisis/operations/${encodeURIComponent(opId)}`, {
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
    const createForm = document.getElementById('create-addisonian-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (document.getElementById('create-incident-id')?.value || 'INC-AC-9604').trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_TYPICAL_ADDISONS'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const sodium = parseFloat(document.getElementById('create-sodium')?.value || '0');
        const potassium = parseFloat(document.getElementById('create-potassium')?.value || '0');
        const bloodGlucose = parseFloat(document.getElementById('create-blood-glucose')?.value || '0');
        const heartRate = parseInt(document.getElementById('create-heart-rate')?.value || '0', 10);
        const systolicBp = parseInt(document.getElementById('create-systolic-bp')?.value || '0', 10);
        const restingCortisol = parseFloat(document.getElementById('create-resting-cortisol')?.value || '0');

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            serum_sodium_meq_l: isNaN(sodium) ? 0 : sodium,
            serum_potassium_meq_l: isNaN(potassium) ? 0 : potassium,
            blood_glucose_mg_dl: isNaN(bloodGlucose) ? 0 : bloodGlucose,
            heart_rate_bpm: isNaN(heartRate) ? 0 : heartRate,
            systolic_blood_pressure_mmhg: isNaN(systolicBp) ? 0 : systolicBp,
            resting_cortisol_mcg_dl: isNaN(restingCortisol) ? 0 : restingCortisol,
          },
        };

        const btn = document.getElementById('btn-create-addisonian-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/addisonian-crisis/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(`Created Addisonian crisis operation ${created.id} for ${created.pet_name}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create Addisonian crisis operation';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-regimen-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const active = currentOps.find((item) => item.id === selectedOpId) || currentOps[0];
        const baseProfile = active?.patient_profile || {
          species: 'CANINE_TYPICAL_ADDISONS',
          weight_kg: 23.5,
          serum_sodium_meq_l: 124.0,
          serum_potassium_meq_l: 7.8,
          blood_glucose_mg_dl: 54.0,
          heart_rate_bpm: 52,
          systolic_blood_pressure_mmhg: 68,
          resting_cortisol_mcg_dl: 0.2,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const species = (
          document.getElementById('stage-species')?.value || baseProfile.species
        ).trim();
        const sodium = parseFloat(
          document.getElementById('stage-sodium')?.value || String(baseProfile.serum_sodium_meq_l)
        );
        const potassium = parseFloat(
          document.getElementById('stage-potassium')?.value || String(baseProfile.serum_potassium_meq_l)
        );
        const heartRate = parseInt(
          document.getElementById('stage-heart-rate')?.value || String(baseProfile.heart_rate_bpm),
          10
        );
        const systolicBp = parseInt(
          document.getElementById('stage-systolic-bp')?.value ||
            String(baseProfile.systolic_blood_pressure_mmhg),
          10
        );

        const profilePayload = {
          ...baseProfile,
          species,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          serum_sodium_meq_l: isNaN(sodium) ? baseProfile.serum_sodium_meq_l : sodium,
          serum_potassium_meq_l: isNaN(potassium) ? baseProfile.serum_potassium_meq_l : potassium,
          heart_rate_bpm: isNaN(heartRate) ? baseProfile.heart_rate_bpm : heartRate,
          systolic_blood_pressure_mmhg: isNaN(systolicBp)
            ? baseProfile.systolic_blood_pressure_mmhg
            : systolicBp,
        };

        const btn = document.getElementById('btn-stage-regimen');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/addisonian-crisis/operations/${encodeURIComponent(selectedOpId)}/stage-regimen`,
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
              `Staged 0.9% NaCl / Dexamethasone-SP / ACTH Stim regimen for ${updated.id} → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage resuscitation regimen';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('electrolyte-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const sodium = parseFloat(document.getElementById('check-sodium')?.value || '131.0');
        const potassium = parseFloat(document.getElementById('check-potassium')?.value || '5.2');
        const sodiumRise = parseFloat(document.getElementById('check-sodium-rise')?.value || '7.0');
        const postActhCortisol = parseFloat(
          document.getElementById('check-post-acth-cortisol')?.value || '0.3'
        );
        const heartRate = parseInt(document.getElementById('check-heart-rate')?.value || '88', 10);
        const callsign = (document.getElementById('check-callsign')?.value || 'DR-ENDO-19').trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();

        const checkPayload = {
          serum_sodium_meq_l: isNaN(sodium) ? 131.0 : sodium,
          serum_potassium_meq_l: isNaN(potassium) ? 5.2 : potassium,
          sodium_rise_24hr_meq_l: isNaN(sodiumRise) ? 7.0 : sodiumRise,
          post_acth_cortisol_mcg_dl: isNaN(postActhCortisol) ? 0.3 : postActhCortisol,
          heart_rate_bpm: isNaN(heartRate) ? 88 : heartRate,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-electrolyte-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/addisonian-crisis/operations/${encodeURIComponent(selectedOpId)}/electrolyte-check`,
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
            showToast(`Recorded electrolyte & post-ACTH cortisol check for ${updated.id}`, 'success');
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

    const clearanceForm = document.getElementById('discharge-clearance-form');
    if (clearanceForm) {
      clearanceForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const finalRatio = parseFloat(
          document.getElementById('clearance-final-ratio')?.value || '31.5'
        );
        const finalPotassium = parseFloat(
          document.getElementById('clearance-final-potassium')?.value || '4.4'
        );
        const vetLicense = (document.getElementById('clearance-vet-license')?.value || 'VET-ENDO-96108').trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const cortisolConfirmed = Boolean(
          document.getElementById('clearance-cortisol-confirmed')?.checked
        );
        const docpAdministered = Boolean(
          document.getElementById('clearance-docp-administered')?.checked
        );
        const prednisoneStarted = Boolean(
          document.getElementById('clearance-prednisone-started')?.checked
        );

        const clearancePayload = {
          final_sodium_potassium_ratio: isNaN(finalRatio) ? 31.5 : finalRatio,
          final_serum_potassium_meq_l: isNaN(finalPotassium) ? 4.4 : finalPotassium,
          post_acth_cortisol_confirmed_low: cortisolConfirmed,
          docp_22_mg_kg_subq_administered: docpAdministered,
          oral_prednisone_physiologic_started: prednisoneStarted,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-discharge-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/addisonian-crisis/operations/${encodeURIComponent(selectedOpId)}/discharge-clearance`,
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
            showToast(`Issued DOCP & eukalemic discharge clearance for ${updated.id}`, 'success');
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
    const listEl = document.getElementById('addisonian-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.addisonian-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.addisonian-op-card');
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
