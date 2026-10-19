/**
 * PetSpotR Acute Traumatic Hemoabdomen, Autotransfusion, Centesis Non-Clotting
 * & Norepinephrine/TXA Resuscitation Cockpit Controller (Milestone 20.2)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'HA-OP-101';

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
    const liveStatus = document.getElementById('hemoabdomen-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('hemoabdomen-error-banner');
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
      case 'HEMORRHAGIC_SHOCK_HEMOABDOMEN_CRITICAL':
        return 'status-pill-critical';
      case 'AUTOTRANSFUSION_TXA_HYPOTENSIVE_STAGED':
        return 'status-pill-staged';
      case 'HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-shock');
    const stagedEl = document.getElementById('kpi-staged-autotransfusion');
    const clearedEl = document.getElementById('kpi-cleared-hemostasis');
    const alertEl = document.getElementById('kpi-clot-neoplasia-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.hemorrhagic_shock_critical_count ??
      ops.filter((op) => op.status === 'HEMORRHAGIC_SHOCK_HEMOABDOMEN_CRITICAL').length;
    const staged =
      metrics?.autotransfusion_txa_staged_count ??
      ops.filter((op) => op.status === 'AUTOTRANSFUSION_TXA_HYPOTENSIVE_STAGED').length;
    const cleared =
      metrics?.hemostasis_cleared_count ??
      ops.filter((op) => op.status === 'HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE').length;
    const alerts =
      metrics?.clot_popping_neoplasia_alert_count ??
      ops.filter((op) => Boolean(op.clot_popping_neoplasia_alert)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (alertEl) alertEl.textContent = String(alerts);
  }

  function syncStageFormWithOperation(op) {
    if (!op || !op.patient_profile) return;
    const p = op.patient_profile;
    const weightEl = document.getElementById('stage-weight-kg');
    const speciesEl = document.getElementById('stage-species');
    const effusionPcvEl = document.getElementById('stage-effusion-pcv');
    const mapEl = document.getElementById('stage-map-mmhg');
    const nonClottingEl = document.getElementById('stage-non-clotting-verified');
    const neoplasiaEl = document.getElementById('stage-suspected-neoplasia');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (effusionPcvEl && p.effusion_pcv_percent !== undefined) {
      effusionPcvEl.value = String(p.effusion_pcv_percent);
    }
    if (mapEl && p.mean_arterial_pressure_mmhg !== undefined) {
      mapEl.value = String(p.mean_arterial_pressure_mmhg);
    }
    if (nonClottingEl && p.effusion_non_clotting_verified !== undefined) {
      nonClottingEl.checked = Boolean(p.effusion_non_clotting_verified);
    }
    if (neoplasiaEl && p.suspected_neoplasia !== undefined) {
      neoplasiaEl.checked = Boolean(p.suspected_neoplasia);
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('hemoabdomen-alert-banner');
    const clearanceBanner = document.getElementById('hemoabdomen-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const plan = active.resuscitation_plan || {};
        const mapMmHg = Number(p.mean_arterial_pressure_mmhg ?? 65);
        const targetMap = Number(plan.permissive_hypotension_target_map_mmhg ?? 65);
        const filterUm = Number(plan.micron_filter_size_um ?? 170);
        const pitfall =
          plan.hemoabdomen_pitfall_alert ||
          'CRITICAL HEMOABDOMEN RESUSCITATION PITFALL ALERT: NEVER administer full-shock 90 mL/kg crystalloid boluses (pops nascent clots via dilutional coagulopathy); enforce ZERO compressive abdominal wraps; and NEVER autotransfuse unverified neoplastic effusions or infuse shed blood without a 170–260 µm blood filter.';
        alertBanner.textContent = `⚠️ PERMISSIVE HYPOTENSION & AUTOTRANSFUSION GUARD (${active.id} — ${active.pet_name} · MAP ${mapMmHg} mmHg · Target MAP ${targetMap} mmHg · Filter ${filterUm} µm): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.hemostasis_clearance) {
        const clr = active.hemostasis_clearance;
        const pcv = Number(clr.final_peripheral_pcv_percent ?? 31.0).toFixed(1);
        const lac = Number(clr.final_plasma_lactate_mmol_l ?? 1.6).toFixed(1);
        clearanceBanner.textContent = `✅ HEMOSTASIS & NORMOTENSIVE CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): Final MAP ${clr.final_map_mmhg} mmHg · Final PCV ${pcv}% · Lactate ${lac} mmol/L · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
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
        '<p class="trust-empty-hint">No active hemoabdomen triage operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const plan = active.resuscitation_plan;
    const checks = Array.isArray(active.hemodynamic_checks) ? active.hemodynamic_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_BLUNT_TRAUMA_HEPATIC');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 30.0).toFixed(1);
    const status = escapeHtml(active.status || 'HEMORRHAGIC_SHOCK_HEMOABDOMEN_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const afsScore = Number(p.afast_afs4_score ?? 0);
    const effusionPcv = Number(p.effusion_pcv_percent ?? 0).toFixed(1);
    const peripheralPcv = Number(p.peripheral_pcv_percent ?? 30).toFixed(1);
    const mapMmHg = Number(p.mean_arterial_pressure_mmhg ?? 65);
    const lactate = Number(p.plasma_lactate_mmol_l ?? 2.0).toFixed(1);

    const planHtml = plan
      ? `
        <div class="hemoabdomen-plan-box" id="workspace-plan-box">
          <h4>Calculated Permissive Hypotensive, TXA &amp; Autotransfusion Resuscitation Plan</h4>
          <div class="hemoabdomen-doses-grid">
            <div><strong>Target MAP:</strong> <span id="plan-target-map">${Number(plan.permissive_hypotension_target_map_mmhg ?? 65)} mmHg</span></div>
            <div><strong>Crystalloid Aliquot:</strong> <span id="plan-crystalloid-aliquot">${Number(plan.crystalloid_aliquot_ml_kg ?? 10.0).toFixed(1)} mL/kg</span></div>
            <div><strong>Tranexamic Acid (10 mg/kg IV):</strong> <span id="plan-txa-mg">${Number(plan.tranexamic_acid_10mg_kg_dose_mg ?? 0).toFixed(1)} mg</span></div>
            <div><strong>Autotransfusion:</strong> <span id="plan-autotransfusion-status">${plan.autotransfusion_indicated ? 'INDICATED' : 'CONTRAINDICATED / DEFERRED'}</span></div>
            <div><strong>Max Salvage Collect (15 mL/kg):</strong> <span id="plan-max-collect-ml">${Number(plan.autotransfusion_max_collect_ml ?? 0).toFixed(1)} mL</span></div>
            <div><strong>Inline Blood Filter:</strong> <span id="plan-filter-um">${Number(plan.micron_filter_size_um ?? 170)} µm</span></div>
            <div><strong>Norepinephrine CRI:</strong> <span id="plan-norepi-cri">${Number(plan.norepinephrine_cri_mcg_kg_min ?? 0).toFixed(2)} mcg/kg/min</span></div>
            <div><strong>Zero Belly Wrap Guard:</strong> <span id="plan-wrap-guard">${plan.zero_abdominal_compression_wrap_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="hemoabdomen-rationale-text" id="workspace-clinical-rationale">${escapeHtml(plan.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-TRAUMA');
        const cMap = Number(chk.mean_arterial_pressure_mmhg ?? 68);
        const cHr = Number(chk.heart_rate_bpm ?? 120);
        const cPcv = Number(chk.peripheral_pcv_percent ?? 30.0).toFixed(1);
        const cTs = Number(chk.total_solids_gdl ?? 5.4).toFixed(1);
        const cLac = Number(chk.plasma_lactate_mmol_l ?? 2.0).toFixed(1);
        const cAuto = Number(chk.autotransfused_volume_ml ?? 0).toFixed(0);
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="hemoabdomen-check-item"><strong>${callsign}</strong> · MAP ${cMap} mmHg · HR ${cHr} bpm · PCV ${cPcv}% · TS ${cTs} g/dL · Lactate ${cLac} mmol/L · Autotransfused ${cAuto} mL — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="hemoabdomen-workspace-card" data-workspace-op-id="${opId}">
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

        <div class="hemoabdomen-vitals-summary" id="workspace-vitals-summary">
          <span><strong>AFAST AFS:</strong> ${afsScore}/4</span>
          <span><strong>Effusion PCV:</strong> ${effusionPcv}%</span>
          <span><strong>Peripheral PCV:</strong> ${peripheralPcv}%</span>
          <span><strong>MAP:</strong> ${mapMmHg} mmHg</span>
          <span><strong>Lactate:</strong> ${lactate} mmol/L</span>
        </div>

        ${planHtml}

        <div class="hemoabdomen-checks-box">
          <h4>Serial Hemodynamic &amp; Autotransfusion Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="hemodynamic-checks-list" class="hemoabdomen-checks-list">
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
    const species = escapeHtml(p.species || 'CANINE_BLUNT_TRAUMA_HEPATIC');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 30.0).toFixed(1);
    const status = escapeHtml(op.status || 'HEMORRHAGIC_SHOCK_HEMOABDOMEN_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const afsScore = Number(p.afast_afs4_score ?? 0);
    const effusionPcv = Number(p.effusion_pcv_percent ?? 0).toFixed(1);
    const peripheralPcv = Number(p.peripheral_pcv_percent ?? 30).toFixed(1);
    const mapMmHg = Number(p.mean_arterial_pressure_mmhg ?? 65);
    const lactate = Number(p.plasma_lactate_mmol_l ?? 2.0).toFixed(1);

    const alertHtml = op.clot_popping_neoplasia_alert
      ? `
        <div class="hemoabdomen-alert-banner">
          ⚠️ CLOT-POPPING &amp; AUTOTRANSFUSION GUARD (MAP ${mapMmHg} mmHg — Target 65 mmHg · 170 µm Filter): ${escapeHtml(plan.hemoabdomen_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.hemostasis_clearance
      ? `
        <div class="hemoabdomen-clearance-banner">
          ✅ HEMOSTASIS &amp; NORMOTENSIVE CLEARANCE (${escapeHtml(op.hemostasis_clearance.vet_license || '')}): MAP ${Number(op.hemostasis_clearance.final_map_mmhg ?? 82)} mmHg · PCV ${Number(op.hemostasis_clearance.final_peripheral_pcv_percent ?? 31.0).toFixed(1)}% · Lactate ${Number(op.hemostasis_clearance.final_plasma_lactate_mmol_l ?? 1.6).toFixed(1)} mmol/L
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card hemoabdomen-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
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

        <div class="trust-kinematics-row hemoabdomen-metrics-row">
          <span class="status-pill status-pill-species">AFS: ${afsScore}/4 • Effusion PCV: ${effusionPcv}% • Peripheral PCV: ${peripheralPcv}% • MAP: ${mapMmHg} mmHg • Lactate: ${lactate} mmol/L</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary hemoabdomen-select-op-btn"
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

    const countBadge = document.getElementById('hemoabdomen-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('hemoabdomen-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="hemoabdomen-ops-empty-msg">No hemoabdomen triage operations match the current filters.</p>';
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
        ? `/api/hemoabdomen-autotransfusion/operations?${qs}`
        : '/api/hemoabdomen-autotransfusion/operations';

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
        `/api/hemoabdomen-autotransfusion/operations/${encodeURIComponent(opId)}`,
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
    const createForm = document.getElementById('create-hemoabdomen-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (
          document.getElementById('create-incident-id')?.value || 'INC-HA-9404'
        ).trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_BLUNT_TRAUMA_HEPATIC'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const afastScore = parseInt(document.getElementById('create-afast-score')?.value || '0', 10);
        const effusionPcv = parseFloat(document.getElementById('create-effusion-pcv')?.value || '0');
        const peripheralPcv = parseFloat(
          document.getElementById('create-peripheral-pcv')?.value || '0'
        );
        const mapMmHg = parseInt(document.getElementById('create-map-mmhg')?.value || '0', 10);
        const lactate = parseFloat(document.getElementById('create-lactate')?.value || '0');
        const nonClottingVerified = Boolean(
          document.getElementById('create-non-clotting-verified')?.checked
        );
        const suspectedNeoplasia = Boolean(
          document.getElementById('create-suspected-neoplasia')?.checked
        );

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            afast_afs4_score: isNaN(afastScore) ? 0 : afastScore,
            effusion_pcv_percent: isNaN(effusionPcv) ? 0 : effusionPcv,
            peripheral_pcv_percent: isNaN(peripheralPcv) ? 0 : peripheralPcv,
            mean_arterial_pressure_mmhg: isNaN(mapMmHg) ? 0 : mapMmHg,
            plasma_lactate_mmol_l: isNaN(lactate) ? 0 : lactate,
            effusion_non_clotting_verified: nonClottingVerified,
            suspected_neoplasia: suspectedNeoplasia,
          },
        };

        const btn = document.getElementById('btn-create-hemoabdomen-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/hemoabdomen-autotransfusion/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Created hemoabdomen operation ${created.id} for ${created.pet_name}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create hemoabdomen operation';
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
          species: 'CANINE_BLUNT_TRAUMA_HEPATIC',
          weight_kg: 34.0,
          afast_afs4_score: 4,
          effusion_pcv_percent: 38.0,
          peripheral_pcv_percent: 32.0,
          mean_arterial_pressure_mmhg: 48,
          plasma_lactate_mmol_l: 7.4,
          effusion_non_clotting_verified: true,
          suspected_neoplasia: false,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const species = (
          document.getElementById('stage-species')?.value || baseProfile.species
        ).trim();
        const effusionPcv = parseFloat(
          document.getElementById('stage-effusion-pcv')?.value ||
            String(baseProfile.effusion_pcv_percent)
        );
        const mapMmHg = parseInt(
          document.getElementById('stage-map-mmhg')?.value ||
            String(baseProfile.mean_arterial_pressure_mmhg),
          10
        );
        const nonClottingVerified = Boolean(
          document.getElementById('stage-non-clotting-verified')?.checked
        );
        const suspectedNeoplasia = Boolean(
          document.getElementById('stage-suspected-neoplasia')?.checked
        );

        const profilePayload = {
          ...baseProfile,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          species,
          effusion_pcv_percent: isNaN(effusionPcv) ? baseProfile.effusion_pcv_percent : effusionPcv,
          mean_arterial_pressure_mmhg: isNaN(mapMmHg)
            ? baseProfile.mean_arterial_pressure_mmhg
            : mapMmHg,
          effusion_non_clotting_verified: nonClottingVerified,
          suspected_neoplasia: suspectedNeoplasia,
        };

        const btn = document.getElementById('btn-stage-plan');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/hemoabdomen-autotransfusion/operations/${encodeURIComponent(selectedOpId)}/stage-plan`,
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
              `Staged resuscitation plan for ${updated.id} → ${updated.status}`,
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

    const checkForm = document.getElementById('hemodynamic-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const mapMmHg = parseInt(document.getElementById('check-map-mmhg')?.value || '68', 10);
        const hrBpm = parseInt(document.getElementById('check-hr-bpm')?.value || '118', 10);
        const peripheralPcv = parseFloat(
          document.getElementById('check-peripheral-pcv')?.value || '33.0'
        );
        const totalSolids = parseFloat(
          document.getElementById('check-total-solids')?.value || '5.4'
        );
        const lactate = parseFloat(document.getElementById('check-lactate')?.value || '2.3');
        const autotransfusedMl = parseFloat(
          document.getElementById('check-autotransfused-ml')?.value || '350.0'
        );
        const callsign = (
          document.getElementById('check-callsign')?.value || 'DR-TRAUMA-14'
        ).trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();

        const checkPayload = {
          mean_arterial_pressure_mmhg: isNaN(mapMmHg) ? 68 : mapMmHg,
          heart_rate_bpm: isNaN(hrBpm) ? 118 : hrBpm,
          peripheral_pcv_percent: isNaN(peripheralPcv) ? 33.0 : peripheralPcv,
          total_solids_gdl: isNaN(totalSolids) ? 5.4 : totalSolids,
          plasma_lactate_mmol_l: isNaN(lactate) ? 2.3 : lactate,
          autotransfused_volume_ml: isNaN(autotransfusedMl) ? 350.0 : autotransfusedMl,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-hemodynamic-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/hemoabdomen-autotransfusion/operations/${encodeURIComponent(selectedOpId)}/hemodynamic-check`,
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
            showToast(`Recorded hemodynamic & autotransfusion check for ${updated.id}`, 'success');
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

    const clearanceForm = document.getElementById('hemostasis-clearance-form');
    if (clearanceForm) {
      clearanceForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const finalMap = parseInt(
          document.getElementById('clearance-final-map')?.value || '84',
          10
        );
        const finalPcv = parseFloat(
          document.getElementById('clearance-final-pcv')?.value || '34.0'
        );
        const finalLactate = parseFloat(
          document.getElementById('clearance-final-lactate')?.value || '1.5'
        );
        const vetLicense = (
          document.getElementById('clearance-vet-license')?.value || 'VET-SURG-90814'
        ).trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const surgicalHemostasis = Boolean(
          document.getElementById('clearance-surgical-hemostasis')?.checked
        );
        const repeatAfastResolved = Boolean(
          document.getElementById('clearance-repeat-afast-resolved')?.checked
        );

        const clearancePayload = {
          final_map_mmhg: isNaN(finalMap) ? 84 : finalMap,
          final_peripheral_pcv_percent: isNaN(finalPcv) ? 34.0 : finalPcv,
          final_plasma_lactate_mmol_l: isNaN(finalLactate) ? 1.5 : finalLactate,
          surgical_hemostasis_confirmed: surgicalHemostasis,
          repeat_afast_free_fluid_resolved: repeatAfastResolved,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-hemostasis-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/hemoabdomen-autotransfusion/operations/${encodeURIComponent(selectedOpId)}/hemostasis-clearance`,
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
              `Issued surgical/spontaneous hemostasis clearance for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to issue hemostasis clearance';
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
    const listEl = document.getElementById('hemoabdomen-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.hemoabdomen-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.hemoabdomen-op-card');
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
