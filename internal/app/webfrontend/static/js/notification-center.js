// Client-side controller for Notification Center Drawer and Alert Preferences Modal
(() => {
  'use strict';

  let miniMap = null;
  let miniMapCenterMarker = null;
  let miniMapRadiusCircle = null;
  let drawerLastFocused = null;
  let modalLastFocused = null;

  // Safe HTML entity escaper
  function escapeHTML(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  // Format notification timestamps relative to current time
  function formatNotificationTime(dateStr) {
    if (!dateStr) return '';
    try {
      const date = new Date(dateStr);
      if (isNaN(date.getTime())) return '';
      const now = new Date();
      const diffMs = now.getTime() - date.getTime();
      if (diffMs < 0) return 'Just now';
      const diffSec = Math.floor(diffMs / 1000);
      const diffMin = Math.floor(diffSec / 60);
      const diffHr = Math.floor(diffMin / 60);
      const diffDay = Math.floor(diffHr / 24);

      if (diffSec < 60) return 'Just now';
      if (diffMin < 60) return `${diffMin}m ago`;
      if (diffHr < 24) return `${diffHr}h ago`;
      if (diffDay < 7) return `${diffDay}d ago`;
      return date.toLocaleDateString();
    } catch (_) {
      return '';
    }
  }

  // Retrieve active CSRF token from identity adapter, cookie, or session endpoint
  async function getCSRFToken() {
    if (window.petspotrIdentity && typeof window.petspotrIdentity.getState === 'function') {
      const idState = window.petspotrIdentity.getState();
      if (idState && idState.csrfToken) {
        return idState.csrfToken;
      }
    }

    const match = document.cookie.match(/(?:^|;\s*)(?:__Host-)?petspotr_csrf=([^;]+)/);
    if (match && match[1]) {
      return decodeURIComponent(match[1]);
    }

    try {
      const res = await fetch('/api/v1/session/csrf', { credentials: 'same-origin', cache: 'no-store' });
      if (res.ok) {
        const data = await res.json();
        return data.csrfToken || '';
      }
    } catch (err) {
      console.warn('Failed to fetch CSRF token:', err);
    }
    return '';
  }

  // Focus trap for accessible dialogs and drawers
  function trapFocus(container, event) {
    if (!container || event.key !== 'Tab') return;
    const focusable = container.querySelectorAll(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    );
    if (focusable.length === 0) return;

    const first = focusable[0];
    const last = focusable[focusable.length - 1];

    if (event.shiftKey) {
      if (document.activeElement === first) {
        event.preventDefault();
        last.focus();
      }
    } else {
      if (document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }
  }

  // --- Notification Drawer Controller ---

  function openNotificationDrawer() {
    const drawer = document.getElementById('notification-drawer');
    const backdrop = document.getElementById('drawer-backdrop');
    const triggerBtn = document.getElementById('btn-notification-drawer');
    if (!drawer) return;

    drawerLastFocused = document.activeElement;
    drawer.classList.add('is-open');
    drawer.setAttribute('aria-hidden', 'false');
    if (triggerBtn) triggerBtn.setAttribute('aria-expanded', 'true');
    if (backdrop) backdrop.classList.remove('hidden');

    const closeBtn = document.getElementById('btn-close-drawer');
    if (closeBtn) closeBtn.focus();
  }

  function closeNotificationDrawer() {
    const drawer = document.getElementById('notification-drawer');
    const backdrop = document.getElementById('drawer-backdrop');
    const triggerBtn = document.getElementById('btn-notification-drawer');
    if (!drawer) return;

    drawer.classList.remove('is-open');
    drawer.setAttribute('aria-hidden', 'true');
    if (triggerBtn) triggerBtn.setAttribute('aria-expanded', 'false');
    if (backdrop) backdrop.classList.add('hidden');

    if (drawerLastFocused && typeof drawerLastFocused.focus === 'function') {
      drawerLastFocused.focus();
    } else if (triggerBtn) {
      triggerBtn.focus();
    }
  }

  function initNotificationDrawer() {
    const triggerBtn = document.getElementById('btn-notification-drawer');
    const closeBtn = document.getElementById('btn-close-drawer');
    const backdrop = document.getElementById('drawer-backdrop');
    const drawer = document.getElementById('notification-drawer');

    if (!drawer) return;

    if (triggerBtn) {
      triggerBtn.addEventListener('click', () => {
        if (drawer.classList.contains('is-open')) {
          closeNotificationDrawer();
        } else {
          openNotificationDrawer();
        }
      });
    }

    if (closeBtn) {
      closeBtn.addEventListener('click', closeNotificationDrawer);
    }

    if (backdrop) {
      backdrop.addEventListener('click', closeNotificationDrawer);
    }

    drawer.addEventListener('keydown', (e) => {
      trapFocus(drawer, e);
    });
  }

  // --- Notification Feed & Cards ---

  function updateLocalUnreadCount() {
    const unreadItems = document.querySelectorAll('#notification-list .notification-item.is-unread');
    const count = unreadItems.length;
    const badge = document.getElementById('notification-badge');
    const countEl = document.getElementById('drawer-unread-count');

    if (badge) {
      badge.textContent = String(count);
      if (count > 0) {
        badge.classList.remove('hidden');
      } else {
        badge.classList.add('hidden');
      }
    }

    if (countEl) {
      countEl.textContent = `${count} unread`;
    }
  }

  async function markNotificationAsRead(id) {
    updateLocalUnreadCount();

    // Persist for guest sessions
    try {
      let readIds = [];
      const raw = localStorage.getItem('petspotr_read_notifications');
      if (raw) readIds = JSON.parse(raw);
      if (!readIds.includes(id)) {
        readIds.push(id);
        localStorage.setItem('petspotr_read_notifications', JSON.stringify(readIds));
      }
    } catch (_) {}

    try {
      const csrfToken = await getCSRFToken();
      const headers = { 'Content-Type': 'application/json' };
      if (csrfToken) headers['X-CSRF-Token'] = csrfToken;

      await fetch('/api/v1/notifications/mark-read', {
        method: 'POST',
        credentials: 'same-origin',
        headers,
        body: JSON.stringify({ notificationIds: [id] }),
        keepalive: true
      });
    } catch (err) {
      console.warn('Failed to mark notification as read on server:', err);
    }
  }

  function createNotificationCard(item) {
    const card = document.createElement('a');
    card.className = `notification-item ${!item.read ? 'is-unread' : ''}`.trim();
    card.href = item.link && item.link.trim() ? item.link : 'javascript:void(0)';
    card.dataset.id = item.id || '';
    card.setAttribute('role', 'article');
    card.setAttribute('aria-label', item.title || 'Notification');

    const header = document.createElement('div');
    header.className = 'notif-header';

    const badgeSpan = document.createElement('span');
    const safeType = (item.type || 'status').toLowerCase();
    badgeSpan.className = `notif-badge-type type-${safeType}`;
    badgeSpan.textContent = safeType.charAt(0).toUpperCase() + safeType.slice(1);
    header.appendChild(badgeSpan);

    const timeEl = document.createElement('time');
    timeEl.className = 'notif-time';
    if (item.createdAt) {
      timeEl.setAttribute('datetime', item.createdAt);
      timeEl.textContent = formatNotificationTime(item.createdAt);
    }
    header.appendChild(timeEl);
    card.appendChild(header);

    const titleEl = document.createElement('h3');
    titleEl.className = 'notif-title';
    titleEl.textContent = item.title || '';
    card.appendChild(titleEl);

    const msgEl = document.createElement('p');
    msgEl.className = 'notif-msg';
    msgEl.textContent = item.message || '';
    card.appendChild(msgEl);

    card.addEventListener('click', (e) => {
      if (card.classList.contains('is-unread')) {
        card.classList.remove('is-unread');
        const id = card.dataset.id;
        if (id) {
          markNotificationAsRead(id);
        }
      }
      if (card.getAttribute('href') === 'javascript:void(0)') {
        e.preventDefault();
      }
    });

    return card;
  }

  async function fetchNotifications() {
    const badge = document.getElementById('notification-badge');
    const countEl = document.getElementById('drawer-unread-count');
    const listEl = document.getElementById('notification-list');
    const emptyEl = document.getElementById('notification-empty');

    if (!listEl) return;

    try {
      const resp = await fetch('/api/v1/notifications', { credentials: 'same-origin', cache: 'no-store' });
      if (!resp.ok) {
        console.warn('Notifications request failed with status', resp.status);
        return;
      }

      const data = await resp.json();
      let notifications = data.notifications || [];

      // Merge local read states for guests
      let guestReadIds = new Set();
      try {
        const raw = localStorage.getItem('petspotr_read_notifications');
        if (raw) guestReadIds = new Set(JSON.parse(raw));
      } catch (_) {}

      notifications.forEach((item) => {
        if (guestReadIds.has(item.id)) {
          item.read = true;
        }
      });

      const unreadCount = notifications.filter((n) => !n.read).length;

      if (badge) {
        badge.textContent = String(unreadCount);
        if (unreadCount > 0) {
          badge.classList.remove('hidden');
        } else {
          badge.classList.add('hidden');
        }
      }

      if (countEl) {
        countEl.textContent = `${unreadCount} unread`;
      }

      if (notifications.length === 0) {
        if (emptyEl) emptyEl.classList.remove('hidden');
        listEl.classList.add('hidden');
        listEl.innerHTML = '';
        return;
      }

      if (emptyEl) emptyEl.classList.add('hidden');
      listEl.classList.remove('hidden');
      listEl.innerHTML = '';

      notifications.forEach((item) => {
        const card = createNotificationCard(item);
        listEl.appendChild(card);
      });
    } catch (err) {
      console.error('Failed to fetch notifications:', err);
    }
  }

  // --- Mark All As Read ---

  function initMarkAllRead() {
    const markAllBtn = document.getElementById('btn-mark-all-read');
    if (!markAllBtn) return;

    markAllBtn.addEventListener('click', async () => {
      try {
        const csrfToken = await getCSRFToken();
        const headers = { 'Content-Type': 'application/json' };
        if (csrfToken) headers['X-CSRF-Token'] = csrfToken;

        const resp = await fetch('/api/v1/notifications/mark-read', {
          method: 'POST',
          credentials: 'same-origin',
          headers,
          body: JSON.stringify({ all: true })
        });

        if (resp.ok) {
          document.querySelectorAll('#notification-list .notification-item.is-unread').forEach((card) => {
            card.classList.remove('is-unread');
          });

          const badge = document.getElementById('notification-badge');
          if (badge) {
            badge.textContent = '0';
            badge.classList.add('hidden');
          }

          const countEl = document.getElementById('drawer-unread-count');
          if (countEl) {
            countEl.textContent = '0 unread';
          }

          try {
            const allCards = document.querySelectorAll('#notification-list .notification-item');
            const ids = Array.from(allCards).map((c) => c.dataset.id).filter(Boolean);
            localStorage.setItem('petspotr_read_notifications', JSON.stringify(ids));
          } catch (_) {}
        }
      } catch (err) {
        console.error('Failed to mark all notifications read:', err);
      }
    });
  }

  // --- Leaflet Mini-Map Alert Zone Controller ---

  function updateMapCoordinates(lat, lng, radiusMiles) {
    const latInput = document.getElementById('pref-lat');
    const lngInput = document.getElementById('pref-lng');
    if (latInput) latInput.value = lat.toFixed(6);
    if (lngInput) lngInput.value = lng.toFixed(6);

    if (!miniMap || typeof L === 'undefined') return;

    const radiusMeters = (radiusMiles || 10) * 1609.34;

    if (miniMapCenterMarker) {
      miniMapCenterMarker.setLatLng([lat, lng]);
    } else {
      miniMapCenterMarker = L.circleMarker([lat, lng], {
        radius: 7,
        color: '#1d4ed8',
        fillColor: '#3b82f6',
        fillOpacity: 0.9,
        weight: 2
      }).addTo(miniMap);
    }

    if (miniMapRadiusCircle) {
      miniMapRadiusCircle.setLatLng([lat, lng]);
      miniMapRadiusCircle.setRadius(radiusMeters);
    } else {
      miniMapRadiusCircle = L.circle([lat, lng], {
        radius: radiusMeters,
        color: '#3b82f6',
        fillColor: '#3b82f6',
        fillOpacity: 0.15,
        weight: 2
      }).addTo(miniMap);
    }
  }

  function initMiniMap(initialLat, initialLng, initialRadiusMiles) {
    const mapElement = document.getElementById('zone-mini-map');
    if (!mapElement || typeof L === 'undefined') return;

    const lat = (typeof initialLat === 'number' && !isNaN(initialLat)) ? initialLat : 47.6062;
    const lng = (typeof initialLng === 'number' && !isNaN(initialLng)) ? initialLng : -122.3321;
    const radius = (typeof initialRadiusMiles === 'number' && !isNaN(initialRadiusMiles)) ? initialRadiusMiles : 10;

    if (!miniMap) {
      miniMap = L.map(mapElement, {
        center: [lat, lng],
        zoom: 11,
        zoomControl: true
      });

      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; OpenStreetMap contributors',
        maxZoom: 19
      }).addTo(miniMap);

      miniMap.on('click', (e) => {
        const clickedLat = e.latlng.lat;
        const clickedLng = e.latlng.lng;
        const radiusInput = document.getElementById('pref-radius');
        const currentRadius = radiusInput ? parseFloat(radiusInput.value) || 10 : 10;
        updateMapCoordinates(clickedLat, clickedLng, currentRadius);
      });
    }

    updateMapCoordinates(lat, lng, radius);
    miniMap.setView([lat, lng], miniMap.getZoom() || 11);
  }

  // --- Preferences Modal Controller ---

  function populatePreferencesForm(prefs) {
    const emailEnabled = document.getElementById('pref-email-enabled');
    const emailInput = document.getElementById('pref-email-address');
    const smsEnabled = document.getElementById('pref-sms-enabled');
    const phoneInput = document.getElementById('pref-phone-number');
    const pushEnabled = document.getElementById('pref-push-enabled');
    const geoZoneEnabled = document.getElementById('pref-geozone-enabled');
    const radiusSelect = document.getElementById('pref-radius');
    const latInput = document.getElementById('pref-lat');
    const lngInput = document.getElementById('pref-lng');

    if (emailEnabled) emailEnabled.checked = Boolean(prefs.emailEnabled);
    if (emailInput && prefs.email) emailInput.value = prefs.email;
    if (smsEnabled) smsEnabled.checked = Boolean(prefs.smsEnabled);
    if (phoneInput && prefs.phone) phoneInput.value = prefs.phone;
    if (pushEnabled) pushEnabled.checked = Boolean(prefs.pushEnabled);
    if (geoZoneEnabled) geoZoneEnabled.checked = prefs.geoZoneEnabled !== false;

    const radius = prefs.radiusMiles || 10;
    if (radiusSelect) {
      radiusSelect.value = String(Math.round(radius));
    }

    const lat = (prefs.coordinates && typeof prefs.coordinates.latitude === 'number')
      ? prefs.coordinates.latitude
      : 47.6062;
    const lng = (prefs.coordinates && typeof prefs.coordinates.longitude === 'number')
      ? prefs.coordinates.longitude
      : -122.3321;

    if (latInput) latInput.value = lat.toFixed(6);
    if (lngInput) lngInput.value = lng.toFixed(6);

    initMiniMap(lat, lng, radius);
  }

  async function loadPreferences() {
    let prefs = {
      emailEnabled: false,
      email: '',
      smsEnabled: false,
      phone: '',
      pushEnabled: false,
      geoZoneEnabled: true,
      coordinates: { latitude: 47.6062, longitude: -122.3321 },
      radiusMiles: 10.0
    };

    const pageLat = parseFloat(document.getElementById('filter-lat')?.value);
    const pageLng = parseFloat(document.getElementById('filter-lng')?.value);
    if (!isNaN(pageLat) && !isNaN(pageLng)) {
      prefs.coordinates = { latitude: pageLat, longitude: pageLng };
    }

    try {
      const resp = await fetch('/api/v1/notifications/preferences', { credentials: 'same-origin', cache: 'no-store' });
      if (resp.ok) {
        const serverPrefs = await resp.json();
        if (serverPrefs && typeof serverPrefs === 'object') {
          Object.assign(prefs, serverPrefs);
        }
      }
    } catch (err) {
      console.warn('Failed to load server alert preferences:', err);
    }

    try {
      const local = localStorage.getItem('petspotr_alert_preferences');
      if (local) {
        const localPrefs = JSON.parse(local);
        if (localPrefs && typeof localPrefs === 'object') {
          Object.assign(prefs, localPrefs);
        }
      }
    } catch (_) {}

    if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
      prefs.pushEnabled = true;
    }

    populatePreferencesForm(prefs);
  }

  function openPreferencesModal() {
    const modal = document.getElementById('notification-preferences-modal');
    const backdrop = document.getElementById('preferences-backdrop');
    if (!modal) return;

    modalLastFocused = document.activeElement;
    modal.classList.remove('hidden');
    if (backdrop) backdrop.classList.remove('hidden');

    loadPreferences();

    setTimeout(() => {
      if (miniMap) {
        miniMap.invalidateSize();
      }
    }, 100);

    const closeBtn = document.getElementById('btn-close-preferences');
    if (closeBtn) closeBtn.focus();
  }

  function closePreferencesModal() {
    const modal = document.getElementById('notification-preferences-modal');
    const backdrop = document.getElementById('preferences-backdrop');
    if (!modal) return;

    modal.classList.add('hidden');
    if (backdrop) backdrop.classList.add('hidden');

    if (modalLastFocused && typeof modalLastFocused.focus === 'function') {
      modalLastFocused.focus();
    }
  }

  function initPreferencesModal() {
    const btnOpenPref = document.getElementById('btn-open-preferences');
    const btnClosePref = document.getElementById('btn-close-preferences');
    const btnCancelPref = document.getElementById('btn-cancel-preferences');
    const backdrop = document.getElementById('preferences-backdrop');
    const modal = document.getElementById('notification-preferences-modal');
    const form = document.getElementById('preferences-form');
    const radiusSelect = document.getElementById('pref-radius');
    const geoBtn = document.getElementById('btn-pref-geolocation');
    const pushBtn = document.getElementById('btn-enable-push');
    const pushPrefCheckbox = document.getElementById('pref-push-enabled');

    if (!modal) return;

    if (btnOpenPref) btnOpenPref.addEventListener('click', openPreferencesModal);
    if (btnClosePref) btnClosePref.addEventListener('click', closePreferencesModal);
    if (btnCancelPref) btnCancelPref.addEventListener('click', closePreferencesModal);
    if (backdrop) backdrop.addEventListener('click', closePreferencesModal);

    modal.addEventListener('keydown', (e) => {
      trapFocus(modal, e);
    });

    if (radiusSelect) {
      radiusSelect.addEventListener('change', () => {
        const lat = parseFloat(document.getElementById('pref-lat')?.value) || 47.6062;
        const lng = parseFloat(document.getElementById('pref-lng')?.value) || -122.3321;
        const radiusMiles = parseFloat(radiusSelect.value) || 10;
        updateMapCoordinates(lat, lng, radiusMiles);
      });
    }

    if (geoBtn) {
      geoBtn.addEventListener('click', () => {
        if (!navigator.geolocation) {
          alert('Geolocation is not supported by your browser.');
          return;
        }

        const originalText = geoBtn.textContent;
        geoBtn.textContent = 'Locating...';
        geoBtn.disabled = true;

        navigator.geolocation.getCurrentPosition(
          (pos) => {
            geoBtn.textContent = originalText;
            geoBtn.disabled = false;
            const lat = pos.coords.latitude;
            const lng = pos.coords.longitude;
            const currentRadius = radiusSelect ? parseFloat(radiusSelect.value) || 10 : 10;
            updateMapCoordinates(lat, lng, currentRadius);
            if (miniMap) {
              miniMap.setView([lat, lng], 12);
            }
          },
          (err) => {
            geoBtn.textContent = originalText;
            geoBtn.disabled = false;
            console.warn('Geolocation query failed or denied:', err);
          },
          { timeout: 10000 }
        );
      });
    }

    // Connect Web Push interactions
    if (pushBtn && pushPrefCheckbox) {
      pushBtn.addEventListener('click', () => {
        setTimeout(() => {
          if (typeof Notification !== 'undefined' && Notification.permission === 'granted') {
            pushPrefCheckbox.checked = true;
          }
        }, 1000);
      });

      pushPrefCheckbox.addEventListener('change', () => {
        if (pushPrefCheckbox.checked && typeof Notification !== 'undefined') {
          if (Notification.permission !== 'granted') {
            pushBtn.click();
          }
        }
      });
    }

    // Form submission
    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();

        const emailEnabled = Boolean(document.getElementById('pref-email-enabled')?.checked);
        const email = (document.getElementById('pref-email-address')?.value || '').trim();
        const smsEnabled = Boolean(document.getElementById('pref-sms-enabled')?.checked);
        const phone = (document.getElementById('pref-phone-number')?.value || '').trim();
        const pushEnabled = Boolean(document.getElementById('pref-push-enabled')?.checked);
        const geoZoneEnabled = Boolean(document.getElementById('pref-geozone-enabled')?.checked);
        const radiusMiles = parseFloat(document.getElementById('pref-radius')?.value) || 10.0;
        let lat = parseFloat(document.getElementById('pref-lat')?.value);
        let lng = parseFloat(document.getElementById('pref-lng')?.value);

        if (isNaN(lat)) lat = 47.6062;
        if (isNaN(lng)) lng = -122.3321;

        if (emailEnabled && !email) {
          alert('Please enter an email address for email alerts.');
          return;
        }
        if (smsEnabled && !phone) {
          alert('Please enter a phone number for SMS alerts.');
          return;
        }
        if (smsEnabled && !/^\+[1-9]\d{1,14}$/.test(phone)) {
          alert('Phone number must be in valid E.164 format (e.g. +12065550199).');
          return;
        }

        const payload = {
          emailEnabled,
          email,
          smsEnabled,
          phone,
          pushEnabled,
          geoZoneEnabled,
          coordinates: {
            latitude: lat,
            longitude: lng
          },
          radiusMiles
        };

        const saveBtn = document.getElementById('btn-save-preferences');
        const originalText = saveBtn ? saveBtn.textContent : 'Save Preferences';
        if (saveBtn) {
          saveBtn.disabled = true;
          saveBtn.textContent = 'Saving...';
        }

        try {
          const csrfToken = await getCSRFToken();
          const headers = { 'Content-Type': 'application/json' };
          if (csrfToken) headers['X-CSRF-Token'] = csrfToken;

          const resp = await fetch('/api/v1/notifications/preferences', {
            method: 'PUT',
            credentials: 'same-origin',
            headers,
            body: JSON.stringify(payload)
          });

          if (!resp.ok) {
            const errText = await resp.text();
            throw new Error(errText || `Server returned ${resp.status}`);
          }

          const result = await resp.json();
          try {
            localStorage.setItem('petspotr_alert_preferences', JSON.stringify(result));
          } catch (_) {}

          if (saveBtn) {
            saveBtn.textContent = 'Saved! ✓';
          }

          setTimeout(() => {
            if (saveBtn) {
              saveBtn.textContent = originalText;
              saveBtn.disabled = false;
            }
            closePreferencesModal();
          }, 500);
        } catch (err) {
          console.error('Failed to save alert preferences:', err);
          alert(`Failed to save preferences: ${err.message}`);
          if (saveBtn) {
            saveBtn.textContent = originalText;
            saveBtn.disabled = false;
          }
        }
      });
    }
  }

  // --- Global Keyboard & Document Events ---

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      const modal = document.getElementById('notification-preferences-modal');
      if (modal && !modal.classList.contains('hidden')) {
        closePreferencesModal();
        return;
      }
      const drawer = document.getElementById('notification-drawer');
      if (drawer && drawer.classList.contains('is-open')) {
        closeNotificationDrawer();
      }
    }
  });

  // Initialization on DOMContentLoaded
  document.addEventListener('DOMContentLoaded', () => {
    initNotificationDrawer();
    fetchNotifications();
    initMarkAllRead();
    initPreferencesModal();
  });

  // Expose API on window for testability and progressive enhancement
  window.petspotrNotificationCenter = {
    fetchNotifications,
    openNotificationDrawer,
    closeNotificationDrawer,
    openPreferencesModal,
    closePreferencesModal,
    initMiniMap,
    updateMapCoordinates
  };
})();
