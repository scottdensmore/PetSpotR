/**
 * PetSpotR Hyperthermia, Heatstroke & Hot-Asphalt Pad Burn Triage
 * Cockpit Controller (Milestone 17.2)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'HT-OP-101';

  const SCENARIO_PRESETS = {
    'solar-cabin-heatstroke': {
      pet_id: 'PET-HT-204',
      pet_name: 'Brutus',
      species: 'Dog - English Bulldog',
      weight_kg: 24.5,
      exposure_type: 'ENCLOSED_VEHICLE_SOLAR_CABIN',
      rectal_core_temp_celsius: 41.9,
      surface_asphalt_temp_celsius: 66.0,
      exposure_duration_mins: 38,
      paw_pad_sloughing_grade: 2,
      brachycephalic_airway_obstruction: true,
      cool_water_convection_fan_active: true,
      ice_bath_vasoconstriction_avoided: true,
    },
    'asphalt-paw-sloughing': {
      pet_id: 'PET-HT-205',
      pet_name: 'Sienna',
      species: 'Dog - Vizsla',
      weight_kg: 22.0,
      exposure_type: 'HOT_ASPHALT_PAW_NECROSIS',
      rectal_core_temp_celsius: 40.3,
      surface_asphalt_temp_celsius: 71.0,
      exposure_duration_mins: 24,
      paw_pad_sloughing_grade: 3,
      brachycephalic_airway_obstruction: false,
      cool_water_convection_fan_active: true,
      ice_bath_vasoconstriction_avoided: true,
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
    const liveStatus = document.getElementById('heat-triage-live-status');
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
      case 'HEATSTROKE_DIC_COAGULOPATHY_CRITICAL':
        return 'badge-heat-critical';
      case 'EVAPORATIVE_COOLING_394C_STAGED':
        return 'badge-heat-staged';
      case 'STABILIZED_NORMOTHERMIC_COOLED':
        return 'badge-heat-safe';
      default:
        return 'badge-heat-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-dic');
    const stagedEl = document.getElementById('kpi-staged-cooling');
    const safeEl = document.getElementById('kpi-stabilized-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'HEATSTROKE_DIC_COAGULOPATHY_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'EVAPORATIVE_COOLING_394C_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'STABILIZED_NORMOTHERMIC_COOLED'
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
        '<p class="trust-empty-hint">No active heat triage operation selected.</p>';
      return;
    }

    const active =
      ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;

    const opId = escapeHtml(active.id || '');
    const petId = escapeHtml(active.pet_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(active.species || '');
    const weightKg = Number(active.weight_kg ?? 20.0).toFixed(1);
    const exposure = escapeHtml(active.exposure_type || '');
    const status = escapeHtml(active.status || '');
    const statusClass = getStatusBadgeClass(active.status);
    const cutoffNote = escapeHtml(active.coagulopathy_cutoff_note || '');
    const coolingProto = escapeHtml(active.required_cooling_protocol || '');
    const clearance = active.clearance_token
      ? `<div class="heat-clearance-banner">✅ NORMOTHERMIC COOLING &amp; PAW PAD CLEARANCE VERIFIED: ${escapeHtml(active.clearance_token)}</div>`
      : '';

    workspaceEl.innerHTML = `
      <div class="heat-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">Active Workspace: ${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${petId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-heat-exposure">${exposure}</span>
            <span class="badge badge-heat-status ${statusClass}">${status}</span>
          </div>
        </div>
        <div class="heat-cutoff-alert-banner">
          🌡️ ${cutoffNote}
        </div>
        ${clearance}
        <div class="heat-gear-recommendations">
          <div><strong>Required Cooling &amp; Debridement Protocol:</strong> ${coolingProto}</div>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const weightKg = Number(op.weight_kg ?? 20.0).toFixed(1);
    const exposureType = escapeHtml(op.exposure_type || 'ENCLOSED_VEHICLE_SOLAR_CABIN');
    const status = escapeHtml(op.status || 'HEATSTROKE_DIC_COAGULOPATHY_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const rectalTemp = Number(op.rectal_core_temp_celsius ?? 40.5).toFixed(1);
    const surfaceTemp = Number(op.surface_asphalt_temp_celsius ?? 60.0).toFixed(1);
    const durationMins = Number(op.exposure_duration_mins ?? 25);
    const pawGrade = Number(op.paw_pad_sloughing_grade ?? 0);
    const cutoffNote = escapeHtml(op.coagulopathy_cutoff_note || '');
    const coolingProtocol = escapeHtml(op.required_cooling_protocol || '');

    const flags = [];
    if (op.brachycephalic_airway_obstruction) {
      flags.push('BRACHYCEPHALIC_AIRWAY_OBSTRUCTION');
    }
    if (Number(op.rectal_core_temp_celsius) >= 41.0) {
      flags.push('HYPERPYREXIA_DIC_COAGULOPATHY_RISK');
    }
    if (pawGrade >= 2) {
      flags.push(`PAW_PAD_EPIDERMAL_SLOUGHING_GRADE_${pawGrade}`);
    }
    if (!op.ice_bath_vasoconstriction_avoided) {
      flags.push('ICE_BATH_PERIPHERAL_VASOCONSTRICTION_HAZARD');
    }

    const flagsHtml = flags
      .map((f) => `<span class="badge badge-heat-flag">${escapeHtml(f)}</span>`)
      .join('');

    const clearanceBannerHtml = op.clearance_token
      ? `
          <div class="heat-clearance-banner">
            ✅ NORMOTHERMIC COOLING &amp; PAW PAD CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
          </div>
        `
      : '';

    const isSafe = op.status === 'STABILIZED_NORMOTHERMIC_COOLED';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const desc = escapeHtml(step.description || '');
        const badgeClass = done ? 'badge-heat-safe' : 'badge-heat-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="heat-step-item">
            <input
              type="checkbox"
              class="heat-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="heat-step-title">${title}</strong>
            <span class="heat-step-desc">${desc}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary heat-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Normothermic Cooling Clearance (${opId})
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card heat-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-exposure="${exposureType}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-heat-exposure">${exposureType}</span>
            <span class="badge badge-heat-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row heat-metrics-row">
          <span class="badge badge-heat-exposure">Rectal Core: ${rectalTemp}°C • Surface/Cabin: ${surfaceTemp}°C • Duration: ${durationMins}m • Paw Sloughing: Grade ${pawGrade}/4</span>
        </div>

        <div class="heat-flags-row">
          ${flagsHtml}
        </div>

        <div class="heat-cutoff-alert-banner">
          ⚠️ ${cutoffNote}
        </div>

        ${clearanceBannerHtml}

        <div class="heat-gear-recommendations">
          <div><strong>Required Cooling Protocol:</strong> ${coolingProtocol}</div>
        </div>

        <div class="heat-steps-list" aria-label="Evaporative Cooling, 39.4°C Cutoff &amp; Paw Pad Burn Protocol Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary heat-select-workspace-btn"
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

    const countBadge = document.getElementById('heat-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('heat-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="heat-ops-empty-msg">No hyperthermia heat triage operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const exposureFilter = (document.getElementById('filter-exposure')?.value || '').trim();
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const params = new URLSearchParams();
      if (exposureFilter) params.set('exposure', exposureFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/heat-triage/operations?${qs}`
        : '/api/v1/heat-triage/operations';

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
    const res = await fetch('/api/v1/heat-triage/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      selectedOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated hyperthermia operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error || 'Failed to evaluate hyperthermia heat triage operation';
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
    setVal('eval-exposure-type', preset.exposure_type);
    setVal('eval-rectal-temp-c', preset.rectal_core_temp_celsius);
    setVal('eval-surface-temp-c', preset.surface_asphalt_temp_celsius);
    setVal('eval-exposure-mins', preset.exposure_duration_mins);
    setVal('eval-paw-sloughing-grade', preset.paw_pad_sloughing_grade);
    setCheck('eval-brachycephalic-airway', preset.brachycephalic_airway_obstruction);
    setCheck('eval-convection-fan', preset.cool_water_convection_fan_active);
    setCheck('eval-ice-bath-avoided', preset.ice_bath_vasoconstriction_avoided);
  }

  function setupFormAndFilters() {
    const form = document.getElementById('heat-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petName = (document.getElementById('eval-pet-name')?.value || '').trim();
        const species = (document.getElementById('eval-species')?.value || 'Dog - Mixed Breed').trim();
        const weightKg = parseFloat(document.getElementById('eval-weight-kg')?.value || '0');
        const exposureType = (document.getElementById('eval-exposure-type')?.value || 'ENCLOSED_VEHICLE_SOLAR_CABIN').trim();
        const rectalTemp = parseFloat(document.getElementById('eval-rectal-temp-c')?.value || '0');
        const surfaceTemp = parseFloat(document.getElementById('eval-surface-temp-c')?.value || '0');
        const durationMins = parseInt(document.getElementById('eval-exposure-mins')?.value || '0', 10);
        const pawGrade = parseInt(document.getElementById('eval-paw-sloughing-grade')?.value || '0', 10);
        const brachy = Boolean(document.getElementById('eval-brachycephalic-airway')?.checked);
        const fanActive = Boolean(document.getElementById('eval-convection-fan')?.checked);
        const iceAvoided = Boolean(document.getElementById('eval-ice-bath-avoided')?.checked);

        if (!petName) {
          showErrorBanner('pet_name is required');
          return;
        }

        const payload = {
          pet_name: petName,
          species,
          weight_kg: isNaN(weightKg) ? 0 : weightKg,
          exposure_type: exposureType,
          rectal_core_temp_celsius: isNaN(rectalTemp) ? 0 : rectalTemp,
          surface_asphalt_temp_celsius: isNaN(surfaceTemp) ? 0 : surfaceTemp,
          exposure_duration_mins: isNaN(durationMins) ? 0 : durationMins,
          paw_pad_sloughing_grade: isNaN(pawGrade) ? 0 : pawGrade,
          brachycephalic_airway_obstruction: brachy,
          cool_water_convection_fan_active: fanActive,
          ice_bath_vasoconstriction_avoided: iceAvoided,
        };

        const btn = document.getElementById('btn-submit-heat-eval');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.heat-scenario-btn');
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

    const filterExposure = document.getElementById('filter-exposure');
    if (filterExposure) {
      filterExposure.addEventListener('change', () => {
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
    const listEl = document.getElementById('heat-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.heat-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/heat-triage/operations/${encodeURIComponent(opId)}/steps`,
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
      const workspaceBtn = e.target.closest('.heat-select-workspace-btn');
      if (workspaceBtn) {
        const opId = workspaceBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const clearanceBtn = e.target.closest('.heat-issue-clearance-btn');
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
                `/api/v1/heat-triage/operations/${encodeURIComponent(opId)}/steps`,
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
          `/api/v1/heat-triage/operations/${encodeURIComponent(opId)}/clearance`,
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
            `Issued normothermic cooling clearance ${updated.clearance_token} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue heat clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing heat clearance', 'error');
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
