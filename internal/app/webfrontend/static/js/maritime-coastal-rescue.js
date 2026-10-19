/**
 * PetSpotR Maritime, Coastal & Tidal Mudflat Pet Rescue Operations
 * Cockpit Controller (Milestone 17.1)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'MC-OP-101';

  const SCENARIO_PRESETS = {
    'tidal-mudflat-entrapment': {
      pet_name: 'Barnacle',
      species: 'Dog - Chesapeake Bay Retriever',
      weight_kg: 36.5,
      maritime_zone: 'TIDAL_MUDFLAT_ESTUARY',
      tide_surge_rate_cm_hr: 64.0,
      swell_height_meters: 0.9,
      water_temp_celsius: 8.0,
      mud_suction_depth_cm: 50.0,
      saltwater_aspiration_suspected: true,
      mud_walkers_air_lance_deployed: false,
      canine_pfd_harness_secured: false,
    },
    'rip-current-surf-tow': {
      pet_name: 'Riptide',
      species: 'Dog - Labrador Retriever',
      weight_kg: 29.0,
      maritime_zone: 'RIP_CURRENT_SURF_BREAK',
      tide_surge_rate_cm_hr: 30.0,
      swell_height_meters: 2.8,
      water_temp_celsius: 13.0,
      mud_suction_depth_cm: 0.0,
      saltwater_aspiration_suspected: true,
      mud_walkers_air_lance_deployed: false,
      canine_pfd_harness_secured: true,
    },
    'salt-marsh-airboat': {
      pet_name: 'Marsh',
      species: 'Dog - Nova Scotia Duck Tolling Retriever',
      weight_kg: 19.5,
      maritime_zone: 'SALT_MARSH_CORDGRASS_SLOUGH',
      tide_surge_rate_cm_hr: 22.0,
      swell_height_meters: 0.4,
      water_temp_celsius: 15.5,
      mud_suction_depth_cm: 18.0,
      saltwater_aspiration_suspected: false,
      mud_walkers_air_lance_deployed: true,
      canine_pfd_harness_secured: true,
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

  function showErrorBanner(msg) {
    const banner = document.getElementById('eval-error-banner');
    if (!banner) return;
    if (!msg) {
      banner.hidden = true;
      banner.textContent = '';
      return;
    }
    banner.textContent = msg;
    banner.hidden = false;
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
    const liveStatus = document.getElementById('maritime-coastal-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function getStatusBadgeClass(status) {
    switch (status) {
      case 'TIDAL_FLOOD_ASPIRATION_CRITICAL':
        return 'badge-mc-critical';
      case 'HOVERCRAFT_MUD_SLED_STAGED':
        return 'badge-mc-staged';
      case 'EXTRACTED_HARBOR_DECK_SAFE':
        return 'badge-mc-safe';
      default:
        return 'badge-mc-critical';
    }
  }

  function updateSummaryKpis(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-tide');
    const stagedEl = document.getElementById('kpi-staged-vessel');
    const safeEl = document.getElementById('kpi-extracted-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'TIDAL_FLOOD_ASPIRATION_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'HOVERCRAFT_MUD_SLED_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'EXTRACTED_HARBOR_DECK_SAFE'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (safeEl) safeEl.textContent = String(safe);
  }

  function renderActiveWorkspace(ops) {
    const workspaceEl = document.getElementById('active-operation-workspace');
    if (!workspaceEl) return;

    if (!Array.isArray(ops) || ops.length === 0) {
      workspaceEl.innerHTML =
        '<p class="trust-empty-hint">No active maritime operation selected.</p>';
      return;
    }

    let active = ops.find((o) => o.id === selectedOpId);
    if (!active) {
      active = ops[0];
      selectedOpId = active.id;
    }

    const opId = escapeHtml(active.id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(active.species || '');
    const zone = escapeHtml(active.maritime_zone || '');
    const status = escapeHtml(active.status || '');
    const statusClass = getStatusBadgeClass(active.status);
    const vessel = escapeHtml(active.required_vessel || '');
    const advisory = escapeHtml(active.tidal_risk_advisory || '');
    const clearance = escapeHtml(active.clearance_token || '');

    let bannerClass = 'mc-staged-banner';
    if (active.status === 'TIDAL_FLOOD_ASPIRATION_CRITICAL') {
      bannerClass = 'mc-tidal-alert-banner';
    } else if (active.status === 'EXTRACTED_HARBOR_DECK_SAFE') {
      bannerClass = 'mc-clearance-banner';
    }

    workspaceEl.innerHTML = `
      <div class="mc-workspace-header" data-workspace-op-id="${opId}">
        <div>
          <h3 class="mc-workspace-title">Active Workspace: ${petName} (${species}) — <code>${opId}</code></h3>
          <p class="mc-workspace-vessel"><strong>Assigned Vessel &amp; Rig:</strong> ${vessel}</p>
        </div>
        <div class="mc-workspace-badges">
          <span class="badge badge-mc-zone">${zone}</span>
          <span class="badge badge-mc-status ${statusClass}">${status}</span>
        </div>
      </div>
      <div class="mc-advisory-banner ${bannerClass}">
        ${advisory}
      </div>
      ${
        clearance
          ? `<div class="mc-clearance-banner">✅ HARBOR DECK EXTRACTION CLEARANCE VERIFIED: ${clearance}</div>`
          : ''
      }
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Dog');
    const weightKg = Number(op.weight_kg ?? 20.0).toFixed(1);
    const zone = escapeHtml(op.maritime_zone || 'TIDAL_MUDFLAT_ESTUARY');
    const status = escapeHtml(op.status || 'TIDAL_FLOOD_ASPIRATION_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const tideSurge = Number(op.tide_surge_rate_cm_hr ?? 0).toFixed(1);
    const swell = Number(op.swell_height_meters ?? 0).toFixed(1);
    const waterTemp = Number(op.water_temp_celsius ?? 12.0).toFixed(1);
    const mudSuction = Number(op.mud_suction_depth_cm ?? 0).toFixed(1);
    const vessel = escapeHtml(op.required_vessel || '');
    const advisory = escapeHtml(op.tidal_risk_advisory || '');

    const flags = [];
    if (op.saltwater_aspiration_suspected) {
      flags.push('SALTWATER_ASPIRATION_SUSPECTED');
    }
    if (Number(op.tide_surge_rate_cm_hr) >= 45.0) {
      flags.push('EXTREME_TIDAL_FLOOD_SURGE');
    }
    if (Number(op.mud_suction_depth_cm) >= 35.0) {
      flags.push('ANAEROBIC_MUD_SUCTION_ENTRAPMENT');
    }
    if (Number(op.water_temp_celsius) <= 10.0) {
      flags.push('COLD_WATER_IMMERSION_HYPOTHERMIA');
    }
    const flagsHtml = flags
      .map((f) => `<span class="badge badge-mc-flag">${escapeHtml(f)}</span>`)
      .join('');

    const alertBannerHtml =
      op.status === 'TIDAL_FLOOD_ASPIRATION_CRITICAL'
        ? `<div class="mc-tidal-alert-banner">⚠️ ${advisory}</div>`
        : '';

    const clearanceBannerHtml = op.clearance_token
      ? `<div class="mc-clearance-banner">✅ HARBOR DECK EXTRACTION CLEARANCE VERIFIED: ${escapeHtml(
          op.clearance_token
        )}</div>`
      : '';

    const isSafe = op.status === 'EXTRACTED_HARBOR_DECK_SAFE';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const badgeClass = done ? 'badge-mc-safe' : 'badge-mc-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="mc-step-item">
            <input
              type="checkbox"
              class="mc-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="mc-step-id">[${stepId}]</strong>
            <span class="mc-step-title">${title}</span>
          </label>
        `;
      })
      .join('');

    const clearanceBtnHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary mc-issue-clearance-btn"
            data-op-id="${opId}"
          >
            ⚓ Issue Harbor Deck Safe Extraction Clearance
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card mc-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-zone="${zone}" data-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-mc-zone">${zone}</span>
            <span class="badge badge-mc-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row mc-metrics-row">
          <span class="badge badge-mc-zone">Tide Surge: ${tideSurge} cm/hr • Swell: ${swell} m • Water Temp: ${waterTemp}°C • Mud Suction: ${mudSuction} cm</span>
        </div>

        <div class="mc-flags-row">
          ${flagsHtml}
        </div>

        ${alertBannerHtml}
        ${clearanceBannerHtml}

        <div class="mc-gear-recommendations">
          <div><strong>Required Maritime Vessel &amp; Rig:</strong> ${vessel}</div>
          <div><strong>Tidal Risk Advisory:</strong> ${advisory}</div>
        </div>

        <div class="mc-steps-list" aria-label="Maritime Suction Relief, PFD Rigging, Saltwater Aspiration O2 &amp; Harbor Deck Rewarming Protocol">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary mc-select-workspace-btn"
            data-op-id="${opId}"
          >
            🔍 Inspect in Active Workspace
          </button>
          ${clearanceBtnHtml}
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops) {
    currentOps = ops;
    updateSummaryKpis(ops);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('mc-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('maritime-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="mc-ops-empty-msg">No maritime rescue operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const zoneFilter = (document.getElementById('filter-zone')?.value || '').trim();
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();

      const params = new URLSearchParams();
      if (zoneFilter) params.set('zone', zoneFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/maritime-rescue/operations?${qs}`
        : '/api/v1/maritime-rescue/operations';

      const res = await fetch(url, {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const ops = Array.isArray(data.operations) ? data.operations : [];
      renderAllOperations(ops);
      return ops;
    } catch (_err) {
      return [];
    }
  }

  async function evaluateOperation(payload) {
    showErrorBanner('');
    const res = await fetch('/api/v1/maritime-rescue/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      selectedOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated maritime rescue operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error || 'Failed to evaluate maritime rescue operation';
    showErrorBanner(msg);
    showToast(msg, 'error');
    return null;
  }

  function populateFormWithPreset(preset) {
    const setVal = (id, val) => {
      const el = document.getElementById(id);
      if (el) el.value = String(val);
    };
    const setChecked = (id, checked) => {
      const el = document.getElementById(id);
      if (el) el.checked = Boolean(checked);
    };

    setVal('eval-pet-name', preset.pet_name);
    setVal('eval-species', preset.species);
    setVal('eval-weight-kg', preset.weight_kg);
    setVal('eval-maritime-zone', preset.maritime_zone);
    setVal('eval-tide-surge-cm', preset.tide_surge_rate_cm_hr);
    setVal('eval-swell-height-m', preset.swell_height_meters);
    setVal('eval-water-temp-c', preset.water_temp_celsius);
    setVal('eval-mud-suction-cm', preset.mud_suction_depth_cm);
    setChecked('eval-saltwater-aspiration', preset.saltwater_aspiration_suspected);
    setChecked('eval-air-lance-deployed', preset.mud_walkers_air_lance_deployed);
    setChecked('eval-pfd-secured', preset.canine_pfd_harness_secured);
  }

  function setupFormAndScenarioButtons() {
    const form = document.getElementById('maritime-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petName = (document.getElementById('eval-pet-name')?.value || '').trim();
        const species = (document.getElementById('eval-species')?.value || 'Dog').trim();
        const weightKg = parseFloat(document.getElementById('eval-weight-kg')?.value || '0');
        const maritimeZone = (
          document.getElementById('eval-maritime-zone')?.value || 'TIDAL_MUDFLAT_ESTUARY'
        ).trim();
        const tideSurge = parseFloat(document.getElementById('eval-tide-surge-cm')?.value || '0');
        const swellHeight = parseFloat(document.getElementById('eval-swell-height-m')?.value || '0');
        const waterTemp = parseFloat(document.getElementById('eval-water-temp-c')?.value || '12.0');
        const mudSuction = parseFloat(document.getElementById('eval-mud-suction-cm')?.value || '0');
        const saltwaterAspiration = Boolean(
          document.getElementById('eval-saltwater-aspiration')?.checked
        );
        const airLanceDeployed = Boolean(
          document.getElementById('eval-air-lance-deployed')?.checked
        );
        const pfdSecured = Boolean(document.getElementById('eval-pfd-secured')?.checked);

        if (!petName) {
          showErrorBanner('pet_name is required');
          return;
        }
        if (isNaN(weightKg) || weightKg <= 0) {
          showErrorBanner('weight_kg must be greater than 0');
          return;
        }

        const payload = {
          pet_name: petName,
          species,
          weight_kg: weightKg,
          maritime_zone: maritimeZone,
          tide_surge_rate_cm_hr: isNaN(tideSurge) ? 0 : tideSurge,
          swell_height_meters: isNaN(swellHeight) ? 0 : swellHeight,
          water_temp_celsius: isNaN(waterTemp) ? 12.0 : waterTemp,
          mud_suction_depth_cm: isNaN(mudSuction) ? 0 : mudSuction,
          saltwater_aspiration_suspected: saltwaterAspiration,
          mud_walkers_air_lance_deployed: airLanceDeployed,
          canine_pfd_harness_secured: pfdSecured,
        };

        const btn = document.getElementById('btn-evaluate-maritime-op');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.mc-scenario-btn');
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
  }

  function setupFilters() {
    const zoneSelect = document.getElementById('filter-zone');
    const statusSelect = document.getElementById('filter-status');

    if (zoneSelect) {
      zoneSelect.addEventListener('change', () => {
        fetchOperations();
      });
    }
    if (statusSelect) {
      statusSelect.addEventListener('change', () => {
        fetchOperations();
      });
    }
  }

  function setupCardInteractions() {
    const listEl = document.getElementById('maritime-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.mc-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/maritime-rescue/operations/${encodeURIComponent(opId)}/steps`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ step_id: stepId, completed }),
          }
        );
        if (res.ok) {
          const updated = await res.json();
          selectedOpId = opId;
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
      const selectBtn = e.target.closest('.mc-select-workspace-btn');
      if (selectBtn) {
        const opId = selectBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const clearanceBtn = e.target.closest('.mc-issue-clearance-btn');
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
                `/api/v1/maritime-rescue/operations/${encodeURIComponent(opId)}/steps`,
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
          `/api/v1/maritime-rescue/operations/${encodeURIComponent(opId)}/clearance`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({}),
          }
        );
        if (res.ok) {
          const updated = await res.json();
          selectedOpId = opId;
          await fetchOperations();
          showToast(
            `Issued harbor deck clearance ${updated.clearance_token} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue maritime clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing maritime clearance', 'error');
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
    setupFormAndScenarioButtons();
    setupFilters();
    setupCardInteractions();
    fetchOperations();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
