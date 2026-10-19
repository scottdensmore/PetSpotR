/**
 * PetSpotR Fireworks/Thunderstorm Acoustic Panic, Bolt-Trajectory Vectoring &
 * Calming Pheromone SAR Cockpit Controller (Milestone 17.5)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'AP-OP-101';

  const SCENARIO_PRESETS = {
    'july4-fireworks-breach': {
      pet_id: 'PET-AP-204',
      pet_name: 'Ranger',
      species: 'Dog - Belgian Malinois',
      weight_kg: 28.4,
      trigger_type: 'JULY4_NYE_FIREWORKS_BARRAGE',
      peak_decibels_db: 136.0,
      barometric_drop_hpa: 2.0,
      bolt_heading_degrees: 225,
      barrier_breach_injury_suspected: true,
      no_chase_passive_lure_enforced: true,
      adaptil_feliway_station_staged: true,
    },
    'thunderstorm-barometric-drop': {
      pet_id: 'PET-AP-205',
      pet_name: 'Freya',
      species: 'Dog - Greyhound',
      weight_kg: 25.0,
      trigger_type: 'SEVERE_THUNDERSTORM_BAROMETRIC_DROP',
      peak_decibels_db: 112.0,
      barometric_drop_hpa: 14.2,
      bolt_heading_degrees: 110,
      barrier_breach_injury_suspected: false,
      no_chase_passive_lure_enforced: true,
      adaptil_feliway_station_staged: true,
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
    const liveStatus = document.getElementById('acoustic-panic-live-status');
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
      case 'ADRENAL_FLIGHT_BOLT_CRITICAL':
        return 'badge-acoustic-critical';
      case 'CALMING_PERIMETER_SNIFF_STAGED':
        return 'badge-acoustic-staged';
      case 'DECOMPRESSED_CONTAINED_SAFE':
        return 'badge-acoustic-safe';
      default:
        return 'badge-acoustic-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-bolt');
    const stagedEl = document.getElementById('kpi-staged-lure');
    const safeEl = document.getElementById('kpi-decompressed-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'ADRENAL_FLIGHT_BOLT_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'CALMING_PERIMETER_SNIFF_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'DECOMPRESSED_CONTAINED_SAFE'
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
        '<p class="trust-empty-hint">No active acoustic panic trajectory operation selected.</p>';
      return;
    }

    const active =
      ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;

    const opId = escapeHtml(active.id || '');
    const petId = escapeHtml(active.pet_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(active.species || '');
    const weightKg = Number(active.weight_kg ?? 18.0).toFixed(1);
    const triggerType = escapeHtml(active.trigger_type || '');
    const status = escapeHtml(active.status || '');
    const statusClass = getStatusBadgeClass(active.status);
    const advisory = escapeHtml(active.adrenal_no_chase_advisory || '');
    const corridor = escapeHtml(active.predicted_refuge_corridor || '');
    const clearance = active.clearance_token
      ? `<div class="acoustic-clearance-banner">✅ ACOUSTIC DECOMPRESSION &amp; CONTAINMENT CLEARANCE VERIFIED: ${escapeHtml(active.clearance_token)}</div>`
      : '';

    workspaceEl.innerHTML = `
      <div class="acoustic-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">Active Workspace: ${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${petId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-acoustic-trigger">${triggerType}</span>
            <span class="badge badge-acoustic-status ${statusClass}">${status}</span>
          </div>
        </div>
        <div class="acoustic-advisory-alert-banner">
          🔇 ${advisory}
        </div>
        ${clearance}
        <div class="acoustic-corridor-recommendations">
          <div><strong>Predicted Acoustic-Shadow Refuge Corridor:</strong> ${corridor}</div>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const weightKg = Number(op.weight_kg ?? 18.0).toFixed(1);
    const triggerType = escapeHtml(op.trigger_type || 'JULY4_NYE_FIREWORKS_BARRAGE');
    const status = escapeHtml(op.status || 'ADRENAL_FLIGHT_BOLT_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const peakDb = Number(op.peak_decibels_db ?? 120.0).toFixed(1);
    const barometricHpa = Number(op.barometric_drop_hpa ?? 2.0).toFixed(1);
    const headingDeg = Number(op.bolt_heading_degrees ?? 180);
    const advisory = escapeHtml(op.adrenal_no_chase_advisory || '');
    const corridor = escapeHtml(op.predicted_refuge_corridor || '');

    const flags = [];
    if (op.barrier_breach_injury_suspected) {
      flags.push('BARRIER_BREACH_CLAW_DENTAL_LACERATION');
    }
    if (!op.no_chase_passive_lure_enforced) {
      flags.push('ACTIVE_CHASE_TRAFFIC_BOLT_HAZARD');
    }
    if (Number(op.peak_decibels_db) >= 115.0) {
      flags.push('EXTREME_ACOUSTIC_DECIBEL_SHOCK');
    }
    if (Number(op.barometric_drop_hpa) >= 12.0) {
      flags.push('RAPID_BAROMETRIC_ELECTROSTATIC_PANIC');
    }

    const flagsHtml = flags
      .map((f) => `<span class="badge badge-acoustic-flag">${escapeHtml(f)}</span>`)
      .join('');

    const clearanceBannerHtml = op.clearance_token
      ? `
          <div class="acoustic-clearance-banner">
            ✅ ACOUSTIC DECOMPRESSION &amp; CONTAINMENT CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
          </div>
        `
      : '';

    const isSafe = op.status === 'DECOMPRESSED_CONTAINED_SAFE';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const desc = escapeHtml(step.description || '');
        const badgeClass = done ? 'badge-acoustic-safe' : 'badge-acoustic-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="acoustic-step-item">
            <input
              type="checkbox"
              class="acoustic-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="acoustic-step-title">${title}</strong>
            <span class="acoustic-step-desc">${desc}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary acoustic-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Acoustic Containment Clearance (${opId})
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card acoustic-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-trigger-type="${triggerType}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-acoustic-trigger">${triggerType}</span>
            <span class="badge badge-acoustic-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row acoustic-metrics-row">
          <span class="badge badge-acoustic-trigger">Peak Impulse: ${peakDb} dB • Barometric Drop: ${barometricHpa} hPa • Bolt Heading: ${headingDeg}°</span>
        </div>

        <div class="acoustic-flags-row">
          ${flagsHtml}
        </div>

        <div class="acoustic-advisory-alert-banner">
          ⚠️ ${advisory}
        </div>

        ${clearanceBannerHtml}

        <div class="acoustic-corridor-recommendations">
          <div><strong>Predicted Refuge Corridor:</strong> ${corridor}</div>
        </div>

        <div class="acoustic-steps-list" aria-label="Silent No-Chase, Calming Pheromone Lure &amp; Acoustic Decompression Protocol Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary acoustic-select-workspace-btn"
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

    const countBadge = document.getElementById('acoustic-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('acoustic-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="acoustic-ops-empty-msg">No acoustic panic trajectory operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const triggerFilter = (document.getElementById('filter-trigger-type')?.value || '').trim();
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const params = new URLSearchParams();
      if (triggerFilter) params.set('trigger_type', triggerFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/acoustic-panic/operations?${qs}`
        : '/api/v1/acoustic-panic/operations';

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
    const res = await fetch('/api/v1/acoustic-panic/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      selectedOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated acoustic panic operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error || 'Failed to evaluate acoustic panic trajectory operation';
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
    setVal('eval-trigger-type', preset.trigger_type);
    setVal('eval-peak-db', preset.peak_decibels_db);
    setVal('eval-barometric-hpa', preset.barometric_drop_hpa);
    setVal('eval-heading-deg', preset.bolt_heading_degrees);
    setCheck('eval-barrier-breach', preset.barrier_breach_injury_suspected);
    setCheck('eval-no-chase-enforced', preset.no_chase_passive_lure_enforced);
    setCheck('eval-pheromone-staged', preset.adaptil_feliway_station_staged);
  }

  function setupFormAndFilters() {
    const form = document.getElementById('acoustic-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petName = (document.getElementById('eval-pet-name')?.value || '').trim();
        const species = (document.getElementById('eval-species')?.value || 'Dog - Mixed Breed').trim();
        const weightKg = parseFloat(document.getElementById('eval-weight-kg')?.value || '0');
        const triggerType = (document.getElementById('eval-trigger-type')?.value || 'JULY4_NYE_FIREWORKS_BARRAGE').trim();
        const peakDb = parseFloat(document.getElementById('eval-peak-db')?.value || '0');
        const barometricHpa = parseFloat(document.getElementById('eval-barometric-hpa')?.value || '0');
        const headingDeg = parseInt(document.getElementById('eval-heading-deg')?.value || '0', 10);
        const barrierBreach = Boolean(document.getElementById('eval-barrier-breach')?.checked);
        const noChaseEnforced = Boolean(document.getElementById('eval-no-chase-enforced')?.checked);
        const pheromoneStaged = Boolean(document.getElementById('eval-pheromone-staged')?.checked);

        if (!petName) {
          showErrorBanner('pet_name is required');
          return;
        }

        const payload = {
          pet_name: petName,
          species,
          weight_kg: isNaN(weightKg) ? 0 : weightKg,
          trigger_type: triggerType,
          peak_decibels_db: isNaN(peakDb) ? 0 : peakDb,
          barometric_drop_hpa: isNaN(barometricHpa) ? 0 : barometricHpa,
          bolt_heading_degrees: isNaN(headingDeg) ? 0 : headingDeg,
          barrier_breach_injury_suspected: barrierBreach,
          no_chase_passive_lure_enforced: noChaseEnforced,
          adaptil_feliway_station_staged: pheromoneStaged,
        };

        const btn = document.getElementById('btn-submit-acoustic-eval');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.acoustic-scenario-btn');
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

    const filterTrigger = document.getElementById('filter-trigger-type');
    if (filterTrigger) {
      filterTrigger.addEventListener('change', () => {
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
    const listEl = document.getElementById('acoustic-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.acoustic-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/acoustic-panic/operations/${encodeURIComponent(opId)}/steps`,
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
      const workspaceBtn = e.target.closest('.acoustic-select-workspace-btn');
      if (workspaceBtn) {
        const opId = workspaceBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const clearanceBtn = e.target.closest('.acoustic-issue-clearance-btn');
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
                `/api/v1/acoustic-panic/operations/${encodeURIComponent(opId)}/steps`,
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
          `/api/v1/acoustic-panic/operations/${encodeURIComponent(opId)}/clearance`,
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
            `Issued acoustic containment clearance ${updated.clearance_token} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue acoustic clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing acoustic clearance', 'error');
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
