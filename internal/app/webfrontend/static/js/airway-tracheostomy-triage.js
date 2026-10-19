/**
 * PetSpotR Brachycephalic Obstructive Airway Syndrome (BOAS), Laryngeal Paralysis
 * & Temporary Tracheostomy Triage Cockpit Controller (Milestone 19.3)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'AT-OP-101';

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
    const liveStatus = document.getElementById('airway-triage-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('airway-form-error-banner');
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
      case 'STRIDOR_CYANOSIS_DYSPNEA_CRITICAL':
        return 'status-pill status-pill-critical';
      case 'BUTORPHANOL_DEX_INTUBATION_STAGED':
        return 'status-pill status-pill-staged';
      case 'PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED':
        return 'status-pill status-pill-safe';
      default:
        return 'status-pill status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-stridor');
    const stagedEl = document.getElementById('kpi-staged-intubation');
    const patentEl = document.getElementById('kpi-patent-extubated');

    const total =
      metrics && typeof metrics.total_operations === 'number'
        ? metrics.total_operations
        : ops.length;
    const critical =
      metrics && typeof metrics.critical_stridor_cyanosis === 'number'
        ? metrics.critical_stridor_cyanosis
        : ops.filter((op) => op.status === 'STRIDOR_CYANOSIS_DYSPNEA_CRITICAL').length;
    const staged =
      metrics && typeof metrics.sedation_intubation_staged === 'number'
        ? metrics.sedation_intubation_staged
        : ops.filter((op) => op.status === 'BUTORPHANOL_DEX_INTUBATION_STAGED').length;
    const patent =
      metrics && typeof metrics.patent_normothermic_extubated === 'number'
        ? metrics.patent_normothermic_extubated
        : ops.filter((op) => op.status === 'PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED').length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (patentEl) patentEl.textContent = String(patent);
  }

  function renderActiveWorkspace(ops) {
    const workspaceEl = document.getElementById('active-operation-workspace');
    if (!workspaceEl) return;

    if (!ops || ops.length === 0) {
      workspaceEl.innerHTML =
        '<p class="trust-empty-hint">No airway or tracheostomy triage operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;

    const opId = escapeHtml(active.id || '');
    const incidentId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const profile = active.patient_profile || {};
    const breed = escapeHtml(profile.breed || '');
    const species = escapeHtml(profile.species || '');
    const weightKg = Number(profile.weight_kg ?? 10.0).toFixed(1);
    const status = escapeHtml(active.status || '');
    const statusPillClass = getStatusPillClass(active.status);

    const plan = active.stabilization_plan || null;
    const hazardAlert = escapeHtml(
      (plan && plan.airway_hazard_alert) ||
        'CRITICAL AIRWAY & TRACHEOSTOMY HAZARD: Never apply forceful physical restraint without Butorphanol sedation, use downsized ET tubes in brachycephalics, and limit tracheal suctioning to <=10 seconds after >=60s pre-oxygenation.'
    );

    const clearance = active.extubation_clearance || null;
    const clearanceHiddenAttr = clearance ? '' : 'hidden';
    const clearanceText = clearance
      ? `✅ PATENT AIRWAY &amp; NORMOTHERMIC EXTUBATION CLEARANCE VERIFIED (${escapeHtml(
          clearance.vet_license
        )}): Room-Air SpO2 ${Number(clearance.final_spo2_on_room_air_percent ?? 98.0).toFixed(
          1
        )}% · Temp ${Number(clearance.final_rectal_temp_fahrenheit ?? 100.8).toFixed(
          1
        )}°F · Harness-Only Verified — ${escapeHtml(clearance.summary)}`
      : '';

    const planHtml = plan
      ? `
        <div id="workspace-stabilization-summary" class="airway-regimen-recommendations">
          <div><strong>Butorphanol + Acepromazine Anxiolysis:</strong> <span id="ws-butorphanol-mg">${Number(
            plan.butorphanol_acepromazine_dose_mg ?? 0
          ).toFixed(2)} mg</span> • <strong>Dexamethasone-SP (0.15 mg/kg IV):</strong> <span id="ws-dex-mg">${Number(
          plan.dexamethasone_sp_015_mg_kg_dose_mg ?? 0
        ).toFixed(2)} mg</span> • <strong>Propofol (2.0 mg/kg IV):</strong> <span id="ws-propofol-mg">${Number(
          plan.propofol_induction_2_mg_kg_dose_mg ?? 0
        ).toFixed(2)} mg</span></div>
          <div><strong>Downsized Endotracheal Tube:</strong> <span id="ws-et-tube-mm">${Number(
            plan.endotracheal_tube_size_mm ?? 0
          ).toFixed(1)} mm</span> • <strong>Active Cooling Ceiling:</strong> <span id="ws-cooling-target">${Number(
          plan.active_cooling_target_max_temp_f ?? 103.0
        ).toFixed(1)}°F</span> • <strong>Temp Tracheostomy Indicated:</strong> <span id="ws-trach-indicated">${
          plan.temporary_tracheostomy_indicated ? 'YES (Staged)' : 'NO'
        }</span></div>
          <div><strong>Clinical Rationale:</strong> <span id="ws-clinical-rationale">${escapeHtml(
            plan.clinical_rationale || ''
          )}</span></div>
        </div>
      `
      : '';

    const logs = Array.isArray(active.trach_care_logs) ? active.trach_care_logs : [];
    const logsItemsHtml = logs
      .map(
        (log) => `
          <li class="airway-care-log-item">
            <span class="badge badge-airway-species">${escapeHtml(log.technician_callsign || '')}</span>
            Pre-O2: ${Number(log.pre_oxygenation_duration_sec ?? 60)}s · Suction: ${Number(
          log.suction_duration_max_sec ?? 8
        )}s (&le;10s) · Saline: ${Number(log.sterile_saline_instillation_ml ?? 1.0).toFixed(
          1
        )} mL · Post-Suction SpO2: ${Number(log.post_suction_spo2_percent ?? 95.0).toFixed(
          1
        )}% — ${escapeHtml(log.notes || '')}
          </li>
        `
      )
      .join('');

    workspaceEl.innerHTML = `
      <div class="airway-workspace-card" id="workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name" id="workspace-op-title">Active Workspace: ${petName} (${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id" id="workspace-op-id">${opId} · ${incidentId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-airway-species" id="workspace-species-badge">${species}</span>
            <span id="workspace-status-pill" class="${statusPillClass}" data-status="${status}">${status}</span>
          </div>
        </div>

        <div id="airway-alert-banner" class="airway-alert-banner">
          ⚠️ ${hazardAlert}
        </div>

        <div id="airway-clearance-banner" class="airway-clearance-banner" ${clearanceHiddenAttr}>
          ${clearanceText}
        </div>

        ${planHtml}

        <div id="workspace-trach-care-logs" class="airway-care-logs-box">
          <strong>Aseptic Tracheostomy &amp; Suction Care Logs (${logs.length}):</strong>
          <ul id="workspace-care-logs-list" class="airway-care-logs-list">
            ${logsItemsHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const incidentId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const profile = op.patient_profile || {};
    const species = escapeHtml(profile.species || 'CANINE_BRACHYCEPHALIC_BOAS');
    const breed = escapeHtml(profile.breed || 'Canine');
    const weightKg = Number(profile.weight_kg ?? 10.0).toFixed(1);
    const tempF = Number(profile.rectal_temp_fahrenheit ?? 101.5).toFixed(1);
    const spo2Pct = Number(profile.spo2_percent ?? 95.0).toFixed(1);
    const effort = escapeHtml(profile.inspiratory_effort || '');
    const grade = escapeHtml(profile.laryngeal_edema_grade || '');
    const status = escapeHtml(op.status || 'STRIDOR_CYANOSIS_DYSPNEA_CRITICAL');
    const statusPillClass = getStatusPillClass(op.status);

    const plan = op.stabilization_plan || null;
    const planHtml = plan
      ? `
        <div class="airway-alert-banner">
          ⚠️ ${escapeHtml(plan.airway_hazard_alert || '')}
        </div>
        <div class="airway-regimen-recommendations">
          <div><strong>Stabilization Dosing:</strong> Butorphanol/Ace ${Number(
            plan.butorphanol_acepromazine_dose_mg ?? 0
          ).toFixed(2)} mg · Dex-SP ${Number(plan.dexamethasone_sp_015_mg_kg_dose_mg ?? 0).toFixed(
          2
        )} mg · Propofol ${Number(plan.propofol_induction_2_mg_kg_dose_mg ?? 0).toFixed(
          2
        )} mg · ET Tube ${Number(plan.endotracheal_tube_size_mm ?? 0).toFixed(
          1
        )} mm · Cooling Stop ${Number(plan.active_cooling_target_max_temp_f ?? 103.0).toFixed(
          1
        )}°F</div>
        </div>
      `
      : '';

    const clearance = op.extubation_clearance || null;
    const clearanceHtml = clearance
      ? `
        <div class="airway-clearance-banner">
          ✅ PATENT AIRWAY &amp; NORMOTHERMIC EXTUBATED (${escapeHtml(
            clearance.vet_license || ''
          )}): Room-Air SpO2 ${Number(clearance.final_spo2_on_room_air_percent ?? 98.0).toFixed(
          1
        )}% · Temp ${Number(clearance.final_rectal_temp_fahrenheit ?? 100.8).toFixed(
          1
        )}°F — ${escapeHtml(clearance.summary || '')}
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card airway-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${incidentId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-airway-species">${species}</span>
            <span class="${statusPillClass}" data-status="${status}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row airway-metrics-row">
          <span class="badge badge-airway-species">Temp: ${tempF}°F • SpO2: ${spo2Pct}% • Effort: ${effort} • Grade: ${grade}</span>
        </div>

        ${planHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary airway-select-op-btn"
            data-op-id="${opId}"
          >
            🔍 Select in Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateSummaryStats(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('airway-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('airway-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="airway-ops-empty-msg">No airway or tracheostomy triage operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const speciesFilter = (document.getElementById('filter-species')?.value || '').trim();
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const params = new URLSearchParams();
      if (speciesFilter) params.set('species', speciesFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/airway-tracheostomy/operations?${qs}`
        : '/api/airway-tracheostomy/operations';

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
        `/api/airway-tracheostomy/operations/${encodeURIComponent(opId)}`,
        {
          headers: { Accept: 'application/json' },
        }
      );
      if (res.ok) {
        const fetched = await res.json();
        const idx = currentOps.findIndex((item) => item.id === fetched.id);
        if (idx >= 0) {
          currentOps[idx] = fetched;
        } else {
          currentOps.push(fetched);
        }
      }
    } catch (_err) {
      // Fallback to local currentOps
    }
    renderActiveWorkspace(currentOps);
  }

  function setupFormsAndFilters() {
    // 1. Create Airway Operation Form
    const createForm = document.getElementById('airway-create-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (document.getElementById('create-incident-id')?.value || '').trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (document.getElementById('create-species')?.value || 'CANINE_BRACHYCEPHALIC_BOAS').trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const rectalTempF = parseFloat(document.getElementById('create-rectal-temp-f')?.value || '0');
        const spo2Pct = parseFloat(document.getElementById('create-spo2-pct')?.value || '0');
        const effort = (document.getElementById('create-inspiratory-effort')?.value || 'SEVERE_PARADOXICAL_CYANOTIC').trim();
        const grade = (document.getElementById('create-edema-grade')?.value || 'GRADE_III_EVERTED_SACCULES_COLLAPSE').trim();
        const gagPresent = Boolean(document.getElementById('create-gag-reflex')?.checked);

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            rectal_temp_fahrenheit: isNaN(rectalTempF) ? 0 : rectalTempF,
            spo2_percent: isNaN(spo2Pct) ? 0 : spo2Pct,
            inspiratory_effort: effort,
            laryngeal_edema_grade: grade,
            gag_reflex_present: gagPresent,
          },
        };

        const btn = document.getElementById('btn-create-airway-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/airway-tracheostomy/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Created airway triage operation ${created.id} for ${created.pet_name} → ${created.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create airway triage operation';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    // 2. Stage Stabilization Form
    const stageForm = document.getElementById('airway-stage-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const active =
          currentOps.find((item) => item.id === selectedOpId) || currentOps[0];
        if (!active) return;

        const baseProfile = active.patient_profile || {};
        const rectalTempF = parseFloat(document.getElementById('stage-rectal-temp-f')?.value || String(baseProfile.rectal_temp_fahrenheit || 102.8));
        const spo2Pct = parseFloat(document.getElementById('stage-spo2-pct')?.value || String(baseProfile.spo2_percent || 94.0));
        const effort = (document.getElementById('stage-inspiratory-effort')?.value || baseProfile.inspiratory_effort || 'MODERATE_STRIDOR_ORTHOPNEA').trim();
        const grade = (document.getElementById('stage-edema-grade')?.value || baseProfile.laryngeal_edema_grade || 'GRADE_III_EVERTED_SACCULES_COLLAPSE').trim();

        const payload = {
          patient_profile: {
            species: baseProfile.species || 'CANINE_BRACHYCEPHALIC_BOAS',
            breed: baseProfile.breed || 'French Bulldog',
            weight_kg: Number(baseProfile.weight_kg || 12.4),
            rectal_temp_fahrenheit: isNaN(rectalTempF) ? 102.8 : rectalTempF,
            spo2_percent: isNaN(spo2Pct) ? 94.0 : spo2Pct,
            inspiratory_effort: effort,
            laryngeal_edema_grade: grade,
            gag_reflex_present: Boolean(baseProfile.gag_reflex_present),
          },
        };

        const btn = document.getElementById('btn-stage-stabilization');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/airway-tracheostomy/operations/${encodeURIComponent(active.id)}/stage-stabilization`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(payload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Staged Butorphanol/Dex-SP/Intubation plan for ${updated.id} → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage stabilization plan';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    // 3. Record Tracheostomy Care Log Form
    const careForm = document.getElementById('airway-trach-care-form');
    if (careForm) {
      careForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const active =
          currentOps.find((item) => item.id === selectedOpId) || currentOps[0];
        if (!active) return;

        const preoxSec = parseInt(document.getElementById('care-preox-sec')?.value || '90', 10);
        const suctionSec = parseInt(document.getElementById('care-suction-sec')?.value || '8', 10);
        const salineMl = parseFloat(document.getElementById('care-saline-ml')?.value || '1.5');
        const postSpo2 = parseFloat(document.getElementById('care-post-spo2')?.value || '96.0');
        const callsign = (document.getElementById('care-tech-callsign')?.value || '').trim();
        const notes = (document.getElementById('care-notes')?.value || '').trim();
        const innerCleaned = Boolean(document.getElementById('care-inner-cannula')?.checked);
        const emphysemaNoted = Boolean(document.getElementById('care-subcut-emphysema')?.checked);

        const payload = {
          pre_oxygenation_duration_sec: isNaN(preoxSec) ? 0 : preoxSec,
          suction_duration_max_sec: isNaN(suctionSec) ? 0 : suctionSec,
          sterile_saline_instillation_ml: isNaN(salineMl) ? 0 : salineMl,
          post_suction_spo2_percent: isNaN(postSpo2) ? 0 : postSpo2,
          technician_callsign: callsign,
          notes,
          inner_cannula_cleaned: innerCleaned,
          subcutaneous_emphysema_noted: emphysemaNoted,
        };

        const btn = document.getElementById('btn-record-trach-care');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/airway-tracheostomy/operations/${encodeURIComponent(active.id)}/trach-care-log`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(payload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Recorded aseptic tracheostomy care log (${suctionSec}s <= 10s) for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record tracheostomy care log';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    // 4. Issue Extubation Clearance Form
    const clearanceForm = document.getElementById('airway-clearance-form');
    if (clearanceForm) {
      clearanceForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const active =
          currentOps.find((item) => item.id === selectedOpId) || currentOps[0];
        if (!active) return;

        const finalSpo2 = parseFloat(document.getElementById('clearance-spo2-pct')?.value || '97.5');
        const finalTemp = parseFloat(document.getElementById('clearance-temp-f')?.value || '101.0');
        const vetLicense = (document.getElementById('clearance-vet-license')?.value || '').trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const gagVigorous = Boolean(document.getElementById('clearance-gag-vigorous')?.checked);
        const trachOccluded = Boolean(document.getElementById('clearance-trach-occluded')?.checked);
        const harnessOnly = Boolean(document.getElementById('clearance-harness-only')?.checked);

        const payload = {
          final_spo2_on_room_air_percent: isNaN(finalSpo2) ? 0 : finalSpo2,
          final_rectal_temp_fahrenheit: isNaN(finalTemp) ? 0 : finalTemp,
          gag_swallow_reflex_vigorous: gagVigorous,
          trach_tube_occluded_trial_passed: trachOccluded,
          harness_only_discharge_verified: harnessOnly,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-extubation-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/airway-tracheostomy/operations/${encodeURIComponent(active.id)}/extubation-clearance`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(payload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Issued extubation clearance for ${updated.id} → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to issue extubation clearance';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const filterSpecies = document.getElementById('filter-species');
    if (filterSpecies) {
      filterSpecies.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const filterStatus = document.getElementById('filter-status');
    if (filterStatus) {
      filterStatus.addEventListener('change', () => {
        fetchOperations();
      });
    }
  }

  function setupCardInteractions() {
    const listEl = document.getElementById('airway-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const selectBtn = e.target.closest('.airway-select-op-btn');
      if (selectBtn) {
        const opId = selectBtn.getAttribute('data-op-id');
        if (opId) {
          await selectOperationById(opId);
        }
        return;
      }

      const card = e.target.closest('.airway-op-card');
      if (card) {
        const opId = card.getAttribute('data-op-id');
        if (opId) {
          await selectOperationById(opId);
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
