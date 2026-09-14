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
        reporterEmail: form.reporterEmail?.value || document.getElementById('reporterEmail')?.value || '',
        phone: form.phone?.value || document.getElementById('phone')?.value || '',
        reportedAt: new Date().toISOString()
      };

      const resp = await fetch('/api/v1/lost-pets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      if (!resp.ok) {
        throw new Error(`Report submission failed with status ${resp.status}`);
      }
      form.reset();
      stagedImages = [];
    } catch (err) {
      console.warn('Network error submitting lost pet report, falling back to outbox:', err);
      await handleOfflineSubmission();
    }
  });
});
