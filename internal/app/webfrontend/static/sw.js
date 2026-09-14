// PetSpotR Service Worker
// Provides layered offline caching, background sync event dispatching, and Web Push notifications.

const SHELL_CACHE = 'petspotr-shell-v1';
const DATA_CACHE = 'petspotr-data-v1';
const TILE_CACHE = 'petspotr-tiles-v1';
const MAX_TILE_CACHE = 250;

const PRECACHE_ASSETS = [
  '/',
  '/offline.html',
  '/manifest.webmanifest',
  '/static/css/styles.css',
  '/static/favicon.svg',
  '/static/js/outbox-sync.js',
  '/static/js/lost-report.js',
  '/static/js/found-report.js',
  '/static/js/pet-directory.js',
  '/static/js/match-dashboard.js',
  '/static/js/push-notifier.js',
  '/static/vendor/leaflet/leaflet.js',
  '/static/vendor/leaflet/leaflet.css',
  '/static/vendor/leaflet/images/marker-icon.png',
  '/static/vendor/leaflet/images/marker-shadow.png'
];

// Precache core assets during install and activate immediately
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(SHELL_CACHE)
      .then((cache) => cache.addAll(PRECACHE_ASSETS))
      .then(() => self.skipWaiting())
  );
});

// Clean up stale cache versions on activation and claim clients
self.addEventListener('activate', (event) => {
  const currentCaches = [SHELL_CACHE, DATA_CACHE, TILE_CACHE];
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames.map((cacheName) => {
          if (!currentCaches.includes(cacheName)) {
            return caches.delete(cacheName);
          }
        })
      );
    }).then(() => self.clients.claim())
  );
});

// LRU tile cache pruning helper
async function pruneTileCache(cache, maxEntries) {
  const keys = await cache.keys();
  if (keys.length > maxEntries) {
    for (let i = 0; i < keys.length - maxEntries; i++) {
      await cache.delete(keys[i]);
    }
  }
}

