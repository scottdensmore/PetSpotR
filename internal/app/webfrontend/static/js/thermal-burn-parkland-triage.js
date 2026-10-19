/**
 * PetSpotR Acute Smoke Inhalation, Thermal Burn Classification
 * (Superficial/Partial/Full-Thickness TBSA%), Parkland Crystalloid
 * Resuscitation Formula, Silver Sulfadiazine & Escharotomy Cockpit Controller (Milestone 24.1)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'BURN-OP-101';
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
    const liveStatus = document.getElementById('burn-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('burn-error-banner');
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
      case 'ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE':
        return 'burn-status-critical';
      case 'PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED':
        return 'burn-status-staged';
      case 'GRANULATION_BED_NORMOALBUMINEMIC_CLEARED':
        return 'burn-status-cleared';
      default:
        return 'burn-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const acuteEl = document.getElementById('kpi-acute-burn');
    const stagedEl = document.getElementById('kpi-parkland-staged');
    const clearedEl = document.getElementById('kpi-cleared-granulation');
    const severeEl = document.getElementById('kpi-severe-tbsa');
    const escharEl = document.getElementById('kpi-circumferential-eschar');

    const total = metrics?.total_operations ?? ops.length;
    const acute =
      metrics?.acute_burn_triage_count ??
      ops.filter((op) => op.status === 'ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE').length;
    const staged =
      metrics?.parkland_staged_count ??
      ops.filter((op) => op.status === 'PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED').length;
    const cleared =
      metrics?.cleared_granulation_count ??
      ops.filter((op) => op.status === 'GRANULATION_BED_NORMOALBUMINEMIC_CLEARED').length;
    const severe =
      metrics?.severe_tbsa_over_20_pct_count ??
      ops.filter((op) => Number(op.tbsa_percent ?? 0) >= 20.0).length;
    const eschar =
      metrics?.circumferential_eschar_count ??
      ops.filter((op) => Boolean(op.circumferential_thoracic_eschar)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (acuteEl) acuteEl.textContent = String(acute);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (severeEl) severeEl.textContent = String(severe);
    if (escharEl) escharEl.textContent = String(eschar);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const wound = op.wound_plan || {};
    const parklandInput = document.getElementById('stage-parkland-schedule-input');
    const ssdInput = document.getElementById('stage-ssd-honey-input');
    const escharInput = document.getElementById('stage-escharotomy-input');
    const analgesiaInput = document.getElementById('stage-analgesia-cri-input');
    const albuminInput = document.getElementById('stage-albumin-transition-input');
    const airwayInput = document.getElementById('stage-corneal-airway-input');

    if (parklandInput && wound.parkland_crystalloid_schedule) {
      parklandInput.value = wound.parkland_crystalloid_schedule;
    }
    if (ssdInput && wound.silver_sulfadiazine_honey_wound_plan) {
      ssdInput.value = wound.silver_sulfadiazine_honey_wound_plan;
    }
    if (escharInput && wound.escharotomy_wet_debridement_plan) {
      escharInput.value = wound.escharotomy_wet_debridement_plan;
    }
    if (analgesiaInput && wound.multimodal_burn_analgesia_cri) {
      analgesiaInput.value = wound.multimodal_burn_analgesia_cri;
    }
    if (albuminInput && wound.albumin_oncotic_24h_transition) {
      albuminInput.value = wound.albumin_oncotic_24h_transition;
    }
    if (airwayInput && wound.corneal_thermal_airway_protection) {
      airwayInput.value = wound.corneal_thermal_airway_protection;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('burn-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'GRANULATION_BED_NORMOALBUMINEMIC_CLEARED'
    ) {
      const by = active.discharged_by || 'Attending Burn Surgeon';
      const summary =
        active.discharge_summary ||
        'Healthy granulation bed and normoalbuminemic profile verified.';
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
      const weightStr = Number(active.weight_kg ?? 20.0).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Canine'} · ${active.breed || 'Mixed Breed'} · ${weightStr} kg)`;
    }

    const depthEl = document.getElementById('active-depth-class');
    if (depthEl) {
      depthEl.textContent = active.depth_class || '';
    }
    const parklandTotalEl = document.getElementById('active-parkland-total');
    if (parklandTotalEl) {
      parklandTotalEl.textContent = `${Number(active.total_24h_parkland_ml ?? 0).toFixed(1)} mL`;
    }
    const first8hEl = document.getElementById('active-first8h-rate');
    if (first8hEl) {
      first8hEl.textContent = `${Number(active.first_8h_rate_ml_hr ?? 0).toFixed(1)} mL/hr`;
    }
    const next16hEl = document.getElementById('active-next16h-rate');
    if (next16hEl) {
      next16hEl.textContent = `${Number(active.next_16h_rate_ml_hr ?? 0).toFixed(1)} mL/hr`;
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent = active.status || 'ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE';
      badgeEl.className = `burn-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const tbsaEl = document.getElementById('active-tbsa-percent');
    if (tbsaEl) {
      tbsaEl.textContent = `${Number(active.tbsa_percent ?? 0).toFixed(1)}% TBSA`;
    }
    const rateEl = document.getElementById('active-fluid-rate');
    if (rateEl) {
      rateEl.textContent = `${Number(active.current_fluid_rate_ml_hr ?? 0).toFixed(1)} mL/hr`;
    }
    const uopEl = document.getElementById('active-urine-output');
    if (uopEl) {
      uopEl.textContent = `${Number(active.urine_output_ml_kg_hr ?? 0).toFixed(1)} mL/kg/hr`;
    }
    const albPcvEl = document.getElementById('active-albumin-pcv');
    if (albPcvEl) {
      const alb = Number(active.serum_albumin_g_dl ?? 0).toFixed(1);
      const pcv = Number(active.pcv_percent ?? 0).toFixed(1);
      albPcvEl.textContent = `${alb} g/dL / ${pcv}%`;
    }

    const park = active.parkland_plan || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-burn-depth-patho', park.burn_depth_pathophysiology || '');
    setText('plan-parkland-summary', park.parkland_24h_calculation_summary || '');
    setText('plan-8h-16h-split', park.first_8h_vs_next_16h_split_note || '');
    setText('plan-escharotomy-alert', park.circumferential_escharotomy_alert || '');
    setText('plan-colloid-warning', park.colloid_24h_capillary_leak_warning || '');
    setText('plan-steroid-abx-warning', park.systemic_steroid_prophylactic_abx_warning || '');

    const wound = active.wound_plan || {};
    setText('plan-parkland-schedule', wound.parkland_crystalloid_schedule || '');
    setText('plan-ssd-honey', wound.silver_sulfadiazine_honey_wound_plan || '');
    setText('plan-escharotomy-debridement', wound.escharotomy_wet_debridement_plan || '');
    setText('plan-analgesia-cri', wound.multimodal_burn_analgesia_cri || '');
    setText('plan-albumin-transition', wound.albumin_oncotic_24h_transition || '');
    setText('plan-corneal-airway', wound.corneal_thermal_airway_protection || '');

    const checks = Array.isArray(active.burn_checks) ? active.burn_checks : [];
    const checksCountEl = document.getElementById('burn-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('burn-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const tbsa = Number(chk.tbsa_percent ?? 0).toFixed(1);
          const rate = Number(chk.current_fluid_rate_ml_hr ?? 0).toFixed(1);
          const uop = Number(chk.urine_output_ml_kg_hr ?? 0).toFixed(1);
          const alb = Number(chk.serum_albumin_g_dl ?? 0).toFixed(1);
          const pcv = Number(chk.pcv_percent ?? 0).toFixed(1);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="burn-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · ${tbsa}% TBSA · Rate ${rate} mL/hr · UOP ${uop} mL/kg/hr · Albumin ${alb} g/dL · PCV ${pcv}% — ${note}</li>`;
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
    const weightKg = Number(op.weight_kg ?? 20.0).toFixed(1);
    const status = escapeHtml(op.status || 'ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE');
    const badgeClass = getStatusBadgeClass(op.status);
    const depth = escapeHtml(op.depth_class || '');
    const tbsa = Number(op.tbsa_percent ?? 0).toFixed(1);
    const total24h = Number(op.total_24h_parkland_ml ?? 0).toFixed(1);
    const first8h = Number(op.first_8h_rate_ml_hr ?? 0).toFixed(1);
    const curRate = Number(op.current_fluid_rate_ml_hr ?? 0).toFixed(1);
    const uop = Number(op.urine_output_ml_kg_hr ?? 0).toFixed(1);
    const alb = Number(op.serum_albumin_g_dl ?? 0).toFixed(1);
    const pcv = Number(op.pcv_percent ?? 0).toFixed(1);
    const escharText = op.circumferential_thoracic_eschar ? 'Eschar+' : 'Eschar-';
    const sootText = op.singed_vibrissae_soot_airway ? 'Soot+' : 'Soot-';

    return `
      <article
        class="trust-screening-card burn-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-depth-class="${depth}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="burn-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="burn-card-metrics">
          <span><strong>Depth Class:</strong> ${depth}</span>
          <span><strong>TBSA%:</strong> ${tbsa}%</span>
          <span><strong>24h Parkland:</strong> ${total24h} mL (8h: ${first8h} mL/hr)</span>
          <span><strong>Current Rate:</strong> ${curRate} mL/hr</span>
          <span><strong>Urine Output:</strong> ${uop} mL/kg/hr</span>
          <span><strong>Albumin / PCV:</strong> ${alb} g/dL / ${pcv}%</span>
          <span><strong>Eschar / Airway:</strong> ${escharText} / ${sootText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary burn-select-op-btn" data-op-id="${opId}">
            🔍 Open Burn Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('burn-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('burn-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="burn-ops-empty-msg">No Thermal Burn &amp; Parkland operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (document.getElementById('burn-status-filter')?.value || '').trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/thermal-burn-parkland/operations?${qs}`
        : '/api/thermal-burn-parkland/operations';

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
      const res = await fetch(`/api/thermal-burn-parkland/operations/${encodeURIComponent(opId)}`, {
        headers: { Accept: 'application/json' },
      });
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
        statusBadge.className = 'burn-status-badge burn-status-cleared';
      }
      eventSource = new EventSource('/api/thermal-burn-parkland/stream');
      eventSource.addEventListener('thermal_burn_parkland_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('burn-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-burn-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (document.getElementById('create-case-id-input')?.value || 'CASE-BURN-104').trim();
        const petName = (document.getElementById('create-pet-name-input')?.value || '').trim();
        const species = (document.getElementById('create-species-input')?.value || 'Canine').trim();
        const breed = (document.getElementById('create-breed-input')?.value || 'Belgian Malinois').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-input')?.value || '25.0');
        const depthClass = (
          document.getElementById('create-depth-class-input')?.value ||
          'FULL_THICKNESS_CIRCUMFERENTIAL_ESCHAR'
        ).trim();
        const tbsa = parseFloat(document.getElementById('create-tbsa-input')?.value || '28.0');
        const fluidRate = parseFloat(document.getElementById('create-fluid-rate-input')?.value || '175.0');
        const uop = parseFloat(document.getElementById('create-uop-input')?.value || '0.9');
        const albumin = parseFloat(document.getElementById('create-albumin-input')?.value || '1.8');
        const pcv = parseFloat(document.getElementById('create-pcv-input')?.value || '56.0');
        const eschar = Boolean(document.getElementById('create-eschar-checkbox')?.checked);
        const airway = Boolean(document.getElementById('create-airway-checkbox')?.checked);

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 25.0 : weightKg,
          depth_class: depthClass,
          tbsa_percent: isNaN(tbsa) ? 28.0 : tbsa,
          current_fluid_rate_ml_hr: isNaN(fluidRate) ? 175.0 : fluidRate,
          urine_output_ml_kg_hr: isNaN(uop) ? 0.9 : uop,
          serum_albumin_g_dl: isNaN(albumin) ? 1.8 : albumin,
          pcv_percent: isNaN(pcv) ? 56.0 : pcv,
          circumferential_thoracic_eschar: eschar,
          singed_vibrissae_soot_airway: airway,
        };

        const btn = document.getElementById('create-burn-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/thermal-burn-parkland/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(`Admitted Thermal Burn patient ${created.pet_name} (${created.id})`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to admit Thermal Burn patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-parkland-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const parkland = (
          document.getElementById('stage-parkland-schedule-input')?.value ||
          'Isotonic Buffered Crystalloid (LRS): 50% in First 8h from time of thermal injury, 50% over Next 16h; withhold colloids in first 24h'
        ).trim();
        const ssd = (
          document.getElementById('stage-ssd-honey-input')?.value ||
          'Topical 1% Silver Sulfadiazine (SSD) + sterile Manuka Honey UMF 15+ barrier q12h'
        ).trim();
        const eschar = (
          document.getElementById('stage-escharotomy-input')?.value ||
          'Immediate longitudinal lateral thoracic escharotomy incisions through full-thickness leathery eschar + staged debridement'
        ).trim();
        const analgesia = (
          document.getElementById('stage-analgesia-cri-input')?.value || ''
        ).trim();
        const albumin = (
          document.getElementById('stage-albumin-transition-input')?.value || ''
        ).trim();
        const airway = (
          document.getElementById('stage-corneal-airway-input')?.value || ''
        ).trim();

        const payload = {
          parkland_crystalloid_schedule: parkland,
          silver_sulfadiazine_honey_wound_plan: ssd,
          escharotomy_wet_debridement_plan: eschar,
          multimodal_burn_analgesia_cri: analgesia,
          albumin_oncotic_24h_transition: albumin,
          corneal_thermal_airway_protection: airway,
        };

        const btn = document.getElementById('stage-parkland-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/thermal-burn-parkland/operations/${encodeURIComponent(selectedOpId)}/stage-parkland`,
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
              `Staged Parkland 8h/16h, SSD & Escharotomy protocol for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage Parkland protocol';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-burn-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const tbsa = parseFloat(
          document.getElementById('check-tbsa-input')?.value || '18.0'
        );
        const fluidRate = parseFloat(
          document.getElementById('check-fluid-rate-input')?.value || '75.0'
        );
        const uop = parseFloat(
          document.getElementById('check-uop-input')?.value || '1.7'
        );
        const albumin = parseFloat(
          document.getElementById('check-albumin-input')?.value || '2.5'
        );
        const pcv = parseFloat(
          document.getElementById('check-pcv-input')?.value || '43.0'
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Marcus Thorne, DACVS / Burn Reconstruction'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          tbsa_percent: isNaN(tbsa) ? 18.0 : tbsa,
          current_fluid_rate_ml_hr: isNaN(fluidRate) ? 75.0 : fluidRate,
          urine_output_ml_kg_hr: isNaN(uop) ? 1.7 : uop,
          serum_albumin_g_dl: isNaN(albumin) ? 2.5 : albumin,
          pcv_percent: isNaN(pcv) ? 43.0 : pcv,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-burn-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/thermal-burn-parkland/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
            showToast(`Recorded burn fluid & wound check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record burn check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-burn-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dischargedBy = (
          document.getElementById('clear-discharged-by-input')?.value ||
          'Dr. Marcus Thorne, DACVS / Burn Reconstruction'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          discharged_by: dischargedBy,
          discharge_summary: summary,
        };

        const btn = document.getElementById('clear-burn-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/thermal-burn-parkland/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Granulation / Reconstructive Step-Down`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to clear burn operation for discharge';
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
    const gridEl = document.getElementById('burn-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.burn-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.burn-op-card');
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
