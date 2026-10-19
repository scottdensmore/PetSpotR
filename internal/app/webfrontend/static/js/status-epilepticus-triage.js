/**
 * PetSpotR Status Epilepticus, Cluster Seizures, Bromethalin/Metaldehyde Neuro-Tremor
 * & Levetiracetam/Propofol CRI Cockpit Controller (Milestone 19.5)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'SE-OP-101';

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
    const liveStatus = document.getElementById('epilepticus-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('epilepticus-error-banner');
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
      case 'ACTIVE_ICTAL_HYPERTHERMIA_CRITICAL':
        return 'status-pill-critical';
      case 'MIDAZOLAM_KEPPRA_METHOCARBAMOL_STAGED':
        return 'status-pill-staged';
      case 'SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-ictal');
    const stagedEl = document.getElementById('kpi-staged-cri');
    const clearedEl = document.getElementById('kpi-cleared-normothermic');
    const hyperthermiaEl = document.getElementById('kpi-hyperthermia-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.active_ictal_critical_count ??
      ops.filter((op) => op.status === 'ACTIVE_ICTAL_HYPERTHERMIA_CRITICAL').length;
    const staged =
      metrics?.anticonvulsant_cri_staged_count ??
      ops.filter((op) => op.status === 'MIDAZOLAM_KEPPRA_METHOCARBAMOL_STAGED').length;
    const cleared =
      metrics?.seizure_free_cleared_count ??
      ops.filter((op) => op.status === 'SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED').length;
    const hyperthermia =
      metrics?.hyperthermia_alert_count ??
      ops.filter((op) => Boolean(op.neurogenic_hyperthermia_alert)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (hyperthermiaEl) hyperthermiaEl.textContent = String(hyperthermia);
  }

  function syncStageFormWithOperation(op) {
    if (!op || !op.patient_profile) return;
    const p = op.patient_profile;
    const weightEl = document.getElementById('stage-weight-kg');
    const etiologyEl = document.getElementById('stage-neuro-etiology');
    const tempEl = document.getElementById('stage-rectal-temp');
    const mgcsEl = document.getElementById('stage-mgcs');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (etiologyEl && p.neuro_etiology) etiologyEl.value = p.neuro_etiology;
    if (tempEl && p.rectal_temp_fahrenheit !== undefined) {
      tempEl.value = String(p.rectal_temp_fahrenheit);
    }
    if (mgcsEl && p.modified_glasgow_coma_score !== undefined) {
      mgcsEl.value = String(p.modified_glasgow_coma_score);
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('epilepticus-alert-banner');
    const clearanceBanner = document.getElementById('epilepticus-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const reg = active.titration_regimen || {};
        const tempF = Number(p.rectal_temp_fahrenheit ?? 103.0).toFixed(1);
        const stopF = Number(reg.stop_cooling_threshold_temp_f ?? 103.0).toFixed(1);
        const pitfall =
          reg.neuroprotection_pitfall_alert ||
          "CRITICAL NEUROPROTECTION PITFALL ALERT: NEVER place fingers or objects into an ictal animal's mouth; enforce ZERO oral medication while gag reflex is absent; NEVER use oral diazepam in cats (hepatic necrosis); and STOP active cooling at 103.0°F.";
        alertBanner.textContent = `⚠️ NEUROGENIC HYPERTHERMIA & ICTAL AIRWAY GUARD (${active.id} — ${active.pet_name} · ${tempF}°F · Stop Cooling at ${stopF}°F): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.discharge_clearance) {
        const clr = active.discharge_clearance;
        const tempF = Number(clr.final_rectal_temp_fahrenheit ?? 101.2).toFixed(1);
        clearanceBanner.textContent = `✅ SEIZURE-FREE 24H NORMOTHERMIC DISCHARGE CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): ${clr.seizure_free_hours}h Seizure-Free · Final Temp ${tempF}°F · MGCS ${clr.final_modified_glasgow_score}/18 · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
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
        '<p class="trust-empty-hint">No active status epilepticus triage operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const reg = active.titration_regimen;
    const checks = Array.isArray(active.neuro_checks) ? active.neuro_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 20.0).toFixed(1);
    const etiology = escapeHtml(p.neuro_etiology || 'IDIOPATHIC_STATUS_EPILEPTICUS');
    const status = escapeHtml(active.status || 'ACTIVE_ICTAL_HYPERTHERMIA_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const ictalMins = Number(p.continuous_ictal_minutes ?? 0);
    const clusters = Number(p.cluster_count_24hr ?? 0);
    const tempF = Number(p.rectal_temp_fahrenheit ?? 101.5).toFixed(1);
    const bgMgDl = Number(p.blood_glucose_mg_dl ?? 90.0).toFixed(1);
    const mgcs = Number(p.modified_glasgow_coma_score ?? 15);

    const regimenHtml = reg
      ? `
        <div class="epilepticus-regimen-box" id="workspace-regimen-box">
          <h4>Calculated Anticonvulsant, Muscle Relaxant &amp; Osmotherapy Titration Regimen</h4>
          <div class="epilepticus-doses-grid">
            <div><strong>Midazolam (0.2 mg/kg IV/IN):</strong> <span id="regimen-midazolam-mg">${Number(reg.midazolam_first_line_bolus_mg ?? 0).toFixed(2)} mg</span></div>
            <div><strong>Levetiracetam Load (60 mg/kg IV):</strong> <span id="regimen-levetiracetam-mg">${Number(reg.levetiracetam_loading_60mg_kg_mg ?? 0).toFixed(1)} mg</span></div>
            <div><strong>Phenobarbital Load (16 mg/kg):</strong> <span id="regimen-phenobarbital-mg">${Number(reg.phenobarbital_loading_dose_mg ?? 0).toFixed(1)} mg</span></div>
            <div><strong>Methocarbamol (55 mg/kg IV):</strong> <span id="regimen-methocarbamol-mg">${Number(reg.methocarbamol_55mg_kg_dose_mg ?? 0).toFixed(1)} mg</span></div>
            <div><strong>Mannitol (0.5 g/kg IV):</strong> <span id="regimen-mannitol-g">${Number(reg.mannitol_05g_kg_cerebral_edema_grams ?? 0).toFixed(2)} g</span></div>
            <div><strong>Propofol CRI Rate:</strong> <span id="regimen-propofol-cri">${Number(reg.propofol_cri_rate_mg_kg_min ?? 0).toFixed(2)} mg/kg/min</span></div>
            <div><strong>Stop Active Cooling At:</strong> <span id="regimen-stop-cooling">${Number(reg.stop_cooling_threshold_temp_f ?? 103.0).toFixed(1)}°F</span></div>
            <div><strong>Zero Oral Gag Guard:</strong> <span id="regimen-oral-guard">${reg.zero_oral_gag_aspiration_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="epilepticus-rationale-text" id="workspace-clinical-rationale">${escapeHtml(reg.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-ICU');
        const cTemp = Number(chk.rectal_temp_fahrenheit ?? 101.5).toFixed(1);
        const cMgcs = Number(chk.modified_glasgow_coma_score ?? 15);
        const cPlr = escapeHtml(chk.pupillary_light_reflex || 'NORMAL_BILATERAL');
        const cBg = Number(chk.blood_glucose_mg_dl ?? 90.0).toFixed(1);
        const cIctal = chk.seizure_activity_observed ? 'ICTAL ACTIVITY' : 'SEIZURE-FREE';
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="epilepticus-check-item"><strong>${callsign}</strong> · Temp ${cTemp}°F · MGCS ${cMgcs}/18 · PLR ${cPlr} · BG ${cBg} mg/dL · ${cIctal} — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="epilepticus-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name" id="workspace-pet-title">Active Workspace: ${petName} (${species}${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id" id="workspace-op-code">${opId} · ${incId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="status-pill status-pill-etiology" id="workspace-etiology-pill">${etiology}</span>
            <span class="status-pill ${statusClass}" id="workspace-status-pill">${status}</span>
          </div>
        </div>

        <div class="epilepticus-vitals-summary" id="workspace-vitals-summary">
          <span><strong>Ictal Duration:</strong> ${ictalMins} mins</span>
          <span><strong>24h Clusters:</strong> ${clusters}</span>
          <span><strong>Rectal Temp:</strong> ${tempF}°F</span>
          <span><strong>Blood Glucose:</strong> ${bgMgDl} mg/dL</span>
          <span><strong>MGCS:</strong> ${mgcs}/18</span>
        </div>

        ${regimenHtml}

        <div class="epilepticus-neuro-checks-box">
          <h4>Serial Neuro ICU Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="neuro-checks-list" class="epilepticus-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const reg = op.titration_regimen || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 20.0).toFixed(1);
    const etiology = escapeHtml(p.neuro_etiology || 'IDIOPATHIC_STATUS_EPILEPTICUS');
    const status = escapeHtml(op.status || 'ACTIVE_ICTAL_HYPERTHERMIA_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const ictalMins = Number(p.continuous_ictal_minutes ?? 0);
    const clusters = Number(p.cluster_count_24hr ?? 0);
    const tempF = Number(p.rectal_temp_fahrenheit ?? 101.5).toFixed(1);
    const bgMgDl = Number(p.blood_glucose_mg_dl ?? 90.0).toFixed(1);
    const mgcs = Number(p.modified_glasgow_coma_score ?? 15);

    const alertHtml = op.neurogenic_hyperthermia_alert
      ? `
        <div class="epilepticus-alert-banner">
          ⚠️ NEUROGENIC HYPERTHERMIA &amp; ICTAL AIRWAY ALERT (${tempF}°F — Cease Active Cooling at 103.0°F): ${escapeHtml(reg.neuroprotection_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.discharge_clearance
      ? `
        <div class="epilepticus-clearance-banner">
          ✅ 24H SEIZURE-FREE NORMOTHERMIC CLEARANCE (${escapeHtml(op.discharge_clearance.vet_license || '')}): ${Number(op.discharge_clearance.seizure_free_hours ?? 24)}h Seizure-Free · ${Number(op.discharge_clearance.final_rectal_temp_fahrenheit ?? 101.2).toFixed(1)}°F · MGCS ${Number(op.discharge_clearance.final_modified_glasgow_score ?? 18)}/18
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card epilepticus-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-etiology="${etiology}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species}${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${incId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="status-pill status-pill-etiology">${etiology}</span>
            <span class="status-pill ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row epilepticus-metrics-row">
          <span class="status-pill status-pill-etiology">Ictal: ${ictalMins}m • Clusters: ${clusters} • Temp: ${tempF}°F • BG: ${bgMgDl} mg/dL • MGCS: ${mgcs}/18</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary epilepticus-select-op-btn"
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

    const countBadge = document.getElementById('epilepticus-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('epilepticus-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="epilepticus-ops-empty-msg">No status epilepticus triage operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const etiologyFilter = (document.getElementById('filter-etiology')?.value || '').trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);
      if (etiologyFilter) params.set('neuro_etiology', etiologyFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/status-epilepticus/operations?${qs}`
        : '/api/status-epilepticus/operations';

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
      const res = await fetch(`/api/status-epilepticus/operations/${encodeURIComponent(opId)}`, {
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
    const createForm = document.getElementById('create-epilepticus-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (document.getElementById('create-incident-id')?.value || 'INC-SE-9204').trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (document.getElementById('create-species')?.value || 'CANINE').trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const neuroEtiology = (
          document.getElementById('create-neuro-etiology')?.value || 'IDIOPATHIC_STATUS_EPILEPTICUS'
        ).trim();
        const ictalMins = parseInt(document.getElementById('create-ictal-mins')?.value || '0', 10);
        const clusterCount = parseInt(document.getElementById('create-cluster-count')?.value || '0', 10);
        const rectalTemp = parseFloat(document.getElementById('create-rectal-temp')?.value || '0');
        const bloodGlucose = parseFloat(document.getElementById('create-blood-glucose')?.value || '0');
        const mgcs = parseInt(document.getElementById('create-mgcs')?.value || '0', 10);

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            neuro_etiology: neuroEtiology,
            continuous_ictal_minutes: isNaN(ictalMins) ? 0 : ictalMins,
            cluster_count_24hr: isNaN(clusterCount) ? 0 : clusterCount,
            rectal_temp_fahrenheit: isNaN(rectalTemp) ? 0 : rectalTemp,
            blood_glucose_mg_dl: isNaN(bloodGlucose) ? 0 : bloodGlucose,
            modified_glasgow_coma_score: isNaN(mgcs) ? 0 : mgcs,
          },
        };

        const btn = document.getElementById('btn-create-epilepticus-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/status-epilepticus/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(`Created status epilepticus operation ${created.id} for ${created.pet_name}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create status epilepticus operation';
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
          species: 'CANINE',
          weight_kg: 24.0,
          neuro_etiology: 'IDIOPATHIC_STATUS_EPILEPTICUS',
          continuous_ictal_minutes: 10,
          cluster_count_24hr: 3,
          rectal_temp_fahrenheit: 105.0,
          blood_glucose_mg_dl: 80.0,
          modified_glasgow_coma_score: 11,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const neuroEtiology = (
          document.getElementById('stage-neuro-etiology')?.value || baseProfile.neuro_etiology
        ).trim();
        const rectalTemp = parseFloat(
          document.getElementById('stage-rectal-temp')?.value || String(baseProfile.rectal_temp_fahrenheit)
        );
        const mgcs = parseInt(
          document.getElementById('stage-mgcs')?.value || String(baseProfile.modified_glasgow_coma_score),
          10
        );

        const profilePayload = {
          ...baseProfile,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          neuro_etiology: neuroEtiology,
          rectal_temp_fahrenheit: isNaN(rectalTemp) ? baseProfile.rectal_temp_fahrenheit : rectalTemp,
          modified_glasgow_coma_score: isNaN(mgcs) ? baseProfile.modified_glasgow_coma_score : mgcs,
        };

        const btn = document.getElementById('btn-stage-regimen');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/status-epilepticus/operations/${encodeURIComponent(selectedOpId)}/stage-regimen`,
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
            showToast(`Staged titration regimen for ${updated.id} → ${updated.status}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage titration regimen';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('neuro-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const rectalTemp = parseFloat(document.getElementById('check-rectal-temp')?.value || '101.8');
        const mgcs = parseInt(document.getElementById('check-mgcs')?.value || '16', 10);
        const plr = (document.getElementById('check-plr')?.value || 'NORMAL_BILATERAL').trim();
        const bloodGlucose = parseFloat(document.getElementById('check-blood-glucose')?.value || '96.0');
        const callsign = (document.getElementById('check-callsign')?.value || 'DR-NEURO-41').trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();
        const seizureObserved = Boolean(document.getElementById('check-seizure-observed')?.checked);

        const checkPayload = {
          seizure_activity_observed: seizureObserved,
          rectal_temp_fahrenheit: isNaN(rectalTemp) ? 101.8 : rectalTemp,
          modified_glasgow_coma_score: isNaN(mgcs) ? 16 : mgcs,
          pupillary_light_reflex: plr,
          blood_glucose_mg_dl: isNaN(bloodGlucose) ? 96.0 : bloodGlucose,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-neuro-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/status-epilepticus/operations/${encodeURIComponent(selectedOpId)}/neuro-check`,
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
            showToast(`Recorded serial neuro ICU check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record serial neuro ICU check';
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

        const seizureFreeHours = parseInt(
          document.getElementById('clearance-seizure-free-hours')?.value || '26',
          10
        );
        const finalMgcs = parseInt(document.getElementById('clearance-final-mgcs')?.value || '18', 10);
        const finalTemp = parseFloat(document.getElementById('clearance-final-temp')?.value || '101.2');
        const vetLicense = (document.getElementById('clearance-vet-license')?.value || 'VET-NEURO-90411').trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const ambulatoryMenace = Boolean(document.getElementById('clearance-ambulatory-menace')?.checked);
        const rescueMidazolam = Boolean(document.getElementById('clearance-rescue-midazolam')?.checked);

        const clearancePayload = {
          seizure_free_hours: isNaN(seizureFreeHours) ? 26 : seizureFreeHours,
          final_modified_glasgow_score: isNaN(finalMgcs) ? 18 : finalMgcs,
          final_rectal_temp_fahrenheit: isNaN(finalTemp) ? 101.2 : finalTemp,
          ambulatory_menace_intact: ambulatoryMenace,
          rescue_intranasal_midazolam_dispensed: rescueMidazolam,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-discharge-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/status-epilepticus/operations/${encodeURIComponent(selectedOpId)}/discharge-clearance`,
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
            showToast(`Issued 24h seizure-free normothermic clearance for ${updated.id}`, 'success');
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

    const filterEtiology = document.getElementById('filter-etiology');
    if (filterEtiology) {
      filterEtiology.addEventListener('change', () => {
        fetchOperations();
      });
    }
  }

  function setupCardInteractions() {
    const listEl = document.getElementById('epilepticus-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.epilepticus-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.epilepticus-op-card');
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
