/**
 * PetSpotR Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing
 * & Acute Hemolytic Transfusion Reaction Cockpit Controller (Milestone 19.1)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'TB-OP-101';

  const SCENARIO_PRESETS = {
    'feline-type-b-anaphylaxis': {
      pet_id: 'PET-TB-204',
      pet_name: 'Bramble',
      species: 'Cat - British Shorthair',
      weight_kg: 4.1,
      blood_group_type: 'FELINE_TYPE_B_ALLOANTIBODY_RISK',
      recipient_pcv_pct: 9.5,
      donor_pcv_pct: 42.0,
      target_pcv_pct: 24.0,
      major_minor_crossmatch_compatible: true,
      in_line_170_micron_filter_attached: true,
      lactated_ringers_co_infusion_avoided: true,
    },
    'emergency-canine-feline-xeno': {
      pet_id: 'PET-TB-205',
      pet_name: 'Kira',
      species: 'Cat - Abyssinian',
      weight_kg: 3.4,
      blood_group_type: 'EMERGENCY_CANINE_TO_FELINE_XENOTRANSFUSION',
      recipient_pcv_pct: 8.0,
      donor_pcv_pct: 46.0,
      target_pcv_pct: 22.0,
      major_minor_crossmatch_compatible: true,
      in_line_170_micron_filter_attached: true,
      lactated_ringers_co_infusion_avoided: true,
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
    const liveStatus = document.getElementById('bloodbank-triage-live-status');
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
      case 'ALLOANTIBODY_HEMOLYTIC_ANAPHYLAXIS_CRITICAL':
        return 'badge-bloodbank-critical';
      case 'CROSSMATCHED_MICRON_FILTER_RATE_STAGED':
        return 'badge-bloodbank-staged';
      case 'TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE':
        return 'badge-bloodbank-safe';
      default:
        return 'badge-bloodbank-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-hemolytic');
    const stagedEl = document.getElementById('kpi-staged-crossmatch');
    const safeEl = document.getElementById('kpi-cleared-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'ALLOANTIBODY_HEMOLYTIC_ANAPHYLAXIS_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'CROSSMATCHED_MICRON_FILTER_RATE_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE'
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
        '<p class="trust-empty-hint">No active transfusion bloodbank triage operation selected.</p>';
      return;
    }

    const active =
      ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;

    const opId = escapeHtml(active.id || '');
    const petId = escapeHtml(active.pet_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(active.species || '');
    const weightKg = Number(active.weight_kg ?? 4.0).toFixed(1);
    const bloodGroup = escapeHtml(active.blood_group_type || '');
    const status = escapeHtml(active.status || '');
    const statusClass = getStatusBadgeClass(active.status);
    const warningNote = escapeHtml(active.alloantibody_calcium_warning_note || '');
    const regimen = escapeHtml(active.required_transfusion_regimen || '');
    const clearance = active.clearance_token
      ? `<div class="bloodbank-clearance-banner">✅ TRANSFUSION PCV RESTORATION &amp; NORMOTHERMIC CLEARANCE VERIFIED: ${escapeHtml(active.clearance_token)}</div>`
      : '';

    workspaceEl.innerHTML = `
      <div class="bloodbank-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">Active Workspace: ${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${petId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-bloodbank-group">${bloodGroup}</span>
            <span class="badge badge-bloodbank-status ${statusClass}">${status}</span>
          </div>
        </div>
        <div class="bloodbank-alloantibody-alert-banner">
          ⚠️ ${warningNote}
        </div>
        ${clearance}
        <div class="bloodbank-regimen-recommendations">
          <div><strong>Required Transfusion &amp; Biologic Trial Regimen:</strong> ${regimen}</div>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Feline');
    const weightKg = Number(op.weight_kg ?? 4.0).toFixed(1);
    const bloodGroup = escapeHtml(op.blood_group_type || 'FELINE_TYPE_B_ALLOANTIBODY_RISK');
    const status = escapeHtml(op.status || 'ALLOANTIBODY_HEMOLYTIC_ANAPHYLAXIS_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const recPcv = Number(op.recipient_pcv_pct ?? 10.5).toFixed(1);
    const donorPcv = Number(op.donor_pcv_pct ?? 40.0).toFixed(1);
    const targetPcv = Number(op.target_pcv_pct ?? 24.0).toFixed(1);
    const warningNote = escapeHtml(op.alloantibody_calcium_warning_note || '');
    const regimen = escapeHtml(op.required_transfusion_regimen || '');

    const flags = [];
    if (Number(op.recipient_pcv_pct) < 14.0) {
      flags.push('SEVERE_HYPOXIC_ANEMIA_PCV_CRITICAL');
    }
    if (op.blood_group_type === 'FELINE_TYPE_B_ALLOANTIBODY_RISK') {
      flags.push('NATURAL_ANTI_A_ALLOANTIBODY_HAZARD');
    }
    if (op.blood_group_type === 'EMERGENCY_CANINE_TO_FELINE_XENOTRANSFUSION') {
      flags.push('SINGLE_DOSE_XENOTRANSFUSION_BRIDGE');
    }
    if (!op.lactated_ringers_co_infusion_avoided) {
      flags.push('LACTATED_RINGERS_CPDA1_CLOT_HAZARD');
    }

    const flagsHtml = flags
      .map((f) => `<span class="badge badge-bloodbank-flag">${escapeHtml(f)}</span>`)
      .join('');

    const clearanceBannerHtml = op.clearance_token
      ? `
          <div class="bloodbank-clearance-banner">
            ✅ TRANSFUSION PCV RESTORATION &amp; NORMOTHERMIC CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
          </div>
        `
      : '';

    const isSafe = op.status === 'TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const desc = escapeHtml(step.description || '');
        const badgeClass = done ? 'badge-bloodbank-safe' : 'badge-bloodbank-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="bloodbank-step-item">
            <input
              type="checkbox"
              class="bloodbank-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="bloodbank-step-title">${title}</strong>
            <span class="bloodbank-step-desc">${desc}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary bloodbank-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Transfusion PCV Restoration Clearance (${opId})
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card bloodbank-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-blood-group-type="${bloodGroup}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-bloodbank-group">${bloodGroup}</span>
            <span class="badge badge-bloodbank-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row bloodbank-metrics-row">
          <span class="badge badge-bloodbank-group">Recipient PCV: ${recPcv}% • Donor PCV: ${donorPcv}% • Target PCV: ${targetPcv}%</span>
        </div>

        <div class="bloodbank-flags-row">
          ${flagsHtml}
        </div>

        <div class="bloodbank-alloantibody-alert-banner">
          ⚠️ ${warningNote}
        </div>

        ${clearanceBannerHtml}

        <div class="bloodbank-regimen-recommendations">
          <div><strong>Required Transfusion Regimen:</strong> ${regimen}</div>
        </div>

        <div class="bloodbank-steps-list" aria-label="Blood Typing, 170-Micron Filter 0.9% NaCl Setup, Biologic Trial &amp; Post-PCV Protocol Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary bloodbank-select-workspace-btn"
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

    const countBadge = document.getElementById('bloodbank-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('bloodbank-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="bloodbank-ops-empty-msg">No transfusion or blood typing triage operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const bgFilter = (document.getElementById('filter-blood-group')?.value || '').trim();
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const params = new URLSearchParams();
      if (bgFilter) params.set('blood_group_type', bgFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/transfusion-bloodbank/operations?${qs}`
        : '/api/v1/transfusion-bloodbank/operations';

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
    const res = await fetch('/api/v1/transfusion-bloodbank/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      selectedOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated transfusion operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error || 'Failed to evaluate transfusion bloodbank triage operation';
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
    setVal('eval-blood-group', preset.blood_group_type);
    setVal('eval-recipient-pcv', preset.recipient_pcv_pct);
    setVal('eval-donor-pcv', preset.donor_pcv_pct);
    setVal('eval-target-pcv', preset.target_pcv_pct);
    setCheck('eval-crossmatch-ok', preset.major_minor_crossmatch_compatible);
    setCheck('eval-micron-filter', preset.in_line_170_micron_filter_attached);
    setCheck('eval-lrs-avoided', preset.lactated_ringers_co_infusion_avoided);
  }

  function setupFormAndFilters() {
    const form = document.getElementById('bloodbank-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petName = (document.getElementById('eval-pet-name')?.value || '').trim();
        const species = (document.getElementById('eval-species')?.value || 'Cat - Domestic Shorthair').trim();
        const weightKg = parseFloat(document.getElementById('eval-weight-kg')?.value || '0');
        const bloodGroupType = (document.getElementById('eval-blood-group')?.value || 'FELINE_TYPE_B_ALLOANTIBODY_RISK').trim();
        const recipientPcv = parseFloat(document.getElementById('eval-recipient-pcv')?.value || '0');
        const donorPcv = parseFloat(document.getElementById('eval-donor-pcv')?.value || '0');
        const targetPcv = parseFloat(document.getElementById('eval-target-pcv')?.value || '0');
        const crossmatchOk = Boolean(document.getElementById('eval-crossmatch-ok')?.checked);
        const micronFilter = Boolean(document.getElementById('eval-micron-filter')?.checked);
        const lrsAvoided = Boolean(document.getElementById('eval-lrs-avoided')?.checked);

        if (!petName) {
          showErrorBanner('pet_name is required');
          return;
        }

        const payload = {
          pet_name: petName,
          species,
          weight_kg: isNaN(weightKg) ? 0 : weightKg,
          blood_group_type: bloodGroupType,
          recipient_pcv_pct: isNaN(recipientPcv) ? 0 : recipientPcv,
          donor_pcv_pct: isNaN(donorPcv) ? 0 : donorPcv,
          target_pcv_pct: isNaN(targetPcv) ? 0 : targetPcv,
          major_minor_crossmatch_compatible: crossmatchOk,
          in_line_170_micron_filter_attached: micronFilter,
          lactated_ringers_co_infusion_avoided: lrsAvoided,
        };

        const btn = document.getElementById('btn-submit-bloodbank-eval');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.bloodbank-scenario-btn');
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

    const filterBg = document.getElementById('filter-blood-group');
    if (filterBg) {
      filterBg.addEventListener('change', () => {
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
    const listEl = document.getElementById('bloodbank-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.bloodbank-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/transfusion-bloodbank/operations/${encodeURIComponent(opId)}/steps`,
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
      const workspaceBtn = e.target.closest('.bloodbank-select-workspace-btn');
      if (workspaceBtn) {
        const opId = workspaceBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const clearanceBtn = e.target.closest('.bloodbank-issue-clearance-btn');
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
                `/api/v1/transfusion-bloodbank/operations/${encodeURIComponent(opId)}/steps`,
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
          `/api/v1/transfusion-bloodbank/operations/${encodeURIComponent(opId)}/clearance`,
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
            `Issued transfusion PCV restoration clearance ${updated.clearance_token} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue transfusion clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing transfusion clearance', 'error');
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
