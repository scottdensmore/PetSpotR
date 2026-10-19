/**
 * PetSpotR Acute Ophthalmic Emergencies, Descemetoceles, Proptosed Globe,
 * Acute Congestive Glaucoma (IOP > 40 mmHg) & Mannitol/Latanoprost Triage Cockpit Controller (Milestone 22.2)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'OG-OP-101';

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
    const liveStatus = document.getElementById('ophthalmic-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('ophthalmic-error-banner');
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
      case 'VISION_THREATENING_IOP_DESCEMETOCELE_CRITICAL':
        return 'status-pill-critical';
      case 'LATANOPROST_MANNITOL_TARSORRHAPHY_STAGED':
        return 'status-pill-staged';
      case 'IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-globe');
    const stagedEl = document.getElementById('kpi-staged-latanoprost');
    const clearedEl = document.getElementById('kpi-cleared-iop');
    const alertsEl = document.getElementById('kpi-latanoprost-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.vision_threatening_critical_count ??
      ops.filter((op) => op.status === 'VISION_THREATENING_IOP_DESCEMETOCELE_CRITICAL').length;
    const staged =
      metrics?.latanoprost_tarsorrhaphy_staged_count ??
      ops.filter((op) => op.status === 'LATANOPROST_MANNITOL_TARSORRHAPHY_STAGED').length;
    const cleared =
      metrics?.iop_normalized_cleared_count ??
      ops.filter((op) => op.status === 'IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED').length;
    const alerts =
      metrics?.latanoprost_steroid_alert_count ??
      ops.filter((op) => Boolean(op.latanoprost_luxation_steroid_alert)).length;

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
    const iopEl = document.getElementById('stage-iop-mmhg');
    const stainEl = document.getElementById('stage-fluorescein-pattern');
    const musclesEl = document.getElementById('stage-muscles-avulsed');
    const luxationEl = document.getElementById('stage-anterior-luxation');
    const plrEl = document.getElementById('stage-consensual-plr');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (iopEl && p.intraocular_pressure_mmhg !== undefined) {
      iopEl.value = String(p.intraocular_pressure_mmhg);
    }
    if (stainEl && p.fluorescein_stain_pattern) {
      stainEl.value = p.fluorescein_stain_pattern;
    }
    if (musclesEl && p.extraocular_muscles_avulsed_count !== undefined) {
      musclesEl.value = String(p.extraocular_muscles_avulsed_count);
    }
    if (luxationEl && p.anterior_lens_luxation_present !== undefined) {
      luxationEl.checked = Boolean(p.anterior_lens_luxation_present);
    }
    if (plrEl && p.consensual_plr_present !== undefined) {
      plrEl.checked = Boolean(p.consensual_plr_present);
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('ophthalmic-alert-banner');
    const clearanceBanner = document.getElementById('ophthalmic-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const plan = active.preservation_plan || {};
        const iop = Number(p.intraocular_pressure_mmhg ?? 64);
        const stain = p.fluorescein_stain_pattern || 'FLUORESCEIN_NEGATIVE_INTACT';
        const lux = p.anterior_lens_luxation_present ? 'YES' : 'NO';
        const muscles = Number(p.extraocular_muscles_avulsed_count ?? 0);
        const pitfall =
          plan.ophthalmic_pitfall_alert ||
          'CRITICAL OPHTHALMIC GLOBE & GLAUCOMA PITFALL ALERT: NEVER administer Latanoprost (0.005%) when anterior lens luxation is present or in feline uveitic glaucoma; NEVER apply topical corticosteroids or petroleum-based ointments to deep/melting corneal ulcers or perforated descemetoceles; and NEVER perform manual jugular compression or neck-lead restraint (spikes IOP by 15–25 mmHg and ruptures fragile descemetoceles).';
        alertBanner.textContent = `⚠️ OPHTHALMIC GLOBE & GLAUCOMA GUARD (${active.id} — ${active.pet_name} · IOP ${iop} mmHg · Stain ${stain} · Lens Luxation ${lux} · Avulsed Muscles ${muscles}): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.discharge_clearance) {
        const clr = active.discharge_clearance;
        const finalIop = Number(clr.final_iop_mmhg ?? 16);
        clearanceBanner.textContent = `✅ IOP NORMALIZED & GLOBE PROTECTED CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): Final IOP ${finalIop} mmHg · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
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
        '<p class="trust-empty-hint">No active Ophthalmic &amp; Globe Preservation operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const plan = active.preservation_plan;
    const checks = Array.isArray(active.tonometry_checks) ? active.tonometry_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_PRIMARY_CONGESTIVE_GLAUCOMA');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 12.5).toFixed(1);
    const status = escapeHtml(active.status || 'VISION_THREATENING_IOP_DESCEMETOCELE_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const iop = Number(p.intraocular_pressure_mmhg ?? 64);
    const stain = escapeHtml(p.fluorescein_stain_pattern || 'FLUORESCEIN_NEGATIVE_INTACT');
    const luxText = p.anterior_lens_luxation_present
      ? 'YES (Latanoprost Contraindicated)'
      : 'NO';
    const muscles = Number(p.extraocular_muscles_avulsed_count ?? 0);
    const plrText = p.consensual_plr_present ? 'PRESENT' : 'ABSENT';

    const planHtml = plan
      ? `
        <div class="ophthalmic-plan-box" id="workspace-plan-box">
          <h4>Calculated Globe Preservation, Osmotic Mannitol IV, Latanoprost Eligibility &amp; Anticollagenase Plan</h4>
          <div class="ophthalmic-doses-grid">
            <div><strong>Latanoprost 0.005% Allowed:</strong> <span id="plan-latanoprost">${plan.latanoprost_0005_pct_allowed ? 'ALLOWED (1 drop)' : 'CONTRAINDICATED'}</span></div>
            <div><strong>Dorzolamide/Timolol Drops:</strong> <span id="plan-dorzolamide">${escapeHtml(plan.dorzolamide_timolol_drops_freq || 'q8h')}</span></div>
            <div><strong>Mannitol 1.0 g/kg Slow IV:</strong> <span id="plan-mannitol">${Number(plan.mannitol_1_g_kg_iv_grams ?? 12.5).toFixed(1)} g IV</span></div>
            <div><strong>Autologous Serum q1h:</strong> <span id="plan-serum">${plan.autologous_serum_drops_q1h ? 'INDICATED (q1h)' : 'Not Indicated'}</span></div>
            <div><strong>Temporary Tarsorrhaphy:</strong> <span id="plan-tarsorrhaphy">${plan.temporary_tarsorrhaphy_indicated ? 'INDICATED (&lt; 3 EOMs)' : 'Not Indicated'}</span></div>
            <div><strong>Zero Steroid / Neck-Lead Guard:</strong> <span id="plan-steroid-guard">${plan.zero_topical_steroid_ointment_neck_lead_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="ophthalmic-rationale-text" id="workspace-clinical-rationale">${escapeHtml(plan.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-OPHTHO');
        const cIop = Number(chk.intraocular_pressure_mmhg ?? 18);
        const cSeidel = chk.seidel_test_negative ? 'YES' : 'NO';
        const cDazzle = chk.dazzle_reflex_present ? 'YES' : 'NO';
        const cEcollar = chk.rigid_e_collar_verified ? 'YES' : 'NO';
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="ophthalmic-check-item"><strong>${callsign}</strong> · IOP ${cIop} mmHg · Seidel Neg: ${cSeidel} · Dazzle: ${cDazzle} · Rigid E-Collar: ${cEcollar} — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="ophthalmic-workspace-card" data-workspace-op-id="${opId}">
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

        <div class="ophthalmic-vitals-summary" id="workspace-vitals-summary">
          <span><strong>IOP:</strong> ${iop} mmHg</span>
          <span><strong>Fluorescein:</strong> ${stain}</span>
          <span><strong>Anterior Lens Luxation:</strong> ${luxText}</span>
          <span><strong>Avulsed EOMs:</strong> ${muscles}</span>
          <span><strong>Consensual PLR:</strong> ${plrText}</span>
        </div>

        ${planHtml}

        <div class="ophthalmic-tonometry-checks-box">
          <h4>Serial Rebound Tonometry (IOP), Seidel Test &amp; Dazzle Reflex Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="tonometry-checks-list" class="ophthalmic-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const plan = op.preservation_plan || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_PRIMARY_CONGESTIVE_GLAUCOMA');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 12.5).toFixed(1);
    const status = escapeHtml(op.status || 'VISION_THREATENING_IOP_DESCEMETOCELE_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const iop = Number(p.intraocular_pressure_mmhg ?? 64);
    const stain = escapeHtml(p.fluorescein_stain_pattern || 'FLUORESCEIN_NEGATIVE_INTACT');
    const luxYesNo = p.anterior_lens_luxation_present ? 'YES' : 'NO';
    const muscles = Number(p.extraocular_muscles_avulsed_count ?? 0);
    const plrYesNo = p.consensual_plr_present ? 'YES' : 'NO';

    const alertHtml = op.latanoprost_luxation_steroid_alert
      ? `
        <div class="ophthalmic-alert-banner">
          ⚠️ LATANOPROST, LENS LUXATION &amp; CORNEAL STEROID GUARD (IOP ${iop} mmHg · Stain ${stain}): ${escapeHtml(plan.ophthalmic_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.discharge_clearance
      ? `
        <div class="ophthalmic-clearance-banner">
          ✅ IOP NORMALIZED &amp; GLOBE PROTECTED (${escapeHtml(op.discharge_clearance.vet_license || '')}): Final IOP ${Number(op.discharge_clearance.final_iop_mmhg ?? 16)} mmHg · Hard E-Collar &amp; 5-Min Drop Spacing Verified
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card ophthalmic-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
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

        <div class="trust-kinematics-row ophthalmic-metrics-row">
          <span class="status-pill status-pill-species">IOP: ${iop} mmHg • Stain: ${stain} • Lens Luxation: ${luxYesNo} • Avulsed EOMs: ${muscles} • Consensual PLR: ${plrYesNo}</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary ophthalmic-select-op-btn"
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

    const countBadge = document.getElementById('ophthalmic-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('ophthalmic-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="ophthalmic-ops-empty-msg">No Ophthalmic &amp; Globe Preservation operations match the current filters.</p>';
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
        ? `/api/ophthalmic-globe/operations?${qs}`
        : '/api/ophthalmic-globe/operations';

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
      const res = await fetch(`/api/ophthalmic-globe/operations/${encodeURIComponent(opId)}`, {
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
    const createForm = document.getElementById('create-ophthalmic-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (
          document.getElementById('create-incident-id')?.value || 'INC-OG-9954'
        ).trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_PRIMARY_CONGESTIVE_GLAUCOMA'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const iopMmHg = parseInt(document.getElementById('create-iop-mmhg')?.value || '0', 10);
        const stainPattern = (
          document.getElementById('create-fluorescein-pattern')?.value ||
          'FLUORESCEIN_NEGATIVE_INTACT'
        ).trim();
        const musclesAvulsed = parseInt(
          document.getElementById('create-muscles-avulsed')?.value || '0',
          10
        );
        const anteriorLuxation = Boolean(
          document.getElementById('create-anterior-luxation')?.checked
        );
        const consensualPlr = Boolean(document.getElementById('create-consensual-plr')?.checked);

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            intraocular_pressure_mmhg: isNaN(iopMmHg) ? 0 : iopMmHg,
            fluorescein_stain_pattern: stainPattern,
            anterior_lens_luxation_present: anteriorLuxation,
            extraocular_muscles_avulsed_count: isNaN(musclesAvulsed) ? 0 : musclesAvulsed,
            consensual_plr_present: consensualPlr,
          },
        };

        const btn = document.getElementById('btn-create-ophthalmic-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/ophthalmic-globe/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Created Ophthalmic Globe operation ${created.id} for ${created.pet_name}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create Ophthalmic Globe operation';
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
          species: 'CANINE_PRIMARY_CONGESTIVE_GLAUCOMA',
          weight_kg: 12.5,
          intraocular_pressure_mmhg: 64,
          fluorescein_stain_pattern: 'FLUORESCEIN_NEGATIVE_INTACT',
          anterior_lens_luxation_present: true,
          extraocular_muscles_avulsed_count: 0,
          consensual_plr_present: true,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const species = (
          document.getElementById('stage-species')?.value || baseProfile.species
        ).trim();
        const iopMmHg = parseInt(
          document.getElementById('stage-iop-mmhg')?.value ||
            String(baseProfile.intraocular_pressure_mmhg),
          10
        );
        const stainPattern = (
          document.getElementById('stage-fluorescein-pattern')?.value ||
          baseProfile.fluorescein_stain_pattern
        ).trim();
        const musclesAvulsed = parseInt(
          document.getElementById('stage-muscles-avulsed')?.value ||
            String(baseProfile.extraocular_muscles_avulsed_count),
          10
        );
        const anteriorLuxation = Boolean(
          document.getElementById('stage-anterior-luxation')?.checked
        );
        const consensualPlr = Boolean(document.getElementById('stage-consensual-plr')?.checked);

        const profilePayload = {
          ...baseProfile,
          species,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          intraocular_pressure_mmhg: isNaN(iopMmHg)
            ? baseProfile.intraocular_pressure_mmhg
            : iopMmHg,
          fluorescein_stain_pattern: stainPattern,
          anterior_lens_luxation_present: anteriorLuxation,
          extraocular_muscles_avulsed_count: isNaN(musclesAvulsed)
            ? baseProfile.extraocular_muscles_avulsed_count
            : musclesAvulsed,
          consensual_plr_present: consensualPlr,
        };

        const btn = document.getElementById('btn-stage-plan');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/ophthalmic-globe/operations/${encodeURIComponent(selectedOpId)}/stage-plan`,
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
              `Staged Globe Preservation & IOP Lowering plan for ${updated.id} → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage preservation plan';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('tonometry-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const iopMmHg = parseInt(document.getElementById('check-iop-mmhg')?.value || '18', 10);
        const callsign = (
          document.getElementById('check-callsign')?.value || 'DR-OPHTHO-11'
        ).trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();
        const seidelNeg = Boolean(document.getElementById('check-seidel-negative')?.checked);
        const dazzlePresent = Boolean(document.getElementById('check-dazzle-present')?.checked);
        const ecollarVerified = Boolean(document.getElementById('check-ecollar-verified')?.checked);

        const checkPayload = {
          intraocular_pressure_mmhg: isNaN(iopMmHg) ? 18 : iopMmHg,
          seidel_test_negative: seidelNeg,
          dazzle_reflex_present: dazzlePresent,
          rigid_e_collar_verified: ecollarVerified,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-tonometry-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/ophthalmic-globe/operations/${encodeURIComponent(selectedOpId)}/tonometry-check`,
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
            showToast(`Recorded tonometry, Seidel & dazzle check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record tonometry check';
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

        const finalIop = parseInt(
          document.getElementById('clearance-final-iop')?.value || '16',
          10
        );
        const vetLicense = (
          document.getElementById('clearance-vet-license')?.value || 'VET-OPHTHO-99508'
        ).trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const tectonicIntegrity = Boolean(
          document.getElementById('clearance-tectonic-integrity')?.checked
        );
        const ecollarFitted = Boolean(
          document.getElementById('clearance-ecollar-fitted')?.checked
        );
        const dropSpacingEducated = Boolean(
          document.getElementById('clearance-drop-spacing-educated')?.checked
        );

        const clearancePayload = {
          final_iop_mmhg: isNaN(finalIop) ? 16 : finalIop,
          globe_tectonic_integrity_confirmed: tectonicIntegrity,
          hard_plastic_e_collar_fitted: ecollarFitted,
          topical_drop_5_min_spacing_educated: dropSpacingEducated,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-discharge-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/ophthalmic-globe/operations/${encodeURIComponent(selectedOpId)}/discharge-clearance`,
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
              `Issued IOP normalized & globe protected clearance for ${updated.id}`,
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
    const listEl = document.getElementById('ophthalmic-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.ophthalmic-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.ophthalmic-op-card');
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
