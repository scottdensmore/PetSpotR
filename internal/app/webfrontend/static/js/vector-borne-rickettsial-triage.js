/**
 * PetSpotR Acute Vector-Borne & Rickettsial Emergencies (Lyme Nephritis, Anaplasmosis,
 * Ehrlichiosis, Rocky Mountain Spotted Fever, Babesiosis & Cytauxzoonosis),
 * SNAP 4Dx Plus / PCR Triage, Doxycycline, Atovaquone + Azithromycin &
 * Imidocarb Dipropionate Cockpit Controller (Milestone 24.5)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'VBR-OP-101';
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
    const liveStatus = document.getElementById('vbr-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('vbr-error-banner');
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
      case 'DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED':
        return 'vbr-status-critical';
      case 'THROMBOCYTOPENIA_PROTEINURIA_MONITORED':
        return 'vbr-status-staged';
      case 'PCR_NEGATIVE_PLATELETS_RENAL_CLEARED':
        return 'vbr-status-cleared';
      default:
        return 'vbr-status-critical';
    }
  }

  function updateKPIs(ops, overview) {
    const totalEl = document.getElementById('kpi-total-ops');
    const stagedEl = document.getElementById('kpi-antimicrobial-staged');
    const monitoredEl = document.getElementById('kpi-thrombocytopenia-monitored');
    const clearedEl = document.getElementById('kpi-pcr-cleared');
    const lymeEl = document.getElementById('kpi-lyme-proteinuria');
    const protozoalEl = document.getElementById('kpi-protozoal-ops');

    const total = overview?.total_operations ?? ops.length;
    const staged =
      overview?.antimicrobial_staged_count ??
      ops.filter((op) => op.status === 'DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED').length;
    const monitored =
      overview?.thrombocytopenia_monitored_count ??
      ops.filter((op) => op.status === 'THROMBOCYTOPENIA_PROTEINURIA_MONITORED').length;
    const cleared =
      overview?.pcr_negative_cleared_count ??
      ops.filter((op) => op.status === 'PCR_NEGATIVE_PLATELETS_RENAL_CLEARED').length;
    const lymeCount =
      overview?.lyme_nephritis_proteinuria_count ??
      ops.filter(
        (op) =>
          op.lyme_nephritis_upc_positive ||
          Number(op.urine_pc_ratio ?? 0) >= 0.5 ||
          op.syndrome === 'LYME_NEPHRITIS_BORRELIA_UPC'
      ).length;
    const protozoalCount =
      overview?.babesia_cytauxzoon_protozoal_count ??
      ops.filter(
        (op) =>
          op.syndrome === 'BABESIOSIS_GIBSONI_CANIS' ||
          op.syndrome === 'CYTAUXZOONOSIS_FELIS_SCHIZONTS'
      ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (monitoredEl) monitoredEl.textContent = String(monitored);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (lymeEl) lymeEl.textContent = String(lymeCount);
    if (protozoalEl) protozoalEl.textContent = String(protozoalCount);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const protoSelect = document.getElementById('stage-protocol-input');
    const doxyInput = document.getElementById('stage-doxy-input');
    const atovInput = document.getElementById('stage-atovaquone-input');
    const imidoInput = document.getElementById('stage-imidocarb-input');
    const waterChaseCheck = document.getElementById('stage-water-chase-checkbox');
    const atropineCheck = document.getElementById('stage-atropine-checkbox');

    if (protoSelect && op.antimicrobial_protocol) {
      protoSelect.value = op.antimicrobial_protocol;
    }
    if (doxyInput && op.doxycycline_mg_kg_day !== undefined) {
      doxyInput.value = String(op.doxycycline_mg_kg_day);
    }
    if (atovInput && op.atovaquone_mg_kg_tid !== undefined) {
      atovInput.value = String(op.atovaquone_mg_kg_tid);
    }
    if (imidoInput && op.imidocarb_mg_kg_im !== undefined) {
      imidoInput.value = String(op.imidocarb_mg_kg_im);
    }
    if (waterChaseCheck) {
      waterChaseCheck.checked = Boolean(op.esophageal_water_chase_verified ?? true);
    }
    if (atropineCheck) {
      atropineCheck.checked = Boolean(op.atropine_premed_verified ?? true);
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('vbr-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (active && active.status === 'PCR_NEGATIVE_PLATELETS_RENAL_CLEARED') {
      const by =
        active.discharged_by ||
        active.clearance?.cleared_by ||
        'Attending Infectious Disease Specialist';
      const summary =
        active.discharge_summary ||
        active.clearance?.clearance_summary ||
        'PCR negative, platelets >= 150 K/uL, PCV >= 35%, UPC < 0.5.';
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
      const weightStr = Number(active.weight_kg ?? 15.0).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Canine'} · ${active.breed || 'Mixed Breed'} · ${weightStr} kg)`;
    }

    const synEl = document.getElementById('active-syndrome');
    if (synEl) synEl.textContent = active.syndrome || '';
    const assayEl = document.getElementById('active-assay');
    if (assayEl) assayEl.textContent = active.assay || '';
    const protoEl = document.getElementById('active-protocol');
    if (protoEl) protoEl.textContent = active.antimicrobial_protocol || '';

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent =
        active.status || 'DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED';
      badgeEl.className = `vbr-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const pltEl = document.getElementById('active-platelets');
    if (pltEl) {
      pltEl.textContent = `${Number(active.platelet_count_kul ?? 0).toFixed(1)} K/µL`;
    }
    const pcvEl = document.getElementById('active-pcv');
    if (pcvEl) {
      pcvEl.textContent = `${Number(active.pcv_percent ?? 0).toFixed(1)}%`;
    }
    const upcEl = document.getElementById('active-upc');
    if (upcEl) {
      upcEl.textContent = Number(active.urine_pc_ratio ?? 0).toFixed(2);
    }
    const albEl = document.getElementById('active-albumin');
    if (albEl) {
      albEl.textContent = `${Number(active.serum_albumin_g_dl ?? 0).toFixed(1)} g/dL`;
    }

    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-pathophysiology', active.pathophysiology_summary || '');

    const st = active.antimicrobial_staging || {};
    setText('plan-stricture-guard', st.esophageal_stricture_guard_note || '');
    setText('plan-fatty-meal-guard', st.fatty_meal_bioavailability_note || '');
    setText('plan-atropine-guard', st.cholinergic_atropine_guard_note || '');
    setText('plan-lyme-pln-guard', st.lyme_nephritis_immunomodulation_note || '');

    const alertsListEl = document.getElementById('active-safety-alerts-list');
    if (alertsListEl) {
      const alerts = Array.isArray(active.safety_guard_alerts)
        ? active.safety_guard_alerts
        : [];
      alertsListEl.innerHTML = alerts
        .map((a) => `<li class="vbr-check-item">${escapeHtml(a)}</li>`)
        .join('');
    }

    const checks = Array.isArray(active.hematology_checks)
      ? active.hematology_checks
      : [];
    const checksCountEl = document.getElementById('hematology-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('hematology-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const plt = Number(chk.platelet_count_kul ?? 0).toFixed(1);
          const pcv = Number(chk.pcv_percent ?? 0).toFixed(1);
          const upc = Number(chk.urine_pc_ratio ?? 0).toFixed(2);
          const alb = Number(chk.serum_albumin_g_dl ?? 0).toFixed(1);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="vbr-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · Platelets ${plt} K/µL · PCV ${pcv}% · UPC ${upc} · Albumin ${alb} g/dL — ${note}</li>`;
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
    const weightKg = Number(op.weight_kg ?? 15.0).toFixed(1);
    const status = escapeHtml(
      op.status || 'DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED'
    );
    const badgeClass = getStatusBadgeClass(op.status);
    const syndrome = escapeHtml(op.syndrome || '');
    const assay = escapeHtml(op.assay || '');
    const protocol = escapeHtml(op.antimicrobial_protocol || '');
    const plt = Number(op.platelet_count_kul ?? 0).toFixed(1);
    const pcv = Number(op.pcv_percent ?? 0).toFixed(1);
    const upc = Number(op.urine_pc_ratio ?? 0).toFixed(2);
    const alb = Number(op.serum_albumin_g_dl ?? 0).toFixed(1);
    const waterChaseText = op.esophageal_water_chase_verified
      ? 'VERIFIED'
      : 'Pending';

    return `
      <article
        class="trust-screening-card vbr-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-syndrome="${syndrome}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="vbr-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="vbr-card-metrics">
          <span><strong>Syndrome:</strong> ${syndrome}</span>
          <span><strong>Assay:</strong> ${assay}</span>
          <span><strong>Protocol:</strong> ${protocol}</span>
          <span><strong>Platelets:</strong> ${plt} K/µL</span>
          <span><strong>PCV:</strong> ${pcv}%</span>
          <span><strong>UPC Ratio:</strong> ${upc}</span>
          <span><strong>Albumin:</strong> ${alb} g/dL</span>
          <span><strong>Water Chase:</strong> ${waterChaseText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary vbr-select-op-btn" data-op-id="${opId}">
            🔍 Open Vector-Borne Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, overview) {
    currentOps = ops;
    updateKPIs(ops, overview);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('vbr-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('vbr-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="vbr-ops-empty-msg">No Vector-Borne &amp; Rickettsial operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('vbr-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/vector-borne-rickettsial/operations?${qs}`
        : '/api/vector-borne-rickettsial/operations';

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
      renderAllOperations(ops, data.overview || data.metrics);
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
        `/api/vector-borne-rickettsial/operations/${encodeURIComponent(opId)}`,
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
        statusBadge.className = 'vbr-status-badge vbr-status-cleared';
      }
      eventSource = new EventSource('/api/vector-borne-rickettsial/stream');
      eventSource.addEventListener('vector_borne_rickettsial_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('vbr-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-vbr-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value ||
          'CASE-VBR-104'
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
        const syndrome = (
          document.getElementById('create-syndrome-input')?.value ||
          'EHRLICHIOSIS_ANAPLASMOSIS_MORULAE'
        ).trim();
        const assay = (
          document.getElementById('create-assay-input')?.value ||
          'SNAP_4DX_PLUS_C6_PEPTIDE'
        ).trim();
        const protocol = (
          document.getElementById('create-protocol-input')?.value ||
          'DOXYCYCLINE_10MG_KG_WATER_CHASE'
        ).trim();
        const platelets = parseFloat(
          document.getElementById('create-platelets-input')?.value || '38.0'
        );
        const pcv = parseFloat(
          document.getElementById('create-pcv-input')?.value || '28.0'
        );
        const upc = parseFloat(
          document.getElementById('create-upc-input')?.value || '0.4'
        );
        const albumin = parseFloat(
          document.getElementById('create-albumin-input')?.value || '2.6'
        );
        const doxy = parseFloat(
          document.getElementById('create-doxy-input')?.value || '10.0'
        );
        const waterChase = Boolean(
          document.getElementById('create-water-chase-checkbox')?.checked
        );
        const atropinePremed = Boolean(
          document.getElementById('create-atropine-checkbox')?.checked
        );

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 22.5 : weightKg,
          syndrome,
          assay,
          antimicrobial_protocol: protocol,
          platelet_count_kul: isNaN(platelets) ? 38.0 : platelets,
          pcv_percent: isNaN(pcv) ? 28.0 : pcv,
          urine_pc_ratio: isNaN(upc) ? 0.4 : upc,
          serum_albumin_g_dl: isNaN(albumin) ? 2.6 : albumin,
          doxycycline_mg_kg_day: isNaN(doxy) ? 10.0 : doxy,
          atovaquone_mg_kg_tid:
            protocol === 'ATOVAQUONE_15MG_KG_TID_AZITHROMYCIN' ? 15.0 : 0.0,
          imidocarb_mg_kg_im:
            protocol === 'IMIDOCARB_6_6MG_KG_ATROPINE_PREMED' ? 6.6 : 0.0,
          esophageal_water_chase_verified: waterChase,
          atropine_premed_verified: atropinePremed,
        };

        const btn = document.getElementById('create-vbr-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/vector-borne-rickettsial/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted Vector-Borne patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to admit Vector-Borne patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-antimicrobial-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const protocol = (
          document.getElementById('stage-protocol-input')?.value ||
          'DOXYCYCLINE_10MG_KG_WATER_CHASE'
        ).trim();
        const doxy = parseFloat(
          document.getElementById('stage-doxy-input')?.value || '10.0'
        );
        const atovaquone = parseFloat(
          document.getElementById('stage-atovaquone-input')?.value || '15.0'
        );
        const imidocarb = parseFloat(
          document.getElementById('stage-imidocarb-input')?.value || '0.0'
        );
        const waterChase = Boolean(
          document.getElementById('stage-water-chase-checkbox')?.checked
        );
        const atropinePremed = Boolean(
          document.getElementById('stage-atropine-checkbox')?.checked
        );
        const stagedBy = (
          document.getElementById('stage-by-input')?.value ||
          'Dr. Mara Vance, DACVIM (Infectious Disease)'
        ).trim();
        const rationale = (
          document.getElementById('stage-rationale-input')?.value || ''
        ).trim();

        const payload = {
          protocol,
          doxycycline_mg_kg_day: isNaN(doxy) ? 10.0 : doxy,
          atovaquone_mg_kg_tid: isNaN(atovaquone) ? 15.0 : atovaquone,
          imidocarb_mg_kg_im: isNaN(imidocarb) ? 0.0 : imidocarb,
          esophageal_water_chase_verified: waterChase,
          atropine_premed_verified: atropinePremed,
          staged_by: stagedBy,
          clinical_rationale: rationale,
        };

        const btn = document.getElementById('stage-antimicrobial-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/vector-borne-rickettsial/operations/${encodeURIComponent(selectedOpId)}/antimicrobial`,
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
              `Staged Antimicrobial & Safety Guard Regimen for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to stage antimicrobial protocol';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-hematology-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const platelets = parseFloat(
          document.getElementById('check-platelets-input')?.value || '185.0'
        );
        const pcv = parseFloat(
          document.getElementById('check-pcv-input')?.value || '39.0'
        );
        const upc = parseFloat(
          document.getElementById('check-upc-input')?.value || '0.25'
        );
        const albumin = parseFloat(
          document.getElementById('check-albumin-input')?.value || '3.2'
        );
        const morulaeCleared = Boolean(
          document.getElementById('check-morulae-cleared-checkbox')?.checked
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Mara Vance, DACVIM (Infectious Disease)'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          platelet_count_kul: isNaN(platelets) ? 185.0 : platelets,
          pcv_percent: isNaN(pcv) ? 39.0 : pcv,
          urine_pc_ratio: isNaN(upc) ? 0.25 : upc,
          serum_albumin_g_dl: isNaN(albumin) ? 3.2 : albumin,
          morulae_or_piroplasms_cleared: morulaeCleared,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-hematology-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/vector-borne-rickettsial/operations/${encodeURIComponent(selectedOpId)}/hematology-check`,
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
              `Recorded Hematology & Renal UPC Check for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to record hematology/renal check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-vbr-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const platelets = parseFloat(
          document.getElementById('clear-platelets-input')?.value || '195.0'
        );
        const pcv = parseFloat(
          document.getElementById('clear-pcv-input')?.value || '40.0'
        );
        const upc = parseFloat(
          document.getElementById('clear-upc-input')?.value || '0.20'
        );
        const pcrNegative = Boolean(
          document.getElementById('clear-pcr-negative-checkbox')?.checked
        );
        const normothermicEating = Boolean(
          document.getElementById('clear-normothermic-checkbox')?.checked
        );
        const clearedBy = (
          document.getElementById('clear-cleared-by-input')?.value ||
          'Dr. Mara Vance, DACVIM (Infectious Disease)'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          platelet_count_kul: isNaN(platelets) ? 195.0 : platelets,
          pcv_percent: isNaN(pcv) ? 40.0 : pcv,
          urine_pc_ratio: isNaN(upc) ? 0.2 : upc,
          pcr_negative_verified: pcrNegative,
          normothermic_eating_verified: normothermicEating,
          cleared_by: clearedBy,
          clearance_summary: summary,
        };

        const btn = document.getElementById('clear-vbr-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/vector-borne-rickettsial/operations/${encodeURIComponent(selectedOpId)}/clearance`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for PCR-Negative Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error ||
              'Failed to clear Vector-Borne operation for discharge';
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
    const gridEl = document.getElementById('vbr-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.vbr-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.vbr-op-card');
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
