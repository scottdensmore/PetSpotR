/**
 * PetSpotR Post-Reunion Behavioral Reintegration, Cortisol Decompression Tracker &
 * Escape-Recidivism Prevention Cockpit Controller (Milestone 13.5)
 */

(function () {
  'use strict';

  let sseSource = null;
  let currentPlans = [];

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
    const liveStatus = document.getElementById('reunion-decompression-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  /**
   * Map DecompressionPhase to WCAG AAA badge CSS class
   */
  function getPhaseBadgeClass(phase) {
    switch (phase) {
      case 'ACUTE_REFUGE_72H':
        return 'badge-phase-acute';
      case 'CORTISOL_TAPER_DAY_4_7':
        return 'badge-phase-taper';
      case 'ROUTINE_REINTEGRATION_DAY_8_14':
        return 'badge-phase-reintegration';
      case 'GRADUATED_STABLE':
        return 'badge-phase-graduated';
      default:
        return 'badge-phase-acute';
    }
  }

  /**
   * Map RecidivismRiskLevel to WCAG AAA badge CSS class
   */
  function getRiskBadgeClass(risk) {
    switch (risk) {
      case 'CRITICAL_FLIGHT_RISK':
        return 'badge-risk-critical';
      case 'ELEVATED_PERIMETER_RISK':
        return 'badge-risk-elevated';
      case 'FORTIFIED_LOW_RISK':
        return 'badge-risk-fortified';
      default:
        return 'badge-risk-critical';
    }
  }

  /**
   * Update KPI summary counters
   */
  function updateKpiCounters(plans) {
    const totalEl = document.getElementById('kpi-decomp-total');
    const criticalEl = document.getElementById('kpi-decomp-critical');
    const refeedingEl = document.getElementById('kpi-decomp-refeeding');
    const graduatedEl = document.getElementById('kpi-decomp-graduated');

    const total = plans.length;
    const critical = plans.filter((p) => p.recidivismRisk === 'CRITICAL_FLIGHT_RISK').length;
    const refeeding = plans.filter((p) => Boolean(p.refeedingSyndromeWarning)).length;
    const graduated = plans.filter((p) => p.phase === 'GRADUATED_STABLE').length;

    if (totalEl) totalEl.textContent = String(total);
    if (criticalEl) criticalEl.textContent = String(critical);
    if (refeedingEl) refeedingEl.textContent = String(refeeding);
    if (graduatedEl) graduatedEl.textContent = String(graduated);
  }

  /**
   * Render a single ReunionDecompressionPlan card
   */
  function renderPlanCardHtml(p) {
    const planId = escapeHtml(p.planId || '');
    const petName = escapeHtml(p.petName || '');
    const species = escapeHtml(p.species || 'CAT');
    const daysMissing = Number(p.daysMissing ?? 1);
    const weightLossPct = Number(p.weightLossPct ?? 0).toFixed(1);
    const phase = escapeHtml(p.phase || 'ACUTE_REFUGE_72H');
    const phaseClass = getPhaseBadgeClass(p.phase);
    const risk = escapeHtml(p.recidivismRisk || 'CRITICAL_FLIGHT_RISK');
    const riskClass = getRiskBadgeClass(p.recidivismRisk);
    const riskScore = Number(p.recidivismRiskScore ?? 80);
    const cortisol = Number(p.cortisolStressIndex ?? 80);
    const collarReadiness = Number(p.collarReadinessScore ?? 20);
    const summary = escapeHtml(p.clinicalSummary || '');

    const refeedingBannerHtml = p.refeedingSyndromeWarning
      ? `
        <div class="decomp-refeeding-warning-banner">
          ⚠️ REFEEDING SYNDROME ALERT: ${daysMissing}d missing / ${weightLossPct}% weight loss — Enforce 6 micro-meals/day warm electrolyte slurry to prevent hypophosphatemia.
        </div>
      `
      : '';

    const graduationBannerHtml = p.graduationCertificateId
      ? `
        <div class="decomp-graduation-banner">
          🎓 GRADUATED STABLE REINTEGRATION CERTIFICATE: ${escapeHtml(p.graduationCertificateId)}
        </div>
      `
      : '';

    const checklistItems = Array.isArray(p.checklistItems) ? p.checklistItems : [];
    const checklistHtml = checklistItems
      .map((item) => {
        const done = Boolean(item.completed);
        const badgeClass = done ? 'badge-hop-match' : 'badge-hop-miss';
        const statusText = done ? 'DONE' : 'PENDING';
        const category = escapeHtml(item.category || '');
        const title = escapeHtml(item.title || '');
        return `
          <div class="decomp-checklist-item">
            <span class="badge ${badgeClass}">${statusText}</span>
            <strong class="decomp-item-category">[${category}]</strong>
            <span class="decomp-item-title">${title}</span>
          </div>
        `;
      })
      .join('');

    const perimeterFindings = Array.isArray(p.perimeterFindings) ? p.perimeterFindings : [];
    const perimeterHtml = perimeterFindings
      .map((f) => {
        const remediated = Boolean(f.remediated);
        const badgeClass = remediated ? 'badge-hop-match' : 'badge-hop-miss';
        const statusText = remediated ? 'FORTIFIED' : 'UNSECURED';
        const zone = escapeHtml(f.zoneLabel || '');
        const vuln = escapeHtml(f.vulnerabilityType || '');
        const action = escapeHtml(f.remediationAction || '');
        return `
          <div class="decomp-perimeter-item">
            <span class="badge ${badgeClass}">${statusText}</span>
            <strong>${zone} (${vuln}):</strong>
            <span class="text-secondary">${action}</span>
          </div>
        `;
      })
      .join('');

    let actionsHtml = '';
    if (p.phase !== 'GRADUATED_STABLE') {
      actionsHtml = `
        <button type="button" class="btn btn-sm btn-secondary btn-remediate-perimeter" data-plan-id="${planId}">🛡️ Fortify Perimeter &amp; Equip GPS Collar + Double Leash</button>
        <button type="button" class="btn btn-sm btn-primary btn-graduate-decomp-plan" data-plan-id="${planId}">🎓 Complete 14-Day Decompression &amp; Issue Graduation Certificate</button>
      `;
    }

    return `
      <article class="trust-screening-card decomp-plan-card" data-plan-id="${planId}" data-plan-phase="${phase}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${daysMissing}d missing · ${weightLossPct}% weight loss)</strong>
            <code class="trust-pet-id">${planId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-decomp-phase ${phaseClass}">${phase}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-recidivism-risk ${riskClass}">${risk} (${riskScore}/100)</span>
          <span class="badge badge-cortisol-metric">Cortisol: ${cortisol}/100 • Collar Readiness: ${collarReadiness}/100</span>
        </div>

        ${refeedingBannerHtml}
        ${graduationBannerHtml}

        <div class="decomp-checklist-list" aria-label="14-Day Clinical &amp; Behavioral Decompression Checklist">
          ${checklistHtml}
        </div>

        <div class="decomp-perimeter-list" aria-label="Home Perimeter Escape-Vector Audit">
          ${perimeterHtml}
        </div>

        <p class="trust-screening-notes">${summary}</p>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  /**
   * Render all decompression plans in `#decomp-plans-list`
   */
  function renderAllPlans(plans) {
    currentPlans = plans;
    updateKpiCounters(plans);

    const countBadge = document.getElementById('decomp-plans-count-badge');
    if (countBadge) {
      countBadge.textContent = `${plans.length} Plans`;
    }

    const listEl = document.getElementById('decomp-plans-list');
    if (!listEl) return;

    if (plans.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="decomp-plans-empty-msg">No post-reunion decompression plans recorded yet.</p>';
      return;
    }

    listEl.innerHTML = plans.map(renderPlanCardHtml).join('');
  }

  /**
   * Fetch all decompression plans from `/api/v1/reunion-decompression/plans`
   */
  async function fetchPlans() {
    try {
      const res = await fetch('/api/v1/reunion-decompression/plans', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const plans = Array.isArray(data.plans) ? data.plans : [];
      renderAllPlans(plans);
      return plans;
    } catch (_err) {
      return [];
    }
  }

  /**
   * Submit a create decompression plan request to `/api/v1/reunion-decompression/plans`
   */
  async function submitCreatePlanRequest(payload) {
    const res = await fetch('/api/v1/reunion-decompression/plans', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchPlans();
      showToast(
        `Generated 14-day decompression plan ${created.planId} for ${created.petName} → ${created.phase} (${created.recidivismRisk})`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to generate decompression plan', 'error');
    return null;
  }

  /**
   * Setup Generate Post-Reunion Decompression Plan Form and Simulation Presets
   */
  function setupCreateFormAndSimulators() {
    const form = document.getElementById('form-create-decomp-plan');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petId = (document.getElementById('rd-pet-id')?.value || 'pet-2026-002').trim();
        const petName = (document.getElementById('rd-pet-name')?.value || 'Luna').trim();
        const species = (document.getElementById('rd-species')?.value || 'CAT').trim();
        const daysMissing = parseInt(document.getElementById('rd-days-missing')?.value || '8', 10);
        const weightLossPct = parseFloat(
          document.getElementById('rd-weight-loss-pct')?.value || '12.5'
        );
        const escapeVector = (
          document.getElementById('rd-escape-vector')?.value || 'DOOR_DASH_BOLT'
        ).trim();
        const gpsCollarEquipped = Boolean(document.getElementById('rd-gps-collar')?.checked);
        const doubleLeashVerified = Boolean(document.getElementById('rd-double-leash')?.checked);

        const submitBtn = document.getElementById('btn-create-decomp-plan');
        if (submitBtn) submitBtn.disabled = true;

        try {
          await submitCreatePlanRequest({
            petId,
            petName,
            species,
            daysMissing: Number.isNaN(daysMissing) ? 8 : daysMissing,
            weightLossPct: Number.isNaN(weightLossPct) ? 12.5 : weightLossPct,
            escapeVector,
            gpsCollarEquipped,
            doubleLeashVerified,
          });
        } catch (_err) {
          showToast('Network error generating decompression plan', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simStarvationBtn = document.getElementById('btn-sim-starvation-refeeding');
    if (simStarvationBtn) {
      simStarvationBtn.addEventListener('click', async () => {
        simStarvationBtn.disabled = true;
        try {
          const daysEl = document.getElementById('rd-days-missing');
          const weightEl = document.getElementById('rd-weight-loss-pct');
          const vectorEl = document.getElementById('rd-escape-vector');
          if (daysEl) daysEl.value = '11';
          if (weightEl) weightEl.value = '16.0';
          if (vectorEl) vectorEl.value = 'DOOR_DASH_BOLT';

          await submitCreatePlanRequest({
            petId: 'pet-2026-002',
            petName: 'Luna',
            species: 'CAT',
            daysMissing: 11,
            weightLossPct: 16.0,
            escapeVector: 'DOOR_DASH_BOLT',
            gpsCollarEquipped: false,
            doubleLeashVerified: false,
          });
        } finally {
          simStarvationBtn.disabled = false;
        }
      });
    }

    const simFireworksBtn = document.getElementById('btn-sim-fireworks-bolt');
    if (simFireworksBtn) {
      simFireworksBtn.addEventListener('click', async () => {
        simFireworksBtn.disabled = true;
        try {
          const daysEl = document.getElementById('rd-days-missing');
          const weightEl = document.getElementById('rd-weight-loss-pct');
          const vectorEl = document.getElementById('rd-escape-vector');
          if (daysEl) daysEl.value = '4';
          if (weightEl) weightEl.value = '5.0';
          if (vectorEl) vectorEl.value = 'FIREWORKS_PANIC_LEASH_SLIP';

          await submitCreatePlanRequest({
            petId: 'pet-2026-001',
            petName: 'Buster',
            species: 'DOG',
            daysMissing: 4,
            weightLossPct: 5.0,
            escapeVector: 'FIREWORKS_PANIC_LEASH_SLIP',
            gpsCollarEquipped: false,
            doubleLeashVerified: false,
          });
        } finally {
          simFireworksBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup card action buttons in `#decomp-plans-list`
   */
  function setupPlanCardActions() {
    const listEl = document.getElementById('decomp-plans-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const remediateBtn = e.target.closest('.btn-remediate-perimeter');
      const graduateBtn = e.target.closest('.btn-graduate-decomp-plan');

      const btn = remediateBtn || graduateBtn;
      if (!btn) return;

      const planId = btn.getAttribute('data-plan-id');
      if (!planId) return;

      btn.disabled = true;
      try {
        if (remediateBtn) {
          const res = await fetch(
            `/api/v1/reunion-decompression/plans/${encodeURIComponent(planId)}/progress`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                remediateAllPerimeter: true,
                gpsCollarEquipped: true,
                doubleLeashVerified: true,
                notes:
                  'Remediated home perimeter escape vectors and verified LTE GPS collar + 3-point double leash.',
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchPlans();
            showToast(
              `Fortified perimeter & equipped GPS collar for ${planId} → ${updated.phase} (Collar Readiness: ${updated.collarReadinessScore}/100)`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to update perimeter remediation', 'error');
          }
        } else if (graduateBtn) {
          const res = await fetch(
            `/api/v1/reunion-decompression/plans/${encodeURIComponent(planId)}/progress`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                completeAllChecklist: true,
                remediateAllPerimeter: true,
                gpsCollarEquipped: true,
                doubleLeashVerified: true,
                notes: 'All 14-day cortisol & perimeter fortification milestones verified.',
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchPlans();
            showToast(
              `Graduated ${planId} → ${updated.phase} (${updated.graduationCertificateId})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to graduate decompression plan', 'error');
          }
        }
      } catch (_err) {
        showToast('Network error updating decompression plan', 'error');
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
   * Subscribe to SSE stream `/api/v1/reunion-decompression/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/reunion-decompression/stream');
      const refresh = () => {
        fetchPlans();
      };
      sseSource.addEventListener('decompression_plan_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'decompression_plan_updated') {
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
    setupPlanCardActions();
    fetchPlans();
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
