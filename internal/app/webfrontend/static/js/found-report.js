// Client-side controller for Found Pet report form with AI auto-extraction
document.addEventListener('DOMContentLoaded', () => {
  const dropzone = document.getElementById('found-dropzone');
  const photoInput = document.getElementById('foundPhotoInput');
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

  function clearExtractedTraits() {
    currentDistinctiveMarkings = [];
    if (inputSpecies) inputSpecies.value = '';
    if (inputBreed) inputBreed.value = '';
    if (inputPrimaryColor) inputPrimaryColor.value = '';
    if (inputSecondaryColor) inputSecondaryColor.value = '';
    if (chipSpecies) chipSpecies.textContent = 'Species: Not analyzed';
    if (chipBreed) chipBreed.textContent = 'Breed: Not analyzed';
    if (chipColor) chipColor.textContent = 'Colors: Not analyzed';
  }

  if (dropzone && photoInput) {
    dropzone.addEventListener('click', () => photoInput.click());
    dropzone.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        photoInput.click();
      }
    });

    dropzone.addEventListener('dragover', (e) => {
      e.preventDefault();
      dropzone.classList.add('is-dragging');
    });

    dropzone.addEventListener('dragleave', () => {
      dropzone.classList.remove('is-dragging');
    });

    dropzone.addEventListener('drop', (e) => {
      e.preventDefault();
      dropzone.classList.remove('is-dragging');
      if (e.dataTransfer.files && e.dataTransfer.files[0]) {
        handleFile(e.dataTransfer.files[0]);
      }
    });

    photoInput.addEventListener('change', (e) => {
      if (e.target.files && e.target.files[0]) {
        handleFile(e.target.files[0]);
      }
    });
  }

  function handleFile(file) {
    const photoError = document.getElementById('found-photo-error');
    const photoStatus = document.getElementById('found-photo-status');
    if (photoError) {
      photoError.textContent = '';
      photoError.hidden = true;
    }
    if (!file.type.startsWith('image/')) {
      if (photoError) {
        photoError.textContent = 'Please select a valid image file.';
        photoError.hidden = false;
      }
      dropzone?.focus();
      return;
    }
    if (photoStatus) {
      photoStatus.textContent = `Photo selected: ${file.name}`;
    }

    const reader = new FileReader();
    reader.onload = async (e) => {
      currentImageUrl = e.target.result;
      if (imagePreview && previewContainer) {
        imagePreview.src = currentImageUrl;
        previewContainer.hidden = false;
      }

      // Trigger AI Feature Auto-Extraction
      await extractAIFeatures(currentImageUrl);
    };
    reader.readAsDataURL(file);
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

  // Form Submission
  if (form) {
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const location = document.getElementById('foundLocation')?.value || '';
      const finderEmail = document.getElementById('finderEmail')?.value || '';

      clearFormError();
      const locationInput = document.getElementById('foundLocation');
      const emailInput = document.getElementById('finderEmail');

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
      if (!pendingSubmission) {
        pendingSubmission = {
          petId: `found-${crypto.randomUUID()}`,
          foundAt: new Date().toISOString()
        };
      }

      const payload = {
        ...pendingSubmission,
        imageUrl: currentImageUrl || 'https://storage.petspotr.io/found-sample.jpg',
        location: location.trim(),
        finderEmail: finderEmail.trim(),
        species: inputSpecies?.value || 'Dog',
        breed: inputBreed?.value || '',
        primaryColor: inputPrimaryColor?.value || '',
        secondaryColor: inputSecondaryColor?.value || '',
        distinctiveMarkings: currentDistinctiveMarkings,
        custodyStatus: document.getElementById('custodyStatus')?.value || 'Finder Home'
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
        showFormError('Network error submitting found pet report.');
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