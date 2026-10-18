/**
 * PetSpotR Subterranean, Storm-Drain, Culvert & Confined-Space
 * Acoustic/Fiber-Optic Extraction Cockpit Controller (Milestone 15.4)
 */

(function () {
  'use strict';

  let sseSource = null;
  let currentOps = [];

  const SCENARIO_PRESETS = {
    'culvert-gas-surge': {
      petId: 'PET-CS-204',
      petName: 'Shadow',
      species: 'Feline',
      weightKg: 4.0,
      spaceType: 'STORM_DRAIN_CULVERT',
      pipeDiameterCm: 34,
      depthMeters: 3.6,
      oxygenPercent: 18.1,
      h2sPpm: 15.2,
      inflowRiskIndex: 85,
      manholeSectorGrid: 'MH-NORTH-42B (Ravenna Creek Storm Culvert)',
      leadTechCallsign: 'TECH-SUB-01 (Vance)',
      notes: 'Simulated H2S gas pocket (15.2 ppm) and flash storm-culvert inflow surge (85/100).',
    },
    'crawlspace-wedged': {
      petId: 'PET-CS-205',
      petName: 'Barnaby',
      species: 'Canine',
      weightKg: 11.5,
      spaceType: 'FOUNDATION_CRAWLSPACE',
      pipeDiameterCm: 55,
      depthMeters: 1.6,
      oxygenPercent: 20.6,
      h2sPpm: 1.0,
      inflowRiskIndex: 20,
      manholeSectorGrid: 'SECTOR-CRAWL-19A (Ballard Pier Sub-Foundation)',
      leadTechCallsign: 'TECH-SUB-04 (Moreno)',
      notes: 'Simulated canine wedged behind sub-foundation pier footing; tracked crawler ROV deployed.',
    },
    'chimney-flue-drop': {
      petId: 'PET-CS-206',
      petName: 'Sooty',
      species: 'Feline',
      weightKg: 3.8,
      spaceType: 'CHIMNEY_FLUE_SHAFT',
      pipeDiameterCm: 28,
      depthMeters: 4.8,
      oxygenPercent: 20.8,
      h2sPpm: 0.5,
      inflowRiskIndex: 15,
      manholeSectorGrid: 'FLUE-SHAFT-07C (Queen Anne Masonry Chimney)',
      leadTechCallsign: 'TECH-SUB-02 (Kowalski)',
      notes: 'Simulated feline vertical drop inside 28cm clay-lined chimney flue shaft; 360° IR camera lowered.',
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
    const liveStatus = document.getElementById('confined-space-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function getStatusBadgeClass(status) {
    switch (status) {
      case 'ATMOSPHERIC_HAZARD_CRITICAL':
        return 'badge-cs-critical';
      case 'BORESCOPE_PIPE_GATE_RIGGED':
        return 'badge-cs-rigged';
      case 'EXTRACTED_SUBTERRANEAN_SAFE':
        return 'badge-cs-extracted';
      default:
        return 'badge-cs-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('stat-total-confined-ops');
    const gasEl = document.getElementById('stat-critical-gas-alerts');
    const riggedEl = document.getElementById('stat-rigged-borescopes');
    const extractedEl = document.getElementById('stat-extracted-safe-count');

    const total = ops.length;
    const criticalGas = ops.filter(
      (op) => Boolean(op.toxicGasAlert) || op.status === 'ATMOSPHERIC_HAZARD_CRITICAL'
    ).length;
    const rigged = ops.filter(
      (op) => op.status === 'BORESCOPE_PIPE_GATE_RIGGED'
    ).length;
    const extracted = ops.filter(
      (op) => op.status === 'EXTRACTED_SUBTERRANEAN_SAFE'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (gasEl) gasEl.textContent = String(criticalGas);
    if (riggedEl) riggedEl.textContent = String(rigged);
    if (extractedEl) extractedEl.textContent = String(extracted);
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.operationId || '');
    const petName = escapeHtml(op.petName || '');
    const species = escapeHtml(op.species || 'Canine');
    const weightKg = Number(op.weightKg ?? 4.0).toFixed(1);
    const sectorGrid = escapeHtml(op.manholeSectorGrid || 'MH-SECTOR-01A');
    const spaceType = escapeHtml(op.spaceType || 'STORM_DRAIN_CULVERT');
    const status = escapeHtml(op.status || 'ATMOSPHERIC_HAZARD_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const hazardScore = Number(op.hazardScore ?? 50);
    const readinessScore = Number(op.riggingReadinessScore ?? 0);
    const diameter = Number(op.pipeDiameterCm ?? 35).toFixed(0);
    const depth = Number(op.depthMeters ?? 3.0).toFixed(1);
    const o2 = Number(op.oxygenPercent ?? 20.9).toFixed(1);
    const h2s = Number(op.h2sPpm ?? 0.0).toFixed(1);
    const inflow = Number(op.inflowRiskIndex ?? 0);
    const opticsRig = escapeHtml(op.recommendedOpticsRig || '');
    const ventilation = escapeHtml(op.recommendedVentilation || '');
    const callsign = escapeHtml(op.leadTechCallsign || 'TECH-SUB-01');

    const flags = Array.isArray(op.riskFlags) ? op.riskFlags : [];
    const flagsHtml = flags
      .map((f) => `<span class="badge badge-cs-flag">${escapeHtml(f)}</span>`)
      .join('');

    const gasBannerHtml =
      op.toxicGasAlert || op.flashInflowAlert
        ? `
          <div class="cs-gas-alert-banner">
            ⚠️ ATMOSPHERIC &amp; CULVERT SURGE HAZARD ALERT: O2 ${o2}% · H2S ${h2s} ppm · Inflow Risk ${inflow}/100 — Maintain positive-pressure axial ventilation &amp; upstream pneumatic pipe-plug dam.
          </div>
        `
        : '';

    const clearanceBannerHtml = op.clearanceCertificateHash
      ? `
          <div class="cs-clearance-banner">
            ✅ SUBTERRANEAN EXTRACTION CLEARANCE VERIFIED: ${escapeHtml(op.clearanceCertificateHash)} (${callsign})
          </div>
        `
      : '';

    const isExtracted = op.status === 'EXTRACTED_SUBTERRANEAN_SAFE';
    const steps = Array.isArray(op.protocolSteps) ? op.protocolSteps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const category = escapeHtml(step.category || '');
        const label = escapeHtml(step.label || '');
        const badgeClass = done ? 'badge-cs-extracted' : 'badge-cs-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="cs-step-item">
            <input
              type="checkbox"
              class="cs-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isExtracted ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="cs-step-category">[${category}]</strong>
            <span class="cs-step-label">${label}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isExtracted
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary cs-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Subterranean Extraction Clearance Certificate
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card cs-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg · ${sectorGrid})</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-cs-space">${spaceType}</span>
            <span class="badge badge-cs-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row cs-metrics-row">
          <span class="badge badge-cs-space">Hazard Score: ${hazardScore}/100 • Rigging Readiness: ${readinessScore}% • Ø ${diameter}cm • Depth: ${depth}m • O2: ${o2}% • H2S: ${h2s} ppm • Inflow: ${inflow}/100</span>
        </div>

        <div class="cs-flags-row">
          ${flagsHtml}
        </div>

        ${gasBannerHtml}
        ${clearanceBannerHtml}

        <div class="cs-rig-recommendations">
          <div><strong>Optics &amp; Acoustic Rig:</strong> ${opticsRig}</div>
          <div><strong>Ventilation &amp; Damming:</strong> ${ventilation}</div>
        </div>

        <div class="cs-steps-list" aria-label="Confined-Space Borescope, Ventilation &amp; Pipe-Gate Protocol">
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

    const countBadge = document.getElementById('cs-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('cs-ops-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="cs-ops-empty-msg">No confined-space rescue operations recorded yet.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const res = await fetch('/api/v1/confined-space/operations', {
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
    const res = await fetch('/api/v1/confined-space/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchOperations();
      showToast(
        `Evaluated confined-space operation ${created.operationId} for ${created.petName} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate confined-space operation', 'error');
    return null;
  }

  function populateFormWithPreset(preset) {
    const setVal = (id, val) => {
      const el = document.getElementById(id);
      if (el) el.value = String(val);
    };
    setVal('cs-pet-id', preset.petId);
    setVal('cs-pet-name', preset.petName);
    setVal('cs-species', preset.species);
    setVal('cs-weight-kg', preset.weightKg);
    setVal('cs-space-type', preset.spaceType);
    setVal('cs-pipe-diameter', preset.pipeDiameterCm);
    setVal('cs-depth-meters', preset.depthMeters);
    setVal('cs-oxygen-percent', preset.oxygenPercent);
    setVal('cs-h2s-ppm', preset.h2sPpm);
    setVal('cs-inflow-risk', preset.inflowRiskIndex);
    setVal('cs-sector-grid', preset.manholeSectorGrid);
    setVal('cs-tech-callsign', preset.leadTechCallsign);
    setVal('cs-notes', preset.notes);
  }

  function setupFormAndScenarioButtons() {
    const form = document.getElementById('form-evaluate-cs-op');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const payload = {
          petId: (document.getElementById('cs-pet-id')?.value || 'PET-CS-204').trim(),
          petName: (document.getElementById('cs-pet-name')?.value || 'Shadow').trim(),
          species: (document.getElementById('cs-species')?.value || 'Feline').trim(),
          weightKg: parseFloat(document.getElementById('cs-weight-kg')?.value || '4.0') || 4.0,
          spaceType: (document.getElementById('cs-space-type')?.value || 'STORM_DRAIN_CULVERT').trim(),
          pipeDiameterCm: parseFloat(document.getElementById('cs-pipe-diameter')?.value || '34') || 34,
          depthMeters: parseFloat(document.getElementById('cs-depth-meters')?.value || '3.6') || 3.6,
          oxygenPercent: parseFloat(document.getElementById('cs-oxygen-percent')?.value || '18.4') || 18.4,
          h2sPpm: parseFloat(document.getElementById('cs-h2s-ppm')?.value || '13.5') || 0,
          inflowRiskIndex: parseInt(document.getElementById('cs-inflow-risk')?.value || '80', 10) || 0,
          manholeSectorGrid: (document.getElementById('cs-sector-grid')?.value || 'MH-NORTH-42B').trim(),
          leadTechCallsign: (document.getElementById('cs-tech-callsign')?.value || 'TECH-SUB-01').trim(),
          notes: (document.getElementById('cs-notes')?.value || '').trim(),
        };

        const btn = document.getElementById('btn-evaluate-cs-op');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.cs-scenario-btn');
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
    const listEl = document.getElementById('cs-ops-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.cs-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/confined-space/operations/${encodeURIComponent(opId)}/step`,
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
            `Updated step ${stepId} for ${opId} → Readiness ${updated.riggingReadinessScore}% (${updated.status})`,
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
      const clearanceBtn = e.target.closest('.cs-issue-clearance-btn');
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
                `/api/v1/confined-space/operations/${encodeURIComponent(opId)}/step`,
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
          `/api/v1/confined-space/operations/${encodeURIComponent(opId)}/clearance`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              leadTechCallsign: 'TECH-SUB-01 (Vance)',
              notes: 'Subterranean extraction completed and normobaric O2 vitals verified.',
            }),
          }
        );
        if (res.ok) {
          const updated = await res.json();
          await fetchOperations();
          showToast(
            `Issued subterranean clearance ${updated.clearanceCertificateHash} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue subterranean clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing subterranean clearance', 'error');
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

  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/confined-space/stream');
      const refresh = () => {
        fetchOperations();
      };
      sseSource.addEventListener('confined_space_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'confined_space_updated') {
            refresh();
          }
        } catch (_e) {
          // Ignore keepalive comments
        }
      };
    } catch (_err) {
      // Ignore SSE errors in headless environments
    }
  }

  function init() {
    setupThemeToggle();
    setupFormAndScenarioButtons();
    setupCardInteractions();
    fetchOperations();
    setupSSEStream();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  window.addEventListener('beforeunload', () => {
    if (sseSource) {
      sseSource.close();
    }
  });
})();
