/**
 * PetSpotR Acute Pancreatitis, Canine/Feline Spec cPL/fPL,
 * POCUS Hypoechoic Pancreas/Hyperechoic Mesentery, Maropitant/Fuzapladib
 * & Early Enteral Nutrition Cockpit Controller (Milestone 22.4)
 */

(function () {
  'use strict';

  let currentOps = [];
  let selectedOpId = 'PANC-OP-101';
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
  }

  function showErrorBanner(message) {
    const banner = document.getElementById('pancreatitis-error-banner');
    if (!banner) return;
    if (!message) {
      banner.textContent = '';
      banner.hidden = true;
      return;
    }
    banner.textContent = message;
    banner.hidden = false;
  }

  function updateOverviewMetrics(ops, metrics) {
    const totalEl = document.getElementById('metric-total-ops');
    const triageEl = document.getElementById('metric-acute-triage');
    const stagedEl = document.getElementById('metric-enteral-staged');
    const clearedEl = document.getElementById('metric-cleared-enteral');
    const severePlEl = document.getElementById('metric-severe-spec-pl');
    const triaditisEl = document.getElementById('metric-feline-triaditis');

    const total = metrics?.total_operations ?? ops.length;
    const triage =
      metrics?.acute_pancreatitis_triage_count ??
      ops.filter((op) => op.status === 'ACUTE_NECROTIZING_PANCREATITIS_TRIAGE').length;
    const staged =
      metrics?.enteral_nutrition_staged_count ??
      ops.filter((op) => op.status === 'FUZAPLADIB_MAROPITANT_ENTERAL_STAGED').length;
    const cleared =
      metrics?.cleared_enteral_count ??
      ops.filter((op) => op.status === 'TOLERATING_ENTERAL_PAIN_FREE_CLEARED').length;
    const severePl =
      metrics?.severe_spec_pl_elevated_count ??
      ops.filter((op) => {
        const isFeline = String(op.species || '').toUpperCase().includes('FELINE');
        return isFeline ? Number(op.spec_pl_ug_l) >= 5 : Number(op.spec_pl_ug_l) > 400;
      }).length;
    const triaditis =
      metrics?.feline_triaditis_count ??
      ops.filter(
        (op) =>
          Boolean(op.feline_triaditis_suspected) ||
          op.severity_class === 'FELINE_TRIADITIS_CHOLANGIOHEPATITIS'
      ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (triageEl) triageEl.textContent = String(triage);
    if (stagedEl) stagedEl.textContent = String(staged);
    if (clearedEl) clearedEl.textContent = String(cleared);
    if (severePlEl) severePlEl.textContent = String(severePl);
    if (triaditisEl) triaditisEl.textContent = String(triaditis);

    const countBadge = document.getElementById('pancreatitis-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Operations`;
    }
  }

  function renderOperationCardHtml(op) {
    const id = escapeHtml(op.id || '');
    const caseId = escapeHtml(op.case_id || '');
    const petName = escapeHtml(op.pet_name || '');
    const species = escapeHtml(op.species || 'Canine');
    const breed = escapeHtml(op.breed || 'Mixed');
    const weightKg = Number(op.weight_kg ?? 8.0).toFixed(1);
    const status = escapeHtml(op.status || 'ACUTE_NECROTIZING_PANCREATITIS_TRIAGE');
    const severity = escapeHtml(op.severity_class || 'SEVERE_NECROTIZING_SIRS_MODS');
    const specPl = Number(op.spec_pl_ug_l ?? 0);
    const painScore = Number(op.modified_glasgow_pain_score ?? 0);
    const rerPct = Number(op.enteral_rer_percent ?? 0);
    const hypoechoic = op.pocus_hypoechoic_pancreas ? 'YES' : 'NO';
    const hyperechoicFat = op.hyperechoic_peripancreatic_fat ? 'YES' : 'NO';
    const prayer = op.prayer_position_present ? 'YES' : 'NO';
    const triaditis = op.feline_triaditis_suspected ? 'YES (B12 Indicated)' : 'NO';

    return `
      <article
        class="glass-card pancreatitis-op-card"
        data-op-id="${id}"
        data-status="${status}"
        role="listitem"
      >
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${id} · ${petName} (${species} — ${breed} · ${weightKg} kg)</strong>
            <code class="trust-pet-id">${caseId} · ${severity}</code>
          </div>
          <span class="pancreatitis-status-badge" data-badge-status="${status}">${status}</span>
        </div>

        <div class="pancreatitis-card-metrics">
          <span><strong>Spec PL:</strong> ${specPl} &micro;g/L</span>
          <span><strong>CMPS-SF Pain Score:</strong> ${painScore}/24</span>
          <span><strong>Enteral RER:</strong> ${rerPct}%</span>
          <span><strong>POCUS Hypoechoic Pancreas:</strong> ${hypoechoic}</span>
          <span><strong>Hyperechoic Fat:</strong> ${hyperechoicFat}</span>
          <span><strong>Prayer Position:</strong> ${prayer}</span>
          <span><strong>Feline Triaditis:</strong> ${triaditis}</span>
        </div>

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary select-pancreatitis-op-btn" data-op-id="${id}">
            🔍 Open Workspace (${id})
          </button>
        </div>
      </article>
    `;
  }

  function renderActiveWorkspace(active) {
    if (!active) return;

    const titleEl = document.getElementById('active-op-title');
    const subtitleEl = document.getElementById('active-op-subtitle');
    const statusBadgeEl = document.getElementById('active-op-status-badge');
    const specPlEl = document.getElementById('active-spec-pl');
    const painEl = document.getElementById('active-pain-score');
    const rerEl = document.getElementById('active-enteral-rer');

    if (titleEl) {
      titleEl.textContent = `${active.id} — ${active.pet_name} (${active.species} · ${active.breed} · ${Number(active.weight_kg ?? 0).toFixed(1)} kg)`;
    }
    if (subtitleEl) {
      subtitleEl.textContent = `Case ${active.case_id} · Severity: ${active.severity_class}`;
    }
    if (statusBadgeEl) {
      statusBadgeEl.textContent = active.status;
      statusBadgeEl.setAttribute('data-badge-status', active.status);
    }
    if (specPlEl) {
      specPlEl.textContent = `${Number(active.spec_pl_ug_l ?? 0)} µg/L`;
    }
    if (painEl) {
      painEl.textContent = `${Number(active.modified_glasgow_pain_score ?? 0)}/24`;
    }
    if (rerEl) {
      rerEl.textContent = `${Number(active.enteral_rer_percent ?? 0)}% RER`;
    }

    const diag = active.diagnostic_plan || {};
    const diagSpecPl = document.getElementById('diag-spec-pl-result');
    const diagPocus = document.getElementById('diag-pocus-sign');
    const diagDuodenal = document.getElementById('diag-duodenal-corrugation');
    const diagEhbdo = document.getElementById('diag-ehbdo');
    const diagTriaditis = document.getElementById('diag-triaditis-note');
    const diagNpo = document.getElementById('diag-npo-warning');

    if (diagSpecPl) diagSpecPl.textContent = diag.spec_pl_assay_result || '';
    if (diagPocus) diagPocus.textContent = diag.pocus_sonographic_sign || '';
    if (diagDuodenal) diagDuodenal.textContent = diag.duodenal_corrugation_status || '';
    if (diagEhbdo) diagEhbdo.textContent = diag.biliary_obstruction_ehbdo || '';
    if (diagTriaditis) diagTriaditis.textContent = diag.triaditis_cobalamin_note || '';
    if (diagNpo) diagNpo.textContent = diag.outdated_npo_myth_warning || '';

    const nut = active.nutrition_plan || {};
    const planFuz = document.getElementById('plan-fuzapladib-text');
    const planAnti = document.getElementById('plan-antiemetic-text');
    const planAnalgesia = document.getElementById('plan-analgesia-text');
    const planTube = document.getElementById('plan-enteral-tube-text');
    const planRer = document.getElementById('plan-rer-schedule-text');
    const planFluid = document.getElementById('plan-fluid-text');

    if (planFuz) planFuz.textContent = nut.fuzapladib_protocol || '';
    if (planAnti) planAnti.textContent = nut.antiemetic_dual_protocol || '';
    if (planAnalgesia) planAnalgesia.textContent = nut.visceral_analgesia_cri || '';
    if (planTube) planTube.textContent = nut.early_enteral_tube_plan || '';
    if (planRer) planRer.textContent = nut.rer_caloric_ramp_schedule || '';
    if (planFluid) planFluid.textContent = nut.fluid_crystalloid_plan || '';

    const checksListEl = document.getElementById('enteral-checks-list');
    if (checksListEl) {
      const checks = Array.isArray(active.enteral_checks) ? active.enteral_checks : [];
      checksListEl.innerHTML = checks
        .map((chk) => {
          const cid = escapeHtml(chk.id || 'CHK');
          const by = escapeHtml(chk.recorded_by || 'Clinician');
          const cPl = Number(chk.spec_pl_ug_l ?? 0);
          const cPain = Number(chk.modified_glasgow_pain_score ?? 0);
          const cRer = Number(chk.enteral_rer_percent ?? 0);
          const cVom = escapeHtml(chk.vomiting_regurg_status || '');
          const cNote = escapeHtml(chk.clinical_note || '');
          return `<li class="pancreatitis-check-item"><strong>${cid} (${by})</strong> · Spec PL: ${cPl} &micro;g/L · Pain Score: ${cPain}/24 · Enteral: ${cRer}% RER · Emesis Status: ${cVom} — ${cNote}</li>`;
        })
        .join('');
    }

    const clearanceBanner = document.getElementById('pancreatitis-clearance-banner');
    const clearanceSummaryEl = document.getElementById('clearance-summary-text');
    if (clearanceBanner) {
      if (active.status === 'TOLERATING_ENTERAL_PAIN_FREE_CLEARED') {
        clearanceBanner.hidden = false;
        if (clearanceSummaryEl) {
          clearanceSummaryEl.textContent = `${active.id} (${active.pet_name}) — Discharged by ${active.discharged_by || 'Attending DACVECC'}: ${active.discharge_summary || 'Tolerating 100% RER enteral nutrition pain-free.'}`;
        }
      } else {
        clearanceBanner.hidden = true;
      }
    }
  }

  function renderAllOperations(ops, metrics) {
    currentOps = ops;
    updateOverviewMetrics(ops, metrics);

    const gridEl = document.getElementById('pancreatitis-operations-grid');
    if (gridEl) {
      if (ops.length === 0) {
        gridEl.innerHTML =
          '<p class="trust-empty-hint">No Pancreatitis operations match the selected filter.</p>';
      } else {
        gridEl.innerHTML = ops.map(renderOperationCardHtml).join('');
      }
    }

    const active = ops.find((item) => item.id === selectedOpId) || ops[0];
    if (active) {
      selectedOpId = active.id;
      renderActiveWorkspace(active);
    }
  }

  async function fetchOperations() {
    try {
      const statusFilter = (
        document.getElementById('pancreatitis-status-filter')?.value || ''
      ).trim();
      const params = new URLSearchParams();
      if (statusFilter) params.set('status', statusFilter);

      const qs = params.toString();
      const url = qs
        ? `/api/pancreatitis-cpl/operations?${qs}`
        : '/api/pancreatitis-cpl/operations';

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
      const res = await fetch(`/api/pancreatitis-cpl/operations/${encodeURIComponent(opId)}`, {
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
        renderActiveWorkspace(op);
        return;
      }
    } catch (_err) {
      // Fallback to cached operation
    }
    const cached = currentOps.find((item) => item.id === opId);
    if (cached) {
      renderActiveWorkspace(cached);
    }
  }

  function setupSSEButton() {
    const connectBtn = document.getElementById('connect-sse-btn');
    const badgeEl = document.getElementById('sse-status-badge');
    if (!connectBtn) return;

    connectBtn.addEventListener('click', () => {
      if (sseSource) {
        sseSource.close();
        sseSource = null;
      }
      if (badgeEl) {
        badgeEl.textContent = 'SSE Connected (Live)';
      }
      try {
        sseSource = new EventSource('/api/pancreatitis-cpl/stream');
        sseSource.addEventListener('message', () => {
          fetchOperations();
        });
        sseSource.addEventListener('pancreatitis_cpl_updated', () => {
          fetchOperations();
        });
      } catch (_err) {
        // Ignore EventSource error in headless environments
      }
    });
  }

  function setupFormsAndFilters() {
    const filterEl = document.getElementById('pancreatitis-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        fetchOperations();
      });
    }

    const stageForm = document.getElementById('stage-nutrition-form');
    if (stageForm) {
      stageForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const payload = {
          fuzapladib_protocol: (document.getElementById('stage-fuzapladib-input')?.value || '').trim(),
          antiemetic_dual_protocol: (
            document.getElementById('stage-antiemetic-input')?.value || ''
          ).trim(),
          visceral_analgesia_cri: (
            document.getElementById('stage-analgesia-input')?.value || ''
          ).trim(),
          early_enteral_tube_plan: (
            document.getElementById('stage-enteral-tube-input')?.value || ''
          ).trim(),
          rer_caloric_ramp_schedule: (
            document.getElementById('stage-rer-schedule-input')?.value || ''
          ).trim(),
          fluid_crystalloid_plan: (
            document.getElementById('stage-fluid-plan-input')?.value || ''
          ).trim(),
        };

        const btn = document.getElementById('stage-nutrition-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pancreatitis-cpl/operations/${encodeURIComponent(selectedOpId)}/stage-nutrition`,
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
            renderActiveWorkspace(updated);
            showToast(
              `Staged Fuzapladib, Maropitant & Early Enteral Nutrition for ${updated.id}`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showErrorBanner(errData.error || 'Failed to stage enteral nutrition');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const checkForm = document.getElementById('record-enteral-check-form');
    if (checkForm) {
      checkForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const specPl = parseInt(document.getElementById('check-spec-pl-input')?.value || '320', 10);
        const painScore = parseInt(
          document.getElementById('check-pain-score-input')?.value || '4',
          10
        );
        const rerPct = parseInt(
          document.getElementById('check-rer-percent-input')?.value || '75',
          10
        );
        const vomitingStatus = (
          document.getElementById('check-vomiting-input')?.value || 'ZERO_EMESIS_TOLERATING_NE_TUBE'
        ).trim();
        const recordedBy = (
          document.getElementById('check-recorded-by-input')?.value || 'Dr. Elena Rostova, DACVECC'
        ).trim();
        const clinicalNote = (document.getElementById('check-note-input')?.value || '').trim();

        const payload = {
          spec_pl_ug_l: isNaN(specPl) ? 320 : specPl,
          modified_glasgow_pain_score: isNaN(painScore) ? 4 : painScore,
          enteral_rer_percent: isNaN(rerPct) ? 75 : rerPct,
          vomiting_regurg_status: vomitingStatus,
          recorded_by: recordedBy,
          clinical_note: clinicalNote,
        };

        const btn = document.getElementById('record-enteral-check-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pancreatitis-cpl/operations/${encodeURIComponent(selectedOpId)}/record-check`,
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
            renderActiveWorkspace(updated);
            showToast(`Recorded Spec PL & Enteral RER check for ${updated.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            showErrorBanner(errData.error || 'Failed to record enteral check');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const clearForm = document.getElementById('clear-pancreatitis-form');
    if (clearForm) {
      clearForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const dischargedBy = (
          document.getElementById('clear-discharged-by-input')?.value ||
          'Dr. Elena Rostova, DACVECC'
        ).trim();
        const summary = (document.getElementById('clear-summary-input')?.value || '').trim();

        const payload = {
          discharged_by: dischargedBy,
          summary,
        };

        const btn = document.getElementById('clear-pancreatitis-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch(
            `/api/pancreatitis-cpl/operations/${encodeURIComponent(selectedOpId)}/clear-discharge`,
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
            renderActiveWorkspace(updated);
            showToast(`Cleared ${updated.id} for pain-free low-fat discharge`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            showErrorBanner(errData.error || 'Failed to clear pancreatitis discharge');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }

    const createForm = document.getElementById('create-pancreatitis-form');
    if (createForm) {
      createForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        showErrorBanner('');

        const payload = {
          case_id: (document.getElementById('create-case-id')?.value || 'CASE-PANC-104').trim(),
          pet_name: (document.getElementById('create-pet-name')?.value || '').trim(),
          species: (document.getElementById('create-species')?.value || 'Canine').trim(),
          breed: (document.getElementById('create-breed')?.value || 'Mixed').trim(),
          weight_kg: parseFloat(document.getElementById('create-weight-kg')?.value || '7.4'),
          severity_class: (
            document.getElementById('create-severity-class')?.value ||
            'SEVERE_NECROTIZING_SIRS_MODS'
          ).trim(),
          spec_pl_ug_l: parseInt(document.getElementById('create-spec-pl')?.value || '780', 10),
          modified_glasgow_pain_score: parseInt(
            document.getElementById('create-pain-score')?.value || '13',
            10
          ),
          enteral_rer_percent: 0,
          pocus_hypoechoic_pancreas: Boolean(
            document.getElementById('create-pocus-hypoechoic')?.checked
          ),
          hyperechoic_peripancreatic_fat: Boolean(
            document.getElementById('create-hyperechoic-fat')?.checked
          ),
          prayer_position_present: Boolean(
            document.getElementById('create-prayer-position')?.checked
          ),
          feline_triaditis_suspected: Boolean(
            document.getElementById('create-triaditis')?.checked
          ),
        };

        const btn = document.getElementById('create-pancreatitis-btn');
        if (btn) btn.disabled = true;
        try {
          const res = await fetch('/api/pancreatitis-cpl/operations', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });
          if (res.ok) {
            const created = await res.json();
            selectedOpId = created.id;
            await fetchOperations();
            renderActiveWorkspace(created);
            showToast(`Created Pancreatitis operation ${created.id}`, 'success');
          } else {
            const errData = await res.json().catch(() => ({}));
            showErrorBanner(errData.error || 'Failed to create pancreatitis operation');
          }
        } finally {
          if (btn) btn.disabled = false;
        }
      });
    }
  }

  function setupCardInteractions() {
    const gridEl = document.getElementById('pancreatitis-operations-grid');
    if (!gridEl) return;

    gridEl.addEventListener('click', (e) => {
      const btn = e.target.closest('.select-pancreatitis-op-btn');
      if (btn) {
        const opId = btn.getAttribute('data-op-id');
        if (opId) {
          selectOperationById(opId);
        }
        return;
      }
      const card = e.target.closest('.pancreatitis-op-card');
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
