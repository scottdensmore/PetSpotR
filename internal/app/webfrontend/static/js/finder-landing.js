/**
 * PetSpotR - Mobile Finder Landing Page Controller
 *
 * Manages modal interactions, mediated messaging, quick sighting reports,
 * and clipboard copy actions with CSP compliance.
 */
(() => {
  'use strict';

  // DOM Elements
  let btnFound = null;
  let btnMessage = null;
  let btnSighting = null;
  let btnCopy = null;
  let cameraInput = null;
  let modalCamera = null;
  let modalMessage = null;
  let modalSighting = null;
  let formCamera = null;
  let formMessage = null;
  let formSighting = null;
  let previewImg = null;
  let gpsStatus = null;

  let capturedDataUrl = '';
  let cameraCoordinates = null;
  let sightingCoordinates = null;

  async function getCsrfToken() {
    let token = window.petspotrIdentity?.getState?.()?.csrfToken;
    if (token) return token;
    const match = document.cookie.match(/(?:^|;\s*)(?:__Host-)?petspotr_csrf(?:_sec)?=([^;]+)/);
    if (match) return decodeURIComponent(match[1]);
    try {
      const res = await fetch('/api/v1/session/csrf', { credentials: 'same-origin', cache: 'no-store' });
      if (res.ok) {
        const data = await res.json();
        return data.csrfToken || '';
      }
    } catch {
      // ignore
    }
    return '';
  }

  function openCameraModal() {
    if (modalCamera) {
      modalCamera.classList.remove('hidden');
      const firstInput = modalCamera.querySelector('input:not([type="hidden"]), textarea');
      if (firstInput) firstInput.focus();
    }
  }

  function closeCameraModal() {
    if (modalCamera) {
      modalCamera.classList.add('hidden');
    }
  }

  function openContactModal() {
    if (modalMessage) {
      modalMessage.classList.remove('hidden');
      const firstInput = modalMessage.querySelector('input, textarea');
      if (firstInput) firstInput.focus();
    }
  }

  function closeContactModal() {
    if (modalMessage) {
      modalMessage.classList.add('hidden');
    }
  }

  function openSightingModal() {
    if (modalSighting) {
      modalSighting.classList.remove('hidden');
      acquireSightingLocation();
      const firstInput = modalSighting.querySelector('input, textarea');
      if (firstInput) firstInput.focus();
    }
  }

  function closeSightingModal() {
    if (modalSighting) {
      modalSighting.classList.add('hidden');
    }
  }

  function promptCopy(text, label) {
    window.prompt('Copy shortlink:', text);
    if (label) {
      label.textContent = 'Copied!';
      setTimeout(() => { label.textContent = 'Copy Link'; }, 2000);
    }
  }

  async function copyShortlink() {
    const canonicalURL = btnCopy?.dataset?.shortUrl || (window.location.origin + window.location.pathname);
    const label = document.getElementById('copy-btn-label');

    if (typeof navigator !== 'undefined' && navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      try {
        await navigator.clipboard.writeText(canonicalURL);
        if (label) {
          label.textContent = 'Copied!';
          setTimeout(() => { label.textContent = 'Copy Link'; }, 2000);
        }
      } catch (err) {
        promptCopy(canonicalURL, label);
      }
    } else {
      promptCopy(canonicalURL, label);
    }
  }

  function acquireSightingLocation() {
    const statusEl = document.getElementById('sighting-gps-status');
    if (typeof navigator !== 'undefined' && navigator.geolocation) {
      if (statusEl) {
        statusEl.textContent = '📍 Acquiring GPS location...';
        statusEl.classList.remove('hidden');
      }
      navigator.geolocation.getCurrentPosition(
        (pos) => {
          sightingCoordinates = {
            latitude: pos.coords.latitude,
            longitude: pos.coords.longitude,
          };
          if (statusEl) {
            statusEl.textContent = `📍 GPS Acquired: ${pos.coords.latitude.toFixed(4)}, ${pos.coords.longitude.toFixed(4)}`;
          }
        },
        (err) => {
          console.warn('Sighting geolocation warning:', err);
          if (statusEl) {
            statusEl.textContent = '⚠️ GPS location unavailable or declined';
          }
        },
        { timeout: 10000, enableHighAccuracy: true }
      );
    } else {
      if (statusEl) {
        statusEl.textContent = '⚠️ Geolocation not supported';
        statusEl.classList.remove('hidden');
      }
    }
  }

  async function handleCameraSubmit(e) {
    e.preventDefault();
    const feedback = document.getElementById('camera-feedback');
    const submitBtn = document.getElementById('btn-submit-camera-report');
    if (submitBtn) {
      submitBtn.disabled = true;
      submitBtn.textContent = 'Submitting...';
    }

    const petContainer = document.querySelector('.finder-landing-container');
    const petID = petContainer?.dataset?.petId || window.location.pathname.split('/').pop() || '';
    const rawSpecies = petContainer?.dataset?.petSpecies || 'Dog';
    const species = rawSpecies.charAt(0).toUpperCase() + rawSpecies.slice(1).toLowerCase();
    const petName = petContainer?.dataset?.petName || 'Missing Pet';

    const locInput = document.getElementById('camera-finder-location');
    const notesInput = document.getElementById('camera-finder-notes');
    const emailInput = document.getElementById('camera-finder-email');

    const locationVal = locInput?.value?.trim() ||
      (cameraCoordinates ? `${cameraCoordinates.latitude.toFixed(5)}, ${cameraCoordinates.longitude.toFixed(5)}` : 'Street discovery');
    const notesVal = notesInput?.value?.trim() || '';
    const emailVal = emailInput?.value?.trim() || 'finder@petspotr.app';

    const imageObject = `images/found-pets/${Date.now()}-camera.jpg`;
    const payload = {
      species: species,
      location: locationVal,
      description: `Street camera recovery for ${petName}${notesVal ? ': ' + notesVal : ''}`,
      imageObject: imageObject,
      images: [{ object: imageObject, tag: 'primary' }],
      imageUrl: capturedDataUrl || `${window.location.origin}/static/favicon.svg`,
      finderEmail: emailVal,
    };
    if (cameraCoordinates) {
      payload.coordinates = {
        latitude: cameraCoordinates.latitude,
        longitude: cameraCoordinates.longitude,
      };
    }

    try {
      const headers = { 'Content-Type': 'application/json' };
      const csrf = await getCsrfToken();
      if (csrf) headers['X-CSRF-Token'] = csrf;

      const res = await fetch('/api/v1/found-pets', {
        method: 'POST',
        headers,
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        if (feedback) {
          feedback.className = 'form-feedback alert-success';
          feedback.textContent = 'Found pet report submitted! Thank you for helping reunite this pet!';
          feedback.classList.remove('hidden');
        }
        setTimeout(() => {
          closeCameraModal();
          if (feedback) feedback.classList.add('hidden');
          if (formCamera) formCamera.reset();
          if (previewImg) {
            previewImg.src = '';
            previewImg.classList.add('hidden');
          }
          capturedDataUrl = '';
          cameraCoordinates = null;
        }, 1500);
      } else {
        const errText = await res.text();
        throw new Error(errText || `Server responded with ${res.status}`);
      }
    } catch (err) {
      if (feedback) {
        feedback.className = 'form-feedback alert-error';
        feedback.textContent = `Submission failed: ${err.message || 'Please try again'}`;
        feedback.classList.remove('hidden');
      }
    } finally {
      if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Submit Found Report';
      }
    }
  }

  async function handleContactSubmit(e) {
    e.preventDefault();
    const feedback = document.getElementById('contact-feedback');
    const submitBtn = document.getElementById('btn-submit-contact-message');
    if (submitBtn) {
      submitBtn.disabled = true;
      submitBtn.textContent = 'Sending...';
    }

    const petContainer = document.querySelector('.finder-landing-container');
    const petID = petContainer?.dataset?.petId || window.location.pathname.split('/').pop() || '';
    const phoneOrEmail = document.getElementById('contact-sender-phone')?.value?.trim() || '';
    const text = document.getElementById('contact-message-body')?.value?.trim() || '';
    const senderName = document.getElementById('contact-sender-name')?.value?.trim() || '';

    let email = phoneOrEmail;
    if (!email || !email.includes('@')) {
      email = 'finder@petspotr.app';
    }

    const messageText = senderName ? `From ${senderName}: ${text}` : text;
    const payload = {
      senderEmail: email,
      message: messageText,
      matchId: petID,
    };

    try {
      const headers = {
        'Content-Type': 'application/json',
        'Idempotency-Key': 'msg-' + Date.now() + '-' + Math.random().toString(36).slice(2, 9),
      };
      const csrf = await getCsrfToken();
      if (csrf) headers['X-CSRF-Token'] = csrf;

      const res = await fetch('/api/v1/reunions/contact', {
        method: 'POST',
        headers,
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        if (feedback) {
          feedback.className = 'form-feedback alert-success';
          feedback.textContent = 'Message dispatched to pet owner via secure channel!';
          feedback.classList.remove('hidden');
        }
        setTimeout(() => {
          closeContactModal();
          if (feedback) feedback.classList.add('hidden');
          if (formMessage) formMessage.reset();
        }, 1500);
      } else {
        const errText = await res.text();
        throw new Error(errText || `Server returned ${res.status}`);
      }
    } catch (err) {
      if (feedback) {
        feedback.className = 'form-feedback alert-error';
        feedback.textContent = `Failed to send message: ${err.message || 'Please try again'}`;
        feedback.classList.remove('hidden');
      }
    } finally {
      if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Send Secure Message';
      }
    }
  }

  async function handleSightingSubmit(e) {
    e.preventDefault();
    const feedback = document.getElementById('sighting-feedback');
    const submitBtn = document.getElementById('btn-submit-sighting');
    if (submitBtn) {
      submitBtn.disabled = true;
      submitBtn.textContent = 'Submitting...';
    }

    const petContainer = document.querySelector('.finder-landing-container');
    const rawSpecies = petContainer?.dataset?.petSpecies || 'Dog';
    const species = rawSpecies.charAt(0).toUpperCase() + rawSpecies.slice(1).toLowerCase();
    const locationVal = document.getElementById('sighting-location')?.value?.trim() || 'Nearby';
    const sightingTime = document.getElementById('sighting-time')?.value?.trim() || '';
    const sightingNotes = document.getElementById('sighting-notes')?.value?.trim() || '';
    const finderEmail = document.getElementById('sighting-finder-email')?.value?.trim() || 'finder@petspotr.app';

    let description = sightingNotes;
    if (sightingTime) {
      description = `Seen at ${sightingTime}${sightingNotes ? ': ' + sightingNotes : ''}`;
    }
    if (!description) description = `Quick sighting report at ${locationVal}`;

    const photoEl = document.querySelector('.finder-pet-photo');
    const petPhotoUrl = photoEl ? photoEl.src : '';

    const payload = {
      species: species,
      location: locationVal,
      description: description,
      finderEmail: finderEmail,
      imageUrl: petPhotoUrl || `${window.location.origin}/static/favicon.svg`,
    };
    if (sightingCoordinates) {
      payload.coordinates = {
        latitude: sightingCoordinates.latitude,
        longitude: sightingCoordinates.longitude,
      };
    }

    try {
      const headers = { 'Content-Type': 'application/json' };
      const csrf = await getCsrfToken();
      if (csrf) headers['X-CSRF-Token'] = csrf;

      const res = await fetch('/api/v1/found-pets', {
        method: 'POST',
        headers,
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        if (feedback) {
          feedback.className = 'form-feedback alert-success';
          feedback.textContent = 'Sighting reported! Thank you for helping reunite this pet!';
          feedback.classList.remove('hidden');
        }
        setTimeout(() => {
          closeSightingModal();
          if (feedback) feedback.classList.add('hidden');
          if (formSighting) formSighting.reset();
          sightingCoordinates = null;
        }, 1500);
      } else {
        const errText = await res.text();
        throw new Error(errText || `Server returned ${res.status}`);
      }
    } catch (err) {
      if (feedback) {
        feedback.className = 'form-feedback alert-error';
        feedback.textContent = `Failed to submit sighting: ${err.message || 'Please try again'}`;
        feedback.classList.remove('hidden');
      }
    } finally {
      if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Submit Sighting';
      }
    }
  }

  function init() {
    btnFound = document.getElementById('btn-finder-found');
    btnMessage = document.getElementById('btn-finder-message');
    btnSighting = document.getElementById('btn-finder-sighting');
    btnCopy = document.getElementById('btn-copy-shortlink');
    cameraInput = document.getElementById('finder-camera-input');
    modalCamera = document.getElementById('modal-finder-camera');
    modalMessage = document.getElementById('modal-finder-message');
    modalSighting = document.getElementById('modal-finder-sighting');
    formCamera = document.getElementById('form-finder-camera');
    formMessage = document.getElementById('form-finder-message');
    formSighting = document.getElementById('form-finder-sighting');
    previewImg = document.getElementById('camera-preview-img');
    gpsStatus = document.getElementById('camera-gps-status');

    if (btnFound) {
      btnFound.addEventListener('click', (e) => {
        e.preventDefault();
        if (cameraInput) {
          cameraInput.click();
        }
      });
    }

    if (cameraInput) {
      cameraInput.addEventListener('change', () => {
        const file = cameraInput.files && cameraInput.files[0];
        if (!file) return;

        const reader = new FileReader();
        reader.onload = (e) => {
          capturedDataUrl = e.target.result;
          if (previewImg) {
            previewImg.src = capturedDataUrl;
            previewImg.classList.remove('hidden');
          }
        };
        reader.readAsDataURL(file);

        if (gpsStatus) {
          gpsStatus.textContent = '📍 Acquiring GPS location...';
        }
        if (typeof navigator !== 'undefined' && navigator.geolocation) {
          navigator.geolocation.getCurrentPosition(
            (pos) => {
              cameraCoordinates = {
                latitude: pos.coords.latitude,
                longitude: pos.coords.longitude,
              };
              if (gpsStatus) {
                gpsStatus.textContent = `📍 GPS Acquired: ${pos.coords.latitude.toFixed(5)}, ${pos.coords.longitude.toFixed(5)}`;
              }
              const locInput = document.getElementById('camera-finder-location');
              if (locInput && !locInput.value) {
                locInput.value = `${pos.coords.latitude.toFixed(5)}, ${pos.coords.longitude.toFixed(5)}`;
              }
            },
            (err) => {
              console.warn('Camera geolocation warning:', err);
              if (gpsStatus) {
                gpsStatus.textContent = '⚠️ GPS location unavailable or declined';
              }
            },
            { timeout: 10000, enableHighAccuracy: true }
          );
        } else {
          if (gpsStatus) {
            gpsStatus.textContent = '⚠️ Geolocation not supported by browser';
          }
        }

        openCameraModal();
      });
    }

    if (btnMessage) {
      btnMessage.addEventListener('click', (e) => {
        e.preventDefault();
        openContactModal();
      });
    }

    if (btnSighting) {
      btnSighting.addEventListener('click', (e) => {
        e.preventDefault();
        openSightingModal();
      });
    }

    if (btnCopy) {
      btnCopy.addEventListener('click', (e) => {
        e.preventDefault();
        copyShortlink();
      });
    }

    // Modal close buttons
    document.querySelectorAll('[data-close-modal="camera"]').forEach((el) => {
      el.addEventListener('click', (e) => {
        e.preventDefault();
        closeCameraModal();
      });
    });

    document.querySelectorAll('[data-close-modal="message"]').forEach((el) => {
      el.addEventListener('click', (e) => {
        e.preventDefault();
        closeContactModal();
      });
    });

    document.querySelectorAll('[data-close-modal="sighting"]').forEach((el) => {
      el.addEventListener('click', (e) => {
        e.preventDefault();
        closeSightingModal();
      });
    });

    if (formCamera) {
      formCamera.addEventListener('submit', handleCameraSubmit);
    }

    if (formMessage) {
      formMessage.addEventListener('submit', handleContactSubmit);
    }

    if (formSighting) {
      formSighting.addEventListener('submit', handleSightingSubmit);
    }

    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        closeCameraModal();
        closeContactModal();
        closeSightingModal();
      }
    });

    document.querySelectorAll('.modal-overlay').forEach((overlay) => {
      overlay.addEventListener('click', (e) => {
        if (e.target === overlay) {
          overlay.classList.add('hidden');
        }
      });
    });
  }

  // Export globally for testing / programmatic access if needed
  window.PetSpotRFinder = {
    openContactModal,
    closeContactModal,
    openSightingModal,
    closeSightingModal,
    openCameraModal,
    closeCameraModal,
    copyShortlink,
    handleContactSubmit,
    handleSightingSubmit,
    handleCameraSubmit,
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
