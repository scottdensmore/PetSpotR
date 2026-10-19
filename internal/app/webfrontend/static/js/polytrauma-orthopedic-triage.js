/**
 * PetSpotR Acute Bite Wound, Vehicular Polytrauma, Degloving/Shearing Injury,
 * Open Fracture Gustilo-Anderson Classification, Wet-to-Dry/NPWT Vacuum-Assisted Closure &
 * Damage-Control Orthopedics Cockpit Controller (Milestone 24.2)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'ORTHO-OP-101';
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
    const liveStatus = document.getElementById('ortho-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('ortho-error-banner');
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
      case 'ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE':
        return 'ortho-status-critical';
      case 'LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED':
        return 'ortho-status-staged';
      case 'GRANULATION_FIXATION_STABLE_CLEARED':
        return 'ortho-status-cleared';
      default:
        return 'ortho-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const acuteEl = document.getElementById('kpi-acute-triage');
    const stagedEl = document.getElementById('kpi-npwt-staged');
    const clearedEl = document.getElementById('kpi-cleared-fixation');
    const gustiloEl = document.getElementById('kpi-gustilo-iii-count');
    const thoracicEl = document.getElementById('kpi-thoracic-bite-count');

    const total = metrics?.total_operations ?? ops.length;
    const acute =
      metrics?.acute_open_fracture_triage_count ??
      ops.filter((op) => op.status === 'ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE').length;
    const staged =
      metrics?.npwt_splint_staged_count ??
      ops.filter((op) => op.status === 'LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED').length;
    const cleared =
      metrics?.cleared_fixation_count ??
      ops.filter((op) => op.status === 'GRANULATION_FIXATION_STABLE_CLEARED').length;
    const gustiloIII =
      metrics?.gustilo_grade_iii_shearing_count ??
      ops.filter((op) => op.gustilo_class === 'GUSTILO_GRADE_III_SHEARING_DEGLOVING').length;
    const thoracicBite =
      metrics?.thoracic_bite_penetration_count ??
      ops.filter(
        (op) =>
          op.thoracic_wall_penetration_suspected ||
          op.gustilo_class === 'BIG_DOG_LITTLE_DOG_THORACIC_BITE_FLAIL'
      ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (acuteEl) acuteEl.textContent = String(acute);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (gustiloEl) gustiloEl.textContent = String(gustiloIII);
    if (thoracicEl) thoracicEl.textContent = String(thoracicBite);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const fix = op.fixation_plan || {};
    const gelInput = document.getElementById('stage-gel-clip-input');
    const lavageInput = document.getElementById('stage-lavage-plan-input');
    const npwtInput = document.getElementById('stage-npwt-plan-input');
    const abxInput = document.getElementById('stage-abx-culture-input');
    const splintInput = document.getElementById('stage-splint-plan-input');
    const esfInput = document.getElementById('stage-esf-plan-input');

    if (gelInput && fix.sterile_water_soluble_gel_clip_plan) {
      gelInput.value = fix.sterile_water_soluble_gel_clip_plan;
    }
    if (lavageInput && fix.copious_lavage_35_ml_syringe_18g_plan) {
      lavageInput.value = fix.copious_lavage_35_ml_syringe_18g_plan;
    }
    if (npwtInput && fix.npwt_vacuum_or_tie_over_dressing_plan) {
      npwtInput.value = fix.npwt_vacuum_or_tie_over_dressing_plan;
    }
    if (abxInput && fix.empiric_abx_culture_before_lavage) {
      abxInput.value = fix.empiric_abx_culture_before_lavage;
    }
    if (splintInput && fix.robert_jones_or_spica_splint_plan) {
      splintInput.value = fix.robert_jones_or_spica_splint_plan;
    }
    if (esfInput && fix.definitive_external_skeletal_fixator) {
      esfInput.value = fix.definitive_external_skeletal_fixator;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('ortho-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (active && active.status === 'GRANULATION_FIXATION_STABLE_CLEARED') {
      const by = active.discharged_by || 'Attending Orthopedic Surgeon';
      const summary =
        active.discharge_summary ||
        'Healthy granulation bed and orthopedic fixation stability verified.';
      if (summaryTextEl) {
        summaryTextEl.textContent = `${active.id} (${active.pet_name}) — Discharged by ${by}: ${summary}`;
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
      const weightStr = Number(active.weight_kg ?? 19.5).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Canine'} · ${active.breed || 'Mixed Breed'} · ${weightStr} kg)`;
    }

    const gustiloEl = document.getElementById('active-gustilo-class');
    if (gustiloEl) {
      gustiloEl.textContent = active.gustilo_class || '';
    }
    const exposedEl = document.getElementById('active-exposed-bone-flag');
    if (exposedEl) {
      exposedEl.textContent = active.exposed_cortical_bone_tendon ? 'YES' : 'NO';
    }
    const thoracicEl = document.getElementById('active-thoracic-bite-flag');
    if (thoracicEl) {
      thoracicEl.textContent = active.thoracic_wall_penetration_suspected
        ? 'SUSPECTED'
        : 'NO';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent =
        active.status || 'ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE';
      badgeEl.className = `ortho-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const attEl = document.getElementById('active-att-score');
    if (attEl) {
      attEl.textContent = `${Number(active.animal_trauma_triage_score ?? 0)}/18`;
    }
    const lavageEl = document.getElementById('active-lavage-volume');
    if (lavageEl) {
      lavageEl.textContent = `${Number(active.lavage_volume_ml ?? 0)} mL`;
    }
    const npwtEl = document.getElementById('active-npwt-vacuum');
    if (npwtEl) {
      npwtEl.textContent = `${Number(active.npwt_vacuum_mmhg ?? 0)} mmHg`;
    }
    const lacToeEl = document.getElementById('active-lactate-toe-delta');
    if (lacToeEl) {
      const lac = Number(active.blood_lactate_mmol_l ?? 0).toFixed(1);
      const toe = Number(active.distal_toe_temp_delta_c ?? 0).toFixed(1);
      lacToeEl.textContent = `${lac} mmol/L / ${toe}°C`;
    }

    const assess = active.assessment_plan || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-gustilo-patho', assess.gustilo_grade_pathophysiology || '');
    setText('plan-iceberg-alert', assess.iceberg_bite_dead_space_alert || '');
    setText('plan-lavage-note', assess.high_pressure_lavage_7_to_8_psi_note || '');
    setText('plan-closure-guard', assess.primary_closure_contraindication || '');
    setText('plan-splint-rule', assess.joint_above_joint_below_splint_rule || '');
    setText('plan-neurovasc-note', assess.vascular_neuro_distal_limb_note || '');

    const fix = active.fixation_plan || {};
    setText('plan-gel-clip', fix.sterile_water_soluble_gel_clip_plan || '');
    setText('plan-copious-lavage', fix.copious_lavage_35_ml_syringe_18g_plan || '');
    setText('plan-npwt-dressing', fix.npwt_vacuum_or_tie_over_dressing_plan || '');
    setText('plan-abx-culture', fix.empiric_abx_culture_before_lavage || '');
    setText('plan-robert-jones', fix.robert_jones_or_spica_splint_plan || '');
    setText('plan-definitive-esf', fix.definitive_external_skeletal_fixator || '');

    const checks = Array.isArray(active.wound_checks) ? active.wound_checks : [];
    const checksCountEl = document.getElementById('wound-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('wound-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const att = Number(chk.animal_trauma_triage_score ?? 0);
          const lav = Number(chk.lavage_volume_ml ?? 0);
          const npwt = Number(chk.npwt_vacuum_mmhg ?? 0);
          const lac = Number(chk.blood_lactate_mmol_l ?? 0).toFixed(1);
          const toe = Number(chk.distal_toe_temp_delta_c ?? 0).toFixed(1);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="ortho-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · ATT ${att}/18 · Lavage ${lav} mL · NPWT ${npwt} mmHg · Lactate ${lac} mmol/L · Toe ΔT ${toe}°C — ${note}</li>`;
        })
        .join('');
    }
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const caseId = escapeHtml(op.case_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const breed = escapeHtml(op.breed || 'Mixed Breed');
    const weightKg = Number(op.weight_kg ?? 19.5).toFixed(1);
    const status = escapeHtml(
      op.status || 'ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE'
    );
    const badgeClass = getStatusBadgeClass(op.status);
    const gustiloClass = escapeHtml(op.gustilo_class || '');
    const att = Number(op.animal_trauma_triage_score ?? 0);
    const lav = Number(op.lavage_volume_ml ?? 0);
    const npwt = Number(op.npwt_vacuum_mmhg ?? 0);
    const lac = Number(op.blood_lactate_mmol_l ?? 0).toFixed(1);
    const toe = Number(op.distal_toe_temp_delta_c ?? 0).toFixed(1);
    const exposedText = op.exposed_cortical_bone_tendon ? 'YES' : 'NO';
    const thoracicText = op.thoracic_wall_penetration_suspected
      ? 'SUSPECTED'
      : 'NO';

    return `
      <article
        class="trust-screening-card ortho-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-gustilo-class="${gustiloClass}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="ortho-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="ortho-card-metrics">
          <span><strong>Gustilo Class:</strong> ${gustiloClass}</span>
          <span><strong>ATT Score:</strong> ${att}/18</span>
          <span><strong>Lavage:</strong> ${lav} mL</span>
          <span><strong>NPWT:</strong> ${npwt} mmHg</span>
          <span><strong>Lactate:</strong> ${lac} mmol/L</span>
          <span><strong>Toe ΔT:</strong> ${toe}°C</span>
          <span><strong>Exposed Bone:</strong> ${exposedText}</span>
          <span><strong>Thoracic Bite:</strong> ${thoracicText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary ortho-select-op-btn" data-op-id="${opId}">
            🔍 Open Ortho Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('ortho-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('ortho-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="ortho-ops-empty-msg">No Polytrauma &amp; Orthopedic operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('ortho-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/polytrauma-orthopedic/operations?${qs}`
        : '/api/polytrauma-orthopedic/operations';

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
      renderAllOperations(ops, data.metrics);
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
        `/api/polytrauma-orthopedic/operations/${encodeURIComponent(opId)}`,
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
        statusBadge.className = 'ortho-status-badge ortho-status-cleared';
      }
      eventSource = new EventSource('/api/polytrauma-orthopedic/stream');
      eventSource.addEventListener('polytrauma_ortho_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('ortho-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-ortho-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value ||
          'CASE-ORTHO-104'
        ).trim();
        const petName = (
          document.getElementById('create-pet-name-input')?.value || ''
        ).trim();
        const species = (
          document.getElementById('create-species-input')?.value || 'Canine'
        ).trim();
        const breed = (
          document.getElementById('create-breed-input')?.value ||
          'Australian Shepherd'
        ).trim();
        const weightKg = parseFloat(
          document.getElementById('create-weight-input')?.value || '22.5'
        );
        const gustiloClass = (
          document.getElementById('create-gustilo-class-input')?.value ||
          'GUSTILO_GRADE_III_SHEARING_DEGLOVING'
        ).trim();
        const attScore = parseInt(
          document.getElementById('create-att-score-input')?.value || '8',
          10
        );
        const lavageML = parseInt(
          document.getElementById('create-lavage-input')?.value || '1800',
          10
        );
        const npwtMmHg = parseInt(
          document.getElementById('create-npwt-input')?.value || '0',
          10
        );
        const lactate = parseFloat(
          document.getElementById('create-lactate-input')?.value || '3.9'
        );
        const toeDelta = parseFloat(
          document.getElementById('create-toe-delta-input')?.value || '1.6'
        );
        const exposedBone = Boolean(
          document.getElementById('create-exposed-bone-checkbox')?.checked
        );
        const thoracicPenetration = Boolean(
          document.getElementById('create-thoracic-penetration-checkbox')
            ?.checked
        );

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 22.5 : weightKg,
          gustilo_class: gustiloClass,
          animal_trauma_triage_score: isNaN(attScore) ? 8 : attScore,
          lavage_volume_ml: isNaN(lavageML) ? 1800 : lavageML,
          npwt_vacuum_mmhg: isNaN(npwtMmHg) ? 0 : npwtMmHg,
          blood_lactate_mmol_l: isNaN(lactate) ? 3.9 : lactate,
          distal_toe_temp_delta_c: isNaN(toeDelta) ? 1.6 : toeDelta,
          exposed_cortical_bone_tendon: exposedBone,
          thoracic_wall_penetration_suspected: thoracicPenetration,
        };

        const btn = document.getElementById('create-ortho-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/polytrauma-orthopedic/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted Polytrauma / Open Fracture patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to admit Polytrauma / Open Fracture patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-fixation-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const gelClip = (
          document.getElementById('stage-gel-clip-input')?.value ||
          'Pack open wound bed generously with sterile K-Y water-soluble lubricant prior to wide #40 surgical clip'
        ).trim();
        const lavagePlan = (
          document.getElementById('stage-lavage-plan-input')?.value ||
          'Copious 7–8 PSI warm isotonic LRS lavage (2,500 mL via 1 L bag in 300 mmHg pressure cuff + 18G needle)'
        ).trim();
        const npwtPlan = (
          document.getElementById('stage-npwt-plan-input')?.value ||
          'Apply NPWT open-pore polyurethane foam at -125 mmHg continuous vacuum'
        ).trim();
        const abxPlan = (
          document.getElementById('stage-abx-culture-input')?.value || ''
        ).trim();
        const splintPlan = (
          document.getElementById('stage-splint-plan-input')?.value || ''
        ).trim();
        const esfPlan = (
          document.getElementById('stage-esf-plan-input')?.value || ''
        ).trim();

        const payload = {
          sterile_water_soluble_gel_clip_plan: gelClip,
          copious_lavage_35_ml_syringe_18g_plan: lavagePlan,
          npwt_vacuum_or_tie_over_dressing_plan: npwtPlan,
          empiric_abx_culture_before_lavage: abxPlan,
          robert_jones_or_spica_splint_plan: splintPlan,
          definitive_external_skeletal_fixator: esfPlan,
        };

        const btn = document.getElementById('stage-fixation-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/polytrauma-orthopedic/operations/${encodeURIComponent(selectedOpId)}/stage-fixation`,
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
              `Staged 7–8 PSI Lavage / NPWT (-125 mmHg) / Robert Jones & ESF for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to stage debridement & fixation protocol';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-wound-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const attScore = parseInt(
          document.getElementById('check-att-score-input')?.value || '3',
          10
        );
        const lavageML = parseInt(
          document.getElementById('check-lavage-input')?.value || '3000',
          10
        );
        const npwtMmHg = parseInt(
          document.getElementById('check-npwt-input')?.value || '-125',
          10
        );
        const lactate = parseFloat(
          document.getElementById('check-lactate-input')?.value || '1.4'
        );
        const toeDelta = parseFloat(
          document.getElementById('check-toe-delta-input')?.value || '0.3'
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Caleb Thorne, DACVS (Orthopedics)'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          animal_trauma_triage_score: isNaN(attScore) ? 3 : attScore,
          lavage_volume_ml: isNaN(lavageML) ? 3000 : lavageML,
          npwt_vacuum_mmhg: isNaN(npwtMmHg) ? -125 : npwtMmHg,
          blood_lactate_mmol_l: isNaN(lactate) ? 1.4 : lactate,
          distal_toe_temp_delta_c: isNaN(toeDelta) ? 0.3 : toeDelta,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-wound-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/polytrauma-orthopedic/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
              `Recorded orthopedic & wound bed check for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record wound check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-ortho-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dischargedBy = (
          document.getElementById('clear-discharged-by-input')?.value ||
          'Dr. Caleb Thorne, DACVS (Orthopedics)'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          discharged_by: dischargedBy,
          discharge_summary: summary,
        };

        const btn = document.getElementById('clear-ortho-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/polytrauma-orthopedic/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Granulation & Fixation Stable Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error ||
              'Failed to clear Polytrauma & Orthopedic operation for discharge';
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
    const gridEl = document.getElementById('ortho-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.ortho-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.ortho-op-card');
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
