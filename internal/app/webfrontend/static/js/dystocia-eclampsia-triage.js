/**
 * PetSpotR Dystocia, Maternal C-Section, Eclampsia (Hypocalcemia)
 * & Oxytocin Contraindication Triage Cockpit Controller (Milestone 18.4)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'DE-OP-101';

  const SCENARIO_PRESETS = {
    'obstructive-csection': {
      pet_id: 'PET-DE-204',
      pet_name: 'Duchess',
      species: 'Dog - French Bulldog Dam',
      weight_kg: 11.4,
      presentation_type: 'OBSTRUCTIVE_BRACHYCEPHALIC_FETAL_OVERSIZE',
      lowest_fetal_heart_rate_bpm: 148,
      ionized_calcium_mmol_l: 0.82,
      inter_puppy_interval_mins: 160,
      green_black_uteroverdin_no_fetus: true,
      pelvic_canal_obstruction_excluded: false,
      ecg_monitored_calcium_infusion: true,
    },
    'puerperal-eclampsia': {
      pet_id: 'PET-DE-205',
      pet_name: 'Chloe',
      species: 'Dog - Chihuahua Dam',
      weight_kg: 3.2,
      presentation_type: 'PUERPERAL_ECLAMPSIA_HYPOCALCEMIA_TETANY',
      lowest_fetal_heart_rate_bpm: 196,
      ionized_calcium_mmol_l: 0.68,
      inter_puppy_interval_mins: 45,
      green_black_uteroverdin_no_fetus: false,
      pelvic_canal_obstruction_excluded: true,
      ecg_monitored_calcium_infusion: true,
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
    const liveStatus = document.getElementById('dystocia-triage-live-status');
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
      case 'FETAL_DISTRESS_OBSTRUCTIVE_CRITICAL':
        return 'badge-dystocia-critical';
      case 'CALCIUM_ECG_WHELPING_STAGED':
        return 'badge-dystocia-staged';
      case 'DELIVERED_NORMOCALCEMIC_LACTATING_SAFE':
        return 'badge-dystocia-safe';
      default:
        return 'badge-dystocia-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-fetal');
    const stagedEl = document.getElementById('kpi-staged-whelping');
    const safeEl = document.getElementById('kpi-delivered-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'FETAL_DISTRESS_OBSTRUCTIVE_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'CALCIUM_ECG_WHELPING_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'DELIVERED_NORMOCALCEMIC_LACTATING_SAFE'
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
        '<p class="trust-empty-hint">No active dystocia or eclampsia triage operation selected.</p>';
      return;
    }

    const active =
      ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;

    const opId = escapeHtml(active.id || '');
    const petId = escapeHtml(active.pet_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(active.species || '');
    const weightKg = Number(active.weight_kg ?? 10.0).toFixed(1);
    const presentation = escapeHtml(active.presentation_type || '');
    const status = escapeHtml(active.status || '');
    const statusClass = getStatusBadgeClass(active.status);
    const contraNote = escapeHtml(active.oxytocin_contraindication_note || '');
    const obstetricProtocol = escapeHtml(active.required_obstetric_protocol || '');
    const clearance = active.clearance_token
      ? `<div class="dystocia-clearance-banner">✅ DELIVERED NORMOCALCEMIC &amp; LACTATING CLEARANCE VERIFIED: ${escapeHtml(active.clearance_token)}</div>`
      : '';

    workspaceEl.innerHTML = `
      <div class="dystocia-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">Active Workspace: ${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${petId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-dystocia-presentation">${presentation}</span>
            <span class="badge badge-dystocia-status ${statusClass}">${status}</span>
          </div>
        </div>
        <div class="dystocia-oxytocin-alert-banner">
          ⚠️ ${contraNote}
        </div>
        ${clearance}
        <div class="dystocia-protocol-recommendations">
          <div><strong>Required Obstetric &amp; Calcium Protocol:</strong> ${obstetricProtocol}</div>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine Dam');
    const weightKg = Number(op.weight_kg ?? 10.0).toFixed(1);
    const presentationType = escapeHtml(
      op.presentation_type || 'OBSTRUCTIVE_BRACHYCEPHALIC_FETAL_OVERSIZE'
    );
    const status = escapeHtml(
      op.status || 'FETAL_DISTRESS_OBSTRUCTIVE_CRITICAL'
    );
    const statusClass = getStatusBadgeClass(op.status);
    const fetalHr = Number(op.lowest_fetal_heart_rate_bpm ?? 160);
    const ionizedCa = Number(op.ionized_calcium_mmol_l ?? 1.1).toFixed(2);
    const intervalMins = Number(op.inter_puppy_interval_mins ?? 120);
    const contraNote = escapeHtml(op.oxytocin_contraindication_note || '');
    const obstetricProtocol = escapeHtml(op.required_obstetric_protocol || '');

    const flags = [];
    if (op.green_black_uteroverdin_no_fetus) {
      flags.push('UTEROVERDIN_PLACENTAL_DETACHMENT');
    }
    if (fetalHr < 180) {
      flags.push('FETAL_BRADYCARDIA_DISTRESS');
    }
    if (!op.pelvic_canal_obstruction_excluded) {
      flags.push('OXYTOCIN_CONTRAINDICATED_OBSTRUCTION');
    }
    if (Number(op.ionized_calcium_mmol_l) < 0.9) {
      flags.push('ECLAMPSIA_HYPOCALCEMIC_TETANY');
    }

    const flagsHtml = flags
      .map((f) => `<span class="badge badge-dystocia-flag">${escapeHtml(f)}</span>`)
      .join('');

    const clearanceBannerHtml = op.clearance_token
      ? `
          <div class="dystocia-clearance-banner">
            ✅ DELIVERED NORMOCALCEMIC &amp; LACTATING CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
          </div>
        `
      : '';

    const isSafe = op.status === 'DELIVERED_NORMOCALCEMIC_LACTATING_SAFE';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const desc = escapeHtml(step.description || '');
        const badgeClass = done ? 'badge-dystocia-safe' : 'badge-dystocia-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="dystocia-step-item">
            <input
              type="checkbox"
              class="dystocia-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="dystocia-step-title">${title}</strong>
            <span class="dystocia-step-desc">${desc}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary dystocia-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Delivered Normocalcemic Clearance (${opId})
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card dystocia-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-presentation-type="${presentationType}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-dystocia-presentation">${presentationType}</span>
            <span class="badge badge-dystocia-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row dystocia-metrics-row">
          <span class="badge badge-dystocia-presentation">Lowest Fetal HR: ${fetalHr} bpm • Ionized Ca (iCa): ${ionizedCa} mmol/L • Inter-Puppy Interval: ${intervalMins} mins</span>
        </div>

        <div class="dystocia-flags-row">
          ${flagsHtml}
        </div>

        <div class="dystocia-oxytocin-alert-banner">
          ⚠️ ${contraNote}
        </div>

        ${clearanceBannerHtml}

        <div class="dystocia-protocol-recommendations">
          <div><strong>Required Obstetric Protocol:</strong> ${obstetricProtocol}</div>
        </div>

        <div class="dystocia-steps-list" aria-label="Doppler Fetal HR, ECG Calcium Gluconate &amp; C-Section Protocol Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary dystocia-select-workspace-btn"
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

    const countBadge = document.getElementById('dystocia-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('dystocia-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="dystocia-ops-empty-msg">No dystocia, maternal C-section, or eclampsia triage operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const presentationFilter = (
        document.getElementById('filter-presentation-type')?.value || ''
      ).trim();
      const statusFilter = (
        document.getElementById('filter-status')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (presentationFilter) params.set('presentation_type', presentationFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/dystocia-eclampsia/operations?${qs}`
        : '/api/v1/dystocia-eclampsia/operations';

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
    const res = await fetch('/api/v1/dystocia-eclampsia/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      selectedOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated dystocia & eclampsia operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg =
      errData.error || 'Failed to evaluate dystocia & eclampsia triage operation';
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
    setVal('eval-fetal-hr', preset.lowest_fetal_heart_rate_bpm);
    setVal('eval-ionized-calcium', preset.ionized_calcium_mmol_l);
    setVal('eval-interval-mins', preset.inter_puppy_interval_mins);
    setCheck('eval-uteroverdin', preset.green_black_uteroverdin_no_fetus);
    setCheck('eval-obstruction-excluded', preset.pelvic_canal_obstruction_excluded);
    setCheck('eval-ecg-calcium', preset.ecg_monitored_calcium_infusion);
  }

  function setupFormAndFilters() {
    const form = document.getElementById('dystocia-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petName = (document.getElementById('eval-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('eval-species')?.value || 'Dog - Mixed Breed Dam'
        ).trim();
        const weightKg = parseFloat(document.getElementById('eval-weight-kg')?.value || '0');
        const presentationType = (
          document.getElementById('eval-presentation-type')?.value ||
          'OBSTRUCTIVE_BRACHYCEPHALIC_FETAL_OVERSIZE'
        ).trim();
        const fetalHr = parseInt(document.getElementById('eval-fetal-hr')?.value || '0', 10);
        const ionizedCa = parseFloat(
          document.getElementById('eval-ionized-calcium')?.value || '0'
        );
        const intervalMins = parseInt(
          document.getElementById('eval-interval-mins')?.value || '0',
          10
        );
        const uteroverdin = Boolean(document.getElementById('eval-uteroverdin')?.checked);
        const obstructionExcluded = Boolean(
          document.getElementById('eval-obstruction-excluded')?.checked
        );
        const ecgCalcium = Boolean(document.getElementById('eval-ecg-calcium')?.checked);

        if (!petName) {
          showErrorBanner('pet_name is required');
          return;
        }

        const payload = {
          pet_name: petName,
          species,
          weight_kg: isNaN(weightKg) ? 0 : weightKg,
          presentation_type: presentationType,
          lowest_fetal_heart_rate_bpm: isNaN(fetalHr) ? 0 : fetalHr,
          ionized_calcium_mmol_l: isNaN(ionizedCa) ? 0 : ionizedCa,
          inter_puppy_interval_mins: isNaN(intervalMins) ? -1 : intervalMins,
          green_black_uteroverdin_no_fetus: uteroverdin,
          pelvic_canal_obstruction_excluded: obstructionExcluded,
          ecg_monitored_calcium_infusion: ecgCalcium,
        };

        const btn = document.getElementById('btn-submit-dystocia-eval');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.dystocia-scenario-btn');
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

    const filterPresentation = document.getElementById('filter-presentation-type');
    if (filterPresentation) {
      filterPresentation.addEventListener('change', () => {
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
    const listEl = document.getElementById('dystocia-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.dystocia-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/dystocia-eclampsia/operations/${encodeURIComponent(opId)}/steps`,
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
      const workspaceBtn = e.target.closest('.dystocia-select-workspace-btn');
      if (workspaceBtn) {
        const opId = workspaceBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const clearanceBtn = e.target.closest('.dystocia-issue-clearance-btn');
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
                `/api/v1/dystocia-eclampsia/operations/${encodeURIComponent(opId)}/steps`,
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
          `/api/v1/dystocia-eclampsia/operations/${encodeURIComponent(opId)}/clearance`,
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
            `Issued delivered normocalcemic clearance ${updated.clearance_token} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue dystocia clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing dystocia clearance', 'error');
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
