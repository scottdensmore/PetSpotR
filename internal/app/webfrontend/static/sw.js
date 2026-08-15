// PetSpotR Web Push Service Worker
self.addEventListener('install', (event) => {
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(self.clients.claim());
});

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
