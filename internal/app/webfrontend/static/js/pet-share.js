/**
 * PetSpotR - Client-Side Poster Customizer & Social Share Controller
 *
 * Provides interactive poster customization, debounced live preview iframe
 * synchronization, native Web Share API with clipboard fallback, toast feedback,
 * print execution, and accessible modal lifecycle.
 */
(() => {
  'use strict';

  // Modal & Control DOM references
  let modal = null;
  let btnClose = null;
  let rewardInput = null;
  let emergencyInput = null;
  let phoneToggle = null;
  let phoneInput = null;
  let btnPrint = null;
  let standaloneLink = null;
  let btnShare = null;
  let btnCopy = null;
  let previewFrame = null;

  // State
  let activePetId = '';
  let activePetName = '';
  let debounceTimer = null;
  let previouslyFocusedElement = null;

  /**
   * Displays an accessible toast notification.
   * Delegates to window.PetSpotROutbox.showToast if present,
   * otherwise creates/appends to #toast-container directly.
   *
   * @param {string} message
   * @param {'info'|'success'|'error'|'warning'} type
   * @returns {HTMLElement|null}
   */
  function showToast(message, type = 'info') {
    if (typeof window !== 'undefined' && window.PetSpotROutbox && typeof window.PetSpotROutbox.showToast === 'function') {
      return window.PetSpotROutbox.showToast(message, type);
    }

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
   * Constructs the canonical shortlink URL for finder landing page.
   *
   * @param {string} petId
   * @returns {string}
   */
  function getShortURL(petId) {
    if (!petId) return '';
    const origin = window.location.origin || `${window.location.protocol}//${window.location.host}`;
    return `${origin}/p/${encodeURIComponent(petId)}`;
  }

  /**
   * Builds the preview/printable poster URL with query parameters based on current input values.
   *
   * @param {string} petId
   * @returns {string}
   */
  function buildPosterURL(petId) {
    if (!petId) return '';
    const parts = [];

    const rewardVal = rewardInput ? rewardInput.value.trim() : '';
    if (rewardVal) {
      parts.push(`reward=${encodeURIComponent(rewardVal)}`);
    }

    const emergencyVal = emergencyInput ? emergencyInput.value.trim() : '';
    if (emergencyVal) {
      parts.push(`emergency=${encodeURIComponent(emergencyVal)}`);
    }

    if (phoneToggle && phoneToggle.checked) {
      const phoneVal = phoneInput ? phoneInput.value.trim() : '';
      if (phoneVal) {
        parts.push(`phone=${encodeURIComponent(phoneVal)}`);
      }
    }

    const basePath = `/pets/${encodeURIComponent(petId)}/poster`;
    return parts.length > 0 ? `${basePath}?${parts.join('&')}` : basePath;
  }

  /**
   * Updates preview iframe and standalone link href immediately.
   */
  function updatePreview() {
    if (!activePetId) return;
    const url = buildPosterURL(activePetId);

    if (previewFrame) {
      previewFrame.src = url;
    }
    if (standaloneLink) {
      standaloneLink.href = url;
    }
  }

  /**
   * Debounces updates by ~250ms to prevent excessive iframe reloads while typing.
   */
  function schedulePreviewUpdate() {
    if (debounceTimer) {
      clearTimeout(debounceTimer);
    }
    debounceTimer = setTimeout(() => {
      updatePreview();
    }, 250);
  }

  /**
   * Opens the poster customization & share modal for a specific pet.
   *
   * @param {string} petId
   * @param {string} [petName]
   */
  function openModal(petId, petName = '') {
    if (!petId) return;
    activePetId = petId;
    activePetName = petName;

    // Reset controls
    if (rewardInput) rewardInput.value = '';
    if (emergencyInput) emergencyInput.value = '';
    if (phoneToggle) phoneToggle.checked = false;
    if (phoneInput) {
      phoneInput.value = '';
      phoneInput.classList.add('hidden');
    }

    // Set initial preview iframe and standalone link href
    const initialURL = `/pets/${encodeURIComponent(petId)}/poster`;
    if (previewFrame) {
      previewFrame.src = initialURL;
    }
    if (standaloneLink) {
      standaloneLink.href = initialURL;
    }

    // Show modal & set accessibility attributes
    if (modal) {
      previouslyFocusedElement = document.activeElement;
      modal.classList.remove('hidden');
      modal.setAttribute('aria-hidden', 'false');

      // Set focus to the first interactive input or close button
      if (rewardInput) {
        rewardInput.focus();
      } else if (btnClose) {
        btnClose.focus();
      }
    }
  }

  /**
   * Closes the poster customization & share modal.
   */
  function closeModal() {
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }

    if (modal) {
      modal.classList.add('hidden');
      modal.setAttribute('aria-hidden', 'true');
    }

    if (previewFrame) {
      previewFrame.src = 'about:blank';
    }

    if (previouslyFocusedElement && typeof previouslyFocusedElement.focus === 'function') {
      previouslyFocusedElement.focus();
      previouslyFocusedElement = null;
    }
  }

  /**
   * Traps keyboard focus within the open modal dialog.
   *
   * @param {KeyboardEvent} e
   */
  function trapFocus(e) {
    if (!modal || modal.classList.contains('hidden')) return;

    const focusable = modal.querySelectorAll(
      'button:not([disabled]), [href], input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    );
    const visible = Array.from(focusable).filter((el) => el.offsetParent !== null && !el.classList.contains('hidden'));
    if (visible.length === 0) return;

    const first = visible[0];
    const last = visible[visible.length - 1];

    if (e.shiftKey) {
      if (document.activeElement === first || !modal.contains(document.activeElement)) {
        e.preventDefault();
        last.focus();
      }
    } else {
      if (document.activeElement === last || !modal.contains(document.activeElement)) {
        e.preventDefault();
        first.focus();
      }
    }
  }

  /**
   * Handles printing the customized poster.
   * Tries to invoke print on the preview iframe contentWindow,
   * falling back to opening the standalone URL in a new tab or window.print().
   */
  function handlePrint() {
    let printed = false;
    if (previewFrame && previewFrame.contentWindow) {
      try {
        previewFrame.contentWindow.focus();
        previewFrame.contentWindow.print();
        printed = true;
      } catch (err) {
        console.warn('Printing via iframe contentWindow failed:', err);
      }
    }

    if (!printed) {
      const url = buildPosterURL(activePetId) || (standaloneLink ? standaloneLink.href : '');
      if (url && url !== '#' && url !== 'about:blank') {
        const printWindow = window.open(url, '_blank');
        if (printWindow) {
          printWindow.focus();
        } else {
          window.print();
        }
      } else {
        window.print();
      }
    }
  }

  /**
   * Copies the finder shortlink URL to the clipboard with toast feedback.
   *
   * @returns {Promise<boolean>}
   */
  async function handleCopyShortlink() {
    if (!activePetId) return false;
    const shortURL = getShortURL(activePetId);
    let copied = false;

    if (typeof navigator !== 'undefined' && navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      try {
        await navigator.clipboard.writeText(shortURL);
        copied = true;
      } catch (err) {
        console.warn('navigator.clipboard.writeText failed:', err);
      }
    }

    if (!copied && typeof document !== 'undefined') {
      try {
        const textarea = document.createElement('textarea');
        textarea.value = shortURL;
        textarea.setAttribute('readonly', '');
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        textarea.style.top = '-9999px';
        document.body.appendChild(textarea);
        textarea.select();
        copied = document.execCommand('copy');
        document.body.removeChild(textarea);
      } catch (err) {
        console.warn('Fallback document.execCommand copy failed:', err);
      }
    }

    if (copied) {
      showToast('Link copied to clipboard!', 'success');
    } else {
      showToast('Failed to copy link to clipboard.', 'error');
    }

    return copied;
  }

  /**
   * Invokes native Web Share API with title, text, and shortlink URL.
   * Catches AbortError gracefully; falls back to clipboard copy on failure or lack of support.
   */
  async function handleShareSocial() {
    if (!activePetId) return;
    const shortURL = getShortURL(activePetId);
    const petNameDisplay = activePetName ? activePetName : 'Lost Pet';
    const shareData = {
      title: `Help find ${petNameDisplay}! | PetSpotR`,
      text: `Please help find ${petNameDisplay}! Report sightings or contact info here:`,
      url: shortURL,
    };

    if (typeof navigator !== 'undefined' && typeof navigator.share === 'function') {
      try {
        await navigator.share(shareData);
        return;
      } catch (err) {
        if (err && err.name === 'AbortError') {
          // User cancelled native share sheet; gracefully exit
          return;
        }
        console.warn('navigator.share failed, falling back to clipboard copy:', err);
      }
    }

    await handleCopyShortlink();
  }

  /**
   * Initializes DOM element bindings and event listeners.
   */
  function init() {
    modal = document.getElementById('poster-modal');
    if (!modal) return;

    btnClose = document.getElementById('btn-close-poster-modal');
    rewardInput = document.getElementById('poster-reward-input');
    emergencyInput = document.getElementById('poster-emergency-input');
    phoneToggle = document.getElementById('poster-phone-toggle');
    phoneInput = document.getElementById('poster-phone-input');
    btnPrint = document.getElementById('btn-print-poster');
    standaloneLink = document.getElementById('link-standalone-poster');
    btnShare = document.getElementById('btn-share-social');
    btnCopy = document.getElementById('btn-copy-shortlink');
    previewFrame = document.getElementById('poster-preview-frame');

    // Initial accessibility state
    if (modal.classList.contains('hidden')) {
      modal.setAttribute('aria-hidden', 'true');
    } else {
      modal.setAttribute('aria-hidden', 'false');
    }

    // Modal close listeners
    if (btnClose) {
      btnClose.addEventListener('click', (e) => {
        e.preventDefault();
        closeModal();
      });
    }

    modal.addEventListener('click', (e) => {
      if (e.target === modal) {
        closeModal();
      }
    });

    document.addEventListener('keydown', (e) => {
      if (!modal || modal.classList.contains('hidden')) return;

      if (e.key === 'Escape') {
        e.preventDefault();
        closeModal();
      } else if (e.key === 'Tab') {
        trapFocus(e);
      }
    });

    // Input synchronization listeners
    if (rewardInput) {
      rewardInput.addEventListener('input', schedulePreviewUpdate);
      rewardInput.addEventListener('change', schedulePreviewUpdate);
    }

    if (emergencyInput) {
      emergencyInput.addEventListener('input', schedulePreviewUpdate);
      emergencyInput.addEventListener('change', schedulePreviewUpdate);
    }

    if (phoneToggle) {
      phoneToggle.addEventListener('change', () => {
        if (phoneToggle.checked) {
          if (phoneInput) {
            phoneInput.classList.remove('hidden');
            phoneInput.focus();
          }
        } else {
          if (phoneInput) {
            phoneInput.classList.add('hidden');
          }
        }
        schedulePreviewUpdate();
      });
    }

    if (phoneInput) {
      phoneInput.addEventListener('input', schedulePreviewUpdate);
      phoneInput.addEventListener('change', schedulePreviewUpdate);
    }

    // Modal action button listeners
    if (btnPrint) {
      btnPrint.addEventListener('click', (e) => {
        e.preventDefault();
        handlePrint();
      });
    }

    if (btnShare) {
      btnShare.addEventListener('click', (e) => {
        e.preventDefault();
        handleShareSocial();
      });
    }

    if (btnCopy) {
      btnCopy.addEventListener('click', (e) => {
        e.preventDefault();
        handleCopyShortlink();
      });
    }
  }

  // Export public API
  window.PetShare = {
    openModal: openModal,
    initPosterModal: openModal,
    closeModal: closeModal,
    updatePreview: updatePreview,
    schedulePreviewUpdate: schedulePreviewUpdate,
    buildPosterURL: buildPosterURL,
    printPoster: handlePrint,
    shareSocial: handleShareSocial,
    copyShortlink: handleCopyShortlink,
    showToast: showToast,
    init: init,
  };

  // Auto-bind on DOM ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
