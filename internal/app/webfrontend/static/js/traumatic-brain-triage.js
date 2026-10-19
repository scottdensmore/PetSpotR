/**
 * PetSpotR Traumatic Brain Injury (TBI), Modified Glasgow Coma Scale (MGCS),
 * Cushing's Reflex & Hypertonic Saline/Mannitol Cockpit Controller (Milestone 21.1)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'TBI-OP-101';

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
    const liveStatus = document.getElementById('tbi-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('tbi-error-banner');
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
      case 'CUSHING_REFLEX_ICP_HERNIATION_CRITICAL':
        return 'status-pill-critical';
      case 'HTS_MANNITOL_30DEG_HEAD_ELEVATION_STAGED':
        return 'status-pill-staged';
      case 'MGCS_RECOVERED_NORMOCAPNIC_CLEARED':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-cushing-critical');
    const stagedEl = document.getElementById('kpi-staged-hts-mannitol');
    const clearedEl = document.getElementById('kpi-cleared-mgcs');
    const alertsEl = document.getElementById('kpi-cushing-steroid-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.cushing_herniation_count ??
      ops.filter((op) => op.status === 'CUSHING_REFLEX_ICP_HERNIATION_CRITICAL').length;
    const staged =
      metrics?.hypertonic_mannitol_staged ??
      ops.filter((op) => op.status === 'HTS_MANNITOL_30DEG_HEAD_ELEVATION_STAGED').length;
    const cleared =
      metrics?.mgcs_recovered_cleared_count ??
      ops.filter((op) => op.status === 'MGCS_RECOVERED_NORMOCAPNIC_CLEARED').length;
    const alerts =
      metrics?.cushing_steroid_alert_count ??
      ops.filter((op) => Boolean(op.cushing_herniation_steroid_alert)).length;

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
    const mgcsEl = document.getElementById('stage-mgcs');
    const mapEl = document.getElementById('stage-map-mmhg');
    const hrEl = document.getElementById('stage-hr-bpm');
    const etco2El = document.getElementById('stage-etco2-mmhg');
    const pupilsEl = document.getElementById('stage-pupils');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (mgcsEl && p.modified_glasgow_coma_score !== undefined) {
      mgcsEl.value = String(p.modified_glasgow_coma_score);
    }
    if (mapEl && p.mean_arterial_pressure_mmhg !== undefined) {
      mapEl.value = String(p.mean_arterial_pressure_mmhg);
    }
    if (hrEl && p.heart_rate_bpm !== undefined) {
      hrEl.value = String(p.heart_rate_bpm);
    }
    if (etco2El && p.end_tidal_co2_mmhg !== undefined) {
      etco2El.value = String(p.end_tidal_co2_mmhg);
    }
    if (pupilsEl && p.pupillary_status) {
      pupilsEl.value = p.pupillary_status;
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('tbi-alert-banner');
    const clearanceBanner = document.getElementById('tbi-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const plan = active.neuroprotective_plan || {};
        const mgcs = Number(p.modified_glasgow_coma_score ?? 8);
        const map = Number(p.mean_arterial_pressure_mmhg ?? 148);
        const hr = Number(p.heart_rate_bpm ?? 46);
        const etco2 = Number(p.end_tidal_co2_mmhg ?? 48.0).toFixed(1);
        const headDeg = Number(plan.head_elevation_angle_degrees ?? 30);
        const pitfall =
          plan.tbi_pitfall_alert ||
          "CRITICAL TBI, CUSHING'S REFLEX & ICP HERNIATION PITFALL ALERT: NEVER administer corticosteroids (Dexamethasone/Prednisolone) in Traumatic Brain Injury (contraindicated per CRASH trial — increases mortality and hyperglycemia); NEVER compress external jugular veins or flex the neck; and NEVER give Mannitol to hypovolemic/hypotensive patients before restoring intravascular volume with 7.5% Hypertonic Saline.";
        alertBanner.textContent = `⚠️ CUSHING'S REFLEX, ICP HERNIATION & STEROID CONTRAINDICATION GUARD (${active.id} — ${active.pet_name} · MGCS ${mgcs}/18 · MAP ${map} mmHg · HR ${hr} bpm · EtCO2 ${etco2} mmHg · Head ${headDeg}°): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.discharge_clearance) {
        const clr = active.discharge_clearance;
        const mgcs = Number(clr.final_modified_glasgow_score ?? 17);
        const map = Number(clr.final_map_mmhg ?? 96);
        clearanceBanner.textContent = `✅ MGCS RECOVERED & NORMOCAPNIC DISCHARGE CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): Final MGCS ${mgcs}/18 · Final MAP ${map} mmHg · Isocoric Responsive Pupils · Intact Gag/Swallow · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
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
        '<p class="trust-empty-hint">No active Traumatic Brain Injury (TBI) operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const plan = active.neuroprotective_plan;
    const checks = Array.isArray(active.neuro_checks) ? active.neuro_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_BLUNT_VEHICULAR_TBI');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 42.0).toFixed(1);
    const status = escapeHtml(active.status || 'CUSHING_REFLEX_ICP_HERNIATION_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const mgcs = Number(p.modified_glasgow_coma_score ?? 8);
    const map = Number(p.mean_arterial_pressure_mmhg ?? 148);
    const hr = Number(p.heart_rate_bpm ?? 46);
    const etco2 = Number(p.end_tidal_co2_mmhg ?? 48.0).toFixed(1);
    const pupils = escapeHtml(p.pupillary_status || 'ANISOCORIA_UNILATERAL_MYDRIASIS');
    const cushing = p.cushing_reflex_present ? 'PRESENT' : 'ABSENT';

    const planHtml = plan
      ? `
        <div class="tbi-plan-box" id="workspace-plan-box">
          <h4>Calculated 7.5% Hypertonic Saline, Mannitol &amp; 30&deg; Slant-Board Neuroprotective Plan</h4>
          <div class="tbi-doses-grid">
            <div><strong>7.5% Hypertonic Saline:</strong> <span id="plan-hts-dose">${Number(plan.hypertonic_saline_75pct_ml_kg ?? 4.0).toFixed(1)} mL/kg (${Number(plan.hypertonic_saline_total_dose_ml ?? 168.0).toFixed(1)} mL)</span></div>
            <div><strong>Mannitol 0.5 g/kg Slow IV:</strong> <span id="plan-mannitol-grams">${Number(plan.mannitol_05_g_kg_slow_20min_grams ?? 21.0).toFixed(2)} g (20m Filter)</span></div>
            <div><strong>Slant-Board Head Elevation:</strong> <span id="plan-head-elevation">${Number(plan.head_elevation_angle_degrees ?? 30)}&deg;</span></div>
            <div><strong>Target Minimum MAP:</strong> <span id="plan-target-map">&ge; ${Number(plan.target_map_minimum_mmhg ?? 80)} mmHg (CPP Guard)</span></div>
            <div><strong>Target Normocapnic EtCO2:</strong> <span id="plan-target-etco2">${escapeHtml(plan.target_etco2_range_mmhg || '35-40 mmHg')}</span></div>
            <div><strong>No Jugular / No Steroid Guard:</strong> <span id="plan-steroid-guard">${plan.zero_jugular_compression_corticosteroid_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="tbi-rationale-text" id="workspace-clinical-rationale">${escapeHtml(plan.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-NEURO');
        const cMgcs = Number(chk.modified_glasgow_coma_score ?? 14);
        const cMap = Number(chk.mean_arterial_pressure_mmhg ?? 96);
        const cHr = Number(chk.heart_rate_bpm ?? 88);
        const cEtco2 = Number(chk.end_tidal_co2_mmhg ?? 37.0).toFixed(1);
        const cPupils = escapeHtml(chk.pupillary_status || 'RESPONSIVE_ISOCORIC');
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="tbi-check-item"><strong>${callsign}</strong> · MGCS ${cMgcs}/18 · MAP ${cMap} mmHg · HR ${cHr} bpm · EtCO2 ${cEtco2} mmHg · ${cPupils} — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="tbi-workspace-card" data-workspace-op-id="${opId}">
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

        <div class="tbi-vitals-summary" id="workspace-vitals-summary">
          <span><strong>MGCS:</strong> ${mgcs}/18</span>
          <span><strong>MAP:</strong> ${map} mmHg</span>
          <span><strong>HR:</strong> ${hr} bpm</span>
          <span><strong>EtCO2:</strong> ${etco2} mmHg</span>
          <span><strong>Pupils:</strong> ${pupils}</span>
          <span><strong>Cushing's Reflex:</strong> ${cushing}</span>
        </div>

        ${planHtml}

        <div class="tbi-neuro-checks-box">
          <h4>Serial MGCS, MAP, EtCO2 &amp; Pupillary Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="neuro-checks-list" class="tbi-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const plan = op.neuroprotective_plan || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_BLUNT_VEHICULAR_TBI');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 42.0).toFixed(1);
    const status = escapeHtml(op.status || 'CUSHING_REFLEX_ICP_HERNIATION_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const mgcs = Number(p.modified_glasgow_coma_score ?? 8);
    const map = Number(p.mean_arterial_pressure_mmhg ?? 148);
    const hr = Number(p.heart_rate_bpm ?? 46);
    const etco2 = Number(p.end_tidal_co2_mmhg ?? 48.0).toFixed(1);
    const pupils = escapeHtml(p.pupillary_status || 'ANISOCORIA_UNILATERAL_MYDRIASIS');

    const alertHtml = op.cushing_herniation_steroid_alert
      ? `
        <div class="tbi-alert-banner">
          ⚠️ CUSHING'S REFLEX &amp; ICP HERNIATION ALERT (MGCS ${mgcs}/18 · MAP ${map} mmHg · HR ${hr} bpm · 30&deg; Head Elevation): ${escapeHtml(plan.tbi_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.discharge_clearance
      ? `
        <div class="tbi-clearance-banner">
          ✅ MGCS RECOVERED &amp; NORMOCAPNIC CLEARED (${escapeHtml(op.discharge_clearance.vet_license || '')}): MGCS ${Number(op.discharge_clearance.final_modified_glasgow_score ?? 17)}/18 · MAP ${Number(op.discharge_clearance.final_map_mmhg ?? 96)} mmHg · Isocoric Responsive Pupils
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card tbi-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
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

        <div class="trust-kinematics-row tbi-metrics-row">
          <span class="status-pill status-pill-species">MGCS: ${mgcs}/18 • MAP: ${map} mmHg • HR: ${hr} bpm • EtCO2: ${etco2} mmHg • Pupils: ${pupils}</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary tbi-select-op-btn"
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

    const countBadge = document.getElementById('tbi-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('tbi-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="tbi-ops-empty-msg">No Traumatic Brain Injury (TBI) operations match the current filters.</p>';
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
        ? `/api/traumatic-brain/operations?${qs}`
        : '/api/traumatic-brain/operations';

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
      const res = await fetch(`/api/traumatic-brain/operations/${encodeURIComponent(opId)}`, {
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
    const createForm = document.getElementById('create-tbi-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (document.getElementById('create-incident-id')?.value || 'INC-TBI-9804').trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_BLUNT_VEHICULAR_TBI'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const mgcs = parseInt(document.getElementById('create-mgcs')?.value || '0', 10);
        const mapMmHg = parseInt(document.getElementById('create-map-mmhg')?.value || '0', 10);
        const hrBpm = parseInt(document.getElementById('create-hr-bpm')?.value || '0', 10);
        const etco2MmHg = parseFloat(document.getElementById('create-etco2-mmhg')?.value || '0');
        const pupils = (
          document.getElementById('create-pupils')?.value || 'ANISOCORIA_UNILATERAL_MYDRIASIS'
        ).trim();
        const cushingReflex = Boolean(document.getElementById('create-cushing-reflex')?.checked);

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            modified_glasgow_coma_score: isNaN(mgcs) ? 0 : mgcs,
            mean_arterial_pressure_mmhg: isNaN(mapMmHg) ? 0 : mapMmHg,
            heart_rate_bpm: isNaN(hrBpm) ? 0 : hrBpm,
            end_tidal_co2_mmhg: isNaN(etco2MmHg) ? 0 : etco2MmHg,
            pupillary_status: pupils,
            cushing_reflex_present: cushingReflex,
          },
        };

        const btn = document.getElementById('btn-create-tbi-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/traumatic-brain/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(`Created TBI operation ${created.id} for ${created.pet_name}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create TBI operation';
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
          species: 'CANINE_BLUNT_VEHICULAR_TBI',
          weight_kg: 42.0,
          modified_glasgow_coma_score: 12,
          mean_arterial_pressure_mmhg: 104,
          heart_rate_bpm: 82,
          end_tidal_co2_mmhg: 38.0,
          pupillary_status: 'MIOTIC_PINPOINT_BILATERAL',
          cushing_reflex_present: false,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const species = (
          document.getElementById('stage-species')?.value || baseProfile.species
        ).trim();
        const mgcs = parseInt(
          document.getElementById('stage-mgcs')?.value || String(baseProfile.modified_glasgow_coma_score),
          10
        );
        const mapMmHg = parseInt(
          document.getElementById('stage-map-mmhg')?.value || String(baseProfile.mean_arterial_pressure_mmhg),
          10
        );
        const hrBpm = parseInt(
          document.getElementById('stage-hr-bpm')?.value || String(baseProfile.heart_rate_bpm),
          10
        );
        const etco2MmHg = parseFloat(
          document.getElementById('stage-etco2-mmhg')?.value || String(baseProfile.end_tidal_co2_mmhg)
        );
        const pupils = (
          document.getElementById('stage-pupils')?.value || baseProfile.pupillary_status
        ).trim();

        const profilePayload = {
          ...baseProfile,
          species,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          modified_glasgow_coma_score: isNaN(mgcs) ? baseProfile.modified_glasgow_coma_score : mgcs,
          mean_arterial_pressure_mmhg: isNaN(mapMmHg) ? baseProfile.mean_arterial_pressure_mmhg : mapMmHg,
          heart_rate_bpm: isNaN(hrBpm) ? baseProfile.heart_rate_bpm : hrBpm,
          end_tidal_co2_mmhg: isNaN(etco2MmHg) ? baseProfile.end_tidal_co2_mmhg : etco2MmHg,
          pupillary_status: pupils,
        };

        const btn = document.getElementById('btn-stage-plan');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/traumatic-brain/operations/${encodeURIComponent(selectedOpId)}/stage-plan`,
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
            showToast(`Staged 7.5% HTS / Mannitol / 30° Head Elevation plan for ${updated.id} → ${updated.status}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage neuroprotective plan';
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

        const mgcs = parseInt(document.getElementById('check-mgcs')?.value || '15', 10);
        const mapMmHg = parseInt(document.getElementById('check-map-mmhg')?.value || '96', 10);
        const hrBpm = parseInt(document.getElementById('check-hr-bpm')?.value || '88', 10);
        const etco2MmHg = parseFloat(document.getElementById('check-etco2-mmhg')?.value || '37.0');
        const pupils = (
          document.getElementById('check-pupils')?.value || 'RESPONSIVE_ISOCORIC'
        ).trim();
        const callsign = (document.getElementById('check-callsign')?.value || 'DR-NEURO-11').trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();

        const checkPayload = {
          modified_glasgow_coma_score: isNaN(mgcs) ? 15 : mgcs,
          mean_arterial_pressure_mmhg: isNaN(mapMmHg) ? 96 : mapMmHg,
          heart_rate_bpm: isNaN(hrBpm) ? 88 : hrBpm,
          end_tidal_co2_mmhg: isNaN(etco2MmHg) ? 37.0 : etco2MmHg,
          pupillary_status: pupils,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-neuro-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/traumatic-brain/operations/${encodeURIComponent(selectedOpId)}/neuro-check`,
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
            showToast(`Recorded serial MGCS & neuro check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record neuro check';
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

        const finalMgcs = parseInt(
          document.getElementById('clearance-final-mgcs')?.value || '17',
          10
        );
        const finalMap = parseInt(
          document.getElementById('clearance-final-map')?.value || '96',
          10
        );
        const vetLicense = (
          document.getElementById('clearance-vet-license')?.value || 'VET-NEURO-98104'
        ).trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const pupilsIsocoric = Boolean(
          document.getElementById('clearance-pupils-isocoric')?.checked
        );
        const gagReflex = Boolean(document.getElementById('clearance-gag-reflex')?.checked);
        const ambulatoryNormocapnic = Boolean(
          document.getElementById('clearance-ambulatory-normocapnic')?.checked
        );

        const clearancePayload = {
          final_modified_glasgow_score: isNaN(finalMgcs) ? 17 : finalMgcs,
          final_map_mmhg: isNaN(finalMap) ? 96 : finalMap,
          pupils_isocoric_responsive: pupilsIsocoric,
          gag_swallow_reflex_intact: gagReflex,
          ambulatory_normocapnic_verified: ambulatoryNormocapnic,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-discharge-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/traumatic-brain/operations/${encodeURIComponent(selectedOpId)}/discharge-clearance`,
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
            showToast(`Issued MGCS-recovered normocapnic clearance for ${updated.id}`, 'success');
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
    const listEl = document.getElementById('tbi-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.tbi-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.tbi-op-card');
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
