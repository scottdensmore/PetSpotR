/**
 * PetSpotR Seismic Rubble, Urban Search & Rescue (USAR) Structural Collapse
 * & Delsar Acoustic Void-Space Extraction Cockpit Controller (Milestone 16.5)
 */

(function () {
  'use strict';

  let currentOps = [];

  const SCENARIO_PRESETS = {
    'pancake-slab-aftershock': {
      petId: 'PET-USAR-204',
      petName: 'Brutus',
      species: 'Canine',
      weightKg: 32.5,
      voidType: 'PANCAKE_FLOOR_SLAB_VOID',
      rubbleOverburdenMeters: 2.9,
      aftershockPGA: 0.36,
      silicaDustMgM3: 8.6,
      hoursEntrapped: 19.5,
      structuralStabilityIdx: 20,
      collapseStructureGrid: 'SECTOR-BRAVO-04 (Collapsed Parking Deck B2)',
      leadUsarSpecCallsign: 'USAR-TF1-SPEC-01 (Castellano)',
      notes: 'Simulated pancake concrete floor-slab void (2.9m overburden) with 0.36g aftershock PGA and crush-syndrome rhabdomyolysis risk.',
    },
    'leanto-truss-void': {
      petId: 'PET-USAR-205',
      petName: 'Jasper',
      species: 'Canine',
      weightKg: 21.0,
      voidType: 'LEAN_TO_TIMBER_TRUSS_CAVITY',
      rubbleOverburdenMeters: 1.4,
      aftershockPGA: 0.14,
      silicaDustMgM3: 3.2,
      hoursEntrapped: 7.5,
      structuralStabilityIdx: 68,
      collapseStructureGrid: 'SECTOR-ALPHA-09 (Lean-To Timber Truss Residential Void)',
      leadUsarSpecCallsign: 'USAR-TF1-SPEC-03 (Nakamura)',
      notes: 'Simulated canine inside lean-to timber roof truss void; SearchCam snake locked and 4x4 T-spot shore erected.',
    },
    'vshape-masonry-pocket': {
      petId: 'PET-USAR-206',
      petName: 'Cleo',
      species: 'Feline',
      weightKg: 4.3,
      voidType: 'V_SHAPE_MASONRY_RUBBLE',
      rubbleOverburdenMeters: 0.8,
      aftershockPGA: 0.08,
      silicaDustMgM3: 1.6,
      hoursEntrapped: 4.0,
      structuralStabilityIdx: 80,
      collapseStructureGrid: 'SECTOR-CHARLIE-02 (Unreinforced Masonry V-Shape Pocket)',
      leadUsarSpecCallsign: 'USAR-TF1-SPEC-02 (Kowalski)',
      notes: 'Simulated feline sheltered inside center-collapsed V-shape masonry pocket; positive-pressure HEPA air line and Paratech shore staged.',
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
    const liveStatus = document.getElementById('usar-collapse-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function getStatusBadgeClass(status) {
    switch (status) {
      case 'SECONDARY_COLLAPSE_CRUSH_CRITICAL':
        return 'badge-usar-critical';
      case 'SHORING_CRIBBING_DELSAR_STAGED':
        return 'badge-usar-staged';
      case 'EXTRACTED_USAR_DECON_SAFE':
        return 'badge-usar-safe';
      default:
        return 'badge-usar-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('stat-total-usar-ops');
    const collapseEl = document.getElementById('stat-critical-collapse-alerts');
    const stagedEl = document.getElementById('stat-staged-shoring-rigs');
    const safeEl = document.getElementById('stat-extracted-usar-count');

    const total = ops.length;
    const criticalCollapse = ops.filter(
      (op) =>
        Boolean(op.secondaryCollapseAlert) ||
        Boolean(op.crushRhabdoAlert) ||
        op.status === 'SECONDARY_COLLAPSE_CRUSH_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'SHORING_CRIBBING_DELSAR_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'EXTRACTED_USAR_DECON_SAFE'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (collapseEl) collapseEl.textContent = String(criticalCollapse);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (safeEl) safeEl.textContent = String(safe);
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.operationId || '');
    const petName = escapeHtml(op.petName || '');
    const species = escapeHtml(op.species || 'Canine');
    const weightKg = Number(op.weightKg ?? 25.0).toFixed(1);
    const structureGrid = escapeHtml(op.collapseStructureGrid || 'SECTOR-BRAVO-04');
    const voidType = escapeHtml(op.voidType || 'PANCAKE_FLOOR_SLAB_VOID');
    const status = escapeHtml(op.status || 'SECONDARY_COLLAPSE_CRUSH_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const hazardScore = Number(op.hazardScore ?? 50);
    const readinessScore = Number(op.shoringReadinessScore ?? 0);
    const overburden = Number(op.rubbleOverburdenMeters ?? 2.0).toFixed(1);
    const hoursEntrapped = Number(op.hoursEntrapped ?? 12.0).toFixed(1);
    const aftershockPGA = Number(op.aftershockPGA ?? 0.25).toFixed(2);
    const silicaDust = Number(op.silicaDustMgM3 ?? 5.0).toFixed(1);
    const stabilityIdx = Number(op.structuralStabilityIdx ?? 35);
    const acousticArray = escapeHtml(op.recommendedAcousticArray || '');
    const shoringRig = escapeHtml(op.recommendedShoringRig || '');
    const callsign = escapeHtml(op.leadUsarSpecCallsign || 'USAR-TF1-SPEC-01');

    const flags = Array.isArray(op.riskFlags) ? op.riskFlags : [];
    const flagsHtml = flags
      .map((f) => `<span class="badge badge-usar-flag">${escapeHtml(f)}</span>`)
      .join('');

    const collapseBannerHtml =
      op.secondaryCollapseAlert || op.crushRhabdoAlert
        ? `
          <div class="usar-collapse-alert-banner">
            ⚠️ SECONDARY COLLAPSE &amp; CRUSH-SYNDROME HYPERKALEMIA ALERT: Overburden ${overburden}m (${hoursEntrapped}h) · Aftershock PGA ${aftershockPGA}g · Stability ${stabilityIdx}/100 — Erect 6x6 timber box-cribbing &amp; pneumatic raker shore before slab breach.
          </div>
        `
        : '';

    const clearanceBannerHtml = op.clearanceCertificateHash
      ? `
          <div class="usar-clearance-banner">
            ✅ USAR VOID EXTRACTION &amp; SILICA DECON CLEARANCE VERIFIED: ${escapeHtml(op.clearanceCertificateHash)} (${callsign})
          </div>
        `
      : '';

    const isSafe = op.status === 'EXTRACTED_USAR_DECON_SAFE';
    const steps = Array.isArray(op.protocolSteps) ? op.protocolSteps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const category = escapeHtml(step.category || '');
        const label = escapeHtml(step.label || '');
        const badgeClass = done ? 'badge-usar-safe' : 'badge-usar-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="usar-step-item">
            <input
              type="checkbox"
              class="usar-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="usar-step-category">[${category}]</strong>
            <span class="usar-step-label">${label}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary usar-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue USAR Void Extraction &amp; Silica Decon Clearance Certificate
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card usar-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg · ${structureGrid})</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-usar-void">${voidType}</span>
            <span class="badge badge-usar-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row usar-metrics-row">
          <span class="badge badge-usar-void">Hazard Score: ${hazardScore}/100 • Shoring Readiness: ${readinessScore}% • Overburden: ${overburden}m (${hoursEntrapped}h) • PGA: ${aftershockPGA}g • Silica: ${silicaDust} mg/m³ • Stability: ${stabilityIdx}/100</span>
        </div>

        <div class="usar-flags-row">
          ${flagsHtml}
        </div>

        ${collapseBannerHtml}
        ${clearanceBannerHtml}

        <div class="usar-gear-recommendations">
          <div><strong>Delsar Seismic / Borehole Array:</strong> ${acousticArray}</div>
          <div><strong>Timber Box-Cribbing &amp; Shoring Rig:</strong> ${shoringRig}</div>
        </div>

        <div class="usar-steps-list" aria-label="Delsar Seismic Triangulation, Box-Cribbing Shoring &amp; Crush-Syndrome Extraction Protocol">
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

    const countBadge = document.getElementById('usar-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('usar-ops-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="usar-ops-empty-msg">No USAR structural collapse operations recorded yet.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const res = await fetch('/api/v1/usar-collapse/operations', {
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
    const res = await fetch('/api/v1/usar-collapse/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchOperations();
      showToast(
        `Evaluated USAR collapse operation ${created.operationId} for ${created.petName} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate USAR collapse operation', 'error');
    return null;
  }

  function populateFormWithPreset(preset) {
    const setVal = (id, val) => {
      const el = document.getElementById(id);
      if (el) el.value = String(val);
    };
    setVal('usar-pet-id', preset.petId);
    setVal('usar-pet-name', preset.petName);
    setVal('usar-species', preset.species);
    setVal('usar-weight-kg', preset.weightKg);
    setVal('usar-void-type', preset.voidType);
    setVal('usar-rubble-overburden', preset.rubbleOverburdenMeters);
    setVal('usar-aftershock-pga', preset.aftershockPGA);
    setVal('usar-silica-dust', preset.silicaDustMgM3);
    setVal('usar-hours-entrapped', preset.hoursEntrapped);
    setVal('usar-stability-idx', preset.structuralStabilityIdx);
    setVal('usar-structure-grid', preset.collapseStructureGrid);
    setVal('usar-spec-callsign', preset.leadUsarSpecCallsign);
    setVal('usar-notes', preset.notes);
  }

  function setupFormAndScenarioButtons() {
    const form = document.getElementById('form-evaluate-usar-op');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const payload = {
          petId: (document.getElementById('usar-pet-id')?.value || 'PET-USAR-204').trim(),
          petName: (document.getElementById('usar-pet-name')?.value || 'Brutus').trim(),
          species: (document.getElementById('usar-species')?.value || 'Canine').trim(),
          weightKg: parseFloat(document.getElementById('usar-weight-kg')?.value || '32.5') || 32.5,
          voidType: (document.getElementById('usar-void-type')?.value || 'PANCAKE_FLOOR_SLAB_VOID').trim(),
          rubbleOverburdenMeters: parseFloat(document.getElementById('usar-rubble-overburden')?.value || '2.9') || 0,
          aftershockPGA: parseFloat(document.getElementById('usar-aftershock-pga')?.value || '0.36') || 0,
          silicaDustMgM3: parseFloat(document.getElementById('usar-silica-dust')?.value || '8.6') || 0,
          hoursEntrapped: parseFloat(document.getElementById('usar-hours-entrapped')?.value || '19.5') || 0,
          structuralStabilityIdx: parseInt(document.getElementById('usar-stability-idx')?.value || '20', 10) || 20,
          collapseStructureGrid: (document.getElementById('usar-structure-grid')?.value || 'SECTOR-BRAVO-04').trim(),
          leadUsarSpecCallsign: (document.getElementById('usar-spec-callsign')?.value || 'USAR-TF1-SPEC-01').trim(),
          notes: (document.getElementById('usar-notes')?.value || '').trim(),
        };

        const btn = document.getElementById('btn-evaluate-usar-op');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.usar-scenario-btn');
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
    const listEl = document.getElementById('usar-ops-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.usar-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/usar-collapse/operations/${encodeURIComponent(opId)}/step`,
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
            `Updated step ${stepId} for ${opId} → Shoring Readiness ${updated.shoringReadinessScore}% (${updated.status})`,
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
      const clearanceBtn = e.target.closest('.usar-issue-clearance-btn');
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
                `/api/v1/usar-collapse/operations/${encodeURIComponent(opId)}/step`,
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
          `/api/v1/usar-collapse/operations/${encodeURIComponent(opId)}/clearance`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              leadUsarSpecCallsign: 'USAR-TF1-SPEC-01 (Castellano)',
              notes: 'Void extraction completed, crush-syndrome fluid bolus verified, and silica dust decon cleared.',
            }),
          }
        );
        if (res.ok) {
          const updated = await res.json();
          await fetchOperations();
          showToast(
            `Issued USAR clearance ${updated.clearanceCertificateHash} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue USAR clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing USAR clearance', 'error');
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
