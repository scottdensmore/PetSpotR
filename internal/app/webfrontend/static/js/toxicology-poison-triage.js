/**
 * PetSpotR Anticoagulant Rodenticide, Xylitol Hypoglycemia/Hepatotoxicity
 * & Lipid Emulsion (ILE) Poison Control Cockpit Controller (Milestone 19.2)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'TP-OP-101';

  const SCENARIO_PRESETS = {
    'xylitol-hypoglycemia-hepatic': {
      pet_id: 'PET-TP-204',
      pet_name: 'Mochi',
      species: 'Dog - French Bulldog',
      weight_kg: 11.2,
      agent_type: 'XYLITOL_INSULIN_SURGE_HEPATOTOXIN',
      estimated_dose_mg_kg: 540.0,
      prothrombin_time_ratio: 1.9,
      blood_glucose_mg_dl: 38.0,
      time_since_ingestion_mins: 40,
      charcoal_avoided_for_xylitol: true,
      vitamin_k3_menadione_avoided: true,
    },
    'feline-lily-nephrotoxin': {
      pet_id: 'PET-TP-205',
      pet_name: 'Cleo',
      species: 'Cat - Domestic Shorthair',
      weight_kg: 4.3,
      agent_type: 'LILIUM_SPP_FELINE_NEPHROTOXIN',
      estimated_dose_mg_kg: 85.0,
      prothrombin_time_ratio: 1.0,
      blood_glucose_mg_dl: 108.0,
      time_since_ingestion_mins: 35,
      charcoal_avoided_for_xylitol: true,
      vitamin_k3_menadione_avoided: true,
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
    const liveStatus = document.getElementById('poison-triage-live-status');
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
      case 'COAGULOPATHY_HEPATIC_NEURO_TOXIC_CRITICAL':
        return 'badge-poison-critical';
      case 'VITAMIN_K1_ILE_CHARCOAL_DECON_STAGED':
        return 'badge-poison-staged';
      case 'DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED':
        return 'badge-poison-safe';
      default:
        return 'badge-poison-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-poison');
    const stagedEl = document.getElementById('kpi-staged-decon');
    const safeEl = document.getElementById('kpi-cleared-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'COAGULOPATHY_HEPATIC_NEURO_TOXIC_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'VITAMIN_K1_ILE_CHARCOAL_DECON_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED'
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
        '<p class="trust-empty-hint">No active toxicology poison triage operation selected.</p>';
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
    const agentType = escapeHtml(active.agent_type || '');
    const status = escapeHtml(active.status || '');
    const statusClass = getStatusBadgeClass(active.status);
    const warningNote = escapeHtml(active.toxicology_pitfall_warning_note || '');
    const regimen = escapeHtml(active.required_antidote_decon_regimen || '');
    const clearance = active.clearance_token
      ? `<div class="poison-clearance-banner">✅ DECONTAMINATED PT &amp; GLUCOSE NORMAL CLEARANCE VERIFIED: ${escapeHtml(active.clearance_token)}</div>`
      : '';

    workspaceEl.innerHTML = `
      <div class="poison-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">Active Workspace: ${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${petId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-poison-agent">${agentType}</span>
            <span class="badge badge-poison-status ${statusClass}">${status}</span>
          </div>
        </div>
        <div class="poison-pitfall-alert-banner">
          ⚠️ ${warningNote}
        </div>
        ${clearance}
        <div class="poison-regimen-recommendations">
          <div><strong>Required Antidote &amp; Decontamination Regimen:</strong> ${regimen}</div>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const weightKg = Number(op.weight_kg ?? 10.0).toFixed(1);
    const agentType = escapeHtml(op.agent_type || 'XYLITOL_INSULIN_SURGE_HEPATOTOXIN');
    const status = escapeHtml(op.status || 'COAGULOPATHY_HEPATIC_NEURO_TOXIC_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const doseMgKg = Number(op.estimated_dose_mg_kg ?? 100.0).toFixed(1);
    const ptRatio = Number(op.prothrombin_time_ratio ?? 1.0).toFixed(1);
    const glucoseMgDl = Number(op.blood_glucose_mg_dl ?? 90.0).toFixed(1);
    const elapsedMins = Number(op.time_since_ingestion_mins ?? 45);
    const warningNote = escapeHtml(op.toxicology_pitfall_warning_note || '');
    const regimen = escapeHtml(op.required_antidote_decon_regimen || '');

    const flags = [];
    if (Number(op.blood_glucose_mg_dl) < 60.0) {
      flags.push('XYLITOL_HYPOGLYCEMIC_CRISIS');
    }
    if (Number(op.prothrombin_time_ratio) >= 1.5) {
      flags.push('PROLONGED_PT_COAGULOPATHY_ALERT');
    }
    if (op.agent_type === 'LILIUM_SPP_FELINE_NEPHROTOXIN') {
      flags.push('FELINE_LILY_ANURIC_RENAL_RISK');
    }
    if (op.agent_type === 'BROMETHALIN_NEUROTOXIC_CEREBRAL_EDEMA') {
      flags.push('BROMETHALIN_CEREBRAL_EDEMA_ALERT');
    }
    if (!op.charcoal_avoided_for_xylitol) {
      flags.push('CHARCOAL_XYLITOL_PITFALL_HAZARD');
    }
    if (!op.vitamin_k3_menadione_avoided) {
      flags.push('VITAMIN_K3_MENADIONE_NEPHROTOXIC_HAZARD');
    }

    const flagsHtml = flags
      .map((f) => `<span class="badge badge-poison-flag">${escapeHtml(f)}</span>`)
      .join('');

    const clearanceBannerHtml = op.clearance_token
      ? `
          <div class="poison-clearance-banner">
            ✅ DECONTAMINATED PT &amp; GLUCOSE NORMAL CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
          </div>
        `
      : '';

    const isSafe = op.status === 'DECONTAMINATED_PT_GLUCOSE_NORMAL_CLEARED';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const desc = escapeHtml(step.description || '');
        const badgeClass = done ? 'badge-poison-safe' : 'badge-poison-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="poison-step-item">
            <input
              type="checkbox"
              class="poison-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="poison-step-title">${title}</strong>
            <span class="poison-step-desc">${desc}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary poison-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Toxicology &amp; Poison Clearance (${opId})
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card poison-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-agent-type="${agentType}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-poison-agent">${agentType}</span>
            <span class="badge badge-poison-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row poison-metrics-row">
          <span class="badge badge-poison-agent">Dose: ${doseMgKg} mg/kg • PT Ratio: ${ptRatio}x • Blood Glucose: ${glucoseMgDl} mg/dL • Elapsed: ${elapsedMins} mins</span>
        </div>

        <div class="poison-flags-row">
          ${flagsHtml}
        </div>

        <div class="poison-pitfall-alert-banner">
          ⚠️ ${warningNote}
        </div>

        ${clearanceBannerHtml}

        <div class="poison-regimen-recommendations">
          <div><strong>Required Antidote &amp; Decontamination Regimen:</strong> ${regimen}</div>
        </div>

        <div class="poison-steps-list" aria-label="Emesis Gate, Phytonadione Vitamin K1 / 20% ILE Lipid Sink, Dextrose/NAC/SAMe Diuresis &amp; 48h PT Recheck Protocol Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary poison-select-workspace-btn"
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

    const countBadge = document.getElementById('poison-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('poison-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="poison-ops-empty-msg">No toxicology or poison control triage operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const agentFilter = (document.getElementById('filter-agent-type')?.value || '').trim();
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const params = new URLSearchParams();
      if (agentFilter) params.set('agent_type', agentFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/toxicology-poison/operations?${qs}`
        : '/api/v1/toxicology-poison/operations';

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
    const res = await fetch('/api/v1/toxicology-poison/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      selectedOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated toxicology poison operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error || 'Failed to evaluate toxicology poison triage operation';
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
    setVal('eval-agent-type', preset.agent_type);
    setVal('eval-dose-mgkg', preset.estimated_dose_mg_kg);
    setVal('eval-pt-ratio', preset.prothrombin_time_ratio);
    setVal('eval-glucose-mgdl', preset.blood_glucose_mg_dl);
    setVal('eval-ingestion-mins', preset.time_since_ingestion_mins);
    setCheck('eval-charcoal-avoided', preset.charcoal_avoided_for_xylitol);
    setCheck('eval-k3-avoided', preset.vitamin_k3_menadione_avoided);
  }

  function setupFormAndFilters() {
    const form = document.getElementById('poison-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petName = (document.getElementById('eval-pet-name')?.value || '').trim();
        const species = (document.getElementById('eval-species')?.value || 'Dog - Mixed Breed').trim();
        const weightKg = parseFloat(document.getElementById('eval-weight-kg')?.value || '0');
        const agentType = (document.getElementById('eval-agent-type')?.value || 'XYLITOL_INSULIN_SURGE_HEPATOTOXIN').trim();
        const doseMgKg = parseFloat(document.getElementById('eval-dose-mgkg')?.value || '0');
        const ptRatio = parseFloat(document.getElementById('eval-pt-ratio')?.value || '0');
        const glucoseMgDl = parseFloat(document.getElementById('eval-glucose-mgdl')?.value || '0');
        const ingestionMins = parseInt(document.getElementById('eval-ingestion-mins')?.value || '0', 10);
        const charcoalAvoided = Boolean(document.getElementById('eval-charcoal-avoided')?.checked);
        const k3Avoided = Boolean(document.getElementById('eval-k3-avoided')?.checked);

        if (!petName) {
          showErrorBanner('pet_name is required');
          return;
        }

        const payload = {
          pet_name: petName,
          species,
          weight_kg: isNaN(weightKg) ? 0 : weightKg,
          agent_type: agentType,
          estimated_dose_mg_kg: isNaN(doseMgKg) ? 0 : doseMgKg,
          prothrombin_time_ratio: isNaN(ptRatio) ? 0 : ptRatio,
          blood_glucose_mg_dl: isNaN(glucoseMgDl) ? 0 : glucoseMgDl,
          time_since_ingestion_mins: isNaN(ingestionMins) ? 0 : ingestionMins,
          charcoal_avoided_for_xylitol: charcoalAvoided,
          vitamin_k3_menadione_avoided: k3Avoided,
        };

        const btn = document.getElementById('btn-submit-poison-eval');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.poison-scenario-btn');
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

    const filterAgent = document.getElementById('filter-agent-type');
    if (filterAgent) {
      filterAgent.addEventListener('change', () => {
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
    const listEl = document.getElementById('poison-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.poison-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/toxicology-poison/operations/${encodeURIComponent(opId)}/steps`,
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
      const workspaceBtn = e.target.closest('.poison-select-workspace-btn');
      if (workspaceBtn) {
        const opId = workspaceBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const clearanceBtn = e.target.closest('.poison-issue-clearance-btn');
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
                `/api/v1/toxicology-poison/operations/${encodeURIComponent(opId)}/steps`,
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
          `/api/v1/toxicology-poison/operations/${encodeURIComponent(opId)}/clearance`,
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
            `Issued toxicology & poison clearance ${updated.clearance_token} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue toxicology clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing toxicology clearance', 'error');
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
