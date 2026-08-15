// Client-side controller for Found Pet report form with AI auto-extraction
document.addEventListener('DOMContentLoaded', () => {
  const dropzone = document.getElementById('found-dropzone');
  const photoInput = document.getElementById('foundPhotoInput');
  const previewContainer = document.getElementById('found-preview-container');
  const imagePreview = document.getElementById('foundImagePreview');
  const spinner = document.getElementById('ai-spinner');
  const form = document.getElementById('found-pet-form');

  const chipSpecies = document.getElementById('chip-species');
  const chipBreed = document.getElementById('chip-breed');
  const chipColor = document.getElementById('chip-color');

  const inputSpecies = document.getElementById('foundSpecies');
  const inputBreed = document.getElementById('foundBreed');
  const inputPrimaryColor = document.getElementById('foundPrimaryColor');
  const inputSecondaryColor = document.getElementById('foundSecondaryColor');

  let currentImageUrl = '';

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
      alert('Please select a valid image file.');
      return;
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
    if (spinner) spinner.hidden = false;

    try {
      const resp = await fetch('/api/v1/found-pets/extract-features', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ imageUrl: imageUrl })
      });

      if (resp.ok) {
        const traits = await resp.json();
        // Update Form Fields
        if (inputSpecies && traits.species) inputSpecies.value = traits.species;
        if (inputBreed && traits.breed) inputBreed.value = traits.breed;
        if (inputPrimaryColor && traits.primaryColor) inputPrimaryColor.value = traits.primaryColor;
        if (inputSecondaryColor && traits.secondaryColor) inputSecondaryColor.value = traits.secondaryColor || '';

        // Update AI Chips
        if (chipSpecies) chipSpecies.textContent = `Species: ${traits.species || 'Unknown'}`;
        if (chipBreed) chipBreed.textContent = `Breed: ${traits.breed || 'Mixed'}`;
        if (chipColor) chipColor.textContent = `Color: ${traits.primaryColor || 'N/A'}`;
      }
    } catch (err) {
      console.error('AI extraction error:', err);
    } finally {
      if (spinner) spinner.hidden = true;
    }
  }

  // Form Submission
  if (form) {
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      const location = document.getElementById('foundLocation')?.value || '';
      const finderEmail = document.getElementById('finderEmail')?.value || '';

      if (!location.trim() || !finderEmail.trim()) {
        alert('Please enter found location and finder contact email.');
        return;
      }

      const payload = {
        imageUrl: currentImageUrl || 'https://storage.petspotr.io/found-sample.jpg',
        location: location.trim(),
        finderEmail: finderEmail.trim(),
        species: inputSpecies?.value || 'Dog',
        breed: inputBreed?.value || '',
        primaryColor: inputPrimaryColor?.value || '',
        custodyStatus: document.getElementById('custodyStatus')?.value || 'Finder Home'
      };

      try {
        const resp = await fetch('/api/v1/found-pets', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });

        if (resp.ok) {
          const modal = document.getElementById('found-success-modal');
          if (modal) modal.hidden = false;
        } else {
          alert('Failed to submit found pet report.');
        }
      } catch (err) {
        console.error('Submission error:', err);
        alert('Network error submitting found pet report.');
      }
    });
  }
});
