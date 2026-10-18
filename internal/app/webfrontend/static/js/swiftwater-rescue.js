/**
 * PetSpotR Aquatic, Floodwater & Ice-Rescue Swiftwater Canine/Feline
 * Extraction Cockpit Controller (Milestone 15.1)
 */

(function () {
  'use strict';

  let sseSource = null;
  let currentOps = [];

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
    const liveStatus = document.getElementById('swiftwater-rescue-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  /**
   * Map SwiftwaterExtractionStatus to WCAG AAA badge CSS class
   */
  function getStatusBadgeClass(status) {
    switch (status) {
      case 'STRANDED_IN_SWIFTWATER':
        return 'badge-sw-stranded';
      case 'THROW_BAG_RIGGED_APPROACH':
        return 'badge-sw-rigged';
      case 'EXTRACTED_WARMING_STABLE':
        return 'badge-sw-extracted';
      default:
        return 'badge-sw-stranded';
    }
  }

  /**
   * Map HypothermiaHydroRisk to WCAG AAA badge CSS class
   */
  function getHydroRiskBadgeClass(risk) {
    switch (risk) {
      case 'EXTREME_ICE_HYPOTHERMIA_RISK':
        return 'badge-hydro-extreme';
      case 'ELEVATED_FLOOD_CURRENT_RISK':
        return 'badge-hydro-elevated';
      case 'SHORELINE_REWARMED_SAFE':
        return 'badge-hydro-safe';
      default:
        return 'badge-hydro-extreme';
    }
  }

  /**
   * Update KPI summary counters
   */
  function updateKpiCounters(ops) {
    const totalEl = document.getElementById('kpi-sw-total');
    const hypothermiaEl = document.getElementById('kpi-sw-hypothermia');
    const coldshockEl = document.getElementById('kpi-sw-coldshock');
    const extractedEl = document.getElementById('kpi-sw-extracted');

    const total = ops.length;
    const hypothermia = ops.filter(
      (op) => op.hydroRisk === 'EXTREME_ICE_HYPOTHERMIA_RISK'
    ).length;
    const coldshock = ops.filter((op) => Boolean(op.coldShockAlert)).length;
    const extracted = ops.filter(
      (op) => op.extractionStatus === 'EXTRACTED_WARMING_STABLE'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (hypothermiaEl) hypothermiaEl.textContent = String(hypothermia);
    if (coldshockEl) coldshockEl.textContent = String(coldshock);
    if (extractedEl) extractedEl.textContent = String(extracted);
  }

  /**
   * Render a single SwiftwaterRescueOp card
   */
  function renderSwiftwaterOpCardHtml(op) {
    const opId = escapeHtml(op.opId || '');
    const petName = escapeHtml(op.petName || '');
    const species = escapeHtml(op.species || 'DOG');
    const waterwayName = escapeHtml(op.waterwayName || '');
    const hazardEnvironment = escapeHtml(op.hazardEnvironment || 'FRACTURED_LAKE_ICE_SHELF');
    const waterTemp = Number(op.waterTempFahrenheit ?? 34);
    const currentKnots = Number(op.currentVelocityKnots ?? 1.5).toFixed(1);
    const status = escapeHtml(op.extractionStatus || 'STRANDED_IN_SWIFTWATER');
    const statusClass = getStatusBadgeClass(op.extractionStatus);
    const hydroRisk = escapeHtml(op.hydroRisk || 'EXTREME_ICE_HYPOTHERMIA_RISK');
    const riskClass = getHydroRiskBadgeClass(op.hydroRisk);
    const riskScore = Number(op.hypothermiaRiskScore ?? 80);
    const readinessScore = Number(op.riggingReadinessScore ?? 0);

    const coldShockBannerHtml = op.coldShockAlert
      ? `
        <div class="sw-coldshock-alert-banner">
          🥶 COLD SHOCK &amp; CIRCUM-RESCUE COLLAPSE ALERT: ${waterTemp}°F water (${currentKnots} kts) at ${waterwayName} — Execute horizontal haul &amp; active core rewarming immediately.
        </div>
      `
      : '';

    const clearanceBannerHtml = op.extractionClearanceCode
      ? `
        <div class="sw-clearance-banner">
          ✅ SWIFTWATER EXTRACTION &amp; REWARMING VERIFIED: ${escapeHtml(op.extractionClearanceCode)}
        </div>
      `
      : '';

    const steps = Array.isArray(op.protocolSteps) ? op.protocolSteps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const badgeClass = done ? 'badge-hop-match' : 'badge-hop-miss';
        const statusText = done ? 'COMPLETE' : 'PENDING';
        const category = escapeHtml(step.category || '');
        const title = escapeHtml(step.title || '');
        return `
          <div class="sw-step-item">
            <span class="badge ${badgeClass}">${statusText}</span>
            <strong class="sw-step-category">[${category}]</strong>
            <span class="sw-step-title">${title}</span>
          </div>
        `;
      })
      .join('');

    let actionsHtml = '';
    if (op.extractionStatus !== 'EXTRACTED_WARMING_STABLE') {
      actionsHtml = `
        <button type="button" class="btn btn-sm btn-secondary btn-deploy-sw-rigging" data-op-id="${opId}">🪢 Deploy Throw-Bag Rigging &amp; Inflatable Ice Sled</button>
        <button type="button" class="btn btn-sm btn-primary btn-complete-sw-extraction" data-op-id="${opId}">🔥 Complete Shoreline Extraction &amp; Bair Hugger Rewarming</button>
      `;
    }

    return `
      <article class="trust-screening-card sw-op-card" data-op-id="${opId}" data-extraction-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${waterwayName} · ${hazardEnvironment})</strong>
            <code class="trust-pet-id">${opId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-sw-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-hydro-risk ${riskClass}">${hydroRisk}</span>
          <span class="badge badge-sw-metric">Hypothermia Risk: ${riskScore}/100 • Rigging Readiness: ${readinessScore}/100 • Water Temp: ${waterTemp}°F • Current: ${currentKnots} kts</span>
        </div>

        ${coldShockBannerHtml}
        ${clearanceBannerHtml}

        <div class="sw-steps-list" aria-label="NFPA 1670 Swiftwater Rigging &amp; Rewarming Checklist">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  /**
   * Render all swiftwater ops in `#sw-ops-list`
   */
  function renderAllOps(ops) {
    currentOps = ops;
    updateKpiCounters(ops);

    const countBadge = document.getElementById('sw-ops-count-badge');
    if (countBadge) {
      countBadge.textContent = `${ops.length} Ops`;
    }

    const listEl = document.getElementById('sw-ops-list');
    if (!listEl) return;

    if (ops.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="sw-ops-empty-msg">No swiftwater or ice-rescue operations recorded yet.</p>';
      return;
    }

    listEl.innerHTML = ops.map(renderSwiftwaterOpCardHtml).join('');
  }

  /**
   * Fetch all swiftwater ops from `/api/v1/swiftwater-rescue/ops`
   */
  async function fetchOps() {
    try {
      const res = await fetch('/api/v1/swiftwater-rescue/ops', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const ops = Array.isArray(data.ops) ? data.ops : [];
      renderAllOps(ops);
      return ops;
    } catch (_err) {
      return [];
    }
  }

  /**
   * Submit a create swiftwater op request to `/api/v1/swiftwater-rescue/ops`
   */
  async function submitCreateOpRequest(payload) {
    const res = await fetch('/api/v1/swiftwater-rescue/ops', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchOps();
      showToast(
        `Evaluated swiftwater op ${created.opId} for ${created.petName} → ${created.extractionStatus} (${created.hydroRisk})`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate swiftwater rescue op', 'error');
    return null;
  }

  /**
   * Setup Evaluate Swiftwater Op Form and Scenario Simulators
   */
  function setupCreateFormAndSimulators() {
    const form = document.getElementById('form-create-sw-op');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petId = (document.getElementById('sw-pet-id')?.value || 'pet-2026-001').trim();
        const petName = (document.getElementById('sw-pet-name')?.value || 'Yukon').trim();
        const species = (document.getElementById('sw-pet-species')?.value || 'DOG').trim();
        const waterwayName = (
          document.getElementById('sw-waterway-name')?.value || 'Green Lake North Ice Shelf'
        ).trim();
        const hazardEnvironment = (
          document.getElementById('sw-hazard-env')?.value || 'FRACTURED_LAKE_ICE_SHELF'
        ).trim();
        const waterTempFahrenheit = parseInt(
          document.getElementById('sw-water-temp')?.value || '34',
          10
        );
        const currentVelocityKnots = parseFloat(
          document.getElementById('sw-current-knots')?.value || '1.5'
        );
        const pfdThrowBagDeployed = Boolean(
          document.getElementById('sw-throwbag-deployed')?.checked
        );
        const warmingBlanketReady = Boolean(
          document.getElementById('sw-warming-ready')?.checked
        );

        const submitBtn = document.getElementById('btn-create-sw-op');
        if (submitBtn) submitBtn.disabled = true;

        try {
          await submitCreateOpRequest({
            petId,
            petName,
            species,
            waterwayName,
            hazardEnvironment,
            waterTempFahrenheit: Number.isNaN(waterTempFahrenheit) ? 34 : waterTempFahrenheit,
            currentVelocityKnots: Number.isNaN(currentVelocityKnots) ? 1.5 : currentVelocityKnots,
            pfdThrowBagDeployed,
            warmingBlanketReady,
          });
        } catch (_err) {
          showToast('Network error evaluating swiftwater rescue op', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simIceBtn = document.getElementById('btn-sim-ice-breakthrough');
    if (simIceBtn) {
      simIceBtn.addEventListener('click', async () => {
        simIceBtn.disabled = true;
        try {
          const waterwayEl = document.getElementById('sw-waterway-name');
          const hazardEl = document.getElementById('sw-hazard-env');
          const tempEl = document.getElementById('sw-water-temp');
          const knotsEl = document.getElementById('sw-current-knots');
          if (waterwayEl) waterwayEl.value = 'Bitter Lake Fractured Ice Shelf';
          if (hazardEl) hazardEl.value = 'FRACTURED_LAKE_ICE_SHELF';
          if (tempEl) tempEl.value = '33';
          if (knotsEl) knotsEl.value = '1.5';

          await submitCreateOpRequest({
            petId: 'pet-2026-001',
            petName: 'Kodiak',
            species: 'DOG',
            waterwayName: 'Bitter Lake Fractured Ice Shelf',
            hazardEnvironment: 'FRACTURED_LAKE_ICE_SHELF',
            waterTempFahrenheit: 33,
            currentVelocityKnots: 1.5,
            pfdThrowBagDeployed: false,
            warmingBlanketReady: false,
          });
        } finally {
          simIceBtn.disabled = false;
        }
      });
    }

    const simFloodBtn = document.getElementById('btn-sim-flash-flood-culvert');
    if (simFloodBtn) {
      simFloodBtn.addEventListener('click', async () => {
        simFloodBtn.disabled = true;
        try {
          const waterwayEl = document.getElementById('sw-waterway-name');
          const hazardEl = document.getElementById('sw-hazard-env');
          const tempEl = document.getElementById('sw-water-temp');
          const knotsEl = document.getElementById('sw-current-knots');
          if (waterwayEl) waterwayEl.value = 'Longfellow Creek Flash Flood Culvert';
          if (hazardEl) hazardEl.value = 'FLASH_FLOOD_STORM_CHANNEL';
          if (tempEl) tempEl.value = '46';
          if (knotsEl) knotsEl.value = '8.0';

          await submitCreateOpRequest({
            petId: 'pet-2026-002',
            petName: 'Cascade',
            species: 'CAT',
            waterwayName: 'Longfellow Creek Flash Flood Culvert',
            hazardEnvironment: 'FLASH_FLOOD_STORM_CHANNEL',
            waterTempFahrenheit: 46,
            currentVelocityKnots: 8.0,
            pfdThrowBagDeployed: false,
            warmingBlanketReady: false,
          });
        } finally {
          simFloodBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup card action buttons in `#sw-ops-list`
   */
  function setupOpCardActions() {
    const listEl = document.getElementById('sw-ops-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const deployBtn = e.target.closest('.btn-deploy-sw-rigging');
      const completeBtn = e.target.closest('.btn-complete-sw-extraction');

      const btn = deployBtn || completeBtn;
      if (!btn) return;

      const opId = btn.getAttribute('data-op-id');
      if (!opId) return;

      btn.disabled = true;
      try {
        if (deployBtn) {
          const res = await fetch(
            `/api/v1/swiftwater-rescue/ops/${encodeURIComponent(opId)}/extract`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                deployRiggingAndFlotation: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchOps();
            showToast(
              `Deployed throw-bag rigging & inflatable ice sled for ${opId} → ${updated.extractionStatus} (Rigging Readiness: ${updated.riggingReadinessScore}/100)`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to deploy swiftwater rigging', 'error');
          }
        } else if (completeBtn) {
          const res = await fetch(
            `/api/v1/swiftwater-rescue/ops/${encodeURIComponent(opId)}/extract`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                completeShorelineExtraction: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchOps();
            showToast(
              `Completed shoreline extraction & rewarming ${opId} → ${updated.extractionStatus} (${updated.extractionClearanceCode})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to complete swiftwater extraction', 'error');
          }
        }
      } catch (_err) {
        showToast('Network error updating swiftwater rescue op', 'error');
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
   * Subscribe to SSE stream `/api/v1/swiftwater-rescue/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/swiftwater-rescue/stream');
      const refresh = () => {
        fetchOps();
      };
      sseSource.addEventListener('swiftwater_op_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'swiftwater_op_updated') {
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
    setupOpCardActions();
    fetchOps();
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
