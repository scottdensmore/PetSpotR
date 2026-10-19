/**
 * PetSpotR Pit-Viper Envenomation, Bufo Toad Bufotoxin, Porcupine Quill & Toxic Flora Triage
 * Cockpit Controller (Milestone 17.4)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'ET-OP-101';

  const SCENARIO_PRESETS = {
    'rattlesnake-hemotoxic': {
      pet_id: 'PET-ET-204',
      pet_name: 'Sidewinder',
      species: 'Dog - Catahoula Leopard Dog',
      weight_kg: 24.8,
      toxin_type: 'PIT_VIPER_CROTALID_HEI',
      edema_spread_cm_hr: 7.2,
      echinocyte_pct: 42.0,
      quill_count_muzzle_thorax: 0,
      oral_mucosa_flushed_forward: true,
      quills_cut_or_pulled_awake_avoided: true,
      antivenom_vials_staged: 3,
    },
    'porcupine-muzzle-quills': {
      pet_id: 'PET-ET-205',
      pet_name: 'Thistle',
      species: 'Dog - Labrador Retriever',
      weight_kg: 29.5,
      toxin_type: 'PORCUPINE_RETROGRADE_QUILLS',
      edema_spread_cm_hr: 1.5,
      echinocyte_pct: 0.0,
      quill_count_muzzle_thorax: 26,
      oral_mucosa_flushed_forward: true,
      quills_cut_or_pulled_awake_avoided: true,
      antivenom_vials_staged: 0,
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
    const liveStatus = document.getElementById('toxin-triage-live-status');
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
      case 'COAGULOPATHY_NEUROTOXIC_CRITICAL':
        return 'badge-toxin-critical';
      case 'ANTIVENOM_LAVAGE_SEDATION_STAGED':
        return 'badge-toxin-staged';
      case 'STABILIZED_NEUTRALIZED_CLEARED':
        return 'badge-toxin-safe';
      default:
        return 'badge-toxin-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-toxin');
    const stagedEl = document.getElementById('kpi-staged-antivenom');
    const safeEl = document.getElementById('kpi-stabilized-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'COAGULOPATHY_NEUROTOXIC_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'ANTIVENOM_LAVAGE_SEDATION_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'STABILIZED_NEUTRALIZED_CLEARED'
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
        '<p class="trust-empty-hint">No active envenomation or toxin triage operation selected.</p>';
      return;
    }

    const active =
      ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;

    const opId = escapeHtml(active.id || '');
    const petId = escapeHtml(active.pet_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(active.species || '');
    const weightKg = Number(active.weight_kg ?? 20.0).toFixed(1);
    const toxinType = escapeHtml(active.toxin_type || '');
    const status = escapeHtml(active.status || '');
    const statusClass = getStatusBadgeClass(active.status);
    const warningNote = escapeHtml(active.contraindication_warning_note || '');
    const antidoteProto = escapeHtml(active.required_antidote_protocol || '');
    const clearance = active.clearance_token
      ? `<div class="toxin-clearance-banner">✅ ENVENOMATION &amp; TOXIN NEUTRALIZATION CLEARANCE VERIFIED: ${escapeHtml(active.clearance_token)}</div>`
      : '';

    workspaceEl.innerHTML = `
      <div class="toxin-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">Active Workspace: ${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${petId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-toxin-type">${toxinType}</span>
            <span class="badge badge-toxin-status ${statusClass}">${status}</span>
          </div>
        </div>
        <div class="toxin-contraindication-banner">
          ⚠️ ${warningNote}
        </div>
        ${clearance}
        <div class="toxin-gear-recommendations">
          <div><strong>Required Antidote &amp; Extraction Protocol:</strong> ${antidoteProto}</div>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const weightKg = Number(op.weight_kg ?? 20.0).toFixed(1);
    const toxinType = escapeHtml(op.toxin_type || 'PIT_VIPER_CROTALID_HEI');
    const status = escapeHtml(op.status || 'COAGULOPATHY_NEUROTOXIC_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const edemaSpread = Number(op.edema_spread_cm_hr ?? 0.0).toFixed(1);
    const echinocytePct = Number(op.echinocyte_pct ?? 0.0).toFixed(1);
    const quillCount = Number(op.quill_count_muzzle_thorax ?? 0);
    const antivenomVials = Number(op.antivenom_vials_staged ?? 0);
    const warningNote = escapeHtml(op.contraindication_warning_note || '');
    const antidoteProtocol = escapeHtml(op.required_antidote_protocol || '');

    const flags = [];
    if (Number(op.edema_spread_cm_hr) >= 4.0) {
      flags.push('RAPID_HEMORRHAGIC_EDEMA_PROGRESSION');
    }
    if (Number(op.echinocyte_pct) >= 25.0) {
      flags.push('SEVERE_ECHINOCYTOSIS_COAGULOPATHY');
    }
    if (quillCount >= 10) {
      flags.push(`RETROGRADE_QUILL_MIGRATION_RISK_${quillCount}`);
    }
    if (!op.quills_cut_or_pulled_awake_avoided) {
      flags.push('SPLINTERED_CUT_QUILL_MIGRATION_HAZARD');
    }
    if (op.toxin_type === 'BUFO_MARINUS_CARDIOTOXIN' && !op.oral_mucosa_flushed_forward) {
      flags.push('CAUDAL_PHARYNGEAL_BUFOTOXIN_ASPIRATION_HAZARD');
    }

    const flagsHtml = flags
      .map((f) => `<span class="badge badge-toxin-flag">${escapeHtml(f)}</span>`)
      .join('');

    const clearanceBannerHtml = op.clearance_token
      ? `
          <div class="toxin-clearance-banner">
            ✅ ENVENOMATION &amp; TOXIN NEUTRALIZATION CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
          </div>
        `
      : '';

    const isSafe = op.status === 'STABILIZED_NEUTRALIZED_CLEARED';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const desc = escapeHtml(step.description || '');
        const badgeClass = done ? 'badge-toxin-safe' : 'badge-toxin-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="toxin-step-item">
            <input
              type="checkbox"
              class="toxin-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="toxin-step-title">${title}</strong>
            <span class="toxin-step-desc">${desc}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary toxin-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Toxin Neutralization Clearance (${opId})
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card toxin-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-toxin-type="${toxinType}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-toxin-type">${toxinType}</span>
            <span class="badge badge-toxin-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row toxin-metrics-row">
          <span class="badge badge-toxin-type">Edema Spread: ${edemaSpread} cm/hr • Echinocytes: ${echinocytePct}% • Quills: ${quillCount} • Antivenom Vials: ${antivenomVials}</span>
        </div>

        <div class="toxin-flags-row">
          ${flagsHtml}
        </div>

        <div class="toxin-contraindication-banner">
          ⚠️ ${warningNote}
        </div>

        ${clearanceBannerHtml}

        <div class="toxin-gear-recommendations">
          <div><strong>Required Antidote Protocol:</strong> ${antidoteProtocol}</div>
        </div>

        <div class="toxin-steps-list" aria-label="Envenomation, Oral Lavage &amp; Sedated Quill Extraction Protocol Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary toxin-select-workspace-btn"
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

    const countBadge = document.getElementById('toxin-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('toxin-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="toxin-ops-empty-msg">No envenomation or toxin triage operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const toxinFilter = (document.getElementById('filter-toxin-type')?.value || '').trim();
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const params = new URLSearchParams();
      if (toxinFilter) params.set('toxin_type', toxinFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/envenomation-toxin/operations?${qs}`
        : '/api/v1/envenomation-toxin/operations';

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
    const res = await fetch('/api/v1/envenomation-toxin/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      selectedOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated envenomation operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error || 'Failed to evaluate envenomation toxin triage operation';
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
    setVal('eval-toxin-type', preset.toxin_type);
    setVal('eval-edema-spread', preset.edema_spread_cm_hr);
    setVal('eval-echinocyte-pct', preset.echinocyte_pct);
    setVal('eval-quill-count', preset.quill_count_muzzle_thorax);
    setVal('eval-antivenom-vials', preset.antivenom_vials_staged);
    setCheck('eval-oral-flushed', preset.oral_mucosa_flushed_forward);
    setCheck('eval-quills-cut-avoided', preset.quills_cut_or_pulled_awake_avoided);
  }

  function setupFormAndFilters() {
    const form = document.getElementById('toxin-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petName = (document.getElementById('eval-pet-name')?.value || '').trim();
        const species = (document.getElementById('eval-species')?.value || 'Dog - Mixed Breed').trim();
        const weightKg = parseFloat(document.getElementById('eval-weight-kg')?.value || '0');
        const toxinType = (document.getElementById('eval-toxin-type')?.value || 'PIT_VIPER_CROTALID_HEI').trim();
        const edemaSpread = parseFloat(document.getElementById('eval-edema-spread')?.value || '0');
        const echinocytePct = parseFloat(document.getElementById('eval-echinocyte-pct')?.value || '0');
        const quillCount = parseInt(document.getElementById('eval-quill-count')?.value || '0', 10);
        const antivenomVials = parseInt(document.getElementById('eval-antivenom-vials')?.value || '0', 10);
        const oralFlushed = Boolean(document.getElementById('eval-oral-flushed')?.checked);
        const quillsCutAvoided = Boolean(document.getElementById('eval-quills-cut-avoided')?.checked);

        if (!petName) {
          showErrorBanner('pet_name is required');
          return;
        }

        const payload = {
          pet_name: petName,
          species,
          weight_kg: isNaN(weightKg) ? 0 : weightKg,
          toxin_type: toxinType,
          edema_spread_cm_hr: isNaN(edemaSpread) ? 0 : edemaSpread,
          echinocyte_pct: isNaN(echinocytePct) ? 0 : echinocytePct,
          quill_count_muzzle_thorax: isNaN(quillCount) ? 0 : quillCount,
          antivenom_vials_staged: isNaN(antivenomVials) ? 0 : antivenomVials,
          oral_mucosa_flushed_forward: oralFlushed,
          quills_cut_or_pulled_awake_avoided: quillsCutAvoided,
        };

        const btn = document.getElementById('btn-submit-toxin-eval');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.toxin-scenario-btn');
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

    const filterToxin = document.getElementById('filter-toxin-type');
    if (filterToxin) {
      filterToxin.addEventListener('change', () => {
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
    const listEl = document.getElementById('toxin-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.toxin-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/envenomation-toxin/operations/${encodeURIComponent(opId)}/steps`,
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
      const workspaceBtn = e.target.closest('.toxin-select-workspace-btn');
      if (workspaceBtn) {
        const opId = workspaceBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const clearanceBtn = e.target.closest('.toxin-issue-clearance-btn');
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
                `/api/v1/envenomation-toxin/operations/${encodeURIComponent(opId)}/steps`,
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
          `/api/v1/envenomation-toxin/operations/${encodeURIComponent(opId)}/clearance`,
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
            `Issued toxin neutralization clearance ${updated.clearance_token} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue toxin clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing toxin clearance', 'error');
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
