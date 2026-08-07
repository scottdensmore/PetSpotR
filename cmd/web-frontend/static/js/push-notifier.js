// Client-side controller for Service Worker registration and Web Push Subscriptions
document.addEventListener('DOMContentLoaded', async () => {
  if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
    console.warn('Web Push is not supported in this browser environment.');
    return;
  }

  try {
    const reg = await navigator.serviceWorker.register('/sw.js');
    console.log('PetSpotR Service Worker registered successfully:', reg.scope);
  } catch (err) {
    console.error('Service Worker registration failed:', err);
  }

  const pushBtn = document.getElementById('btn-enable-push');
  if (pushBtn) {
    updatePushButtonState(pushBtn);

    pushBtn.addEventListener('click', async () => {
      await requestPushSubscription(pushBtn);
    });
  }
});

function updatePushButtonState(btn) {
  if (Notification.permission === 'granted') {
    btn.textContent = '🔔 Push Alerts Active';
    btn.classList.add('btn-secondary');
    btn.disabled = true;
  } else if (Notification.permission === 'denied') {
    btn.textContent = '🔕 Push Alerts Blocked';
    btn.disabled = true;
  } else {
    btn.textContent = '🔔 Enable Instant Push Alerts';
  }
}

async function requestPushSubscription(btn) {
  try {
    const permission = await Notification.requestPermission();
    if (permission !== 'granted') {
      alert('Notification permission was denied.');
      updatePushButtonState(btn);
      return;
    }

    const reg = await navigator.serviceWorker.ready;
    let sub = await reg.pushManager.getSubscription();

    if (!sub) {
      // Mock VAPID Application Server Key
      const applicationServerKey = 'BEl62iUYgUivxIkv69yViEuiBIa-Ib9-Skv69yViEuiBIa';
      sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: applicationServerKey
      });
    }

    // Send subscription object to backend API
    const resp = await fetch('/api/v1/push/subscribe', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(sub)
    });

    if (resp.ok) {
      updatePushButtonState(btn);
      alert('Instant Web Push notifications enabled successfully!');
    } else {
      alert('Failed to register push subscription on server.');
    }
  } catch (err) {
    console.error('Error enabling push subscription:', err);
  }
}
