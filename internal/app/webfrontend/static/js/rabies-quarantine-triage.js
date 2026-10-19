/**
 * PetSpotR Acute Rabies Quarantine, Lyssavirus Post-Exposure Prophylaxis (PEP),
 * Strict 10-Day Domestic Bite Observation vs. 45-Day/4-Month Unvaccinated Quarantine,
 * Biosecurity Double-Barrier Isolation & Public Health Rabies Control Officer (RCO)
 * Statutory Reporting Cockpit Controller (Milestone 25.4)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'RBQ-OP-101';
  let eventSource = null;

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
    const liveStatus = document.getElementById('rbq-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('rbq-error-banner');
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
      case 'BIOSECURITY_ISOLATION_RCO_HOLD_STAGED':
        return 'rbq-status-critical';
      case 'SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED':
        return 'rbq-status-staged';
      case 'STATUTORY_QUARANTINE_SURVIVED_CLEARED':
        return 'rbq-status-cleared';
      default:
        return 'rbq-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const stagedEl = document.getElementById('kpi-biosecurity-staged');
    const monitoredEl = document.getElementById('kpi-neuro-monitored');
    const clearedEl = document.getElementById('kpi-quarantine-cleared');
    const tenDayEl = document.getElementById('kpi-ten-day-bite-count');
    const rffitEl = document.getElementById('kpi-protective-rffit-count');

    const total = metrics?.total_operations ?? ops.length;
    const staged =
      metrics?.biosecurity_isolation_rco_hold_staged_count ??
      ops.filter(
        (op) => op.status === 'BIOSECURITY_ISOLATION_RCO_HOLD_STAGED'
      ).length;
    const monitored =
      metrics?.serial_neuro_salivary_monitored_count ??
      ops.filter(
        (op) => op.status === 'SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED'
      ).length;
    const cleared =
      metrics?.statutory_quarantine_survived_cleared_count ??
      ops.filter(
        (op) => op.status === 'STATUTORY_QUARANTINE_SURVIVED_CLEARED'
      ).length;
    const tenDay =
      metrics?.ten_day_domestic_bite_hold_count ??
      ops.filter(
        (op) => op.statutory_category === 'TEN_DAY_DOMESTIC_BITE_CONFINEMENT'
      ).length;
    const protectiveRffit =
      metrics?.protective_rffit_titer_count ??
      ops.filter((op) => Number(op.rffit_titer_iu_ml ?? 0) >= 0.5).length;

    if (totalEl) totalEl.textContent = String(total);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (monitoredEl) monitoredEl.textContent = String(monitored);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (tenDayEl) tenDayEl.textContent = String(tenDay);
    if (rffitEl) rffitEl.textContent = String(protectiveRffit);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const stg = op.quarantine_staging || {};
    const protocolInput = document.getElementById('stage-protocol-input');
    const rcoInput = document.getElementById('stage-rco-filing-input');
    const reqDaysInput = document.getElementById('stage-required-days-input');
    const noVaccineCheckbox = document.getElementById(
      'stage-no-vaccine-hold-checkbox'
    );
    const doubleBarrierCheckbox = document.getElementById(
      'stage-double-barrier-checkbox'
    );
    const ppePlanInput = document.getElementById('stage-ppe-plan-input');
    const lavagePlanInput = document.getElementById('stage-lavage-plan-input');
    const rffitPlanInput = document.getElementById('stage-rffit-plan-input');

    if (protocolInput && (stg.protocol || op.protocol)) {
      protocolInput.value = stg.protocol || op.protocol;
    }
    if (rcoInput && (stg.rco_case_filing_number || op.rco_case_filing_number)) {
      rcoInput.value = stg.rco_case_filing_number || op.rco_case_filing_number;
    }
    if (
      reqDaysInput &&
      (stg.required_quarantine_days || op.required_quarantine_days)
    ) {
      reqDaysInput.value = String(
        stg.required_quarantine_days || op.required_quarantine_days
      );
    }
    if (noVaccineCheckbox) {
      noVaccineCheckbox.checked = Boolean(
        op.no_vaccine_during_10day_bite_hold_verified ?? true
      );
    }
    if (doubleBarrierCheckbox) {
      doubleBarrierCheckbox.checked = Boolean(
        op.double_barrier_locked_enclosure_verified ?? true
      );
    }
    if (ppePlanInput && stg.double_barrier_biosecurity_ppe_plan) {
      ppePlanInput.value = stg.double_barrier_biosecurity_ppe_plan;
    }
    if (lavagePlanInput && stg.virucidal_wound_lavage_booster_pep_plan) {
      lavagePlanInput.value = stg.virucidal_wound_lavage_booster_pep_plan;
    }
    if (rffitPlanInput && stg.prospective_serologic_rffit_plan) {
      rffitPlanInput.value = stg.prospective_serologic_rffit_plan;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('rbq-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'STATUTORY_QUARANTINE_SURVIVED_CLEARED'
    ) {
      const by =
        active.discharged_by ||
        active.clearance?.cleared_by ||
        'Rabies Control Officer';
      const summary =
        active.discharge_summary ||
        active.clearance?.clearance_summary ||
        'Statutory quarantine survived, cranial nerves intact, RCO release signed, and exit rabies vaccination verified.';
      if (summaryTextEl) {
        summaryTextEl.textContent = `${active.id} (${active.pet_name}) — Cleared by ${by}: ${summary}`;
      }
      clearanceBanner.hidden = false;
    } else {
      clearanceBanner.hidden = true;
    }
  }

  function renderActiveWorkspace(ops) {
    if (!ops || ops.length === 0) {
      updateClearanceBanner(null);
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;

    syncFormsWithActiveOperation(active);
    updateClearanceBanner(active);

    const titleEl = document.getElementById('active-op-title');
    if (titleEl) {
      const weightStr = Number(active.weight_kg ?? 25.0).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Canine'} · ${active.breed || 'Mixed Breed'} · ${weightStr} kg)`;
    }

    const rcoEl = document.getElementById('active-rco-filing');
    if (rcoEl) {
      rcoEl.textContent = active.rco_case_filing_number || '';
    }
    const catEl = document.getElementById('active-statutory-category');
    if (catEl) {
      catEl.textContent = active.statutory_category || '';
    }
    const guardEl = document.getElementById('active-vaccine-hold-guard');
    if (guardEl) {
      guardEl.textContent = active.no_vaccine_during_10day_bite_hold_verified
        ? 'VERIFIED (HOLD VACCINE UNTIL DAY 10)'
        : 'VIOLATION';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent =
        active.status || 'BIOSECURITY_ISOLATION_RCO_HOLD_STAGED';
      badgeEl.className = `rbq-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const daysEl = document.getElementById('active-quarantine-days');
    if (daysEl) {
      const elapsed = Number(active.quarantine_day_elapsed ?? 0);
      const req = Number(active.required_quarantine_days ?? 10);
      daysEl.textContent = `Day ${elapsed} / ${req} days`;
    }
    const titerEl = document.getElementById('active-rffit-titer');
    if (titerEl) {
      const titer = Number(active.rffit_titer_iu_ml ?? 0).toFixed(2);
      const prot =
        Number(active.rffit_titer_iu_ml ?? 0) >= 0.5
          ? 'PROTECTIVE'
          : 'SUB-THRESHOLD';
      titerEl.textContent = `${titer} IU/mL (${prot})`;
    }
    const bulbarEl = document.getElementById('active-bulbar-status');
    if (bulbarEl) {
      bulbarEl.textContent = active.cranial_nerve_bulbar_palsy_absent
        ? 'ABSENT (NORMAL)'
        : 'PRESENT (ALERT)';
    }
    const bioEl = document.getElementById('active-biosecurity-guard');
    if (bioEl) {
      const lockStr = active.double_barrier_locked_enclosure_verified
        ? 'LOCKED'
        : 'BREACH';
      const dfaStr = active.brain_stem_hippocampus_preserved_guard
        ? 'DFA 4°C GUARD'
        : 'UNVERIFIED';
      bioEl.textContent = `${lockStr} · ${dfaStr}`;
    }

    const stg = active.quarantine_staging || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-patho-summary', active.pathophysiology_summary || '');
    setText('plan-rco-note', active.statutory_quarantine_rco_note || '');
    setText('plan-neuro-note', active.neuro_salivary_shedding_note || '');
    setText(
      'plan-vaccine-guard',
      stg.ten_day_bite_hold_vaccine_guard_note || ''
    );

    setText(
      'plan-double-barrier-ppe',
      stg.double_barrier_biosecurity_ppe_plan || ''
    );
    setText(
      'plan-wound-lavage-pep',
      stg.virucidal_wound_lavage_booster_pep_plan || ''
    );
    setText(
      'plan-rffit-serology',
      stg.prospective_serologic_rffit_plan || ''
    );
    setText(
      'plan-dfa-preservation',
      stg.dfa_neuropathology_preservation_note || ''
    );

    const checks = Array.isArray(active.neuro_observation_checks)
      ? active.neuro_observation_checks
      : [];
    const checksCountEl = document.getElementById('neuro-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('neuro-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Rabies Control Officer');
          const day = Number(chk.quarantine_day_elapsed ?? 0);
          const titer = Number(chk.rffit_titer_iu_ml ?? 0).toFixed(2);
          const bulbar = chk.cranial_nerve_bulbar_palsy_absent
            ? 'Absent'
            : 'PRESENT';
          const dysphagia = chk.hypersalivation_dysphagia_absent
            ? 'Absent'
            : 'PRESENT';
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="rbq-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · Day ${day} · RFFIT ${titer} IU/mL · Bulbar Palsy: ${bulbar} · Dysphagia/Hypersalivation: ${dysphagia} — ${note}</li>`;
        })
        .join('');
    }
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const caseId = escapeHtml(op.case_id || '');
    const rcoFiling = escapeHtml(op.rco_case_filing_number || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const breed = escapeHtml(op.breed || 'Mixed Breed');
    const weightKg = Number(op.weight_kg ?? 25.0).toFixed(1);
    const status = escapeHtml(
      op.status || 'BIOSECURITY_ISOLATION_RCO_HOLD_STAGED'
    );
    const badgeClass = getStatusBadgeClass(op.status);
    const category = escapeHtml(op.statutory_category || '');
    const protocol = escapeHtml(op.protocol || '');
    const elapsed = Number(op.quarantine_day_elapsed ?? 0);
    const reqDays = Number(op.required_quarantine_days ?? 10);
    const titer = Number(op.rffit_titer_iu_ml ?? 0).toFixed(2);
    const vacGuard = op.no_vaccine_during_10day_bite_hold_verified
      ? 'VERIFIED'
      : 'VIOLATION';
    const lockGuard = op.double_barrier_locked_enclosure_verified
      ? 'VERIFIED'
      : 'UNVERIFIED';
    const bulbarText = op.cranial_nerve_bulbar_palsy_absent
      ? 'Absent (Normal)'
      : 'PRESENT';

    return `
      <article
        class="trust-screening-card rbq-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-statutory-category="${category}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId} · ${rcoFiling}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="rbq-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="rbq-card-metrics">
          <span><strong>Category:</strong> ${category}</span>
          <span><strong>Protocol:</strong> ${protocol}</span>
          <span><strong>Quarantine Progress:</strong> Day ${elapsed} / ${reqDays}</span>
          <span><strong>RFFIT Titer:</strong> ${titer} IU/mL</span>
          <span><strong>10-Day No-Vaccine Guard:</strong> ${vacGuard}</span>
          <span><strong>Double-Barrier Locked:</strong> ${lockGuard}</span>
          <span><strong>Bulbar Palsy:</strong> ${bulbarText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary rbq-select-op-btn" data-op-id="${opId}">
            🔍 Open Rabies Quarantine Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('rbq-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('rbq-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="rbq-ops-empty-msg">No Rabies Quarantine operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('rbq-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/rabies-quarantine/operations?${qs}`
        : '/api/rabies-quarantine/operations';

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
      renderAllOperations(ops, data.metrics || data.overview);
      return ops;
    } catch (_err) {
      return [];
    }
  }

  async function selectOperationById(opId) {
    if (!opId) return;
    selectedOpId = opId;
    try {
      const res = await fetch(
        `/api/rabies-quarantine/operations/${encodeURIComponent(opId)}`,
        {
          headers: { Accept: 'application/json' },
        }
      );
      if (res.ok) {
        const op = await res.json();
        const idx = currentOps.findIndex((item) => item.id === op.id);
        if (idx >= 0) {
          currentOps[idx] = op;
        } else {
          currentOps.unshift(op);
        }
      }
    } catch (_err) {
      // Fallback to cached currentOps
    }
    renderActiveWorkspace(currentOps);
  }

  function setupSSEButton() {
    const connectBtn = document.getElementById('connect-sse-btn');
    const statusBadge = document.getElementById('sse-status-badge');
    if (!connectBtn) return;

    connectBtn.addEventListener('click', () => {
      if (eventSource) {
        eventSource.close();
        eventSource = null;
      }
      if (statusBadge) {
        statusBadge.textContent = 'SSE Connected';
        statusBadge.className = 'rbq-status-badge rbq-status-cleared';
      }
      eventSource = new EventSource('/api/rabies-quarantine/stream');
      eventSource.addEventListener('rabies_quarantine_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('rbq-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-rbq-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value ||
          'CASE-RBQ-104'
        ).trim();
        const petName = (
          document.getElementById('create-pet-name-input')?.value || ''
        ).trim();
        const species = (
          document.getElementById('create-species-input')?.value || 'Canine'
        ).trim();
        const breed = (
          document.getElementById('create-breed-input')?.value ||
          'Belgian Malinois'
        ).trim();
        const weightKg = parseFloat(
          document.getElementById('create-weight-input')?.value || '29.5'
        );
        const rcoFiling = (
          document.getElementById('create-rco-filing-input')?.value ||
          'RCO-STAT-2026-104'
        ).trim();
        const category = (
          document.getElementById('create-statutory-category-input')?.value ||
          'TEN_DAY_DOMESTIC_BITE_CONFINEMENT'
        ).trim();
        const protocol = (
          document.getElementById('create-protocol-input')?.value ||
          'STRICT_TEN_DAY_NO_VACCINE_DURING_BITE_HOLD'
        ).trim();
        const dayElapsed = parseInt(
          document.getElementById('create-day-elapsed-input')?.value || '1',
          10
        );
        const reqDays = parseInt(
          document.getElementById('create-required-days-input')?.value || '10',
          10
        );
        const rffitTiter = parseFloat(
          document.getElementById('create-rffit-titer-input')?.value || '0.40'
        );
        const noVaccineHold = Boolean(
          document.getElementById('create-no-vaccine-hold-checkbox')?.checked
        );
        const doubleBarrier = Boolean(
          document.getElementById('create-double-barrier-checkbox')?.checked
        );

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 29.5 : weightKg,
          rco_case_filing_number: rcoFiling,
          statutory_category: category,
          protocol,
          quarantine_day_elapsed: isNaN(dayElapsed) ? 1 : dayElapsed,
          required_quarantine_days: isNaN(reqDays) ? 10 : reqDays,
          rffit_titer_iu_ml: isNaN(rffitTiter) ? 0.4 : rffitTiter,
          no_vaccine_during_10day_bite_hold_verified: noVaccineHold,
          double_barrier_locked_enclosure_verified: doubleBarrier,
        };

        const btn = document.getElementById('create-rbq-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/rabies-quarantine/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted Rabies Quarantine patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to admit Rabies Quarantine patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-quarantine-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const protocol = (
          document.getElementById('stage-protocol-input')?.value ||
          'STRICT_TEN_DAY_NO_VACCINE_DURING_BITE_HOLD'
        ).trim();
        const rcoFiling = (
          document.getElementById('stage-rco-filing-input')?.value ||
          'RCO-STAT-2026-101'
        ).trim();
        const reqDays = parseInt(
          document.getElementById('stage-required-days-input')?.value || '10',
          10
        );
        const noVaccineHold = Boolean(
          document.getElementById('stage-no-vaccine-hold-checkbox')?.checked
        );
        const doubleBarrier = Boolean(
          document.getElementById('stage-double-barrier-checkbox')?.checked
        );
        const ppePlan = (
          document.getElementById('stage-ppe-plan-input')?.value ||
          'Padlocked double-door stainless-steel isolation run with rabid biohazard signage, pre-exposure immunized handlers, Kevlar gauntlets, face shield, and catch-pole'
        ).trim();
        const lavagePlan = (
          document.getElementById('stage-lavage-plan-input')?.value ||
          '15-minute virucidal 1% povidone-iodine irrigation; withhold animal rabies vaccine during 10-day human-bite hold'
        ).trim();
        const rffitPlan = (
          document.getElementById('stage-rffit-plan-input')?.value ||
          'Paired Day 0 and Day 5-7 RFFIT neutralizing antibody serum titers >= 0.50 IU/mL'
        ).trim();

        const payload = {
          protocol,
          rco_case_filing_number: rcoFiling,
          required_quarantine_days: isNaN(reqDays) ? 10 : reqDays,
          no_vaccine_during_10day_bite_hold_verified: noVaccineHold,
          double_barrier_locked_enclosure_verified: doubleBarrier,
          double_barrier_biosecurity_ppe_plan: ppePlan,
          virucidal_wound_lavage_booster_pep_plan: lavagePlan,
          prospective_serologic_rffit_plan: rffitPlan,
          brain_stem_hippocampus_preserved_guard: true,
          staged_by:
            'Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)',
        };

        const btn = document.getElementById('stage-quarantine-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/rabies-quarantine/operations/${encodeURIComponent(selectedOpId)}/staging`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(payload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Staged Biosecurity Isolation & RCO Hold for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to stage Rabies Quarantine operation';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-neuro-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dayElapsed = parseInt(
          document.getElementById('check-day-elapsed-input')?.value || '10',
          10
        );
        const rffitTiter = parseFloat(
          document.getElementById('check-rffit-titer-input')?.value || '1.45'
        );
        const bulbarAbsent = Boolean(
          document.getElementById('check-bulbar-absent-checkbox')?.checked
        );
        const dysphagiaAbsent = Boolean(
          document.getElementById('check-dysphagia-absent-checkbox')?.checked
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          quarantine_day_elapsed: isNaN(dayElapsed) ? 10 : dayElapsed,
          rffit_titer_iu_ml: isNaN(rffitTiter) ? 1.45 : rffitTiter,
          cranial_nerve_bulbar_palsy_absent: bulbarAbsent,
          hypersalivation_dysphagia_absent: dysphagiaAbsent,
          double_barrier_locked_enclosure_verified: true,
          no_vaccine_during_10day_bite_hold_verified: true,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-neuro-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/rabies-quarantine/operations/${encodeURIComponent(selectedOpId)}/neuro-check`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(payload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Recorded neurological & salivary shedding check for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to record neurological observation check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-rbq-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dayElapsed = parseInt(
          document.getElementById('clear-day-elapsed-input')?.value || '10',
          10
        );
        const rffitTiter = parseFloat(
          document.getElementById('clear-rffit-titer-input')?.value || '1.45'
        );
        const rcoReleaseSigned = Boolean(
          document.getElementById('clear-rco-release-checkbox')?.checked
        );
        const exitVaccineVerified = Boolean(
          document.getElementById('clear-exit-vaccine-checkbox')?.checked
        );
        const clearedBy = (
          document.getElementById('clear-cleared-by-input')?.value ||
          'Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          quarantine_day_elapsed: isNaN(dayElapsed) ? 10 : dayElapsed,
          rffit_titer_iu_ml: isNaN(rffitTiter) ? 1.45 : rffitTiter,
          cranial_nerve_bulbar_palsy_absent: true,
          rco_written_release_signed: rcoReleaseSigned,
          exit_rabies_vaccination_verified: exitVaccineVerified,
          cleared_by: clearedBy,
          clearance_summary: summary,
        };

        const btn = document.getElementById('clear-rbq-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/rabies-quarantine/operations/${encodeURIComponent(selectedOpId)}/clearance`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(payload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Cleared ${updated.id} (${updated.pet_name}) for Statutory RCO Release & Exit Vaccination`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to clear Rabies Quarantine operation';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }
  }

  function setupCardInteractions() {
    const gridEl = document.getElementById('rbq-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.rbq-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.rbq-op-card');
      if (card) {
        const opId = card.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
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
    setupSSEButton();
    setupFormsAndFilters();
    setupCardInteractions();
    fetchOperations();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
