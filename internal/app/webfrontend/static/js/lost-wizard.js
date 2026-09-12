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
  const imagePreview = document.getElementById('imagePreview');
  const previewContainer = document.getElementById('preview-container');
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
    const photoError = document.getElementById('photo-error');
    const photoStatus = document.getElementById('lost-photo-status');
    if (photoError) {
      photoError.textContent = '';
      photoError.hidden = true;
    }
    if (!file.type.startsWith('image/')) {
      if (photoError) {
        photoError.textContent = 'Please select an image file.';
        photoError.hidden = false;
      }
      dropzone?.focus();
      return;
    }
    if (photoStatus) {
      photoStatus.textContent = `Photo selected: ${file.name}`;
    }
    const reader = new FileReader();
    reader.onload = (e) => {
      if (imagePreview && previewContainer) {
        imagePreview.src = e.target.result;
        previewContainer.hidden = false;
      }
    };
    reader.readAsDataURL(file);
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

        const payload = {
          ...pendingSubmission,
          petName: document.getElementById('petName')?.value || '',
          species: document.getElementById('species')?.value || 'Dog',
          breed: document.getElementById('breed')?.value || '',
          primaryColor: document.getElementById('primaryColor')?.value || '',
          description: document.getElementById('description')?.value || '',
          location: document.getElementById('location')?.value || '',
          reporterEmail: reporterEmail.value.trim(),
          phone: document.getElementById('phone')?.value || ''
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