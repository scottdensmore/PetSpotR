/**
 * PetSpotR Livestock, Equine & Exotic Companion Large-Animal Containment
 * & Trailer Dispatch Cockpit Controller (Milestone 15.3)
 */

(function () {
  'use strict';

  let sseSource = null;
  let currentDispatches = [];

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
    const liveStatus = document.getElementById('equine-livestock-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  /**
   * Map LargeAnimalDispatchStatus to WCAG AAA badge CSS class
   */
  function getStatusBadgeClass(status) {
    switch (status) {
      case 'LOOSE_HERD_CORRIDOR_ALERT':
        return 'badge-eq-loose';
      case 'PORTABLE_CORRAL_STAGED':
        return 'badge-eq-corralled';
      case 'LOADED_SAFE_PADDOCK_CUSTODY':
        return 'badge-eq-loaded';
      default:
        return 'badge-eq-loose';
    }
  }

  /**
   * Map StampedeCollisionRisk to WCAG AAA badge CSS class
   */
  function getCollisionRiskBadgeClass(risk) {
    switch (risk) {
      case 'CRITICAL_HIGHWAY_STAMPEDE_RISK':
        return 'badge-collision-critical';
      case 'ELEVATED_PASTURE_PERIMETER_RISK':
        return 'badge-collision-elevated';
      case 'CONTAINED_PADDOCK_SAFE':
        return 'badge-collision-safe';
      default:
        return 'badge-collision-critical';
    }
  }

  /**
   * Update KPI summary counters
   */
  function updateKpiCounters(dispatches) {
    const totalEl = document.getElementById('kpi-eq-total');
    const stampedeEl = document.getElementById('kpi-eq-stampede');
    const corralledEl = document.getElementById('kpi-eq-corralled');
    const loadedEl = document.getElementById('kpi-eq-loaded');

    const total = dispatches.length;
    const stampede = dispatches.filter((d) => Boolean(d.highwayStampedeAlert)).length;
    const corralled = dispatches.filter(
      (d) => d.dispatchStatus === 'PORTABLE_CORRAL_STAGED'
    ).length;
    const loaded = dispatches.filter(
      (d) => d.dispatchStatus === 'LOADED_SAFE_PADDOCK_CUSTODY'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (stampedeEl) stampedeEl.textContent = String(stampede);
    if (corralledEl) corralledEl.textContent = String(corralled);
    if (loadedEl) loadedEl.textContent = String(loaded);
  }

  /**
   * Render a single EquineLivestockDispatch card
   */
  function renderDispatchCardHtml(d) {
    const dispatchId = escapeHtml(d.dispatchId || '');
    const animalName = escapeHtml(d.animalName || '');
    const animalCategory = escapeHtml(d.animalCategory || 'EQUINE_WARMBLOOD');
    const headCount = Number(d.headCount ?? 1);
    const weightKg = Number(d.estimatedWeightKg ?? 500);
    const breachLocation = escapeHtml(d.breachLocation || '');
    const status = escapeHtml(d.dispatchStatus || 'LOOSE_HERD_CORRIDOR_ALERT');
    const statusClass = getStatusBadgeClass(d.dispatchStatus);
    const collisionRisk = escapeHtml(d.collisionRisk || 'CRITICAL_HIGHWAY_STAMPEDE_RISK');
    const riskClass = getCollisionRiskBadgeClass(d.collisionRisk);
    const riskScore = Number(d.collisionRiskScore ?? 80);
    const readinessScore = Number(d.trailerReadinessScore ?? 0);

    const stampedeBannerHtml = d.highwayStampedeAlert
      ? `
        <div class="eq-stampede-alert-banner">
          🚨 HIGHWAY STAMPEDE &amp; SIREN BLACKOUT PROTOCOL: ${animalCategory} (${headCount} head · ${weightKg}kg) at ${breachLocation} — Enforce responder siren/air-horn blackout, deploy calm lead-mare sweet-feed bucket lure &amp; stage 12ft portable corral wing-panels.
        </div>
      `
      : '';

    const paddockBannerHtml = d.paddockClearanceCode
      ? `
        <div class="eq-paddock-banner">
          ✅ GOOSENECK TRAILER &amp; PADDOCK CUSTODY VERIFIED: ${escapeHtml(d.paddockClearanceCode)}
        </div>
      `
      : '';

    const steps = Array.isArray(d.containmentSteps) ? d.containmentSteps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const badgeClass = done ? 'badge-hop-match' : 'badge-hop-miss';
        const statusText = done ? 'STAGED' : 'PENDING';
        const category = escapeHtml(step.category || '');
        const title = escapeHtml(step.title || '');
        return `
          <div class="eq-step-item">
            <span class="badge ${badgeClass}">${statusText}</span>
            <strong class="eq-step-category">[${category}]</strong>
            <span class="eq-step-title">${title}</span>
          </div>
        `;
      })
      .join('');

    let actionsHtml = '';
    if (d.dispatchStatus !== 'LOADED_SAFE_PADDOCK_CUSTODY') {
      actionsHtml = `
        <button type="button" class="btn btn-sm btn-secondary btn-stage-eq-corral" data-dispatch-id="${dispatchId}">🚧 Stage 12ft Corral Panels &amp; Gooseneck Trailer</button>
        <button type="button" class="btn btn-sm btn-primary btn-complete-eq-load" data-dispatch-id="${dispatchId}">🚛 Complete Safe Trailer Load &amp; Paddock Custody</button>
      `;
    }

    return `
      <article class="trust-screening-card eq-dispatch-card" data-dispatch-id="${dispatchId}" data-dispatch-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${animalName} (${animalCategory} · ${headCount} head · ${weightKg}kg)</strong>
            <code class="trust-pet-id">${dispatchId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-eq-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-collision-risk ${riskClass}">${collisionRisk}</span>
          <span class="badge badge-eq-metric">Collision Risk: ${riskScore}/100 • Trailer Readiness: ${readinessScore}/100 • Breach: ${breachLocation}</span>
        </div>

        ${stampedeBannerHtml}
        ${paddockBannerHtml}

        <div class="eq-steps-list" aria-label="Large-Animal Corral &amp; Gooseneck Trailer Containment Checklist">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  /**
   * Render all dispatches in `#eq-dispatches-list`
   */
  function renderAllDispatches(dispatches) {
    currentDispatches = dispatches;
    updateKpiCounters(dispatches);

    const countBadge = document.getElementById('eq-dispatches-count-badge');
    if (countBadge) {
      countBadge.textContent = `${dispatches.length} Dispatches`;
    }

    const listEl = document.getElementById('eq-dispatches-list');
    if (!listEl) return;

    if (dispatches.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="eq-dispatches-empty-msg">No equine or livestock containment dispatches recorded yet.</p>';
      return;
    }

    listEl.innerHTML = dispatches.map(renderDispatchCardHtml).join('');
  }

  /**
   * Fetch all dispatches from `/api/v1/equine-livestock/dispatches`
   */
  async function fetchDispatches() {
    try {
      const res = await fetch('/api/v1/equine-livestock/dispatches', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const dispatches = Array.isArray(data.dispatches) ? data.dispatches : [];
      renderAllDispatches(dispatches);
      return dispatches;
    } catch (_err) {
      return [];
    }
  }

  /**
   * Submit a create dispatch request to `/api/v1/equine-livestock/dispatches`
   */
  async function submitCreateDispatchRequest(payload) {
    const res = await fetch('/api/v1/equine-livestock/dispatches', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchDispatches();
      showToast(
        `Evaluated large-animal dispatch ${created.dispatchId} for ${created.animalName} → ${created.dispatchStatus} (${created.collisionRisk})`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate equine livestock dispatch', 'error');
    return null;
  }

  /**
   * Setup Create Dispatch Form and Scenario Simulators
   */
  function setupCreateFormAndSimulators() {
    const form = document.getElementById('form-create-eq-dispatch');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petId = (document.getElementById('eq-pet-id')?.value || 'pet-2026-001').trim();
        const animalName = (
          document.getElementById('eq-animal-name')?.value || 'Valegro & Starlight'
        ).trim();
        const animalCategory = (
          document.getElementById('eq-animal-category')?.value || 'EQUINE_WARMBLOOD'
        ).trim();
        const headCount = parseInt(
          document.getElementById('eq-head-count')?.value || '2',
          10
        );
        const estimatedWeightKg = parseInt(
          document.getElementById('eq-weight-kg')?.value || '540',
          10
        );
        const breachLocation = (
          document.getElementById('eq-breach-location')?.value ||
          'SR-202 Snoqualmie Valley Bridle Corridor'
        ).trim();
        const stockTrailerDispatched = Boolean(
          document.getElementById('eq-trailer-dispatched')?.checked
        );
        const corralPanelsErected = Boolean(
          document.getElementById('eq-panels-erected')?.checked
        );

        const submitBtn = document.getElementById('btn-create-eq-dispatch');
        if (submitBtn) submitBtn.disabled = true;

        try {
          await submitCreateDispatchRequest({
            petId,
            animalName,
            animalCategory,
            headCount: Number.isNaN(headCount) ? 2 : headCount,
            estimatedWeightKg: Number.isNaN(estimatedWeightKg) ? 540 : estimatedWeightKg,
            breachLocation,
            stockTrailerDispatched,
            corralPanelsErected,
          });
        } catch (_err) {
          showToast('Network error evaluating equine livestock dispatch', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simThoroughbredBtn = document.getElementById('btn-sim-spooked-thoroughbred');
    if (simThoroughbredBtn) {
      simThoroughbredBtn.addEventListener('click', async () => {
        simThoroughbredBtn.disabled = true;
        try {
          const catEl = document.getElementById('eq-animal-category');
          const headEl = document.getElementById('eq-head-count');
          const weightEl = document.getElementById('eq-weight-kg');
          const locEl = document.getElementById('eq-breach-location');
          if (catEl) catEl.value = 'EQUINE_WARMBLOOD';
          if (headEl) headEl.value = '1';
          if (weightEl) weightEl.value = '520';
          if (locEl) locEl.value = 'SR-202 Milepost 14 State Highway Shoulder';

          await submitCreateDispatchRequest({
            petId: 'pet-2026-001',
            animalName: 'Thunderbolt',
            animalCategory: 'EQUINE_WARMBLOOD',
            headCount: 1,
            estimatedWeightKg: 520,
            breachLocation: 'SR-202 Milepost 14 State Highway Shoulder',
            stockTrailerDispatched: false,
            corralPanelsErected: false,
          });
        } finally {
          simThoroughbredBtn.disabled = false;
        }
      });
    }

    const simAlpacaBtn = document.getElementById('btn-sim-alpaca-herd-breach');
    if (simAlpacaBtn) {
      simAlpacaBtn.addEventListener('click', async () => {
        simAlpacaBtn.disabled = true;
        try {
          const catEl = document.getElementById('eq-animal-category');
          const headEl = document.getElementById('eq-head-count');
          const weightEl = document.getElementById('eq-weight-kg');
          const locEl = document.getElementById('eq-breach-location');
          if (catEl) catEl.value = 'CAMELID_ALPACA_HERD';
          if (headEl) headEl.value = '4';
          if (weightEl) weightEl.value = '180';
          if (locEl) locEl.value = 'Carnation Valley West Pasture Fence Breach';

          await submitCreateDispatchRequest({
            petId: 'pet-2026-002',
            animalName: 'Cusco Quartet',
            animalCategory: 'CAMELID_ALPACA_HERD',
            headCount: 4,
            estimatedWeightKg: 180,
            breachLocation: 'Carnation Valley West Pasture Fence Breach',
            stockTrailerDispatched: false,
            corralPanelsErected: false,
          });
        } finally {
          simAlpacaBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup card action buttons in `#eq-dispatches-list`
   */
  function setupDispatchCardActions() {
    const listEl = document.getElementById('eq-dispatches-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const stageBtn = e.target.closest('.btn-stage-eq-corral');
      const loadBtn = e.target.closest('.btn-complete-eq-load');

      const btn = stageBtn || loadBtn;
      if (!btn) return;

      const dispatchId = btn.getAttribute('data-dispatch-id');
      if (!dispatchId) return;

      btn.disabled = true;
      try {
        if (stageBtn) {
          const res = await fetch(
            `/api/v1/equine-livestock/dispatches/${encodeURIComponent(dispatchId)}/contain`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                stageCorralAndTrailer: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchDispatches();
            showToast(
              `Staged 12ft corral panels & gooseneck stock trailer for ${dispatchId} → ${updated.dispatchStatus} (Trailer Readiness: ${updated.trailerReadinessScore}/100)`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to stage corral panels and trailer', 'error');
          }
        } else if (loadBtn) {
          const res = await fetch(
            `/api/v1/equine-livestock/dispatches/${encodeURIComponent(dispatchId)}/contain`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                completeSafeTrailerLoad: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchDispatches();
            showToast(
              `Completed safe trailer load & paddock custody ${dispatchId} → ${updated.dispatchStatus} (${updated.paddockClearanceCode})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to complete safe trailer load', 'error');
          }
        }
      } catch (_err) {
        showToast('Network error updating equine livestock dispatch', 'error');
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
   * Subscribe to SSE stream `/api/v1/equine-livestock/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/equine-livestock/stream');
      const refresh = () => {
        fetchDispatches();
      };
      sseSource.addEventListener('livestock_dispatch_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'livestock_dispatch_updated') {
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
    setupDispatchCardActions();
    fetchDispatches();
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
