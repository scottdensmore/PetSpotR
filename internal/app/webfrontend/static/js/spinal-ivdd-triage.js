/**
 * PetSpotR Acute Spinal Cord Injury, Intervertebral Disc Extrusion (IVDD),
 * Deep Pain Nociception, Schiff-Sherrington Posture & Hemilaminectomy Cockpit Controller (Milestone 22.3)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'IVDD-OP-101';
  let sseSource = null;

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
    const liveStatus = document.getElementById('ivdd-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('ivdd-error-banner');
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
      case 'ACUTE_PARAPLEGIA_NOCICEPTION_TRIAGE':
        return 'ivdd-status-critical';
      case 'CT_MYELOGRAM_HEMILAMINECTOMY_STAGED':
        return 'ivdd-status-staged';
      case 'POSTOP_NOCICEPTION_INTACT_BLADDER_CLEARED':
        return 'ivdd-status-cleared';
      default:
        return 'ivdd-status-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const triageEl = document.getElementById('kpi-acute-triage');
    const stagedEl = document.getElementById('kpi-staged-hemi');
    const clearedEl = document.getElementById('kpi-cleared-postop');
    const deepPainAbsentEl = document.getElementById('kpi-deep-pain-absent');
    const schiffEl = document.getElementById('kpi-schiff-sherrington');

    const total = metrics?.total_operations ?? ops.length;
    const triage =
      metrics?.acute_paraplegia_triage_count ??
      ops.filter((op) => op.status === 'ACUTE_PARAPLEGIA_NOCICEPTION_TRIAGE').length;
    const staged =
      metrics?.hemilaminectomy_staged_count ??
      ops.filter((op) => op.status === 'CT_MYELOGRAM_HEMILAMINECTOMY_STAGED').length;
    const cleared =
      metrics?.cleared_postop_count ??
      ops.filter((op) => op.status === 'POSTOP_NOCICEPTION_INTACT_BLADDER_CLEARED').length;
    const deepPainAbsent =
      metrics?.deep_pain_absent_emergency_count ??
      ops.filter(
        (op) =>
          !op.deep_pain_present || op.frankel_grade === 'GRADE_5_PARAPLEGIA_DEEP_PAIN_ABSENT'
      ).length;
    const schiff =
      metrics?.schiff_sherrington_count ??
      ops.filter((op) => Boolean(op.schiff_sherrington_present)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (triageEl) triageEl.textContent = String(triage);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (deepPainAbsentEl) deepPainAbsentEl.textContent = String(deepPainAbsent);
    if (schiffEl) schiffEl.textContent = String(schiff);
  }

  function syncFormsWithOperation(op) {
    if (!op) return;
    const decomp = op.decompression_plan || {};
    const winEl = document.getElementById('stage-window-input');
    const imgEl = document.getElementById('stage-imaging-input');
    const siteEl = document.getElementById('stage-site-input');
    const bladderEl = document.getElementById('stage-bladder-input');
    const analgesiaEl = document.getElementById('stage-analgesia-input');
    const crateEl = document.getElementById('stage-crate-rest-input');

    if (winEl && decomp.surgical_window_target) winEl.value = decomp.surgical_window_target;
    if (imgEl && decomp.imaging_modality) imgEl.value = decomp.imaging_modality;
    if (siteEl && decomp.hemilaminectomy_site) siteEl.value = decomp.hemilaminectomy_site;
    if (bladderEl && decomp.bladder_management_protocol) {
      bladderEl.value = decomp.bladder_management_protocol;
    }
    if (analgesiaEl && decomp.analgesia_cri_protocol) {
      analgesiaEl.value = decomp.analgesia_cri_protocol;
    }
    if (crateEl && decomp.strict_crate_rest_protocol) {
      crateEl.value = decomp.strict_crate_rest_protocol;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('ivdd-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (active && active.status === 'POSTOP_NOCICEPTION_INTACT_BLADDER_CLEARED') {
      const by = active.discharged_by || 'Dr. Elena Rostova, DACVIM (Neurology)';
      const summary =
        active.discharge_summary ||
        'Cleared for strict 4–6 week confined crate rest and neuro-rehabilitation.';
      if (summaryTextEl) {
        summaryTextEl.textContent = `${active.id} (${active.pet_name}) — Discharged by ${by}: ${summary}`;
      }
      clearanceBanner.hidden = false;
    } else {
      clearanceBanner.hidden = true;
    }
  }

  function renderActiveWorkspace(ops) {
    const workspaceEl = document.getElementById('ivdd-active-workspace');
    if (!workspaceEl) return;

    if (!ops || ops.length === 0) {
      workspaceEl.innerHTML =
        '<p class="trust-empty-hint">No active Spinal Cord Injury &amp; IVDD operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncFormsWithOperation(active);
    updateClearanceBanner(active);

    const noci = active.nociception_plan || {};
    const decomp = active.decompression_plan || {};
    const checks = Array.isArray(active.neuro_checks) ? active.neuro_checks : [];

    const opId = escapeHtml(active.id || '');
    const caseId = escapeHtml(active.case_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const breed = escapeHtml(active.breed || 'Canine');
    const weightKg = Number(active.weight_kg ?? 12.0).toFixed(1);
    const status = escapeHtml(active.status || 'ACUTE_PARAPLEGIA_NOCICEPTION_TRIAGE');
    const statusClass = getStatusBadgeClass(active.status);
    const frankel = escapeHtml(active.frankel_grade || 'GRADE_5_PARAPLEGIA_DEEP_PAIN_ABSENT');
    const zone = escapeHtml(active.localization_zone || 'T3_L3_THORACOLUMBAR_UMN');
    const trunci = escapeHtml(active.cutaneous_trunci_vertebra || 'L1');
    const deepPainText = active.deep_pain_present ? 'INTACT' : 'ABSENT (EMERGENCY)';
    const schiffText = active.schiff_sherrington_present ? 'PRESENT' : 'ABSENT';
    const targetMap = Number(active.target_map_mmhg ?? 85);
    const priorMeds = escapeHtml(active.prior_steroid_or_nsaid || 'None');

    const checksHtml = checks
      .map((chk) => {
        const cId = escapeHtml(chk.id || 'NC');
        const cBy = escapeHtml(chk.recorded_by || 'Neurology ICU');
        const cGrade = escapeHtml(chk.frankel_grade || '');
        const cDp = escapeHtml(chk.deep_pain_status || '');
        const cTrunci = escapeHtml(chk.cutaneous_trunci_level || '');
        const cBladder = escapeHtml(chk.bladder_tone_status || '');
        const cNote = escapeHtml(chk.clinical_note || '');
        return `<li class="ivdd-check-item"><strong>${cId} · ${cBy}</strong> · ${cGrade} · Deep Pain: ${cDp} · Trunci: ${cTrunci} · Bladder: ${cBladder} — ${cNote}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="ivdd-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name" id="active-op-title">${petName} (${breed} · ${weightKg} kg · ${opId})</strong>
            <code class="trust-pet-id" id="active-op-case-id">${opId} · ${caseId} · ${zone}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="ivdd-status-badge ${statusClass}" id="active-op-status-badge">${status}</span>
          </div>
        </div>

        <div class="ivdd-vitals-summary" id="workspace-vitals-summary">
          <span><strong>Frankel Grade:</strong> <span id="active-frankel-grade">${frankel}</span></span>
          <span><strong>Cutaneous Trunci Cutoff:</strong> <span id="active-cutaneous-trunci">${trunci}</span></span>
          <span><strong>Deep Pain:</strong> <span id="active-deep-pain">${deepPainText}</span></span>
          <span><strong>Schiff-Sherrington:</strong> <span id="active-schiff-sherrington">${schiffText}</span></span>
          <span><strong>Target MAP:</strong> <span id="active-target-map">&ge; ${targetMap} mmHg</span></span>
          <span><strong>Prior Meds:</strong> <span id="active-prior-meds">${priorMeds}</span></span>
        </div>

        <div class="ivdd-plan-box" id="workspace-nociception-box">
          <h4>Nociception, Withdrawal Reflex Differentiation, Myelomalacia &amp; Steroid/NSAID Washout Assessment</h4>
          <div class="ivdd-doses-grid">
            <div><strong>Deep Pain Perception:</strong> <span id="plan-deep-pain">${escapeHtml(noci.deep_pain_perception || '')}</span></div>
            <div><strong>Withdrawal Reflex Note:</strong> <span id="plan-withdrawal-note">${escapeHtml(noci.withdrawal_reflex_note || '')}</span></div>
            <div><strong>Schiff-Sherrington Posture:</strong> <span id="plan-schiff-note">${escapeHtml(noci.schiff_sherrington_posture || '')}</span></div>
            <div><strong>Cutaneous Trunci Cutoff:</strong> <span id="plan-trunci-cutoff">${escapeHtml(noci.cutaneous_trunci_cutoff || '')}</span></div>
            <div><strong>Myelomalacia Risk Guard:</strong> <span id="plan-myelomalacia-warning">${escapeHtml(noci.myelomalacia_risk_warning || '')}</span></div>
            <div><strong>Steroid/NSAID Washout Alert:</strong> <span id="plan-washout-alert">${escapeHtml(noci.steroid_nsaid_washout_alert || '')}</span></div>
          </div>
        </div>

        <div class="ivdd-plan-box" id="workspace-decompression-box">
          <h4>Staged 3D CT/MRI, Hemilaminectomy, Neurogenic Bladder &amp; Crate Rest Plan</h4>
          <div class="ivdd-doses-grid">
            <div><strong>Surgical Window Target:</strong> <span id="plan-surgical-window">${escapeHtml(decomp.surgical_window_target || '')}</span></div>
            <div><strong>Imaging Modality:</strong> <span id="plan-imaging-modality">${escapeHtml(decomp.imaging_modality || '')}</span></div>
            <div><strong>Hemilaminectomy Site:</strong> <span id="plan-hemilaminectomy-site">${escapeHtml(decomp.hemilaminectomy_site || '')}</span></div>
            <div><strong>Bladder Management:</strong> <span id="plan-bladder-protocol">${escapeHtml(decomp.bladder_management_protocol || '')}</span></div>
            <div><strong>Analgesia CRI Protocol:</strong> <span id="plan-analgesia-protocol">${escapeHtml(decomp.analgesia_cri_protocol || '')}</span></div>
            <div><strong>Strict Crate Rest:</strong> <span id="plan-crate-rest">${escapeHtml(decomp.strict_crate_rest_protocol || '')}</span></div>
          </div>
        </div>

        <div class="ivdd-neuro-checks-box">
          <h4>Serial Nociception, Cutaneous Trunci &amp; Bladder Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="neuro-checks-list" class="ivdd-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const opId = escapeHtml(op.id || '');
    const caseId = escapeHtml(op.case_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const breed = escapeHtml(op.breed || 'Canine');
    const weightKg = Number(op.weight_kg ?? 12.0).toFixed(1);
    const status = escapeHtml(op.status || 'ACUTE_PARAPLEGIA_NOCICEPTION_TRIAGE');
    const statusClass = getStatusBadgeClass(op.status);
    const frankel = escapeHtml(op.frankel_grade || '');
    const zone = escapeHtml(op.localization_zone || '');
    const trunci = escapeHtml(op.cutaneous_trunci_vertebra || '');
    const deepPainYesNo = op.deep_pain_present ? 'INTACT' : 'ABSENT';
    const schiffYesNo = op.schiff_sherrington_present ? 'YES' : 'NO';
    const priorMeds = escapeHtml(op.prior_steroid_or_nsaid || '');

    const alertHtml =
      !op.deep_pain_present || op.schiff_sherrington_present
        ? `
          <div class="ivdd-alert-banner">
            ⚠️ GRADE 5 NOCICEPTION &amp; WASHOUT GUARD (${opId} — ${petName}): Deep Pain ${op.deep_pain_present ? 'INTACT' : 'ABSENT (&lt;12h Window)'} · Cutaneous Trunci ${trunci} · ${priorMeds}
          </div>
        `
        : '';

    const clearanceHtml =
      op.status === 'POSTOP_NOCICEPTION_INTACT_BLADDER_CLEARED'
        ? `
          <div class="ivdd-clearance-banner">
            ✅ POST-OP NOCICEPTION &amp; BLADDER CLEARED (${escapeHtml(op.discharged_by || '')}): ${escapeHtml(op.discharge_summary || '')}
          </div>
        `
        : '';

    return `
      <article class="trust-screening-card ivdd-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-zone="${zone}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId} · ${zone}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="ivdd-status-badge ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row ivdd-metrics-row">
          <span class="ivdd-status-badge ivdd-status-staged">Frankel: ${frankel} • Cutaneous Trunci: ${trunci} • Deep Pain: ${deepPainYesNo} • Schiff-Sherrington: ${schiffYesNo}</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary ivdd-select-op-btn"
            data-op-id="${opId}"
          >
            🔍 Select Operation (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateSummaryStats(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('ivdd-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('ivdd-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="ivdd-ops-empty-msg">No Spinal Cord Injury &amp; IVDD operations match the current filters.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (document.getElementById('ivdd-status-filter')?.value || '').trim();
      const zoneFilter = (document.getElementById('ivdd-zone-filter')?.value || '').trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);
      if (zoneFilter) params.set('localization_zone', zoneFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/spinal-ivdd/operations?${qs}`
        : '/api/spinal-ivdd/operations';

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
      const res = await fetch(`/api/spinal-ivdd/operations/${encodeURIComponent(opId)}`, {
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
    const btn = document.getElementById('connect-sse-btn');
    const badge = document.getElementById('sse-status-badge');
    if (!btn) return;

    btn.addEventListener('click', () => {
      if (sseSource) {
        sseSource.close();
        sseSource = null;
      }
      if (badge) {
        badge.textContent = 'SSE Connecting...';
        badge.className = 'ivdd-status-badge ivdd-status-staged';
      }
      const es = new EventSource('/api/spinal-ivdd/stream');
      sseSource = es;

      es.onopen = () => {
        if (badge) {
          badge.textContent = 'SSE Connected (Live)';
          badge.className = 'ivdd-status-badge ivdd-status-cleared';
        }
      };

      es.addEventListener('spinal_ivdd_updated', () => {
        fetchOperations();
      });

      es.onerror = () => {
        if (badge) {
          badge.textContent = 'SSE Reconnecting';
          badge.className = 'ivdd-status-badge ivdd-status-critical';
        }
      };
    });
  }

  function setupFormsAndFilters() {
    const createForm = document.getElementById('create-ivdd-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (document.getElementById('create-case-id-input')?.value || '').trim();
        const petName = (document.getElementById('create-pet-name-input')?.value || '').trim();
        const species = (document.getElementById('create-species-input')?.value || 'Canine').trim();
        const breed = (document.getElementById('create-breed-input')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-input')?.value || '0');
        const onsetHours = parseFloat(document.getElementById('create-onset-input')?.value || '0');
        const frankelGrade = (
          document.getElementById('create-frankel-input')?.value ||
          'GRADE_5_PARAPLEGIA_DEEP_PAIN_ABSENT'
        ).trim();
        const localizationZone = (
          document.getElementById('create-zone-input')?.value || 'T3_L3_THORACOLUMBAR_UMN'
        ).trim();
        const trunci = (document.getElementById('create-trunci-input')?.value || 'L1').trim();
        const priorMed = (document.getElementById('create-prior-med-input')?.value || '').trim();
        const deepPain = Boolean(document.getElementById('create-deep-pain-input')?.checked);
        const schiff = Boolean(document.getElementById('create-schiff-input')?.checked);

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 0 : weightKg,
          onset_hours: isNaN(onsetHours) ? 0 : onsetHours,
          frankel_grade: frankelGrade,
          localization_zone: localizationZone,
          cutaneous_trunci_vertebra: trunci,
          prior_steroid_or_nsaid: priorMed,
          deep_pain_present: deepPain,
          schiff_sherrington_present: schiff,
          surgical_window_hours:
            frankelGrade === 'GRADE_5_PARAPLEGIA_DEEP_PAIN_ABSENT' ? 12.0 : 24.0,
          target_map_mmhg: 85,
        };

        const btn = document.getElementById('create-ivdd-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/spinal-ivdd/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Created Spinal IVDD operation ${created.id} for ${created.pet_name}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create Spinal IVDD operation';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-decompression-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const payload = {
          surgical_window_target: (
            document.getElementById('stage-window-input')?.value || ''
          ).trim(),
          imaging_modality: (document.getElementById('stage-imaging-input')?.value || '').trim(),
          hemilaminectomy_site: (document.getElementById('stage-site-input')?.value || '').trim(),
          bladder_management_protocol: (
            document.getElementById('stage-bladder-input')?.value || ''
          ).trim(),
          analgesia_cri_protocol: (
            document.getElementById('stage-analgesia-input')?.value || ''
          ).trim(),
          strict_crate_rest_protocol: (
            document.getElementById('stage-crate-rest-input')?.value || ''
          ).trim(),
        };

        const btn = document.getElementById('stage-decompression-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/spinal-ivdd/operations/${encodeURIComponent(selectedOpId)}/stage-decompression`,
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
              `Staged CT/MRI & Hemilaminectomy decompression for ${updated.id} → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage decompression plan';
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

        const payload = {
          frankel_grade: (
            document.getElementById('neuro-frankel-input')?.value ||
            'GRADE_3_NON_AMBULATORY_PARAPARESIS'
          ).trim(),
          deep_pain_status: (document.getElementById('neuro-deep-pain-input')?.value || '').trim(),
          cutaneous_trunci_level: (
            document.getElementById('neuro-trunci-input')?.value || ''
          ).trim(),
          bladder_tone_status: (
            document.getElementById('neuro-bladder-input')?.value || ''
          ).trim(),
          recorded_by: (document.getElementById('neuro-recorded-by-input')?.value || '').trim(),
          clinical_note: (document.getElementById('neuro-note-input')?.value || '').trim(),
        };

        const btn = document.getElementById('record-neuro-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/spinal-ivdd/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
            showToast(`Recorded nociception & cutaneous trunci check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record neurological check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-ivdd-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const payload = {
          discharged_by: (
            document.getElementById('clear-discharged-by-input')?.value || ''
          ).trim(),
          discharge_summary: (document.getElementById('clear-summary-input')?.value || '').trim(),
        };

        const btn = document.getElementById('clear-ivdd-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/spinal-ivdd/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
              `Cleared post-op nociception & bladder discharge for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to clear neurology discharge';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const filterStatus = document.getElementById('ivdd-status-filter');
    if (filterStatus) {
      filterStatus.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const filterZone = document.getElementById('ivdd-zone-filter');
    if (filterZone) {
      filterZone.addEventListener('change', () => {
        fetchOperations();
      });
    }
  }

  function setupCardInteractions() {
    const gridEl = document.getElementById('ivdd-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.ivdd-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.ivdd-op-card');
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
