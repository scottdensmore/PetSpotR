/**
 * PetSpotR Canine Parvovirus, Panleukopenia, Distemper & Biosecurity
 * Isolation Cohort Cockpit Controller (Milestone 18.3)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'BI-OP-101';

  const SCENARIO_PRESETS = {
    'parvo-neutropenic-sepsis': {
      pet_id: 'PET-BI-204',
      pet_name: 'Koda',
      species: 'Dog - Doberman Pinscher Puppy',
      weight_kg: 9.8,
      pathogen_type: 'CANINE_PARVOVIRUS_CPV2_ENTERITIS',
      neutrophil_count_cells_ul: 540,
      room_pressure_differential_pa: -5.5,
      disinfectant_contact_mins: 10,
      hemorrhagic_diarrhea_present: true,
      potassium_peroxymonosulfate_footbath: true,
      dedicated_ppe_gown_gloves_enforced: true,
    },
    'distemper-myoclonus-cohort': {
      pet_id: 'PET-BI-205',
      pet_name: 'Willow',
      species: 'Dog - Siberian Husky Juvenile',
      weight_kg: 15.6,
      pathogen_type: 'CANINE_DISTEMPER_CDV_NEURO',
      neutrophil_count_cells_ul: 2100,
      room_pressure_differential_pa: -7.0,
      disinfectant_contact_mins: 12,
      hemorrhagic_diarrhea_present: false,
      potassium_peroxymonosulfate_footbath: true,
      dedicated_ppe_gown_gloves_enforced: true,
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
    const liveStatus = document.getElementById('biosec-triage-live-status');
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
      case 'NEUTROPENIC_SEPSIS_BREACH_CRITICAL':
        return 'badge-biosec-critical';
      case 'NEGATIVE_PRESSURE_ISO_COHORT_STAGED':
        return 'badge-biosec-staged';
      case 'PCR_NEGATIVE_DECON_CLEARED':
        return 'badge-biosec-safe';
      default:
        return 'badge-biosec-critical';
    }
  }

  function updateSummaryStats(ops) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-sepsis');
    const stagedEl = document.getElementById('kpi-staged-iso');
    const safeEl = document.getElementById('kpi-cleared-safe');

    const total = ops.length;
    const critical = ops.filter(
      (op) => op.status === 'NEUTROPENIC_SEPSIS_BREACH_CRITICAL'
    ).length;
    const staged = ops.filter(
      (op) => op.status === 'NEGATIVE_PRESSURE_ISO_COHORT_STAGED'
    ).length;
    const safe = ops.filter(
      (op) => op.status === 'PCR_NEGATIVE_DECON_CLEARED'
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
        '<p class="trust-empty-hint">No active biosecurity isolation operation selected.</p>';
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
    const pathogen = escapeHtml(active.pathogen_type || '');
    const status = escapeHtml(active.status || '');
    const statusClass = getStatusBadgeClass(active.status);
    const quatNote = escapeHtml(active.quat_disinfectant_failure_note || '');
    const regimen = escapeHtml(active.required_isolation_regimen || '');
    const clearance = active.clearance_token
      ? `<div class="biosec-clearance-banner">✅ PCR-NEGATIVE SHEDDING &amp; VIRKON DECON CLEARANCE VERIFIED: ${escapeHtml(active.clearance_token)}</div>`
      : '';

    workspaceEl.innerHTML = `
      <div class="biosec-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">Active Workspace: ${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${petId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-biosec-pathogen">${pathogen}</span>
            <span class="badge badge-biosec-status ${statusClass}">${status}</span>
          </div>
        </div>
        <div class="biosec-quat-alert-banner">
          ☣️ ${quatNote}
        </div>
        ${clearance}
        <div class="biosec-regimen-recommendations">
          <div><strong>Required Biosecurity Isolation Regimen:</strong> ${regimen}</div>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const weightKg = Number(op.weight_kg ?? 10.0).toFixed(1);
    const pathogenType = escapeHtml(op.pathogen_type || 'CANINE_PARVOVIRUS_CPV2_ENTERITIS');
    const status = escapeHtml(op.status || 'NEUTROPENIC_SEPSIS_BREACH_CRITICAL');
    const statusClass = getStatusBadgeClass(op.status);
    const neutrophils = Number(op.neutrophil_count_cells_ul ?? 1000);
    const pressurePa = Number(op.room_pressure_differential_pa ?? -5.0).toFixed(1);
    const contactMins = Number(op.disinfectant_contact_mins ?? 10);
    const quatNote = escapeHtml(op.quat_disinfectant_failure_note || '');
    const regimen = escapeHtml(op.required_isolation_regimen || '');

    const flags = [];
    if (neutrophils < 1500) {
      flags.push('SEVERE_NEUTROPENIC_LEUKOPENIA');
    }
    if (op.hemorrhagic_diarrhea_present) {
      flags.push('HEMORRHAGIC_ENTERITIS_ENDOTOXEMIA');
    }
    if (Number(op.room_pressure_differential_pa) > -2.5) {
      flags.push('NEGATIVE_PRESSURE_BREACH_HAZARD');
    }
    if (contactMins < 10 || !op.potassium_peroxymonosulfate_footbath || !op.dedicated_ppe_gown_gloves_enforced) {
      flags.push('BIOSECURITY_FOMITE_CONTAINMENT_ALERT');
    }

    const flagsHtml = flags
      .map((f) => `<span class="badge badge-biosec-flag">${escapeHtml(f)}</span>`)
      .join('');

    const clearanceBannerHtml = op.clearance_token
      ? `
          <div class="biosec-clearance-banner">
            ✅ PCR-NEGATIVE SHEDDING &amp; VIRKON DECON CLEARANCE VERIFIED: ${escapeHtml(op.clearance_token)}
          </div>
        `
      : '';

    const isSafe = op.status === 'PCR_NEGATIVE_DECON_CLEARED';
    const steps = Array.isArray(op.protocol_steps) ? op.protocol_steps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const stepId = escapeHtml(step.id || '');
        const title = escapeHtml(step.title || '');
        const desc = escapeHtml(step.description || '');
        const badgeClass = done ? 'badge-biosec-safe' : 'badge-biosec-critical';
        const badgeText = done ? 'DONE' : 'REQ';
        return `
          <label class="biosec-step-item">
            <input
              type="checkbox"
              class="biosec-step-checkbox"
              data-op-id="${opId}"
              data-step-id="${stepId}"
              ${done ? 'checked' : ''}
              ${isSafe ? 'disabled' : ''}
            >
            <span class="badge ${badgeClass}">${badgeText}</span>
            <strong class="biosec-step-title">${title}</strong>
            <span class="biosec-step-desc">${desc}</span>
          </label>
        `;
      })
      .join('');

    const actionsHtml = !isSafe
      ? `
          <button
            type="button"
            class="btn btn-sm btn-primary biosec-issue-clearance-btn"
            data-op-id="${opId}"
          >
            🛡️ Issue PCR-Negative Biosecurity Clearance (${opId})
          </button>
        `
      : '';

    return `
      <article class="trust-screening-card biosec-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-pathogen-type="${pathogenType}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-biosec-pathogen">${pathogenType}</span>
            <span class="badge badge-biosec-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row biosec-metrics-row">
          <span class="badge badge-biosec-pathogen">Neutrophils: ${neutrophils} cells/uL • Room Pressure: ${pressurePa} Pa • Decon Contact: ${contactMins} min</span>
        </div>

        <div class="biosec-flags-row">
          ${flagsHtml}
        </div>

        <div class="biosec-quat-alert-banner">
          ⚠️ ${quatNote}
        </div>

        ${clearanceBannerHtml}

        <div class="biosec-regimen-recommendations">
          <div><strong>Required Isolation Regimen:</strong> ${regimen}</div>
        </div>

        <div class="biosec-steps-list" aria-label="Biosecurity Isolation &amp; Neutropenic Sepsis Protocol Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary biosec-select-workspace-btn"
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

    const countBadge = document.getElementById('biosec-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('biosec-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="biosec-ops-empty-msg">No biosecurity isolation cohort triage operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const pathogenFilter = (document.getElementById('filter-pathogen-type')?.value || '').trim();
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const params = new URLSearchParams();
      if (pathogenFilter) params.set('pathogen_type', pathogenFilter);
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/v1/biosecurity-isolation/operations?${qs}`
        : '/api/v1/biosecurity-isolation/operations';

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
    const res = await fetch('/api/v1/biosecurity-isolation/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      selectedOpId = created.id;
      await fetchOperations();
      showToast(
        `Evaluated biosecurity isolation operation ${created.id} for ${created.pet_name} → ${created.status}`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    const msg = errData.error || 'Failed to evaluate biosecurity isolation operation';
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
    setVal('eval-pathogen-type', preset.pathogen_type);
    setVal('eval-neutrophils', preset.neutrophil_count_cells_ul);
    setVal('eval-pressure-pa', preset.room_pressure_differential_pa);
    setVal('eval-contact-mins', preset.disinfectant_contact_mins);
    setCheck('eval-hemorrhagic', preset.hemorrhagic_diarrhea_present);
    setCheck('eval-footbath', preset.potassium_peroxymonosulfate_footbath);
    setCheck('eval-ppe-enforced', preset.dedicated_ppe_gown_gloves_enforced);
  }

  function setupFormAndFilters() {
    const form = document.getElementById('biosec-eval-form');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petName = (document.getElementById('eval-pet-name')?.value || '').trim();
        const species = (document.getElementById('eval-species')?.value || 'Dog - Mixed Breed').trim();
        const weightKg = parseFloat(document.getElementById('eval-weight-kg')?.value || '0');
        const pathogenType = (document.getElementById('eval-pathogen-type')?.value || 'CANINE_PARVOVIRUS_CPV2_ENTERITIS').trim();
        const neutrophils = parseInt(document.getElementById('eval-neutrophils')?.value || '0', 10);
        const pressurePa = parseFloat(document.getElementById('eval-pressure-pa')?.value || '0');
        const contactMins = parseInt(document.getElementById('eval-contact-mins')?.value || '0', 10);
        const hemorrhagic = Boolean(document.getElementById('eval-hemorrhagic')?.checked);
        const footbath = Boolean(document.getElementById('eval-footbath')?.checked);
        const ppeEnforced = Boolean(document.getElementById('eval-ppe-enforced')?.checked);

        if (!petName) {
          showErrorBanner('pet_name is required');
          return;
        }

        const payload = {
          pet_name: petName,
          species,
          weight_kg: isNaN(weightKg) ? 0 : weightKg,
          pathogen_type: pathogenType,
          neutrophil_count_cells_ul: isNaN(neutrophils) ? 0 : neutrophils,
          room_pressure_differential_pa: isNaN(pressurePa) ? 0 : pressurePa,
          disinfectant_contact_mins: isNaN(contactMins) ? 0 : contactMins,
          hemorrhagic_diarrhea_present: hemorrhagic,
          potassium_peroxymonosulfate_footbath: footbath,
          dedicated_ppe_gown_gloves_enforced: ppeEnforced,
        };

        const btn = document.getElementById('btn-submit-biosec-eval');
        if (btn) btn.disabled = true;
        try {
          await evaluateOperation(payload);
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const scenarioBtns = document.querySelectorAll('.biosec-scenario-btn');
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

    const filterPathogen = document.getElementById('filter-pathogen-type');
    if (filterPathogen) {
      filterPathogen.addEventListener('change', () => {
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
    const listEl = document.getElementById('biosec-operations-list');
    if (!listEl) return;

    listEl.addEventListener('change', async (e) => {
      const checkbox = e.target.closest('.biosec-step-checkbox');
      if (!checkbox) return;

      const opId = checkbox.getAttribute('data-op-id');
      const stepId = checkbox.getAttribute('data-step-id');
      if (!opId || !stepId) return;

      const completed = Boolean(checkbox.checked);
      checkbox.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/biosecurity-isolation/operations/${encodeURIComponent(opId)}/steps`,
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
      const workspaceBtn = e.target.closest('.biosec-select-workspace-btn');
      if (workspaceBtn) {
        const opId = workspaceBtn.getAttribute('data-op-id');
        if (opId) {
          selectedOpId = opId;
          renderActiveWorkspace(currentOps);
        }
        return;
      }

      const clearanceBtn = e.target.closest('.biosec-issue-clearance-btn');
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
                `/api/v1/biosecurity-isolation/operations/${encodeURIComponent(opId)}/steps`,
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
          `/api/v1/biosecurity-isolation/operations/${encodeURIComponent(opId)}/clearance`,
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
            `Issued PCR-negative biosecurity clearance ${updated.clearance_token} for ${opId}`,
            'success'
          );
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to issue biosecurity clearance', 'error');
        }
      } catch (_err) {
        showToast('Network error issuing biosecurity clearance', 'error');
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
