/**
 * PetSpotR Gastric Dilatation-Volvulus (GDV / Bloat), Splenic Torsion
 * & Trocar Decompression Triage Cockpit Controller (Milestone 18.2)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'GDV-OP-101';

  const SCENARIO_PRESETS = {
    'acute-360-gdv-vpc': {
      pet_id: 'PET-GDV-204',
      pet_name: 'Goliath',
      species: 'Dog - German Shepherd',
      weight_kg: 42.0,
      presentation_type: 'ACUTE_GDV_360_PYLORIC_TORSION',
      plasma_lactate_mmol_l: 8.6,
      heart_rate_bpm: 204,
      tympanic_abdominal_girth_cm: 116.0,
      non_productive_retching_present: true,
      cranial_cephalic_dual_iv_access: true,
      ventricular_arrhythmia_vpcs: true,
    },
    'splenic-torsion-hemo': {
      pet_id: 'PET-GDV-205',
      pet_name: 'Valkyrie',
      species: 'Dog -Saint Bernard',
      weight_kg: 68.5,
      presentation_type: 'SPLENIC_TORSION_HEMOABDOMEN',
      plasma_lactate_mmol_l: 6.8,
      heart_rate_bpm: 184,
      tympanic_abdominal_girth_cm: 108.0,
      non_productive_retching_present: true,
      cranial_cephalic_dual_iv_access: true,
      ventricular_arrhythmia_vpcs: true,
    },
  };

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
    const liveStatus = document.getElementById('gdv-triage-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('eval-error-banner');
    if (!banner) return;
    if (!message) {
      banner.textContent = '';
      banner.hidden = true;
      return;
    }
    banner.textContent = message;
    banner.hidden = false;
  }

  function getStatusBadgeClass(status) {
    switch (status) {
      case 'GASTRIC_NECROSIS_VPC_SHOCK_CRITICAL':
        return 'badge-gdv-critical';
      case 'TROCAR_DECOMPRESSED_DUAL_IV_STAGED':
        return 'badge-gdv-staged';
      case 'GASTROPEXY_SINUS_RHYTHM_CLEARED':
        return 'badge-gdv-safe';
      default:
        return 'badge-gdv-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-gdv');
    const stagedEl = document.getElementById('kpi-staged-trocar');
    const safeEl = document.getElementById('kpi-cleared-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'GASTRIC_NECROSIS_VPC_SHOCK_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'TROCAR_DECOMPRESSED_DUAL_IV_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'GASTROPEXY_SINUS_RHYTHM_CLEARED'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (safeEl) safeEl.textContent = String(safe);
  }

  function renderActiveWorkspace(ops) {
    const workspaceEl = document.getElementById('active-operation-workspace');
    if (!workspaceEl) return;

    if (!ops || ops.length === 0) {
      workspaceEl.innerHTML =
        '<p class="trust-empty-hint">No active GDV bloat triage operation selected.</p>';
      return;
    }

    const active =
      ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;

    const opId = escapeHtml(active.id || '');
    const petId = escapeHtml(active.pet_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(active.species || '');
    const weightKg = Number(active.weight_kg ?? 30.0).toFixed(1);
    const presentation = escapeHtml(active.presentation_type || '');
    const status = escapeHtml(active.status || '');
    const statusClass = getStatusBadgeClass(active.status);
    const warningNote = escapeHtml(active.caudal_vena_cava_warning_note || '');
    const regimen = escapeHtml(active.required_decompression_regimen || '');
    const clearance = active.clearance_token
      ? `<div class="gdv-clearance-banner">✅ GASTROPEXY &amp; SINUS RHYTHM CLEARANCE VERIFIED: ${escapeHtml(active.clearance_token)}</div>`
      : '';

    workspaceEl.innerHTML = `
      <div class="gdv-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">Active Workspace: ${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${petId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-gdv-presentation">${presentation}</span>
            <span class="badge badge-gdv-status ${statusClass}">${status}</span>
          </div>
        </div>
        <div class="gdv-venacava-alert-banner">
          ⚠️ ${warningNote}
        </div>
        ${clearance}
        <div class="gdv-regimen-recommendations">
          <div><strong>Required Decompression &amp; Resuscitation Regimen:</strong> ${regimen}</div>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const weightKg = Number(op.weight_kg ?? 30.0).toFixed(1);
    const presentation = escapeHtml(op.presentation_type || 'ACUTE_GDV_360_PYLORIC_TORSION');
    const status = escapeHtml(op.status || 'GASTRIC_NECROSIS_VPC_SHOCK_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const lactateMmol = Number(op.plasma_lactate_mmol_l ?? 6.5).toFixed(1);
    const heartRate = Number(op.heart_rate_bpm ?? 180);
    const girthCm = Number(op.tympanic_abdominal_girth_cm ?? 95.0).toFixed(1);
    const warningNote = escapeHtml(op.caudal_vena_cava_warning_note || '');
    const regimen = escapeHtml(op.required_decompression_regimen || '');

    const flags = [];
    if (op.non_productive_retching_present) {
      flags.push('NON_PRODUCTIVE_RETCHING_TYMPANY');
    }
    if (Number(op.plasma_lactate_mmol_l) >= 6.0) {
      flags.push('GASTRIC_NECROSIS_HYPERLACTATEMIA');
    }
    if (op.ventricular_arrhythmia_vpcs) {
      flags.push('REPERFUSION_VPC_ARRHYTHMIA_ALERT');
    }
    if (!op.cranial_cephalic_dual_iv_access) {
      flags.push('CAUDAL_VENA_CAVA_SAPHENOUS_HAZARD');
    }

    const flagsHtml = flags
      .map((f) => `<span class="badge badge-gdv-flag">${escapeHtml(f)}</span>`)
      .join('');

    const clearanceBannerHtml = op.clearance_token
      ? `
          <div class="gdv-clearance-banner">
            ✅ GASTROPEXY &amp; SINUS RHYTHM CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
          </div>
        `
      : '';

    const isSafe = op.status === 'GASTROPEXY_SINUS_RHYTHM_CLEARED';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const desc = escapeHtml(step.description || '');
        const badgeClass = done ? 'badge-gdv-safe' : 'badge-gdv-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="gdv-step-item">
            <input
              type="checkbox"
              class="gdv-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="gdv-step-title">${title}</strong>
            <span class="gdv-step-desc">${desc}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary gdv-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Gastropexy &amp; Sinus Rhythm Clearance (${opId})
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card gdv-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-presentation-type="${presentation}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-gdv-presentation">${presentation}</span>
            <span class="badge badge-gdv-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row gdv-metrics-row">
          <span class="badge badge-gdv-presentation">Plasma Lactate: ${lactateMmol} mmol/L • Heart Rate: ${heartRate} bpm • Tympanic Girth: ${girthCm} cm</span>
        </div>

        <div class="gdv-flags-row">
          ${flagsHtml}
        </div>

        <div class="gdv-venacava-alert-banner">
          ⚠️ ${warningNote}
        </div>

        ${clearanceBannerHtml}

        <div class="gdv-regimen-recommendations">
          <div><strong>Required Decompression Regimen:</strong> ${regimen}</div>
        </div>

        <div class="gdv-steps-list" aria-label="Cranial Cephalic IV, Trocar Decompression, Double-Bubble X-Ray &amp; Gastropexy Protocol Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary gdv-select-workspace-btn"
            data-op-id="${opId}"
          >
            🔍 Inspect in Active Workspace
          </button>
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops) {
    currentOps = ops;
    updateSummaryStats(ops);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('gdv-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('gdv-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="gdv-ops-empty-msg">No GDV / bloat or splenic torsion triage operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const presFilter = (document.getElementById('filter-presentation-type')?.value || '').trim();
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const params = new URLSearchParams();
      if (presFilter) params.set('presentation_type', presFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/gdv-bloat/operations?${qs}`
        : '/api/v1/gdv-bloat/operations';

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
      renderAllOperations(ops);
      return ops;
    } catch (_err) {
      return [];
    }
  }

  async function evaluateOperation(payload) {
    showErrorBanner('');
    const res = await fetch('/api/v1/gdv-bloat/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      selectedOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated GDV bloat operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error || 'Failed to evaluate GDV bloat triage operation';
    showErrorBanner(msg);
    showToast(msg, 'error');
    return null;
  }

  function populateFormWithPreset(preset) {
    const setVal = (id, val) => {
      const el = document.getElementById(id);
      if (el) el.value = String(val);
    };
    const setCheck = (id, val) => {
      const el = document.getElementById(id);
      if (el) el.checked = Boolean(val);
    };
    setVal('eval-pet-name', preset.pet_name);
    setVal('eval-species', preset.species);
    setVal('eval-weight-kg', preset.weight_kg);
    setVal('eval-presentation-type', preset.presentation_type);
    setVal('eval-lactate-mmol', preset.plasma_lactate_mmol_l);
    setVal('eval-heart-rate', preset.heart_rate_bpm);
    setVal('eval-girth-cm', preset.tympanic_abdominal_girth_cm);
    setCheck('eval-retching', preset.non_productive_retching_present);
    setCheck('eval-cranial-iv', preset.cranial_cephalic_dual_iv_access);
    setCheck('eval-vpcs', preset.ventricular_arrhythmia_vpcs);
  }

  function setupFormAndFilters() {
    const form = document.getElementById('gdv-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petName = (document.getElementById('eval-pet-name')?.value || '').trim();
        const species = (document.getElementById('eval-species')?.value || 'Dog - Deep-Chested Breed').trim();
        const weightKg = parseFloat(document.getElementById('eval-weight-kg')?.value || '0');
        const presentationType = (document.getElementById('eval-presentation-type')?.value || 'ACUTE_GDV_360_PYLORIC_TORSION').trim();
        const lactateMmol = parseFloat(document.getElementById('eval-lactate-mmol')?.value || '0');
        const heartRate = parseInt(document.getElementById('eval-heart-rate')?.value || '0', 10);
        const girthCm = parseFloat(document.getElementById('eval-girth-cm')?.value || '0');
        const retching = Boolean(document.getElementById('eval-retching')?.checked);
        const cranialIV = Boolean(document.getElementById('eval-cranial-iv')?.checked);
        const vpcs = Boolean(document.getElementById('eval-vpcs')?.checked);

        if (!petName) {
          showErrorBanner('pet_name is required');
          return;
        }

        const payload = {
          pet_name: petName,
          species,
          weight_kg: isNaN(weightKg) ? 0 : weightKg,
          presentation_type: presentationType,
          plasma_lactate_mmol_l: isNaN(lactateMmol) ? 0 : lactateMmol,
          heart_rate_bpm: isNaN(heartRate) ? 0 : heartRate,
          tympanic_abdominal_girth_cm: isNaN(girthCm) ? 0 : girthCm,
          non_productive_retching_present: retching,
          cranial_cephalic_dual_iv_access: cranialIV,
          ventricular_arrhythmia_vpcs: vpcs,
        };

        const btn = document.getElementById('btn-submit-gdv-eval');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.gdv-scenario-btn');
    scenarioBtns.forEach((btn) => {
      btn.addEventListener('click', async () => {
        const scenarioKey = btn.getAttribute('data-scenario');
        const preset = SCENARIO_PRESETS[scenarioKey];
        if (!preset) return;

        populateFormWithPreset(preset);
        btn.disabled = true;
        try {
          await evaluateOperation(preset);
        } finally {
          btn.disabled = false;
        }
      });
    });

    const filterPres = document.getElementById('filter-presentation-type');
    if (filterPres) {
      filterPres.addEventListener('change', () => {
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
    const listEl = document.getElementById('gdv-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.gdv-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/gdv-bloat/operations/${encodeURIComponent(opId)}/steps`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ step_id: stepId, completed }),
          }
        );
        if (res.ok) {
          const updated = await res.json();
          selectedOpId = updated.id;
          await fetchOperations();
          showToast(
            `Updated step ${stepId} for ${opId} → ${updated.status}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to update protocol step', 'error');
        }
      } catch (_err) {
        showToast('Network error updating protocol step', 'error');
      } finally {
        checkbox.disabled = false;
      }
    });

    listEl.addEventListener('click', async (e) => {
      const workspaceBtn = e.target.closest('.gdv-select-workspace-btn');
      if (workspaceBtn) {
        const opId = workspaceBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const clearanceBtn = e.target.closest('.gdv-issue-clearance-btn');
      if (!clearanceBtn) return;

      const opId = clearanceBtn.getAttribute('data-op-id');
      if (!opId) return;

      clearanceBtn.disabled = true;
      try {
        const op = currentOps.find((item) => item.id === opId);
        if (op && Array.isArray(op.protocol_steps)) {
          for (const step of op.protocol_steps) {
            if (!step.completed) {
              await fetch(
                `/api/v1/gdv-bloat/operations/${encodeURIComponent(opId)}/steps`,
                {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({ step_id: step.id, completed: true }),
                }
              );
            }
          }
        }

        const res = await fetch(
          `/api/v1/gdv-bloat/operations/${encodeURIComponent(opId)}/clearance`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({}),
          }
        );
        if (res.ok) {
          const updated = await res.json();
          selectedOpId = updated.id;
          await fetchOperations();
          showToast(
            `Issued gastropexy & sinus rhythm clearance ${updated.clearance_token} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue GDV clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing GDV clearance', 'error');
      } finally {
        clearanceBtn.disabled = false;
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
    setupFormAndFilters();
    setupCardInteractions();
    fetchOperations();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
