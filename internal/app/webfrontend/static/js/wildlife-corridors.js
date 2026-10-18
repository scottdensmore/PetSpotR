/**
 * PetSpotR Predator & Wildlife Conflict Corridor Alerting, Nocturnal Vulnerability Index &
 * Coyote/Raptor Sighting Correlation Cockpit Controller (Milestone 14.1)
 */

(function () {
  'use strict';

  let sseSource = null;
  let currentReports = [];

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
    const liveStatus = document.getElementById('wildlife-corridors-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  /**
   * Map CorridorPhase to WCAG AAA badge CSS class
   */
  function getPhaseBadgeClass(phase) {
    switch (phase) {
      case 'ACTIVE_PREDATOR_WATCH':
        return 'badge-corridor-watch';
      case 'HAZING_DETERRENT_DEPLOYED':
        return 'badge-corridor-hazing';
      case 'CORRIDOR_CLEARED':
        return 'badge-corridor-cleared';
      default:
        return 'badge-corridor-watch';
    }
  }

  /**
   * Map PredatorThreatLevel to WCAG AAA badge CSS class
   */
  function getThreatBadgeClass(threat) {
    switch (threat) {
      case 'EXTREME_NOCTURNAL_PREDATION_RISK':
        return 'badge-threat-extreme';
      case 'ELEVATED_CREPUSCULAR_CORRIDOR':
        return 'badge-threat-elevated';
      case 'MITIGATED_SAFE_ZONE':
        return 'badge-threat-safe';
      default:
        return 'badge-threat-extreme';
    }
  }

  /**
   * Update KPI summary counters
   */
  function updateKpiCounters(reports) {
    const totalEl = document.getElementById('kpi-corridor-total');
    const extremeEl = document.getElementById('kpi-corridor-extreme');
    const smallPreyEl = document.getElementById('kpi-corridor-small-prey');
    const clearedEl = document.getElementById('kpi-corridor-cleared');

    const total = reports.length;
    const extreme = reports.filter(
      (r) => r.threatLevel === 'EXTREME_NOCTURNAL_PREDATION_RISK'
    ).length;
    const smallPrey = reports.filter((r) => Boolean(r.smallPreyAlert)).length;
    const cleared = reports.filter((r) => r.phase === 'CORRIDOR_CLEARED').length;

    if (totalEl) totalEl.textContent = String(total);
    if (extremeEl) extremeEl.textContent = String(extreme);
    if (smallPreyEl) smallPreyEl.textContent = String(smallPrey);
    if (clearedEl) clearedEl.textContent = String(cleared);
  }

  /**
   * Render a single WildlifeCorridorReport card
   */
  function renderReportCardHtml(r) {
    const reportId = escapeHtml(r.reportId || '');
    const petName = escapeHtml(r.petName || '');
    const petSpecies = escapeHtml(r.petSpecies || 'CAT');
    const petWeightKg = Number(r.petWeightKg ?? 4.0).toFixed(1);
    const predatorSpecies = escapeHtml(r.predatorSpecies || 'COYOTE_PACK');
    const packCount = Number(r.packCount ?? 1);
    const solarPhase = escapeHtml(r.solarPhase || 'NIGHT');
    const corridorTerrain = escapeHtml(r.corridorTerrain || 'URBAN_RAVINE_GREENBELT');
    const distanceMeters = Number(r.distanceMeters ?? 100).toFixed(0);
    const phase = escapeHtml(r.phase || 'ACTIVE_PREDATOR_WATCH');
    const phaseClass = getPhaseBadgeClass(r.phase);
    const threatLevel = escapeHtml(r.threatLevel || 'EXTREME_NOCTURNAL_PREDATION_RISK');
    const threatClass = getThreatBadgeClass(r.threatLevel);
    const vulnIndex = Number(r.vulnerabilityIndex ?? 85);
    const readiness = Number(r.deterrentReadinessScore ?? 0);

    const smallPreyBannerHtml = r.smallPreyAlert
      ? `
        <div class="corridor-small-prey-banner">
          ⚠️ SMALL PREY VULNERABILITY ALERT: ${petName} (${petWeightKg}kg ${petSpecies}) vs ${predatorSpecies} in ${corridorTerrain} (${solarPhase}) — Enforce immediate indoor/garage safe-haven shelter.
        </div>
      `
      : '';

    const clearanceBannerHtml = r.clearanceCertificate
      ? `
        <div class="corridor-clearance-banner">
          🛡️ CORRIDOR SAFE-HAVEN VERIFIED: ${escapeHtml(r.clearanceCertificate)}
        </div>
      `
      : '';

    const steps = Array.isArray(r.mitigationSteps) ? r.mitigationSteps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const badgeClass = done ? 'badge-hop-match' : 'badge-hop-miss';
        const statusText = done ? 'DONE' : 'PENDING';
        const category = escapeHtml(step.category || '');
        const title = escapeHtml(step.title || '');
        return `
          <div class="corridor-mitigation-item">
            <span class="badge ${badgeClass}">${statusText}</span>
            <strong class="corridor-item-category">[${category}]</strong>
            <span class="corridor-item-title">${title}</span>
          </div>
        `;
      })
      .join('');

    let actionsHtml = '';
    if (r.phase !== 'CORRIDOR_CLEARED') {
      actionsHtml = `
        <button type="button" class="btn btn-sm btn-secondary btn-deploy-deterrents" data-report-id="${reportId}">🔦 Deploy Strobe Hazing &amp; Lock Attractants</button>
        <button type="button" class="btn btn-sm btn-primary btn-clear-corridor" data-report-id="${reportId}">🛡️ Complete All Mitigations &amp; Issue Corridor Clearance</button>
      `;
    }

    return `
      <article class="trust-screening-card corridor-report-card" data-report-id="${reportId}" data-report-phase="${phase}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${petSpecies} · ${petWeightKg}kg · ${predatorSpecies} ×${packCount} @ ${distanceMeters}m)</strong>
            <code class="trust-pet-id">${reportId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-corridor-phase ${phaseClass}">${phase}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-predator-threat ${threatClass}">${threatLevel}</span>
          <span class="badge badge-corridor-metric">Vulnerability Index: ${vulnIndex}/100 • Deterrent Readiness: ${readiness}/100</span>
        </div>

        ${smallPreyBannerHtml}
        ${clearanceBannerHtml}

        <div class="corridor-mitigation-list" aria-label="Predator Hazing &amp; Corridor Mitigation Steps">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  /**
   * Render all corridor reports in `#corridor-reports-list`
   */
  function renderAllReports(reports) {
    currentReports = reports;
    updateKpiCounters(reports);

    const countBadge = document.getElementById('corridor-reports-count-badge');
    if (countBadge) {
      countBadge.textContent = `${reports.length} Reports`;
    }

    const listEl = document.getElementById('corridor-reports-list');
    if (!listEl) return;

    if (reports.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="corridor-reports-empty-msg">No wildlife corridor reports recorded yet.</p>';
      return;
    }

    listEl.innerHTML = reports.map(renderReportCardHtml).join('');
  }

  /**
   * Fetch all reports from `/api/v1/wildlife-corridors/reports`
   */
  async function fetchReports() {
    try {
      const res = await fetch('/api/v1/wildlife-corridors/reports', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const reports = Array.isArray(data.reports) ? data.reports : [];
      renderAllReports(reports);
      return reports;
    } catch (_err) {
      return [];
    }
  }

  /**
   * Submit a create wildlife corridor report request to `/api/v1/wildlife-corridors/reports`
   */
  async function submitCreateReportRequest(payload) {
    const res = await fetch('/api/v1/wildlife-corridors/reports', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchReports();
      showToast(
        `Evaluated wildlife corridor report ${created.reportId} for ${created.petName} → ${created.phase} (${created.threatLevel})`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate wildlife corridor report', 'error');
    return null;
  }

  /**
   * Setup Create Corridor Report Form and Scenario Simulators
   */
  function setupCreateFormAndSimulators() {
    const form = document.getElementById('form-create-corridor-report');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petId = (document.getElementById('corridor-pet-id')?.value || 'pet-2026-002').trim();
        const petName = (document.getElementById('corridor-pet-name')?.value || 'Mochi').trim();
        const petSpecies = (
          document.getElementById('corridor-pet-species')?.value || 'CAT'
        ).trim();
        const petWeightKg = parseFloat(
          document.getElementById('corridor-pet-weight')?.value || '4.2'
        );
        const predatorSpecies = (
          document.getElementById('corridor-predator-species')?.value || 'COYOTE_PACK'
        ).trim();
        const packCount = parseInt(
          document.getElementById('corridor-pack-count')?.value || '3',
          10
        );
        const solarPhase = (
          document.getElementById('corridor-solar-phase')?.value || 'NIGHT'
        ).trim();
        const corridorTerrain = (
          document.getElementById('corridor-terrain')?.value || 'URBAN_RAVINE_GREENBELT'
        ).trim();
        const distanceMeters = parseFloat(
          document.getElementById('corridor-distance')?.value || '85'
        );
        const strobeDeterrentActive = Boolean(
          document.getElementById('corridor-strobe-active')?.checked
        );
        const attractantsCleared = Boolean(
          document.getElementById('corridor-attractants-cleared')?.checked
        );

        const submitBtn = document.getElementById('btn-create-corridor-report');
        if (submitBtn) submitBtn.disabled = true;

        try {
          await submitCreateReportRequest({
            petId,
            petName,
            petSpecies,
            petWeightKg: Number.isNaN(petWeightKg) ? 4.2 : petWeightKg,
            predatorSpecies,
            packCount: Number.isNaN(packCount) ? 3 : packCount,
            solarPhase,
            corridorTerrain,
            distanceMeters: Number.isNaN(distanceMeters) ? 85 : distanceMeters,
            strobeDeterrentActive,
            attractantsCleared,
          });
        } catch (_err) {
          showToast('Network error evaluating wildlife corridor report', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simCoyoteBtn = document.getElementById('btn-sim-coyote-ravine');
    if (simCoyoteBtn) {
      simCoyoteBtn.addEventListener('click', async () => {
        simCoyoteBtn.disabled = true;
        try {
          const speciesEl = document.getElementById('corridor-pet-species');
          const weightEl = document.getElementById('corridor-pet-weight');
          const predEl = document.getElementById('corridor-predator-species');
          const solarEl = document.getElementById('corridor-solar-phase');
          if (speciesEl) speciesEl.value = 'CAT';
          if (weightEl) weightEl.value = '4.0';
          if (predEl) predEl.value = 'COYOTE_PACK';
          if (solarEl) solarEl.value = 'NIGHT';

          await submitCreateReportRequest({
            petId: 'pet-2026-002',
            petName: 'Mochi',
            petSpecies: 'CAT',
            petWeightKg: 4.0,
            predatorSpecies: 'COYOTE_PACK',
            packCount: 4,
            solarPhase: 'NIGHT',
            corridorTerrain: 'URBAN_RAVINE_GREENBELT',
            distanceMeters: 65,
            strobeDeterrentActive: false,
            attractantsCleared: false,
          });
        } finally {
          simCoyoteBtn.disabled = false;
        }
      });
    }

    const simRaptorBtn = document.getElementById('btn-sim-raptor-ridge');
    if (simRaptorBtn) {
      simRaptorBtn.addEventListener('click', async () => {
        simRaptorBtn.disabled = true;
        try {
          const speciesEl = document.getElementById('corridor-pet-species');
          const weightEl = document.getElementById('corridor-pet-weight');
          const predEl = document.getElementById('corridor-predator-species');
          const solarEl = document.getElementById('corridor-solar-phase');
          if (speciesEl) speciesEl.value = 'DOG';
          if (weightEl) weightEl.value = '3.2';
          if (predEl) predEl.value = 'GREAT_HORNED_OWL';
          if (solarEl) solarEl.value = 'DAWN_DUSK_CREPUSCULAR';

          await submitCreateReportRequest({
            petId: 'pet-2026-001',
            petName: 'Peanut',
            petSpecies: 'DOG',
            petWeightKg: 3.2,
            predatorSpecies: 'GREAT_HORNED_OWL',
            packCount: 1,
            solarPhase: 'DAWN_DUSK_CREPUSCULAR',
            corridorTerrain: 'BRUSH_CANYON_INTERFACE',
            distanceMeters: 90,
            strobeDeterrentActive: false,
            attractantsCleared: false,
          });
        } finally {
          simRaptorBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup card action buttons in `#corridor-reports-list`
   */
  function setupReportCardActions() {
    const listEl = document.getElementById('corridor-reports-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const deployBtn = e.target.closest('.btn-deploy-deterrents');
      const clearBtn = e.target.closest('.btn-clear-corridor');

      const btn = deployBtn || clearBtn;
      if (!btn) return;

      const reportId = btn.getAttribute('data-report-id');
      if (!reportId) return;

      btn.disabled = true;
      try {
        if (deployBtn) {
          const res = await fetch(
            `/api/v1/wildlife-corridors/reports/${encodeURIComponent(reportId)}/mitigate`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                deployDeterrents: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchReports();
            showToast(
              `Deployed ultrasonic/strobe deterrents & locked attractants for ${reportId} → ${updated.phase} (Readiness: ${updated.deterrentReadinessScore}/100)`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to deploy predator deterrents', 'error');
          }
        } else if (clearBtn) {
          const res = await fetch(
            `/api/v1/wildlife-corridors/reports/${encodeURIComponent(reportId)}/mitigate`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                completeAllMitigations: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchReports();
            showToast(
              `Cleared corridor ${reportId} → ${updated.phase} (${updated.clearanceCertificate})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to clear wildlife corridor', 'error');
          }
        }
      } catch (_err) {
        showToast('Network error updating wildlife corridor report', 'error');
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
   * Subscribe to SSE stream `/api/v1/wildlife-corridors/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/wildlife-corridors/stream');
      const refresh = () => {
        fetchReports();
      };
      sseSource.addEventListener('wildlife_corridor_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'wildlife_corridor_updated') {
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
    setupReportCardActions();
    fetchReports();
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
