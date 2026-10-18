/**
 * PetSpotR Aviation & Cross-Jurisdiction Pet Airlift / Ground Relay
 * Transport Logistics Cockpit Controller (Milestone 14.4)
 */

(function () {
  'use strict';

  let sseSource = null;
  let currentMissions = [];

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
    const liveStatus = document.getElementById('transport-relay-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  /**
   * Map TransportRelayStatus to WCAG AAA badge CSS class
   */
  function getStatusBadgeClass(status) {
    switch (status) {
      case 'PRE_FLIGHT_STAGING':
        return 'badge-relay-staging';
      case 'IN_TRANSIT_ACTIVE_RELAY':
        return 'badge-relay-transit';
      case 'DELIVERED_DESTINATION_VERIFIED':
        return 'badge-relay-delivered';
      default:
        return 'badge-relay-staging';
    }
  }

  /**
   * Map TransportThermalRisk to WCAG AAA badge CSS class
   */
  function getThermalRiskBadgeClass(risk) {
    switch (risk) {
      case 'BRACHYCEPHALIC_CABIN_RISK':
        return 'badge-thermal-hypoxia';
      case 'TARMAC_THERMAL_WATCH':
        return 'badge-thermal-watch';
      case 'PRESSURIZED_SAFE_ENVELOPE':
        return 'badge-thermal-safe';
      default:
        return 'badge-thermal-hypoxia';
    }
  }

  /**
   * Update KPI summary counters
   */
  function updateKpiCounters(missions) {
    const totalEl = document.getElementById('kpi-relay-total');
    const hypoxiaEl = document.getElementById('kpi-relay-hypoxia');
    const intransitEl = document.getElementById('kpi-relay-intransit');
    const deliveredEl = document.getElementById('kpi-relay-delivered');

    const total = missions.length;
    const hypoxia = missions.filter((m) => Boolean(m.hypoxiaCabinAlert)).length;
    const intransit = missions.filter((m) => m.relayStatus === 'IN_TRANSIT_ACTIVE_RELAY').length;
    const delivered = missions.filter((m) => m.relayStatus === 'DELIVERED_DESTINATION_VERIFIED').length;

    if (totalEl) totalEl.textContent = String(total);
    if (hypoxiaEl) hypoxiaEl.textContent = String(hypoxia);
    if (intransitEl) intransitEl.textContent = String(intransit);
    if (deliveredEl) deliveredEl.textContent = String(delivered);
  }

  /**
   * Render a single TransportRelayMission card
   */
  function renderMissionCardHtml(m) {
    const missionId = escapeHtml(m.missionId || '');
    const petName = escapeHtml(m.petName || '');
    const species = escapeHtml(m.species || 'DOG');
    const originHub = escapeHtml(m.originHub || '');
    const destinationHub = escapeHtml(m.destinationHub || '');
    const distanceMiles = Number(m.totalDistanceMiles ?? 0);
    const altitudeFeet = Number(m.cruiseAltitudeFeet ?? 0);
    const status = escapeHtml(m.relayStatus || 'PRE_FLIGHT_STAGING');
    const statusClass = getStatusBadgeClass(m.relayStatus);
    const risk = escapeHtml(m.thermalRisk || 'BRACHYCEPHALIC_CABIN_RISK');
    const riskClass = getThermalRiskBadgeClass(m.thermalRisk);
    const mode = escapeHtml(m.transportMode || 'GENERAL_AVIATION_AIRLIFT');
    const stressScore = Number(m.transportStressScore ?? 80);
    const readiness = Number(m.flightReadinessScore ?? 0);

    const hypoxiaBannerHtml = m.hypoxiaCabinAlert
      ? `
        <div class="relay-hypoxia-alert-banner">
          ⚠️ BRACHYCEPHALIC &amp; ALTITUDE HYPOXIA ALERT: Planned altitude ${altitudeFeet} ft (Brachycephalic: ${Boolean(m.brachycephalicFlag)}) — Enforce &le; 6,500 ft pressurized cabin ceiling, continuous pulse oximetry &amp; cooling mat telemetry.
        </div>
      `
      : '';

    const manifestBannerHtml = m.manifestCompletionCode
      ? `
        <div class="relay-manifest-banner">
          ✅ INTERSTATE RELAY MANIFEST VERIFIED: ${escapeHtml(m.manifestCompletionCode)}
        </div>
      `
      : '';

    const legs = Array.isArray(m.relayLegChecks) ? m.relayLegChecks : [];
    const legsHtml = legs
      .map((leg) => {
        const done = Boolean(leg.completed);
        const badgeClass = done ? 'badge-hop-match' : 'badge-hop-miss';
        const statusText = done ? 'VERIFIED' : 'REQUIRED';
        const category = escapeHtml(leg.category || '');
        const title = escapeHtml(leg.title || '');
        return `
          <div class="relay-leg-item">
            <span class="badge ${badgeClass}">${statusText}</span>
            <strong class="relay-leg-category">[${category}]</strong>
            <span class="relay-leg-title">${title}</span>
          </div>
        `;
      })
      .join('');

    let actionsHtml = '';
    if (m.relayStatus !== 'DELIVERED_DESTINATION_VERIFIED') {
      actionsHtml = `
        <button type="button" class="btn btn-sm btn-secondary btn-launch-relay-leg" data-mission-id="${missionId}">🛫 Clear USDA CVI &amp; IATA Crate to Launch Relay</button>
        <button type="button" class="btn btn-sm btn-primary btn-complete-relay-delivery" data-mission-id="${missionId}">🏁 Complete Final FBO / Ground Handoff &amp; Verify Manifest</button>
      `;
    }

    return `
      <article class="trust-screening-card relay-mission-card" data-mission-id="${missionId}" data-relay-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${originHub} &rarr; ${destinationHub} · ${distanceMiles} mi @ ${altitudeFeet} ft)</strong>
            <code class="trust-pet-id">${missionId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-relay-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-thermal-risk ${riskClass}">${risk}</span>
          <span class="badge badge-relay-metric">Mode: ${mode} • Transport Stress: ${stressScore}/100 • Flight Readiness: ${readiness}/100</span>
        </div>

        ${hypoxiaBannerHtml}
        ${manifestBannerHtml}

        <div class="relay-legs-list" aria-label="Interstate CVI, Cabin Pressure &amp; FBO Handoff Checklist">
          ${legsHtml}
        </div>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  /**
   * Render all transport relay missions in `#relay-missions-list`
   */
  function renderAllMissions(missions) {
    currentMissions = missions;
    updateKpiCounters(missions);

    const countBadge = document.getElementById('relay-missions-count-badge');
    if (countBadge) {
      countBadge.textContent = `${missions.length} Missions`;
    }

    const listEl = document.getElementById('relay-missions-list');
    if (!listEl) return;

    if (missions.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="relay-missions-empty-msg">No transport relay missions recorded yet.</p>';
      return;
    }

    listEl.innerHTML = missions.map(renderMissionCardHtml).join('');
  }

  /**
   * Fetch all transport relay missions from `/api/v1/transport-relay/missions`
   */
  async function fetchMissions() {
    try {
      const res = await fetch('/api/v1/transport-relay/missions', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const missions = Array.isArray(data.missions) ? data.missions : [];
      renderAllMissions(missions);
      return missions;
    } catch (_err) {
      return [];
    }
  }

  /**
   * Submit a create transport relay mission request to `/api/v1/transport-relay/missions`
   */
  async function submitCreateMissionRequest(payload) {
    const res = await fetch('/api/v1/transport-relay/missions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchMissions();
      showToast(
        `Planned transport relay mission ${created.missionId} for ${created.petName} → ${created.relayStatus} (${created.thermalRisk})`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to plan transport relay mission', 'error');
    return null;
  }

  /**
   * Setup Plan Transport Relay Mission Form and Scenario Simulators
   */
  function setupCreateFormAndSimulators() {
    const form = document.getElementById('form-create-relay-mission');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petId = (document.getElementById('relay-pet-id')?.value || 'pet-2026-001').trim();
        const petName = (document.getElementById('relay-pet-name')?.value || 'Winston').trim();
        const species = (document.getElementById('relay-pet-species')?.value || 'DOG').trim();
        const originHub = (
          document.getElementById('relay-origin-hub')?.value || 'KBOI Boise Air Terminal FBO'
        ).trim();
        const destinationHub = (
          document.getElementById('relay-destination-hub')?.value || 'KBFI Boeing Field Seattle'
        ).trim();
        const transportMode = (
          document.getElementById('relay-transport-mode')?.value || 'GENERAL_AVIATION_AIRLIFT'
        ).trim();
        const totalDistanceMiles = parseInt(
          document.getElementById('relay-distance-miles')?.value || '410',
          10
        );
        const cruiseAltitudeFeet = parseInt(
          document.getElementById('relay-altitude-feet')?.value || '8500',
          10
        );
        const brachycephalicFlag = Boolean(document.getElementById('relay-brachycephalic')?.checked);
        const usdaCviCertified = Boolean(document.getElementById('relay-usda-cvi')?.checked);
        const iataCrateVerified = Boolean(document.getElementById('relay-iata-crate')?.checked);

        const submitBtn = document.getElementById('btn-create-relay-mission');
        if (submitBtn) submitBtn.disabled = true;

        try {
          await submitCreateMissionRequest({
            petId,
            petName,
            species,
            originHub,
            destinationHub,
            transportMode,
            totalDistanceMiles: Number.isNaN(totalDistanceMiles) ? 410 : totalDistanceMiles,
            cruiseAltitudeFeet: Number.isNaN(cruiseAltitudeFeet) ? 8500 : cruiseAltitudeFeet,
            brachycephalicFlag,
            usdaCviCertified,
            iataCrateVerified,
          });
        } catch (_err) {
          showToast('Network error planning transport relay mission', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simBrachyBtn = document.getElementById('btn-sim-brachy-airlift');
    if (simBrachyBtn) {
      simBrachyBtn.addEventListener('click', async () => {
        simBrachyBtn.disabled = true;
        try {
          const modeEl = document.getElementById('relay-transport-mode');
          const distEl = document.getElementById('relay-distance-miles');
          const altEl = document.getElementById('relay-altitude-feet');
          const brachyEl = document.getElementById('relay-brachycephalic');
          if (modeEl) modeEl.value = 'GENERAL_AVIATION_AIRLIFT';
          if (distEl) distEl.value = '410';
          if (altEl) altEl.value = '8500';
          if (brachyEl) brachyEl.checked = true;

          await submitCreateMissionRequest({
            petId: 'pet-2026-001',
            petName: 'Louie',
            species: 'DOG',
            originHub: 'KBOI Boise Air Terminal FBO',
            destinationHub: 'KBFI Boeing Field Seattle',
            transportMode: 'GENERAL_AVIATION_AIRLIFT',
            totalDistanceMiles: 410,
            cruiseAltitudeFeet: 8500,
            brachycephalicFlag: true,
            usdaCviCertified: false,
            iataCrateVerified: false,
          });
        } finally {
          simBrachyBtn.disabled = false;
        }
      });
    }

    const simGroundBtn = document.getElementById('btn-sim-ground-relay');
    if (simGroundBtn) {
      simGroundBtn.addEventListener('click', async () => {
        simGroundBtn.disabled = true;
        try {
          const modeEl = document.getElementById('relay-transport-mode');
          const distEl = document.getElementById('relay-distance-miles');
          const altEl = document.getElementById('relay-altitude-feet');
          const brachyEl = document.getElementById('relay-brachycephalic');
          if (modeEl) modeEl.value = 'CLIMATE_CONTROLLED_VAN_RELAY';
          if (distEl) distEl.value = '320';
          if (altEl) altEl.value = '2200';
          if (brachyEl) brachyEl.checked = false;

          await submitCreateMissionRequest({
            petId: 'pet-2026-002',
            petName: 'Maple',
            species: 'DOG',
            originHub: 'KPDX Portland Hillsboro Relay',
            destinationHub: 'KBLI Bellingham International Hub',
            transportMode: 'CLIMATE_CONTROLLED_VAN_RELAY',
            totalDistanceMiles: 320,
            cruiseAltitudeFeet: 2200,
            brachycephalicFlag: false,
            usdaCviCertified: false,
            iataCrateVerified: false,
          });
        } finally {
          simGroundBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup card action buttons in `#relay-missions-list`
   */
  function setupMissionCardActions() {
    const listEl = document.getElementById('relay-missions-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const launchBtn = e.target.closest('.btn-launch-relay-leg');
      const completeBtn = e.target.closest('.btn-complete-relay-delivery');

      const btn = launchBtn || completeBtn;
      if (!btn) return;

      const missionId = btn.getAttribute('data-mission-id');
      if (!missionId) return;

      btn.disabled = true;
      try {
        if (launchBtn) {
          const res = await fetch(
            `/api/v1/transport-relay/missions/${encodeURIComponent(missionId)}/advance`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                clearPreFlightAndLaunch: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchMissions();
            showToast(
              `Cleared USDA CVI & IATA crate for ${missionId} → ${updated.relayStatus} (Flight Readiness: ${updated.flightReadinessScore}/100)`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to launch transport relay leg', 'error');
          }
        } else if (completeBtn) {
          const res = await fetch(
            `/api/v1/transport-relay/missions/${encodeURIComponent(missionId)}/advance`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                completeRelayDelivery: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchMissions();
            showToast(
              `Verified destination delivery for ${missionId} → ${updated.relayStatus} (${updated.manifestCompletionCode})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to complete transport relay delivery', 'error');
          }
        }
      } catch (_err) {
        showToast('Network error advancing transport relay mission', 'error');
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
   * Subscribe to SSE stream `/api/v1/transport-relay/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/transport-relay/stream');
      const refresh = () => {
        fetchMissions();
      };
      sseSource.addEventListener('transport_relay_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'transport_relay_updated') {
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
    setupMissionCardActions();
    fetchMissions();
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
