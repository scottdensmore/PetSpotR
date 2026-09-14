// Client-side controller for Lost Pet multi-step wizard
document.addEventListener('DOMContentLoaded', () => {
  let currentStep = 1;
  let pendingSubmission = null;
  let submissionInFlight = false;
  const totalSteps = 4;

  const btnPrev = document.getElementById('btn-prev');
  const btnNext = document.getElementById('btn-next');
  const btnSubmit = document.getElementById('btn-submit');
  const form = document.getElementById('lost-pet-form');
  const dropzone = document.getElementById('dropzone');
  const photoInput = document.getElementById('photoInput');
  const photoCountBadge = document.getElementById('photo-count-badge');
  const stagingContainer = document.getElementById('staging-container');
  const photoTemplate = document.getElementById('staged-photo-template');
  const submissionStatus = document.getElementById('lost-report-status');

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
    const formError = document.getElementById('lost-form-error');
    if (formError) {
      formError.textContent = message;
      formError.hidden = false;
    }
  }

  function clearFormError() {
    const formError = document.getElementById('lost-form-error');
    if (formError) {
      formError.textContent = '';
      formError.hidden = true;
    }
  }

  document.getElementById('petName')?.addEventListener('input', () => clearFieldError('petName', 'petName-error'));
  document.getElementById('location')?.addEventListener('input', () => clearFieldError('location', 'location-error'));
  document.getElementById('reporterEmail')?.addEventListener('input', () => clearFieldError('reporterEmail', 'reporterEmail-error'));

  function setSubmissionBusy(busy) {
    if (form) form.setAttribute('aria-busy', String(busy));
    if (btnSubmit) {
      btnSubmit.setAttribute('aria-disabled', String(busy));
      btnSubmit.textContent = busy ? 'Submitting report...' : 'Submit Report';
    }
    if (submissionStatus) {
      submissionStatus.textContent = busy ? 'Submitting your lost pet report...' : '';
      submissionStatus.hidden = !busy;
    }
  }

  // Step Switching
  function showStep(step) {
    for (let i = 1; i <= totalSteps; i++) {
      const stepEl = document.getElementById(`wizard-step-${i}`);
      const badgeEl = document.getElementById(`step-badge-${i}`);
      if (stepEl) {
        stepEl.hidden = i !== step;
      }
      if (badgeEl) {
        badgeEl.classList.toggle('active', i === step);
        badgeEl.classList.toggle('complete', i < step);
      }
    }

    if (btnPrev) btnPrev.classList.toggle('is-invisible', step === 1);
    if (btnNext) btnNext.hidden = step === totalSteps;
    if (btnSubmit) btnSubmit.hidden = step !== totalSteps;
  }

  if (btnNext) {
    btnNext.addEventListener('click', () => {
      if (validateCurrentStep(currentStep)) {
        currentStep = Math.min(totalSteps, currentStep + 1);
        showStep(currentStep);
      }
    });
  }

  if (btnPrev) {
    btnPrev.addEventListener('click', () => {
      currentStep = Math.max(1, currentStep - 1);
      showStep(currentStep);
    });
  }

  function validateCurrentStep(step) {
    if (step === 1) {
      const petName = document.getElementById('petName');
      if (!petName || !petName.value.trim()) {
        showFieldError('petName', 'petName-error', 'Please enter a pet name.');
        return false;
      }
      clearFieldError('petName', 'petName-error');
    } else if (step === 3) {
      const location = document.getElementById('location');
      if (!location || !location.value.trim()) {
        showFieldError('location', 'location-error', 'Please enter the last seen location.');
        return false;
      }
      clearFieldError('location', 'location-error');
    }
    return true;
  }

  // Multi-Photo Staging & Upload
  let stagedImages = [];

  function updatePhotoCounter() {
    if (photoCountBadge) {
      photoCountBadge.textContent = `${stagedImages.length} / 3 photos added`;
    }
    if (stagingContainer) {
      stagingContainer.hidden = stagedImages.length === 0;
    }
  }

  function renderStagedPhotos() {
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
          <img class="photo-staging-thumb lost-preview-image" src="" alt="Staged pet photo">
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

    updatePhotoCounter();
  }

  function removeStagedPhoto(index) {
    stagedImages.splice(index, 1);
    renderStagedPhotos();
    const photoStatus = document.getElementById('lost-photo-status');
    if (photoStatus) {
      photoStatus.textContent = stagedImages.length > 0
        ? `${stagedImages.length} photo(s) attached.`
        : '';
    }
    const photoError = document.getElementById('photo-error');
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
          fileName: file.name,
          contentType: file.type || 'image/jpeg'
        })
      });
      if (res.ok) {
        const presigned = await res.json();
        if (presigned.uploadUrl) {
          try {
            await fetch(presigned.uploadUrl, {
              method: 'PUT',
              headers: { 'Content-Type': file.type || 'image/jpeg' },
              body: file
            });
          } catch (e) {
            console.warn('Direct upload warning:', e);
          }
        }
        return {
          object: presigned.fileName || `images/lost-pets/${file.name}`,
          url: presigned.publicUrl || ''
        };
      }
    } catch (e) {
      console.warn('Presigned URL fetch warning:', e);
    }
    return {
      object: `images/lost-pets/${Date.now()}-${file.name}`,
      url: ''
    };
  }

  async function handleFiles(files) {
    const photoError = document.getElementById('photo-error');
    const photoStatus = document.getElementById('lost-photo-status');
    if (photoError) {
      photoError.textContent = '';
      photoError.hidden = true;
    }

    if (!files || files.length === 0) return;

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
          photoError.textContent = 'Please select an image file.';
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
          object: uploaded.object,
          url: uploaded.url,
          previewUrl: previewUrl,
          tag: defaultTag
        });

        renderStagedPhotos();
        if (photoStatus) {
          photoStatus.textContent = `Added ${file.name} (${stagedImages.length} / 3 photos).`;
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
      if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
        handleFiles(e.dataTransfer.files);
      }
    });

    photoInput.addEventListener('change', (e) => {
      if (e.target.files && e.target.files.length > 0) {
        handleFiles(e.target.files);
        photoInput.value = '';
      }
    });
  }

  // Form Submission AJAX
  if (form) {
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (submissionInFlight) return;
      submissionInFlight = true;
      setSubmissionBusy(true);

      try {
        const reporterEmail = document.getElementById('reporterEmail');

        clearFormError();
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
        if (!reporterEmail || !reporterEmail.value.trim()) {
          showFieldError('reporterEmail', 'reporterEmail-error', 'Please enter a contact email address.');
          return;
        }
        clearFieldError('reporterEmail', 'reporterEmail-error');

        if (!pendingSubmission) {
          pendingSubmission = {
            petId: `lost-${crypto.randomUUID()}`,
            reportedAt: new Date().toISOString()
          };
        }

        const primaryImg = stagedImages.find(img => img.tag === 'primary' || img.tag === 'face') || stagedImages[0];
        const payload = {
          ...pendingSubmission,
          petName: document.getElementById('petName')?.value || '',
          species: document.getElementById('species')?.value || 'Dog',
          breed: document.getElementById('breed')?.value || '',
          primaryColor: document.getElementById('primaryColor')?.value || '',
          description: document.getElementById('description')?.value || '',
          location: document.getElementById('location')?.value || '',
          reporterEmail: reporterEmail.value.trim(),
          phone: document.getElementById('phone')?.value || '',
          images: stagedImages.map(img => ({ object: img.object, tag: img.tag || 'primary' })),
          imageObject: primaryImg ? primaryImg.object : ''
        };

        try {
          const headers = { 'Content-Type': 'application/json' };
          if (identityState?.enabled) {
            headers['X-CSRF-Token'] = identityState.csrfToken;
          }
          const resp = await fetch('/api/v1/lost-pets', {
            method: 'POST',
            headers,
            body: JSON.stringify(payload)
          });

          if (resp.ok) {
            pendingSubmission = null;
            const modal = document.getElementById('success-modal');
            if (modal) {
              modal.hidden = false;
              document.getElementById('lost-success-return')?.focus();
            }
          } else {
            if (resp.status < 500) pendingSubmission = null;
            showFormError('Failed to submit report. Please check input fields.');
          }
        } catch (err) {
          console.error('Submission error:', err);
          showFormError('Network error submitting report.');
        }
      } finally {
        submissionInFlight = false;
        setSubmissionBusy(false);
      }
    });
  }
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      const modal = document.getElementById('success-modal');
      if (modal && !modal.hidden) {
        modal.hidden = true;
        btnSubmit?.focus();
      }
    }
  });
});