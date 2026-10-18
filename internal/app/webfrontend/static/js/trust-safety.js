/**
 * PetSpotR Multi-Modal False-Positive & Scam/Hoax Sighting Shield
 * Trust & Safety Moderator Quarantine Cockpit Controller (Milestone 13.2)
 */

(function () {
  'use strict';

  let sseSource = null;

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
   * Display toast notification & screen reader announcement
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
    const announcer = document.getElementById('trust-aria-announcer');
    if (announcer) {
      announcer.textContent = message;
    }
  }

  /**
   * Map TrustScreeningVerdict to WCAG AAA badge CSS class
   */
  function getVerdictBadgeClass(verdict) {
    switch (verdict) {
      case 'VERIFIED_AUTHENTIC':
        return 'badge-verdict-authentic';
      case 'HOLD_FOR_MODERATION':
        return 'badge-verdict-hold';
      case 'QUARANTINED_SCAM_RISK':
        return 'badge-verdict-quarantined';
      case 'REJECTED_CONFIRMED_HOAX':
        return 'badge-verdict-rejected';
      default:
        return 'badge-verdict-hold';
    }
  }

  /**
   * Render a single SightingTrustScreening card
   */
  function renderScreeningCardHtml(s) {
    const screeningId = escapeHtml(s.screeningId || '');
    const petName = escapeHtml(s.petName || '');
    const petId = escapeHtml(s.petId || '');
    const reporterHandle = escapeHtml(s.reporterHandle || '');
    const reporterTrustScore = Number(s.reporterTrustScore ?? 0);
    const verdict = escapeHtml(s.verdict || 'HOLD_FOR_MODERATION');
    const verdictClass = getVerdictBadgeClass(s.verdict);
    const riskScore = Number(s.riskScore ?? 0);
    const velocity = Number(s.impliedVelocityKmh ?? 0).toFixed(1);
    const distance = Number(s.distanceFromPrevKm ?? 0).toFixed(1);
    const elapsed = Number(s.elapsedMinutes ?? 0).toFixed(0);
    const imageTag = escapeHtml(s.imageIntegrityTag || 'CAMERA_EXIF_INTACT');
    const notes = escapeHtml(s.submittedNotes || '');
    const summary = escapeHtml(s.explanationSummary || '');
    const flags = Array.isArray(s.anomalyFlags) ? s.anomalyFlags : [];

    const flagsHtml = flags
      .map((flag) => {
        const safeFlag = escapeHtml(flag);
        return `<span class="badge badge-anomaly-flag" data-flag="${safeFlag}">⚠️ ${safeFlag}</span>`;
      })
      .join('');

    const isQuarantinedOrRejected =
      s.verdict === 'QUARANTINED_SCAM_RISK' || s.verdict === 'REJECTED_CONFIRMED_HOAX';
    const quarantineBannerHtml = isQuarantinedOrRejected
      ? `<div class="trust-quarantine-banner">🛡️ OWNER SHIELD ACTIVE: Extortion/Hoax blocked — distressed owner notification suppressed</div>`
      : '';

    const promotedBannerHtml = s.promotedSightingId
      ? `<div class="trust-promoted-banner">📍 Promoted to Live Community Sighting: ${escapeHtml(s.promotedSightingId)}</div>`
      : '';

    const summaryHtml = summary ? `<p class="trust-explanation-summary">${summary}</p>` : '';

    return `
      <article class="trust-screening-card" data-screening-id="${screeningId}" data-verdict="${verdict}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName}</strong>
            <code class="trust-pet-id">${petId}</code>
            <span class="trust-reporter-handle">👤 ${reporterHandle}</span>
            <span class="badge badge-reporter-trust">Trust: ${reporterTrustScore}/100</span>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-trust-verdict ${verdictClass}">${verdict}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-risk-score">Risk: ${riskScore}/100</span>
          <span class="badge badge-kinematic-velocity">Velocity: ${velocity} km/h (${distance} km / ${elapsed}m)</span>
          <span class="badge badge-image-integrity">${imageTag}</span>
        </div>

        <div class="trust-anomaly-flags">
          ${flagsHtml}
        </div>

        <p class="trust-screening-notes">${notes}</p>
        ${summaryHtml}
        ${quarantineBannerHtml}
        ${promotedBannerHtml}

        <div class="trust-moderator-actions">
          <button type="button" class="btn btn-sm btn-secondary btn-approve-screening" data-screening-id="${screeningId}">✅ Approve &amp; Promote to Sighting</button>
          <button type="button" class="btn btn-sm btn-secondary btn-reject-screening" data-screening-id="${screeningId}">🚫 Confirm Scam / Hoax &amp; Block</button>
        </div>
      </article>
    `;
  }

  /**
   * Render all screenings in `#trust-screenings-list`
   */
  function renderAllScreenings(screenings) {
    const countBadge = document.getElementById('trust-screenings-count-badge');
    if (countBadge) {
      countBadge.textContent = `${screenings.length} Screenings`;
    }

    const listEl = document.getElementById('trust-screenings-list');
    if (!listEl) return;

    if (screenings.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="trust-screenings-empty-msg">No trust screenings recorded yet.</p>';
      return;
    }

    listEl.innerHTML = screenings.map(renderScreeningCardHtml).join('');
  }

  /**
   * Fetch all trust screenings from `/api/v1/trust-safety/screenings`
   */
  async function fetchScreenings() {
    try {
      const res = await fetch('/api/v1/trust-safety/screenings', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return;
      const data = await res.json();
      const screenings = Array.isArray(data.screenings) ? data.screenings : [];
      renderAllScreenings(screenings);
    } catch (_err) {
      // Ignore transient network errors
    }
  }

  /**
   * Submit a trust screening payload to `POST /api/v1/trust-safety/screenings`
   */
  async function submitScreeningPayload(payload) {
    try {
      const res = await fetch('/api/v1/trust-safety/screenings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      if (res.ok) {
        const created = await res.json();
        await fetchScreenings();
        showToast(
          `Screened sighting for ${payload.petName || payload.petId}: ${created.verdict || ''}`,
          'success'
        );
      } else {
        const errData = await res.json().catch(() => ({}));
        showToast(errData.error || 'Failed to screen inbound sighting', 'error');
      }
    } catch (_err) {
      showToast('Network error screening inbound sighting', 'error');
    }
  }

  /**
   * Setup Inbound Sighting Trust & Scam Shield Evaluator form and simulation buttons
   */
  function setupEvaluatorFormAndSimulations() {
    const form = document.getElementById('form-submit-trust-screening');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petId = (document.getElementById('ts-pet-id')?.value || '').trim();
        const petName = (document.getElementById('ts-pet-name')?.value || '').trim();
        const reporterHandle = (document.getElementById('ts-reporter-handle')?.value || '').trim();
        const reporterTrustScore = parseInt(
          document.getElementById('ts-reporter-trust')?.value || '82',
          10
        );
        const prevRefLat = parseFloat(document.getElementById('ts-prev-lat')?.value || '47.6062');
        const prevRefLng = parseFloat(document.getElementById('ts-prev-lng')?.value || '-122.3321');
        const sightingLat = parseFloat(
          document.getElementById('ts-sighting-lat')?.value || '47.6095'
        );
        const sightingLng = parseFloat(
          document.getElementById('ts-sighting-lng')?.value || '-122.3350'
        );
        const elapsedMinutes = parseFloat(
          document.getElementById('ts-elapsed-min')?.value || '30'
        );
        const imageIntegrityTag =
          document.getElementById('ts-image-integrity')?.value || 'CAMERA_EXIF_INTACT';
        const submittedNotes = (
          document.getElementById('ts-submitted-notes')?.value || ''
        ).trim();

        const submitBtn = document.getElementById('btn-submit-trust-screening');
        if (submitBtn) submitBtn.disabled = true;
        try {
          await submitScreeningPayload({
            petId,
            petName,
            reporterHandle,
            reporterTrustScore: Number.isNaN(reporterTrustScore) ? 82 : reporterTrustScore,
            prevRefLat: Number.isNaN(prevRefLat) ? 47.6062 : prevRefLat,
            prevRefLng: Number.isNaN(prevRefLng) ? -122.3321 : prevRefLng,
            sightingLat: Number.isNaN(sightingLat) ? 47.6095 : sightingLat,
            sightingLng: Number.isNaN(sightingLng) ? -122.3350 : sightingLng,
            elapsedMinutes: Number.isNaN(elapsedMinutes) ? 30 : elapsedMinutes,
            imageIntegrityTag,
            submittedNotes,
          });
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simExtortionBtn = document.getElementById('btn-sim-extortion-scam');
    if (simExtortionBtn) {
      simExtortionBtn.addEventListener('click', async () => {
        const petId = (document.getElementById('ts-pet-id')?.value || 'pet-trust-daisy').trim();
        const petName = (document.getElementById('ts-pet-name')?.value || 'Daisy').trim();

        simExtortionBtn.disabled = true;
        try {
          await submitScreeningPayload({
            petId,
            petName,
            reporterHandle: 'burner-extort-77',
            reporterTrustScore: 10,
            prevRefLat: 47.6062,
            prevRefLng: -122.3321,
            sightingLat: 47.6085,
            sightingLng: -122.3340,
            elapsedMinutes: 20,
            imageIntegrityTag: 'SYNTHETIC_GAN_SIGNATURE',
            submittedNotes:
              'Send $400 via Bitcoin or Zelle upfront and give me the 6-digit verification code to get your dog back',
          });
        } finally {
          simExtortionBtn.disabled = false;
        }
      });
    }

    const simTeleportBtn = document.getElementById('btn-sim-teleport-hoax');
    if (simTeleportBtn) {
      simTeleportBtn.addEventListener('click', async () => {
        const petId = (document.getElementById('ts-pet-id')?.value || 'pet-trust-daisy').trim();
        const petName = (document.getElementById('ts-pet-name')?.value || 'Daisy').trim();

        simTeleportBtn.disabled = true;
        try {
          await submitScreeningPayload({
            petId,
            petName,
            reporterHandle: 'highway-spotter-91',
            reporterTrustScore: 45,
            prevRefLat: 47.6062,
            prevRefLng: -122.3321,
            sightingLat: 47.2000,
            sightingLng: -122.4500,
            elapsedMinutes: 15,
            imageIntegrityTag: 'STRIPPED_EXIF',
            submittedNotes: 'Saw the dog running near highway exit',
          });
        } finally {
          simTeleportBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup Moderator Approve & Reject action buttons in `#trust-screenings-list`
   */
  function setupModeratorQueueActions() {
    const listEl = document.getElementById('trust-screenings-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const approveBtn = e.target.closest('.btn-approve-screening');
      const rejectBtn = e.target.closest('.btn-reject-screening');
      if (!approveBtn && !rejectBtn) return;

      const btn = approveBtn || rejectBtn;
      const screeningId = btn.getAttribute('data-screening-id');
      if (!screeningId) return;

      const payload = approveBtn
        ? {
            verdict: 'VERIFIED_AUTHENTIC',
            moderatorNotes: 'Moderator verified & cleared for live SAR map',
          }
        : {
            verdict: 'REJECTED_CONFIRMED_HOAX',
            moderatorNotes: 'Confirmed extortion/hoax actor — permanently blocked',
          };

      btn.disabled = true;
      try {
        const res = await fetch(
          `/api/v1/trust-safety/screenings/${encodeURIComponent(screeningId)}/decision`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          }
        );
        if (res.ok) {
          await fetchScreenings();
          showToast(`Updated screening ${screeningId} to ${payload.verdict}`, 'success');
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to apply moderator decision', 'error');
        }
      } catch (_err) {
        showToast('Network error applying moderator decision', 'error');
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
   * Subscribe to SSE stream `/api/v1/trust-safety/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/trust-safety/stream');
      const refresh = () => {
        fetchScreenings();
      };
      sseSource.addEventListener('trust_screening_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'trust_screening_updated') {
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
    setupEvaluatorFormAndSimulations();
    setupModeratorQueueActions();
    fetchScreenings();
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
