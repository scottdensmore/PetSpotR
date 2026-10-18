/**
 * PetSpotR Humane Live-Trap IoT Telemetry, Bait Decay & Rapid Dispatch Cockpit
 * Controller (Milestone 13.3)
 */

(function () {
  'use strict';

  let sseSource = null;
  let currentTraps = [];
  let lastDeployedTrapId = null;

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
    const liveStatus = document.getElementById('humane-traps-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  /**
   * Map HumaneTrapStatus to WCAG AAA badge CSS class
   */
  function getStatusBadgeClass(status) {
    switch (status) {
      case 'ARMED_ACTIVE':
        return 'badge-trap-armed';
      case 'DOOR_TRIPPED_URGENT':
        return 'badge-trap-tripped';
      case 'RESPONDER_DISPATCHED':
        return 'badge-trap-dispatched';
      case 'TARGET_CAPTURED_SECURED':
        return 'badge-trap-captured';
      case 'THERMAL_LOCKOUT_DISARMED':
        return 'badge-trap-thermal';
      default:
        return 'badge-trap-armed';
    }
  }

  /**
   * Map BaitFreshnessState to WCAG AAA badge CSS class
   */
  function getBaitBadgeClass(baitState) {
    switch (baitState) {
      case 'FRESH_PEAK_SCENT':
        return 'badge-bait-fresh';
      case 'DEGRADING_REFRESH_SOON':
        return 'badge-bait-degrading';
      case 'SPOILED_EXPIRED':
        return 'badge-bait-spoiled';
      default:
        return 'badge-bait-fresh';
    }
  }

  /**
   * Update KPI summary counters
   */
  function updateKpiCounters(traps) {
    const totalEl = document.getElementById('kpi-traps-total');
    const urgentEl = document.getElementById('kpi-traps-urgent');
    const capturedEl = document.getElementById('kpi-traps-captured');
    const thermalEl = document.getElementById('kpi-traps-thermal');

    const total = traps.length;
    const urgent = traps.filter(
      (t) => t.status === 'DOOR_TRIPPED_URGENT' || t.status === 'RESPONDER_DISPATCHED'
    ).length;
    const captured = traps.filter((t) => t.status === 'TARGET_CAPTURED_SECURED').length;
    const thermal = traps.filter((t) => t.status === 'THERMAL_LOCKOUT_DISARMED').length;

    if (totalEl) totalEl.textContent = String(total);
    if (urgentEl) urgentEl.textContent = String(urgent);
    if (capturedEl) capturedEl.textContent = String(captured);
    if (thermalEl) thermalEl.textContent = String(thermal);
  }

  /**
   * Render a single HumaneTrapDeployment card
   */
  function renderTrapCardHtml(t) {
    const trapId = escapeHtml(t.trapId || '');
    const petName = escapeHtml(t.petName || '');
    const species = escapeHtml(t.targetSpecies || 'CAT');
    const targetWeight = Number(t.targetWeightKg ?? 0).toFixed(1);
    const sectorId = escapeHtml(t.sectorId || '');
    const status = escapeHtml(t.status || 'ARMED_ACTIVE');
    const statusClass = getStatusBadgeClass(t.status);
    const trapLabel = escapeHtml(t.trapLabel || '');
    const trapType = escapeHtml(t.trapType || 'BOX_TRAP_GRAVITY');
    const baitType = escapeHtml(t.baitType || 'SARDINES_IN_OIL');
    const baitPct = Number(t.baitFreshnessPct ?? 100);
    const baitState = escapeHtml(t.baitState || 'FRESH_PEAK_SCENT');
    const baitClass = getBaitBadgeClass(t.baitState);
    const tempC = Number(t.internalTempC ?? 18.0).toFixed(1);
    const batteryPct = Number(t.batteryPct ?? 100);
    const rssiDbm = Number(t.rssiDbm ?? -65);
    const accelG = Number(t.accelerometerPeakG ?? 0).toFixed(2);
    const estWeightVal = Number(t.estimatedCaptureWeightKg ?? 0);
    const estWeightSuffix = estWeightVal > 0 ? ` · Est. ${estWeightVal.toFixed(1)} kg` : '';
    const summary = escapeHtml(t.telemetrySummary || '');
    const flags = Array.isArray(t.alertFlags) ? t.alertFlags : [];

    const sectorHtml = sectorId
      ? `<span class="badge badge-reporter-trust">Sector: ${sectorId}</span>`
      : '';

    const flagsHtml = flags
      .map((flag) => {
        const safeFlag = escapeHtml(flag);
        return `<span class="badge badge-trap-alert" data-flag="${safeFlag}">⚠️ ${safeFlag}</span>`;
      })
      .join('');

    const isUrgentSla =
      t.status === 'DOOR_TRIPPED_URGENT' || t.status === 'RESPONDER_DISPATCHED';
    let slaDetail = ' — Immediate field verification required!';
    if (t.assignedResponderName) {
      slaDetail = ` — Dispatched: ${escapeHtml(t.assignedResponderName)} (${escapeHtml(t.assignedResponderId || '')}, ETA ${Number(t.responderEtaMinutes ?? 6)}m)`;
    } else if (t.checkSlaDeadline) {
      slaDetail = ` — Deadline: ${escapeHtml(String(t.checkSlaDeadline))} (Immediate field verification required!)`;
    }
    const slaBannerHtml = isUrgentSla
      ? `<div class="trap-sla-urgent-banner">⏱️ 15-MIN HUMANE CHECK SLA ACTIVE${slaDetail}</div>`
      : '';

    const promotedBannerHtml = t.promotedSightingId
      ? `<div class="trap-promoted-sighting-banner trust-promoted-banner">📍 Promoted Sighting: ${escapeHtml(t.promotedSightingId)}</div>`
      : '';

    let actionsHtml = '';
    if (t.status === 'DOOR_TRIPPED_URGENT') {
      actionsHtml += `<button type="button" class="btn btn-sm btn-primary btn-dispatch-trap-responder" data-trap-id="${trapId}">🚑 Dispatch SAR Responder (Maya Lin · 6m ETA)</button>`;
    }
    if (t.status === 'DOOR_TRIPPED_URGENT' || t.status === 'RESPONDER_DISPATCHED') {
      actionsHtml += `<button type="button" class="btn btn-sm btn-secondary btn-resolve-target-captured" data-trap-id="${trapId}">✅ Confirm Target Pet Captured</button>`;
      actionsHtml += `<button type="button" class="btn btn-sm btn-secondary btn-resolve-bycatch-rearm" data-trap-id="${trapId}">🦝 Release Bycatch &amp; Re-Arm</button>`;
    }
    if (t.status === 'THERMAL_LOCKOUT_DISARMED') {
      actionsHtml += `<button type="button" class="btn btn-sm btn-secondary btn-resolve-thermal-rearm" data-trap-id="${trapId}">⛺ Install Thermal Shade &amp; Re-Arm</button>`;
    }

    return `
      <article class="trust-screening-card humane-trap-card" data-trap-id="${trapId}" data-trap-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${targetWeight} kg)</strong>
            <code class="trust-pet-id">${trapId}</code>
            ${sectorHtml}
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-trap-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="humane-trap-label-row">
          <strong>${trapLabel}</strong>
          <span class="text-secondary">(${trapType} · ${baitType})</span>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-bait-state ${baitClass}">Bait: ${baitPct}% (${baitState})</span>
          <span class="badge badge-kinematic-velocity">Temp: ${tempC}°C</span>
          <span class="badge badge-risk-score">Battery: ${batteryPct}% (${rssiDbm} dBm)</span>
          <span class="badge badge-image-integrity">Peak Accel: ${accelG}G${estWeightSuffix}</span>
        </div>

        <div class="trust-anomaly-flags">
          ${flagsHtml}
        </div>

        ${slaBannerHtml}
        ${promotedBannerHtml}

        <p class="trust-screening-notes">${summary}</p>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  /**
   * Render all traps in `#humane-traps-list`
   */
  function renderAllTraps(traps) {
    currentTraps = traps;
    updateKpiCounters(traps);

    const countBadge = document.getElementById('humane-traps-count-badge');
    if (countBadge) {
      countBadge.textContent = `${traps.length} Traps`;
    }

    const listEl = document.getElementById('humane-traps-list');
    if (!listEl) return;

    if (traps.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="humane-traps-empty-msg">No humane traps deployed yet.</p>';
      return;
    }

    listEl.innerHTML = traps.map(renderTrapCardHtml).join('');
  }

  /**
   * Fetch all humane traps from `/api/v1/humane-traps`
   */
  async function fetchTraps() {
    try {
      const res = await fetch('/api/v1/humane-traps', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const traps = Array.isArray(data.traps) ? data.traps : [];
      renderAllTraps(traps);
      return traps;
    } catch (_err) {
      return [];
    }
  }

  /**
   * Find the first ARMED_ACTIVE trap (preferring the trap most recently deployed in this tab) or fallback to `trap-ht-102`
   */
  async function getTargetArmedTrapForSimulation() {
    const traps = await fetchTraps();
    if (lastDeployedTrapId) {
      const ownTrap = traps.find(
        (t) => t.trapId === lastDeployedTrapId && t.status === 'ARMED_ACTIVE'
      );
      if (ownTrap) return ownTrap;
    }
    const armed = traps.find((t) => t.status === 'ARMED_ACTIVE');
    if (armed) return armed;
    const fallback = traps.find((t) => t.trapId === 'trap-ht-102') || traps[0];
    return fallback || { trapId: 'trap-ht-102', targetWeightKg: 4.2 };
  }

  /**
   * Setup Deploy & Arm New Humane Trap form and IoT Telemetry Simulator buttons
   */
  function setupDeployFormAndSimulators() {
    const form = document.getElementById('form-deploy-humane-trap');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petId = (document.getElementById('ht-pet-id')?.value || 'pet-2026-002').trim();
        const petName = (document.getElementById('ht-pet-name')?.value || 'Luna').trim();
        const targetSpecies = (
          document.getElementById('ht-target-species')?.value || 'CAT'
        ).trim();
        const targetWeightKg = parseFloat(
          document.getElementById('ht-target-weight-kg')?.value || '4.2'
        );
        const trapLabel = (
          document.getElementById('ht-trap-label')?.value ||
          'Tomahawk Tru-Catch #415 — Magnolia Bluff Ravine'
        ).trim();
        const trapType = (
          document.getElementById('ht-trap-type')?.value || 'BOX_TRAP_GRAVITY'
        ).trim();
        const baitType = (
          document.getElementById('ht-bait-type')?.value || 'SARDINES_IN_OIL'
        ).trim();
        const latitude = parseFloat(document.getElementById('ht-latitude')?.value || '47.6632');
        const longitude = parseFloat(document.getElementById('ht-longitude')?.value || '-122.4085');
        const sectorId = (document.getElementById('ht-sector-id')?.value || 'SEC-W-03').trim();
        const internalTempC = parseFloat(
          document.getElementById('ht-internal-temp-c')?.value || '17.5'
        );

        const submitBtn = document.getElementById('btn-deploy-humane-trap');
        if (submitBtn) submitBtn.disabled = true;

        try {
          const res = await fetch('/api/v1/humane-traps', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              petId,
              petName,
              targetSpecies,
              targetWeightKg: Number.isNaN(targetWeightKg) ? 4.2 : targetWeightKg,
              trapLabel,
              trapType,
              baitType,
              latitude: Number.isNaN(latitude) ? 47.6632 : latitude,
              longitude: Number.isNaN(longitude) ? -122.4085 : longitude,
              sectorId,
              internalTempC: Number.isNaN(internalTempC) ? 17.5 : internalTempC,
            }),
          });
          if (res.ok) {
            const created = await res.json();
            lastDeployedTrapId = created.trapId;
            await fetchTraps();
            showToast(
              `Deployed & armed humane trap ${created.trapId} (${created.trapLabel})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to deploy humane trap', 'error');
          }
        } catch (_err) {
          showToast('Network error deploying humane trap', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simTargetTripBtn = document.getElementById('btn-sim-door-trip-target');
    if (simTargetTripBtn) {
      simTargetTripBtn.addEventListener('click', async () => {
        simTargetTripBtn.disabled = true;
        try {
          const targetTrap = await getTargetArmedTrapForSimulation();
          const estWeight = Number(targetTrap.targetWeightKg || 4.2);
          const res = await fetch(
            `/api/v1/humane-traps/${encodeURIComponent(targetTrap.trapId)}/telemetry`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                doorClosed: true,
                accelerometerPeakG: 3.2,
                estimatedCaptureWeightKg: estWeight,
              }),
            }
          );
          if (res.ok) {
            await fetchTraps();
            showToast(
              `URGENT: Door tripped on ${targetTrap.trapId} — 15-min Humane Check SLA started!`,
              'warning'
            );
          }
        } finally {
          simTargetTripBtn.disabled = false;
        }
      });
    }

    const simBycatchBtn = document.getElementById('btn-sim-bycatch-trip');
    if (simBycatchBtn) {
      simBycatchBtn.addEventListener('click', async () => {
        simBycatchBtn.disabled = true;
        try {
          const targetTrap = await getTargetArmedTrapForSimulation();
          const res = await fetch(
            `/api/v1/humane-traps/${encodeURIComponent(targetTrap.trapId)}/telemetry`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                doorClosed: true,
                accelerometerPeakG: 2.4,
                estimatedCaptureWeightKg: 1.1,
              }),
            }
          );
          if (res.ok) {
            await fetchTraps();
            showToast(
              `BYCATCH ALERT: Door tripped on ${targetTrap.trapId} with 1.1 kg weight mismatch!`,
              'warning'
            );
          }
        } finally {
          simBycatchBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup card action buttons in `#humane-traps-list`
   */
  function setupTrapCardActions() {
    const listEl = document.getElementById('humane-traps-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const dispatchBtn = e.target.closest('.btn-dispatch-trap-responder');
      const capturedBtn = e.target.closest('.btn-resolve-target-captured');
      const bycatchBtn = e.target.closest('.btn-resolve-bycatch-rearm');
      const thermalBtn = e.target.closest('.btn-resolve-thermal-rearm');

      const btn = dispatchBtn || capturedBtn || bycatchBtn || thermalBtn;
      if (!btn) return;

      const trapId = btn.getAttribute('data-trap-id');
      if (!trapId) return;

      let payload;
      if (dispatchBtn) {
        payload = {
          action: 'DISPATCH',
          responderId: 'vol-sar-104',
          responderName: 'Maya Lin (SAR Tech)',
          etaMinutes: 6,
        };
      } else if (capturedBtn) {
        payload = {
          action: 'RESOLVE',
          outcome: 'TARGET_PET_CAPTURED',
          notes: 'Target pet verified via microchip scan and secured in recovery carrier.',
        };
      } else if (bycatchBtn) {
        payload = {
          action: 'RESOLVE',
          outcome: 'BYCATCH_RELEASED_REARMED',
          notes: 'Non-target raccoon safely released; fresh sardines placed and trap re-armed.',
        };
      } else if (thermalBtn) {
        payload = {
          action: 'RESOLVE',
          outcome: 'THERMAL_SHADED_REARMED',
          notes:
            'Reflective thermal shade tarp installed, internal temp cooled to 22.0°C, fresh bait placed.',
        };
      }

      btn.disabled = true;
      try {
        const res = await fetch(`/api/v1/humane-traps/${encodeURIComponent(trapId)}/dispatch`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        });
        if (res.ok) {
          const updated = await res.json();
          await fetchTraps();
          showToast(`Updated trap ${trapId} status to ${updated.status}`, 'success');
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to update trap dispatch/resolution', 'error');
        }
      } catch (_err) {
        showToast('Network error updating humane trap', 'error');
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
   * Subscribe to SSE stream `/api/v1/humane-traps/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/humane-traps/stream');
      const refresh = () => {
        fetchTraps();
      };
      sseSource.addEventListener('humane_trap_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'humane_trap_updated') {
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
    setupDeployFormAndSimulators();
    setupTrapCardActions();
    fetchTraps();
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
