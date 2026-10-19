/**
 * PetSpotR Immune-Mediated Hemolytic Anemia (IMHA) & Thrombocytopenia (IMTP),
 * Saline Agglutination, Spherocytosis & Thromboprophylaxis Triage Cockpit Controller (Milestone 20.5)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'IM-OP-101';

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
    const liveStatus = document.getElementById('imha-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('imha-error-banner');
    if (!banner) return;
    if (!message) {
      banner.textContent = '';
      banner.hidden = true;
      return;
    }
    banner.textContent = message;
    banner.hidden = false;
  }

  function getStatusPillClass(status) {
    switch (status) {
      case 'HEMOLYTIC_AGGLUTINATION_CRISIS_CRITICAL':
        return 'status-pill-critical';
      case 'PREDNISOLONE_CLOPIDOGREL_PRBC_STAGED':
        return 'status-pill-staged';
      case 'PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE':
        return 'status-pill-cleared';
      default:
        return 'status-pill-critical';
    }
  }

  function updateSummaryStats(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const criticalEl = document.getElementById('kpi-critical-hemolytic');
    const stagedEl = document.getElementById('kpi-staged-regimen');
    const clearedEl = document.getElementById('kpi-cleared-stable');
    const pteAlertsEl = document.getElementById('kpi-pte-alerts');

    const total = metrics?.total_operations ?? ops.length;
    const critical =
      metrics?.critical_hemolytic_count ??
      ops.filter((op) => op.status === 'HEMOLYTIC_AGGLUTINATION_CRISIS_CRITICAL').length;
    const staged =
      metrics?.staged_regimen_count ??
      ops.filter((op) => op.status === 'PREDNISOLONE_CLOPIDOGREL_PRBC_STAGED').length;
    const cleared =
      metrics?.cleared_stable_count ??
      ops.filter((op) => op.status === 'PCV_STABLE_RETICULOCYTOSIS_TAPER_SAFE').length;
    const alerts =
      metrics?.pte_alert_count ??
      ops.filter((op) => Boolean(op.pulmonary_thromboembolism_alert)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (pteAlertsEl) pteAlertsEl.textContent = String(alerts);
  }

  function syncStageFormWithOperation(op) {
    if (!op || !op.patient_profile) return;
    const p = op.patient_profile;
    const weightEl = document.getElementById('stage-weight-kg');
    const speciesEl = document.getElementById('stage-species');
    const pcvEl = document.getElementById('stage-pcv');
    const biliEl = document.getElementById('stage-bilirubin');
    const pltEl = document.getElementById('stage-platelets');
    const spheroEl = document.getElementById('stage-spherocytes');

    if (weightEl && p.weight_kg !== undefined) weightEl.value = String(p.weight_kg);
    if (speciesEl && p.species) speciesEl.value = p.species;
    if (pcvEl && p.packed_cell_volume_percent !== undefined) {
      pcvEl.value = String(p.packed_cell_volume_percent);
    }
    if (biliEl && p.total_bilirubin_mg_dl !== undefined) {
      biliEl.value = String(p.total_bilirubin_mg_dl);
    }
    if (pltEl && p.platelet_count_k_ul !== undefined) {
      pltEl.value = String(p.platelet_count_k_ul);
    }
    if (spheroEl && p.spherocytes_per_hpf !== undefined) {
      spheroEl.value = String(p.spherocytes_per_hpf);
    }
  }

  function updateGlobalBanners(active) {
    const alertBanner = document.getElementById('imha-alert-banner');
    const clearanceBanner = document.getElementById('imha-clearance-banner');

    if (alertBanner) {
      if (active) {
        const p = active.patient_profile || {};
        const reg = active.immunosuppressive_regimen || {};
        const pcv = Number(p.packed_cell_volume_percent ?? 12.0).toFixed(1);
        const bili = Number(p.total_bilirubin_mg_dl ?? 6.4).toFixed(1);
        const plt = Number(p.platelet_count_k_ul ?? 85).toFixed(0);
        const sphero = Number(p.spherocytes_per_hpf ?? 18);
        const pitfall =
          reg.imha_pitfall_alert ||
          'CRITICAL IMHA & PULMONARY THROMBOEMBOLISM PITFALL ALERT: NEVER omit dual thromboprophylaxis (Clopidogrel 2 mg/kg/day + Rivaroxaban 1 mg/kg/day) in hypercoagulable IMHA when platelets >= 30 K/µL (pulmonary thromboembolism is the #1 cause of acute mortality in canine IMHA); NEVER perform jugular venipuncture in hypercoagulable IMHA or severely thrombocytopenic Evans Syndrome (< 30 K/µL) patients; and NEVER mistake rouleaux for true autoagglutination without a 1:4 saline dilution wash.';
        alertBanner.textContent = `⚠️ PULMONARY THROMBOEMBOLISM (PTE) & ZERO-JUGULAR VENIPUNCTURE GUARD (${active.id} — ${active.pet_name} · PCV ${pcv}% · TBili ${bili} mg/dL · Platelets ${plt} K/µL · Spherocytes ${sphero}/HPF): ${pitfall}`;
        alertBanner.hidden = false;
      }
    }

    if (clearanceBanner) {
      if (active && active.discharge_clearance) {
        const clr = active.discharge_clearance;
        const pcv = Number(clr.final_pcv_stable_48_hr_percent ?? 28.5).toFixed(1);
        const retic = Number(clr.final_reticulocyte_count_k_ul ?? 138).toFixed(0);
        clearanceBanner.textContent = `✅ 48H STABLE PCV & REGENERATIVE RETICULOCYTOSIS DISCHARGE CLEARANCE VERIFIED (${active.id} — ${active.pet_name}): PCV ${pcv}% · Reticulocytes ${retic} K/µL · 1:4 Saline Agglutination Resolved · Vet License ${clr.vet_license} — ${clr.summary || ''}`;
        clearanceBanner.hidden = false;
      } else {
        clearanceBanner.hidden = true;
      }
    }
  }

  function renderActiveWorkspace(ops) {
    const workspaceEl = document.getElementById('active-operation-workspace');
    if (!workspaceEl) return;

    if (!ops || ops.length === 0) {
      workspaceEl.innerHTML =
        '<p class="trust-empty-hint">No active IMHA &amp; Thromboprophylaxis operation selected.</p>';
      return;
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    selectedOpId = active.id;
    syncStageFormWithOperation(active);
    updateGlobalBanners(active);

    const p = active.patient_profile || {};
    const reg = active.immunosuppressive_regimen;
    const checks = Array.isArray(active.hemolysis_checks) ? active.hemolysis_checks : [];

    const opId = escapeHtml(active.id || '');
    const incId = escapeHtml(active.incident_id || '');
    const petName = escapeHtml(active.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_PRIMARY_IMHA');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 11.5).toFixed(1);
    const status = escapeHtml(active.status || 'HEMOLYTIC_AGGLUTINATION_CRISIS_CRITICAL');
    const statusClass = getStatusPillClass(active.status);
    const pcv = Number(p.packed_cell_volume_percent ?? 12.0).toFixed(1);
    const bili = Number(p.total_bilirubin_mg_dl ?? 6.4).toFixed(1);
    const plt = Number(p.platelet_count_k_ul ?? 85).toFixed(0);
    const sphero = Number(p.spherocytes_per_hpf ?? 18);
    const salinePos = p.saline_agglutination_4_to_1_positive ? 'POSITIVE' : 'NEGATIVE';
    const hemoglobinuria = p.intravascular_hemoglobinuria ? 'PRESENT' : 'ABSENT';

    const regimenHtml = reg
      ? `
        <div class="imha-regimen-box" id="workspace-regimen-box">
          <h4>Calculated Immunosuppressive, Thromboprophylaxis &amp; Crossmatched pRBC Regimen</h4>
          <div class="imha-doses-grid">
            <div><strong>Prednisolone (2 mg/kg/day):</strong> <span id="regimen-prednisolone-mg">${Number(reg.prednisolone_2_mg_kg_day_mg ?? 0).toFixed(1)} mg/day</span></div>
            <div><strong>Mycophenolate (10 mg/kg BID):</strong> <span id="regimen-mycophenolate-mg">${Number(reg.mycophenolate_10_mg_kg_bid_mg ?? 0).toFixed(1)} mg BID</span></div>
            <div><strong>Clopidogrel (2 mg/kg/day):</strong> <span id="regimen-clopidogrel-mg">${Number(reg.clopidogrel_2_mg_kg_day_mg ?? 0).toFixed(1)} mg/day</span></div>
            <div><strong>Rivaroxaban (1 mg/kg/day):</strong> <span id="regimen-rivaroxaban-mg">${Number(reg.rivaroxaban_1_mg_kg_day_mg ?? 0).toFixed(1)} mg/day</span></div>
            <div><strong>Vincristine (0.02 mg/kg IV):</strong> <span id="regimen-vincristine-mg">${Number(reg.vincristine_002_mg_kg_imtp_mg ?? 0).toFixed(2)} mg</span></div>
            <div><strong>Crossmatched pRBC (15 mL/kg):</strong> <span id="regimen-prbc-ml">${Number(reg.packed_rbc_transfusion_ml ?? 0).toFixed(1)} mL</span></div>
            <div><strong>Zero-Jugular Venipuncture Guard:</strong> <span id="regimen-jugular-guard">${reg.zero_jugular_venipuncture_guard ? 'ENFORCED' : 'OFF'}</span></div>
          </div>
          <p class="imha-rationale-text" id="workspace-clinical-rationale">${escapeHtml(reg.clinical_rationale || '')}</p>
        </div>
      `
      : '';

    const checksHtml = checks
      .map((chk) => {
        const callsign = escapeHtml(chk.clinician_callsign || 'DR-HEME');
        const cPcv = Number(chk.packed_cell_volume_percent ?? 22.0).toFixed(1);
        const cRetic = Number(chk.reticulocyte_count_k_ul ?? 95).toFixed(0);
        const cBili = Number(chk.total_bilirubin_mg_dl ?? 1.8).toFixed(1);
        const cSaline = chk.saline_agglutination_persistent ? 'PERSISTENT' : 'RESOLVED';
        const cRr = Number(chk.resting_respiratory_rate_bpm ?? 24);
        const cNotes = escapeHtml(chk.notes || '');
        return `<li class="imha-check-item"><strong>${callsign}</strong> · PCV ${cPcv}% · Reticulocytes ${cRetic} K/µL · TBili ${cBili} mg/dL · 1:4 Saline ${cSaline} · RR ${cRr} bpm — ${cNotes}</li>`;
      })
      .join('');

    workspaceEl.innerHTML = `
      <div class="imha-workspace-card" data-workspace-op-id="${opId}">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name" id="workspace-pet-title">Active Workspace: ${petName} (${species}${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id" id="workspace-op-code">${opId} · ${incId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="status-pill status-pill-species" id="workspace-species-pill">${species}</span>
            <span class="status-pill ${statusClass}" id="workspace-status-pill">${status}</span>
          </div>
        </div>

        <div class="imha-vitals-summary" id="workspace-vitals-summary">
          <span><strong>PCV:</strong> ${pcv}%</span>
          <span><strong>Total Bilirubin:</strong> ${bili} mg/dL</span>
          <span><strong>Platelets:</strong> ${plt} K/µL</span>
          <span><strong>Spherocytes:</strong> ${sphero}/HPF</span>
          <span><strong>1:4 Saline Agglutination:</strong> ${salinePos}</span>
          <span><strong>Hemoglobinuria:</strong> ${hemoglobinuria}</span>
        </div>

        ${regimenHtml}

        <div class="imha-serial-checks-box">
          <h4>Serial q6h Hemolysis, Reticulocyte &amp; PTE Respiratory Checks (<span id="workspace-checks-count">${checks.length}</span>)</h4>
          <ul id="hemolysis-checks-list" class="imha-checks-list">
            ${checksHtml}
          </ul>
        </div>
      </div>
    `;
  }

  function renderOperationCardHtml(op) {
    const p = op.patient_profile || {};
    const reg = op.immunosuppressive_regimen || {};
    const opId = escapeHtml(op.id || '');
    const incId = escapeHtml(op.incident_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(p.species || 'CANINE_PRIMARY_IMHA');
    const breed = p.breed ? ` · ${escapeHtml(p.breed)}` : '';
    const weightKg = Number(p.weight_kg ?? 11.5).toFixed(1);
    const status = escapeHtml(op.status || 'HEMOLYTIC_AGGLUTINATION_CRISIS_CRITICAL');
    const statusClass = getStatusPillClass(op.status);
    const pcv = Number(p.packed_cell_volume_percent ?? 12.0).toFixed(1);
    const bili = Number(p.total_bilirubin_mg_dl ?? 6.4).toFixed(1);
    const plt = Number(p.platelet_count_k_ul ?? 85).toFixed(0);
    const sphero = Number(p.spherocytes_per_hpf ?? 18);
    const salineShort = p.saline_agglutination_4_to_1_positive ? 'POS' : 'NEG';

    const alertHtml = op.pulmonary_thromboembolism_alert
      ? `
        <div class="imha-alert-banner">
          ⚠️ PULMONARY THROMBOEMBOLISM (PTE) &amp; ZERO-JUGULAR VENIPUNCTURE ALERT (PCV ${pcv}% · Platelets ${plt} K/µL): ${escapeHtml(reg.imha_pitfall_alert || '')}
        </div>
      `
      : '';

    const clearanceHtml = op.discharge_clearance
      ? `
        <div class="imha-clearance-banner">
          ✅ 48H STABLE PCV &amp; RETICULOCYTOSIS TAPER SAFE (${escapeHtml(op.discharge_clearance.vet_license || '')}): PCV ${Number(op.discharge_clearance.final_pcv_stable_48_hr_percent ?? 28.5).toFixed(1)}% · Reticulocytes ${Number(op.discharge_clearance.final_reticulocyte_count_k_ul ?? 138).toFixed(0)} K/µL
        </div>
      `
      : '';

    return `
      <article class="trust-screening-card imha-op-card" data-operation-id="${opId}" data-op-id="${opId}" data-status="${status}" data-species="${species}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species}${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${incId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="status-pill status-pill-species">${species}</span>
            <span class="status-pill ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row imha-metrics-row">
          <span class="status-pill status-pill-species">PCV: ${pcv}% • TBili: ${bili} mg/dL • Platelets: ${plt} K/µL • Spherocytes: ${sphero}/HPF • 1:4 Saline: ${salineShort}</span>
        </div>

        ${alertHtml}
        ${clearanceHtml}

        <div class="trust-moderator-actions">
          <button
            type="button"
            class="btn btn-sm btn-secondary imha-select-op-btn"
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

    const countBadge = document.getElementById('imha-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('imha-operations-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="imha-ops-empty-msg">No IMHA &amp; Thromboprophylaxis operations match the current filters.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (document.getElementById('filter-status')?.value || '').trim();
      const speciesFilter = (document.getElementById('filter-species')?.value || '').trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);
      if (speciesFilter) params.set('species', speciesFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/imha-thrombosis/operations?${qs}`
        : '/api/imha-thrombosis/operations';

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
      const res = await fetch(`/api/imha-thrombosis/operations/${encodeURIComponent(opId)}`, {
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

  function setupFormsAndFilters() {
    const createForm = document.getElementById('create-imha-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const incidentId = (document.getElementById('create-incident-id')?.value || 'INC-IM-9704').trim();
        const petName = (document.getElementById('create-pet-name')?.value || '').trim();
        const species = (
          document.getElementById('create-species')?.value || 'CANINE_PRIMARY_IMHA'
        ).trim();
        const breed = (document.getElementById('create-breed')?.value || '').trim();
        const weightKg = parseFloat(document.getElementById('create-weight-kg')?.value || '0');
        const pcv = parseFloat(document.getElementById('create-pcv')?.value || '0');
        const bilirubin = parseFloat(document.getElementById('create-bilirubin')?.value || '0');
        const platelets = parseFloat(document.getElementById('create-platelets')?.value || '0');
        const spherocytes = parseInt(document.getElementById('create-spherocytes')?.value || '0', 10);
        const salinePositive = Boolean(document.getElementById('create-saline-agglutination')?.checked);
        const hemoglobinuria = Boolean(document.getElementById('create-hemoglobinuria')?.checked);

        const payload = {
          incident_id: incidentId,
          pet_name: petName,
          patient_profile: {
            species,
            breed,
            weight_kg: isNaN(weightKg) ? 0 : weightKg,
            packed_cell_volume_percent: isNaN(pcv) ? 0 : pcv,
            total_bilirubin_mg_dl: isNaN(bilirubin) ? 0 : bilirubin,
            platelet_count_k_ul: isNaN(platelets) ? 0 : platelets,
            spherocytes_per_hpf: isNaN(spherocytes) ? 0 : spherocytes,
            saline_agglutination_4_to_1_positive: salinePositive,
            intravascular_hemoglobinuria: hemoglobinuria,
          },
        };

        const btn = document.getElementById('btn-create-imha-op');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/imha-thrombosis/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(`Created IMHA operation ${created.id} for ${created.pet_name}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to create IMHA operation';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-regimen-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const active = currentOps.find((item) => item.id === selectedOpId) || currentOps[0];
        const baseProfile = active?.patient_profile || {
          species: 'CANINE_PRIMARY_IMHA',
          weight_kg: 11.5,
          packed_cell_volume_percent: 12.0,
          total_bilirubin_mg_dl: 6.4,
          platelet_count_k_ul: 85.0,
          spherocytes_per_hpf: 18,
          saline_agglutination_4_to_1_positive: true,
          intravascular_hemoglobinuria: true,
        };

        const weightKg = parseFloat(
          document.getElementById('stage-weight-kg')?.value || String(baseProfile.weight_kg)
        );
        const species = (
          document.getElementById('stage-species')?.value || baseProfile.species
        ).trim();
        const pcv = parseFloat(
          document.getElementById('stage-pcv')?.value || String(baseProfile.packed_cell_volume_percent)
        );
        const bilirubin = parseFloat(
          document.getElementById('stage-bilirubin')?.value || String(baseProfile.total_bilirubin_mg_dl)
        );
        const platelets = parseFloat(
          document.getElementById('stage-platelets')?.value || String(baseProfile.platelet_count_k_ul)
        );
        const spherocytes = parseInt(
          document.getElementById('stage-spherocytes')?.value || String(baseProfile.spherocytes_per_hpf),
          10
        );

        const profilePayload = {
          ...baseProfile,
          species,
          weight_kg: isNaN(weightKg) ? baseProfile.weight_kg : weightKg,
          packed_cell_volume_percent: isNaN(pcv) ? baseProfile.packed_cell_volume_percent : pcv,
          total_bilirubin_mg_dl: isNaN(bilirubin) ? baseProfile.total_bilirubin_mg_dl : bilirubin,
          platelet_count_k_ul: isNaN(platelets) ? baseProfile.platelet_count_k_ul : platelets,
          spherocytes_per_hpf: isNaN(spherocytes) ? baseProfile.spherocytes_per_hpf : spherocytes,
        };

        const btn = document.getElementById('btn-stage-regimen');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/imha-thrombosis/operations/${encodeURIComponent(selectedOpId)}/stage-regimen`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(profilePayload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Staged Prednisolone / Clopidogrel / pRBC regimen for ${updated.id} → ${updated.status}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to stage immunosuppressive regimen';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('hemolysis-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const pcv = parseFloat(document.getElementById('check-pcv')?.value || '23.0');
        const reticulocytes = parseFloat(document.getElementById('check-reticulocytes')?.value || '110.0');
        const bilirubin = parseFloat(document.getElementById('check-bilirubin')?.value || '1.8');
        const rr = parseInt(document.getElementById('check-respiratory-rate')?.value || '24', 10);
        const callsign = (document.getElementById('check-callsign')?.value || 'DR-HEME-11').trim();
        const notes = (document.getElementById('check-notes')?.value || '').trim();
        const salinePersistent = Boolean(document.getElementById('check-saline-persistent')?.checked);

        const checkPayload = {
          packed_cell_volume_percent: isNaN(pcv) ? 23.0 : pcv,
          reticulocyte_count_k_ul: isNaN(reticulocytes) ? 110.0 : reticulocytes,
          total_bilirubin_mg_dl: isNaN(bilirubin) ? 1.8 : bilirubin,
          saline_agglutination_persistent: salinePersistent,
          resting_respiratory_rate_bpm: isNaN(rr) ? 24 : rr,
          clinician_callsign: callsign,
          notes,
        };

        const btn = document.getElementById('btn-record-hemolysis-check');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/imha-thrombosis/operations/${encodeURIComponent(selectedOpId)}/hemolysis-check`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(checkPayload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(`Recorded q6h PCV, reticulocyte & PTE check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record hemolysis check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearanceForm = document.getElementById('discharge-clearance-form');
    if (clearanceForm) {
      clearanceForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const finalPcv = parseFloat(document.getElementById('clearance-final-pcv')?.value || '28.5');
        const finalRetic = parseFloat(
          document.getElementById('clearance-final-reticulocytes')?.value || '138.0'
        );
        const vetLicense = (document.getElementById('clearance-vet-license')?.value || 'VET-IM-97410').trim();
        const summary = (document.getElementById('clearance-summary')?.value || '').trim();
        const agglutinationResolved = Boolean(
          document.getElementById('clearance-agglutination-resolved')?.checked
        );
        const noPteDyspnea = Boolean(document.getElementById('clearance-no-pte-dyspnea')?.checked);
        const thromboprophylaxisDispensed = Boolean(
          document.getElementById('clearance-thromboprophylaxis-dispensed')?.checked
        );

        const clearancePayload = {
          final_pcv_stable_48_hr_percent: isNaN(finalPcv) ? 28.5 : finalPcv,
          final_reticulocyte_count_k_ul: isNaN(finalRetic) ? 138.0 : finalRetic,
          saline_agglutination_resolved: agglutinationResolved,
          no_pulmonary_thromboembolism_dyspnea: noPteDyspnea,
          thromboprophylaxis_dispensed: thromboprophylaxisDispensed,
          vet_license: vetLicense,
          summary,
        };

        const btn = document.getElementById('btn-issue-discharge-clearance');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/imha-thrombosis/operations/${encodeURIComponent(selectedOpId)}/discharge-clearance`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(clearancePayload),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            selectedOpId = updated.id;
            await fetchOperations();
            showToast(
              `Issued 48h stable PCV & reticulocytosis taper clearance for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to issue discharge clearance';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const filterStatus = document.getElementById('filter-status');
    if (filterStatus) {
      filterStatus.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const filterSpecies = document.getElementById('filter-species');
    if (filterSpecies) {
      filterSpecies.addEventListener('change', () => {
        fetchOperations();
      });
    }
  }

  function setupCardInteractions() {
    const listEl = document.getElementById('imha-operations-list');
    if (!listEl) return;

    listEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.imha-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.imha-op-card');
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
