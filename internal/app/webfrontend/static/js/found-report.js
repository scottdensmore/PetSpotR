// Client-side controller for Found Pet report form with AI auto-extraction
document.addEventListener('DOMContentLoaded', () => {
  const dropzone = document.getElementById('found-dropzone');
  const photoInput = document.getElementById('foundPhotoInput');
  const photoCountBadge = document.getElementById('photo-count-badge');
  const stagingContainer = document.getElementById('staging-container');
  const photoTemplate = document.getElementById('staged-photo-template');
  const previewContainer = document.getElementById('found-preview-container');
  const imagePreview = document.getElementById('foundImagePreview');
  const spinner = document.getElementById('ai-spinner');
  const extractionStatus = document.getElementById('ai-extraction-status');
  const form = document.getElementById('found-pet-form');

  const chipSpecies = document.getElementById('chip-species');
  const chipBreed = document.getElementById('chip-breed');
  const chipColor = document.getElementById('chip-color');

  const inputSpecies = document.getElementById('foundSpecies');
  const inputBreed = document.getElementById('foundBreed');
  const inputPrimaryColor = document.getElementById('foundPrimaryColor');
  const inputSecondaryColor = document.getElementById('foundSecondaryColor');
  const foundMicrochipInput = document.getElementById('found-pet-microchip');
  const foundMicrochipFeedback = document.getElementById('found-microchip-feedback');

  const icarPrefixMap = {
    '985': 'HomeAgain',
    '981': 'AKC Reunite',
    '977': 'PetLink',
    '982': '24Petwatch',
    '965': 'BuddyID',
  };

  function validateMicrochipInput(value) {
    if (!value) {
      return { valid: true, empty: true, message: '' };
    }
    const clean = value.replace(/[\s\-\*]/g, '');
    if (!clean) {
      return { valid: true, empty: true, message: '' };
    }
    if (clean.length === 15 && /^\d{15}$/.test(clean)) {
      const prefix = clean.slice(0, 3);
      const registry = icarPrefixMap[prefix] || 'Standard ISO Registry';
      return {
        valid: true,
        empty: false,
        message: `✓ Valid 15-Digit ISO Microchip • ${registry}`,
      };
    }
    if (clean.length === 9 && /^\d{9}$/.test(clean)) {
      return {
        valid: true,
        empty: false,
        message: '✓ Valid 9-Digit Avid Microchip • Avid Registry',
      };
    }
    if (clean.length === 10 && /^[0-9a-zA-Z]{10}$/.test(clean)) {
      return {
        valid: true,
        empty: false,
        message: '✓ Valid 10-Character Euro/Trovan Microchip',
      };
    }
    return {
      valid: false,
      empty: false,
      message: 'ℹ️ Standard microchips are 15 digits (ISO), 9 digits (Avid), or 10 alphanumeric (Euro).',
    };
  }

  if (foundMicrochipInput && foundMicrochipFeedback) {
    foundMicrochipInput.addEventListener('input', () => {
      const res = validateMicrochipInput(foundMicrochipInput.value);
      if (res.empty) {
        foundMicrochipFeedback.textContent = '';
        foundMicrochipFeedback.className = 'microchip-feedback-msg';
      } else if (res.valid) {
        foundMicrochipFeedback.textContent = res.message;
        foundMicrochipFeedback.className = 'microchip-feedback-msg valid';
      } else {
        foundMicrochipFeedback.textContent = res.message;
        foundMicrochipFeedback.className = 'microchip-feedback-msg invalid';
      }
    });
  }

  function showFieldError(fieldId, errorId, message) {
    const field = document.getElementById(fieldId);
    const errorEl = document.getElementById(errorId);
    if (field) field.setAttribute('aria-invalid', 'true');
    if (errorEl) {
      errorEl.textContent = message;
      errorEl.hidden = false;
    }
    field?.focus();
  }

  function clearFieldError(fieldId, errorId) {
    const field = document.getElementById(fieldId);
    const errorEl = document.getElementById(errorId);
    if (field) field.removeAttribute('aria-invalid');
    if (errorEl) {
      errorEl.textContent = '';
      errorEl.hidden = true;
    }
  }

  function showFormError(message) {
    const formError = document.getElementById('found-form-status');
    if (formError) {
      formError.textContent = message;
      formError.hidden = false;
    }
  }

  function clearFormError() {
    const formError = document.getElementById('found-form-status');
    if (formError) {
      formError.textContent = '';
      formError.hidden = true;
    }
  }

  document.getElementById('foundLocation')?.addEventListener('input', () => clearFieldError('foundLocation', 'found-location-error'));
  document.getElementById('finderEmail')?.addEventListener('input', () => clearFieldError('finderEmail', 'finder-email-error'));

  let currentImageUrl = '';
  let currentDistinctiveMarkings = [];
  let pendingSubmission = null;
  let extractionSequence = 0;
  let stagedImages = [];
  window.petspotrStagedImages = stagedImages;

  function clearExtractedTraits() {
    currentDistinctiveMarkings = [];
    if (inputSpecies) inputSpecies.value = '';
    if (inputBreed) inputBreed.value = '';
    if (inputPrimaryColor) inputPrimaryColor.value = '';
    if (inputSecondaryColor) inputSecondaryColor.value = '';
    if (chipSpecies) chipSpecies.textContent = 'Species: Not analyzed';
    if (chipBreed) chipBreed.textContent = 'Breed: Not analyzed';
    if (chipColor) chipColor.textContent = 'Colors: Not analyzed';
    if (foundMicrochipFeedback) {
      foundMicrochipFeedback.textContent = '';
      foundMicrochipFeedback.className = 'microchip-feedback-msg';
    }
  }

  function updatePhotoCount() {
    window.petspotrStagedImages = stagedImages;
    if (photoCountBadge) {
      photoCountBadge.textContent = `${stagedImages.length} / 3 photos added`;
    }
    if (stagingContainer) {
      stagingContainer.hidden = stagedImages.length === 0;
    }
    if (dropzone) {
      if (stagedImages.length >= 3) {
        dropzone.setAttribute('aria-disabled', 'true');
      } else {
        dropzone.setAttribute('aria-disabled', 'false');
      }
    }
  }
  const updatePhotoCounter = updatePhotoCount;

  function renderStagedPhotos() {
    window.petspotrStagedImages = stagedImages;
    if (!stagingContainer) return;
    stagingContainer.replaceChildren();

    stagedImages.forEach((item, index) => {
      let card;
      if (photoTemplate && 'content' in photoTemplate) {
        card = photoTemplate.content.firstElementChild.cloneNode(true);
      } else {
        card = document.createElement('div');
        card.className = 'photo-staging-card glass-card';
        card.innerHTML = `
          <img class="photo-staging-thumb found-preview-image" src="" alt="Staged pet photo">
          <div class="photo-staging-controls form-field">
            <select class="form-control photo-tag-select" aria-label="Select photo angle">
              <option value="primary">Primary / Face</option>
              <option value="coat">Coat Pattern</option>
              <option value="collar">Collar & Tags</option>
            </select>
            <button type="button" class="btn btn-secondary btn-sm btn-remove-photo" aria-label="Remove photo">&times;</button>
          </div>
        `;
      }
      card.dataset.index = String(index);

      const img = card.querySelector('img');
      if (img) {
        img.src = item.previewUrl || '';
        img.alt = `Staged photo ${index + 1} (${item.tag})`;
      }

      const tagSelect = card.querySelector('.photo-tag-select');
      if (tagSelect) {
        tagSelect.value = item.tag;
        tagSelect.addEventListener('change', (e) => {
          stagedImages[index].tag = e.target.value;
        });
      }

      const btnRemove = card.querySelector('.btn-remove-photo');
      if (btnRemove) {
        btnRemove.addEventListener('click', () => {
          removeStagedPhoto(index);
        });
      }

      stagingContainer.appendChild(card);
    });

    updatePhotoCount();
  }
  const renderStagedThumbnails = renderStagedPhotos;

  async function hasExifMetadata(file) {
    if (!file || !file.type || !file.type.startsWith('image/')) return false;
    try {
      const slice = await file.slice(0, 131072).arrayBuffer();
      const bytes = new Uint8Array(slice);
      if (bytes.length < 4) return false;
      if (bytes[0] === 0xFF && bytes[1] === 0xD8) {
        let offset = 2;
        while (offset < bytes.length - 4) {
          if (bytes[offset] !== 0xFF) break;
          const marker = bytes[offset + 1];
          if (marker === 0xE1) {
            if (offset + 10 <= bytes.length) {
              const tag = String.fromCharCode(bytes[offset + 4], bytes[offset + 5], bytes[offset + 6], bytes[offset + 7]);
              if (tag === 'Exif') return true;
            }
          }
          if (marker === 0xDA || marker === 0xD9) break;
          const len = (bytes[offset + 2] << 8) | bytes[offset + 3];
          if (len < 2) break;
          offset += 2 + len;
        }
      }
      if ((bytes[0] === 0x49 && bytes[1] === 0x49) || (bytes[0] === 0x4D && bytes[1] === 0x4D)) {
        return true;
      }
    } catch (_) {}
    return false;
  }

  async function sanitizeFileMetadata(file) {
    if (!file || file.isSanitized) return file;
    const exifPresent = await hasExifMetadata(file);
    if (!exifPresent) return file;

    return new Promise((resolve) => {
      const img = new Image();
      const url = URL.createObjectURL(file);
      img.onload = () => {
        URL.revokeObjectURL(url);
        try {
          const canvas = document.createElement('canvas');
          canvas.width = img.naturalWidth || img.width;
          canvas.height = img.naturalHeight || img.height;
          const ctx = canvas.getContext('2d');
          ctx.drawImage(img, 0, 0);
          canvas.toBlob((blob) => {
            if (blob) {
              const sanitizedFile = new File([blob], file.name, { type: 'image/jpeg' });
              sanitizedFile.isSanitized = true;
              resolve(sanitizedFile);
            } else {
              resolve(file);
            }
          }, 'image/jpeg', 0.92);
        } catch (_) {
          resolve(file);
        }
      };
      img.onerror = () => {
        URL.revokeObjectURL(url);
        resolve(file);
      };
      img.src = url;
    });
  }

  window.addEventListener('petspotr:image-enhanced', async (e) => {
    const { originalFile, enhancedFile, objectUrl } = e.detail;
    const target = stagedImages.find(img => img.file === originalFile || (originalFile && img.file?.name === originalFile.name));
    if (target) {
      target.file = enhancedFile;
      if (objectUrl) {
        target.previewUrl = objectUrl;
        currentImageUrl = objectUrl;
        if (imagePreview) {
          imagePreview.src = objectUrl;
          if (previewContainer) previewContainer.hidden = false;
        }
      }
      target.isEnhanced = true;
      renderStagedThumbnails();
      try {
        const uploaded = await uploadToPresignedUrl(enhancedFile);
        if (uploaded && uploaded.object) {
          target.object = uploaded.object;
          target.url = uploaded.url;
        }
      } catch (err) {
        console.warn('Direct upload warning for enhanced file:', err);
      }
    }
  });

  function removeStagedPhoto(index) {
    stagedImages.splice(index, 1);
    renderStagedPhotos();
    if (stagedImages.length === 0) {
      currentImageUrl = '';
      if (previewContainer) {
        previewContainer.hidden = true;
      }
      if (imagePreview) {
        imagePreview.hidden = true;
        imagePreview.src = '';
      }
      if (spinner) {
        spinner.hidden = true;
      }
      if (extractionStatus) {
        extractionStatus.textContent = '';
      }
    } else {
      const primary = stagedImages.find(img => img.tag === 'primary') || stagedImages[0];
      currentImageUrl = primary.previewUrl || primary.url || '';
      if (imagePreview && currentImageUrl) {
        imagePreview.src = currentImageUrl;
      }
    }
    const photoStatus = document.getElementById('found-photo-status');
    if (photoStatus) {
      photoStatus.textContent = stagedImages.length > 0
        ? `${stagedImages.length} photo(s) attached.`
        : '';
    }
    const photoError = document.getElementById('found-photo-error');
    if (photoError) {
      photoError.textContent = '';
      photoError.hidden = true;
    }
  }

  function readFileDataUrl(file) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result);
      reader.onerror = () => reject(reader.error);
      reader.readAsDataURL(file);
    });
  }

  async function uploadToPresignedUrl(file) {
    const fileToUpload = await sanitizeFileMetadata(file);
    const headers = { 'Content-Type': 'application/json' };
    const csrfToken = window.petspotrIdentity?.getState?.()?.csrfToken ||
      document.cookie.split('; ').find(row => row.startsWith('petspotr_csrf='))?.split('=')[1];
    if (csrfToken) {
      headers['X-CSRF-Token'] = csrfToken;
    }
    try {
      const res = await fetch('/api/v1/uploads/presigned-url', {
        method: 'POST',
        headers,
        body: JSON.stringify({
          fileName: fileToUpload.name,
          contentType: fileToUpload.type || 'image/jpeg'
        })
      });
      if (res.ok) {
        const presigned = await res.json();
        if (presigned.uploadUrl) {
          try {
            await fetch(presigned.uploadUrl, {
              method: 'PUT',
              headers: { 'Content-Type': fileToUpload.type || 'image/jpeg' },
              body: fileToUpload
            });
          } catch (e) {
            console.warn('Direct upload warning:', e);
          }
        }
        return {
          object: presigned.fileName || `images/found-pets/${fileToUpload.name}`,
          url: presigned.publicUrl || ''
        };
      }
    } catch (e) {
      console.warn('Presigned URL fetch warning:', e);
    }
    return {
      object: `images/found-pets/${Date.now()}-${fileToUpload.name}`,
      url: ''
    };
  }

  async function handleFiles(files) {
    const photoError = document.getElementById('found-photo-error');
    const photoStatus = document.getElementById('found-photo-status');
    if (photoError) {
      photoError.textContent = '';
      photoError.hidden = true;
    }

    if (!files || files.length === 0) return;
    if (stagedImages.length >= 3) return;

    const remaining = 3 - stagedImages.length;
    if (remaining <= 0) {
      if (photoError) {
        photoError.textContent = 'Maximum 3 photos allowed.';
        photoError.hidden = false;
      }
      dropzone?.focus();
      return;
    }

    const toProcess = Array.from(files).slice(0, remaining);
    if (files.length > remaining) {
      if (photoError) {
        photoError.textContent = `Only ${remaining} more photo(s) can be added (maximum 3).`;
        photoError.hidden = false;
      }
    }

    for (const file of toProcess) {
      if (!file.type.startsWith('image/')) {
        if (photoError) {
          photoError.textContent = 'Please select a valid image file.';
          photoError.hidden = false;
        }
        continue;
      }
      if (file.size > 10 * 1024 * 1024) {
        if (photoError) {
          photoError.textContent = 'Photo file size exceeds 10MB limit.';
          photoError.hidden = false;
        }
        continue;
      }

      if (photoStatus) {
        photoStatus.textContent = `Uploading ${file.name}...`;
      }

      try {
        const previewUrl = await readFileDataUrl(file);
        const uploaded = await uploadToPresignedUrl(file);

        const existingTags = stagedImages.map(img => img.tag);
        let defaultTag = 'primary';
        if (existingTags.includes('primary')) {
          defaultTag = !existingTags.includes('coat') ? 'coat' : 'collar';
        }

        stagedImages.push({
          file,
          object: uploaded.object,
          url: uploaded.url,
          previewUrl: previewUrl,
          tag: defaultTag
        });

        renderStagedPhotos();
        if (photoStatus) {
          photoStatus.textContent = `Added ${file.name} (${stagedImages.length} / 3 photos).`;
        }

        // Trigger AI Feature Auto-Extraction on primary/first uploaded image of batch
        if (file === toProcess[0]) {
          currentImageUrl = previewUrl;
          if (imagePreview && previewContainer) {
            imagePreview.src = currentImageUrl;
            previewContainer.hidden = false;
          }
          await extractAIFeatures(currentImageUrl);
        }
      } catch (err) {
        console.error('File staging error:', err);
        if (photoError) {
          photoError.textContent = 'Failed to process photo upload.';
          photoError.hidden = false;
        }
      }
    }
  }

  // Drag & Drop Photo Upload
  if (dropzone && photoInput) {
    dropzone.addEventListener('click', () => {
      if (stagedImages.length >= 3) return;
      photoInput.click();
    });
    dropzone.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        if (stagedImages.length >= 3) return;
        photoInput.click();
      }
    });

    dropzone.addEventListener('dragover', (e) => {
      e.preventDefault();
      if (stagedImages.length >= 3) return;
      dropzone.classList.add('is-dragging');
    });

    dropzone.addEventListener('dragleave', () => {
      dropzone.classList.remove('is-dragging');
    });

    dropzone.addEventListener('drop', (e) => {
      e.preventDefault();
      dropzone.classList.remove('is-dragging');
      if (stagedImages.length >= 3) return;
      if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
        handleFiles(e.dataTransfer.files);
      }
    });

    photoInput.addEventListener('change', (e) => {
      if (stagedImages.length >= 3) {
        setTimeout(() => { photoInput.value = ''; }, 0);
        return;
      }
      if (e.target.files && e.target.files.length > 0) {
        handleFiles(e.target.files);
        setTimeout(() => { photoInput.value = ''; }, 0);
      }
    });

    updatePhotoCount();
  }

  async function extractAIFeatures(imageUrl) {
    const extractionID = ++extractionSequence;
    clearExtractedTraits();
    if (extractionStatus) extractionStatus.textContent = 'Analyzing the selected image.';
    if (spinner) spinner.hidden = false;

    try {
      const resp = await fetch('/api/v1/found-pets/extract-features', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ imageUrl: imageUrl })
      });

      if (extractionID !== extractionSequence) return;
      if (!resp.ok) throw new Error(`feature extraction failed with status ${resp.status}`);

      const traits = await resp.json();
      if (extractionID !== extractionSequence) return;
      // Update Form Fields
      if (inputSpecies && traits.species) inputSpecies.value = traits.species;
      if (inputBreed && traits.breed) inputBreed.value = traits.breed;
      if (inputPrimaryColor && traits.primaryColor) inputPrimaryColor.value = traits.primaryColor;
      if (inputSecondaryColor && traits.secondaryColor) inputSecondaryColor.value = traits.secondaryColor || '';
      currentDistinctiveMarkings = Array.isArray(traits.distinctiveMarkings)
        ? traits.distinctiveMarkings.filter((marking) => typeof marking === 'string')
        : [];

      // Update AI Chips
      if (chipSpecies) chipSpecies.textContent = `Species: ${traits.species || 'Unknown'}`;
      if (chipBreed) chipBreed.textContent = `Breed: ${traits.breed || 'Mixed'}`;
      if (chipColor) {
        const colors = [traits.primaryColor, traits.secondaryColor].filter(Boolean).join(' / ');
        chipColor.textContent = `Colors: ${colors || 'N/A'}`;
      }
      if (extractionStatus) extractionStatus.textContent = 'Image analysis complete. Review the detected traits below.';
    } catch (err) {
      if (extractionID !== extractionSequence) return;
      console.error('AI extraction error:', err);
      if (extractionStatus) {
        extractionStatus.textContent = 'Image analysis failed. Choose the pet traits manually or try another image.';
      }
    } finally {
      if (spinner && extractionID === extractionSequence) spinner.hidden = true;
    }
  }

  async function handleOfflineSubmission() {
    const stagedPhotos = stagedImages.map(img => ({
      fileName: img.file ? img.file.name : (img.fileName || 'photo.jpg'),
      contentType: img.file ? (img.file.type || 'image/jpeg') : (img.contentType || 'image/jpeg'),
      tag: img.tag || 'primary',
      blob: img.file
    }));

    const payload = {
      petId: (pendingSubmission && pendingSubmission.petId) || `found-${crypto.randomUUID()}`,
      species: inputSpecies?.value || document.getElementById('foundSpecies')?.value || 'Dog',
      breed: inputBreed?.value || document.getElementById('foundBreed')?.value || '',
      primaryColor: inputPrimaryColor?.value || document.getElementById('foundPrimaryColor')?.value || '',
      secondaryColor: inputSecondaryColor?.value || document.getElementById('foundSecondaryColor')?.value || '',
      distinctiveMarkings: currentDistinctiveMarkings,
      custodyStatus: document.getElementById('custodyStatus')?.value || 'Finder Home',
      microchipId: foundMicrochipInput ? foundMicrochipInput.value.trim() : '',
      location: document.getElementById('foundLocation')?.value || '',
      finderEmail: document.getElementById('finderEmail')?.value || '',
      foundAt: (pendingSubmission && pendingSubmission.foundAt) || new Date().toISOString()
    };

    if (window.PetSpotROutbox) {
      await window.PetSpotROutbox.enqueueReport({ type: 'found', payload, photos: stagedPhotos });
      window.PetSpotROutbox.showToast('Report saved offline. It will submit automatically when you reconnect.');
    }

    pendingSubmission = null;
    if (form) form.reset();
    stagedImages = [];
    renderStagedPhotos();
    currentImageUrl = '';
    if (previewContainer) previewContainer.hidden = true;
    if (imagePreview) {
      imagePreview.hidden = true;
      imagePreview.src = '';
    }
    if (spinner) spinner.hidden = true;
    if (extractionStatus) extractionStatus.textContent = '';
    const photoStatus = document.getElementById('found-photo-status');
    if (photoStatus) photoStatus.textContent = '';
    const photoError = document.getElementById('found-photo-error');
    if (photoError) {
      photoError.textContent = '';
      photoError.hidden = true;
    }
    if (foundMicrochipFeedback) {
      foundMicrochipFeedback.textContent = '';
      foundMicrochipFeedback.className = 'microchip-feedback-msg';
    }
    clearExtractedTraits();
    clearFormError();
    clearFieldError('foundLocation', 'found-location-error');
    clearFieldError('finderEmail', 'finder-email-error');
  }

  // Form Submission
  if (form) {
    let submissionInFlight = false;
    const btnSubmitFound = document.getElementById('btn-submit-found');

    function setSubmissionBusy(busy) {
      if (form) form.setAttribute('aria-busy', String(busy));
      if (btnSubmitFound) {
        btnSubmitFound.setAttribute('aria-disabled', String(busy));
      }
    }

    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (submissionInFlight) return;
      submissionInFlight = true;
      setSubmissionBusy(true);

      try {
        const location = document.getElementById('foundLocation')?.value || '';
        const finderEmail = document.getElementById('finderEmail')?.value || '';

        clearFormError();
        const locationInput = document.getElementById('foundLocation');
        const emailInput = document.getElementById('finderEmail');

        let hasError = false;
        if (!location.trim()) {
          showFieldError('foundLocation', 'found-location-error', 'Please enter found location.');
          hasError = true;
        } else {
          clearFieldError('foundLocation', 'found-location-error');
        }

        if (!finderEmail.trim()) {
          showFieldError('finderEmail', 'finder-email-error', 'Please enter finder contact email.');
          if (!hasError) {
            emailInput?.focus();
          }
          hasError = true;
        } else {
          clearFieldError('finderEmail', 'finder-email-error');
        }

        if (hasError) {
          showFormError('Please enter found location and finder contact email.');
          return;
        }

        if (!navigator.onLine) {
          await handleOfflineSubmission();
          return;
        }

        let identityState = null;
        if (window.petspotrIdentity) {
          try {
            identityState = await window.petspotrIdentity.requireSession();
          } catch (error) {
            if (error.code === 'identity-required') {
              window.petspotrIdentity.focusSignIn();
              const idError = document.getElementById('identity-error');
              if (idError) {
                idError.textContent = 'Sign in with Google before submitting your report.';
                idError.hidden = false;
              }
              showFormError('Sign in with Google before submitting your report.');
              return;
            }
            console.error('Identity error:', error);
            const idError = document.getElementById('identity-error');
            if (idError) {
              idError.textContent = 'Identity services are temporarily unavailable. Please try again.';
              idError.hidden = false;
            }
            showFormError('Identity services are temporarily unavailable. Please try again.');
            return;
          }
        }
        if (!pendingSubmission) {
          pendingSubmission = {
            petId: `found-${crypto.randomUUID()}`,
            foundAt: new Date().toISOString()
          };
        }

        const primaryImg = stagedImages.find(img => img.tag === 'primary' || img.tag === 'face') || stagedImages[0];
        const primaryObject = primaryImg ? primaryImg.object : '';
        const legacyImageUrl = (primaryImg && primaryImg.url) ? primaryImg.url : (currentImageUrl || 'https://storage.petspotr.io/found-sample.jpg');

        const payload = {
          ...pendingSubmission,
          imageUrl: legacyImageUrl,
          imageObject: primaryObject,
          images: stagedImages.map(img => ({ object: img.object, tag: img.tag || 'primary' })),
          location: location.trim(),
          finderEmail: finderEmail.trim(),
          species: inputSpecies?.value || 'Dog',
          breed: inputBreed?.value || '',
          primaryColor: inputPrimaryColor?.value || '',
          secondaryColor: inputSecondaryColor?.value || '',
          distinctiveMarkings: currentDistinctiveMarkings,
          custodyStatus: document.getElementById('custodyStatus')?.value || 'Finder Home',
          microchipId: foundMicrochipInput ? foundMicrochipInput.value.trim() : ''
        };

        try {
          const headers = { 'Content-Type': 'application/json' };
          if (identityState?.enabled) {
            headers['X-CSRF-Token'] = identityState.csrfToken;
          }
          const resp = await fetch('/api/v1/found-pets', {
            method: 'POST',
            headers,
            body: JSON.stringify(payload)
          });

          if (resp.ok) {
            pendingSubmission = null;
            const modal = document.getElementById('found-success-modal');
            if (modal) {
              modal.hidden = false;
              document.getElementById('found-success-return')?.focus();
            }
          } else {
            if (resp.status < 500) pendingSubmission = null;
            showFormError('Failed to submit found pet report.');
          }
        } catch (err) {
          console.error('Submission error:', err);
          if (!navigator.onLine || err instanceof TypeError || (err.message && err.message.toLowerCase().includes('failed to fetch'))) {
            await handleOfflineSubmission();
            return;
          }
          showFormError('Network error submitting found pet report.');
        }
      } finally {
        submissionInFlight = false;
        setSubmissionBusy(false);
      }
    });
  }
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      const modal = document.getElementById('found-success-modal');
      if (modal && !modal.hidden) {
        modal.hidden = true;
        document.getElementById('btn-submit-found')?.focus();
      }
    }
  });
});