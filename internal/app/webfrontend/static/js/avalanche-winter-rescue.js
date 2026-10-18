/**
 * PetSpotR Cold-Weather Avalanche, Deep-Snow Drift & Thermal Hypothermia
 * Rewarming SAR Cockpit Controller (Milestone 16.3)
 */

(function () {
  'use strict';

  let currentOps = [];

  const SCENARIO_PRESETS = {
    'avalanche-slab-burial': {
      petId: 'PET-AW-204',
      petName: 'Kodiak',
      species: 'Canine',
      weightKg: 29.0,
      terrainType: 'AVALANCHE_DEBRIS_CHUTE',
      burialDepthCm: 90,
      windChillCelsius: -24.0,
      coreTempCelsius: 33.4,
      minutesBuried: 26,
      avalancheDangerScale: 4,
      snowpackSectorGrid: 'CHUTE-NORTH-04A (Alpental Backcountry Bowl)',
      leadSkiPatrolCallsign: 'PATROL-ALPINE-01 (Sorensen)',
      notes: 'Simulated D3 wind-slab debris toe burial (90cm) with severe core hypothermia (33.4°C) and afterdrop risk.',
    },
    'tree-well-immersion': {
      petId: 'PET-AW-205',
      petName: 'Timber',
      species: 'Canine',
      weightKg: 20.5,
      terrainType: 'TREE_WELL_POWDER_PIT',
      burialDepthCm: 55,
      windChillCelsius: -12.0,
      coreTempCelsius: 35.8,
      minutesBuried: 14,
      avalancheDangerScale: 3,
      snowpackSectorGrid: 'GLADE-WEST-11B (Subalpine Hemlock Tree-Well Zone)',
      leadSkiPatrolCallsign: 'PATROL-ALPINE-03 (Lindqvist)',
      notes: 'Simulated canine inverted in unconsolidated hemlock tree-well powder funnel; side V-trench excavation active.',
    },
    'blizzard-drift-shelter': {
      petId: 'PET-AW-206',
      petName: 'Lumi',
      species: 'Feline',
      weightKg: 4.4,
      terrainType: 'BLIZZARD_SNOWDRIFT_TRENCH',
      burialDepthCm: 25,
      windChillCelsius: -8.5,
      coreTempCelsius: 36.8,
      minutesBuried: 8,
      avalancheDangerScale: 2,
      snowpackSectorGrid: 'DRIFT-CABIN-02C (Snoqualmie Pass Timberline)',
      leadSkiPatrolCallsign: 'PATROL-ALPINE-02 (Mercer)',
      notes: 'Simulated feline sheltered inside wind-loaded snowdrift trench; active thoracic heat-pack bivy staged.',
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
    const liveStatus = document.getElementById('avalanche-winter-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function getStatusBadgeClass(status) {
    switch (status) {
      case 'BURIAL_HYPOTHERMIA_CRITICAL':
        return 'badge-aw-critical';
      case 'PROBE_TRENCH_REWARMING_STAGED':
        return 'badge-aw-staged';
      case 'EXTRACTED_NORMOTHERMIC_SAFE':
        return 'badge-aw-safe';
      default:
        return 'badge-aw-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('stat-total-winter-ops');
    const burialEl = document.getElementById('stat-critical-burial-alerts');
    const stagedEl = document.getElementById('stat-staged-rewarm-bivies');
    const safeEl = document.getElementById('stat-normothermic-safe-count');

    const total = ops.length;
    const criticalBurial = ops.filter(
      (op) =>
        Boolean(op.asphyxiaAirPocketAlert) ||
        Boolean(op.severeFrostbiteAfterdropAlert) ||
        op.status === 'BURIAL_HYPOTHERMIA_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'PROBE_TRENCH_REWARMING_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'EXTRACTED_NORMOTHERMIC_SAFE'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (burialEl) burialEl.textContent = String(criticalBurial);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (safeEl) safeEl.textContent = String(safe);
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.operationId || '');
    const petName = escapeHtml(op.petName || '');
    const species = escapeHtml(op.species || 'Canine');
    const weightKg = Number(op.weightKg ?? 25.0).toFixed(1);
    const sectorGrid = escapeHtml(op.snowpackSectorGrid || 'ALPINE-BOWL-01A');
    const terrainType = escapeHtml(op.terrainType || 'AVALANCHE_DEBRIS_CHUTE');
    const status = escapeHtml(op.status || 'BURIAL_HYPOTHERMIA_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const hazardScore = Number(op.hazardScore ?? 50);
    const readinessScore = Number(op.rewarmingReadinessScore ?? 0);
    const burialDepth = Number(op.burialDepthCm ?? 50).toFixed(0);
    const minutesBuried = Number(op.minutesBuried ?? 15);
    const coreTemp = Number(op.coreTempCelsius ?? 35.0).toFixed(1);
    const windChill = Number(op.windChillCelsius ?? -15.0).toFixed(1);
    const avyScale = Number(op.avalancheDangerScale ?? 3);
    const locatorGear = escapeHtml(op.recommendedLocatorGear || '');
    const rewarmingBivy = escapeHtml(op.recommendedRewarmingBivy || '');
    const callsign = escapeHtml(op.leadSkiPatrolCallsign || 'PATROL-ALPINE-01');

    const flags = Array.isArray(op.riskFlags) ? op.riskFlags : [];
    const flagsHtml = flags
      .map((f) => `<span class="badge badge-aw-flag">${escapeHtml(f)}</span>`)
      .join('');

    const burialBannerHtml =
      op.asphyxiaAirPocketAlert || op.severeFrostbiteAfterdropAlert
        ? `
          <div class="aw-burial-alert-banner">
            ⚠️ SNOWPACK ASPHYXIA &amp; HYPOTHERMIC AFTERDROP ALERT: Burial ${burialDepth}cm (${minutesBuried} min) · Core ${coreTemp}°C · Wind-Chill ${windChill}°C — Excavate downhill V-conveyor airway trench &amp; lift horizontally into active thoracic heat-pack bivy.
          </div>
        `
        : '';

    const clearanceBannerHtml = op.clearanceCertificateHash
      ? `
          <div class="aw-clearance-banner">
            ✅ WINTER NORMOTHERMIC EXTRACTION CLEARANCE VERIFIED: ${escapeHtml(op.clearanceCertificateHash)} (${callsign})
          </div>
        `
      : '';

    const isSafe = op.status === 'EXTRACTED_NORMOTHERMIC_SAFE';
    const steps = Array.isArray(op.protocolSteps) ? op.protocolSteps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const category = escapeHtml(step.category || '');
        const label = escapeHtml(step.label || '');
        const badgeClass = done ? 'badge-aw-safe' : 'badge-aw-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="aw-step-item">
            <input
              type="checkbox"
              class="aw-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="aw-step-category">[${category}]</strong>
            <span class="aw-step-label">${label}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary aw-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Winter Normothermic Extraction Clearance Certificate
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card aw-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg · ${sectorGrid})</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-aw-terrain">${terrainType}</span>
            <span class="badge badge-aw-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row aw-metrics-row">
          <span class="badge badge-aw-terrain">Hazard Score: ${hazardScore}/100 • Rewarming Readiness: ${readinessScore}% • Burial: ${burialDepth}cm (${minutesBuried}m) • Core: ${coreTemp}°C • Wind-Chill: ${windChill}°C • Avy Scale: D${avyScale}</span>
        </div>

        <div class="aw-flags-row">
          ${flagsHtml}
        </div>

        ${burialBannerHtml}
        ${clearanceBannerHtml}

        <div class="aw-gear-recommendations">
          <div><strong>Snowpack Locator &amp; Probe Gear:</strong> ${locatorGear}</div>
          <div><strong>Hypothermia Rewarming Bivy:</strong> ${rewarmingBivy}</div>
        </div>

        <div class="aw-steps-list" aria-label="Avalanche Beacon Pinpoint, V-Trench Excavation &amp; Active Core Rewarming Protocol">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops) {
    currentOps = ops;
    updateSummaryStats(ops);

    const countBadge = document.getElementById('aw-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('aw-ops-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="aw-ops-empty-msg">No winter rescue operations recorded yet.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const res = await fetch('/api/v1/winter-rescue/operations', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const ops = Array.isArray(data.operations)
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
    const res = await fetch('/api/v1/winter-rescue/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchOperations();
      showToast(
        `Evaluated winter rescue operation ${created.operationId} for ${created.petName} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate winter rescue operation', 'error');
    return null;
  }

  function populateFormWithPreset(preset) {
    const setVal = (id, val) => {
      const el = document.getElementById(id);
      if (el) el.value = String(val);
    };
    setVal('aw-pet-id', preset.petId);
    setVal('aw-pet-name', preset.petName);
    setVal('aw-species', preset.species);
    setVal('aw-weight-kg', preset.weightKg);
    setVal('aw-terrain-type', preset.terrainType);
    setVal('aw-burial-depth', preset.burialDepthCm);
    setVal('aw-wind-chill', preset.windChillCelsius);
    setVal('aw-core-temp', preset.coreTempCelsius);
    setVal('aw-minutes-buried', preset.minutesBuried);
    setVal('aw-danger-scale', preset.avalancheDangerScale);
    setVal('aw-sector-grid', preset.snowpackSectorGrid);
    setVal('aw-patrol-callsign', preset.leadSkiPatrolCallsign);
    setVal('aw-notes', preset.notes);
  }

  function setupFormAndScenarioButtons() {
    const form = document.getElementById('form-evaluate-aw-op');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const payload = {
          petId: (document.getElementById('aw-pet-id')?.value || 'PET-AW-204').trim(),
          petName: (document.getElementById('aw-pet-name')?.value || 'Kodiak').trim(),
          species: (document.getElementById('aw-species')?.value || 'Canine').trim(),
          weightKg: parseFloat(document.getElementById('aw-weight-kg')?.value || '29.0') || 29.0,
          terrainType: (document.getElementById('aw-terrain-type')?.value || 'AVALANCHE_DEBRIS_CHUTE').trim(),
          burialDepthCm: parseFloat(document.getElementById('aw-burial-depth')?.value || '90') || 0,
          windChillCelsius: parseFloat(document.getElementById('aw-wind-chill')?.value || '-24.0') || -24.0,
          coreTempCelsius: parseFloat(document.getElementById('aw-core-temp')?.value || '33.4') || 33.4,
          minutesBuried: parseInt(document.getElementById('aw-minutes-buried')?.value || '26', 10) || 0,
          avalancheDangerScale: parseInt(document.getElementById('aw-danger-scale')?.value || '4', 10) || 3,
          snowpackSectorGrid: (document.getElementById('aw-sector-grid')?.value || 'CHUTE-NORTH-04A').trim(),
          leadSkiPatrolCallsign: (document.getElementById('aw-patrol-callsign')?.value || 'PATROL-ALPINE-01').trim(),
          notes: (document.getElementById('aw-notes')?.value || '').trim(),
        };

        const btn = document.getElementById('btn-evaluate-aw-op');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.aw-scenario-btn');
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

  function setupCardInteractions() {
    const listEl = document.getElementById('aw-ops-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.aw-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/winter-rescue/operations/${encodeURIComponent(opId)}/step`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ stepId, completed }),
          }
        );
        if (res.ok) {
          const updated = await res.json();
          await fetchOperations();
          showToast(
            `Updated step ${stepId} for ${opId} → Readiness ${updated.rewarmingReadinessScore}% (${updated.status})`,
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
      const clearanceBtn = e.target.closest('.aw-issue-clearance-btn');
      if (!clearanceBtn) return;

      const opId = clearanceBtn.getAttribute('data-op-id');
      if (!opId) return;

      clearanceBtn.disabled = true;
      try {
        const op = currentOps.find((item) => item.operationId === opId);
        if (op && Array.isArray(op.protocolSteps)) {
          for (const step of op.protocolSteps) {
            if (step.required && !step.completed) {
              await fetch(
                `/api/v1/winter-rescue/operations/${encodeURIComponent(opId)}/step`,
                {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({ stepId: step.id, completed: true }),
                }
              );
            }
          }
        }

        const res = await fetch(
          `/api/v1/winter-rescue/operations/${encodeURIComponent(opId)}/clearance`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              leadSkiPatrolCallsign: 'PATROL-ALPINE-01 (Sorensen)',
              notes: 'Snowpack extraction completed and normothermic core vitals verified.',
            }),
          }
        );
        if (res.ok) {
          const updated = await res.json();
          await fetchOperations();
          showToast(
            `Issued winter clearance ${updated.clearanceCertificateHash} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue winter clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing winter clearance', 'error');
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
    setupCardInteractions();
    fetchOperations();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
