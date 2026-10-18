/**
 * PetSpotR Neonatal Orphan Litter & Incubator Critical Care Triage Cockpit Controller (Milestone 17.3)
 */

(function () {
  'use strict';

  let currentOps = [];
  let activeOpId = 'NI-OP-101';

  const SCENARIO_PRESETS = {
    'fading-kitten-triad': {
      pet_name: 'Thistle Litter',
      species: 'Cat - Domestic Shorthair Neonates',
      weight_grams: 86.0,
      litter_count: 4,
      age_days: 3,
      litter_type: 'UNWEANED_KITTEN_LITTER_0_2WK',
      neonate_rectal_temp_celsius: 33.4,
      blood_glucose_mg_dl: 41.0,
      incubator_humidity_pct: 64.0,
      suckling_reflex_present: false,
      warmed_before_feeding_gate: false,
      sublingual_dextrose_applied: true,
    },
    'cleft-palate-gavage': {
      pet_name: 'Barnaby Litter',
      species: 'Dog - English Bulldog Neonates',
      weight_grams: 215.0,
      litter_count: 3,
      age_days: 4,
      litter_type: 'NEONATAL_PUPPY_LITTER_CLEFT',
      neonate_rectal_temp_celsius: 36.6,
      blood_glucose_mg_dl: 84.0,
      incubator_humidity_pct: 60.0,
      suckling_reflex_present: false,
      warmed_before_feeding_gate: true,
      sublingual_dextrose_applied: true,
    },
    'singleton-colostrum-orphan': {
      pet_name: 'Waffles',
      species: 'Dog - Cavalier King Charles Neonate',
      weight_grams: 172.0,
      litter_count: 1,
      age_days: 7,
      litter_type: 'SINGLETON_ORPHAN_COLOSTRUM_DEPRIVED',
      neonate_rectal_temp_celsius: 36.9,
      blood_glucose_mg_dl: 92.0,
      incubator_humidity_pct: 58.0,
      suckling_reflex_present: true,
      warmed_before_feeding_gate: true,
      sublingual_dextrose_applied: true,
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
    const liveStatus = document.getElementById('neonatal-triage-live-status');
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
      case 'FADING_TRIAD_HYPOTHERMIC_CRITICAL':
        return 'badge-ni-critical';
      case 'INCUBATOR_WARMED_DEXTROSE_STAGED':
        return 'badge-ni-staged';
      case 'STABILIZED_NURSING_GAINING_SAFE':
        return 'badge-ni-safe';
      default:
        return 'badge-ni-critical';
    }
  }

  function updateSummaryKpis(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-fading');
    const stagedEl = document.getElementById('kpi-staged-incubator');
    const safeEl = document.getElementById('kpi-stabilized-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'FADING_TRIAD_HYPOTHERMIC_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'INCUBATOR_WARMED_DEXTROSE_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'STABILIZED_NURSING_GAINING_SAFE'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (safeEl) safeEl.textContent = String(safe);
  }

  function renderWorkspaceHtml(op) {
    const workspaceEl = document.getElementById('active-operation-workspace');
    if (!workspaceEl) return;

    if (!op) {
      workspaceEl.innerHTML = `
        <div class="trust-panel-header">
          <h2 id="heading-active-workspace">Active Incubator Critical Care Workspace</h2>
        </div>
        <p class="trust-empty-hint">Select or evaluate a neonatal litter operation to open the active workspace.</p>
      `;
      return;
    }

    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Cat - Domestic Shorthair Neonate');
    const weightGrams = Number(op.weight_grams ?? 100).toFixed(1);
    const litterCount = Number(op.litter_count ?? 1);
    const ageDays = Number(op.age_days ?? 1);
    const litterType = escapeHtml(op.litter_type || 'UNWEANED_KITTEN_LITTER_0_2WK');
    const status = escapeHtml(op.status || 'FADING_TRIAD_HYPOTHERMIC_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const rectalTemp = Number(op.neonate_rectal_temp_celsius ?? 35.0).toFixed(1);
    const glucose = Number(op.blood_glucose_mg_dl ?? 60.0).toFixed(0);
    const humidity = Number(op.incubator_humidity_pct ?? 60.0).toFixed(0);
    const sucklingText = op.suckling_reflex_present ? 'Present' : 'Absent';
    const ileusNote = escapeHtml(op.ileus_warming_gate_note || '');
    const protocol = escapeHtml(op.required_incubator_protocol || '');

    const clearanceHtml = op.clearance_token
      ? `
        <div class="ni-clearance-banner" id="workspace-clearance-banner">
          ✅ NEONATAL NURSING &amp; WEIGHT-GAIN CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
        </div>
      `
      : '';

    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const desc = escapeHtml(step.description || '');
        const badgeClass = done ? 'badge-ni-safe' : 'badge-ni-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="ni-step-item">
            <input
              type="checkbox"
              class="ni-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="ni-step-id">[${stepId}]</strong>
            <span class="ni-step-title">${title}</span>
            <span class="ni-step-desc">— ${desc}</span>
          </label>
        `;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="trust-panel-header">
        <h2 id="heading-active-workspace">Active Incubator Critical Care Workspace</h2>
        <code class="trust-pet-id" id="workspace-op-id">${opId}</code>
      </div>
      <div class="ni-workspace-body" data-workspace-op-id="${opId}" data-status="${status}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name" id="workspace-pet-title">${petName} (${species} · ${weightGrams} g · Litter of ${litterCount} · ${ageDays}d)</strong>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-ni-litter">${litterType}</span>
            <span class="badge badge-ni-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="ni-ileus-gate-banner">
          ${ileusNote}
        </div>

        <div class="ni-protocol-box">
          <div><strong>Required Incubator &amp; Enteral Protocol:</strong> ${protocol}</div>
          <div><strong>Telemetry Vitals:</strong> Rectal Temp ${rectalTemp}°C • Blood Glucose ${glucose} mg/dL • Humidity ${humidity}% • Suckling Reflex: ${sucklingText}</div>
        </div>

        ${clearanceHtml}

        <div class="ni-steps-list" aria-label="Workspace Neonatal Incubator Resuscitation Checklist">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-primary ni-issue-clearance-btn"
            id="workspace-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Neonatal Stabilized Nursing Clearance Token
          </button>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Cat - Domestic Shorthair Neonate');
    const weightGrams = Number(op.weight_grams ?? 100).toFixed(1);
    const litterCount = Number(op.litter_count ?? 1);
    const ageDays = Number(op.age_days ?? 1);
    const litterType = escapeHtml(op.litter_type || 'UNWEANED_KITTEN_LITTER_0_2WK');
    const status = escapeHtml(op.status || 'FADING_TRIAD_HYPOTHERMIC_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const rectalTemp = Number(op.neonate_rectal_temp_celsius ?? 35.0).toFixed(1);
    const glucose = Number(op.blood_glucose_mg_dl ?? 60.0).toFixed(0);
    const humidity = Number(op.incubator_humidity_pct ?? 60.0).toFixed(0);
    const sucklingText = op.suckling_reflex_present ? 'Yes' : 'No';
    const warmedGateText = op.warmed_before_feeding_gate ? 'Cleared' : 'Blocked';
    const ileusNote = escapeHtml(op.ileus_warming_gate_note || '');
    const protocol = escapeHtml(op.required_incubator_protocol || '');

    const clearanceBannerHtml = op.clearance_token
      ? `
          <div class="ni-clearance-banner">
            ✅ NEONATAL NURSING &amp; WEIGHT-GAIN CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
          </div>
        `
      : '';

    const isSafe = op.status === 'STABILIZED_NURSING_GAINING_SAFE';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const badgeClass = done ? 'badge-ni-safe' : 'badge-ni-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="ni-step-item">
            <input
              type="checkbox"
              class="ni-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="ni-step-id">[${stepId}]</strong>
            <span class="ni-step-title">${title}</span>
          </label>
        `;
      })
      .join('');

    const clearanceBtnHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary ni-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue Neonatal Nursing Clearance
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card neonatal-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-litter-type="${litterType}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightGrams} g · Litter of ${litterCount} · ${ageDays}d)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-ni-litter">${litterType}</span>
            <span class="badge badge-ni-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row ni-metrics-row">
          <span class="badge badge-ni-litter">Rectal Temp: ${rectalTemp}°C • Glucose: ${glucose} mg/dL • Humidity: ${humidity}% • Suckling: ${sucklingText} • Warmed Gate: ${warmedGateText}</span>
        </div>

        <div class="ni-ileus-gate-banner">
          ${ileusNote}
        </div>

        ${clearanceBannerHtml}

        <div class="ni-protocol-box">
          <div><strong>Required Incubator Protocol:</strong> ${protocol}</div>
        </div>

        <div class="ni-steps-list" aria-label="Neonatal Incubator &amp; Gavage Protocol Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary ni-select-workspace-btn"
            data-op-id="${opId}"
          >
            🔍 Open in Workspace
          </button>
          ${clearanceBtnHtml}
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops) {
    currentOps = ops;
    updateSummaryKpis(ops);

    const countBadge = document.getElementById('ni-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('neonatal-operations-list');
    if (listEl) {
      if (ops.length === 0) {
        listEl.innerHTML =
          '<p class="trust-empty-hint" id="ni-ops-empty-msg">No neonatal incubator triage operations match the current filter.</p>';
      } else {
        listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
      }
    }

    const activeOp =
      ops.find((item) => item.id === activeOpId) ||
      (ops.length > 0 ? ops[0] : null);
    if (activeOp) {
      activeOpId = activeOp.id;
    }
    renderWorkspaceHtml(activeOp);
  }

  async function fetchOperations() {
    try {
      const litterFilter =
        document.getElementById('filter-litter-type')?.value || '';
      const statusFilter =
        document.getElementById('filter-status')?.value || '';
      const params = new URLSearchParams();
      if (litterFilter) params.set('litter_type', litterFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/neonatal-triage/operations?${qs}`
        : '/api/v1/neonatal-triage/operations';

      const res = await fetch(url, {
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
    showErrorBanner('');
    const res = await fetch('/api/v1/neonatal-triage/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      activeOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated neonatal operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error || 'Failed to evaluate neonatal triage operation';
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
    setVal('eval-weight-grams', preset.weight_grams);
    setVal('eval-litter-count', preset.litter_count);
    setVal('eval-age-days', preset.age_days);
    setVal('eval-litter-type', preset.litter_type);
    setVal('eval-rectal-temp-c', preset.neonate_rectal_temp_celsius);
    setVal('eval-glucose-mgdl', preset.blood_glucose_mg_dl);
    setVal('eval-humidity-pct', preset.incubator_humidity_pct);
    setCheck('eval-suckling-reflex', preset.suckling_reflex_present);
    setCheck('eval-warmed-gate', preset.warmed_before_feeding_gate);
    setCheck('eval-dextrose-applied', preset.sublingual_dextrose_applied);
  }

  function setupFiltersAndForm() {
    const litterFilterEl = document.getElementById('filter-litter-type');
    const statusFilterEl = document.getElementById('filter-status');
    if (litterFilterEl) {
      litterFilterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }
    if (statusFilterEl) {
      statusFilterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const form = document.getElementById('neonatal-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const payload = {
          pet_name: (document.getElementById('eval-pet-name')?.value || '').trim(),
          species: (document.getElementById('eval-species')?.value || '').trim(),
          weight_grams: parseFloat(
            document.getElementById('eval-weight-grams')?.value || '0'
          ),
          litter_count: parseInt(
            document.getElementById('eval-litter-count')?.value || '0',
            10
          ),
          age_days: parseInt(
            document.getElementById('eval-age-days')?.value || '0',
            10
          ),
          litter_type: (
            document.getElementById('eval-litter-type')?.value || ''
          ).trim(),
          neonate_rectal_temp_celsius: parseFloat(
            document.getElementById('eval-rectal-temp-c')?.value || '0'
          ),
          blood_glucose_mg_dl: parseFloat(
            document.getElementById('eval-glucose-mgdl')?.value || '0'
          ),
          incubator_humidity_pct: parseFloat(
            document.getElementById('eval-humidity-pct')?.value || '0'
          ),
          suckling_reflex_present: Boolean(
            document.getElementById('eval-suckling-reflex')?.checked
          ),
          warmed_before_feeding_gate: Boolean(
            document.getElementById('eval-warmed-gate')?.checked
          ),
          sublingual_dextrose_applied: Boolean(
            document.getElementById('eval-dextrose-applied')?.checked
          ),
        };

        const btn = document.getElementById('btn-evaluate-neonatal');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.ni-scenario-btn');
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

  async function handleStepChange(checkbox) {
    const opId = checkbox.getAttribute('data-op-id');
    const stepId = checkbox.getAttribute('data-step-id');
    if (!opId || !stepId) return;

    const completed = Boolean(checkbox.checked);
    checkbox.disabled = true;
    try {
      const res = await fetch(
        `/api/v1/neonatal-triage/operations/${encodeURIComponent(opId)}/steps`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ step_id: stepId, completed }),
        }
      );
      if (res.ok) {
        const updated = await res.json();
        activeOpId = updated.id || opId;
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
  }

  async function handleIssueClearance(clearanceBtn) {
    const opId = clearanceBtn.getAttribute('data-op-id');
    if (!opId) return;

    clearanceBtn.disabled = true;
    try {
      const op = currentOps.find((item) => item.id === opId);
      if (op && Array.isArray(op.protocol_steps)) {
        for (const step of op.protocol_steps) {
          if (!step.completed) {
            await fetch(
              `/api/v1/neonatal-triage/operations/${encodeURIComponent(opId)}/steps`,
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
        `/api/v1/neonatal-triage/operations/${encodeURIComponent(opId)}/clearance`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({}),
        }
      );
      if (res.ok) {
        const updated = await res.json();
        activeOpId = updated.id || opId;
        await fetchOperations();
        showToast(
          `Issued neonatal clearance ${updated.clearance_token} for ${opId}`,
          'success'
        );
      } else {
        const errData = await res.json().catch(() => ({}));
        showToast(errData.error || 'Failed to issue neonatal clearance', 'error');
      }
    } catch (_err) {
      showToast('Network error issuing neonatal clearance', 'error');
    } finally {
      clearanceBtn.disabled = false;
    }
  }

  function setupInteractiveContainers() {
    const mainEl = document.getElementById('neonatal-incubator-main');
    if (!mainEl) return;

    mainEl.addEventListener('change', (e) => {
      const checkbox = e.target.closest('.ni-step-checkbox');
      if (checkbox) {
        handleStepChange(checkbox);
      }
    });

    mainEl.addEventListener('click', (e) => {
      const selectBtn = e.target.closest('.ni-select-workspace-btn');
      if (selectBtn) {
        const opId = selectBtn.getAttribute('data-op-id');
        if (opId) {
          activeOpId = opId;
          const found = currentOps.find((item) => item.id === opId);
          if (found) {
            renderWorkspaceHtml(found);
          }
        }
        return;
      }

      const clearanceBtn = e.target.closest('.ni-issue-clearance-btn');
      if (clearanceBtn) {
        handleIssueClearance(clearanceBtn);
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
    setupFiltersAndForm();
    setupInteractiveContainers();
    fetchOperations();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
