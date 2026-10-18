/**
 * PetSpotR Forensic DNA, Coat-Pattern Biometrics & Senior/Blind Pet
 * Cognitive Disorientation Triage Cockpit Controller (Milestone 14.5)
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
    const liveStatus = document.getElementById('biometric-forensics-live-status');
    if (liveStatus) {
      liveStatus.textContent = message;
    }
  }

  /**
   * Map ForensicResolutionStatus to WCAG AAA badge CSS class
   */
  function getStatusBadgeClass(status) {
    switch (status) {
      case 'PENDING_BIOMETRIC_SCAN':
        return 'badge-forensic-pending';
      case 'DERMATOGLYPHIC_CORRELATED':
        return 'badge-forensic-correlated';
      case 'IDENTITY_CONFIRMED_SAFE_EXTRACTION':
        return 'badge-forensic-confirmed';
      default:
        return 'badge-forensic-pending';
    }
  }

  /**
   * Map CognitiveEntrapmentRisk to WCAG AAA badge CSS class
   */
  function getEntrapmentRiskBadgeClass(risk) {
    switch (risk) {
      case 'CRITICAL_SUNDOWNING_ENTRAPMENT':
        return 'badge-entrapment-critical';
      case 'ELEVATED_SENSORY_DISORIENTATION':
        return 'badge-entrapment-elevated';
      case 'CONTAINED_SAFE_PERIPHERY':
        return 'badge-entrapment-safe';
      default:
        return 'badge-entrapment-critical';
    }
  }

  /**
   * Update KPI summary counters
   */
  function updateKpiCounters(cases) {
    const totalEl = document.getElementById('kpi-forensic-total');
    const entrapmentEl = document.getElementById('kpi-forensic-entrapment');
    const sundowningEl = document.getElementById('kpi-forensic-sundowning');
    const confirmedEl = document.getElementById('kpi-forensic-confirmed');

    const total = cases.length;
    const entrapment = cases.filter(
      (c) => c.entrapmentRisk === 'CRITICAL_SUNDOWNING_ENTRAPMENT'
    ).length;
    const sundowning = cases.filter((c) => Boolean(c.sundowningWanderAlert)).length;
    const confirmed = cases.filter(
      (c) => c.resolutionStatus === 'IDENTITY_CONFIRMED_SAFE_EXTRACTION'
    ).length;

    if (totalEl) totalEl.textContent = String(total);
    if (entrapmentEl) entrapmentEl.textContent = String(entrapment);
    if (sundowningEl) sundowningEl.textContent = String(sundowning);
    if (confirmedEl) confirmedEl.textContent = String(confirmed);
  }

  /**
   * Render a single BiometricForensicCase card
   */
  function renderForensicCaseCardHtml(c) {
    const caseId = escapeHtml(c.caseId || '');
    const petName = escapeHtml(c.petName || '');
    const species = escapeHtml(c.species || 'DOG');
    const ageYears = Number(c.ageYears ?? 14);
    const sensoryCondition = escapeHtml(c.sensoryCondition || 'CCD_SUNDOWNING_DEMENTIA');
    const biometricModality = escapeHtml(c.biometricModality || 'NASAL_PLANUM_RIDGE_PRINT');
    const nearestHazardType = escapeHtml(c.nearestHazardType || 'RETENTION_POND_CULVERT');
    const status = escapeHtml(c.resolutionStatus || 'PENDING_BIOMETRIC_SCAN');
    const statusClass = getStatusBadgeClass(c.resolutionStatus);
    const entrapmentRisk = escapeHtml(c.entrapmentRisk || 'CRITICAL_SUNDOWNING_ENTRAPMENT');
    const riskClass = getEntrapmentRiskBadgeClass(c.entrapmentRisk);
    const riskScore = Number(c.entrapmentRiskScore ?? 95);
    const confidenceScore = Number(c.biometricConfidenceScore ?? 0);

    const sundowningBannerHtml = c.sundowningWanderAlert
      ? `
        <div class="forensic-sundowning-alert-banner">
          🧠 SENIOR COGNITIVE SUNDOWNING &amp; ENTRAPMENT ALERT: ${sensoryCondition} (${ageYears}yo) near ${nearestHazardType} — Prioritize 150m dead-end corner &amp; culvert sweep + heated olfactory bedding lure.
        </div>
      `
      : '';

    const certBannerHtml = c.forensicCertificateId
      ? `
        <div class="forensic-certificate-banner">
          ✅ BIOMETRIC FORENSIC IDENTITY CONFIRMED: ${escapeHtml(c.forensicCertificateId)}
        </div>
      `
      : '';

    const steps = Array.isArray(c.verificationSteps) ? c.verificationSteps : [];
    const stepsHtml = steps
      .map((step) => {
        const done = Boolean(step.completed);
        const badgeClass = done ? 'badge-hop-match' : 'badge-hop-miss';
        const statusText = done ? 'VERIFIED' : 'PENDING';
        const category = escapeHtml(step.category || '');
        const title = escapeHtml(step.title || '');
        return `
          <div class="forensic-step-item">
            <span class="badge ${badgeClass}">${statusText}</span>
            <strong class="forensic-step-category">[${category}]</strong>
            <span class="forensic-step-title">${title}</span>
          </div>
        `;
      })
      .join('');

    let actionsHtml = '';
    if (c.resolutionStatus !== 'IDENTITY_CONFIRMED_SAFE_EXTRACTION') {
      actionsHtml = `
        <button type="button" class="btn btn-sm btn-secondary btn-correlate-biometric" data-case-id="${caseId}">🔬 Correlate Nasal Planum Biometrics &amp; Sweep 150m Culvert</button>
        <button type="button" class="btn btn-sm btn-primary btn-confirm-forensic-extraction" data-case-id="${caseId}">🧬 Confirm STR DNA / Biometric Identity &amp; Safe Extraction</button>
      `;
    }

    return `
      <article class="trust-screening-card forensic-case-card" data-case-id="${caseId}" data-resolution-status="${status}" role="listitem">
        <div class="trust-screening-card-header">
          <div class="trust-screening-identity">
            <strong class="trust-pet-name">${petName} (${species} · ${ageYears}yo · ${sensoryCondition})</strong>
            <code class="trust-pet-id">${caseId}</code>
          </div>
          <div class="trust-screening-verdict-wrap">
            <span class="badge badge-forensic-status ${statusClass}">${status}</span>
          </div>
        </div>

        <div class="trust-kinematics-row">
          <span class="badge badge-entrapment-risk ${riskClass}">${entrapmentRisk}</span>
          <span class="badge badge-forensic-metric">Entrapment Risk: ${riskScore}/100 • Biometric Confidence: ${confidenceScore}/100 • Modality: ${biometricModality} • Hazard: ${nearestHazardType}</span>
        </div>

        ${sundowningBannerHtml}
        ${certBannerHtml}

        <div class="forensic-steps-list" aria-label="Biometric Verification &amp; Sundowning Hazard Sweep Checklist">
          ${stepsHtml}
        </div>

        <div class="trust-moderator-actions">
          ${actionsHtml}
        </div>
      </article>
    `;
  }

  /**
   * Render all forensic cases in `#forensic-cases-list`
   */
  function renderAllCases(cases) {
    currentCases = cases;
    updateKpiCounters(cases);

    const countBadge = document.getElementById('forensic-cases-count-badge');
    if (countBadge) {
      countBadge.textContent = `${cases.length} Cases`;
    }

    const listEl = document.getElementById('forensic-cases-list');
    if (!listEl) return;

    if (cases.length === 0) {
      listEl.innerHTML =
        '<p class="trust-empty-hint" id="forensic-cases-empty-msg">No biometric forensic cases recorded yet.</p>';
      return;
    }

    listEl.innerHTML = cases.map(renderForensicCaseCardHtml).join('');
  }

  /**
   * Fetch all forensic cases from `/api/v1/biometric-forensics/cases`
   */
  async function fetchCases() {
    try {
      const res = await fetch('/api/v1/biometric-forensics/cases', {
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
   * Submit a create forensic case request to `/api/v1/biometric-forensics/cases`
   */
  async function submitCreateCaseRequest(payload) {
    const res = await fetch('/api/v1/biometric-forensics/cases', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const created = await res.json();
      await fetchCases();
      showToast(
        `Evaluated forensic case ${created.caseId} for ${created.petName} → ${created.resolutionStatus} (${created.entrapmentRisk})`,
        'success'
      );
      return created;
    }
    const errData = await res.json().catch(() => ({}));
    showToast(errData.error || 'Failed to evaluate biometric forensic case', 'error');
    return null;
  }

  /**
   * Setup Evaluate Forensic Case Form and Scenario Simulators
   */
  function setupCreateFormAndSimulators() {
    const form = document.getElementById('form-create-forensic-case');
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const petId = (document.getElementById('forensic-pet-id')?.value || 'pet-2026-001').trim();
        const petName = (document.getElementById('forensic-pet-name')?.value || 'Barnaby').trim();
        const species = (document.getElementById('forensic-pet-species')?.value || 'DOG').trim();
        const ageYears = parseInt(
          document.getElementById('forensic-age-years')?.value || '14',
          10
        );
        const sensoryCondition = (
          document.getElementById('forensic-sensory-condition')?.value || 'CCD_SUNDOWNING_DEMENTIA'
        ).trim();
        const biometricModality = (
          document.getElementById('forensic-modality')?.value || 'NASAL_PLANUM_RIDGE_PRINT'
        ).trim();
        const nearestHazardType = (
          document.getElementById('forensic-hazard-type')?.value || 'RETENTION_POND_CULVERT'
        ).trim();
        const nasalPrintMatched = Boolean(document.getElementById('forensic-nasal-matched')?.checked);
        const waterCulvertCleared = Boolean(
          document.getElementById('forensic-culvert-cleared')?.checked
        );

        const submitBtn = document.getElementById('btn-create-forensic-case');
        if (submitBtn) submitBtn.disabled = true;

        try {
          await submitCreateCaseRequest({
            petId,
            petName,
            species,
            ageYears: Number.isNaN(ageYears) ? 14 : ageYears,
            sensoryCondition,
            biometricModality,
            nearestHazardType,
            nasalPrintMatched,
            waterCulvertCleared,
          });
        } catch (_err) {
          showToast('Network error evaluating biometric forensic case', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }

    const simSundowningBtn = document.getElementById('btn-sim-sundowning-senior');
    if (simSundowningBtn) {
      simSundowningBtn.addEventListener('click', async () => {
        simSundowningBtn.disabled = true;
        try {
          const ageEl = document.getElementById('forensic-age-years');
          const sensoryEl = document.getElementById('forensic-sensory-condition');
          const modalityEl = document.getElementById('forensic-modality');
          const hazardEl = document.getElementById('forensic-hazard-type');
          if (ageEl) ageEl.value = '15';
          if (sensoryEl) sensoryEl.value = 'CCD_SUNDOWNING_DEMENTIA';
          if (modalityEl) modalityEl.value = 'NASAL_PLANUM_RIDGE_PRINT';
          if (hazardEl) hazardEl.value = 'RETENTION_POND_CULVERT';

          await submitCreateCaseRequest({
            petId: 'pet-2026-001',
            petName: 'Winston',
            species: 'DOG',
            ageYears: 15,
            sensoryCondition: 'CCD_SUNDOWNING_DEMENTIA',
            biometricModality: 'NASAL_PLANUM_RIDGE_PRINT',
            nearestHazardType: 'RETENTION_POND_CULVERT',
            nasalPrintMatched: false,
            waterCulvertCleared: false,
          });
        } finally {
          simSundowningBtn.disabled = false;
        }
      });
    }

    const simBlindCatBtn = document.getElementById('btn-sim-blind-cat-bramble');
    if (simBlindCatBtn) {
      simBlindCatBtn.addEventListener('click', async () => {
        simBlindCatBtn.disabled = true;
        try {
          const ageEl = document.getElementById('forensic-age-years');
          const sensoryEl = document.getElementById('forensic-sensory-condition');
          const modalityEl = document.getElementById('forensic-modality');
          const hazardEl = document.getElementById('forensic-hazard-type');
          if (ageEl) ageEl.value = '13';
          if (sensoryEl) sensoryEl.value = 'BILATERAL_CATARACT_BLIND';
          if (modalityEl) modalityEl.value = 'IRIS_MELANIN_TOPOLOGY';
          if (hazardEl) hazardEl.value = 'THICK_BLACKBERRY_BRAMBLE';

          await submitCreateCaseRequest({
            petId: 'pet-2026-002',
            petName: 'Pearl',
            species: 'CAT',
            ageYears: 13,
            sensoryCondition: 'BILATERAL_CATARACT_BLIND',
            biometricModality: 'IRIS_MELANIN_TOPOLOGY',
            nearestHazardType: 'THICK_BLACKBERRY_BRAMBLE',
            nasalPrintMatched: false,
            waterCulvertCleared: false,
          });
        } finally {
          simBlindCatBtn.disabled = false;
        }
      });
    }
  }

  /**
   * Setup card action buttons in `#forensic-cases-list`
   */
  function setupCaseCardActions() {
    const listEl = document.getElementById('forensic-cases-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const correlateBtn = e.target.closest('.btn-correlate-biometric');
      const confirmBtn = e.target.closest('.btn-confirm-forensic-extraction');

      const btn = correlateBtn || confirmBtn;
      if (!btn) return;

      const caseId = btn.getAttribute('data-case-id');
      if (!caseId) return;

      btn.disabled = true;
      try {
        if (correlateBtn) {
          const res = await fetch(
            `/api/v1/biometric-forensics/cases/${encodeURIComponent(caseId)}/verify`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                correlateBiometricAndSweepHazard: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchCases();
            showToast(
              `Correlated nasal planum biometrics & swept 150m culvert for ${caseId} → ${updated.resolutionStatus} (Biometric Confidence: ${updated.biometricConfidenceScore}/100)`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to correlate biometric case', 'error');
          }
        } else if (confirmBtn) {
          const res = await fetch(
            `/api/v1/biometric-forensics/cases/${encodeURIComponent(caseId)}/verify`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                confirmForensicExtraction: true,
              }),
            }
          );
          if (res.ok) {
            const updated = await res.json();
            await fetchCases();
            showToast(
              `Confirmed forensic extraction ${caseId} → ${updated.resolutionStatus} (${updated.forensicCertificateId})`,
              'success'
            );
          } else {
            const errData = await res.json().catch(() => ({}));
            showToast(errData.error || 'Failed to confirm forensic extraction', 'error');
          }
        }
      } catch (_err) {
        showToast('Network error updating biometric forensic case', 'error');
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
   * Subscribe to SSE stream `/api/v1/biometric-forensics/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/biometric-forensics/stream');
      const refresh = () => {
        fetchCases();
      };
      sseSource.addEventListener('forensic_case_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'forensic_case_updated') {
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
