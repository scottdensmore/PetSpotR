// PetSpotR Client Outbox Engine
// Implements IndexedDB storage ('petspotr_offline_db') for offline report drafts
// and an orchestrated background synchronization pipeline.

(function () {
  'use strict';

  const DB_NAME = 'petspotr_offline_db';
  const DB_VERSION = 1;
  const STORE_NAME = 'outbox_reports';
  const BEACON_STORE_NAME = 'petspotr_beacon_outbox';

  let dbInstance = null;
  let openPromise = null;
  let isSyncing = false;

  /**
   * Safe access to IndexedDB factory across environments.
   */
  function getIndexedDB() {
    if (typeof indexedDB !== 'undefined') {
      return indexedDB;
    }
    if (typeof window !== 'undefined' && window.indexedDB) {
      return window.indexedDB;
    }
    if (typeof globalThis !== 'undefined' && globalThis.indexedDB) {
      return globalThis.indexedDB;
    }
    return null;
  }

  /**
   * Generates a RFC4122 v4 UUID with fallback.
   */
  function generateUUID() {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
      const r = (Math.random() * 16) | 0;
      const v = c === 'x' ? r : (r & 0x3) | 0x8;
      return v.toString(16);
    });
  }

  /**
   * Retrieves CSRF token from identity session state or cookie.
   */
  function getCsrfToken() {
    if (typeof window !== 'undefined' && window.petspotrIdentity?.getState) {
      const state = window.petspotrIdentity.getState();
      if (state?.csrfToken) return state.csrfToken;
    }
    if (typeof document !== 'undefined' && document.cookie) {
      const match = document.cookie.match(/(?:^|;\s*)(?:__Host-)?petspotr_csrf=([^;]+)/);
      if (match) return decodeURIComponent(match[1]);
    }
    return '';
  }

  /**
   * Resets any stranded 'syncing' records back to 'pending'.
   * Recovers drafts stranded by tab closure, page refresh, or crashes during sync.
   */
  function recoverStrandedRecords(db) {
    if (!db || !db.objectStoreNames.contains(STORE_NAME)) {
      return Promise.resolve();
    }
    return new Promise((resolve) => {
      try {
        const tx = db.transaction(STORE_NAME, 'readwrite');
        const store = tx.objectStore(STORE_NAME);
        const req = store.getAll();
        req.onsuccess = function () {
          const records = req.result || [];
          const syncing = records.filter((r) => r.status === 'syncing');
          if (syncing.length === 0) {
            return resolve();
          }
          let remaining = syncing.length;
          for (const record of syncing) {
            record.status = 'pending';
            const putReq = store.put(record);
            putReq.onsuccess = function () {
              remaining--;
              if (remaining === 0) resolve();
            };
            putReq.onerror = function () {
              remaining--;
              if (remaining === 0) resolve();
            };
          }
        };
        req.onerror = function () {
          resolve();
        };
        tx.oncomplete = function () {
          resolve();
        };
        tx.onerror = function () {
          resolve();
        };
      } catch (err) {
        resolve();
      }
    });
  }

  /**
   * Opens or upgrades the petspotr_offline_db database.
   * Creates outbox_reports object store with 'id' keyPath,
   * and 'by_status' and 'by_created' indexes.
   * Caches in-flight openPromise to prevent concurrent opening races.
   */
  function openDB() {
    if (dbInstance) {
      return Promise.resolve(dbInstance);
    }
    if (openPromise) {
      return openPromise;
    }

    const idb = getIndexedDB();
    if (!idb) {
      return Promise.reject(new Error('IndexedDB is not supported in this environment'));
    }

    openPromise = new Promise((resolve, reject) => {
      const request = idb.open(DB_NAME, DB_VERSION);

      request.onupgradeneeded = function (event) {
        const db = event.target.result;
        if (!db.objectStoreNames.contains(STORE_NAME)) {
          const store = db.createObjectStore(STORE_NAME, { keyPath: 'id' });
          store.createIndex('by_status', 'status', { unique: false });
          store.createIndex('by_created', 'createdAt', { unique: false });
        }
        if (!db.objectStoreNames.contains(BEACON_STORE_NAME)) {
          const beaconStore = db.createObjectStore(BEACON_STORE_NAME, { keyPath: 'id' });
          beaconStore.createIndex('by_pet', 'petId', { unique: false });
          beaconStore.createIndex('by_created', 'createdAt', { unique: false });
        }
      };

      request.onsuccess = async function () {
        dbInstance = request.result;
        dbInstance.onclose = function () {
          dbInstance = null;
          openPromise = null;
        };
        dbInstance.onerror = function (err) {
          console.warn('IndexedDB connection error:', err);
        };

        // If existing database was created without BEACON_STORE_NAME, dynamically upgrade
        if (!dbInstance.objectStoreNames.contains(BEACON_STORE_NAME)) {
          const currentVersion = dbInstance.version;
          dbInstance.close();
          dbInstance = null;
          const upgradeReq = idb.open(DB_NAME, currentVersion + 1);
          upgradeReq.onupgradeneeded = function (e) {
            const upDb = e.target.result;
            if (!upDb.objectStoreNames.contains(BEACON_STORE_NAME)) {
              const bStore = upDb.createObjectStore(BEACON_STORE_NAME, { keyPath: 'id' });
              bStore.createIndex('by_pet', 'petId', { unique: false });
              bStore.createIndex('by_created', 'createdAt', { unique: false });
            }
          };
          upgradeReq.onsuccess = async function () {
            dbInstance = upgradeReq.result;
            try {
              await recoverStrandedRecords(dbInstance);
            } catch (e) {
              console.warn('Failed to recover stranded outbox records:', e);
            }
            resolve(dbInstance);
          };
          upgradeReq.onerror = function () {
            reject(upgradeReq.error);
          };
          return;
        }

        try {
          await recoverStrandedRecords(dbInstance);
        } catch (e) {
          console.warn('Failed to recover stranded outbox records:', e);
        }

        resolve(dbInstance);
      };

      request.onerror = function () {
        dbInstance = null;
        openPromise = null;
        reject(request.error);
      };
    });

    return openPromise;
  }

  /**
   * Requests background sync registration via Service Worker SyncManager.
   */
  function registerSync() {
    if (
      typeof navigator !== 'undefined' &&
      'serviceWorker' in navigator &&
      typeof window !== 'undefined' &&
      'SyncManager' in window
    ) {
      return navigator.serviceWorker.ready
        .then((reg) => reg.sync.register('petspotr-outbox-sync'))
        .catch(() => {});
    }
    return Promise.resolve();
  }

  /**
   * Enqueues a new report into IndexedDB outbox_reports.
   *
   * @param {Object} options
   * @param {'lost'|'found'} options.type
   * @param {Object} options.payload
   * @param {Array<{fileName: string, contentType: string, tag: string, blob: Blob}>} options.photos
   * @returns {Promise<string>} The generated report ID.
   */
  async function enqueueReport({ type = 'lost', payload = {}, photos = [] } = {}) {
    const id = generateUUID();
    const normalizedPhotos = (Array.isArray(photos) ? photos : []).map((photo, index) => {
      const isBlob =
        (typeof Blob !== 'undefined' && photo instanceof Blob) ||
        (photo && typeof photo.slice === 'function' && typeof photo.size === 'number');

      if (isBlob) {
        return {
          fileName: photo.name || `photo-${Date.now()}-${index + 1}.jpg`,
          contentType: photo.type || 'image/jpeg',
          tag: index === 0 ? 'primary' : 'face',
          blob: photo
        };
      }

      return {
        fileName:
          photo.fileName ||
          (photo.blob && photo.blob.name) ||
          `photo-${Date.now()}-${index + 1}.jpg`,
        contentType:
          photo.contentType ||
          (photo.blob && photo.blob.type) ||
          'image/jpeg',
        tag: photo.tag || (index === 0 ? 'primary' : 'face'),
        blob: photo.blob || photo
      };
    });

    const record = {
      id,
      type,
      payload: payload || {},
      photos: normalizedPhotos,
      status: 'pending',
      attempts: 0,
      createdAt: new Date().toISOString()
    };

    const db = await openDB();
    await new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readwrite');
      const store = tx.objectStore(STORE_NAME);
      const req = store.add(record);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });

    registerSync();

    const count = await getQueueCount();
    if (typeof window !== 'undefined') {
      window.dispatchEvent(
        new CustomEvent('petspotr:outbox-updated', {
          detail: { id, count }
        })
      );
    }

    await updateUI();
    return id;
  }

  /**
   * Returns an array of reports with status === 'pending'.
   * @returns {Promise<Array<Object>>}
   */
  async function getPendingReports() {
    const db = await openDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readonly');
      const store = tx.objectStore(STORE_NAME);
      if (store.indexNames.contains('by_status')) {
        const index = store.index('by_status');
        const req = index.getAll('pending');
        req.onsuccess = () => resolve(req.result || []);
        req.onerror = () => reject(req.error);
      } else {
        const req = store.getAll();
        req.onsuccess = () => {
          const records = (req.result || []).filter((r) => r.status === 'pending');
          resolve(records);
        };
        req.onerror = () => reject(req.error);
      }
    });
  }

  /**
   * Returns count of active reports in outbox_reports (pending or syncing).
   * @returns {Promise<number>}
   */
  async function getQueueCount() {
    try {
      const db = await openDB();
      return new Promise((resolve, reject) => {
        const tx = db.transaction(STORE_NAME, 'readonly');
        const store = tx.objectStore(STORE_NAME);
        const req = store.getAll();
        req.onsuccess = () => {
          const records = req.result || [];
          const count = records.filter(
            (r) => r.status === 'pending' || r.status === 'syncing'
          ).length;
          resolve(count);
        };
        req.onerror = () => reject(req.error);
      });
    } catch (err) {
      console.warn('Failed to retrieve queue count:', err);
      return 0;
    }
  }

  /**
   * Returns all reports in outbox_reports.
   * @returns {Promise<Array<Object>>}
   */
  async function getAllReports() {
    const db = await openDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readonly');
      const store = tx.objectStore(STORE_NAME);
      const req = store.getAll();
      req.onsuccess = () => resolve(req.result || []);
      req.onerror = () => reject(req.error);
    });
  }

  /**
   * Updates an existing record in outbox_reports.
   */
  async function updateRecord(record) {
    const db = await openDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readwrite');
      const store = tx.objectStore(STORE_NAME);
      const req = store.put(record);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  }

  /**
   * Deletes a record by ID from outbox_reports.
   */
  async function deleteRecord(id) {
    const db = await openDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readwrite');
      const store = tx.objectStore(STORE_NAME);
      const req = store.delete(id);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  }

  /**
   * Clears all records from outbox_reports.
   */
  async function clearOutbox() {
    const db = await openDB();
    return new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readwrite');
      const store = tx.objectStore(STORE_NAME);
      const req = store.clear();
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  }

  /**
   * Displays an accessible toast notification in #toast-container.
   * Automatically creates the container if not present and removes toast after 4s.
   *
   * @param {string} message
   * @param {'info'|'success'|'error'|'warning'} type
   * @returns {HTMLElement|null}
   */
  function showToast(message, type = 'info') {
    if (typeof document === 'undefined') return null;

    let container = document.getElementById('toast-container');
    if (!container) {
      container = document.createElement('div');
      container.id = 'toast-container';
      container.className = 'toast-container';
      container.setAttribute('role', 'region');
      container.setAttribute('aria-live', 'polite');
      container.setAttribute('aria-label', 'Notifications');
      if (document.body) {
        document.body.appendChild(container);
      }
    }

    const toast = document.createElement('div');
    toast.className = `toast-item toast-${type}`;
    toast.setAttribute('role', 'status');
    toast.textContent = message;

    if (container) {
      container.appendChild(toast);
    }

    setTimeout(() => {
      if (toast.parentNode) {
        toast.parentNode.removeChild(toast);
      }
    }, 4000);

    return toast;
  }

  /**
   * Synchronizes UI status indicators (#outbox-count-badge, #offline-indicator, #offline-status-text).
   */
  async function updateUI() {
    if (typeof document === 'undefined') return;

    const count = await getQueueCount();
    const indicator = document.getElementById('offline-indicator');
    const statusText = document.getElementById('offline-status-text');
    const countBadge = document.getElementById('outbox-count-badge');
    const dirNotice = document.getElementById('offline-directory-notice');

    if (dirNotice) {
      const isOnline = typeof navigator !== 'undefined' ? navigator.onLine : true;
      dirNotice.hidden = isOnline;
    }

    if (countBadge) {
      if (count > 0) {
        countBadge.textContent = String(count);
        countBadge.hidden = false;
      } else {
        countBadge.textContent = '0';
        countBadge.hidden = true;
      }
    }

    if (indicator) {
      const isOnline = typeof navigator !== 'undefined' ? navigator.onLine : true;
      if (!isOnline) {
        indicator.hidden = false;
        indicator.classList.add('is-offline');
        indicator.classList.remove('is-syncing', 'is-pending');
        if (statusText) {
          statusText.textContent = 'Offline';
        }
      } else if (isSyncing) {
        indicator.hidden = false;
        indicator.classList.add('is-syncing');
        indicator.classList.remove('is-offline', 'is-pending');
        if (statusText) {
          statusText.textContent =
            count > 0
              ? `Syncing ${count} report${count === 1 ? '' : 's'}...`
              : 'Syncing...';
        }
      } else if (count > 0) {
        indicator.hidden = false;
        indicator.classList.add('is-pending');
        indicator.classList.remove('is-offline', 'is-syncing');
        if (statusText) {
          statusText.textContent = 'Pending Sync';
        }
      } else {
        indicator.hidden = true;
        indicator.classList.remove('is-offline', 'is-syncing', 'is-pending');
        if (statusText) {
          statusText.textContent = 'Online';
        }
      }
    }
  }

  /**
   * Synchronizes all pending reports to backend APIs.
   * Prevents concurrent sync runs via isSyncing guard.
   *
   * @returns {Promise<{synced: number, failed: number}>}
   */
  async function syncAll() {
    if (isSyncing) {
      return { synced: 0, failed: 0 };
    }
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      return { synced: 0, failed: 0 };
    }

    isSyncing = true;
    let synced = 0;
    let failed = 0;

    try {
      const db = await openDB();
      await recoverStrandedRecords(db);

      const pendingReports = await getPendingReports();
      if (!pendingReports || pendingReports.length === 0) {
        return { synced: 0, failed: 0 };
      }

      if (typeof window !== 'undefined') {
        window.dispatchEvent(
          new CustomEvent('petspotr:sync-started', {
            detail: { count: pendingReports.length }
          })
        );
      }

      await updateUI();

      for (const record of pendingReports) {
        record.status = 'syncing';
        await updateRecord(record);
        await updateUI();

        try {
          const finalizedImages = [];
          const photos = Array.isArray(record.photos) ? record.photos : [];
          let seenPrimary = false;

          for (let i = 0; i < photos.length; i++) {
            const photo = photos[i];
            const csrfToken = getCsrfToken();
            const presignedHeaders = { 'Content-Type': 'application/json' };
            if (csrfToken) {
              presignedHeaders['X-CSRF-Token'] = csrfToken;
            }

            const presignedBody = {
              purpose: record.type === 'lost' ? 'lost-pet' : 'found-pet',
              fileName: photo.fileName || `photo-${Date.now()}.jpg`,
              contentType:
                photo.contentType ||
                (photo.blob ? photo.blob.type : 'image/jpeg') ||
                'image/jpeg'
            };

            const presignedRes = await fetch('/api/v1/uploads/presigned-url', {
              method: 'POST',
              headers: presignedHeaders,
              body: JSON.stringify(presignedBody)
            });

            if (!presignedRes.ok) {
              throw new Error(`Presigned URL request failed: HTTP ${presignedRes.status}`);
            }

            const presigned = await presignedRes.json();
            if (presigned.uploadUrl && photo.blob) {
              const uploadRes = await fetch(presigned.uploadUrl, {
                method: 'PUT',
                headers: {
                  'Content-Type':
                    photo.contentType ||
                    (photo.blob ? photo.blob.type : 'image/jpeg') ||
                    'image/jpeg'
                },
                body: photo.blob
              });
              if (!uploadRes.ok) {
                throw new Error(`Direct photo upload failed: HTTP ${uploadRes.status}`);
              }
            }

            const objectName =
              presigned.fileName ||
              presigned.objectName ||
              `images/${record.type === 'lost' ? 'lost-pets' : 'found-pets'}/${photo.fileName || 'photo.jpg'}`;

            let tag = photo.tag;
            if (tag === 'primary') {
              if (seenPrimary) {
                tag = 'face';
              } else {
                seenPrimary = true;
              }
            } else if (!tag) {
              if (!seenPrimary && i === 0) {
                tag = 'primary';
                seenPrimary = true;
              } else {
                tag = 'face';
              }
            }

            finalizedImages.push({
              object: objectName,
              tag: tag || 'face',
              url: presigned.publicUrl || ''
            });
          }

          // Ensure exactly one primary tag if any images exist
          if (finalizedImages.length > 0 && !finalizedImages.some((img) => img.tag === 'primary')) {
            finalizedImages[0].tag = 'primary';
          }

          const primaryImg =
            finalizedImages.find((img) => img.tag === 'primary' || img.tag === 'face') ||
            finalizedImages[0];
          const primaryObject = primaryImg
            ? primaryImg.object
            : record.payload?.imageObject || '';

          const reportPayload = {
            ...record.payload,
            images:
              finalizedImages.length > 0
                ? finalizedImages.map((img) => ({ object: img.object, tag: img.tag }))
                : record.payload?.images || [],
            imageObject: primaryObject
          };

          if (record.type === 'found' && !reportPayload.imageUrl) {
            reportPayload.imageUrl =
              primaryImg && primaryImg.url
                ? primaryImg.url
                : primaryObject
                ? `https://storage.petspotr.io/${primaryObject}`
                : 'https://storage.petspotr.io/found-sample.jpg';
          }

          const endpoint =
            record.type === 'lost' ? '/api/v1/lost-pets' : '/api/v1/found-pets';
          const reportHeaders = { 'Content-Type': 'application/json' };
          const csrfToken = getCsrfToken();
          if (csrfToken) {
            reportHeaders['X-CSRF-Token'] = csrfToken;
          }

          const reportRes = await fetch(endpoint, {
            method: 'POST',
            headers: reportHeaders,
            body: JSON.stringify(reportPayload)
          });

          if (reportRes.ok) {
            await deleteRecord(record.id);
            synced++;
            if (typeof window !== 'undefined') {
              window.dispatchEvent(
                new CustomEvent('petspotr:report-synced', {
                  detail: { id: record.id, type: record.type, payload: reportPayload }
                })
              );
            }
            const toastPetName = record.payload?.petName || 'found pet';
            showToast(
              `Report for ${toastPetName} synchronized successfully.`,
              'success'
            );
          } else {
            const errText = await reportRes.text().catch(() => '');
            throw new Error(`Report submission failed: HTTP ${reportRes.status} ${errText}`);
          }
        } catch (err) {
          failed++;
          record.attempts = (record.attempts || 0) + 1;
          record.lastError = err.message || String(err);
          record.status = record.attempts >= 5 ? 'failed' : 'pending';
          await updateRecord(record);
          console.warn(
            `Sync failed for report ${record.id} (attempt ${record.attempts}):`,
            err
          );
        }
      }

      return { synced, failed };
    } finally {
      isSyncing = false;
      await updateUI();
      const remainingCount = await getQueueCount();
      if (typeof window !== 'undefined') {
        window.dispatchEvent(
          new CustomEvent('petspotr:outbox-updated', {
            detail: { count: remainingCount, synced, failed }
          })
        );
      }
    }
  }

  let isFlushingBeaconPings = false;

  /**
   * Enqueues a collar beacon ping into IndexedDB petspotr_beacon_outbox.
   *
   * @param {string} petId
   * @param {Object} pingData
   * @returns {Promise<string>} The generated ping record ID.
   */
  async function queueBeaconPing(petId, pingData = {}) {
    const id = generateUUID();
    const record = {
      id,
      petId,
      payload: pingData,
      status: 'pending',
      attempts: 0,
      createdAt: new Date().toISOString()
    };

    const db = await openDB();
    await new Promise((resolve, reject) => {
      const tx = db.transaction(BEACON_STORE_NAME, 'readwrite');
      const store = tx.objectStore(BEACON_STORE_NAME);
      const req = store.add(record);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });

    if (typeof window !== 'undefined') {
      window.dispatchEvent(
        new CustomEvent('petspotr:beacon-ping-queued', {
          detail: { id, petId, record }
        })
      );
    }
    return id;
  }

  /**
   * Returns all queued beacon pings from petspotr_beacon_outbox.
   * @returns {Promise<Array<Object>>}
   */
  async function getQueuedBeaconPings() {
    const db = await openDB();
    if (!db.objectStoreNames.contains(BEACON_STORE_NAME)) {
      return [];
    }
    return new Promise((resolve, reject) => {
      const tx = db.transaction(BEACON_STORE_NAME, 'readonly');
      const store = tx.objectStore(BEACON_STORE_NAME);
      const req = store.getAll();
      req.onsuccess = () => resolve(req.result || []);
      req.onerror = () => reject(req.error);
    });
  }

  /**
   * Deletes a beacon ping record from petspotr_beacon_outbox by ID.
   * @param {string} id
   * @returns {Promise<void>}
   */
  async function deleteBeaconPing(id) {
    const db = await openDB();
    if (!db.objectStoreNames.contains(BEACON_STORE_NAME)) {
      return;
    }
    return new Promise((resolve, reject) => {
      const tx = db.transaction(BEACON_STORE_NAME, 'readwrite');
      const store = tx.objectStore(BEACON_STORE_NAME);
      const req = store.delete(id);
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
  }

  /**
   * Flushes all queued beacon pings to POST /api/v1/search-parties/{petId}/beacon-pings.
   * @returns {Promise<{synced: number, failed: number}>}
   */
  async function flushBeaconPings() {
    if (isFlushingBeaconPings) {
      return { synced: 0, failed: 0 };
    }
    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      return { synced: 0, failed: 0 };
    }

    isFlushingBeaconPings = true;
    let synced = 0;
    let failed = 0;

    try {
      const pings = await getQueuedBeaconPings();
      if (!pings || pings.length === 0) {
        return { synced: 0, failed: 0 };
      }

      const csrfToken = getCsrfToken();

      for (const record of pings) {
        try {
          const headers = { 'Content-Type': 'application/json' };
          if (csrfToken) {
            headers['X-CSRF-Token'] = csrfToken;
          }

          const endpoint = `/api/v1/search-parties/${encodeURIComponent(record.petId)}/beacon-pings`;
          const res = await fetch(endpoint, {
            method: 'POST',
            headers,
            body: JSON.stringify(record.payload)
          });

          if (res.ok) {
            await deleteBeaconPing(record.id);
            synced++;
            if (typeof window !== 'undefined') {
              window.dispatchEvent(
                new CustomEvent('petspotr:beacon-ping-synced', {
                  detail: { id: record.id, petId: record.petId }
                })
              );
            }
          } else if (res.status >= 400 && res.status < 500) {
            // Drop client rejection to prevent queue clog
            await deleteBeaconPing(record.id);
          } else {
            failed++;
            break; // Stop on 5xx or server/network error
          }
        } catch (err) {
          failed++;
          console.warn('Failed to flush beacon ping:', err);
          break;
        }
      }

      if (synced > 0) {
        showToast(`Synchronized ${synced} collar beacon ping(s).`, 'success');
        if (typeof window !== 'undefined') {
          window.dispatchEvent(
            new CustomEvent('petspotr:beacon-pings-flushed', {
              detail: { synced, failed }
            })
          );
        }
      }

      return { synced, failed };
    } finally {
      isFlushingBeaconPings = false;
    }
  }

  // Define public module API
  const PetSpotROutbox = {
    enqueueReport,
    getPendingReports,
    getQueueCount,
    getAllReports,
    updateRecord,
    deleteReport: deleteRecord,
    clearOutbox,
    syncAll,
    showToast,
    updateUI,
    registerSync,
    openDB,
    queueBeaconPing,
    getQueuedBeaconPings,
    deleteBeaconPing,
    flushBeaconPings,
    BEACON_STORE_NAME
  };

  // Bind to global scope
  if (typeof window !== 'undefined') {
    window.PetSpotROutbox = PetSpotROutbox;
    window.OutboxSync = PetSpotROutbox;

    window.addEventListener('online', () => {
      PetSpotROutbox.syncAll();
      PetSpotROutbox.flushBeaconPings();
    });

    window.addEventListener('offline', () => {
      PetSpotROutbox.updateUI();
    });

    if (typeof navigator !== 'undefined' && 'serviceWorker' in navigator && typeof window !== 'undefined') {
      if (typeof document !== 'undefined' && document.readyState === 'complete') {
        navigator.serviceWorker.register('/sw.js').catch(() => {});
      } else {
        window.addEventListener('load', () => {
          navigator.serviceWorker.register('/sw.js').catch(() => {});
        });
      }
    }

    if (typeof navigator !== 'undefined' && 'serviceWorker' in navigator) {
      navigator.serviceWorker.addEventListener('message', (event) => {
        if (event.data?.type === 'PETSPOTR_TRIGGER_SYNC') {
          PetSpotROutbox.syncAll();
        }
      });
    }

    if (typeof document !== 'undefined') {
      const onReady = () => {
        PetSpotROutbox.updateUI();
        if (typeof navigator !== 'undefined' && navigator.onLine) {
          PetSpotROutbox.getPendingReports()
            .then((pending) => {
              if (pending && pending.length > 0) {
                PetSpotROutbox.syncAll();
              }
            })
            .catch(() => {});
          PetSpotROutbox.getQueuedBeaconPings()
            .then((queued) => {
              if (queued && queued.length > 0) {
                PetSpotROutbox.flushBeaconPings();
              }
            })
            .catch(() => {});
        }
      };

      if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', onReady);
      } else {
        onReady();
      }
    }
  }

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = PetSpotROutbox;
  }
})();
