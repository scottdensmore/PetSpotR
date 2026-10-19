/**
 * PetSpotR Hyperbaric Oxygen (HBOT), Carbon Monoxide (CO) Carboxyhemoglobin
 * & Cyanide Smoke Triage Cockpit Controller (Milestone 18.1)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'HB-OP-101';

  const SCENARIO_PRESETS = {
    'house-fire-cyanide': {
      pet_id: 'PET-HB-204',
      pet_name: 'Phoenix',
      species: 'Cat - Siamese',
      weight_kg: 4.6,
      exposure_source: 'STRUCTURAL_HOUSE_FIRE_SYNTHETICS',
      carboxyhemoglobin_pct: 36.8,
      blood_lactate_mmol_l: 12.4,
      chamber_pressure_ata: 2.2,
      singe_vibrissae_soot_nares: true,
      pulse_oximetry_false_normal_noted: true,
      hydroxocobalamin_administered: true,
    },
    'garage-exhaust-co': {
      pet_id: 'PET-HB-205',
      pet_name: 'Baxter',
      species: 'Dog - Labrador Retriever',
      weight_kg: 28.0,
      exposure_source: 'ENCLOSED_GARAGE_EXHAUST_CO',
      carboxyhemoglobin_pct: 22.5,
      blood_lactate_mmol_l: 4.8,
      chamber_pressure_ata: 2.0,
      singe_vibrissae_soot_nares: false,
      pulse_oximetry_false_normal_noted: true,
      hydroxocobalamin_administered: false,
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
    const liveStatus = document.getElementById('hbot-triage-live-status');
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
      case 'CARBOXYHEMOGLOBIN_CYANIDE_CRITICAL':
        return 'badge-hbot-critical';
      case 'HBOT_CHAMBER_HYDROXOCOBALAMIN_STAGED':
        return 'badge-hbot-staged';
      case 'NORMOXIC_COHB_CLEARED_SAFE':
        return 'badge-hbot-safe';
      default:
        return 'badge-hbot-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-cohb');
    const stagedEl = document.getElementById('kpi-staged-hbot');
    const safeEl = document.getElementById('kpi-cleared-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'CARBOXYHEMOGLOBIN_CYANIDE_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'HBOT_CHAMBER_HYDROXOCOBALAMIN_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'NORMOXIC_COHB_CLEARED_SAFE'
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
        '<p class="trust-empty-hint">No active hyperbaric CO triage operation selected.</p>';
      return;
    }

    const active =
      ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;

    const opId = escapeHtml(active.id || '');
    const petId = escapeHtml(active.pet_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(active.species || '');
    const weightKg = Number(active.weight_kg ?? 10.0).toFixed(1);
    const source = escapeHtml(active.exposure_source || '');
    const status = escapeHtml(active.status || '');
    const statusClass = getStatusBadgeClass(active.status);
    const warningNote = escapeHtml(active.co_oximetry_warning_note || '');
    const hbotRegimen = escapeHtml(active.required_hbot_regimen || '');
    const clearance = active.clearance_token
      ? `<div class="hbot-clearance-banner">✅ NORMOXIC COHb &amp; HBOT WASHOUT CLEARANCE VERIFIED: ${escapeHtml(active.clearance_token)}</div>`
      : '';

    workspaceEl.innerHTML = `
      <div class="hbot-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">Active Workspace: ${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${petId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-hbot-source">${source}</span>
            <span class="badge badge-hbot-status ${statusClass}">${status}</span>
          </div>
        </div>
        <div class="hbot-cooximetry-alert-banner">
          🫧 ${warningNote}
        </div>
        ${clearance}
        <div class="hbot-regimen-recommendations">
          <div><strong>Required HBOT &amp; Cyanide Chelation Regimen:</strong> ${hbotRegimen}</div>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const weightKg = Number(op.weight_kg ?? 10.0).toFixed(1);
    const exposureSource = escapeHtml(op.exposure_source || 'STRUCTURAL_HOUSE_FIRE_SYNTHETICS');
    const status = escapeHtml(op.status || 'CARBOXYHEMOGLOBIN_CYANIDE_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const cohbPct = Number(op.carboxyhemoglobin_pct ?? 25.0).toFixed(1);
    const lactateMmol = Number(op.blood_lactate_mmol_l ?? 8.0).toFixed(1);
    const chamberAta = Number(op.chamber_pressure_ata ?? 2.0).toFixed(1);
    const warningNote = escapeHtml(op.co_oximetry_warning_note || '');
    const hbotRegimen = escapeHtml(op.required_hbot_regimen || '');

    const flags = [];
    if (op.singe_vibrissae_soot_nares) {
      flags.push('SINGED_VIBRISSAE_SOOT_NARES');
    }
    if (Number(op.carboxyhemoglobin_pct) >= 20.0) {
      flags.push('SEVERE_CARBOXYHEMOGLOBIN_HYPOXIA');
    }
    if (Number(op.blood_lactate_mmol_l) >= 8.0) {
      flags.push('CYANIDE_LACTATEMIA_HISTOTOXIC_CRISIS');
    }
    if (!op.pulse_oximetry_false_normal_noted) {
      flags.push('FALSE_NORMAL_SPO2_UNVERIFIED_HAZARD');
    }

    const flagsHtml = flags
      .map((f) => `<span class="badge badge-hbot-flag">${escapeHtml(f)}</span>`)
      .join('');

    const clearanceBannerHtml = op.clearance_token
      ? `
          <div class="hbot-clearance-banner">
            ✅ NORMOXIC COHb &amp; HBOT WASHOUT CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
          </div>
        `
      : '';

    const isSafe = op.status === 'NORMOXIC_COHB_CLEARED_SAFE';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const desc = escapeHtml(step.description || '');
        const badgeClass = done ? 'badge-hbot-safe' : 'badge-hbot-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="hbot-step-item">
            <input
              type="checkbox"
              class="hbot-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="hbot-step-title">${title}</strong>
            <span class="hbot-step-desc">${desc}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary hbot-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Normoxic COHb Clearance (${opId})
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card hbot-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-exposure-source="${exposureSource}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-hbot-source">${exposureSource}</span>
            <span class="badge badge-hbot-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row hbot-metrics-row">
          <span class="badge badge-hbot-source">COHb: ${cohbPct}% • Blood Lactate: ${lactateMmol} mmol/L • HBOT Chamber: ${chamberAta} ATA</span>
        </div>

        <div class="hbot-flags-row">
          ${flagsHtml}
        </div>

        <div class="hbot-cooximetry-alert-banner">
          ⚠️ ${warningNote}
        </div>

        ${clearanceBannerHtml}

        <div class="hbot-regimen-recommendations">
          <div><strong>Required HBOT Regimen:</strong> ${hbotRegimen}</div>
        </div>

        <div class="hbot-steps-list" aria-label="CO-Oximetry, Hydroxocobalamin &amp; Hyperbaric Chamber Protocol Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary hbot-select-workspace-btn"
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

    const countBadge = document.getElementById('hbot-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('hbot-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="hbot-ops-empty-msg">No hyperbaric oxygen or CO/cyanide triage operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const sourceFilter = (document.getElementById('filter-exposure-source')?.value || '').trim();
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const params = new URLSearchParams();
      if (sourceFilter) params.set('exposure_source', sourceFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/hyperbaric-co/operations?${qs}`
        : '/api/v1/hyperbaric-co/operations';

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
    const res = await fetch('/api/v1/hyperbaric-co/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      selectedOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated hyperbaric CO operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error || 'Failed to evaluate hyperbaric CO triage operation';
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
    setVal('eval-exposure-source', preset.exposure_source);
    setVal('eval-cohb-pct', preset.carboxyhemoglobin_pct);
    setVal('eval-lactate-mmol', preset.blood_lactate_mmol_l);
    setVal('eval-chamber-ata', preset.chamber_pressure_ata);
    setCheck('eval-singe-soot', preset.singe_vibrissae_soot_nares);
    setCheck('eval-spo2-warning-noted', preset.pulse_oximetry_false_normal_noted);
    setCheck('eval-hydroxocobalamin', preset.hydroxocobalamin_administered);
  }

  function setupFormAndFilters() {
    const form = document.getElementById('hbot-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petName = (document.getElementById('eval-pet-name')?.value || '').trim();
        const species = (document.getElementById('eval-species')?.value || 'Dog - Mixed Breed').trim();
        const weightKg = parseFloat(document.getElementById('eval-weight-kg')?.value || '0');
        const exposureSource = (document.getElementById('eval-exposure-source')?.value || 'STRUCTURAL_HOUSE_FIRE_SYNTHETICS').trim();
        const cohbPct = parseFloat(document.getElementById('eval-cohb-pct')?.value || '0');
        const lactateMmol = parseFloat(document.getElementById('eval-lactate-mmol')?.value || '0');
        const chamberAta = parseFloat(document.getElementById('eval-chamber-ata')?.value || '0');
        const singeSoot = Boolean(document.getElementById('eval-singe-soot')?.checked);
        const spo2Noted = Boolean(document.getElementById('eval-spo2-warning-noted')?.checked);
        const hydroxocobalamin = Boolean(document.getElementById('eval-hydroxocobalamin')?.checked);

        if (!petName) {
          showErrorBanner('pet_name is required');
          return;
        }

        const payload = {
          pet_name: petName,
          species,
          weight_kg: isNaN(weightKg) ? 0 : weightKg,
          exposure_source: exposureSource,
          carboxyhemoglobin_pct: isNaN(cohbPct) ? -1 : cohbPct,
          blood_lactate_mmol_l: isNaN(lactateMmol) ? 0 : lactateMmol,
          chamber_pressure_ata: isNaN(chamberAta) ? 0 : chamberAta,
          singe_vibrissae_soot_nares: singeSoot,
          pulse_oximetry_false_normal_noted: spo2Noted,
          hydroxocobalamin_administered: hydroxocobalamin,
        };

        const btn = document.getElementById('btn-submit-hbot-eval');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.hbot-scenario-btn');
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

    const filterSource = document.getElementById('filter-exposure-source');
    if (filterSource) {
      filterSource.addEventListener('change', () => {
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
    const listEl = document.getElementById('hbot-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.hbot-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/hyperbaric-co/operations/${encodeURIComponent(opId)}/steps`,
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
      const workspaceBtn = e.target.closest('.hbot-select-workspace-btn');
      if (workspaceBtn) {
        const opId = workspaceBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const clearanceBtn = e.target.closest('.hbot-issue-clearance-btn');
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
                `/api/v1/hyperbaric-co/operations/${encodeURIComponent(opId)}/steps`,
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
          `/api/v1/hyperbaric-co/operations/${encodeURIComponent(opId)}/clearance`,
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
            `Issued normoxic COHb clearance ${updated.clearance_token} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue hyperbaric clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing hyperbaric clearance', 'error');
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
