// Browser adapter for the Google Identity Platform session boundary.
(() => {
  const firebaseVersion = '12.16.0';
  const state = {
    enabled: false,
    principal: null,
    csrfToken: '',
    busy: false,
    error: '',
    unavailable: false,
  };
  let config = null;
  let adapter = null;
  let initializationError = null;

  function snapshot() {
    return {
      enabled: state.enabled,
      principal: state.principal,
      csrfToken: state.csrfToken,
      busy: state.busy,
      error: state.error,
      unavailable: state.unavailable,
    };
  }

  function announce() {
    document.dispatchEvent(new CustomEvent('petspotr:identity-changed', { detail: snapshot() }));
  }

  async function fetchJSON(url, options = {}) {
    const response = await fetch(url, {
      credentials: 'same-origin',
      cache: 'no-store',
      ...options,
    });
    if (!response.ok) {
      const error = new Error(`Identity request failed with status ${response.status}`);
      error.status = response.status;
      throw error;
    }
    return response.json();
  }

  async function refreshCSRF() {
    const result = await fetchJSON('/api/v1/session/csrf');
    if (!result.csrfToken) throw new Error('Identity response did not include a CSRF token');
    state.csrfToken = result.csrfToken;
    return state.csrfToken;
  }

  async function createFirebaseAdapter(clientConfig) {
    if (typeof window.petspotrFirebaseAuthAdapterFactory === 'function') {
      return window.petspotrFirebaseAuthAdapterFactory(clientConfig);
    }

    const appSDK = await import(`https://www.gstatic.com/firebasejs/${firebaseVersion}/firebase-app.js`);
    const authSDK = await import(`https://www.gstatic.com/firebasejs/${firebaseVersion}/firebase-auth.js`);
    const app = appSDK.initializeApp({
      apiKey: clientConfig.apiKey,
      authDomain: clientConfig.authDomain,
      projectId: clientConfig.projectId,
    });
    const auth = authSDK.getAuth(app);
    await authSDK.setPersistence(auth, authSDK.inMemoryPersistence);
    if (clientConfig.authEmulatorUrl) {
      authSDK.connectAuthEmulator(auth, clientConfig.authEmulatorUrl, { disableWarnings: true });
    }
    const provider = new authSDK.GoogleAuthProvider();
    provider.setCustomParameters({ prompt: 'select_account' });
    return {
      async signInWithGoogle() {
        const result = await authSDK.signInWithPopup(auth, provider);
        return result.user.getIdToken(true);
      },
      async signOut() {
        await authSDK.signOut(auth);
      },
    };
  }

  async function currentSession() {
    try {
      state.principal = await fetchJSON('/api/v1/session');
      await refreshCSRF();
    } catch (error) {
      if (error.status !== 401) throw error;
      state.principal = null;
      state.csrfToken = '';
    }
  }

  async function signInWithGoogle() {
    await ready;
    if (initializationError) throw initializationError;
    if (!state.enabled || state.busy) return snapshot();
    state.busy = true;
    state.error = '';
    announce();
    try {
      const authAdapter = adapter;
      if (!authAdapter) throw new Error('Google sign-in is unavailable');
      const idToken = await authAdapter.signInWithGoogle();
      await authAdapter.signOut();
      const loginCSRF = await refreshCSRF();
      state.principal = await fetchJSON('/api/v1/session', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': loginCSRF,
        },
        body: JSON.stringify({ idToken }),
      });
      await refreshCSRF();
    } catch (error) {
      state.principal = null;
      state.csrfToken = '';
      state.error = 'Google sign-in could not be completed. Please try again.';
      throw error;
    } finally {
      state.busy = false;
      announce();
    }
    return snapshot();
  }

  async function signOutSession() {
    await ready;
    if (!state.enabled || !state.principal || state.busy) return snapshot();
    state.busy = true;
    state.error = '';
    announce();
    try {
      const csrfToken = state.csrfToken || await refreshCSRF();
      const response = await fetch('/api/v1/session', {
        method: 'DELETE',
        credentials: 'same-origin',
        cache: 'no-store',
        headers: { 'X-CSRF-Token': csrfToken },
      });
      if (!response.ok) throw new Error(`Identity request failed with status ${response.status}`);
      state.principal = null;
      state.csrfToken = '';
    } catch (error) {
      state.error = 'Sign out could not be completed. Please try again.';
      throw error;
    } finally {
      state.busy = false;
      announce();
    }
    return snapshot();
  }

  async function requireSession() {
    await ready;
    if (initializationError) throw initializationError;
    if (!state.enabled) return snapshot();
    if (!state.principal) {
      const error = new Error('Google sign-in is required');
      error.code = 'identity-required';
      throw error;
    }
    if (!state.csrfToken) await refreshCSRF();
    return snapshot();
  }

  function renderIdentity(stateSnapshot) {
    const panel = document.getElementById('identity-panel');
    if (!panel) return;
    panel.hidden = !stateSnapshot.enabled;
    if (!stateSnapshot.enabled) return;

    const status = document.getElementById('identity-status');
    const error = document.getElementById('identity-error');
    const signIn = document.getElementById('google-sign-in');
    const signOut = document.getElementById('identity-sign-out');
    const contactEmail = document.querySelector('[data-identity-email]');
    const signedOutMessage = panel.dataset.identityPrompt ||
      'Sign in with Google before submitting this private report.';
    const focusWasOnSignIn = document.activeElement === signIn;
    const focusWasOnSignOut = document.activeElement === signOut;
    panel.setAttribute('aria-busy', String(stateSnapshot.busy));
    if (status) {
      status.textContent = stateSnapshot.unavailable
        ? 'Sign-in is temporarily unavailable.'
        : stateSnapshot.busy && stateSnapshot.principal
        ? 'Signing out...'
        : stateSnapshot.busy
        ? 'Signing in with Google...'
        : stateSnapshot.principal
        ? `Signed in as ${stateSnapshot.principal.email}`
        : signedOutMessage;
    }
    if (error) {
      error.textContent = stateSnapshot.error;
      error.hidden = !stateSnapshot.error;
    }
    if (signIn) {
      signIn.hidden = Boolean(stateSnapshot.principal) || stateSnapshot.unavailable;
      signIn.setAttribute('aria-disabled', String(stateSnapshot.busy));
    }
    if (signOut) {
      signOut.hidden = !stateSnapshot.principal;
      signOut.setAttribute('aria-disabled', String(stateSnapshot.busy));
    }
    if (contactEmail) {
      if (stateSnapshot.principal) {
        contactEmail.value = stateSnapshot.principal.email;
        contactEmail.setAttribute('readonly', '');
      } else {
        contactEmail.value = '';
        contactEmail.removeAttribute('readonly');
      }
    }
    if (!stateSnapshot.busy && stateSnapshot.principal && focusWasOnSignIn) {
      signOut?.focus();
    } else if (!stateSnapshot.busy && !stateSnapshot.principal && focusWasOnSignOut) {
      signIn?.focus();
    }
  }

  const ready = (async () => {
    try {
      config = await fetchJSON('/api/v1/session/client-config');
      state.enabled = Boolean(config.enabled && config.provider === 'google.com');
      if (state.enabled) {
        adapter = await createFirebaseAdapter(config);
        await currentSession();
      }
    } catch (error) {
      initializationError = error;
      initializationError.code = 'identity-unavailable';
      state.enabled = true;
      state.unavailable = true;
      state.error = 'Identity services are temporarily unavailable.';
    }
    announce();
    return snapshot();
  })();

  window.petspotrIdentity = {
    ready,
    getState: snapshot,
    requireSession,
    signInWithGoogle,
    signOut: signOutSession,
    focusSignIn() {
      document.getElementById('google-sign-in')?.focus();
    },
  };
  window.petspotrIdentityReady = ready;

  document.addEventListener('DOMContentLoaded', () => {
    document.getElementById('google-sign-in')?.addEventListener('click', () => {
      signInWithGoogle().catch(() => {});
    });
    document.getElementById('identity-sign-out')?.addEventListener('click', () => {
      signOutSession().catch(() => {});
    });
    document.addEventListener('petspotr:identity-changed', (event) => renderIdentity(event.detail));
    ready.then(renderIdentity);
  });
})();
