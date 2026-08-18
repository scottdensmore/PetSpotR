// Client-side controller for Lost Pet multi-step wizard
document.addEventListener('DOMContentLoaded', () => {
  let currentStep = 1;
  let pendingSubmission = null;
  const totalSteps = 4;

  const btnPrev = document.getElementById('btn-prev');
  const btnNext = document.getElementById('btn-next');
  const btnSubmit = document.getElementById('btn-submit');
  const form = document.getElementById('lost-pet-form');
  const dropzone = document.getElementById('dropzone');
  const photoInput = document.getElementById('photoInput');
  const imagePreview = document.getElementById('imagePreview');
  const previewContainer = document.getElementById('preview-container');

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
        alert('Please enter a pet name.');
        return false;
      }
    } else if (step === 3) {
      const location = document.getElementById('location');
      if (!location || !location.value.trim()) {
        alert('Please enter the last seen location.');
        return false;
      }
    }
    return true;
  }

  // Drag & Drop Photo Upload
  if (dropzone && photoInput) {
    dropzone.addEventListener('click', () => photoInput.click());

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
    if (!file.type.startsWith('image/')) {
      alert('Please select an image file.');
      return;
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
      const reporterEmail = document.getElementById('reporterEmail');

      let identityState = null;
      if (window.petspotrIdentity) {
        try {
          identityState = await window.petspotrIdentity.requireSession();
        } catch (error) {
          if (error.code === 'identity-required') {
            window.petspotrIdentity.focusSignIn();
            alert('Sign in with Google before submitting your report.');
            return;
          }
          console.error('Identity error:', error);
          alert('Identity services are temporarily unavailable. Please try again.');
          return;
        }
      }
      if (!reporterEmail || !reporterEmail.value.trim()) {
        alert('Please enter a contact email address.');
        return;
      }

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
          if (modal) modal.hidden = false;
        } else {
          if (resp.status < 500) pendingSubmission = null;
          alert('Failed to submit report. Please check input fields.');
        }
      } catch (err) {
        console.error('Submission error:', err);
        alert('Network error submitting report.');
      }
    });
  }
});
