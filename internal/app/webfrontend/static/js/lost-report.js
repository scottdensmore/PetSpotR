// Client-side controller for Lost Pet report form with offline outbox support
document.addEventListener('DOMContentLoaded', () => {
  const form = document.getElementById('lost-pet-form');
  if (!form) return;

  // Prevent duplicate event binding if lost-wizard.js has already bound to this form
  if (form.dataset.wizardBound === 'true') {
    return;
  }
  form.dataset.lostReportBound = 'true';

  let stagedImages = [];

  async function handleOfflineSubmission() {
    const stagedPhotos = stagedImages.map(img => ({
      fileName: img.file ? img.file.name : (img.fileName || 'photo.jpg'),
      contentType: img.file ? (img.file.type || 'image/jpeg') : (img.contentType || 'image/jpeg'),
      tag: img.tag || 'primary',
      blob: img.file
    }));

    const payload = {
      petName: form.petName?.value || document.getElementById('petName')?.value || '',
      species: form.species?.value || document.getElementById('species')?.value || 'Dog',
      breed: form.breed?.value || document.getElementById('breed')?.value || '',
      primaryColor: form.primaryColor?.value || document.getElementById('primaryColor')?.value || '',
      description: form.description?.value || document.getElementById('description')?.value || '',
      reporterEmail: form.reporterEmail?.value || document.getElementById('reporterEmail')?.value || '',
      phone: form.phone?.value || document.getElementById('phone')?.value || '',
      location: form.location?.value || document.getElementById('location')?.value || '',
      reportedAt: new Date().toISOString()
    };

    if (window.PetSpotROutbox) {
      await window.PetSpotROutbox.enqueueReport({
        type: 'lost',
        payload,
        photos: stagedPhotos
      });
      window.PetSpotROutbox.showToast('Report saved offline. It will submit automatically when you reconnect.');
    }
    form.reset();
    stagedImages = [];
  }

  form.addEventListener('submit', async (e) => {
    e.preventDefault();

    const reporterEmail = form.reporterEmail?.value || document.getElementById('reporterEmail')?.value || '';
    if (!reporterEmail.trim()) {
      const errorEl = document.getElementById('lost-form-error') || document.getElementById('reporterEmail-error');
      if (errorEl) {
        errorEl.textContent = 'Please enter a contact email address.';
        errorEl.hidden = false;
      }
      return;
    }

    if (!navigator.onLine) {
      await handleOfflineSubmission();
      return;
    }

    try {
      const payload = {
        petName: form.petName?.value || document.getElementById('petName')?.value || '',
        species: form.species?.value || document.getElementById('species')?.value || 'Dog',
        breed: form.breed?.value || document.getElementById('breed')?.value || '',
        primaryColor: form.primaryColor?.value || document.getElementById('primaryColor')?.value || '',
        description: form.description?.value || document.getElementById('description')?.value || '',
        location: form.location?.value || document.getElementById('location')?.value || '',
        reporterEmail: reporterEmail.trim(),
        phone: form.phone?.value || document.getElementById('phone')?.value || '',
        reportedAt: new Date().toISOString()
      };

      const resp = await fetch('/api/v1/lost-pets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      if (resp.ok) {
        form.reset();
        stagedImages = [];
      } else {
        const errorEl = document.getElementById('lost-form-error');
        if (errorEl) {
          errorEl.textContent = 'Failed to submit report. Please check input fields.';
          errorEl.hidden = false;
        }
      }
    } catch (err) {
      if (!navigator.onLine || err instanceof TypeError || (err.message && err.message.toLowerCase().includes('failed to fetch'))) {
        await handleOfflineSubmission();
        return;
      }
      console.error('Submission error:', err);
      const errorEl = document.getElementById('lost-form-error');
      if (errorEl) {
        errorEl.textContent = 'Network error submitting report.';
        errorEl.hidden = false;
      }
    }
  });
});
