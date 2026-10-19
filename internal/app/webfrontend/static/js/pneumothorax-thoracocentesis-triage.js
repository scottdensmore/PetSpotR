/**
 * PetSpotR Tension Pneumothorax, Flail Chest, 7th–9th ICS Thoracocentesis &
 * Heimlich/Pleural Seal Chest Tube Cockpit Controller (Milestone 21.3)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'PTX-OP-101';

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
    const liveStatus = document.getElementById('pneumothorax-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('pneumothorax-error-banner');
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
      case 'TENSION_PNEUMOTHORAX_ABSENT_GLIDE_CRITICAL':
        return 'status-pill-critical';
      case 'ICS_7_9_THORACOCENTESIS_CHEST_TUBE_STAGED':
        return 'status-pill-staged';
      case 'PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-tension');
    const stagedEl = document.getElementById('kpi-staged-thoracocentesis');
    const clearedEl = document.getElementById('kpi-cleared-pleural-seal');
    const alertsEl = document.getElementById('kpi-tension-radiograph-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.critical_tension_count ??
      ops.filter((op) => op.status === 'TENSION_PNEUMOTHORAX_ABSENT_GLIDE_CRITICAL').length;
    const staged =
      metrics?.thoracocentesis_tube_staged_count ??
      ops.filter((op) => op.status === 'ICS_7_9_THORACOCENTESIS_CHEST_TUBE_STAGED').length;
    const cleared =
      metrics?.pleural_seal_removed_count ??
      ops.filter((op) => op.status === 'PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED').length;
    const alerts =
      metrics?.tension_radiograph_alert_count ??
      ops.filter((op) => Boolean(op.tension_radiograph_delay_alert)).length;

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
    const rrEl = document.getElementById('stage-resp-rate');
    const spo2El = document.getElementById('stage-spo2');
    const airEl = document.getElementById('stage-cumulative-air');
    const tapsEl = document.getElementById('stage-tap-count');
    const glideEl = document.getElementById('stage-glide-sign');
    const woundEl = document.getElementById('stage-open-wound');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (rrEl && p.respiratory_rate_bpm !== undefined) rrEl.value = String(p.respiratory_rate_bpm);
    if (spo2El && p.spo2_percent !== undefined) spo2El.value = String(p.spo2_percent);
    if (airEl && p.cumulative_air_aspirated_ml_kg !== undefined) {
      airEl.value = String(p.cumulative_air_aspirated_ml_kg);
    }
    if (tapsEl && p.tap_count_last_24hr !== undefined) {
      tapsEl.value = String(p.tap_count_last_24hr);
    }
    if (glideEl) glideEl.checked = Boolean(p.tfast_glide_sign_present);
    if (woundEl) woundEl.checked = Boolean(p.open_sucking_chest_wound);
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('pneumothorax-alert-banner');
    const clearanceBanner = document.getElementById('pneumothorax-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const plan = active.decompression_plan || {};
        const landmark = plan.needle_landmark_intercostal_space || '7th-9th ICS Dorsal Third';
        const spo2 = Number(p.spo2_percent ?? 83.0).toFixed(1);
        const air = Number(p.cumulative_air_aspirated_ml_kg ?? 62.0).toFixed(1);
        const suction = Number(plan.continuous_suction_cm_h2o ?? -15);
        const pitfall =
          plan.pneumothorax_pitfall_alert ||
          'CRITICAL TENSION PNEUMOTHORAX & INTERCOSTAL VESSEL PITFALL ALERT: NEVER take thoracic radiographs before needle thoracocentesis in tension pneumothorax (restraint on the X-ray table precipitates fatal obstructive shock cardiac arrest); NEVER insert the needle along the caudal margin of the rib (which lacerates the intercostal artery/vein/nerve bundle — always advance along the CRANIAL rib margin at the 7th–9th ICS dorsal third); and NEVER initiate positive-pressure ventilation before pleural decompression.';
        alertBanner.textContent = `⚠️ TENSION PNEUMOTHORAX, CRANIAL RIB MARGIN & ZERO RADIOGRAPH DELAY GUARD (${active.id} — ${active.pet_name} · ${landmark} · SpO2 ${spo2}% · Air ${air} mL/kg · Suction ${suction} cmH2O): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.removal_clearance) {
        const clr = active.removal_clearance;
        const hrs = Number(clr.zero_air_production_hours ?? 28);
        const fluid = Number(clr.final_fluid_output_ml_kg_day ?? 1.1).toFixed(1);
        const spo2 = Number(clr.room_air_spo2_percent ?? 98.0).toFixed(1);
        clearanceBanner.textContent = `✅ PLEURAL SEAL & ASEPTIC CHEST TUBE REMOVAL CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): Zero Air ${hrs}h · Fluid ${fluid} mL/kg/day · Room-Air SpO2 ${spo2}% · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
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
        '<p class="trust-empty-hint">No active Pneumothorax &amp; Thoracocentesis operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const plan = active.decompression_plan;
    const checks = Array.isArray(active.drainage_checks) ? active.drainage_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_SPONTANEOUS_BULLAE');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 25.0).toFixed(1);
    const status = escapeHtml(active.status || 'TENSION_PNEUMOTHORAX_ABSENT_GLIDE_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const rr = Number(p.respiratory_rate_bpm ?? 76);
    const spo2 = Number(p.spo2_percent ?? 83.0).toFixed(1);
    const glide = p.tfast_glide_sign_present ? 'PRESENT' : 'ABSENT';
    const air = Number(p.cumulative_air_aspirated_ml_kg ?? 62.0).toFixed(1);
    const taps = Number(p.tap_count_last_24hr ?? 3);
    const wound = p.open_sucking_chest_wound ? 'YES' : 'NO';

    const planHtml = plan
      ? `
        <div class="pneumothorax-plan-box" id="workspace-plan-box">
          <h4>Calculated 7th–9th ICS Thoracocentesis, Cranial Rib Margin &amp; Pleural Suction Plan</h4>
          <div class="pneumothorax-doses-grid">
            <div><strong>Needle Landmark:</strong> <span id="plan-needle-landmark">${escapeHtml(plan.needle_landmark_intercostal_space || '7th-9th ICS Dorsal Third')}</span></div>
            <div><strong>Cranial Rib Margin Guard:</strong> <span id="plan-cranial-rib-guard">${plan.cranial_rib_margin_vessel_guard ? 'ENFORCED' : 'OFF'}</span></div>
            <div><strong>Expected Tap Volume (30 mL/kg):</strong> <span id="plan-expected-tap">${Number(plan.expected_negative_pressure_tap_ml ?? 750.0).toFixed(1)} mL</span></div>
            <div><strong>Thoracostomy Tube Indicated:</strong> <span id="plan-chest-tube-indicated">${plan.thoracostomy_tube_indicated ? 'YES' : 'NO'}</span></div>
            <div><strong>Continuous Suction:</strong> <span id="plan-continuous-suction">${Number(plan.continuous_suction_cm_h2o ?? -15)} cmH2O</span></div>
            <div><strong>Autologous Blood Patch:</strong> <span id="plan-blood-patch">${Number(plan.autologous_blood_patch_ml_kg ?? 0.0).toFixed(1)} mL/kg</span></div>
            <div><strong>Zero Radiograph Delay Guard:</strong> <span id="plan-radiograph-guard">${plan.zero_pre_decompression_radiograph_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="pneumothorax-rationale-text" id="workspace-clinical-rationale">${escapeHtml(plan.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-PULM');
        const cAir = Number(chk.air_aspirated_rate_ml_kg_hr ?? 0.0).toFixed(1);
        const cFluid = Number(chk.fluid_aspirated_rate_ml_kg_day ?? 1.4).toFixed(1);
        const cGlide = chk.tfast_glide_sign_restored ? 'RESTORED' : 'ABSENT';
        const cSpo2 = Number(chk.spo2_percent ?? 97.0).toFixed(1);
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="pneumothorax-check-item"><strong>${callsign}</strong> · Air ${cAir} mL/kg/hr · Fluid ${cFluid} mL/kg/day · Glide ${cGlide} · SpO2 ${cSpo2}% — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="pneumothorax-workspace-card" data-workspace-op-id="${opId}">
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

        <div class="pneumothorax-vitals-summary" id="workspace-vitals-summary">
          <span><strong>Resp Rate:</strong> ${rr} bpm</span>
          <span><strong>SpO2:</strong> ${spo2}%</span>
          <span><strong>TFAST Glide Sign:</strong> ${glide}</span>
          <span><strong>Cumulative Air:</strong> ${air} mL/kg</span>
          <span><strong>24h Taps:</strong> ${taps}</span>
          <span><strong>Open Chest Wound:</strong> ${wound}</span>
        </div>

        ${planHtml}

        <div class="pneumothorax-drainage-checks-box">
          <h4>Serial Pleural Air/Fluid Drainage &amp; TFAST Glide Sign Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="drainage-checks-list" class="pneumothorax-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const plan = op.decompression_plan || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_SPONTANEOUS_BULLAE');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 25.0).toFixed(1);
    const status = escapeHtml(op.status || 'TENSION_PNEUMOTHORAX_ABSENT_GLIDE_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const rr = Number(p.respiratory_rate_bpm ?? 76);
    const spo2 = Number(p.spo2_percent ?? 83.0).toFixed(1);
    const glide = p.tfast_glide_sign_present ? 'PRESENT' : 'ABSENT';
    const air = Number(p.cumulative_air_aspirated_ml_kg ?? 62.0).toFixed(1);
    const taps = Number(p.tap_count_last_24hr ?? 3);

    const alertHtml = op.tension_radiograph_delay_alert
      ? `
        <div class="pneumothorax-alert-banner">
          ⚠️ TENSION PNEUMOTHORAX &amp; CRANIAL RIB MARGIN GUARD (SpO2 ${spo2}% · Air ${air} mL/kg · 7th–9th ICS Dorsal Third): ${escapeHtml(plan.pneumothorax_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.removal_clearance
      ? `
        <div class="pneumothorax-clearance-banner">
          ✅ PLEURAL SEAL &amp; TUBE REMOVED (${escapeHtml(op.removal_clearance.vet_license || '')}): Zero Air ${Number(op.removal_clearance.zero_air_production_hours ?? 28)}h · Fluid ${Number(op.removal_clearance.final_fluid_output_ml_kg_day ?? 1.1).toFixed(1)} mL/kg/day · SpO2 ${Number(op.removal_clearance.room_air_spo2_percent ?? 98.0).toFixed(1)}%
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card pneumothorax-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
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

        <div class="trust-kinematics-row pneumothorax-metrics-row">
          <span class="status-pill status-pill-species">RR: ${rr} bpm • SpO2: ${spo2}% • Glide: ${glide} • Air: ${air} mL/kg • Taps: ${taps}</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary pneumothorax-select-op-btn"
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

    const countBadge = document.getElementById('pneumothorax-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('pneumothorax-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="pneumothorax-ops-empty-msg">No Pneumothorax &amp; Thoracocentesis operations match the current filters.</p>';
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
        ? `/api/pneumothorax-thoracocentesis/operations?${qs}`
        : '/api/pneumothorax-thoracocentesis/operations';

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
        `/api/pneumothorax-thoracocentesis/operations/${encodeURIComponent(opId)}`,
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

  function setupFormsAndFilters() {
    const createForm = document.getElementById('create-pneumothorax-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (
          document.getElementById('create-incident-id')?.value || 'INC-PTX-9914'
        ).trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_SPONTANEOUS_BULLAE'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const respRate = parseInt(document.getElementById('create-resp-rate')?.value || '0', 10);
        const spo2 = parseFloat(document.getElementById('create-spo2')?.value || '0');
        const cumulativeAir = parseFloat(
          document.getElementById('create-cumulative-air')?.value || '0'
        );
        const tapCount = parseInt(document.getElementById('create-tap-count')?.value || '0', 10);
        const glideSign = Boolean(document.getElementById('create-glide-sign')?.checked);
        const openWound = Boolean(document.getElementById('create-open-wound')?.checked);

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            respiratory_rate_bpm: isNaN(respRate) ? 0 : respRate,
            spo2_percent: isNaN(spo2) ? 0 : spo2,
            tfast_glide_sign_present: glideSign,
            cumulative_air_aspirated_ml_kg: isNaN(cumulativeAir) ? 0 : cumulativeAir,
            tap_count_last_24hr: isNaN(tapCount) ? 0 : tapCount,
            open_sucking_chest_wound: openWound,
          },
        };

        const btn = document.getElementById('btn-create-pneumothorax-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/pneumothorax-thoracocentesis/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Created pneumothorax operation ${created.id} for ${created.pet_name}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create pneumothorax operation';
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
          species: 'CANINE_SPONTANEOUS_BULLAE',
          weight_kg: 25.0,
          respiratory_rate_bpm: 76,
          spo2_percent: 83.0,
          tfast_glide_sign_present: false,
          cumulative_air_aspirated_ml_kg: 62.0,
          tap_count_last_24hr: 3,
          open_sucking_chest_wound: false,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const species = (
          document.getElementById('stage-species')?.value || baseProfile.species
        ).trim();
        const respRate = parseInt(
          document.getElementById('stage-resp-rate')?.value ||
            String(baseProfile.respiratory_rate_bpm),
          10
        );
        const spo2 = parseFloat(
          document.getElementById('stage-spo2')?.value || String(baseProfile.spo2_percent)
        );
        const cumulativeAir = parseFloat(
          document.getElementById('stage-cumulative-air')?.value ||
            String(baseProfile.cumulative_air_aspirated_ml_kg)
        );
        const tapCount = parseInt(
          document.getElementById('stage-tap-count')?.value ||
            String(baseProfile.tap_count_last_24hr),
          10
        );
        const glideSign = Boolean(document.getElementById('stage-glide-sign')?.checked);
        const openWound = Boolean(document.getElementById('stage-open-wound')?.checked);

        const profilePayload = {
          ...baseProfile,
          species,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          respiratory_rate_bpm: isNaN(respRate) ? baseProfile.respiratory_rate_bpm : respRate,
          spo2_percent: isNaN(spo2) ? baseProfile.spo2_percent : spo2,
          tfast_glide_sign_present: glideSign,
          cumulative_air_aspirated_ml_kg: isNaN(cumulativeAir)
            ? baseProfile.cumulative_air_aspirated_ml_kg
            : cumulativeAir,
          tap_count_last_24hr: isNaN(tapCount) ? baseProfile.tap_count_last_24hr : tapCount,
          open_sucking_chest_wound: openWound,
        };

        const btn = document.getElementById('btn-stage-plan');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pneumothorax-thoracocentesis/operations/${encodeURIComponent(selectedOpId)}/stage-plan`,
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
              `Staged 7th–9th ICS thoracocentesis & chest tube plan for ${updated.id} → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage decompression plan';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('drainage-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const airRate = parseFloat(document.getElementById('check-air-rate')?.value || '0.0');
        const fluidRate = parseFloat(document.getElementById('check-fluid-rate')?.value || '1.4');
        const spo2 = parseFloat(document.getElementById('check-spo2')?.value || '97.0');
        const callsign = (document.getElementById('check-callsign')?.value || 'DR-PULM-11').trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();
        const glideRestored = Boolean(document.getElementById('check-glide-restored')?.checked);
        const subqEmphysema = Boolean(document.getElementById('check-subq-emphysema')?.checked);

        const checkPayload = {
          air_aspirated_rate_ml_kg_hr: isNaN(airRate) ? 0.0 : airRate,
          fluid_aspirated_rate_ml_kg_day: isNaN(fluidRate) ? 1.4 : fluidRate,
          tfast_glide_sign_restored: glideRestored,
          spo2_percent: isNaN(spo2) ? 97.0 : spo2,
          subcutaneous_emphysema_present: subqEmphysema,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-drainage-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pneumothorax-thoracocentesis/operations/${encodeURIComponent(selectedOpId)}/drainage-check`,
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
            showToast(`Recorded pleural drainage & TFAST glide check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record pleural drainage check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearanceForm = document.getElementById('removal-clearance-form');
    if (clearanceForm) {
      clearanceForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const zeroAirHours = parseInt(
          document.getElementById('clearance-zero-air-hours')?.value || '26',
          10
        );
        const finalFluid = parseFloat(
          document.getElementById('clearance-final-fluid')?.value || '1.2'
        );
        const roomAirSpo2 = parseFloat(
          document.getElementById('clearance-room-air-spo2')?.value || '97.5'
        );
        const vetLicense = (
          document.getElementById('clearance-vet-license')?.value || 'VET-PULM-99140'
        ).trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const glideConfirmed = Boolean(
          document.getElementById('clearance-glide-confirmed')?.checked
        );
        const tubeRemoved = Boolean(document.getElementById('clearance-tube-removed')?.checked);

        const clearancePayload = {
          zero_air_production_hours: isNaN(zeroAirHours) ? 26 : zeroAirHours,
          final_fluid_output_ml_kg_day: isNaN(finalFluid) ? 1.2 : finalFluid,
          bilateral_glide_sign_confirmed: glideConfirmed,
          room_air_spo2_percent: isNaN(roomAirSpo2) ? 97.5 : roomAirSpo2,
          chest_tube_aseptically_removed: tubeRemoved,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-removal-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pneumothorax-thoracocentesis/operations/${encodeURIComponent(selectedOpId)}/removal-clearance`,
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
              `Issued pleural seal & chest tube removal clearance for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to issue chest tube removal clearance';
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
    const listEl = document.getElementById('pneumothorax-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.pneumothorax-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.pneumothorax-op-card');
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
