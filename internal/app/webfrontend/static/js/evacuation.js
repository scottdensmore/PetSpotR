// PetSpotR Disaster Evacuation Operations Dashboard Controller
// Strict CSP compliant - zero inline script execution, zero eval, pure DOM event listeners

(function () {
  'use strict';

  // --- State ---
  let activeHubs = [];
  let selectedFile = null;
  let currentTab = 'tab-hubs';

  // --- Accessibility Announcement & Toast Helpers ---
  function announce(message) {
    const announcer = document.getElementById('evacuation-aria-announcer');
    if (announcer) {
      announcer.textContent = '';
      setTimeout(() => {
        announcer.textContent = message;
      }, 50);
    }
  }

  function showToast(message, type) {
    const container = document.getElementById('toast-container');
    if (!container) return;

    const toast = document.createElement('div');
    toast.className = `toast toast-${type || 'info'} glass-card`;
    toast.setAttribute('role', 'alert');
    toast.textContent = message;

    container.appendChild(toast);

    setTimeout(() => {
      toast.classList.add('toast-fade-out');
      setTimeout(() => {
        if (toast.parentNode) {
          toast.parentNode.removeChild(toast);
        }
      }, 300);
    }, 4000);
  }

  function escapeHTML(str) {
    if (str === null || str === undefined) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  function formatFileSize(bytes) {
    if (typeof bytes !== 'number' || isNaN(bytes) || bytes <= 0) return '0 B';
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / 1048576).toFixed(2) + ' MB';
  }

  function formatHubType(typeStr) {
    switch (typeStr) {
      case 'POP_UP_CRISIS_CENTER':
        return 'Pop-Up Crisis Center';
      case 'FAIRGROUND_STAGING':
        return 'Fairground Staging';
      case 'PERMANENT_SHELTER':
        return 'Permanent Shelter';
      case 'MOBILE_TRIAGE':
        return 'Mobile Triage Unit';
      default:
        return typeStr ? typeStr.replace(/_/g, ' ') : 'Emergency Facility';
    }
  }

  function formatTransferStatus(statusStr) {
    switch (statusStr) {
      case 'STAGED':
        return { label: 'Staged', className: 'transfer-status-staged' };
      case 'IN_TRANSIT':
        return { label: 'In Transit 🚚', className: 'transfer-status-intransit' };
      case 'RECEIVED':
        return { label: 'Received ✓', className: 'transfer-status-received' };
      case 'RECONCILED':
        return { label: 'Reconciled 🎯', className: 'transfer-status-reconciled' };
      default:
        return { label: statusStr || 'Unknown', className: 'badge' };
    }
  }

  function formatMicrochipBadge(standard, valid) {
    if (!valid) {
      return '<span class="badge badge-warning text-small">Invalid Format</span>';
    }
    switch (standard) {
      case 'ISO_11784_11785':
      case 'iso_15':
        return '<span class="badge badge-info text-small">ISO 15-Digit</span>';
      case 'AVID_9_DIGIT':
      case 'avid_9':
        return '<span class="badge badge-secondary text-small">Avid 9-Digit</span>';
      case 'AVID_10_DIGIT':
      case 'euro_10':
      case 'EURO_FDX_A':
        return '<span class="badge badge-secondary text-small">Euro FDX-A</span>';
      default:
        return '<span class="badge badge-success text-small">Verified</span>';
    }
  }

  // --- Tab Navigation ---
  function initTabs() {
    const tabs = [
      { tabId: 'tab-hubs', panelId: 'tab-panel-hubs' },
      { tabId: 'tab-intake', panelId: 'tab-panel-intake' },
      { tabId: 'tab-transfers', panelId: 'tab-panel-transfers' },
      { tabId: 'tab-reunifications', panelId: 'tab-panel-reunifications' },
    ];

    const tabElements = tabs.map((t) => document.getElementById(t.tabId)).filter(Boolean);

    function switchTab(targetId) {
      tabs.forEach(({ tabId, panelId }) => {
        const tabEl = document.getElementById(tabId);
        const panelEl = document.getElementById(panelId);
        const isMatch = tabId === targetId;

        if (tabEl) {
          tabEl.setAttribute('aria-selected', isMatch ? 'true' : 'false');
          tabEl.tabIndex = isMatch ? 0 : -1;
          if (isMatch) {
            tabEl.classList.add('active');
            tabEl.focus();
          } else {
            tabEl.classList.remove('active');
          }
        }

        if (panelEl) {
          if (isMatch) {
            panelEl.removeAttribute('hidden');
          } else {
            panelEl.setAttribute('hidden', '');
          }
        }
      });

      currentTab = targetId;

      // Lazy refresh depending on tab
      if (targetId === 'tab-transfers') {
        loadTransfers();
      } else if (targetId === 'tab-reunifications') {
        loadReunificationQueue();
      } else if (targetId === 'tab-hubs') {
        loadHubs();
      }
    }

    tabElements.forEach((tabBtn, index) => {
      tabBtn.addEventListener('click', () => {
        switchTab(tabBtn.id);
      });

      tabBtn.addEventListener('keydown', (e) => {
        let newIndex = index;
        if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
          e.preventDefault();
          newIndex = (index + 1) % tabElements.length;
          switchTab(tabElements[newIndex].id);
        } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
          e.preventDefault();
          newIndex = (index - 1 + tabElements.length) % tabElements.length;
          switchTab(tabElements[newIndex].id);
        } else if (e.key === 'Home') {
          e.preventDefault();
          switchTab(tabElements[0].id);
        } else if (e.key === 'End') {
          e.preventDefault();
          switchTab(tabElements[tabElements.length - 1].id);
        }
      });
    });
  }

  // --- Hubs and Capacity Metrics ---
  async function loadHubs() {
    try {
      const res = await fetch('/api/v1/evacuations/hubs');
      if (!res.ok) {
        throw new Error(`Failed to load hubs: HTTP ${res.status}`);
      }
      const hubs = await res.json();
      activeHubs = Array.isArray(hubs) ? hubs : [];

      updateRegionalMetrics(activeHubs);
      renderHubsGrid(activeHubs);
      populateFacilitySelects(activeHubs);
    } catch (err) {
      console.error('Error loading evacuation hubs:', err);
      const grid = document.getElementById('hubs-card-grid');
      if (grid) {
        grid.innerHTML = '<div class="alert alert-error">Unable to load evacuation facilities. Please try again.</div>';
      }
    }
  }

  function updateRegionalMetrics(hubs) {
    let totalCapacity = 0;
    let totalEvacuated = 0;
    let activeHubsCount = 0;

    hubs.forEach((hub) => {
      if (hub.status === 'ACTIVE' || hub.status === 'FULL') {
        activeHubsCount++;
      }
      totalCapacity += hub.totalCapacity || 0;
      totalEvacuated += hub.currentOccupancy || 0;
    });

    const openCapacity = Math.max(0, totalCapacity - totalEvacuated);

    const activeHubsEl = document.getElementById('metric-active-hubs');
    if (activeHubsEl) activeHubsEl.textContent = activeHubsCount.toLocaleString();

    const openCapEl = document.getElementById('metric-open-capacity');
    if (openCapEl) openCapEl.textContent = openCapacity.toLocaleString();

    const evacuatedEl = document.getElementById('metric-evacuated-total');
    if (evacuatedEl) evacuatedEl.textContent = totalEvacuated.toLocaleString();
  }

  function renderHubsGrid(hubs) {
    const grid = document.getElementById('hubs-card-grid');
    if (!grid) return;

    if (hubs.length === 0) {
      grid.innerHTML = '<div class="empty-state text-muted">No emergency evacuation facilities registered. Click "+ Register Emergency Hub" to add one.</div>';
      return;
    }

    grid.innerHTML = '';

    hubs.forEach((hub) => {
      const card = document.createElement('div');
      card.className = 'hub-card glass-card';

      const totalCap = hub.totalCapacity || 1;
      const currentOcc = hub.currentOccupancy || 0;
      const pct = Math.min(100, Math.round((currentOcc / totalCap) * 100));

      let levelClass = 'occupancy-low';
      let level = 'low';
      if (pct >= 90) {
        levelClass = 'occupancy-high';
        level = 'high';
      } else if (pct >= 70) {
        levelClass = 'occupancy-medium';
        level = 'medium';
      }

      const isFull = currentOcc >= totalCap || hub.status === 'FULL';

      card.innerHTML = `
        <div class="hub-card-header">
          <div>
            <h3 class="hub-card-title">${escapeHTML(hub.name)}</h3>
            <span class="hub-type-badge">${escapeHTML(formatHubType(hub.type))}</span>
          </div>
          ${isFull ? '<span class="badge badge-danger">FULL</span>' : '<span class="badge badge-success">ACTIVE</span>'}
        </div>

        <div class="hub-details-list">
          <div>📍 <strong>Address:</strong> ${escapeHTML(hub.address || 'Address unlisted')}</div>
          ${hub.contactName ? `<div>👤 <strong>Lead:</strong> ${escapeHTML(hub.contactName)}</div>` : ''}
          ${hub.contactPhone ? `<div>📞 <strong>Phone:</strong> ${escapeHTML(hub.contactPhone)}</div>` : ''}
          ${hub.contactEmail ? `<div>✉️ <strong>Email:</strong> ${escapeHTML(hub.contactEmail)}</div>` : ''}
        </div>

        <div class="occupancy-gauge-container">
          <div class="occupancy-gauge-header">
            <span class="occupancy-gauge-label">Occupancy:</span>
            <span class="occupancy-gauge-value">${currentOcc.toLocaleString()} / ${totalCap.toLocaleString()} (${pct}%)</span>
          </div>
          <div class="occupancy-bar-track progress-bar-occupancy" role="progressbar" aria-valuenow="${pct}" aria-valuemin="0" aria-valuemax="100" aria-label="${escapeHTML(hub.name)} occupancy">
            <div class="occupancy-bar-fill ${levelClass}" data-level="${level}" style="width: ${pct}%;"></div>
          </div>
          <div class="species-breakdown-row">
            <span>🐕 Dogs: <strong>${(hub.dogOccupancy || 0).toLocaleString()} / ${(hub.dogCapacity || 0).toLocaleString()}</strong></span>
            <span>🐈 Cats: <strong>${(hub.catOccupancy || 0).toLocaleString()} / ${(hub.catCapacity || 0).toLocaleString()}</strong></span>
          </div>
        </div>
      `;

      grid.appendChild(card);
    });
  }

  function populateFacilitySelects(hubs) {
    const intakeSelect = document.getElementById('intake-hub-select');
    const originSelect = document.getElementById('input-transfer-origin') || document.getElementById('transfer-origin-hub');
    const destSelect = document.getElementById('input-transfer-destination') || document.getElementById('transfer-dest-hub');

    const updateSelect = (selectEl, defaultPlaceholder) => {
      if (!selectEl) return;
      const currentVal = selectEl.value;

      while (selectEl.options.length > 1) {
        selectEl.remove(1);
      }

      hubs.forEach((hub) => {
        const opt = document.createElement('option');
        opt.value = hub.hubId;
        opt.textContent = `${hub.name} (${hub.currentOccupancy || 0}/${hub.totalCapacity || 0})`;
        selectEl.appendChild(opt);
      });

      if (currentVal) {
        selectEl.value = currentVal;
      }
    };

    updateSelect(intakeSelect, '-- Select Destination Facility --');
    updateSelect(originSelect, '-- Select Origin Hub --');
    updateSelect(destSelect, '-- Select Destination Hub --');
  }

  // --- Bulk Intake Dropzone & Uploader ---
  function initBulkIntake() {
    const dropzone = document.getElementById('intake-dropzone');
    const fileInput = document.getElementById('intake-file-input');
    const fileDetails = document.getElementById('intake-file-details');
    const processBtn = document.getElementById('btn-process-batch');
    const hubSelect = document.getElementById('intake-hub-select');
    const feedback = document.getElementById('intake-feedback');

    if (!dropzone || !fileInput) return;

    function handleFile(file) {
      if (!file) return;

      const validExts = ['.csv', '.json'];
      const fileNameLower = file.name.toLowerCase();
      const isValid = validExts.some((ext) => fileNameLower.endsWith(ext));

      if (!isValid) {
        if (feedback) {
          feedback.innerHTML = '<div class="alert alert-error">Invalid file format. Please provide a .csv or .json roster file.</div>';
        }
        selectedFile = null;
        if (processBtn) processBtn.disabled = true;
        if (fileDetails) fileDetails.hidden = true;
        return;
      }

      selectedFile = file;

      if (fileDetails) {
        fileDetails.hidden = false;
        fileDetails.textContent = `📄 ${file.name} (${formatFileSize(file.size)})`;
      }

      if (feedback) {
        feedback.innerHTML = '';
      }

      checkReadyToProcess();
    }

    function checkReadyToProcess() {
      if (!processBtn) return;
      const hasHub = hubSelect && hubSelect.value.trim() !== '';
      const hasFile = selectedFile !== null;
      processBtn.disabled = !(hasHub && hasFile);
    }

    if (hubSelect) {
      hubSelect.addEventListener('change', checkReadyToProcess);
    }

    // Drag and drop handlers
    dropzone.addEventListener('dragover', (e) => {
      e.preventDefault();
      e.stopPropagation();
      dropzone.classList.add('dragover');
    });

    dropzone.addEventListener('dragleave', (e) => {
      e.preventDefault();
      e.stopPropagation();
      dropzone.classList.remove('dragover');
    });

    dropzone.addEventListener('drop', (e) => {
      e.preventDefault();
      e.stopPropagation();
      dropzone.classList.remove('dragover');

      if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length > 0) {
        handleFile(e.dataTransfer.files[0]);
      }
    });

    dropzone.addEventListener('click', (e) => {
      if (e.target !== fileInput) {
        fileInput.click();
      }
    });

    dropzone.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        fileInput.click();
      }
    });

    fileInput.addEventListener('change', () => {
      if (fileInput.files && fileInput.files.length > 0) {
        handleFile(fileInput.files[0]);
      }
    });

    // Process Batch Button
    if (processBtn) {
      processBtn.addEventListener('click', async () => {
        if (!selectedFile) {
          showToast('Please select a roster file to upload', 'warning');
          return;
        }

        const hubId = hubSelect ? hubSelect.value.trim() : '';
        if (!hubId) {
          if (feedback) {
            feedback.innerHTML = '<div class="alert alert-error">Please select a destination emergency facility.</div>';
          }
          showToast('Destination emergency facility is required', 'warning');
          return;
        }

        processBtn.disabled = true;
        processBtn.textContent = '⏳ Processing Intake Batch...';
        if (feedback) {
          feedback.innerHTML = '<div class="alert alert-info">Reconciling animal records and microchips...</div>';
        }

        try {
          const formData = new FormData();
          formData.append('file', selectedFile);
          formData.append('hubId', hubId);

          const res = await fetch('/api/v1/evacuations/intake-batch', {
            method: 'POST',
            body: formData,
          });

          if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || `Intake failed with HTTP ${res.status}`);
          }

          const summary = await res.json();

          renderBatchSummary(summary);
          renderBatchResultsTable(summary.results || []);

          if (feedback) {
            const matchMsg = summary.instantMatches > 0
              ? ` Found <strong>${summary.instantMatches} instant crisis microchip match(es)</strong>!`
              : '';
            feedback.innerHTML = `<div class="alert alert-success">Successfully ingested ${summary.ingestedCount} animal records.${matchMsg}</div>`;
          }

          showToast(`Ingested ${summary.ingestedCount} records successfully`, 'success');
          announce(`Batch complete. ${summary.ingestedCount} animals ingested, ${summary.instantMatches} crisis matches found.`);

          // Refresh hubs occupancy metrics and reunifications queue
          loadHubs();
          loadReunificationQueue();
        } catch (err) {
          console.error('Batch intake error:', err);
          if (feedback) {
            feedback.innerHTML = `<div class="alert alert-error">Error processing batch intake: ${escapeHTML(err.message)}</div>`;
          }
          showToast('Failed to process batch intake', 'error');
        } finally {
          processBtn.textContent = '⚡ Process Batch Intake';
          processBtn.disabled = false;
        }
      });
    }
  }

  function renderBatchSummary(summary) {
    const summaryCard = document.getElementById('intake-batch-summary');
    if (!summaryCard) return;

    summaryCard.removeAttribute('hidden');

    const totalEl = document.getElementById('summary-total-rows');
    const importedEl = document.getElementById('summary-imported-rows');
    const matchedEl = document.getElementById('summary-matched-rows');
    const errorEl = document.getElementById('summary-error-rows');

    if (totalEl) totalEl.textContent = (summary.totalProcessed || 0).toLocaleString();
    if (importedEl) importedEl.textContent = (summary.ingestedCount || 0).toLocaleString();
    if (matchedEl) matchedEl.textContent = (summary.instantMatches || 0).toLocaleString();
    if (errorEl) errorEl.textContent = (summary.errorCount || 0).toLocaleString();
  }

  function renderBatchResultsTable(results) {
    const tbody = document.getElementById('intake-results-body');
    if (!tbody) return;

    tbody.innerHTML = '';

    if (!Array.isArray(results) || results.length === 0) {
      tbody.innerHTML = '<tr class="empty-row"><td colspan="6" class="text-center text-muted">No records to display.</td></tr>';
      return;
    }

    results.forEach((r) => {
      const tr = document.createElement('tr');

      const isError = r.status === 'ERROR';
      const isMatched = Boolean(r.matchedLostPetId);

      const statusBadge = isError
        ? '<span class="badge badge-danger">ERROR</span>'
        : '<span class="badge badge-success">INGESTED</span>';

      const microchipContent = r.microchip
        ? `<code>${escapeHTML(r.microchip)}</code> ${formatMicrochipBadge(r.microchipStandard, r.microchipValid)}`
        : '<span class="text-muted">None</span>';

      const reconStatus = isMatched
        ? `<span class="badge badge-microchip-exact">🔥 Exact Microchip Match</span>`
        : (isError ? '<span class="text-muted text-error">Validation Failed</span>' : '<span class="text-muted">No Instant Match</span>');

      const detailsContent = isMatched
        ? `Matched Lost Report: <strong>${escapeHTML(r.matchedLostPetId)}</strong>`
        : (r.errorMessage ? `<span class="text-error">${escapeHTML(r.errorMessage)}</span>` : `Ingested into facility (Row #${r.rowIndex})`);

      tr.innerHTML = `
        <td>${statusBadge}</td>
        <td><strong>${escapeHTML(r.petId || ('Row #' + r.rowIndex))}</strong></td>
        <td>${escapeHTML(r.species || '—')}</td>
        <td>${microchipContent}</td>
        <td>${reconStatus}</td>
        <td>${detailsContent}</td>
      `;

      tbody.appendChild(tr);
    });
  }

  // --- Mutual Aid Transfer Ledger ---
  async function loadTransfers() {
    const filterEl = document.getElementById('transfers-status-filter');
    const filter = filterEl ? filterEl.value : 'ALL';

    const url = filter && filter !== 'ALL'
      ? `/api/v1/evacuations/transfers?status=${encodeURIComponent(filter)}`
      : '/api/v1/evacuations/transfers';

    try {
      const res = await fetch(url);
      if (!res.ok) {
        throw new Error(`Failed to load transfers: HTTP ${res.status}`);
      }
      const manifests = await res.json();
      renderTransfersTable(Array.isArray(manifests) ? manifests : []);
    } catch (err) {
      console.error('Error loading transfers:', err);
      const tbody = document.getElementById('transfers-ledger-body');
      if (tbody) {
        tbody.innerHTML = '<tr class="empty-row"><td colspan="8" class="text-center text-error">Failed to load transfer manifests.</td></tr>';
      }
    }
  }

  function renderTransfersTable(manifests) {
    const tbody = document.getElementById('transfers-ledger-body');
    if (!tbody) return;

    tbody.innerHTML = '';

    if (manifests.length === 0) {
      tbody.innerHTML = '<tr class="empty-row"><td colspan="8" class="text-center text-muted">No mutual aid transfers recorded. Click "Stage Mutual Aid Transfer" to create one.</td></tr>';
      return;
    }

    manifests.forEach((m) => {
      const tr = document.createElement('tr');

      const statusInfo = formatTransferStatus(m.status);
      const animalCount = m.totalAnimals || (m.animalIds ? m.animalIds.length : 0);

      const dateStr = m.updatedAt || m.createdAt;
      const formattedDate = dateStr ? new Date(dateStr).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', month: 'short', day: 'numeric' }) : '—';

      let actionButtons = '';
      if (m.status === 'STAGED') {
        actionButtons = `<button type="button" class="btn btn-sm btn-primary btn-transit-transfer" data-action="transit" data-transfer-id="${escapeHTML(m.transferId)}">Mark In Transit 🚚</button>`;
      } else if (m.status === 'IN_TRANSIT') {
        actionButtons = `<button type="button" class="btn btn-sm btn-primary btn-receive-transfer" data-action="receive" data-transfer-id="${escapeHTML(m.transferId)}">Mark Received ✓</button>`;
      } else if (m.status === 'RECEIVED') {
        actionButtons = `<button type="button" class="btn btn-sm btn-secondary btn-reconcile-transfer" data-action="reconcile" data-transfer-id="${escapeHTML(m.transferId)}">Reconcile 🎯</button>`;
      } else {
        actionButtons = '<span class="text-muted text-small">Completed ✓</span>';
      }

      tr.innerHTML = `
        <td><code>${escapeHTML(m.transferId)}</code></td>
        <td><strong>${escapeHTML(m.originHubName || m.originHubId)}</strong></td>
        <td><strong>${escapeHTML(m.destHubName || m.destHubId)}</strong></td>
        <td>${animalCount} animal(s)</td>
        <td><span class="${statusInfo.className}">${statusInfo.label}</span></td>
        <td>${escapeHTML(m.transporterName || '—')} ${m.transporterPhone ? `(${escapeHTML(m.transporterPhone)})` : ''}</td>
        <td>${formattedDate}</td>
        <td>${actionButtons}</td>
      `;

      tbody.appendChild(tr);
    });
  }

  async function updateTransferStatus(transferId, newStatus) {
    try {
      const res = await fetch(`/api/v1/evacuations/transfers/${encodeURIComponent(transferId)}/status`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: newStatus }),
      });

      if (!res.ok) {
        const errData = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
        throw new Error(errData.error || `HTTP ${res.status}`);
      }

      showToast(`Transfer ${transferId} transitioned to ${newStatus}`, 'success');
      announce(`Transfer status updated to ${newStatus}`);

      // Refresh both ledger and hubs to update occupancies
      loadTransfers();
      loadHubs();
    } catch (err) {
      console.error(`Failed to transition transfer ${transferId} to ${newStatus}:`, err);
      showToast(`Error updating transfer: ${err.message}`, 'error');
    }
  }

  function initTransfers() {
    const filterEl = document.getElementById('transfers-status-filter');
    if (filterEl) {
      filterEl.addEventListener('change', () => {
        loadTransfers();
      });
    }

    const ledgerTable = document.getElementById('transfers-ledger-table');
    if (ledgerTable) {
      ledgerTable.addEventListener('click', (e) => {
        const btn = e.target.closest('button[data-action]');
        if (!btn) return;

        const action = btn.getAttribute('data-action');
        const transferId = btn.getAttribute('data-transfer-id');

        if (!transferId) return;

        if (action === 'transit') {
          updateTransferStatus(transferId, 'IN_TRANSIT');
        } else if (action === 'receive') {
          updateTransferStatus(transferId, 'RECEIVED');
        } else if (action === 'reconcile') {
          updateTransferStatus(transferId, 'RECONCILED');
        }
      });
    }

    // Modal: Stage Transfer
    const openBtn = document.getElementById('btn-stage-transfer');
    const modal = document.getElementById('modal-stage-transfer');
    const form = document.getElementById('form-stage-transfer');

    if (openBtn && modal) {
      openBtn.addEventListener('click', () => {
        modal.classList.remove('hidden');
      });
    }

    if (modal) {
      modal.querySelectorAll('[data-close-modal="stage-transfer"]').forEach((el) => {
        el.addEventListener('click', () => {
          modal.classList.add('hidden');
        });
      });

      modal.addEventListener('click', (e) => {
        if (e.target === modal) {
          modal.classList.add('hidden');
        }
      });
    }

    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();

        const originHubId = (form.elements['originHubId']?.value || '').trim();
        const destHubId = (form.elements['destHubId']?.value || '').trim();
        const animalIdsRaw = (form.elements['animalIds']?.value || '').trim();
        const transporterName = (form.elements['transporterName']?.value || '').trim();
        const transporterPhone = (form.elements['transporterPhone']?.value || '').trim();
        const vehicleNotes = (form.elements['vehicleNotes']?.value || '').trim();

        if (!originHubId || !destHubId) {
          showToast('Origin and Destination facilities are required', 'warning');
          return;
        }

        if (originHubId === destHubId) {
          showToast('Origin and Destination facilities cannot be the same', 'warning');
          return;
        }

        const animalIds = animalIdsRaw
          .split(/[\n,]+/)
          .map((id) => id.trim())
          .filter((id) => id.length > 0);

        if (animalIds.length === 0) {
          showToast('At least one animal ID is required for transfer', 'warning');
          return;
        }

        try {
          const res = await fetch('/api/v1/evacuations/transfers', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              originHubId,
              destHubId,
              animalIds,
              transporterName,
              transporterPhone,
              vehicleNotes,
            }),
          });

          if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || `Failed to create transfer: HTTP ${res.status}`);
          }

          if (modal) modal.classList.add('hidden');
          form.reset();

          showToast(`Mutual aid transfer staged for ${animalIds.length} animals`, 'success');
          announce(`Mutual aid transfer manifest created successfully.`);

          loadTransfers();
          loadHubs();
        } catch (err) {
          console.error('Error creating transfer manifest:', err);
          showToast(`Error creating transfer: ${err.message}`, 'error');
        }
      });
    }
  }

  // --- Crisis Reunifications Queue ---
  async function loadReunificationQueue() {
    try {
      const res = await fetch('/api/v1/evacuations/reunification-queue');
      if (!res.ok) {
        throw new Error(`Failed to load reunification queue: HTTP ${res.status}`);
      }
      const queue = await res.json();
      const items = Array.isArray(queue) ? queue : [];

      renderReunifications(items);
      updatePendingReunificationsCount(items);
    } catch (err) {
      console.error('Error loading reunification queue:', err);
      const grid = document.getElementById('reunifications-grid');
      if (grid) {
        grid.innerHTML = '<div class="alert alert-error">Unable to load crisis reunification queue.</div>';
      }
    }
  }

  function updatePendingReunificationsCount(items) {
    const countEl = document.getElementById('metric-reunifications-pending');
    if (!countEl) return;

    const pending = items.filter((item) => item.status === 'PENDING').length;
    countEl.textContent = pending.toLocaleString();
  }

  function renderReunifications(items) {
    const grid = document.getElementById('reunifications-grid');
    if (!grid) return;

    grid.innerHTML = '';

    if (items.length === 0) {
      grid.innerHTML = '<div class="empty-state text-muted">No pending crisis reunifications in queue. Outstanding work!</div>';
      return;
    }

    items.forEach((item) => {
      const card = document.createElement('div');
      card.className = 'glass-card reunification-card crisis-match-card';
      card.setAttribute('data-match-id', item.matchId);

      const isContacted = item.status === 'CONTACTED';
      const isChipExact = item.priority === 'MICROCHIP_EXACT';

      const priorityBadge = isChipExact
        ? '<span class="badge badge-microchip-exact">🔥 Exact Microchip Match</span>'
        : '<span class="badge badge-ai-match">⚡ High AI Vector Similarity</span>';

      const timeStatus = isContacted
        ? '<span class="reunification-time text-success">Contacted ✓</span>'
        : '<span class="reunification-time text-muted">Awaiting Dispatch</span>';

      const contactBtn = isContacted
        ? '<button type="button" class="btn btn-secondary btn-contact-owner" disabled>Contacted ✓</button>'
        : `<button type="button" class="btn btn-primary btn-contact-owner" data-action="contact-owner" data-match-id="${escapeHTML(item.matchId)}">🚨 Contact Owner &amp; Dispatch Alert</button>`;

      card.innerHTML = `
        <div class="reunification-header">
          ${priorityBadge}
          ${timeStatus}
        </div>
        <div class="reunification-comparison">
          <div class="reunification-entity evacuated-entity">
            <h4>Evacuated Animal</h4>
            <div class="entity-details">
              <p class="entity-name">${escapeHTML(item.petName || 'Displaced Pet')}</p>
              <p class="entity-chip">Chip: <code>${escapeHTML(item.microchipId || 'None')}</code></p>
              <p class="entity-facility text-secondary">${escapeHTML(item.currentHubName || item.currentHubId || 'Emergency Hub')}</p>
            </div>
          </div>
          <div class="reunification-arrow" aria-hidden="true">⇄</div>
          <div class="reunification-entity lost-entity">
            <h4>Registered Lost Report</h4>
            <div class="entity-details">
              <p class="entity-name">${escapeHTML(item.petName || 'Missing Pet')}</p>
              <p class="entity-chip">Chip: <code>${escapeHTML(item.microchipId || 'None')}</code></p>
              <p class="entity-owner text-secondary">${escapeHTML(item.ownerName || 'Owner on File')}${item.ownerContact ? ` (${escapeHTML(item.ownerContact)})` : ''}</p>
            </div>
          </div>
        </div>
        <div class="reunification-actions">
          ${contactBtn}
        </div>
      `;

      grid.appendChild(card);
    });
  }

  function initReunifications() {
    const grid = document.getElementById('reunifications-grid');
    if (!grid) return;

    grid.addEventListener('click', async (e) => {
      const btn = e.target.closest('button[data-action="contact-owner"]');
      if (!btn) return;

      const matchId = btn.getAttribute('data-match-id');
      if (!matchId) return;

      btn.disabled = true;
      btn.textContent = '⏳ Dispatching Alert...';

      try {
        const res = await fetch(`/api/v1/evacuations/reunification-queue/${encodeURIComponent(matchId)}/contact`, {
          method: 'POST',
        });

        if (!res.ok) {
          throw new Error(`Failed to contact owner: HTTP ${res.status}`);
        }

        btn.textContent = 'Contacted ✓';
        btn.classList.remove('btn-primary');
        btn.classList.add('btn-secondary');

        // Update header time in this card
        const card = btn.closest('.crisis-match-card');
        if (card) {
          const timeEl = card.querySelector('.reunification-time');
          if (timeEl) {
            timeEl.textContent = 'Contacted ✓';
            timeEl.className = 'reunification-time text-success';
          }
        }

        showToast('Emergency owner contact alert dispatched', 'success');
        announce('Owner contact alert successfully dispatched.');

        // Update pending metrics count
        const countEl = document.getElementById('metric-reunifications-pending');
        if (countEl) {
          const curVal = parseInt(countEl.textContent, 10) || 0;
          if (curVal > 0) {
            countEl.textContent = (curVal - 1).toLocaleString();
          }
        }
      } catch (err) {
        console.error(`Error contacting owner for match ${matchId}:`, err);
        btn.disabled = false;
        btn.textContent = '🚨 Contact Owner & Dispatch Alert';
        showToast('Failed to dispatch alert to owner', 'error');
      }
    });
  }

  // --- New Hub Registration Modal ---
  function initNewHubModal() {
    const openBtn = document.getElementById('btn-new-hub');
    const modal = document.getElementById('modal-new-hub');
    const form = document.getElementById('form-new-hub');

    if (openBtn && modal) {
      openBtn.addEventListener('click', () => {
        modal.classList.remove('hidden');
      });
    }

    if (modal) {
      modal.querySelectorAll('[data-close-modal="new-hub"]').forEach((el) => {
        el.addEventListener('click', () => {
          modal.classList.add('hidden');
        });
      });

      modal.addEventListener('click', (e) => {
        if (e.target === modal) {
          modal.classList.add('hidden');
        }
      });
    }

    if (form) {
      form.addEventListener('submit', async (e) => {
        e.preventDefault();

        const name = (form.elements['name']?.value || '').trim();
        const type = form.elements['type']?.value || 'POP_UP_CRISIS_CENTER';
        const address = (form.elements['address']?.value || '').trim();
        const lat = parseFloat(form.elements['latitude']?.value);
        const lng = parseFloat(form.elements['longitude']?.value);
        const totalCapacity = parseInt(form.elements['totalCapacity']?.value, 10);
        const dogCapacity = parseInt(form.elements['dogCapacity']?.value, 10);
        const catCapacity = parseInt(form.elements['catCapacity']?.value, 10);
        const contactName = (form.elements['contactName']?.value || '').trim();
        const contactPhone = (form.elements['contactPhone']?.value || '').trim();
        const contactEmail = (form.elements['contactEmail']?.value || '').trim();

        if (!name) {
          showToast('Facility name is required', 'warning');
          return;
        }

        if (isNaN(totalCapacity) || totalCapacity <= 0) {
          showToast('Total capacity must be greater than zero', 'warning');
          return;
        }

        const payload = {
          name,
          type,
          address,
          coordinates: {
            latitude: isNaN(lat) ? 0 : lat,
            longitude: isNaN(lng) ? 0 : lng,
          },
          totalCapacity,
          dogCapacity: isNaN(dogCapacity) ? 0 : dogCapacity,
          catCapacity: isNaN(catCapacity) ? 0 : catCapacity,
          contactName,
          contactPhone,
          contactEmail,
        };

        try {
          const res = await fetch('/api/v1/evacuations/hubs', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
          });

          if (!res.ok) {
            const errText = await res.text();
            throw new Error(errText || `Failed to create hub: HTTP ${res.status}`);
          }

          if (modal) modal.classList.add('hidden');
          form.reset();

          showToast(`Facility "${name}" registered successfully`, 'success');
          announce(`Emergency facility ${name} registered successfully.`);

          loadHubs();
        } catch (err) {
          console.error('Error registering facility hub:', err);
          showToast(`Error creating facility: ${err.message}`, 'error');
        }
      });
    }
  }

  // --- Keyboard Shortcuts & Global Handlers ---
  function initKeyboard() {
    window.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        const modals = document.querySelectorAll('.modal-overlay:not(.hidden)');
        modals.forEach((m) => m.classList.add('hidden'));
      }
    });
  }

  // --- Initialization ---
  function init() {
    initTabs();
    initBulkIntake();
    initTransfers();
    initReunifications();
    initNewHubModal();
    initKeyboard();

    // Initial load
    loadHubs();
    loadReunificationQueue();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
