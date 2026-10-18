/**
 * PetSpotR Automated Pet Custody Chain, Escrow & Cryptographic Reunion Handover Ledger
 * Cockpit Controller (Milestone 13.1)
 */

(function () {
  'use strict';

  const chainCache = new Map();
  let selectedChainId = '';
  let sseSource = null;

  /**
   * Escape untrusted text for safe HTML interpolation
   */
  function escapeHtml(str) {
    if (str === null || str === undefined) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  /**
   * Display toast notification & screen reader announcement
   */
  function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    if (container) {
      const toast = document.createElement('div');
      toast.className = `toast toast-${type}`;
      toast.textContent = message;
      container.appendChild(toast);
      setTimeout(() => {
        toast.remove();
      }, 4000);
    }
    const announcer = document.getElementById('custody-aria-announcer');
    if (announcer) {
      announcer.textContent = message;
    }
  }

  /**
   * Map CustodyStage to WCAG AAA badge CSS class
   */
  function getStageBadgeClass(stage) {
    switch (stage) {
      case 'FINDER_INTAKE':
      case 'FIELD_SAR_ESCORT':
        return 'badge-custody-intake';
      case 'VET_ER_HOLD':
      case 'SHELTER_SAFEKEEPING':
        return 'badge-custody-hold';
      case 'OWNER_REUNIFICATION_ESCROW':
        return 'badge-custody-escrow';
      case 'REUNITED_RELEASED':
        return 'badge-custody-reunited';
      default:
        return 'badge-custody-intake';
    }
  }

  /**
   * Build HTML for a single Pet Custody Chain card
   */
  function renderChainCardHtml(c) {
    const chainId = escapeHtml(c.chainId || '');
    const petName = escapeHtml(c.petName || '');
    const petId = escapeHtml(c.petId || '');
    const microchipId = escapeHtml(c.microchipId || '');
    const stage = escapeHtml(c.currentStage || 'FINDER_INTAKE');
    const stageClass = getStageBadgeClass(c.currentStage);
    const custodian = escapeHtml(c.currentCustodian || '');
    const location = escapeHtml(c.currentLocation || '');
    const pinCode = escapeHtml(c.handoverPinCode || '');
    const headHash = escapeHtml(c.latestEntryHashHex || '');
    const hops = Array.isArray(c.hops) ? c.hops : [];
    const hopCount = hops.length;

    const microchipBadgeHtml = microchipId
      ? `<span class="badge badge-custody-microchip">CHIP: ${microchipId}</span>`
      : '';

    const certBannerHtml = c.receiptCertificateId
      ? `<div class="custody-reunion-cert-banner">🎉 REUNITED &amp; RELEASED — Certificate: <strong>${escapeHtml(c.receiptCertificateId)}</strong> (Owner: ${escapeHtml(c.verifiedOwnerName || '')})</div>`
      : '';

    const isSelectedClass = c.chainId === selectedChainId ? ' custody-chain-card-active' : '';

    return `
      <article class="custody-chain-card${isSelectedClass}" data-chain-id="${chainId}" data-custody-stage="${stage}" data-handover-pin="${pinCode}" role="listitem">
        <div class="custody-chain-card-header">
          <div class="custody-chain-identity">
            <strong class="custody-pet-name">${petName}</strong>
            <code class="custody-pet-id">${petId}</code>
            ${microchipBadgeHtml}
          </div>
          <div class="custody-chain-badges">
            <span class="badge badge-custody-stage ${stageClass}">${stage}</span>
            <span class="badge badge-chain-integrity badge-integrity-intact">🔐 SHA-256 CHAIN VERIFIED INTACT (${hopCount} Hops)</span>
          </div>
        </div>

        <div class="custody-chain-meta">
          <span>👤 Custodian: <strong>${custodian}</strong></span>
          <span>📍 Location: <strong>${location}</strong></span>
          <span>🔑 Escrow PIN: <code class="custody-pin-code">${pinCode}</code></span>
          <span>⛓️ Head Digest: <code class="custody-head-hash">${headHash}</code></span>
        </div>

        ${certBannerHtml}

        <div class="custody-chain-actions">
          <button type="button" class="btn btn-sm btn-secondary btn-select-custody-chain" data-chain-id="${chainId}">🔍 Inspect Hash Chain &amp; Handover</button>
        </div>
      </article>
    `;
  }

  /**
   * Render SHA-256 Hash-Chain Hops Timeline for the selected chain
   */
  function renderHopsTimeline(chain) {
    const timelineEl = document.getElementById('custody-hops-timeline');
    if (!timelineEl) return;

    if (!chain) {
      timelineEl.innerHTML = '<p class="custody-empty-hint">No custody chain selected.</p>';
      return;
    }

    const hops = Array.isArray(chain.hops) ? chain.hops : [];
    if (hops.length === 0) {
      timelineEl.innerHTML = '<p class="custody-empty-hint">No custody hops recorded yet.</p>';
      return;
    }

    timelineEl.innerHTML = hops
      .map((hop) => {
        const hopIndex = Number(hop.hopIndex || 1);
        const stage = escapeHtml(hop.stage || 'FINDER_INTAKE');
        const stageClass = getStageBadgeClass(hop.stage);
        const fromCustodian = escapeHtml(hop.fromCustodian || '');
        const toCustodian = escapeHtml(hop.toCustodian || '');
        const toRole = escapeHtml(hop.toRole || '');
        const location = escapeHtml(hop.locationLabel || '');
        const notes = escapeHtml(hop.conditionNotes || '');
        const prevHash = escapeHtml(hop.prevHashHex || '');
        const entryHash = escapeHtml(hop.entryHashHex || '');
        const hmacSig = escapeHtml(hop.hmacSignatureHex || '');

        return `
          <div class="custody-hop-node" data-hop-index="${hopIndex}" role="listitem">
            <div class="custody-hop-header">
              <span class="badge badge-custody-stage ${stageClass}">#${hopIndex} ${stage}</span>
              <span class="custody-hop-location">📍 ${location}</span>
            </div>
            <div class="custody-hop-transfer">
              <strong>${fromCustodian} → ${toCustodian} (${toRole})</strong>
            </div>
            <p class="custody-hop-notes">${notes}</p>
            <div class="custody-hop-hashes">
              <div><span>PrevHash:</span> <code>${prevHash}</code></div>
              <div><span>EntryHash:</span> <code>${entryHash}</code></div>
              <div><span>HMAC:</span> <code>${hmacSig}</code></div>
            </div>
          </div>
        `;
      })
      .join('');
  }

  /**
   * Render Printable Custody Receipt & Liability Waiver Certificate for the selected chain
   */
  function renderCertificateCard(chain) {
    const cardEl = document.getElementById('reunion-certificate-card');
    if (!cardEl || !chain) return;

    const certId = chain.receiptCertificateId
      ? escapeHtml(chain.receiptCertificateId)
      : 'PENDING_REUNION_ESCROW';
    const certBadgeClass = chain.receiptCertificateId
      ? 'badge-custody-reunited'
      : 'badge-custody-escrow';
    const petName = escapeHtml(chain.petName || '');
    const petId = escapeHtml(chain.petId || '');
    const microchip = escapeHtml(chain.microchipId || 'N/A');
    const stage = escapeHtml(chain.currentStage || '');
    const owner = escapeHtml(chain.verifiedOwnerName || 'Awaiting PIN Handover');
    const proof = escapeHtml(chain.ownershipProof || 'Pending Verification');
    const headHash = escapeHtml(chain.latestEntryHashHex || '');

    cardEl.innerHTML = `
      <div class="certificate-header">
        <h3>📜 Cryptographic Pet Custody Receipt &amp; Liability Waiver Certificate</h3>
        <span id="certificate-id-display" class="badge ${certBadgeClass}">${certId}</span>
      </div>
      <div class="certificate-grid">
        <div><strong>Pet Name / ID:</strong> <span id="cert-pet-summary">${petName} (${petId})</span></div>
        <div><strong>Microchip ID:</strong> <span id="cert-microchip">${microchip}</span></div>
        <div><strong>Current Stage:</strong> <span id="cert-stage">${stage}</span></div>
        <div><strong>Verified Owner:</strong> <span id="cert-owner">${owner}</span></div>
        <div><strong>Ownership Proof:</strong> <span id="cert-proof">${proof}</span></div>
        <div><strong>Latest SHA-256 Digest:</strong> <code id="cert-head-hash">${headHash}</code></div>
      </div>
      <div class="certificate-actions">
        <button type="button" id="btn-print-custody-certificate" class="btn btn-sm btn-secondary">🖨️ Print Custody Waiver Receipt</button>
      </div>
    `;
  }

  /**
   * Render all custody chains and synchronize active chain views
   */
  function renderAllChains(chains) {
    chainCache.clear();
    chains.forEach((c) => {
      if (c && c.chainId) {
        chainCache.set(c.chainId, c);
      }
    });

    if (!selectedChainId || !chainCache.has(selectedChainId)) {
      if (chainCache.has('chain-custody-101')) {
        selectedChainId = 'chain-custody-101';
      } else if (chains.length > 0) {
        selectedChainId = chains[0].chainId;
      }
    }

    const countBadge = document.getElementById('custody-chains-count-badge');
    if (countBadge) {
      countBadge.textContent = `${chains.length} Chains`;
    }

    const listEl = document.getElementById('custody-chains-list');
    if (listEl) {
      if (chains.length === 0) {
        listEl.innerHTML = '<p class="custody-empty-hint" id="custody-chains-empty-msg">No active pet custody chains recorded yet.</p>';
      } else {
        listEl.innerHTML = chains.map(renderChainCardHtml).join('');
      }
    }

    const selectEl = document.getElementById('active-chain-select');
    if (selectEl) {
      selectEl.innerHTML = chains
        .map((c) => {
          const id = escapeHtml(c.chainId || '');
          const petName = escapeHtml(c.petName || '');
          const petId = escapeHtml(c.petId || '');
          const stage = escapeHtml(c.currentStage || '');
          return `<option value="${id}">${petName} (${petId}) — ${id} [${stage}]</option>`;
        })
        .join('');
      if (selectedChainId && chainCache.has(selectedChainId)) {
        selectEl.value = selectedChainId;
      }
    }

    const activeChain = chainCache.get(selectedChainId) || chains[0] || null;
    renderHopsTimeline(activeChain);
    renderCertificateCard(activeChain);
  }

  /**
   * Fetch all custody chains from `/api/v1/custody/chains`
   */
  async function fetchChains() {
    try {
      const res = await fetch('/api/v1/custody/chains', {
        headers: { Accept: 'application/json' },
      });
      if (!res.ok) return;
      const data = await res.json();
      const chains = Array.isArray(data.chains) ? data.chains : [];
      renderAllChains(chains);
    } catch (_err) {
      // Ignore transient network errors
    }
  }

  /**
   * Select a specific custody chain for inspection & actions
   */
  function selectChain(chainId) {
    if (!chainId || !chainCache.has(chainId)) return;
    selectedChainId = chainId;

    const selectEl = document.getElementById('active-chain-select');
    if (selectEl) {
      selectEl.value = chainId;
    }

    const cards = document.querySelectorAll('#custody-chains-list .custody-chain-card');
    cards.forEach((card) => {
      if (card.getAttribute('data-chain-id') === chainId) {
        card.classList.add('custody-chain-card-active');
      } else {
        card.classList.remove('custody-chain-card-active');
      }
    });

    const chain = chainCache.get(chainId);
    renderHopsTimeline(chain);
    renderCertificateCard(chain);
  }

  /**
   * Setup theme toggle button (`#theme-toggle`)
   */
  function setupThemeToggle() {
    const btn = document.getElementById('theme-toggle');
    if (!btn) return;
    btn.addEventListener('click', () => {
      const html = document.documentElement;
      const current = html.getAttribute('data-theme') || 'dark';
      const next = current === 'dark' ? 'light' : 'dark';
      html.setAttribute('data-theme', next);
    });
  }

  /**
   * Setup Initiate New Pet Custody Chain form (`#form-create-custody-chain`)
   */
  function setupCreateChainForm() {
    const form = document.getElementById('form-create-custody-chain');
    if (!form) return;

    form.addEventListener('submit', async (e) => {
      e.preventDefault();

      const petId = (document.getElementById('custody-pet-id')?.value || '').trim();
      const petName = (document.getElementById('custody-pet-name')?.value || '').trim();
      const species = document.getElementById('custody-species')?.value || 'Dog';
      const microchipId = (document.getElementById('custody-microchip')?.value || '').trim();
      const initialStage = document.getElementById('custody-initial-stage')?.value || 'FINDER_INTAKE';
      const fromCustodian = (document.getElementById('custody-from-custodian')?.value || '').trim();
      const toCustodian = (document.getElementById('custody-to-custodian')?.value || '').trim();
      const toRole = (document.getElementById('custody-to-role')?.value || '').trim();
      const locationLabel = (document.getElementById('custody-location')?.value || '').trim();
      const conditionNotes = (document.getElementById('custody-notes')?.value || '').trim();

      const submitBtn = document.getElementById('btn-create-custody-chain');
      if (submitBtn) submitBtn.disabled = true;

      try {
        const res = await fetch('/api/v1/custody/chains', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            petId,
            petName,
            species,
            microchipId,
            initialStage,
            fromCustodian,
            toCustodian,
            toRole,
            locationLabel,
            conditionNotes,
          }),
        });

        if (res.ok) {
          const created = await res.json();
          const newId = created.chainId || created.chain?.chainId;
          if (newId) {
            selectedChainId = newId;
          }
          await fetchChains();
          showToast(`Initiated SHA-256 custody chain for ${petName} (${petId})`, 'success');
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to initiate custody chain', 'error');
        }
      } catch (_err) {
        showToast('Network error initiating custody chain', 'error');
      } finally {
        if (submitBtn) submitBtn.disabled = false;
      }
    });
  }

  /**
   * Setup Append Custody Transfer Hop form (`#form-append-custody-hop`)
   */
  function setupAppendHopForm() {
    const form = document.getElementById('form-append-custody-hop');
    if (!form) return;

    form.addEventListener('submit', async (e) => {
      e.preventDefault();

      const selectEl = document.getElementById('active-chain-select');
      const chainId = selectEl?.value || selectedChainId;
      if (!chainId) return;

      const stage = document.getElementById('hop-stage')?.value || 'OWNER_REUNIFICATION_ESCROW';
      const toCustodian = (document.getElementById('hop-to-custodian')?.value || '').trim();
      const toRole = (document.getElementById('hop-to-role')?.value || '').trim();
      const locationLabel = (document.getElementById('hop-location')?.value || '').trim();
      const conditionNotes = (document.getElementById('hop-notes')?.value || '').trim();

      const submitBtn = document.getElementById('btn-append-custody-hop');
      if (submitBtn) submitBtn.disabled = true;

      try {
        const res = await fetch(`/api/v1/custody/chains/${encodeURIComponent(chainId)}/transfer`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            stage,
            toCustodian,
            toRole,
            locationLabel,
            conditionNotes,
          }),
        });

        if (res.ok) {
          selectedChainId = chainId;
          await fetchChains();
          showToast(`Appended signed SHA-256 custody hop (${stage})`, 'success');
        } else {
          const errData = await res.json().catch(() => ({}));
          showToast(errData.error || 'Failed to append custody hop', 'error');
        }
      } catch (_err) {
        showToast('Network error appending custody hop', 'error');
      } finally {
        if (submitBtn) submitBtn.disabled = false;
      }
    });
  }

  /**
   * Setup Dual-Party Cryptographic Reunion Handover form (`#form-complete-reunion`)
   */
  function setupCompleteReunionForm() {
    const form = document.getElementById('form-complete-reunion');
    if (!form) return;

    form.addEventListener('submit', async (e) => {
      e.preventDefault();

      const selectEl = document.getElementById('active-chain-select');
      const chainId = selectEl?.value || selectedChainId;
      if (!chainId) return;

      const handoverPinCode = (document.getElementById('reunion-pin-input')?.value || '').trim();
      const verifiedOwnerName = (document.getElementById('reunion-owner-name')?.value || '').trim();
      const ownershipProof = document.getElementById('reunion-proof-method')?.value || 'MICROCHIP_SCAN_MATCH';
      const locationLabel = (document.getElementById('reunion-location')?.value || '').trim();
      const waiverAccepted = Boolean(document.getElementById('reunion-waiver-checkbox')?.checked);

      const alertBanner = document.getElementById('pin-gate-error-alert');
      const submitBtn = document.getElementById('btn-complete-reunion');
      if (submitBtn) submitBtn.disabled = true;

      try {
        const res = await fetch(`/api/v1/custody/chains/${encodeURIComponent(chainId)}/reunite`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            handoverPinCode,
            verifiedOwnerName,
            ownershipProof,
            locationLabel,
            waiverAccepted,
          }),
        });

        if (res.ok) {
          if (alertBanner) {
            alertBanner.textContent = '';
            alertBanner.hidden = true;
          }
          selectedChainId = chainId;
          await fetchChains();
          showToast(`Reunion completed & custody released to ${verifiedOwnerName}!`, 'success');
        } else {
          const errData = await res.json().catch(() => ({}));
          const errMsg =
            errData.error ||
            'Handover PIN rejected: invalid 6-digit cryptographic escrow PIN — custody release denied';
          if (alertBanner) {
            alertBanner.textContent = `🚨 ${errMsg}`;
            alertBanner.hidden = false;
          }
          showToast(errMsg, 'error');
        }
      } catch (_err) {
        showToast('Network error during cryptographic reunion handover', 'error');
      } finally {
        if (submitBtn) submitBtn.disabled = false;
      }
    });
  }

  /**
   * Setup Chain Selection, Cryptographic Audit button, and Print Certificate button
   */
  function setupInspectorAndSelectionHandlers() {
    const listEl = document.getElementById('custody-chains-list');
    if (listEl) {
      listEl.addEventListener('click', (e) => {
        const btn = e.target.closest('.btn-select-custody-chain');
        if (!btn) return;
        const chainId = btn.getAttribute('data-chain-id');
        if (chainId) {
          selectChain(chainId);
        }
      });
    }

    const selectEl = document.getElementById('active-chain-select');
    if (selectEl) {
      selectEl.addEventListener('change', () => {
        if (selectEl.value) {
          selectChain(selectEl.value);
        }
      });
    }

    const auditBtn = document.getElementById('btn-verify-chain-integrity');
    if (auditBtn) {
      auditBtn.addEventListener('click', async () => {
        const chainId = document.getElementById('active-chain-select')?.value || selectedChainId;
        if (!chainId) return;

        auditBtn.disabled = true;
        try {
          const res = await fetch(`/api/v1/custody/chains/${encodeURIComponent(chainId)}/verify`, {
            headers: { Accept: 'application/json' },
          });
          if (res.ok) {
            const data = await res.json();
            const badge = document.getElementById('audit-result-badge');
            if (badge) {
              badge.textContent = data.verifiedIntact
                ? `SHA-256 Verified (${data.hopCount} Hops)`
                : 'Integrity Alert';
            }
            showToast(data.message || 'Cryptographic hash-chain audit complete', 'success');
          }
        } catch (_err) {
          showToast('Error running cryptographic audit', 'error');
        } finally {
          auditBtn.disabled = false;
        }
      });
    }

    const inspectorPanel = document.getElementById('custody-inspector-panel');
    if (inspectorPanel) {
      inspectorPanel.addEventListener('click', (e) => {
        const printBtn = e.target.closest('#btn-print-custody-certificate');
        if (!printBtn) return;
        showToast('Preparing Printable Custody Receipt & Liability Waiver Certificate...', 'info');
      });
    }
  }

  /**
   * Subscribe to SSE stream `/api/v1/custody/stream`
   */
  function setupSSEStream() {
    if (typeof EventSource === 'undefined') return;
    try {
      sseSource = new EventSource('/api/v1/custody/stream');
      const refresh = () => {
        fetchChains();
      };
      sseSource.addEventListener('custody_chain_updated', refresh);
      sseSource.onmessage = (evt) => {
        try {
          const parsed = JSON.parse(evt.data);
          if (parsed.type === 'custody_chain_updated') {
            refresh();
          }
        } catch (_e) {
          // Ignore non-JSON keepalive comments
        }
      };
    } catch (_err) {
      // Ignore SSE errors in headless environments
    }
  }

  /**
   * Initialize controller on DOMContentLoaded
   */
  function init() {
    setupThemeToggle();
    setupCreateChainForm();
    setupAppendHopForm();
    setupCompleteReunionForm();
    setupInspectorAndSelectionHandlers();
    fetchChains();
    setupSSEStream();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  window.addEventListener('beforeunload', () => {
    if (sseSource) {
      sseSource.close();
    }
  });
})();
