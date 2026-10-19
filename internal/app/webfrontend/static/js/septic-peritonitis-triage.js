/**
 * PetSpotR Septic Peritonitis, Abdominocentesis Blood-Fluid Glucose/Lactate Gradient
 * & Closed-Suction Jackson-Pratt Drain Cockpit Controller (Milestone 20.3)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'SP-OP-101';

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
    const liveStatus = document.getElementById('peritonitis-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('peritonitis-error-banner');
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
      case 'SEPTIC_EFFUSION_INTRACELLULAR_BACTERIA_CRITICAL':
        return 'status-pill-critical';
      case 'EMPIRIC_ANTIBIOTIC_LAVAGE_JP_DRAIN_STAGED':
        return 'status-pill-staged';
      case 'NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-septic');
    const stagedEl = document.getElementById('kpi-staged-lavage');
    const clearedEl = document.getElementById('kpi-cleared-jp-removed');
    const alertsEl = document.getElementById('kpi-septic-gradient-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.septic_effusion_critical_count ??
      ops.filter((op) => op.status === 'SEPTIC_EFFUSION_INTRACELLULAR_BACTERIA_CRITICAL').length;
    const staged =
      metrics?.antibiotic_lavage_jp_staged_count ??
      ops.filter((op) => op.status === 'EMPIRIC_ANTIBIOTIC_LAVAGE_JP_DRAIN_STAGED').length;
    const cleared =
      metrics?.non_septic_cleared_count ??
      ops.filter((op) => op.status === 'NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED').length;
    const alerts =
      metrics?.septic_gradient_alert_count ??
      ops.filter((op) => Boolean(op.septic_gradient_antibiotic_delay_alert)).length;

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
    const bgEl = document.getElementById('stage-blood-glucose');
    const effBgEl = document.getElementById('stage-effusion-glucose');
    const lacEl = document.getElementById('stage-blood-lactate');
    const effLacEl = document.getElementById('stage-effusion-lactate');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (bgEl && p.blood_glucose_mg_dl !== undefined) bgEl.value = String(p.blood_glucose_mg_dl);
    if (effBgEl && p.effusion_glucose_mg_dl !== undefined) {
      effBgEl.value = String(p.effusion_glucose_mg_dl);
    }
    if (lacEl && p.blood_lactate_mmol_l !== undefined) lacEl.value = String(p.blood_lactate_mmol_l);
    if (effLacEl && p.effusion_lactate_mmol_l !== undefined) {
      effLacEl.value = String(p.effusion_lactate_mmol_l);
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('peritonitis-alert-banner');
    const clearanceBanner = document.getElementById('peritonitis-clearance-banner');

    if (alertBanner) {
      if (active) {
        const plan = active.surgical_plan || {};
        const glGrad = Number(plan.blood_fluid_glucose_gradient_mg_dl ?? 0).toFixed(1);
        const lacGrad = Number(plan.fluid_blood_lactate_gradient_mmol_l ?? 0).toFixed(2);
        const lavLiters = Number(plan.warm_sterile_saline_lavage_liters ?? 0).toFixed(2);
        const pitfall =
          plan.peritonitis_pitfall_alert ||
          'CRITICAL SEPTIC PERITONITIS PITFALL ALERT: NEVER delay broad-spectrum IV antibiotics beyond 1 hour (Surviving Sepsis Campaign); NEVER add chlorhexidine or povidone-iodine to abdominal lavage fluid (causes fatal chemical peritonitis); and ALWAYS aspirate all lavage fluid before closing with Jackson-Pratt closed-suction drains.';
        alertBanner.textContent = `⚠️ SEPTIC GRADIENT & ANTISEPTIC LAVAGE GUARD (${active.id} — ${active.pet_name} · Glucose Gradient ${glGrad} mg/dL · Lactate Gradient ${lacGrad} mmol/L · Lavage ${lavLiters} L 0.9% NaCl): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.drain_clearance) {
        const clr = active.drain_clearance;
        const output = Number(clr.final_drain_output_ml_kg_day ?? 1.5).toFixed(1);
        const alb = Number(clr.final_serum_albumin_g_dl ?? 2.5).toFixed(1);
        clearanceBanner.textContent = `✅ NON-SEPTIC CYTOLOGY & JP DRAIN REMOVAL CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): Final Drain Output ${output} mL/kg/day · Serum Albumin ${alb} g/dL · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
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
        '<p class="trust-empty-hint">No active septic peritonitis triage operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const plan = active.surgical_plan;
    const checks = Array.isArray(active.drain_checks) ? active.drain_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_GI_DEHISCENCE_PERFORATION');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 25.0).toFixed(1);
    const status = escapeHtml(active.status || 'SEPTIC_EFFUSION_INTRACELLULAR_BACTERIA_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const bgMgDl = Number(p.blood_glucose_mg_dl ?? 110.0).toFixed(1);
    const effBgMgDl = Number(p.effusion_glucose_mg_dl ?? 50.0).toFixed(1);
    const lacMmol = Number(p.blood_lactate_mmol_l ?? 2.0).toFixed(1);
    const effLacMmol = Number(p.effusion_lactate_mmol_l ?? 5.0).toFixed(1);
    const neutrophils = Number(p.degenerate_neutrophils_count_ul ?? 0);
    const bacteriaSeen = p.intracellular_bacteria_seen ? 'POSITIVE' : 'NEGATIVE';

    const planHtml = plan
      ? `
        <div class="peritonitis-plan-box" id="workspace-plan-box">
          <h4>Calculated Abdominocentesis Gradients, Empiric IV Antibiotics, Lavage &amp; JP Drain Plan</h4>
          <div class="peritonitis-doses-grid">
            <div><strong>Blood-Fluid Glucose Gradient (&gt;20):</strong> <span id="plan-glucose-gradient">${Number(plan.blood_fluid_glucose_gradient_mg_dl ?? 0).toFixed(1)} mg/dL</span></div>
            <div><strong>Fluid-Blood Lactate Gradient (&gt;2.0):</strong> <span id="plan-lactate-gradient">${Number(plan.fluid_blood_lactate_gradient_mmol_l ?? 0).toFixed(2)} mmol/L</span></div>
            <div><strong>Ampicillin/Sulbactam (30 mg/kg IV):</strong> <span id="plan-amp-sulbactam-mg">${Number(plan.ampicillin_sulbactam_30mg_kg_mg ?? 0).toFixed(1)} mg</span></div>
            <div><strong>Enrofloxacin IV Dose:</strong> <span id="plan-enrofloxacin-mg">${Number(plan.enrofloxacin_10mg_kg_dose_mg ?? 0).toFixed(1)} mg</span></div>
            <div><strong>Warm 0.9% NaCl Lavage (300 mL/kg):</strong> <span id="plan-lavage-liters">${Number(plan.warm_sterile_saline_lavage_liters ?? 0).toFixed(2)} L</span></div>
            <div><strong>Closed-Suction JP Drains:</strong> <span id="plan-jp-drains">${Number(plan.jackson_pratt_closed_suction_drains ?? 1)}</span></div>
            <div><strong>Zero Antiseptic Lavage Guard:</strong> <span id="plan-antiseptic-guard">${plan.zero_antiseptic_lavage_addition_guard ? 'ENFORCED (NO CHLORHEXIDINE/IODINE)' : 'OFF'}</span></div>
          </div>
          <p class="peritonitis-rationale-text" id="workspace-clinical-rationale">${escapeHtml(plan.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-SURG');
        const cOut = Number(chk.effusion_production_ml_kg_day ?? 2.0).toFixed(1);
        const cAlb = Number(chk.serum_albumin_g_dl ?? 2.4).toFixed(1);
        const cMap = Number(chk.mean_arterial_pressure_mm_hg ?? 80);
        const cBact = chk.cytology_intracellular_bacteria ? 'BACTERIA POSITIVE' : 'BACTERIA NEGATIVE';
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="peritonitis-check-item"><strong>${callsign}</strong> · Output ${cOut} mL/kg/day · Albumin ${cAlb} g/dL · MAP ${cMap} mmHg · ${cBact} — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="peritonitis-workspace-card" data-workspace-op-id="${opId}">
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

        <div class="peritonitis-vitals-summary" id="workspace-vitals-summary">
          <span><strong>Blood/Fluid BG:</strong> ${bgMgDl} / ${effBgMgDl} mg/dL</span>
          <span><strong>Blood/Fluid Lactate:</strong> ${lacMmol} / ${effLacMmol} mmol/L</span>
          <span><strong>Degenerate Neutrophils:</strong> ${neutrophils}/µL</span>
          <span><strong>Intracellular Bacteria:</strong> ${bacteriaSeen}</span>
        </div>

        ${planHtml}

        <div class="peritonitis-drain-checks-box">
          <h4>Serial Jackson-Pratt Drain Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="drain-checks-list" class="peritonitis-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const plan = op.surgical_plan || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_GI_DEHISCENCE_PERFORATION');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 25.0).toFixed(1);
    const status = escapeHtml(op.status || 'SEPTIC_EFFUSION_INTRACELLULAR_BACTERIA_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const bgMgDl = Number(p.blood_glucose_mg_dl ?? 110.0).toFixed(1);
    const effBgMgDl = Number(p.effusion_glucose_mg_dl ?? 50.0).toFixed(1);
    const lacMmol = Number(p.blood_lactate_mmol_l ?? 2.0).toFixed(1);
    const effLacMmol = Number(p.effusion_lactate_mmol_l ?? 5.0).toFixed(1);
    const neutrophils = Number(p.degenerate_neutrophils_count_ul ?? 0);
    const glGrad = Number(plan.blood_fluid_glucose_gradient_mg_dl ?? 0).toFixed(1);
    const lacGrad = Number(plan.fluid_blood_lactate_gradient_mmol_l ?? 0).toFixed(2);

    const alertHtml = op.septic_gradient_antibiotic_delay_alert
      ? `
        <div class="peritonitis-alert-banner">
          ⚠️ SEPTIC GRADIENT &amp; 1-HOUR ANTIBIOTIC ALERT (ΔBG ${glGrad} mg/dL · ΔLactate ${lacGrad} mmol/L): ${escapeHtml(plan.peritonitis_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.drain_clearance
      ? `
        <div class="peritonitis-clearance-banner">
          ✅ NON-SEPTIC CYTOLOGY JP DRAIN CLEARANCE (${escapeHtml(op.drain_clearance.vet_license || '')}): Output ${Number(op.drain_clearance.final_drain_output_ml_kg_day ?? 1.5).toFixed(1)} mL/kg/day · Albumin ${Number(op.drain_clearance.final_serum_albumin_g_dl ?? 2.5).toFixed(1)} g/dL
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card peritonitis-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
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

        <div class="trust-kinematics-row peritonitis-metrics-row">
          <span class="status-pill status-pill-species">BG: ${bgMgDl}/${effBgMgDl} mg/dL • Lactate: ${lacMmol}/${effLacMmol} mmol/L • Neutrophils: ${neutrophils}/µL</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary peritonitis-select-op-btn"
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

    const countBadge = document.getElementById('peritonitis-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('peritonitis-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="peritonitis-ops-empty-msg">No septic peritonitis triage operations match the current filters.</p>';
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
        ? `/api/septic-peritonitis/operations?${qs}`
        : '/api/septic-peritonitis/operations';

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
      const res = await fetch(`/api/septic-peritonitis/operations/${encodeURIComponent(opId)}`, {
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
    const createForm = document.getElementById('create-peritonitis-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (document.getElementById('create-incident-id')?.value || 'INC-SP-9504').trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_GI_DEHISCENCE_PERFORATION'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const bloodGlucose = parseFloat(document.getElementById('create-blood-glucose')?.value || '0');
        const effusionGlucose = parseFloat(document.getElementById('create-effusion-glucose')?.value || '0');
        const bloodLactate = parseFloat(document.getElementById('create-blood-lactate')?.value || '0');
        const effusionLactate = parseFloat(document.getElementById('create-effusion-lactate')?.value || '0');
        const degenerateNeutrophils = parseInt(
          document.getElementById('create-degenerate-neutrophils')?.value || '0',
          10
        );
        const intracellularBacteria = Boolean(
          document.getElementById('create-intracellular-bacteria')?.checked
        );

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            blood_glucose_mg_dl: isNaN(bloodGlucose) ? 0 : bloodGlucose,
            effusion_glucose_mg_dl: isNaN(effusionGlucose) ? 0 : effusionGlucose,
            blood_lactate_mmol_l: isNaN(bloodLactate) ? 0 : bloodLactate,
            effusion_lactate_mmol_l: isNaN(effusionLactate) ? 0 : effusionLactate,
            degenerate_neutrophils_count_ul: isNaN(degenerateNeutrophils) ? 0 : degenerateNeutrophils,
            intracellular_bacteria_seen: intracellularBacteria,
          },
        };

        const btn = document.getElementById('btn-create-peritonitis-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/septic-peritonitis/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(`Created septic peritonitis operation ${created.id} for ${created.pet_name}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create septic peritonitis operation';
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
          species: 'CANINE_GI_DEHISCENCE_PERFORATION',
          weight_kg: 31.0,
          blood_glucose_mg_dl: 118.0,
          effusion_glucose_mg_dl: 42.0,
          blood_lactate_mmol_l: 2.4,
          effusion_lactate_mmol_l: 6.8,
          degenerate_neutrophils_count_ul: 48000,
          intracellular_bacteria_seen: true,
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
        const effusionGlucose = parseFloat(
          document.getElementById('stage-effusion-glucose')?.value || String(baseProfile.effusion_glucose_mg_dl)
        );
        const bloodLactate = parseFloat(
          document.getElementById('stage-blood-lactate')?.value || String(baseProfile.blood_lactate_mmol_l)
        );
        const effusionLactate = parseFloat(
          document.getElementById('stage-effusion-lactate')?.value || String(baseProfile.effusion_lactate_mmol_l)
        );

        const profilePayload = {
          ...baseProfile,
          species,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          blood_glucose_mg_dl: isNaN(bloodGlucose) ? baseProfile.blood_glucose_mg_dl : bloodGlucose,
          effusion_glucose_mg_dl: isNaN(effusionGlucose) ? baseProfile.effusion_glucose_mg_dl : effusionGlucose,
          blood_lactate_mmol_l: isNaN(bloodLactate) ? baseProfile.blood_lactate_mmol_l : bloodLactate,
          effusion_lactate_mmol_l: isNaN(effusionLactate) ? baseProfile.effusion_lactate_mmol_l : effusionLactate,
        };

        const btn = document.getElementById('btn-stage-plan');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/septic-peritonitis/operations/${encodeURIComponent(selectedOpId)}/stage-plan`,
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
            showToast(`Staged surgical plan for ${updated.id} → ${updated.status}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage surgical plan';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('drain-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const effusionProduction = parseFloat(
          document.getElementById('check-effusion-production')?.value || '1.8'
        );
        const serumAlbumin = parseFloat(
          document.getElementById('check-serum-albumin')?.value || '2.4'
        );
        const mapMmHg = parseInt(document.getElementById('check-map')?.value || '82', 10);
        const callsign = (document.getElementById('check-callsign')?.value || 'DR-SURG-14').trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();
        const intracellularBacteria = Boolean(
          document.getElementById('check-intracellular-bacteria')?.checked
        );
        const degenerateNeutrophils = Boolean(
          document.getElementById('check-degenerate-neutrophils')?.checked
        );

        const checkPayload = {
          effusion_production_ml_kg_day: isNaN(effusionProduction) ? 1.8 : effusionProduction,
          cytology_intracellular_bacteria: intracellularBacteria,
          degenerate_neutrophils_present: degenerateNeutrophils,
          serum_albumin_g_dl: isNaN(serumAlbumin) ? 2.4 : serumAlbumin,
          mean_arterial_pressure_mm_hg: isNaN(mapMmHg) ? 82 : mapMmHg,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-drain-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/septic-peritonitis/operations/${encodeURIComponent(selectedOpId)}/drain-check`,
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
            showToast(`Recorded Jackson-Pratt drain check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record Jackson-Pratt drain check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearanceForm = document.getElementById('drain-clearance-form');
    if (clearanceForm) {
      clearanceForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const finalOutput = parseFloat(
          document.getElementById('clearance-final-output')?.value || '1.5'
        );
        const finalAlbumin = parseFloat(
          document.getElementById('clearance-final-albumin')?.value || '2.5'
        );
        const vetLicense = (
          document.getElementById('clearance-vet-license')?.value || 'VET-SURG-90512'
        ).trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const bacteriaNegative = Boolean(
          document.getElementById('clearance-bacteria-negative')?.checked
        );
        const enteralNutrition = Boolean(
          document.getElementById('clearance-enteral-nutrition')?.checked
        );
        const jpRemoved = Boolean(document.getElementById('clearance-jp-removed')?.checked);

        const clearancePayload = {
          final_drain_output_ml_kg_day: isNaN(finalOutput) ? 1.5 : finalOutput,
          final_cytology_bacteria_negative: bacteriaNegative,
          final_serum_albumin_g_dl: isNaN(finalAlbumin) ? 2.5 : finalAlbumin,
          enteral_nutrition_tolerated: enteralNutrition,
          jp_drain_aseptically_removed: jpRemoved,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-drain-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/septic-peritonitis/operations/${encodeURIComponent(selectedOpId)}/drain-clearance`,
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
            showToast(`Issued non-septic JP drain removal clearance for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to issue drain removal clearance';
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
    const listEl = document.getElementById('peritonitis-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.peritonitis-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.peritonitis-op-card');
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
