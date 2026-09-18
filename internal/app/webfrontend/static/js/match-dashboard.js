// Client-side controller for Pet Match Comparison Dashboard
document.addEventListener('DOMContentLoaded', () => {
  const container = document.getElementById('matches-list-container');
  const scoreFilter = document.getElementById('scoreFilter');
  const zoomModal = document.getElementById('zoom-modal');
  const zoomedImage = document.getElementById('zoomed-image');
  const decisionStatus = document.getElementById('match-decision-status');
  const threadModal = document.getElementById('match-thread-modal');
  const threadMessages = document.getElementById('match-thread-messages');
  const threadEmpty = document.getElementById('match-thread-empty');
  const threadReadOnly = document.getElementById('match-thread-readonly');
  const threadForm = document.getElementById('match-thread-form');
  const threadMessage = document.getElementById('match-thread-message');
  const threadSend = document.getElementById('match-thread-send');
  const threadStatus = document.getElementById('match-thread-status');
  const threadError = document.getElementById('match-thread-error');
  const threadPresence = document.getElementById('match-thread-presence');
  const presenceDot = document.getElementById('match-presence-dot');
  const presenceText = document.getElementById('match-presence-text');
  const threadTyping = document.getElementById('match-thread-typing');
  const threadTypingText = document.getElementById('match-thread-typing-text');
  const threadResolvedBanner = document.getElementById('match-thread-resolved-banner');
  const btnChatResolve = document.getElementById('btn-chat-resolve-reunion');
  const threadAttachBtn = document.getElementById('match-thread-attach-btn');
  const threadFileInput = document.getElementById('match-thread-file-input');
  const threadStagedTray = document.getElementById('match-thread-staged-tray');
  const threadStagedThumbs = document.getElementById('match-thread-staged-thumbs');

  let allMatches = [];
  let matchResultsLoaded = false;
  let identityEnabled = false;
  let matchLoadRevision = 0;
  let lastIdentityState = '';
  let decisionReturnTarget = null;
  let decisionInFlight = false;
  let identityRevision = 0;
  let identityPrincipalKey = '';
  let activeThread = null;
  let threadRevision = 0;
  let threadSendInFlight = false;
  let activeThreadSendToken = null;
  let threadReturnTarget = null;
  let lastZoomTrigger = null;
  let lastContactTrigger = null;
  let lastReunionTrigger = null;
  let pendingThreadAttempt = null;
  let threadIdentityFocusPending = false;
  let activeEventSource = null;
  let typingAutoHideTimer = null;
  let typingDebounceTimer = null;
  let lastTypingPingTime = 0;
  let stagedPhotos = [];
  const matchStatuses = new Set(['PENDING_REVIEW', 'CONFIRMED', 'REJECTED', 'REUNITED']);
  const allowedImageHosts = new Set(['storage.petspotr.io']);

  function openModal(modal) {
    if (modal) modal.hidden = false;
  }

  function closeModal(modal) {
    if (modal) modal.hidden = true;
  }
  function closeZoomModal() {
    if (!zoomModal || zoomModal.hidden) return;
    closeModal(zoomModal);
    const trigger = lastZoomTrigger;
    lastZoomTrigger = null;
    trigger?.focus();
  }

  function closeContactModal() {
    const contactModal = document.getElementById('contact-modal');
    closeModal(contactModal);
    const trigger = lastContactTrigger;
    lastContactTrigger = null;
    (trigger || scoreFilter)?.focus();
  }

  function closeReunionModal() {
    const reunionModal = document.getElementById('reunion-modal');
    closeModal(reunionModal);
    const trigger = lastReunionTrigger;
    lastReunionTrigger = null;
    (trigger || scoreFilter)?.focus();
  }


  function setDecisionBusy(button, busy) {
    container?.querySelectorAll('.action-btn').forEach(actionButton => {
      actionButton.setAttribute('aria-disabled', String(busy));
    });
    button.textContent = busy ? 'Saving decision...' : button.dataset.idleText;
    if (decisionStatus) {
      decisionStatus.textContent = busy ? 'Saving match decision...' : '';
      decisionStatus.hidden = !busy;
    }
  }

  function findDecisionReturnButton() {
    if (!decisionReturnTarget || !container) return null;
    return Array.from(container.querySelectorAll('.action-btn')).find(button =>
      button.dataset.matchId === decisionReturnTarget.matchId &&
      button.dataset.action === decisionReturnTarget.action,
    ) || null;
  }

  function closeActionModal() {
    const modal = document.getElementById('match-action-modal');
    closeModal(modal);
    const returnButton = findDecisionReturnButton();
    decisionReturnTarget = null;
    (returnButton || scoreFilter)?.focus();
  }

  function clearThreadStatus() {
    if (threadStatus) {
      threadStatus.textContent = '';
      threadStatus.hidden = true;
    }
    if (threadError) {
      threadError.textContent = '';
      threadError.hidden = true;
    }
  }

  function setThreadStatus(message, isError = false) {
    clearThreadStatus();
    const target = isError ? threadError : threadStatus;
    if (!target) return;
    target.textContent = message;
    target.hidden = false;
  }

  function setThreadSendBusy(busy) {
    threadSendInFlight = busy;
    if (threadForm) threadForm.setAttribute('aria-busy', String(busy));
    if (threadSend) {
      threadSend.setAttribute('aria-disabled', String(busy));
      threadSend.textContent = busy ? 'Sending message...' : 'Send message';
    }
    if (threadAttachBtn) {
      threadAttachBtn.setAttribute('aria-disabled', String(busy));
    }
    if (threadMessage) threadMessage.readOnly = busy;
    if (busy) setThreadStatus('Sending private message...');
  }

  function findThreadReturnButton() {
    if (!threadReturnTarget || !container) return null;
    return container.querySelector(`.message-btn[data-match-id="${CSS.escape(threadReturnTarget)}"]`);
  }

  function setPresenceUI(status) {
    if (status === 'online') {
      if (presenceDot) {
        presenceDot.classList.remove('status-offline', 'status-typing');
        presenceDot.classList.add('status-online');
      }
      if (threadPresence) {
        threadPresence.classList.remove('status-offline', 'status-typing');
        threadPresence.classList.add('status-online');
      }
      if (presenceText) presenceText.textContent = 'Online';
    } else if (status === 'connecting') {
      if (presenceDot) {
        presenceDot.classList.remove('status-online', 'status-typing');
        presenceDot.classList.add('status-offline');
      }
      if (threadPresence) {
        threadPresence.classList.remove('status-online', 'status-typing');
        threadPresence.classList.add('status-offline');
      }
      if (presenceText) presenceText.textContent = 'Connecting...';
    } else {
      if (presenceDot) {
        presenceDot.classList.remove('status-online', 'status-typing');
        presenceDot.classList.add('status-offline');
      }
      if (threadPresence) {
        threadPresence.classList.remove('status-online', 'status-typing');
        threadPresence.classList.add('status-offline');
      }
      if (presenceText) presenceText.textContent = 'Offline';
    }
  }

  async function getCsrfToken() {
    let token = window.petspotrIdentity?.getState?.()?.csrfToken;
    if (!token && window.petspotrIdentity?.requireSession) {
      try {
        const state = await window.petspotrIdentity.requireSession();
        token = state?.csrfToken;
      } catch (_) {}
    }
    if (!token && typeof document !== 'undefined' && document.cookie) {
      const match = document.cookie.match(/(?:^|;\s*)petspotr_csrf(?:_sec)?=([^;]+)/);
      if (match) token = decodeURIComponent(match[1]);
    }
    return token || '';
  }

  async function sendPresence(matchId, status, keepalive = false) {
    if (!matchId) return;
    try {
      const csrfToken = await getCsrfToken();
      const headers = { 'Content-Type': 'application/json' };
      if (csrfToken) headers['X-CSRF-Token'] = csrfToken;
      await fetch('/api/v1/reunions/presence', {
        method: 'POST',
        headers,
        body: JSON.stringify({ matchId, status }),
        keepalive,
      });
    } catch (_) {
      // Best-effort presence update
    }
  }

  function handleStagedFiles(files) {
    if (!files || files.length === 0) return;
    const remaining = 3 - stagedPhotos.length;
    if (remaining <= 0) {
      setThreadStatus('Maximum 3 photos allowed.', true);
      return;
    }
    const toProcess = Array.from(files).slice(0, remaining);
    if (files.length > remaining) {
      setThreadStatus(`Only ${remaining} more photo(s) can be added (maximum 3).`, true);
    }
    for (const file of toProcess) {
      if (!file.type || !file.type.startsWith('image/')) {
        setThreadStatus('Please select an image file (JPEG, PNG, WebP).', true);
        continue;
      }
      if (file.size > 10 * 1024 * 1024) {
        setThreadStatus('Photo file size exceeds 10MB limit.', true);
        continue;
      }
      let previewUrl = '';
      try {
        previewUrl = URL.createObjectURL(file);
      } catch (_) {
        previewUrl = '';
      }
      stagedPhotos.push({ file, previewUrl });
    }
    if (threadFileInput) threadFileInput.value = '';
    renderStagedPhotos();
  }

  function renderStagedPhotos() {
    if (!threadStagedTray || !threadStagedThumbs) return;
    threadStagedTray.hidden = stagedPhotos.length === 0;
    threadStagedThumbs.replaceChildren();

    stagedPhotos.forEach((item, index) => {
      const chip = createElement('div', { className: 'staged-thumb-chip' });
      if (item.previewUrl) {
        const img = createElement('img', { className: 'staged-thumb-img' });
        img.src = item.previewUrl;
        img.alt = `Staged photo ${index + 1}`;
        chip.append(img);
      } else {
        const label = createElement('span', { text: item.file.name || `Photo ${index + 1}` });
        chip.append(label);
      }

      const removeBtn = createElement('button', {
        className: 'btn-remove-thumb staged-chip-remove',
        text: '×',
      });
      removeBtn.type = 'button';
      removeBtn.setAttribute('aria-label', `Remove photo ${index + 1}`);
      removeBtn.addEventListener('click', () => {
        if (item.previewUrl) {
          try { URL.revokeObjectURL(item.previewUrl); } catch (_) {}
        }
        stagedPhotos.splice(index, 1);
        renderStagedPhotos();
      });

      chip.append(removeBtn);
      threadStagedThumbs.append(chip);
    });
  }

  function clearStagedPhotos() {
    stagedPhotos.forEach(item => {
      if (item.previewUrl) {
        try { URL.revokeObjectURL(item.previewUrl); } catch (_) {}
      }
    });
    stagedPhotos = [];
    if (threadFileInput) threadFileInput.value = '';
    renderStagedPhotos();
  }

  function resetThreadView() {
    threadMessages?.replaceChildren();
    if (threadEmpty) threadEmpty.hidden = true;
    if (threadReadOnly) threadReadOnly.hidden = true;
    if (threadForm) threadForm.hidden = false;
    if (threadMessage) threadMessage.value = '';
    if (threadResolvedBanner) threadResolvedBanner.hidden = true;
    if (threadTyping) threadTyping.hidden = true;
    if (btnChatResolve) btnChatResolve.disabled = false;
    clearStagedPhotos();
    setPresenceUI('offline');
    activeThreadSendToken = null;
    setThreadSendBusy(false);
    clearThreadStatus();
    pendingThreadAttempt = null;
  }

  function closeThreadModal(restoreFocus = true) {
    threadRevision += 1;
    const returnButton = restoreFocus ? findThreadReturnButton() : null;
    const closingMatchId = activeThread?.matchId;
    activeThread = null;
    threadReturnTarget = null;

    if (closingMatchId) {
      void sendPresence(closingMatchId, 'idle');
    }

    if (activeEventSource) {
      activeEventSource.close();
      activeEventSource = null;
    }

    if (typingDebounceTimer) {
      clearTimeout(typingDebounceTimer);
      typingDebounceTimer = null;
    }
    if (typingAutoHideTimer) {
      clearTimeout(typingAutoHideTimer);
      typingAutoHideTimer = null;
    }
    lastTypingPingTime = 0;

    resetThreadView();
    closeModal(threadModal);
    if (restoreFocus) (returnButton || scoreFilter)?.focus();
  }

  function validMediatedMessageBody(value) {
    if (typeof value !== 'string' || value.length === 0 || Array.from(value).length > 1000) return false;
    return !Array.from(value).some(char => {
      const code = char.codePointAt(0);
      return (code < 0x20 && char !== '\n' && char !== '\r' && char !== '\t') || code === 0x7f;
    });
  }

  function resolveImageUrl(val) {
    if (typeof val !== 'string' || !val.trim()) return null;
    let url = val.trim();
    if (!url.startsWith('https://') && !url.startsWith('http://') && !url.startsWith('/')) {
      url = 'https://storage.petspotr.io/' + url.replace(/^\/+/, '');
    }
    return trustedImageURL(url);
  }

  function normalizeThreadMessage(value) {
    if (!value || typeof value !== 'object' || !validRecordId(value.messageId)) return null;
    if (value.senderRole !== 'reporter' && value.senderRole !== 'finder') return null;
    if (!validMediatedMessageBody(value.message)) return null;
    if (typeof value.sentAt !== 'string' || value.sentAt.length > 64) return null;
    const sentAt = new Date(value.sentAt);
    if (Number.isNaN(sentAt.getTime())) return null;

    let images = [];
    if (Array.isArray(value.images)) {
      images = value.images.map(img => {
        if (typeof img !== 'string' || img.length === 0 || img.length > 2048) return null;
        return resolveImageUrl(img);
      }).filter(img => img !== null).slice(0, 3);
    }

    return {
      messageId: value.messageId,
      senderRole: value.senderRole,
      message: value.message,
      images,
      sentAt,
    };
  }

  function createThreadMessageElement(message) {
    const item = createElement('li', { className: 'match-thread-message' });
    if (message.messageId) {
      item.dataset.messageId = message.messageId;
    }
    const meta = createElement('div', { className: 'match-thread-message-meta' });
    meta.append(
      createElement('strong', { text: message.senderRole === 'reporter' ? 'Reporter' : 'Finder' }),
      createElement('time', { text: message.sentAt.toLocaleString() }),
    );
    const body = createElement('p', { text: message.message, className: 'match-thread-message-body' });
    item.append(meta, body);

    if (message.images && message.images.length > 0) {
      const attachWrap = createElement('div', { className: 'staged-thumbs' });
      message.images.forEach(imgUrl => {
        const img = createElement('img', { className: 'thumbnail-img' });
        img.src = imgUrl;
        img.alt = 'Verification photo attachment';
        img.loading = 'lazy';
        img.addEventListener('click', () => {
          lastZoomTrigger = img;
          if (zoomedImage && zoomModal) {
            zoomedImage.src = imgUrl;
            openModal(zoomModal);
          }
        });
        attachWrap.append(img);
      });
      item.append(attachWrap);
    }
    return item;
  }

  function renderThreadMessages(messages) {
    if (!threadMessages) return;
    const normalized = Array.isArray(messages)
      ? messages.map(normalizeThreadMessage).filter(message => message !== null).slice(0, 100)
      : [];
    const items = normalized.map(createThreadMessageElement);
    threadMessages.replaceChildren(...items);
    if (threadEmpty) threadEmpty.hidden = items.length !== 0;
    threadMessages.scrollTop = threadMessages.scrollHeight;
  }

  function appendThreadMessage(raw) {
    if (!threadMessages) return;
    const message = normalizeThreadMessage(raw);
    if (!message) return;
    const escapeId = typeof CSS !== 'undefined' && CSS.escape
      ? CSS.escape(message.messageId)
      : message.messageId.replace(/["\\]/g, '\\$&');
    if (threadMessages.querySelector(`[data-message-id="${escapeId}"]`)) {
      return;
    }
    const item = createThreadMessageElement(message);
    threadMessages.append(item);
    if (threadEmpty) threadEmpty.hidden = true;
    threadMessages.scrollTop = threadMessages.scrollHeight;
    try {
      item.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    } catch (_) {}
  }

  function threadIsWritable(status) {
    return status === 'PENDING_REVIEW' || status === 'CONFIRMED';
  }

  function applyThreadWritableState(status) {
    const writable = threadIsWritable(status);
    if (threadForm) threadForm.hidden = !writable;
    if (threadReadOnly) threadReadOnly.hidden = writable;
  }

  function newThreadIdempotencyKey() {
    if (typeof crypto.randomUUID === 'function') return `thread-${crypto.randomUUID()}`;
    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    return `thread-${Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')}`;
  }

  function createElement(tagName, options = {}) {
    const element = document.createElement(tagName);
    if (options.className) element.className = options.className;
    if (options.text !== undefined) element.textContent = options.text;
    return element;
  }

  function renderMatchState(message, className, role) {
    allMatches = [];
    matchResultsLoaded = false;
    if (!container) return;
    if (!message) {
      container.replaceChildren();
      return;
    }
    const stateMessage = createElement('p', { text: message, className });
    stateMessage.setAttribute('role', role);
    if (role === 'status') stateMessage.setAttribute('aria-live', 'polite');
    container.replaceChildren(stateMessage);
  }

  function replaceMatchState(message, className) {
    matchLoadRevision += 1;
    renderMatchState(message, className, 'status');
  }

  function validString(value, allowEmpty = false) {
    return typeof value === 'string' && (allowEmpty || value.trim().length > 0);
  }

  function validRecordId(value) {
    return validString(value);
  }

  function validScore(value) {
    return typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= 1;
  }

  function trustedImageURL(value) {
    if (typeof value !== 'string' || value.length === 0 || value.length > 2048) return null;

    let parsed;
    try {
      parsed = new URL(value, window.location.origin);
    } catch (_) {
      return null;
    }

    if (parsed.username || parsed.password) return null;
    if (parsed.origin === window.location.origin && (parsed.protocol === 'http:' || parsed.protocol === 'https:')) {
      return parsed.href;
    }
    if (parsed.protocol !== 'https:') return null;
    if (allowedImageHosts.has(parsed.hostname)) return parsed.href;
    return null;
  }

  function normalizePet(value, requireName) {
    if (!value || typeof value !== 'object' || !validRecordId(value.petId)) return null;
    const petName = value.petName ?? '';
    if (!validString(petName, !requireName) || !validString(value.breed) || !validString(value.location)) {
      return null;
    }

    let images = [];
    if (Array.isArray(value.images)) {
      images = value.images.map(img => {
        if (!img) return null;
        if (typeof img === 'string') {
          const url = trustedImageURL(img);
          return url ? { url, tag: '' } : null;
        }
        if (typeof img === 'object') {
          const raw = img.url || img.imageUrl || img.object;
          const url = trustedImageURL(raw);
          if (!url) return null;
          return {
            url,
            tag: typeof img.tag === 'string' ? img.tag.trim() : '',
          };
        }
        return null;
      }).filter(img => img !== null);
    }

    const trustedMainUrl = trustedImageURL(value.imageUrl);
    const imageUrl = trustedMainUrl || (images.length > 0 ? images[0].url : null);

    return {
      petId: value.petId,
      petName,
      breed: value.breed,
      location: value.location,
      imageUrl,
      images,
    };
  }

  function normalizeMatch(value) {
    if (!value || typeof value !== 'object') return null;
    if (!validRecordId(value.matchId) || !validRecordId(value.foundPetId) || !validRecordId(value.matchedPetId)) {
      return null;
    }
    if (!validScore(value.score) || !matchStatuses.has(value.status)) return null;
    if (!value.scores || typeof value.scores !== 'object') return null;
    if (!validScore(value.scores.visual) || !validScore(value.scores.color) || !validScore(value.scores.spatial)) {
      return null;
    }
    if (typeof value.scores.distanceMiles !== 'number' || !Number.isFinite(value.scores.distanceMiles) ||
        value.scores.distanceMiles < 0 || value.scores.distanceMiles > 25000) {
      return null;
    }

    if (typeof value.matchedAt !== 'string' || value.matchedAt.length > 64) return null;
    const matchedAt = new Date(value.matchedAt);
    if (Number.isNaN(matchedAt.getTime())) return null;
    const lostPet = normalizePet(value.lostPet, true);
    const foundPet = normalizePet(value.foundPet, false);
    if (!lostPet || !foundPet) return null;

    const scores = {
      visual: value.scores.visual,
      color: value.scores.color,
      spatial: value.scores.spatial,
      distanceMiles: value.scores.distanceMiles,
    };
    if (value.scores.vector !== undefined && value.scores.vector !== null && validScore(value.scores.vector)) {
      scores.vector = value.scores.vector;
    }
    if (value.scores.trait !== undefined && value.scores.trait !== null && validScore(value.scores.trait)) {
      scores.trait = value.scores.trait;
    }

    return {
      matchId: value.matchId,
      foundPetId: value.foundPetId,
      matchedPetId: value.matchedPetId,
      score: value.score,
      status: value.status,
      matchedAt,
      scores,
      lostPet,
      foundPet,
    };
  }

  async function fetchMatches() {
    const loadRevision = ++matchLoadRevision;
    renderMatchState('Loading match records...', 'text-secondary', 'status');
    try {
      const resp = await fetch('/api/v1/matches');
      if (!resp.ok) throw new Error(`Match API returned status ${resp.status}`);
      const payload = await resp.json();
      if (!Array.isArray(payload)) throw new Error('Match API returned a non-array payload');
      if (loadRevision !== matchLoadRevision) return;
      allMatches = payload.map(normalizeMatch).filter(match => match !== null);
      matchResultsLoaded = true;
      renderMatches();
    } catch (err) {
      if (loadRevision !== matchLoadRevision) return;
      console.error('Failed to fetch matches:', err);
      renderMatchState('Failed to load match records.', 'match-load-error', 'alert');
    }
  }

  function renderMatches() {
    if (!container) return;
    const minScore = parseFloat(scoreFilter?.value || '0.70');
    const filtered = allMatches.filter(m => m.score >= minScore);

    if (filtered.length === 0) {
      const emptyCard = createElement('div', {
        className: 'glass-card match-empty',
      });
      const emptyIcon = createElement('div', {
        text: '⌕',
        className: 'match-empty-icon',
      });
      emptyIcon.setAttribute('aria-hidden', 'true');
      emptyCard.append(
        emptyIcon,
        createElement('h3', {
          text: `No Candidate Matches Above ${Math.round(minScore * 100)}% Threshold`,
        }),
        createElement('p', {
          text: 'Try lowering the score filter threshold to see additional match candidates.',
          className: 'text-secondary',
        }),
      );
      container.replaceChildren(emptyCard);
      return;
    }

    container.replaceChildren(...filtered.map(createMatchCard));
    bindCardEvents();
  }

  function createImagePanel(label, pet, accentClass, includeName) {
    const panel = createElement('div', {
      className: 'image-panel',
    });
    const header = createElement('div', {
      className: 'image-panel-header',
    });
    header.append(createElement('span', {
      text: label,
      className: `image-panel-label ${accentClass}`,
    }));

    const initialSrc = pet.imageUrl || (pet.images && pet.images.length > 0 ? (typeof pet.images[0] === 'string' ? pet.images[0] : (pet.images[0].url || pet.images[0].imageUrl || pet.images[0].object)) : null);

    const zoomButton = createElement('button', {
      className: 'zoom-btn btn btn-secondary',
      text: initialSrc ? '🔍 Zoom' : 'Image unavailable',
    });
    zoomButton.type = 'button';
    if (initialSrc) {
      zoomButton.dataset.src = initialSrc;
      zoomButton.setAttribute('data-src', initialSrc);
    } else {
      zoomButton.disabled = true;
    }
    header.append(zoomButton);
    panel.append(header);

    let image = null;
    if (initialSrc) {
      image = createElement('img', {
        className: 'match-pet-image',
      });
      image.src = initialSrc;
      image.alt = includeName ? `${pet.petName} photo` : 'Found pet photo';
      panel.append(image);
    } else {
      const placeholder = createElement('div', {
        text: 'Image unavailable',
        className: 'image-unavailable',
      });
      placeholder.setAttribute('role', 'img');
      placeholder.setAttribute('aria-label', includeName ? `${pet.petName} image unavailable` : 'Found pet image unavailable');
      panel.append(placeholder);
    }

    if (pet.images && pet.images.length > 1) {
      const strip = createElement('div', {
        className: 'thumbnail-strip',
      });
      const thumbButtons = [];
      pet.images.forEach((img, i) => {
        const btn = createElement('button', {
          className: `thumbnail-btn${i === 0 ? ' is-active' : ''}`,
        });
        btn.type = 'button';
        btn.setAttribute('aria-current', i === 0 ? 'true' : 'false');
        btn.setAttribute('aria-label', 'View photo ' + (i + 1) + (img.tag ? ' (' + img.tag + ')' : ''));

        const thumbSrc = typeof img === 'string' ? img : (img.url || img.imageUrl || img.object || '');
        const thumbImg = createElement('img', {
          className: 'thumbnail-img',
        });
        thumbImg.src = thumbSrc;
        thumbImg.alt = includeName ? `${pet.petName} thumbnail ${i + 1}` : `Found pet thumbnail ${i + 1}`;
        btn.append(thumbImg);

        btn.addEventListener('click', () => {
          thumbButtons.forEach((btn, idx) => {
            btn.classList.toggle('is-active', idx === i);
            btn.setAttribute('aria-current', idx === i ? 'true' : 'false');
          });
          if (image) {
            image.src = thumbSrc;
            image.alt = includeName ? `${pet.petName} photo ${i + 1}` : `Found pet photo ${i + 1}`;
          }
          zoomButton.dataset.src = thumbSrc;
          zoomButton.setAttribute('data-src', thumbSrc);
          zoomButton.disabled = false;
          zoomButton.textContent = '🔍 Zoom';
        });

        thumbButtons.push(btn);
        strip.append(btn);
      });
      panel.append(strip);
    }

    panel.append(
      createElement('h4', {
        text: includeName ? `${pet.petName} (${pet.breed})` : `Found Pet (${pet.breed})`,
        className: 'pet-name',
      }),
      createElement('p', {
        text: `${includeName ? 'Last Seen' : 'Found At'}: ${pet.location}`,
        className: 'pet-location',
      }),
    );
    return panel;
  }

  function createScore(scoreGrid, label, value, className) {
    const score = createElement('div');
    const row = createElement('div', {
      className: 'score-row',
    });
    row.append(
      createElement('span', { text: label }),
      createElement('span', { text: `${Math.round(value * 100)}%`, className: 'score-value' }),
    );
    const progress = createElement('progress', {
      className: `score-progress ${className}`,
    });
    progress.max = 100;
    progress.value = Math.round(value * 100);
    progress.setAttribute('aria-label', label);
    score.append(row, progress);
    scoreGrid.append(score);
  }

  function createActionButton(text, className, matchId, action) {
    const button = createElement('button', { className, text });
    button.type = 'button';
    button.dataset.matchId = matchId;
    if (action) button.dataset.action = action;
    return button;
  }

  function createMatchCard(m) {
    const scorePct = Math.round(m.score * 100);
    const badgeClass = scorePct >= 90 ? 'match-badge-high' : scorePct >= 80 ? 'match-badge-medium' : 'match-badge-low';
    const statusBadgeText = m.status === 'CONFIRMED' ? 'CONFIRMED REUNION' :
      m.status === 'REJECTED' ? 'REJECTED MATCH' :
        m.status === 'REUNITED' ? 'REUNITED' : `${scorePct}% HIGH CONFIDENCE MATCH`;

    const card = createElement('article', { className: 'glass-card match-card' });
    card.dataset.matchId = m.matchId;

    const summary = createElement('div', {
      className: 'match-summary',
    });
    const identity = createElement('div', {
      className: 'match-identity',
    });
    identity.append(
      createElement('span', {
        text: statusBadgeText,
        className: `match-badge ${badgeClass}`,
      }),
      createElement('span', {
        text: `Match ID: ${m.matchId}`,
        className: 'match-id',
      }),
    );
    summary.append(
      identity,
      createElement('span', {
        text: `Calculated: ${m.matchedAt.toLocaleString()}`,
        className: 'match-date',
      }),
    );

    const comparison = createElement('div', {
      className: 'match-comparison',
    });
    comparison.append(
      createImagePanel('Reported Lost Pet', m.lostPet, 'image-panel-label-lost', true),
      createImagePanel('Found Pet Candidate', m.foundPet, 'image-panel-label-found', false),
    );

    const scores = createElement('div', {
      className: 'match-scores',
    });
    scores.append(createElement('h4', {
      text: '✨ Gemma 4 AI Similarity Scoring Breakdown',
      className: 'match-scores-title',
    }));
    const scoreGrid = createElement('div', {
      className: 'score-grid',
    });
    if (m.scores.vector !== undefined && m.scores.vector !== null) {
      createScore(scoreGrid, '✨ Multimodal AI Vector Match:', m.scores.vector, 'score-vector');
    }
    createScore(scoreGrid, 'Discrete Trait Match:', m.scores.trait !== undefined ? m.scores.trait : m.scores.visual, 'score-visual');
    if (m.scores.color !== undefined) {
      createScore(scoreGrid, 'Color Alignment:', m.scores.color, 'score-color');
    }
    createScore(scoreGrid, `Geospatial Proximity (${m.scores.distanceMiles} mi):`, m.scores.spatial, 'score-spatial');
    scores.append(scoreGrid);

    card.append(summary, comparison, scores);
    const controls = createElement('div', {
      className: 'match-controls',
    });
    if (identityEnabled) {
      const messageButton = createActionButton(
        'Open private messages', 'btn btn-secondary message-btn', m.matchId,
      );
      messageButton.dataset.matchStatus = m.status;
      controls.append(
        messageButton,
        createActionButton('Reject Match', 'btn btn-secondary action-btn', m.matchId, 'reject'),
        createActionButton('Confirm Match', 'btn btn-primary action-btn', m.matchId, 'confirm'),
      );
    } else {
      controls.append(
        createActionButton('💬 Contact Finder / Owner', 'btn btn-secondary contact-btn', m.matchId),
        createActionButton('Reject Match', 'btn btn-secondary action-btn', m.matchId, 'reject'),
        createActionButton('Confirm Reunion Match', 'btn btn-primary action-btn', m.matchId, 'confirm'),
      );
      const reunionButton = createActionButton('🎉 Mark as Reunited', 'btn btn-primary reunion-btn', m.matchId);
      reunionButton.dataset.petId = m.lostPet.petId;
      controls.append(reunionButton);
    }
    card.append(controls);
    return card;
  }

  async function loadThread(match, loadRevision, loadIdentityRevision) {
    setThreadStatus('Loading private messages...');
    try {
      const response = await fetch(`/api/v1/reunions/contact?matchId=${encodeURIComponent(match.matchId)}`);
      if (loadRevision !== threadRevision || loadIdentityRevision !== identityRevision ||
          activeThread?.matchId !== match.matchId) return false;
      if (!response.ok) throw new Error(`Private thread API returned status ${response.status}`);
      const payload = await response.json();
      if (loadRevision !== threadRevision || loadIdentityRevision !== identityRevision ||
          activeThread?.matchId !== match.matchId) return false;
      if (!payload || payload.matchId !== match.matchId || !Array.isArray(payload.messages)) {
        throw new Error('Private thread API returned an invalid payload');
      }
      renderThreadMessages(payload.messages);
      applyThreadWritableState(match.status);
      clearThreadStatus();
      return true;
    } catch (error) {
      if (loadRevision !== threadRevision || loadIdentityRevision !== identityRevision ||
          activeThread?.matchId !== match.matchId) return false;
      console.error('Failed to load private match messages:', error);
      threadMessages?.replaceChildren();
      if (threadEmpty) threadEmpty.hidden = true;
      setThreadStatus('Private messages could not be loaded. Try again.', true);
      return false;
    }
  }

  function openThreadModal(matchID, status) {
    const match = allMatches.find(candidate => candidate.matchId === matchID);
    if (!match || match.status !== status || !threadModal) return;
    threadRevision += 1;
    const loadRevision = threadRevision;
    const loadIdentityRevision = identityRevision;
    activeThread = { matchId: matchID, status };
    threadReturnTarget = matchID;

    if (activeEventSource) {
      activeEventSource.close();
      activeEventSource = null;
    }
    if (typingDebounceTimer) {
      clearTimeout(typingDebounceTimer);
      typingDebounceTimer = null;
    }
    if (typingAutoHideTimer) {
      clearTimeout(typingAutoHideTimer);
      typingAutoHideTimer = null;
    }
    lastTypingPingTime = 0;

    resetThreadView();
    applyThreadWritableState(status);

    if (threadResolvedBanner) {
      threadResolvedBanner.hidden = (status !== 'REUNITED');
    }
    if (btnChatResolve) {
      btnChatResolve.disabled = (status === 'REUNITED');
    }

    const matchIDInput = document.getElementById('match-thread-match-id');
    if (matchIDInput) matchIDInput.value = matchID;
    openModal(threadModal);
    threadModal.querySelector('.match-thread-close')?.focus();

    if (typeof EventSource === 'function') {
      setPresenceUI('connecting');
      const evtSource = new EventSource('/api/v1/reunions/events?matchId=' + encodeURIComponent(matchID));
      activeEventSource = evtSource;

      evtSource.addEventListener('open', () => {
        if (activeThread?.matchId !== matchID || activeEventSource !== evtSource) return;
        setPresenceUI('online');
      });

      evtSource.addEventListener('error', () => {
        if (activeThread?.matchId !== matchID || activeEventSource !== evtSource) return;
        setPresenceUI('connecting');
      });

      evtSource.addEventListener('message', (e) => {
        if (activeThread?.matchId !== matchID || activeEventSource !== evtSource) return;
        try {
          const msgData = JSON.parse(e.data);
          appendThreadMessage(msgData);
        } catch (err) {
          console.warn('Failed to parse SSE message:', err);
        }
      });

      evtSource.addEventListener('presence', (e) => {
        if (activeThread?.matchId !== matchID || activeEventSource !== evtSource) return;
        try {
          const payload = JSON.parse(e.data);
          if (!payload || typeof payload !== 'object') return;
          if (payload.status === 'typing') {
            const role = payload.senderRole;
            const roleLabel = role === 'reporter' ? 'Reporter' : role === 'finder' ? 'Finder' : 'Other participant';
            if (threadTypingText) threadTypingText.textContent = `${roleLabel} is typing...`;
            if (threadTyping) threadTyping.hidden = false;
            if (typingAutoHideTimer) clearTimeout(typingAutoHideTimer);
            typingAutoHideTimer = setTimeout(() => {
              if (threadTyping) threadTyping.hidden = true;
              typingAutoHideTimer = null;
            }, 3000);
          } else if (payload.status === 'idle') {
            if (typingAutoHideTimer) {
              clearTimeout(typingAutoHideTimer);
              typingAutoHideTimer = null;
            }
            if (threadTyping) threadTyping.hidden = true;
          }
        } catch (err) {
          console.warn('Failed to parse presence payload:', err);
        }
      });

      evtSource.addEventListener('reunion_resolved', () => {
        if (activeThread?.matchId !== matchID || activeEventSource !== evtSource) return;
        if (threadResolvedBanner) threadResolvedBanner.hidden = false;
        if (threadForm) threadForm.hidden = true;
        if (threadReadOnly) threadReadOnly.hidden = false;
        if (activeThread) activeThread.status = 'REUNITED';
        if (btnChatResolve) btnChatResolve.disabled = true;
        void fetchMatches();
      });
    }

    void loadThread(activeThread, loadRevision, loadIdentityRevision);
  }

  function bindCardEvents() {
    // Zoom Handler
    container.querySelectorAll('.zoom-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        lastZoomTrigger = e.currentTarget;
        const src = e.currentTarget.getAttribute('data-src');
        if (zoomedImage && zoomModal && src) {
          zoomedImage.src = src;
          openModal(zoomModal);
        }
      });
    });

    // Contact Handler
    container.querySelectorAll('.contact-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        lastContactTrigger = e.currentTarget;
        const matchId = e.currentTarget.getAttribute('data-match-id');
        const contactMatchIdInput = document.getElementById('contact-match-id');
        const contactModal = document.getElementById('contact-modal');
        if (contactMatchIdInput && contactModal) {
          contactMatchIdInput.value = matchId;
          openModal(contactModal);
          document.getElementById('contact-sender-email')?.focus();
        }
      });
    });

    // Authenticated participant message handler
    container.querySelectorAll('.message-btn').forEach(btn => {
      btn.addEventListener('click', (event) => {
        const button = event.currentTarget;
        openThreadModal(button.dataset.matchId || '', button.dataset.matchStatus || '');
      });
    });

    // Reunion Resolution Handler
    container.querySelectorAll('.reunion-btn').forEach(btn => {
      btn.addEventListener('click', (e) => {
        lastReunionTrigger = e.currentTarget;
        const matchId = e.currentTarget.getAttribute('data-match-id');
        const petId = e.currentTarget.getAttribute('data-pet-id');
        const reunionMatchIdInput = document.getElementById('reunion-match-id');
        const reunionPetIdInput = document.getElementById('reunion-pet-id');
        const reunionModal = document.getElementById('reunion-modal');
        if (reunionModal && reunionMatchIdInput && reunionPetIdInput) {
          reunionMatchIdInput.value = matchId;
          reunionPetIdInput.value = petId;
          openModal(reunionModal);
          document.getElementById('reunion-rating')?.focus();
        }
      });
    });

    // Action Handlers
    container.querySelectorAll('.action-btn').forEach(btn => {
      btn.addEventListener('click', async (e) => {
        const button = e.currentTarget;
        if (decisionInFlight || button.getAttribute('aria-disabled') === 'true') return;
        const matchId = button.getAttribute('data-match-id');
        const action = button.getAttribute('data-action');
        decisionInFlight = true;
        const decisionIdentityRevision = identityRevision;
        const decisionUsesIdentity = identityEnabled;
        button.dataset.idleText = button.textContent;
        setDecisionBusy(button, true);

        try {
          let identityState = null;
          if (identityEnabled && window.petspotrIdentity) {
            identityState = await window.petspotrIdentity.requireSession();
          }
          if (decisionUsesIdentity && decisionIdentityRevision !== identityRevision) return;

          const headers = { 'Content-Type': 'application/json' };
          if (identityState?.enabled) headers['X-CSRF-Token'] = identityState.csrfToken;
          const resp = await fetch('/api/v1/matches/action', {
            method: 'POST',
            headers,
            body: JSON.stringify({ matchId: matchId, action: action })
          });
          if (decisionUsesIdentity && decisionIdentityRevision !== identityRevision) return;

          if (!resp.ok) {
            const message = resp.status === 409
              ? 'A different decision was already recorded for this participant.'
              : 'Your decision could not be saved. Please try again.';
            showActionModal('', 'decision-error', message, { matchId, action });
            return;
          }
          const res = await resp.json();
          await fetchMatches();
          if (decisionUsesIdentity && decisionIdentityRevision !== identityRevision) return;
          showActionModal(res.status, action, '', { matchId, action });
        } catch (err) {
          console.error('Action error:', err);
          if (decisionUsesIdentity && decisionIdentityRevision !== identityRevision) return;
          if (err.code === 'identity-required') window.petspotrIdentity?.focusSignIn();
          const message = err.code === 'identity-required'
            ? 'Sign in again before recording a match decision.'
            : 'Your decision could not be saved. Please try again.';
          showActionModal('', 'decision-error', message, { matchId, action });
        } finally {
          decisionInFlight = false;
          setDecisionBusy(button, false);
        }
      });
    });
  }

  function handleTyping() {
    if (!activeThread?.matchId || !threadIsWritable(activeThread.status)) return;
    const msgVal = threadMessage?.value.trim() || '';
    if (!msgVal) {
      if (typingDebounceTimer) {
        clearTimeout(typingDebounceTimer);
        typingDebounceTimer = null;
      }
      void sendPresence(activeThread.matchId, 'idle');
      return;
    }
    const now = Date.now();
    if (now - lastTypingPingTime >= 1500) {
      lastTypingPingTime = now;
      void sendPresence(activeThread.matchId, 'typing');
    } else if (!typingDebounceTimer) {
      typingDebounceTimer = setTimeout(() => {
        typingDebounceTimer = null;
        if (activeThread?.matchId && threadIsWritable(activeThread.status) && (threadMessage?.value.trim() || '')) {
          lastTypingPingTime = Date.now();
          void sendPresence(activeThread.matchId, 'typing');
        }
      }, 1500 - (now - lastTypingPingTime));
    }
  }

  if (threadMessage) {
    threadMessage.addEventListener('input', () => {
      handleTyping();
    });
  }

  if (threadAttachBtn && threadFileInput) {
    threadAttachBtn.addEventListener('click', (e) => {
      e.preventDefault();
      if (threadSendInFlight) return;
      threadFileInput.click();
    });
    threadFileInput.addEventListener('change', (e) => {
      handleStagedFiles(e.target.files);
    });
  }

  if (btnChatResolve) {
    btnChatResolve.addEventListener('click', (e) => {
      e.preventDefault();
      if (!activeThread?.matchId || activeThread.status === 'REUNITED') return;
      const match = allMatches.find(candidate => candidate.matchId === activeThread.matchId);
      const petId = match?.lostPet?.petId || match?.matchedPetId || '';
      const reunionMatchIdInput = document.getElementById('reunion-match-id');
      const reunionPetIdInput = document.getElementById('reunion-pet-id');
      const reunionModal = document.getElementById('reunion-modal');
      if (reunionModal && reunionMatchIdInput && reunionPetIdInput) {
        reunionMatchIdInput.value = activeThread.matchId;
        reunionPetIdInput.value = petId;
        lastReunionTrigger = btnChatResolve;
        openModal(reunionModal);
        document.getElementById('reunion-rating')?.focus();
      }
    });
  }

  if (threadForm) {
    threadForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      if (threadSendInFlight || !activeThread || !threadIsWritable(activeThread.status)) return;
      const message = threadMessage?.value.trim() || '';
      if (!validMediatedMessageBody(message)) {
        setThreadStatus('Enter a message of up to 1,000 characters.', true);
        threadMessage?.focus();
        return;
      }

      const attempt = pendingThreadAttempt?.matchId === activeThread.matchId &&
        pendingThreadAttempt.message === message
        ? pendingThreadAttempt
        : { matchId: activeThread.matchId, message, key: newThreadIdempotencyKey() };
      pendingThreadAttempt = attempt;
      const sendRevision = threadRevision;
      const sendIdentityRevision = identityRevision;
      const sendToken = Symbol('private-thread-send');
      activeThreadSendToken = sendToken;
      setThreadSendBusy(true);

      try {
        const identityState = await window.petspotrIdentity?.requireSession();
        if (sendRevision !== threadRevision || sendIdentityRevision !== identityRevision) return;
        if (!identityState?.enabled) throw new Error('identity-required');

        const uploadedImages = [];
        if (stagedPhotos.length > 0) {
          for (let i = 0; i < stagedPhotos.length; i++) {
            const staged = stagedPhotos[i];
            setThreadStatus(`Uploading photo ${i + 1} of ${stagedPhotos.length}...`);
            const uploadHeaders = { 'Content-Type': 'application/json' };
            if (identityState.csrfToken) uploadHeaders['X-CSRF-Token'] = identityState.csrfToken;

            const res = await fetch('/api/v1/uploads/presigned-url', {
              method: 'POST',
              headers: uploadHeaders,
              body: JSON.stringify({
                fileName: staged.file.name,
                contentType: staged.file.type || 'image/jpeg',
              }),
            });
            if (sendRevision !== threadRevision || sendIdentityRevision !== identityRevision) return;
            if (!res.ok) {
              const uploadError = new Error(`Presigned upload URL request failed (${res.status})`);
              uploadError.status = res.status;
              throw uploadError;
            }
            if (presigned.uploadUrl) {
              const putRes = await fetch(presigned.uploadUrl, {
                method: 'PUT',
                headers: { 'Content-Type': staged.file.type || 'image/jpeg' },
                body: staged.file,
              });
              if (!putRes.ok) {
                const putErr = new Error(`Direct photo upload failed (${putRes.status})`);
                putErr.status = putRes.status;
                throw putErr;
              }
            }
            const key = presigned.fileName || presigned.object || staged.file.name;
            uploadedImages.push(key);
          }
        }

        const reqPayload = {
          matchId: attempt.matchId,
          message: attempt.message,
        };
        if (uploadedImages.length > 0) {
          reqPayload.images = uploadedImages;
        }

        setThreadStatus('Sending private message...');
        const response = await fetch('/api/v1/reunions/contact', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': identityState.csrfToken,
            'Idempotency-Key': attempt.key,
          },
          body: JSON.stringify(reqPayload),
        });
        if (sendRevision !== threadRevision || sendIdentityRevision !== identityRevision) return;
        if (!response.ok) {
          const error = new Error(`Private message API returned status ${response.status}`);
          error.status = response.status;
          throw error;
        }
        pendingThreadAttempt = null;
        clearStagedPhotos();
        if (threadMessage) threadMessage.value = '';
        if (typingDebounceTimer) {
          clearTimeout(typingDebounceTimer);
          typingDebounceTimer = null;
        }
        void sendPresence(activeThread.matchId, 'idle');
        const refreshed = await loadThread(activeThread, sendRevision, sendIdentityRevision);
        if (sendRevision !== threadRevision || sendIdentityRevision !== identityRevision) return;
        if (!refreshed) {
          setThreadStatus(
            'Private message was sent, but the conversation could not be refreshed. Close and reopen private messages.',
            true,
          );
          threadMessage?.focus();
          return;
        }
        setThreadStatus('Private message sent.');
        threadMessage?.focus();
      } catch (error) {
        if (sendRevision !== threadRevision || sendIdentityRevision !== identityRevision) return;
        console.error('Private message submit error:', error);
        const message = error.status === 409
          ? 'This message conflicts with the conversation or the conversation is read-only.'
          : 'Private message not sent. Try again.';
        setThreadStatus(message, true);
        threadMessage?.focus();
      } finally {
        if (activeThreadSendToken === sendToken) {
          activeThreadSendToken = null;
          if (sendRevision === threadRevision) setThreadSendBusy(false);
        }
      }
    });
  }

  // Bind Contact Form Submission
  const contactForm = document.getElementById('contact-form');
  if (contactForm) {
    contactForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const matchId = document.getElementById('contact-match-id')?.value || '';
      const senderEmail = document.getElementById('contact-sender-email')?.value || '';
      const message = document.getElementById('contact-message')?.value || '';

      try {
        const resp = await fetch('/api/v1/reunions/contact', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ matchId, senderEmail, message })
        });

        if (resp.ok) {
          closeContactModal();
          showActionModal('CONFIRMED', 'contact');
        }
      } catch (err) {
        console.error('Contact submit error:', err);
      }
    });
  }

  // Bind Reunion Resolution Form Submission
  const reunionForm = document.getElementById('reunion-form');
  if (reunionForm) {
    reunionForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const matchId = document.getElementById('reunion-match-id')?.value || '';
      const petId = document.getElementById('reunion-pet-id')?.value || '';
      const rating = parseInt(document.getElementById('reunion-rating')?.value || '5', 10);
      const feedback = document.getElementById('reunion-feedback')?.value || '';

      try {
        const headers = { 'Content-Type': 'application/json' };
        const csrfToken = await getCsrfToken();
        if (csrfToken) headers['X-CSRF-Token'] = csrfToken;

        const resp = await fetch('/api/v1/reunions/resolve', {
          method: 'POST',
          headers,
          body: JSON.stringify({ matchId, petId, rating, feedback })
        });

        if (resp.ok) {
          closeReunionModal();
          showActionModal('REUNITED', 'resolve');
          if (activeThread?.matchId === matchId) {
            const resolvedBanner = document.getElementById('match-thread-resolved-banner');
            if (resolvedBanner) resolvedBanner.hidden = false;
            if (threadForm) threadForm.hidden = true;
            if (threadReadOnly) threadReadOnly.hidden = false;
            activeThread.status = 'REUNITED';
            const btnChat = document.getElementById('btn-chat-resolve-reunion');
            if (btnChat) btnChat.disabled = true;
          }
          await fetchMatches();
        }
      } catch (err) {
        console.error('Reunion resolve error:', err);
      }
    });
  }

  function showActionModal(status, action, errorMessage = '', returnTarget = null) {
    const modal = document.getElementById('match-action-modal');
    const icon = document.getElementById('action-modal-icon');
    const title = document.getElementById('action-modal-title');
    const desc = document.getElementById('action-modal-desc');

    if (title && desc && modal) {
      decisionReturnTarget = returnTarget;
      if (action === 'decision-error') {
        if (icon) icon.hidden = true;
        title.textContent = 'Decision not saved';
        desc.textContent = errorMessage;
      } else if (action === 'confirm' && status === 'PENDING_REVIEW') {
        if (icon) icon.hidden = false;
        title.textContent = 'Decision recorded';
        desc.textContent = 'Waiting for the other participant to confirm this match.';
      } else if (action === 'confirm') {
        if (icon) icon.hidden = false;
        title.textContent = 'Match confirmed';
        desc.textContent = 'Both participants confirmed this match.';
      } else {
        if (icon) icon.hidden = false;
        title.textContent = 'Match Rejected';
        desc.textContent = 'Match candidate removed from active list and feedback logged.';
      }
      openModal(modal);
      modal.querySelector('.modal-close')?.focus();
    }
  }

  if (scoreFilter) {
    scoreFilter.addEventListener('change', () => {
      if (matchResultsLoaded) renderMatches();
    });
  }

  if (zoomModal) {
    zoomModal.addEventListener('click', () => closeZoomModal());
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && !zoomModal.hidden) {
        closeZoomModal();
      }
    });
  }

  const contactModal = document.getElementById('contact-modal');
  contactModal?.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeContactModal();
    }
  });

  const reunionModal = document.getElementById('reunion-modal');
  reunionModal?.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeReunionModal();
    }
  });

  document.querySelectorAll('.modal-close').forEach(button => {
    button.addEventListener('click', (event) => {
      const modal = event.currentTarget.closest('.modal-overlay');
      if (modal?.id === 'match-action-modal') {
        closeActionModal();
      } else if (modal?.id === 'contact-modal') {
        closeContactModal();
      } else if (modal?.id === 'reunion-modal') {
        closeReunionModal();
      } else {
        closeModal(modal);
      }
    });
  });

  document.querySelectorAll('.match-thread-close').forEach(button => {
    button.addEventListener('click', () => closeThreadModal());
  });

  threadModal?.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeThreadModal();
      return;
    }
    if (event.key !== 'Tab') return;
    const focusable = Array.from(threadModal.querySelectorAll(
      'button:not([disabled]):not([hidden]), textarea:not([disabled]):not([hidden]), input:not([disabled]):not([hidden])',
    )).filter(element => element.getClientRects().length > 0);
    if (focusable.length === 0) return;
    const headerClose = threadModal.querySelector('.modal-heading-actions .match-thread-close') || focusable[0];
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && (document.activeElement === first || document.activeElement === headerClose)) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      (headerClose || first).focus();
    }
  });

  window.addEventListener('beforeunload', () => {
    if (activeThread?.matchId) {
      void sendPresence(activeThread.matchId, 'idle', true);
    }
    if (activeEventSource) {
      activeEventSource.close();
      activeEventSource = null;
    }
  });

  const actionModal = document.getElementById('match-action-modal');
  actionModal?.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      closeActionModal();
      return;
    }
    if (event.key === 'Tab') {
      event.preventDefault();
      actionModal.querySelector('.modal-close')?.focus();
    }
  });

  async function applyIdentityState(stateSnapshot) {
    identityEnabled = Boolean(stateSnapshot.enabled);
    if (!identityEnabled) {
      await fetchMatches();
      return;
    }
    if (stateSnapshot.unavailable || stateSnapshot.busy || !stateSnapshot.principal) {
      replaceMatchState('', 'match-auth-required');
      return;
    }
    await fetchMatches();
  }

  function scheduleIdentityState(stateSnapshot) {
    const principalKey = stateSnapshot.principal
      ? `${stateSnapshot.principal.issuer}\u0000${stateSnapshot.principal.subject}`
      : '';
    const stateKey = [
      stateSnapshot.enabled,
      stateSnapshot.unavailable,
      stateSnapshot.busy,
      principalKey,
      stateSnapshot.csrfToken,
    ].join('|');
    if (stateKey === lastIdentityState) return;
    lastIdentityState = stateKey;
    identityRevision += 1;
    if (activeThread && stateSnapshot.enabled && (
      stateSnapshot.unavailable || stateSnapshot.busy || !stateSnapshot.principal ||
      (identityPrincipalKey && identityPrincipalKey !== principalKey)
    )) {
      threadIdentityFocusPending = Boolean(threadModal?.contains(document.activeElement));
      closeThreadModal(false);
    }
    if (threadIdentityFocusPending && stateSnapshot.enabled && !stateSnapshot.busy) {
      threadIdentityFocusPending = false;
      queueMicrotask(() => {
        if (stateSnapshot.unavailable) {
          scoreFilter?.focus();
        } else if (stateSnapshot.principal) {
          document.getElementById('identity-sign-out')?.focus();
        } else {
          window.petspotrIdentity?.focusSignIn();
        }
      });
    }
    identityPrincipalKey = principalKey;
    void applyIdentityState(stateSnapshot);
  }

  if (window.petspotrIdentity) {
    document.addEventListener('petspotr:identity-changed', (event) => {
      scheduleIdentityState(event.detail);
    });
    window.petspotrIdentity.ready.then(scheduleIdentityState);
  } else {
    void fetchMatches();
  }
});
