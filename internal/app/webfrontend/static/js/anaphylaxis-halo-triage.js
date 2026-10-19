/**
 * PetSpotR Acute Anaphylaxis, Hemoabdomen Halo Sign (Gallbladder Wall Edema),
 * Epinephrine IM/CRI & Venous Pool Resuscitation Triage Cockpit Controller (Milestone 21.2)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'AH-OP-101';

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
    const liveStatus = document.getElementById('anaphylaxis-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('anaphylaxis-error-banner');
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
      case 'ANAPHYLACTIC_SHOCK_HALO_SIGN_CRITICAL':
        return 'status-pill-critical';
      case 'EPINEPHRINE_IM_CRI_CRYSTALLOID_FFP_STAGED':
        return 'status-pill-staged';
      case 'BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-shock');
    const stagedEl = document.getElementById('kpi-staged-epinephrine');
    const clearedEl = document.getElementById('kpi-cleared-biphasic');
    const hepaticEl = document.getElementById('kpi-hepatic-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.critical_shock_halo_count ??
      ops.filter((op) => op.status === 'ANAPHYLACTIC_SHOCK_HALO_SIGN_CRITICAL').length;
    const staged =
      metrics?.epinephrine_ffp_staged_count ??
      ops.filter((op) => op.status === 'EPINEPHRINE_IM_CRI_CRYSTALLOID_FFP_STAGED').length;
    const cleared =
      metrics?.biphasic_24hr_cleared_count ??
      ops.filter((op) => op.status === 'BIPHASIC_24H_NORMOTENSIVE_HALO_RESOLVED').length;
    const alerts =
      metrics?.hepatic_coagulopathy_alert_count ??
      ops.filter((op) => Boolean(op.hepatic_congestion_coagulopathy_alert)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (hepaticEl) hepaticEl.textContent = String(alerts);
  }

  function syncStageFormWithOperation(op) {
    if (!op || !op.patient_profile) return;
    const p = op.patient_profile;
    const weightEl = document.getElementById('stage-weight-kg');
    const speciesEl = document.getElementById('stage-species');
    const gbWallEl = document.getElementById('stage-gb-wall-mm');
    const mapEl = document.getElementById('stage-map-mmhg');
    const pcvEl = document.getElementById('stage-pcv-pct');
    const ptEl = document.getElementById('stage-pt-sec');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (gbWallEl && p.gallbladder_wall_thickness_mm !== undefined) {
      gbWallEl.value = String(p.gallbladder_wall_thickness_mm);
    }
    if (mapEl && p.mean_arterial_pressure_mmhg !== undefined) {
      mapEl.value = String(p.mean_arterial_pressure_mmhg);
    }
    if (pcvEl && p.packed_cell_volume_percent !== undefined) {
      pcvEl.value = String(p.packed_cell_volume_percent);
    }
    if (ptEl && p.prothrombin_time_seconds !== undefined) {
      ptEl.value = String(p.prothrombin_time_seconds);
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('anaphylaxis-alert-banner');
    const clearanceBanner = document.getElementById('anaphylaxis-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const plan = active.resuscitation_plan || {};
        const gbWall = Number(p.gallbladder_wall_thickness_mm ?? 6.4).toFixed(1);
        const map = Number(p.mean_arterial_pressure_mmhg ?? 46);
        const alt = Number(p.alanine_aminotransferase_u_l ?? 2450);
        const pcv = Number(p.packed_cell_volume_percent ?? 68.0).toFixed(1);
        const pt = Number(p.prothrombin_time_seconds ?? 28.5).toFixed(1);
        const pitfall =
          plan.anaphylaxis_pitfall_alert ||
          "CRITICAL ANAPHYLAXIS, GALLBLADDER HALO SIGN & HEMOABDOMEN PITFALL ALERT: NEVER mistake the ultrasound gallbladder wall striated 'Halo Sign' (> 3.0 mm edema) and anaphylactic non-clotting hemoabdomen for surgical gallbladder mucocele rupture; NEVER administer Diphenhydramine IV rapid bolus (give 2.0 mg/kg IM only AFTER Epinephrine); and NEVER discharge before >= 24h biphasic anaphylaxis observation.";
        alertBanner.textContent = `⚠️ ANAPHYLAXIS, GALLBLADDER HALO SIGN & HEMOABDOMEN GUARD (${active.id} — ${active.pet_name} · GB Wall ${gbWall} mm · MAP ${map} mmHg · ALT ${alt} U/L · PCV ${pcv}% · PT ${pt} s): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.discharge_clearance) {
        const clr = active.discharge_clearance;
        const hrs = Number(clr.biphasic_observation_hours ?? 26);
        const finalMap = Number(clr.final_map_mmhg ?? 92);
        clearanceBanner.textContent = `✅ 24H BIPHASIC NORMOTENSIVE & HALO SIGN RESOLVED CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): ${hrs}h Observation · Final MAP ${finalMap} mmHg · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
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
        '<p class="trust-empty-hint">No active Anaphylaxis &amp; Gallbladder Halo Sign operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const plan = active.resuscitation_plan;
    const checks = Array.isArray(active.biphasic_checks) ? active.biphasic_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_HYMENOPTERA_HEMOABDOMEN');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 6.8).toFixed(1);
    const status = escapeHtml(active.status || 'ANAPHYLACTIC_SHOCK_HALO_SIGN_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const gbWall = Number(p.gallbladder_wall_thickness_mm ?? 6.4).toFixed(1);
    const haloText = p.gallbladder_halo_sign_present ? 'HALO SIGN +' : 'Normal';
    const map = Number(p.mean_arterial_pressure_mmhg ?? 46);
    const alt = Number(p.alanine_aminotransferase_u_l ?? 2450);
    const pcv = Number(p.packed_cell_volume_percent ?? 68.0).toFixed(1);
    const pt = Number(p.prothrombin_time_seconds ?? 28.5).toFixed(1);

    const planHtml = plan
      ? `
        <div class="anaphylaxis-plan-box" id="workspace-plan-box">
          <h4>Calculated Epinephrine IM/CRI, Crystalloid Shock Bolus, Fresh Frozen Plasma &amp; H1/H2 Blockade Plan</h4>
          <div class="anaphylaxis-doses-grid">
            <div><strong>Epinephrine IM (0.01 mg/kg):</strong> <span id="plan-epi-im">${Number(plan.epinephrine_im_001_mg_kg_dose_mg ?? 0.07).toFixed(2)} mg IM</span></div>
            <div><strong>Epinephrine CRI (0.05 mcg/kg/min):</strong> <span id="plan-epi-cri">${Number(plan.epinephrine_cri_005_mcg_kg_min ?? 0.34).toFixed(2)} mcg/min</span></div>
            <div><strong>Isotonic Crystalloid Bolus:</strong> <span id="plan-crystalloid-bolus">${Number(plan.isotonic_crystalloid_bolus_ml ?? 136.0).toFixed(1)} mL IV</span></div>
            <div><strong>Fresh Frozen Plasma (15 mL/kg):</strong> <span id="plan-ffp-ml">${Number(plan.fresh_frozen_plasma_15_ml_kg_ml ?? 102.0).toFixed(1)} mL IV</span></div>
            <div><strong>Diphenhydramine IM (2.0 mg/kg):</strong> <span id="plan-diphenhydramine-im">${Number(plan.diphenhydramine_im_2_mg_kg_mg ?? 13.6).toFixed(1)} mg IM</span></div>
            <div><strong>Famotidine IV (0.5 mg/kg):</strong> <span id="plan-famotidine-iv">${Number(plan.famotidine_iv_05_mg_kg_mg ?? 3.4).toFixed(1)} mg IV</span></div>
            <div><strong>Surgical Cholecystectomy / IV Epi Guard:</strong> <span id="plan-cholecystectomy-guard">${plan.zero_iv_undiluted_epi_surgical_cholecystectomy_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="anaphylaxis-rationale-text" id="workspace-clinical-rationale">${escapeHtml(plan.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-ECC');
        const cMap = Number(chk.mean_arterial_pressure_mmhg ?? 84);
        const cGb = Number(chk.gallbladder_wall_thickness_mm ?? 2.3).toFixed(1);
        const cPcv = Number(chk.packed_cell_volume_percent ?? 47.0).toFixed(1);
        const cAlt = Number(chk.alanine_aminotransferase_u_l ?? 420);
        const cRelapse = chk.biphasic_relapse_signs_noted ? 'YES' : 'NO';
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="anaphylaxis-check-item"><strong>${callsign}</strong> · MAP ${cMap} mmHg · GB Wall ${cGb} mm · PCV ${cPcv}% · ALT ${cAlt} U/L · Relapse: ${cRelapse} — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="anaphylaxis-workspace-card" data-workspace-op-id="${opId}">
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

        <div class="anaphylaxis-vitals-summary" id="workspace-vitals-summary">
          <span><strong>GB Wall:</strong> ${gbWall} mm (${haloText})</span>
          <span><strong>MAP:</strong> ${map} mmHg</span>
          <span><strong>Serum ALT:</strong> ${alt} U/L</span>
          <span><strong>PCV:</strong> ${pcv}%</span>
          <span><strong>PT:</strong> ${pt} s</span>
        </div>

        ${planHtml}

        <div class="anaphylaxis-biphasic-checks-box">
          <h4>Serial 24h Biphasic Observation, MAP &amp; Gallbladder Wall Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="biphasic-checks-list" class="anaphylaxis-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const plan = op.resuscitation_plan || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_HYMENOPTERA_HEMOABDOMEN');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 6.8).toFixed(1);
    const status = escapeHtml(op.status || 'ANAPHYLACTIC_SHOCK_HALO_SIGN_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const gbWall = Number(p.gallbladder_wall_thickness_mm ?? 6.4).toFixed(1);
    const haloYesNo = p.gallbladder_halo_sign_present ? 'YES' : 'NO';
    const map = Number(p.mean_arterial_pressure_mmhg ?? 46);
    const alt = Number(p.alanine_aminotransferase_u_l ?? 2450);
    const pcv = Number(p.packed_cell_volume_percent ?? 68.0).toFixed(1);
    const pt = Number(p.prothrombin_time_seconds ?? 28.5).toFixed(1);

    const alertHtml = op.hepatic_congestion_coagulopathy_alert
      ? `
        <div class="anaphylaxis-alert-banner">
          ⚠️ HEPATIC VENOUS CONGESTION &amp; HALO SIGN GUARD (GB Wall ${gbWall} mm · MAP ${map} mmHg · PT ${pt} s): ${escapeHtml(plan.anaphylaxis_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.discharge_clearance
      ? `
        <div class="anaphylaxis-clearance-banner">
          ✅ 24H BIPHASIC NORMOTENSIVE &amp; HALO RESOLVED (${escapeHtml(op.discharge_clearance.vet_license || '')}): ${Number(op.discharge_clearance.biphasic_observation_hours ?? 26)}h Observation · Final MAP ${Number(op.discharge_clearance.final_map_mmhg ?? 92)} mmHg
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card anaphylaxis-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
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

        <div class="trust-kinematics-row anaphylaxis-metrics-row">
          <span class="status-pill status-pill-species">GB Wall: ${gbWall} mm • Halo Sign: ${haloYesNo} • MAP: ${map} mmHg • ALT: ${alt} U/L • PCV: ${pcv}% • PT: ${pt} s</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary anaphylaxis-select-op-btn"
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

    const countBadge = document.getElementById('anaphylaxis-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('anaphylaxis-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="anaphylaxis-ops-empty-msg">No Anaphylaxis &amp; Gallbladder Halo Sign operations match the current filters.</p>';
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
        ? `/api/anaphylaxis-halo/operations?${qs}`
        : '/api/anaphylaxis-halo/operations';

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
      const res = await fetch(`/api/anaphylaxis-halo/operations/${encodeURIComponent(opId)}`, {
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
    const createForm = document.getElementById('create-anaphylaxis-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (
          document.getElementById('create-incident-id')?.value || 'INC-AH-9904'
        ).trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_HYMENOPTERA_HEMOABDOMEN'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const gbWallMm = parseFloat(document.getElementById('create-gb-wall-mm')?.value || '0');
        const haloSign = Boolean(document.getElementById('create-halo-sign-present')?.checked);
        const mapMmHg = parseInt(document.getElementById('create-map-mmhg')?.value || '0', 10);
        const altUl = parseInt(document.getElementById('create-alt-ul')?.value || '0', 10);
        const pcvPct = parseFloat(document.getElementById('create-pcv-pct')?.value || '0');
        const ptSec = parseFloat(document.getElementById('create-pt-sec')?.value || '0');

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            gallbladder_wall_thickness_mm: isNaN(gbWallMm) ? 0 : gbWallMm,
            gallbladder_halo_sign_present: haloSign,
            mean_arterial_pressure_mmhg: isNaN(mapMmHg) ? 0 : mapMmHg,
            alanine_aminotransferase_u_l: isNaN(altUl) ? 0 : altUl,
            packed_cell_volume_percent: isNaN(pcvPct) ? 0 : pcvPct,
            prothrombin_time_seconds: isNaN(ptSec) ? 0 : ptSec,
          },
        };

        const btn = document.getElementById('btn-create-anaphylaxis-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/anaphylaxis-halo/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Created Anaphylaxis Halo operation ${created.id} for ${created.pet_name}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create Anaphylaxis Halo operation';
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
          species: 'CANINE_HYMENOPTERA_HEMOABDOMEN',
          weight_kg: 6.8,
          gallbladder_wall_thickness_mm: 6.4,
          gallbladder_halo_sign_present: true,
          mean_arterial_pressure_mmhg: 46,
          alanine_aminotransferase_u_l: 2450,
          packed_cell_volume_percent: 68.0,
          prothrombin_time_seconds: 28.5,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const species = (
          document.getElementById('stage-species')?.value || baseProfile.species
        ).trim();
        const gbWallMm = parseFloat(
          document.getElementById('stage-gb-wall-mm')?.value ||
            String(baseProfile.gallbladder_wall_thickness_mm)
        );
        const mapMmHg = parseInt(
          document.getElementById('stage-map-mmhg')?.value ||
            String(baseProfile.mean_arterial_pressure_mmhg),
          10
        );
        const pcvPct = parseFloat(
          document.getElementById('stage-pcv-pct')?.value ||
            String(baseProfile.packed_cell_volume_percent)
        );
        const ptSec = parseFloat(
          document.getElementById('stage-pt-sec')?.value ||
            String(baseProfile.prothrombin_time_seconds)
        );

        const resolvedGbWall = isNaN(gbWallMm)
          ? baseProfile.gallbladder_wall_thickness_mm
          : gbWallMm;

        const profilePayload = {
          ...baseProfile,
          species,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          gallbladder_wall_thickness_mm: resolvedGbWall,
          gallbladder_halo_sign_present: resolvedGbWall > 3.0,
          mean_arterial_pressure_mmhg: isNaN(mapMmHg)
            ? baseProfile.mean_arterial_pressure_mmhg
            : mapMmHg,
          packed_cell_volume_percent: isNaN(pcvPct)
            ? baseProfile.packed_cell_volume_percent
            : pcvPct,
          prothrombin_time_seconds: isNaN(ptSec) ? baseProfile.prothrombin_time_seconds : ptSec,
        };

        const btn = document.getElementById('btn-stage-plan');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/anaphylaxis-halo/operations/${encodeURIComponent(selectedOpId)}/stage-plan`,
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
              `Staged Epinephrine IM/CRI, Crystalloid & FFP plan for ${updated.id} → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage resuscitation plan';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('biphasic-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const mapMmHg = parseInt(document.getElementById('check-map-mmhg')?.value || '84', 10);
        const gbWallMm = parseFloat(document.getElementById('check-gb-wall-mm')?.value || '2.3');
        const pcvPct = parseFloat(document.getElementById('check-pcv-pct')?.value || '47.0');
        const altUl = parseInt(document.getElementById('check-alt-ul')?.value || '420', 10);
        const callsign = (document.getElementById('check-callsign')?.value || 'DR-ECC-41').trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();
        const relapse = Boolean(document.getElementById('check-biphasic-relapse')?.checked);

        const checkPayload = {
          mean_arterial_pressure_mmhg: isNaN(mapMmHg) ? 84 : mapMmHg,
          gallbladder_wall_thickness_mm: isNaN(gbWallMm) ? 2.3 : gbWallMm,
          packed_cell_volume_percent: isNaN(pcvPct) ? 47.0 : pcvPct,
          alanine_aminotransferase_u_l: isNaN(altUl) ? 420 : altUl,
          biphasic_relapse_signs_noted: relapse,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-biphasic-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/anaphylaxis-halo/operations/${encodeURIComponent(selectedOpId)}/biphasic-check`,
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
            showToast(`Recorded 24h biphasic & gallbladder Halo check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record biphasic check';
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

        const obsHours = parseInt(
          document.getElementById('clearance-observation-hours')?.value || '26',
          10
        );
        const finalMap = parseInt(
          document.getElementById('clearance-final-map')?.value || '88',
          10
        );
        const vetLicense = (
          document.getElementById('clearance-vet-license')?.value || 'VET-ECC-99108'
        ).trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const haloResolved = Boolean(document.getElementById('clearance-halo-resolved')?.checked);
        const coagNormalized = Boolean(
          document.getElementById('clearance-coagulopathy-normalized')?.checked
        );
        const epipenEducated = Boolean(
          document.getElementById('clearance-epipen-educated')?.checked
        );

        const clearancePayload = {
          biphasic_observation_hours: isNaN(obsHours) ? 26 : obsHours,
          final_map_mmhg: isNaN(finalMap) ? 88 : finalMap,
          gallbladder_halo_resolved: haloResolved,
          coagulopathy_normalized: coagNormalized,
          epipen_auto_injector_educated: epipenEducated,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-discharge-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/anaphylaxis-halo/operations/${encodeURIComponent(selectedOpId)}/discharge-clearance`,
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
              `Issued 24h biphasic & Halo Sign resolved clearance for ${updated.id}`,
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
    const listEl = document.getElementById('anaphylaxis-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.anaphylaxis-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.anaphylaxis-op-card');
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
