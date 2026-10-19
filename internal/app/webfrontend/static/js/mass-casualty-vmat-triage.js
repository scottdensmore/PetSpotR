/**
 * PetSpotR Acute Disaster Mass-Casualty Veterinary Triage (START/SALT Veterinary Color-Tagging:
 * Immediate Red, Delayed Yellow, Minimal Green, Expectant Black), Decontamination Corridor,
 * Field Hospital Surge Capacity & FEMA/AVMA VMAT Deployment Command Cockpit Controller (Milestone 25.5)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'MCI-OP-101';
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
    const liveStatus = document.getElementById('mci-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('mci-error-banner');
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
      case 'START_SALT_TAGGED_DECON_STAGED':
        return 'mci-status-critical';
      case 'DAMAGE_CONTROL_SURGE_ICU_MONITORED':
        return 'mci-status-staged';
      case 'REUNIFIED_EVAC_TRANSPORT_CLEARED':
        return 'mci-status-cleared';
      default:
        return 'mci-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const stagedEl = document.getElementById('kpi-decon-staged');
    const monitoredEl = document.getElementById('kpi-surge-monitored');
    const clearedEl = document.getElementById('kpi-evac-cleared');
    const redCountEl = document.getElementById('kpi-red-immediate-count');
    const reverseSurgeEl = document.getElementById('kpi-reverse-surge-count');

    const total = metrics?.total_operations ?? ops.length;
    const staged =
      metrics?.start_salt_tagged_decon_staged_count ??
      ops.filter((op) => op.status === 'START_SALT_TAGGED_DECON_STAGED').length;
    const monitored =
      metrics?.damage_control_surge_monitored_count ??
      ops.filter((op) => op.status === 'DAMAGE_CONTROL_SURGE_ICU_MONITORED')
        .length;
    const cleared =
      metrics?.reunified_evac_cleared_count ??
      ops.filter((op) => op.status === 'REUNIFIED_EVAC_TRANSPORT_CLEARED')
        .length;
    const redCount =
      metrics?.red_immediate_priority_1_count ??
      ops.filter(
        (op) => op.triage_tag === 'RED_IMMEDIATE_PRIORITY_1_LIFE_THREAT'
      ).length;
    const reverseSurge =
      metrics?.reverse_triage_surge_triggered_count ??
      ops.filter((op) => Boolean(op.reverse_triage_surge_triggered)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (monitoredEl) monitoredEl.textContent = String(monitored);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (redCountEl) redCountEl.textContent = String(redCount);
    if (reverseSurgeEl) reverseSurgeEl.textContent = String(reverseSurge);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const stg = op.field_triage_staging || {};
    const protocolInput = document.getElementById('stage-protocol-input');
    const tagInput = document.getElementById('stage-triage-tag-input');
    const deconPlanInput = document.getElementById('stage-decon-plan-input');
    const dcrPlanInput = document.getElementById('stage-dcr-plan-input');
    const rfidInput = document.getElementById('stage-rfid-collar-input');
    const icsInput = document.getElementById('stage-ics214-input');
    const occupancyInput = document.getElementById(
      'stage-surge-occupancy-input'
    );
    const grossDeconCheckbox = document.getElementById(
      'stage-gross-decon-checkbox'
    );
    const contagiousCheckbox = document.getElementById(
      'stage-contagious-segregated-checkbox'
    );

    if (protocolInput && (stg.protocol || op.protocol)) {
      protocolInput.value = stg.protocol || op.protocol;
    }
    if (tagInput && (stg.triage_tag || op.triage_tag)) {
      tagInput.value = stg.triage_tag || op.triage_tag;
    }
    if (deconPlanInput && stg.warm_zone_decontamination_corridor_plan) {
      deconPlanInput.value = stg.warm_zone_decontamination_corridor_plan;
    }
    if (dcrPlanInput && stg.damage_control_resuscitation_plan) {
      dcrPlanInput.value = stg.damage_control_resuscitation_plan;
    }
    if (
      rfidInput &&
      (stg.waterproof_rfid_triage_collar_number ||
        op.waterproof_rfid_triage_collar_number)
    ) {
      rfidInput.value =
        stg.waterproof_rfid_triage_collar_number ||
        op.waterproof_rfid_triage_collar_number;
    }
    if (
      icsInput &&
      (stg.ics214_incident_number || op.ics214_incident_number)
    ) {
      icsInput.value = stg.ics214_incident_number || op.ics214_incident_number;
    }
    if (occupancyInput && op.surge_bed_occupancy_percent !== undefined) {
      occupancyInput.value = String(op.surge_bed_occupancy_percent);
    }
    if (grossDeconCheckbox) {
      grossDeconCheckbox.checked = Boolean(
        op.gross_decontamination_prior_to_cold_zone_verified
      );
    }
    if (contagiousCheckbox) {
      contagiousCheckbox.checked = Boolean(op.contagious_cohort_segregated);
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('mci-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'REUNIFIED_EVAC_TRANSPORT_CLEARED'
    ) {
      const by =
        active.discharged_by ||
        active.clearance?.cleared_by ||
        'FEMA VMAT Incident Commander';
      const summary =
        active.discharge_summary ||
        active.clearance?.clearance_summary ||
        'Perfusion normalized, SpO2 >= 95.0%, waterproof RFID collar matched, and FEMA ICS-214 signed.';
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

    const tagEl = document.getElementById('active-triage-tag');
    if (tagEl) {
      tagEl.textContent = active.triage_tag || '';
    }
    const protoEl = document.getElementById('active-protocol');
    if (protoEl) {
      protoEl.textContent = active.protocol || '';
    }
    const rfidEl = document.getElementById('active-rfid-collar');
    if (rfidEl) {
      rfidEl.textContent = active.waterproof_rfid_triage_collar_number || '';
    }
    const icsEl = document.getElementById('active-ics214');
    if (icsEl) {
      icsEl.textContent = active.ics214_incident_number || '';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent =
        active.status || 'START_SALT_TAGGED_DECON_STAGED';
      badgeEl.className = `mci-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const atsEl = document.getElementById('active-ats-score');
    if (atsEl) {
      atsEl.textContent = `${Number(active.animal_triage_score_ats ?? 0)}/25`;
    }
    const lactateEl = document.getElementById('active-lactate');
    if (lactateEl) {
      lactateEl.textContent = `${Number(active.lactate_mmol_l ?? 0).toFixed(2)} mmol/L`;
    }
    const spo2El = document.getElementById('active-spo2');
    if (spo2El) {
      spo2El.textContent = `${Number(active.spo2_percent ?? 0).toFixed(1)}%`;
    }
    const occEl = document.getElementById('active-surge-occupancy');
    if (occEl) {
      const occ = Number(active.surge_bed_occupancy_percent ?? 0).toFixed(1);
      const revStr = active.reverse_triage_surge_triggered
        ? 'REVERSE TRIAGE'
        : 'STANDARD';
      const deconStr = active.gross_decontamination_prior_to_cold_zone_verified
        ? 'DECON VERIFIED'
        : 'BLOCKED';
      occEl.textContent = `${occ}% (${revStr}) · ${deconStr}`;
    }

    const stg = active.field_triage_staging || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-start-salt-summary', active.start_salt_triage_summary || '');
    setText(
      'plan-decon-biosecurity',
      active.decon_corridor_biosecurity_note || ''
    );
    setText('plan-surge-ics', active.surge_capacity_ics_command_note || '');
    setText(
      'plan-cold-zone-guard',
      stg.cold_zone_cross_contamination_guard_note || ''
    );

    setText(
      'plan-warm-zone-decon',
      stg.warm_zone_decontamination_corridor_plan || ''
    );
    setText(
      'plan-damage-control',
      stg.damage_control_resuscitation_plan || ''
    );
    setText('plan-ics214-custody', stg.ics214_chain_of_custody_plan || '');
    setText(
      'plan-biosecurity-isolation',
      stg.biosecurity_cohort_isolation_plan || ''
    );
    setText(
      'plan-reverse-triage',
      stg.reverse_triage_surge_allocation_plan || ''
    );

    const checks = Array.isArray(active.reassessment_checks)
      ? active.reassessment_checks
      : [];
    const checksCountEl = document.getElementById('reassessment-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('reassessment-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'VMAT Commander');
          const tag = escapeHtml(chk.triage_tag || '');
          const ats = Number(chk.animal_triage_score_ats ?? 0);
          const lac = Number(chk.lactate_mmol_l ?? 0).toFixed(2);
          const spo2 = Number(chk.spo2_percent ?? 0).toFixed(1);
          const occ = Number(chk.surge_bed_occupancy_percent ?? 0).toFixed(1);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="mci-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · Tag: ${tag} · ATS ${ats}/25 · Lactate ${lac} mmol/L · SpO2 ${spo2}% · Surge Occupancy ${occ}% — ${note}</li>`;
        })
        .join('');
    }
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const caseId = escapeHtml(op.case_id || '');
    const rfid = escapeHtml(op.waterproof_rfid_triage_collar_number || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const breed = escapeHtml(op.breed || 'Mixed Breed');
    const weightKg = Number(op.weight_kg ?? 25.0).toFixed(1);
    const status = escapeHtml(op.status || 'START_SALT_TAGGED_DECON_STAGED');
    const badgeClass = getStatusBadgeClass(op.status);
    const triageTag = escapeHtml(op.triage_tag || '');
    const protocol = escapeHtml(op.protocol || '');
    const ats = Number(op.animal_triage_score_ats ?? 0);
    const lac = Number(op.lactate_mmol_l ?? 0).toFixed(2);
    const spo2 = Number(op.spo2_percent ?? 0).toFixed(1);
    const occ = Number(op.surge_bed_occupancy_percent ?? 0).toFixed(1);
    const deconText = op.gross_decontamination_prior_to_cold_zone_verified
      ? 'VERIFIED'
      : 'PENDING';

    return `
      <article
        class="trust-screening-card mci-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-triage-tag="${triageTag}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId} · ${rfid}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="mci-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="mci-card-metrics">
          <span><strong>START Tag:</strong> ${triageTag}</span>
          <span><strong>Protocol:</strong> ${protocol}</span>
          <span><strong>ATS Score:</strong> ${ats}/25</span>
          <span><strong>Lactate:</strong> ${lac} mmol/L</span>
          <span><strong>SpO2:</strong> ${spo2}%</span>
          <span><strong>Surge Occupancy:</strong> ${occ}%</span>
          <span><strong>Warm-Zone Decon:</strong> ${deconText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary mci-select-op-btn" data-op-id="${opId}">
            🔍 Open VMAT Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('mci-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('mci-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="mci-ops-empty-msg">No Mass-Casualty VMAT operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('mci-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/mass-casualty-vmat/operations?${qs}`
        : '/api/mass-casualty-vmat/operations';

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
        `/api/mass-casualty-vmat/operations/${encodeURIComponent(opId)}`,
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
        statusBadge.className = 'mci-status-badge mci-status-cleared';
      }
      eventSource = new EventSource('/api/mass-casualty-vmat/stream');
      eventSource.addEventListener('mass_casualty_vmat_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('mci-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-mci-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value ||
          'CASE-MCI-104'
        ).trim();
        const petName = (
          document.getElementById('create-pet-name-input')?.value || ''
        ).trim();
        const species = (
          document.getElementById('create-species-input')?.value || 'Canine'
        ).trim();
        const breed = (
          document.getElementById('create-breed-input')?.value ||
          'German Shepherd (SAR K9)'
        ).trim();
        const weightKg = parseFloat(
          document.getElementById('create-weight-input')?.value || '32.0'
        );
        const triageTag = (
          document.getElementById('create-triage-tag-input')?.value ||
          'RED_IMMEDIATE_PRIORITY_1_LIFE_THREAT'
        ).trim();
        const protocol = (
          document.getElementById('create-protocol-input')?.value ||
          'HOT_WARM_COLD_ZONE_DAWN_DECON_CORRIDOR'
        ).trim();
        const atsScore = parseInt(
          document.getElementById('create-ats-score-input')?.value || '18',
          10
        );
        const lactate = parseFloat(
          document.getElementById('create-lactate-input')?.value || '6.4'
        );
        const spo2 = parseFloat(
          document.getElementById('create-spo2-input')?.value || '88.0'
        );
        const surgeOccupancy = parseFloat(
          document.getElementById('create-surge-occupancy-input')?.value ||
            '92.0'
        );
        const rfidCollar = (
          document.getElementById('create-rfid-collar-input')?.value ||
          'VMAT-RFID-9104'
        ).trim();
        const ics214 = (
          document.getElementById('create-ics214-input')?.value ||
          'ICS214-FEMA-2026-104'
        ).trim();
        const grossDecon = Boolean(
          document.getElementById('create-gross-decon-checkbox')?.checked
        );
        const contagiousSegregated = Boolean(
          document.getElementById('create-contagious-segregated-checkbox')
            ?.checked
        );

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 32.0 : weightKg,
          triage_tag: triageTag,
          protocol,
          animal_triage_score_ats: isNaN(atsScore) ? 18 : atsScore,
          lactate_mmol_l: isNaN(lactate) ? 6.4 : lactate,
          spo2_percent: isNaN(spo2) ? 88.0 : spo2,
          surge_bed_occupancy_percent: isNaN(surgeOccupancy)
            ? 92.0
            : surgeOccupancy,
          waterproof_rfid_triage_collar_number: rfidCollar,
          ics214_incident_number: ics214,
          gross_decontamination_prior_to_cold_zone_verified: grossDecon,
          contagious_cohort_segregated: contagiousSegregated,
        };

        const btn = document.getElementById('create-mci-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/mass-casualty-vmat/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted Mass-Casualty VMAT patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to admit Mass-Casualty VMAT patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-vmat-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const protocol = (
          document.getElementById('stage-protocol-input')?.value ||
          'HOT_WARM_COLD_ZONE_DAWN_DECON_CORRIDOR'
        ).trim();
        const triageTag = (
          document.getElementById('stage-triage-tag-input')?.value ||
          'RED_IMMEDIATE_PRIORITY_1_LIFE_THREAT'
        ).trim();
        const deconPlan = (
          document.getElementById('stage-decon-plan-input')?.value ||
          'Warm-zone tepid low-pressure water + Dawn dish detergent dermal degreasing wash and 0.9% sterile saline ocular/oral flush verified prior to Cold-Zone VMAT ICU entry'
        ).trim();
        const dcrPlan = (
          document.getElementById('stage-dcr-plan-input')?.value ||
          'Direct arterial pressure bandage + Tranexamic Acid (TXA) 10 mg/kg IV + Hypertonic Saline 7.2% 4 mL/kg IV + Methadone 0.2 mg/kg IV analgesia and high-flow oxygen cage'
        ).trim();
        const rfidCollar = (
          document.getElementById('stage-rfid-collar-input')?.value ||
          'VMAT-RFID-9101'
        ).trim();
        const ics214 = (
          document.getElementById('stage-ics214-input')?.value ||
          'ICS214-FEMA-2026-101'
        ).trim();
        const surgeOccupancy = parseFloat(
          document.getElementById('stage-surge-occupancy-input')?.value ||
            '91.0'
        );
        const grossDecon = Boolean(
          document.getElementById('stage-gross-decon-checkbox')?.checked
        );
        const contagiousSegregated = Boolean(
          document.getElementById('stage-contagious-segregated-checkbox')
            ?.checked
        );

        const payload = {
          protocol,
          triage_tag: triageTag,
          warm_zone_decontamination_corridor_plan: deconPlan,
          damage_control_resuscitation_plan: dcrPlan,
          waterproof_rfid_triage_collar_number: rfidCollar,
          ics214_incident_number: ics214,
          surge_bed_occupancy_percent: isNaN(surgeOccupancy)
            ? 91.0
            : surgeOccupancy,
          gross_decontamination_prior_to_cold_zone_verified: grossDecon,
          contagious_cohort_segregated: contagiousSegregated,
          staged_by:
            'Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)',
        };

        const btn = document.getElementById('stage-vmat-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/mass-casualty-vmat/operations/${encodeURIComponent(selectedOpId)}/staging`,
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
              `Staged Warm-Zone Decon, Damage-Control & ICS-214 Protocol for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to stage VMAT field triage protocol';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-reassessment-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const triageTag = (
          document.getElementById('check-triage-tag-input')?.value ||
          'YELLOW_DELAYED_PRIORITY_2_URGENT_SURGERY'
        ).trim();
        const atsScore = parseInt(
          document.getElementById('check-ats-score-input')?.value || '5',
          10
        );
        const lactate = parseFloat(
          document.getElementById('check-lactate-input')?.value || '1.85'
        );
        const spo2 = parseFloat(
          document.getElementById('check-spo2-input')?.value || '97.5'
        );
        const surgeOccupancy = parseFloat(
          document.getElementById('check-surge-occupancy-input')?.value ||
            '78.0'
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          triage_tag: triageTag,
          animal_triage_score_ats: isNaN(atsScore) ? 5 : atsScore,
          lactate_mmol_l: isNaN(lactate) ? 1.85 : lactate,
          spo2_percent: isNaN(spo2) ? 97.5 : spo2,
          surge_bed_occupancy_percent: isNaN(surgeOccupancy)
            ? 78.0
            : surgeOccupancy,
          gross_decontamination_prior_to_cold_zone_verified: true,
          contagious_cohort_segregated: true,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-reassessment-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/mass-casualty-vmat/operations/${encodeURIComponent(selectedOpId)}/reassessment`,
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
              `Recorded VMAT surge reassessment check for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to record VMAT reassessment check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-mci-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const lactate = parseFloat(
          document.getElementById('clear-lactate-input')?.value || '1.85'
        );
        const spo2 = parseFloat(
          document.getElementById('clear-spo2-input')?.value || '97.5'
        );
        const rfidVerified = Boolean(
          document.getElementById('clear-rfid-verified-checkbox')?.checked
        );
        const icsSigned = Boolean(
          document.getElementById('clear-ics214-signed-checkbox')?.checked
        );
        const clearedBy = (
          document.getElementById('clear-cleared-by-input')?.value ||
          'Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          lactate_mmol_l: isNaN(lactate) ? 1.85 : lactate,
          spo2_percent: isNaN(spo2) ? 97.5 : spo2,
          waterproof_rfid_matched_verified: rfidVerified,
          ics214_chain_of_custody_signed: icsSigned,
          cleared_by: clearedBy,
          clearance_summary: summary,
        };

        const btn = document.getElementById('clear-mci-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/mass-casualty-vmat/operations/${encodeURIComponent(selectedOpId)}/clearance`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Owner Reunification / Medevac Transport`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to clear Mass-Casualty VMAT operation';
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
    const gridEl = document.getElementById('mci-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.mci-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.mci-op-card');
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
