/**
 * PetSpotR Acute Esophageal Foreign Body, Megaesophagus, Myasthenia Gravis
 * (AChR Antibody / Edrophonium-Neostigmine), Aspiration Pneumonia &
 * Pyridostigmine/Bailey Chair Cockpit Controller (Milestone 23.5)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'MEGA-OP-101';
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
    const liveStatus = document.getElementById('mega-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('mega-error-banner');
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
      case 'ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE':
        return 'mega-status-critical';
      case 'ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED':
        return 'mega-status-staged';
      case 'UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED':
        return 'mega-status-cleared';
      default:
        return 'mega-status-critical';
    }
  }

  function updateKPIs(ops, metrics) {
    const totalEl = document.getElementById('kpi-total-ops');
    const acuteEl = document.getElementById('kpi-acute-obstruction');
    const stagedEl = document.getElementById('kpi-endoscopy-staged');
    const clearedEl = document.getElementById('kpi-cleared-bailey');
    const fbEl = document.getElementById('kpi-foreign-body-count');
    const pneumoniaEl = document.getElementById('kpi-aspiration-pneumonia');

    const total = metrics?.total_operations ?? ops.length;
    const acute =
      metrics?.acute_obstruction_triage_count ??
      ops.filter(
        (op) => op.status === 'ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE'
      ).length;
    const staged =
      metrics?.endoscopy_pyridostigmine_staged_count ??
      ops.filter(
        (op) => op.status === 'ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED'
      ).length;
    const cleared =
      metrics?.cleared_bailey_chair_count ??
      ops.filter(
        (op) => op.status === 'UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED'
      ).length;
    const fb =
      metrics?.foreign_body_obstruction_count ??
      ops.filter(
        (op) =>
          op.pathology_type === 'ACUTE_ESOPHAGEAL_BONE_FISHHOOK_FOREIGN_BODY'
      ).length;
    const pneumonia =
      metrics?.aspiration_pneumonia_active_count ??
      ops.filter((op) => Boolean(op.aspiration_pneumonia_present)).length;

    if (totalEl) totalEl.textContent = String(total);
    if (acuteEl) acuteEl.textContent = String(acute);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (fbEl) fbEl.textContent = String(fb);
    if (pneumoniaEl) pneumoniaEl.textContent = String(pneumonia);
  }

  function syncFormsWithActiveOperation(op) {
    if (!op) return;
    const bailey = op.bailey_plan || {};
    const retInput = document.getElementById('stage-retrieval-input');
    const sucInput = document.getElementById('stage-sucralfate-input');
    const pyrInput = document.getElementById('stage-pyridostigmine-input');
    const abxInput = document.getElementById('stage-pneumonia-abx-input');
    const baiInput = document.getElementById('stage-bailey-protocol-input');
    const silInput = document.getElementById('stage-sildenafil-input');

    if (retInput && bailey.endoscopic_retrieval_or_gastric_push) {
      retInput.value = bailey.endoscopic_retrieval_or_gastric_push;
    }
    if (sucInput && bailey.post_removal_sucralfate_omeprazole) {
      sucInput.value = bailey.post_removal_sucralfate_omeprazole;
    }
    if (pyrInput && bailey.pyridostigmine_anticholinesterase) {
      pyrInput.value = bailey.pyridostigmine_anticholinesterase;
    }
    if (abxInput && bailey.aspiration_pneumonia_antibiotic_plan) {
      abxInput.value = bailey.aspiration_pneumonia_antibiotic_plan;
    }
    if (baiInput && bailey.upright_bailey_chair_feeding_protocol) {
      baiInput.value = bailey.upright_bailey_chair_feeding_protocol;
    }
    if (silInput && bailey.sildenafil_les_relaxation_note) {
      silInput.value = bailey.sildenafil_les_relaxation_note;
    }
  }

  function updateClearanceBanner(active) {
    const clearanceBanner = document.getElementById('mega-clearance-banner');
    const summaryTextEl = document.getElementById('clearance-summary-text');
    if (!clearanceBanner) return;

    if (
      active &&
      active.status === 'UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED'
    ) {
      const by = active.discharged_by || 'Attending Internist';
      const summary =
        active.discharge_summary ||
        'Upright Bailey Chair vertical feeding protocol established and normothermic clearance verified.';
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
      const weightStr = Number(active.weight_kg ?? 15.0).toFixed(1);
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species || 'Canine'} · ${active.breed || 'Mixed Breed'} · ${weightStr} kg)`;
    }

    const pathoEl = document.getElementById('active-pathology-type');
    if (pathoEl) {
      pathoEl.textContent = active.pathology_type || '';
    }
    const achrEl = document.getElementById('active-achr-titer');
    if (achrEl) {
      achrEl.textContent = `${Number(active.achr_antibody_titer_nmol_l ?? 0.1).toFixed(2)} nmol/L`;
    }
    const pneumoEl = document.getElementById('active-pneumonia-flag');
    if (pneumoEl) {
      pneumoEl.textContent = active.aspiration_pneumonia_present ? 'YES' : 'NO';
    }

    const badgeEl = document.getElementById('active-op-status-badge');
    if (badgeEl) {
      badgeEl.textContent =
        active.status || 'ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE';
      badgeEl.className = `mega-status-badge ${getStatusBadgeClass(active.status)}`;
    }

    const spo2El = document.getElementById('active-spo2');
    if (spo2El) {
      spo2El.textContent = `${Number(active.spo2_percent ?? 0).toFixed(1)}%`;
    }
    const aaEl = document.getElementById('active-aa-gradient');
    if (aaEl) {
      aaEl.textContent = `${Number(active.alveolar_arterial_gradient_mmhg ?? 0).toFixed(1)} mmHg`;
    }
    const regurgEl = document.getElementById('active-regurg-episodes');
    if (regurgEl) {
      regurgEl.textContent = `${Number(active.regurgitation_episodes_24h ?? 0)} episodes/24h`;
    }
    const upEsophEl = document.getElementById('active-upright-esophagitis');
    if (upEsophEl) {
      upEsophEl.textContent = `${Number(active.upright_hold_minutes ?? 0)} min · Grade ${Number(active.mucosal_esophagitis_grade ?? 0)}/4`;
    }

    const asp = active.aspiration_plan || {};
    const setText = (id, val) => {
      const el = document.getElementById(id);
      if (el && val !== undefined) el.textContent = val;
    };
    setText('plan-regurg-note', asp.regurgitation_vs_vomiting_note || '');
    setText(
      'plan-radiograph-finding',
      asp.thoracic_radiograph_pocus_finding || ''
    );
    setText(
      'plan-perforation-alert',
      asp.perforation_pneumomediastinum_alert || ''
    );
    setText('plan-achr-screen', asp.achr_antibody_addison_screen_note || '');
    setText('plan-barium-guard', asp.barium_swallow_contraindication || '');
    setText(
      'plan-crisis-note',
      asp.cholinergic_vs_myasthenic_crisis_note || ''
    );

    const bailey = active.bailey_plan || {};
    setText(
      'plan-endoscopy-retrieval',
      bailey.endoscopic_retrieval_or_gastric_push || ''
    );
    setText(
      'plan-sucralfate-omeprazole',
      bailey.post_removal_sucralfate_omeprazole || ''
    );
    setText(
      'plan-pyridostigmine',
      bailey.pyridostigmine_anticholinesterase || ''
    );
    setText(
      'plan-pneumonia-abx',
      bailey.aspiration_pneumonia_antibiotic_plan || ''
    );
    setText(
      'plan-bailey-chair',
      bailey.upright_bailey_chair_feeding_protocol || ''
    );
    setText('plan-sildenafil', bailey.sildenafil_les_relaxation_note || '');

    const checks = Array.isArray(active.esophageal_checks)
      ? active.esophageal_checks
      : [];
    const checksCountEl = document.getElementById('esophageal-checks-count');
    if (checksCountEl) {
      checksCountEl.textContent = String(checks.length);
    }

    const checksListEl = document.getElementById('esophageal-checks-list');
    if (checksListEl) {
      checksListEl.innerHTML = checks
        .map((chk) => {
          const idAttr = escapeHtml(chk.id || '');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const spo2 = Number(chk.spo2_percent ?? 0).toFixed(1);
          const aa = Number(chk.alveolar_arterial_gradient_mmhg ?? 0).toFixed(1);
          const regurg = Number(chk.regurgitation_episodes_24h ?? 0);
          const hold = Number(chk.upright_hold_minutes ?? 0);
          const grade = Number(chk.mucosal_esophagitis_grade ?? 0);
          const note = escapeHtml(chk.clinical_note || '');
          return `<li class="mega-check-item" data-check-id="${idAttr}"><strong>${by}</strong> · SpO2 ${spo2}% · A-a Gradient ${aa} mmHg · Regurg ${regurg}/24h · Bailey Hold ${hold} min · Esophagitis Grade ${grade}/4 — ${note}</li>`;
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
      op.status || 'ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE'
    );
    const badgeClass = getStatusBadgeClass(op.status);
    const patho = escapeHtml(op.pathology_type || '');
    const spo2 = Number(op.spo2_percent ?? 0).toFixed(1);
    const aa = Number(op.alveolar_arterial_gradient_mmhg ?? 0).toFixed(1);
    const regurg = Number(op.regurgitation_episodes_24h ?? 0);
    const hold = Number(op.upright_hold_minutes ?? 0);
    const grade = Number(op.mucosal_esophagitis_grade ?? 0);
    const achr = Number(op.achr_antibody_titer_nmol_l ?? 0).toFixed(2);
    const pneumoText = op.aspiration_pneumonia_present
      ? 'Pneumonia+'
      : 'Pneumonia-';

    return `
      <article
        class="trust-screening-card mega-op-card"
        data-op-id="${opId}"
        data-operation-id="${opId}"
        data-status="${status}"
        data-pathology="${patho}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${opId} · ${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="mega-status-badge ${badgeClass}">${status}</span>
          </div>
        </div>

        <div class="mega-card-metrics">
          <span><strong>Pathology:</strong> ${patho}</span>
          <span><strong>SpO2:</strong> ${spo2}%</span>
          <span><strong>A-a Gradient:</strong> ${aa} mmHg</span>
          <span><strong>Regurg (24h):</strong> ${regurg} episodes</span>
          <span><strong>Bailey Hold:</strong> ${hold} min</span>
          <span><strong>Esophagitis:</strong> Grade ${grade}/4</span>
          <span><strong>AChR Titer:</strong> ${achr} nmol/L</span>
          <span><strong>Aspiration Pneumonia:</strong> ${pneumoText}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary mega-select-op-btn" data-op-id="${opId}">
            🔍 Open Megaesophagus Workspace (${opId})
          </button>
        </div>
      </article>
    `;
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateKPIs(ops, metrics);
    renderActiveWorkspace(ops);

    const countBadge = document.getElementById('mega-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const gridEl = document.getElementById('mega-operations-grid');
    if (!gridEl) return;

    if (ops.length === 0) {
      gridEl.innerHTML =
        '<p class="trust-empty-hint" id="mega-ops-empty-msg">No Megaesophagus &amp; Foreign Body operations match the selected filter.</p>';
      return;
    }

    gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('mega-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/megaesophagus-foreignbody/operations?${qs}`
        : '/api/megaesophagus-foreignbody/operations';

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
        `/api/megaesophagus-foreignbody/operations/${encodeURIComponent(opId)}`,
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
        statusBadge.className = 'mega-status-badge mega-status-cleared';
      }
      eventSource = new EventSource('/api/megaesophagus-foreignbody/stream');
      eventSource.addEventListener('megaesophagus_foreignbody_updated', () => {
        fetchOperations();
      });
      eventSource.onmessage = () => {
        fetchOperations();
      };
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('mega-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const createForm = document.getElementById('create-mega-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const caseId = (
          document.getElementById('create-case-id-input')?.value ||
          'CASE-MEGA-104'
        ).trim();
        const petName = (
          document.getElementById('create-pet-name-input')?.value || ''
        ).trim();
        const species = (
          document.getElementById('create-species-input')?.value || 'Canine'
        ).trim();
        const breed = (
          document.getElementById('create-breed-input')?.value ||
          'Golden Retriever'
        ).trim();
        const weightKg = parseFloat(
          document.getElementById('create-weight-input')?.value || '28.2'
        );
        const pathology = (
          document.getElementById('create-pathology-input')?.value ||
          'ACUTE_ESOPHAGEAL_BONE_FISHHOOK_FOREIGN_BODY'
        ).trim();
        const spo2 = parseFloat(
          document.getElementById('create-spo2-input')?.value || '92.0'
        );
        const aaGradient = parseFloat(
          document.getElementById('create-aa-gradient-input')?.value || '31.0'
        );
        const regurg = parseInt(
          document.getElementById('create-regurg-input')?.value || '5',
          10
        );
        const upright = parseInt(
          document.getElementById('create-upright-input')?.value || '0',
          10
        );
        const esophagitis = parseInt(
          document.getElementById('create-esophagitis-input')?.value || '2',
          10
        );
        const achr = parseFloat(
          document.getElementById('create-achr-input')?.value || '0.15'
        );
        const pneumoniaEl = document.getElementById('create-pneumonia-checkbox');
        const pneumonia = pneumoniaEl ? Boolean(pneumoniaEl.checked) : true;

        const payload = {
          case_id: caseId,
          pet_name: petName,
          species,
          breed,
          weight_kg: isNaN(weightKg) ? 28.2 : weightKg,
          pathology_type: pathology,
          spo2_percent: isNaN(spo2) ? 92.0 : spo2,
          alveolar_arterial_gradient_mmhg: isNaN(aaGradient)
            ? 31.0
            : aaGradient,
          regurgitation_episodes_24h: isNaN(regurg) ? 5 : regurg,
          upright_hold_minutes: isNaN(upright) ? 0 : upright,
          mucosal_esophagitis_grade: isNaN(esophagitis) ? 2 : esophagitis,
          achr_antibody_titer_nmol_l: isNaN(achr) ? 0.15 : achr,
          aspiration_pneumonia_present: pneumonia,
        };

        const btn = document.getElementById('create-mega-op-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/megaesophagus-foreignbody/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            showToast(
              `Admitted Megaesophagus/FB patient ${created.pet_name} (${created.id})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to admit Megaesophagus/FB patient';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const stageForm = document.getElementById('stage-bailey-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const retrieval = (
          document.getElementById('stage-retrieval-input')?.value ||
          'Flexible esophagoscopy under cuffed endotracheal tube anesthesia; alligator forceps extraction'
        ).trim();
        const sucralfate = (
          document.getElementById('stage-sucralfate-input')?.value ||
          'Sucralfate oral suspension slurry 0.5–1.0 g PO q8h + Omeprazole 1 mg/kg PO/IV q12h'
        ).trim();
        const pyrido = (
          document.getElementById('stage-pyridostigmine-input')?.value || ''
        ).trim();
        const abx = (
          document.getElementById('stage-pneumonia-abx-input')?.value || ''
        ).trim();
        const bailey = (
          document.getElementById('stage-bailey-protocol-input')?.value ||
          'Upright Bailey Chair 90-degree vertical feeding of Knox gelatin / meatball slurry with strict 25–30 min hold'
        ).trim();
        const sildenafil = (
          document.getElementById('stage-sildenafil-input')?.value || ''
        ).trim();

        const payload = {
          endoscopic_retrieval_or_gastric_push: retrieval,
          post_removal_sucralfate_omeprazole: sucralfate,
          pyridostigmine_anticholinesterase: pyrido,
          aspiration_pneumonia_antibiotic_plan: abx,
          upright_bailey_chair_feeding_protocol: bailey,
          sildenafil_les_relaxation_note: sildenafil,
        };

        const btn = document.getElementById('stage-bailey-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/megaesophagus-foreignbody/operations/${encodeURIComponent(selectedOpId)}/stage-bailey`,
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
              `Staged Endoscopy / Pyridostigmine / Bailey Chair plan for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to stage Endoscopy / Bailey Chair plan';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-esophageal-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const spo2 = parseFloat(
          document.getElementById('check-spo2-input')?.value || '98.0'
        );
        const aaGradient = parseFloat(
          document.getElementById('check-aa-gradient-input')?.value || '10.0'
        );
        const regurg = parseInt(
          document.getElementById('check-regurg-input')?.value || '0',
          10
        );
        const upright = parseInt(
          document.getElementById('check-upright-input')?.value || '30',
          10
        );
        const esophagitis = parseInt(
          document.getElementById('check-esophagitis-input')?.value || '1',
          10
        );
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value ||
          'Dr. Clara Sterling, DACVIM (SAIM)'
        ).trim();
        const clinicalNote = (
          document.getElementById('check-note-input')?.value || ''
        ).trim();

        const payload = {
          spo2_percent: isNaN(spo2) ? 98.0 : spo2,
          alveolar_arterial_gradient_mmhg: isNaN(aaGradient)
            ? 10.0
            : aaGradient,
          regurgitation_episodes_24h: isNaN(regurg) ? 0 : regurg,
          upright_hold_minutes: isNaN(upright) ? 30 : upright,
          mucosal_esophagitis_grade: isNaN(esophagitis) ? 1 : esophagitis,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-esophageal-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/megaesophagus-foreignbody/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
              `Recorded esophageal & aspiration check for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg = errData.error || 'Failed to record esophageal check';
            showErrorBanner(msg);
            showToast(msg, 'error');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-mega-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dischargedBy = (
          document.getElementById('clear-discharged-by-input')?.value ||
          'Dr. Clara Sterling, DACVIM (SAIM)'
        ).trim();
        const summary = (
          document.getElementById('clear-summary-input')?.value || ''
        ).trim();

        const payload = {
          discharged_by: dischargedBy,
          discharge_summary: summary,
        };

        const btn = document.getElementById('clear-mega-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/megaesophagus-foreignbody/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
              `Cleared ${updated.id} (${updated.pet_name}) for Upright Bailey Chair Discharge`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            const msg =
              errData.error || 'Failed to clear operation for discharge';
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
    const gridEl = document.getElementById('mega-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.mega-select-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.mega-op-card');
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
