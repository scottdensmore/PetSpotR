/**
 * PetSpotR Municipal 311 / Animal Control Dispatch CAD Bridge, Roadway Hazard Alerts &
 * Good Samaritan Safe-Capture Routing Cockpit Controller (Milestone 14.2)
 */

(function () {
  'use strict';

  let sseSource = null;
  let currentIncidents = [];

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
    const liveStatus = document.getElementById('municipal-cad-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  /**
   * Map CADDispatchStatus to WCAG AAA badge CSS class
   */
  function getStatusBadgeClass(status) {
    switch (status) {
      case 'INBOUND_311_TRIAGE':
        return 'badge-cad-triage';
      case 'HIGHWAY_CONTAINMENT_DISPATCHED':
        return 'badge-cad-containment';
      case 'RESOLVED_SAFE_CUSTODY':
        return 'badge-cad-resolved';
      default:
        return 'badge-cad-triage';
    }
  }

  /**
   * Map RoadwayHazardSeverity to WCAG AAA badge CSS class
   */
  function getSeverityBadgeClass(severity) {
    switch (severity) {
      case 'FREEWAY_MEDIAN_CRITICAL':
        return 'badge-hazard-freeway';
      case 'ARTERIAL_TRAFFIC_HIGH':
        return 'badge-hazard-arterial';
      case 'CONTAINED_CORRIDOR_SAFE':
        return 'badge-hazard-safe';
      default:
        return 'badge-hazard-freeway';
    }
  }

  /**
   * Update KPI summary counters
   */
  function updateKpiCounters(incidents) {
    const totalEl = document.getElementById('kpi-cad-total');
    const criticalEl = document.getElementById('kpi-cad-critical');
    const noChaseEl = document.getElementById('kpi-cad-no-chase');
    const resolvedEl = document.getElementById('kpi-cad-resolved');

    const total = incidents.length;
    const critical = incidents.filter(
      (inc) => inc.hazardSeverity === 'FREEWAY_MEDIAN_CRITICAL'
    ).length;
    const noChase = incidents.filter((inc) => Boolean(inc.doNotChaseAlert)).length;
    const resolved = incidents.filter(
      (inc) => inc.dispatchStatus === 'RESOLVED_SAFE_CUSTODY'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (noChaseEl) noChaseEl.textContent = String(noChase);
    if (resolvedEl) resolvedEl.textContent = String(resolved);
  }

  /**
   * Render a single MunicipalCADIncident card
   */
  function renderIncidentCardHtml(inc) {
    const incidentId = escapeHtml(inc.incidentId || '');
    const callNum = escapeHtml(inc.cadCallNumber || '');
    const petName = escapeHtml(inc.petName || '');
    const species = escapeHtml(inc.species || 'DOG');
    const roadway = escapeHtml(inc.roadwayName || '');
    const speedMph = Number(inc.speedLimitMph ?? 45);
    const laneCount = Number(inc.laneCount ?? 2);
    const behavior = escapeHtml(inc.petBehaviorState || 'HUDDLED_IN_MEDIAN');
    const status = escapeHtml(inc.dispatchStatus || 'INBOUND_311_TRIAGE');
    const statusClass = getStatusBadgeClass(inc.dispatchStatus);
    const severity = escapeHtml(inc.hazardSeverity || 'FREEWAY_MEDIAN_CRITICAL');
    const severityClass = getSeverityBadgeClass(inc.hazardSeverity);
    const strikeRisk = Number(inc.trafficStrikeRiskScore ?? 80);
    const readiness = Number(inc.containmentReadinessScore ?? 0);
    const acoUnit = escapeHtml(inc.acoUnitAssigned || '');

    const noChaseBannerHtml = inc.doNotChaseAlert
      ? `
        <div class="cad-no-chase-banner">
          🛑 DO NOT CHASE — GOOD SAMARITAN CALMING PROTOCOL: High-velocity roadway (${speedMph} MPH, ${behavior}). Chasing drives panicked pets into active traffic lanes. Sit sideways, avert gaze &amp; toss high-odor lures while awaiting ACO L-barrier.
        </div>
      `
      : '';

    const resolutionBannerHtml = inc.custodyResolutionCode
      ? `
        <div class="cad-resolution-banner">
          ✅ SAFE CUSTODY INTERCEPT VERIFIED: ${escapeHtml(inc.custodyResolutionCode)} (ACO Unit ${acoUnit || 'ACO-UNIT-14'} · DOT VMS Cleared)
        </div>
      `
      : '';

    const steps = Array.isArray(inc.protocolSteps) ? inc.protocolSteps : [];
    const stepsHtml = steps
      .map((s) => {
        const done = Boolean(s.completed);
        const badgeClass = done ? 'badge-hop-match' : 'badge-hop-miss';
        const statusText = done ? 'DONE' : 'PENDING';
        const role = escapeHtml(s.role || '');
        const instruction = escapeHtml(s.instruction || '');
        return `
          <div class="decomp-checklist-item cad-protocol-step">
            <span class="badge ${badgeClass}">${statusText}</span>
            <strong class="decomp-item-category">[${role}]</strong>
            <span class="decomp-item-title">${instruction}</span>
          </div>
        `;
      })
      .join('');

    let actionsHtml = '';
    if (inc.dispatchStatus !== 'RESOLVED_SAFE_CUSTODY') {
      actionsHtml = `
        <button type="button" class="btn btn-sm btn-secondary btn-deploy-cad-containment" data-incident-id="${incidentId}">🚧 Deploy DOT VMS Advisory &amp; ACO L-Barrier Containment</button>
        <button type="button" class="btn btn-sm btn-primary btn-resolve-cad-custody" data-incident-id="${incidentId}">✅ Confirm Safe Slip-Lead Capture &amp; ACO Custody Handoff</button>
      `;
    }

    const unitSuffix = acoUnit ? ` • Unit: ${acoUnit}` : '';

    return `
      <article class="trust-screening-card cad-incident-card" data-incident-id="${incidentId}" data-dispatch-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${roadway} · ${speedMph} MPH / ${laneCount} lanes)</strong>
            <code class="trust-pet-id">${incidentId} · ${callNum}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-cad-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-hazard-severity ${severityClass}">${severity} (Strike Risk: ${strikeRisk}/100)</span>
          <span class="badge badge-cad-metric">Containment Readiness: ${readiness}/100 • Behavior: ${behavior}${unitSuffix}</span>
        </div>

        ${noChaseBannerHtml}
        ${resolutionBannerHtml}

        <div class="decomp-checklist-list cad-protocol-list" aria-label="Highway &amp; Good Samaritan Containment Protocol">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  /**
   * Render all CAD incidents in `#cad-incidents-list`
   */
  function renderAllIncidents(incidents) {
    currentIncidents = incidents;
    updateKpiCounters(incidents);

    const countBadge = document.getElementById('cad-incidents-count-badge');
    if (countBadge) {
      countBadge.textContent = `${incidents.length} Calls`;
    }

    const listEl = document.getElementById('cad-incidents-list');
    if (!listEl) return;

    if (incidents.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="cad-incidents-empty-msg">No municipal 311 or animal control CAD incidents recorded yet.</p>';
      return;
    }

    listEl.innerHTML = incidents.map(renderIncidentCardHtml).join('');
  }

  /**
   * Fetch all incidents from `/api/v1/municipal-cad/incidents`
   */
  async function fetchIncidents() {
    try {
      const res = await fetch('/api/v1/municipal-cad/incidents', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const incidents = Array.isArray(data.incidents) ? data.incidents : [];
      renderAllIncidents(incidents);
      return incidents;
    } catch (_err) {
      return [];
    }
  }

  /**
   * Submit a create CAD incident request to `/api/v1/municipal-cad/incidents`
   */
  async function submitCreateIncidentRequest(payload) {
    const res = await fetch('/api/v1/municipal-cad/incidents', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchIncidents();
      showToast(
        `Ingested 311 CAD call ${created.cadCallNumber} (${created.incidentId}) for ${created.petName} → ${created.dispatchStatus} (${created.hazardSeverity})`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to ingest 311 CAD incident', 'error');
    return null;
  }

  /**
   * Setup Ingest Municipal 311 CAD Call Form and Scenario Presets
   */
  function setupCreateFormAndSimulators() {
    const form = document.getElementById('form-create-cad-incident');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const cadCallNumber = (
          document.getElementById('cad-call-number')?.value || '311-SEA-8902'
        ).trim();
        const petId = (document.getElementById('cad-pet-id')?.value || 'pet-2026-001').trim();
        const petName = (document.getElementById('cad-pet-name')?.value || 'Cooper').trim();
        const species = (document.getElementById('cad-pet-species')?.value || 'DOG').trim();
        const roadwayName = (
          document.getElementById('cad-roadway-name')?.value ||
          'I-5 Northbound Milepost 168 Median'
        ).trim();
        const speedLimitMph = parseInt(
          document.getElementById('cad-speed-limit')?.value || '60',
          10
        );
        const laneCount = parseInt(document.getElementById('cad-lane-count')?.value || '4', 10);
        const petBehaviorState = (
          document.getElementById('cad-behavior-state')?.value || 'BOLTING_INTO_TRAFFIC'
        ).trim();
        const acoUnitAssigned = (document.getElementById('cad-aco-unit')?.value || '').trim();
        const dotSignAdvisoryActive = Boolean(
          document.getElementById('cad-dot-sign-active')?.checked
        );

        const submitBtn = document.getElementById('btn-create-cad-incident');
        if (submitBtn) submitBtn.disabled = true;

        try {
          await submitCreateIncidentRequest({
            cadCallNumber,
            petId,
            petName,
            species,
            roadwayName,
            speedLimitMph: Number.isNaN(speedLimitMph) ? 60 : speedLimitMph,
            laneCount: Number.isNaN(laneCount) ? 4 : laneCount,
            petBehaviorState,
            acoUnitAssigned,
            dotSignAdvisoryActive,
          });
        } catch (_err) {
          showToast('Network error ingesting 311 CAD call', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simFreewayBtn = document.getElementById('btn-sim-freeway-median');
    if (simFreewayBtn) {
      simFreewayBtn.addEventListener('click', async () => {
        simFreewayBtn.disabled = true;
        try {
          const roadEl = document.getElementById('cad-roadway-name');
          const speedEl = document.getElementById('cad-speed-limit');
          const laneEl = document.getElementById('cad-lane-count');
          const behEl = document.getElementById('cad-behavior-state');
          if (roadEl) roadEl.value = 'I-5 Southbound Ship Canal Bridge Median';
          if (speedEl) speedEl.value = '65';
          if (laneEl) laneEl.value = '5';
          if (behEl) behEl.value = 'BOLTING_INTO_TRAFFIC';

          await submitCreateIncidentRequest({
            cadCallNumber: '311-SEA-9410',
            petId: 'pet-2026-001',
            petName: 'Shadow',
            species: 'DOG',
            roadwayName: 'I-5 Southbound Ship Canal Bridge Median',
            speedLimitMph: 65,
            laneCount: 5,
            petBehaviorState: 'BOLTING_INTO_TRAFFIC',
            acoUnitAssigned: '',
            dotSignAdvisoryActive: false,
          });
        } finally {
          simFreewayBtn.disabled = false;
        }
      });
    }

    const simArterialBtn = document.getElementById('btn-sim-arterial-bridge');
    if (simArterialBtn) {
      simArterialBtn.addEventListener('click', async () => {
        simArterialBtn.disabled = true;
        try {
          const roadEl = document.getElementById('cad-roadway-name');
          const speedEl = document.getElementById('cad-speed-limit');
          const laneEl = document.getElementById('cad-lane-count');
          const behEl = document.getElementById('cad-behavior-state');
          if (roadEl) roadEl.value = 'Ballard Bridge Northbound Bascule Span';
          if (speedEl) speedEl.value = '45';
          if (laneEl) laneEl.value = '4';
          if (behEl) behEl.value = 'HUDDLED_IN_MEDIAN';

          await submitCreateIncidentRequest({
            cadCallNumber: '311-SEA-9425',
            petId: 'pet-2026-002',
            petName: 'Milo',
            species: 'DOG',
            roadwayName: 'Ballard Bridge Northbound Bascule Span',
            speedLimitMph: 45,
            laneCount: 4,
            petBehaviorState: 'HUDDLED_IN_MEDIAN',
            acoUnitAssigned: '',
            dotSignAdvisoryActive: false,
          });
        } finally {
          simArterialBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup card action buttons in `#cad-incidents-list`
   */
  function setupIncidentCardActions() {
    const listEl = document.getElementById('cad-incidents-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const deployBtn = e.target.closest('.btn-deploy-cad-containment');
      const resolveBtn = e.target.closest('.btn-resolve-cad-custody');

      const btn = deployBtn || resolveBtn;
      if (!btn) return;

      const incidentId = btn.getAttribute('data-incident-id');
      if (!incidentId) return;

      btn.disabled = true;
      try {
        if (deployBtn) {
          const res = await fetch(
            `/api/v1/municipal-cad/incidents/${encodeURIComponent(incidentId)}/dispatch`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                deployHighwayContainment: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchIncidents();
            showToast(
              `Deployed DOT VMS & ACO L-barrier for ${incidentId} → ${updated.dispatchStatus} (Readiness: ${updated.containmentReadinessScore}/100)`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to deploy highway containment', 'error');
          }
        } else if (resolveBtn) {
          const res = await fetch(
            `/api/v1/municipal-cad/incidents/${encodeURIComponent(incidentId)}/dispatch`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                resolveSafeCustody: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchIncidents();
            showToast(
              `Resolved ${incidentId} → ${updated.dispatchStatus} (${updated.custodyResolutionCode})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to resolve safe custody', 'error');
          }
        }
      } catch (_err) {
        showToast('Network error updating municipal CAD incident', 'error');
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
   * Subscribe to SSE stream `/api/v1/municipal-cad/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/municipal-cad/stream');
      const refresh = () => {
        fetchIncidents();
      };
      sseSource.addEventListener('municipal_cad_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'municipal_cad_updated') {
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
    setupIncidentCardActions();
    fetchIncidents();
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
