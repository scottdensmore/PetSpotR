/**
 * PetSpotR Pericardial Effusion, Cardiac Tamponade, Electrical Alternans
 * & Right-Sided 4th–5th ICS Pericardiocentesis Cockpit Controller (Milestone 21.4)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'PT-OP-101';

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
    const liveStatus = document.getElementById('tamponade-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('tamponade-error-banner');
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
      case 'TAMPONADE_RA_COLLAPSE_ALTERNANS_CRITICAL':
        return 'status-pill-critical';
      case 'RIGHT_ICS_4_5_PERICARDIOCENTESIS_STAGED':
        return 'status-pill-staged';
      case 'DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-tamponade');
    const stagedEl = document.getElementById('kpi-staged-centesis');
    const clearedEl = document.getElementById('kpi-cleared-decompressed');
    const hazardEl = document.getElementById('kpi-furosemide-coronary-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.critical_tamponade_count ??
      ops.filter((op) => op.status === 'TAMPONADE_RA_COLLAPSE_ALTERNANS_CRITICAL').length;
    const staged =
      metrics?.right_ics_4_5_tap_staged_count ??
      ops.filter((op) => op.status === 'RIGHT_ICS_4_5_PERICARDIOCENTESIS_STAGED').length;
    const cleared =
      metrics?.decompressed_sinus_cleared_count ??
      ops.filter((op) => op.status === 'DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED').length;
    const alerts =
      metrics?.furosemide_left_coronary_hazard_alerts ??
      ops.filter((op) => Boolean(op.furosemide_left_coronary_hazard_alert)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (hazardEl) hazardEl.textContent = String(alerts);
  }

  function syncStageFormWithOperation(op) {
    if (!op || !op.patient_profile) return;
    const p = op.patient_profile;
    const weightEl = document.getElementById('stage-weight-kg');
    const speciesEl = document.getElementById('stage-species');
    const hrEl = document.getElementById('stage-heart-rate');
    const mapEl = document.getElementById('stage-map');
    const pulsusEl = document.getElementById('stage-pulsus-paradoxus');
    const pcvEl = document.getElementById('stage-effusion-pcv');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (hrEl && p.heart_rate_bpm !== undefined) hrEl.value = String(p.heart_rate_bpm);
    if (mapEl && p.mean_arterial_pressure_mmhg !== undefined) {
      mapEl.value = String(p.mean_arterial_pressure_mmhg);
    }
    if (pulsusEl && p.pulsus_paradoxus_mmhg !== undefined) {
      pulsusEl.value = String(p.pulsus_paradoxus_mmhg);
    }
    if (pcvEl && p.effusion_pcv_percent !== undefined) {
      pcvEl.value = String(p.effusion_pcv_percent);
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('tamponade-alert-banner');
    const clearanceBanner = document.getElementById('tamponade-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const plan = active.centesis_plan || {};
        const approach = plan.approach_side_and_ics || 'Right 4th-5th ICS Cardiac Notch';
        const hr = Number(p.heart_rate_bpm ?? 185);
        const map = Number(p.mean_arterial_pressure_mmhg ?? 52);
        const pulsus = Number(p.pulsus_paradoxus_mmhg ?? 18);
        const pitfall =
          plan.tamponade_pitfall_alert ||
          'CRITICAL PERICARDIAL TAMPONADE, FUROSEMIDE PRELOAD & LEFT CORONARY ARTERY PITFALL ALERT: NEVER administer Furosemide or diuretics in cardiac tamponade (diuretics deplete right ventricular preload and precipitate immediate fatal cardiovascular collapse); NEVER perform left-sided pericardiocentesis (which risks lacerating the left paraconal interventricular coronary artery — always use the RIGHT 4th–5th ICS cardiac notch); and ALWAYS test the first aspirated aliquot in a red-top clot tube to confirm non-clotting defibrinated fluid and rule out iatrogenic intracardiac ventricular puncture.';
        alertBanner.textContent = `⚠️ PERICARDIAL TAMPONADE, FUROSEMIDE PRELOAD & LEFT CORONARY ARTERY GUARD (${active.id} — ${active.pet_name} · Approach ${approach} · HR ${hr} bpm · MAP ${map} mmHg · Pulsus Paradoxus ${pulsus} mmHg): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.discharge_clearance) {
        const clr = active.discharge_clearance;
        const finalMap = Number(clr.final_map_mmhg ?? 96);
        clearanceBanner.textContent = `✅ PERICARDIOCENTESIS DECOMPRESSION & SINUS RHYTHM DISCHARGE CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): Final MAP ${finalMap} mmHg · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
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
        '<p class="trust-empty-hint">No active Pericardial Tamponade &amp; Pericardiocentesis operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const plan = active.centesis_plan;
    const checks = Array.isArray(active.tap_checks) ? active.tap_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 33.0).toFixed(1);
    const status = escapeHtml(active.status || 'TAMPONADE_RA_COLLAPSE_ALTERNANS_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const hr = Number(p.heart_rate_bpm ?? 185);
    const map = Number(p.mean_arterial_pressure_mmhg ?? 52);
    const raCollapse = p.right_atrial_diastolic_collapse ? 'YES' : 'NO';
    const alternans = p.electrical_alternans_present ? 'YES' : 'NO';
    const pulsus = Number(p.pulsus_paradoxus_mmhg ?? 18);
    const pcv = Number(p.effusion_pcv_percent ?? 28.0).toFixed(1);

    const planHtml = plan
      ? `
        <div class="tamponade-plan-box" id="workspace-plan-box">
          <h4>Calculated Right 4th–5th ICS Pericardiocentesis, Lidocaine &amp; Preload Expansion Plan</h4>
          <div class="tamponade-doses-grid">
            <div><strong>Approach Side &amp; ICS:</strong> <span id="plan-approach-ics">${escapeHtml(plan.approach_side_and_ics || 'Right 4th-5th ICS Cardiac Notch')}</span></div>
            <div><strong>Anatomic Landmark:</strong> <span id="plan-landmark">${escapeHtml(plan.costochondral_junction_landmark || 'Cranial to 5th Rib at Costochondral Junction')}</span></div>
            <div><strong>2% Lidocaine Block:</strong> <span id="plan-lidocaine-infilt">${Number(plan.lidocaine_2_pct_infiltration_ml ?? 3.3).toFixed(2)} mL</span></div>
            <div><strong>IV Lidocaine (2 mg/kg):</strong> <span id="plan-lidocaine-iv">${Number(plan.lidocaine_iv_2_mg_kg_arrhythmia_mg ?? 66.0).toFixed(1)} mg IV</span></div>
            <div><strong>Crystalloid Preload (10 mL/kg):</strong> <span id="plan-crystalloid-bolus">${Number(plan.crystalloid_preload_expansion_ml ?? 330.0).toFixed(1)} mL IV</span></div>
            <div><strong>Clot-Tube Non-Clotting Check:</strong> <span id="plan-clot-tube-guard">${plan.clot_tube_iatrogenic_puncture_check ? 'REQUIRED' : 'OFF'}</span></div>
            <div><strong>Zero Furosemide Diuretic Guard:</strong> <span id="plan-furosemide-guard">${plan.zero_furosemide_diuretic_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="tamponade-rationale-text" id="workspace-clinical-rationale">${escapeHtml(plan.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-CARDIO');
        const cVol = Number(chk.volume_aspirated_ml ?? 260.0).toFixed(1);
        const cClot = chk.sample_non_clotting_verified ? 'YES' : 'NO';
        const cMap = Number(chk.post_tap_map_mmhg ?? 88);
        const cHr = Number(chk.post_tap_heart_rate_bpm ?? 108);
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="tamponade-check-item"><strong>${callsign}</strong> · Aspirated ${cVol} mL · Non-Clotting Verified: ${cClot} · Post-Tap MAP ${cMap} mmHg · Post-Tap HR ${cHr} bpm — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="tamponade-workspace-card" data-workspace-op-id="${opId}">
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

        <div class="tamponade-vitals-summary" id="workspace-vitals-summary">
          <span><strong>HR:</strong> ${hr} bpm</span>
          <span><strong>MAP:</strong> ${map} mmHg</span>
          <span><strong>RA Diastolic Collapse:</strong> ${raCollapse}</span>
          <span><strong>Electrical Alternans:</strong> ${alternans}</span>
          <span><strong>Pulsus Paradoxus:</strong> ${pulsus} mmHg</span>
          <span><strong>Effusion PCV:</strong> ${pcv}%</span>
        </div>

        ${planHtml}

        <div class="tamponade-tap-checks-box">
          <h4>Pericardiocentesis Tap &amp; Red-Top Clot Tube Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="tap-checks-list" class="tamponade-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const plan = op.centesis_plan || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 33.0).toFixed(1);
    const status = escapeHtml(op.status || 'TAMPONADE_RA_COLLAPSE_ALTERNANS_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const hr = Number(p.heart_rate_bpm ?? 185);
    const map = Number(p.mean_arterial_pressure_mmhg ?? 52);
    const raCollapse = p.right_atrial_diastolic_collapse ? 'YES' : 'NO';
    const alternans = p.electrical_alternans_present ? 'YES' : 'NO';
    const pcv = Number(p.effusion_pcv_percent ?? 28.0).toFixed(1);

    const alertHtml = op.furosemide_left_coronary_hazard_alert
      ? `
        <div class="tamponade-alert-banner">
          ⚠️ FUROSEMIDE PRELOAD &amp; LEFT CORONARY ARTERY GUARD (Approach Right 4th-5th ICS Cardiac Notch · HR ${hr} bpm · MAP ${map} mmHg): ${escapeHtml(plan.tamponade_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.discharge_clearance
      ? `
        <div class="tamponade-clearance-banner">
          ✅ DECOMPRESSED &amp; SINUS CLEARED (${escapeHtml(op.discharge_clearance.vet_license || '')}): Final MAP ${Number(op.discharge_clearance.final_map_mmhg ?? 96)} mmHg · 24h TFAST Clear
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card tamponade-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
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

        <div class="trust-kinematics-row tamponade-metrics-row">
          <span class="status-pill status-pill-species">HR: ${hr} bpm • MAP: ${map} mmHg • RA Collapse: ${raCollapse} • Alternans: ${alternans} • Effusion PCV: ${pcv}%</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary tamponade-select-op-btn"
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

    const countBadge = document.getElementById('tamponade-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('tamponade-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="tamponade-ops-empty-msg">No Pericardial Tamponade &amp; Pericardiocentesis operations match the current filters.</p>';
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
        ? `/api/pericardial-tamponade/operations?${qs}`
        : '/api/pericardial-tamponade/operations';

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
      const res = await fetch(`/api/pericardial-tamponade/operations/${encodeURIComponent(opId)}`, {
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
    const createForm = document.getElementById('create-tamponade-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (document.getElementById('create-incident-id')?.value || 'INC-PT-9924').trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const heartRate = parseInt(document.getElementById('create-heart-rate')?.value || '0', 10);
        const mapMmHg = parseInt(document.getElementById('create-map')?.value || '0', 10);
        const pulsusParadoxus = parseInt(
          document.getElementById('create-pulsus-paradoxus')?.value || '0',
          10
        );
        const effusionPcv = parseFloat(document.getElementById('create-effusion-pcv')?.value || '0');
        const raCollapse = Boolean(document.getElementById('create-ra-collapse')?.checked);
        const electricalAlternans = Boolean(
          document.getElementById('create-electrical-alternans')?.checked
        );

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            heart_rate_bpm: isNaN(heartRate) ? 0 : heartRate,
            mean_arterial_pressure_mmhg: isNaN(mapMmHg) ? 0 : mapMmHg,
            right_atrial_diastolic_collapse: raCollapse,
            electrical_alternans_present: electricalAlternans,
            pulsus_paradoxus_mmhg: isNaN(pulsusParadoxus) ? 0 : pulsusParadoxus,
            effusion_pcv_percent: isNaN(effusionPcv) ? 0 : effusionPcv,
          },
        };

        const btn = document.getElementById('btn-create-tamponade-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/pericardial-tamponade/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Created pericardial tamponade operation ${created.id} for ${created.pet_name}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create pericardial tamponade operation';
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
          species: 'CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA',
          weight_kg: 33.0,
          heart_rate_bpm: 185,
          mean_arterial_pressure_mmhg: 52,
          right_atrial_diastolic_collapse: true,
          electrical_alternans_present: true,
          pulsus_paradoxus_mmhg: 18,
          effusion_pcv_percent: 28.0,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const species = (
          document.getElementById('stage-species')?.value || baseProfile.species
        ).trim();
        const heartRate = parseInt(
          document.getElementById('stage-heart-rate')?.value || String(baseProfile.heart_rate_bpm),
          10
        );
        const mapMmHg = parseInt(
          document.getElementById('stage-map')?.value ||
            String(baseProfile.mean_arterial_pressure_mmhg),
          10
        );
        const pulsusParadoxus = parseInt(
          document.getElementById('stage-pulsus-paradoxus')?.value ||
            String(baseProfile.pulsus_paradoxus_mmhg),
          10
        );
        const effusionPcv = parseFloat(
          document.getElementById('stage-effusion-pcv')?.value ||
            String(baseProfile.effusion_pcv_percent)
        );

        const profilePayload = {
          ...baseProfile,
          species,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          heart_rate_bpm: isNaN(heartRate) ? baseProfile.heart_rate_bpm : heartRate,
          mean_arterial_pressure_mmhg: isNaN(mapMmHg)
            ? baseProfile.mean_arterial_pressure_mmhg
            : mapMmHg,
          pulsus_paradoxus_mmhg: isNaN(pulsusParadoxus)
            ? baseProfile.pulsus_paradoxus_mmhg
            : pulsusParadoxus,
          effusion_pcv_percent: isNaN(effusionPcv)
            ? baseProfile.effusion_pcv_percent
            : effusionPcv,
        };

        const btn = document.getElementById('btn-stage-plan');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pericardial-tamponade/operations/${encodeURIComponent(selectedOpId)}/stage-plan`,
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
              `Staged Right 4th-5th ICS pericardiocentesis plan for ${updated.id} → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage pericardiocentesis plan';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('tap-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const volumeAspirated = parseFloat(
          document.getElementById('check-volume-aspirated')?.value || '260.0'
        );
        const postTapMap = parseInt(
          document.getElementById('check-post-tap-map')?.value || '88',
          10
        );
        const postTapHr = parseInt(
          document.getElementById('check-post-tap-hr')?.value || '108',
          10
        );
        const callsign = (document.getElementById('check-callsign')?.value || 'DR-CARDIO-11').trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();
        const nonClottingVerified = Boolean(
          document.getElementById('check-non-clotting-verified')?.checked
        );
        const ventricularArrhythmias = Boolean(
          document.getElementById('check-ventricular-arrhythmias')?.checked
        );

        const checkPayload = {
          volume_aspirated_ml: isNaN(volumeAspirated) ? 260.0 : volumeAspirated,
          sample_non_clotting_verified: nonClottingVerified,
          ventricular_arrhythmias_noted: ventricularArrhythmias,
          post_tap_map_mmhg: isNaN(postTapMap) ? 88 : postTapMap,
          post_tap_heart_rate_bpm: isNaN(postTapHr) ? 108 : postTapHr,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-tap-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pericardial-tamponade/operations/${encodeURIComponent(selectedOpId)}/tap-check`,
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
            showToast(`Recorded pericardiocentesis tap check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record pericardiocentesis tap check';
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

        const finalMap = parseInt(
          document.getElementById('clearance-final-map')?.value || '94',
          10
        );
        const vetLicense = (
          document.getElementById('clearance-vet-license')?.value || 'VET-CARDIO-99210'
        ).trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const raCollapseResolved = Boolean(
          document.getElementById('clearance-ra-collapse-resolved')?.checked
        );
        const tfastNoReaccumulation = Boolean(
          document.getElementById('clearance-tfast-no-reaccumulation')?.checked
        );
        const sinusNoVpcs = Boolean(document.getElementById('clearance-sinus-no-vpcs')?.checked);
        const oncologyEducated = Boolean(
          document.getElementById('clearance-oncology-educated')?.checked
        );

        const clearancePayload = {
          final_map_mmhg: isNaN(finalMap) ? 94 : finalMap,
          right_atrial_collapse_resolved: raCollapseResolved,
          repeat_tfast_24hr_no_reaccumulation: tfastNoReaccumulation,
          sinus_rhythm_no_vpcs: sinusNoVpcs,
          yunnan_baiyao_or_oncology_plan_educated: oncologyEducated,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-discharge-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pericardial-tamponade/operations/${encodeURIComponent(selectedOpId)}/discharge-clearance`,
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
              `Issued pericardiocentesis decompression discharge clearance for ${updated.id}`,
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
    const listEl = document.getElementById('tamponade-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.tamponade-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.tamponade-op-card');
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
