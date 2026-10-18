/**
 * PetSpotR Ectothermic Reptile, Chelonian & Amphibian Thermal-Dormancy
 * Brisk-Basking & Micro-Habitat Recovery Cockpit Controller (Milestone 16.4)
 */

(function () {
  'use strict';

  let currentOps = [];

  const SCENARIO_PRESETS = {
    'sulcata-cold-burrow': {
      petId: 'PET-RE-204',
      petName: 'Goliath',
      species: 'Reptile',
      weightKg: 22.0,
      taxonType: 'CHELONIAN_TORTOISE_BURROW',
      substrateTempCelsius: 10.8,
      preferredOptimumTempC: 31.5,
      relativeHumidityPct: 26.0,
      hoursDormant: 52.0,
      burrowDepthCm: 115.0,
      microHabitatGrid: 'BURROW-SOUTH-03A (Collapsed Foundation Crawlspace Embankment)',
      leadHerpTechCallsign: 'HERP-TECH-01 (Dr. Alvarez)',
      notes: 'Simulated Sulcata Tortoise cold-stunned at 10.8°C in 115cm collapsed foundation burrow with severe desiccation risk.',
    },
    'python-appliance-void': {
      petId: 'PET-RE-205',
      petName: 'Nagini',
      species: 'Reptile',
      weightKg: 2.8,
      taxonType: 'BOID_PYTHON_CONSTRICTOR',
      substrateTempCelsius: 23.0,
      preferredOptimumTempC: 31.0,
      relativeHumidityPct: 64.0,
      hoursDormant: 16.0,
      burrowDepthCm: 30.0,
      microHabitatGrid: 'VOID-KITCHEN-07B (Sub-Cabinet Hydronic Baseboard Conduit)',
      leadHerpTechCallsign: 'HERP-TECH-02 (Takahashi)',
      notes: 'Simulated Ball Python coiled inside warm appliance wall void; compressor power isolated and 31°C ceramic IR hide staged.',
    },
    'dragon-sunroom-crevice': {
      petId: 'PET-RE-206',
      petName: 'Solstice',
      species: 'Reptile',
      weightKg: 0.48,
      taxonType: 'AGAMID_IGUANID_ARBOREAL',
      substrateTempCelsius: 24.5,
      preferredOptimumTempC: 33.0,
      relativeHumidityPct: 46.0,
      hoursDormant: 9.0,
      burrowDepthCm: 10.0,
      microHabitatGrid: 'PERCH-SUNROOM-02C (South-Facing Conservatory Cedar Rafter)',
      leadHerpTechCallsign: 'HERP-TECH-03 (Kowalski)',
      notes: 'Simulated Bearded Dragon dormant in conservatory rafter crevice; UV-B/IR basking perch and acoustic feeder lure deployed.',
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
    const liveStatus = document.getElementById('reptile-ectotherm-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function getStatusBadgeClass(status) {
    switch (status) {
      case 'COLD_TORPOR_BRUMATION_CRITICAL':
        return 'badge-re-critical';
      case 'BASKING_LURE_CREVICE_STAGED':
        return 'badge-re-staged';
      case 'RECOVERED_THERMAL_GRADIENT_SAFE':
        return 'badge-re-safe';
      default:
        return 'badge-re-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('stat-total-ectotherm-ops');
    const torporEl = document.getElementById('stat-critical-torpor-alerts');
    const stagedEl = document.getElementById('stat-staged-basking-lures');
    const safeEl = document.getElementById('stat-thermal-safe-count');

    const total = ops.length;
    const criticalTorpor = ops.filter(
      (op) =>
        Boolean(op.criticalTorporAlert) ||
        Boolean(op.desiccationDysecdysisAlert) ||
        op.status === 'COLD_TORPOR_BRUMATION_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'BASKING_LURE_CREVICE_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'RECOVERED_THERMAL_GRADIENT_SAFE'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (torporEl) torporEl.textContent = String(criticalTorpor);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (safeEl) safeEl.textContent = String(safe);
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.operationId || '');
    const petName = escapeHtml(op.petName || '');
    const species = escapeHtml(op.species || 'Reptile');
    const weightKg = Number(op.weightKg ?? 5.0).toFixed(2);
    const habitatGrid = escapeHtml(op.microHabitatGrid || 'SUBSTRATE-SECTOR-01A');
    const taxonType = escapeHtml(op.taxonType || 'CHELONIAN_TORTOISE_BURROW');
    const status = escapeHtml(op.status || 'COLD_TORPOR_BRUMATION_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const hazardScore = Number(op.hazardScore ?? 50);
    const readinessScore = Number(op.thermalReadinessScore ?? 0);
    const substrateTemp = Number(op.substrateTempCelsius ?? 15.0).toFixed(1);
    const optimumTemp = Number(op.preferredOptimumTempC ?? 31.0).toFixed(1);
    const humidityPct = Number(op.relativeHumidityPct ?? 50).toFixed(0);
    const hoursDormant = Number(op.hoursDormant ?? 12).toFixed(0);
    const burrowDepth = Number(op.burrowDepthCm ?? 40).toFixed(0);
    const thermalLure = escapeHtml(op.recommendedThermalLure || '');
    const containment = escapeHtml(op.recommendedContainment || '');
    const callsign = escapeHtml(op.leadHerpTechCallsign || 'HERP-TECH-01');

    const flags = Array.isArray(op.riskFlags) ? op.riskFlags : [];
    const flagsHtml = flags
      .map((f) => `<span class="badge badge-re-flag">${escapeHtml(f)}</span>`)
      .join('');

    const torporBannerHtml =
      op.criticalTorporAlert || op.desiccationDysecdysisAlert
        ? `
          <div class="re-torpor-alert-banner">
            ⚠️ ECTOTHERMIC COLD-STUN TORPOR &amp; DESICCATION ALERT: Substrate ${substrateTemp}°C vs POTZ ${optimumTemp}°C · Humidity ${humidityPct}% RH (${hoursDormant}h dormant) — Stage 100W ceramic IR basking lure, sphagnum humidity hide &amp; gradual coelomic rewarming (≤2°C/hr).
          </div>
        `
        : '';

    const clearanceBannerHtml = op.clearanceCertificateHash
      ? `
          <div class="re-clearance-banner">
            ✅ ECTOTHERM THERMAL-GRADIENT RECOVERY CLEARANCE VERIFIED: ${escapeHtml(op.clearanceCertificateHash)} (${callsign})
          </div>
        `
      : '';

    const isSafe = op.status === 'RECOVERED_THERMAL_GRADIENT_SAFE';
    const steps = Array.isArray(op.protocolSteps) ? op.protocolSteps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const category = escapeHtml(step.category || '');
        const label = escapeHtml(step.label || '');
        const badgeClass = done ? 'badge-re-safe' : 'badge-re-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="re-step-item">
            <input
              type="checkbox"
              class="re-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="re-step-category">[${category}]</strong>
            <span class="re-step-label">${label}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary re-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Ectotherm Thermal-Gradient Recovery Clearance Certificate
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card re-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg · ${habitatGrid})</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-re-taxon">${taxonType}</span>
            <span class="badge badge-re-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row re-metrics-row">
          <span class="badge badge-re-taxon">Hazard Score: ${hazardScore}/100 • Thermal Readiness: ${readinessScore}% • Substrate: ${substrateTemp}°C (POTZ ${optimumTemp}°C) • Humidity: ${humidityPct}% RH • Dormant: ${hoursDormant}h • Depth: ${burrowDepth}cm</span>
        </div>

        <div class="re-flags-row">
          ${flagsHtml}
        </div>

        ${torporBannerHtml}
        ${clearanceBannerHtml}

        <div class="re-gear-recommendations">
          <div><strong>Recommended Thermal Basking Lure:</strong> ${thermalLure}</div>
          <div><strong>Coelomic Containment &amp; Hydration:</strong> ${containment}</div>
        </div>

        <div class="re-steps-list" aria-label="Borescope Crevice Sweep, Ceramic IR Basking Lure &amp; Gradual Coelomic Rewarming Protocol">
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

    const countBadge = document.getElementById('re-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('re-ops-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="re-ops-empty-msg">No ectotherm rescue operations recorded yet.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const res = await fetch('/api/v1/ectotherm-rescue/operations', {
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
    const res = await fetch('/api/v1/ectotherm-rescue/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchOperations();
      showToast(
        `Evaluated ectotherm rescue operation ${created.operationId} for ${created.petName} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate ectotherm rescue operation', 'error');
    return null;
  }

  function populateFormWithPreset(preset) {
    const setVal = (id, val) => {
      const el = document.getElementById(id);
      if (el) el.value = String(val);
    };
    setVal('re-pet-id', preset.petId);
    setVal('re-pet-name', preset.petName);
    setVal('re-species', preset.species);
    setVal('re-weight-kg', preset.weightKg);
    setVal('re-taxon-type', preset.taxonType);
    setVal('re-substrate-temp', preset.substrateTempCelsius);
    setVal('re-optimum-temp', preset.preferredOptimumTempC);
    setVal('re-humidity-pct', preset.relativeHumidityPct);
    setVal('re-hours-dormant', preset.hoursDormant);
    setVal('re-burrow-depth', preset.burrowDepthCm);
    setVal('re-habitat-grid', preset.microHabitatGrid);
    setVal('re-herp-callsign', preset.leadHerpTechCallsign);
    setVal('re-notes', preset.notes);
  }

  function setupFormAndScenarioButtons() {
    const form = document.getElementById('form-evaluate-re-op');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const payload = {
          petId: (document.getElementById('re-pet-id')?.value || 'PET-RE-204').trim(),
          petName: (document.getElementById('re-pet-name')?.value || 'Goliath').trim(),
          species: (document.getElementById('re-species')?.value || 'Reptile').trim(),
          weightKg: parseFloat(document.getElementById('re-weight-kg')?.value || '22.0') || 22.0,
          taxonType: (document.getElementById('re-taxon-type')?.value || 'CHELONIAN_TORTOISE_BURROW').trim(),
          substrateTempCelsius: parseFloat(document.getElementById('re-substrate-temp')?.value || '10.8') || 10.8,
          preferredOptimumTempC: parseFloat(document.getElementById('re-optimum-temp')?.value || '31.5') || 31.5,
          relativeHumidityPct: parseFloat(document.getElementById('re-humidity-pct')?.value || '26.0') || 26.0,
          hoursDormant: parseFloat(document.getElementById('re-hours-dormant')?.value || '52.0') || 0,
          burrowDepthCm: parseFloat(document.getElementById('re-burrow-depth')?.value || '115.0') || 0,
          microHabitatGrid: (document.getElementById('re-habitat-grid')?.value || 'BURROW-SOUTH-03A').trim(),
          leadHerpTechCallsign: (document.getElementById('re-herp-callsign')?.value || 'HERP-TECH-01').trim(),
          notes: (document.getElementById('re-notes')?.value || '').trim(),
        };

        const btn = document.getElementById('btn-evaluate-re-op');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.re-scenario-btn');
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
    const listEl = document.getElementById('re-ops-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.re-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/ectotherm-rescue/operations/${encodeURIComponent(opId)}/step`,
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
            `Updated step ${stepId} for ${opId} → Thermal Readiness ${updated.thermalReadinessScore}% (${updated.status})`,
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
      const clearanceBtn = e.target.closest('.re-issue-clearance-btn');
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
                `/api/v1/ectotherm-rescue/operations/${encodeURIComponent(opId)}/step`,
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
          `/api/v1/ectotherm-rescue/operations/${encodeURIComponent(opId)}/clearance`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              leadHerpTechCallsign: 'HERP-TECH-01 (Dr. Alvarez)',
              notes: 'Micro-habitat extraction completed and gradual coelomic POTZ rewarming verified.',
            }),
          }
        );
        if (res.ok) {
          const updated = await res.json();
          await fetchOperations();
          showToast(
            `Issued ectotherm clearance ${updated.clearanceCertificateHash} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue ectotherm clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing ectotherm clearance', 'error');
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
