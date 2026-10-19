/**
 * PetSpotR Diabetic Ketoacidosis (DKA), Regular Insulin CRI, Hypokalemia/Hypophosphatemia
 * & Cerebral Edema Triage Cockpit Controller (Milestone 20.1)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'DKA-OP-101';

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
    const liveStatus = document.getElementById('dka-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('dka-error-banner');
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
      case 'KETOACIDOSIS_DEHYDRATED_CRITICAL':
        return 'status-pill-critical';
      case 'KCL_PHOS_REGULAR_INSULIN_CRI_STAGED':
        return 'status-pill-staged';
      case 'KETONES_CLEARED_SUBQ_GLARGINE_READY':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-acidosis');
    const stagedEl = document.getElementById('kpi-staged-insulin-cri');
    const clearedEl = document.getElementById('kpi-cleared-glargine');
    const hypokalemiaEl = document.getElementById('kpi-hypokalemia-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.critical_acidosis_count ??
      ops.filter((op) => op.status === 'KETOACIDOSIS_DEHYDRATED_CRITICAL').length;
    const staged =
      metrics?.insulin_cri_staged_count ??
      ops.filter((op) => op.status === 'KCL_PHOS_REGULAR_INSULIN_CRI_STAGED').length;
    const cleared =
      metrics?.ketone_cleared_count ??
      ops.filter((op) => op.status === 'KETONES_CLEARED_SUBQ_GLARGINE_READY').length;
    const alerts =
      metrics?.hypokalemia_edema_alert_count ??
      ops.filter((op) => Boolean(op.hypokalemia_cerebral_edema_alert)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (hypokalemiaEl) hypokalemiaEl.textContent = String(alerts);
  }

  function syncStageFormWithOperation(op) {
    if (!op || !op.patient_profile) return;
    const p = op.patient_profile;
    const weightEl = document.getElementById('stage-weight-kg');
    const speciesEl = document.getElementById('stage-species');
    const bgEl = document.getElementById('stage-blood-glucose');
    const kEl = document.getElementById('stage-potassium');
    const phosEl = document.getElementById('stage-phosphorus');
    const dehydrationEl = document.getElementById('stage-dehydration');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (bgEl && p.blood_glucose_mg_dl !== undefined) bgEl.value = String(p.blood_glucose_mg_dl);
    if (kEl && p.serum_potassium_meq_l !== undefined) kEl.value = String(p.serum_potassium_meq_l);
    if (phosEl && p.serum_phosphorus_mg_dl !== undefined) {
      phosEl.value = String(p.serum_phosphorus_mg_dl);
    }
    if (dehydrationEl && p.dehydration_percent !== undefined) {
      dehydrationEl.value = String(p.dehydration_percent);
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('dka-alert-banner');
    const clearanceBanner = document.getElementById('dka-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const reg = active.insulin_regimen || {};
        const bg = Number(p.blood_glucose_mg_dl ?? 485).toFixed(0);
        const k = Number(p.serum_potassium_meq_l ?? 2.8).toFixed(1);
        const phos = Number(p.serum_phosphorus_mg_dl ?? 1.6).toFixed(1);
        const maxDrop = Number(reg.max_glucose_drop_rate_mg_dl_hr ?? 50).toFixed(0);
        const pitfall =
          reg.dka_pitfall_alert ||
          'CRITICAL DKA & CEREBRAL EDEMA PITFALL ALERT: NEVER initiate Regular Insulin CRI before 4–6h of isotonic fluid rehydration and potassium correction (K+ >= 3.3 mEq/L); NEVER drop blood glucose > 50–75 mg/dL/hr (rapid osmotic shift triggers fatal cerebral edema); and treat hypophosphatemia (< 1.5 mg/dL) to prevent Heinz body hemolytic anemia.';
        alertBanner.textContent = `⚠️ HYPOKALEMIA, HYPOPHOSPHATEMIA & CEREBRAL EDEMA GUARD (${active.id} — ${active.pet_name} · BG ${bg} mg/dL · K+ ${k} mEq/L · Phos ${phos} mg/dL · Max Drop <= ${maxDrop} mg/dL/hr): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.discharge_clearance) {
        const clr = active.discharge_clearance;
        const bhb = Number(clr.final_beta_hydroxybutyrate_mmol_l ?? 0.4).toFixed(2);
        const ph = Number(clr.final_venous_ph ?? 7.39).toFixed(2);
        const k = Number(clr.final_serum_potassium_meq_l ?? 4.2).toFixed(1);
        const phos = Number(clr.final_serum_phosphorus_mg_dl ?? 3.8).toFixed(1);
        clearanceBanner.textContent = `✅ KETONES CLEARED & SUBQ GLARGINE DISCHARGE CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): BHB ${bhb} mmol/L · pH ${ph} · K+ ${k} mEq/L · Phos ${phos} mg/dL · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
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
        '<p class="trust-empty-hint">No active DKA &amp; Regular Insulin CRI operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const reg = active.insulin_regimen;
    const checks = Array.isArray(active.serial_checks) ? active.serial_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'FELINE_DKA_HEPATIC_LIPIDOSIS');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 5.4).toFixed(1);
    const status = escapeHtml(active.status || 'KETOACIDOSIS_DEHYDRATED_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const bg = Number(p.blood_glucose_mg_dl ?? 485).toFixed(0);
    const bhb = Number(p.beta_hydroxybutyrate_mmol_l ?? 5.8).toFixed(1);
    const ph = Number(p.venous_ph ?? 7.12).toFixed(2);
    const k = Number(p.serum_potassium_meq_l ?? 2.8).toFixed(1);
    const phos = Number(p.serum_phosphorus_mg_dl ?? 1.6).toFixed(1);
    const osmolality = Number(p.effective_osmolality_mosm_kg ?? 328).toFixed(0);
    const dehydration = Number(p.dehydration_percent ?? 8.0).toFixed(1);

    const regimenHtml = reg
      ? `
        <div class="dka-regimen-box" id="workspace-regimen-box">
          <h4>Calculated Sliding-Scale KCl, Potassium Phosphate CRI &amp; Regular Insulin CRI Regimen</h4>
          <div class="dka-doses-grid">
            <div><strong>Isotonic Rehydration Delay:</strong> <span id="regimen-rehydration-delay">${Number(reg.fluid_rehydration_delay_hours ?? 6)}h Delay</span></div>
            <div><strong>Sliding-Scale KCl:</strong> <span id="regimen-kcl-meq">${Number(reg.potassium_chloride_meq_per_liter ?? 40)} mEq/L</span></div>
            <div><strong>Potassium Phosphate CRI:</strong> <span id="regimen-kphos-cri">${Number(reg.potassium_phosphate_mmol_kg_hr ?? 0).toFixed(2)} mmol/kg/hr</span></div>
            <div><strong>Regular Insulin CRI:</strong> <span id="regimen-insulin-cri">${Number(reg.regular_insulin_cri_u_kg_day ?? 1.1).toFixed(1)} U/kg/day</span></div>
            <div><strong>Dextrose Supplementation:</strong> <span id="regimen-dextrose-pct">${Number(reg.dextrose_supplementation_percent ?? 0).toFixed(1)}%</span></div>
            <div><strong>Max BG Drop Rate:</strong> <span id="regimen-max-bg-drop">&le; ${Number(reg.max_glucose_drop_rate_mg_dl_hr ?? 50).toFixed(0)} mg/dL/hr</span></div>
            <div><strong>Hold Insulin if K+ &lt; 3.3:</strong> <span id="regimen-k-guard">${reg.hold_insulin_if_potassium_below_33_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="dka-rationale-text" id="workspace-clinical-rationale">${escapeHtml(reg.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-ENDO');
        const cBg = Number(chk.blood_glucose_mg_dl ?? 200).toFixed(0);
        const cBhb = Number(chk.beta_hydroxybutyrate_mmol_l ?? 1.5).toFixed(1);
        const cK = Number(chk.serum_potassium_meq_l ?? 3.8).toFixed(1);
        const cPhos = Number(chk.serum_phosphorus_mg_dl ?? 3.0).toFixed(1);
        const cDex = Number(chk.dextrose_percent_active ?? 2.5).toFixed(1);
        const cMentation = chk.mentation_normal_no_cerebral_edema
          ? 'MENTATION NORMAL'
          : 'CEREBRAL EDEMA SUSPECTED';
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="dka-check-item"><strong>${callsign}</strong> · BG ${cBg} mg/dL · BHB ${cBhb} mmol/L · K+ ${cK} mEq/L · Phos ${cPhos} mg/dL · Dextrose ${cDex}% · ${cMentation} — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="dka-workspace-card" data-workspace-op-id="${opId}">
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

        <div class="dka-vitals-summary" id="workspace-vitals-summary">
          <span><strong>BG:</strong> ${bg} mg/dL</span>
          <span><strong>BHB Ketones:</strong> ${bhb} mmol/L</span>
          <span><strong>Venous pH:</strong> ${ph}</span>
          <span><strong>Serum K+:</strong> ${k} mEq/L</span>
          <span><strong>Phosphorus:</strong> ${phos} mg/dL</span>
          <span><strong>Osmolality:</strong> ${osmolality} mOsm/kg</span>
          <span><strong>Dehydration:</strong> ${dehydration}%</span>
        </div>

        ${regimenHtml}

        <div class="dka-serial-checks-box">
          <h4>Serial q2h Glucose, Ketone &amp; Electrolyte Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="serial-checks-list" class="dka-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const reg = op.insulin_regimen || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'FELINE_DKA_HEPATIC_LIPIDOSIS');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 5.4).toFixed(1);
    const status = escapeHtml(op.status || 'KETOACIDOSIS_DEHYDRATED_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const bg = Number(p.blood_glucose_mg_dl ?? 485).toFixed(0);
    const bhb = Number(p.beta_hydroxybutyrate_mmol_l ?? 5.8).toFixed(1);
    const ph = Number(p.venous_ph ?? 7.12).toFixed(2);
    const k = Number(p.serum_potassium_meq_l ?? 2.8).toFixed(1);
    const phos = Number(p.serum_phosphorus_mg_dl ?? 1.6).toFixed(1);

    const alertHtml = op.hypokalemia_cerebral_edema_alert
      ? `
        <div class="dka-alert-banner">
          ⚠️ HYPOKALEMIA / HYPOPHOSPHATEMIA &amp; CEREBRAL EDEMA ALERT (K+ ${k} mEq/L · Phos ${phos} mg/dL · Max Drop &le; 50 mg/dL/hr): ${escapeHtml(reg.dka_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.discharge_clearance
      ? `
        <div class="dka-clearance-banner">
          ✅ KETONES CLEARED &amp; SUBQ GLARGINE READY (${escapeHtml(op.discharge_clearance.vet_license || '')}): BHB ${Number(op.discharge_clearance.final_beta_hydroxybutyrate_mmol_l ?? 0.4).toFixed(2)} mmol/L · pH ${Number(op.discharge_clearance.final_venous_ph ?? 7.39).toFixed(2)} · K+ ${Number(op.discharge_clearance.final_serum_potassium_meq_l ?? 4.2).toFixed(1)} mEq/L
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card dka-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
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

        <div class="trust-kinematics-row dka-metrics-row">
          <span class="status-pill status-pill-species">BG: ${bg} mg/dL • BHB: ${bhb} mmol/L • pH: ${ph} • K+: ${k} mEq/L • Phos: ${phos} mg/dL</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary dka-select-op-btn"
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

    const countBadge = document.getElementById('dka-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('dka-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="dka-ops-empty-msg">No DKA &amp; Regular Insulin CRI operations match the current filters.</p>';
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
        ? `/api/dka-insulin/operations?${qs}`
        : '/api/dka-insulin/operations';

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
      const res = await fetch(`/api/dka-insulin/operations/${encodeURIComponent(opId)}`, {
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
    const createForm = document.getElementById('create-dka-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (document.getElementById('create-incident-id')?.value || 'INC-DKA-9304').trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_DKA_PANCREATITIS'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const bloodGlucose = parseFloat(document.getElementById('create-blood-glucose')?.value || '0');
        const bhb = parseFloat(document.getElementById('create-bhb')?.value || '0');
        const venousPh = parseFloat(document.getElementById('create-venous-ph')?.value || '0');
        const potassium = parseFloat(document.getElementById('create-potassium')?.value || '0');
        const phosphorus = parseFloat(document.getElementById('create-phosphorus')?.value || '0');
        const osmolality = parseFloat(document.getElementById('create-osmolality')?.value || '0');
        const dehydration = parseFloat(document.getElementById('create-dehydration')?.value || '0');

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            blood_glucose_mg_dl: isNaN(bloodGlucose) ? 0 : bloodGlucose,
            beta_hydroxybutyrate_mmol_l: isNaN(bhb) ? 0 : bhb,
            venous_ph: isNaN(venousPh) ? 0 : venousPh,
            serum_potassium_meq_l: isNaN(potassium) ? 0 : potassium,
            serum_phosphorus_mg_dl: isNaN(phosphorus) ? 0 : phosphorus,
            effective_osmolality_mosm_kg: isNaN(osmolality) ? 0 : osmolality,
            dehydration_percent: isNaN(dehydration) ? 0 : dehydration,
          },
        };

        const btn = document.getElementById('btn-create-dka-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/dka-insulin/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(`Created DKA operation ${created.id} for ${created.pet_name}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create DKA operation';
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
          species: 'FELINE_DKA_HEPATIC_LIPIDOSIS',
          weight_kg: 5.4,
          blood_glucose_mg_dl: 485.0,
          beta_hydroxybutyrate_mmol_l: 5.8,
          venous_ph: 7.12,
          serum_potassium_meq_l: 2.8,
          serum_phosphorus_mg_dl: 1.6,
          effective_osmolality_mosm_kg: 328.0,
          dehydration_percent: 8.0,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const species = (
          document.getElementById('stage-species')?.value || baseProfile.species
        ).trim();
        const bloodGlucose = parseFloat(
          document.getElementById('stage-blood-glucose')?.value || String(baseProfile.blood_glucose_mg_dl)
        );
        const potassium = parseFloat(
          document.getElementById('stage-potassium')?.value || String(baseProfile.serum_potassium_meq_l)
        );
        const phosphorus = parseFloat(
          document.getElementById('stage-phosphorus')?.value || String(baseProfile.serum_phosphorus_mg_dl)
        );
        const dehydration = parseFloat(
          document.getElementById('stage-dehydration')?.value || String(baseProfile.dehydration_percent)
        );

        const profilePayload = {
          ...baseProfile,
          species,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          blood_glucose_mg_dl: isNaN(bloodGlucose) ? baseProfile.blood_glucose_mg_dl : bloodGlucose,
          serum_potassium_meq_l: isNaN(potassium) ? baseProfile.serum_potassium_meq_l : potassium,
          serum_phosphorus_mg_dl: isNaN(phosphorus) ? baseProfile.serum_phosphorus_mg_dl : phosphorus,
          dehydration_percent: isNaN(dehydration) ? baseProfile.dehydration_percent : dehydration,
        };

        const btn = document.getElementById('btn-stage-regimen');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/dka-insulin/operations/${encodeURIComponent(selectedOpId)}/stage-regimen`,
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
            showToast(`Staged KCl / KPhos / Regular Insulin CRI regimen for ${updated.id} → ${updated.status}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage insulin regimen';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('serial-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const bloodGlucose = parseFloat(document.getElementById('check-blood-glucose')?.value || '210.0');
        const bhb = parseFloat(document.getElementById('check-bhb')?.value || '1.8');
        const potassium = parseFloat(document.getElementById('check-potassium')?.value || '3.9');
        const phosphorus = parseFloat(document.getElementById('check-phosphorus')?.value || '3.1');
        const dextrosePercent = parseFloat(document.getElementById('check-dextrose-percent')?.value || '2.5');
        const callsign = (document.getElementById('check-callsign')?.value || 'DR-ENDO-14').trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();
        const mentationNormal = Boolean(document.getElementById('check-mentation-normal')?.checked);

        const checkPayload = {
          blood_glucose_mg_dl: isNaN(bloodGlucose) ? 210.0 : bloodGlucose,
          beta_hydroxybutyrate_mmol_l: isNaN(bhb) ? 1.8 : bhb,
          serum_potassium_meq_l: isNaN(potassium) ? 3.9 : potassium,
          serum_phosphorus_mg_dl: isNaN(phosphorus) ? 3.1 : phosphorus,
          dextrose_percent_active: isNaN(dextrosePercent) ? 2.5 : dextrosePercent,
          mentation_normal_no_cerebral_edema: mentationNormal,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-serial-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/dka-insulin/operations/${encodeURIComponent(selectedOpId)}/serial-check`,
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
            showToast(`Recorded q2h glucose & electrolyte check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record serial check';
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

        const finalBhb = parseFloat(document.getElementById('clearance-final-bhb')?.value || '0.4');
        const finalPh = parseFloat(document.getElementById('clearance-final-ph')?.value || '7.39');
        const finalPotassium = parseFloat(
          document.getElementById('clearance-final-potassium')?.value || '4.2'
        );
        const finalPhosphorus = parseFloat(
          document.getElementById('clearance-final-phosphorus')?.value || '3.8'
        );
        const vetLicense = (document.getElementById('clearance-vet-license')?.value || 'VET-ENDO-91042').trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const voluntaryEating = Boolean(document.getElementById('clearance-voluntary-eating')?.checked);
        const subqTransition = Boolean(document.getElementById('clearance-subq-transition')?.checked);

        const clearancePayload = {
          final_beta_hydroxybutyrate_mmol_l: isNaN(finalBhb) ? 0.4 : finalBhb,
          final_venous_ph: isNaN(finalPh) ? 7.39 : finalPh,
          final_serum_potassium_meq_l: isNaN(finalPotassium) ? 4.2 : finalPotassium,
          final_serum_phosphorus_mg_dl: isNaN(finalPhosphorus) ? 3.8 : finalPhosphorus,
          voluntary_eating_no_vomiting: voluntaryEating,
          subq_insulin_transition_completed: subqTransition,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-discharge-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/dka-insulin/operations/${encodeURIComponent(selectedOpId)}/discharge-clearance`,
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
            showToast(`Issued ketone-cleared SubQ Glargine clearance for ${updated.id}`, 'success');
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
    const listEl = document.getElementById('dka-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.dka-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.dka-op-card');
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
