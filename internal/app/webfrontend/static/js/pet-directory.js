// Client-side controller for Public Pet Directory and Interactive Map
document.addEventListener('DOMContentLoaded', () => {
  const filterForm = document.getElementById('pet-filter-form');
  const speciesSelect = document.getElementById('filter-species');
  const statusSelect = document.getElementById('filter-status');
  const queryInput = document.getElementById('filter-query');
  const latInput = document.getElementById('filter-lat');
  const lngInput = document.getElementById('filter-lng');
  const radiusSelect = document.getElementById('filter-radius');
  const geoBtn = document.getElementById('btn-geolocation');
  const geoBtnLabel = document.getElementById('geo-btn-label');
  const btnGrid = document.getElementById('btn-view-grid');
  const btnMap = document.getElementById('btn-view-map');
  const petsGrid = document.getElementById('pets-grid');
  const petsMapContainer = document.getElementById('pets-map-container');
  const mapStatusOverlay = document.getElementById('map-status-overlay');

  let mapInstance = null;
  let centerMarker = null;
  let proximityCircle = null;

  // HTML entity escaper for safe DOM insertion
  function escapeHTML(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  // Non-blocking notification helper (visible in both Grid and Map views)
  function showStatusMessage(message, isError = false) {
    if (mapStatusOverlay) {
      mapStatusOverlay.textContent = message;
      mapStatusOverlay.style.display = 'block';
    }

    let toast = document.getElementById('directory-toast');
    if (!toast) {
      toast = document.createElement('div');
      toast.id = 'directory-toast';
      toast.setAttribute('role', 'alert');
      toast.setAttribute('aria-live', 'polite');
      toast.style.cssText =
        'position:fixed;bottom:24px;right:24px;z-index:9999;max-width:380px;padding:12px 18px;border-radius:8px;font-size:0.9rem;box-shadow:0 8px 24px rgba(0,0,0,0.3);transition:opacity 0.3s ease;backdrop-filter:blur(10px);';
      document.body.appendChild(toast);
    }

    toast.style.background = isError ? 'rgba(239, 68, 68, 0.95)' : 'rgba(31, 41, 55, 0.95)';
    toast.style.color = '#ffffff';
    toast.style.border = isError ? '1px solid #dc2626' : '1px solid var(--border-glass, #374151)';
    toast.textContent = message;
    toast.style.opacity = '1';
    toast.style.display = 'block';

    clearTimeout(toast._timeout);
    toast._timeout = setTimeout(() => {
      toast.style.opacity = '0';
      setTimeout(() => {
        if (toast.style.opacity === '0') {
          toast.style.display = 'none';
        }
      }, 300);
    }, 5000);
  }

  // Update or create center marker & proximity circle overlay
  function updateProximityOverlay(lat, lng, radiusMiles) {
    if (!mapInstance || typeof L === 'undefined') return;

    const radiusMeters = (radiusMiles || 10) * 1609.34;
    const centerIcon = L.divIcon({
      className: 'custom-map-pin',
      html: '<div class="pin-center" title="Search Center"></div>',
      iconSize: [20, 20],
      iconAnchor: [10, 10]
    });

    if (centerMarker) {
      centerMarker.setLatLng([lat, lng]);
    } else {
      centerMarker = L.marker([lat, lng], {
        icon: centerIcon,
        title: 'Search Center',
        zIndexOffset: 1000
      }).addTo(mapInstance);
    }

    if (proximityCircle) {
      proximityCircle.setLatLng([lat, lng]);
      proximityCircle.setRadius(radiusMeters);
    } else {
      proximityCircle = L.circle([lat, lng], {
        radius: radiusMeters,
        color: '#3b82f6',
        fillColor: '#3b82f6',
        fillOpacity: 0.12,
        weight: 2
      }).addTo(mapInstance);
    }
  }

  // 1. View Switcher: toggles between Grid and Map views
  function switchView(mode) {
    if (!btnGrid || !btnMap || !petsGrid || !petsMapContainer) return;

    if (mode === 'map') {
      btnMap.classList.add('active');
      btnMap.setAttribute('aria-pressed', 'true');
      btnGrid.classList.remove('active');
      btnGrid.setAttribute('aria-pressed', 'false');

      petsGrid.classList.add('hidden');
      petsMapContainer.classList.remove('hidden');

      try {
        localStorage.setItem('petspotr_view_mode', 'map');
      } catch (_) {}

      // Update URL search parameter without page reload
      if (window.history && window.history.replaceState) {
        const url = new URL(window.location.href);
        if (url.searchParams.get('view') !== 'map') {
          url.searchParams.set('view', 'map');
          window.history.replaceState({}, '', url.toString());
        }
      }

      // Lazily initialize Leaflet map on first map view activation
      if (!mapInstance) {
        mapInstance = initMap();
      } else {
        mapInstance.invalidateSize();
      }

      setTimeout(() => {
        if (mapInstance) {
          mapInstance.invalidateSize();
        }
      }, 50);
    } else {
      btnGrid.classList.add('active');
      btnGrid.setAttribute('aria-pressed', 'true');
      btnMap.classList.remove('active');
      btnMap.setAttribute('aria-pressed', 'false');

      petsGrid.classList.remove('hidden');
      petsMapContainer.classList.add('hidden');

      try {
        localStorage.setItem('petspotr_view_mode', 'grid');
      } catch (_) {}

      // Update URL search parameter without page reload
      if (window.history && window.history.replaceState) {
        const url = new URL(window.location.href);
        if (url.searchParams.get('view') === 'map') {
          url.searchParams.delete('view');
          window.history.replaceState({}, '', url.toString());
        }
      }
    }
  }

  function initViewSwitcher() {
    if (!btnGrid || !btnMap) return;

    btnGrid.addEventListener('click', () => switchView('grid'));
    btnMap.addEventListener('click', () => switchView('map'));

    // Check URL parameter first, then localStorage, defaulting to 'grid'
    const urlParams = new URLSearchParams(window.location.search);
    const viewParam = urlParams.get('view');
    let initialMode = 'grid';

    if (viewParam === 'map') {
      initialMode = 'map';
    } else if (viewParam === 'grid') {
      initialMode = 'grid';
    } else {
      try {
        const savedMode = localStorage.getItem('petspotr_view_mode');
        if (savedMode === 'map') {
          initialMode = 'map';
        }
      } catch (_) {}
    }

    if (initialMode === 'map') {
      switchView('map');
    }
  }

  // 2. Leaflet Map Initialization
  function initMap() {
    if (typeof L === 'undefined') {
      console.warn('Leaflet global L is not loaded.');
      return null;
    }

    const mapElement = document.getElementById('pets-map');
    if (!mapElement) return null;

    if (mapInstance) return mapInstance;

    // Safely parse JSON data from #pets-data script tag
    const scriptTag = document.getElementById('pets-data');
    let items = [];
    if (scriptTag && scriptTag.textContent) {
      try {
        items = JSON.parse(scriptTag.textContent) || [];
      } catch (err) {
        console.warn('Failed to parse #pets-data JSON:', err);
      }
    }

    // Initialize Leaflet map instance
    const map = L.map(mapElement, {
      scrollWheelZoom: false
    });

    // Add OpenStreetMap raster tile layer
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
      maxZoom: 19
    }).addTo(map);

    const markersGroup = L.featureGroup().addTo(map);

    // Render pet markers
    items.forEach((item) => {
      const lat = parseFloat(item.lat);
      const lng = parseFloat(item.lng);
      if (isNaN(lat) || isNaN(lng) || (lat === 0 && lng === 0)) {
        return;
      }

      const isLost = item.status === 'lost';
      const pinIcon = L.divIcon({
        className: 'custom-map-pin',
        html:
          '<div class="map-pin-inner pin-' +
          (isLost ? 'lost' : 'found') +
          '"><span>' +
          (isLost ? '🐾' : '✓') +
          '</span></div>',
        iconSize: [32, 32],
        iconAnchor: [16, 32],
        popupAnchor: [0, -32]
      });

      const speciesCap = item.species ? item.species.charAt(0).toUpperCase() + item.species.slice(1) : 'Pet';
      const title = item.petName ? item.petName.trim() : (isLost ? `Lost ${speciesCap}` : `Found ${speciesCap}`);
      const breedPart = item.breed ? ` · ${escapeHTML(item.breed)}` : '';
      const locationPart = item.location ? `📍 ${escapeHTML(item.location)}` : '';
      const datePart = item.reportedAt ? `Reported ${escapeHTML(item.reportedAt)}` : '';

      let imageHtml = '';
      if (item.imageUrl) {
        imageHtml = `<img src="${escapeHTML(item.imageUrl)}" alt="${escapeHTML(title)}" class="popup-img" loading="lazy">`;
      } else {
        imageHtml = `<div class="popup-img" style="display:flex;align-items:center;justify-content:center;background:rgba(15,23,42,0.6);font-size:2rem;color:var(--text-secondary);" aria-hidden="true">🐾</div>`;
      }

      const popupHtml = `
        <div class="map-pet-popup-card">
          ${imageHtml}
          <h3 class="popup-title">${escapeHTML(title)}</h3>
          <p class="popup-meta"><strong>${isLost ? 'Lost' : 'Found'}</strong> · ${escapeHTML(speciesCap)}${breedPart}</p>
          ${locationPart ? `<p class="popup-meta">${locationPart}</p>` : ''}
          ${datePart ? `<p class="popup-meta"><small>${datePart}</small></p>` : ''}
          <a href="#pet-card-${encodeURIComponent(item.petId)}" class="btn btn-primary popup-btn" data-pet-id="${escapeHTML(item.petId)}">View Details</a>
        </div>
      `;

      const marker = L.marker([lat, lng], {
        icon: pinIcon,
        title: title
      }).bindPopup(popupHtml, {
        className: 'map-pet-popup',
        maxWidth: 240
      });

      markersGroup.addLayer(marker);
    });

    // Initial proximity center and circle overlay if coordinates are present
    const initLat = parseFloat(latInput?.value);
    const initLng = parseFloat(lngInput?.value);
    const initRadius = parseFloat(radiusSelect?.value) || 10;
    const hasInitialCoords = !isNaN(initLat) && !isNaN(initLng) && (initLat !== 0 || initLng !== 0);

    mapInstance = map;

    if (hasInitialCoords) {
      updateProximityOverlay(initLat, initLng, initRadius);
    }

    // Fit map bounds to markers if present, or center on search coords/default
    if (markersGroup.getLayers().length > 0) {
      map.fitBounds(markersGroup.getBounds(), { padding: [50, 50], maxZoom: 16 });
    } else if (hasInitialCoords) {
      map.setView([initLat, initLng], 12);
    } else {
      map.setView([37.7749, -122.4194], 11);
    }

    // Click-to-pin listener
    map.on('click', (e) => {
      const clickedLat = Number(e.latlng.lat.toFixed(6));
      const clickedLng = Number(e.latlng.lng.toFixed(6));

      if (latInput) latInput.value = clickedLat;
      if (lngInput) lngInput.value = clickedLng;

      if (radiusSelect) {
        radiusSelect.removeAttribute('disabled');
        radiusSelect.disabled = false;
      }

      if (geoBtnLabel) {
        geoBtnLabel.textContent = 'Location Set';
      }

      const activeRadius = parseFloat(radiusSelect?.value) || 10;
      updateProximityOverlay(clickedLat, clickedLng, activeRadius);

      if (mapStatusOverlay) {
        mapStatusOverlay.textContent = `Location set to ${clickedLat.toFixed(4)}, ${clickedLng.toFixed(4)}. Radius: ${activeRadius} miles.`;
      }

      // Prompt popup offering to apply proximity filter
      const promptDiv = document.createElement('div');
      promptDiv.style.textAlign = 'center';
      promptDiv.style.padding = '4px';
      promptDiv.innerHTML = `
        <p style="margin: 0 0 8px 0; font-weight: 600; font-size: 0.9rem;">Search within ${activeRadius} miles of here?</p>
        <button type="button" class="btn btn-primary btn-sm popup-btn" id="btn-apply-map-pin">Search Here</button>
      `;

      L.popup({ className: 'map-pet-popup', offset: [0, -10] })
        .setLatLng(e.latlng)
        .setContent(promptDiv)
        .openOn(map);

      promptDiv.querySelector('#btn-apply-map-pin')?.addEventListener('click', () => {
        filterForm?.submit();
      });
    });

    // Popup interaction for "View Details": switch to Grid view and scroll to card
    map.on('popupopen', (e) => {
      const popupNode = e.popup.getElement();
      if (!popupNode) return;
      const detailBtn = popupNode.querySelector('.popup-btn[data-pet-id]');
      if (detailBtn) {
        detailBtn.addEventListener('click', (ev) => {
          ev.preventDefault();
          const petId = detailBtn.getAttribute('data-pet-id');
          switchView('grid');
          const targetCard = document.querySelector(`.pet-card[data-pet-id="${petId}"]`);
          if (targetCard) {
            targetCard.scrollIntoView({ behavior: 'smooth', block: 'center' });
            targetCard.focus();
            targetCard.style.outline = '3px solid var(--accent-primary)';
            setTimeout(() => {
              targetCard.style.outline = '';
            }, 2000);
          }
        });
      }
    });

    return map;
  }

  // 3. Geolocation Controller
  function initGeolocation() {
    if (!geoBtn) return;

    geoBtn.addEventListener('click', () => {
      if (!navigator.geolocation) {
        showStatusMessage('Geolocation is not supported by your browser. You can click on the map to set a search center.', true);
        return;
      }

      const originalLabel = geoBtnLabel ? geoBtnLabel.textContent : 'Near Me';
      if (geoBtnLabel) geoBtnLabel.textContent = 'Locating...';
      geoBtn.disabled = true;

      navigator.geolocation.getCurrentPosition(
        (position) => {
          geoBtn.disabled = false;
          const lat = Number(position.coords.latitude.toFixed(6));
          const lng = Number(position.coords.longitude.toFixed(6));

          if (latInput) latInput.value = lat;
          if (lngInput) lngInput.value = lng;
          if (radiusSelect) {
            radiusSelect.removeAttribute('disabled');
            radiusSelect.disabled = false;
          }
          if (geoBtnLabel) geoBtnLabel.textContent = 'Location Set';

          showStatusMessage('Location detected! Updating search results...');

          const activeRadius = parseFloat(radiusSelect?.value) || 10;
          if (mapInstance) {
            updateProximityOverlay(lat, lng, activeRadius);
            mapInstance.setView([lat, lng], 12);
          }

          if (filterForm) {
            filterForm.submit();
          }
        },
        (error) => {
          geoBtn.disabled = false;
          if (geoBtnLabel) geoBtnLabel.textContent = originalLabel;

          let message = 'Location access was denied. You can click on the map to set a search center.';
          if (error.code === error.POSITION_UNAVAILABLE) {
            message = 'Location information is unavailable. You can click on the map to set a search center.';
          } else if (error.code === error.TIMEOUT) {
            message = 'The request to get your location timed out. Please try again or click on the map.';
          }
          showStatusMessage(message, true);
        },
        {
          enableHighAccuracy: true,
          timeout: 10000,
          maximumAge: 60000
        }
      );
    });
  }

  // 4. Radius Select Controller
  function initRadiusSelect() {
    if (!radiusSelect) return;

    radiusSelect.addEventListener('change', () => {
      const lat = parseFloat(latInput?.value);
      const lng = parseFloat(lngInput?.value);

      if (!isNaN(lat) && !isNaN(lng) && (lat !== 0 || lng !== 0)) {
        const activeRadius = parseFloat(radiusSelect.value) || 10;
        if (mapInstance) {
          updateProximityOverlay(lat, lng, activeRadius);
        }
        if (filterForm) {
          filterForm.submit();
        }
      }
    });
  }

  // 5. Existing filter listeners and accessibility
  if (speciesSelect) {
    speciesSelect.addEventListener('change', () => {
      filterForm?.submit();
    });
  }

  if (statusSelect) {
    statusSelect.addEventListener('change', () => {
      filterForm?.submit();
    });
  }

  document.querySelectorAll('.pet-card').forEach((card) => {
    card.setAttribute('tabindex', '0');
  });

  // Initialize all controllers
  initViewSwitcher();
  initGeolocation();
  initRadiusSelect();
});
