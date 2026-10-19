/**
 * PetSpotR Acute Hepatic Failure, Hepatic Encephalopathy, Sago Palm (Cycad) /
 * Amanita / Blue-Green Algae (Microcystin) Hepatotoxicity, N-Acetylcysteine (NAC)
 * & Lactulose/Levetiracetam Cockpit Controller (Milestone 23.2)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'HEP-OP-101';
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
    const liveStatus = document.getElementById('hepatic-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('hepatic-error-banner');
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
      case 'ACUTE_FULMINANT_HEPATOTOXICITY_HE_TRIAGE':
        return 'hepatic-status-critical';
      case 'NAC_LACTULOSE_VITAMIN_K1_STAGED':
        return 'hepatic-status-staged';
      case 'AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED':
        return 'hepatic-status-cleared';
      default:
        return 'hepatic-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const fulminantEl = document.getElementById('kpi-fulminant-triage');
    const stagedEl = document.getElementById('kpi-nac-staged');
    const clearedEl = document.getElementById('kpi-cleared-hepatic');
    const sagoEl = document.getElementById('kpi-sago-microcystin');
    const heEl = document.getElementById('kpi-hyperammonemic-he');

    const total = metrics?.total_operations ?? ops.length;
    const fulminant =
      metrics?.fulminant_he_triage_count ??
      ops.filter((op) => op.status === 'ACUTE_FULMINANT_HEPATOTOXICITY_HE_TRIAGE').length;
    const staged =
      metrics?.nac_lactulose_staged_count ??
      ops.filter((op) => op.status === 'NAC_LACTULOSE_VITAMIN_K1_STAGED').length;
    const cleared =
      metrics?.cleared_hepatic_count ??
      ops.filter((op) => op.status === 'AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED').length;
    const sago =
      metrics?.sago_palm_microcystin_count ??
      ops.filter(
        (op) =>
          op.etiology_type === 'SAGO_PALM_CYCASIN_HEPATOTOXICITY' ||
          op.etiology_type === 'BLUE_GREEN_ALGAE_MICROCYSTIN_AMANITA'
      ).length;
    const he =
      metrics?.hyperammonemic_encephalopathy_count ??
      ops.filter(
        (op) =>
          Number(op.blood_ammonia_umol_l ?? 0) >= 70.0 ||
          Number(op.encephalopathy_grade ?? 0) >= 2 ||
          Boolean(op.head_pressing_seizures_present)
      ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (fulminantEl) fulminantEl.textContent = String(fulminant);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (sagoEl) sagoEl.textContent = String(sago);
    if (heEl) heEl.textContent = String(he);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const sup = op.support_plan || {};
    const nacInput = document.getElementById('stage-nac-protocol-input');
    const sameInput = document.getElementById('stage-same-silybin-input');
    const lactInput = document.getElementById('stage-lactulose-protocol-input');
    const levInput = document.getElementById('stage-levetiracetam-protocol-input');
    const dexInput = document.getElementById('stage-dextrose-vitk1-input');
    const elecInput = document.getElementById('stage-electrolyte-thiamine-input');

    if (nacInput && sup.nac_glutathione_loading_protocol) {
      nacInput.value = sup.nac_glutathione_loading_protocol;
    }
    if (sameInput && sup.same_silybin_hepatoprotectant) {
      sameInput.value = sup.same_silybin_hepatoprotectant;
    }
    if (lactInput && sup.lactulose_enema_oral_protocol) {
      lactInput.value = sup.lactulose_enema_oral_protocol;
    }
    if (levInput && sup.levetiracetam_neuro_protocol) {
      levInput.value = sup.levetiracetam_neuro_protocol;
    }
    if (dexInput && sup.dextrose_vitamin_k1_ffp_plan) {
      dexInput.value = sup.dextrose_vitamin_k1_ffp_plan;
    }
    if (elecInput && sup.potassium_phosphate_thiamine_plan) {
      elecInput.value = sup.potassium_phosphate_thiamine_plan;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('hepatic-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED'
    ) {
      const by = active.discharged_by || 'Attending Criticalist';
      const summary =
        active.discharge_summary ||
        'Blood ammonia normalized, hepatic encephalopathy resolved, and synthetic coagulation stable.';
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

    const etioEl = document.getElementById('active-etiology-type');
    if (etioEl) {
      etioEl.textContent = active.etiology_type || '';
    }
    const gradeEl = document.getElementById('active-he-grade');
    if (gradeEl) {
      gradeEl.textContent = `Grade ${Number(active.encephalopathy_grade ?? 0)}/4`;
    }
    const seizFlagEl = document.getElementById('active-seizures-flag');
    if (seizFlagEl) {
      seizFlagEl.textContent = active.head_pressing_seizures_present ? 'YES' : 'NO';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent = active.status || 'ACUTE_FULMINANT_HEPATOTOXICITY_HE_TRIAGE';
      badgeEl.className = `hepatic-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const ammoniaEl = document.getElementById('active-blood-ammonia');
    if (ammoniaEl) {
      ammoniaEl.textContent = `${Number(active.blood_ammonia_umol_l ?? 0).toFixed(1)} umol/L`;
    }
    const altEl = document.getElementById('active-alt-value');
    if (altEl) {
      altEl.textContent = `${Number(active.alt_iu_l ?? 0)} IU/L`;
    }
    const biliEl = document.getElementById('active-bilirubin-value');
    if (biliEl) {
      biliEl.textContent = `${Number(active.total_bilirubin_mg_dl ?? 0).toFixed(1)} mg/dL`;
    }
    const glucPtEl = document.getElementById('active-glucose-pt-value');
    if (glucPtEl) {
      const gluc = Number(active.blood_glucose_mg_dl ?? 0).toFixed(1);
      const pt = Number(active.pt_seconds ?? 0).toFixed(1);
      glucPtEl.textContent = `${gluc} mg/dL · ${pt} sec`;
    }

    const hePlan = active.encephalopathy_plan || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-etiology-pathophysiology', hePlan.etiology_pathophysiology || '');
    setText('plan-hyperammonemia-neuro', hePlan.hyperammonemia_neuro_status || '');
    setText('plan-hypoglycemia-coagulopathy', hePlan.hypoglycemia_coagulopathy_note || '');
    setText('plan-lactulose-trapping', hePlan.lactulose_ammonium_trapping_plan || '');
    setText('plan-benzo-methionine-warning', hePlan.benzodiazepine_methionine_warning || '');
    setText('plan-refeeding-electrolyte', hePlan.refeeding_electrolyte_alert || '');

    const sup = active.support_plan || {};
    setText('plan-nac-protocol', sup.nac_glutathione_loading_protocol || '');
    setText('plan-same-silybin', sup.same_silybin_hepatoprotectant || '');
    setText('plan-lactulose-protocol', sup.lactulose_enema_oral_protocol || '');
    setText('plan-levetiracetam-protocol', sup.levetiracetam_neuro_protocol || '');
    setText('plan-dextrose-vitk1-ffp', sup.dextrose_vitamin_k1_ffp_plan || '');
    setText('plan-potassium-phosphate-thiamine', sup.potassium_phosphate_thiamine_plan || '');

    const checks = Array.isArray(active.metabolic_checks) ? active.metabolic_checks : [];
    const checksCountEl = document.getElementById('metabolic-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('metabolic-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const amm = Number(chk.blood_ammonia_umol_l ?? 0).toFixed(1);
          const alt = Number(chk.alt_iu_l ?? 0);
          const bili = Number(chk.total_bilirubin_mg_dl ?? 0).toFixed(1);
          const gluc = Number(chk.blood_glucose_mg_dl ?? 0).toFixed(1);
          const pt = Number(chk.pt_seconds ?? 0).toFixed(1);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="hepatic-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · Ammonia ${amm} umol/L · ALT ${alt} IU/L · Bilirubin ${bili} mg/dL · Glucose ${gluc} mg/dL · PT ${pt} sec — ${note}</li>`;
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
    const status = escapeHtml(op.status || 'ACUTE_FULMINANT_HEPATOTOXICITY_HE_TRIAGE');
    const badgeClass = getStatusBadgeClass(op.status);
    const etiology = escapeHtml(op.etiology_type || '');
    const ammonia = Number(op.blood_ammonia_umol_l ?? 0).toFixed(1);
    const alt = Number(op.alt_iu_l ?? 0);
    const bili = Number(op.total_bilirubin_mg_dl ?? 0).toFixed(1);
    const gluc = Number(op.blood_glucose_mg_dl ?? 0).toFixed(1);
    const pt = Number(op.pt_seconds ?? 0).toFixed(1);
    const heGrade = Number(op.encephalopathy_grade ?? 0);
    const seizText = op.head_pressing_seizures_present
      ? 'YES (Levetiracetam 60 mg/kg IV)'
      : 'NO';

    return `
      <article
        class="trust-screening-card hepatic-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-etiology="${etiology}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="hepatic-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="hepatic-card-metrics">
          <span><strong>Etiology:</strong> ${etiology}</span>
          <span><strong>Ammonia:</strong> ${ammonia} umol/L</span>
          <span><strong>ALT:</strong> ${alt} IU/L</span>
          <span><strong>Bilirubin:</strong> ${bili} mg/dL</span>
          <span><strong>Glucose:</strong> ${gluc} mg/dL</span>
          <span><strong>PT:</strong> ${pt} sec</span>
          <span><strong>HE Grade:</strong> Grade ${heGrade}/4</span>
          <span><strong>Head Pressing / Seizures:</strong> ${seizText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary hepatic-select-op-btn" data-op-id="${opId}">
            🔍 Open Hepatic Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('hepatic-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('hepatic-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="hepatic-ops-empty-msg">No Hepatic Encephalopathy operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (document.getElementById('hepatic-status-filter')?.value || '').trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/hepatic-encephalopathy/operations?${qs}`
        : '/api/hepatic-encephalopathy/operations';

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
        `/api/hepatic-encephalopathy/operations/${encodeURIComponent(opId)}`,
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
        statusBadge.className = 'hepatic-status-badge hepatic-status-cleared';
      }
      eventSource = new EventSource('/api/hepatic-encephalopathy/stream');
      eventSource.addEventListener('hepatic_encephalopathy_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('hepatic-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-hepatic-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value || 'CASE-HEP-104'
        ).trim();
        const petName = (document.getElementById('create-pet-name-input')?.value || '').trim();
        const species = (
          document.getElementById('create-species-input')?.value || 'Canine'
        ).trim();
        const breed = (
          document.getElementById('create-breed-input')?.value || 'Golden Retriever'
        ).trim();
        const weightKg = parseFloat(document.getElementById('create-weight-input')?.value || '29.5');
        const etiologyType = (
          document.getElementById('create-etiology-input')?.value ||
          'SAGO_PALM_CYCASIN_HEPATOTOXICITY'
        ).trim();
        const altIul = parseInt(document.getElementById('create-alt-input')?.value || '3120', 10);
        const bilirubin = parseFloat(
          document.getElementById('create-bilirubin-input')?.value || '7.2'
        );
        const ammonia = parseFloat(
          document.getElementById('create-ammonia-input')?.value || '260.0'
        );
        const glucose = parseFloat(
          document.getElementById('create-glucose-input')?.value || '52.0'
        );
        const ptSeconds = parseFloat(document.getElementById('create-pt-input')?.value || '27.8');
        const heGrade = parseInt(
          document.getElementById('create-he-grade-input')?.value || '3',
          10
        );
        const seizuresPresent = Boolean(
          document.getElementById('create-seizures-checkbox')?.checked
        );

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 29.5 : weightKg,
          etiology_type: etiologyType,
          alt_iu_l: isNaN(altIul) ? 3120 : altIul,
          total_bilirubin_mg_dl: isNaN(bilirubin) ? 7.2 : bilirubin,
          blood_ammonia_umol_l: isNaN(ammonia) ? 260.0 : ammonia,
          blood_glucose_mg_dl: isNaN(glucose) ? 52.0 : glucose,
          pt_seconds: isNaN(ptSeconds) ? 27.8 : ptSeconds,
          encephalopathy_grade: isNaN(heGrade) ? 3 : heGrade,
          head_pressing_seizures_present: seizuresPresent,
        };

        const btn = document.getElementById('create-hepatic-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/hepatic-encephalopathy/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted Hepatic Failure / HE patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to admit Hepatic patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-support-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const nac = (
          document.getElementById('stage-nac-protocol-input')?.value ||
          'N-Acetylcysteine (NAC) 140 mg/kg IV loading dose diluted 1:4 via 0.2-micron filter over 20–30 min, followed by 70 mg/kg IV q6h × 7 doses'
        ).trim();
        const sameSilybin = (
          document.getElementById('stage-same-silybin-input')?.value ||
          'SAMe 20 mg/kg PO q24h + Silybin-Phosphatidylcholine 50 mg/kg IV/PO + Ursodiol 15 mg/kg PO q24h'
        ).trim();
        const lactulose = (
          document.getElementById('stage-lactulose-protocol-input')?.value ||
          'Lactulose 1:3 warm water retention enema (15 mL/kg retained 20–30 min q6h) + oral Lactulose 0.5 mL/kg PO q8h + Metronidazole 7.5 mg/kg IV q12h'
        ).trim();
        const levetiracetam = (
          document.getElementById('stage-levetiracetam-protocol-input')?.value ||
          'Levetiracetam 60 mg/kg IV loading dose, then 20 mg/kg IV q8h + 30° head elevation (NEVER administer Diazepam/Midazolam)'
        ).trim();
        const dexVitK = (
          document.getElementById('stage-dextrose-vitk1-input')?.value || ''
        ).trim();
        const electrolyte = (
          document.getElementById('stage-electrolyte-thiamine-input')?.value || ''
        ).trim();

        const payload = {
          nac_glutathione_loading_protocol: nac,
          same_silybin_hepatoprotectant: sameSilybin,
          lactulose_enema_oral_protocol: lactulose,
          levetiracetam_neuro_protocol: levetiracetam,
          dextrose_vitamin_k1_ffp_plan: dexVitK,
          potassium_phosphate_thiamine_plan: electrolyte,
        };

        const btn = document.getElementById('stage-support-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/hepatic-encephalopathy/operations/${encodeURIComponent(selectedOpId)}/stage-support`,
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
              `Staged NAC 140 mg/kg / Lactulose / Levetiracetam / Vitamin K1 protocol for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage NAC / Lactulose support protocol';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-metabolic-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const altIul = parseInt(
          document.getElementById('check-alt-input')?.value || '480',
          10
        );
        const bilirubin = parseFloat(
          document.getElementById('check-bilirubin-input')?.value || '1.6'
        );
        const ammonia = parseFloat(
          document.getElementById('check-ammonia-input')?.value || '42.0'
        );
        const glucose = parseFloat(
          document.getElementById('check-glucose-input')?.value || '96.0'
        );
        const ptSeconds = parseFloat(
          document.getElementById('check-pt-input')?.value || '12.8'
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Priya Nair, DACVECC'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          alt_iu_l: isNaN(altIul) ? 480 : altIul,
          total_bilirubin_mg_dl: isNaN(bilirubin) ? 1.6 : bilirubin,
          blood_ammonia_umol_l: isNaN(ammonia) ? 42.0 : ammonia,
          blood_glucose_mg_dl: isNaN(glucose) ? 96.0 : glucose,
          pt_seconds: isNaN(ptSeconds) ? 12.8 : ptSeconds,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-metabolic-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/hepatic-encephalopathy/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
            showToast(`Recorded hepatic metabolic & ammonia check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record hepatic metabolic check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-hepatic-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dischargedBy = (
          document.getElementById('clear-discharged-by-input')?.value ||
          'Dr. Priya Nair, DACVECC'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          discharged_by: dischargedBy,
          discharge_summary: summary,
        };

        const btn = document.getElementById('clear-hepatic-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/hepatic-encephalopathy/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Ammonia-Normalized Hepatic Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to clear Hepatic operation for discharge';
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
    const gridEl = document.getElementById('hepatic-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.hepatic-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.hepatic-op-card');
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
