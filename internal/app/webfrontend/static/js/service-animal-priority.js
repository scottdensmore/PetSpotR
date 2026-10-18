/**
 * PetSpotR Working Dog & Service Animal ADA / Critical Medical-Alert Handler
 * Priority Recovery Cockpit Controller (Milestone 15.2)
 */

(function () {
  'use strict';

  let sseSource = null;
  let currentCases = [];

  /**
   * Escape untrusted text for safe HTML interpolation
   */
  function escapeHtml(str) {
    if (str === null || str === undefined) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  /**
   * Display toast notification & update live status banner
   */
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
    const liveStatus = document.getElementById('service-animal-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  /**
   * Map ServiceRecoveryStatus to WCAG AAA badge CSS class
   */
  function getStatusBadgeClass(status) {
    switch (status) {
      case 'HANDLER_MEDICAL_ESCALATED':
        return 'badge-sa-escalated';
      case 'TACTICAL_VEST_INTERCEPT_STAGED':
        return 'badge-sa-staged';
      case 'REUNITED_ADA_TEAM_RESTORED':
        return 'badge-sa-restored';
      default:
        return 'badge-sa-escalated';
    }
  }

  /**
   * Map HandlerDependencyUrgency to WCAG AAA badge CSS class
   */
  function getUrgencyBadgeClass(urgency) {
    switch (urgency) {
      case 'LIFE_CRITICAL_MEDICAL_DEPENDENCY':
        return 'badge-urgency-critical';
      case 'HIGH_MOBILITY_GUIDE_DEPENDENCY':
        return 'badge-urgency-mobility';
      case 'HANDLER_TEAM_STABILIZED':
        return 'badge-urgency-stabilized';
      default:
        return 'badge-urgency-critical';
    }
  }

  /**
   * Update KPI summary counters
   */
  function updateKpiCounters(cases) {
    const totalEl = document.getElementById('kpi-sa-total');
    const criticalEl = document.getElementById('kpi-sa-critical');
    const standbyEl = document.getElementById('kpi-sa-standby');
    const restoredEl = document.getElementById('kpi-sa-restored');

    const total = cases.length;
    const critical = cases.filter(
      (c) => c.dependencyUrgency === 'LIFE_CRITICAL_MEDICAL_DEPENDENCY'
    ).length;
    const standby = cases.filter((c) => Boolean(c.handlerMedicalStandbyAlert)).length;
    const restored = cases.filter(
      (c) => c.recoveryStatus === 'REUNITED_ADA_TEAM_RESTORED'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (standbyEl) standbyEl.textContent = String(standby);
    if (restoredEl) restoredEl.textContent = String(restored);
  }

  /**
   * Render a single ServiceAnimalCase card
   */
  function renderServiceAnimalCaseCardHtml(c) {
    const caseId = escapeHtml(c.caseId || '');
    const petName = escapeHtml(c.petName || '');
    const species = escapeHtml(c.species || 'DOG');
    const handlerName = escapeHtml(c.handlerName || '');
    const serviceTaskType = escapeHtml(c.serviceTaskType || 'DIABETIC_HYPOGLYCEMIA_ALERT');
    const vestEquipmentState = escapeHtml(c.vestEquipmentState || 'ADA_CAPE_ENTANGLED_HAZARD');
    const hoursSeparated = Number(c.hoursSeparated ?? 8);
    const status = escapeHtml(c.recoveryStatus || 'HANDLER_MEDICAL_ESCALATED');
    const statusClass = getStatusBadgeClass(c.recoveryStatus);
    const dependencyUrgency = escapeHtml(
      c.dependencyUrgency || 'LIFE_CRITICAL_MEDICAL_DEPENDENCY'
    );
    const urgencyClass = getUrgencyBadgeClass(c.dependencyUrgency);
    const urgencyScore = Number(c.medicalUrgencyScore ?? 90);
    const readinessScore = Number(c.interceptReadinessScore ?? 0);

    const standbyBannerHtml = c.handlerMedicalStandbyAlert
      ? `
        <div class="sa-medical-standby-banner">
          🚨 ADA MEDICAL-ALERT HANDLER STANDBY &amp; SNAG HAZARD: ${serviceTaskType} (${handlerName}) with ${vestEquipmentState} — Link handler to paramedic standby &amp; sweep brush/fences for cape snags.
        </div>
      `
      : '';

    const certBannerHtml = c.adaRestorationCertificate
      ? `
        <div class="sa-restoration-banner">
          ✅ ADA SERVICE ANIMAL TEAM RESTORED: ${escapeHtml(c.adaRestorationCertificate)}
        </div>
      `
      : '';

    const steps = Array.isArray(c.recoverySteps) ? c.recoverySteps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const badgeClass = done ? 'badge-hop-match' : 'badge-hop-miss';
        const statusText = done ? 'VERIFIED' : 'PENDING';
        const category = escapeHtml(step.category || '');
        const title = escapeHtml(step.title || '');
        return `
          <div class="sa-step-item">
            <span class="badge ${badgeClass}">${statusText}</span>
            <strong class="sa-step-category">[${category}]</strong>
            <span class="sa-step-title">${title}</span>
          </div>
        `;
      })
      .join('');

    let actionsHtml = '';
    if (c.recoveryStatus !== 'REUNITED_ADA_TEAM_RESTORED') {
      actionsHtml = `
        <button type="button" class="btn btn-sm btn-secondary btn-stage-sa-intercept" data-case-id="${caseId}">📡 Stage EMS Handler Standby &amp; Non-Confrontational Vest Intercept</button>
        <button type="button" class="btn btn-sm btn-primary btn-restore-sa-team" data-case-id="${caseId}">🦮 Restore Protected ADA Service Animal Team</button>
      `;
    }

    return `
      <article class="trust-screening-card sa-case-card" data-case-id="${caseId}" data-recovery-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${serviceTaskType} · Handler: ${handlerName})</strong>
            <code class="trust-pet-id">${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-sa-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-sa-urgency ${urgencyClass}">${dependencyUrgency}</span>
          <span class="badge badge-sa-metric">Medical Urgency: ${urgencyScore}/100 • Intercept Readiness: ${readinessScore}/100 • Equipment: ${vestEquipmentState} • Separated: ${hoursSeparated}h</span>
        </div>

        ${standbyBannerHtml}
        ${certBannerHtml}

        <div class="sa-steps-list" aria-label="ADA Handler Standby &amp; Non-Confrontational Vest Intercept Checklist">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  /**
   * Render all service animal cases in `#sa-cases-list`
   */
  function renderAllCases(cases) {
    currentCases = cases;
    updateKpiCounters(cases);

    const countBadge = document.getElementById('sa-cases-count-badge');
    if (countBadge) {
      countBadge.textContent = `${cases.length} Cases`;
    }

    const listEl = document.getElementById('sa-cases-list');
    if (!listEl) return;

    if (cases.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="sa-cases-empty-msg">No service animal priority cases recorded yet.</p>';
      return;
    }

    listEl.innerHTML = cases.map(renderServiceAnimalCaseCardHtml).join('');
  }

  /**
   * Fetch all service animal cases from `/api/v1/service-animal-priority/cases`
   */
  async function fetchCases() {
    try {
      const res = await fetch('/api/v1/service-animal-priority/cases', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const cases = Array.isArray(data.cases) ? data.cases : [];
      renderAllCases(cases);
      return cases;
    } catch (_err) {
      return [];
    }
  }

  /**
   * Submit a create service animal case request to `/api/v1/service-animal-priority/cases`
   */
  async function submitCreateCaseRequest(payload) {
    const res = await fetch('/api/v1/service-animal-priority/cases', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchCases();
      showToast(
        `Evaluated service animal case ${created.caseId} for ${created.petName} → ${created.recoveryStatus} (${created.dependencyUrgency})`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate service animal priority case', 'error');
    return null;
  }

  /**
   * Setup Evaluate Service Animal Case Form and Scenario Simulators
   */
  function setupCreateFormAndSimulators() {
    const form = document.getElementById('form-create-sa-case');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petId = (document.getElementById('sa-pet-id')?.value || 'pet-2026-001').trim();
        const petName = (document.getElementById('sa-pet-name')?.value || 'Apollo').trim();
        const species = (document.getElementById('sa-pet-species')?.value || 'DOG').trim();
        const handlerName = (
          document.getElementById('sa-handler-name')?.value || 'Maya Lin (T1D Handler)'
        ).trim();
        const serviceTaskType = (
          document.getElementById('sa-task-type')?.value || 'DIABETIC_HYPOGLYCEMIA_ALERT'
        ).trim();
        const vestEquipmentState = (
          document.getElementById('sa-vest-state')?.value || 'ADA_CAPE_ENTANGLED_HAZARD'
        ).trim();
        const hoursSeparated = parseInt(
          document.getElementById('sa-hours-separated')?.value || '8',
          10
        );
        const handlerStandbyLinked = Boolean(
          document.getElementById('sa-standby-linked')?.checked
        );
        const recallCueBroadcasted = Boolean(
          document.getElementById('sa-recall-broadcasted')?.checked
        );

        const submitBtn = document.getElementById('btn-create-sa-case');
        if (submitBtn) submitBtn.disabled = true;

        try {
          await submitCreateCaseRequest({
            petId,
            petName,
            species,
            handlerName,
            serviceTaskType,
            vestEquipmentState,
            hoursSeparated: Number.isNaN(hoursSeparated) ? 8 : hoursSeparated,
            handlerStandbyLinked,
            recallCueBroadcasted,
          });
        } catch (_err) {
          showToast('Network error evaluating service animal case', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simDiabeticBtn = document.getElementById('btn-sim-diabetic-alert-dog');
    if (simDiabeticBtn) {
      simDiabeticBtn.addEventListener('click', async () => {
        simDiabeticBtn.disabled = true;
        try {
          const taskEl = document.getElementById('sa-task-type');
          const vestEl = document.getElementById('sa-vest-state');
          const hoursEl = document.getElementById('sa-hours-separated');
          if (taskEl) taskEl.value = 'DIABETIC_HYPOGLYCEMIA_ALERT';
          if (vestEl) vestEl.value = 'ADA_CAPE_ENTANGLED_HAZARD';
          if (hoursEl) hoursEl.value = '9';

          await submitCreateCaseRequest({
            petId: 'pet-2026-001',
            petName: 'Koda',
            species: 'DOG',
            handlerName: 'Dr. Elena Rostova (T1D Handler)',
            serviceTaskType: 'DIABETIC_HYPOGLYCEMIA_ALERT',
            vestEquipmentState: 'ADA_CAPE_ENTANGLED_HAZARD',
            hoursSeparated: 9,
            handlerStandbyLinked: false,
            recallCueBroadcasted: false,
          });
        } finally {
          simDiabeticBtn.disabled = false;
        }
      });
    }

    const simGuideDogBtn = document.getElementById('btn-sim-guide-dog-harness');
    if (simGuideDogBtn) {
      simGuideDogBtn.addEventListener('click', async () => {
        simGuideDogBtn.disabled = true;
        try {
          const taskEl = document.getElementById('sa-task-type');
          const vestEl = document.getElementById('sa-vest-state');
          const hoursEl = document.getElementById('sa-hours-separated');
          if (taskEl) taskEl.value = 'BLIND_GUIDE_NAVIGATION';
          if (vestEl) vestEl.value = 'HIGH_VIS_WORKING_HARNESS';
          if (hoursEl) hoursEl.value = '4';

          await submitCreateCaseRequest({
            petId: 'pet-2026-002',
            petName: 'Beacon',
            species: 'DOG',
            handlerName: 'Marcus Vance (Guide Handler)',
            serviceTaskType: 'BLIND_GUIDE_NAVIGATION',
            vestEquipmentState: 'HIGH_VIS_WORKING_HARNESS',
            hoursSeparated: 4,
            handlerStandbyLinked: false,
            recallCueBroadcasted: false,
          });
        } finally {
          simGuideDogBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup card action buttons in `#sa-cases-list`
   */
  function setupCaseCardActions() {
    const listEl = document.getElementById('sa-cases-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const stageBtn = e.target.closest('.btn-stage-sa-intercept');
      const restoreBtn = e.target.closest('.btn-restore-sa-team');

      const btn = stageBtn || restoreBtn;
      if (!btn) return;

      const caseId = btn.getAttribute('data-case-id');
      if (!caseId) return;

      btn.disabled = true;
      try {
        if (stageBtn) {
          const res = await fetch(
            `/api/v1/service-animal-priority/cases/${encodeURIComponent(caseId)}/recover`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                stageTacticalIntercept: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchCases();
            showToast(
              `Staged tactical vest intercept & EMS standby for ${caseId} → ${updated.recoveryStatus} (Intercept Readiness: ${updated.interceptReadinessScore}/100)`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to stage tactical intercept', 'error');
          }
        } else if (restoreBtn) {
          const res = await fetch(
            `/api/v1/service-animal-priority/cases/${encodeURIComponent(caseId)}/recover`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                restoreAdaTeamReunion: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchCases();
            showToast(
              `Restored ADA service animal team ${caseId} → ${updated.recoveryStatus} (${updated.adaRestorationCertificate})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to restore ADA service animal team', 'error');
          }
        }
      } catch (_err) {
        showToast('Network error updating service animal case', 'error');
      } finally {
        btn.disabled = false;
      }
    });
  }

  /**
   * Setup theme toggle button (`#theme-toggle`)
   */
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

  /**
   * Subscribe to SSE stream `/api/v1/service-animal-priority/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/service-animal-priority/stream');
      const refresh = () => {
        fetchCases();
      };
      sseSource.addEventListener('service_animal_case_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'service_animal_case_updated') {
            refresh();
          }
        } catch (_e) {
          // Ignore non-JSON keepalive comments
        }
      };
    } catch (_err) {
      // Ignore SSE errors in headless environments
    }
  }

  /**
   * Initialize controller on DOMContentLoaded
   */
  function init() {
    setupThemeToggle();
    setupCreateFormAndSimulators();
    setupCaseCardActions();
    fetchCases();
    setupSSEStream();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  window.addEventListener('beforeunload', () => {
    if (sseSource) {
      sseSource.close();
    }
  });
})();
