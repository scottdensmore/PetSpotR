/**
 * PetSpotR Avian, Exotic Parrot & Falconry Telemetry Receiver,
 * Flight-Corridor Thermals & Lure-Call Recovery Cockpit Controller (Milestone 16.2)
 */

(function () {
  'use strict';

  let currentOps = [];

  const SCENARIO_PRESETS = {
    'macaw-thermal-ridge': {
      petId: 'PET-AV-204',
      petName: 'Kona',
      species: 'Avian',
      weightGrams: 980,
      avianClass: 'PSITTACINE_PARROT_MACAW',
      perchAltitudeMeters: 26.0,
      thermalUpdraftKnots: 22.5,
      ambientTempCelsius: 7.5,
      hoursAloft: 24.0,
      telemetrySignalDbm: -78,
      flightCorridorBearing: 'CORRIDOR-SSE-162° (Discovery Park Bluff Ridge Thermal)',
      leadAvianTechCallsign: 'AVIAN-TECH-01 (Ortega)',
      notes: 'Simulated blue-and-gold macaw in 26m conifer crown with 22.5 kt thermal drift and 7.5°C cold exposure.',
    },
    'falcon-vhf-beacon': {
      petId: 'PET-AV-205',
      petName: 'Valkyrie',
      species: 'Avian',
      weightGrams: 810,
      avianClass: 'FALCONRY_RAPTOR_TELEMETRY',
      perchAltitudeMeters: 13.5,
      thermalUpdraftKnots: 10.5,
      ambientTempCelsius: 16.5,
      hoursAloft: 4.5,
      telemetrySignalDbm: -52,
      flightCorridorBearing: 'CORRIDOR-ENE-074° (Skagit Valley Windbreak Line)',
      leadAvianTechCallsign: 'AVIAN-TECH-04 (Ashford)',
      notes: 'Simulated gyr-peregrine hybrid localized via 216 MHz VHF tail transmitter (-52 dBm); leather lure staged.',
    },
    'cockatiel-courtyard-perch': {
      petId: 'PET-AV-206',
      petName: 'Peaches',
      species: 'Avian',
      weightGrams: 94,
      avianClass: 'COCKATIEL_PARAKEET_SMALL',
      perchAltitudeMeters: 4.5,
      thermalUpdraftKnots: 5.0,
      ambientTempCelsius: 20.5,
      hoursAloft: 2.5,
      telemetrySignalDbm: -42,
      flightCorridorBearing: 'CORRIDOR-WNW-290° (Green Lake Courtyard Maples)',
      leadAvianTechCallsign: 'AVIAN-TECH-02 (Lin)',
      notes: 'Simulated lutino cockatiel on low courtyard maple branch; millet spray and flock contact call staged.',
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
    const liveStatus = document.getElementById('avian-recovery-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function getStatusBadgeClass(status) {
    switch (status) {
      case 'THERMAL_DRIFT_PREDATION_CRITICAL':
        return 'badge-av-critical';
      case 'ACOUSTIC_LURE_PERCH_STAGED':
        return 'badge-av-staged';
      case 'RECOVERED_AVIARY_SAFE':
        return 'badge-av-recovered';
      default:
        return 'badge-av-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('stat-total-avian-ops');
    const thermalEl = document.getElementById('stat-critical-thermal-alerts');
    const stagedEl = document.getElementById('stat-staged-acoustic-lures');
    const recoveredEl = document.getElementById('stat-recovered-aviary-count');

    const total = ops.length;
    const criticalThermal = ops.filter(
      (op) =>
        Boolean(op.thermalDriftAlert) ||
        Boolean(op.cropStarvationColdAlert) ||
        op.status === 'THERMAL_DRIFT_PREDATION_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'ACOUSTIC_LURE_PERCH_STAGED'
    ).length;
    const recovered = ops.filter(
      (op) => op.status === 'RECOVERED_AVIARY_SAFE'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (thermalEl) thermalEl.textContent = String(criticalThermal);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (recoveredEl) recoveredEl.textContent = String(recovered);
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.operationId || '');
    const petName = escapeHtml(op.petName || '');
    const species = escapeHtml(op.species || 'Avian');
    const weightGrams = Number(op.weightGrams ?? 500).toFixed(0);
    const corridorBearing = escapeHtml(op.flightCorridorBearing || 'CORRIDOR-SSE-155°');
    const avianClass = escapeHtml(op.avianClass || 'PSITTACINE_PARROT_MACAW');
    const status = escapeHtml(op.status || 'THERMAL_DRIFT_PREDATION_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const hazardScore = Number(op.hazardScore ?? 50);
    const readinessScore = Number(op.lureReadinessScore ?? 0);
    const altitude = Number(op.perchAltitudeMeters ?? 20.0).toFixed(1);
    const updraft = Number(op.thermalUpdraftKnots ?? 15.0).toFixed(1);
    const temp = Number(op.ambientTempCelsius ?? 12.0).toFixed(1);
    const hoursAloft = Number(op.hoursAloft ?? 10.0).toFixed(1);
    const signalDbm = Number(op.telemetrySignalDbm ?? -65);
    const lureAudio = escapeHtml(op.recommendedLureAudio || '');
    const captureRig = escapeHtml(op.recommendedCaptureRig || '');
    const callsign = escapeHtml(op.leadAvianTechCallsign || 'AVIAN-TECH-01');

    const flags = Array.isArray(op.riskFlags) ? op.riskFlags : [];
    const flagsHtml = flags
      .map((f) => `<span class="badge badge-av-flag">${escapeHtml(f)}</span>`)
      .join('');

    const thermalBannerHtml =
      op.thermalDriftAlert || op.cropStarvationColdAlert
        ? `
          <div class="av-thermal-alert-banner">
            ⚠️ FLIGHT-CORRIDOR THERMAL DRIFT &amp; CROP DEPLETION ALERT: Perch ${altitude}m · Updraft ${updraft} kt · Temp ${temp}°C · Aloft ${hoursAloft}h — Stage 45° sightline perch &amp; directional conspecific acoustic lure immediately.
          </div>
        `
        : '';

    const clearanceBannerHtml = op.clearanceCertificateHash
      ? `
          <div class="av-clearance-banner">
            ✅ AVIAN RECOVERY CLEARANCE VERIFIED: ${escapeHtml(op.clearanceCertificateHash)} (${callsign})
          </div>
        `
      : '';

    const isRecovered = op.status === 'RECOVERED_AVIARY_SAFE';
    const steps = Array.isArray(op.protocolSteps) ? op.protocolSteps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const category = escapeHtml(step.category || '');
        const label = escapeHtml(step.label || '');
        const badgeClass = done ? 'badge-av-recovered' : 'badge-av-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="av-step-item">
            <input
              type="checkbox"
              class="av-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isRecovered ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="av-step-category">[${category}]</strong>
            <span class="av-step-label">${label}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isRecovered
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary av-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Avian Aviary Safe Recovery Certificate
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card av-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightGrams}g · ${corridorBearing})</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-av-class">${avianClass}</span>
            <span class="badge badge-av-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row av-metrics-row">
          <span class="badge badge-av-class">Hazard Score: ${hazardScore}/100 • Lure Readiness: ${readinessScore}% • Perch: ${altitude}m • Updraft: ${updraft} kt • Temp: ${temp}°C • Aloft: ${hoursAloft}h • VHF: ${signalDbm} dBm</span>
        </div>

        <div class="av-flags-row">
          ${flagsHtml}
        </div>

        ${thermalBannerHtml}
        ${clearanceBannerHtml}

        <div class="av-rig-recommendations">
          <div><strong>Acoustic Lure &amp; VHF Receiver:</strong> ${lureAudio}</div>
          <div><strong>Perch &amp; Restraint Rig:</strong> ${captureRig}</div>
        </div>

        <div class="av-steps-list" aria-label="Avian Telemetry, Acoustic Lure &amp; Zero-Keel-Compression Protocol">
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

    const countBadge = document.getElementById('av-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('av-ops-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="av-ops-empty-msg">No avian recovery operations recorded yet.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const res = await fetch('/api/v1/avian-recovery/operations', {
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
    const res = await fetch('/api/v1/avian-recovery/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchOperations();
      showToast(
        `Evaluated avian recovery operation ${created.operationId} for ${created.petName} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate avian recovery operation', 'error');
    return null;
  }

  function populateFormWithPreset(preset) {
    const setVal = (id, val) => {
      const el = document.getElementById(id);
      if (el) el.value = String(val);
    };
    setVal('av-pet-id', preset.petId);
    setVal('av-pet-name', preset.petName);
    setVal('av-species', preset.species);
    setVal('av-weight-grams', preset.weightGrams);
    setVal('av-class-type', preset.avianClass);
    setVal('av-perch-altitude', preset.perchAltitudeMeters);
    setVal('av-thermal-updraft', preset.thermalUpdraftKnots);
    setVal('av-ambient-temp', preset.ambientTempCelsius);
    setVal('av-hours-aloft', preset.hoursAloft);
    setVal('av-telemetry-dbm', preset.telemetrySignalDbm);
    setVal('av-corridor-bearing', preset.flightCorridorBearing);
    setVal('av-tech-callsign', preset.leadAvianTechCallsign);
    setVal('av-notes', preset.notes);
  }

  function setupFormAndScenarioButtons() {
    const form = document.getElementById('form-evaluate-av-op');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const payload = {
          petId: (document.getElementById('av-pet-id')?.value || 'PET-AV-204').trim(),
          petName: (document.getElementById('av-pet-name')?.value || 'Kona').trim(),
          species: (document.getElementById('av-species')?.value || 'Avian').trim(),
          weightGrams: parseFloat(document.getElementById('av-weight-grams')?.value || '980') || 980,
          avianClass: (document.getElementById('av-class-type')?.value || 'PSITTACINE_PARROT_MACAW').trim(),
          perchAltitudeMeters: parseFloat(document.getElementById('av-perch-altitude')?.value || '25.0') || 25.0,
          thermalUpdraftKnots: parseFloat(document.getElementById('av-thermal-updraft')?.value || '21.5') || 0,
          ambientTempCelsius: parseFloat(document.getElementById('av-ambient-temp')?.value || '8.0') || 8.0,
          hoursAloft: parseFloat(document.getElementById('av-hours-aloft')?.value || '22.0') || 0,
          telemetrySignalDbm: parseInt(document.getElementById('av-telemetry-dbm')?.value || '-79', 10) || -79,
          flightCorridorBearing: (document.getElementById('av-corridor-bearing')?.value || 'CORRIDOR-SSE-162°').trim(),
          leadAvianTechCallsign: (document.getElementById('av-tech-callsign')?.value || 'AVIAN-TECH-01').trim(),
          notes: (document.getElementById('av-notes')?.value || '').trim(),
        };

        const btn = document.getElementById('btn-evaluate-av-op');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.av-scenario-btn');
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
    const listEl = document.getElementById('av-ops-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.av-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/avian-recovery/operations/${encodeURIComponent(opId)}/step`,
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
            `Updated step ${stepId} for ${opId} → Lure Readiness ${updated.lureReadinessScore}% (${updated.status})`,
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
      const clearanceBtn = e.target.closest('.av-issue-clearance-btn');
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
                `/api/v1/avian-recovery/operations/${encodeURIComponent(opId)}/step`,
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
          `/api/v1/avian-recovery/operations/${encodeURIComponent(opId)}/clearance`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              leadAvianTechCallsign: 'AVIAN-TECH-01 (Ortega)',
              notes: 'Zero-keel-compression restraint and 30°C warmed isolette aviary transfer verified.',
            }),
          }
        );
        if (res.ok) {
          const updated = await res.json();
          await fetchOperations();
          showToast(
            `Issued avian recovery clearance ${updated.clearanceCertificateHash} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue avian clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing avian clearance', 'error');
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