// Fetch routing and caching strategies
self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') {
    return;
  }

  const url = new URL(event.request.url);

  // 1. OSM Raster Map Tiles (*.tile.openstreetmap.org): Cache-First
  if (url.hostname.includes('tile.openstreetmap.org')) {
    event.respondWith(
      caches.open(TILE_CACHE).then(async (cache) => {
        const cachedResponse = await cache.match(event.request);
        if (cachedResponse) {
          return cachedResponse;
        }
        try {
          const networkResponse = await fetch(event.request);
          if (networkResponse && (networkResponse.status === 200 || networkResponse.type === 'opaque')) {
            cache.put(event.request, networkResponse.clone());
            pruneTileCache(cache, MAX_TILE_CACHE).catch(() => {});
          }
          return networkResponse;
        } catch (err) {
          throw err;
        }
      })
    );
    return;
  }

  // 4. HTML Navigations (request.mode === 'navigate'): Network-First with fallback to cached shell or /offline.html
  if (event.request.mode === 'navigate') {
    event.respondWith((async () => {
      try {
        const networkResponse = await fetch(event.request);
        if (networkResponse && networkResponse.status === 200) {
          const cache = await caches.open(SHELL_CACHE);
          cache.put(event.request, networkResponse.clone());
        }
        return networkResponse;
      } catch (err) {
        const cachedResponse = (await caches.match(event.request)) || (await caches.match(event.request, { ignoreSearch: true }));
        if (cachedResponse) {
          return cachedResponse;
        }
        const offlinePage = await caches.match('/offline.html');
        if (offlinePage) {
          return offlinePage;
        }
        throw err;
      }
    })());
    return;
  }

  // 2. Directory API (/api/v1/lost-pets, /api/v1/found-pets): Network-First with 3s timeout and fallback to DATA_CACHE
  if (url.pathname.startsWith('/api/v1/lost-pets') || url.pathname.startsWith('/api/v1/found-pets')) {
    event.respondWith((async () => {
      const cache = await caches.open(DATA_CACHE);
      let timeoutId;
      const timeoutPromise = new Promise((_, reject) => {
        timeoutId = setTimeout(() => reject(new Error('Network timeout')), 3000);
      });

      try {
        const networkResponse = await Promise.race([fetch(event.request), timeoutPromise]);
        clearTimeout(timeoutId);
        if (networkResponse && networkResponse.ok) {
          cache.put(event.request, networkResponse.clone());
          return networkResponse;
        }
        if (networkResponse && networkResponse.status >= 500) {
          const cachedResponse = await cache.match(event.request);
          if (cachedResponse) {
            const blob = await cachedResponse.blob();
            const headers = new Headers(cachedResponse.headers);
            headers.set('X-PetSpotR-Offline', 'true');
            return new Response(blob, {
              status: cachedResponse.status,
              statusText: cachedResponse.statusText,
              headers: headers
            });
          }
        }
        return networkResponse;
      } catch (err) {
        clearTimeout(timeoutId);
        const cachedResponse = await cache.match(event.request);
        if (cachedResponse) {
          const blob = await cachedResponse.blob();
          const headers = new Headers(cachedResponse.headers);
          headers.set('X-PetSpotR-Offline', 'true');
          return new Response(blob, {
            status: cachedResponse.status,
            statusText: cachedResponse.statusText,
            headers: headers
          });
        }
        throw err;
      }
    })());
    return;
  }

  // 3. Static Assets (/static/*, /manifest.webmanifest, /offline.html): Stale-While-Revalidate
  if (url.pathname.startsWith('/static/') || url.pathname === '/manifest.webmanifest' || url.pathname === '/offline.html') {
    event.respondWith((async () => {
      const cache = await caches.open(SHELL_CACHE);
      const cachedResponse = await cache.match(event.request);
      const fetchPromise = fetch(event.request).then((networkResponse) => {
        if (networkResponse && networkResponse.status === 200) {
          cache.put(event.request, networkResponse.clone());
        }
        return networkResponse;
      });

      if (cachedResponse) {
        fetchPromise.catch(() => {});
        return cachedResponse;
      }

      return fetchPromise;
    })());
    return;
  }
});

// Background sync listener for outbox queue replay
self.addEventListener('sync', (event) => {
  if (event.tag === 'petspotr-outbox-sync') {
    event.waitUntil(notifyClientsToSync());
  }
});

async function notifyClientsToSync() {
  const clientList = await clients.matchAll({ type: 'window', includeUncontrolled: true });
  if (clientList.length > 0) {
    for (const client of clientList) {
      client.postMessage({ type: 'PETSPOTR_TRIGGER_SYNC' });
    }
  }
}

// Handle incoming Web Push notifications
self.addEventListener('push', (event) => {
  let payload = {
    title: 'PetSpotR Alert 🐾',
    body: 'A high-confidence match has been identified!',
    icon: '/static/img/icon-192.png',
    url: '/matches'
  };

  if (event.data) {
    try {
      const data = event.data.json();
      payload = { ...payload, ...data };
    } catch (err) {
      payload.body = event.data.text();
    }
  }

  const options = {
    body: payload.body,
    icon: payload.icon || '/static/img/icon-192.png',
    badge: '/static/img/badge.png',
    vibrate: [100, 50, 100],
    data: {
      url: payload.url || '/matches'
    },
    actions: [
      { action: 'view', title: 'View Match Comparison' }
    ]
  };

  event.waitUntil(
    self.registration.showNotification(payload.title, options)
  );
});

// Handle notification click event
self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const targetUrl = (event.notification.data && event.notification.data.url) || '/matches';

  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windowClients) => {
      for (let client of windowClients) {
        if (client.url.includes(targetUrl) && 'focus' in client) {
          return client.focus();
        }
      }
      if (clients.openWindow) {
        return clients.openWindow(targetUrl);
      }
    })
  );
});
