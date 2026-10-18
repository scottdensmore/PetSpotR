/**
 * PetSpotR Wildfire Smoke, Ashfall, Hazmat Plume & Respiratory
 * Decontamination Triage Cockpit Controller (Milestone 16.1)
 */

(function () {
  'use strict';

  let currentOps = [];

  const SCENARIO_PRESETS = {
    'wildfire-ash-plume': {
      petId: 'PET-HZ-204',
      petName: 'Ash',
      species: 'Feline',
      weightKg: 4.1,
      exposureType: 'WILDFIRE_SMOKE_ASHFALL',
      aqiIndex: 450,
      pm25UgM3: 410,
      carboxyhemoglobinPct: 17.8,
      coatContaminationPct: 72,
      respiratoryRateBpm: 72,
      deconCorridorZone: 'DECON-CORRIDOR-HOT-01 (Cascade Wildfire Perimeter)',
      leadToxTechCallsign: 'TOX-VET-01 (Dr. Aris Thorne)',
      notes: 'Simulated severe wildfire smoke & alkaline ashfall plume (AQI 450, COHb 17.8%, Coat 72%).',
    },
    'petrochemical-coat-spill': {
      petId: 'PET-HZ-205',
      petName: 'Mariner',
      species: 'Canine',
      weightKg: 24.5,
      exposureType: 'PETROCHEMICAL_SPILL_COAT',
      aqiIndex: 115,
      pm25UgM3: 44,
      carboxyhemoglobinPct: 2.8,
      coatContaminationPct: 58,
      respiratoryRateBpm: 38,
      deconCorridorZone: 'DECON-CORRIDOR-WARM-02 (Duwamish Waterway Spill Station)',
      leadToxTechCallsign: 'TOX-VET-04 (Chen)',
      notes: 'Simulated diesel bilge hydrocarbon coat contamination; warm lipophilic dish-surfactant wash staged.',
    },
    'cyanotoxin-pond-bloom': {
      petId: 'PET-HZ-206',
      petName: 'Reed',
      species: 'Canine',
      weightKg: 19.2,
      exposureType: 'TOXIC_ALGAE_CYANOTOXIN',
      aqiIndex: 55,
      pm25UgM3: 14,
      carboxyhemoglobinPct: 1.2,
      coatContaminationPct: 45,
      respiratoryRateBpm: 32,
      deconCorridorZone: 'DECON-CORRIDOR-COLD-03 (Green Lake Cyanotoxin Triage)',
      leadToxTechCallsign: 'TOX-VET-02 (Patel)',
      notes: 'Simulated blue-green algae microcystin pond bloom exposure; dermal rinse and cholestyramine binder staged.',
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
    const liveStatus = document.getElementById('hazmat-decon-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function getStatusBadgeClass(status) {
    switch (status) {
      case 'RESPIRATORY_DERMAL_CRITICAL':
        return 'badge-hz-critical';
      case 'DECON_WASH_NEBULIZER_STAGED':
        return 'badge-hz-staged';
      case 'DECONTAMINATED_CLEARED_SAFE':
        return 'badge-hz-cleared';
      default:
        return 'badge-hz-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('stat-total-hazmat-ops');
    const smokeEl = document.getElementById('stat-critical-smoke-alerts');
    const stagedEl = document.getElementById('stat-staged-decon-washes');
    const clearedEl = document.getElementById('stat-cleared-safe-count');

    const total = ops.length;
    const criticalSmoke = ops.filter(
      (op) => Boolean(op.smokeInhalationAlert) || op.status === 'RESPIRATORY_DERMAL_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'DECON_WASH_NEBULIZER_STAGED'
    ).length;
    const cleared = ops.filter(
      (op) => op.status === 'DECONTAMINATED_CLEARED_SAFE'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (smokeEl) smokeEl.textContent = String(criticalSmoke);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.operationId || '');
    const petName = escapeHtml(op.petName || '');
    const species = escapeHtml(op.species || 'Canine');
    const weightKg = Number(op.weightKg ?? 4.0).toFixed(1);
    const corridorZone = escapeHtml(op.deconCorridorZone || 'DECON-CORRIDOR-WARM-01');
    const exposureType = escapeHtml(op.exposureType || 'WILDFIRE_SMOKE_ASHFALL');
    const status = escapeHtml(op.status || 'RESPIRATORY_DERMAL_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const hazardScore = Number(op.hazardScore ?? 50);
    const readinessScore = Number(op.deconReadinessScore ?? 0);
    const aqi = Number(op.aqiIndex ?? 100);
    const pm25 = Number(op.pm25UgM3 ?? 35).toFixed(0);
    const cohb = Number(op.carboxyhemoglobinPct ?? 1.5).toFixed(1);
    const coat = Number(op.coatContaminationPct ?? 0).toFixed(0);
    const resp = Number(op.respiratoryRateBpm ?? 30);
    const washAgent = escapeHtml(op.recommendedWashAgent || '');
    const oxygenTherapy = escapeHtml(op.recommendedOxygenTherapy || '');
    const callsign = escapeHtml(op.leadToxTechCallsign || 'TOX-VET-01');

    const flags = Array.isArray(op.riskFlags) ? op.riskFlags : [];
    const flagsHtml = flags
      .map((f) => `<span class="badge badge-hz-flag">${escapeHtml(f)}</span>`)
      .join('');

    const smokeBannerHtml =
      op.smokeInhalationAlert || op.dermalToxinIngestionAlert
        ? `
          <div class="hz-smoke-alert-banner">
            ⚠️ RESPIRATORY &amp; DERMAL TOXIN HAZARD ALERT: AQI ${aqi} · PM2.5 ${pm25} µg/m³ · COHb ${cohb}% · Coat ${coat}% — Fit E-collar immediately, flush corneas with sterile saline &amp; stage 100% O2 nebulizer.
          </div>
        `
        : '';

    const clearanceBannerHtml = op.clearanceCertificateHash
      ? `
          <div class="hz-clearance-banner">
            ✅ HAZMAT &amp; RESPIRATORY DECON CLEARANCE VERIFIED: ${escapeHtml(op.clearanceCertificateHash)} (${callsign})
          </div>
        `
      : '';

    const isCleared = op.status === 'DECONTAMINATED_CLEARED_SAFE';
    const steps = Array.isArray(op.protocolSteps) ? op.protocolSteps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const category = escapeHtml(step.category || '');
        const label = escapeHtml(step.label || '');
        const badgeClass = done ? 'badge-hz-cleared' : 'badge-hz-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="hz-step-item">
            <input
              type="checkbox"
              class="hz-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isCleared ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="hz-step-category">[${category}]</strong>
            <span class="hz-step-label">${label}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isCleared
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary hz-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Hazmat &amp; Respiratory Decon Clearance Certificate
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card hz-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg · ${corridorZone})</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-hz-exposure">${exposureType}</span>
            <span class="badge badge-hz-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row hz-metrics-row">
          <span class="badge badge-hz-exposure">Hazard Score: ${hazardScore}/100 • Decon Readiness: ${readinessScore}% • AQI: ${aqi} • PM2.5: ${pm25} µg/m³ • COHb: ${cohb}% • Coat: ${coat}% • Resp: ${resp} bpm</span>
        </div>

        <div class="hz-flags-row">
          ${flagsHtml}
        </div>

        ${smokeBannerHtml}
        ${clearanceBannerHtml}

        <div class="hz-decon-recommendations">
          <div><strong>Ocular &amp; Dermal Wash Agent:</strong> ${washAgent}</div>
          <div><strong>Oxygen &amp; Nebulization Therapy:</strong> ${oxygenTherapy}</div>
        </div>

        <div class="hz-steps-list" aria-label="Hazmat Ocular Flush, Surfactant Wash &amp; Oxygen Nebulization Protocol">
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

    const countBadge = document.getElementById('hz-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('hz-ops-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="hz-ops-empty-msg">No hazmat or smoke decontamination operations recorded yet.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const res = await fetch('/api/v1/hazmat-decon/operations', {
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
    const res = await fetch('/api/v1/hazmat-decon/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchOperations();
      showToast(
        `Evaluated hazmat decon operation ${created.operationId} for ${created.petName} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate hazmat decon operation', 'error');
    return null;
  }

  function populateFormWithPreset(preset) {
    const setVal = (id, val) => {
      const el = document.getElementById(id);
      if (el) el.value = String(val);
    };
    setVal('hz-pet-id', preset.petId);
    setVal('hz-pet-name', preset.petName);
    setVal('hz-species', preset.species);
    setVal('hz-weight-kg', preset.weightKg);
    setVal('hz-exposure-type', preset.exposureType);
    setVal('hz-aqi-index', preset.aqiIndex);
    setVal('hz-pm25', preset.pm25UgM3);
    setVal('hz-cohb-pct', preset.carboxyhemoglobinPct);
    setVal('hz-coat-pct', preset.coatContaminationPct);
    setVal('hz-resp-rate', preset.respiratoryRateBpm);
    setVal('hz-corridor-zone', preset.deconCorridorZone);
    setVal('hz-tox-callsign', preset.leadToxTechCallsign);
    setVal('hz-notes', preset.notes);
  }

  function setupFormAndScenarioButtons() {
    const form = document.getElementById('form-evaluate-hz-op');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const payload = {
          petId: (document.getElementById('hz-pet-id')?.value || 'PET-HZ-204').trim(),
          petName: (document.getElementById('hz-pet-name')?.value || 'Ash').trim(),
          species: (document.getElementById('hz-species')?.value || 'Feline').trim(),
          weightKg: parseFloat(document.getElementById('hz-weight-kg')?.value || '4.1') || 4.1,
          exposureType: (document.getElementById('hz-exposure-type')?.value || 'WILDFIRE_SMOKE_ASHFALL').trim(),
          aqiIndex: parseInt(document.getElementById('hz-aqi-index')?.value || '450', 10) || 0,
          pm25UgM3: parseFloat(document.getElementById('hz-pm25')?.value || '410') || 0,
          carboxyhemoglobinPct: parseFloat(document.getElementById('hz-cohb-pct')?.value || '17.8') || 0,
          coatContaminationPct: parseFloat(document.getElementById('hz-coat-pct')?.value || '72') || 0,
          respiratoryRateBpm: parseInt(document.getElementById('hz-resp-rate')?.value || '72', 10) || 30,
          deconCorridorZone: (document.getElementById('hz-corridor-zone')?.value || 'DECON-CORRIDOR-HOT-01').trim(),
          leadToxTechCallsign: (document.getElementById('hz-tox-callsign')?.value || 'TOX-VET-01').trim(),
          notes: (document.getElementById('hz-notes')?.value || '').trim(),
        };

        const btn = document.getElementById('btn-evaluate-hz-op');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.hz-scenario-btn');
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
    const listEl = document.getElementById('hz-ops-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.hz-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/hazmat-decon/operations/${encodeURIComponent(opId)}/step`,
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
            `Updated step ${stepId} for ${opId} → Readiness ${updated.deconReadinessScore}% (${updated.status})`,
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
      const clearanceBtn = e.target.closest('.hz-issue-clearance-btn');
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
                `/api/v1/hazmat-decon/operations/${encodeURIComponent(opId)}/step`,
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
          `/api/v1/hazmat-decon/operations/${encodeURIComponent(opId)}/clearance`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              leadToxTechCallsign: 'TOX-VET-01 (Dr. Aris Thorne)',
              notes: 'Warm-zone surfactant wash, 100% O2 nebulization, and COHb clearance verified.',
            }),
          }
        );
        if (res.ok) {
          const updated = await res.json();
          await fetchOperations();
          showToast(
            `Issued hazmat decon clearance ${updated.clearanceCertificateHash} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue hazmat decon clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing hazmat decon clearance', 'error');
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
