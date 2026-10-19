/**
 * PetSpotR Cardiopulmonary Resuscitation (RECOVER Guidelines),
 * Open/Closed-Chest Compression, EtCO2 > 15 mmHg & Defibrillation Cockpit Controller (Milestone 22.1)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'CPR-OP-101';

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
    const liveStatus = document.getElementById('cpr-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('cpr-error-banner');
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
      case 'CARDIOPULMONARY_ARREST_BLS_ALS_CRITICAL':
        return 'status-pill-critical';
      case 'RECOVER_2MIN_CYCLE_EPI_DEFIB_STAGED':
        return 'status-pill-staged';
      case 'ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-arrest');
    const stagedEl = document.getElementById('kpi-staged-cycle');
    const clearedEl = document.getElementById('kpi-cleared-rosc');
    const alertsEl = document.getElementById('kpi-pitfall-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.arrest_in_progress_critical_count ??
      ops.filter((op) => op.status === 'CARDIOPULMONARY_ARREST_BLS_ALS_CRITICAL').length;
    const staged =
      metrics?.epi_defib_cycle_staged_count ??
      ops.filter((op) => op.status === 'RECOVER_2MIN_CYCLE_EPI_DEFIB_STAGED').length;
    const cleared =
      metrics?.rosc_post_arrest_cleared_count ??
      ops.filter((op) => op.status === 'ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE').length;
    const alerts =
      metrics?.hyperventilation_high_dose_alert_count ??
      ops.filter((op) => Boolean(op.hyperventilation_high_dose_epi_alert)).length;

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
    const rhythmEl = document.getElementById('stage-arrest-rhythm');
    const etco2El = document.getElementById('stage-etco2-mmhg');
    const cyclesEl = document.getElementById('stage-cycles-completed');
    const revEl = document.getElementById('stage-reversible-present');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (rhythmEl && p.arrest_rhythm) rhythmEl.value = p.arrest_rhythm;
    if (etco2El && p.end_tidal_co2_mmhg !== undefined) {
      etco2El.value = String(p.end_tidal_co2_mmhg);
    }
    if (cyclesEl && p.compression_cycles_completed !== undefined) {
      cyclesEl.value = String(p.compression_cycles_completed);
    }
    if (revEl && p.reversible_opioid_alpha2_present !== undefined) {
      revEl.checked = Boolean(p.reversible_opioid_alpha2_present);
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('cpr-alert-banner');
    const clearanceBanner = document.getElementById('cpr-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const plan = active.als_protocol_plan || {};
        const rhythm = p.arrest_rhythm || 'VFIB_PULSELESS_VT_SHOCKABLE';
        const etco2 = Number(p.end_tidal_co2_mmhg ?? 11.0).toFixed(1);
        const cycles = Number(p.compression_cycles_completed ?? 1);
        const hand =
          plan.hand_placement_technique ||
          'Directly Over Heart (4th-6th ICS) Lateral Recumbency';
        const pitfall =
          plan.recover_pitfall_alert ||
          'CRITICAL RECOVER 2012/2024 BLS/ALS PITFALL ALERT: NEVER hyperventilate arrest patients > 10 breaths/min (increases intrathoracic pressure, abolishes venous return, and causes cerebral vasoconstriction); NEVER administer Epinephrine every 2-minute cycle (give 0.01 mg/kg IV/IO EVERY OTHER cycle q3–5 min); and NEVER use routine High-Dose Epinephrine (0.1 mg/kg).';
        alertBanner.textContent = `⚠️ RECOVER 2012/2024 BLS/ALS GUARD (${active.id} — ${active.pet_name} · ${rhythm} · EtCO2 ${etco2} mmHg · Cycle ${cycles} · ${hand}): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.rosc_clearance) {
        const clr = active.rosc_clearance;
        const mins = Number(clr.sustained_rosc_minutes ?? 30);
        const finalEtco2 = Number(clr.final_etco2_mmhg ?? 39.0).toFixed(1);
        const finalMap = Number(clr.final_map_mmhg ?? 92);
        clearanceBanner.textContent = `✅ ROSC ACHIEVED & NORMOCAPNIC PCA CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): Sustained ROSC ${mins} min · Final EtCO2 ${finalEtco2} mmHg · Final MAP ${finalMap} mmHg · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
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
        '<p class="trust-empty-hint">No active RECOVER CPR operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const plan = active.als_protocol_plan;
    const checks = Array.isArray(active.cycle_checks) ? active.cycle_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_KEEL_CHEST_THORACIC_PUMP');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 30.0).toFixed(1);
    const status = escapeHtml(active.status || 'CARDIOPULMONARY_ARREST_BLS_ALS_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const rhythm = escapeHtml(p.arrest_rhythm || 'VFIB_PULSELESS_VT_SHOCKABLE');
    const etco2 = Number(p.end_tidal_co2_mmhg ?? 11.0).toFixed(1);
    const cycles = Number(p.compression_cycles_completed ?? 1);
    const revText = p.reversible_opioid_alpha2_present
      ? 'YES (Naloxone/Atipamezole)'
      : 'NO';

    const planHtml = plan
      ? `
        <div class="cpr-plan-box" id="workspace-plan-box">
          <h4>Calculated RECOVER BLS/ALS Compression, Ventilation, Low-Dose Epinephrine &amp; Defibrillation Plan</h4>
          <div class="cpr-doses-grid">
            <div><strong>Hand Placement:</strong> <span id="plan-hand-placement">${escapeHtml(plan.hand_placement_technique || '')}</span></div>
            <div><strong>Target Compression Rate:</strong> <span id="plan-compression-rate">${Number(plan.target_compression_rate_bpm ?? 110)} bpm</span></div>
            <div><strong>Target Ventilation Rate:</strong> <span id="plan-ventilation-rate">${Number(plan.target_ventilation_rate_bpm ?? 10)} breaths/min</span></div>
            <div><strong>Low-Dose Epinephrine (0.01 mg/kg):</strong> <span id="plan-epi-low-dose">${Number(plan.low_dose_epinephrine_001_mg_kg_mg ?? 0.30).toFixed(2)} mg IV/IO</span></div>
            <div><strong>Atropine (0.04 mg/kg):</strong> <span id="plan-atropine-dose">${Number(plan.atropine_004_mg_kg_dose_mg ?? 1.20).toFixed(2)} mg IV/IO</span></div>
            <div><strong>Biphasic Defibrillation:</strong> <span id="plan-biphasic-defib">${Number(plan.biphasic_defib_joules ?? 60.0).toFixed(1)} J</span></div>
            <div><strong>Amiodarone (5.0 mg/kg):</strong> <span id="plan-amiodarone-dose">${Number(plan.amiodarone_5_mg_kg_dose_mg ?? 150.0).toFixed(1)} mg IV/IO</span></div>
            <div><strong>Anti-Hyperventilation / q3–5m Epi Guard:</strong> <span id="plan-epi-vent-guard">${plan.zero_hyperventilation_every_other_cycle_epi_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="cpr-rationale-text" id="workspace-clinical-rationale">${escapeHtml(plan.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-RECOVER');
        const cNum = Number(chk.cycle_number ?? 1);
        const cEtco2 = Number(chk.end_tidal_co2_mmhg ?? 18.0).toFixed(1);
        const cRhythm = escapeHtml(chk.rhythm_at_2_min_pause || '');
        const cPause = Number(chk.pause_duration_seconds ?? 6);
        const cRot = chk.compressor_rotated ? 'YES' : 'NO';
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="cpr-check-item"><strong>Cycle #${cNum} (${callsign})</strong> · EtCO2 ${cEtco2} mmHg · Rhythm ${cRhythm} · Pause ${cPause}s · Rotated: ${cRot} — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="cpr-workspace-card" data-workspace-op-id="${opId}">
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

        <div class="cpr-vitals-summary" id="workspace-vitals-summary">
          <span><strong>Arrest Rhythm:</strong> ${rhythm}</span>
          <span><strong>EtCO2:</strong> ${etco2} mmHg</span>
          <span><strong>2-Min Cycles:</strong> ${cycles}</span>
          <span><strong>Reversible Agent:</strong> ${revText}</span>
        </div>

        ${planHtml}

        <div class="cpr-cycle-checks-box">
          <h4>2-Minute BLS/ALS Compression &amp; Capnography EtCO2 Cycle Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="cycle-checks-list" class="cpr-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const plan = op.als_protocol_plan || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_KEEL_CHEST_THORACIC_PUMP');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 30.0).toFixed(1);
    const status = escapeHtml(op.status || 'CARDIOPULMONARY_ARREST_BLS_ALS_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const rhythm = escapeHtml(p.arrest_rhythm || 'VFIB_PULSELESS_VT_SHOCKABLE');
    const etco2 = Number(p.end_tidal_co2_mmhg ?? 11.0).toFixed(1);
    const cycles = Number(p.compression_cycles_completed ?? 1);
    const revYesNo = p.reversible_opioid_alpha2_present ? 'YES' : 'NO';

    const alertHtml = op.hyperventilation_high_dose_epi_alert
      ? `
        <div class="cpr-alert-banner">
          ⚠️ RECOVER BLS/ALS GUARD (EtCO2 ${etco2} mmHg · ${rhythm} · Cycle ${cycles}): ${escapeHtml(plan.recover_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.rosc_clearance
      ? `
        <div class="cpr-clearance-banner">
          ✅ ROSC ACHIEVED &amp; NORMOCAPNIC PCA STABLE (${escapeHtml(op.rosc_clearance.vet_license || '')}): Sustained ROSC ${Number(op.rosc_clearance.sustained_rosc_minutes ?? 30)} min · Final EtCO2 ${Number(op.rosc_clearance.final_etco2_mmhg ?? 38.0).toFixed(1)} mmHg · Final MAP ${Number(op.rosc_clearance.final_map_mmhg ?? 92)} mmHg
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card cpr-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
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

        <div class="trust-kinematics-row cpr-metrics-row">
          <span class="status-pill status-pill-species">Rhythm: ${rhythm} • EtCO2: ${etco2} mmHg • Cycles: ${cycles} • Reversible: ${revYesNo}</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary cpr-select-op-btn"
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

    const countBadge = document.getElementById('cpr-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('cpr-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="cpr-ops-empty-msg">No RECOVER CPR operations match the current filters.</p>';
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
        ? `/api/recover-cpr/operations?${qs}`
        : '/api/recover-cpr/operations';

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
      const res = await fetch(`/api/recover-cpr/operations/${encodeURIComponent(opId)}`, {
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
    const createForm = document.getElementById('create-cpr-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (
          document.getElementById('create-incident-id')?.value || 'INC-CPR-9944'
        ).trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_ROUND_CHEST_CARDIAC_PUMP'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const arrestRhythm = (
          document.getElementById('create-arrest-rhythm')?.value || 'VFIB_PULSELESS_VT_SHOCKABLE'
        ).trim();
        const etco2MmHg = parseFloat(document.getElementById('create-etco2-mmhg')?.value || '0');
        const cyclesCompleted = parseInt(
          document.getElementById('create-cycles-completed')?.value || '1',
          10
        );
        const reversiblePresent = Boolean(
          document.getElementById('create-reversible-present')?.checked
        );

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            arrest_rhythm: arrestRhythm,
            end_tidal_co2_mmhg: isNaN(etco2MmHg) ? 0 : etco2MmHg,
            compression_cycles_completed: isNaN(cyclesCompleted) ? 1 : cyclesCompleted,
            reversible_opioid_alpha2_present: reversiblePresent,
          },
        };

        const btn = document.getElementById('btn-create-cpr-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/recover-cpr/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(`Created RECOVER CPR operation ${created.id} for ${created.pet_name}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create RECOVER CPR operation';
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
          species: 'CANINE_KEEL_CHEST_THORACIC_PUMP',
          weight_kg: 30.0,
          arrest_rhythm: 'VFIB_PULSELESS_VT_SHOCKABLE',
          end_tidal_co2_mmhg: 11.0,
          compression_cycles_completed: 1,
          reversible_opioid_alpha2_present: true,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const species = (
          document.getElementById('stage-species')?.value || baseProfile.species
        ).trim();
        const arrestRhythm = (
          document.getElementById('stage-arrest-rhythm')?.value || baseProfile.arrest_rhythm
        ).trim();
        const etco2MmHg = parseFloat(
          document.getElementById('stage-etco2-mmhg')?.value ||
            String(baseProfile.end_tidal_co2_mmhg)
        );
        const cyclesCompleted = parseInt(
          document.getElementById('stage-cycles-completed')?.value ||
            String(baseProfile.compression_cycles_completed),
          10
        );
        const reversiblePresent = Boolean(
          document.getElementById('stage-reversible-present')?.checked
        );

        const profilePayload = {
          ...baseProfile,
          species,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          arrest_rhythm: arrestRhythm,
          end_tidal_co2_mmhg: isNaN(etco2MmHg) ? baseProfile.end_tidal_co2_mmhg : etco2MmHg,
          compression_cycles_completed: isNaN(cyclesCompleted)
            ? baseProfile.compression_cycles_completed
            : cyclesCompleted,
          reversible_opioid_alpha2_present: reversiblePresent,
        };

        const btn = document.getElementById('btn-stage-plan');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/recover-cpr/operations/${encodeURIComponent(selectedOpId)}/stage-plan`,
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
              `Staged RECOVER 2-Min ALS & Defib plan for ${updated.id} → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage ALS protocol plan';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('cycle-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const cycleNumber = parseInt(
          document.getElementById('check-cycle-number')?.value || '2',
          10
        );
        const etco2MmHg = parseFloat(
          document.getElementById('check-etco2-mmhg')?.value || '22.5'
        );
        const rhythmAtPause = (
          document.getElementById('check-rhythm-pause')?.value || 'VFIB_PULSELESS_VT_SHOCKABLE'
        ).trim();
        const pauseSeconds = parseInt(
          document.getElementById('check-pause-seconds')?.value || '6',
          10
        );
        const callsign = (
          document.getElementById('check-callsign')?.value || 'DR-RECOVER-01'
        ).trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();
        const rotated = Boolean(document.getElementById('check-compressor-rotated')?.checked);

        const checkPayload = {
          cycle_number: isNaN(cycleNumber) ? 2 : cycleNumber,
          end_tidal_co2_mmhg: isNaN(etco2MmHg) ? 22.5 : etco2MmHg,
          rhythm_at_2_min_pause: rhythmAtPause,
          compressor_rotated: rotated,
          pause_duration_seconds: isNaN(pauseSeconds) ? 6 : pauseSeconds,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-cycle-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/recover-cpr/operations/${encodeURIComponent(selectedOpId)}/cycle-check`,
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
            showToast(`Recorded 2-minute cycle & EtCO2 check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record 2-minute cycle check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearanceForm = document.getElementById('rosc-clearance-form');
    if (clearanceForm) {
      clearanceForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const roscMinutes = parseInt(
          document.getElementById('clearance-rosc-minutes')?.value || '30',
          10
        );
        const finalEtco2 = parseFloat(
          document.getElementById('clearance-final-etco2')?.value || '39.0'
        );
        const finalMap = parseInt(
          document.getElementById('clearance-final-map')?.value || '92',
          10
        );
        const vetLicense = (
          document.getElementById('clearance-vet-license')?.value || 'VET-RECOVER-99410'
        ).trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const spo2Titrated = Boolean(
          document.getElementById('clearance-spo2-titrated')?.checked
        );
        const hypothermiaMaintained = Boolean(
          document.getElementById('clearance-hypothermia-maintained')?.checked
        );

        const clearancePayload = {
          sustained_rosc_minutes: isNaN(roscMinutes) ? 30 : roscMinutes,
          final_etco2_mmhg: isNaN(finalEtco2) ? 39.0 : finalEtco2,
          final_map_mmhg: isNaN(finalMap) ? 92 : finalMap,
          spo2_titrated_94_to_98_no_hyperoxia: spo2Titrated,
          permissive_hypothermia_or_normothermia_maintained: hypothermiaMaintained,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-rosc-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/recover-cpr/operations/${encodeURIComponent(selectedOpId)}/rosc-clearance`,
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
              `Issued sustained ROSC & normocapnic PCA clearance for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to issue ROSC clearance';
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
    const listEl = document.getElementById('cpr-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.cpr-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.cpr-op-card');
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
