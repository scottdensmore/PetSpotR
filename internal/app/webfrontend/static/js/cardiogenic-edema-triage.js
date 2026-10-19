/**
 * PetSpotR Acute Cardiogenic Pulmonary Edema, Mitral Valve Chordae Rupture,
 * Furosemide/Pimobendan & Oxygen Cage Cockpit Controller (Milestone 19.4)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'CE-OP-101';

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
    const liveStatus = document.getElementById('cardiogenic-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('cardiogenic-error-banner');
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
      case 'FULMINANT_PULMONARY_EDEMA_CRITICAL':
        return 'status-pill-critical';
      case 'FUROSEMIDE_PIMOBENDAN_O2_CAGE_STAGED':
        return 'status-pill-staged';
      case 'EUPNEIC_BLINES_RESOLVED_CLEARED':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateKPIs(metrics, ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-fulminant-critical');
    const stagedEl = document.getElementById('kpi-furosemide-staged');
    const clearedEl = document.getElementById('kpi-eupneic-cleared');
    const locksEl = document.getElementById('kpi-fluid-guard-locks');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.fulminant_critical ??
      ops.filter((op) => op.status === 'FULMINANT_PULMONARY_EDEMA_CRITICAL').length;
    const staged =
      metrics?.furosemide_o2_staged ??
      ops.filter((op) => op.status === 'FUROSEMIDE_PIMOBENDAN_O2_CAGE_STAGED').length;
    const cleared =
      metrics?.eupneic_cleared ??
      ops.filter((op) => op.status === 'EUPNEIC_BLINES_RESOLVED_CLEARED').length;
    const locks =
      metrics?.active_fluid_guard_lock ??
      ops.filter((op) => Boolean(op.fluid_overload_contraindication_alert)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (locksEl) locksEl.textContent = String(locks);
  }

  function syncStageFormWithOperation(op) {
    if (!op || !op.patient_profile) return;
    const p = op.patient_profile;
    const setVal = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined && val !== null) {
        el.value = String(val);
      }
    };
    setVal('stage-species', p.species);
    setVal('stage-weight-kg', p.weight_kg);
    setVal('stage-resp-rate', p.respiratory_rate_bpm);
    setVal('stage-heart-rate', p.heart_rate_bpm);
    setVal('stage-murmur-grade', p.murmur_grade);
    setVal('stage-tfast-blines', p.tfast_b_lines_per_site);
    setVal('stage-la-ao-ratio', p.la_ao_ratio);
    const pleuralEl = document.getElementById('stage-pleural-effusion');
    if (pleuralEl) {
      pleuralEl.checked = Boolean(p.pleural_effusion_present);
    }
    const labelEl = document.getElementById('stage-target-op-label');
    if (labelEl) {
      labelEl.textContent = op.id;
    }
  }

  function updateBannersForActiveOp(active) {
    const alertBanner = document.getElementById('cardiogenic-alert-banner');
    const clearanceBanner = document.getElementById('cardiogenic-clearance-banner');

    if (!active) {
      if (alertBanner) alertBanner.hidden = true;
      if (clearanceBanner) clearanceBanner.hidden = true;
      return;
    }

    if (alertBanner) {
      if (active.fluid_overload_contraindication_alert) {
        const pitfall =
          active.diuretic_regimen?.cardiogenic_pitfall_alert ||
          'STRICT ZERO IV CRYSTALLOID FLUID BOLUS CONTRAINDICATION — Never administer isotonic IV crystalloids or force dorsal recumbency radiographs in acute congestive heart failure prior to oxygen cage, furosemide, and butorphanol stabilization.';
        alertBanner.textContent = `⚠️ ZERO IV CRYSTALLOID FLUID CONTRAINDICATION GUARD ACTIVE (${active.pet_name} · ${active.id}): ${pitfall}`;
        alertBanner.hidden = false;
      } else {
        alertBanner.hidden = true;
      }
    }

    if (clearanceBanner) {
      if (active.discharge_clearance) {
        const c = active.discharge_clearance;
        const spo2 = Number(c.room_air_spo2_percent ?? 98.0).toFixed(1);
        clearanceBanner.textContent = `✅ EUPNEIC & TFAST B-LINES RESOLVED DISCHARGE CLEARANCE ISSUED (${active.pet_name} · ${active.id} · Vet ${c.vet_license}): Sleeping RR ${c.final_sleeping_resp_rate_bpm} bpm · B-Lines ${c.final_tfast_b_lines_count}/site · Room Air SpO2 ${spo2}% — ${c.summary}`;
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
        '<p class="trust-empty-hint">No active cardiogenic pulmonary edema operation selected.</p>';
      updateBannersForActiveOp(null);
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateBannersForActiveOp(active);

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const p = active.patient_profile || {};
    const species = escapeHtml(p.species || '');
    const weightKg = Number(p.weight_kg ?? 8.0).toFixed(1);
    const rr = Number(p.respiratory_rate_bpm ?? 60);
    const hr = Number(p.heart_rate_bpm ?? 160);
    const murmur = escapeHtml(p.murmur_grade || '');
    const bLines = Number(p.tfast_b_lines_per_site ?? 0);
    const laAo = Number(p.la_ao_ratio ?? 1.8).toFixed(2);
    const status = escapeHtml(active.status || '');
    const pillClass = getStatusPillClass(active.status);

    let regimenHtml = '';
    if (active.diuretic_regimen) {
      const r = active.diuretic_regimen;
      regimenHtml = `
        <div class="cardiogenic-regimen-box" id="workspace-regimen-box">
          <div><strong>Furosemide IV Bolus:</strong> ${Number(r.furosemide_initial_bolus_mg ?? 0).toFixed(2)} mg | <strong>Furosemide CRI (0.66 mg/kg/hr):</strong> ${Number(r.furosemide_cri_066_mg_kg_hr_mg_hr ?? 0).toFixed(2)} mg/hr | <strong>Pimobendan (0.25 mg/kg):</strong> ${Number(r.pimobendan_025_mg_kg_dose_mg ?? 0).toFixed(2)} mg | <strong>Butorphanol (0.2 mg/kg):</strong> ${Number(r.butorphanol_anxiolytic_dose_mg ?? 0).toFixed(2)} mg | <strong>Oxygen Cage FiO2:</strong> ${Number(r.oxygen_cage_fio2_percent ?? 50)}%</div>
          <div><strong>Clinical Rationale:</strong> ${escapeHtml(r.clinical_rationale || '')}</div>
        </div>
      `;
    }

    const checks = Array.isArray(active.diuresis_checks) ? active.diuresis_checks : [];
    const checksItemsHtml = checks
      .map((chk) => {
        return `
          <div class="cardiogenic-check-item">
            <span>RR ${Number(chk.resting_respiratory_rate_bpm ?? 0)} bpm · UOP ${Number(chk.urine_output_ml_kg_hr ?? 0).toFixed(1)} mL/kg/hr · B-Lines ${Number(chk.tfast_b_lines_remaining ?? 0)}/site · SBP ${Number(chk.systolic_blood_pressure_mmhg ?? 0)} mmHg · Creat ${Number(chk.creatinine_mg_dl ?? 0).toFixed(1)} mg/dL · K+ ${Number(chk.potassium_meq_l ?? 0).toFixed(1)} mEq/L (${escapeHtml(chk.clinician_callsign || '')}) — ${escapeHtml(chk.notes || '')}</span>
          </div>
        `;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="cardiogenic-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name" id="workspace-pet-title">Active Workspace: ${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${incId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="status-pill ${pillClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="status-pill status-pill-staged">RR: ${rr} bpm · HR: ${hr} bpm · Murmur: ${murmur} · TFAST B-Lines: ${bLines}/site · La:Ao: ${laAo}</span>
        </div>

        ${regimenHtml}

        <div class="cardiogenic-checks-list" id="workspace-checks-list">
          <strong>Diuresis &amp; TFAST B-Line Telemetry Checks (${checks.length}):</strong>
          ${checksItemsHtml}
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const p = op.patient_profile || {};
    const species = escapeHtml(p.species || '');
    const weightKg = Number(p.weight_kg ?? 8.0).toFixed(1);
    const rr = Number(p.respiratory_rate_bpm ?? 60);
    const hr = Number(p.heart_rate_bpm ?? 160);
    const murmur = escapeHtml(p.murmur_grade || '');
    const bLines = Number(p.tfast_b_lines_per_site ?? 0);
    const laAo = Number(p.la_ao_ratio ?? 1.8).toFixed(2);
    const status = escapeHtml(op.status || '');
    const pillClass = getStatusPillClass(op.status);

    const alertHtml = op.fluid_overload_contraindication_alert
      ? `
        <div class="cardiogenic-alert-banner">
          ⚠️ ZERO IV CRYSTALLOID FLUID GUARD ACTIVE: ${escapeHtml(
            op.diuretic_regimen?.cardiogenic_pitfall_alert ||
              'Avoid IV crystalloid fluid boluses and dorsal recumbency radiographs prior to stabilization.'
          )}
        </div>
      `
      : '';

    const clearanceHtml = op.discharge_clearance
      ? `
        <div class="cardiogenic-clearance-banner">
          ✅ EUPNEIC &amp; B-LINES RESOLVED CLEARANCE (${escapeHtml(
            op.discharge_clearance.vet_license || ''
          )}): SRR ${Number(op.discharge_clearance.final_sleeping_resp_rate_bpm ?? 0)} bpm · B-Lines ${Number(
          op.discharge_clearance.final_tfast_b_lines_count ?? 0
        )}/site · SpO2 ${Number(op.discharge_clearance.room_air_spo2_percent ?? 98.0).toFixed(1)}% — ${escapeHtml(
          op.discharge_clearance.summary || ''
        )}
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card cardiogenic-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${incId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="status-pill ${pillClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="status-pill status-pill-staged">RR: ${rr} bpm • HR: ${hr} bpm • ${murmur} • TFAST B-Lines: ${bLines}/site • La:Ao: ${laAo}</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary cardiogenic-select-op-btn"
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
    updateKPIs(metrics, ops);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('cardiogenic-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('cardiogenic-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="cardiogenic-ops-empty-msg">No cardiogenic pulmonary edema operations match the current filter.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/cardiogenic-edema/operations?${qs}`
        : '/api/cardiogenic-edema/operations';

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

  function setupFormsAndFilters() {
    // 1. Create Operation Form
    const createForm = document.getElementById('cardiogenic-create-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const payload = {
          incident_id: (document.getElementById('create-incident-id')?.value || '').trim(),
          pet_name: (document.getElementById('create-pet-name')?.value || '').trim(),
          patient_profile: {
            species: (document.getElementById('create-species')?.value || 'CANINE_MMVD_STAGE_C_D').trim(),
            weight_kg: parseFloat(document.getElementById('create-weight-kg')?.value || '0'),
            respiratory_rate_bpm: parseInt(document.getElementById('create-resp-rate')?.value || '0', 10),
            heart_rate_bpm: parseInt(document.getElementById('create-heart-rate')?.value || '0', 10),
            murmur_grade: (document.getElementById('create-murmur-grade')?.value || 'GRADE_VI_THRILL_CHORDAE_RUPTURE').trim(),
            tfast_b_lines_per_site: parseInt(document.getElementById('create-tfast-blines')?.value || '0', 10),
            la_ao_ratio: parseFloat(document.getElementById('create-la-ao-ratio')?.value || '0'),
            pleural_effusion_present: Boolean(document.getElementById('create-pleural-effusion')?.checked),
          },
        };

        const btn = document.getElementById('btn-create-cardiogenic-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/cardiogenic-edema/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted cardiogenic pulmonary edema patient ${created.pet_name} (${created.id}) → ${created.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create cardiogenic edema operation';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    // 2. Stage Diuretic Regimen Form
    const stageForm = document.getElementById('cardiogenic-stage-regimen-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const profile = {
          species: (document.getElementById('stage-species')?.value || 'CANINE_MMVD_STAGE_C_D').trim(),
          weight_kg: parseFloat(document.getElementById('stage-weight-kg')?.value || '0'),
          respiratory_rate_bpm: parseInt(document.getElementById('stage-resp-rate')?.value || '0', 10),
          heart_rate_bpm: parseInt(document.getElementById('stage-heart-rate')?.value || '0', 10),
          murmur_grade: (document.getElementById('stage-murmur-grade')?.value || 'GRADE_VI_THRILL_CHORDAE_RUPTURE').trim(),
          tfast_b_lines_per_site: parseInt(document.getElementById('stage-tfast-blines')?.value || '0', 10),
          la_ao_ratio: parseFloat(document.getElementById('stage-la-ao-ratio')?.value || '0'),
          pleural_effusion_present: Boolean(document.getElementById('stage-pleural-effusion')?.checked),
        };

        const btn = document.getElementById('btn-stage-regimen');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/cardiogenic-edema/operations/${encodeURIComponent(selectedOpId)}/stage-regimen`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(profile),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Staged Furosemide/Pimobendan/O2 Cage regimen for ${updated.pet_name} (${updated.id}) → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage diuretic regimen';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    // 3. Record Diuresis Check Form
    const checkForm = document.getElementById('cardiogenic-diuresis-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const checkPayload = {
          resting_respiratory_rate_bpm: parseInt(document.getElementById('check-resp-rate')?.value || '0', 10),
          urine_output_ml_kg_hr: parseFloat(document.getElementById('check-urine-output')?.value || '0'),
          tfast_b_lines_remaining: parseInt(document.getElementById('check-tfast-blines')?.value || '0', 10),
          systolic_blood_pressure_mmhg: parseInt(document.getElementById('check-sbp')?.value || '0', 10),
          creatinine_mg_dl: parseFloat(document.getElementById('check-creatinine')?.value || '0'),
          potassium_meq_l: parseFloat(document.getElementById('check-potassium')?.value || '0'),
          clinician_callsign: (document.getElementById('check-clinician')?.value || '').trim(),
          notes: (document.getElementById('check-notes')?.value || '').trim(),
        };

        const btn = document.getElementById('btn-record-diuresis-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/cardiogenic-edema/operations/${encodeURIComponent(selectedOpId)}/diuresis-check`,
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
            showToast(
              `Recorded diuresis telemetry check for ${updated.pet_name} (${updated.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record diuresis check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    // 4. Issue Discharge Clearance Form
    const clearanceForm = document.getElementById('cardiogenic-discharge-clearance-form');
    if (clearanceForm) {
      clearanceForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const clearancePayload = {
          final_sleeping_resp_rate_bpm: parseInt(document.getElementById('clearance-final-srr')?.value || '0', 10),
          final_tfast_b_lines_count: parseInt(document.getElementById('clearance-final-blines')?.value || '0', 10),
          room_air_spo2_percent: parseFloat(document.getElementById('clearance-spo2')?.value || '0'),
          renal_azotemia_stable: Boolean(document.getElementById('clearance-renal-stable')?.checked),
          home_srr_monitoring_educated: Boolean(document.getElementById('clearance-srr-educated')?.checked),
          vet_license: (document.getElementById('clearance-vet-license')?.value || '').trim(),
          summary: (document.getElementById('clearance-summary')?.value || '').trim(),
        };

        const btn = document.getElementById('btn-issue-discharge-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/cardiogenic-edema/operations/${encodeURIComponent(selectedOpId)}/discharge-clearance`,
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
              `Issued eupneic & B-line resolved discharge clearance for ${updated.pet_name} (${updated.id}) → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to issue cardiogenic discharge clearance';
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
  }

  function setupCardInteractions() {
    const listEl = document.getElementById('cardiogenic-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const selectBtn = e.target.closest('.cardiogenic-select-op-btn');
      if (selectBtn) {
        const opId = selectBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const card = e.target.closest('.cardiogenic-op-card');
      if (card) {
        const opId = card.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
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
