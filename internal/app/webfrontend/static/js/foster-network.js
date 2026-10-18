/**
 * PetSpotR Emergency Foster Network Capacity, Quarantine Biosecurity &
 * Medical Foster Matching Cockpit Controller (Milestone 14.3)
 */

(function () {
  'use strict';

  let sseSource = null;
  let currentPlacements = [];

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
    const liveStatus = document.getElementById('foster-network-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  /**
   * Map FosterPlacementStatus to WCAG AAA badge CSS class
   */
  function getStatusBadgeClass(status) {
    switch (status) {
      case 'PENDING_BIOSECURITY_CLEARANCE':
        return 'badge-foster-pending';
      case 'QUARANTINE_VERIFIED_STAGED':
        return 'badge-foster-staged';
      case 'ACTIVE_IN_FOSTER_HOME':
        return 'badge-foster-active';
      default:
        return 'badge-foster-pending';
    }
  }

  /**
   * Map FosterBiosecurityTier to WCAG AAA badge CSS class
   */
  function getTierBadgeClass(tier) {
    switch (tier) {
      case 'ISO_QUARANTINE_CONTAGION':
        return 'badge-bio-iso';
      case 'MEDICAL_RECOVERY_SINGLE_PET':
        return 'badge-bio-medical';
      case 'BIOSECURITY_CLEARED_STANDARD':
        return 'badge-bio-cleared';
      default:
        return 'badge-bio-iso';
    }
  }

  /**
   * Update KPI summary counters
   */
  function updateKpiCounters(placements) {
    const totalEl = document.getElementById('kpi-foster-total');
    const isoEl = document.getElementById('kpi-foster-iso');
    const contagionEl = document.getElementById('kpi-foster-contagion');
    const activeEl = document.getElementById('kpi-foster-active');

    const total = placements.length;
    const iso = placements.filter((p) => p.biosecurityTier === 'ISO_QUARANTINE_CONTAGION').length;
    const contagion = placements.filter((p) => Boolean(p.isoQuarantineAlert)).length;
    const active = placements.filter((p) => p.placementStatus === 'ACTIVE_IN_FOSTER_HOME').length;

    if (totalEl) totalEl.textContent = String(total);
    if (isoEl) isoEl.textContent = String(iso);
    if (contagionEl) contagionEl.textContent = String(contagion);
    if (activeEl) activeEl.textContent = String(active);
  }

  /**
   * Render a single FosterPlacementMatch card
   */
  function renderPlacementCardHtml(p) {
    const placementId = escapeHtml(p.placementId || '');
    const petName = escapeHtml(p.petName || '');
    const species = escapeHtml(p.species || 'DOG');
    const hostName = escapeHtml(p.fosterHostName || '');
    const residentPets = Number(p.residentPetsCount ?? 0);
    const status = escapeHtml(p.placementStatus || 'PENDING_BIOSECURITY_CLEARANCE');
    const statusClass = getStatusBadgeClass(p.placementStatus);
    const tier = escapeHtml(p.biosecurityTier || 'ISO_QUARANTINE_CONTAGION');
    const tierClass = getTierBadgeClass(p.biosecurityTier);
    const riskScore = Number(p.contagionRiskScore ?? 80);
    const contagionFlag = escapeHtml(p.contagionFlag || 'PARVO_SUSPECTED');
    const medNeed = escapeHtml(p.medicalNeedLevel || 'INJECTABLE_SUBQ_FLUIDS');
    const readiness = Number(p.hostReadinessScore ?? 0);

    const isoBannerHtml = p.isoQuarantineAlert
      ? `
        <div class="foster-iso-quarantine-banner">
          ☣️ BIOSECURITY ISOLATION PROTOCOL REQUIRED: ${contagionFlag} (${medNeed}) — Enforce AHP bleach footbath, dedicated gowning airlock &amp; zero shared HVAC/yard with ${residentPets} resident pets.
        </div>
      `
      : '';

    const authBannerHtml = p.placementAuthorizationCode
      ? `
        <div class="foster-authorization-banner">
          ✅ FOSTER BIOSECURITY AUTHORIZED: ${escapeHtml(p.placementAuthorizationCode)}
        </div>
      `
      : '';

    const checks = Array.isArray(p.biosecurityChecks) ? p.biosecurityChecks : [];
    const checksHtml = checks
      .map((chk) => {
        const done = Boolean(chk.completed);
        const badgeClass = done ? 'badge-hop-match' : 'badge-hop-miss';
        const statusText = done ? 'VERIFIED' : 'REQUIRED';
        const category = escapeHtml(chk.category || '');
        const title = escapeHtml(chk.title || '');
        return `
          <div class="foster-check-item">
            <span class="badge ${badgeClass}">${statusText}</span>
            <strong class="foster-check-category">[${category}]</strong>
            <span class="foster-check-title">${title}</span>
          </div>
        `;
      })
      .join('');

    let actionsHtml = '';
    if (p.placementStatus !== 'ACTIVE_IN_FOSTER_HOME') {
      actionsHtml = `
        <button type="button" class="btn btn-sm btn-secondary btn-verify-foster-bio" data-placement-id="${placementId}">🧪 Verify HEPA Quarantine Rig &amp; Vet Tech Med Cert</button>
        <button type="button" class="btn btn-sm btn-primary btn-activate-foster-placement" data-placement-id="${placementId}">🏡 Complete Biosecurity Clearance &amp; Authorize Foster Placement</button>
      `;
    }

    return `
      <article class="trust-screening-card foster-placement-card" data-placement-id="${placementId}" data-placement-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · Host: ${hostName} · ${residentPets} resident pets)</strong>
            <code class="trust-pet-id">${placementId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-foster-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-bio-tier ${tierClass}">${tier} (Risk: ${riskScore}/100)</span>
          <span class="badge badge-foster-metric">Contagion: ${contagionFlag} • Med Need: ${medNeed} • Host Readiness: ${readiness}/100</span>
        </div>

        ${isoBannerHtml}
        ${authBannerHtml}

        <div class="foster-checks-list" aria-label="Quarantine Biosecurity &amp; Medical Competency Checklist">
          ${checksHtml}
        </div>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  /**
   * Render all foster placements in `#foster-placements-list`
   */
  function renderAllPlacements(placements) {
    currentPlacements = placements;
    updateKpiCounters(placements);

    const countBadge = document.getElementById('foster-placements-count-badge');
    if (countBadge) {
      countBadge.textContent = `${placements.length} Placements`;
    }

    const listEl = document.getElementById('foster-placements-list');
    if (!listEl) return;

    if (placements.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="foster-placements-empty-msg">No emergency foster placement matches recorded yet.</p>';
      return;
    }

    listEl.innerHTML = placements.map(renderPlacementCardHtml).join('');
  }

  /**
   * Fetch all foster placements from `/api/v1/foster-network/placements`
   */
  async function fetchPlacements() {
    try {
      const res = await fetch('/api/v1/foster-network/placements', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return [];
      const data = await res.json();
      const placements = Array.isArray(data.placements) ? data.placements : [];
      renderAllPlacements(placements);
      return placements;
    } catch (_err) {
      return [];
    }
  }

  /**
   * Submit a create foster placement request to `/api/v1/foster-network/placements`
   */
  async function submitCreatePlacementRequest(payload) {
    const res = await fetch('/api/v1/foster-network/placements', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchPlacements();
      showToast(
        `Evaluated foster match ${created.placementId} for ${created.petName} → ${created.placementStatus} (${created.biosecurityTier})`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate foster placement', 'error');
    return null;
  }

  /**
   * Setup Evaluate Foster Placement Form and Scenario Simulators
   */
  function setupCreateFormAndSimulators() {
    const form = document.getElementById('form-create-foster-placement');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petId = (document.getElementById('foster-pet-id')?.value || 'pet-2026-002').trim();
        const petName = (document.getElementById('foster-pet-name')?.value || 'Barnaby').trim();
        const species = (document.getElementById('foster-pet-species')?.value || 'DOG').trim();
        const fosterHostName = (
          document.getElementById('foster-host-name')?.value || 'Elena Vance (RN / Vet Tech)'
        ).trim();
        const contagionFlag = (
          document.getElementById('foster-contagion-flag')?.value || 'PARVO_SUSPECTED'
        ).trim();
        const medicalNeedLevel = (
          document.getElementById('foster-medical-need')?.value || 'INJECTABLE_SUBQ_FLUIDS'
        ).trim();
        const residentPetsCount = parseInt(
          document.getElementById('foster-resident-pets')?.value || '0',
          10
        );
        const hepaIsolationReady = Boolean(document.getElementById('foster-hepa-ready')?.checked);
        const medCertVerified = Boolean(document.getElementById('foster-med-cert')?.checked);

        const submitBtn = document.getElementById('btn-create-foster-placement');
        if (submitBtn) submitBtn.disabled = true;

        try {
          await submitCreatePlacementRequest({
            petId,
            petName,
            species,
            fosterHostName,
            contagionFlag,
            medicalNeedLevel,
            residentPetsCount: Number.isNaN(residentPetsCount) ? 0 : residentPetsCount,
            hepaIsolationReady,
            medCertVerified,
          });
        } catch (_err) {
          showToast('Network error evaluating foster placement', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simParvoBtn = document.getElementById('btn-sim-parvo-iso');
    if (simParvoBtn) {
      simParvoBtn.addEventListener('click', async () => {
        simParvoBtn.disabled = true;
        try {
          const contagionEl = document.getElementById('foster-contagion-flag');
          const medEl = document.getElementById('foster-medical-need');
          const residentEl = document.getElementById('foster-resident-pets');
          if (contagionEl) contagionEl.value = 'PARVO_SUSPECTED';
          if (medEl) medEl.value = 'INJECTABLE_SUBQ_FLUIDS';
          if (residentEl) residentEl.value = '2';

          await submitCreatePlacementRequest({
            petId: 'pet-2026-002',
            petName: 'Barnaby',
            species: 'DOG',
            fosterHostName: 'Elena Vance (RN / Vet Tech)',
            contagionFlag: 'PARVO_SUSPECTED',
            medicalNeedLevel: 'INJECTABLE_SUBQ_FLUIDS',
            residentPetsCount: 2,
            hepaIsolationReady: false,
            medCertVerified: false,
          });
        } finally {
          simParvoBtn.disabled = false;
        }
      });
    }

    const simRingwormBtn = document.getElementById('btn-sim-ringworm-cat');
    if (simRingwormBtn) {
      simRingwormBtn.addEventListener('click', async () => {
        simRingwormBtn.disabled = true;
        try {
          const contagionEl = document.getElementById('foster-contagion-flag');
          const medEl = document.getElementById('foster-medical-need');
          const residentEl = document.getElementById('foster-resident-pets');
          if (contagionEl) contagionEl.value = 'RINGWORM_DERMATOPHYTE';
          if (medEl) medEl.value = 'WOUND_DRESSING_BID';
          if (residentEl) residentEl.value = '0';

          await submitCreatePlacementRequest({
            petId: 'pet-2026-001',
            petName: 'Mochi',
            species: 'CAT',
            fosterHostName: 'Marcus Sterling',
            contagionFlag: 'RINGWORM_DERMATOPHYTE',
            medicalNeedLevel: 'WOUND_DRESSING_BID',
            residentPetsCount: 0,
            hepaIsolationReady: false,
            medCertVerified: false,
          });
        } finally {
          simRingwormBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup card action buttons in `#foster-placements-list`
   */
  function setupPlacementCardActions() {
    const listEl = document.getElementById('foster-placements-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const verifyBtn = e.target.closest('.btn-verify-foster-bio');
      const activateBtn = e.target.closest('.btn-activate-foster-placement');

      const btn = verifyBtn || activateBtn;
      if (!btn) return;

      const placementId = btn.getAttribute('data-placement-id');
      if (!placementId) return;

      btn.disabled = true;
      try {
        if (verifyBtn) {
          const res = await fetch(
            `/api/v1/foster-network/placements/${encodeURIComponent(placementId)}/transition`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                verifyBiosecurityRig: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchPlacements();
            showToast(
              `Verified HEPA quarantine rig & vet tech med cert for ${placementId} → ${updated.placementStatus} (Host Readiness: ${updated.hostReadinessScore}/100)`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to verify foster biosecurity rig', 'error');
          }
        } else if (activateBtn) {
          const res = await fetch(
            `/api/v1/foster-network/placements/${encodeURIComponent(placementId)}/transition`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                activateFosterPlacement: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchPlacements();
            showToast(
              `Authorized foster placement ${placementId} → ${updated.placementStatus} (${updated.placementAuthorizationCode})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to activate foster placement', 'error');
          }
        }
      } catch (_err) {
        showToast('Network error updating foster placement', 'error');
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
   * Subscribe to SSE stream `/api/v1/foster-network/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/foster-network/stream');
      const refresh = () => {
        fetchPlacements();
      };
      sseSource.addEventListener('foster_placement_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'foster_placement_updated') {
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
    setupPlacementCardActions();
    fetchPlacements();
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
