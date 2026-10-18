/**
 * PetSpotR Incident Command (ICS-211 / ICS-204) Volunteer Shift Roster,
 * Cryptographic Role Credentialing, Responder Fatigue Guardrail Monitor &
 * SAR Gamification Cockpit Controller (Milestone 12.3)
 */

(function () {
  'use strict';

  const shiftCache = new Map();
  const volunteerCache = new Map();
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
    const announcer = document.getElementById('ics-aria-announcer');
    if (announcer) {
      announcer.textContent = message;
    }
  }

  /**
   * Map FatigueAlertLevel to WCAG AAA badge CSS class
   */
  function getFatigueBadgeClass(fatigueLevel) {
    switch (fatigueLevel) {
      case 'MANDATORY_REST':
        return 'badge-fatigue-rest';
      case 'FATIGUE_WARNING':
        return 'badge-fatigue-warning';
      case 'NORMAL_FIT':
      default:
        return 'badge-fatigue-normal';
    }
  }

  /**
   * Build HTML for a single volunteer card
   */
  function renderVolunteerCardHtml(v) {
    const safeId = escapeHtml(v.volunteerId || '');
    const safeName = escapeHtml(v.name || '');
    const safeCallsign = escapeHtml(v.callsign || '');
    const safeRank = escapeHtml(v.rankTitle || 'Rookie Spotter (Tier 1)');
    const totalXp = Number(v.totalXp || 0);
    const completedShifts = Number(v.completedShifts || 0);
    const totalDistanceKm = Number(v.totalDistanceKm || 0).toFixed(1);
    const totalHoursLogged = Number(v.totalHoursLogged || 0).toFixed(1);
    const verifiedSightings = Number(v.verifiedSightings || 0);
    const reunionsAssisted = Number(v.reunionsAssisted || 0);

    const creds = Array.isArray(v.credentials) ? v.credentials : [];
    const credsHtml = creds.length > 0
      ? creds
          .map((c) => {
            const role = escapeHtml(c.role || '');
            const lic = escapeHtml(c.licenseNumber || 'VERIFIED');
            const sig = escapeHtml(c.signatureHex || '');
            return `<span class="badge badge-sar-credential" data-role="${role}" title="HMAC-SHA256: ${sig}">🔐 ${role} (${lic})</span>`;
          })
          .join('')
      : '<span class="ics-empty-inline">No cryptographic credentials issued</span>';

    const badges = Array.isArray(v.earnedBadges) ? v.earnedBadges : [];
    const badgesHtml = badges.length > 0
      ? badges
          .map((b) => {
            const safeBadge = escapeHtml(b);
            return `<span class="badge badge-sar-merit" data-badge="${safeBadge}">🏅 ${safeBadge}</span>`;
          })
          .join('')
      : '<span class="ics-empty-inline">No merit badges earned yet</span>';

    return `
      <article class="volunteer-card" data-volunteer-id="${safeId}" role="listitem">
        <div class="volunteer-card-header">
          <div class="volunteer-identity">
            <strong class="volunteer-name">${safeName}</strong>
            <span class="badge badge-callsign">${safeCallsign}</span>
          </div>
          <div class="volunteer-rank-group">
            <span class="badge badge-sar-rank">${safeRank}</span>
            <span class="badge badge-xp-total">${totalXp} XP</span>
          </div>
        </div>
        <div class="volunteer-credentials-list">${credsHtml}</div>
        <div class="volunteer-badges-list">${badgesHtml}</div>
        <div class="volunteer-stats-row">
          <span>📋 ${completedShifts} Shifts</span>
          <span>🥾 ${totalDistanceKm} km</span>
          <span>⏱️ ${totalHoursLogged}h</span>
          <span>🔭 ${verifiedSightings} Sightings</span>
          <span>🐾 ${reunionsAssisted} Reunions</span>
        </div>
      </article>
    `;
  }

  /**
   * Render volunteers list and synchronize dropdown selects
   */
  function renderVolunteers(volunteers, selectVolunteerId = '') {
    const listEl = document.getElementById('volunteers-leaderboard');
    const countBadge = document.getElementById('volunteers-count-badge');
    const credSelect = document.getElementById('cred-volunteer-select');
    const shiftSelect = document.getElementById('shift-volunteer-select');

    volunteerCache.clear();
    volunteers.forEach((v) => {
      if (v && v.volunteerId) {
        volunteerCache.set(v.volunteerId, v);
      }
    });

    if (countBadge) {
      countBadge.textContent = `${volunteers.length} Responders`;
    }

    if (listEl) {
      if (volunteers.length === 0) {
        listEl.innerHTML = '<p class="ics-empty-hint" id="volunteers-empty-msg">No SAR responders registered yet.</p>';
      } else {
        listEl.innerHTML = volunteers.map(renderVolunteerCardHtml).join('');
      }
    }

    [credSelect, shiftSelect].forEach((sel) => {
      if (!sel) return;
      const previousVal = selectVolunteerId || sel.value;
      sel.innerHTML = volunteers
        .map(
          (v) =>
            `<option value="${escapeHtml(v.volunteerId)}">${escapeHtml(v.name)} (${escapeHtml(v.callsign)})</option>`
        )
        .join('');
      if (previousVal && volunteers.some((v) => v.volunteerId === previousVal)) {
        sel.value = previousVal;
      }
    });
  }

  /**
   * Build HTML for a single ICS-204 shift card
   */
  function renderShiftCardHtml(s) {
    const safeShiftId = escapeHtml(s.shiftId || '');
    const safeIncidentId = escapeHtml(s.incidentId || '');
    const safeVolName = escapeHtml(s.volunteerName || '');
    const safeCallsign = escapeHtml(s.callsign || '');
    const safeRole = escapeHtml(s.assignedRole || '');
    const safeSector = escapeHtml(s.assignedSectorId || '');
    const safeStatus = escapeHtml(s.status || 'ON_SECTOR_PATROL');
    const safeFatigue = escapeHtml(s.fatigueLevel || 'NORMAL_FIT');
    const fatigueBadgeClass = getFatigueBadgeClass(s.fatigueLevel);
    const activeHours = Number(s.activeHours || 0).toFixed(1);
    const distanceKm = Number(s.distanceCoveredKm || 0).toFixed(1);
    const safeNotes = escapeHtml(s.fatigueNotes || '');

    const actionsHtml =
      s.status !== 'CHECKED_OUT'
        ? `
          <div class="shift-card-actions">
            <button type="button" class="btn btn-sm btn-secondary btn-sim-fatigue-warn" data-shift-id="${safeShiftId}">⚠️ Simulate 6.5h Patrol (Fatigue Warning)</button>
            <button type="button" class="btn btn-sm btn-secondary btn-sim-mandatory-rest" data-shift-id="${safeShiftId}">🛑 Simulate 8.5h / 52km Patrol (Mandatory Rest)</button>
            <button type="button" class="btn btn-sm btn-primary btn-checkout-shift" data-shift-id="${safeShiftId}">✅ Check Out &amp; Award XP</button>
          </div>
        `
        : '';

    return `
      <article class="ics-shift-card" data-shift-id="${safeShiftId}" data-fatigue-level="${safeFatigue}" data-shift-status="${safeStatus}" role="listitem">
        <div class="shift-card-header">
          <div class="shift-responder-info">
            <strong class="shift-volunteer-name">${safeVolName}</strong>
            <span class="badge badge-callsign">${safeCallsign}</span>
            <span class="badge badge-assigned-role badge-sar-credential">${safeRole}</span>
          </div>
          <div class="shift-status-badges">
            <span class="badge badge-shift-status">${safeStatus}</span>
            <span class="badge badge-fatigue-level ${fatigueBadgeClass}">${safeFatigue}</span>
          </div>
        </div>
        <div class="shift-card-meta">
          <span>📍 Sector: <strong>${safeSector}</strong></span>
          <span>🚨 Incident: <code>${safeIncidentId}</code></span>
          <span class="shift-hours">${activeHours}h active</span>
          <span class="shift-distance">${distanceKm} km covered</span>
        </div>
        <p class="shift-fatigue-notes">${safeNotes}</p>
        ${actionsHtml}
      </article>
    `;
  }

  /**
   * Render shifts list in #ics-shifts-list
   */
  function renderShifts(shifts) {
    const listEl = document.getElementById('ics-shifts-list');
    const countBadge = document.getElementById('shifts-count-badge');

    shiftCache.clear();
    shifts.forEach((s) => {
      if (s && s.shiftId) {
        shiftCache.set(s.shiftId, s);
      }
    });

    if (countBadge) {
      countBadge.textContent = `${shifts.length} Shifts`;
    }

    if (listEl) {
      if (shifts.length === 0) {
        listEl.innerHTML = '<p class="ics-empty-hint" id="shifts-empty-msg">No active ICS-211 shifts checked in yet.</p>';
      } else {
        listEl.innerHTML = shifts.map(renderShiftCardHtml).join('');
      }
    }
  }

  /**
   * Fetch volunteers from API
   */
  async function fetchVolunteers(selectVolunteerId = '') {
    try {
      const res = await fetch('/api/v1/ics/volunteers');
      if (!res.ok) return;
      const data = await res.json();
      const volunteers = Array.isArray(data.volunteers) ? data.volunteers : [];
      renderVolunteers(volunteers, selectVolunteerId);
    } catch (_err) {
      // Ignore transient network errors
    }
  }

  /**
   * Fetch shifts from API
   */
  async function fetchShifts() {
    try {
      const res = await fetch('/api/v1/ics/shifts');
      if (!res.ok) return;
      const data = await res.json();
      const shifts = Array.isArray(data.shifts) ? data.shifts : [];
      renderShifts(shifts);
    } catch (_err) {
      // Ignore transient network errors
    }
  }

  /**
   * Setup theme toggle button
   */
  function setupThemeToggle() {
    const toggleBtn = document.getElementById('theme-toggle');
    if (!toggleBtn) return;

    toggleBtn.addEventListener('click', () => {
      const currentTheme = document.documentElement.getAttribute('data-theme') || 'dark';
      const nextTheme = currentTheme === 'dark' ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', nextTheme);
    });
  }

  /**
   * Handle Register Volunteer form submission
   */
  function setupRegisterVolunteerForm() {
    const form = document.getElementById('form-register-volunteer');
    if (!form) return;

    form.addEventListener('submit', async (e) => {
      e.preventDefault();

      const nameEl = document.getElementById('vol-name');
      const callsignEl = document.getElementById('vol-callsign');
      const roleEl = document.getElementById('vol-primary-role');
      const licenseEl = document.getElementById('vol-license');

      const name = nameEl ? nameEl.value.trim() : '';
      const callsign = callsignEl ? callsignEl.value.trim() : '';
      const primaryRole = roleEl ? roleEl.value : 'FIELD_SEARCHER';
      const licenseNumber = licenseEl ? licenseEl.value.trim() : '';

      if (!name || !callsign) {
        showToast('Responder name and callsign are required.', 'error');
        return;
      }

      try {
        const res = await fetch('/api/v1/ics/volunteers', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name,
            callsign,
            primaryRole,
            licenseNumber,
          }),
        });

        const payload = await res.json();
        if (!res.ok) {
          showToast(payload.error || 'Failed to register responder', 'error');
          return;
        }

        const created = payload.volunteer || payload;
        await fetchVolunteers(created.volunteerId || '');
        if (nameEl) nameEl.value = '';
        if (callsignEl) callsignEl.value = '';
        if (licenseEl) licenseEl.value = '';
        showToast(`Registered SAR responder ${created.name} (${created.callsign})`, 'success');
      } catch (_err) {
        showToast('Network error registering responder', 'error');
      }
    });
  }

  /**
   * Handle Issue Cryptographic Credential form submission
   */
  function setupIssueCredentialForm() {
    const form = document.getElementById('form-issue-credential');
    if (!form) return;

    form.addEventListener('submit', async (e) => {
      e.preventDefault();

      const volSelect = document.getElementById('cred-volunteer-select');
      const roleSelect = document.getElementById('cred-role-select');
      const issuedByEl = document.getElementById('cred-issued-by');
      const licenseEl = document.getElementById('cred-license-number');

      const volunteerId = volSelect ? volSelect.value.trim() : '';
      const role = roleSelect ? roleSelect.value.trim() : '';
      const issuedBy = issuedByEl ? issuedByEl.value.trim() : 'FEMA / FAA / NASAR Command';
      const licenseNumber = licenseEl ? licenseEl.value.trim() : '';

      if (!volunteerId || !role) {
        showToast('Select a responder and role credential.', 'error');
        return;
      }

      try {
        const res = await fetch(`/api/v1/ics/volunteers/${encodeURIComponent(volunteerId)}/credentials`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            role,
            issuedBy,
            licenseNumber,
          }),
        });

        const payload = await res.json();
        if (!res.ok) {
          showToast(payload.error || 'Failed to issue credential', 'error');
          return;
        }

        await fetchVolunteers();
        if (licenseEl) licenseEl.value = '';
        showToast(`Issued HMAC-SHA256 signed ${role} credential`, 'success');
      } catch (_err) {
        showToast('Network error issuing credential', 'error');
      }
    });
  }

  /**
   * Handle ICS-211 Shift Check-In form submission
   */
  function setupCheckinShiftForm() {
    const form = document.getElementById('form-checkin-shift');
    if (!form) return;

    form.addEventListener('submit', async (e) => {
      e.preventDefault();

      const incidentEl = document.getElementById('shift-incident-id');
      const volSelect = document.getElementById('shift-volunteer-select');
      const roleSelect = document.getElementById('shift-assigned-role');
      const sectorEl = document.getElementById('shift-sector-id');
      const alertEl = document.getElementById('role-gate-error-alert');

      const incidentId = incidentEl ? incidentEl.value.trim() : 'INC-2026-WILDFIRE-01';
      const volunteerId = volSelect ? volSelect.value.trim() : '';
      const assignedRole = roleSelect ? roleSelect.value.trim() : 'FIELD_SEARCHER';
      const assignedSectorId = sectorEl ? sectorEl.value.trim() : 'SECTOR-ALPHA-04';

      try {
        const res = await fetch('/api/v1/ics/shifts', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            incidentId,
            volunteerId,
            assignedRole,
            assignedSectorId,
          }),
        });

        const payload = await res.json();
        if (!res.ok) {
          const errMsg = payload.error || 'Role-gate verification failed (403 Forbidden)';
          if (alertEl) {
            alertEl.textContent = `🚫 ICS-211 Role-Gate Blocked: ${errMsg}`;
            alertEl.hidden = false;
          }
          showToast(errMsg, 'error');
          return;
        }

        if (alertEl) {
          alertEl.hidden = true;
          alertEl.textContent = '';
        }

        await Promise.all([fetchShifts(), fetchVolunteers()]);
        showToast(`Checked into ICS-211 shift (${assignedSectorId})`, 'success');
      } catch (_err) {
        showToast('Network error checking into shift', 'error');
      }
    });
  }

  /**
   * Handle Shift Progression & Fatigue Simulation Buttons via Event Delegation
   */
  function setupShiftActionsDelegation() {
    const listEl = document.getElementById('ics-shifts-list');
    if (!listEl) return;

    listEl.addEventListener('click', async (e) => {
      const btn = e.target.closest('button[data-shift-id]');
      if (!btn) return;

      const shiftId = btn.getAttribute('data-shift-id');
      if (!shiftId) return;

      const cachedShift = shiftCache.get(shiftId) || {};
      let bodyPayload = null;

      if (btn.classList.contains('btn-sim-fatigue-warn')) {
        bodyPayload = {
          activeHours: 6.5,
          distanceCoveredKm: 19.5,
          verifiedSightingsDelta: 1,
        };
      } else if (btn.classList.contains('btn-sim-mandatory-rest')) {
        bodyPayload = {
          activeHours: 8.5,
          distanceCoveredKm: 52.0,
          nightShift: true,
        };
      } else if (btn.classList.contains('btn-checkout-shift')) {
        bodyPayload = {
          activeHours: cachedShift.activeHours || 4.0,
          distanceCoveredKm: cachedShift.distanceCoveredKm || 12.0,
          reunionsAssistedDelta: 1,
          checkout: true,
        };
      }

      if (!bodyPayload) return;

      btn.disabled = true;
      try {
        const res = await fetch(`/api/v1/ics/shifts/${encodeURIComponent(shiftId)}/progress`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(bodyPayload),
        });

        if (res.ok) {
          await Promise.all([fetchShifts(), fetchVolunteers()]);
          showToast('Updated ICS shift telemetry & responder XP progression', 'success');
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to update shift progress', 'error');
        }
      } catch (_err) {
        showToast('Network error updating shift progress', 'error');
      } finally {
        btn.disabled = false;
      }
    });
  }

  /**
   * Subscribe to SSE stream `/api/v1/ics/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/ics/stream');
      const refreshAll = () => {
        fetchVolunteers();
        fetchShifts();
      };
      sseSource.addEventListener('ics_volunteer_updated', refreshAll);
      sseSource.addEventListener('ics_shift_updated', refreshAll);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (
            parsed.type === 'ics_volunteer_updated' ||
            parsed.type === 'ics_shift_updated'
          ) {
            refreshAll();
          }
        } catch (_e) {
          // Ignore malformed keepalive or non-JSON messages
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
    setupRegisterVolunteerForm();
    setupIssueCredentialForm();
    setupCheckinShiftForm();
    setupShiftActionsDelegation();
    fetchVolunteers();
    fetchShifts();
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
