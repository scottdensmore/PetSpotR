/**
 * PetSpotR Feline Urethral Obstruction (Blocked Tom), Hyperkalemic Bradycardia
 * & Calcium Gluconate Cardioprotection Cockpit Controller (Milestone 18.5)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'UO-OP-101';

  const SCENARIO_PRESETS = {
    'hyperk-atrial-standstill': {
      pet_id: 'PET-UO-204',
      pet_name: 'Garfield',
      species: 'Cat - Persian Neutered Male',
      weight_kg: 6.4,
      obstruction_cause: 'STRUWITE_CRYSTALLINE_MATRIX_PLUG',
      serum_potassium_mmol_l: 9.4,
      heart_rate_bpm: 112,
      bladder_palpation_diameter_cm: 10.2,
      absent_p_wave_tented_t_wave: true,
      calcium_gluconate_before_sedation: true,
      manual_bladder_expression_avoided: true,
    },
    'canine-os-penis-calculus': {
      pet_id: 'PET-UO-205',
      pet_name: 'Buster',
      species: 'Dog - Dalmatian Intact Male',
      weight_kg: 26.5,
      obstruction_cause: 'CANINE_OS_PENIS_CALCULUS_LODGMENT',
      serum_potassium_mmol_l: 7.6,
      heart_rate_bpm: 128,
      bladder_palpation_diameter_cm: 12.5,
      absent_p_wave_tented_t_wave: true,
      calcium_gluconate_before_sedation: true,
      manual_bladder_expression_avoided: true,
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
    const liveStatus = document.getElementById('urethral-triage-live-status');
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
      case 'HYPERKALEMIC_BRADYCARDIA_ATRIAL_STANDSTILL_CRITICAL':
        return 'badge-urethral-critical';
      case 'CALCIUM_CARDIOPROTECT_RETROHYDROPROPULSION_STAGED':
        return 'badge-urethral-staged';
      case 'INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED':
        return 'badge-urethral-safe';
      default:
        return 'badge-urethral-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-hyperk');
    const stagedEl = document.getElementById('kpi-staged-unblock');
    const safeEl = document.getElementById('kpi-cleared-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'HYPERKALEMIC_BRADYCARDIA_ATRIAL_STANDSTILL_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'CALCIUM_CARDIOPROTECT_RETROHYDROPROPULSION_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED'
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
        '<p class="trust-empty-hint">No active urethral obstruction triage operation selected.</p>';
      return;
    }

    const active =
      ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;

    const opId = escapeHtml(active.id || '');
    const petId = escapeHtml(active.pet_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(active.species || '');
    const weightKg = Number(active.weight_kg ?? 5.5).toFixed(1);
    const cause = escapeHtml(active.obstruction_cause || '');
    const status = escapeHtml(active.status || '');
    const statusClass = getStatusBadgeClass(active.status);
    const warningNote = escapeHtml(active.cardioprotection_warning_note || '');
    const protocol = escapeHtml(active.required_unblocking_protocol || '');
    const clearance = active.clearance_token
      ? `<div class="urethral-clearance-banner">✅ INDWELLING CLOSED DIURESIS &amp; NORMOKALEMIC CLEARANCE VERIFIED: ${escapeHtml(active.clearance_token)}</div>`
      : '';

    workspaceEl.innerHTML = `
      <div class="urethral-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">Active Workspace: ${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${petId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-urethral-cause">${cause}</span>
            <span class="badge badge-urethral-status ${statusClass}">${status}</span>
          </div>
        </div>
        <div class="urethral-cardioprotect-alert-banner">
          ⚠️ ${warningNote}
        </div>
        ${clearance}
        <div class="urethral-protocol-recommendations">
          <div><strong>Required Unblocking &amp; Cardioprotection Protocol:</strong> ${protocol}</div>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Feline');
    const weightKg = Number(op.weight_kg ?? 5.5).toFixed(1);
    const cause = escapeHtml(op.obstruction_cause || 'STRUWITE_CRYSTALLINE_MATRIX_PLUG');
    const status = escapeHtml(op.status || 'HYPERKALEMIC_BRADYCARDIA_ATRIAL_STANDSTILL_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const potassiumMmol = Number(op.serum_potassium_mmol_l ?? 8.5).toFixed(1);
    const heartRate = Number(op.heart_rate_bpm ?? 120);
    const bladderCm = Number(op.bladder_palpation_diameter_cm ?? 9.0).toFixed(1);
    const warningNote = escapeHtml(op.cardioprotection_warning_note || '');
    const protocol = escapeHtml(op.required_unblocking_protocol || '');

    const flags = [];
    if (op.absent_p_wave_tented_t_wave) {
      flags.push('ABSENT_PWAVE_ATRIAL_STANDSTILL');
    }
    if (Number(op.serum_potassium_mmol_l) >= 7.0) {
      flags.push('CRITICAL_HYPERKALEMIC_CARDIOTOXICITY');
    }
    if (Number(op.heart_rate_bpm) < 140) {
      flags.push('FELINE_HYPERKALEMIC_BRADYCARDIA');
    }
    if (!op.calcium_gluconate_before_sedation) {
      flags.push('SEDATION_WITHOUT_CALCIUM_HAZARD');
    }
    if (!op.manual_bladder_expression_avoided) {
      flags.push('MANUAL_BLADDER_RUPTURE_HAZARD');
    }

    const flagsHtml = flags
      .map((f) => `<span class="badge badge-urethral-flag">${escapeHtml(f)}</span>`)
      .join('');

    const clearanceBannerHtml = op.clearance_token
      ? `
          <div class="urethral-clearance-banner">
            ✅ INDWELLING CLOSED DIURESIS &amp; NORMOKALEMIC CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
          </div>
        `
      : '';

    const isSafe = op.status === 'INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const desc = escapeHtml(step.description || '');
        const badgeClass = done ? 'badge-urethral-safe' : 'badge-urethral-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="urethral-step-item">
            <input
              type="checkbox"
              class="urethral-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="urethral-step-title">${title}</strong>
            <span class="urethral-step-desc">${desc}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary urethral-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Indwelling Diuresis &amp; Normokalemic Clearance (${opId})
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card urethral-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-obstruction-cause="${cause}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-urethral-cause">${cause}</span>
            <span class="badge badge-urethral-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row urethral-metrics-row">
          <span class="badge badge-urethral-cause">Serum K+: ${potassiumMmol} mmol/L • Heart Rate: ${heartRate} bpm • Bladder Diameter: ${bladderCm} cm</span>
        </div>

        <div class="urethral-flags-row">
          ${flagsHtml}
        </div>

        <div class="urethral-cardioprotect-alert-banner">
          ⚠️ ${warningNote}
        </div>

        ${clearanceBannerHtml}

        <div class="urethral-protocol-recommendations">
          <div><strong>Required Unblocking Protocol:</strong> ${protocol}</div>
        </div>

        <div class="urethral-steps-list" aria-label="10% Calcium Gluconate, Sacrococcygeal Epidural, Closed Collection Diuresis &amp; Normokalemic Clearance Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary urethral-select-workspace-btn"
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

    const countBadge = document.getElementById('urethral-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('urethral-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="urethral-ops-empty-msg">No feline urethral obstruction triage operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const causeFilter = (document.getElementById('filter-obstruction-cause')?.value || '').trim();
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const params = new URLSearchParams();
      if (causeFilter) params.set('obstruction_cause', causeFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/urethral-obstruction/operations?${qs}`
        : '/api/v1/urethral-obstruction/operations';

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
    const res = await fetch('/api/v1/urethral-obstruction/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      selectedOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated urethral obstruction operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error || 'Failed to evaluate urethral obstruction triage operation';
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
    setVal('eval-obstruction-cause', preset.obstruction_cause);
    setVal('eval-potassium-mmol', preset.serum_potassium_mmol_l);
    setVal('eval-heart-rate', preset.heart_rate_bpm);
    setVal('eval-bladder-cm', preset.bladder_palpation_diameter_cm);
    setCheck('eval-absent-pwave', preset.absent_p_wave_tented_t_wave);
    setCheck('eval-calcium-before-sedation', preset.calcium_gluconate_before_sedation);
    setCheck('eval-manual-expression-avoided', preset.manual_bladder_expression_avoided);
  }

  function setupFormAndFilters() {
    const form = document.getElementById('urethral-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petName = (document.getElementById('eval-pet-name')?.value || '').trim();
        const species = (document.getElementById('eval-species')?.value || 'Cat - Domestic Shorthair Neutered Male').trim();
        const weightKg = parseFloat(document.getElementById('eval-weight-kg')?.value || '0');
        const obstructionCause = (document.getElementById('eval-obstruction-cause')?.value || 'STRUWITE_CRYSTALLINE_MATRIX_PLUG').trim();
        const potassiumMmol = parseFloat(document.getElementById('eval-potassium-mmol')?.value || '0');
        const heartRate = parseInt(document.getElementById('eval-heart-rate')?.value || '0', 10);
        const bladderCm = parseFloat(document.getElementById('eval-bladder-cm')?.value || '0');
        const absentPWave = Boolean(document.getElementById('eval-absent-pwave')?.checked);
        const calciumBeforeSedation = Boolean(document.getElementById('eval-calcium-before-sedation')?.checked);
        const manualExpressionAvoided = Boolean(document.getElementById('eval-manual-expression-avoided')?.checked);

        if (!petName) {
          showErrorBanner('pet_name is required');
          return;
        }

        const payload = {
          pet_name: petName,
          species,
          weight_kg: isNaN(weightKg) ? 0 : weightKg,
          obstruction_cause: obstructionCause,
          serum_potassium_mmol_l: isNaN(potassiumMmol) ? 0 : potassiumMmol,
          heart_rate_bpm: isNaN(heartRate) ? 0 : heartRate,
          bladder_palpation_diameter_cm: isNaN(bladderCm) ? 0 : bladderCm,
          absent_p_wave_tented_t_wave: absentPWave,
          calcium_gluconate_before_sedation: calciumBeforeSedation,
          manual_bladder_expression_avoided: manualExpressionAvoided,
        };

        const btn = document.getElementById('btn-submit-urethral-eval');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.urethral-scenario-btn');
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

    const filterCause = document.getElementById('filter-obstruction-cause');
    if (filterCause) {
      filterCause.addEventListener('change', () => {
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
    const listEl = document.getElementById('urethral-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.urethral-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/urethral-obstruction/operations/${encodeURIComponent(opId)}/steps`,
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
      const workspaceBtn = e.target.closest('.urethral-select-workspace-btn');
      if (workspaceBtn) {
        const opId = workspaceBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const clearanceBtn = e.target.closest('.urethral-issue-clearance-btn');
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
                `/api/v1/urethral-obstruction/operations/${encodeURIComponent(opId)}/steps`,
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
          `/api/v1/urethral-obstruction/operations/${encodeURIComponent(opId)}/clearance`,
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
            `Issued indwelling diuresis & normokalemic clearance ${updated.clearance_token} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue urethral clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing urethral clearance', 'error');
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
