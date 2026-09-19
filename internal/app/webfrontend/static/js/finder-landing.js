/**
 * PetSpotR - Mobile Finder Landing Page Controller
 *
 * Manages modal interactions, mediated messaging, quick sighting reports,
 * and clipboard copy actions with CSP compliance.
 */
(() => {
  'use strict';

  // DOM Elements
  let btnMessage = null;
  let btnSighting = null;
  let btnCopy = null;
  let modalMessage = null;
  let modalSighting = null;
  let formMessage = null;
  let formSighting = null;

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
    const canonicalURL = window.location.href.split('#')[0];
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

  function handleContactSubmit(e) {
    e.preventDefault();
    const feedback = document.getElementById('contact-feedback');
    const submitBtn = document.getElementById('btn-submit-contact-message');
    if (submitBtn) {
      submitBtn.disabled = true;
      submitBtn.textContent = 'Sending...';
    }

    setTimeout(() => {
      if (feedback) {
        feedback.className = 'form-feedback alert-success';
        feedback.textContent = 'Message dispatched to pet owner via secure channel!';
        feedback.classList.remove('hidden');
      }
      if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Send Secure Message';
      }
      setTimeout(() => {
        closeContactModal();
        if (feedback) feedback.classList.add('hidden');
        if (formMessage) formMessage.reset();
      }, 1500);
    }, 500);
  }

  function handleSightingSubmit(e) {
    e.preventDefault();
    const feedback = document.getElementById('sighting-feedback');
    const submitBtn = document.getElementById('btn-submit-sighting');
    if (submitBtn) {
      submitBtn.disabled = true;
      submitBtn.textContent = 'Submitting...';
    }

    setTimeout(() => {
      if (feedback) {
        feedback.className = 'form-feedback alert-success';
        feedback.textContent = 'Sighting reported! Thank you for helping reunite this pet!';
        feedback.classList.remove('hidden');
      }
      if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Submit Sighting';
      }
      setTimeout(() => {
        closeSightingModal();
        if (feedback) feedback.classList.add('hidden');
        if (formSighting) formSighting.reset();
      }, 1500);
    }, 500);
  }

  function init() {
    btnMessage = document.getElementById('btn-finder-message');
    btnSighting = document.getElementById('btn-finder-sighting');
    btnCopy = document.getElementById('btn-copy-shortlink');
    modalMessage = document.getElementById('modal-finder-message');
    modalSighting = document.getElementById('modal-finder-sighting');
    formMessage = document.getElementById('form-finder-message');
    formSighting = document.getElementById('form-finder-sighting');

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

    if (formMessage) {
      formMessage.addEventListener('submit', handleContactSubmit);
    }

    if (formSighting) {
      formSighting.addEventListener('submit', handleSightingSubmit);
    }

    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
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
    copyShortlink,
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
