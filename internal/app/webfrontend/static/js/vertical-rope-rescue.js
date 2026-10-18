/**
 * PetSpotR High-Rise, Cliffside, Tree-Canopy & Technical Rope/Arborist
 * Vertical Rescue Cockpit Controller (Milestone 15.5)
 */

(function () {
  'use strict';

  const SCENARIO_PRESETS = {
    'canopy-crown-gale': {
      petId: 'PET-VR-910',
      petName: 'Willow',
      species: 'Feline',
      weightKg: 3.9,
      environmentType: 'TREE_CANOPY_CROWN',
      elevationMeters: 26.0,
      slopeAngleDeg: 86.0,
      windGustKnots: 31.0,
      hoursStranded: 40.0,
      branchStabilityIndex: 18,
      leadRiggerCallsign: 'ARBOR-11 (ISA Certified Canopy Specialist)',
      notes: 'Cat stranded 26m high in wind-sheared deadwood cottonwood crown; deploying 8m ballistic catch net and SRS twin-line.',
    },
    'cliff-ledge-crag': {
      petId: 'PET-VR-911',
      petName: 'Summit',
      species: 'Canine',
      weightKg: 24.0,
      environmentType: 'CLIFFSIDE_ROCK_LEDGE',
      elevationMeters: 16.5,
      slopeAngleDeg: 82.0,
      windGustKnots: 14.0,
      hoursStranded: 9.0,
      branchStabilityIndex: 75,
      leadRiggerCallsign: 'ROPE-TECH-8 (NFPA 2500 High-Angle Lead)',
      notes: 'Sheer basalt gorge ledge 16.5m above scree field; rigging vortex high-directional TTRS and canine dorsal harness.',
    },
    'highrise-balcony-ledge': {
      petId: 'PET-VR-912',
      petName: 'Pixel',
      species: 'Feline',
      weightKg: 4.4,
      environmentType: 'HIGH_RISE_BALCONY_PARAPET',
      elevationMeters: 28.0,
      slopeAngleDeg: 90.0,
      windGustKnots: 19.0,
      hoursStranded: 14.0,
      branchStabilityIndex: 88,
      leadRiggerCallsign: 'TOWER-RIG-3 (Urban High-Rise Specialist)',
      notes: '9th-floor concrete balcony parapet ledge; high feline panic-leap risk; deploying dark-mesh calming bag and plaza cushion.',
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
    const liveStatus = document.getElementById('vertical-rescue-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function getStatusBadgeClass(status) {
    switch (status) {
      case 'FALL_SHOCK_HAZARD_CRITICAL':
        return 'badge-vr-critical';
      case 'TWIN_TENSION_RIGGED_ASCENT':
        return 'badge-vr-rigged';
      case 'LOWERED_GROUND_SAFE':
        return 'badge-vr-lowered';
      default:
        return 'badge-vr-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('stat-total-vertical-ops');
    const criticalEl = document.getElementById('stat-critical-fall-alerts');
    const riggedEl = document.getElementById('stat-rigged-twin-lines');
    const loweredEl = document.getElementById('stat-lowered-safe-count');
    const countBadge = document.getElementById('vr-ops-count-badge');

    const total = ops.length;
    const critical = ops.filter(
      (op) =>
        Boolean(op.highRiseFallShockAlert) ||
        Boolean(op.canopyFailureAlert) ||
        op.status === 'FALL_SHOCK_HAZARD_CRITICAL'
    ).length;
    const rigged = ops.filter(
      (op) => op.status === 'TWIN_TENSION_RIGGED_ASCENT'
    ).length;
    const lowered = ops.filter(
      (op) => op.status === 'LOWERED_GROUND_SAFE'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (riggedEl) riggedEl.textContent = String(rigged);
    if (loweredEl) loweredEl.textContent = String(lowered);
    if (countBadge) countBadge.textContent = `${total} Ops`;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.operationId || '');
    const petId = escapeHtml(op.petId || '');
    const petName = escapeHtml(op.petName || '');
    const species = escapeHtml(op.species || 'Feline');
    const weightKg = Number(op.weightKg ?? 4.2).toFixed(1);
    const callsign = escapeHtml(op.leadRiggerCallsign || 'ARBOR-LEAD-1');
    const status = escapeHtml(op.status || 'FALL_SHOCK_HAZARD_CRITICAL');
    const statusBadgeClass = getStatusBadgeClass(op.status);
    const envType = escapeHtml(op.environmentType || 'TREE_CANOPY_CROWN');
    const elevation = Number(op.elevationMeters ?? 15.0).toFixed(1);
    const slope = Number(op.slopeAngleDeg ?? 80).toFixed(0);
    const wind = Number(op.windGustKnots ?? 15).toFixed(0);
    const stability = Number(op.branchStabilityIndex ?? 50);
    const hazardScore = Number(op.hazardScore ?? 75);
    const readinessScore = Number(op.riggingReadinessScore ?? 0);
    const ropeSys = escapeHtml(op.recommendedRopeSystem || '');
    const captureBag = escapeHtml(op.recommendedCaptureBag || '');
    const dropZone = escapeHtml(op.dropZoneGrid || '');

    const flags = Array.isArray(op.riskFlags) ? op.riskFlags : [];
    const flagsHtml = flags
      .map((flag) => `<span class="badge badge-vr-flag">${escapeHtml(flag)}</span>`)
      .join('');

    const alertBannerHtml =
      op.highRiseFallShockAlert || op.canopyFailureAlert
        ? `
          <div class="vr-fall-alert-banner" role="alert">
            ⚠️ HIGH-RISE FALL-SHOCK &amp; CANOPY SHEAR ALERT: ${elevation}m elevation (${wind} kt gusts, Stability ${stability}/100) — Deploy ${dropZone} &amp; ${captureBag} immediately.
          </div>
        `
        : '';

    const clearanceBannerHtml = op.clearanceCertificateHash
      ? `
        <div class="vr-clearance-banner">
          ✅ VERTICAL GROUND-SAFE LOWERING CERTIFIED: <strong>${escapeHtml(op.clearanceCertificateHash)}</strong>
        </div>
      `
      : '';

    const steps = Array.isArray(op.protocolSteps) ? op.protocolSteps : [];
    const stepsHtml = steps
      .map((step) => {
        const stepId = escapeHtml(step.id || '');
        const category = escapeHtml(step.category || '');
        const label = escapeHtml(step.label || '');
        const checkedAttr = step.completed ? 'checked' : '';
        return `
          <label class="vr-step-item">
            <input
              type="checkbox"
              class="vr-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${checkedAttr}
            >
            <span class="badge badge-vr-env">[${category}]</span>
            <span class="vr-step-label">${label}</span>
          </label>
        `;
      })
      .join('');

    return `
      <article class="trust-screening-card vr-op-card" data-op-id="${opId}" data-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg · ${callsign})</strong>
            <code class="trust-pet-id">${opId} · ${petId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-vr-status ${statusBadgeClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-vr-env">${envType}</span>
          <span class="badge badge-vr-env">Elevation: ${elevation}m • Slope: ${slope}° • Wind: ${wind} kts • Stability: ${stability}/100</span>
          <span class="badge badge-vr-env">Hazard Score: ${hazardScore}/100 • Rigging Readiness: ${readinessScore}%</span>
        </div>

        <div class="vr-flags-row">
          ${flagsHtml}
        </div>

        ${alertBannerHtml}
        ${clearanceBannerHtml}

        <div class="vr-equipment-summary">
          <div><strong>Rope Rigging:</strong> ${ropeSys}</div>
          <div><strong>Capture Restraint:</strong> ${captureBag}</div>
          <div><strong>Drop-Zone Netting:</strong> ${dropZone}</div>
        </div>

        <div class="vr-steps-list" aria-label="NFPA 2500 / ANSI Z133 Vertical Rescue Protocol Checklist">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-primary vr-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🏅 Issue Ground-Safe Vertical Clearance Certificate
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops) {
    updateSummaryStats(ops);
    const listEl = document.getElementById('vr-operations-list');
    if (!listEl) return;
    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const res = await fetch('/api/v1/vertical-rescue/operations', {
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

  async function submitEvaluation(payload) {
    const res = await fetch('/api/v1/vertical-rescue/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchOperations();
      showToast(
        `Evaluated vertical rescue ${created.operationId} for ${created.petName} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate vertical rescue operation', 'error');
    return null;
  }

  function populateFormWithPreset(preset) {
    const setVal = (id, val) => {
      const el = document.getElementById(id);
      if (el) el.value = String(val);
    };
    setVal('vr-pet-id', preset.petId);
    setVal('vr-pet-name', preset.petName);
    setVal('vr-species', preset.species);
    setVal('vr-weight-kg', preset.weightKg);
    setVal('vr-environment-type', preset.environmentType);
    setVal('vr-elevation-meters', preset.elevationMeters);
    setVal('vr-slope-angle', preset.slopeAngleDeg);
    setVal('vr-wind-gust', preset.windGustKnots);
    setVal('vr-hours-stranded', preset.hoursStranded);
    setVal('vr-branch-stability', preset.branchStabilityIndex);
    setVal('vr-lead-rigger', preset.leadRiggerCallsign);
    setVal('vr-notes', preset.notes);
  }

  function setupFormAndSimulators() {
    const form = document.getElementById('vr-evaluate-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const payload = {
          petId: (document.getElementById('vr-pet-id')?.value || 'PET-VR-904').trim(),
          petName: (document.getElementById('vr-pet-name')?.value || 'Zephyr').trim(),
          species: (document.getElementById('vr-species')?.value || 'Feline').trim(),
          weightKg: parseFloat(document.getElementById('vr-weight-kg')?.value || '4.3'),
          environmentType: (
            document.getElementById('vr-environment-type')?.value || 'TREE_CANOPY_CROWN'
          ).trim(),
          elevationMeters: parseFloat(
            document.getElementById('vr-elevation-meters')?.value || '24.0'
          ),
          slopeAngleDeg: parseFloat(document.getElementById('vr-slope-angle')?.value || '85'),
          windGustKnots: parseFloat(document.getElementById('vr-wind-gust')?.value || '28'),
          hoursStranded: parseFloat(
            document.getElementById('vr-hours-stranded')?.value || '30'
          ),
          branchStabilityIndex: parseInt(
            document.getElementById('vr-branch-stability')?.value || '22',
            10
          ),
          leadRiggerCallsign: (
            document.getElementById('vr-lead-rigger')?.value || 'ARBOR-LEAD-9'
          ).trim(),
          notes: (document.getElementById('vr-notes')?.value || '').trim(),
        };

        await submitEvaluation(payload);
      });
    }

    const simButtons = document.querySelectorAll('.vr-scenario-btn');
    simButtons.forEach((btn) => {
      btn.addEventListener('click', async () => {
        const scenarioKey = btn.getAttribute('data-scenario');
        const preset = SCENARIO_PRESETS[scenarioKey];
        if (!preset) return;
        populateFormWithPreset(preset);
        await submitEvaluation(preset);
      });
    });
  }

  function setupCardInteractions() {
    const listEl = document.getElementById('vr-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.vr-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/vertical-rescue/operations/${encodeURIComponent(opId)}/step`,
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
            `Updated step ${stepId} on ${opId} (Rigging Readiness: ${updated.riggingReadinessScore}%)`,
            'info'
          );
        }
      } finally {
        checkbox.disabled = false;
      }
    });

    listEl.addEventListener('click', async (e) => {
      const clearanceBtn = e.target.closest('.vr-issue-clearance-btn');
      if (!clearanceBtn) return;

      const opId = clearanceBtn.getAttribute('data-op-id');
      if (!opId) return;

      clearanceBtn.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/vertical-rescue/operations/${encodeURIComponent(opId)}/clearance`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
          }
        );
        if (res.ok) {
          const cleared = await res.json();
          await fetchOperations();
          showToast(
            `Issued vertical clearance ${cleared.clearanceCertificateHash} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Complete all required rigging steps before clearance', 'error');
        }
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
    setupFormAndSimulators();
    setupCardInteractions();
    fetchOperations();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
