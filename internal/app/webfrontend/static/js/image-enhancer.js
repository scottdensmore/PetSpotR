/**
 * PetSpotR - Image Enhancer & EXIF Metadata Extractor
 *
 * Provides:
 * 1. Automatic EXIF metadata extraction (GPS coordinates & capture time) on photo upload.
 * 2. Privacy-first, zero-PII pre-fill prompt requiring explicit user consent ("Apply" vs "Keep Manual").
 * 3. AI image quality auto-enhancement toggle with interactive before/after comparison slider.
 *
 * Strict Content Security Policy compliant: zero inline scripts or eval.
 */
(() => {
  'use strict';

  // Intelligent EXIF pre-fill prompt prefix
  const EXIF_PROMPT_LABEL = '📍 Found photo location: ';

  // Format ISO timestamp to YYYY-MM-DDTHH:MM for datetime-local input fields
  function formatDatetimeLocal(isoStr) {
    if (!isoStr) return '';
    const d = new Date(isoStr);
    if (isNaN(d.getTime())) return '';
    const pad = n => String(n).padStart(2, '0');
    const year = d.getFullYear();
    const month = pad(d.getMonth() + 1);
    const day = pad(d.getDate());
    const hours = pad(d.getHours());
    const minutes = pad(d.getMinutes());
    return `${year}-${month}-${day}T${hours}:${minutes}`;
  }

  // Reads a File as an Object URL
  function fileToObjectUrl(file) {
    return URL.createObjectURL(file);
  }

  // API Client for metadata extraction
  async function apiExtractMetadata(file) {
    const formData = new FormData();
    formData.append('file', file);

    const res = await fetch('/api/v1/images/extract-metadata', {
      method: 'POST',
      body: formData,
    });

    if (!res.ok) {
      return null;
    }

    return await res.json();
  }

  // API Client for auto-enhancement
  async function apiEnhanceImage(file) {
    const formData = new FormData();
    formData.append('file', file);

    const res = await fetch('/api/v1/images/enhance', {
      method: 'POST',
      body: formData,
    });

    if (!res.ok) {
      throw new Error('Enhancement request failed: ' + res.status);
    }

    return await res.blob();
  }

  // Setup EXIF detection & enhancement for a specific context
  function setupEnhancerContext(options) {
    const {
      dropzoneEl,
      fileInputEl,
      toastEl,
      coordsDisplayEl,
      btnApplyEl,
      btnDismissEl,
      autoEnhanceContainerEl,
      btnAutoEnhanceEl,
      enhanceStatusEl,
      comparisonContainerEl,
      imgBeforeEl,
      imgAfterWrapEl,
      imgAfterEl,
      sliderEl,
      btnAcceptEl,
      btnRevertEl,
      onApplyGps,
      onUseEnhanced,
      onRevertEnhanced,
      previewTargetImgEl,
    } = options;

    let currentFile = null;
    let originalObjectUrl = null;
    let enhancedObjectUrl = null;
    let enhancedBlob = null;
    let pendingMetadata = null;

    function resetExifToast() {
      if (toastEl) {
        toastEl.classList.add('hidden');
      }
      pendingMetadata = null;
    }

    function showExifToast(meta) {
      if (!toastEl || !meta || !meta.gps) return;
      pendingMetadata = meta;

      const lat = meta.gps.latitude.toFixed(4);
      const lng = meta.gps.longitude.toFixed(4);
      const coordsText = `${lat}, ${lng}`;

      if (coordsDisplayEl) {
        coordsDisplayEl.textContent = coordsText;
      }
      toastEl.classList.remove('hidden');
    }

    function handleFileChosen(file) {
      if (!file || !file.type.startsWith('image/')) return;
      currentFile = file;
      enhancedBlob = null;

      if (originalObjectUrl) {
        try { URL.revokeObjectURL(originalObjectUrl); } catch (_) {}
      }
      originalObjectUrl = fileToObjectUrl(file);

      // Reset enhancement comparison view
      if (comparisonContainerEl) {
        comparisonContainerEl.classList.add('hidden');
      }
      if (autoEnhanceContainerEl) {
        autoEnhanceContainerEl.classList.remove('hidden');
      }
      if (enhanceStatusEl) {
        enhanceStatusEl.textContent = '';
      }
      if (btnAutoEnhanceEl) {
        btnAutoEnhanceEl.disabled = false;
      }

      // Asynchronously extract metadata
      apiExtractMetadata(file).then(meta => {
        if (currentFile !== file) return;
        if (meta && meta.gps && meta.gps.latitude != null && meta.gps.longitude != null) {
          showExifToast(meta);
        } else {
          resetExifToast();
        }
      }).catch(() => {
        if (currentFile !== file) return;
        resetExifToast();
      });
    }

    // Bind Apply & Dismiss buttons for EXIF prefill
    if (btnApplyEl) {
      btnApplyEl.addEventListener('click', () => {
        if (pendingMetadata && onApplyGps) {
          onApplyGps(pendingMetadata);
        }
        resetExifToast();
      });
    }

    if (btnDismissEl) {
      btnDismissEl.addEventListener('click', () => {
        resetExifToast();
      });
    }

    // Bind Auto-Enhance button
    if (btnAutoEnhanceEl) {
      btnAutoEnhanceEl.addEventListener('click', async () => {
        if (!currentFile) return;

        try {
          btnAutoEnhanceEl.disabled = true;
          if (enhanceStatusEl) {
            enhanceStatusEl.textContent = 'Enhancing image quality...';
          }

          const blob = await apiEnhanceImage(currentFile);
          enhancedBlob = blob;

          if (enhancedObjectUrl) {
            try { URL.revokeObjectURL(enhancedObjectUrl); } catch (_) {}
          }
          enhancedObjectUrl = URL.createObjectURL(blob);

          if (imgBeforeEl) imgBeforeEl.src = originalObjectUrl;
          if (imgAfterEl) {
            imgAfterEl.src = enhancedObjectUrl;
            imgAfterEl.style.setProperty('--slider-pos', '50%');
          }
          if (imgAfterWrapEl) {
            imgAfterWrapEl.style.width = '50%';
            imgAfterWrapEl.style.setProperty('--slider-pos', '50%');
          }
          if (sliderEl) {
            sliderEl.value = '50';
            if (sliderEl.parentElement) {
              sliderEl.parentElement.style.setProperty('--slider-pos', '50%');
            }
          }

          if (comparisonContainerEl) {
            comparisonContainerEl.classList.remove('hidden');
          }
          if (enhanceStatusEl) {
            enhanceStatusEl.textContent = '✨ Enhancement ready. Compare before/after:';
          }
        } catch (err) {
          console.warn('Auto-enhancement failed:', err);
          if (enhanceStatusEl) {
            enhanceStatusEl.textContent = 'Enhancement unavailable for this image.';
          }
        } finally {
          btnAutoEnhanceEl.disabled = false;
        }
      });
    }

    // Comparison slider listener
    if (sliderEl && imgAfterWrapEl) {
      sliderEl.addEventListener('input', (e) => {
        const val = e.target.value;
        imgAfterWrapEl.style.width = `${val}%`;
        imgAfterWrapEl.style.setProperty('--slider-pos', `${val}%`);
        if (imgAfterEl) {
          imgAfterEl.style.setProperty('--slider-pos', `${val}%`);
        }
        if (sliderEl.parentElement) {
          sliderEl.parentElement.style.setProperty('--slider-pos', `${val}%`);
        }
      });
    }

    // Accept / Revert buttons
    if (btnAcceptEl) {
      btnAcceptEl.addEventListener('click', () => {
        if (enhancedObjectUrl) {
          if (currentFile && enhancedBlob) {
            const enhancedFile = new File([enhancedBlob], currentFile.name, { type: 'image/jpeg' });
            window.dispatchEvent(new CustomEvent('petspotr:image-enhanced', {
              detail: {
                originalFile: currentFile,
                enhancedFile: enhancedFile,
                blob: enhancedBlob,
                objectUrl: enhancedObjectUrl,
              },
            }));
          }
          if (previewTargetImgEl) {
            previewTargetImgEl.src = enhancedObjectUrl;
          }
          if (onUseEnhanced) {
            onUseEnhanced(enhancedObjectUrl);
          }
          if (enhanceStatusEl) {
            enhanceStatusEl.textContent = '✓ Enhanced version selected.';
          }
        }
      });
    }

    if (btnRevertEl) {
      btnRevertEl.addEventListener('click', () => {
        if (originalObjectUrl) {
          if (previewTargetImgEl) {
            previewTargetImgEl.src = originalObjectUrl;
          }
          if (onRevertEnhanced) {
            onRevertEnhanced(originalObjectUrl);
          }
          if (enhanceStatusEl) {
            enhanceStatusEl.textContent = 'Original photo retained.';
          }
        }
      });
    }

    // Hook dropzone events if present
    if (dropzoneEl && fileInputEl) {
      dropzoneEl.addEventListener('click', (e) => {
        // If clicking directly on dropzone (or child other than input)
        if (e.target !== fileInputEl) {
          fileInputEl.click();
        }
      });

      dropzoneEl.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          fileInputEl.click();
        }
      });

      dropzoneEl.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropzoneEl.classList.add('is-dragging');
      });

      dropzoneEl.addEventListener('dragleave', () => {
        dropzoneEl.classList.remove('is-dragging');
      });

      dropzoneEl.addEventListener('drop', (e) => {
        e.preventDefault();
        dropzoneEl.classList.remove('is-dragging');
        if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length > 0) {
          handleFileChosen(e.dataTransfer.files[0]);
        }
      });

      fileInputEl.addEventListener('change', (e) => {
        if (e.target.files && e.target.files.length > 0) {
          handleFileChosen(e.target.files[0]);
        }
      }, { capture: true });
    } else if (fileInputEl) {
      fileInputEl.addEventListener('change', (e) => {
        if (e.target.files && e.target.files.length > 0) {
          handleFileChosen(e.target.files[0]);
        }
      }, { capture: true });
    }

    return {
      handleFileChosen,
    };
  }

  // Initialize enhancers
  function initEnhancers() {
    // -------------------------------------------------------------
    // 1. Report Lost Pet Wizard (report-lost.html)
    // -------------------------------------------------------------
    const lostDropzoneContainer = document.getElementById('photo-dropzone');
    const lostDropzoneBtn = document.getElementById('dropzone');
    const lostFileInput = document.getElementById('photoInput');
    const lostForm = document.getElementById('lost-pet-form');

    if (lostForm && lostFileInput) {
      setupEnhancerContext({
        dropzoneEl: lostDropzoneBtn || lostDropzoneContainer,
        fileInputEl: lostFileInput,
        toastEl: document.getElementById('exif-toast'),
        coordsDisplayEl: document.getElementById('exif-coords-display'),
        btnApplyEl: document.getElementById('btn-exif-apply'),
        btnDismissEl: document.getElementById('btn-exif-dismiss'),
        autoEnhanceContainerEl: document.getElementById('auto-enhance-control'),
        btnAutoEnhanceEl: document.getElementById('btn-auto-enhance'),
        enhanceStatusEl: document.getElementById('auto-enhance-status'),
        comparisonContainerEl: document.getElementById('enhance-slider-view'),
        imgBeforeEl: document.getElementById('enhance-img-before'),
        imgAfterWrapEl: document.getElementById('enhance-img-after-wrap'),
        imgAfterEl: document.getElementById('enhance-img-after'),
        sliderEl: document.getElementById('enhance-slider'),
        btnAcceptEl: document.getElementById('btn-accept-enhanced'),
        btnRevertEl: document.getElementById('btn-revert-enhanced'),
        onApplyGps: (meta) => {
          const lat = meta.gps.latitude.toFixed(4);
          const lng = meta.gps.longitude.toFixed(4);

          const latFields = [document.getElementById('latitude'), document.getElementById('lost-lat')].filter(Boolean);
          const lngFields = [document.getElementById('longitude'), document.getElementById('lost-lng')].filter(Boolean);
          const timeField = document.getElementById('lost-time');
          const dateInput = document.getElementById('lostDate') || document.getElementById('lostTime') || document.getElementById('lastSeenTime');
          const locInput = document.getElementById('location');

          latFields.forEach(f => { f.value = meta.gps.latitude; });
          lngFields.forEach(f => { f.value = meta.gps.longitude; });
          if (meta.captureTime) {
            if (timeField) timeField.value = meta.captureTime;
            if (dateInput) dateInput.value = formatDatetimeLocal(meta.captureTime);
          }
          if (locInput) {
            locInput.value = `${lat}, ${lng}`;
          }

          // Update map placeholder if present
          const mapPin = document.querySelector('#wizard-step-3 .map-placeholder span');
          if (mapPin) {
            mapPin.textContent = `Lat: ${lat}, Long: ${lng} (From Photo EXIF)`;
          }
        },
        onUseEnhanced: (enhancedUrl) => {
          const previewImg = document.querySelector('.photo-staging-thumb.lost-preview-image');
          if (previewImg) previewImg.src = enhancedUrl;
        },
        onRevertEnhanced: (originalUrl) => {
          const previewImg = document.querySelector('.photo-staging-thumb.lost-preview-image');
          if (previewImg) previewImg.src = originalUrl;
        },
      });
    }

    // -------------------------------------------------------------
    // 2. Report Found Pet Form (report-found.html)
    // -------------------------------------------------------------
    const foundDropzoneBtn = document.getElementById('found-dropzone');
    const foundDropzoneContainer = document.querySelector('.found-report #photo-dropzone');
    const foundFileInput = document.getElementById('foundPhotoInput');
    const foundForm = document.getElementById('found-pet-form');

    if (foundForm && foundFileInput) {
      setupEnhancerContext({
        dropzoneEl: foundDropzoneBtn || foundDropzoneContainer,
        fileInputEl: foundFileInput,
        toastEl: document.getElementById('found-exif-toast'),
        coordsDisplayEl: document.getElementById('found-exif-coords-display'),
        btnApplyEl: document.getElementById('btn-found-exif-apply'),
        btnDismissEl: document.getElementById('btn-found-exif-dismiss'),
        autoEnhanceContainerEl: document.getElementById('found-auto-enhance'),
        btnAutoEnhanceEl: document.getElementById('btn-found-auto-enhance'),
        enhanceStatusEl: document.getElementById('found-auto-enhance-status'),
        comparisonContainerEl: document.getElementById('found-enhance-slider-view'),
        imgBeforeEl: document.getElementById('found-enhance-img-before'),
        imgAfterWrapEl: document.getElementById('found-enhance-img-after-wrap'),
        imgAfterEl: document.getElementById('found-enhance-img-after'),
        sliderEl: document.getElementById('found-enhance-slider'),
        btnAcceptEl: document.getElementById('btn-found-accept-enhanced'),
        btnRevertEl: document.getElementById('btn-found-revert-enhanced'),
        previewTargetImgEl: document.getElementById('foundImagePreview'),
        onApplyGps: (meta) => {
          const lat = meta.gps.latitude.toFixed(4);
          const lng = meta.gps.longitude.toFixed(4);

          const latField = document.getElementById('found-lat');
          const lngField = document.getElementById('found-lng');
          const timeField = document.getElementById('found-time');
          const dateInput = document.getElementById('foundDate');
          const locInput = document.getElementById('foundLocation');

          if (latField) latField.value = meta.gps.latitude;
          if (lngField) lngField.value = meta.gps.longitude;
          if (meta.captureTime) {
            if (timeField) timeField.value = meta.captureTime;
            if (dateInput) dateInput.value = formatDatetimeLocal(meta.captureTime);
          }
          if (locInput && !locInput.value.trim()) {
            locInput.value = `${lat}, ${lng}`;
          }
        },
        onUseEnhanced: (enhancedUrl) => {
          const previewImg = document.getElementById('foundImagePreview');
          if (previewImg) previewImg.src = enhancedUrl;
          const stagedThumb = document.querySelector('.found-preview-image');
          if (stagedThumb) stagedThumb.src = enhancedUrl;
        },
        onRevertEnhanced: (originalUrl) => {
          const previewImg = document.getElementById('foundImagePreview');
          if (previewImg) previewImg.src = originalUrl;
          const stagedThumb = document.querySelector('.found-preview-image');
          if (stagedThumb) stagedThumb.src = originalUrl;
        },
      });
    }

    // -------------------------------------------------------------
    // 3. Quick Sighting Modal (modal-report-sighting across pages)
    // -------------------------------------------------------------
    const sightingModals = document.querySelectorAll('#modal-report-sighting');
    sightingModals.forEach((modal) => {
      const dropzoneEl = modal.querySelector('#photo-dropzone, .sighting-dropzone');
      const fileInputEl = modal.querySelector('#sighting-photo-input');
      const toastEl = modal.querySelector('#sighting-exif-toast, .exif-prefill-toast');
      const coordsDisplayEl = modal.querySelector('.exif-coords-display, #sighting-exif-toast strong');
      const btnApplyEl = modal.querySelector('.btn-exif-apply, #btn-sighting-exif-apply');
      const btnDismissEl = modal.querySelector('.btn-exif-dismiss, #btn-sighting-exif-dismiss');
      const autoEnhanceContainerEl = modal.querySelector('#sighting-auto-enhance, .auto-enhance-container');
      const btnAutoEnhanceEl = modal.querySelector('.btn-auto-enhance, #btn-sighting-auto-enhance');
      const enhanceStatusEl = modal.querySelector('.auto-enhance-status');
      const comparisonContainerEl = modal.querySelector('.enhance-comparison-container');
      const imgBeforeEl = modal.querySelector('.enhance-img-before');
      const imgAfterWrapEl = modal.querySelector('.enhance-img-after-wrap');
      const imgAfterEl = modal.querySelector('.enhance-img-after');
      const sliderEl = modal.querySelector('.enhance-slider');
      const btnAcceptEl = modal.querySelector('.btn-accept-enhanced');
      const btnRevertEl = modal.querySelector('.btn-revert-enhanced');
      const previewThumb = modal.querySelector('#sighting-photo-preview');
      const previewContainer = modal.querySelector('#sighting-preview-container');

      if (dropzoneEl && fileInputEl) {
        setupEnhancerContext({
          dropzoneEl,
          fileInputEl,
          toastEl,
          coordsDisplayEl,
          btnApplyEl,
          btnDismissEl,
          autoEnhanceContainerEl,
          btnAutoEnhanceEl,
          enhanceStatusEl,
          comparisonContainerEl,
          imgBeforeEl,
          imgAfterWrapEl,
          imgAfterEl,
          sliderEl,
          btnAcceptEl,
          btnRevertEl,
          previewTargetImgEl: previewThumb,
          onApplyGps: (meta) => {
            const lat = meta.gps.latitude.toFixed(4);
            const lng = meta.gps.longitude.toFixed(4);

            const latInput = modal.querySelector('#sighting-lat');
            const lngInput = modal.querySelector('#sighting-lng');
            const locInput = modal.querySelector('#sighting-location');
            const gpsStatus = modal.querySelector('#sighting-gps-status');
            const timeModeSelect = modal.querySelector('#sighting-time-mode');
            const customTimeGroup = modal.querySelector('#sighting-custom-time-group');
            const customTimeInput = modal.querySelector('#sighting-custom-time');

            if (latInput) latInput.value = meta.gps.latitude;
            if (lngInput) lngInput.value = meta.gps.longitude;
            if (gpsStatus) {
              gpsStatus.textContent = `📍 GPS acquired from photo: ${lat}, ${lng}`;
              gpsStatus.classList.add('acquired');
            }
            if (locInput && !locInput.value.trim()) {
              locInput.value = `${lat}, ${lng}`;
            }

            if (meta.captureTime) {
              if (timeModeSelect) timeModeSelect.value = 'earlier';
              if (customTimeGroup) customTimeGroup.hidden = false;
              if (customTimeInput) customTimeInput.value = formatDatetimeLocal(meta.captureTime);
            }
          },
          onUseEnhanced: (enhancedUrl) => {
            if (previewThumb) {
              previewThumb.src = enhancedUrl;
              if (previewContainer) previewContainer.classList.remove('hidden');
            }
          },
          onRevertEnhanced: (originalUrl) => {
            if (previewThumb) {
              previewThumb.src = originalUrl;
              if (previewContainer) previewContainer.classList.remove('hidden');
            }
          },
        });

        // Also display thumbnail when chosen in sighting modal
        fileInputEl.addEventListener('change', (e) => {
          if (e.target.files && e.target.files[0] && previewThumb && previewContainer) {
            previewThumb.src = fileToObjectUrl(e.target.files[0]);
            previewContainer.classList.remove('hidden');
          }
        });
      }
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initEnhancers);
  } else {
    initEnhancers();
  }
})();
